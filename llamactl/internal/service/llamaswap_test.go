package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/andermurias/llamactl/internal/config"
	"github.com/andermurias/llamactl/internal/llamaswap"
)

// testConfig returns a Config with Listen pointing at the given test server address.
func testConfig(addr string) *config.Config {
	cfg := config.Load()
	cfg.Listen = addr // e.g. "127.0.0.1:PORT"
	return cfg
}

// TestGetModelsInfo_APIReachable verifies that GetModelsInfo correctly parses
// /v1/models and /running when llama-swap is healthy.
func TestGetModelsInfo_APIReachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"object": "list",
				"data": []map[string]string{
					{"id": "test-model", "object": "model", "owned_by": "test"},
				},
			})
		case "/running":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"running": []string{"test-model"},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	// srv.Listener.Addr().String() is "127.0.0.1:PORT" — exactly what cfg.Listen expects.
	cfg := testConfig(srv.Listener.Addr().String())
	info := GetModelsInfo(cfg)

	if !info.APIReachable {
		t.Fatal("expected APIReachable=true when server is healthy")
	}
	if len(info.APIModels) != 1 {
		t.Fatalf("expected 1 model, got %d", len(info.APIModels))
	}
	if info.APIModels[0].ID != "test-model" {
		t.Errorf("unexpected model ID: %q", info.APIModels[0].ID)
	}
	if !info.LoadedIDs["test-model"] {
		t.Error("expected test-model to appear in LoadedIDs")
	}
}

// TestGetModelsInfo_APIUnreachable verifies graceful handling when the server is down.
func TestGetModelsInfo_APIUnreachable(t *testing.T) {
	// Use a port that nothing is listening on.
	cfg := testConfig("127.0.0.1:19999")
	info := GetModelsInfo(cfg)

	if info.APIReachable {
		t.Fatal("expected APIReachable=false when server is down")
	}
	if len(info.APIModels) != 0 {
		t.Errorf("expected 0 models when unreachable, got %d", len(info.APIModels))
	}
}

// TestFormatBytes covers the FormatBytes helper across the GB/MB/B ranges.
func TestFormatBytes(t *testing.T) {
	cases := []struct {
		bytes    int64
		expected string
	}{
		{0, "0B"},
		{500, "500B"},
		{1024 * 1024, "1M"},
		{int64(2.5 * 1024 * 1024 * 1024), "2.5G"},
	}
	for _, c := range cases {
		got := llamaswap.FormatBytes(c.bytes)
		if got != c.expected {
			t.Errorf("FormatBytes(%d) = %q, want %q", c.bytes, got, c.expected)
		}
	}
}
