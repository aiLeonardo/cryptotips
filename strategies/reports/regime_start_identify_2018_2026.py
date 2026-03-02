#!/usr/bin/env python3
import csv
import datetime as dt
import json
import math
import os
import subprocess
from dataclasses import dataclass
from typing import List, Dict, Tuple

START = dt.date(2018, 1, 1)
END = dt.date(2026, 2, 28)
SYMBOL = "BTCUSDT"

MYSQL = ["mysql", "-N", "-h127.0.0.1", "-ucrypto_user", "-pcrypto_pass", "-Dcrypto_tips"]


def mysql_rows(interval: str) -> List[Tuple[dt.date, float, float, float, float, float]]:
    sql = f"""
SELECT DATE(open_time), CAST(`open` AS DOUBLE), CAST(high AS DOUBLE), CAST(low AS DOUBLE), CAST(`close` AS DOUBLE), CAST(volume AS DOUBLE)
FROM crypto_kline
WHERE symbol='{SYMBOL}' AND `interval`='{interval}'
  AND open_time >= '{START.isoformat()} 00:00:00'
  AND open_time <= '{END.isoformat()} 23:59:59'
ORDER BY open_time ASC;
"""
    p = subprocess.run(MYSQL + ["-e", sql], capture_output=True, text=True, check=True)
    rows = []
    for line in p.stdout.strip().splitlines():
        if not line.strip():
            continue
        d, o, h, l, c, v = line.split("\t")
        rows.append((dt.date.fromisoformat(d), float(o), float(h), float(l), float(c), float(v)))
    return rows


def ema(values: List[float], period: int) -> List[float]:
    out = [0.0] * len(values)
    if not values:
        return out
    alpha = 2.0 / (period + 1.0)
    out[0] = values[0]
    for i in range(1, len(values)):
        out[i] = alpha * values[i] + (1 - alpha) * out[i - 1]
    return out


def rolling_std(values: List[float], window: int) -> List[float]:
    out = [0.0] * len(values)
    for i in range(len(values)):
        if i + 1 < window:
            continue
        seg = values[i + 1 - window : i + 1]
        m = sum(seg) / len(seg)
        var = sum((x - m) ** 2 for x in seg) / len(seg)
        out[i] = math.sqrt(var)
    return out


def pct_return(values: List[float], lookback: int) -> List[float]:
    out = [0.0] * len(values)
    for i in range(len(values)):
        j = i - lookback
        if j >= 0 and values[j] > 0:
            out[i] = values[i] / values[j] - 1.0
    return out


def drawdown(values: List[float], lookback: int) -> List[float]:
    out = [0.0] * len(values)
    for i in range(len(values)):
        st = max(0, i - lookback + 1)
        hh = max(values[st : i + 1])
        out[i] = (hh - values[i]) / hh if hh > 0 else 0.0
    return out


def classify_trend_1d(daily_rows):
    dates = [r[0] for r in daily_rows]
    close = [r[4] for r in daily_rows]
    ema200 = ema(close, 200)
    slope20 = [0.0] * len(close)
    for i in range(len(close)):
        j = i - 20
        if j >= 0 and ema200[j] > 0:
            slope20[i] = ema200[i] / ema200[j] - 1.0
    ret90 = pct_return(close, 90)
    ret180 = pct_return(close, 180)
    dd365 = drawdown(close, 365)

    states, scores = [], []
    for i in range(len(close)):
        bull = 0.0
        bear = 0.0
        rng = 0.0
        if close[i] > ema200[i]:
            bull += 0.35
        else:
            bear += 0.25
        if slope20[i] >= 0.004:
            bull += 0.30
        elif slope20[i] <= -0.004:
            bear += 0.30
        else:
            rng += 0.20
        if ret180[i] >= 0.18:
            bull += 0.20
        elif ret180[i] <= -0.18:
            bear += 0.20
        else:
            rng += 0.10
        if dd365[i] >= 0.35:
            bear += 0.15
        elif dd365[i] <= 0.18 and ret90[i] > -0.05:
            bull += 0.10
        else:
            rng += 0.15

        if abs(slope20[i]) <= 0.0025:
            rng += 0.30

        if bull >= bear and bull >= rng:
            state = "BULL"
            score = min(1.0, bull)
        elif bear >= bull and bear >= rng:
            state = "BEAR"
            score = -min(1.0, bear)
        else:
            state = "RANGE"
            score = 0.0 if rng == 0 else (0.2 if bull > bear else -0.2)
        states.append(state)
        scores.append(score)

    # confirmation smoothing (5d)
    smoothed = states[:]
    for i in range(4, len(states)):
        win = states[i - 4 : i + 1]
        if win.count(win[-1]) >= 4:
            smoothed[i] = win[-1]
        else:
            smoothed[i] = smoothed[i - 1]

    feats = {
        "date": dates,
        "close": close,
        "ema200": ema200,
        "slope20": slope20,
        "ret90": ret90,
        "ret180": ret180,
        "dd365": dd365,
        "score": scores,
        "state": smoothed,
    }
    return feats


