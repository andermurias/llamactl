#!/usr/bin/env bash
# Download required models from HuggingFace
# Run this once before starting the servers

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AI_DIR="$(dirname "$SCRIPT_DIR")"

CHAT_MODEL_DIR="$AI_DIR/models/chat"
EMBED_MODEL_DIR="$AI_DIR/models/embeddings"

download_if_missing() {
  local url="$1"
  local dest="$2"
  local name
  name="$(basename "$dest")"

  if [[ -f "$dest" ]]; then
    echo "  ✅ $name already exists, skipping."
    return
  fi

  echo "  ⬇️  Downloading $name..."
  curl -L --progress-bar -o "$dest" "$url"
  echo "  ✅ $name downloaded."
}

echo "=== Downloading Models ==="
echo ""

echo "→ Chat model (Gemma 3 12B Q4_K_M, ~7.2GB)..."
download_if_missing \
  "https://huggingface.co/bartowski/google_gemma-3-12b-it-GGUF/resolve/main/google_gemma-3-12b-it-Q4_K_M.gguf" \
  "$CHAT_MODEL_DIR/gemma-3-12b-it-Q4_K_M.gguf"

echo ""
echo "→ Embedding model (nomic-embed-text v1.5 Q8_0, ~270MB)..."
download_if_missing \
  "https://huggingface.co/nomic-ai/nomic-embed-text-v1.5-GGUF/resolve/main/nomic-embed-text-v1.5.Q8_0.gguf" \
  "$EMBED_MODEL_DIR/nomic-embed-text-v1.5.Q8_0.gguf"

echo ""
echo "=== All models ready ==="
echo "Run ./start-all.sh to launch the servers."
