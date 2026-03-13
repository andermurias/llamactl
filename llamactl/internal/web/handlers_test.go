// Package web — handlers_test.go tests the HTTP handler layer of the
// llamactl web UI.  All tests spin up an in-process httptest server so no
// real launchd services, llama-swap process, or filesystem state beyond
// temporary files are required.
package web_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andermurias/llamactl/internal/config"
	"github.com/andermurias/llamactl/internal/web"
)

// ── Test helpers ───────────────────────────────────────────────────────────

// freePort returns a TCP port that is not currently in use on 127.0.0.1.
// It briefly opens a listener to obtain an OS-assigned free port, then
// closes it so the port is available for the test (tiny TOCTOU window, but
// acceptable in a controlled test environment).
func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("freePort: %v", err)
	}
	defer l.Close()
	return l.Addr().String() // "127.0.0.1:PORT"
}

// newTestServer creates a web.Server with an isolated temp-dir config.
// The test config intentionally points PlistPath at a nonexistent file so
// that llama-swap and ComfyUI always appear "not installed" — launchd is
// never called.  Log files and the YAML config file are pre-populated.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	dir := t.TempDir()

	cfg := config.Load()

	// Override all file paths to temp dir so the handler can read/write them.
	cfg.LogFile = filepath.Join(dir, "llamaswap.log")
	cfg.ComfyUILog = filepath.Join(dir, "comfyui.log")
	cfg.ConfigFile = filepath.Join(dir, "llama-swap.yaml")

	// Plist paths don't exist → service.GetStatus reports IsInstalled=false,
	// which is safe and avoids any launchd interaction.
	cfg.PlistPath = filepath.Join(dir, "no-such.plist")
	cfg.ComfyUIPID = filepath.Join(dir, "comfyui.pid")

	// Pre-populate log fixtures.
	mustWrite(t, cfg.LogFile, "2024-01-01 startup\n2024-01-01 ready\n2024-01-01 request\n")
	mustWrite(t, cfg.ComfyUILog, "ComfyUI boot\nComfyUI ready\n")

	// Pre-populate YAML config fixture.
	mustWrite(t, cfg.ConfigFile, "# llamactl test config\nhttpListenAddress: 127.0.0.1:8080\n")

	// Use a dynamically allocated free port so the test is not affected by
	// whatever is running on the developer's machine.
	cfg.Listen = freePort(t)

	srv, err := web.New(cfg)
	if err != nil {
		t.Fatalf("web.New: %v", err)
	}
	return httptest.NewServer(srv.Handler())
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// get performs a GET and returns the response + body string.
func get(t *testing.T, base, path string) (*http.Response, string) {
	t.Helper()
	resp, err := http.Get(base + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, string(b)
}

// postJSON performs a POST with a JSON body and returns the response + body.
func postJSON(t *testing.T, base, path string, body any) (*http.Response, string) {
	t.Helper()
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		t.Fatalf("encode body: %v", err)
	}
	resp, err := http.Post(base+path, "application/json", &buf)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, string(b)
}

// parseJSON unmarshals a JSON body string into v; fails the test on error.
func parseJSON(t *testing.T, body string, v any) {
	t.Helper()
	if err := json.Unmarshal([]byte(body), v); err != nil {
		t.Fatalf("parseJSON: %v\nbody: %s", err, body)
	}
}

// ── Dashboard ──────────────────────────────────────────────────────────────

// TestHandleIndex_ReturnsHTML verifies the dashboard returns a 200 HTML page
// with the expected landmark content.
func TestHandleIndex_ReturnsHTML(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	resp, body := get(t, srv.URL, "/")

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("expected HTML content-type, got %q", ct)
	}
	for _, want := range []string{"llamactl", "llama-swap", "Services", "Models", "Logs", "Config"} {
		if !strings.Contains(body, want) {
			t.Errorf("dashboard missing expected text %q", want)
		}
	}
}

// ── /api/status ───────────────────────────────────────────────────────────

