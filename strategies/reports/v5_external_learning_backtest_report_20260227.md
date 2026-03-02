# cryptotips v5 外部学习 + 回测总结（2026-02-27）

## 1) 学习来源与方法总结

> 说明：`web_search` 因环境缺少 Brave API Key 不可用，本次使用 `web_fetch` 拉取公开文档/论文页面完成外部学习。

### 来源清单（覆盖 6 类平台）

| 平台 | URL | 核心思想 | 适配 BTC/USDT 现货理由 | 潜在风险 |
|---|---|---|---|---|
| QuantConnect | https://www.quantconnect.com/docs/v2/writing-algorithms/algorithm-framework/alpha/key-concepts | Insight/组合构建分层，信号与仓位解耦 | 便于把“信号打分”和“风险约束”独立实现 | 过拟合于回测框架参数 |
| TradingView | https://www.tradingview.com/pine-script-docs/concepts/strategies/ | MA 交叉、策略测试器、订单模拟 | 可快速验证规则类策略（趋势/回归） | Pine 假设与真实撮合有偏差 |
| FMZ | https://www.fmz.com/digest-topic/9783 | Sharpe/回撤/收益等指标计算实践 | 直接指导本项目指标口径与对比维度 | 指标漂亮但策略未必稳健 |
| Numerai Signals | https://docs.numer.ai/numerai-signals/signals-overview | 多源特征、模型集成、信号中性化 | 可迁移到 BTC 因子去相关/集成思想 | 数据与目标定义与币圈不同 |
| Numerai Signals | https://docs.numer.ai/numerai-signals/scoring | FNC/MMC、neutralization 强调正交 alpha | 适合做“多因子去冗余”打分框架 | 黑盒目标，不易直接复刻 |
| Kaggle | https://www.kaggle.com/competitions/g-research-crypto-forecasting/overview | 高频/多资产特征工程、泄露控制 | 可借鉴特征工程与时序验证方法 | 页面抽取受限，细节需后续补抓 |
| arXiv | https://arxiv.org/abs/1904.04912 | Deep Momentum：趋势+波动缩放+Sharpe 优化 | 对 BTC 中周期动量策略有借鉴 | 训练与交易成本敏感 |
| arXiv | https://arxiv.org/abs/1808.03668 | 深度时序特征提取（CNN/LSTM） | 可借鉴“特征提取->规则执行”分层 | LOB 级数据项目当前不具备 |

### 12 条策略思路（评分：1-5）

| # | 思路 | 平台来源 | 可复现性 | 可解释性 | 现货适配性 | 备注 |
|---|---|---|---:|---:|---:|---|
| 1 | EMA200 趋势过滤 + EMA20 回踩确认 | QuantConnect/TradingView | 5 | 5 | 5 | 已有 v4 主体 |
| 2 | Donchian/近期高点突破入场 | TradingView/FMZ | 5 | 5 | 5 | 趋势增强 |
| 3 | ATR 波动过滤（过低/过高不做） | FMZ | 5 | 5 | 5 | 抑制噪音与极端 |
| 4 | 成交量放大确认突破 | TradingView | 5 | 4 | 5 | 避免假突破 |
| 5 | RSI+布林带下轨回收做多 | TradingView | 5 | 4 | 5 | 现货友好 |
| 6 | 均值回归+趋势门控（只在大级别多头做） | QuantConnect | 4 | 4 | 5 | 可控回撤 |
| 7 | 多因子动量打分（30/90日动量+EMA结构+RSI） | Numerai/arXiv | 4 | 4 | 5 | 本次已落地 |
| 8 | 因子中性化/去相关后再集成 | Numerai | 3 | 3 | 4 | 下一轮实现 |
| 9 | 波动目标仓位（vol targeting） | arXiv 1904.04912 | 4 | 4 | 5 | 下一轮可加强 |
|10 | 交易成本/换手惩罚项 | arXiv/FMZ | 4 | 4 | 5 | 当前回测未显式建模 |
|11 | 时间分层（日线择时 + 4h 执行） | QuantConnect | 5 | 5 | 5 | 本项目已采用 |
|12 | 模型集成（规则 + ML 概率输出） | Numerai/Kaggle | 3 | 3 | 4 | 需要数据管线升级 |

