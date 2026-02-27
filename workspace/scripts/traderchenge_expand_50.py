#!/usr/bin/env python3
import json
import math
import os
import re
import subprocess
import time
from datetime import datetime, timezone
from pathlib import Path

WORKSPACE = Path('/home/ubuntu/.openclaw/workspace')
BASE = WORKSPACE / 'data/youtube/traderchenge'
AUDIO_DIR = BASE / 'audio'
TRANS_DIR = BASE / 'transcripts'
META_DIR = BASE / 'meta'
CHUNKS_DIR = BASE / 'chunks'
REPORT_PATH = WORKSPACE / 'reports/traderchenge_expand_50_report.md'
INDEX_PATH = META_DIR / 'index.json'
RAW_JSON = WORKSPACE / 'traderchenge_videos_raw.json'
COOKIES = WORKSPACE / 'secrets/youtube_cookies.txt'
WHISPER_SCRIPT = Path('/home/ubuntu/.npm-global/lib/node_modules/openclaw/skills/openai-whisper-api/scripts/transcribe.sh')
CHANNEL_URL = 'https://www.youtube.com/@TraderChenge/videos'
TARGET_NEW = 50
MAX_CHUNK_BYTES = 12 * 1024 * 1024
MAX_RETRIES = 2  # retries; attempts = retries + 1


def now_iso():
    return datetime.now(timezone.utc).isoformat()


def run(cmd, timeout=None):
    p = subprocess.run(cmd, capture_output=True, text=True, timeout=timeout)
    return p.returncode, p.stdout, p.stderr


def ensure_dirs():
    for d in [AUDIO_DIR, TRANS_DIR, META_DIR, CHUNKS_DIR, REPORT_PATH.parent]:
        d.mkdir(parents=True, exist_ok=True)


def load_existing_processed_ids():
    ids = set()
    if INDEX_PATH.exists():
        try:
            idx = json.loads(INDEX_PATH.read_text(encoding='utf-8'))
            for it in idx.get('items', []):
                vid = it.get('videoId')
                if vid:
                    ids.add(vid)
        except Exception:
            pass

    for p in TRANS_DIR.glob('*.txt'):
        if p.stat().st_size > 0:
            ids.add(p.stem)

    for p in META_DIR.glob('*.json'):
        if p.name != 'index.json':
            ids.add(p.stem)

    return ids


def load_raw_video_map():
    m = {}
    if RAW_JSON.exists():
        try:
            arr = json.loads(RAW_JSON.read_text(encoding='utf-8'))
            if isinstance(arr, list):
                for it in arr:
                    vid = it.get('videoId')
                    if vid:
                        m[vid] = it
        except Exception:
            pass
    return m


def discover_channel_entries(limit=300):
    cmd = [
        'yt-dlp',
        '--cookies', str(COOKIES),
        '--flat-playlist',
        '--dump-single-json',
        '--playlist-end', str(limit),
        CHANNEL_URL,
    ]
    code, out, err = run(cmd, timeout=600)
    if code != 0:
        raise RuntimeError(f'discover failed: {err[-500:]}')
    obj = json.loads(out)
    entries = []
    for e in obj.get('entries', []):
        vid = e.get('id') or e.get('url')
        if vid and re.match(r'^[A-Za-z0-9_-]{11}$', vid):
            entries.append({
                'videoId': vid,
                'title': e.get('title'),
                'url': f'https://www.youtube.com/watch?v={vid}',
                'duration': e.get('duration'),
                'channel': e.get('channel') or e.get('uploader'),
                'upload_date': e.get('upload_date'),
            })
    return entries


def download_audio(video_id):
    outtmpl = str(AUDIO_DIR / '%(id)s.%(ext)s')
    url = f'https://www.youtube.com/watch?v={video_id}'
    cmd = [
        'yt-dlp',
        '--cookies', str(COOKIES),
        '--js-runtimes', 'node',
        '--remote-components', 'ejs:github',
        '--extractor-args', 'youtube:player_client=web,android',
        '-f', 'bestaudio',
        '-x', '--audio-format', 'm4a',
        '--audio-quality', '5',
        '--no-playlist',
        '-o', outtmpl,
        url,
    ]

    for attempt in range(1, MAX_RETRIES + 2):
        code, out, err = run(cmd, timeout=1800)
        candidates = sorted(AUDIO_DIR.glob(f'{video_id}.*'))
        for c in candidates:
            if c.stat().st_size > 0:
                return True, str(c), attempt, err[-1200:]
        if attempt <= MAX_RETRIES:
            time.sleep(2)

    return False, '', MAX_RETRIES + 1, (err[-1200:] if 'err' in locals() else 'download failed')


def ffprobe_duration(audio_path):
    cmd = ['ffprobe', '-v', 'error', '-show_entries', 'format=duration', '-of', 'default=nw=1:nk=1', audio_path]
    code, out, err = run(cmd)
    if code != 0:
        return None
    try:
        return float(out.strip())
    except Exception:
        return None


