// Package modelmanager handles HuggingFace model discovery, installation,
// and lifecycle management (enable / disable / remove) for the llama-swap stack.
//
// Design principles:
//   - No UI code here — returns structs and errors only.
//   - Disabled models are stored in ~/AI/llamactl-disabled.yaml so the main
//     llama-swap.yaml is never corrupted.
//   - MLX is always preferred over GGUF on Apple Silicon (better performance,
//     no manual download needed).
//   - Default GGUF quantization: Q4_K_M (good quality / size balance).
package modelmanager

// ── HuggingFace API types ─────────────────────────────────────────────────────

// HFModel represents a model returned by the HuggingFace search/info API.
type HFModel struct {
	ID          string   `json:"modelId"`
	Downloads   int      `json:"downloads"`
	Likes       int      `json:"likes"`
	Tags        []string `json:"tags"`
	PipelineTag string   `json:"pipeline_tag"`
	Siblings    []HFFile `json:"siblings,omitempty"`
}

// HFFile is one file in a HuggingFace model repository.
type HFFile struct {
	Filename string `json:"rfilename"`
	Size     int64  `json:"size,omitempty"`
}

// MLXInfo describes the result of checking for an MLX variant of a model.
type MLXInfo struct {
	Found bool
	HFID  string // e.g., "mlx-community/gemma-3-12b-it-4bit"
}

// ── llama-swap model config ───────────────────────────────────────────────────

// ModelConfig is the data that goes into the models: section of llama-swap.yaml.
type ModelConfig struct {
	Cmd           string `yaml:"cmd"`
	UseModelName  string `yaml:"useModelName,omitempty"`
	TTL           int    `yaml:"ttl"`
	CheckEndpoint string `yaml:"checkEndpoint,omitempty"`
	Proxy         string `yaml:"proxy,omitempty"`
}

// ModelType classifies how a model is served.
type ModelType string

const (
	ModelTypeMLX  ModelType = "mlx"
	ModelTypeGGUF ModelType = "gguf"
)

// ── Disabled model store ──────────────────────────────────────────────────────

// DisabledEntry stores everything needed to re-enable a model.
type DisabledEntry struct {
	Config ModelConfig `yaml:"config"`
	Group  string      `yaml:"group,omitempty"` // group the model belonged to
	HFID   string      `yaml:"hf_id,omitempty"` // original HuggingFace model ID
	Type   ModelType   `yaml:"type,omitempty"`  // mlx or gguf
}

// DisabledStore is the top-level structure of ~/AI/llamactl-disabled.yaml.
type DisabledStore struct {
	Disabled map[string]DisabledEntry `yaml:"disabled"`
}

// ── Install request / result ─────────────────────────────────────────────────

// InstallRequest describes what to install.
type InstallRequest struct {
	HFID         string    `json:"hf_id"`         // HuggingFace model ID or URL
	ModelID      string    `json:"model_id"`      // key in llama-swap.yaml (auto-derived if empty)
	ForceType    ModelType `json:"force_type"`    // "mlx" | "gguf" | "" (auto)
	Quantization string    `json:"quantization"`  // GGUF quant, e.g., "Q4_K_M" (default)
	Group        string    `json:"group"`         // group to add to (default: "llm")
	TTL          int       `json:"ttl"`           // idle timeout seconds (default: 300)
	GGUFRepo     string    `json:"gguf_repo"`     // explicit GGUF repo override
}

// InstallResult is returned when an install operation completes.
type InstallResult struct {
	ModelID    string    `json:"model_id"`
	Type       ModelType `json:"type"`
	HFID       string    `json:"hf_id"`
	FilePath   string    `json:"file_path,omitempty"` // GGUF only
	ConfigPath string    `json:"config_path"`
	Message    string    `json:"message"`
}

// ManageAction represents enable / disable / remove operations.
type ManageAction string

const (
	ActionEnable  ManageAction = "enable"
	ActionDisable ManageAction = "disable"
	ActionRemove  ManageAction = "remove"
)

// ManageRequest is the request body for POST /api/models/manage.
type ManageRequest struct {
	ModelID     string       `json:"model_id"`
	Action      ManageAction `json:"action"`
	DeleteFiles bool         `json:"delete_files"` // for remove: also delete GGUF from disk
}
