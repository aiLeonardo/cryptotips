---
name: binance-api
description: 从币安（Binance）API拉取和处理加密货币市场数据的完整指南，项目语言为 Go（Golang）。当用户需要获取币安行情数据、K线数据、账户信息、订单簿、交易历史、资金费率、持仓数据等任何与币安相关的数据拉取任务时，必须使用此skill。涵盖现货（Spot）、合约（Futures USDT-M / COIN-M）和期权市场。无论用户说"拉数据"、"获取行情"、"查K线"、"binance api"还是"从币安获取XXX"，都应触发此skill。所有代码均为 Go 实现。
---

# Binance API 数据拉取 Skill（Golang）

## 快速决策树

```
需要币安数据？
├── 公开市场数据（行情/K线/深度）→ 无需 API Key，直接调用 REST
├── 账户/订单/持仓数据           → 需要 API Key + Secret（HMAC-SHA256 签名）
└── 实时推送数据                 → WebSocket → 见 references/websocket.md
```

---

## 1. SDK

**使用 `go-binance`**（现货 + 合约 + WebSocket 全覆盖，社区最主流）：

```bash
go get github.com/adshao/go-binance/v2
```

仓库：https://github.com/adshao/go-binance

---

## 2. 基础配置

```go
// internal/binance/client.go
package binance

import (
    "os"

    gobinance "github.com/adshao/go-binance/v2"
    "github.com/adshao/go-binance/v2/futures"
)

// NewSpotClient 创建现货客户端
func NewSpotClient() *gobinance.Client {
    return gobinance.NewClient(
        os.Getenv("BINANCE_API_KEY"),
        os.Getenv("BINANCE_API_SECRET"),
    )
}

// NewFuturesClient 创建 USDT-M 合约客户端
func NewFuturesClient() *futures.Client {
    return gobinance.NewFuturesClient(
        os.Getenv("BINANCE_API_KEY"),
        os.Getenv("BINANCE_API_SECRET"),
    )
}

// 切换测试网（在创建 client 前调用）
// gobinance.UseTestnet = true
```

Base URL 常量（SDK 已内置，仅供参考）：

```
现货:       https://api.binance.com
USDT-M合约: https://fapi.binance.com
COIN-M合约: https://dapi.binance.com
现货测试网:  https://testnet.binance.vision
合约测试网:  https://testnet.binancefuture.com
```

---

## 3. 现货市场数据

### 行情 / Ticker

```go
client := gobinance.NewClient("", "") // 公开数据无需 key

// 单个交易对价格
prices, err := client.NewListPricesService().
    Symbol("BTCUSDT").
    Do(context.Background())
// prices[0].Symbol, prices[0].Price

// 所有交易对价格
allPrices, err := client.NewListPricesService().Do(context.Background())

// 24h 涨跌统计
ticker, err := client.NewListPriceChangeStatsService().
    Symbol("BTCUSDT").
    Do(context.Background())
// ticker[0].PriceChangePercent, ticker[0].Volume, ticker[0].LastPrice

// 最优买卖报价
bookTicker, err := client.NewListBookTickersService().
    Symbol("BTCUSDT").
    Do(context.Background())
// bookTicker[0].BidPrice, bookTicker[0].AskPrice
```

### K 线

```go
// interval: 1m 3m 5m 15m 30m 1h 2h 4h 6h 8h 12h 1d 3d 1w 1M
klines, err := client.NewKlinesService().
    Symbol("BTCUSDT").
    Interval("1h").
    Limit(500). // 最大 1000
    Do(context.Background())

for _, k := range klines {
    // k.OpenTime(ms), k.Open, k.High, k.Low, k.Close
    // k.Volume, k.CloseTime(ms), k.QuoteAssetVolume, k.TradeNum
    // k.TakerBuyBaseAssetVolume, k.TakerBuyQuoteAssetVolume
}

// 指定时间范围
start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
end   := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC).UnixMilli()

klines, err = client.NewKlinesService().
    Symbol("BTCUSDT").
    Interval("4h").
    StartTime(start).
    EndTime(end).
    Limit(1000).
    Do(context.Background())
```

