# spot_btc_conservative_v1

首个保守 BTC 现货策略（paper only）。

## 逻辑
- 交易对：`BTCUSDT`（spot）
- 趋势过滤：`1d EMA200` + `4h EMA200`
- 入场：`4h EMA20` 回踩并重新站上确认
- 风险：总资金 10000 USDT，单笔风险 0.8%（80 USDT），日损上限 1.5%（150 USDT），周损上限 4.0%（400 USDT）
- 出场：1R 止盈 40%，2R 止盈 30%，剩余 30% 使用跟踪止损（简化版）

## 模式
- 当前仅支持 `paper`，不会调用真实下单接口。

## 运行示例
```bash
./cryptotips strategy run --id spot_btc_conservative_v1 --mode paper --daemon=false
```

## 状态查看
```bash
./cryptotips strategy status --id spot_btc_conservative_v1 --mode paper
```
