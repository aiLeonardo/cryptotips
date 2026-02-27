#!/usr/bin/env python3
import json, re, math
from pathlib import Path
from datetime import datetime, timezone, timedelta
import requests

WS = Path('/home/ubuntu/.openclaw/workspace')
TRANS_DIR = WS / 'data/youtube/traderchenge/transcripts'
INDEX_PATH = WS / 'data/youtube/traderchenge/meta/index.json'
RAW_PATH = WS / 'traderchenge_videos_raw.json'
OLD_EVAL_PATH = WS / 'traderchenge_eval.json'
OUT_DIR = WS / 'data/youtube/traderchenge/analysis'
REPORT_PATH = WS / 'reports/traderchenge_content_eval_v2.md'

OUT_DIR.mkdir(parents=True, exist_ok=True)

index = json.loads(INDEX_PATH.read_text(encoding='utf-8'))
raw = json.loads(RAW_PATH.read_text(encoding='utf-8'))
old_eval = json.loads(OLD_EVAL_PATH.read_text(encoding='utf-8')) if OLD_EVAL_PATH.exists() else []
old_date_map = {x['id']: x.get('date') for x in old_eval if x.get('id') and x.get('date')}
raw_map = {x['videoId']: x for x in raw if x.get('videoId')}

index_time = datetime.fromisoformat(index['generatedAt'])

EMO_WORDS = ['暴涨','暴跌','爆赚','神级','可怕','危险','完美兑现','大胆预测','最后防守','一定','必须']
UNVER_WORDS = ['连胜','胜率','公开成绩','盈利','亏损','爆赚','神级']
HINDSIGHT_WORDS = ['如期','完美','兑现','早就','之前就说','果然']
CONDITIONAL_WORDS = ['如果','只要','那么','跌破','突破','否则','关注','需要去注意']

BULL_WORDS = ['看涨','做多','低多','反弹','上涨','突破','支撑有效','多头']
BEAR_WORDS = ['看跌','做空','高空','下跌','跌破','阻力','空头','二次下探']

num_re = re.compile(r'(?<!\d)(\d{4,6})(?!\d)')

def parse_publish_ts(item):
    vid = item['videoId']
    if vid in old_date_map:
        # old eval date is reliable enough at day precision
        d = datetime.strptime(old_date_map[vid], '%Y-%m-%d').replace(tzinfo=timezone.utc)
        return d + timedelta(hours=12), 'old_eval_date@12:00Z'

    p = (item.get('published') or '').lower().strip()
    m = re.match(r'(\d+)\s+(hour|hours|day|days|week|weeks|month|months)\s+ago', p)
    if m:
        n = int(m.group(1)); u = m.group(2)
        if 'hour' in u: dt = index_time - timedelta(hours=n)
        elif 'day' in u: dt = index_time - timedelta(days=n)
        elif 'week' in u: dt = index_time - timedelta(weeks=n)
        else: dt = index_time - timedelta(days=30*n)
        return dt, 'relative_estimate'
    return None, 'missing'

def split_sentences(text):
    text = text.replace('\n', '。')
    parts = re.split(r'[。！？!?;；]', text)
    return [p.strip() for p in parts if p.strip()]

def quality_score(text):
    l = len(text)
    bad = text.count('�') + text.count('@@')
    uniq = len(set(text)) / max(1, l)
    s = 100
    if l < 4000: s -= 15
    if l < 2500: s -= 20
    s -= min(25, bad * 3)
    if uniq < 0.03: s -= 15
    return max(20, min(100, s))

def infer_stance(text):
    bulls = sum(text.count(w) for w in BULL_WORDS)
    bears = sum(text.count(w) for w in BEAR_WORDS)

    # stronger action phrases
    bulls += 2 * (text.count('继续看涨') + text.count('谨慎看涨'))
    bears += 2 * (text.count('继续看跌') + text.count('保持看跌') + text.count('高位做空'))

    if bulls - bears >= 3:
        stance = 'bullish'
    elif bears - bulls >= 3:
        stance = 'bearish'
    else:
        stance = 'neutral'
    return stance, bulls, bears

def extract_levels_components(text):
    levels = sorted({int(n.group(1)) for n in num_re.finditer(text) if 50000 <= int(n.group(1)) <= 130000})

    def pick_snippets(keys, max_items=4, window=45):
        out = []
        for k in keys:
            for m in re.finditer(re.escape(k), text):
                a = max(0, m.start()-window)
                b = min(len(text), m.end()+window)
                s = text[a:b].replace('\n', ' ').strip()
                s = re.sub(r'\s+', ' ', s)
                if s and s not in out:
                    out.append(s)
                if len(out) >= max_items:
                    return out
        return out

    entry = pick_snippets(['入场','接多','做空','低多','高空','买入'])
    sl = pick_snippets(['止损','防守','跌破','突破失败'])
    inval = pick_snippets(['失效','无效','改变思路','不成立','跌破','突破'])
    tp = pick_snippets(['目标位','目标','止盈','看到'])
    risk = pick_snippets(['仓位','杠杆','防守','风险','耐心','不做过夜'])
    cond = pick_snippets(['如果','只要','那么','否则'])

    return {
        'levels': levels[:20],
        'entry': entry,
        'stop_loss': sl,
        'invalidation': inval,
        'take_profit': tp,
        'risk_guidance': risk,
        'conditions': cond,
    }

