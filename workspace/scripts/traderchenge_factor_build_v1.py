#!/usr/bin/env python3
import json
import re
from collections import Counter, defaultdict
from datetime import datetime, timedelta, timezone
from pathlib import Path

import numpy as np
import pandas as pd

BASE = Path('/home/ubuntu/.openclaw/workspace')
TRANS_DIR = BASE / 'data/youtube/traderchenge/transcripts'
META_INDEX = BASE / 'data/youtube/traderchenge/meta/index.json'
ANALYSIS_DIR = BASE / 'data/youtube/traderchenge/analysis'
REPORTS_DIR = BASE / 'reports'
MARKET_DIR = BASE / 'data/market_ext'

ONTOLOGY_JSON = ANALYSIS_DIR / 'traderchenge_factor_ontology_v1.json'
ONTOLOGY_MD = REPORTS_DIR / 'traderchenge_factor_ontology_v1.md'
FACTOR_DATASET = ANALYSIS_DIR / 'traderchenge_factor_dataset_v1.csv'
FACTOR_DEF = ANALYSIS_DIR / 'traderchenge_factor_definitions_v1.json'
EVAL_JSON = ANALYSIS_DIR / 'traderchenge_factor_eval_metrics_v1.json'
EVAL_MD = REPORTS_DIR / 'traderchenge_factor_eval_v1.md'
INTEG_MD = REPORTS_DIR / 'traderchenge_factor_integration_v1.md'


CATEGORY_KEYWORDS = {
    'support_resistance': ['支撑', '阻力', '压力位', '支阻', '关键位', '水平位', '区间', '筹码密集', '测试'],
    'breakout_retest': ['突破', '跌破', '回踩', '假跌破', '假突破', '确认', '收回', '测试'],
    'pattern': ['楔形', '三角', '旗形', '吞没', '中继', '结构', '形态', '波浪', '等距'],
    'volume_price': ['放量', '缩量', '量能', '成交量', '十字星', '上影线', '下影线', '波动', '横盘'],
    'risk_management': ['止损', '仓位', '胜率', '风险', '短线', '杠杆', '空仓', '止跌', '信号'],
}

BULL_WORDS = ['看涨', '做多', '反弹', '上涨', '抄底', '止跌']
BEAR_WORDS = ['看跌', '做空', '下跌', '跌破', '暴跌', '空仓']


def parse_relative_time(s: str, ref: datetime) -> datetime:
    s = (s or '').strip().lower()
    m = re.match(r'^(\d+)\s+(minute|minutes|hour|hours|day|days|week|weeks|month|months|year|years)\s+ago$', s)
    if not m:
        return ref
    n = int(m.group(1))
    unit = m.group(2)
    if 'minute' in unit:
        delta = timedelta(minutes=n)
    elif 'hour' in unit:
        delta = timedelta(hours=n)
    elif 'day' in unit:
        delta = timedelta(days=n)
    elif 'week' in unit:
        delta = timedelta(weeks=n)
    elif 'month' in unit:
        delta = timedelta(days=30*n)
    else:
        delta = timedelta(days=365*n)
    return ref - delta


def load_transcripts():
    data = {}
    for p in sorted(TRANS_DIR.glob('*.txt')):
        data[p.stem] = p.read_text(encoding='utf-8', errors='ignore')
    return data


def build_ontology(transcripts: dict):
    cat_stats = {}
    for cat, kws in CATEGORY_KEYWORDS.items():
        c = Counter()
        examples = []
        for vid, txt in transcripts.items():
            for kw in kws:
                n = txt.count(kw)
                if n > 0:
                    c[kw] += n
            if any(kw in txt for kw in kws) and len(examples) < 8:
                sample = txt[:180].replace('\n', ' ')
                examples.append({'video_id': vid, 'snippet': sample})
        cat_stats[cat] = {
            'keywords': kws,
            'total_hits': int(sum(c.values())),
            'keyword_freq': dict(c.most_common()),
            'examples': examples,
        }

    ontology = {
        'version': 'v1',
        'generated_at': datetime.now(timezone.utc).isoformat(),
        'source_count': len(transcripts),
        'categories': cat_stats,
        'factor_mapping': {
            'SRCluster': ['support_resistance', 'pattern'],
            'BreakRetestScore': ['breakout_retest', 'support_resistance'],
            'PatternPressure': ['pattern'],
            'MomentumExhaustion': ['volume_price', 'pattern'],
            'VolConfirmScore': ['volume_price', 'breakout_retest'],
            'InvalidationDistance': ['risk_management', 'support_resistance'],
            'NarrativeConsistency': ['risk_management', 'breakout_retest', 'pattern'],
        },
    }
    return ontology


