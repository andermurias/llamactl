#!/usr/bin/env python3
"""
Whisper STT server — OpenAI-compatible speech-to-text using mlx-whisper.

Endpoints:
  GET  /health                        — health check
  POST /v1/audio/transcriptions       — OpenAI-compatible transcription
  POST /audio/transcriptions          — same, without /v1 prefix

Access via llama-swap (process managed, idle TTL unloads):
  POST http://localhost:8080/upstream/whisper-stt/v1/audio/transcriptions

OpenWebUI STT configuration:
  Engine:  OpenAI
  API URL: http://<mac-ip>:8080/upstream/whisper-stt/v1
  API Key: local   (not validated)

Model: mlx-community/whisper-large-v3-turbo (~1.6 GB, downloads on first use)
"""

import argparse
import os
import tempfile

import uvicorn
from fastapi import FastAPI, File, Form, UploadFile
from fastapi.responses import JSONResponse, PlainTextResponse

# Default model — override with --model flag
MODEL_REPO = "mlx-community/whisper-large-v3-turbo"

app = FastAPI(
    title="Whisper MLX",
    description="OpenAI-compatible speech-to-text using mlx-whisper on Apple Silicon",
)


@app.get("/health")
async def health():
    return {"status": "healthy", "model": MODEL_REPO}


async def _transcribe(
    file: UploadFile,
    language: str | None,
    temperature: float,
    response_format: str,
    prompt: str | None,
) -> JSONResponse | PlainTextResponse:
    import mlx_whisper

    suffix = os.path.splitext(file.filename or "audio.wav")[1] or ".wav"

    with tempfile.NamedTemporaryFile(suffix=suffix, delete=False) as tmp:
        tmp.write(await file.read())
        tmp_path = tmp.name

    try:
        kwargs: dict = {
            "path_or_hf_repo": MODEL_REPO,
            "temperature": temperature,
            "verbose": False,
        }
        if language:
            kwargs["language"] = language
        if prompt:
            kwargs["initial_prompt"] = prompt

        result = mlx_whisper.transcribe(tmp_path, **kwargs)
        text = result.get("text", "").strip()

        if response_format == "text":
            return PlainTextResponse(text)
        elif response_format == "srt":
            return PlainTextResponse(_to_srt(result.get("segments", [])))
        elif response_format == "verbose_json":
            return JSONResponse({
                "text": text,
                "language": result.get("language"),
                "duration": result.get("duration"),
                "segments": result.get("segments", []),
            })
        else:  # json (default)
            return JSONResponse({"text": text})
    finally:
        os.unlink(tmp_path)


def _to_srt(segments: list) -> str:
    lines = []
    for i, seg in enumerate(segments, 1):
        start = _fmt_time(seg.get("start", 0))
        end = _fmt_time(seg.get("end", 0))
        lines.append(f"{i}\n{start} --> {end}\n{seg.get('text', '').strip()}\n")
    return "\n".join(lines)


def _fmt_time(seconds: float) -> str:
    h = int(seconds // 3600)
    m = int((seconds % 3600) // 60)
    s = seconds % 60
    return f"{h:02d}:{m:02d}:{s:06.3f}".replace(".", ",")


@app.post("/v1/audio/transcriptions")
async def transcribe_v1(
    file: UploadFile = File(...),
    model: str = Form(default="whisper-stt"),
    language: str = Form(default=None),
    response_format: str = Form(default="json"),
    temperature: float = Form(default=0.0),
    prompt: str = Form(default=None),
):
    return await _transcribe(file, language, temperature, response_format, prompt)


# Without /v1 prefix — catches requests from some OpenWebUI configurations
@app.post("/audio/transcriptions")
async def transcribe(
    file: UploadFile = File(...),
    model: str = Form(default="whisper-stt"),
    language: str = Form(default=None),
    response_format: str = Form(default="json"),
    temperature: float = Form(default=0.0),
    prompt: str = Form(default=None),
):
    return await _transcribe(file, language, temperature, response_format, prompt)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Whisper MLX transcription server")
    parser.add_argument("--host", default="127.0.0.1", help="Bind host")
    parser.add_argument("--port", type=int, default=8882, help="Bind port")
    parser.add_argument(
        "--model",
        default=MODEL_REPO,
        help="mlx-community Whisper model repo (default: whisper-large-v3-turbo)",
    )
    args = parser.parse_args()
    MODEL_REPO = args.model

    uvicorn.run(app, host=args.host, port=args.port, log_level="warning")