### 订单簿

```go
// limit: 5 10 20 50 100 500 1000 5000（weight 随 limit 增大）
orderBook, err := client.NewDepthService().
    Symbol("BTCUSDT").
    Limit(20).
    Do(context.Background())

// orderBook.Bids[0].Price, orderBook.Bids[0].Quantity
// orderBook.Asks[0].Price, orderBook.Asks[0].Quantity
```

### 成交记录

```go
// 最近成交
trades, err := client.NewRecentTradesService().
    Symbol("BTCUSDT").
    Limit(100).
    Do(context.Background())

// 聚合成交（推荐，同价位合并）
aggTrades, err := client.NewAggTradesService().
    Symbol("BTCUSDT").
    Limit(500).
    Do(context.Background())
```

---

## 4. 现货账户数据（需签名）

```go
client := gobinance.NewClient(apiKey, apiSecret)

// 账户余额
account, err := client.NewGetAccountService().Do(context.Background())
for _, b := range account.Balances {
    if b.Free != "0" || b.Locked != "0" {
        fmt.Printf("%s  free=%s  locked=%s\n", b.Asset, b.Free, b.Locked)
    }
}

// 当前挂单
openOrders, err := client.NewListOpenOrdersService().
    Symbol("BTCUSDT").
    Do(context.Background())

// 历史订单
orders, err := client.NewListOrdersService().
    Symbol("BTCUSDT").
    Limit(100).
    Do(context.Background())

// 账户成交历史
myTrades, err := client.NewListTradesService().
    Symbol("BTCUSDT").
    Limit(500).
    Do(context.Background())
```

---

## 5. 合约市场（USDT-M Futures）

### 公开数据

```go
import "github.com/adshao/go-binance/v2/futures"

client := gobinance.NewFuturesClient("", "")

// K 线（用法同现货）
klines, err := client.NewKlinesService().
    Symbol("BTCUSDT").Interval("1h").Limit(500).
    Do(context.Background())

// 标记价格 + 当前资金费率
markPrice, err := client.NewGetMarkPriceService().
    Symbol("BTCUSDT").
    Do(context.Background())
// markPrice.MarkPrice, markPrice.LastFundingRate, markPrice.NextFundingTime

// 资金费率历史
fundingRates, err := client.NewGetFundingRateService().
    Symbol("BTCUSDT").
    Limit(100).
    Do(context.Background())
// f.FundingTime(ms), f.FundingRate

// 未平仓合约量（当前）
oi, err := client.NewGetOpenInterestService().
    Symbol("BTCUSDT").
    Do(context.Background())

// 未平仓合约历史（period: 5m/15m/30m/1h/2h/4h/6h/12h/1d）
oiHist, err := client.NewGetOpenInterestStatisticsService().
    Symbol("BTCUSDT").
    Period("1h").
    Limit(200).
    Do(context.Background())

// 多空持仓比（大户账户）
lsRatio, err := client.NewGetTopLongShortAccountRatioService().
    Symbol("BTCUSDT").
    Period("1h").
    Limit(30).
    Do(context.Background())

// 合约深度
depth, err := client.NewDepthService().
    Symbol("BTCUSDT").
    Limit(20).
    Do(context.Background())
```

### 合约账户数据

```go
authClient := gobinance.NewFuturesClient(apiKey, apiSecret)

// 账户总览
account, err := authClient.NewGetAccountService().Do(context.Background())
// account.TotalWalletBalance, account.AvailableBalance, account.TotalUnrealizedProfit

// 当前持仓
for _, p := range account.Positions {
    if p.PositionAmt != "0" {
        fmt.Printf("%s  amt=%s  unrealizedPnl=%s  entryPrice=%s\n",
            p.Symbol, p.PositionAmt, p.UnrealizedProfit, p.EntryPrice)
    }
}

// 账户余额（逐资产）
balances, err := authClient.NewGetBalanceService().Do(context.Background())

// 收益历史（REALIZED_PNL / FUNDING_FEE / TRANSFER / COMMISSION 等）
income, err := authClient.NewGetIncomeHistoryService().
    IncomeType("REALIZED_PNL").
    Limit(100).
    Do(context.Background())

// 历史成交
userTrades, err := authClient.NewListAccountTradeService().
    Symbol("BTCUSDT").
    Limit(100).
    Do(context.Background())
```

