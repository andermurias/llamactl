// Package service contains the core business logic for llamactl.
//
// This package is intentionally free of UI/presentation concerns — it returns
// plain structs and errors. This makes it easy to:
//   - Reuse from a future HTTP API or web controller
//   - Unit-test without mocking terminal output
//   - Extend by junior contributors without touching the UI layer
package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/andermurias/llamactl/internal/config"
	"github.com/andermurias/llamactl/internal/launchd"
	"github.com/andermurias/llamactl/internal/llamaswap"
)

// ── Status ─────────────────────────────────────────────────────────────────

// LlamaSwapStatus holds the full runtime state of the llama-swap service.
// All fields are safe to read even when the service is not running.
type LlamaSwapStatus struct {
	IsInstalled  bool     // plist file exists on disk
	IsLoaded     bool     // service is bootstrapped into launchd
	IsRunning    bool     // OS process is alive
	PID          int      // process ID; 0 when not running
	Uptime       string   // human-readable uptime from ps(1)
	AutoStart    bool     // RunAtLoad value in the plist
	APIAddr      string   // listen address, e.g. "127.0.0.1:8080"
	ConfigFile   string   // path to llama-swap.yaml
	LogFile      string   // path to the log file
	LoadedModels []string // model IDs currently resident in memory
	APIReachable bool     // true if /running endpoint returned 200
}

// GetStatus queries launchd and the llama-swap API, returning a snapshot
// of the service's current state. Never returns nil.
func GetStatus(cfg *config.Config) *LlamaSwapStatus {
	s := &LlamaSwapStatus{
		APIAddr:    cfg.Listen,
		ConfigFile: cfg.ConfigFile,
		LogFile:    cfg.LogFile,
	}

	s.IsInstalled = fileExists(cfg.PlistPath)
	if !s.IsInstalled {
		return s
	}

	svc := launchd.LlamaSwapSvc(cfg)
	s.IsLoaded = launchd.IsLoaded(svc)
	if !s.IsLoaded {
		return s
	}

	s.PID = launchd.GetPID(svc)
	// launchd sometimes reports state=not-running (e.g. after a previous crash)
	// even while the process is alive.  Fall back to pgrep as a second source.
	if s.PID == 0 {
		s.PID = pgrepFirst("llama-swap")
	}
	s.IsRunning = s.PID > 0
	if s.IsRunning {
		s.Uptime = processUptime(s.PID)
	}

	s.AutoStart = launchd.ReadAutoStartCfg(cfg)
	s.APIReachable = llamaswap.IsReachable(cfg)
	if s.APIReachable {
		s.LoadedModels, _ = llamaswap.GetRunning(cfg)
	}

	return s
}

// ── Lifecycle ──────────────────────────────────────────────────────────────

// Start ensures the service is installed, then kicks it off.
// Returns the PID on success. Idempotent: if already running, returns current PID.
func Start(cfg *config.Config) (int, error) {
	if !fileExists(cfg.PlistPath) {
		if err := Install(cfg, false); err != nil {
			return 0, fmt.Errorf("auto-install: %w", err)
		}
	}

	svc := launchd.LlamaSwapSvc(cfg)
	if launchd.IsRunning(svc) {
		return launchd.GetPID(svc), nil
	}

	if err := launchd.Kickstart(svc); err != nil {
		return 0, fmt.Errorf("kickstart: %w", err)
	}

	for i := 0; i < 8; i++ {
		time.Sleep(2 * time.Second)
		if launchd.IsRunning(svc) {
			return launchd.GetPID(svc), nil
		}
	}

	return 0, fmt.Errorf("service did not start — check: llamactl logs")
}

// Stop sends SIGTERM to the running service via launchd.
func Stop(cfg *config.Config) error {
	svc := launchd.LlamaSwapSvc(cfg)
	if !launchd.IsLoaded(svc) {
		return fmt.Errorf("service is not loaded")
	}
	return launchd.KillSvc(svc, "SIGTERM")
}

// Install writes the launchd plist and bootstraps the service (does NOT start it).
func Install(cfg *config.Config, autoStart bool) error {
	if !fileExists(cfg.LlamaSwapBin) {
		return fmt.Errorf("llama-swap binary not found at %s\n  Install with: brew install llama-swap", cfg.LlamaSwapBin)
	}
	if !fileExists(cfg.ConfigFile) {
		return fmt.Errorf("config not found at %s", cfg.ConfigFile)
	}

	if err := launchd.WriteLlamaSwapPlist(cfg, autoStart); err != nil {
		return fmt.Errorf("write plist: %w", err)
	}

	svc := launchd.LlamaSwapSvc(cfg)
	if launchd.IsLoaded(svc) {
		_ = launchd.Bootout(svc)
		time.Sleep(time.Second)
	}

	if err := launchd.Bootstrap(svc); err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}
	return nil
}

// Uninstall removes the service from launchd (does not delete the plist file).
func Uninstall(cfg *config.Config) error {
	svc := launchd.LlamaSwapSvc(cfg)
	if launchd.IsLoaded(svc) {
		_ = launchd.Bootout(svc)
	}
	return nil
}

// Enable sets RunAtLoad=true and reloads the service.
func Enable(cfg *config.Config) error {
	if err := launchd.WriteLlamaSwapPlist(cfg, true); err != nil {
		return err
	}
	svc := launchd.LlamaSwapSvc(cfg)
	if launchd.IsLoaded(svc) {
		_ = launchd.Bootout(svc)
		time.Sleep(time.Second)
	}
	return launchd.Bootstrap(svc)
}

