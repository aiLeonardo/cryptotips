# 币安期权 API（Go）

官方文档：https://binance-docs.github.io/apidocs/voptions/en/

> go-binance 目前未内置期权模块，使用 `net/http` 直接调用 REST API。

合约 symbol 格式：`{标的}-{到期日YYMMDD}-{行权价}-{C/P}`  
示例：`BTC-241227-100000-C`（BTC 2024年12月27日到期，行权价100000，看涨）

Base URL：`https://eapi.binance.com`

---

## 基础客户端

```go
// pkg/binance/options.go
package binance

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "net/url"
)

const optionsBaseURL = "https://eapi.binance.com"

type OptionsClient struct {
    httpClient *http.Client
    apiKey     string
    apiSecret  string
}

func NewOptionsClient(apiKey, apiSecret string) *OptionsClient {
    return &OptionsClient{
        httpClient: &http.Client{},
        apiKey:     apiKey,
        apiSecret:  apiSecret,
    }
}

func (c *OptionsClient) publicGet(path string, params url.Values) ([]byte, error) {
    u := optionsBaseURL + path
    if len(params) > 0 {
        u += "?" + params.Encode()
    }
    resp, err := c.httpClient.Get(u)
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
```

---

## 期权行情

```go
// 交易品种信息（获取所有合约列表）
body, err := client.publicGet("/eapi/v1/exchangeInfo", nil)

// 返回结构参考
type ExchangeInfo struct {
    OptionSymbols []struct {
        Symbol      string `json:"symbol"`
        Underlying  string `json:"underlying"`  // "BTCUSDT"
        Side        string `json:"side"`         // "CALL" / "PUT"
        StrikePrice string `json:"strikePrice"`
        ExpiryDate  int64  `json:"expiryDate"`  // ms 时间戳
    } `json:"optionSymbols"`
}

// 单个期权行情
params := url.Values{"symbol": {"BTC-241227-100000-C"}}
body, err = client.publicGet("/eapi/v1/ticker", params)

// 期权深度
params = url.Values{"symbol": {"BTC-241227-100000-C"}, "limit": {"10"}}
body, err = client.publicGet("/eapi/v1/depth", params)

// K 线
params = url.Values{
    "symbol":   {"BTC-241227-100000-C"},
    "interval": {"1h"},
    "limit":    {"100"},
}
body, err = client.publicGet("/eapi/v1/klines", params)

// 历史成交
params = url.Values{"symbol": {"BTC-241227-100000-C"}, "limit": {"100"}}
body, err = client.publicGet("/eapi/v1/trades", params)
```

---

## 获取期权链

```go
type OptionSymbol struct {
    Symbol      string `json:"symbol"`
    Underlying  string `json:"underlying"`
    Side        string `json:"side"`
    StrikePrice string `json:"strikePrice"`
    ExpiryDate  int64  `json:"expiryDate"`
}

func GetOptionChain(client *OptionsClient, underlying string) ([]OptionSymbol, error) {
    body, err := client.publicGet("/eapi/v1/exchangeInfo", nil)
    if err != nil {
        return nil, err
    }

    var info struct {
        OptionSymbols []OptionSymbol `json:"optionSymbols"`
    }
    if err := json.Unmarshal(body, &info); err != nil {
        return nil, err
    }

    var result []OptionSymbol
    for _, s := range info.OptionSymbols {
        if s.Underlying == underlying {
            result = append(result, s)
        }
    }
    return result, nil
}

// 使用
chain, err := GetOptionChain(client, "BTCUSDT")
// 按到期日分组
byExpiry := make(map[int64][]OptionSymbol)
for _, s := range chain {
    byExpiry[s.ExpiryDate] = append(byExpiry[s.ExpiryDate], s)
}
```

---

## 账户数据（需签名）

期权账户接口需要 HMAC 签名，使用主 SKILL.md 中的 `SignedGET` 函数：

```go
// 账户信息
body, err := SignedGET(optionsBaseURL, "/eapi/v1/account", apiKey, apiSecret, url.Values{})

// 持仓
body, err = SignedGET(optionsBaseURL, "/eapi/v1/position", apiKey, apiSecret, url.Values{})

// 成交历史
params := url.Values{"symbol": {"BTC-241227-100000-C"}, "limit": {"100"}}
body, err = SignedGET(optionsBaseURL, "/eapi/v1/userTrades", apiKey, apiSecret, params)
```
