#!/usr/bin/env python3
import json
import math
import re
import time
from datetime import datetime, timedelta, timezone
from pathlib import Path

import pandas as pd
import requests

BASE = Path('/home/ubuntu/.openclaw/workspace')
META_INDEX = BASE / 'data/youtube/traderchenge/meta/index.json'
OUT_DIR = BASE / 'data/market_ext'
REPORT = BASE / 'reports/traderchenge_data_extension_v1.md'

BINANCE_SPOT = 'https://api.binance.com'
BINANCE_FUT = 'https://fapi.binance.com'


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


def get_json(url, params=None, retries=4, sleep=1.0):
    err = None
    for _ in range(retries):
        try:
            r = requests.get(url, params=params, timeout=20)
            if r.status_code == 200:
                return r.json()
            err = f'{r.status_code} {r.text[:200]}'
        except Exception as e:
            err = str(e)
        time.sleep(sleep)
    raise RuntimeError(f'GET failed: {url} params={params} err={err}')


def fetch_klines(symbol: str, interval: str, start_ms: int, end_ms: int, futures=False) -> pd.DataFrame:
    rows = []
    cur = start_ms
    endpoint = '/fapi/v1/klines' if futures else '/api/v3/klines'
    base = BINANCE_FUT if futures else BINANCE_SPOT
    while cur < end_ms:
        data = get_json(base + endpoint, {
            'symbol': symbol,
            'interval': interval,
            'startTime': cur,
            'endTime': end_ms,
            'limit': 1000,
        })
        if not data:
            break
        rows.extend(data)
        last_open = int(data[-1][0])
        next_cur = last_open + 1
        if next_cur <= cur:
            break
        cur = next_cur
        if len(data) < 1000:
            break
        time.sleep(0.08)
    if not rows:
        return pd.DataFrame()
    df = pd.DataFrame(rows, columns=[
        'open_time', 'open', 'high', 'low', 'close', 'volume', 'close_time',
        'quote_volume', 'trades', 'taker_base', 'taker_quote', 'ignore'
    ])
    for c in ['open', 'high', 'low', 'close', 'volume', 'quote_volume', 'taker_base', 'taker_quote']:
        df[c] = pd.to_numeric(df[c], errors='coerce')
    df['open_time'] = pd.to_datetime(df['open_time'], unit='ms', utc=True)
    df['close_time'] = pd.to_datetime(df['close_time'], unit='ms', utc=True)
    return df.drop_duplicates('open_time').sort_values('open_time').reset_index(drop=True)


def fetch_funding(symbol: str, start_ms: int, end_ms: int) -> pd.DataFrame:
    out, cur = [], start_ms
    while cur < end_ms:
        data = get_json(BINANCE_FUT + '/fapi/v1/fundingRate', {
            'symbol': symbol, 'startTime': cur, 'endTime': end_ms, 'limit': 1000
        })
        if not data:
            break
        out.extend(data)
        last = int(data[-1]['fundingTime'])
        cur = last + 1
        if len(data) < 1000:
            break
        time.sleep(0.08)
    if not out:
        return pd.DataFrame()
    df = pd.DataFrame(out)
    df['fundingTime'] = pd.to_datetime(df['fundingTime'], unit='ms', utc=True)
    df['fundingRate'] = pd.to_numeric(df['fundingRate'], errors='coerce')
    if 'markPrice' in df.columns:
        df['markPrice'] = pd.to_numeric(df['markPrice'], errors='coerce')
    return df


def fetch_futures_stat(path: str, symbol='BTCUSDT', period='1h', limit=500) -> pd.DataFrame:
    data = get_json(BINANCE_FUT + path, {
        'symbol': symbol, 'period': period, 'limit': limit
    })
    if not data:
        return pd.DataFrame()
    df = pd.DataFrame(data)
    tcol = 'timestamp' if 'timestamp' in df.columns else 'time'
    if tcol in df.columns:
        df[tcol] = pd.to_datetime(pd.to_numeric(df[tcol], errors='coerce'), unit='ms', utc=True)
        df = df.rename(columns={tcol: 'ts'})
    for c in df.columns:
        if c not in ['symbol', 'ts']:
            try:
                df[c] = pd.to_numeric(df[c], errors='coerce')
            except Exception:
                pass
    return df.sort_values('ts').reset_index(drop=True) if 'ts' in df.columns else df


