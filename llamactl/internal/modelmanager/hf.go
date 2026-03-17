package modelmanager

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const hfAPIBase = "https://huggingface.co/api"

// hfClient is shared so we don't open a new connection per request.
var hfClient = &http.Client{Timeout: 30 * time.Second}

// ── Search ───────────────────────────────────────────────────────────────────

// SearchModels searches HuggingFace for models matching the query.
// modelType can be "text-generation", "mlx", "gguf", or "" for all.
// Results are sorted by downloads descending.
func SearchModels(query, modelType string, limit int) ([]HFModel, error) {
	if limit <= 0 {
		limit = 20
	}
	u, _ := url.Parse(hfAPIBase + "/models")
	q := u.Query()
	q.Set("search", query)
	q.Set("sort", "downloads")
	q.Set("direction", "-1")
	q.Set("limit", fmt.Sprintf("%d", limit))
	if modelType != "" {
		q.Set("filter", modelType)
	}
	u.RawQuery = q.Encode()

	resp, err := hfClient.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("HuggingFace API unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HuggingFace API returned %d", resp.StatusCode)
	}

	var models []HFModel
	if err := json.NewDecoder(resp.Body).Decode(&models); err != nil {
		return nil, fmt.Errorf("parse HF response: %w", err)
	}
	return models, nil
}

// ── Model info ───────────────────────────────────────────────────────────────

// GetModelInfo returns full info for a specific HF model ID (includes file list).
// Returns nil, nil if the model does not exist (404).
func GetModelInfo(hfID string) (*HFModel, error) {
	resp, err := hfClient.Get(hfAPIBase + "/models/" + hfID)
	if err != nil {
		return nil, fmt.Errorf("HF API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HF API returned %d for %s", resp.StatusCode, hfID)
	}

	var m HFModel
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, fmt.Errorf("parse HF model info: %w", err)
	}
	return &m, nil
}

// ── MLX detection ────────────────────────────────────────────────────────────

// FindMLXVariant checks if an MLX version of a model exists in mlx-community.
// It tries several common naming conventions (4bit, 8bit, bf16, plain).
func FindMLXVariant(hfID string) MLXInfo {
	// Extract model basename (e.g., "google/gemma-3-12b-it" → "gemma-3-12b-it")
	basename := modelBasename(hfID)

	candidates := []string{
		"mlx-community/" + basename + "-4bit",
		"mlx-community/" + basename + "-8bit",
		"mlx-community/" + basename + "-bf16",
		"mlx-community/" + basename,
	}

	for _, candidate := range candidates {
		m, err := GetModelInfo(candidate)
		if err == nil && m != nil {
			return MLXInfo{Found: true, HFID: candidate}
		}
	}
	return MLXInfo{Found: false}
}

// ── GGUF detection ───────────────────────────────────────────────────────────

// FindGGUFFiles returns .gguf files from a HF model repo.
// If preferredQuant is set (e.g., "Q4_K_M"), matching files are returned first.
// If the original repo has no .gguf files, it returns an empty slice (caller
// should try FindGGUFRepo to find a dedicated GGUF repo).
func FindGGUFFiles(hfID, preferredQuant string) ([]HFFile, error) {
	m, err := GetModelInfo(hfID)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, fmt.Errorf("model not found: %s", hfID)
	}

	var ggufFiles []HFFile
	for _, f := range m.Siblings {
		if strings.HasSuffix(strings.ToLower(f.Filename), ".gguf") {
			ggufFiles = append(ggufFiles, f)
		}
	}

	if len(ggufFiles) == 0 {
		return nil, nil // caller should try FindGGUFRepo
	}

	return filterByQuant(ggufFiles, preferredQuant), nil
}

// FindGGUFRepo searches for a dedicated GGUF repository for a given model.
// Common patterns: bartowski/<basename>-GGUF, TheBloke/<basename>-GGUF.
// Returns the repo ID and GGUF files, or empty if none found.
func FindGGUFRepo(hfID, preferredQuant string) (string, []HFFile, error) {
	basename := modelBasename(hfID)

	// Common GGUF repo naming patterns (order = priority)
	candidates := []string{
		"bartowski/" + basename + "-GGUF",
		"bartowski/" + strings.ReplaceAll(basename, "-", "_") + "-GGUF",
		"TheBloke/" + basename + "-GGUF",
		"TheBloke/" + basename + "-GGUF",
	}

	for _, candidate := range candidates {
		files, err := FindGGUFFiles(candidate, preferredQuant)
		if err == nil && len(files) > 0 {
			return candidate, files, nil
		}
	}

	// Last resort: search HF for GGUF variants
	results, err := SearchModels(basename+" GGUF", "gguf", 5)
	if err != nil {
		return "", nil, nil // graceful degradation
	}
	for _, r := range results {
		files, err := FindGGUFFiles(r.ID, preferredQuant)
		if err == nil && len(files) > 0 {
			return r.ID, files, nil
		}
	}

	return "", nil, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// NormalizeHFID converts a HuggingFace URL to a model ID.
// e.g., "https://huggingface.co/google/gemma-3-12b-it" → "google/gemma-3-12b-it"
func NormalizeHFID(input string) string {
	input = strings.TrimSpace(input)
	if strings.HasPrefix(input, "https://huggingface.co/") {
		input = strings.TrimPrefix(input, "https://huggingface.co/")
	}
	if strings.HasPrefix(input, "http://huggingface.co/") {
		input = strings.TrimPrefix(input, "http://huggingface.co/")
	}
	return strings.TrimSuffix(input, "/")
}

// modelBasename returns the last component of a HF model ID.
// "google/gemma-3-12b-it" → "gemma-3-12b-it"
func modelBasename(hfID string) string {
	parts := strings.SplitN(hfID, "/", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return hfID
}

// DeriveModelID creates a short llamactl model key from a HF model ID.
// "mlx-community/gemma-3-12b-it-4bit" → "gemma-3-12b-it-mlx"
// "google/gemma-3-12b-it"             → "gemma-3-12b-it"
func DeriveModelID(hfID string) string {
	basename := modelBasename(hfID)
	basename = strings.ToLower(basename)

	// MLX repos often end in -4bit, -8bit, -bf16 — strip and add -mlx suffix
	for _, suffix := range []string{"-4bit", "-8bit", "-bf16"} {
		if strings.HasSuffix(basename, suffix) {
			return strings.TrimSuffix(basename, suffix) + "-mlx"
		}
	}
	// If coming from mlx-community, always add -mlx
	if strings.HasPrefix(hfID, "mlx-community/") {
		return basename + "-mlx"
	}
	return basename
}

// filterByQuant returns gguf files matching the preferred quantization.
// If no match, returns the full list sorted so Q4_K_M-like files come first.
func filterByQuant(files []HFFile, preferredQuant string) []HFFile {
	if preferredQuant == "" {
		preferredQuant = "Q4_K_M"
	}
	upper := strings.ToUpper(preferredQuant)

	var matched, rest []HFFile
	for _, f := range files {
		if strings.Contains(strings.ToUpper(f.Filename), upper) {
			matched = append(matched, f)
		} else {
			rest = append(rest, f)
		}
	}
	if len(matched) > 0 {
		return matched
	}
	return rest
}

// PickBestGGUF selects the single best GGUF file to download.
// Preference: exact quant match → any Q4 → first in list.
func PickBestGGUF(files []HFFile, preferredQuant string) *HFFile {
	if len(files) == 0 {
		return nil
	}
	filtered := filterByQuant(files, preferredQuant)
	return &filtered[0]
}
