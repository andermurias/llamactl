//go:build e2e

package e2e_test

import (
	"net/http"
	"strings"
	"testing"
)

// ── llamactl web API tests ───────────────────────────────────────────────────
// Validate every endpoint exposed by the llamactl web server.
// No model loading is required — these tests only check API shape and plumbing.

// TestLlamaCtlAPI_StatusShape verifies /api/status has the correct JSON schema.
func TestLlamaCtlAPI_StatusShape(t *testing.T) {
	skipIfUnavailable(t, llamaCtlAddr())
	code, body := get(t, llamaCtlAddr()+"/api/status")
	if code != http.StatusOK {
		t.Fatalf("/api/status: expected 200, got %d", code)
	}

	var resp struct {
		LlamaSwap struct {
			IsInstalled  bool   `json:"IsInstalled"`
			IsLoaded     bool   `json:"IsLoaded"`
			IsRunning    bool   `json:"IsRunning"`
			APIReachable bool   `json:"APIReachable"`
			ConfigFile   string `json:"ConfigFile"`
			LogFile      string `json:"LogFile"`
		} `json:"llamaswap"`
		ComfyUI struct {
			LogFile string `json:"LogFile"`
		} `json:"comfyui"`
	}
	mustParseJSON(t, body, &resp)

	if resp.LlamaSwap.ConfigFile == "" {
		t.Error("status.llamaswap.ConfigFile is empty")
	}
	if resp.LlamaSwap.LogFile == "" {
		t.Error("status.llamaswap.LogFile is empty")
	}
	if resp.ComfyUI.LogFile == "" {
		t.Error("status.comfyui.LogFile is empty")
	}
	t.Logf("✓ /api/status schema OK  (IsRunning=%v, APIReachable=%v)",
		resp.LlamaSwap.IsRunning, resp.LlamaSwap.APIReachable)
}

// TestLlamaCtlAPI_StatusLlamaSwapRunning checks that when llama-swap is up
// the status endpoint correctly reflects it.
func TestLlamaCtlAPI_StatusLlamaSwapRunning(t *testing.T) {
	skipIfUnavailable(t, llamaCtlAddr())
	// First check that llama-swap itself is reachable so we can assert correctly.
	skipIfUnavailable(t, llamaSwapAddr()+"/running")

	_, body := get(t, llamaCtlAddr()+"/api/status")
	var resp struct {
		LlamaSwap struct {
			IsRunning    bool `json:"IsRunning"`
			APIReachable bool `json:"APIReachable"`
		} `json:"llamaswap"`
	}
	mustParseJSON(t, body, &resp)

	if !resp.LlamaSwap.APIReachable {
		t.Error("llamactl reports APIReachable=false but llama-swap is responding")
	}
	if !resp.LlamaSwap.IsRunning {
		t.Error("llamactl reports IsRunning=false but llama-swap process is live")
	}
	t.Logf("✓ status correctly shows llama-swap is running")
}

// TestLlamaCtlAPI_Logs_LlamaSwap verifies /api/logs?service=llamaswap returns lines.
func TestLlamaCtlAPI_Logs_LlamaSwap(t *testing.T) {
	skipIfUnavailable(t, llamaCtlAddr())
	code, body := get(t, llamaCtlAddr()+"/api/logs?service=llamaswap&lines=10")
	if code != http.StatusOK {
		t.Fatalf("/api/logs: expected 200, got %d", code)
	}
	var resp struct {
		Lines   []string `json:"lines"`
		Service string   `json:"service"`
	}
	mustParseJSON(t, body, &resp)
	if len(resp.Lines) == 0 {
		t.Error("/api/logs returned empty lines array")
	}
	if resp.Service != "llamaswap" {
		t.Errorf("expected service=llamaswap, got %q", resp.Service)
	}
	t.Logf("✓ /api/logs returned %d lines", len(resp.Lines))
}

// TestLlamaCtlAPI_Logs_LineLimit verifies the ?lines=N parameter is respected.
func TestLlamaCtlAPI_Logs_LineLimit(t *testing.T) {
	skipIfUnavailable(t, llamaCtlAddr())
	_, body := get(t, llamaCtlAddr()+"/api/logs?service=llamaswap&lines=3")
	var resp struct {
		Lines []string `json:"lines"`
	}
	mustParseJSON(t, body, &resp)
	if len(resp.Lines) > 3 {
		t.Errorf("?lines=3 returned %d lines (expected ≤3)", len(resp.Lines))
	}
	t.Logf("✓ line limit respected (%d lines)", len(resp.Lines))
}

// TestLlamaCtlAPI_Config_Get verifies /api/config returns the YAML content.
func TestLlamaCtlAPI_Config_Get(t *testing.T) {
	skipIfUnavailable(t, llamaCtlAddr())
	code, body := get(t, llamaCtlAddr()+"/api/config")
	if code != http.StatusOK {
		t.Fatalf("/api/config: expected 200, got %d", code)
	}
	var resp struct {
		Content string `json:"content"`
		Path    string `json:"path"`
	}
	mustParseJSON(t, body, &resp)
	if resp.Content == "" {
		t.Error("/api/config returned empty content")
	}
	if !strings.Contains(resp.Content, "models:") {
		t.Error("config content does not contain 'models:' — may be wrong file")
	}
	t.Logf("✓ /api/config returned %d bytes of YAML", len(resp.Content))
}

// TestLlamaCtlAPI_Models verifies /api/models returns the full model info.
func TestLlamaCtlAPI_Models(t *testing.T) {
	skipIfUnavailable(t, llamaCtlAddr())
	code, body := get(t, llamaCtlAddr()+"/api/models")
	if code != http.StatusOK {
		t.Fatalf("/api/models: expected 200, got %d", code)
	}
	var resp struct {
		APIModels    []openAIModel `json:"APIModels"`
		APIReachable bool          `json:"APIReachable"`
		GGUFFiles    []struct {
			Path string `json:"Path"`
			Size int64  `json:"Size"`
		} `json:"GGUFFiles"`
		HFModels []string `json:"HFModels"`
	}
	mustParseJSON(t, body, &resp)
	if !resp.APIReachable {
		t.Error("/api/models: APIReachable=false")
	}
	if len(resp.APIModels) == 0 {
		t.Error("/api/models: APIModels is empty")
	}
	t.Logf("✓ /api/models: %d API models, %d GGUF files, %d HF models",
		len(resp.APIModels), len(resp.GGUFFiles), len(resp.HFModels))
}

// TestLlamaCtlAPI_InvalidMethod verifies unsupported methods return 4xx.
func TestLlamaCtlAPI_InvalidMethod(t *testing.T) {
	skipIfUnavailable(t, llamaCtlAddr())

	endpoints := []string{"/api/status", "/api/models", "/api/logs"}
	for _, ep := range endpoints {
		req, _ := newRequest("DELETE", llamaCtlAddr()+ep, nil)
		resp, err := client.Do(req)
		if err != nil {
			t.Errorf("DELETE %s: %v", ep, err)
			continue
		}
		resp.Body.Close()
		if resp.StatusCode < 400 {
			t.Errorf("DELETE %s: expected 4xx, got %d", ep, resp.StatusCode)
		}
	}
	t.Logf("✓ invalid methods correctly rejected")
}
