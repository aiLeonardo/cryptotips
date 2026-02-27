#!/usr/bin/env python3
import json
import math
import os
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
INDEX_PATH = META_DIR / 'index.json'
SOURCE_REPORT = WORKSPACE / 'reports/traderchenge_expand_50_report.md'
RETRY_REPORT = WORKSPACE / 'reports/traderchenge_expand_50_retry_report.md'
COOKIES = WORKSPACE / 'secrets/youtube_cookies.txt'
WHISPER_SCRIPT = Path('/home/ubuntu/.npm-global/lib/node_modules/openclaw/skills/openai-whisper-api/scripts/transcribe.sh')
MAX_CHUNK_BYTES = 12 * 1024 * 1024
BATCH_SIZE = 10
HAS_OPENAI_KEY = bool(os.environ.get('OPENAI_API_KEY'))


def now_iso():
    return datetime.now(timezone.utc).isoformat()


def run(cmd, timeout=None):
    p = subprocess.run(cmd, capture_output=True, text=True, timeout=timeout)
    return p.returncode, p.stdout, p.stderr


def load_index():
    if not INDEX_PATH.exists():
        return {'items': []}
    return json.loads(INDEX_PATH.read_text(encoding='utf-8'))


def save_index(idx):
    idx['generatedAt'] = now_iso()
    idx['count'] = len(idx.get('items', []))
    idx['success'] = sum(1 for i in idx['items'] if i.get('status') == 'ok')
    idx['downloadFailed'] = sum(1 for i in idx['items'] if i.get('status') == 'download_failed')
    idx['transcribeFailed'] = sum(1 for i in idx['items'] if i.get('status') == 'transcribe_failed')
    INDEX_PATH.write_text(json.dumps(idx, ensure_ascii=False, indent=2), encoding='utf-8')


def collect_failed_ids(idx):
    ids = []
    # Prefer the original +50 failed list from source report so we always target the same scope.
    if SOURCE_REPORT.exists():
        for line in SOURCE_REPORT.read_text(encoding='utf-8').splitlines():
            line = line.strip()
            if line.startswith('- `') and 'download_failed' in line:
                ids.append(line.split('`')[1])

    # Also include any currently marked download_failed in index (safety net).
    for it in idx.get('items', []):
        if it.get('status') == 'download_failed' and it.get('videoId'):
            ids.append(it['videoId'])
    # de-dup keep order
    seen = set()
    out = []
    for v in ids:
        if v not in seen:
            seen.add(v)
            out.append(v)
    return out


def find_audio_file(video_id):
    files = sorted(AUDIO_DIR.glob(f'{video_id}.*'))
    for f in files:
        if f.is_file() and f.stat().st_size > 0:
            return f
    return None


def download_with_profiles(video_id):
    outtmpl = str(AUDIO_DIR / '%(id)s.%(ext)s')
    url = f'https://www.youtube.com/watch?v={video_id}'
    profiles = [
        ('bestaudio_best', ['-f', 'bestaudio/best']),
        ('140_251_bestaudio_best', ['-f', '140/251/bestaudio/best']),
        ('m3u8_fallback', ['-f', 'bestaudio[protocol*=m3u8]/best[protocol*=m3u8]/bestaudio/best', '--extractor-args', 'youtube:player_client=ios,web'])
    ]

    last_err = ''
    for profile_name, fmt_args in profiles:
        cmd = [
            'yt-dlp',
            '--cookies', str(COOKIES),
            '--js-runtimes', 'node',
            '--remote-components', 'ejs:github',
            '--socket-timeout', '20',
            '--retries', '1',
            '--fragment-retries', '1',
            '--no-playlist',
            '-o', outtmpl,
        ] + fmt_args + [url]

        code, out, err = run(cmd, timeout=120)
        candidate = find_audio_file(video_id)
        if code == 0 and candidate:
            return True, str(candidate), profile_name, err[-1500:]
        if candidate:
            return True, str(candidate), profile_name, (err[-1500:] + ' | note: non-zero exit with file present')
        last_err = (err or out or '')[-2000:]
        time.sleep(1)

    return False, '', 'none', last_err


def ffprobe_duration(path):
    code, out, err = run(['ffprobe', '-v', 'error', '-show_entries', 'format=duration', '-of', 'default=nw=1:nk=1', str(path)])
    if code != 0:
        return None
    try:
        return float(out.strip())
    except Exception:
        return None


