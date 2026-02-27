#!/usr/bin/env python3
import json, os, subprocess
from pathlib import Path
from datetime import datetime, timezone

WORKSPACE = Path('/home/ubuntu/.openclaw/workspace')
BASE = WORKSPACE / 'data/youtube/traderchenge'
AUDIO_DIR = BASE / 'audio'
META_DIR = BASE / 'meta'
TRANS_DIR = BASE / 'transcripts'
INDEX_PATH = META_DIR / 'index.json'
SRC_REPORT = WORKSPACE / 'reports/traderchenge_expand_50_report.md'
RETRY_REPORT = WORKSPACE / 'reports/traderchenge_expand_50_retry_report.md'
COOKIES = WORKSPACE / 'secrets/youtube_cookies.txt'
HAS_KEY = bool(os.environ.get('OPENAI_API_KEY'))
BATCH=10
CHUNK_RULE=12*1024*1024


def now(): return datetime.now(timezone.utc).isoformat()

def run(cmd):
    p=subprocess.run(cmd,capture_output=True,text=True)
    return p.returncode,p.stdout,p.stderr

def find_audio(vid):
    fs=sorted(AUDIO_DIR.glob(f'{vid}.*'))
    for f in fs:
        if f.exists() and f.stat().st_size>0:return f
    return None

def download(vid):
    url=f'https://www.youtube.com/watch?v={vid}'
    outtmpl=str(AUDIO_DIR/'%(id)s.%(ext)s')
    profiles=[
      ('bestaudio_best',['-f','bestaudio/best']),
      ('140_251_bestaudio_best',['-f','140/251/bestaudio/best']),
      ('m3u8_fallback',['-f','bestaudio[protocol*=m3u8]/best[protocol*=m3u8]/bestaudio/best','--extractor-args','youtube:player_client=ios,web'])
    ]
    for name,args in profiles:
      cmd=['timeout','20s','yt-dlp','--cookies',str(COOKIES),'--js-runtimes','node','--remote-components','ejs:github','--no-playlist','-o',outtmpl]+args+[url]
      c,o,e=run(cmd)
      f=find_audio(vid)
      if f:return True,str(f),name,(e or o)[-1200:]
    return False,'','none','all profiles failed/timed out'

idx=json.loads(INDEX_PATH.read_text(encoding='utf-8'))
items=idx.get('items',[])
by_id={i.get('videoId'):i for i in items if i.get('videoId')}
failed=[]
for line in SRC_REPORT.read_text(encoding='utf-8').splitlines():
    line=line.strip()
    if line.startswith('- `') and 'download_failed' in line:
        failed.append(line.split('`')[1])
failed=list(dict.fromkeys(failed))

report=['# TraderChenge Expansion Retry Report (+50 recovery)','',f'- Started (UTC): {now()}',f'- Target failed IDs: **{len(failed)}**',f'- Batch size: **{BATCH}**',f'- OPENAI_API_KEY present: **{"yes" if HAS_KEY else "no"}**','', '## Batch progress']

a=b=c=0
for i in range(0,len(failed),BATCH):
    batch=failed[i:i+BATCH]
    bo=bdf=btf=0
    notes=[]
    for vid in batch:
      m=by_id.get(vid,{"videoId":vid,"url":f"https://www.youtube.com/watch?v={vid}","channel":"@TraderChenge"})
      m['retry']={'lastRetryAt':now(),'run':'expand_50_recovery_fast'}
      tr=TRANS_DIR/f'{vid}.txt'
      if tr.exists() and tr.stat().st_size>0:
         m['status']='ok'; bo+=1; a+=1
         m['transcription']={'ok':True,'skipped':True,'transcriptPath':str(tr),'reason':'already_exists'}
         m['download']=m.get('download',{'ok':True,'skipped':True})
         notes.append(f'- `{vid}`: ok (transcript existed)')
      else:
         af=find_audio(vid)
         if af: okd,ap,pf,de=True,str(af),'existing_file',''
         else: okd,ap,pf,de=download(vid)
         m['download']={'ok':okd,'audioPath':ap,'profileUsed':pf,'stderr':de,'retriedAt':now()}
         if not okd:
            m['status']='download_failed'; bdf+=1; b+=1
            m['errors']=list(set((m.get('errors') or [])+['audio download failed on retry']))
            notes.append(f'- `{vid}`: download_failed ({pf})')
         else:
            # transcription step (chunk rule acknowledged; cannot proceed without key)
            m['transcription']={'ok':False,'transcriptPath':str(tr),'chunkCount':0,'chunkRuleBytes':CHUNK_RULE,'errors':['Missing OPENAI_API_KEY'],'retriedAt':now()}
            m['status']='transcribe_failed'; btf+=1; c+=1
            m['errors']=list(set((m.get('errors') or [])+['Missing OPENAI_API_KEY','transcription failed on retry']))
            notes.append(f'- `{vid}`: transcribe_failed (profile={pf})')
      m['finishedAt']=now()
      (META_DIR/f'{vid}.json').write_text(json.dumps(m,ensure_ascii=False,indent=2),encoding='utf-8')
      by_id[vid]=m

    idx['items']=sorted(list(by_id.values()),key=lambda x:x.get('startedAt',''))
    idx['recoveryRetry']={'updatedAt':now(),'sourceFailedIds':len(failed),'processedSoFar':i+len(batch),'batchesDone':(i//BATCH)+1}
    idx['generatedAt']=now(); idx['count']=len(idx['items'])
    idx['success']=sum(1 for x in idx['items'] if x.get('status')=='ok')
    idx['downloadFailed']=sum(1 for x in idx['items'] if x.get('status')=='download_failed')
    idx['transcribeFailed']=sum(1 for x in idx['items'] if x.get('status')=='transcribe_failed')
    INDEX_PATH.write_text(json.dumps(idx,ensure_ascii=False,indent=2),encoding='utf-8')

    report.append(f'### Batch {(i//BATCH)+1} ({i+1}-{i+len(batch)})')
    report.append(f'- Batch result: ok={bo}, download_failed={bdf}, transcribe_failed={btf}')
    report.append(f'- Cumulative retry result: ok={a}, download_failed={b}, transcribe_failed={c}')
    report.extend(notes); report.append('')
    RETRY_REPORT.write_text('\n'.join(report)+'\n',encoding='utf-8')

fok=fdf=ftf=0; blockers=[]
for vid in failed:
    st=(by_id.get(vid) or {}).get('status')
    if st=='ok': fok+=1
    elif st=='download_failed': fdf+=1; blockers.append(f'{vid}: download_failed')
    elif st=='transcribe_failed': ftf+=1; blockers.append(f'{vid}: transcribe_failed')
    else: blockers.append(f'{vid}: unknown_status')

report += ['## Final summary',f'- Finished (UTC): {now()}',f'- Retry target IDs: **{len(failed)}**',f'- Recovered to ok: **{fok}**',f'- Still download_failed: **{fdf}**',f'- Still transcribe_failed: **{ftf}**','','## Remaining blockers']
report += [f'- {b}' for b in blockers] if blockers else ['- None']
RETRY_REPORT.write_text('\n'.join(report)+'\n',encoding='utf-8')
print(json.dumps({'targetFailedIds':len(failed),'recoveredOk':fok,'remainingDownloadFailed':fdf,'remainingTranscribeFailed':ftf,'index':str(INDEX_PATH),'report':str(RETRY_REPORT)}))