def build_chunks(video_id, audio_path):
    src = Path(audio_path)
    if not src.exists() or src.stat().st_size <= MAX_CHUNK_BYTES:
        return [src]

    duration = ffprobe_duration(str(src))
    if not duration or duration <= 0:
        return [src]

    chunk_dir = CHUNKS_DIR / video_id
    chunk_dir.mkdir(parents=True, exist_ok=True)

    target_chunks = max(2, math.ceil(src.stat().st_size / MAX_CHUNK_BYTES))
    seg_seconds = max(30, math.ceil(duration / target_chunks))

    # idempotent reuse if chunks already exist
    existing = sorted(chunk_dir.glob('chunk_*.m4a'))
    if existing:
        return existing

    out_pattern = str(chunk_dir / 'chunk_%03d.m4a')
    cmd = [
        'ffmpeg', '-y', '-i', str(src),
        '-f', 'segment',
        '-segment_time', str(seg_seconds),
        '-c:a', 'aac', '-b:a', '96k',
        out_pattern,
    ]
    code, out, err = run(cmd, timeout=1800)
    chunks = sorted(chunk_dir.glob('chunk_*.m4a'))
    if code != 0 or not chunks:
        return [src]

    # if any chunk still too large, fallback to direct file (to avoid loop complexity)
    too_big = [c for c in chunks if c.stat().st_size > MAX_CHUNK_BYTES + 1024 * 1024]
    if too_big:
        return [src]
    return chunks


def transcribe_file(audio_path, out_txt):
    cmd = ['bash', str(WHISPER_SCRIPT), str(audio_path), '--model', 'whisper-1', '--out', str(out_txt)]
    for attempt in range(1, MAX_RETRIES + 2):
        code, out, err = run(cmd, timeout=3600)
        if code == 0 and out_txt.exists() and out_txt.stat().st_size > 0:
            return True, attempt, err[-1200:]
        if attempt <= MAX_RETRIES:
            time.sleep(2)
    return False, MAX_RETRIES + 1, (err[-1200:] if 'err' in locals() else 'transcribe failed')


def transcribe_with_chunking(video_id, audio_path):
    transcript_path = TRANS_DIR / f'{video_id}.txt'
    if transcript_path.exists() and transcript_path.stat().st_size > 0:
        return True, str(transcript_path), 0, 0, []

    chunks = build_chunks(video_id, audio_path)
    chunk_dir = CHUNKS_DIR / video_id
    chunk_dir.mkdir(parents=True, exist_ok=True)
    chunk_txts = []
    errors = []
    total_attempts = 0

    for i, chunk in enumerate(chunks, start=1):
        chunk_txt = chunk_dir / f'{chunk.stem}.txt'
        if chunk_txt.exists() and chunk_txt.stat().st_size > 0:
            chunk_txts.append(chunk_txt)
            continue
        ok, attempts, err = transcribe_file(str(chunk), chunk_txt)
        total_attempts += attempts
        if not ok:
            errors.append(f'chunk {i} failed: {err}')
        else:
            chunk_txts.append(chunk_txt)

    if len(chunk_txts) != len(chunks):
        return False, str(transcript_path), len(chunks), total_attempts, errors

    merged = []
    for t in chunk_txts:
        try:
            txt = t.read_text(encoding='utf-8').strip()
            if txt:
                merged.append(txt)
        except Exception as e:
            errors.append(f'read chunk transcript error {t.name}: {e}')

    if not merged:
        return False, str(transcript_path), len(chunks), total_attempts, errors + ['empty merged transcript']

    transcript_path.write_text('\n\n'.join(merged), encoding='utf-8')
    return True, str(transcript_path), len(chunks), total_attempts, errors


def read_existing_index_items():
    if INDEX_PATH.exists():
        try:
            d = json.loads(INDEX_PATH.read_text(encoding='utf-8'))
            if isinstance(d.get('items'), list):
                return d, d['items']
        except Exception:
            pass
    return {}, []


