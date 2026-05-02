package modelconfig_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/andermurias/llamactl/internal/modelconfig"
)

const sampleLlamaCmd = `llama-server
--model /Users/andermurias/AI/models/chat/gemma-3-12b-it-Q4_K_M.gguf
--alias gemma-3-12b-it
--host 127.0.0.1
--port 5800
-ngl 99
--ctx-size 8192
--threads 8`

const sampleMLXCmd = `/opt/homebrew/Caskroom/miniforge/base/envs/mlx-server/bin/mlx_lm.server
--model mlx-community/gemma-3-12b-it-4bit
--host 127.0.0.1
--port 5801
--log-level WARNING
--max-tokens 16384
--prefill-step-size 256
--prompt-cache-size 2
--prompt-cache-bytes 2147483648`

// ── ParseCmd ──────────────────────────────────────────────────────────────────

func TestParseCmd_LlamaServer(t *testing.T) {
	got := modelconfig.ParseCmd(sampleLlamaCmd)

	cases := map[string]string{
		"-ngl":       "99",
		"--ctx-size": "8192",
		"--threads":  "8",
		"--model":    "/Users/andermurias/AI/models/chat/gemma-3-12b-it-Q4_K_M.gguf",
	}
	for flag, want := range cases {
		if got[flag] != want {
			t.Errorf("ParseCmd[%q] = %q, want %q", flag, got[flag], want)
		}
	}
}

func TestParseCmd_MLX(t *testing.T) {
	got := modelconfig.ParseCmd(sampleMLXCmd)

	cases := map[string]string{
		"--max-tokens":         "16384",
		"--prefill-step-size":  "256",
		"--prompt-cache-size":  "2",
		"--prompt-cache-bytes": "2147483648",
	}
	for flag, want := range cases {
		if got[flag] != want {
			t.Errorf("ParseCmd[%q] = %q, want %q", flag, got[flag], want)
		}
	}
}

func TestParseCmd_MissingFlag(t *testing.T) {
	got := modelconfig.ParseCmd(sampleLlamaCmd)
	if _, ok := got["--parallel"]; ok {
		t.Error("ParseCmd should not return a key for a flag not present in cmd")
	}
}

// ── WriteCmd ──────────────────────────────────────────────────────────────────

func TestWriteCmd_UpdatesExisting(t *testing.T) {
	updates := map[string]string{"-ngl": "20", "--ctx-size": "4096"}
	got := modelconfig.WriteCmd(sampleLlamaCmd, updates)

	parsed := modelconfig.ParseCmd(got)
	if parsed["-ngl"] != "20" {
		t.Errorf("WriteCmd: -ngl = %q, want 20", parsed["-ngl"])
	}
	if parsed["--ctx-size"] != "4096" {
		t.Errorf("WriteCmd: --ctx-size = %q, want 4096", parsed["--ctx-size"])
	}
	// Unchanged flags stay intact
	if parsed["--threads"] != "8" {
		t.Errorf("WriteCmd: --threads = %q, want 8 (unchanged)", parsed["--threads"])
	}
}

func TestWriteCmd_AddsNewFlag(t *testing.T) {
	updates := map[string]string{"--parallel": "2"}
	got := modelconfig.WriteCmd(sampleLlamaCmd, updates)

	parsed := modelconfig.ParseCmd(got)
	if parsed["--parallel"] != "2" {
		t.Errorf("WriteCmd: --parallel = %q, want 2 (new flag)", parsed["--parallel"])
	}
}

func TestWriteCmd_PreservesStructure(t *testing.T) {
	updates := map[string]string{"-ngl": "20"}
	got := modelconfig.WriteCmd(sampleLlamaCmd, updates)

	parsed := modelconfig.ParseCmd(got)
	if parsed["--model"] != "/Users/andermurias/AI/models/chat/gemma-3-12b-it-Q4_K_M.gguf" {
		t.Errorf("WriteCmd: --model changed unexpectedly: %q", parsed["--model"])
	}
}

// ── GGUF reader ───────────────────────────────────────────────────────────────

func TestReadGGUFMeta_RealFile(t *testing.T) {
	path := "/Users/andermurias/AI/models/chat/gemma-3-12b-it-Q4_K_M.gguf"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("GGUF test file not present on this machine")
	}

	meta, err := modelconfig.ReadGGUFMeta(path)
	if err != nil {
		t.Fatalf("ReadGGUFMeta: %v", err)
	}
	if meta.NumLayers < 10 {
		t.Errorf("NumLayers = %d, want >= 10", meta.NumLayers)
	}
	if meta.HiddenSize < 1024 {
		t.Errorf("HiddenSize = %d, want >= 1024", meta.HiddenSize)
	}
	if meta.MaxContext < 512 {
		t.Errorf("MaxContext = %d, want >= 512", meta.MaxContext)
	}
}