// TestHandleStatus_Shape checks that /api/status returns a valid JSON object
// with the expected top-level keys, even when services are stopped.
func TestHandleStatus_Shape(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	resp, body := get(t, srv.URL, "/api/status")

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	var data map[string]any
	parseJSON(t, body, &data)

	for _, key := range []string{"llamaswap", "comfyui"} {
		if _, ok := data[key]; !ok {
			t.Errorf("status response missing key %q; body: %s", key, body)
		}
	}
}

// TestHandleStatus_StoppedServices checks that IsRunning is false when no
// real service is present.
func TestHandleStatus_StoppedServices(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	_, body := get(t, srv.URL, "/api/status")

	var data map[string]any
	parseJSON(t, body, &data)

	lsMap, ok := data["llamaswap"].(map[string]any)
	if !ok {
		t.Fatalf("llamaswap value is not an object; body: %s", body)
	}
	if lsMap["IsRunning"] == true {
		t.Error("expected IsRunning=false for uninstalled llama-swap")
	}
}

// ── /api/logs ─────────────────────────────────────────────────────────────

// TestHandleLogs_LlamaSwap checks that the llamaswap log endpoint returns
// the pre-populated lines.
func TestHandleLogs_LlamaSwap(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	resp, body := get(t, srv.URL, "/api/logs?service=llamaswap&lines=50")

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	var data map[string]any
	parseJSON(t, body, &data)

	lines, ok := data["lines"].([]any)
	if !ok {
		t.Fatalf("expected lines array; body: %s", body)
	}
	if len(lines) == 0 {
		t.Error("expected at least 1 log line")
	}
	// Verify a known line is present.
	found := false
	for _, l := range lines {
		if strings.Contains(l.(string), "ready") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected line containing 'ready'; got: %v", lines)
	}
}

// TestHandleLogs_ComfyUI checks that the comfyui log source is respected.
func TestHandleLogs_ComfyUI(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	_, body := get(t, srv.URL, "/api/logs?service=comfyui")

	var data map[string]any
	parseJSON(t, body, &data)

	if svc, _ := data["service"].(string); svc != "comfyui" {
		t.Errorf("expected service=comfyui, got %q; body: %s", svc, body)
	}
	lines, _ := data["lines"].([]any)
	if len(lines) == 0 {
		t.Error("expected at least 1 ComfyUI log line")
	}
}

// TestHandleLogs_DefaultsToLlamaSwap checks that omitting ?service defaults
// to the llamaswap log.
func TestHandleLogs_DefaultsToLlamaSwap(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	_, body := get(t, srv.URL, "/api/logs")

	var data map[string]any
	parseJSON(t, body, &data)

	if _, ok := data["lines"]; !ok {
		t.Errorf("expected lines key; body: %s", body)
	}
}

// ── /api/models ───────────────────────────────────────────────────────────

// TestHandleModels_UnreachableAPI checks graceful degradation when
// llama-swap is not running (cfg.Listen points to a dead port).
func TestHandleModels_UnreachableAPI(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	resp, body := get(t, srv.URL, "/api/models")

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	var data map[string]any
	parseJSON(t, body, &data)

	if data["APIReachable"] == true {
		t.Error("expected APIReachable=false when llama-swap is not running")
	}
}

// ── /api/config ───────────────────────────────────────────────────────────

// TestHandleConfig_GetReturnsContent verifies GET returns the YAML content
// of the config file.
func TestHandleConfig_GetReturnsContent(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	resp, body := get(t, srv.URL, "/api/config")

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	var data map[string]any
	parseJSON(t, body, &data)

	content, ok := data["content"].(string)
	if !ok || content == "" {
		t.Fatalf("expected non-empty content field; body: %s", body)
	}
	if !strings.Contains(content, "httpListenAddress") {
		t.Errorf("expected config to contain 'httpListenAddress'; got: %s", content)
	}
	if _, ok := data["path"]; !ok {
		t.Error("expected 'path' field in response")
	}
}

