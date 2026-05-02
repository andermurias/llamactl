package modelconfig

import "strings"

func init() {
	RegisterDetect("llamaserver", func(cmd string) bool {
		return strings.Contains(cmd, "llama-server")
	})
	Register(BackendSchema{
		Name: "llamaserver",
		Parameters: []Parameter{
			{
				Flag:        "-ngl",
				ID:          "ngl",
				Label:       "GPU Layers",
				Type:        ParamTypeInt,
				Min:         0,
				Max:         0, // 0 = use meta.NumLayers at render time
				Step:        1,
				Default:     99,
				Unit:        "layers",
				Description: "Layers offloaded to Metal GPU. 0 = CPU-only. Use num_layers for full offload. Lower = less GPU RAM, slower tok/s.",
			},
			{
				Flag:        "--ctx-size",
				ID:          "ctx_size",
				Label:       "Context Size",
				Type:        ParamTypeInt,
				Min:         512,
				Max:         131072,
				Step:        512,
				Default:     4096,
				Unit:        "tokens",
				Description: "Maximum context window. Each 1K tokens adds ~0.5 MB × num_kv_heads × layers to KV cache.",
			},
			{
				Flag:        "--threads",
				ID:          "threads",
				Label:       "CPU Threads",
				Type:        ParamTypeInt,
				Min:         1,
				Max:         10, // M4 Mini: 4P + 6E cores
				Step:        1,
				Default:     4,
				Unit:        "threads",
				Description: "CPU threads for layers not on GPU. More = faster on offloaded models, busier Mac. Irrelevant if ngl = num_layers.",
			},
			{
				Flag:        "--parallel",
				ID:          "parallel",
				Label:       "Parallel Slots",
				Type:        ParamTypeInt,
				Min:         1,
				Max:         4,
				Step:        1,
				Default:     1,
				Unit:        "slots",
				Description: "Concurrent request slots. Each slot reserves a full KV cache. Recommended: 1 for large models.",
			},
		},
		Presets: []Preset{
			// Sentinel values in ngl:
			//   0  → all layers (meta.num_layers)   — JS resolves
			//  -1  → 70% of layers                  — JS resolves
			//  -2  → 50% of layers                  — JS resolves
			{Name: "quality", Label: "Quality", Values: map[string]int{
				"ngl": 0, "ctx_size": 8192, "threads": 4, "parallel": 1,
			}},
			{Name: "balanced", Label: "Balanced", Values: map[string]int{
				"ngl": -1, "ctx_size": 4096, "threads": 4, "parallel": 1,
			}},
			{Name: "lightweight", Label: "Lightweight", Values: map[string]int{
				"ngl": -2, "ctx_size": 2048, "threads": 3, "parallel": 1,
			}},
		},
	})
}