def classify_cp_1w(weekly_rows):
    dates = [r[0] for r in weekly_rows]
    close = [r[4] for r in weekly_rows]
    ema52 = ema(close, 52)
    slope8 = [0.0] * len(close)
    for i in range(len(close)):
        j = i - 8
        if j >= 0 and ema52[j] > 0:
            slope8[i] = ema52[i] / ema52[j] - 1.0
    rets = [0.0]
    for i in range(1, len(close)):
        rets.append(math.log(close[i] / close[i - 1]) if close[i - 1] > 0 else 0.0)
    vol8 = rolling_std(rets, 8)
    dd52 = drawdown(close, 52)
    ret26 = pct_return(close, 26)

    raw_state, raw_score = [], []
    for i in range(len(close)):
        bull = 0.0
        bear = 0.0
        rng = 0.0
        if close[i] > ema52[i]:
            bull += 0.35
        else:
            bear += 0.30
        if slope8[i] >= 0.02:
            bull += 0.25
        elif slope8[i] <= -0.02:
            bear += 0.25
        else:
            rng += 0.25
        if ret26[i] >= 0.15:
            bull += 0.20
        elif ret26[i] <= -0.15:
            bear += 0.20
        else:
            rng += 0.10
        if dd52[i] >= 0.30:
            bear += 0.20
        elif dd52[i] <= 0.15:
            bull += 0.10
        else:
            rng += 0.15
        if vol8[i] <= 0.035:
            rng += 0.20

        if bull >= bear and bull >= rng:
            st = "BULL"
            sc = min(1.0, bull)
        elif bear >= bull and bear >= rng:
            st = "BEAR"
            sc = -min(1.0, bear)
        else:
            st = "RANGE"
            sc = 0.0
        raw_state.append(st)
        raw_score.append(sc)

    # 2-week confirm (HMM-like persistence)
    sm = raw_state[:]
    for i in range(1, len(sm)):
        if raw_state[i] != sm[i - 1]:
            if i + 1 < len(sm) and raw_state[i + 1] == raw_state[i]:
                sm[i] = raw_state[i]
            else:
                sm[i] = sm[i - 1]
    return {
        "date": dates,
        "close": close,
        "ema52": ema52,
        "slope8": slope8,
        "ret26": ret26,
        "dd52": dd52,
        "vol8": vol8,
        "score": raw_score,
        "state": sm,
    }


def expand_weekly_to_daily(weekly_feats, daily_dates):
    w_dates = weekly_feats["date"]
    idx = 0
    out_state, out_score, out_meta = [], [], []
    for d in daily_dates:
        while idx + 1 < len(w_dates) and w_dates[idx + 1] <= d:
            idx += 1
        out_state.append(weekly_feats["state"][idx])
        out_score.append(weekly_feats["score"][idx])
        out_meta.append(idx)
    return out_state, out_score, out_meta


