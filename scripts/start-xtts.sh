#!/usr/bin/env bash
# Start XTTS v2 TTS server (Coqui XTTS v2, OpenAI-compatible /v1/audio/speech).
# Called by llama-swap as a managed subprocess; port passed as first argument.
#
# Usage: start-xtts.sh <port>
#   port defaults to 18087 if not provided.

set -euo pipefail

PORT="${1:-18087}"
XTTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../xtts-fastapi" && pwd)"

export REFERENCE_WAV="/Users/andermurias/AI/reference.wav"
export PYTORCH_ENABLE_MPS_FALLBACK=1
export COQUI_TOS_AGREED=1

cd "$XTTS_DIR"
exec /opt/homebrew/bin/uv run --no-sync uvicorn server:app \
  --host 127.0.0.1 \
  --port "$PORT"