func TestReadGGUFMeta_NotAGGUF(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "fake*.gguf")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("this is not a gguf file")
	f.Close()

	_, err = modelconfig.ReadGGUFMeta(f.Name())
	if err == nil {
		t.Error("ReadGGUFMeta: expected error for non-GGUF file, got nil")
	}
}

// ── Metadata cache ────────────────────────────────────────────────────────────

func TestMetaCache_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "meta.yaml")

	want := modelconfig.ModelMeta{
		ModelID:       "test-model",
		Backend:       "llamaserver",
		NumLayers:     46,
		NumHeads:      32,
		NumKVHeads:    8,
		HiddenSize:    5120,
		MaxContext:    8192,
		FileSizeBytes: 7516192768,
	}

	if err := modelconfig.UpsertMeta(path, want); err != nil {
		t.Fatalf("UpsertMeta: %v", err)
	}

	got, ok := modelconfig.GetMeta(path, "test-model")
	if !ok {
		t.Fatal("GetMeta: model not found after upsert")
	}
	if got.NumLayers != want.NumLayers {
		t.Errorf("NumLayers = %d, want %d", got.NumLayers, want.NumLayers)
	}
	if got.FileSizeBytes != want.FileSizeBytes {
		t.Errorf("FileSizeBytes = %d, want %d", got.FileSizeBytes, want.FileSizeBytes)
	}
}

func TestMetaCache_EmptyOnMissing(t *testing.T) {
	c, err := modelconfig.LoadMetaCache("/tmp/does-not-exist-llamactl-meta-test.yaml")
	if err != nil {
		t.Fatalf("LoadMetaCache: expected nil error for missing file, got %v", err)
	}
	if len(c.Models) != 0 {
		t.Errorf("expected empty cache, got %d models", len(c.Models))
	}
}

// ── Estimation ────────────────────────────────────────────────────────────────

func TestEstimateRAM_GGUF_FullGPU(t *testing.T) {
	meta := modelconfig.ModelMeta{
		NumLayers:     46,
		NumKVHeads:    8,
		NumHeads:      32,
		HiddenSize:    5120,
		FileSizeBytes: 7_516_192_768,
	}
	values := map[string]int{"ngl": 46, "ctx_size": 4096, "parallel": 1}

	est := modelconfig.EstimateRAM("llamaserver", meta, values)

	if est.WeightsGB < 7.0 || est.WeightsGB > 8.0 {
		t.Errorf("WeightsGB = %.2f, want 7–8", est.WeightsGB)
	}
	if est.KVCacheGB <= 0 {
		t.Errorf("KVCacheGB = %.4f, want > 0", est.KVCacheGB)
	}
	if est.TotalGB < 7.5 {
		t.Errorf("TotalGB = %.2f, want >= 7.5", est.TotalGB)
	}
}

func TestEstimateRAM_GGUF_PartialOffload(t *testing.T) {
	meta := modelconfig.ModelMeta{
		NumLayers: 46, NumKVHeads: 8, NumHeads: 32,
		HiddenSize: 5120, FileSizeBytes: 7_516_192_768,
	}
	fullGPU := modelconfig.EstimateRAM("llamaserver", meta, map[string]int{"ngl": 46, "ctx_size": 4096, "parallel": 1})
	halfGPU := modelconfig.EstimateRAM("llamaserver", meta, map[string]int{"ngl": 23, "ctx_size": 4096, "parallel": 1})

	// On unified memory total RAM is the same regardless of ngl split.
	if fullGPU.TotalGB < halfGPU.TotalGB*0.9 || fullGPU.TotalGB > halfGPU.TotalGB*1.1 {
		t.Errorf("TotalGB should not change significantly with ngl: full=%.2f half=%.2f",
			fullGPU.TotalGB, halfGPU.TotalGB)
	}
}

func TestEstimateTPS_DegradesWithOffload(t *testing.T) {
	meta := modelconfig.ModelMeta{NumLayers: 46, FileSizeBytes: 7_516_192_768}
	fullGPU := modelconfig.EstimateTPS("llamaserver", meta, map[string]int{"ngl": 46})
	halfGPU := modelconfig.EstimateTPS("llamaserver", meta, map[string]int{"ngl": 23})

	if fullGPU <= halfGPU {
		t.Errorf("full GPU TPS (%v) should be higher than half GPU (%v)", fullGPU, halfGPU)
	}
}
