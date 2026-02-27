#!/usr/bin/env python3
import json
import os
import subprocess
import time
from pathlib import Path
from datetime import datetime, timezone

WORKSPACE = Path('/home/ubuntu/.openclaw/workspace')
COOKIES = WORKSPACE / 'secrets/youtube_cookies.txt'
RAW_JSON = WORKSPACE / 'traderchenge_videos_raw.json'
BASE = WORKSPACE / 'data/youtube/traderchenge'
AUDIO_DIR = BASE / 'audio'
TRANS_DIR = BASE / 'transcripts'
META_DIR = BASE / 'meta'
REPORT = WORKSPACE / 'reports/traderchenge_transcribe_v1.md'
INDEX = META_DIR / 'index.json'
WHISPER_SCRIPT = Path('/home/ubuntu/.npm-global/lib/node_modules/openclaw/skills/openai-whisper-api/scripts/transcribe.sh')

for d in [AUDIO_DIR, TRANS_DIR, META_DIR, REPORT.parent]:
    d.mkdir(parents=True, exist_ok=True)

with RAW_JSON.open('r', encoding='utf-8') as f:
    videos = json.load(f)

# 20-video sample: recent list, spaced roughly every 3 videos (~3 days from daily posting cadence)
sample = [videos[i] for i in range(0, min(len(videos), 60), 3)][:20]


def run_cmd(cmd, timeout=None):
    p = subprocess.run(cmd, capture_output=True, text=True, timeout=timeout)
    return p.returncode, p.stdout, p.stderr


def download_audio(video_id: str):
    url = f'https://www.youtube.com/watch?v={video_id}'
    outtmpl = str(AUDIO_DIR / '%(id)s.%(ext)s')
    cmd = [
        'yt-dlp',
        '--cookies', str(COOKIES),
        '--js-runtimes', 'node',
        '--remote-components', 'ejs:github',
        '-f', 'bestaudio',
        '-x', '--audio-format', 'm4a',
        '--no-playlist',
        '-o', outtmpl,
        url,
    ]
    for attempt in range(1, 4):
        code, out, err = run_cmd(cmd)
        candidate = AUDIO_DIR / f'{video_id}.m4a'
        if code == 0 and candidate.exists() and candidate.stat().st_size > 0:
            return True, str(candidate), attempt, out, err
        time.sleep(2)
    # discover any file with this id fallback
    files = list(AUDIO_DIR.glob(f'{video_id}.*'))
    if files:
        return True, str(files[0]), 3, '', 'download return code non-zero but file exists'
    return False, '', 3, out if 'out' in locals() else '', err if 'err' in locals() else ''


def transcribe(audio_path: str, video_id: str):
    transcript_path = TRANS_DIR / f'{video_id}.txt'

    # Try skill script first if present
    if WHISPER_SCRIPT.exists():
        cmd = ['bash', str(WHISPER_SCRIPT), audio_path, '--model', 'whisper-1', '--out', str(transcript_path)]
        for attempt in range(1, 4):
            code, out, err = run_cmd(cmd, timeout=1800)
            if code == 0 and transcript_path.exists() and transcript_path.stat().st_size > 0:
                return True, str(transcript_path), attempt, 'skill-script', out, err
            time.sleep(2)

    # Fallback to curl direct
    api_key = os.environ.get('OPENAI_API_KEY', '')
    if not api_key:
        return False, str(transcript_path), 0, 'curl', '', 'OPENAI_API_KEY missing for curl fallback'

    cmd = [
        'curl', '-sS', 'https://api.openai.com/v1/audio/transcriptions',
        '-H', f'Authorization: Bearer {api_key}',
        '-F', 'model=whisper-1',
        '-F', f'file=@{audio_path}',
    ]
    for attempt in range(1, 4):
        code, out, err = run_cmd(cmd, timeout=1800)
        if code == 0 and out.strip():
            text = out.strip()
            try:
                obj = json.loads(text)
                # Standard response has {"text":"..."}
                if isinstance(obj, dict) and 'text' in obj:
                    text = obj['text']
            except Exception:
                pass
            transcript_path.write_text(text, encoding='utf-8')
            if transcript_path.exists() and transcript_path.stat().st_size > 0:
                return True, str(transcript_path), attempt, 'curl', out[:4000], err[:4000]
        time.sleep(2)

    return False, str(transcript_path), 3, 'curl', out if 'out' in locals() else '', err if 'err' in locals() else ''