def narrative_features(text: str):
    bull = sum(text.count(w) for w in BULL_WORDS)
    bear = sum(text.count(w) for w in BEAR_WORDS)
    polarity = (bull - bear) / (bull + bear + 1e-6)
    nums = [float(x) for x in re.findall(r'(?<!\d)(\d{4,6})(?!\d)', text)]
    levels = [n for n in nums if 10000 <= n <= 150000]
    top_levels = levels[:5]
    return {
        'bull_words': bull,
        'bear_words': bear,
        'narrative_polarity': polarity,
        'mentioned_levels': top_levels,
        'mentioned_level_mean': float(np.mean(top_levels)) if top_levels else np.nan,
    }


def add_market_indicators(df):
    d = df.copy().sort_values('open_time').reset_index(drop=True)
    d['ret1'] = d['close'].pct_change()
    d['ret24'] = d['close'].pct_change(24)
    d['ret72'] = d['close'].pct_change(72)
    d['vol_z'] = (d['volume'] - d['volume'].rolling(48).mean()) / (d['volume'].rolling(48).std() + 1e-9)

    tr = np.maximum.reduce([
        (d['high'] - d['low']).values,
        np.abs((d['high'] - d['close'].shift(1)).values),
        np.abs((d['low'] - d['close'].shift(1)).values),
    ])
    d['atr14'] = pd.Series(tr).rolling(14).mean()

    # RSI
    delta = d['close'].diff()
    gain = delta.clip(lower=0).rolling(14).mean()
    loss = (-delta.clip(upper=0)).rolling(14).mean() + 1e-9
    rs = gain / loss
    d['rsi14'] = 100 - (100 / (1 + rs))

    d['rolling_high_20'] = d['high'].rolling(20).max()
    d['rolling_low_20'] = d['low'].rolling(20).min()

    # pivot clusters
    piv_h = d['high'].rolling(5, center=True).max()
    piv_l = d['low'].rolling(5, center=True).min()
    d['pivot_high'] = np.where(d['high'] >= piv_h, d['high'], np.nan)
    d['pivot_low'] = np.where(d['low'] <= piv_l, d['low'], np.nan)
    return d


def nearest_cluster_distance(hist: pd.DataFrame, price: float) -> float:
    lv = pd.concat([hist['pivot_high'].dropna(), hist['pivot_low'].dropna()])
    if lv.empty:
        return np.nan
    # use recent 30 pivot levels and cluster by simple rounding bucket
    lv = lv.tail(30)
    bucket = (lv / 200).round() * 200
    center = lv.groupby(bucket).mean()
    nearest = np.min(np.abs(center.values - price))
    return float(nearest / max(price, 1e-9))


def build_event_dataset(transcripts: dict, meta: dict, mkt: pd.DataFrame):
    ref = datetime.fromisoformat(meta['generatedAt'])
    items = {it['videoId']: it for it in meta['items']}

    rows = []
    for vid, txt in transcripts.items():
        if vid not in items:
            continue
        pub = parse_relative_time(items[vid].get('published', ''), ref).replace(tzinfo=timezone.utc)
        # align to nearest hour
        t = pd.Timestamp(pub).floor('h').tz_convert('UTC').tz_localize(None)
        if t < mkt['open_time'].min() or t > mkt['open_time'].max():
            continue

        idx = (mkt['open_time'] - t).abs().idxmin()
        snap = mkt.loc[idx]
        hist = mkt.iloc[max(0, idx - 120):idx + 1]
        nf = narrative_features(txt)

        sr_dist = nearest_cluster_distance(hist, float(snap['close']))
        brk_dir = np.sign(snap['close'] - snap['rolling_high_20']) - np.sign(snap['close'] - snap['rolling_low_20'])
        retest = 1 - min(abs(snap['close'] - snap['rolling_high_20']) / (snap['atr14'] + 1e-9), 2)
        break_retest = float(np.clip(brk_dir * retest, -2, 2))

        pattern_pressure = float(np.clip(
            txt.count('楔形') * -0.2 + txt.count('三角') * -0.15 + txt.count('吞没') * -0.1 + txt.count('中继') * -0.08 + txt.count('突破') * 0.12,
            -3, 3
        ))

        momentum_exhaust = float(np.clip(((snap['rsi14'] - 50) / 25.0) * (-np.sign(snap['ret24'] if pd.notna(snap['ret24']) else 0)), -3, 3))

        vol_confirm = float(np.clip((snap['vol_z'] if pd.notna(snap['vol_z']) else 0) * np.sign(snap['ret1'] if pd.notna(snap['ret1']) else 0), -3, 3))

        invalid_dist = np.nan
        if pd.notna(nf['mentioned_level_mean']):
            invalid_dist = abs(snap['close'] - nf['mentioned_level_mean']) / (snap['atr14'] + 1e-9)

        trend24 = np.sign(snap['ret24'] if pd.notna(snap['ret24']) else 0)
        narrative_cons = float(np.clip(nf['narrative_polarity'] * trend24, -1, 1))

        close = float(snap['close'])
        row24 = mkt[mkt['open_time'] >= snap['open_time']].head(25)
        row72 = mkt[mkt['open_time'] >= snap['open_time']].head(73)
        fwd24 = (row24['close'].iloc[-1] / close - 1) if len(row24) >= 25 else np.nan
        fwd72 = (row72['close'].iloc[-1] / close - 1) if len(row72) >= 73 else np.nan

        rows.append({
            'video_id': vid,
            'event_time': snap['open_time'],
            'price': close,
            'bull_words': nf['bull_words'],
            'bear_words': nf['bear_words'],
            'narrative_polarity': nf['narrative_polarity'],
            'SRCluster': -sr_dist if pd.notna(sr_dist) else np.nan,
            'BreakRetestScore': break_retest,
            'PatternPressure': pattern_pressure,
            'MomentumExhaustion': momentum_exhaust,
            'VolConfirmScore': vol_confirm,
            'InvalidationDistance': invalid_dist,
            'NarrativeConsistency': narrative_cons,
            'target_24h': fwd24,
            'target_72h': fwd72,
        })

    df = pd.DataFrame(rows).sort_values('event_time').reset_index(drop=True)
    return df


