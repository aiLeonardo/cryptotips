# TraderChenge 因子接入建议 v1

## 1) 数据管道建议
- 新增 `market_ext` 数据层：spot/perp klines + 衍生指标，按1h主频汇总。
- 新增 `youtube_event` 事实表：video_id, event_time, transcript_hash, narrative_features。
- 通过 Airflow/Cron 两阶段：T+0 拉行情，T+1 回填24h/72h标签。

## 2) 计算频率
- 市场因子：每小时滚动计算。
- 叙事因子：视频发布/转写完成触发计算；无新视频则沿用最近状态。
- 评估重训：每周一次全量，日更增量监控IC漂移。

## 3) 风险控制
- 因子门控：当 funding/OI 缺失时，降级到纯价格-成交量因子并降低仓位。
- 使用 InvalidationDistance 设定动态止损带：ATR x k。
- 多因子合成前进行相关性去冗余（|rho|>0.7仅保留一项）。

## 4) 与 cryptotips 集成
- 不强耦合现有表结构，建议新增 `factor_signal_v2`（宽表）与 `factor_eval_v2`（评估表）。
- API层新增 `/factors/traderchenge/latest` 与 `/factors/traderchenge/backtest`。
- 交易执行层采用“信号->风险预算->下单”三段式，避免直接因子驱动裸下单。