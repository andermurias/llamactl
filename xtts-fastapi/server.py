import io
import os
from contextlib import asynccontextmanager
from typing import Literal, Optional

import numpy as np
import soundfile as sf
import torch
from fastapi import FastAPI, HTTPException
from fastapi.responses import Response
from pydub import AudioSegment
from pydub.utils import which as pydub_which
from pydantic import BaseModel

# Point pydub at Homebrew's ffmpeg
if pydub_which("ffmpeg") is None:
    AudioSegment.converter = "/opt/homebrew/bin/ffmpeg"
    AudioSegment.ffmpeg = "/opt/homebrew/bin/ffmpeg"
    AudioSegment.ffprobe = "/opt/homebrew/bin/ffprobe"

MODEL_DIR = os.path.expanduser(
    "~/Library/Application Support/tts/tts_models--multilingual--multi-dataset--xtts_v2"
)
REFERENCE_WAV = os.environ.get("REFERENCE_WAV", "/Users/andermurias/AI/reference.wav")
VOICES_DIR = os.path.join(os.path.dirname(__file__), "voices")

# XTTS v2 hard limit: 402 text tokens. ~250 chars of Spanish is a safe chunk size.
MAX_CHUNK_CHARS = 250

# Spanish-tuned synthesis defaults. Can be overridden per-request.
DEFAULTS = dict(
    temperature=0.72,  # warm, natural Spanish radio; 0.65 was too flat
    top_p=0.85,
    top_k=50,
    repetition_penalty=10.0,
    speed=1.0,
    language="es",
    gpt_cond_len=20,   # voice refs are ~18-20s; claiming 30 just pads silence
    gpt_cond_chunk_len=4,  # 5 chunks of 4s from 20s ref > 3 chunks of 6s → stable embedding
)

model = None
speakers: dict = {}       # built-in XTTS speakers from speakers_xtts.pth
custom_voices: dict = {}  # user voices loaded from voices/ directory
custom_latents: dict = {} # pre-computed from reference.wav


def _audio_to_wav_path(src: str) -> str:
    """Convert any audio file to a temp WAV path XTTS can consume."""
    if src.lower().endswith(".wav"):
        return src
    seg = AudioSegment.from_file(src)
    wav_buf = io.BytesIO()
    seg.export(wav_buf, format="wav")
    wav_buf.seek(0)
    # Write to a temp file since get_conditioning_latents needs a path
    import tempfile
    tmp = tempfile.NamedTemporaryFile(suffix=".wav", delete=False)
    tmp.write(wav_buf.read())
    tmp.flush()
    return tmp.name


def split_text(text: str, max_chars: int = MAX_CHUNK_CHARS) -> list[str]:
    """Split text into chunks at sentence boundaries, respecting max_chars."""
    import pysbd
    seg = pysbd.Segmenter(language="es", clean=True)
    sentences = seg.segment(text)

    chunks = []
    current = ""
    for sent in sentences:
        if len(current) + len(sent) + 1 <= max_chars:
            current = (current + " " + sent).strip()
        else:
            if current:
                chunks.append(current)
            # If a single sentence exceeds limit, hard-split at word boundaries
            if len(sent) > max_chars:
                words = sent.split()
                current = ""
                for word in words:
                    if len(current) + len(word) + 1 <= max_chars:
                        current = (current + " " + word).strip()
                    else:
                        if current:
                            chunks.append(current)
                        current = word
            else:
                current = sent
    if current:
        chunks.append(current)
    return chunks or [text]


def _infer_chunk(chunk: str, language: str, gpt_cond_latent, speaker_embedding, params: dict) -> np.ndarray:
    out = model.inference(
        text=chunk,
        language=language,
        gpt_cond_latent=gpt_cond_latent,
        speaker_embedding=speaker_embedding,
        **params,
    )
    return np.array(out["wav"])


def _load_voice_latents(audio_path: str) -> dict:
    wav_path = _audio_to_wav_path(audio_path)
    try:
        gpt_cond_latent, speaker_embedding = model.get_conditioning_latents(
            audio_path=[wav_path],
            gpt_cond_len=DEFAULTS["gpt_cond_len"],
            gpt_cond_chunk_len=DEFAULTS["gpt_cond_chunk_len"],
        )
    finally:
        if wav_path != audio_path:
            os.unlink(wav_path)
    return {"gpt_cond_latent": gpt_cond_latent, "speaker_embedding": speaker_embedding}


