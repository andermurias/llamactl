package modelmanager

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Manager handles enable / disable / remove operations on installed models.
type Manager struct {
	ConfigFile   string // ~/AI/llama-swap.yaml
	DisabledFile string // ~/AI/llamactl-disabled.yaml
	ModelsDir    string // ~/AI/models (used to find GGUF files to delete)
}

// NewManager creates a Manager.
func NewManager(configFile, disabledFile, modelsDir string) *Manager {
	return &Manager{
		ConfigFile:   configFile,
		DisabledFile: disabledFile,
		ModelsDir:    modelsDir,
	}
}

// ── Enable ────────────────────────────────────────────────────────────────────

// Enable moves a model from the disabled store back into llama-swap.yaml.
func (m *Manager) Enable(modelID string) error {
	store, err := LoadDisabledStore(m.DisabledFile)
	if err != nil {
		return fmt.Errorf("load disabled store: %w", err)
	}

	entry, ok := store.Disabled[modelID]
	if !ok {
		return fmt.Errorf("model %q is not in the disabled list", modelID)
	}

	// Check it's not already in the live config (shouldn't happen, but be safe)
	exists, err := ModelExistsInConfig(m.ConfigFile, modelID)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("model %q already exists in config", modelID)
	}

	group := entry.Group
	if group == "" {
		group = "llm"
	}

	if err := AddModelToConfig(m.ConfigFile, modelID, group, entry.Config); err != nil {
		return fmt.Errorf("add to config: %w", err)
	}

	// Remove from disabled store
	delete(store.Disabled, modelID)
	return SaveDisabledStore(m.DisabledFile, store)
}

// ── Disable ───────────────────────────────────────────────────────────────────

// Disable moves a model from llama-swap.yaml into the disabled store.
// The model config is preserved so it can be re-enabled later.
func (m *Manager) Disable(modelID string) error {
	cfg, group, err := RemoveModelFromConfig(m.ConfigFile, modelID)
	if err != nil {
		return fmt.Errorf("remove from config: %w", err)
	}

	store, err := LoadDisabledStore(m.DisabledFile)
	if err != nil {
		return fmt.Errorf("load disabled store: %w", err)
	}

	store.Disabled[modelID] = DisabledEntry{
		Config: *cfg,
		Group:  group,
		HFID:   extractHFID(cfg.UseModelName),
		Type:   detectModelType(cfg.Cmd),
	}

	return SaveDisabledStore(m.DisabledFile, store)
}

// ── Remove ────────────────────────────────────────────────────────────────────

// Remove deletes a model from llama-swap.yaml (and optionally its GGUF file).
// Unlike Disable, Remove does NOT save the config to the disabled store.
func (m *Manager) Remove(modelID string, deleteFiles bool) error {
	// Try live config first
	exists, err := ModelExistsInConfig(m.ConfigFile, modelID)
	if err != nil {
		return err
	}

	var cfg *ModelConfig
	if exists {
		var removeErr error
		cfg, _, removeErr = RemoveModelFromConfig(m.ConfigFile, modelID)
		if removeErr != nil {
			return fmt.Errorf("remove from config: %w", removeErr)
		}
	} else {
		// Check disabled store
		store, serr := LoadDisabledStore(m.DisabledFile)
		if serr != nil {
			return fmt.Errorf("load disabled store: %w", serr)
		}
		entry, ok := store.Disabled[modelID]
		if !ok {
			return fmt.Errorf("model %q not found in config or disabled list", modelID)
		}
		cfg = &entry.Config
		delete(store.Disabled, modelID)
		if saveErr := SaveDisabledStore(m.DisabledFile, store); saveErr != nil {
			return saveErr
		}
	}

	// Optionally delete the GGUF file
	if deleteFiles && cfg != nil {
		ggufPath := extractGGUFPath(cfg.Cmd)
		if ggufPath != "" {
			if err := os.Remove(ggufPath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("delete GGUF file %s: %w", ggufPath, err)
			}
		}
		// Also clean up HF cache for MLX models
		if detectModelType(cfg.Cmd) == ModelTypeMLX {
			// HF cache cleanup is best-effort; don't fail if it can't be done
			_ = cleanHFCache(cfg.UseModelName)
		}
	}

	return nil
}

// ── List disabled ─────────────────────────────────────────────────────────────

// ListDisabled returns the IDs and entries of all disabled models.
func (m *Manager) ListDisabled() (map[string]DisabledEntry, error) {
	store, err := LoadDisabledStore(m.DisabledFile)
	if err != nil {
		return nil, err
	}
	return store.Disabled, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// detectModelType returns MLX or GGUF based on the cmd string.
func detectModelType(cmd string) ModelType {
	if strings.Contains(cmd, "mlx_lm") {
		return ModelTypeMLX
	}
	return ModelTypeGGUF
}

// extractGGUFPath finds the --model /path/to/file.gguf argument in a command string.
func extractGGUFPath(cmd string) string {
	for _, line := range strings.Split(cmd, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "--model ") {
			path := strings.TrimPrefix(line, "--model ")
			path = strings.TrimSpace(path)
			if strings.HasSuffix(path, ".gguf") {
				return path
			}
		}
	}
	return ""
}

// extractHFID returns the HF model ID from a useModelName or cmd string.
func extractHFID(useModelName string) string {
	if strings.Contains(useModelName, "/") {
		return useModelName
	}
	return ""
}

// cleanHFCache removes a model from the HuggingFace local cache.
// Best-effort: errors are silently ignored.
func cleanHFCache(hfID string) error {
	if hfID == "" || !strings.Contains(hfID, "/") {
		return nil
	}
	home, _ := os.UserHomeDir()
	hfCacheDir := filepath.Join(home, ".cache", "huggingface", "hub")

	// HF cache uses "models--owner--name" directory naming
	parts := strings.SplitN(hfID, "/", 2)
	if len(parts) != 2 {
		return nil
	}
	cacheDirName := "models--" + parts[0] + "--" + parts[1]
	cachePath := filepath.Join(hfCacheDir, cacheDirName)

	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		return nil
	}
	return os.RemoveAll(cachePath)
}
