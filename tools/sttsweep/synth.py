"""Speaks every sttsweep call with each Piper voice, then degrades it to radio quality (300 to
3400 Hz, Opus at 16 kbit/s, 16 kHz mono), as SRS carries a pilot's voice. Usage:
synth.py <calls.jsonl> <voice dir> <clip dir>"""
import json
import pathlib
import subprocess
import sys
import tempfile
import wave

from piper import PiperVoice

calls_path, voice_dir, clip_dir = map(pathlib.Path, sys.argv[1:4])
clip_dir.mkdir(parents=True, exist_ok=True)
calls = [json.loads(line) for line in calls_path.read_text().splitlines() if line.strip()]
for model in sorted(voice_dir.glob("*.onnx")):
    voice = PiperVoice.load(str(model))
    name = model.stem.split("-")[1]
    for call in calls:
        with tempfile.TemporaryDirectory() as tmp:
            raw = pathlib.Path(tmp) / "raw.wav"
            with wave.open(str(raw), "wb") as wf:
                if hasattr(voice, "synthesize_wav"):
                    voice.synthesize_wav(call["text"], wf)
                else:
                    voice.synthesize(call["text"], wf)
            opus = pathlib.Path(tmp) / "radio.opus"
            subprocess.run(["ffmpeg", "-loglevel", "error", "-y", "-i", str(raw), "-af",
                            "highpass=f=300,lowpass=f=3400", "-ar", "16000", "-ac", "1",
                            "-c:a", "libopus", "-b:a", "16k", str(opus)], check=True)
            subprocess.run(["ffmpeg", "-loglevel", "error", "-y", "-i", str(opus), "-ar", "16000",
                            "-ac", "1", "-c:a", "pcm_s16le",
                            str(clip_dir / f"{call['id']}_{name}.wav")], check=True)
    print(f"{name}: {len(calls)} clips", flush=True)
