# btc_trend_breakout_v5

- 类型：趋势突破（长仓，BTC/USDT 现货）
- 信号：日线动量 + 4h 突破确认 + 成交量过滤
- 入场：`v5_trend_breakout` profile
- 风控：risk_per_trade=0.8%，daily=1.5%，weekly=4%，连亏暂停由 replay v4 风控逻辑复用

## 运行

```bash
./cryptotips strategy replay --days 360 --profile v5_trend_breakout
```
