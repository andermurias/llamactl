# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Skills

Load a skill with the `Skill` tool to get focused documentation on a specific area:

| Skill | Load when you need to… |
|-------|------------------------|
| `api-usage` | Use the OpenAI-compatible API or llamactl web API (`/api/*`) |
| `llama-swap` | Understand the proxy config, routing, groups, TTL, or SIGHUP reload |
| `llama-cpp` | Work with llama-server, GGUF models, Metal GPU offload, or embedding mode |
| `mlx-server` | Work with mlx_lm.server, MLX models, or unified-memory sizing |
| `llamactl-cli` | Use or extend CLI commands, or understand launchd service management |
| `llamactl-go` | Navigate the Go package layout, add commands/endpoints, or understand invariants |
| `llamactl-web` | Understand the dashboard, handlers.go, configure panel, or SSE patterns |
| `model-gemma` | Work with any Gemma model (3 12B GGUF/MLX/QAT, 4 E4B) |
| `model-qwen` | Work with any Qwen model (2.5-3B/7B/14B/Coder, 3.5-9B CoT) |
| `model-phi` | Work with Phi-4 (STEM/reasoning) |
| `model-embeddings` | Work with Nomic Embed for RAG or semantic search |
| `model-tts` | Work with Kokoro, Orpheus, XTTS, or Voxtral TTS |
| `model-stt` | Work with Whisper speech-to-text |

## What This Is

A self-hosted, OpenAI-compatible local AI inference stack for Apple Silicon Macs — no cloud, no API keys. `llama-swap` acts as a unified proxy (`localhost:8080`) routing requests by model name to the right backend. `llamactl` is the Go CLI and embedded web dashboard that manages the whole system.

## llamactl (Go CLI + Web UI)

All development work happens in `llamactl/`.

```bash
cd llamactl
make build          # Compile + ad-hoc codesign (required every build — see gotchas)
make install        # Build + deploy binary to ../scripts/llamactl
make dist           # install + reminder to commit bin/
make test           # Unit tests (includes launchd timeouts, ~35s)
make test-fast      # Excludes slow launchd tests
make test-web       # Web handler unit tests only
make test-e2e       # E2E: health + models + API checks, ~5s (requires live services)
make test-inference # E2E: loads models + calls inference, ~3 min
make test-api       # Shell integration tests (bash scripts/test-api.sh --fast)
make lint           # go vet + staticcheck
make clean          # Remove bin/llamactl
make help           # List all targets
```

The compiled binary is committed at `llamactl/bin/llamactl` and symlinked to `scripts/llamactl` → `/opt/homebrew/bin/llamactl`. This allows `git pull` + symlink update as a quick-update path without needing Go installed.

### Package layout

```
llamactl/
├── main.go                    Entry point — calls cmd.Execute()
├── cmd/                       Cobra command definitions only (no business logic)
│   ├── root.go                Global flags, cobra wiring, version embedding
│   ├── comfyui/               llamactl comfyui {start,stop,status,logs,setup}
│   ├── config/config.go       llamactl config {edit,show,validate,path,reload}
│   └── web/web.go             llamactl web {start,stop,restart,status,enable,disable}
└── internal/
    ├── config/config.go       Runtime constants: paths, launchd labels, ports
    ├── service/               Business logic — returns plain structs, no UI code
    │   ├── llamaswap.go       Start/stop/status for llama-swap
    │   └── web.go             Start/stop/status for web UI
    ├── launchd/               Plist write + bootstrap/bootout/kickstart helpers
    ├── llamaswap/             HTTP client for llama-swap API
    ├── comfyui/               ComfyUI process helpers
    ├── modelmanager/          HF search/install, yaml edit, enable/disable/remove
    ├── updater/               Version check against GitHub releases
    └── web/
        ├── server.go          HTTP mux, route registration, //go:embed directives
        ├── handlers.go        All /api/* handlers
        ├── handlers_test.go   Unit tests for handlers
        └── templates/index.html  Single-page dashboard (Go template, no framework)
```

**Critical invariant**: `internal/service/` must not import `internal/web/` or anything CLI-specific. Service functions return typed structs; both `cmd/` and `web/handlers.go` consume them independently.

## Central Configuration

`llama-swap.yaml` (repo root) is the single source of truth for all model routing. Every model entry has:

- `cmd` — full server launch command with `${PORT}` substitution
- `useModelName` — rewrites the model field sent upstream
- `checkEndpoint` — health-check path (default `/health`)
- `ttl` — idle-unload timeout in seconds
- `concurrencyLimit` — max parallel requests

Models in a group with `swap: true` are mutually exclusive — only one loads at a time to maximize GPU memory. Secondary services (TTS, STT, embeddings) live outside swap groups and coexist.

```bash
llamactl config edit      # opens in $EDITOR, validates on save
llamactl config reload    # SIGHUP — no restart needed
llamactl config validate
```

## Architecture

