#!/usr/bin/env bash
# install.sh — Interactive installer for the macOS Local AI Stack
#
# What this does:
#   1. Verify macOS + Apple Silicon
#   2. Install Homebrew packages (miniforge, ffmpeg, espeak-ng, uv)
#   3. Create conda env 'mlx-server' with mlx-lm (MLX inference)
#   4. Clone Kokoro-FastAPI and install its Python dependencies
#   5. Download Kokoro TTS model weights
#   6. Install llama-swap binary (latest release)
#   7. Download GGUF models (optional, user-selectable)
#   8. Install llamactl system-wide
#   9. Optionally enable auto-start at login

set -euo pipefail

# ── Paths ─────────────────────────────────────────────────────────────────────

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AI_DIR="$(dirname "$SCRIPT_DIR")"
KOKORO_DIR="$AI_DIR/Kokoro-FastAPI"
MODELS_CHAT="$AI_DIR/models/chat"
MODELS_EMBED="$AI_DIR/models/embeddings"
LLAMACTL="$SCRIPT_DIR/llamactl"

# ── Colours ───────────────────────────────────────────────────────────────────

if [[ -t 1 ]]; then
  RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
  BLUE='\033[0;34m'; CYAN='\033[0;36m'; BOLD='\033[1m'; RESET='\033[0m'
else
  RED=''; GREEN=''; YELLOW=''; BLUE=''; CYAN=''; BOLD=''; RESET=''
fi

info()  { echo -e "${BLUE}→${RESET} $*"; }
ok()    { echo -e "${GREEN}✓${RESET} $*"; }
warn()  { echo -e "${YELLOW}⚠${RESET} $*"; }
err()   { echo -e "${RED}✗${RESET} $*" >&2; }
step()  { echo ""; echo -e "${BOLD}${CYAN}[$1/$TOTAL_STEPS]${RESET} ${BOLD}$2${RESET}"; }
skip()  { echo -e "  ${YELLOW}↷${RESET} $* — already done, skipping"; }
hr()    { echo -e "  ${RESET}────────────────────────────────────────"; }

ask_yn() {
  local prompt="$1" default="${2:-y}"
  local yn_hint; [[ "$default" == "y" ]] && yn_hint="Y/n" || yn_hint="y/N"
  local answer
  read -r -p "  $prompt [$yn_hint] " answer
  answer="${answer:-$default}"
  [[ "$answer" =~ ^[Yy] ]]
}

TOTAL_STEPS=9

# ── Header ────────────────────────────────────────────────────────────────────

echo ""
echo -e "  ${BOLD}macOS Local AI Stack — Installer${RESET}"
echo -e "  ${RESET}llama-swap · MLX · Kokoro TTS · llamactl"
hr
echo ""

# ── Step 0: System checks ─────────────────────────────────────────────────────

info "Checking system…"

OS=$(uname -s)
ARCH=$(uname -m)

if [[ "$OS" != "Darwin" ]]; then
  err "This stack requires macOS. Detected OS: $OS"
  exit 1
fi

if [[ "$ARCH" != "arm64" ]]; then
  err "Apple Silicon (arm64) required. Detected: $ARCH"
  exit 1
fi

MACOS_VER=$(sw_vers -productVersion)
ok "macOS $MACOS_VER on Apple Silicon"

# Check for Homebrew
if ! command -v brew &>/dev/null; then
  echo ""
  warn "Homebrew not found."
  if ask_yn "Install Homebrew now?"; then
    /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
    eval "$(/opt/homebrew/bin/brew shellenv)"
    ok "Homebrew installed"
  else
    err "Homebrew is required. Install it from https://brew.sh and re-run."
    exit 1
  fi
fi

BREW=$(command -v brew)
ok "Homebrew at $BREW"

# ── Show plan ─────────────────────────────────────────────────────────────────