def main():
    OUT_DIR.mkdir(parents=True, exist_ok=True)
    with open(META_INDEX, 'r', encoding='utf-8') as f:
        idx = json.load(f)

    ref = datetime.fromisoformat(idx['generatedAt'])
    publish_times = []
    for it in idx['items']:
        publish_times.append(parse_relative_time(it.get('published', ''), ref))

    min_pub, max_pub = min(publish_times), max(publish_times)
    start = (min_pub - timedelta(days=45)).replace(tzinfo=timezone.utc)
    end = (max_pub + timedelta(days=10)).replace(tzinfo=timezone.utc)
    start_ms, end_ms = int(start.timestamp()*1000), int(end.timestamp()*1000)

    coverage = []
    intervals = ['5m', '15m', '1h', '4h', '1d', '1w']
    for itv in intervals:
        df = fetch_klines('BTCUSDT', itv, start_ms, end_ms, futures=False)
        p = OUT_DIR / f'BTCUSDT_{itv}.csv'
        df.to_csv(p, index=False)
        coverage.append((itv, len(df), str(df['open_time'].min()) if len(df) else 'NA', str(df['open_time'].max()) if len(df) else 'NA', p.name))

    # futures klines for basis proxy
    fut_1h = fetch_klines('BTCUSDT', '1h', start_ms, end_ms, futures=True)
    fut_1h.to_csv(OUT_DIR / 'BTCUSDT_PERP_1h.csv', index=False)

    # derivatives
    deriv_notes = []
    try:
        fr = fetch_funding('BTCUSDT', start_ms, end_ms)
        fr.to_csv(OUT_DIR / 'BTCUSDT_funding_rate.csv', index=False)
        deriv_notes.append(('funding_rate', len(fr), 'ok', 'fapi/v1/fundingRate'))
    except Exception as e:
        deriv_notes.append(('funding_rate', 0, f'failed: {e}', 'alt: Coinglass/API'))

    deriv_targets = [
        ('open_interest_hist', '/futures/data/openInterestHist'),
        ('global_long_short_account_ratio', '/futures/data/globalLongShortAccountRatio'),
        ('top_long_short_account_ratio', '/futures/data/topLongShortAccountRatio'),
        ('top_long_short_position_ratio', '/futures/data/topLongShortPositionRatio'),
    ]
    for name, path in deriv_targets:
        try:
            df = fetch_futures_stat(path, period='1h', limit=500)
            df.to_csv(OUT_DIR / f'BTCUSDT_{name}_1h.csv', index=False)
            deriv_notes.append((name, len(df), 'ok', path))
        except Exception as e:
            deriv_notes.append((name, 0, f'failed: {e}', 'alt: exchange agg providers'))

    # basis proxy (perp vs spot)
    try:
        spot_1h = pd.read_csv(OUT_DIR / 'BTCUSDT_1h.csv', parse_dates=['open_time'])
        fut_1h = pd.read_csv(OUT_DIR / 'BTCUSDT_PERP_1h.csv', parse_dates=['open_time'])
        merged = spot_1h[['open_time', 'close']].rename(columns={'close': 'spot_close'}).merge(
            fut_1h[['open_time', 'close']].rename(columns={'close': 'perp_close'}), on='open_time', how='inner'
        )
        merged['basis_abs'] = merged['perp_close'] - merged['spot_close']
        merged['basis_pct'] = merged['basis_abs'] / merged['spot_close']
        merged.to_csv(OUT_DIR / 'BTCUSDT_basis_proxy_1h.csv', index=False)
        deriv_notes.append(('basis_proxy', len(merged), 'ok', 'perp_close - spot_close'))
    except Exception as e:
        deriv_notes.append(('basis_proxy', 0, f'failed: {e}', 'alt: quarterly futures basis endpoint'))

    lines = []
    lines.append('# TraderChenge 数据扩建报告 v1')
    lines.append('')
    lines.append(f'- 生成时间: {datetime.now(timezone.utc).isoformat()}')
    lines.append(f'- 视频窗口(估算): {min_pub.isoformat()} ~ {max_pub.isoformat()}')
    lines.append(f'- 拉取窗口: {start.isoformat()} ~ {end.isoformat()}')
    lines.append('')
    lines.append('## BTCUSDT 多周期 OHLCV')
    lines.append('')
    lines.append('| 周期 | 行数 | 起始 | 结束 | 文件 |')
    lines.append('|---|---:|---|---|---|')
    for itv, n, st, ed, fn in coverage:
        lines.append(f'| {itv} | {n} | {st} | {ed} | data/market_ext/{fn} |')
    lines.append('')
    lines.append('## 衍生数据可用性')
    lines.append('')
    lines.append('| 数据 | 行数 | 状态 | 来源/替代 |')
    lines.append('|---|---:|---|---|')
    for name, n, st, src in deriv_notes:
        lines.append(f'| {name} | {n} | {st} | {src} |')
    lines.append('')
    lines.append('## 字段说明（最小可用）')
    lines.append('- funding_rate: fundingTime, fundingRate, markPrice')
    lines.append('- open_interest_hist: ts, sumOpenInterest, sumOpenInterestValue')
    lines.append('- long_short_ratio: ts, longShortRatio, longAccount/shortAccount 或 longPosition/shortPosition')
    lines.append('- basis_proxy: open_time, spot_close, perp_close, basis_abs, basis_pct')
    lines.append('')
    lines.append('## 缺失影响说明')
    lines.append('- 若交易所限流或区域访问受限，衍生指标将短样本或缺失，影响 NarrativeConsistency 与 VolConfirmScore 的衍生增强项。')
    lines.append('- 可替代来源：Coinglass、CryptoQuant、Kaiko（需 API key）。本版本保持可复现的公开接口。')

    REPORT.write_text('\n'.join(lines), encoding='utf-8')
    print('done')


if __name__ == '__main__':
    main()
