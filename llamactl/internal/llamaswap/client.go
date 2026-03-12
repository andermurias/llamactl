package llamaswap

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/andermurias/llamactl/internal/config"
)

// Model represents a model entry from /v1/models.
type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`
}

// RunningModel represents a currently-loaded model from /running.
type RunningModel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

var httpClient = &http.Client{Timeout: 5 * time.Second}

func baseURL(cfg *config.Config) string {
	return "http://" + cfg.Listen
}

// GetModels calls /v1/models and returns the list.
func GetModels(cfg *config.Config) ([]Model, error) {
	resp, err := httpClient.Get(baseURL(cfg) + "/v1/models")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var body struct {
		Data []Model `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	return body.Data, nil
}

// GetRunning calls /running and returns loaded model IDs.
func GetRunning(cfg *config.Config) ([]string, error) {
	resp, err := httpClient.Get(baseURL(cfg) + "/running")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var body struct {
		Running []json.RawMessage `json:"running"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}

	var ids []string
	for _, raw := range body.Running {
		var s string
		if json.Unmarshal(raw, &s) == nil {
			ids = append(ids, s)
			continue
		}
		var obj map[string]interface{}
		if json.Unmarshal(raw, &obj) == nil {
			if id, ok := obj["id"].(string); ok {
				ids = append(ids, id)
			} else if name, ok := obj["name"].(string); ok {
				ids = append(ids, name)
			}
		}
	}
	return ids, nil
}

// IsReachable returns true if the llama-swap API is responding.
func IsReachable(cfg *config.Config) bool {
	resp, err := httpClient.Get(baseURL(cfg) + "/running")
	return err == nil && resp.StatusCode == 200
}

// FileInfo holds a file path and its size in bytes.
type FileInfo struct {
	Path string
	Size int64
}

// GGUFFiles returns all .gguf files under cfg.AIDir/models with their sizes.
func GGUFFiles(cfg *config.Config) ([]FileInfo, error) {
	modelsDir := filepath.Join(cfg.AIDir, "models")
	var files []FileInfo
	err := filepath.WalkDir(modelsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if strings.HasSuffix(strings.ToLower(d.Name()), ".gguf") {
			info, _ := d.Info()
			files = append(files, FileInfo{Path: path, Size: info.Size()})
		}
		return nil
	})
	return files, err
}

// HFCachedModels returns model directory names under ~/.cache/huggingface/hub.
func HFCachedModels() ([]string, int64, error) {
	cacheDir := filepath.Join(os.Getenv("HOME"), ".cache", "huggingface", "hub")
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return nil, 0, err
	}
	var names []string
	var total int64
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "models--") {
			continue
		}
		parts := strings.Split(e.Name(), "--")
		if len(parts) >= 3 {
			names = append(names, parts[len(parts)-1])
		}
		if info, err := dirSize(filepath.Join(cacheDir, e.Name())); err == nil {
			total += info
		}
	}
	return names, total, nil
}

func dirSize(path string) (int64, error) {
	var size int64
	err := filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		info, err := d.Info()
		if err == nil {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// FormatBytes formats bytes as human-readable (GB/MB).
func FormatBytes(b int64) string {
	const gb = 1024 * 1024 * 1024
	const mb = 1024 * 1024
	switch {
	case b >= gb:
		return fmt.Sprintf("%.1fG", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.0fM", float64(b)/float64(mb))
	default:
		return fmt.Sprintf("%dB", b)
	}
}
