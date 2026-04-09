"""
llamactl MCP Server
Exposes llamactl and llama-swap management as MCP tools for Copilot CLI.
"""

import httpx
from fastmcp import FastMCP

LLAMACTL_WEB = "http://localhost:3333"
LLAMASWAP = "http://localhost:8080"
TIMEOUT = 15.0

mcp = FastMCP(
    "llamactl",
    instructions=(
        "Manage the local AI stack on this Mac mini. "
        "Use these tools to control services, list models, view logs, and run inference."
    ),
)


def _get(url: str, params: dict | None = None) -> dict:
    with httpx.Client(timeout=TIMEOUT) as client:
        r = client.get(url, params=params)
        r.raise_for_status()
        return r.json()


def _post(url: str, body: dict) -> dict:
    with httpx.Client(timeout=TIMEOUT) as client:
        r = client.post(url, json=body)
        r.raise_for_status()
        return r.json()


# ── Service management ──────────────────────────────────────────────────────


@mcp.tool()
def get_status() -> dict:
    """
    Get the running status of all services in the local AI stack.
    Returns state, PID and uptime for llamaswap and comfyui.
    """
    return _get(f"{LLAMACTL_WEB}/api/status")


@mcp.tool()
def start_service(service: str) -> dict:
    """
    Start a service. service must be 'llamaswap' or 'comfyui'.
    """
    return _post(f"{LLAMACTL_WEB}/api/action", {"action": "start", "service": service})


@mcp.tool()
def stop_service(service: str) -> dict:
    """
    Stop a service. service must be 'llamaswap' or 'comfyui'.
    """
    return _post(f"{LLAMACTL_WEB}/api/action", {"action": "stop", "service": service})


@mcp.tool()
def restart_service(service: str) -> dict:
    """
    Restart a service. service must be 'llamaswap' or 'comfyui'.
    """
    return _post(f"{LLAMACTL_WEB}/api/action", {"action": "restart", "service": service})


# ── Model management ────────────────────────────────────────────────────────


@mcp.tool()
def list_models() -> dict:
    """
    List all configured AI models and which ones are currently loaded/running.
    Returns model IDs, backend types (MLX/GGUF/TTS/STT), context sizes, and active status.
    """
    return _get(f"{LLAMACTL_WEB}/api/models")


@mcp.tool()
def get_running_models() -> list:
    """
    List only the models currently loaded in memory (actively consuming RAM).
    Uses llama-swap's /running endpoint.
    """
    return _get(f"{LLAMASWAP}/running")


@mcp.tool()
def unload_models() -> dict:
    """
    Unload all currently loaded models from memory to free RAM.
    """
    return _post(f"{LLAMACTL_WEB}/api/models/unload", {})


# ── Logs ────────────────────────────────────────────────────────────────────


@mcp.tool()
def get_logs(service: str = "llamaswap", lines: int = 50) -> dict:
    """
    Get the last N lines of logs for a service.
    service: 'llamaswap' (default) or 'comfyui'
    lines: number of log lines to return (default 50, max 500)
    """
    lines = min(lines, 500)
    return _get(f"{LLAMACTL_WEB}/api/logs", {"service": service, "lines": lines})


# ── System info ─────────────────────────────────────────────────────────────


@mcp.tool()
def get_system_info() -> dict:
    """
    Get system resource information: total RAM, available RAM, 75% RAM budget, CPU cores.
    Useful to know how much memory is available before loading a large model.
    """
    return _get(f"{LLAMACTL_WEB}/api/system")


# ── Inference ───────────────────────────────────────────────────────────────


@mcp.tool()
def run_chat(prompt: str, model: str = "qwen2.5-3b", max_tokens: int = 512) -> str:
    """
    Send a chat completion request to the local AI stack.
    model: any model ID from list_models() — default is qwen2.5-3b (fast, 128K ctx)
    Returns the assistant's response text.
    """
    payload = {
        "model": model,
        "messages": [{"role": "user", "content": prompt}],
        "max_tokens": max_tokens,
        "stream": False,
    }
    with httpx.Client(timeout=120.0) as client:
        r = client.post(f"{LLAMASWAP}/v1/chat/completions", json=payload)
        r.raise_for_status()
        data = r.json()
        return data["choices"][0]["message"]["content"]


@mcp.tool()
def run_embedding(text: str) -> list[float]:
    """
    Generate a text embedding vector using the local nomic-embed-text-v1.5 model.
    Returns a 768-dimensional float vector suitable for semantic search / RAG.
    """
    payload = {"model": "nomic-embed-text-v1.5", "input": text}
    with httpx.Client(timeout=60.0) as client:
        r = client.post(f"{LLAMASWAP}/v1/embeddings", json=payload)
        r.raise_for_status()
        return r.json()["data"][0]["embedding"]


if __name__ == "__main__":
    mcp.run()
