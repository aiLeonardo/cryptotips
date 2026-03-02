#!/usr/bin/env python3
import json
import glob
import os
from datetime import datetime, timezone

ROOT = "/home/ubuntu/cryptotips"
REPORT_DIR = os.path.join(ROOT, "strategies/spot_btc_conservative_v1/reports")
OUT_DIR = os.path.join(ROOT, "strategies/reports")

SEGMENTS = [
    ("A", "2018-01-01", "2021-12-31"),
    ("B", "2022-01-01", "2023-12-31"),
    ("C", "2024-01-01", "2026-02-28"),
]
PROFILES = ["v8", "v9", "v10", "v11"]


def load_latest_report(profile, start, end):
    ss = start.replace("-", "")
    ee = end.replace("-", "")
    pat = os.path.join(REPORT_DIR, f"replay_{profile}_10k_{ss}_{ee}_*.json")
    files = glob.glob(pat)
    if not files:
        raise FileNotFoundError(f"no replay file for {profile} {start}~{end}")

    def key(fp):
        try:
            j = json.load(open(fp, "r", encoding="utf-8"))
            ga = j.get("generated_at_utc", "")
            return ga
        except Exception:
            return ""

    files.sort(key=key)
    fp = files[-1]
    data = json.load(open(fp, "r", encoding="utf-8"))
    data["__file"] = os.path.relpath(fp, ROOT)
    return data


def bear_ratio_from_benchmark_monthly(report):
    m = report.get("benchmark", {}).get("monthly_returns", {}) or {}
    vals = [v for v in m.values() if isinstance(v, (int, float))]
    if not vals:
        return 0.35
    neg = sum(1 for v in vals if v < 0)
    return neg / len(vals)


def years_in_segment(start, end):
    s = datetime.strptime(start, "%Y-%m-%d")
    e = datetime.strptime(end, "%Y-%m-%d")
    days = (e - s).days + 1
    return days / 365.25