echo ""
echo -e "  ${BOLD}This installer will set up:${RESET}"
echo "    • Homebrew packages: miniforge, ffmpeg, espeak-ng, uv"
echo "    • Conda env 'mlx-server': Python 3.11 + mlx-lm (Apple Silicon inference)"
echo "    • Kokoro-FastAPI TTS server (cloned from GitHub)"
echo "    • llama-swap reverse proxy (latest release)"
echo "    • GGUF models: gemma-3-12b (~7 GB) and nomic-embed (~270 MB) — optional"
echo "    • llamactl command (system-wide)"
echo ""
echo -e "  ${BOLD}MLX models${RESET} (qwen3.5-9b, qwen2.5-coder-14b, etc.) download on first use."
echo ""

if ! ask_yn "Continue with installation?"; then
  echo "Aborted."
  exit 0
fi

# ── Step 1: Homebrew packages ─────────────────────────────────────────────────

step 1 "Homebrew packages"

install_brew_pkg() {
  local pkg="$1"
  if brew list --formula "$pkg" &>/dev/null; then
    skip "$pkg"
  else
    info "Installing $pkg…"
    brew install "$pkg"
    ok "$pkg installed"
  fi
}

install_brew_pkg miniforge
install_brew_pkg ffmpeg
install_brew_pkg espeak-ng
install_brew_pkg uv

# ── Step 2: Conda mlx-server environment ─────────────────────────────────────

step 2 "Conda environment 'mlx-server' (Python 3.11 + mlx-lm)"

CONDA_BASE=$("$BREW" --prefix miniforge)/base
CONDA_BIN="$CONDA_BASE/bin/conda"
MLX_ENV="$CONDA_BASE/envs/mlx-server"
MLX_SERVER_BIN="$MLX_ENV/bin/mlx_lm.server"

if [[ -f "$MLX_SERVER_BIN" ]]; then
  MLX_VER=$("$MLX_SERVER_BIN" --version 2>/dev/null | head -1 || echo "installed")
  skip "mlx-server env ($MLX_VER)"
else
  if [[ -d "$MLX_ENV" ]]; then
    info "Environment exists but mlx_lm.server missing — installing mlx-lm…"
    "$CONDA_BIN" run -n mlx-server pip install mlx-lm --quiet
  else
    info "Creating conda environment 'mlx-server'…"
    "$CONDA_BIN" create -n mlx-server python=3.11 -y
    info "Installing mlx-lm…"
    "$CONDA_BIN" run -n mlx-server pip install mlx-lm --quiet
  fi
  ok "mlx-server environment ready  ($MLX_SERVER_BIN)"
fi

# ── Step 3: Kokoro-FastAPI ─────────────────────────────────────────────────────

step 3 "Kokoro-FastAPI TTS server"

if [[ -d "$KOKORO_DIR/.venv" ]]; then
  skip "Kokoro-FastAPI already set up at $KOKORO_DIR"
else
  if [[ ! -d "$KOKORO_DIR" ]]; then
    info "Cloning Kokoro-FastAPI…"
    git clone --depth 1 https://github.com/remsky/Kokoro-FastAPI.git "$KOKORO_DIR"
    ok "Cloned"
  else
    ok "Kokoro-FastAPI directory exists"
  fi

  info "Creating Python 3.10 venv and installing dependencies…"
  cd "$KOKORO_DIR"
  uv venv --python 3.10 .venv
  uv pip install -e . --quiet
  cd "$AI_DIR"
  ok "Kokoro-FastAPI dependencies installed"
fi

# ── Step 4: Kokoro model weights ──────────────────────────────────────────────

step 4 "Kokoro TTS model weights"

KOKORO_MODELS_DIR="$KOKORO_DIR/api/src/models/v1_0"
KOKORO_MODEL="$KOKORO_MODELS_DIR/kokoro-v1_0.pth"
KOKORO_CONFIG="$KOKORO_MODELS_DIR/config.json"

