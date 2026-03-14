# Copilot Instructions — macOS Local AI Stack

> This file gives AI assistants (GitHub Copilot, Claude, GPT-4, etc.) full context
> to continue work on this project without needing to re-read the whole codebase.
> Keep it up to date when you make significant changes.

---

## What this project is

A **self-hosted, OpenAI-compatible local AI stack** for Apple Silicon Macs. It runs
entirely offline — no cloud, no API keys.

The stack exposes a **single endpoint** (`http://localhost:8080`) that routes
to multiple AI backends by model name, managed by
[llama-swap](https://github.com/mostlygeek/llama-swap).

**`llamactl`** is the management layer: a Go CLI + embedded web dashboard for
controlling all services, viewing logs, and editing config.

---

## Repo layout

```
~/AI/                          ← git root
├── llama-swap.yaml            ← Central config — ALL model routing lives here
├── README.md                  ← User-facing docs (installation, API usage, OpenWebUI)
├── llamactl/                  ← Go CLI + web UI source
├── scripts/                   ← Shell helpers, installers, test scripts
├── models/                    ← GGUF model files (git-ignored)
│   ├── chat/
│   └── embeddings/
├── logs/                      ← Runtime logs (git-ignored)
├── Kokoro-FastAPI/            ← TTS server (git-ignored, cloned by installer)
└── .github/
    ├── workflows/release.yml  ← GoReleaser on git tag push
    └── copilot-instructions.md ← THIS FILE
```

---

## llamactl structure (Go project)

```
llamactl/
├── main.go                    Entry point — calls cmd.Execute()
├── go.mod                     Module: github.com/andermurias/llamactl
├── Makefile                   Build / test targets (see below)
├── bin/llamactl               COMPILED BINARY committed to git (required for git-pull upgrade)
├── cmd/
│   ├── root.go                Global flags, cobra wiring, version embedding
│   ├── start.go               llamactl start / stop / restart / status
│   ├── enable.go / disable.go launchd auto-start toggle
│   ├── logs.go                llamactl logs [-f] [--lines N]
│   ├── models.go              llamactl models
│   ├── upgrade.go             llamactl upgrade [--self] [--check]
│   ├── install.go             llamactl install (install launchd plist)
│   ├── uninstall.go           llamactl uninstall
│   ├── status.go              llamactl status
│   ├── version.go             llamactl version
│   ├── helpers.go             Shared CLI output helpers (pterm wrappers)
│   ├── comfyui/               llamactl comfyui {start,stop,status,logs}
│   ├── config/config.go       llamactl config {edit,show,validate,path,reload}
│   └── web/web.go             llamactl web {start,stop,restart,status,enable,disable,...}
└── internal/
    ├── config/config.go       Runtime constants: paths, launchd labels, ports
    ├── launchd/               Plist write + bootstrap/bootout/kickstart/print helpers
    ├── llamaswap/             HTTP client for llama-swap API (models, running, health)
    ├── service/
    │   ├── llamaswap.go       Start/stop/status/GetStatus for llama-swap service
    │   └── web.go             Start/stop/status for the web UI service
    ├── comfyui/               ComfyUI process lifecycle helpers
    ├── updater/               Version check against GitHub releases
    └── web/
        ├── server.go          HTTP mux, route registration, embed directives
        ├── handlers.go        All API handlers (/api/status, /api/models, /api/logs, /api/config)
        ├── handlers_test.go   Unit tests for handlers
        └── templates/         HTML templates (compiled into binary via //go:embed)
            └── index.html     Single-page dashboard (Go template, no framework)
```

---

## Key architectural decisions

### 1. Service layer has NO UI code
`internal/service` returns plain structs and errors. All terminal output
lives in `cmd/`, all HTTP output in `internal/web/handlers.go`.
This makes the service layer testable and ready for a future HTTP controller.

### 2. launchd supervision
Every service (llama-swap, web UI) is a proper `LaunchAgent` plist written to
`~/Library/LaunchAgents/`. This gives: auto-restart on crash, log rotation,
clean start/stop lifecycle via `launchctl`.

**launchd quirks to know:**
- After a crash, `launchctl print` can show `state = not running` even if the
  process is still alive (PID field disappears). `GetStatus()` falls back to
  `pgrep -x <name>` to handle stale state.
- `KeepAlive.Crashed = true` means launchd only auto-restarts on non-zero exit.
  A clean SIGTERM exit requires manual kickstart.
- After `kickstart` on an already-running process, launchd **revokes the stdout/
  stderr file descriptors** of the old process. The old process continues
  responding to HTTP but cannot spawn subprocesses that write to stdout/stderr.
  Symptom: llama-swap returns "upstream command exited prematurely but
  successfully". Fix: kill the old PID and use `launchctl kickstart` for a fresh start.

### 3. Embedded web assets
HTML templates and static files are compiled into the binary via `//go:embed`.
The binary is entirely self-contained — no external files needed at runtime.

### 4. Git-based upgrade
`llamactl upgrade --self` does:
1. `git pull` from `~/AI`
2. `make build` (rebuilds from source)
3. `codesign --force -s -` (ad-hoc re-signing for macOS Gatekeeper)
4. Deploys to `../scripts/llamactl`

The pre-built binary in `llamactl/bin/llamactl` is committed so that
`git pull` + symlink update is also a valid quick-update path (no Go needed).

### 5. E2E tests use build tags
All E2E tests live in `llamactl/test/e2e/` and require `//go:build e2e`.
They are SKIPPED by `go test ./...`. Run explicitly:
```bash
make test-e2e        # fast: health + model list + API checks (~5s)
make test-inference  # slow: loads models + calls inference (~3 min)
```

---

## Models

| Model key | Backend | Purpose | RAM |
|-----------|---------|---------|-----|
| `gemma-3-12b-it` | llama-server (GGUF) | Best Spanish + general chat | ~7 GB |
| `gemma-3-12b-it-mlx` | mlx_lm.server | Same model, MLX-native | ~7 GB |
| `qwen3.5-9b` | mlx_lm.server | Reasoning / CoT (`/think` token), 32K ctx | ~6 GB |
| `qwen2.5-3b` | mlx_lm.server | Fast, 128K context | ~2 GB |
| `qwen2.5-coder-14b` | mlx_lm.server | Code generation | ~8 GB |
| `qwen2.5-14b` | mlx_lm.server | Agents / tool-calling / MCP | ~8 GB |
| `phi-4` | mlx_lm.server | STEM / reasoning | ~8 GB |
| `mistral-nemo-12b` | mlx_lm.server | Web search, internet-aware | ~7 GB |
| `mistral-small-3.1-24b` | mlx_lm.server | High-quality alternative (large) | ~14 GB |
| `qwen2.5-vl-7b` | llama-server (GGUF) | Vision — image+text→text | ~4.5 GB |
| `nomic-embed-text-v1.5` | llama-server (GGUF) | Embeddings / RAG (768 dims) | ~270 MB |
| `whisper-stt` | whisper_server.py | Speech-to-text (Whisper large-v3-turbo) | ~1.6 GB |
| `kokoro-tts` | Kokoro-FastAPI | Text-to-speech (OpenAI /v1/audio/speech) | ~82 MB |

**Only one model is active at a time.** llama-swap unloads idle models after TTL expires.

`mistral-small-3.1-24b` requires >120s to load — it is excluded from the AllModels
smoke test by default. Set `LLAMACTL_TEST_LARGE=1` to include it.

---

## launchd service labels

| Label | Service | Plist path |
|-------|---------|-----------|
| `com.llamastack.llama-swap` | llama-swap proxy | `~/Library/LaunchAgents/` |
| `com.llamastack.llamactl-web` | Web dashboard | `~/Library/LaunchAgents/` |
| `com.llamastack.comfyui` | ComfyUI | `~/Library/LaunchAgents/` |

---

## Ports

| Service | Port |
|---------|------|
| llama-swap proxy | 8080 |
| llamactl web dashboard | 3333 |
| ComfyUI | 8188 |
| mlx_lm model servers | auto-assigned (5800–5899 range) |
| llama-server backends | auto-assigned (5800–5899 range) |
| Kokoro TTS | 8880 |
| Whisper STT | 8778 |

---

## Environment

- **OS**: macOS 14+ (Darwin arm64)
- **Go**: 1.24+
- **Python**: Conda env `mlx-server` at `/opt/homebrew/Caskroom/miniforge/base/envs/mlx-server/`
- **mlx_lm**: `mlx_lm.server` binary in the conda env
- **llama-swap binary**: `~/AI/scripts/llama-swap` (downloaded by installer)
- **llamactl binary**: `~/AI/scripts/llamactl` (symlinked from `/opt/homebrew/bin/llamactl`)
- **Config file**: `~/AI/llama-swap.yaml`
- **Log directory**: `~/AI/logs/`

---

## Makefile targets

```bash
make build          # compile bin/llamactl + codesign
make install        # build + deploy to ../scripts/llamactl
make dist           # install + reminder to commit bin/
make test           # unit tests (go test -race ./...)
make test-fast      # unit tests, skipping slow launchd tests
make test-web       # web handler unit tests only
make test-e2e       # E2E: health + model list + API checks (~5s, needs live services)
make test-inference # E2E: inference tests (~3 min, loads models)
make test-api       # Shell integration tests (bash scripts/test-api.sh --fast)
make lint           # go vet + staticcheck
make clean          # remove bin/llamactl
make help           # list all targets
```

---

## Running tests

### Unit tests (no live services required)
```bash
cd ~/AI/llamactl
go test -race ./internal/...   # fast, ~35s
```

### E2E tests (require live llama-swap on :8080 + llamactl-web on :3333)
```bash
# Start services first
llamactl start && llamactl web start

# Fast E2E (health, models, API — no model loading)
make test-e2e

# Inference E2E (loads models, ~3 min)
make test-inference

# All models including large (>14B)
LLAMACTL_TEST_LARGE=1 make test-inference
```

### E2E test layout
```
llamactl/test/e2e/
├── suite_test.go       Shared helpers, HTTP client, types, TestMain banner
├── health_test.go      TestLlamaSwap_IsUp, TestLlamaCtlWeb_IsUp, etc.
├── models_list_test.go TestModels_NotEmpty, AllExpectedPresent, Shape, Mirror
├── llamactl_api_test.go TestLlamaCtlAPI_* (status/logs/config/models/methods)
└── inference_test.go   TestInference_ChatCompletion_Fast/Streamed/Embeddings/AllModels
```

---

## Web API (llamactl-web, :3333)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/` | Dashboard HTML |
| GET | `/api/status` | JSON: llama-swap running state, PIDs, uptime |
| GET | `/api/models` | JSON: API models + GGUF files + HF cache |
| GET | `/api/logs?service=llamaswap&lines=N` | Last N log lines |
| GET | `/api/config` | Current llama-swap.yaml content |
| POST | `/api/config` | Write new YAML; triggers llama-swap SIGHUP reload |
| POST | `/api/action` | `{"action": "start"\|"stop"\|"restart", "service": "llamaswap"\|"comfyui"}` |

All GET-only endpoints return `405 Method Not Allowed` for non-GET requests.

---

## llama-swap API (proxy, :8080)

Follows the OpenAI API spec:

| Method | Path | Description |
|--------|------|-------------|
| GET | `/v1/models` | List all configured models |
| POST | `/v1/chat/completions` | Chat (set `"model"` to route) |
| POST | `/v1/embeddings` | Embeddings (use `nomic-embed-text-v1.5`) |
| POST | `/v1/audio/speech` | TTS (use `kokoro-tts`) |
| POST | `/v1/audio/transcriptions` | STT (use `whisper-stt`) |
| GET | `/running` | List currently loaded model subprocesses |
| GET | `/upstream/{model}/...` | Direct proxy to a specific model's backend |

---

## Known issues and gotchas

1. **`mistral-small-3.1-24b` 502 on first load** — 24B model exceeds the 120s
   health-check timeout in llama-swap. Not a bug; the model is just large.
   Set `idleTimeout: 0` and a longer `healthCheckTimeout` in the yaml if needed.

2. **launchd stale state after crash** — `launchctl print` shows
   `state = not running` even though the process is alive. `GetStatus()` in
   `internal/service/llamaswap.go` handles this via `pgrepFirst()` fallback.

3. **Unit tests must not create real launchd services** — `newTestServer()` in
   `handlers_test.go` uses a unique per-test label (`com.llamastack.llamactl-test.<pid>`)
   and calls `launchd.Bootout` in `t.Cleanup`. Do not change this pattern.

4. **codesign required after every build** — macOS quarantine will block execution
   otherwise. `make build` and `make install` both call `codesign --force -s -`.

5. **mlx_lm.server health endpoint** — Use `/health` (returns `{"status":"ok"}`).
   llama-server uses `/v1/models` as health check (`checkEndpoint` in yaml).

6. **Streaming test quirk** — The streaming E2E test validates keepalive SSE
   bytes (`: keepalive N/8\n\n`), NOT actual content chunks, because model
   loading time is variable. The assertion checks `strings.Contains(body, ": keepalive")`.

---

## How to add a new model

1. Download the model (GGUF) or note the HuggingFace model ID (MLX).
2. Add a stanza to `llama-swap.yaml` following an existing example.
3. Add the model ID to `expectedModels` in `llamactl/test/e2e/suite_test.go`.
4. Run `make test-e2e` to verify the model appears in `/v1/models`.
5. Run `make test-inference` (optional but recommended) to verify inference works.

---

## How to add a new CLI command

1. Create `llamactl/cmd/myfeature/myfeature.go` with:
   ```go
   func NewCmd(cfg *config.Config) *cobra.Command { ... }
   ```
2. Register in `llamactl/cmd/root.go`:
   ```go
   rootCmd.AddCommand(myfeature.NewCmd(cfg))
   ```
3. Business logic goes in `internal/service/` — no pterm/fmt output there.
4. Add unit tests next to the new package.

## How to add a new web API endpoint

1. Add handler method to `internal/web/handlers.go`.
   - Use `getOnly(w, r)` if it should only accept GET.
   - Return JSON via `json.NewEncoder(w).Encode(...)`.
2. Register route in `internal/web/server.go` inside the `routes()` function.
3. Add test to `internal/web/handlers_test.go`.
4. Add E2E test in `test/e2e/llamactl_api_test.go`.

---

## Session history checkpoints

Detailed history of every work session is in:
```
~/.copilot/session-state/<session-id>/checkpoints/
```
Key sessions:
- `001` — MLX stack setup + model config
- `002` — Multimodal (vision, STT, repo setup)
- `003` — Go rewrite of llamactl CLI
- `004` — pterm UI + git-based upgrade
- `005` — Web UI service layer + launchd refactor
- `006` — Responsive web UI redesign
- `007` — Unified UI redesign, tests, docs
- `008` — E2E test suite + API fixes (handler method enforcement, test isolation,
           embeddings struct tag bug, pgrepFirst fallback for stale launchd state)