def build():
    now = datetime.now(timezone.utc).strftime("%Y%m%d_%H%M%S")

    segment_rows = {}
    segment_bh = {}

    for seg_name, start, end in SEGMENTS:
        rows = {}
        for p in PROFILES:
            rows[p] = load_latest_report(p, start, end)
        segment_rows[seg_name] = rows

        bh_candidates = [rows[p].get("benchmark", {}).get("net_pnl", 0.0) for p in PROFILES]
        bh_net = max(set(round(x, 6) for x in bh_candidates), key=lambda v: bh_candidates.count(v)) if bh_candidates else 0.0
        if not bh_net and bh_candidates:
            bh_net = bh_candidates[0]
        segment_bh[seg_name] = float(bh_net)

    # cost scenarios
    COSTS = [
        {"name": "baseline", "roundtrip_rate": 0.0},
        {"name": "cost_s1", "roundtrip_rate": 0.002},  # 0.10% per side
        {"name": "cost_s2", "roundtrip_rate": 0.003},  # 0.15% per side
    ]

    out = {
        "generated_at_utc": datetime.now(timezone.utc).isoformat(),
        "assumptions": {
            "turnover_notional_proxy": "trade_count * capital_usdt",
            "trading_cost_formula": "trade_count * capital_usdt * roundtrip_rate",
            "funding_formula": "capital_usdt * 5% * years * bear_ratio_proxy (applied to strategy only)",
            "bear_ratio_proxy": "negative_months(benchmark_monthly_returns) / total_months",
            "notes": "框架暂无原生 fee/slippage/funding 模型，以上为近似扣减。",
        },
        "segments": {},
    }

    md = []
    md.append("# v11 稳健性验证（第1步）\n")
    md.append("\n")
    md.append("## 复跑命令\n")
    md.append("```bash\n")
    md.append("cd /home/ubuntu/cryptotips\n")
    for seg_name, start, end in SEGMENTS:
        for p in PROFILES:
            md.append(f"./cryptotips strategy replay --profile {p} --start {start} --end {end}\n")
    md.append("python3 strategies/reports/v11_robustness_step1.py\n")
    md.append("```\n\n")

    decision = {
        "rule": {
            "baseline_excess_positive_segments_min": 2,
            "cost_s2_excess_positive_segments_min": 2,
            "pf_ge_1_segments_min": 2,
        },
        "v11": {},
    }

    v11_baseline_pos = 0
    v11_costs2_pos = 0
    v11_pf_ge1 = 0

    for seg_name, start, end in SEGMENTS:
        rows = segment_rows[seg_name]
        bh_net = segment_bh[seg_name]
        any_report = rows["v11"]
        capital = float(any_report.get("capital_usdt", 10000.0) or 10000.0)
        years = years_in_segment(start, end)
        bear_ratio = bear_ratio_from_benchmark_monthly(any_report)

        seg_obj = {
            "window": {"start": start, "end": end},
            "buy_hold": {"net_pnl": bh_net},
            "bear_ratio_proxy": bear_ratio,
            "years": years,
            "profiles": {},
            "cost_scenarios": {},
        }

        md.append(f"## 分段 {seg_name}（{start} ~ {end}）\n\n")
        md.append("### 基线指标\n\n")
        md.append("| Strategy | TradeCount | WinRate | PF | MaxDD | NetPnL | Excess vs B&H |\n")
        md.append("|---|---:|---:|---:|---:|---:|---:|\n")

        # baseline rows
        for p in PROFILES:
            r = rows[p]
            obj = {
                "trade_count": int(r.get("trade_count", 0)),
                "win_rate": float(r.get("win_rate", 0.0)),
                "profit_factor": float(r.get("profit_factor", 0.0)),
                "max_drawdown": float(r.get("max_drawdown", 0.0)),
                "net_pnl": float(r.get("net_pnl", 0.0)),
                "excess_vs_bh": float(r.get("net_pnl", 0.0) - bh_net),
                "report_json": r.get("__file"),
            }
            seg_obj["profiles"][p] = obj
            md.append(
                f"| {p} | {obj['trade_count']} | {obj['win_rate']*100:.2f}% | {obj['profit_factor']:.3f} | {obj['max_drawdown']:.2f} | {obj['net_pnl']:.2f} | {obj['excess_vs_bh']:.2f} |\n"
            )

        md.append(f"| Buy&Hold | - | - | - | {float(any_report.get('benchmark',{}).get('max_drawdown',0.0)):.2f} | {bh_net:.2f} | 0.00 |\n\n")

        # decision counters for v11 baseline
        if seg_obj["profiles"]["v11"]["excess_vs_bh"] > 0:
            v11_baseline_pos += 1
        if seg_obj["profiles"]["v11"]["profit_factor"] >= 1.0:
            v11_pf_ge1 += 1

        md.append("### 成本敏感性（近似）\n\n")
        md.append(
            "近似公式：`交易成本 = trade_count × capital × roundtrip_rate`；`Funding = capital × 5% × years × bear_ratio_proxy`（仅策略，B&H不计）。\n\n"
        )
        md.append("| Strategy | Baseline NetPnL | Cost S1 NetPnL | S1 Excess vs B&H | Cost S2 NetPnL | S2 Excess vs B&H |\n")
        md.append("|---|---:|---:|---:|---:|---:|\n")

        seg_obj["cost_scenarios"] = {"cost_s1": {}, "cost_s2": {}}

        for p in PROFILES:
            base_net = seg_obj["profiles"][p]["net_pnl"]
            tc = seg_obj["profiles"][p]["trade_count"]
            funding = capital * 0.05 * years * bear_ratio

            s1_net = base_net - (tc * capital * 0.002) - funding
            s2_net = base_net - (tc * capital * 0.003) - funding
            s1_excess = s1_net - bh_net
            s2_excess = s2_net - bh_net

            seg_obj["cost_scenarios"]["cost_s1"][p] = {
                "adj_net_pnl": s1_net,
                "excess_vs_bh": s1_excess,
                "funding_cost": funding,
                "trading_cost": tc * capital * 0.002,
            }
            seg_obj["cost_scenarios"]["cost_s2"][p] = {
                "adj_net_pnl": s2_net,
                "excess_vs_bh": s2_excess,
                "funding_cost": funding,
                "trading_cost": tc * capital * 0.003,
            }

            md.append(f"| {p} | {base_net:.2f} | {s1_net:.2f} | {s1_excess:.2f} | {s2_net:.2f} | {s2_excess:.2f} |\n")

            if p == "v11" and s2_excess > 0:
                v11_costs2_pos += 1

        md.append("| Buy&Hold | {:.2f} | {:.2f} | 0.00 | {:.2f} | 0.00 |\n\n".format(bh_net, bh_net, bh_net))

        out["segments"][seg_name] = seg_obj

    passed = (
        v11_baseline_pos >= decision["rule"]["baseline_excess_positive_segments_min"]
        and v11_costs2_pos >= decision["rule"]["cost_s2_excess_positive_segments_min"]
        and v11_pf_ge1 >= decision["rule"]["pf_ge_1_segments_min"]
    )
    decision["v11"] = {
        "baseline_excess_positive_segments": v11_baseline_pos,
        "cost_s2_excess_positive_segments": v11_costs2_pos,
        "pf_ge_1_segments": v11_pf_ge1,
        "result": "通过" if passed else "不通过",
    }
    out["decision"] = decision

    md.append("## 结论\n\n")
    md.append(f"- 判定规则：baseline 跑赢 B&H 段数 >=2；cost_s2 跑赢 B&H 段数 >=2；PF>=1 段数 >=2。\n")
    md.append(
        f"- v11 实际：baseline 跑赢段数={v11_baseline_pos}，cost_s2 跑赢段数={v11_costs2_pos}，PF>=1 段数={v11_pf_ge1}。\n"
    )
    md.append(f"- 最终判定：**{('通过' if passed else '不通过')}**。\n\n")

    md.append("## 风险提示\n\n")
    md.append("- v10/v8/v9 在部分分段 trade_count 极低（2~10笔），统计显著性不足。\n")
    md.append("- 成本/Funding为近似扣减，未按逐笔真实名义、持仓时长精算。\n")
    md.append("- 若要用于实盘门槛判断，建议补充逐笔 notional 与 long/short 持仓时长导出。\n\n")

    md.append("## 下一步建议\n\n")
    md.append("1. 在 replay 报告中新增逐笔 `entry_notional`,`holding_hours`,`side`，做真实成本重算。\n")
    md.append("2. 做 walk-forward（例如每6个月滚动）而非固定三段，统计稳健区间占比。\n")
    md.append("3. 对 v11 额外做参数扰动（bull/bear floor、cooldown）观察过拟合敏感度。\n")

    out_json = os.path.join(OUT_DIR, f"v11_robustness_step1_{now}.json")
    out_md = os.path.join(OUT_DIR, f"v11_robustness_step1_{now}.md")

    with open(out_json, "w", encoding="utf-8") as f:
        json.dump(out, f, ensure_ascii=False, indent=2)
    with open(out_md, "w", encoding="utf-8") as f:
        f.write("".join(md))

    print(out_json)
    print(out_md)


if __name__ == "__main__":
    build()