def fuse_states(dayf, weekf):
    d_dates = dayf["date"]
    ws, wsc, widx = expand_weekly_to_daily(weekf, d_dates)
    final_state, confidence = [], []
    evidence = []

    def st_to_num(st: str) -> float:
        if st == "BULL":
            return 1.0
        if st == "BEAR":
            return -1.0
        return 0.0

    for i, d in enumerate(d_dates):
        s1, s2 = dayf["state"][i], ws[i]
        sc1, sc2 = dayf["score"][i], wsc[i]

        # weighted consensus (1d reacts faster, 1w suppresses noise)
        v1 = st_to_num(s1) * (0.6 + 0.4 * min(1.0, abs(sc1)))
        v2 = st_to_num(s2) * (0.6 + 0.4 * min(1.0, abs(sc2)))
        combined = 0.55 * v1 + 0.45 * v2

        if combined >= 0.22:
            st = "BULL"
        elif combined <= -0.22:
            st = "BEAR"
        else:
            st = "RANGE"

        agree = 1.0 if s1 == s2 else 0.0
        conf = 0.40 + 0.30 * agree + 0.20 * min(1.0, abs(combined)) + 0.10 * (1.0 - abs(v1 - v2) / 2.0)
        conf = max(0.05, min(0.99, conf))

        final_state.append(st)
        confidence.append(conf)
        wi = widx[i]
        evidence.append({
            "close": dayf["close"][i],
            "ema200": dayf["ema200"][i],
            "slope20": dayf["slope20"][i],
            "ret180": dayf["ret180"][i],
            "dd365": dayf["dd365"][i],
            "w_close": weekf["close"][wi],
            "w_ema52": weekf["ema52"][wi],
            "w_slope8": weekf["slope8"][wi],
            "w_ret26": weekf["ret26"][wi],
            "w_dd52": weekf["dd52"][wi],
            "w_vol8": weekf["vol8"][wi],
            "methodA": s1,
            "methodB": s2,
            "scoreA": sc1,
            "scoreB": sc2,
            "combined": combined,
        })

    # short persistence guard: reject 1-2 day flips
    i = 1
    while i < len(final_state) - 2:
        if final_state[i] != final_state[i - 1] and final_state[i + 1] == final_state[i - 1]:
            final_state[i] = final_state[i - 1]
            confidence[i] = min(confidence[i], 0.62)
        i += 1

    return final_state, confidence, evidence


def build_segments(dates, states, confs, evidence):
    segs = []
    st = 0
    for i in range(1, len(states) + 1):
        if i == len(states) or states[i] != states[st]:
            part_conf = confs[st:i]
            c = sum(part_conf) / len(part_conf)
            ev = evidence[st]
            segs.append({
                "start": dates[st].isoformat(),
                "end": dates[i - 1].isoformat(),
                "state": states[st],
                "confidence": round(c, 3),
                "days": i - st,
                "evidence": ev,
            })
            st = i

    # merge tiny whipsaw segments into previous regime to avoid over-fragmentation
    merged = []
    for s in segs:
        if merged and (s["days"] < 14 or (s["days"] < 21 and s["confidence"] < 0.75)):
            merged[-1]["end"] = s["end"]
            merged[-1]["days"] += s["days"]
            merged[-1]["confidence"] = round((merged[-1]["confidence"] + s["confidence"]) / 2.0, 3)
            continue
        merged.append(s)

    # if same state becomes adjacent after merge, collapse again
    final = []
    for s in merged:
        if final and final[-1]["state"] == s["state"]:
            final[-1]["end"] = s["end"]
            final[-1]["days"] += s["days"]
            final[-1]["confidence"] = round((final[-1]["confidence"] + s["confidence"]) / 2.0, 3)
        else:
            final.append(s)
    return final