// TestHandleConfig_PostSavesAndReturnsOK verifies POST saves new content.
func TestHandleConfig_PostSavesAndReturnsOK(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	newContent := "# updated\nhttpListenAddress: 127.0.0.1:9090\n"
	resp, body := postJSON(t, srv.URL, "/api/config", map[string]string{
		"content": newContent,
	})

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d; body: %s", resp.StatusCode, body)
	}
	var data map[string]any
	parseJSON(t, body, &data)
	if data["ok"] != true {
		t.Errorf("expected ok=true; body: %s", body)
	}

	// Verify the file was actually written by reading it back via GET.
	_, getBody := get(t, srv.URL, "/api/config")
	var getResp map[string]any
	parseJSON(t, getBody, &getResp)
	if !strings.Contains(getResp["content"].(string), "9090") {
		t.Errorf("saved content not reflected in GET; content: %s", getResp["content"])
	}
}

// TestHandleConfig_InvalidMethodReturns405 verifies that PUT/DELETE on
// /api/config returns 405 Method Not Allowed.
func TestHandleConfig_InvalidMethodReturns405(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/config", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for PUT /api/config, got %d", resp.StatusCode)
	}
}

// ── POST-only enforcement ─────────────────────────────────────────────────

// TestPostOnlyEndpoints checks that action endpoints reject GET requests with
// 405 Method Not Allowed.
func TestPostOnlyEndpoints(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	endpoints := []string{
		"/api/llamaswap/start",
		"/api/llamaswap/stop",
		"/api/llamaswap/restart",
		"/api/comfyui/start",
		"/api/comfyui/stop",
	}

	for _, ep := range endpoints {
		resp, err := http.Get(srv.URL + ep)
		if err != nil {
			t.Fatalf("GET %s: %v", ep, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("GET %s: expected 405, got %d", ep, resp.StatusCode)
		}
	}
}

// TestActionEndpoints_ReturnJSON verifies that POST to action endpoints
// always returns JSON (either {"ok":true} or {"error":"..."}).
func TestActionEndpoints_ReturnJSON(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	endpoints := []string{
		"/api/llamaswap/start",
		"/api/llamaswap/stop",
		"/api/llamaswap/restart",
		"/api/comfyui/start",
		"/api/comfyui/stop",
	}

	for _, ep := range endpoints {
		resp, err := http.Post(srv.URL+ep, "application/json", nil)
		if err != nil {
			t.Fatalf("POST %s: %v", ep, err)
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		ct := resp.Header.Get("Content-Type")
		if !strings.Contains(ct, "application/json") {
			t.Errorf("POST %s: expected JSON content-type, got %q", ep, ct)
		}
		// Verify it's parseable JSON (ok or error field).
		var data map[string]any
		if err := json.Unmarshal(b, &data); err != nil {
			t.Errorf("POST %s: response is not valid JSON: %s", ep, string(b))
		}
	}
}

// ── tailFile edge cases ───────────────────────────────────────────────────

// TestHandleLogs_LineLimitRespected verifies that ?lines=N returns at most
// N lines from the log file.
func TestHandleLogs_LineLimitRespected(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	_, body := get(t, srv.URL, "/api/logs?service=llamaswap&lines=1")

	var data map[string]any
	parseJSON(t, body, &data)

	lines, _ := data["lines"].([]any)
	if len(lines) > 1 {
		t.Errorf("expected at most 1 line with ?lines=1, got %d", len(lines))
	}
}

// TestHandleLogs_MissingLogFile returns error field when log file is absent.
func TestHandleLogs_MissingLogFile(t *testing.T) {
	dir := t.TempDir()

	cfg := config.Load()
	cfg.LogFile = filepath.Join(dir, "does-not-exist.log") // never created
	cfg.ComfyUILog = filepath.Join(dir, "comfyui.log")
	cfg.ConfigFile = filepath.Join(dir, "llama-swap.yaml")
	cfg.PlistPath = filepath.Join(dir, "noplist")
	cfg.Listen = freePort(t)

	// Create mandatory files (ConfigFile needed for template render).
	os.WriteFile(cfg.ConfigFile, []byte("# empty\n"), 0o644)
	os.WriteFile(cfg.ComfyUILog, []byte(""), 0o644)

	srvWeb, err := web.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srvWeb.Handler())
	defer ts.Close()

	_, body := get(t, ts.URL, "/api/logs?service=llamaswap")

	var data map[string]any
	parseJSON(t, body, &data)

	if _, hasError := data["error"]; !hasError {
		t.Errorf("expected error field when log file is missing; body: %s", body)
	}
}