def _spearman_corr(a: pd.Series, b: pd.Series) -> float:
    r1 = a.rank(method='average')
    r2 = b.rank(method='average')
    return float(r1.corr(r2))


def eval_factors(df: pd.DataFrame, factors, target_col):
    out = {}
    valid = df.dropna(subset=[target_col])
    for f in factors:
        d = valid.dropna(subset=[f])
        if len(d) < 12:
            out[f] = {'ic': np.nan, 'n': len(d), 'q_ret_spread': np.nan, 'stability': np.nan}
            continue
        ic = _spearman_corr(d[f], d[target_col])
        d = d.copy()
        d['q'] = pd.qcut(d[f].rank(method='first'), 5, labels=False)
        q_mean = d.groupby('q')[target_col].mean()
        spread = float(q_mean.iloc[-1] - q_mean.iloc[0]) if len(q_mean) == 5 else np.nan
        d['month'] = pd.to_datetime(d['event_time']).dt.to_period('M').astype(str)
        mon_ic = d.groupby('month').apply(lambda x: _spearman_corr(x[f], x[target_col]) if len(x) >= 5 else np.nan)
        stability = float(mon_ic.mean() / (mon_ic.std() + 1e-9)) if len(mon_ic.dropna()) > 1 else np.nan
        out[f] = {'ic': float(ic) if pd.notna(ic) else np.nan, 'n': int(len(d)), 'q_ret_spread': spread, 'stability': stability}
    return out


