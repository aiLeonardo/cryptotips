# TraderChenge 因子评估报告 v1

- 样本量: 70 (24h可评估: 59, 72h可评估: 57)

## 24h 目标
| 因子 | IC(Spearman) | 分层收益(Q5-Q1) | 稳定性 | N |
|---|---:|---:|---:|---:|
| SRCluster | 0.7256 | 0.0448 | 1.6529 | 59 |
| BreakRetestScore | 0.2284 | -0.0075 | nan | 59 |
| PatternPressure | 0.1551 | 0.0163 | 1.8605 | 59 |
| MomentumExhaustion | 0.8648 | 0.0548 | 2.6321 | 59 |
| VolConfirmScore | 0.4206 | 0.0092 | -1.9394 | 59 |
| InvalidationDistance | 0.2281 | 0.0192 | -0.4958 | 56 |
| NarrativeConsistency | -0.3086 | -0.0206 | -2.3156 | 59 |

## 72h 目标
| 因子 | IC(Spearman) | 分层收益(Q5-Q1) | 稳定性 | N |
|---|---:|---:|---:|---:|
| SRCluster | -0.6331 | -0.0614 | 0.2030 | 57 |
| BreakRetestScore | -0.2993 | 0.0167 | nan | 57 |
| PatternPressure | -0.1577 | -0.0225 | -0.4202 | 57 |
| MomentumExhaustion | -0.5858 | -0.0422 | 0.2087 | 57 |
| VolConfirmScore | -0.7192 | -0.0597 | -0.2098 | 57 |
| InvalidationDistance | -0.3229 | -0.0381 | 1.6226 | 54 |
| NarrativeConsistency | 0.3160 | 0.0275 | -0.0669 | 57 |

## 筛选建议
- 优先保留绝对IC与分层收益同时为正且跨周期稳定性的因子。
- InvalidationDistance 适合作为风险约束因子，不单独做alpha。
- NarrativeConsistency 可与 BreakRetestScore 组合形成“叙事-结构共振”信号。