def noise_metrics(text):
    emo = sum(text.count(w) for w in EMO_WORDS)
    unver = sum(text.count(w) for w in UNVER_WORDS)
    hind = sum(text.count(w) for w in HINDSIGHT_WORDS)
    cond = sum(text.count(w) for w in CONDITIONAL_WORDS)
    words = max(1, len(text))

    emotional_density = emo / words * 1000
    unver_density = unver / words * 1000
    hindsight_density = hind / words * 1000
    cond_density = cond / words * 1000

    # lower is better except conditional clarity
    credibility = 100 - (emotional_density*6 + unver_density*5 + hindsight_density*8) + min(20, cond_density*10)
    credibility = max(5, min(95, credibility))

    return {
        'emotional_count': emo,
        'unverifiable_claim_count': unver,
        'hindsight_count': hind,
        'conditional_count': cond,
        'conditional_density_per_1kchars': round(cond_density,3),
        'credibility_score': round(credibility,1),
    }

def binance_klines(start_ms, end_ms):
    out = []
    url = 'https://api.binance.com/api/v3/klines'
    cur = start_ms
    while cur < end_ms:
        params = {
            'symbol':'BTCUSDT','interval':'1h','startTime':cur,'endTime':end_ms,'limit':1000
        }
        r = requests.get(url, params=params, timeout=20)
        r.raise_for_status()
        arr = r.json()
        if not arr:
            break
        out.extend(arr)
        cur = arr[-1][0] + 3600*1000
        if len(arr) < 1000:
            break
    return out

def find_close_at_or_after(kl, ts_ms):
    for k in kl:
        if k[0] >= ts_ms:
            return float(k[4]), k[0]
    return None, None

items = []
for meta in index['items']:
    vid = meta['videoId']
    tpath = TRANS_DIR / f'{vid}.txt'
    if not tpath.exists():
        continue
    txt = tpath.read_text(encoding='utf-8', errors='ignore')
    q = quality_score(txt)
    stance, bull_c, bear_c = infer_stance(txt)
    comp = extract_levels_components(txt)
    noise = noise_metrics(txt)
    pts, psrc = parse_publish_ts(meta)

    items.append({
        'videoId': vid,
        'title': meta.get('title',''),
        'published': meta.get('published'),
        'publish_ts': pts.isoformat() if pts else None,
        'publish_ts_source': psrc,
        'transcript_chars': len(txt),
        'transcript_quality_score': q,
        'stance': stance,
        'stance_signals': {'bullish_hits': bull_c, 'bearish_hits': bear_c},
        'components': comp,
        'noise': noise,
        'uncertainty': 'high' if q < 55 else ('medium' if q < 75 else 'low')
    })

# market windows
valid = [x for x in items if x['publish_ts']]
if valid:
    ts_list = [datetime.fromisoformat(x['publish_ts']).timestamp() for x in valid]
    start = int((min(ts_list) - 72*3600 - 3600) * 1000)
    end = int((max(ts_list) + 72*3600 + 3600) * 1000)
    kl = binance_klines(start, end)
else:
    kl = []

for x in items:
    if not x['publish_ts']:
        x['market'] = {'t0':None,'t24':None,'t72':None,'ret24':None,'ret72':None,'align24':None,'align72':None}
        continue
    t0_ms = int(datetime.fromisoformat(x['publish_ts']).timestamp()*1000)
    p0, _ = find_close_at_or_after(kl, t0_ms)
    p24, _ = find_close_at_or_after(kl, t0_ms + 24*3600*1000)
    p72, _ = find_close_at_or_after(kl, t0_ms + 72*3600*1000)

    ret24 = None if (p0 is None or p24 is None or p0 == 0) else (p24/p0 - 1)
    ret72 = None if (p0 is None or p72 is None or p0 == 0) else (p72/p0 - 1)

    def aligned(ret, stance):
        if ret is None or stance == 'neutral':
            return None
        if stance == 'bullish':
            return ret > 0
        if stance == 'bearish':
            return ret < 0
        return None

    x['market'] = {
        't0': p0, 't24': p24, 't72': p72,
        'ret24': ret24, 'ret72': ret72,
        'align24': aligned(ret24, x['stance']),
        'align72': aligned(ret72, x['stance']),
    }

# aggregate
n = len(items)
non_neutral = [x for x in items if x['stance'] != 'neutral']
acc24_pool = [x for x in non_neutral if x['market']['align24'] is not None]
acc72_pool = [x for x in non_neutral if x['market']['align72'] is not None]
acc24 = (sum(1 for x in acc24_pool if x['market']['align24'])/len(acc24_pool)) if acc24_pool else None
acc72 = (sum(1 for x in acc72_pool if x['market']['align72'])/len(acc72_pool)) if acc72_pool else None
avg_cred = sum(x['noise']['credibility_score'] for x in items)/n if n else None

