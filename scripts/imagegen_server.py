#!/usr/bin/env python3
"""
imagegen_server.py — ComfyUI-backed image generation wrapper for llama-swap.

Exposes OpenAI-compatible /v1/images/generations with:
  • Prompt enhancement via configurable LLM API
  • Bundled ComfyUI workflow presets (portrait, photo, anime, ipadapter, upscale)
  • SSE progress streaming
  • Single-request concurrency (429 on overlap)
  • Embedded ComfyUI lifecycle management

Usage:
    python imagegen_server.py ${PORT} \
        --workflow portrait \
        --enhance-url http://localhost:8080/v1/chat/completions \
        --enhance-model gemma-4-e4b-it-6bit-mlx

The script is designed to be managed by llama-swap (start on demand, TTL unload).
"""

import argparse
import asyncio
import base64
import json
import os
import re
import subprocess
import sys
import tempfile
import time
import uuid
from contextlib import asynccontextmanager
from pathlib import Path

import httpx
from fastapi import FastAPI, HTTPException, Request
from fastapi.responses import JSONResponse, StreamingResponse
from pydantic import BaseModel, Field

# ── Config ───────────────────────────────────────────────────────────────────
COMFYUI_DIR = os.environ.get("COMFYUI_DIR", "/Users/andermurias/AI/ComfyUI")
COMFYUI_PYTHON = os.environ.get("COMFYUI_PYTHON", f"{COMFYUI_DIR}/.venv/bin/python")
COMFYUI_PORT_BASE = 9000  # wrapper ComfyUI gets a port near the wrapper port
WORKFLOW_DIR = Path(__file__).parent / "workflows"
VOICE_DIR = Path(os.path.expanduser("~/AI/voices"))
VOICE_DIR.mkdir(parents=True, exist_ok=True)

# Global lock for single-request-at-a-time
generation_lock = asyncio.Lock()
comfyui_proc = None
comfyui_port = None


# ── Pydantic models ──────────────────────────────────────────────────────────
class GenerationRequest(BaseModel):
    prompt: str
    model: str = "cyber-portrait"
    n: int = Field(default=1, ge=1, le=1)
    size: str = "1024x1024"
    quality: str = "standard"
    style: str = "vivid"
    response_format: str = "b64_json"
    # Custom fields
    seed: int = Field(default=-1)
    steps: int = Field(default=25, ge=1, le=50)
    cfg_scale: float = Field(default=7.0, ge=1.0, le=15.0)
    reference_image: str = ""  # base64
    ipadapter_mode: str = "face"  # face | style | both
    skip_enhance: bool = False


class ImageData(BaseModel):
    b64_json: str


class GenerationResponse(BaseModel):
    created: int
    data: list[ImageData]


# ── ComfyUI lifecycle ────────────────────────────────────────────────────────

def _find_free_port(start: int) -> int:
    import socket
    for p in range(start, start + 100):
        with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
            if s.connect_ex(("127.0.0.1", p)) != 0:
                return p
    raise RuntimeError("No free port found")


def _start_comfyui(wrapper_port: int) -> tuple[subprocess.Popen, int]:
    """Launch ComfyUI as a subprocess, return (proc, port)."""
    port = _find_free_port(COMFYUI_PORT_BASE)
    log_file = Path(os.path.expanduser(f"~/AI/logs/imagegen-comfyui-{wrapper_port}.log"))
    log_file.parent.mkdir(parents=True, exist_ok=True)

    cmd = [
        COMFYUI_PYTHON,
        f"{COMFYUI_DIR}/main.py",
        "--listen", "127.0.0.1",
        "--port", str(port),
    ]
    log_fp = open(log_file, "a")
    proc = subprocess.Popen(
        cmd,
        stdout=log_fp,
        stderr=log_fp,
        cwd=COMFYUI_DIR,
        start_new_session=True,
    )
    return proc, port


def _wait_comfyui_ready(port: int, timeout: float = 60.0) -> bool:
    """Poll ComfyUI HTTP until it responds or timeout."""
    import urllib.request
    deadline = time.time() + timeout
    while time.time() < deadline:
        try:
            urllib.request.urlopen(f"http://127.0.0.1:{port}", timeout=2)
            return True
        except Exception:
            time.sleep(1.0)
    return False


def _stop_comfyui(proc: subprocess.Popen):
    """Gracefully terminate ComfyUI."""
    if proc is None:
        return
    try:
        proc.terminate()
        proc.wait(timeout=5)
    except Exception:
        proc.kill()
        proc.wait()


# ── Workflow management ──────────────────────────────────────────────────────

