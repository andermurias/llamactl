package modelconfig

import "math"

const (
	// budgetGB is the practical RAM budget for models on a 16GB M4 Mini.
	budgetGB = 10.5

	// overheadGB is the fixed per-process overhead (Metal context, runtime, heap).
	overheadGB = 0.35

	// kvBytesPerElement is fp16 (2 bytes) — default KV cache dtype in llama.cpp and MLX.
	kvBytesPerElement = 2
)

// EstimateRAM calculates estimated RAM usage for the given backend, model
// metadata, and configuration values.
//
// Unified memory note: on Apple Silicon, GPU and CPU share the same physical
// pool. For llamaserver with partial offload, ngl only determines which
// subsystem processes each layer — total RAM usage is roughly constant.
func EstimateRAM(backend string, meta ModelMeta, values map[string]int) RAMEstimate {
	weightsGB := float64(meta.FileSizeBytes) / 1e9
	var kvCacheGB float64

	switch backend {
	case "llamaserver":
		ctxSize := getOrDefault(values, "ctx_size", 4096)
		parallel := getOrDefault(values, "parallel", 1)
		kvCacheGB = estimateKVCacheLlama(meta, ctxSize, parallel)
	case "mlxlm":
		cacheBytes := getOrDefault(values, "cache_bytes", 2_147_483_648)
		kvCacheGB = float64(cacheBytes) / 1e9
	}

	totalGB := weightsGB + kvCacheGB + overheadGB
	return RAMEstimate{
		WeightsGB:    round2(weightsGB),
		KVCacheGB:    round2(kvCacheGB),
		OverheadGB:   overheadGB,
		TotalGB:      round2(totalGB),
		BudgetGB:     budgetGB,
		FitsInBudget: totalGB <= budgetGB,
		EstTPS:       EstimateTPS(backend, meta, values),
	}
}

// estimateKVCacheLlama calculates KV cache size for llama-server.
// Formula: 2 × layers × kv_heads × head_dim × ctx_size × bytes × slots
func estimateKVCacheLlama(meta ModelMeta, ctxSize, parallel int) float64 {
	if meta.NumLayers == 0 || meta.NumKVHeads == 0 || meta.HiddenSize == 0 || meta.NumHeads == 0 {
		// Fallback when metadata is incomplete.
		return float64(ctxSize) * float64(parallel) * 0.0005
	}
	headDim := meta.HiddenSize / meta.NumHeads
	kvBytes := 2 * meta.NumLayers * meta.NumKVHeads * headDim * ctxSize * kvBytesPerElement * parallel
	return float64(kvBytes) / 1e9
}

// EstimateTPS returns a rough tokens-per-second estimate.
// Basis: M4 Mini empirical ~80 tok/s for 7B fully on GPU, scales as 1/sqrt(params).
// Partial CPU offload degrades by a 10× GPU-vs-CPU ratio.
func EstimateTPS(backend string, meta ModelMeta, values map[string]int) float64 {
	if meta.FileSizeBytes == 0 {
		return 0
	}
	// Approximate parameter count: Q4 ≈ 0.5 bytes/param.
	paramsB := float64(meta.FileSizeBytes) / 1e9 / 0.5
	if paramsB < 0.1 {
		paramsB = 0.1
	}
	baseTPS := 80.0 / math.Sqrt(paramsB/7.0)

	switch backend {
	case "llamaserver":
		numLayers := meta.NumLayers
		if numLayers == 0 {
			numLayers = 1
		}
		ngl := getOrDefault(values, "ngl", numLayers)
		if ngl > numLayers {
			ngl = numLayers
		}
		gpuFrac := float64(ngl) / float64(numLayers)
		effectiveFrac := gpuFrac + (1-gpuFrac)*0.1
		return round1(baseTPS * effectiveFrac)
	default:
		// MLX always on Metal — no offload penalty.
		return round1(baseTPS)
	}
}

func getOrDefault(m map[string]int, key string, def int) int {
	if v, ok := m[key]; ok {
		return v
	}
	return def
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }
func round1(v float64) float64 { return math.Round(v*10) / 10 }
