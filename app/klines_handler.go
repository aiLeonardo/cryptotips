package app

import (
	"fmt"
	"strconv"
	"time"

	"github.com/aiLeonardo/cryptotips/indicator"
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
type ReversalSignalItem struct {
	Time  int64   `json:"time"`
	Type  string  `json:"type"` // top / bottom
	Score float64 `json:"score"`
}

type RegimeStartpointItem struct {
	Time       int64   `json:"time"`
	State      string  `json:"state"` // BULL / BEAR / RANGE
	Confidence float64 `json:"confidence"`
}

type KLinesResp struct {
	Symbol            string                 `json:"symbol"`
	Interval          string                 `json:"interval"`
	KLines            []KLineItem            `json:"klines"`
	QuoteVolumeLog    []float64              `json:"quoteVolumeLog,omitempty"`
	QuoteVolumeEma    []float64              `json:"quoteVolumeLogEma,omitempty"`
	QuoteVolumeZ      []float64              `json:"quoteVolumeZ,omitempty"`
	ReversalSignals   []ReversalSignalItem   `json:"reversalSignals,omitempty"`
	RegimeStartpoints []RegimeStartpointItem `json:"regimeStartpoints,omitempty"`
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
	quoteVolumes := make([]float64, 0, len(records))
	closes := make([]float64, 0, len(records))
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
		quoteVolumes = append(quoteVolumes, r.QuoteVolume)
		closes = append(closes, r.Close)
	}

	logQ, emaLogQ, z := indicator.QuoteVolumeZScore(quoteVolumes, 20)
	signals := detectReversalSignals(items, closes, z, 60, 2.0)

	regimeMarkers := make([]RegimeStartpointItem, 0)
	if len(records) > 0 {
		rangeStart := records[0].OpenTime
		rangeEnd := records[len(records)-1].OpenTime
		realtimeMarkers, rerr := a.getRealtimeRegimeStartpoints(symbol, rangeStart, rangeEnd)
		if rerr != nil {
			a.logger.Warnf("实时计算 regime 起点失败，回退 DB 结果: %v", rerr)
			rm := &models.RegimeStartpointRecord{}
			regimes, derr := rm.ListBySymbolAndRange(a.db, symbol, rangeStart, rangeEnd)
			if derr != nil {
				a.logger.Warnf("回退查询 regime 起点失败: %v", derr)
			} else {
				for _, r := range regimes {
					regimeMarkers = append(regimeMarkers, RegimeStartpointItem{
						Time:       r.StartTime.Unix(),
						State:      r.State,
						Confidence: r.Confidence,
					})
				}
			}
		} else {
			regimeMarkers = realtimeMarkers
		}
	}

	lib.JsonResponse(c, KLinesResp{
		Symbol:            symbol,
		Interval:          interval,
		KLines:            items,
		QuoteVolumeLog:    logQ,
		QuoteVolumeEma:    emaLogQ,
		QuoteVolumeZ:      z,
		ReversalSignals:   signals,
		RegimeStartpoints: regimeMarkers,
	})
}

// detectReversalSignals 基于成交额异常(z-score) + 价格相对区间位置，给出顶部/底部候选点。
func detectReversalSignals(items []KLineItem, closes []float64, z []float64, window int, zThreshold float64) []ReversalSignalItem {
	if len(items) == 0 || len(closes) != len(items) || len(z) != len(items) {
		return nil
	}
	if window <= 1 {
		window = 60
	}

	signals := make([]ReversalSignalItem, 0)
	for i := range closes {
		if i+1 < window || z[i] < zThreshold {
			continue
		}

		start := i + 1 - window
		minClose, maxClose := closes[start], closes[start]
		for j := start + 1; j <= i; j++ {
			if closes[j] < minClose {
				minClose = closes[j]
			}
			if closes[j] > maxClose {
				maxClose = closes[j]
			}
		}

		if maxClose <= 0 || minClose <= 0 {
			continue
		}

		nearTop := closes[i] >= maxClose*0.98
		nearBottom := closes[i] <= minClose*1.02

		score := z[i]
		if nearTop {
			signals = append(signals, ReversalSignalItem{Time: items[i].Time, Type: "top", Score: score})
		}
		if nearBottom {
			signals = append(signals, ReversalSignalItem{Time: items[i].Time, Type: "bottom", Score: score})
		}
	}
	return signals
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
