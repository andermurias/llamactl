//go:build e2e

package e2e_test

import (
	"net/http"
	"testing"
)

// ── Model listing tests ───────────────────────────────────────────────────────
// Verify that /v1/models returns all expected models and that the llamactl
// web /api/models endpoint mirrors the same list. No model is loaded.

// TestModels_NotEmpty asserts /v1/models returns at least one model.
func TestModels_NotEmpty(t *testing.T) {
	code, body := get(t, llamaSwapAddr()+"/v1/models")
	if code != http.StatusOK {
		t.Fatalf("/v1/models: expected 200, got %d", code)
	}
	var resp modelsResponse
	mustParseJSON(t, body, &resp)
	if len(resp.Data) == 0 {
		t.Fatal("/v1/models returned empty data array")
	}
	t.Logf("✓ /v1/models returned %d models", len(resp.Data))
}

// TestModels_AllExpectedPresent checks that every model declared in
// llama-swap.yaml appears in the /v1/models response.
func TestModels_AllExpectedPresent(t *testing.T) {
	_, body := get(t, llamaSwapAddr()+"/v1/models")
	var resp modelsResponse
	mustParseJSON(t, body, &resp)

	// Build a set of returned IDs for O(1) lookups.
	returned := make(map[string]bool, len(resp.Data))
	for _, m := range resp.Data {
		returned[m.ID] = true
	}

	var missing []string
	for _, want := range expectedModels {
		if !returned[want] {
			missing = append(missing, want)
		}
	}
	if len(missing) > 0 {
		t.Errorf("models missing from /v1/models response: %v", missing)
	}
	t.Logf("✓ all %d expected models present", len(expectedModels))
}

// TestModels_ResponseShape verifies each model entry has the required fields.
func TestModels_ResponseShape(t *testing.T) {
	_, body := get(t, llamaSwapAddr()+"/v1/models")
	var resp modelsResponse
	mustParseJSON(t, body, &resp)

	for _, m := range resp.Data {
		if m.ID == "" {
			t.Errorf("model entry has empty id; full entry: %+v", m)
		}
		if m.Object == "" {
			t.Errorf("model %q has empty object field", m.ID)
		}
	}
	t.Logf("✓ all %d model entries have correct shape", len(resp.Data))
}

// TestModels_LlamaCtlMirror verifies that llamactl /api/models lists the same
// models as llama-swap /v1/models (the web UI proxy must be consistent).
func TestModels_LlamaCtlMirror(t *testing.T) {
	skipIfUnavailable(t, llamaCtlAddr()+"/api/models")

	code, body := get(t, llamaCtlAddr()+"/api/models")
	if code != http.StatusOK {
		t.Fatalf("/api/models: expected 200, got %d; body: %s", code, body)
	}

	// llamactl /api/models response shape:
	// { "APIModels": [{id, object, owned_by}], "APIReachable": true, ... }
	var ctlResp struct {
		APIModels []struct {
			ID string `json:"id"`
		} `json:"APIModels"`
		APIReachable bool `json:"APIReachable"`
	}
	mustParseJSON(t, body, &ctlResp)

	if !ctlResp.APIReachable {
		t.Fatal("llamactl /api/models reports APIReachable=false")
	}

	// Build sets for comparison.
	directIDs := make(map[string]bool)
	_, directBody := get(t, llamaSwapAddr()+"/v1/models")
	var directResp modelsResponse
	mustParseJSON(t, directBody, &directResp)
	for _, m := range directResp.Data {
		directIDs[m.ID] = true
	}

	var extra []string
	for _, m := range ctlResp.APIModels {
		if !directIDs[m.ID] {
			extra = append(extra, m.ID)
		}
	}
	if len(extra) > 0 {
		t.Errorf("llamactl /api/models contains IDs not in llama-swap: %v", extra)
	}
	if len(ctlResp.APIModels) != len(directResp.Data) {
		t.Errorf("model count mismatch: llamactl=%d, llama-swap=%d",
			len(ctlResp.APIModels), len(directResp.Data))
	}
	t.Logf("✓ llamactl mirrors llama-swap: %d models", len(ctlResp.APIModels))
}