def build_chunks_12mb(video_id, audio_path):
    src = Path(audio_path)
    if src.stat().st_size <= MAX_CHUNK_BYTES:
        return [src], False

    dur = ffprobe_duration(src)
    if not dur or dur <= 0:
        return [src], False

    chunk_dir = CHUNKS_DIR / video_id
    chunk_dir.mkdir(parents=True, exist_ok=True)

    # enforce with iterative segmentation until all chunks <= 12MB
    target = max(2, math.ceil(src.stat().st_size / MAX_CHUNK_BYTES))
    for attempt in range(6):
        # clean only m4a chunk files (keep txt transcripts)
        for p in chunk_dir.glob('chunk_*.m4a'):
            p.unlink(missing_ok=True)
        seg_sec = max(20, math.ceil(dur / target))
        out_pattern = str(chunk_dir / 'chunk_%03d.m4a')
        cmd = [
            'ffmpeg', '-y', '-i', str(src),
            '-f', 'segment',
            '-segment_time', str(seg_sec),
            '-c:a', 'aac', '-b:a', '96k',
            out_pattern,
        ]
        code, out, err = run(cmd, timeout=3600)
        chunks = sorted(chunk_dir.glob('chunk_*.m4a'))
        if code != 0 or not chunks:
            return [src], False
        too_big = [c for c in chunks if c.stat().st_size > MAX_CHUNK_BYTES]
        if not too_big:
            return chunks, True
        target *= 2

    return [src], False


def transcribe_file(audio_path, out_txt):
    if not HAS_OPENAI_KEY:
        return False, 'Missing OPENAI_API_KEY'
    cmd = ['bash', str(WHISPER_SCRIPT), str(audio_path), '--model', 'whisper-1', '--out', str(out_txt)]
    code, out, err = run(cmd, timeout=5400)
    if code == 0 and out_txt.exists() and out_txt.stat().st_size > 0:
        return True, (err[-1500:] if err else '')
    return False, (err or out or '')[-2000:]


def transcribe_with_chunking(video_id, audio_path):
    final_txt = TRANS_DIR / f'{video_id}.txt'
    if final_txt.exists() and final_txt.stat().st_size > 0:
        return True, str(final_txt), 0, []
    if not HAS_OPENAI_KEY:
        return False, str(final_txt), 0, ['Missing OPENAI_API_KEY']

    chunks, chunked = build_chunks_12mb(video_id, audio_path)
    chunk_dir = CHUNKS_DIR / video_id
    chunk_dir.mkdir(parents=True, exist_ok=True)

    txt_parts = []
    errs = []
    for i, c in enumerate(chunks):
        part_txt = chunk_dir / f'{c.stem}.txt' if chunked else (chunk_dir / 'chunk_000.txt')
        if part_txt.exists() and part_txt.stat().st_size > 0:
            txt_parts.append(part_txt)
            continue
        ok, msg = transcribe_file(c, part_txt)
        if not ok:
            errs.append(f'chunk_{i:03d}: {msg}')
        else:
            txt_parts.append(part_txt)

    if len(txt_parts) != len(chunks):
        return False, str(final_txt), len(chunks), errs

    merged = []
    for p in txt_parts:
        t = p.read_text(encoding='utf-8').strip()
        if t:
            merged.append(t)

    if not merged:
        return False, str(final_txt), len(chunks), errs + ['merged transcript empty']

    final_txt.write_text('\n\n'.join(merged), encoding='utf-8')
    return True, str(final_txt), len(chunks), errs


def write_meta(meta):
    mp = META_DIR / f"{meta['videoId']}.json"
    mp.write_text(json.dumps(meta, ensure_ascii=False, indent=2), encoding='utf-8')


def load_meta(video_id):
    mp = META_DIR / f'{video_id}.json'
    if mp.exists():
        try:
            return json.loads(mp.read_text(encoding='utf-8'))
        except Exception:
            pass
    return {
        'videoId': video_id,
        'url': f'https://www.youtube.com/watch?v={video_id}',
        'title': None,
        'published': None,
        'length': None,
        'views': None,
        'channel': '@TraderChenge',
    }


def append_report(lines):
    RETRY_REPORT.write_text('\n'.join(lines) + '\n', encoding='utf-8')