```
Client (OpenWebUI, curl, etc.)
    ↓ OpenAI API  http://localhost:8080
llama-swap (proxy + router)
    ├─→ llama-server     — GGUF models (Metal, -ngl 99)
    ├─→ mlx_lm.server    — MLX models (HF cache)
    ├─→ Kokoro-FastAPI   — TTS (Kokoro-82M, MPS), :8880
    ├─→ Orpheus-FastAPI  — TTS (Llama-3B + SNAC), :5005
    ├─→ voxtral-tts.c    — TTS (Voxtral-4B, pure C + Accelerate)
    └─→ whisper_server.py — STT (Whisper large-v3-turbo, MLX), :8778
```

The web UI (`localhost:3333`) embeds all HTML/JS/CSS into the binary via `//go:embed` — no separate frontend build step.

## Models

| Model key | Backend | Purpose | RAM |
|-----------|---------|---------|-----|
| `gemma-3-12b-it` | llama-server (GGUF) | Best Spanish + general chat | ~7 GB |
| `qwen3.5-9b` | mlx_lm.server | Reasoning / CoT (`/think` token), 32K ctx | ~6 GB |
| `qwen2.5-3b` | mlx_lm.server | Fast, 128K context | ~2 GB |
| `qwen2.5-coder-14b` | mlx_lm.server | Code generation | ~8 GB |
| `qwen2.5-14b` | mlx_lm.server | Agents / tool-calling / MCP | ~8 GB |
| `phi-4` | mlx_lm.server | STEM / reasoning | ~8 GB |
| `mistral-nemo-12b` | mlx_lm.server | Web search, internet-aware | ~7 GB |
| `mistral-small-3.1-24b` | mlx_lm.server | High-quality (large, slow to load) | ~14 GB |
| `qwen2.5-vl-7b` | llama-server (GGUF) | Vision — image+text→text | ~4.5 GB |
| `nomic-embed-text-v1.5` | llama-server (GGUF) | Embeddings / RAG (768 dims) | ~270 MB |
| `whisper-stt` | whisper_server.py (MLX) | Speech-to-text | ~1.6 GB |
| `kokoro-tts` | Kokoro-FastAPI | Text-to-speech | ~82 MB |
| `deepseek-r1-llama-8b` | mlx_lm.server | Fast coding / reasoning (~20 tok/s) | ~4.5 GB |
| `qwen3.5-9b-optiq` | mlx_lm.server | High-quality 9B (mixed-precision) | ~6 GB |
| `qwen3-14b` | mlx_lm.server | Large model for complex tasks | ~8.3 GB |

`mistral-small-3.1-24b` exceeds the 120s health-check timeout on first load — excluded from inference smoke test by default. Set `LLAMACTL_TEST_LARGE=1` to include it.

## Service Management

```bash
llamactl start / stop / restart / status
llamactl enable / disable          # launchd auto-start at login
llamactl logs / logs -f
llamactl web start / stop / status  # Dashboard on localhost:3333
llamactl comfyui start / stop / status / logs / setup
llamactl models                    # List registered models
llamactl upgrade                   # git pull + rebuild
```

## Ports

| Service | Port |
|---------|------|
| llama-swap proxy | 8080 |
| llamactl web dashboard | 3333 |
| ComfyUI | 8188 |
| Kokoro TTS | 8880 |
| Whisper STT | 8778 |
| mlx_lm / llama-server backends | auto-assigned (5800–5899 range) |

## Web API (llamactl-web, :3333)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/status` | llama-swap running state, PIDs, uptime |
| GET | `/api/models` | API models + GGUF files + HF cache |
| GET | `/api/logs?service=llamaswap&lines=N` | Last N log lines |
| GET/POST | `/api/config` | Get / write llama-swap.yaml (POST triggers SIGHUP reload) |
| POST | `/api/action` | `{"action":"start"\|"stop"\|"restart","service":"llamaswap"\|"comfyui"}` |
| GET | `/api/hf/search?q=<query>` | Search HuggingFace for models |
| GET | `/api/hf/info?id=<hf-id>` | Model info, MLX detection, GGUF file listing |
| POST | `/api/models/install` | Install a model (SSE streaming progress) |
| POST | `/api/models/manage` | Enable / disable / remove a model |
| GET | `/api/models/disabled` | List disabled models |
| GET | `/api/voices` | List uploaded voice files |
| POST | `/api/voices/upload` | Upload voice file (multipart/form-data) |
| DELETE | `/api/voices/delete?name=<file>` | Remove a voice file |

GET-only endpoints return `405 Method Not Allowed` for non-GET requests.

## E2E Tests

All E2E tests require `//go:build e2e` and are skipped by `go test ./...`.

```
llamactl/test/e2e/
├── suite_test.go        Shared helpers, HTTP client, types, TestMain banner
├── health_test.go       IsUp checks for llama-swap and web UI
├── models_list_test.go  NotEmpty, AllExpectedPresent, Shape, Mirror
├── llamactl_api_test.go /api/* endpoint tests (status/logs/config/models/hf-search/disabled/methods)
└── inference_test.go    ChatCompletion_Fast/Streamed/Embeddings/AllModels
```

Run E2E (requires `llamactl start && llamactl web start`):
```bash
make test-e2e        # ~5s
make test-inference  # ~3 min
LLAMACTL_TEST_LARGE=1 make test-inference  # includes 24B model
```

