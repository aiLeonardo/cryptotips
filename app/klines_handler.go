package app

import (
	"fmt"
	"strconv"
	"time"

	"github.com/aiLeonardo/cryptotips/lib"
	"github.com/aiLeonardo/cryptotips/models"

	"github.com/gin-gonic/gin"
)

// KLineItem 返回给前端的单根 K 线（TradingView Lightweight Charts 格式）
// time 使用 Unix 秒，与 TradingView 保持一致
type KLineItem struct {
	Time        int64   `json:"time"`
	Open        float64 `json:"open"`
	High        float64 `json:"high"`
	Low         float64 `json:"low"`
	Close       float64 `json:"close"`
	Volume      float64 `json:"volume"`
	QuoteVolume float64 `json:"quoteVolume"`
}

// KLinesResp 响应体
type KLinesResp struct {
	Symbol   string      `json:"symbol"`
	Interval string      `json:"interval"`
	KLines   []KLineItem `json:"klines"`
}

// KLinesMeta 可用的 symbol/interval 组合（用于前端下拉框）
type KLinesMeta struct {
	Symbols   []string `json:"symbols"`
	Intervals []string `json:"intervals"`
}

// getKLines 返回 K 线数据
// GET /api/klines?symbol=BTCUSDT&interval=1d&start=1609459200000&end=1640995200000&limit=500
func (a *goapi) getKLines(c *gin.Context) {
	symbol := c.Query("symbol")
	interval := c.Query("interval")
	if symbol == "" || interval == "" {
		lib.JsonError(c, fmt.Errorf("symbol 和 interval 为必填参数"))
		return
	}

	// 可选：起止时间（Unix 毫秒）
	var startTime, endTime time.Time
	if s := c.Query("start"); s != "" {
		ms, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			lib.JsonError(c, fmt.Errorf("start 格式错误，应为 Unix 毫秒时间戳"))
			return
		}
		startTime = time.UnixMilli(ms).UTC()
	}
	if e := c.Query("end"); e != "" {
		ms, err := strconv.ParseInt(e, 10, 64)
		if err != nil {
			lib.JsonError(c, fmt.Errorf("end 格式错误，应为 Unix 毫秒时间戳"))
			return
		}
		endTime = time.UnixMilli(ms).UTC()
	}

	// 可选：limit（默认 3000，最大 5000）
	limit := 3000
	if l := c.Query("limit"); l != "" {
		n, err := strconv.Atoi(l)
		if err != nil || n <= 0 {
			lib.JsonError(c, fmt.Errorf("limit 格式错误"))
			return
		}
		if n > 5000 {
			n = 5000
		}
		limit = n
	}

	m := &models.KLineRecord{}
	records, err := m.GetKLines(a.db, symbol, interval, startTime, endTime)
	if err != nil {
		lib.JsonError(c, fmt.Errorf("查询 K 线失败: %w", err))
		return
	}

	// 若结果超过 limit，取最新的 limit 条
	if len(records) > limit {
		records = records[len(records)-limit:]
	}

	items := make([]KLineItem, 0, len(records))
	for _, r := range records {
		items = append(items, KLineItem{
			Time:        r.OpenTime.Unix(), // TradingView 需要 Unix 秒
			Open:        r.Open,
			High:        r.High,
			Low:         r.Low,
			Close:       r.Close,
			Volume:      r.Volume,
			QuoteVolume: r.QuoteVolume,
		})
	}

	lib.JsonResponse(c, KLinesResp{
		Symbol:   symbol,
		Interval: interval,
		KLines:   items,
	})
}

// getKLinesMeta 返回数据库中已有的 symbol 和 interval 列表
// 数据来自 crypto_kline_meta 轻量元数据表，O(1) 查询
// GET /api/klines/meta
func (a *goapi) getKLinesMeta(c *gin.Context) {
	m := &models.KLineMetaRecord{}
	rows, err := m.ListAll(a.db)
	if err != nil {
		lib.JsonError(c, fmt.Errorf("查询元数据失败: %w", err))
		return
	}

	symbolSet := make(map[string]struct{}, len(rows))
	intervalSet := make(map[string]struct{}, len(rows))
	for _, r := range rows {
		symbolSet[r.Symbol] = struct{}{}
		intervalSet[r.Interval] = struct{}{}
	}

	symbols := make([]string, 0, len(symbolSet))
	for s := range symbolSet {
		symbols = append(symbols, s)
	}
	intervals := make([]string, 0, len(intervalSet))
	for i := range intervalSet {
		intervals = append(intervals, i)
	}

	lib.JsonResponse(c, KLinesMeta{
		Symbols:   symbols,
		Intervals: intervals,
	})
}