// Disable sets RunAtLoad=false and reloads the service.
func Disable(cfg *config.Config) error {
	if err := launchd.WriteLlamaSwapPlist(cfg, false); err != nil {
		return err
	}
	svc := launchd.LlamaSwapSvc(cfg)
	if launchd.IsLoaded(svc) {
		_ = launchd.Bootout(svc)
		time.Sleep(time.Second)
	}
	return launchd.Bootstrap(svc)
}

// ── Models ─────────────────────────────────────────────────────────────────

// ModelsInfo aggregates all model-related data for the "models" command.
type ModelsInfo struct {
	APIModels    []llamaswap.Model    // registered models from /v1/models
	LoadedIDs    map[string]bool      // model IDs currently in memory
	GGUFFiles    []llamaswap.FileInfo // local .gguf files under ~/AI/models/
	HFModels     []string             // directory names in ~/.cache/huggingface/hub/
	HFTotalBytes int64                // combined size of the HF cache
	APIReachable bool                 // false when llama-swap is not running
	MetaMap      map[string]ModelMeta // backend info keyed by model ID
}

// ModelMeta holds static metadata extracted from llama-swap.yaml for a single model.
type ModelMeta struct {
	Backend   string // "MLX", "GGUF", "TTS", "STT", "API"
	CtxSize   int    // context window size in k (0 = not found)
	HFID      string // useModelName / HuggingFace model ID
	SizeBytes int64  // file size (GGUF) or HF cache size (MLX), 0 if unknown
}

// GetModelsInfo collects model data from all sources. Never returns nil.
func GetModelsInfo(cfg *config.Config) *ModelsInfo {
	info := &ModelsInfo{LoadedIDs: make(map[string]bool)}

	models, err := llamaswap.GetModels(cfg)
	if err == nil {
		info.APIModels = models
		info.APIReachable = true
	}

	running, _ := llamaswap.GetRunning(cfg)
	for _, r := range running {
		info.LoadedIDs[r] = true
	}

	info.GGUFFiles, _ = llamaswap.GGUFFiles(cfg)
	info.HFModels, info.HFTotalBytes, _ = llamaswap.HFCachedModels()
	info.MetaMap = parseModelMeta(cfg.ConfigFile)

	return info
}

// parseModelMeta reads the llama-swap YAML config and extracts backend type,
// context window size, and HF model ID for each model entry.
func parseModelMeta(configFile string) map[string]ModelMeta {
	meta := make(map[string]ModelMeta)

	data, err := os.ReadFile(configFile)
	if err != nil {
		return meta
	}

	var root struct {
		Models map[string]struct {
			Cmd          string `yaml:"cmd"`
			UseModelName string `yaml:"useModelName"`
		} `yaml:"models"`
	}
	if err := yaml.Unmarshal(data, &root); err != nil {
		return meta
	}

	for id, entry := range root.Models {
		m := ModelMeta{
			Backend:  detectBackend(entry.Cmd),
			HFID:     entry.UseModelName,
		}
		// Extract --ctx-size from cmd, store in k units for display
		if idx := strings.Index(entry.Cmd, "--ctx-size"); idx >= 0 {
			rest := strings.TrimSpace(entry.Cmd[idx+len("--ctx-size"):])
			rest = strings.TrimLeft(rest, " =")
			parts := strings.Fields(rest)
			if len(parts) > 0 {
				if n, err := strconv.Atoi(parts[0]); err == nil {
					m.CtxSize = n / 1024 // store as k (e.g. 32 for 32768)
				}
			}
		}
		// For MLX models, try to get cached model size from HF cache
		if m.Backend == "MLX" && m.HFID != "" {
			m.SizeBytes = hfCacheModelSize(m.HFID)
		}
		meta[id] = m
	}
	return meta
}

// hfCacheModelSize returns the total size in bytes of a model's HF cache directory.
func hfCacheModelSize(hfID string) int64 {
	home, _ := os.UserHomeDir()
	parts := strings.SplitN(hfID, "/", 2)
	if len(parts) != 2 {
		return 0
	}
	dirName := "models--" + strings.ReplaceAll(parts[0], "/", "--") + "--" + strings.ReplaceAll(parts[1], "/", "--")
	modelDir := filepath.Join(home, ".cache", "huggingface", "hub", dirName)
	var size int64
	_ = filepath.Walk(modelDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}

// detectBackend classifies a model's command string into a backend type.
func detectBackend(cmd string) string {
	cmd = strings.ToLower(cmd)
	switch {
	case strings.Contains(cmd, "mlx_lm") || strings.Contains(cmd, "mlx-lm"):
		return "MLX"
	case strings.Contains(cmd, "llama-server") || strings.Contains(cmd, "llama_server") || strings.Contains(cmd, "llama.cpp"):
		return "GGUF"
	case strings.Contains(cmd, "whisper"):
		return "STT"
	case strings.Contains(cmd, "kokoro"):
		return "TTS"
	default:
		return "API"
	}
}

// ── Helpers (unexported) ───────────────────────────────────────────────────

// fileExists returns true if path exists on the filesystem.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// processUptime returns a human-readable uptime string for the given PID
// by calling ps(1). Returns "?" if ps fails.
func processUptime(pid int) string {
	out, err := exec.Command("ps", "-p", fmt.Sprintf("%d", pid), "-o", "etime=").Output()
	if err != nil {
		return "?"
	}
	return strings.TrimSpace(string(out))
}

// pgrepFirst returns the PID of the first running process whose argv[0]
// matches name, or 0 if none found. Used as a fallback when launchd
// reports no PID despite the process being alive (can happen after a
// prior crash while launchd resets its internal state).
func pgrepFirst(name string) int {
	out, err := exec.Command("pgrep", "-x", name).Output()
	if err != nil {
		return 0
	}
	lines := strings.Fields(strings.TrimSpace(string(out)))
	if len(lines) == 0 {
		return 0
	}
	pid, _ := strconv.Atoi(lines[0])
	return pid
}