def load_workflow(preset_name: str) -> dict:
    """Load a bundled workflow JSON and return the node dict."""
    path = WORKFLOW_DIR / f"{preset_name}.json"
    if not path.exists():
        raise HTTPException(400, f"Unknown workflow preset: {preset_name}")
    with open(path) as f:
        return json.load(f)


def inject_workflow(workflow: dict, prompt: str, negative: str, seed: int, steps: int, cfg: float, width: int, height: int, checkpoint: str = "cyberrealisticPony_v170.safetensors") -> dict:
    """Inject user settings into a ComfyUI workflow template."""
    wf = json.loads(json.dumps(workflow))  # deep copy
    for node in wf.values():
        if not isinstance(node, dict):
            continue
        inputs = node.get("inputs", {})
        # Positive prompt
        if "text" in inputs and node.get("class_type", "").lower().endswith("clip_text_encode"):
            inputs["text"] = prompt
        # Negative prompt (second text encode or explicit negative node)
        if "text" in inputs and node.get("_meta", {}).get("title", "").lower() in ("negative prompt", "negative"):
            inputs["text"] = negative
        # KSampler
        if node.get("class_type", "").lower().endswith("ksampler"):
            inputs["seed"] = seed if seed >= 0 else int(time.time() * 1000) % (2**32)
            inputs["steps"] = steps
            inputs["cfg"] = cfg
        # Empty latent image (resolution)
        if node.get("class_type", "").lower().endswith("emptylatentimage"):
            inputs["width"] = width
            inputs["height"] = height
        # Checkpoint loader
        if node.get("class_type", "").lower().endswith("checkpointloadersimple"):
            inputs["ckpt_name"] = checkpoint
    return wf


# ── Prompt enhancement ───────────────────────────────────────────────────────

SYSTEM_PROMPT = (
    "You are a prompt engineer for Stable Diffusion image generation. "
    "Take the user's simple description and expand it into a detailed, "
    "high-quality prompt. Return ONLY a JSON object with these keys: "
    "positive (string), negative (string), quality_tags (string). "
    "Do not output markdown or explanations."
)


async def enhance_prompt(raw_prompt: str, enhance_url: str, enhance_model: str, api_key: str = "") -> tuple[str, str]:
    """Call LLM API to expand a simple prompt into detailed positive/negative prompts."""
    if not enhance_url:
        return raw_prompt, ""

    headers = {"Content-Type": "application/json"}
    if api_key:
        headers["Authorization"] = f"Bearer {api_key}"

    payload = {
        "model": enhance_model,
        "messages": [
            {"role": "system", "content": SYSTEM_PROMPT},
            {"role": "user", "content": raw_prompt},
        ],
        "temperature": 0.7,
        "max_tokens": 1024,
    }

    try:
        async with httpx.AsyncClient(timeout=30.0) as client:
            resp = await client.post(enhance_url, json=payload, headers=headers)
            resp.raise_for_status()
            data = resp.json()
            content = data["choices"][0]["message"]["content"]

            # Try JSON parse
            try:
                parsed = json.loads(content)
                positive = parsed.get("positive", raw_prompt)
                negative = parsed.get("negative", "")
                quality = parsed.get("quality_tags", "masterpiece, best quality, 8k uhd")
                if quality:
                    positive = f"{quality}, {positive}"
                return positive, negative
            except json.JSONDecodeError:
                # Fallback: use raw text as positive
                return content.strip() or raw_prompt, ""
    except Exception:
        # Fallback on any error
        return raw_prompt, ""


# ── ComfyUI job submission ────────────────────────────────────────────────────

async def submit_workflow(comfy_port: int, workflow: dict, client_id: str) -> str:
    """Submit a workflow to ComfyUI and return the prompt_id."""
    payload = {"prompt": workflow, "client_id": client_id}
    async with httpx.AsyncClient(timeout=30.0) as client:
        resp = await client.post(f"http://127.0.0.1:{comfy_port}/prompt", json=payload)
        resp.raise_for_status()
        return resp.json()["prompt_id"]


async def poll_history(comfy_port: int, prompt_id: str, timeout: float = 300.0) -> dict:
    """Poll ComfyUI /history until prompt_id completes or times out."""
    deadline = time.time() + timeout
    async with httpx.AsyncClient(timeout=10.0) as client:
        while time.time() < deadline:
            resp = await client.get(f"http://127.0.0.1:{comfy_port}/history")
            resp.raise_for_status()
            history = resp.json()
            if prompt_id in history:
                return history[prompt_id]
            await asyncio.sleep(0.5)
    raise HTTPException(504, "Image generation timed out")


