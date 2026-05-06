#!/usr/bin/env bash
# Start Orpheus TTS — Spanish/Italian model with OpenAI-compatible API.
# Called by llama-swap as a managed subprocess; port is passed as $1.
# Voices: javi (male warm), sergio (male professional), maria (female friendly)

PORT=${1:-18088}
SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
ORPHEUS_DIR="$SCRIPT_DIR/../Orpheus-FastAPI"
MODELS_DIR="/Users/andermurias/AI/models/tts"
ORPHEUS_MODEL="$MODELS_DIR/Orpheus-3b-Italian_Spanish-FT-Q8_0.gguf"
LLAMA_SERVER="/opt/homebrew/bin/llama-server"
PYTHON="/opt/homebrew/Caskroom/miniforge/base/envs/orpheus-tts/bin/python"
INTERNAL_PORT=18090

LLAMA_PID=""
UVICORN_PID=""

cleanup() {
    echo "[orpheus] Shutting down all children..."
    [ -n "$UVICORN_PID" ] && kill "$UVICORN_PID" 2>/dev/null || true
    [ -n "$LLAMA_PID" ]   && kill "$LLAMA_PID"   2>/dev/null || true
    wait 2>/dev/null || true
    exit 0
}
trap cleanup INT TERM EXIT

"$LLAMA_SERVER" \
    --model "$ORPHEUS_MODEL" \
    --host 127.0.0.1 \
    --port "$INTERNAL_PORT" \
    --n-gpu-layers 99 \
    --ctx-size 8192 \
    --log-disable \
    2>/dev/null &
LLAMA_PID=$!

echo "[orpheus] Waiting for llama-server (pid $LLAMA_PID) on port $INTERNAL_PORT..."
for i in $(seq 1 60); do
    if curl -sf "http://127.0.0.1:$INTERNAL_PORT/health" > /dev/null 2>&1; then
        echo "[orpheus] llama-server ready"
        break
    fi
    sleep 2
done

export ORPHEUS_API_URL="http://127.0.0.1:$INTERNAL_PORT/v1/completions"
export ORPHEUS_MODEL_NAME="Orpheus-3b-Italian_Spanish-FT-Q8_0.gguf"
export ORPHEUS_MAX_TOKENS=8192
export ORPHEUS_TEMPERATURE=0.6
export ORPHEUS_TOP_P=0.9
export ORPHEUS_SAMPLE_RATE=24000
export ORPHEUS_API_TIMEOUT=300

cd "$ORPHEUS_DIR"
"$PYTHON" -m uvicorn app:app \
    --host 127.0.0.1 \
    --port "$PORT" \
    --no-access-log &
UVICORN_PID=$!

echo "[orpheus] uvicorn started (pid $UVICORN_PID) on port $PORT"

# Keep script alive — trap fires on SIGTERM (llama-swap TTL) or if uvicorn crashes
wait $UVICORN_PID