mkdir -p "$KOKORO_MODELS_DIR"

download_if_missing() {
  local url="$1" dest="$2" label="$3"
  if [[ -f "$dest" ]]; then
    skip "$label"
  else
    info "Downloading $label…"
    curl -fsSL --progress-bar -o "$dest" "$url"
    ok "$label downloaded"
  fi
}

download_if_missing \
  "https://github.com/remsky/Kokoro-FastAPI/releases/download/v0.2.2/kokoro-v1_0.pth" \
  "$KOKORO_MODEL" "kokoro-v1_0.pth (~82 MB)"

download_if_missing \
  "https://huggingface.co/hexgrad/Kokoro-82M/resolve/main/config.json" \
  "$KOKORO_CONFIG" "config.json"

# Voices
VOICES_DIR="$KOKORO_DIR/api/src/voices/v1_0"
if [[ ! -d "$VOICES_DIR" ]] || [[ -z "$(ls -A "$VOICES_DIR" 2>/dev/null)" ]]; then
  info "Downloading voice pack…"
  mkdir -p "$VOICES_DIR"
  # Download a set of voices from the HF repo
  VOICE_NAMES=(af_heart af_bella af_nova am_adam am_echo bf_alice bf_emma)
  for v in "${VOICE_NAMES[@]}"; do
    curl -fsSL --progress-bar \
      "https://huggingface.co/hexgrad/Kokoro-82M/resolve/main/voices/${v}.pt" \
      -o "$VOICES_DIR/${v}.pt" 2>/dev/null || \
    warn "Voice $v not found at expected URL — skipping"
  done
  ok "Voice pack downloaded"
else
  skip "Voice files"
fi

# ── Step 5: llama-swap binary ─────────────────────────────────────────────────

step 5 "llama-swap binary"

LLAMA_SWAP_BIN="/opt/homebrew/bin/llama-swap"
REPO="mostlygeek/llama-swap"

if [[ -f "$LLAMA_SWAP_BIN" ]]; then
  CURRENT_VER=$("$LLAMA_SWAP_BIN" -version 2>/dev/null | grep -oE 'version: [0-9]+' | awk '{print $2}' || echo "?")
  skip "llama-swap (version $CURRENT_VER)"
else
  info "Fetching latest release tag…"
  LATEST=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
    | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": "\(.*\)".*/\1/')
  info "Downloading llama-swap $LATEST…"
  URL="https://github.com/$REPO/releases/download/$LATEST/llama-swap-darwin-arm64"
  curl -fsSL --progress-bar "$URL" -o "$LLAMA_SWAP_BIN"
  chmod +x "$LLAMA_SWAP_BIN"
  ok "llama-swap $LATEST installed at $LLAMA_SWAP_BIN"
fi

# ── Step 6: GGUF models ───────────────────────────────────────────────────────

step 6 "GGUF models"

mkdir -p "$MODELS_CHAT" "$MODELS_EMBED"

echo ""
echo "  GGUF models are needed for gemma-3-12b-it and nomic-embed-text."
echo "  MLX alternatives (gemma-3-12b-it-mlx) are available and download on first use."
echo ""

GEMMA_DEST="$MODELS_CHAT/gemma-3-12b-it-Q4_K_M.gguf"
if [[ -f "$GEMMA_DEST" ]]; then
  skip "gemma-3-12b-it-Q4_K_M.gguf"
elif ask_yn "Download gemma-3-12b-it GGUF (~7 GB)?" "n"; then
  info "Downloading Gemma 3 12B Q4_K_M…"
  curl -fL --progress-bar \
    "https://huggingface.co/bartowski/google_gemma-3-12b-it-GGUF/resolve/main/google_gemma-3-12b-it-Q4_K_M.gguf" \
    -o "$GEMMA_DEST"
  ok "gemma-3-12b-it-Q4_K_M.gguf downloaded"
else
  warn "Skipped — you can download later with: bash scripts/download-models.sh"
