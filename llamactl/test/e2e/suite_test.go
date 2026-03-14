// Package e2e contains end-to-end tests for the llamactl AI stack.
//
// These tests hit the live APIs (llama-swap on :8080, llamactl web on :3333)
// and require both services to be running.  They are guarded by the "e2e"
// build tag so they never run accidentally during unit testing.
//
// # Running
//
//	# Fast: health checks + model listing + llamactl API (no model loading)
//	make test-e2e
//
//	# Full: includes actual inference (loads models, may take minutes)
//	make test-inference
//
// # Environment variables
//
//	LLAMA_SWAP_ADDR   base URL for llama-swap   (default: http://localhost:8080)
//	LLAMACTL_ADDR     base URL for llamactl web  (default: http://localhost:3333)
//
//go:build e2e

package e2e_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// ── Addresses ──────────────────────────────────────────────────────────────

func llamaSwapAddr() string {
	if v := os.Getenv("LLAMA_SWAP_ADDR"); v != "" {
		return strings.TrimRight(v, "/")
	}
	return "http://localhost:8080"
}

func llamaCtlAddr() string {
	if v := os.Getenv("LLAMACTL_ADDR"); v != "" {
		return strings.TrimRight(v, "/")
	}
	return "http://localhost:3333"
}

// ── HTTP helpers ────────────────────────────────────────────────────────────

// client is a shared HTTP client with a generous timeout for model loading.
var client = &http.Client{Timeout: 180 * time.Second}

// get performs a GET request and returns status code + body bytes.
func get(t *testing.T, url string) (int, []byte) {
	t.Helper()
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

// postJSON performs a POST with a JSON body and returns status code + body bytes.
func postJSON(t *testing.T, url string, payload any) (int, []byte) {
	t.Helper()
	buf, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	resp, err := client.Post(url, "application/json", bytes.NewReader(buf))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

// mustParseJSON unmarshals body into v, failing the test on error.
func mustParseJSON(t *testing.T, body []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(body, v); err != nil {
		t.Fatalf("JSON parse failed: %v\nbody: %s", err, body)
	}
}

// prettyJSON returns a compact single-line JSON string for diagnostic output.
func prettyJSON(body []byte) string {
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		return string(body)
	}
	b, _ := json.Marshal(v)
	if len(b) > 300 {
		return string(b[:300]) + "…"
	}
	return string(b)
}

// marshalJSON is a test-safe json.Marshal wrapper (returns the raw bytes).
func marshalJSON(v any) ([]byte, error) {
	return json.Marshal(v)
}

// newRequest wraps http.NewRequest, failing the test on error.
func newRequest(method, url string, body io.Reader) (*http.Request, error) {
	return http.NewRequest(method, url, body)
}

// skipIfUnavailable calls t.Skip if the given URL is unreachable.
// Used to make individual test files independently skip-able.
func skipIfUnavailable(t *testing.T, url string) {
	t.Helper()
	c := &http.Client{Timeout: 3 * time.Second}
	resp, err := c.Get(url)
	if err != nil {
		t.Skipf("service not reachable at %s (%v) — skipping", url, err)
	}
	resp.Body.Close()
}

// logBody logs the response body at diagnostic level (t.Log, not t.Fatal).
func logBody(t *testing.T, label string, body []byte) {
	t.Helper()
	t.Logf("%s: %s", label, prettyJSON(body))
}

// expectedModels lists all model IDs that must appear in /v1/models.
// Update this list when models are added to or removed from llama-swap.yaml.
var expectedModels = []string{
	"gemma-3-12b-it",
	"gemma-3-12b-it-mlx",
	"nomic-embed-text-v1.5",
	"qwen3.5-9b",
	"qwen2.5-3b",
	"qwen2.5-coder-14b",
	"qwen2.5-14b",
	"phi-4",
	"qwen2.5-vl-7b",
	"mistral-nemo-12b",
	"mistral-small-3.1-24b",
	"whisper-stt",
	"kokoro-tts",
}

// ── Shared types for JSON responses ─────────────────────────────────────────

type openAIModel struct {
	ID     string `json:"id"`
	Object string `json:"object"`
}

type modelsResponse struct {
	Object string        `json:"object"`
	Data   []openAIModel `json:"data"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model     string        `json:"model"`
	Messages  []chatMessage `json:"messages"`
	MaxTokens int           `json:"max_tokens"`
	Stream    bool          `json:"stream"`
}

type chatChoice struct {
	Message chatMessage `json:"message"`
	Index   int         `json:"index"`
}

type chatResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Model   string       `json:"model"`
	Choices []chatChoice `json:"choices"`
}

type embeddingRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type embeddingData struct {
	Object    string    `json:"object"`
	Embedding []float64 `json:"embedding"`
	Index     int       `json:"index"`
}

type embeddingResponse struct {
	Object string          `json:"object"`
	Data   []embeddingData `json:"data"`
	Model  string          `json:"model"`
}

// ── Diagnostic summary helper ────────────────────────────────────────────────

// TestMain prints a header so CI logs are easy to scan.
func TestMain(m *testing.M) {
	fmt.Printf("\n╔══════════════════════════════════════════════════════╗\n")
	fmt.Printf("║  llamactl E2E test suite                             ║\n")
	fmt.Printf("║  llama-swap : %-38s ║\n", llamaSwapAddr())
	fmt.Printf("║  llamactl   : %-38s ║\n", llamaCtlAddr())
	fmt.Printf("╚══════════════════════════════════════════════════════╝\n\n")
	os.Exit(m.Run())
}
