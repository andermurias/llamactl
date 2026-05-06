#!/usr/bin/env bash
# start-voxtral.sh — llama-swap launcher for voxtral-tts.c HTTP wrapper
# Usage: start-voxtral.sh <port>

set -euo pipefail

PORT="${1:?Usage: start-voxtral.sh <port>}"
VOXTRAL_DIR="/Users/andermurias/AI/voxtral-tts.c"
PYTHON="/opt/homebrew/Caskroom/miniforge/base/envs/orpheus-tts/bin/python3"

# Verify binary and model exist
[[ -x "$VOXTRAL_DIR/voxtral_tts" ]] || { echo "ERROR: voxtral_tts binary not found"; exit 1; }
[[ -f "$VOXTRAL_DIR/voxtral-tts-model/consolidated.safetensors" ]] || {
    echo "ERROR: Voxtral model not downloaded yet (consolidated.safetensors missing)"
    exit 1
}

cd "$VOXTRAL_DIR"
exec "$PYTHON" server.py --host 127.0.0.1 --port "$PORT"
