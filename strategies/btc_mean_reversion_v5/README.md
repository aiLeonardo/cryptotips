# btc_mean_reversion_v5

- 类型：布林带+RSI 均值回归（长仓，BTC/USDT 现货）
- 信号：跌破下轨后回收 + RSI 区间过滤
- 入场：`v5_mean_reversion` profile
- 风控：risk_per_trade=0.8%，daily=1.5%，weekly=4%，连亏暂停由 replay v4 风控逻辑复用

## 运行

```bash
./cryptotips strategy replay --days 360 --profile v5_mean_reversion
```