async def get_image(comfy_port: int, filename: str, subfolder: str, folder_type: str) -> bytes:
    """Fetch generated image bytes from ComfyUI."""
    params = {"filename": filename, "subfolder": subfolder, "type": folder_type}
    async with httpx.AsyncClient(timeout=30.0) as client:
        resp = await client.get(f"http://127.0.0.1:{comfy_port}/view", params=params)
        resp.raise_for_status()
        return resp.content


# ── FastAPI app ───────────────────────────────────────────────────────────────

app = FastAPI(title="llamagen", version="1.0.0")
args = None


@app.on_event("startup")
async def startup():
    global comfyui_proc, comfyui_port
    wrapper_port = int(sys.argv[1]) if len(sys.argv) > 1 else 8081
    comfyui_proc, comfyui_port = _start_comfyui(wrapper_port)
    if not _wait_comfyui_ready(comfyui_port, timeout=60.0):
        _stop_comfyui(comfyui_proc)
        raise RuntimeError("ComfyUI failed to start within 60s")


@app.on_event("shutdown")
async def shutdown():
    global comfyui_proc
    _stop_comfyui(comfyui_proc)


@app.get("/health")
async def health():
    return {"status": "ok"}


@app.post("/v1/images/generations")
async def generate_images(req: GenerationRequest, raw_req: Request):
    # Single-request concurrency
    if generation_lock.locked():
        raise HTTPException(429, "Another image generation is in progress. Please wait.")

    async with generation_lock:
        return await _do_generate(req)


async def _do_generate(req: GenerationRequest) -> dict:
    global comfyui_port
    if comfyui_port is None:
        raise HTTPException(503, "ComfyUI not ready")

    # 1. Enhance prompt
    positive = req.prompt
    negative = ""
    if not req.skip_enhance and args.enhance_url:
        positive, negative = await enhance_prompt(req.prompt, args.enhance_url, args.enhance_model, args.enhance_api_key or "")

    # 2. Load and inject workflow
    preset = args.workflow
    workflow = load_workflow(preset)

    # Parse size
    w, h = 1024, 1024
    if req.size:
        m = re.match(r"(\d+)x(\d+)", req.size)
        if m:
            w, h = int(m.group(1)), int(m.group(2))

    # Default negatives per preset
    default_negatives = {
        "portrait": "bad anatomy, extra limbs, deformed, ugly, blurry, low quality",
        "photo": "cartoon, anime, painting, drawing, low quality, blurry",
        "anime": "photorealistic, 3d render, western cartoon, low quality, blurry",
        "ipadapter": "bad anatomy, extra limbs, deformed, ugly, blurry, low quality",
        "upscale": "artifacts, noise, jpeg artifacts, blurry, low quality",
    }
    if not negative:
        negative = default_negatives.get(preset, "low quality, blurry")

    workflow = inject_workflow(workflow, positive, negative, req.seed, req.steps, req.cfg_scale, w, h)

    # 3. Submit to ComfyUI
    client_id = str(uuid.uuid4())
    prompt_id = await submit_workflow(comfyui_port, workflow, client_id)

    # 4. Poll for completion
    history = await poll_history(comfyui_port, prompt_id, timeout=300.0)

    # 5. Extract output image
    outputs = history.get("outputs", {})
    for node_id, node_output in outputs.items():
        images = node_output.get("images", [])
        if images:
            img_info = images[0]
            img_bytes = await get_image(comfyui_port, img_info["filename"], img_info.get("subfolder", ""), img_info.get("type", "output"))
            b64 = base64.b64encode(img_bytes).decode("utf-8")
            return {
                "created": int(time.time()),
                "data": [{"b64_json": b64}],
            }

    raise HTTPException(500, "No image generated")


# ── Main ───────────────────────────────────────────────────────────────────────

def main():
    global args
    parser = argparse.ArgumentParser(description="llamagen — ComfyUI image generation wrapper")
    parser.add_argument("port", type=int, help="Port to bind FastAPI server")
    parser.add_argument("--workflow", default="portrait", choices=["portrait", "photo", "anime", "ipadapter", "upscale"])
    parser.add_argument("--enhance-url", default="http://localhost:8080/v1/chat/completions")
    parser.add_argument("--enhance-model", default="gemma-4-e4b-it-6bit-mlx")
    parser.add_argument("--enhance-api-key", default="")
    parser.add_argument("--checkpoint", default="cyberrealisticPony_v170.safetensors")
    args = parser.parse_args()

    import uvicorn
    uvicorn.run(app, host="127.0.0.1", port=args.port, log_level="warning")


if __name__ == "__main__":
    main()
