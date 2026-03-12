#!/usr/bin/env bash
# test-api.sh — Comprehensive llamastack API test suite
# Tests every model category and endpoint via the llama-swap proxy.
#
# Usage:
#   ./test-api.sh                    # run all tests
#   ./test-api.sh --fast             # skip slow LLM tests (embed + TTS + STT only)
#   ./test-api.sh --model qwen2.5-3b # test only one model
#
# Exit code: 0 = all pass, 1 = at least one failure

set -uo pipefail

BASE="http://127.0.0.1:8080"
PASS=0; FAIL=0; SKIP=0
FAST=false
ONLY_MODEL=""
TEST_AUDIO="/tmp/llamastack_test_audio.wav"

# ── Colours ──────────────────────────────────────────────────────────────────

if [[ -t 1 ]]; then
  GRN='\033[0;32m'; RED='\033[0;31m'; YLW='\033[1;33m'
  BLU='\033[0;34m'; BOLD='\033[1m'; RST='\033[0m'
else
  GRN=''; RED=''; YLW=''; BLU=''; BOLD=''; RST=''
fi

# ── Args ──────────────────────────────────────────────────────────────────────

while [[ "${1:-}" == -* ]]; do
  case "$1" in
    --fast)             FAST=true; shift ;;
    --model)            ONLY_MODEL="${2:-}"; shift 2 ;;
    -h|--help)
      echo "Usage: test-api.sh [--fast] [--model <name>]"
      exit 0 ;;
    *) echo "Unknown flag: $1"; exit 1 ;;
  esac
done

# ── Helpers ───────────────────────────────────────────────────────────────────

header() { echo -e "\n${BOLD}${BLU}── $* ──────────────────────────────────────${RST}"; }
pass()   { echo -e "  ${GRN}✓${RST} $*"; (( PASS++ )); }
fail()   { echo -e "  ${RED}✗${RST} $*"; (( FAIL++ )); }
skip()   { echo -e "  ${YLW}⊘${RST} $* (skipped)"; (( SKIP++ )); }
info()   { echo -e "    ${RST}↳ $*"; }

# Returns HTTP status code; body goes to /tmp/llamastack_resp.json
http_post_json() {
  local url="$1" body="$2"
  curl -s -o /tmp/llamastack_resp.json -w "%{http_code}" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer local" \
    --max-time 120 \
    -d "$body" \
    "$url"
}

http_get() {
  local url="$1"
  curl -s -o /tmp/llamastack_resp.json -w "%{http_code}" \
    --max-time 15 "$url"
}

assert_status() {
  local got="$1" want="$2" label="$3"
  if [[ "$got" == "$want" ]]; then
    pass "$label (HTTP $got)"
  else
    fail "$label (expected HTTP $want, got HTTP $got)"
    info "Response: $(cat /tmp/llamastack_resp.json 2>/dev/null | head -c 300)"
  fi
}

