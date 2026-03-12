# macOS Local AI Stack

> OpenAI-compatible local inference for Apple Silicon — chat, coding, embeddings, TTS and agents, all behind a single endpoint. No cloud, no API keys, full GPU acceleration.

## What's included

| Model key | Model | Purpose | Size | Max tokens |
|-----------|-------|---------|------|-----------|
| `gemma-3-12b-it` | Gemma 3 12B Q4_K_M (GGUF) | Chat — best Spanish + reasoning | ~7 GB | unlimited (32K ctx) |
| `gemma-3-12b-it-mlx` | Gemma 3 12B 4-bit (MLX) | Chat — same model, MLX-native | ~7 GB | 8 192 |
| `qwen3.5-9b` | Qwen 3.5 9B 4-bit (MLX) | Reasoning · CoT (`/think` token) | ~6 GB | **32 768** |
| `qwen2.5-3b` | Qwen 2.5 3B 4-bit (MLX) | Fast · 128 K context | ~2 GB | 8 192 |
| `qwen2.5-coder-14b` | Qwen 2.5 Coder 14B 4-bit (MLX) | Code generation · fill-in-middle | ~8 GB | **16 384** |
| `qwen2.5-14b` | Qwen 2.5 14B 4-bit (MLX) | Agents · tool-calling · MCP | ~8 GB | **16 384** |
| `phi-4` | Phi-4 14B 4-bit (MLX) | STEM · reasoning · coding | ~8 GB | **16 384** |
| `qwen2.5-vl-7b` | Qwen2.5-VL 7B Q4_K_M (GGUF) | **Vision** — images + text → text | ~4.5 GB + mmproj | 8 192 |
| `nomic-embed-text-v1.5` | nomic-embed-text 1.5 Q8 (GGUF) | Embeddings · RAG · semantic search | ~270 MB | — |
| `whisper-stt` | Whisper large-v3-turbo (MLX) | **Speech-to-text** — transcription | ~1.6 GB | — |
| `kokoro-tts` | Kokoro v1.0 (MPS) | **Text-to-speech** — OpenAI-compatible | ~82 MB | — |

MLX and vision models download from HuggingFace on first use. GGUF models are downloaded by the installer.

> **Thinking models:** `qwen3.5-9b` uses 32 768 max tokens because the hidden `<think>…</think>` reasoning
> chain can consume thousands of tokens before the visible answer begins.

---

## Architecture

```
Client (OpenWebUI, curl, VS Code, …)
         │  OpenAI-compatible API
         ▼  http://localhost:8080
  ┌──────────────┐
  │  llama-swap  │  ← unified proxy; routes by model name;
  └──────┬───────┘    loads one model at a time (idle → unloaded)
         │
   ┌─────┴──────────────────┐
   ▼                        ▼                      ▼
llama-server          mlx_lm.server          Kokoro-FastAPI
(llama.cpp/GGUF)      (MLX native)           (TTS, MPS)
  gemma-3-12b-it        gemma-3-12b-it-mlx     kokoro-tts
  nomic-embed           qwen3.5-9b
                        qwen2.5-3b
                        qwen2.5-coder-14b
                        qwen2.5-14b
                        phi-4
```

Only **one model is active at a time** — all RAM and GPU go to the active model.

---

## Prerequisites

- **Mac with Apple Silicon** (M1 / M2 / M3 / M4)
- **macOS 14 Sonoma** or later
- **16 GB RAM** recommended (8 GB works for the smaller models)
- **Homebrew** — installer will guide you if missing

---

## Installation

```bash
git clone <repo-url> ~/AI
cd ~/AI
bash scripts/install.sh
```

The interactive installer will:
1. Check your system (macOS, Apple Silicon)
2. Install Homebrew packages (`miniforge`, `ffmpeg`, `espeak-ng`, `uv`)
3. Set up a conda environment with `mlx-lm` for Apple Silicon inference
4. Clone and configure Kokoro-FastAPI (TTS server)
5. Download Kokoro model weights
6. Install the `llama-swap` binary
7. Download GGUF models (you can pick which ones)
8. Install `llamactl` system-wide
9. Optionally enable auto-start at login

---

## Managing the service

