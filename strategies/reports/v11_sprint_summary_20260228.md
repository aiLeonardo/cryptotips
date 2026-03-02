# v11 Sprint Summary (2018-01-01~2026-02-28)

## Core Comparison

| Strategy | TradeCount | WinRate | PF | MaxDD | NetPnL | ExcessReturn |
|---|---:|---:|---:|---:|---:|---:|
| Buy&Hold | - | - | - | 46095.58 | 37153.39 | 0.00 |
| v8 | 92 | 48.91% | 1.776 | 516.12 | 2243.45 | -34909.95 |
| v9 | 157 | 44.59% | 1.432 | 2746.64 | 4235.43 | -32917.96 |
| v10 | 15 | 33.33% | 1.731 | 25652.93 | 19013.68 | -18139.72 |
| v11 | 891 | 39.96% | 1.358 | 10490.05 | 41873.60 | 4720.21 |

## Parameter Search

- Search runs: 32 (16 for v11_a收益优先, 16 for v11_b回撤优先)
- Search report JSON: strategies/reports/v11_sprint_search_20260228_0745.json
- Search report MD: strategies/reports/v11_sprint_search_20260228_0745.md
- Best Excess: v11_a with bull_max=0.95, bear_floor=0.74, Excess=4720.21
- Recommended final profile: v11 (defaults aligned to above best point)

## Final Artifacts

- v8: strategies/spot_btc_conservative_v1/reports/replay_v8_10k_20180101_20260228_20260228_074810.json
- v9: strategies/spot_btc_conservative_v1/reports/replay_v9_10k_20180101_20260228_20260228_075058.json
- v10: strategies/spot_btc_conservative_v1/reports/replay_v10_10k_20180101_20260228_20260228_075304.json
- v11: strategies/spot_btc_conservative_v1/reports/replay_v11_10k_20180101_20260228_20260228_075319.json
- v11 search: strategies/reports/v11_sprint_search_20260228_0745.{json,md}
