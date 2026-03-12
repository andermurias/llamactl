#!/usr/bin/env bash
# Start Kokoro-FastAPI TTS server with Apple Silicon MPS acceleration.
# Called by llama-swap as a managed subprocess; port passed as first argument.
#
# Usage: start-kokoro.sh <port>
#   port defaults to 18086 if not provided.

set -euo pipefail

PORT="${1:-18086}"
KOKORO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../Kokoro-FastAPI" && pwd)"

export USE_GPU=true
export USE_ONNX=false
export DEVICE_TYPE=mps
export PYTORCH_ENABLE_MPS_FALLBACK=1
export PYTHONPATH="$KOKORO_DIR:$KOKORO_DIR/api"
export MODEL_DIR="src/models"
export VOICES_DIR="src/voices/v1_0"
export WEB_PLAYER_PATH="$KOKORO_DIR/web"
export ESPEAK_DATA_PATH="/opt/homebrew/share/espeak-ng-data"

cd "$KOKORO_DIR"
exec /opt/homebrew/bin/uv run --no-sync uvicorn api.src.main:app \
  --host 127.0.0.1 \
  --port "$PORT"