```bash
llamactl start       # Start llama-swap (auto-installs launchd on first run)
llamactl stop        # Stop gracefully
llamactl restart     # Stop + start
llamactl status      # PID, uptime, loaded models, auto-start state

llamactl enable      # Auto-start llama-swap at every login
llamactl disable     # Disable auto-start

llamactl logs        # Last 100 lines of log
llamactl logs -f     # Follow logs in real time

llamactl upgrade     # Update llama-swap binary to latest release
llamactl version     # Show llama-swap version
llamactl --verbose <cmd>  # Extra detail on any command
```

---

## API usage

All requests go to `http://localhost:8080`. The proxy handles model routing — just change the `model` field.

### Chat

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "qwen3.5-9b",
    "messages": [{"role": "user", "content": "Explain quantum entanglement simply."}]
  }'
```

### Reasoning (chain-of-thought)

```bash
# Qwen3.5 supports /think and /no-think tokens to control CoT
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "qwen3.5-9b",
    "messages": [{"role": "user", "content": "/think Solve: if 3x + 7 = 22, what is x?"}]
  }'
```

### Embeddings

```bash
curl http://localhost:8080/v1/embeddings \
  -H "Content-Type: application/json" \
  -d '{"model": "nomic-embed-text-v1.5", "input": "Text to vectorise"}'
```

### Text-to-speech

```bash
curl http://localhost:8080/v1/audio/speech \
  -H "Content-Type: application/json" \
  -d '{
    "model": "kokoro-tts",
    "input": "Hello, this is a test of the local TTS system.",
    "voice": "af_heart"
  }' \
  --output speech.mp3
```

Available voices: `af_heart` `af_bella` `af_nova` `am_adam` `am_echo` `bf_alice` `bf_emma`  
and [many more](llama-swap.yaml) — American, British, French, Spanish, Italian, Hindi, Japanese, Chinese, Portuguese.

### List available models

```bash
curl http://localhost:8080/v1/models
```

### llama-swap UI

Open `http://localhost:8080` in your browser for a built-in chat UI and model manager.

---

## Connecting OpenWebUI

1. **Settings → Connections → OpenAI API**
2. **API Base URL**: `http://<your-mac-ip>:8080/v1`
3. **API Key**: any string (e.g. `local`) — not validated
4. Save — all models appear automatically

For **embeddings** in RAG pipelines, use the same base URL.

> **Tip:** Find your Mac's IP with `ipconfig getifaddr en0`

---

## OpenWebUI — Full multimodal setup

### 🗣️ Speech to Text (Whisper)

> Settings → Audio → Speech to Text

| Field | Value |
|-------|-------|
| Engine | OpenAI |
| API URL | `http://<mac-ip>:8080/upstream/whisper-stt/v1` |
| API Key | `local` |

Whisper loads on the first STT request and unloads after 5 min idle. Model: **Whisper large-v3-turbo** (~1.6 GB, downloads on first use).

### 🔊 Text to Speech (Kokoro)

> Settings → Audio → Text to Speech

| Field | Value |
|-------|-------|
| Engine | OpenAI |
| API URL | `http://<mac-ip>:8080/v1` |
| API Key | `local` |
| Model | `kokoro-tts` |
| Voice | `af_heart` (or any voice from the list in `llama-swap.yaml`) |

### 🖼️ Vision — Images in chat

Select **`qwen2.5-vl-7b`** as the model in OpenWebUI. An image attachment button appears automatically in the message input. You can send:
- Screenshots → describe / extract text
- Documents / PDFs (page screenshot) → read and analyse
- Charts / diagrams → explain
- Photos → answer questions

Model downloads ~4.5 GB + mmproj on first use. No special OpenWebUI configuration needed beyond selecting the model.

### 🎨 Image Generation (ComfyUI)

Image generation requires **ComfyUI** as a separate service. OpenWebUI has native ComfyUI support.

**1. Install ComfyUI**
```bash
git clone https://github.com/comfyanonymous/ComfyUI ~/AI/ComfyUI
cd ~/AI/ComfyUI
pip install torch torchvision torchaudio
pip install -r requirements.txt
```

**2. Download a model** (pick one based on quality/speed preference)

