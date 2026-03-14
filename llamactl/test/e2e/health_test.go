//go:build e2e

package e2e_test

import (
	"net/http"
	"testing"
)

// ── Health & connectivity ────────────────────────────────────────────────────
// These tests verify the two core services are up and responding correctly.
// They are intentionally fast (<1 s each) — no model is loaded.

// TestLlamaSwap_IsUp verifies the llama-swap /running endpoint returns 200.
func TestLlamaSwap_IsUp(t *testing.T) {
	url := llamaSwapAddr() + "/running"
	code, body := get(t, url)
	if code != http.StatusOK {
		t.Fatalf("GET /running: expected 200, got %d\nbody: %s", code, body)
	}
	t.Logf("✓ llama-swap /running → %d  %s", code, body)
}

// TestLlamaSwap_RunningResponseShape verifies the /running response is valid
// JSON with a "running" array (may be empty when no model is loaded).
func TestLlamaSwap_RunningResponseShape(t *testing.T) {
	_, body := get(t, llamaSwapAddr()+"/running")
	var resp map[string]any
	mustParseJSON(t, body, &resp)
	if _, ok := resp["running"]; !ok {
		t.Fatalf(`/running response has no "running" key; body: %s`, body)
	}
	t.Logf("✓ /running JSON shape OK; loaded models: %v", resp["running"])
}

// TestLlamaCtlWeb_IsUp verifies the llamactl web dashboard responds on port 3333.
func TestLlamaCtlWeb_IsUp(t *testing.T) {
	url := llamaCtlAddr() + "/"
	code, _ := get(t, url)
	// Accept 200 (dashboard) or 302 (redirect to /dashboard).
	if code != http.StatusOK && code != http.StatusFound {
		t.Fatalf("GET / on llamactl: expected 200 or 302, got %d", code)
	}
	t.Logf("✓ llamactl web / → %d", code)
}

// TestLlamaCtlWeb_APIStatus verifies /api/status returns 200 with JSON.
func TestLlamaCtlWeb_APIStatus(t *testing.T) {
	code, body := get(t, llamaCtlAddr()+"/api/status")
	if code != http.StatusOK {
		t.Fatalf("/api/status: expected 200, got %d", code)
	}
	var resp map[string]any
	mustParseJSON(t, body, &resp)
	if _, ok := resp["llamaswap"]; !ok {
		t.Fatalf(`/api/status missing "llamaswap" key; body: %s`, body)
	}
	t.Logf("✓ /api/status OK")
}
