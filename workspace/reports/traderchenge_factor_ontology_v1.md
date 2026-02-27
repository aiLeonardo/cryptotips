# TraderChenge 因子语义本体 v1

- 样本数量: 70
- 生成时间: 2026-02-27T02:51:26.513991+00:00

| 类别 | 总命中 | 高权重关键词 |
|---|---:|---|
| support_resistance | 1078 | 测试:401, 阻力:263, 支撑:248, 区间:122, 水平位:19, 支阻:12 |
| breakout_retest | 2362 | 跌破:818, 突破:807, 测试:401, 收回:129, 回踩:79, 假突破:66 |
| pattern | 811 | 结构:357, 形态:159, 等距:107, 三角:69, 吞没:69, 中继:36 |
| volume_price | 414 | 波动:179, 横盘:109, 十字星:47, 缩量:27, 放量:25, 下影线:11 |
| risk_management | 1352 | 信号:569, 短线:350, 止跌:236, 风险:49, 止损:48, 胜率:34 |

## 因子映射
- SRCluster: support_resistance, pattern
- BreakRetestScore: breakout_retest, support_resistance
- PatternPressure: pattern
- MomentumExhaustion: volume_price, pattern
- VolConfirmScore: volume_price, breakout_retest
- InvalidationDistance: risk_management, support_resistance
- NarrativeConsistency: risk_management, breakout_retest, pattern