```bash
cd ~/AI/ComfyUI/models/checkpoints

# SDXL-Turbo — recommended (fast, 4 steps, good quality, ~7 GB)
curl -L "https://huggingface.co/stabilityai/sdxl-turbo/resolve/main/sd_xl_turbo_1.0_fp16.safetensors" \
     -o sd_xl_turbo_1.0_fp16.safetensors

# OR SD 1.5 — lighter (~2 GB, faster but older quality)
# curl -L "https://huggingface.co/stable-diffusion-v1-5/stable-diffusion-v1-5/resolve/main/v1-5-pruned-emaonly.safetensors" \
#      -o v1-5-pruned-emaonly.safetensors
```

**3. Start ComfyUI**
```bash
cd ~/AI/ComfyUI
python main.py --listen 0.0.0.0 --port 8188
```

**4. Configure OpenWebUI**

> Settings → Images

| Field | Value |
|-------|-------|
| Image Generation Engine | ComfyUI |
| ComfyUI Base URL | `http://<mac-ip>:8188` |

### 📄 Document reading (RAG)

OpenWebUI's built-in RAG pipeline handles document uploads automatically using the embeddings model already configured.

> Settings → Documents

| Field | Value |
|-------|-------|
| Embedding Model Engine | OpenAI |
| Embedding Model API Base URL | `http://<mac-ip>:8080/v1` |
| Embedding Model | `nomic-embed-text-v1.5` |

Supported formats: PDF, DOCX, TXT, Markdown, HTML, and more. For image-based PDFs or scanned documents, use **`qwen2.5-vl-7b`** and paste a screenshot of the page.

---

## Configuration

Edit `llama-swap.yaml` to add, remove or tune models. Changes apply live if you started with `llamactl start` (llama-swap watches the config file automatically).

### Adding an MLX model

```yaml
my-new-model:
  cmd: |
    /opt/homebrew/Caskroom/miniforge/base/envs/mlx-server/bin/mlx_lm.server
    --model mlx-community/YOUR-MODEL-4bit
    --host 127.0.0.1
    --port ${PORT}
    --log-level WARNING
  useModelName: "mlx-community/YOUR-MODEL-4bit"
  ttl: 600
```

### Key `llama-swap.yaml` options

| Option | What it does |
|--------|-------------|
| `ttl` | Seconds idle before the model is unloaded (0 = never) |
| `useModelName` | Rewrites the `model` field forwarded to the upstream server |
| `${PORT}` | Auto-assigned port; `proxy` defaults to `http://localhost:${PORT}` |
| `checkEndpoint` | Health-check path (default `/health`; use `/v1/models` for llama-server) |

---

## Directory structure

```
~/AI/
├── llama-swap.yaml          # Central config — all model routing lives here
├── scripts/
│   ├── llamactl             # Service manager (symlinked to /opt/homebrew/bin)
│   ├── install.sh           # Interactive first-time installer
│   ├── install-llama-swap.sh# Standalone llama-swap binary updater
│   ├── download-models.sh   # Download GGUF models
│   ├── start-kokoro.sh      # TTS server launcher (called by llama-swap)
│   ├── start-whisper.sh     # STT server launcher (called by llama-swap)
│   └── whisper_server.py    # OpenAI-compatible Whisper FastAPI server
├── models/
│   ├── chat/                # GGUF chat models
│   └── embeddings/          # GGUF embedding models
├── logs/
│   └── llama-swap.log       # All logs (proxy + upstream models)
└── Kokoro-FastAPI/          # TTS server — cloned by install.sh (git-ignored)
```

---

## Troubleshooting

**`llamactl start` fails**
```bash
llamactl logs          # Check the last error
llamactl --verbose start
```

**MLX model stuck on first load**
The first use downloads the model from HuggingFace (~2–8 GB). Check download progress:
```bash
llamactl logs -f
```

**Port 8080 already in use**
```bash
lsof -i :8080          # Find what's using it
```
Change the port in `llamactl` (`LISTEN=` variable) and in `llama-swap.yaml` if needed.

**Kokoro TTS not responding**
```bash
llamactl logs -f       # Watch for TTS startup errors
# MPS errors on first run are normal — PyTorch MPS fallback handles them
```

**Reset everything**
```bash
llamactl uninstall     # Remove launchd plist
bash scripts/install.sh  # Re-run installer
```