## Known Gotchas

1. **codesign required after every build** — macOS quarantine blocks unsigned binaries. `make build` and `make install` both run `codesign --force -s -`. Never skip this.

2. **launchd stale state after crash** — `launchctl print` can show `state = not running` even if the process is alive. `GetStatus()` in `internal/service/llamaswap.go` falls back to `pgrepFirst()` to handle this.

3. **`kickstart` revokes file descriptors** — After `kickstart` on an already-running process, launchd revokes stdout/stderr of the old process. The old process keeps serving HTTP but can't spawn subprocesses that write to stdout/stderr. Symptom: llama-swap returns "upstream command exited prematurely but successfully". Fix: kill the old PID then kickstart.

4. **Unit tests must not create real launchd services** — `newTestServer()` in `handlers_test.go` uses a per-test label (`com.llamastack.llamactl-test.<pid>`) and calls `launchd.Bootout` in `t.Cleanup`. Preserve this pattern.

5. **Streaming test asserts keepalive, not content** — The streaming E2E test checks `strings.Contains(body, ": keepalive")` because model loading time is variable; actual content chunks aren't reliable.

6. **mlx_lm.server health endpoint** — Uses `/health` (returns `{"status":"ok"}`). llama-server uses `/v1/models`. Both are configured via `checkEndpoint` in `llama-swap.yaml`.

## modelmanager Package

`internal/modelmanager/` handles HuggingFace search, model install, and enable/disable/remove:

| File | Purpose |
|------|---------|
| `types.go` | HFModel, ModelConfig, DisabledStore, InstallRequest, ManageRequest |
| `hf.go` | HF API client: search, model info, MLX detection, GGUF file/repo finding |
| `yamledit.go` | yaml.v3 Node manipulation preserving comments in llama-swap.yaml |
| `install.go` | Install workflow: detect type, MLX config gen, GGUF download |
| `manage.go` | Enable/Disable/Remove using disabled store file |

Key behaviors:
- Disabled models stored in `~/AI/llamactl-disabled.yaml` (separate from llama-swap.yaml)
- MLX preferred over GGUF on Apple Silicon; `FindMLXVariant()` checks multiple quant suffixes
- Default GGUF quantization: Q4_K_M
- `DeriveModelID`: `mlx-community/gemma-3-12b-it-4bit` → `gemma-3-12b-it-mlx`
- Install SSE streaming: ends with `data: RESULT:{json}\n\n`

## How to Add a New Model

1. Add a stanza to `llama-swap.yaml` following an existing example.
2. Add the model ID to `expectedModels` in `llamactl/test/e2e/suite_test.go`.
3. Run `make test-e2e` to verify it appears in `/v1/models`.

Via CLI:
```bash
llamactl models install mlx-community/phi-4-4bit      # MLX
llamactl models install bartowski/Phi-4-GGUF --type gguf --quant Q4_K_M
```

## How to Add a New CLI Command

1. Create `llamactl/cmd/myfeature/myfeature.go` with `func NewCmd(cfg *config.Config) *cobra.Command`.
2. Register in `llamactl/cmd/root.go`: `rootCmd.AddCommand(myfeature.NewCmd(cfg))`.
3. Business logic goes in `internal/service/` — no pterm/fmt output there.

## How to Add a New Web API Endpoint

1. Add handler method to `internal/web/handlers.go`. Use `getOnly(w, r)` for GET-only endpoints.
2. Register route in `internal/web/server.go` inside `routes()`.
3. Add unit test to `internal/web/handlers_test.go`.
4. Add E2E test in `test/e2e/llamactl_api_test.go`.

## TTS Backends

Three engines, all expose OpenAI-compatible `/v1/audio/speech`:

- **Kokoro-FastAPI** (`Kokoro-FastAPI/`) — 20+ voices, streaming, word timestamps. Best for English.
- **Orpheus-FastAPI** (`Orpheus-FastAPI/`) — Emotion tags (`<laugh>`, `<sigh>`), 24 voices. Best for Spanish/Italian.
- **voxtral-tts.c** (`voxtral-tts.c/`) — Pure C, no Python. Build: `make apple` (macOS Accelerate), `make blas` (Linux), `make cuda` (NVIDIA).

## Voice Management (XTTS)

Upload custom voice samples for XTTS voice cloning via the dashboard or API.

**API**:
- `GET /api/voices` — list uploaded voices
- `POST /api/voices/upload` — multipart upload (`voice` file, optional `name`)
- `DELETE /api/voices/delete?name=<file>` — remove voice

**Storage**: `~/AI/voices/`
**Supported formats**: WAV, MP3, OGG, FLAC (max 10 MB, 6–30s recommended)

## Environment

- **OS**: macOS 14+ (Darwin arm64)
- **Go**: 1.24+
- **Python**: Conda env `mlx-server` at `/opt/homebrew/Caskroom/miniforge/base/envs/mlx-server/`
- **Config file**: `~/AI/llama-swap.yaml`
- **Log directory**: `~/AI/logs/`
- **launchd labels**: `com.llamastack.llama-swap`, `com.llamastack.llamactl-web`, `com.llamastack.comfyui`