def main():
    ensure_dirs()
    processed = load_existing_processed_ids()
    raw_map = load_raw_video_map()

    entries = discover_channel_entries(limit=300)
    discovered_total = len(entries)
    new_entries = [e for e in entries if e['videoId'] not in processed]
    target_entries = new_entries[:TARGET_NEW]

    _, existing_items = read_existing_index_items()
    existing_by_id = {it.get('videoId'): it for it in existing_items if it.get('videoId')}

    results = []
    downloaded = 0
    transcribed = 0
    failed = 0
    blockers = []

    for idx, e in enumerate(target_entries, start=1):
        vid = e['videoId']
        seed = raw_map.get(vid, {})
        meta = {
            'videoId': vid,
            'title': seed.get('title') or e.get('title'),
            'url': e.get('url'),
            'published': seed.get('published') or e.get('upload_date'),
            'length': seed.get('length') or e.get('duration'),
            'views': seed.get('views'),
            'channel': e.get('channel') or '@TraderChenge',
            'startedAt': now_iso(),
            'status': 'pending',
            'download': {},
            'transcription': {},
            'errors': [],
        }

        # idempotent skip if transcript already exists
        pre_existing_transcript = TRANS_DIR / f'{vid}.txt'
        if pre_existing_transcript.exists() and pre_existing_transcript.stat().st_size > 0:
            meta['status'] = 'ok'
            meta['download'] = {'ok': True, 'skipped': True, 'reason': 'already_present'}
            meta['transcription'] = {'ok': True, 'skipped': True, 'transcriptPath': str(pre_existing_transcript)}
            transcribed += 1
        else:
            ok_d, audio_path, d_attempts, d_err = download_audio(vid)
            meta['download'] = {
                'ok': ok_d,
                'attempts': d_attempts,
                'audioPath': audio_path,
                'stderr': d_err,
            }
            if ok_d:
                downloaded += 1
                ok_t, tr_path, chunk_count, t_attempts, t_errors = transcribe_with_chunking(vid, audio_path)
                meta['transcription'] = {
                    'ok': ok_t,
                    'attempts': t_attempts,
                    'chunkCount': chunk_count,
                    'transcriptPath': tr_path,
                    'errors': t_errors,
                }
                if ok_t:
                    meta['status'] = 'ok'
                    transcribed += 1
                else:
                    meta['status'] = 'transcribe_failed'
                    meta['errors'].extend(t_errors)
                    failed += 1
            else:
                meta['status'] = 'download_failed'
                meta['errors'].append('audio download failed')
                failed += 1

        meta['finishedAt'] = now_iso()
        mp = META_DIR / f'{vid}.json'
        mp.write_text(json.dumps(meta, ensure_ascii=False, indent=2), encoding='utf-8')
        results.append(meta)

    # merge with existing index items by videoId
    for r in results:
        existing_by_id[r['videoId']] = r

    merged_items = list(existing_by_id.values())
    merged_items.sort(key=lambda x: x.get('startedAt', ''))

    total_success = sum(1 for it in merged_items if it.get('status') == 'ok')
    total_dl_fail = sum(1 for it in merged_items if it.get('status') == 'download_failed')
    total_tr_fail = sum(1 for it in merged_items if it.get('status') == 'transcribe_failed')

    idx_doc = {
        'generatedAt': now_iso(),
        'workspace': str(WORKSPACE),
        'channel': '@TraderChenge',
        'source': CHANNEL_URL,
        'count': len(merged_items),
        'success': total_success,
        'downloadFailed': total_dl_fail,
        'transcribeFailed': total_tr_fail,
        'items': merged_items,
        'expansion': {
            'targetNew': TARGET_NEW,
            'discoveredOnChannel': discovered_total,
            'eligibleNewFound': len(new_entries),
            'selectedForThisRun': len(target_entries),
            'runDownloaded': downloaded,
            'runTranscribed': transcribed,
            'runFailed': failed,
        }
    }
    INDEX_PATH.write_text(json.dumps(idx_doc, ensure_ascii=False, indent=2), encoding='utf-8')

    remaining_blockers = []
    if len(target_entries) < TARGET_NEW:
        remaining_blockers.append(f'Only {len(target_entries)} new IDs available after skipping processed set.')
    remaining_blockers.extend(blockers)

    report = []
    report.append('# TraderChenge Expansion Report (+50 videos)')
    report.append('')
    report.append(f'- Generated (UTC): {now_iso()}')
    report.append(f'- Channel: `@TraderChenge`')
    report.append(f'- Discovery source: `{CHANNEL_URL}`')
    report.append('')
    report.append('## Run summary')
    report.append(f'- discovered: **{discovered_total}**')
    report.append(f'- eligible new (after idempotent skip): **{len(new_entries)}**')
    report.append(f'- selected target this run: **{len(target_entries)}** (goal: {TARGET_NEW})')
    report.append(f'- downloaded: **{downloaded}**')
    report.append(f'- transcribed: **{transcribed}**')
    report.append(f'- failed: **{failed}**')
    report.append('')
    report.append('## Cumulative index summary')
    report.append(f'- total indexed: **{len(merged_items)}**')
    report.append(f'- total success: **{total_success}**')
    report.append(f'- total download failed: **{total_dl_fail}**')
    report.append(f'- total transcribe failed: **{total_tr_fail}**')
    report.append('')
    report.append('## Remaining blockers')
    if remaining_blockers:
        for b in remaining_blockers:
            report.append(f'- {b}')
    else:
        report.append('- None')
    report.append('')
    report.append('## Failed IDs this run')
    run_failed = [r for r in results if r.get('status') != 'ok']
    if run_failed:
        for r in run_failed:
            report.append(f"- `{r['videoId']}`: {r.get('status')} | errors: {'; '.join(r.get('errors', []))[:300]}")
    else:
        report.append('- None')

    REPORT_PATH.write_text('\n'.join(report) + '\n', encoding='utf-8')

    print(json.dumps({
        'discovered': discovered_total,
        'eligibleNew': len(new_entries),
        'selected': len(target_entries),
        'downloaded': downloaded,
        'transcribed': transcribed,
        'failed': failed,
        'index': str(INDEX_PATH),
        'report': str(REPORT_PATH),
    }, ensure_ascii=False))


if __name__ == '__main__':
    main()