@asynccontextmanager
async def lifespan(app: FastAPI):
    global model, speakers, custom_voices, custom_latents

    from TTS.tts.configs.xtts_config import XttsConfig
    from TTS.tts.models.xtts import Xtts

    config = XttsConfig()
    config.load_json(os.path.join(MODEL_DIR, "config.json"))

    m = Xtts.init_from_config(config)
    m.load_checkpoint(config, checkpoint_dir=MODEL_DIR, eval=True)
    model = m

    speakers = torch.load(
        os.path.join(MODEL_DIR, "speakers_xtts.pth"), weights_only=False
    )

    # Load user voices from voices/ directory (MP3 or WAV, filename = voice name)
    if os.path.isdir(VOICES_DIR):
        for fname in sorted(os.listdir(VOICES_DIR)):
            if fname.lower().endswith((".mp3", ".wav", ".flac", ".ogg")):
                name = os.path.splitext(fname)[0]
                path = os.path.join(VOICES_DIR, fname)
                custom_voices[name] = _load_voice_latents(path)

    # Pre-compute conditioning for the reference.wav clone voice
    if os.path.isfile(REFERENCE_WAV):
        custom_latents.update(_load_voice_latents(REFERENCE_WAV))

    yield


app = FastAPI(lifespan=lifespan)


MIME = {
    "mp3": "audio/mpeg",
    "wav": "audio/wav",
    "flac": "audio/flac",
    "ogg": "audio/ogg",
}


class SpeechRequest(BaseModel):
    model: str = "xtts-tts"
    input: str
    voice: str = "Marcos Rudaski"
    language: str = "es"
    response_format: Literal["mp3", "wav", "flac", "ogg"] = "mp3"
    speed: float = 1.0
    temperature: Optional[float] = None
    top_p: Optional[float] = None
    top_k: Optional[int] = None
    repetition_penalty: Optional[float] = None


@app.get("/health")
def health():
    return {"status": "ok"}


@app.get("/voices")
def list_voices():
    return {
        "voices": sorted(speakers.keys()),
        "user_voices": sorted(custom_voices.keys()),
        "clone": bool(custom_latents),
        "recommended_es": [
            "Mateo Val", "Inés Olmo",
            "Marcos Rudaski", "Uta Obando", "Alma María", "Ana Florence",
            "Luis Moray", "Eugenio Mataracı", "Ferran Simen", "Gilberto Mathias",
        ],
    }


@app.post("/v1/audio/speech")
def synthesize(req: SpeechRequest):
    if model is None:
        raise HTTPException(status_code=503, detail="Model not loaded")

    if req.voice == "custom":
        if not custom_latents:
            raise HTTPException(status_code=400, detail="No reference.wav found for voice cloning")
        gpt_cond_latent = custom_latents["gpt_cond_latent"]
        speaker_embedding = custom_latents["speaker_embedding"]
    elif req.voice in custom_voices:
        gpt_cond_latent = custom_voices[req.voice]["gpt_cond_latent"]
        speaker_embedding = custom_voices[req.voice]["speaker_embedding"]
    elif req.voice in speakers:
        gpt_cond_latent = speakers[req.voice]["gpt_cond_latent"]
        speaker_embedding = speakers[req.voice]["speaker_embedding"]
    else:
        raise HTTPException(
            status_code=400,
            detail=f"Unknown voice '{req.voice}'. Call GET /voices for the full list.",
        )

    params = dict(
        temperature=req.temperature if req.temperature is not None else DEFAULTS["temperature"],
        top_p=req.top_p if req.top_p is not None else DEFAULTS["top_p"],
        top_k=req.top_k if req.top_k is not None else DEFAULTS["top_k"],
        repetition_penalty=req.repetition_penalty if req.repetition_penalty is not None else DEFAULTS["repetition_penalty"],
        speed=req.speed,
    )

    chunks = split_text(req.input)
    audio_parts = [_infer_chunk(c, req.language, gpt_cond_latent, speaker_embedding, params) for c in chunks]
    audio = np.concatenate(audio_parts) if len(audio_parts) > 1 else audio_parts[0]

    # Encode to WAV first, then convert if needed
    wav_buf = io.BytesIO()
    sf.write(wav_buf, audio, samplerate=24000, format="WAV")
    wav_buf.seek(0)

    fmt = req.response_format
    if fmt == "wav":
        return Response(content=wav_buf.read(), media_type=MIME["wav"])

    segment = AudioSegment.from_wav(wav_buf)
    out_buf = io.BytesIO()
    segment.export(out_buf, format=fmt)
    out_buf.seek(0)

    return Response(content=out_buf.read(), media_type=MIME[fmt])