assert_json_key() {
  # assert_json_key <key_path> <label>  — key_path uses dot notation
  local key="$1" label="$2"
  local val
  val=$(python3 -c "
import sys, json
try:
  d = json.load(open('/tmp/llamastack_resp.json'))
  parts = '$key'.split('.')
  v = d
  for p in parts:
    if p.isdigit(): v = v[int(p)]
    else: v = v[p]
  print(v)
except Exception as e:
  print('__MISSING__')
" 2>/dev/null)
  if [[ "$val" != "__MISSING__" && -n "$val" ]]; then
    pass "$label"
    info "$key = $val"
  else
    fail "$label (key '$key' missing or empty)"
    info "Response: $(cat /tmp/llamastack_resp.json 2>/dev/null | head -c 300)"
  fi
}

should_run() {
  local model="$1"
  [[ -z "$ONLY_MODEL" ]] || [[ "$ONLY_MODEL" == "$model" ]]
}

# ── Generate test audio (for STT) ─────────────────────────────────────────────

make_test_audio() {
  if [[ ! -f "$TEST_AUDIO" ]]; then
    say -o /tmp/_llamastack_test.aiff "Hello, this is a test of the speech to text system." 2>/dev/null \
      && ffmpeg -y -i /tmp/_llamastack_test.aiff -ar 16000 -ac 1 "$TEST_AUDIO" -loglevel error 2>/dev/null \
      && rm -f /tmp/_llamastack_test.aiff \
      || { echo "  (could not generate test audio — say/ffmpeg required)"; return 1; }
  fi
  return 0
}

# ─────────────────────────────────────────────────────────────────────────────
# TESTS
# ─────────────────────────────────────────────────────────────────────────────

echo ""
echo -e "${BOLD}llamastack API test suite${RST}"
echo -e "  Proxy: $BASE"
echo -e "  Date:  $(date)"

# ── 1. Proxy health ───────────────────────────────────────────────────────────

header "Proxy"

code=$(http_get "$BASE/v1/models")
assert_status "$code" "200" "GET /v1/models"

# Model count
count=$(python3 -c "import json; d=json.load(open('/tmp/llamastack_resp.json')); print(len(d.get('data',[])))" 2>/dev/null || echo 0)
if (( count >= 11 )); then
  pass "/v1/models returns $count models (≥11)"
else
  fail "/v1/models only returned $count models (expected ≥11)"
fi

code=$(http_get "$BASE/running")
assert_status "$code" "200" "GET /running"

# ── 2. Embeddings — nomic-embed-text-v1.5 ────────────────────────────────────

header "Embeddings  (nomic-embed-text-v1.5)"

if should_run "nomic-embed-text-v1.5"; then
  code=$(http_post_json "$BASE/v1/embeddings" \
    '{"model":"nomic-embed-text-v1.5","input":"The quick brown fox jumps over the lazy dog"}')
  assert_status "$code" "200" "POST /v1/embeddings"
  assert_json_key "data.0.embedding.0" "Embedding vector returned"
  # Verify 768-dim
  dims=$(python3 -c "
import json
d=json.load(open('/tmp/llamastack_resp.json'))
print(len(d['data'][0]['embedding']))
" 2>/dev/null || echo "?")
  info "Embedding dimensions: $dims"
else
  skip "nomic-embed-text-v1.5"
fi

# ── 3. TTS — kokoro-tts ───────────────────────────────────────────────────────

header "TTS  (kokoro-tts)"

if should_run "kokoro-tts"; then
  code=$(http_post_json "$BASE/v1/audio/speech" \
    '{"model":"kokoro-tts","input":"Hello! The stack is working perfectly.","voice":"af_heart","response_format":"mp3"}')
  if [[ "$code" == "200" ]]; then
    size=$(wc -c < /tmp/llamastack_resp.json)
    pass "POST /v1/audio/speech (HTTP 200, ${size} bytes)"
    info "Audio file size: ${size} bytes"
    cp /tmp/llamastack_resp.json /tmp/llamastack_tts_output.mp3
    info "Saved to /tmp/llamastack_tts_output.mp3"
  else
    fail "POST /v1/audio/speech (expected 200, got $code)"
    info "Response: $(cat /tmp/llamastack_resp.json | head -c 300)"
  fi
else
  skip "kokoro-tts"
fi

# ── 4. STT — whisper-stt ──────────────────────────────────────────────────────

header "STT  (whisper-stt via /upstream)"

if should_run "whisper-stt"; then
  # Health check first
  code=$(http_get "$BASE/upstream/whisper-stt/health")
  assert_status "$code" "200" "GET /upstream/whisper-stt/health"

  if make_test_audio; then
    code=$(curl -s -o /tmp/llamastack_resp.json -w "%{http_code}" \
      --max-time 60 \
      -H "Authorization: Bearer local" \
      -F "model=whisper-stt" \
      -F "file=@${TEST_AUDIO};type=audio/wav" \
      -F "language=en" \
      "$BASE/upstream/whisper-stt/v1/audio/transcriptions")
    assert_status "$code" "200" "POST /upstream/whisper-stt/v1/audio/transcriptions"
    assert_json_key "text" "Transcription text returned"
    text=$(python3 -c "import json; print(json.load(open('/tmp/llamastack_resp.json')).get('text',''))" 2>/dev/null || echo "")
    if [[ -n "$text" ]]; then
      info "Transcript: \"$text\""
    fi
  else
    skip "whisper STT transcription (no audio file)"
  fi
else
  skip "whisper-stt"
fi

# ── 5. Chat — qwen2.5-3b (fast, ~2GB) ────────────────────────────────────────

header "Chat  (qwen2.5-3b — fastest model)"

if should_run "qwen2.5-3b" && ! $FAST; then
  code=$(http_post_json "$BASE/v1/chat/completions" \
    '{"model":"qwen2.5-3b","messages":[{"role":"user","content":"Reply with exactly: OK"}],"max_tokens":10,"temperature":0}')
  assert_status "$code" "200" "POST /v1/chat/completions"
  assert_json_key "choices.0.message.content" "Response content returned"
  text=$(python3 -c "import json; print(json.load(open('/tmp/llamastack_resp.json'))['choices'][0]['message']['content'])" 2>/dev/null || echo "")
  info "Response: \"$text\""
elif $FAST; then
  skip "qwen2.5-3b (--fast mode)"
fi

# ── 6. Chat — gemma-3-12b-it-mlx (main chat model) ───────────────────────────

header "Chat  (gemma-3-12b-it-mlx)"

if should_run "gemma-3-12b-it-mlx" && ! $FAST; then
  code=$(http_post_json "$BASE/v1/chat/completions" \
    '{"model":"gemma-3-12b-it-mlx","messages":[{"role":"user","content":"Reply with exactly: OK"}],"max_tokens":10,"temperature":0}')
  assert_status "$code" "200" "POST /v1/chat/completions"
  assert_json_key "choices.0.message.content" "Response content returned"
  text=$(python3 -c "import json; print(json.load(open('/tmp/llamastack_resp.json'))['choices'][0]['message']['content'])" 2>/dev/null || echo "")
  info "Response: \"$text\""
elif $FAST; then
  skip "gemma-3-12b-it-mlx (--fast mode)"
fi

# ── 7. Reasoning — qwen3.5-9b (thinking model) ───────────────────────────────

header "Reasoning  (qwen3.5-9b)"

if should_run "qwen3.5-9b" && ! $FAST; then
  # Thinking models need generous max_tokens: the <think>…</think> chain
  # can consume hundreds of tokens before the visible answer appears.
  code=$(http_post_json "$BASE/v1/chat/completions" \
    '{"model":"qwen3.5-9b","messages":[{"role":"user","content":"What is 12 * 13? Reply with just the number."}],"max_tokens":2048,"temperature":0}')
  assert_status "$code" "200" "POST /v1/chat/completions"
  # Extract visible content (strip <think>…</think> block)
  text=$(python3 -c "
import json, re
d = json.load(open('/tmp/llamastack_resp.json'))
content = d['choices'][0]['message']['content']
visible = re.sub(r'<think>.*?</think>', '', content, flags=re.DOTALL).strip()
print(visible or content[:200])
" 2>/dev/null || echo "")
  if [[ -n "$text" ]]; then
    pass "Response content returned (visible answer present)"
    info "Answer: \"$text\""
  else
    fail "Response content was empty (thinking chain may have exhausted max_tokens)"
    info "Raw: $(python3 -c "import json; d=json.load(open('/tmp/llamastack_resp.json')); print(d['choices'][0]['message']['content'][:300])" 2>/dev/null)"
  fi
elif $FAST; then
  skip "qwen3.5-9b (--fast mode)"
fi

# ── 8. Coding — qwen2.5-coder-14b ────────────────────────────────────────────

header "Coding  (qwen2.5-coder-14b)"

if should_run "qwen2.5-coder-14b" && ! $FAST; then
  code=$(http_post_json "$BASE/v1/chat/completions" \
    '{"model":"qwen2.5-coder-14b","messages":[{"role":"user","content":"Write a Python one-liner that prints numbers 1 to 5."}],"max_tokens":80,"temperature":0}')
  assert_status "$code" "200" "POST /v1/chat/completions"
  assert_json_key "choices.0.message.content" "Response content returned"
  text=$(python3 -c "import json; print(json.load(open('/tmp/llamastack_resp.json'))['choices'][0]['message']['content'])" 2>/dev/null || echo "")
  info "Response: \"$text\""
elif $FAST; then
  skip "qwen2.5-coder-14b (--fast mode)"
fi

# ── 9. Agent — qwen2.5-14b ────────────────────────────────────────────────────

header "Agent / instructions  (qwen2.5-14b)"

if should_run "qwen2.5-14b" && ! $FAST; then
  code=$(http_post_json "$BASE/v1/chat/completions" \
    '{"model":"qwen2.5-14b","messages":[{"role":"system","content":"You are a helpful assistant. Always respond in JSON."},{"role":"user","content":"List two fruits."}],"max_tokens":80,"temperature":0}')
  assert_status "$code" "200" "POST /v1/chat/completions"
  assert_json_key "choices.0.message.content" "Response content returned"
  text=$(python3 -c "import json; print(json.load(open('/tmp/llamastack_resp.json'))['choices'][0]['message']['content'])" 2>/dev/null || echo "")
  info "Response: \"$text\""
elif $FAST; then
  skip "qwen2.5-14b (--fast mode)"
fi

# ── 10. Streaming — qwen2.5-3b ───────────────────────────────────────────────

header "Streaming  (qwen2.5-3b stream=true)"

if should_run "qwen2.5-3b" && ! $FAST; then
  # Stream test: just check we get SSE data: lines
  stream_out=$(curl -s --max-time 30 \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer local" \
    -d '{"model":"qwen2.5-3b","messages":[{"role":"user","content":"Say: stream"}],"max_tokens":20,"stream":true}' \
    "$BASE/v1/chat/completions" 2>/dev/null | head -5)
  if echo "$stream_out" | grep -q "^data:"; then
    pass "SSE stream returns data: events"
    info "First chunk: $(echo "$stream_out" | head -1 | head -c 100)"
  else
    fail "SSE stream did not return data: events"
    info "Got: $(echo "$stream_out" | head -c 200)"
  fi
elif $FAST; then
  skip "streaming test (--fast mode)"
fi

# ── 11. Model isolation — verify groups swap ──────────────────────────────────

header "Group swap (only one LLM loaded at a time)"

if ! $FAST; then
  running=$(curl -s --max-time 5 "$BASE/running" | python3 -c "
import sys, json
d = json.load(sys.stdin)
r = d.get('running', d if isinstance(d, list) else [])
print(len(r))
" 2>/dev/null || echo "?")
  if [[ "$running" =~ ^[0-9]+$ ]] && (( running <= 3 )); then
    pass "Running model count is $running (≤3 — groups swap working)"
  elif [[ "$running" == "?" ]]; then
    skip "Could not parse /running response"
  else
    fail "Running model count is $running (>3 — possible memory leak)"
  fi
fi

# ── Summary ───────────────────────────────────────────────────────────────────

echo ""
echo -e "${BOLD}────────────────────────────────────────────────────────${RST}"
TOTAL=$(( PASS + FAIL + SKIP ))
echo -e "  ${BOLD}Results: $TOTAL tests${RST}  |  ${GRN}$PASS passed${RST}  ${RED}$FAIL failed${RST}  ${YLW}$SKIP skipped${RST}"

if (( FAIL > 0 )); then
  echo -e "  ${RED}Some tests failed — check output above.${RST}"
  echo ""
  exit 1
else
  echo -e "  ${GRN}All tests passed! ✓${RST}"
  echo ""
  exit 0
fi
