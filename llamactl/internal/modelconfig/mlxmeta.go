package modelconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// hfConfigJSON is the subset of fields we read from HF config.json.
type hfConfigJSON struct {
	NumHiddenLayers   int `json:"num_hidden_layers"`
	NumAttentionHeads int `json:"num_attention_heads"`
	NumKVHeads        int `json:"num_key_value_heads"`
	HiddenSize        int `json:"hidden_size"`
	MaxPositionEmbeds int `json:"max_position_embeddings"`
}

// ReadMLXMeta locates config.json in the HuggingFace cache for hfID and
// returns model architecture metadata. hfID is e.g. "mlx-community/gemma-3-12b-it-4bit".
func ReadMLXMeta(hfID string) (GGUFMeta, error) {
	configPath, err := findHFConfig(hfID)
	if err != nil {
		return GGUFMeta{}, err
	}
	return parseHFConfig(configPath)
}

// ReadMLXMetaFromPath reads config.json at an explicit path (for testing).
func ReadMLXMetaFromPath(configPath string) (GGUFMeta, error) {
	return parseHFConfig(configPath)
}

func parseHFConfig(configPath string) (GGUFMeta, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return GGUFMeta{}, fmt.Errorf("read config.json: %w", err)
	}
	var cfg hfConfigJSON
	if err := json.Unmarshal(data, &cfg); err != nil {
		return GGUFMeta{}, fmt.Errorf("parse config.json: %w", err)
	}
	if cfg.NumHiddenLayers == 0 {
		return GGUFMeta{}, fmt.Errorf("config.json missing num_hidden_layers")
	}
	return GGUFMeta{
		NumLayers:  cfg.NumHiddenLayers,
		NumHeads:   cfg.NumAttentionHeads,
		NumKVHeads: cfg.NumKVHeads,
		HiddenSize: cfg.HiddenSize,
		MaxContext: cfg.MaxPositionEmbeds,
	}, nil
}

// findHFConfig locates config.json for hfID in the HuggingFace disk cache.
// Layout: ~/.cache/huggingface/hub/models--<org>--<name>/snapshots/<hash>/config.json
func findHFConfig(hfID string) (string, error) {
	hfCacheDir := filepath.Join(os.Getenv("HOME"), ".cache", "huggingface", "hub")
	safeID := "models--" + strings.ReplaceAll(hfID, "/", "--")
	snapshotsDir := filepath.Join(hfCacheDir, safeID, "snapshots")

	entries, err := os.ReadDir(snapshotsDir)
	if err != nil {
		return "", fmt.Errorf("HF cache not found for %q (expected %s): %w", hfID, snapshotsDir, err)
	}

	for _, e := range entries {
		if e.IsDir() {
			candidate := filepath.Join(snapshotsDir, e.Name(), "config.json")
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
		}
	}
	return "", fmt.Errorf("config.json not found in HF cache for %q", hfID)
}
