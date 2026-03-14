# E2E Test Suite

End-to-end tests for the macOS Local AI Stack. These tests run against
**live services** and validate that the full stack is working correctly.

## Prerequisites

Both services must be running before executing E2E tests:

```bash
llamactl start       # llama-swap on :8080
llamactl web start   # llamactl web dashboard on :3333
```

## Running tests

```bash
# From ~/AI/llamactl

# Fast: health + model listing + API shape checks (~5 s)
make test-e2e

# Inference: loads models and validates responses (~3 min)
make test-inference

# Large models (>14B, e.g. mistral-small-3.1-24b)
LLAMACTL_TEST_LARGE=1 make test-inference

# Full suite (fast + inference)
go test -v -tags e2e -timeout 600s ./test/e2e/...

# Single test
go test -v -tags e2e -timeout 60s -run TestModels_AllExpectedPresent ./test/e2e/...
```

## Build tag

All files use `//go:build e2e`. This means:

- `go test ./...` — **skips all E2E tests** (safe for CI without live services)
- `go test -tags e2e ./test/e2e/...` — runs E2E tests

## Test files

### `suite_test.go`
Shared infrastructure for all tests:
- `llamaSwapAddr()` / `llamaCtlAddr()` — read `LLAMA_SWAP_ADDR` / `LLAMACTL_ADDR`
  env vars or default to `localhost:8080` / `localhost:3333`
- `postJSON()` / `getJSON()` — typed HTTP helpers
- `skipIfUnavailable()` — skip test if service is not reachable
- `mustParseJSON()` — unmarshal with `t.Fatal` on error
- `expectedModels` — canonical list of all 13 model IDs
- Shared response types: `modelsResponse`, `chatResponse`, `embeddingResponse`, etc.
- `TestMain` — prints ASCII banner with addresses before running tests

### `health_test.go`
Connectivity checks — these are the fastest tests (~1ms each):

| Test | What it checks |
|------|---------------|
| `TestLlamaSwap_IsUp` | `GET /running` returns 200 |
| `TestLlamaSwap_RunningResponseShape` | `/running` JSON has `running` array |
| `TestLlamaCtlWeb_IsUp` | `GET /` returns 200 |
| `TestLlamaCtlWeb_APIStatus` | `GET /api/status` returns valid JSON |

### `models_list_test.go`
Model registry validation:

| Test | What it checks |
|------|---------------|
| `TestModels_NotEmpty` | `/v1/models` returns ≥ 1 model |
| `TestModels_AllExpectedPresent` | All 13 expected model IDs present |
| `TestModels_ResponseShape` | Each model entry has `id` and `object` fields |
| `TestModels_LlamaCtlMirror` | llamactl `/api/models` count matches llama-swap |

### `llamactl_api_test.go`
llamactl web API validation:

| Test | What it checks |
|------|---------------|
| `TestLlamaCtlAPI_StatusShape` | `/api/status` schema (IsRunning, APIReachable, etc.) |
| `TestLlamaCtlAPI_StatusLlamaSwapRunning` | Status correctly reports llama-swap as running |
| `TestLlamaCtlAPI_Logs_LlamaSwap` | `/api/logs` returns ≥ 1 log line |
| `TestLlamaCtlAPI_Logs_LineLimit` | `lines=3` query param respected |
| `TestLlamaCtlAPI_Config_Get` | `/api/config` returns non-empty YAML |
| `TestLlamaCtlAPI_Models` | `/api/models` returns API models + GGUF + HF counts |
| `TestLlamaCtlAPI_InvalidMethod` | GET-only endpoints return 405 for POST/DELETE |

### `inference_test.go`
Model inference validation (loads models on demand):

| Test | Model | What it checks | Timeout |
|------|-------|---------------|---------|
| `TestInference_ChatCompletion_Fast` | qwen2.5-3b | Chat returns a text response | 60s |
| `TestInference_ChatCompletion_Streamed` | qwen2.5-3b | SSE stream starts flowing | 30s |
| `TestInference_Embeddings` | nomic-embed-text-v1.5 | Returns 768-dim float vector | 30s |
| `TestInference_AllModels_Smoke` | all chat models | Each model returns 200 with choices | 600s |

The AllModels smoke test covers 8 standard models (≤14B). Set
`LLAMACTL_TEST_LARGE=1` to also test `mistral-small-3.1-24b` (24B model,
may take >120s to load).

## Environment variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `LLAMA_SWAP_ADDR` | `http://localhost:8080` | llama-swap address |
| `LLAMACTL_ADDR` | `http://localhost:3333` | llamactl web address |
| `LLAMACTL_TEST_LARGE` | `` | Set to `1` to include >14B models in smoke test |

## Adding a test

1. Pick the appropriate file (or create a new one).
2. Add `//go:build e2e` at the top if creating a new file.
3. Use `skipIfUnavailable(t, llamaSwapAddr()+"/running")` at the start of
   tests that need a live llama-swap.
4. Use shared helpers from `suite_test.go` — avoid raw `http.Get`.
5. Keep fast tests in `health_test.go`, `models_list_test.go`, or
   `llamactl_api_test.go`. Put anything that loads a model in `inference_test.go`.
6. Update `make test-e2e` regex if adding to a fast test file.