---

## 6. 批量拉取历史 K 线（自动分页）

单次最多 1000 根，超出需循环分页。

```go
// pkg/binance/klines.go
package binance

import (
    "context"
    "time"

    gobinance "github.com/adshao/go-binance/v2"
    "github.com/adshao/go-binance/v2/futures"
)

// FetchAllKlines 现货：拉取完整时间段 K 线，自动分页
func FetchAllKlines(
    client *gobinance.Client,
    symbol, interval string,
    startTime, endTime time.Time,
) ([]*gobinance.Kline, error) {
    var result []*gobinance.Kline
    cursor := startTime.UnixMilli()
    endMs  := endTime.UnixMilli()

    for cursor < endMs {
        batch, err := client.NewKlinesService().
            Symbol(symbol).
            Interval(interval).
            StartTime(cursor).
            EndTime(endMs).
            Limit(1000).
            Do(context.Background())
        if err != nil {
            return nil, err
        }
        if len(batch) == 0 {
            break
        }
        result = append(result, batch...)
        cursor = batch[len(batch)-1].CloseTime + 1
        time.Sleep(100 * time.Millisecond) // 避免触发限速
    }
    return result, nil
}

// FetchAllFuturesKlines 合约：拉取完整时间段 K 线，自动分页
func FetchAllFuturesKlines(
    client *futures.Client,
    symbol, interval string,
    startTime, endTime time.Time,
) ([]*futures.Kline, error) {
    var result []*futures.Kline
    cursor := startTime.UnixMilli()
    endMs  := endTime.UnixMilli()

    for cursor < endMs {
        batch, err := client.NewKlinesService().
            Symbol(symbol).
            Interval(interval).
            StartTime(cursor).
            EndTime(endMs).
            Limit(1000).
            Do(context.Background())
        if err != nil {
            return nil, err
        }
        if len(batch) == 0 {
            break
        }
        result = append(result, batch...)
        cursor = batch[len(batch)-1].CloseTime + 1
        time.Sleep(100 * time.Millisecond)
    }
    return result, nil
}
```

使用：

```go
client := gobinance.NewClient("", "")
start  := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
end    := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

klines, err := binance.FetchAllKlines(client, "BTCUSDT", "1h", start, end)
fmt.Printf("共拉取 %d 根K线\n", len(klines))
```

---

## 7. 签名请求（直接 net/http，不用 SDK）

适用于 SDK 暂未封装的接口。

```go
// pkg/binance/request.go
package binance

import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "strconv"
    "time"
)

func sign(payload, secret string) string {
    h := hmac.New(sha256.New, []byte(secret))
    h.Write([]byte(payload))
    return hex.EncodeToString(h.Sum(nil))
}

// SignedGET 发送带签名的 GET 请求
func SignedGET(baseURL, path, apiKey, apiSecret string, params url.Values) ([]byte, error) {
    params.Set("timestamp",  strconv.FormatInt(time.Now().UnixMilli(), 10))
    params.Set("recvWindow", "5000")

    query := params.Encode()
    params.Set("signature", sign(query, apiSecret))

    req, err := http.NewRequest("GET", baseURL+path+"?"+params.Encode(), nil)
    if err != nil {
        return nil, err
    }
    req.Header.Set("X-MBX-APIKEY", apiKey)

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    body, _ := io.ReadAll(resp.Body)
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, body)
    }
    return body, nil
}

// 使用示例
// body, err := SignedGET("https://fapi.binance.com", "/fapi/v2/balance",
//     apiKey, apiSecret, url.Values{})
```

---

## 8. 速率限制 + 重试

