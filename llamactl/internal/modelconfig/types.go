// Package modelconfig provides backend-specific parameter schemas, cmd parsing,
// model metadata reading, and RAM/TPS estimation for the Configure panel.
package modelconfig

import "time"

// ParameterType controls how the frontend renders the control.
type ParameterType string

const (
	ParamTypeInt   ParameterType = "int"
	ParamTypeBytes ParameterType = "bytes" // stored as int bytes, displayed as GB
	ParamTypeBool  ParameterType = "bool"
)

// Parameter describes one configurable flag for a backend.
type Parameter struct {
	Flag        string        `json:"flag"`        // e.g. "-ngl", "--ctx-size"
	ID          string        `json:"id"`          // e.g. "ngl", "ctx_size" (JS key)
	Label       string        `json:"label"`       // human label shown in UI
	Type        ParameterType `json:"type"`
	Min         int           `json:"min"`
	Max         int           `json:"max"` // 0 means "use meta value at render time"
	Step        int           `json:"step"`
	Default     int           `json:"default"`
	Unit        string        `json:"unit"`        // "layers", "tokens", "threads"
	Description string        `json:"description"`
}

// Preset is a named collection of parameter values (Quality / Balanced / Lightweight).
type Preset struct {
	Name   string         `json:"name"`   // "quality"
	Label  string         `json:"label"`  // "Quality"
	Values map[string]int `json:"values"` // param ID → value
}

// BackendSchema is the full schema for one backend.
type BackendSchema struct {
	Name       string      `json:"name"`
	Parameters []Parameter `json:"parameters"`
	Presets    []Preset    `json:"presets"`
}

// ModelMeta holds architecture metadata read from a local file (GGUF header or
// config.json). Stored in ~/AI/llamactl-model-meta.yaml and never re-fetched
// once cached.
type ModelMeta struct {
	ModelID       string    `yaml:"model_id"        json:"model_id"`
	Backend       string    `yaml:"backend"         json:"backend"`
	NumLayers     int       `yaml:"num_layers"      json:"num_layers"`
	NumHeads      int       `yaml:"num_heads"       json:"num_heads"`
	NumKVHeads    int       `yaml:"num_kv_heads"    json:"num_kv_heads"`
	HiddenSize    int       `yaml:"hidden_size"     json:"hidden_size"`
	MaxContext    int       `yaml:"max_context"     json:"max_context"`
	FileSizeBytes int64     `yaml:"file_size_bytes" json:"file_size_bytes"`
	UpdatedAt     time.Time `yaml:"updated_at"      json:"updated_at"`
}

// MetaCache is the top-level structure of ~/AI/llamactl-model-meta.yaml.
type MetaCache struct {
	Models map[string]ModelMeta `yaml:"models"`
}

// RAMEstimate is the output of EstimateRAM.
type RAMEstimate struct {
	WeightsGB    float64 `json:"weights_gb"`
	KVCacheGB    float64 `json:"kv_cache_gb"`
	OverheadGB   float64 `json:"overhead_gb"`
	TotalGB      float64 `json:"total_gb"`
	BudgetGB     float64 `json:"budget_gb"`
	FitsInBudget bool    `json:"fits_in_budget"`
	EstTPS       float64 `json:"est_tps"`
}

// ConfigState is the full response for GET /api/models/config.
type ConfigState struct {
	ModelID  string         `json:"model_id"`
	Backend  string         `json:"backend"`
	Schema   BackendSchema  `json:"schema"`
	Meta     ModelMeta      `json:"meta"`
	Values   map[string]int `json:"values"`
	Estimate RAMEstimate    `json:"estimate"`
}

// ConfigSaveRequest is the body for POST /api/models/config.
type ConfigSaveRequest struct {
	ModelID string         `json:"model_id"`
	Values  map[string]int `json:"values"`
}