def main():
    for d in [AUDIO_DIR, TRANS_DIR, META_DIR, CHUNKS_DIR, RETRY_REPORT.parent]:
        d.mkdir(parents=True, exist_ok=True)

    idx = load_index()
    failed_ids = collect_failed_ids(idx)
    items = idx.get('items', [])
    by_id = {it.get('videoId'): it for it in items if it.get('videoId')}

    report_lines = [
        '# TraderChenge Expansion Retry Report (+50 recovery)',
        '',
        f'- Started (UTC): {now_iso()}',
        f'- Target failed IDs: **{len(failed_ids)}**',
        f'- Batch size: **{BATCH_SIZE}**',
        f'- OPENAI_API_KEY present: **{"yes" if HAS_OPENAI_KEY else "no"}**',
        '',
        '## Batch progress',
    ]

    cumulative_ok = 0
    cumulative_dl_fail = 0
    cumulative_tr_fail = 0

    for bstart in range(0, len(failed_ids), BATCH_SIZE):
        batch = failed_ids[bstart:bstart + BATCH_SIZE]
        bno = (bstart // BATCH_SIZE) + 1
        bok = bdf = btf = 0
        batch_notes = []

        for vid in batch:
            meta = load_meta(vid)
            meta['retry'] = meta.get('retry', {})
            meta['retry']['lastRetryAt'] = now_iso()
            meta['retry']['run'] = 'expand_50_recovery'

            transcript = TRANS_DIR / f'{vid}.txt'
            if transcript.exists() and transcript.stat().st_size > 0:
                meta['status'] = 'ok'
                meta['transcription'] = {
                    'ok': True,
                    'skipped': True,
                    'transcriptPath': str(transcript),
                    'reason': 'transcript_already_exists',
                }
                meta['download'] = meta.get('download', {'ok': True, 'skipped': True})
                bok += 1
                cumulative_ok += 1
                batch_notes.append(f'- `{vid}`: ok (transcript existed)')
                write_meta(meta)
                by_id[vid] = meta
                continue

            existing_audio = find_audio_file(vid)
            if existing_audio:
                ok_d, audio_path, profile, derr = True, str(existing_audio), 'existing_file', ''
            else:
                ok_d, audio_path, profile, derr = download_with_profiles(vid)
            meta['download'] = {
                'ok': ok_d,
                'audioPath': audio_path,
                'profileUsed': profile,
                'stderr': derr,
                'retriedAt': now_iso(),
            }

            if not ok_d:
                meta['status'] = 'download_failed'
                meta['errors'] = list(set((meta.get('errors') or []) + ['audio download failed on retry']))
                bdf += 1
                cumulative_dl_fail += 1
                batch_notes.append(f'- `{vid}`: download_failed ({profile})')
                meta['finishedAt'] = now_iso()
                write_meta(meta)
                by_id[vid] = meta
                continue

            ok_t, tr_path, chunk_count, terrs = transcribe_with_chunking(vid, audio_path)
            meta['transcription'] = {
                'ok': ok_t,
                'transcriptPath': tr_path,
                'chunkCount': chunk_count,
                'chunkRuleBytes': MAX_CHUNK_BYTES,
                'errors': terrs,
                'retriedAt': now_iso(),
            }
            if ok_t:
                meta['status'] = 'ok'
                bok += 1
                cumulative_ok += 1
                batch_notes.append(f'- `{vid}`: ok (profile={profile}, chunks={chunk_count})')
            else:
                meta['status'] = 'transcribe_failed'
                meta['errors'] = list(set((meta.get('errors') or []) + terrs + ['transcription failed on retry']))
                btf += 1
                cumulative_tr_fail += 1
                batch_notes.append(f'- `{vid}`: transcribe_failed (profile={profile}, chunks={chunk_count})')

            meta['finishedAt'] = now_iso()
            write_meta(meta)
            by_id[vid] = meta

        # checkpoint index after each batch
        merged_items = list(by_id.values())
        merged_items.sort(key=lambda x: x.get('startedAt', ''))
        idx['items'] = merged_items
        idx['recoveryRetry'] = {
            'updatedAt': now_iso(),
            'sourceFailedIds': len(failed_ids),
            'processedSoFar': min(bstart + BATCH_SIZE, len(failed_ids)),
            'batchesDone': bno,
        }
        save_index(idx)

        report_lines.append(f'### Batch {bno} ({bstart + 1}-{bstart + len(batch)})')
        report_lines.append(f'- Batch result: ok={bok}, download_failed={bdf}, transcribe_failed={btf}')
        report_lines.append(f'- Cumulative retry result: ok={cumulative_ok}, download_failed={cumulative_dl_fail}, transcribe_failed={cumulative_tr_fail}')
        report_lines.extend(batch_notes)
        report_lines.append('')
        append_report(report_lines)

    # Final snapshot based on current index statuses for the original failed IDs
    idx = load_index()
    final_by_id = {it.get('videoId'): it for it in idx.get('items', []) if it.get('videoId')}
    final_ok = final_df = final_tf = 0
    blockers = []
    for vid in failed_ids:
        st = (final_by_id.get(vid) or {}).get('status')
        if st == 'ok':
            final_ok += 1
        elif st == 'download_failed':
            final_df += 1
            blockers.append(f'{vid}: download_failed')
        elif st == 'transcribe_failed':
            final_tf += 1
            blockers.append(f'{vid}: transcribe_failed')
        else:
            blockers.append(f'{vid}: unknown_status')

    report_lines.append('## Final summary')
    report_lines.append(f'- Finished (UTC): {now_iso()}')
    report_lines.append(f'- Retry target IDs: **{len(failed_ids)}**')
    report_lines.append(f'- Recovered to ok: **{final_ok}**')
    report_lines.append(f'- Still download_failed: **{final_df}**')
    report_lines.append(f'- Still transcribe_failed: **{final_tf}**')
    report_lines.append('')
    report_lines.append('## Remaining blockers')
    if blockers:
        for b in blockers:
            report_lines.append(f'- {b}')
    else:
        report_lines.append('- None')
    append_report(report_lines)

    print(json.dumps({
        'targetFailedIds': len(failed_ids),
        'recoveredOk': final_ok,
        'remainingDownloadFailed': final_df,
        'remainingTranscribeFailed': final_tf,
        'index': str(INDEX_PATH),
        'report': str(RETRY_REPORT),
    }, ensure_ascii=False))


if __name__ == '__main__':
    main()
