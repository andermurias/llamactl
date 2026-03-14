//go:build e2e

package e2e_test

import (
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// ── Inference tests ───────────────────────────────────────────────────────────
// These tests actually load a model and call the chat/embeddings API.
// They are slower (model loading: 30–120 s depending on model size and cache).
//
// The fast inference test uses qwen2.5-3b (smallest available, ~2 GB) and
// requests only max_tokens=5 to minimise generation time.
//
// Run individually via:
//
//	make test-inference
//	go test -v -tags e2e -timeout 300s -run TestInference ./test/e2e/...

// TestInference_ChatCompletion_Fast loads qwen2.5-3b and performs a minimal
// completion to verify the full chat pipeline works end-to-end.
func TestInference_ChatCompletion_Fast(t *testing.T) {
	skipIfUnavailable(t, llamaSwapAddr()+"/running")
	t.Log("loading qwen2.5-3b (may take up to 60 s on first call)…")

	start := time.Now()
	code, body := postJSON(t, llamaSwapAddr()+"/v1/chat/completions", chatRequest{
		Model: "qwen2.5-3b",
		Messages: []chatMessage{
			{Role: "user", Content: "Reply with exactly the word PONG."},
		},
		MaxTokens: 10,
		Stream:    false,
	})
	elapsed := time.Since(start).Round(time.Millisecond)

	if code != http.StatusOK {
		t.Fatalf("POST /v1/chat/completions: expected 200, got %d\nbody: %s", code, body)
	}

	var resp chatResponse
	mustParseJSON(t, body, &resp)

	if len(resp.Choices) == 0 {
		t.Fatalf("chat response has no choices; body: %s", body)
	}
	content := resp.Choices[0].Message.Content
	if content == "" {
		t.Errorf("choices[0].message.content is empty")
	}
	if resp.Model == "" {
		t.Error("response model field is empty")
	}

	t.Logf("✓ qwen2.5-3b responded in %s: %q", elapsed, content)
}

// TestInference_ChatCompletion_Streamed verifies streaming (SSE) works on the
// fast model. Only checks the response header and first few bytes.
func TestInference_ChatCompletion_Streamed(t *testing.T) {
	skipIfUnavailable(t, llamaSwapAddr()+"/running")
	t.Log("streaming test via qwen2.5-3b…")

	buf, _ := marshalJSON(chatRequest{
		Model:     "qwen2.5-3b",
		Messages:  []chatMessage{{Role: "user", Content: "Say hi."}},
		MaxTokens: 5,
		Stream:    true,
	})
	req, _ := newRequest("POST", llamaSwapAddr()+"/v1/chat/completions", strings.NewReader(string(buf)))
	req.Header.Set("Content-Type", "application/json")

	streamClient := &http.Client{Timeout: 60 * time.Second}
	resp, err := streamClient.Do(req)
	if err != nil {
		t.Fatalf("streaming request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("streaming: expected 200, got %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/event-stream") {
		t.Errorf("expected text/event-stream content-type for streaming, got %q", ct)
	}
	// Read enough to confirm SSE data is flowing.
	buf2 := make([]byte, 64)
	n, _ := resp.Body.Read(buf2)
	if n == 0 {
		t.Error("no bytes received in streaming response")
	}
	t.Logf("✓ streaming OK, first %d bytes: %q", n, buf2[:n])
}

// TestInference_Embeddings verifies the embeddings pipeline via nomic-embed.
func TestInference_Embeddings(t *testing.T) {
	skipIfUnavailable(t, llamaSwapAddr()+"/running")
	t.Log("loading nomic-embed-text-v1.5…")

	start := time.Now()
	code, body := postJSON(t, llamaSwapAddr()+"/v1/embeddings", embeddingRequest{
		Model: "nomic-embed-text-v1.5",
		Input: "llamactl is an AI stack management tool",
	})
	elapsed := time.Since(start).Round(time.Millisecond)

	if code != http.StatusOK {
		t.Fatalf("POST /v1/embeddings: expected 200, got %d\nbody: %s", code, body)
	}

	var resp embeddingResponse
	mustParseJSON(t, body, &resp)

	if len(resp.Data) == 0 {
		t.Fatalf("embeddings response has no data; body: %s", body)
	}
	emb := resp.Data[0].Embedding
	if len(emb) == 0 {
		t.Error("embedding vector is empty")
	}
	// nomic-embed produces 768-dim vectors.
	if len(emb) < 64 {
		t.Errorf("embedding vector suspiciously short: %d dims", len(emb))
	}

	t.Logf("✓ nomic-embed responded in %s: %d-dim vector", elapsed, len(emb))
}

// TestInference_AllModels_Smoke iterates every chat model and sends a single
// ultra-short request to verify each one loads and responds.  This test is
// intentionally excluded from the default test-e2e target because it loads
// all models sequentially and takes several minutes.
//
// Run explicitly with:
//
//	go test -v -tags e2e -timeout 1800s -run TestInference_AllModels_Smoke ./test/e2e/...
func TestInference_AllModels_Smoke(t *testing.T) {
	skipIfUnavailable(t, llamaSwapAddr()+"/running")

	// Standard chat models (≤14B): load in ~10-12s each on Apple Silicon.
	// These always run in the smoke test.
	standardModels := []string{
		"gemma-3-12b-it",
		"gemma-3-12b-it-mlx",
		"qwen3.5-9b",
		"qwen2.5-3b",
		"qwen2.5-coder-14b",
		"qwen2.5-14b",
		"phi-4",
		"mistral-nemo-12b",
	}

	// Large models (>14B): require LLAMACTL_TEST_LARGE=1 because llama-swap's
	// internal health-check timeout (120 s) can be exceeded on constrained hardware.
	// Run them with: LLAMACTL_TEST_LARGE=1 go test -v -tags e2e -timeout 1800s ...
	largeModels := []string{
		"mistral-small-3.1-24b",
	}

	runLarge := os.Getenv("LLAMACTL_TEST_LARGE") == "1"

	modelsToTest := standardModels
	if runLarge {
		modelsToTest = append(modelsToTest, largeModels...)
	} else {
		t.Logf("ℹ  Skipping large models %v — set LLAMACTL_TEST_LARGE=1 to include them", largeModels)
	}

	for _, modelID := range modelsToTest {
		modelID := modelID // capture for subtest
		t.Run(modelID, func(t *testing.T) {
			t.Logf("loading %s…", modelID)
			start := time.Now()
			code, body := postJSON(t, llamaSwapAddr()+"/v1/chat/completions", chatRequest{
				Model:     modelID,
				Messages:  []chatMessage{{Role: "user", Content: "Say the single word: OK"}},
				MaxTokens: 5,
				Stream:    false,
			})
			elapsed := time.Since(start).Round(time.Second)

			if code != http.StatusOK {
				t.Errorf("model %s: expected 200, got %d\nbody: %s", modelID, code, body)
				return
			}
			var resp chatResponse
			mustParseJSON(t, body, &resp)
			if len(resp.Choices) == 0 {
				t.Errorf("model %s: no choices in response", modelID)
				return
			}
			t.Logf("✓ %s → %q  (%s)", modelID, resp.Choices[0].Message.Content, elapsed)
		})
	}
}
