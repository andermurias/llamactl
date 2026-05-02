package modelconfig

import (
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// defaultCachePath returns ~/AI/llamactl-model-meta.yaml.
func defaultCachePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "AI", "llamactl-model-meta.yaml")
}

// LoadMetaCache reads the metadata cache file. Returns an empty cache if absent.
func LoadMetaCache(path string) (MetaCache, error) {
	if path == "" {
		path = defaultCachePath()
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return MetaCache{Models: map[string]ModelMeta{}}, nil
	}
	if err != nil {
		return MetaCache{}, err
	}
	var c MetaCache
	if err := yaml.Unmarshal(data, &c); err != nil {
		return MetaCache{}, err
	}
	if c.Models == nil {
		c.Models = map[string]ModelMeta{}
	}
	return c, nil
}

// SaveMetaCache writes the cache to disk (creates file if absent).
func SaveMetaCache(path string, c MetaCache) error {
	if path == "" {
		path = defaultCachePath()
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// UpsertMeta adds or updates one model's metadata in the cache and saves.
func UpsertMeta(cachePath string, meta ModelMeta) error {
	c, err := LoadMetaCache(cachePath)
	if err != nil {
		return err
	}
	meta.UpdatedAt = time.Now()
	c.Models[meta.ModelID] = meta
	return SaveMetaCache(cachePath, c)
}

// GetMeta returns metadata for one model from the cache. ok=false if absent.
func GetMeta(cachePath, modelID string) (ModelMeta, bool) {
	c, err := LoadMetaCache(cachePath)
	if err != nil {
		return ModelMeta{}, false
	}
	m, ok := c.Models[modelID]
	return m, ok
}