fi

NOMIC_DEST="$MODELS_EMBED/nomic-embed-text-v1.5.Q8_0.gguf"
if [[ -f "$NOMIC_DEST" ]]; then
  skip "nomic-embed-text-v1.5.Q8_0.gguf"
elif ask_yn "Download nomic-embed-text v1.5 GGUF (~270 MB)?"; then
  info "Downloading nomic-embed-text v1.5…"
  curl -fL --progress-bar \
    "https://huggingface.co/nomic-ai/nomic-embed-text-v1.5-GGUF/resolve/main/nomic-embed-text-v1.5.Q8_0.gguf" \
    -o "$NOMIC_DEST"
  ok "nomic-embed-text-v1.5.Q8_0.gguf downloaded"
else
  warn "Skipped — you can download later with: bash scripts/download-models.sh"
fi

# ── Step 7: llamactl symlink ──────────────────────────────────────────────────

step 7 "llamactl command"

chmod +x "$LLAMACTL"

SYMLINK_TARGET="/opt/homebrew/bin/llamactl"
if [[ -L "$SYMLINK_TARGET" && "$(readlink "$SYMLINK_TARGET")" == "$LLAMACTL" ]]; then
  skip "llamactl symlink already in place"
else
  ln -sf "$LLAMACTL" "$SYMLINK_TARGET"
  ok "llamactl → $SYMLINK_TARGET"
fi

# ── Step 8: Verify llama-swap config ─────────────────────────────────────────

step 8 "Verifying configuration"

CONFIG="$AI_DIR/llama-swap.yaml"
if [[ ! -f "$CONFIG" ]]; then
  err "llama-swap.yaml not found at $CONFIG"
  err "Something is wrong — was the repo cloned correctly?"
  exit 1
fi

# Patch absolute paths in llama-swap.yaml to match this machine's username
CURRENT_USER=$(whoami)
if grep -q "/Users/andermurias/" "$CONFIG" 2>/dev/null; then
  if [[ "$CURRENT_USER" != "andermurias" ]]; then
    info "Patching username in llama-swap.yaml ($CURRENT_USER)…"
    sed -i '' "s|/Users/andermurias/|/Users/$CURRENT_USER/|g" "$CONFIG"
    sed -i '' "s|/Users/andermurias/|/Users/$CURRENT_USER/|g" "$LLAMACTL"
    ok "Paths updated for user $CURRENT_USER"
  else
    ok "llama-swap.yaml paths look correct"
  fi
else
  ok "llama-swap.yaml paths already customised"
fi

# ── Step 9: Install launchd service ──────────────────────────────────────────

step 9 "launchd service"

echo ""
echo "  'llamactl install' sets up a launchd service (no auto-start)."
echo "  'llamactl enable' also auto-starts llama-swap at every login."
echo ""

if ask_yn "Install llama-swap as a launchd service now?"; then
  "$LLAMACTL" install
  if ask_yn "Enable auto-start at login?"; then
    "$LLAMACTL" enable
  else
    "$LLAMACTL" start
  fi
else
  warn "Skipped — run 'llamactl start' to start manually when ready"
fi

# ── Done ──────────────────────────────────────────────────────────────────────

echo ""
hr
echo ""
echo -e "  ${GREEN}${BOLD}Installation complete!${RESET}"
echo ""
echo -e "  ${BOLD}Quick reference:${RESET}"
echo "    llamactl start          # Start the AI stack"
echo "    llamactl status         # Check status + loaded models"
echo "    llamactl logs -f        # Follow logs"
echo "    llamactl enable         # Auto-start at login"
echo ""
echo -e "  ${BOLD}API endpoint:${RESET} http://localhost:8080/v1"
echo -e "  ${BOLD}Web UI:${RESET}       http://localhost:8080"
echo ""
echo "  See README.md for full API usage, model list, and configuration."
echo ""
