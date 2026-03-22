#!/bin/bash
# memory-watchdog.sh — unloads AI models when RAM pressure is critical
#
# Installed as a launchd agent that runs every 60 seconds.
# When free+inactive RAM falls below THRESHOLD_GB, it calls llama-swap's
# /running endpoint to unload all active models, freeing GPU/Metal memory.
#
# Thresholds (on 16 GB machine):
#   WARNING  < 3 GB free → log only
#   CRITICAL < 1.5 GB free → unload all models
#   EMERGENCY < 0.8 GB free → unload models + purge disk cache

set -euo pipefail

LLAMA_SWAP_URL="http://localhost:8080"
LOG_FILE="/Users/andermurias/AI/logs/memory-watchdog.log"
THRESHOLD_WARNING_GB=3.0
THRESHOLD_CRITICAL_GB=1.5
THRESHOLD_EMERGENCY_GB=0.8

log() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') [watchdog] $*" >> "$LOG_FILE"
}

# Get free + inactive pages in GB (available to allocate without swapping)
get_available_gb() {
    local page_size free_pages inactive_pages
    page_size=$(sysctl -n hw.pagesize 2>/dev/null || echo 16384)
    free_pages=$(vm_stat | awk '/Pages free/ {gsub(/\./,"",$3); print $3}')
    inactive_pages=$(vm_stat | awk '/Pages inactive/ {gsub(/\./,"",$3); print $3}')
    echo "scale=2; ($free_pages + $inactive_pages) * $page_size / 1073741824" | bc
}

# Check if llama-swap is running
if ! curl -sf --max-time 2 "$LLAMA_SWAP_URL/v1/models" > /dev/null 2>&1; then
    exit 0  # llama-swap not running, nothing to do
fi

available=$(get_available_gb)

# ── EMERGENCY: < 0.8 GB ──────────────────────────────────────────────────────
if (( $(echo "$available < $THRESHOLD_EMERGENCY_GB" | bc -l) )); then
    log "EMERGENCY: only ${available}GB available — unloading models + purging caches"
    # Unload all models via llama-swap API (kills all model subprocesses)
    curl -sf --max-time 5 -X DELETE "$LLAMA_SWAP_URL/running" > /dev/null 2>&1 || true
    # Purge macOS disk cache (non-destructive: only releases file-backed pages)
    /usr/bin/memory_pressure -S critical > /dev/null 2>&1 || true
    log "EMERGENCY: unload complete, available now: $(get_available_gb)GB"

# ── CRITICAL: < 1.5 GB ───────────────────────────────────────────────────────
elif (( $(echo "$available < $THRESHOLD_CRITICAL_GB" | bc -l) )); then
    log "CRITICAL: only ${available}GB available — unloading all models"
    curl -sf --max-time 5 -X DELETE "$LLAMA_SWAP_URL/running" > /dev/null 2>&1 || true
    log "CRITICAL: unload complete, available now: $(get_available_gb)GB"

# ── WARNING: < 3 GB ──────────────────────────────────────────────────────────
elif (( $(echo "$available < $THRESHOLD_WARNING_GB" | bc -l) )); then
    log "WARNING: ${available}GB available (below ${THRESHOLD_WARNING_GB}GB threshold)"
fi