def main():
    ANALYSIS_DIR.mkdir(parents=True, exist_ok=True)
    REPORTS_DIR.mkdir(parents=True, exist_ok=True)

    transcripts = load_transcripts()
    ontology = build_ontology(transcripts)
    ONTOLOGY_JSON.write_text(json.dumps(ontology, ensure_ascii=False, indent=2), encoding='utf-8')

    md = ['# TraderChenge 因子语义本体 v1', '']
    md.append(f"- 样本数量: {ontology['source_count']}")
    md.append(f"- 生成时间: {ontology['generated_at']}")
    md.append('')
    md.append('| 类别 | 总命中 | 高权重关键词 |')
    md.append('|---|---:|---|')
    for k, v in ontology['categories'].items():
        top = ', '.join([f'{a}:{b}' for a, b in list(v['keyword_freq'].items())[:6]])
        md.append(f'| {k} | {v["total_hits"]} | {top} |')
    md.append('')
    md.append('## 因子映射')
    for f, cats in ontology['factor_mapping'].items():
        md.append(f'- {f}: {", ".join(cats)}')
    ONTOLOGY_MD.write_text('\n'.join(md), encoding='utf-8')

    with open(META_INDEX, 'r', encoding='utf-8') as f:
        meta = json.load(f)

    mkt = pd.read_csv(MARKET_DIR / 'BTCUSDT_1h.csv', parse_dates=['open_time'])
    mkt['open_time'] = pd.to_datetime(mkt['open_time'], utc=True).dt.tz_convert(None)
    mkt = add_market_indicators(mkt)

    df = build_event_dataset(transcripts, meta, mkt)
    df.to_csv(FACTOR_DATASET, index=False)

    factor_defs = {
        'version': 'v1',
        'factors': {
            'SRCluster': '负的价格到近期支撑阻力簇距离（越高表示越接近关键位）',
            'BreakRetestScore': '突破方向 × 回踩强度（基于20期高低点与ATR标准化）',
            'PatternPressure': '转写中形态词汇映射到方向压力分值',
            'MomentumExhaustion': 'RSI位置与24h动量反向程度，刻画动能衰竭',
            'VolConfirmScore': '成交量zscore与当期收益方向一致性',
            'InvalidationDistance': '价格到叙事关键价位距离/ATR（风险空间）',
            'NarrativeConsistency': '叙事极性与市场趋势方向一致性',
        },
        'targets': ['target_24h', 'target_72h'],
        'dataset_path': str(FACTOR_DATASET),
    }
    FACTOR_DEF.write_text(json.dumps(factor_defs, ensure_ascii=False, indent=2), encoding='utf-8')

    factors = list(factor_defs['factors'].keys())
    eval24 = eval_factors(df, factors, 'target_24h')
    eval72 = eval_factors(df, factors, 'target_72h')
    eval_obj = {
        'generated_at': datetime.now(timezone.utc).isoformat(),
        'sample_size': int(len(df)),
        'sample_24h': int(df['target_24h'].notna().sum()),
        'sample_72h': int(df['target_72h'].notna().sum()),
        'metrics_24h': eval24,
        'metrics_72h': eval72,
    }
    EVAL_JSON.write_text(json.dumps(eval_obj, ensure_ascii=False, indent=2), encoding='utf-8')

    lines = ['# TraderChenge 因子评估报告 v1', '']
    lines.append(f"- 样本量: {len(df)} (24h可评估: {eval_obj['sample_24h']}, 72h可评估: {eval_obj['sample_72h']})")
    lines.append('')
    lines.append('## 24h 目标')
    lines.append('| 因子 | IC(Spearman) | 分层收益(Q5-Q1) | 稳定性 | N |')
    lines.append('|---|---:|---:|---:|---:|')
    for f in factors:
        m = eval24[f]
        lines.append(f"| {f} | {m['ic']:.4f} | {m['q_ret_spread']:.4f} | {m['stability']:.4f} | {m['n']} |")
    lines.append('')
    lines.append('## 72h 目标')
    lines.append('| 因子 | IC(Spearman) | 分层收益(Q5-Q1) | 稳定性 | N |')
    lines.append('|---|---:|---:|---:|---:|')
    for f in factors:
        m = eval72[f]
        lines.append(f"| {f} | {m['ic']:.4f} | {m['q_ret_spread']:.4f} | {m['stability']:.4f} | {m['n']} |")
    lines.append('')
    lines.append('## 筛选建议')
    lines.append('- 优先保留绝对IC与分层收益同时为正且跨周期稳定性的因子。')
    lines.append('- InvalidationDistance 适合作为风险约束因子，不单独做alpha。')
    lines.append('- NarrativeConsistency 可与 BreakRetestScore 组合形成“叙事-结构共振”信号。')
    EVAL_MD.write_text('\n'.join(lines), encoding='utf-8')

    integ = [
        '# TraderChenge 因子接入建议 v1',
        '',
        '## 1) 数据管道建议',
        '- 新增 `market_ext` 数据层：spot/perp klines + 衍生指标，按1h主频汇总。',
        '- 新增 `youtube_event` 事实表：video_id, event_time, transcript_hash, narrative_features。',
        '- 通过 Airflow/Cron 两阶段：T+0 拉行情，T+1 回填24h/72h标签。',
        '',
        '## 2) 计算频率',
        '- 市场因子：每小时滚动计算。',
        '- 叙事因子：视频发布/转写完成触发计算；无新视频则沿用最近状态。',
        '- 评估重训：每周一次全量，日更增量监控IC漂移。',
        '',
        '## 3) 风险控制',
        '- 因子门控：当 funding/OI 缺失时，降级到纯价格-成交量因子并降低仓位。',
        '- 使用 InvalidationDistance 设定动态止损带：ATR x k。',
        '- 多因子合成前进行相关性去冗余（|rho|>0.7仅保留一项）。',
        '',
        '## 4) 与 cryptotips 集成',
        '- 不强耦合现有表结构，建议新增 `factor_signal_v2`（宽表）与 `factor_eval_v2`（评估表）。',
        '- API层新增 `/factors/traderchenge/latest` 与 `/factors/traderchenge/backtest`。',
        '- 交易执行层采用“信号->风险预算->下单”三段式，避免直接因子驱动裸下单。',
    ]
    INTEG_MD.write_text('\n'.join(integ), encoding='utf-8')

    print('done')


if __name__ == '__main__':
    main()