def main():
    daily = mysql_rows("1d")
    weekly = mysql_rows("1w")
    if not daily or not weekly:
        raise SystemExit("No BTC data found in DB for 1d/1w")

    dayf = classify_trend_1d(daily)
    weekf = classify_cp_1w(weekly)
    final_state, confs, evidence = fuse_states(dayf, weekf)
    segs = build_segments(dayf["date"], final_state, confs, evidence)
    if segs:
        segs[-1]["end"] = END.isoformat()  # forward-fill to required window end

    startpoints = []
    for s in segs:
        e = s["evidence"]
        basis = {
            "methodA": f"{e['methodA']} score={e['scoreA']:.2f}, close={e['close']:.0f}, ema200={e['ema200']:.0f}, slope20={e['slope20']:.3%}, ret180={e['ret180']:.1%}, dd365={e['dd365']:.1%}",
            "methodB": f"{e['methodB']} score={e['scoreB']:.2f}, w_close={e['w_close']:.0f}, ema52={e['w_ema52']:.0f}, slope8={e['w_slope8']:.2%}, ret26={e['w_ret26']:.1%}, dd52={e['w_dd52']:.1%}, vol8={e['w_vol8']:.3f}",
        }
        startpoints.append({
            "date": s["start"],
            "state": s["state"],
            "confidence": s["confidence"],
            "basis": basis,
        })

    uncertain = [s for s in segs if s["confidence"] < 0.60]

    ts = dt.datetime.utcnow().strftime("%Y%m%d_%H%M%S")
    out_dir = "/home/ubuntu/cryptotips/strategies/reports"
    os.makedirs(out_dir, exist_ok=True)
    json_path = os.path.join(out_dir, f"btc_regime_startpoints_20180101_20260228_{ts}.json")
    md_path = os.path.join(out_dir, f"btc_regime_startpoints_20180101_20260228_{ts}.md")

    payload = {
        "window": [START.isoformat(), END.isoformat()],
        "symbol": SYMBOL,
        "method": {
            "A": "Trend structure (1d EMA200/slope20/ret180/dd365 + 5d confirm)",
            "B": "Change-point/HMM-like scoring (1w EMA52/slope8/ret26/dd52/vol8 + 2w persistence)",
            "fusion": "Cross-confirmation with conflict-to-range and 7d persistence",
            "params": {
                "A": {"slope_bull": 0.004, "slope_bear": -0.004, "ret180": 0.18, "dd_bear": 0.35},
                "B": {"slope8_bull": 0.02, "slope8_bear": -0.02, "ret26": 0.15, "dd52_bear": 0.30, "vol_range": 0.035},
            },
        },
        "segments": segs,
        "startpoints": startpoints,
        "uncertain_segments": uncertain,
    }
    with open(json_path, "w", encoding="utf-8") as f:
        json.dump(payload, f, ensure_ascii=False, indent=2)

    lines = []
    lines.append("# BTC 2018-01-01 ~ 2026-02-28 牛/熊/盘整起点识别\n")
    lines.append("## 方法\n")
    lines.append("- 方法A（趋势结构法，1d）：EMA200、EMA200 20日斜率、180日收益、365日回撤，5日确认\n")
    lines.append("- 方法B（变点/状态分段法，1w）：EMA52、8周斜率、26周收益、52周回撤、8周波动，2周持续确认\n")
    lines.append("- 融合：两法一致优先；冲突默认降级 RANGE，7日持久性过滤\n")

    lines.append("## 起点总表\n")
    lines.append("| 日期 | 状态 | 置信度 | 主要依据 |\n|---|---|---:|---|\n")
    for sp in startpoints:
        basis = sp["basis"]["methodA"] + " ; " + sp["basis"]["methodB"]
        lines.append(f"| {sp['date']} | {sp['state']} | {sp['confidence']:.2f} | {basis} |\n")

    lines.append("\n## 完整分段表\n")
    lines.append("| Start | End | State | Confidence |\n|---|---|---|---:|\n")
    for s in segs:
        lines.append(f"| {s['start']} | {s['end']} | {s['state']} | {s['confidence']:.2f} |\n")

    if uncertain:
        lines.append("\n## 不确定段（置信度 < 0.60）\n")
        for s in uncertain:
            lines.append(f"- {s['start']} ~ {s['end']} {s['state']} (conf={s['confidence']:.2f})\n")

    with open(md_path, "w", encoding="utf-8") as f:
        f.write("".join(lines))

    print(json.dumps({
        "json": json_path,
        "md": md_path,
        "segment_count": len(segs),
        "startpoint_count": len(startpoints),
        "uncertain_count": len(uncertain),
    }, ensure_ascii=False))


if __name__ == "__main__":
    main()