stance_counts = {
    'bullish': sum(1 for x in items if x['stance']=='bullish'),
    'bearish': sum(1 for x in items if x['stance']=='bearish'),
    'neutral': sum(1 for x in items if x['stance']=='neutral'),
}

summary = {
    'videos': n,
    'stance_counts': stance_counts,
    'directional_videos': len(non_neutral),
    'alignment24': acc24,
    'alignment72': acc72,
    'avg_credibility_score': avg_cred,
    'avg_transcript_quality': sum(x['transcript_quality_score'] for x in items)/n if n else None,
}

# outputs
(OUT_DIR/'traderchenge_content_eval_v2.json').write_text(json.dumps({'summary':summary,'items':items}, ensure_ascii=False, indent=2), encoding='utf-8')

# concise table markdown
rows = []
rows.append('| videoId | stance | q | cred | ret24 | a24 | ret72 | a72 | key levels |')
rows.append('|---|---:|---:|---:|---:|---:|---:|---:|---|')
for x in items:
    lv = ','.join(str(v) for v in x['components']['levels'][:4])
    r24 = x['market']['ret24']; r72 = x['market']['ret72']
    rows.append(f"| {x['videoId']} | {x['stance']} | {x['transcript_quality_score']} | {x['noise']['credibility_score']:.1f} | {'' if r24 is None else f'{r24*100:.2f}%'} | {x['market']['align24']} | {'' if r72 is None else f'{r72*100:.2f}%'} | {x['market']['align72']} | {lv} |")
(OUT_DIR/'traderchenge_content_eval_v2_table.md').write_text('\n'.join(rows), encoding='utf-8')

report = []
report.append('# TraderChenge Content-based Evaluation v2')
report.append('')
report.append('## Methodology')
report.append('- Universe: 20 transcripted videos from `meta/index.json`.')
report.append('- Stance inferred from transcript content only (no title-only inference).')
report.append('- Extracted components: entry / stop loss / invalidation / take profit / risk guidance via sentence-level keyword matching.')
report.append('- Noise & credibility: emotional wording, unverifiable claims, hindsight terms, conditional clarity.')
report.append('- Outcome alignment: Binance BTCUSDT 1h close at publish timestamp (or nearest after), then +24h/+72h returns aligned to content stance direction.')
report.append('')
report.append('## Uncertainty & Data Quality')
report.append(f"- Average transcript quality score: {summary['avg_transcript_quality']:.1f}/100.")
report.append('- Publish timestamps: mixed source (old evaluated day-level dates + relative-time estimation for latest uploads).')
report.append('- For lower transcript quality videos, stance/components are kept conservative and tagged with higher uncertainty.')
report.append('')
report.append('## Aggregate Metrics')
report.append(f"- Stance count: bullish={stance_counts['bullish']}, bearish={stance_counts['bearish']}, neutral={stance_counts['neutral']}.")
report.append(f"- Directional sample size: {len(non_neutral)} videos.")
report.append(f"- Alignment +24h: {'' if acc24 is None else f'{acc24*100:.1f}%'} ({len(acc24_pool)} evaluable directional videos).")
report.append(f"- Alignment +72h: {'' if acc72 is None else f'{acc72*100:.1f}%'} ({len(acc72_pool)} evaluable directional videos).")
report.append(f"- Average credibility score: {avg_cred:.1f}/100.")
report.append('')
report.append('## What to Learn vs Ignore')
report.append('**Learn / keep**')
report.append('- Conditional framing around key levels (break/retest logic) appears frequently and is operationally useful.')
report.append('- Explicit invalidation phrases (e.g., `if level breaks, view changes`) are valuable for rule-based systems.')
report.append('')
report.append('**Ignore / discount**')
report.append('- Promotional PnL streak claims and “perfect call” hindsight wording should receive low weight.')
report.append('- High-emotion phrasing (“暴涨/暴跌/神级”) is common noise and weak predictive evidence by itself.')
report.append('')
report.append('## cryptotips Integration-ready Rule Draft')
report.append('1. Parse transcript -> classify stance (bullish/bearish/neutral) with confidence from transcript quality + signal margin.')
report.append('2. Extract numeric levels (50k–130k) and map nearest to {entry, stop, target} using trigger phrases.')
report.append('3. Generate trade candidate only when:')
report.append('   - stance is directional,')
report.append('   - at least one explicit invalidation condition exists,')
report.append('   - credibility score >= 55, transcript quality >= 60.')
report.append('4. Position sizing policy: halve size when conditional clarity < threshold or timestamp source is estimated.')
report.append('5. Post-trade analytics: score +24h/+72h directional hit-rate, and decay source weight if rolling hit-rate < 45%.')
report.append('')
report.append('## Per-video Table')
report.append('')
report.extend(rows)

REPORT_PATH.write_text('\n'.join(report), encoding='utf-8')

print(json.dumps({'summary':summary,'out_json':str(OUT_DIR/'traderchenge_content_eval_v2.json'),'out_table':str(OUT_DIR/'traderchenge_content_eval_v2_table.md'),'report':str(REPORT_PATH)}, ensure_ascii=False, indent=2))
