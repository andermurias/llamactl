package modelmanager

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	defaultQuantization = "Q4_K_M"
	defaultTTL          = 300
	defaultGroup        = "llm"

	// mlxServerBin is the conda-environment mlx_lm.server binary.
	mlxServerBin = "/opt/homebrew/Caskroom/miniforge/base/envs/mlx-server/bin/mlx_lm.server"

	// llamaServerBin is the llama.cpp server binary expected on PATH.
	llamaServerBin = "llama-server"
)

// Installer carries the paths it needs to write files and configs.
type Installer struct {
	AIDir      string // ~/AI
	ConfigFile string // ~/AI/llama-swap.yaml
	LogDir     string // ~/AI/logs
	ModelsDir  string // ~/AI/models/chat
}

// NewInstaller creates an Installer from the standard AI directory.
func NewInstaller(aiDir, configFile, logDir string) *Installer {
	return &Installer{
		AIDir:      aiDir,
		ConfigFile: configFile,
		LogDir:     logDir,
		ModelsDir:  filepath.Join(aiDir, "models", "chat"),
	}
}

// ── Main install entry point ──────────────────────────────────────────────────

// Install detects the best backend (MLX vs GGUF) and installs the model.
// It writes the model entry to llama-swap.yaml and, for GGUF, downloads the file.
// progress is an optional function called with status messages as install proceeds.
func (ins *Installer) Install(req InstallRequest, progress func(string)) (*InstallResult, error) {
	if progress == nil {
		progress = func(string) {}
	}

	// Normalize the HF ID (strip URL prefix, etc.)
	req.HFID = NormalizeHFID(req.HFID)
	if req.HFID == "" {
		return nil, fmt.Errorf("empty HuggingFace model ID")
	}

	// Auto-derive model ID if not specified
	if req.ModelID == "" {
		req.ModelID = DeriveModelID(req.HFID)
	}
	if req.Group == "" {
		req.Group = defaultGroup
	}
	if req.TTL == 0 {
		req.TTL = defaultTTL
	}
	if req.Quantization == "" {
		req.Quantization = defaultQuantization
	}

	// Check for duplicate
	exists, err := ModelExistsInConfig(ins.ConfigFile, req.ModelID)
	if err != nil {
		return nil, fmt.Errorf("check config: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("model %q already exists in config — disable it first or choose a different model_id", req.ModelID)
	}

	// Determine install type
	installType := req.ForceType
	if installType == "" {
		installType = ins.detectBestType(req.HFID, req.Quantization, progress)
	}

	switch installType {
	case ModelTypeMLX:
		return ins.installMLX(req, progress)
	case ModelTypeGGUF:
		return ins.installGGUF(req, progress)
	default:
		return nil, fmt.Errorf("unsupported model type: %q", installType)
	}
}

// ── Type detection ────────────────────────────────────────────────────────────

// detectBestType chooses MLX if an mlx-community variant exists, otherwise GGUF.
func (ins *Installer) detectBestType(hfID, quant string, progress func(string)) ModelType {
	progress(fmt.Sprintf("Checking for MLX variant of %s…", hfID))
	mlxInfo := FindMLXVariant(hfID)
	if mlxInfo.Found {
		progress(fmt.Sprintf("✓ MLX variant found: %s", mlxInfo.HFID))
		return ModelTypeMLX
	}
	progress("No MLX variant found — will use GGUF (llama.cpp)")
	return ModelTypeGGUF
}

// ── MLX install ───────────────────────────────────────────────────────────────

// installMLX adds an MLX model to llama-swap.yaml.
// MLX models are downloaded from HuggingFace on first use by mlx_lm.server,
// so no file download is needed here.
func (ins *Installer) installMLX(req InstallRequest, progress func(string)) (*InstallResult, error) {
	// Resolve the actual MLX HF ID (may differ from the original)
	mlxHFID := req.HFID
	if !strings.HasPrefix(req.HFID, "mlx-community/") {
		info := FindMLXVariant(req.HFID)
		if info.Found {
			mlxHFID = info.HFID
		}
		// If still not mlx-community, treat the given ID as-is (user may have
		// passed mlx-community/<model> directly)
	}

	// Fix the model ID if we changed the HFID
	if req.ModelID == DeriveModelID(req.HFID) && mlxHFID != req.HFID {
		req.ModelID = DeriveModelID(mlxHFID)
	}

	// Re-check after potential ID change
	if exists, _ := ModelExistsInConfig(ins.ConfigFile, req.ModelID); exists {
		return nil, fmt.Errorf("model %q already exists in config", req.ModelID)
	}

	progress(fmt.Sprintf("Configuring MLX model: %s → %s", mlxHFID, req.ModelID))

	cmd := fmt.Sprintf("%s\n--model %s\n--host 127.0.0.1\n--port ${PORT}\n--log-level WARNING",
		mlxServerBin, mlxHFID)

	config := ModelConfig{
		Cmd:          cmd,
		UseModelName: mlxHFID,
		TTL:          req.TTL,
		CheckEndpoint: "/health",
	}

	if err := AddModelToConfig(ins.ConfigFile, req.ModelID, req.Group, config); err != nil {
		return nil, fmt.Errorf("update config: %w", err)
	}

	progress(fmt.Sprintf("✓ Added %q to llama-swap.yaml (MLX — model will download on first use)", req.ModelID))

	return &InstallResult{
		ModelID:    req.ModelID,
		Type:       ModelTypeMLX,
		HFID:       mlxHFID,
		ConfigPath: ins.ConfigFile,
		Message:    fmt.Sprintf("MLX model configured. Run 'llamactl restart' then call the model to trigger download (~%.0f GB).", float64(estimateMLXSize(mlxHFID))),
	}, nil
}

// ── GGUF install ──────────────────────────────────────────────────────────────

// installGGUF downloads a GGUF model and adds it to llama-swap.yaml.
func (ins *Installer) installGGUF(req InstallRequest, progress func(string)) (*InstallResult, error) {
	// Find the GGUF file to download
	progress(fmt.Sprintf("Looking for GGUF files in %s…", req.HFID))

	ggufRepo := req.GGUFRepo
	var files []HFFile
	var err error

	if ggufRepo != "" {
		files, err = FindGGUFFiles(ggufRepo, req.Quantization)
	} else {
		files, err = FindGGUFFiles(req.HFID, req.Quantization)
		if err == nil && len(files) == 0 {
			// No GGUFs in the original repo — search for a dedicated GGUF repo
			progress("No GGUF files in original repo, searching for GGUF variants…")
			ggufRepo, files, err = FindGGUFRepo(req.HFID, req.Quantization)
			if err != nil {
				return nil, fmt.Errorf("find GGUF repo: %w", err)
			}
		}
	}
	if err != nil {
		return nil, fmt.Errorf("find GGUF files: %w", err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no GGUF files found for %s with quantization %s", req.HFID, req.Quantization)
	}

	best := PickBestGGUF(files, req.Quantization)
	if best == nil {
		return nil, fmt.Errorf("could not select a GGUF file")
	}

	// Use the GGUF repo ID if we found one, otherwise the original
	repoForDownload := req.HFID
	if ggufRepo != "" {
		repoForDownload = ggufRepo
	}

	progress(fmt.Sprintf("✓ Will download: %s/%s", repoForDownload, best.Filename))

	// Ensure models directory exists
	if err := os.MkdirAll(ins.ModelsDir, 0o755); err != nil {
		return nil, fmt.Errorf("create models dir: %w", err)
	}

	destPath := filepath.Join(ins.ModelsDir, best.Filename)

	// Download using huggingface-cli (available in mlx-server conda env)
	progress(fmt.Sprintf("Downloading %s (this may take several minutes)…", best.Filename))
	if err := ins.downloadGGUF(repoForDownload, best.Filename, destPath, progress); err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}

	progress(fmt.Sprintf("✓ Downloaded to %s", destPath))

	// Generate llama-server command
	logFile := filepath.Join(ins.LogDir, req.ModelID+".log")
	cmd := fmt.Sprintf("llama-server\n--model %s\n--alias %s\n--host 127.0.0.1\n--port ${PORT}\n-ngl 99\n--ctx-size 32768\n--parallel 1\n--log-file %s",
		destPath, req.ModelID, logFile)

	config := ModelConfig{
		Cmd:           cmd,
		UseModelName:  req.ModelID,
		TTL:           req.TTL,
		CheckEndpoint: "/v1/models",
	}

	if err := AddModelToConfig(ins.ConfigFile, req.ModelID, req.Group, config); err != nil {
		return nil, fmt.Errorf("update config: %w", err)
	}

	progress(fmt.Sprintf("✓ Added %q to llama-swap.yaml (GGUF)", req.ModelID))

	return &InstallResult{
		ModelID:    req.ModelID,
		Type:       ModelTypeGGUF,
		HFID:       req.HFID,
		FilePath:   destPath,
		ConfigPath: ins.ConfigFile,
		Message:    fmt.Sprintf("GGUF model installed. Run 'llamactl restart' to apply."),
	}, nil
}

// downloadGGUF uses huggingface-cli to download a single GGUF file.
// Falls back to curl/wget if huggingface-cli is not available.
func (ins *Installer) downloadGGUF(repoID, filename, destPath string, progress func(string)) error {
	// Try huggingface-cli first (preferred — handles auth, resume, etc.)
	hfCLI := findHFCLI()
	if hfCLI != "" {
		cmd := exec.Command(hfCLI,
			"download", repoID, filename,
			"--local-dir", ins.ModelsDir,
		)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err == nil {
			return nil
		}
		// huggingface-cli failed — fall through to direct download
		progress("huggingface-cli failed, trying direct download…")
	}

	// Direct URL download via curl
	downloadURL := fmt.Sprintf("https://huggingface.co/%s/resolve/main/%s", repoID, filename)
	progress(fmt.Sprintf("curl %s", downloadURL))

	curl := exec.Command("curl", "-L", "--progress-bar", "-o", destPath, downloadURL)
	curl.Stdout = os.Stdout
	curl.Stderr = os.Stderr
	return curl.Run()
}

// findHFCLI returns the path to the huggingface-cli binary, or "" if not found.
func findHFCLI() string {
	// Check conda mlx-server env first
	candidates := []string{
		"/opt/homebrew/Caskroom/miniforge/base/envs/mlx-server/bin/huggingface-cli",
		"/opt/homebrew/Caskroom/miniforge/base/bin/huggingface-cli",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	// Try PATH
	if p, err := exec.LookPath("huggingface-cli"); err == nil {
		return p
	}
	return ""
}

// estimateMLXSize returns an approximate GB size for display purposes,
// derived from common model naming conventions.
func estimateMLXSize(hfID string) float64 {
	lower := strings.ToLower(hfID)
	switch {
	case strings.Contains(lower, "3b"):
		return 2
	case strings.Contains(lower, "7b") || strings.Contains(lower, "8b") || strings.Contains(lower, "9b"):
		return 4
	case strings.Contains(lower, "12b") || strings.Contains(lower, "13b"):
		return 7
	case strings.Contains(lower, "14b"):
		return 8
	case strings.Contains(lower, "24b") || strings.Contains(lower, "22b"):
		return 14
	case strings.Contains(lower, "70b") || strings.Contains(lower, "72b"):
		return 40
	}
	return 5 // fallback estimate
}
