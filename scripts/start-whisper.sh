#!/usr/bin/env bash
# Start the Whisper STT server using mlx-whisper (Metal-accelerated on Apple Silicon).
# Called by llama-swap as a managed subprocess; port is passed as first argument.
#
# Access via llama-swap (recommended — process managed with idle TTL):
#   POST http://localhost:8080/upstream/whisper-stt/v1/audio/transcriptions
#
# OpenWebUI STT configuration:
#   Engine:  OpenAI
#   API URL: http://<mac-ip>:8080/upstream/whisper-stt/v1
#   API Key: local

set -euo pipefail

PORT="${1:-8882}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PYTHON="/opt/homebrew/Caskroom/miniforge/base/envs/mlx-server/bin/python"

exec "$PYTHON" "$SCRIPT_DIR/whisper_server.py" --port "$PORT"
