# btc_momentum_factor_v5

- 类型：多因子动量打分（长仓，BTC/USDT 现货）
- 信号：30/90日动量 + EMA结构 + RSI + 成交量（>=4分入场）
- 入场：`v5_momentum_factor` profile
- 风控：risk_per_trade=0.8%，daily=1.5%，weekly=4%，连亏暂停由 replay v4 风控逻辑复用

## 运行

```bash
./cryptotips strategy replay --days 360 --profile v5_momentum_factor
```
