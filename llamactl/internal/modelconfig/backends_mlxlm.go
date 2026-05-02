package modelconfig

import "strings"

func init() {
	RegisterDetect("mlxlm", func(cmd string) bool {
		return strings.Contains(cmd, "mlx_lm.server")
	})
	Register(BackendSchema{
		Name: "mlxlm",
		Parameters: []Parameter{
			{
				Flag:        "--prompt-cache-bytes",
				ID:          "cache_bytes",
				Label:       "KV Cache Size",
				Type:        ParamTypeBytes,
				Min:         536870912,  // 512 MB
				Max:         8589934592, // 8 GB
				Step:        536870912,  // 512 MB steps
				Default:     2147483648, // 2 GB
				Unit:        "bytes",
				Description: "Maximum KV cache RAM. All model weights always on Metal — this caps how much context history is kept in memory.",
			},
			{
				Flag:        "--prompt-cache-size",
				ID:          "cache_size",
				Label:       "Prompt Cache Slots",
				Type:        ParamTypeInt,
				Min:         1,
				Max:         8,
				Step:        1,
				Default:     2,
				Unit:        "slots",
				Description: "Cached prompts retained across requests. Higher = faster repeat-context calls, more RAM.",
			},
			{
				Flag:        "--prefill-step-size",
				ID:          "prefill_step",
				Label:       "Prefill Step Size",
				Type:        ParamTypeInt,
				Min:         64,
				Max:         1024,
				Step:        64,
				Default:     256,
				Unit:        "tokens",
				Description: "Tokens processed per prefill chunk. Lower = less peak RAM spike on long prompts. Higher = faster prefill on short prompts.",
			},
			{
				Flag:        "--max-tokens",
				ID:          "max_tokens",
				Label:       "Max Output Tokens",
				Type:        ParamTypeInt,
				Min:         1024,
				Max:         65536,
				Step:        1024,
				Default:     8192,
				Unit:        "tokens",
				Description: "Maximum tokens generated per request. Does not affect RAM. Prevents runaway generation.",
			},
		},
		Presets: []Preset{
			{Name: "quality", Label: "Quality", Values: map[string]int{
				"cache_bytes": 4294967296, "cache_size": 3, "prefill_step": 512, "max_tokens": 32768,
			}},
			{Name: "balanced", Label: "Balanced", Values: map[string]int{
				"cache_bytes": 2147483648, "cache_size": 2, "prefill_step": 256, "max_tokens": 16384,
			}},
			{Name: "lightweight", Label: "Lightweight", Values: map[string]int{
				"cache_bytes": 1073741824, "cache_size": 1, "prefill_step": 128, "max_tokens": 8192,
			}},
		},
	})
}
