#!/usr/bin/env python3
"""Voice a scenario's narration with ElevenLabs for the page's audio player.

    python3 cockpit/scripts/narrate.py <id> [<id> ...]
    python3 cockpit/scripts/narrate.py --dry <id>      # count characters, call nothing

Reads cockpit/scenarios/<id>/narration.md (lines starting with # are notes,
not spoken) and speaks it in one request to
POST /v1/text-to-speech/{voice}/with-timestamps, so the intonation carries
from sentence to sentence. Writes cockpit/static/audio/<id>.mp3 and the
scenario's entry in cockpit/static/audio/index.json: its duration, the key
of the text it was voiced from, and when each sentence starts, which the
player shows as a caption.

The audio is committed, so attendees need no key. The key is read from the
ELEVENLABS_API_KEY environment variable, then ELEVENLABS_API_KEY in the
repo's .env.dev (git-ignored), then ~/.config/eda-patterns/elevenlabs-key.
Never another project's key, so voicing is billed where it is meant to be. Takes are cached in
~/.cache/eda-patterns-narration by voice, model, settings and text, so a
rerun pays only for scripts that changed.

The narrator is the one the SAM provider videos use, so Solace workshops
sound alike.
"""

import base64
import hashlib
import json
import os
import pathlib
import re
import sys
import time
import urllib.error
import urllib.request

ROOT = pathlib.Path(__file__).resolve().parents[1]
SCENARIOS = ROOT / "scenarios"
AUDIO = ROOT / "static" / "audio"
INDEX = AUDIO / "index.json"
ENV_DEV = ROOT.parent / ".env.dev"
CACHE = pathlib.Path.home() / ".cache" / "eda-patterns-narration"
KEY_FILES = [pathlib.Path.home() / ".config" / "eda-patterns" / "elevenlabs-key"]
API = "https://api.elevenlabs.io/v1/text-to-speech"

NARRATOR = {
    "voice": "cjVigY5qzO86Huf0OWal",
    "model": "eleven_multilingual_v2",
    "settings": {"stability": 0.35, "similarity_boost": 0.75, "style": 0.2,
                 "speed": 1.0, "use_speaker_boost": True},
    "seed": 20261116,
}


def script_text(sid: str) -> str:
    """The spoken text: paragraphs of narration.md, notes removed."""
    path = SCENARIOS / sid / "narration.md"
    if not path.exists():
        raise SystemExit(f"{sid}: no {path.relative_to(ROOT.parent)}")
    lines = [ln for ln in path.read_text().splitlines() if not ln.lstrip().startswith("#")]
    paras = re.split(r"\n\s*\n", "\n".join(lines).strip())
    return "\n\n".join(" ".join(p.split()) for p in paras if p.strip())


def take_key(text: str) -> str:
    blob = json.dumps([NARRATOR["voice"], NARRATOR["model"], NARRATOR["settings"],
                       NARRATOR["seed"], text], sort_keys=True)
    return hashlib.sha256(blob.encode()).hexdigest()[:24]


def sentences(text: str, alignment: dict) -> list:
    """Each sentence with the second it starts and ends, for captions."""
    starts = alignment["character_start_times_seconds"]
    ends = alignment["character_end_times_seconds"]
    out = []
    # A sentence ends at . ! or ? and any closing quote or bracket after it;
    # text after the last one is a sentence too. check_scenarios.py checks
    # the sentences cover the whole script.
    for m in re.finditer(r"[^.!?]+(?:[.!?]+[\"')\u201d\u2019]*|$)", text):
        if not m.group().strip():
            continue
        s, e = m.start(), m.end() - 1
        while s < e and text[s].isspace():
            s += 1
        out.append({"start": round(starts[s], 2), "end": round(ends[e], 2),
                    "text": text[s:m.end()].strip()})
    return out


def api_key() -> str:
    if os.environ.get("ELEVENLABS_API_KEY"):
        return os.environ["ELEVENLABS_API_KEY"].strip()
    if ENV_DEV.exists():
        for line in ENV_DEV.read_text().splitlines():
            m = re.match(r"\s*(?:export\s+)?ELEVENLABS_API_KEY\s*=\s*([^#\s]*)", line)
            if m and m.group(1).strip("'\""):
                return m.group(1).strip("'\"")
    for f in KEY_FILES:
        if f.exists() and f.read_text().strip():
            return f.read_text().strip()
    return ""


def voice(text: str, key: str) -> tuple:
    url = f"{API}/{NARRATOR['voice']}/with-timestamps?output_format=mp3_44100_128"
    body = json.dumps({"text": text, "model_id": NARRATOR["model"],
                       "voice_settings": NARRATOR["settings"], "seed": NARRATOR["seed"]}).encode()
    for attempt in range(3):
        req = urllib.request.Request(url, data=body, method="POST",
                                     headers={"xi-api-key": key, "content-type": "application/json"})
        try:
            with urllib.request.urlopen(req, timeout=120) as resp:
                j = json.load(resp)
            align = j["alignment"]
            if "".join(align["characters"]) != text:
                raise SystemExit("ElevenLabs timings do not match the text sent")
            return base64.b64decode(j["audio_base64"]), align
        except urllib.error.HTTPError as e:
            if (e.code == 429 or e.code >= 500) and attempt < 2:
                time.sleep(3 * (attempt + 1))
                continue
            raise SystemExit(f"ElevenLabs {e.code}: {e.read()[:300].decode(errors='replace')}")
        except urllib.error.URLError as e:
            if attempt < 2:
                time.sleep(2 * (attempt + 1))
                continue
            raise SystemExit(f"ElevenLabs did not answer ({e.reason})")
    raise SystemExit("unreachable")


def main(argv: list) -> int:
    dry = "--dry" in argv
    ids = [a for a in argv if not a.startswith("--")]
    if not ids:
        print(__doc__.strip().splitlines()[2])
        return 2
    texts = {sid: script_text(sid) for sid in ids}
    for sid, text in texts.items():
        k = take_key(text)
        cached = (CACHE / f"{k}.mp3").exists() and (CACHE / f"{k}.json").exists()
        print(f"{sid}: {len(text)} characters, {'cached' if cached else 'to voice'}")
    if dry:
        return 0

    key = None
    CACHE.mkdir(parents=True, exist_ok=True)
    AUDIO.mkdir(parents=True, exist_ok=True)
    index = json.loads(INDEX.read_text()) if INDEX.exists() else {}
    for sid, text in texts.items():
        k = take_key(text)
        mp3, js = CACHE / f"{k}.mp3", CACHE / f"{k}.json"
        if not (mp3.exists() and js.exists()):
            key = key or api_key()
            if not key:
                raise SystemExit(f"no ElevenLabs key: set ELEVENLABS_API_KEY in {ENV_DEV} or write {KEY_FILES[0]}")
            print(f"  voicing {sid}...", flush=True)
            audio, align = voice(text, key)
            mp3.write_bytes(audio)
            js.write_text(json.dumps(align))
        align = json.loads(js.read_text())
        (AUDIO / f"{sid}.mp3").write_bytes(mp3.read_bytes())
        caps = sentences(text, align)
        index[sid] = {
            "src": f"/static/audio/{sid}.mp3",
            "duration": round(align["character_end_times_seconds"][-1] + 0.3, 1),
            "key": k,
            "sentences": caps,
        }
        print(f"  {sid}: {index[sid]['duration']}s, {len(caps)} sentences -> {AUDIO / (sid + '.mp3')}")
    INDEX.write_text(json.dumps(dict(sorted(index.items())), indent=1) + "\n")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
