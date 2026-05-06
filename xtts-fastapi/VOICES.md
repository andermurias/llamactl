# XTTS v2 — Voice Reference & Integration Guide

## API endpoint

```
POST http://localhost:8080/v1/audio/speech
     model: "xtts-tts"
```

All requests go through llama-swap on port 8080. The server starts on demand and shuts down after 180s of inactivity.

---

## Request format

```json
{
  "model": "xtts-tts",
  "input": "Texto que quieres sintetizar.",
  "voice": "Marcos Rudaski",
  "language": "es",
  "response_format": "wav",
  "speed": 1.0,
  "temperature": 0.65,
  "top_p": 0.85,
  "repetition_penalty": 10.0
}
```

Only `model`, `input`, and `voice` are required. All others are optional.

### `language` values

`es` (default), `en`, `fr`, `de`, `pt`, `it`, `pl`, `nl`, `cs`, `ar`, `zh-cn`, `ja`, `hu`, `ko`, `hi`

### `voice` — special value

`"custom"` — clones your voice from `/Users/andermurias/AI/reference.wav`. Pre-computed at startup.

---

## Tuning parameters

| Param | Default | Effect |
|---|---|---|
| `temperature` | `0.65` | Lower = more stable, less expressive. Range: 0.4–1.0 |
| `top_p` | `0.85` | Nucleus sampling threshold |
| `repetition_penalty` | `10.0` | Prevents stuttering/loops. Don't lower below 5.0 |
| `speed` | `1.0` | Speaking rate. `0.85` sounds more deliberate, `1.2` faster |

Good Spanish starting points:
- Natural, clear: `temperature=0.65, speed=1.0` (default)
- More expressive: `temperature=0.75, speed=0.95`
- Fast narration: `temperature=0.6, speed=1.15`

---

## Voice list (58 built-in speakers)

### Recommended for Spanish

| Voice | Gender | Notes |
|---|---|---|
| `Marcos Rudaski` | Male | Best male Spanish — natural, warm. Default. |
| `Uta Obando` | Female | Best female Spanish — clear, neutral accent. |
| `Alma María` | Female | Expressive, slightly warmer tone. |
| `Ana Florence` | Female | Soft, slower cadence — good for calm content. |
| `Luis Moray` | Male | Deeper male voice, formal tone. |
| `Eugenio Mataracı` | Male | Clear diction, medium pace. |
| `Ferran Simen` | Male | Lighter male voice, good for conversational text. |
| `Gilberto Mathias` | Male | Authoritative, mid-range. |

### All 58 voices (alphabetical)

```
Aaron Dreschner     Abrahan Mack        Adde Michal
Alexandra Hisakawa  Alison Dietlinde    Alma María
Ana Florence        Andrew Chipper      Annmarie Nele
Asya Anara          Badr Odhiambo       Baldur Sanjin
Barbora MacLean     Brenda Stern        Camilla Holmström
Chandra MacFarland  Claribel Dervla     Craig Gutsy
Daisy Studious      Damien Black        Damjan Chapman
Dionisio Schuyler   Eugenio Mataracı    Ferran Simen
Filip Traverse      Gilberto Mathias    Gitta Nikolina
Gracie Wise         Henriette Usha      Ige Behringer
Ilkin Urbano        Kazuhiko Atallah    Kumar Dahl
Lidiya Szekeres     Lilya Stainthorpe   Ludvig Milivoj
Luis Moray          Maja Ruoho          Marcos Rudaski
Narelle Moon        Nova Hogarth        Rosemary Okafor
Royston Min         Sofia Hellen        Suad Qasim
Szofi Granger       Tammie Ema          Tammy Grit
Tanja Adelina       Torcull Diarmuid    Uta Obando
Viktor Eka          Viktor Menelaos     Vjollca Johnnie
Wulf Carlevaro      Xavier Hayasaka     Zacharie Aimilios
Zofija Kendrick
```

Get this list live at any time:
```bash
curl http://localhost:8080/upstream/xtts-tts/voices
```

---

## Quick test (replacing Kokoro)

```bash
# Was:
curl http://localhost:8080/v1/audio/speech \
  -d '{"model":"kokoro-tts","input":"Hello!","voice":"af_heart"}' \
  --output out.wav

# Now, Spanish male voice:
curl http://localhost:8080/v1/audio/speech \
  -H "Content-Type: application/json" \
  -d '{"model":"xtts-tts","input":"Hola, ¿cómo estás?","voice":"Marcos Rudaski"}' \
  --output out.wav

# Spanish female voice:
curl http://localhost:8080/v1/audio/speech \
  -H "Content-Type: application/json" \
  -d '{"model":"xtts-tts","input":"Hola, ¿cómo estás?","voice":"Uta Obando"}' \
  --output out.wav

# Your cloned voice:
curl http://localhost:8080/v1/audio/speech \
  -H "Content-Type: application/json" \
  -d '{"model":"xtts-tts","input":"Hola, ¿cómo estás?","voice":"custom"}' \
  --output out.wav
```

In any app that uses the OpenAI TTS API, change:
- `model` → `"xtts-tts"` (was `"kokoro-tts"` or `"tts-1"`)
- `voice` → any name from the list above (was `"af_heart"` etc.)
- Add `"language": "es"` if the app doesn't already send it

---

## Differences vs Kokoro

| | Kokoro (`kokoro-tts`) | XTTS v2 (`xtts-tts`) |
|---|---|---|
| Best for | English | Spanish / multilingual |
| Voices | Named voice codes (`af_heart`, etc.) | Full names (`Marcos Rudaski`) |
| Voice cloning | No | Yes (`"voice": "custom"`) |
| Start time | ~3s | ~15s |
| TTL | 120s | 180s |
| Languages | Primarily English | 17 languages |