```go
// pkg/binance/ratelimit.go
package binance

import (
    "fmt"
    "log"
    "strings"
    "sync"
    "time"
)

// 币安限制（现货）：1200 weight/minute
// /api/v3/klines          weight=2
// /api/v3/depth limit=100 weight=5
// /api/v3/depth limit=5000 weight=250
// 保守策略：每秒 ≤ 10 个请求

// RateLimiter 简单令牌桶
type RateLimiter struct {
    mu     sync.Mutex
    tokens int
    max    int
}

func NewRateLimiter(perSecond int) *RateLimiter {
    rl := &RateLimiter{tokens: perSecond, max: perSecond}
    go func() {
        for range time.Tick(time.Second) {
            rl.mu.Lock()
            rl.tokens = rl.max
            rl.mu.Unlock()
        }
    }()
    return rl
}

func (rl *RateLimiter) Wait() {
    for {
        rl.mu.Lock()
        if rl.tokens > 0 {
            rl.tokens--
            rl.mu.Unlock()
            return
        }
        rl.mu.Unlock()
        time.Sleep(10 * time.Millisecond)
    }
}

// WithRetry 遇到 429/418 时指数退避重试
func WithRetry(maxRetries int, fn func() error) error {
    for i := range maxRetries {
        err := fn()
        if err == nil {
            return nil
        }
        if strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "418") {
            wait := time.Duration(1<<i) * 10 * time.Second // 10s → 20s → 40s
            log.Printf("触发限速，等待 %v 后重试 (%d/%d)...", wait, i+1, maxRetries)
            time.Sleep(wait)
            continue
        }
        return err
    }
    return fmt.Errorf("超出最大重试次数 %d", maxRetries)
}
```

---

## 9. 数据落地

```go
// CSV 落地（轻量场景）
package binance

import (
    "encoding/csv"
    "os"
    "strconv"

    gobinance "github.com/adshao/go-binance/v2"
)

func SaveKlinesToCSV(klines []*gobinance.Kline, filename string) error {
    f, err := os.Create(filename)
    if err != nil {
        return err
    }
    defer f.Close()

    w := csv.NewWriter(f)
    defer w.Flush()

    w.Write([]string{
        "open_time", "open", "high", "low", "close",
        "volume", "close_time", "quote_volume", "trades",
        "taker_buy_base", "taker_buy_quote",
    })
    for _, k := range klines {
        w.Write([]string{
            strconv.FormatInt(k.OpenTime, 10),
            k.Open, k.High, k.Low, k.Close, k.Volume,
            strconv.FormatInt(k.CloseTime, 10),
            k.QuoteAssetVolume,
            strconv.FormatInt(k.TradeNum, 10),
            k.TakerBuyBaseAssetVolume,
            k.TakerBuyQuoteAssetVolume,
        })
    }
    return nil
}
```

大量历史数据推荐直接从 https://data.binance.vision 下载 zip 压缩包，比 API 拉取快100倍。

---

## 10. 注意事项

| 事项 | 说明 |
|------|------|
| API Key 安全 | 存环境变量，不要写死在代码里 |
| 时间同步 | 本机时间误差须 < 1000ms，否则签名失败；`recvWindow` 最大 60000 |
| 合约精度 | 价格/数量须符合 `exchangeInfo` 的 `tickSize`/`stepSize`，用 `shopspring/decimal` 库处理浮点 |
| 测试网切换 | `gobinance.UseTestnet = true`（在创建 client **之前**设置） |
| 时间戳 | 所有时间戳均为 UTC 毫秒，Go 中用 `time.UnixMilli(ts)` 转换 |
| 错误响应 | SDK 会将 HTTP 4xx/5xx 包装为 error，检查 `*common.APIError` 类型获取 code/msg |

---

## 参考文档

- **官方 REST API（现货）**: https://binance-docs.github.io/apidocs/spot/en/
- **官方 REST API（合约）**: https://binance-docs.github.io/apidocs/futures/en/
- **go-binance SDK**: https://github.com/adshao/go-binance
- **历史数据下载**: https://data.binance.vision
- **测试网**: https://testnet.binance.vision

需要 WebSocket 实时数据？→ 查看 `references/websocket.md`  
需要期权数据？→ 查看 `references/options.md`
