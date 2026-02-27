# TraderChenge 数据扩建报告 v1

- 生成时间: 2026-02-27T02:51:17.607147+00:00
- 视频窗口(估算): 2026-01-27T13:24:44.155428+00:00 ~ 2026-02-26T13:24:44.155428+00:00
- 拉取窗口: 2025-12-13T13:24:44.155428+00:00 ~ 2026-03-08T13:24:44.155428+00:00

## BTCUSDT 多周期 OHLCV

| 周期 | 行数 | 起始 | 结束 | 文件 |
|---|---:|---|---|---|
| 5m | 21762 | 2025-12-13 13:25:00+00:00 | 2026-02-27 02:50:00+00:00 | data/market_ext/BTCUSDT_5m.csv |
| 15m | 7254 | 2025-12-13 13:30:00+00:00 | 2026-02-27 02:45:00+00:00 | data/market_ext/BTCUSDT_15m.csv |
| 1h | 1813 | 2025-12-13 14:00:00+00:00 | 2026-02-27 02:00:00+00:00 | data/market_ext/BTCUSDT_1h.csv |
| 4h | 453 | 2025-12-13 16:00:00+00:00 | 2026-02-27 00:00:00+00:00 | data/market_ext/BTCUSDT_4h.csv |
| 1d | 76 | 2025-12-14 00:00:00+00:00 | 2026-02-27 00:00:00+00:00 | data/market_ext/BTCUSDT_1d.csv |
| 1w | 11 | 2025-12-15 00:00:00+00:00 | 2026-02-23 00:00:00+00:00 | data/market_ext/BTCUSDT_1w.csv |

## 衍生数据可用性

| 数据 | 行数 | 状态 | 来源/替代 |
|---|---:|---|---|
| funding_rate | 227 | ok | fapi/v1/fundingRate |
| open_interest_hist | 500 | ok | /futures/data/openInterestHist |
| global_long_short_account_ratio | 500 | ok | /futures/data/globalLongShortAccountRatio |
| top_long_short_account_ratio | 500 | ok | /futures/data/topLongShortAccountRatio |
| top_long_short_position_ratio | 500 | ok | /futures/data/topLongShortPositionRatio |
| basis_proxy | 1813 | ok | perp_close - spot_close |

## 字段说明（最小可用）
- funding_rate: fundingTime, fundingRate, markPrice
- open_interest_hist: ts, sumOpenInterest, sumOpenInterestValue
- long_short_ratio: ts, longShortRatio, longAccount/shortAccount 或 longPosition/shortPosition
- basis_proxy: open_time, spot_close, perp_close, basis_abs, basis_pct

## 缺失影响说明
- 若交易所限流或区域访问受限，衍生指标将短样本或缺失，影响 NarrativeConsistency 与 VolConfirmScore 的衍生增强项。
- 可替代来源：Coinglass、CryptoQuant、Kaiko（需 API key）。本版本保持可复现的公开接口。