---

## 2) 候选策略评估表（筛选 >=3 个）

| 候选策略 | 是否落地 | 入场规则（公式化） | 出场规则（公式化） | 仓位与风控 |
|---|---|---|---|---|
| `btc_trend_breakout_v5` | ✅ | `trend_ok && slope_ok && breakout/highest_n && vol_ratio>=1.0` | `TP1=1R(40%)`、`TP2=2R(30%)`、其余移动止损/超时 | `risk_per_trade=0.8%`, daily 1.5%, weekly 4%, 连亏暂停保留 |
| `btc_mean_reversion_v5` | ✅ | `prev_close<BB_lower && close>BB_lower && 30<RSI<48 && close<BB_mid` | 同统一 replay 规则（分批止盈+ATR/时间止损） | 同上 |
| `btc_momentum_factor_v5` | ✅ | 因子分数 >=4：`mom30`,`mom90`,`EMA结构`,`RSI`,`vol` | 同统一 replay 规则 | 同上 |

---

## 3) 回测对比总表（360 天，10,000 USDT，BTC/USDT 现货）

- Buy&Hold（同窗口）: `net=-1862.78`, `maxDD=7513.36`, `sharpe=-0.38`

| 策略/基线 | 交易次数 | 胜率 | PF | 净收益(USDT) | 最大回撤(USDT) | Sharpe | Calmar | Excess Return vs B&H |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| v4（当前） | 23 | 56.52% | 1.4455 | 935.05 | 658.75 | 1.4435 | 1.4194 | 2797.83 |
| v5_trend_breakout | 23 | 65.22% | 2.4205 | 669.37 | 190.21 | 2.4160 | 3.5192 | 2532.15 |
| v5_mean_reversion | 23 | 60.87% | 2.0489 | **1043.47** | 470.51 | 1.3534 | 2.2177 | **2906.25** |
| v5_momentum_factor | 38 | 47.37% | 1.1942 | 399.71 | 557.11 | 1.3210 | 0.7175 | 2262.49 |

> 月度收益已写入各 JSON 报告的 `monthly_returns` 字段。

---

## 4) 推荐保留/淘汰清单

### 推荐保留
1. **`v5_mean_reversion`**：当前净收益最高，PF>2，胜率与回撤均可接受。
2. **`v5_trend_breakout`**：净收益次优但回撤最小，风险收益比优秀，适合作为稳健腿。
3. **`v4`**：作为既有基线继续保留，用于回归测试。

### 建议淘汰/降权
- `v5_momentum_factor`：交易频率偏高、PF 较低，当前优势不明显，建议降权或重构因子阈值。

---

## 5) 下一轮优化建议（最多 5 条）

1. **引入因子去相关（Numerai 思路）**：对动量/波动/量能因子做相关性裁剪，减少冗余信号。
2. **波动目标仓位（vol targeting）**：按 rolling ATR/std 动态调整风险敞口，提升跨波动 regime 稳定性。
3. **成本与滑点建模**：将手续费+滑点参数化写入 replay，避免高频策略纸面收益虚高。
4. **walk-forward 验证**：滚动训练/验证窗口，替代单窗口评估，降低过拟合风险。
5. **组合层实现（双策略并行）**：`trend_breakout + mean_reversion` 做资金分配（如 60/40），检验组合夏普。

---

## 回测产物路径

- `strategies/spot_btc_conservative_v1/reports/replay_v4_10k_360d_20260227_093844.json`
- `strategies/spot_btc_conservative_v1/reports/replay_v5_trend_breakout_10k_360d_20260227_093857.json`
- `strategies/spot_btc_conservative_v1/reports/replay_v5_mean_reversion_10k_360d_20260227_093900.json`
- `strategies/spot_btc_conservative_v1/reports/replay_v5_momentum_factor_10k_360d_20260227_093903.json`
