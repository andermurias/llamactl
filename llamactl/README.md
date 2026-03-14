# llamactl

> CLI + Web UI for managing a local AI stack on Apple Silicon

`llamactl` is the management layer for the [macOS Local AI Stack](../README.md). It wraps `llama-swap` and `ComfyUI` with a polished terminal interface and an embedded web dashboard, using `launchd` for process supervision.

---

## Installation

The first-time installer handles everything:

```bash
git clone <repo-url> ~/AI
cd ~/AI
bash scripts/install.sh
```

To install or update just the `llamactl` binary:

```bash
cd ~/AI/llamactl
make install    # build + sign + deploy to ../scripts/llamactl
```

---

## CLI reference

### llama-swap

```bash
llamactl start            # start llama-swap (auto-installs launchd on first run)
llamactl stop             # stop gracefully
llamactl restart          # stop + start
llamactl status           # PID, uptime, loaded models, auto-start state

llamactl enable           # auto-start at login
llamactl disable          # disable auto-start

llamactl logs             # last 100 lines
llamactl logs -f          # follow in real time

llamactl models           # list all models registered with llama-swap
llamactl upgrade          # update llama-swap binary via git pull
llamactl version          # llamactl version + llama-swap version
llamactl --verbose <cmd>  # extra detail on any command
```

### ComfyUI

```bash
llamactl comfyui start    # start ComfyUI on port 8188
llamactl comfyui stop     # stop ComfyUI
llamactl comfyui status   # process state, PID, uptime
llamactl comfyui logs     # tail ComfyUI log
```

### Web UI

```bash
llamactl web start        # start the embedded web UI (port 3333)
llamactl web stop         # stop the web UI
llamactl web restart      # restart the web UI
llamactl web status       # PID, uptime, URL
llamactl web enable       # auto-start web UI at login
llamactl web disable      # disable auto-start
llamactl web logs         # tail the web UI log
llamactl web install      # install without starting
llamactl web uninstall    # remove from launchd
```

Open `http://localhost:3333` to access the dashboard.

### Config editor

```bash
llamactl config edit      # open llama-swap.yaml in $EDITOR; validates before saving
llamactl config show      # print current config to stdout
llamactl config validate  # validate YAML without editing
llamactl config path      # print the path of the config file
llamactl config reload    # send SIGHUP to reload config without restart
```

### Self-upgrade

```bash
llamactl upgrade --self   # git pull + rebuild + redeploy binary
```

---

## Web UI

The embedded web dashboard runs at `http://localhost:3333`.

| Section  | What it shows |
|----------|---------------|
| Services | llama-swap and ComfyUI status, PID, uptime, action buttons |
| Models   | All registered models; loaded/idle badge |
| Logs     | Live-tailing log viewer (llamaswap or ComfyUI) |
| Config   | YAML editor with load/save; llama-swap auto-reloads on save |

The sidebar is always visible on desktop (≥ 768 px) and slides in from the left on mobile.

---

## Development

### Prerequisites

- Go 1.24+
- `codesign` (included with macOS Xcode CLT)

### Build

```bash
cd ~/AI/llamactl

make build    # compile → bin/llamactl (ad-hoc code-signed)
make install  # build + deploy to ../scripts/llamactl
make dist     # build + install + remind to commit bin/llamactl
make test     # run unit tests  (go test -race ./...)
make lint     # go vet + staticcheck
make clean    # remove bin/llamactl
```

### Running tests

```bash
# Unit tests — no live services required (~35s)
go test -race ./internal/...

# Fast E2E tests — health + model listing + API checks, no inference (~5s)
# Requires: llamactl start && llamactl web start
make test-e2e

# Inference E2E — loads models and calls the API (~3 min)
make test-inference

# Include large models (>14B) in inference smoke test
LLAMACTL_TEST_LARGE=1 make test-inference

# Shell integration tests (legacy — bash, requires live llama-swap)
bash ../scripts/test-api.sh --fast
```

#### E2E test layout

```
test/e2e/                    build tag: //go:build e2e
├── suite_test.go            HTTP helpers, shared types, TestMain banner
├── health_test.go           Connectivity: llama-swap /running, llamactl-web /
├── models_list_test.go      /v1/models — count, shape, all 13 models present
├── llamactl_api_test.go     /api/status /api/logs /api/config /api/models
└── inference_test.go        Chat, streaming, embeddings, AllModels smoke
```

All E2E tests are skipped by default (`go test ./...`). Use the Makefile targets or
pass `-tags e2e` explicitly.

### Package layout

```
llamactl/
├── main.go                    entry point; cobra root command
├── Makefile
├── bin/llamactl               compiled binary committed to git (for git-based upgrade)
├── cmd/
│   ├── root.go                global flags, version, command wiring
│   ├── start.go               llamactl start/stop/restart/status/…
│   ├── models.go              llamactl models
│   ├── logs.go                llamactl logs
│   ├── upgrade.go             llamactl upgrade / llamactl upgrade --self
│   ├── web/web.go             llamactl web subcommand group
│   └── config/config.go       llamactl config subcommand group
└── internal/
    ├── config/config.go       runtime paths & constants
    ├── launchd/               launchd plist write + bootstrap/bootout/kickstart helpers
    ├── llamaswap/             llama-swap API client (models, running, health)
    ├── service/               business logic: start/stop/status for all services
    ├── comfyui/               ComfyUI process helpers
    ├── updater/               version check against GitHub releases
    └── web/                   embedded HTTP server (handlers, templates, static)
```

### Adding a new command

1. Create `cmd/myfeature/myfeature.go` with a `NewCmd(cfg) *cobra.Command` constructor.
2. Register it in `cmd/root.go`: `rootCmd.AddCommand(myfeature.NewCmd(cfg))`.
3. Put business logic in `internal/service/` (no UI code there).
4. Write unit tests alongside the package.

### Adding a new web API route

1. Add the handler method to `internal/web/handlers.go`.
2. Register the route in `internal/web/server.go` inside `routes()`.
3. Add tests to `internal/web/handlers_test.go`.

---

## Architecture notes

- **No UI in service layer** — `internal/service` returns plain structs/errors.  All terminal output lives in `cmd/`.  All HTTP output lives in `internal/web/handlers.go`.  This makes the service layer trivially testable and reusable from a future HTTP controller.
- **launchd supervision** — each service (llama-swap, web UI) is a proper `LaunchAgent`. This means auto-restart on crash, log rotation, and clean process lifecycle management.
- **Embedded web assets** — HTML templates and static files are compiled into the binary via `//go:embed`, so the web UI ships as a single self-contained binary.
- **Git-based upgrade** — `llamactl upgrade --self` does `git pull` from `~/AI`, rebuilds the binary, re-signs it with `codesign --force -s -`, and redeploys to `scripts/llamactl`.  No GitHub release API or external download required.

---

## Changelog

See [CHANGELOG.md](CHANGELOG.md).