results = []
for idx, v in enumerate(sample, start=1):
    video_id = v.get('videoId')
    started = datetime.now(timezone.utc).isoformat()
    item = {
        'index': idx,
        'videoId': video_id,
        'title': v.get('title'),
        'published': v.get('published'),
        'length': v.get('length'),
        'views': v.get('views'),
        'url': f'https://www.youtube.com/watch?v={video_id}',
        'startedAt': started,
        'download': {},
        'transcription': {},
        'status': 'pending',
        'error': None,
    }

    t0 = time.time()
    ok_d, audio_path, d_attempts, d_out, d_err = download_audio(video_id)
    item['download'] = {
        'ok': ok_d,
        'attempts': d_attempts,
        'audioPath': audio_path,
        'stderr': (d_err or '')[-4000:],
    }

    if not ok_d:
        item['status'] = 'download_failed'
        item['error'] = 'audio download failed'
    else:
        ok_t, tr_path, t_attempts, method, t_out, t_err = transcribe(audio_path, video_id)
        item['transcription'] = {
            'ok': ok_t,
            'attempts': t_attempts,
            'method': method,
            'transcriptPath': tr_path,
            'stderr': (t_err or '')[-4000:],
        }
        if ok_t:
            item['status'] = 'ok'
        else:
            item['status'] = 'transcribe_failed'
            item['error'] = 'transcription failed'

    item['durationSec'] = round(time.time() - t0, 2)
    item['finishedAt'] = datetime.now(timezone.utc).isoformat()

    meta_path = META_DIR / f'{video_id}.json'
    meta_path.write_text(json.dumps(item, ensure_ascii=False, indent=2), encoding='utf-8')
    item['metaPath'] = str(meta_path)
    results.append(item)

success = sum(1 for r in results if r['status'] == 'ok')
dl_fail = sum(1 for r in results if r['status'] == 'download_failed')
tr_fail = sum(1 for r in results if r['status'] == 'transcribe_failed')

index = {
    'generatedAt': datetime.now(timezone.utc).isoformat(),
    'workspace': str(WORKSPACE),
    'channel': '@TraderChenge',
    'sampleStrategy': 'every 3 videos from recent list, capped at 20',
    'source': str(RAW_JSON),
    'count': len(results),
    'success': success,
    'downloadFailed': dl_fail,
    'transcribeFailed': tr_fail,
    'items': results,
}
INDEX.write_text(json.dumps(index, ensure_ascii=False, indent=2), encoding='utf-8')

readiness = 'READY' if success >= 15 else 'PARTIAL'
report = f"""# TraderChenge Audio Download + Transcription Report (v1)

- Generated (UTC): {index['generatedAt']}
- Source list: `{RAW_JSON}`
- Sample strategy: every ~3 days via every 3 videos from recent list
- Sample size: {len(results)}

## Outcomes

- ✅ Success (download + transcript): **{success}**
- ❌ Download failed: **{dl_fail}**
- ❌ Transcription failed: **{tr_fail}**

## Output locations

- Audio: `{AUDIO_DIR}`
- Transcripts: `{TRANS_DIR}`
- Per-video metadata: `{META_DIR}/<videoId>.json`
- Consolidated index: `{INDEX}`

## Next-step readiness (content stance extraction)

- Status: **{readiness}**
- Notes: {'Sufficient transcript coverage for stance extraction pipeline.' if success >= 15 else 'Coverage below preferred threshold; rerun failed items before stance extraction.'}

## Failed items (if any)

"""
for r in results:
    if r['status'] != 'ok':
        report += f"- `{r['videoId']}`: {r['status']} | error={r.get('error')}\n"

REPORT.write_text(report, encoding='utf-8')

print(json.dumps({
    'count': len(results),
    'success': success,
    'downloadFailed': dl_fail,
    'transcribeFailed': tr_fail,
    'index': str(INDEX),
    'report': str(REPORT),
}, ensure_ascii=False))
