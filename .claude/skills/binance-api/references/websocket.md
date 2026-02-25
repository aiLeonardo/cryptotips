# Binance WebSocket 实时数据（Go）

官方文档：https://binance-docs.github.io/apidocs/spot/en/#websocket-market-streams

go-binance 内置 WebSocket 支持，无需额外安装。

---

## 现货 WebSocket

```go
import gobinance "github.com/adshao/go-binance/v2"

// --- K 线实时推送 ---
// wsKlineHandler 每根新 K 线tick都会触发，k.Kline.IsFinal=true 表示该K线已收盘
wsKlineHandler := func(event *gobinance.WsKlineEvent) {
    k := event.Kline
    if k.IsFinal {
        fmt.Printf("[收盘] %s %s  O=%s H=%s L=%s C=%s V=%s\n",
            event.Symbol, k.Interval, k.Open, k.High, k.Low, k.Close, k.Volume)
    }
}
errHandler := func(err error) { log.Println("WS error:", err) }

doneC, stopC, err := gobinance.WsKlineServe("BTCUSDT", "1m", wsKlineHandler, errHandler)
if err != nil {
    log.Fatal(err)
}
// 停止：close(stopC)
// 等待退出：<-doneC

// --- 实时成交（聚合）---
gobinance.WsAggTradeServe("BTCUSDT", func(event *gobinance.WsAggTradeEvent) {
    fmt.Printf("成交价: %s  数量: %s  方向: isBuyerMaker=%v\n",
        event.Price, event.Quantity, event.IsBuyerMaker)
}, errHandler)

// --- 订单簿（差量推送）---
gobinance.WsDiffDepthServe("BTCUSDT", func(event *gobinance.WsDepthEvent) {
    for _, bid := range event.Bids {
        // bid.Price, bid.Quantity（Quantity="0" 表示删除该档位）
    }
}, errHandler)

// --- 最优买卖报价（BookTicker）---
gobinance.WsBookTickerServe("BTCUSDT", func(event *gobinance.WsBookTickerEvent) {
    fmt.Printf("买一: %s x %s  卖一: %s x %s\n",
        event.BestBidPrice, event.BestBidQty,
        event.BestAskPrice, event.BestAskQty)
}, errHandler)

// --- 24h Ticker ---
gobinance.WsMarketStatServe("BTCUSDT", func(event *gobinance.WsMarketStatEvent) {
    fmt.Printf("涨跌幅: %s%%  成交量: %s\n", event.PriceChangePercent, event.BaseVolume)
}, errHandler)
```

## 合约 WebSocket

```go
import "github.com/adshao/go-binance/v2/futures"

// K 线
futures.WsKlineServe("BTCUSDT", "1m", func(event *futures.WsKlineEvent) {
    k := event.Kline
    if k.IsFinal {
        fmt.Printf("[合约收盘] C=%s V=%s\n", k.Close, k.Volume)
    }
}, errHandler)

// 标记价格（含资金费率）
futures.WsMarkPriceServe("BTCUSDT", func(event *futures.WsMarkPriceEvent) {
    fmt.Printf("标记价: %s  资金费率: %s  下次结算: %d\n",
        event.MarkPrice, event.FundingRate, event.NextFundingTime)
}, errHandler)

// 聚合成交
futures.WsAggTradeServe("BTCUSDT", func(event *futures.WsAggTradeEvent) {
    fmt.Printf("价格: %s  数量: %s\n", event.Price, event.Quantity)
}, errHandler)

// 深度差量
futures.WsDiffDepthServe("BTCUSDT", func(event *futures.WsDepthEvent) {
    // event.Bids, event.Asks
}, errHandler)
```

## 用户数据流（账户/订单实时更新）

```go
// 1. 创建 listenKey
client := gobinance.NewClient(apiKey, apiSecret)
res, err := client.NewStartUserStreamService().Do(context.Background())
listenKey := res.ListenKey

// 2. 每 30 分钟续期（否则 60 分钟后过期）
go func() {
    for range time.Tick(30 * time.Minute) {
        client.NewKeepaliveUserStreamService().
            ListenKey(listenKey).
            Do(context.Background())
    }
}()

// 3. 订阅
doneC, stopC, err := gobinance.WsUserDataServe(listenKey, func(event *gobinance.WsUserDataEvent) {
    switch event.Event {
    case gobinance.UserDataEventTypeOutboundAccountPosition:
        // 账户余额变动：event.AccountUpdate.Balances
    case gobinance.UserDataEventTypeExecutionReport:
        // 订单状态更新：event.OrderUpdate
    }
}, errHandler)

// 合约用户数据流
futuresClient := gobinance.NewFuturesClient(apiKey, apiSecret)
res2, _ := futuresClient.NewStartUserStreamService().Do(context.Background())
futures.WsUserDataServe(res2.ListenKey, func(event *futures.WsUserDataEvent) {
    switch event.Event {
    case futures.UserDataEventTypeAccountUpdate:
        // event.AccountUpdate.Balances, event.AccountUpdate.Positions
    case futures.UserDataEventTypeOrderTradeUpdate:
        // event.OrderTradeUpdate
    }
}, errHandler)
```

## 断线重连封装

go-binance 的 WsServe 系列在连接断开后**不会自动重连**，需要自行封装：

```go
func ServeWithReconnect(
    serve func() (chan struct{}, chan struct{}, error),
    label string,
) {
    for {
        doneC, _, err := serve()
        if err != nil {
            log.Printf("[%s] 连接失败: %v，5秒后重试...", label, err)
            time.Sleep(5 * time.Second)
            continue
        }
        log.Printf("[%s] 已连接", label)
        <-doneC
        log.Printf("[%s] 连接断开，重连中...", label)
        time.Sleep(time.Second)
    }
}

// 使用
go ServeWithReconnect(func() (chan struct{}, chan struct{}, error) {
    return gobinance.WsKlineServe("BTCUSDT", "1m", klineHandler, errHandler)
}, "BTCUSDT-1m-kline")
```
