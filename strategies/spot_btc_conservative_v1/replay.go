package spot_btc_conservative_v1

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/aiLeonardo/cryptotips/indicator"
	"github.com/aiLeonardo/cryptotips/models"
	"gorm.io/gorm"
)

type ReplaySummary struct {
	Profile                string             `json:"profile"`
	TradeCount             int                `json:"trade_count"`
	WinRate                float64            `json:"win_rate"`
	ProfitFactor           float64            `json:"profit_factor"`
	MaxDrawdown            float64            `json:"max_drawdown"`
	NetPnL                 float64            `json:"net_pnl"`
	MonthlyReturns         map[string]float64 `json:"monthly_returns"`
	Sharpe                 float64            `json:"sharpe,omitempty"`
	Calmar                 float64            `json:"calmar,omitempty"`
	CorePnL                float64            `json:"core_pnl,omitempty"`
	TacticalPnL            float64            `json:"tactical_pnl,omitempty"`
	RegimeSwitches         map[string]int     `json:"regime_switches,omitempty"`
	RegimeSwitchTakeProfit float64            `json:"regime_switch_take_profit,omitempty"`
	RegimeTimeframe        string             `json:"regime_timeframe,omitempty"`
	CalibrationReportPath  string             `json:"calibration_report_path,omitempty"`
}

type ReplayBenchmark struct {
	Name           string             `json:"name"`
	InitialCapital float64            `json:"initial_capital"`
	FinalEquity    float64            `json:"final_equity"`
	NetPnL         float64            `json:"net_pnl"`
	MaxDrawdown    float64            `json:"max_drawdown"`
	Sharpe         float64            `json:"sharpe,omitempty"`
	MonthlyReturns map[string]float64 `json:"monthly_returns"`
}

type ReplayReport struct {
	StrategyID             string                   `json:"strategy_id"`
	Symbol                 string                   `json:"symbol"`
	Days                   int                      `json:"days"`
	StartTimeUTC           time.Time                `json:"start_time_utc,omitempty"`
	EndTimeUTC             time.Time                `json:"end_time_utc,omitempty"`
	Profile                string                   `json:"profile"`
	Interval               string                   `json:"interval"`
	CapitalUSDT            float64                  `json:"capital_usdt"`
	RiskPerTradePct        float64                  `json:"risk_per_trade_pct"`
	RiskPerTradeAmt        float64                  `json:"risk_per_trade_usdt"`
	MaxDailyLossPct        float64                  `json:"max_daily_loss_pct"`
	MaxDailyLossAmt        float64                  `json:"max_daily_loss_usdt"`
	MaxWeeklyLossPct       float64                  `json:"max_weekly_loss_pct"`
	MaxWeeklyLossAmt       float64                  `json:"max_weekly_loss_usdt"`
	TradeCount             int                      `json:"trade_count"`
	WinRate                float64                  `json:"win_rate"`
	ProfitFactor           float64                  `json:"profit_factor"`
	MaxDrawdown            float64                  `json:"max_drawdown"`
	NetPnL                 float64                  `json:"net_pnl"`
	MonthlyReturns         map[string]float64       `json:"monthly_returns"`
	Sharpe                 float64                  `json:"sharpe,omitempty"`
	Calmar                 float64                  `json:"calmar,omitempty"`
	Benchmark              ReplayBenchmark          `json:"benchmark"`
	ExcessReturn           float64                  `json:"excess_return"`
	CorePnL                float64                  `json:"core_pnl,omitempty"`
	TacticalPnL            float64                  `json:"tactical_pnl,omitempty"`
	RegimeSwitches         map[string]int           `json:"regime_switches,omitempty"`
	RegimeSwitchTakeProfit float64                  `json:"regime_switch_take_profit,omitempty"`
	RegimeTimeframe        string                   `json:"regime_timeframe,omitempty"`
	CalibrationReportPath  string                   `json:"calibration_report_path,omitempty"`
	Comparison             map[string]ReplaySummary `json:"comparison,omitempty"`
	GeneratedAtUTC         time.Time                `json:"generated_at_utc"`
	ReportPathJSON         string                   `json:"report_path_json"`
	ReportPathMD           string                   `json:"report_path_md"`
	CostPerSidePct         float64                  `json:"cost_per_side_pct,omitempty"`
	ShortFundingAPR        float64                  `json:"short_funding_apr,omitempty"`
}

type replayPosition struct {
	active       bool
	entryPrice   float64
	stopLoss     float64
	riskPerUnit  float64
	qtyTotal     float64
	qtyRemain    float64
	tp1Done      bool
	tp2Done      bool
	trailStop    float64
	realizedPart float64
	holdBars     int
}

type replayShortPosition struct {
	active       bool
	entryPrice   float64
	stopLoss     float64
	riskPerUnit  float64
	qtyTotal     float64
	qtyRemain    float64
	tp1Done      bool
	tp2Done      bool
	trailStop    float64
	realizedPart float64
	holdBars     int
}

type marketRegime string

const (
	regimeBull  marketRegime = "BULL"
	regimeRange marketRegime = "RANGE"
	regimeBear  marketRegime = "BEAR"
)

func BaselineConfigFrom(cfg Config) Config {
	out := cfg
	out.EMA200MinSlope = -1
	out.VolumeMinRatio = 0
	out.ATRMinPct = 0
	out.ATRMaxPct = 10
	out.TrailATRMultiplier = 0
	out.SecondaryBreakoutLookback = 0
	out.SecondaryBreakoutBufferPct = 0
	out.MaxHoldBars = 0
	out.CoreAllocationPct = 0
	return out
}

func RunReplay(db *gorm.DB, days int) (*ReplayReport, error) {
	return RunReplayWithConfig(db, days, LegacyOptimizedConfig(), "optimized")
}

func RunReplayWithConfig(db *gorm.DB, days int, cfg Config, profile string) (*ReplayReport, error) {
	return RunReplayWithConfigAndRange(db, days, cfg, profile, time.Time{}, time.Time{})
}

func RunReplayWithConfigAndRange(db *gorm.DB, days int, cfg Config, profile string, start, end time.Time) (*ReplayReport, error) {
	if days <= 0 {
		days = 180
	}
	if profile == "" {
		profile = "optimized"
	}

	k := &models.KLineRecord{}
	dayData, err := k.GetKLines(db, cfg.Symbol, "1d", time.Time{}, time.Time{})
	if err != nil {
		return nil, err
	}
	h4Data, err := k.GetKLines(db, cfg.Symbol, "4h", time.Time{}, time.Time{})
	if err != nil {
		return nil, err
	}
	if len(h4Data) == 0 {
		h1Data, err := k.GetKLines(db, cfg.Symbol, "1h", time.Time{}, time.Time{})
		if err != nil {
			return nil, err
		}
		h4Data = aggregate1hTo4h(h1Data)
	}
	if len(dayData) == 0 || len(h4Data) == 0 {
		return nil, fmt.Errorf("insufficient kline data for replay")
	}

	windowEnd := h4Data[len(h4Data)-1].OpenTime.UTC()
	windowStart := windowEnd.AddDate(0, 0, -days)
	if !start.IsZero() && !end.IsZero() {
		windowStart = start.UTC()
		windowEnd = end.UTC()
	}

	var sum ReplaySummary
	var calib *RegimeCalibrationReport
	if profile == "v7" || profile == "v7_1" || profile == "v8" || profile == "v9" {
		calibVersion := profile
		if profile == "v8" || profile == "v9" {
			calibVersion = "v7_1"
		}
		calib, err = calibrateRegime(dayData, h4Data, cfg, windowStart, calibVersion)
		if err != nil {
			return nil, err
		}
		if profile == "v8" {
			sum, err = runV8Replay(cfg, dayData, h4Data, windowStart, windowEnd, calibToParams(calib))
		} else if profile == "v9" {
			sum, err = runV9Replay(cfg, dayData, h4Data, windowStart, windowEnd, calibToParams(calib))
		} else {
			sum, err = runV7Replay(cfg, dayData, h4Data, windowStart, calib.SelectedTimeframe, calibToParams(calib))
		}
		if err != nil {
			return nil, err
		}
		sum.RegimeTimeframe = "1w"
		if profile != "v8" && profile != "v9" {
			sum.RegimeTimeframe = calib.SelectedTimeframe
		}
		sum.CalibrationReportPath = calib.MDPath
	} else {
		sum, err = runProfileReplayInRange(cfg, profile, dayData, h4Data, windowStart, windowEnd)
		if err != nil {
			return nil, err
		}
	}
	benchmark := calcBuyHoldBenchmark(cfg.RiskCapitalUSDT, h4Data, windowStart, windowEnd)

	report := &ReplayReport{
		StrategyID:             cfg.StrategyID,
		Symbol:                 cfg.Symbol,
		Days:                   days,
		StartTimeUTC:           windowStart,
		EndTimeUTC:             windowEnd,
		Profile:                profile,
		Interval:               "4h",
		CapitalUSDT:            cfg.RiskCapitalUSDT,
		RiskPerTradePct:        cfg.RiskPerTrade,
		RiskPerTradeAmt:        cfg.RiskCapitalUSDT * cfg.RiskPerTrade,
		MaxDailyLossPct:        cfg.MaxDailyLossPct,
		MaxDailyLossAmt:        cfg.RiskCapitalUSDT * cfg.MaxDailyLossPct,
		MaxWeeklyLossPct:       cfg.MaxWeeklyLossPct,
		MaxWeeklyLossAmt:       cfg.RiskCapitalUSDT * cfg.MaxWeeklyLossPct,
		TradeCount:             sum.TradeCount,
		WinRate:                sum.WinRate,
		ProfitFactor:           sum.ProfitFactor,
		MaxDrawdown:            sum.MaxDrawdown,
		NetPnL:                 sum.NetPnL,
		MonthlyReturns:         sum.MonthlyReturns,
		Sharpe:                 sum.Sharpe,
		Calmar:                 sum.Calmar,
		CorePnL:                sum.CorePnL,
		TacticalPnL:            sum.TacticalPnL,
		RegimeSwitches:         sum.RegimeSwitches,
		RegimeSwitchTakeProfit: sum.RegimeSwitchTakeProfit,
		RegimeTimeframe:        sum.RegimeTimeframe,
		CalibrationReportPath:  sum.CalibrationReportPath,
		Benchmark:              benchmark,
		ExcessReturn:           sum.NetPnL - benchmark.NetPnL,
		GeneratedAtUTC:         time.Now().UTC(),
		CostPerSidePct:         cfg.CostPerSidePct,
		ShortFundingAPR:        cfg.ShortFundingAPR,
	}

	if profile == "v4" {
		report.Comparison = map[string]ReplaySummary{}
		bCfg := BaselineConfigFrom(LegacyOptimizedConfig())
		v3Cfg := V3Config()
		v4Cfg := V4Config()
		for name, c := range map[string]Config{"baseline": bCfg, "v3": v3Cfg, "v4": v4Cfg} {
			rs, e := runProfileReplayInRange(c, name, dayData, h4Data, windowStart, windowEnd)
			if e != nil {
				return nil, e
			}
			report.Comparison[name] = rs
		}
	}
	if profile == "v6" {
		report.Comparison = map[string]ReplaySummary{}
		for name, c := range map[string]Config{"v4": V4Config(), "v5_mean_reversion": V5MeanReversionConfig(), "v6": V6RegimeConfig()} {
			rs, e := runProfileReplayInRange(c, name, dayData, h4Data, windowStart, windowEnd)
			if e != nil {
				return nil, e
			}
			report.Comparison[name] = rs
		}
		if err := saveV6ComparisonReport(report); err != nil {
			return nil, err
		}
	}
	if profile == "v7" || profile == "v7_1" || profile == "v8" || profile == "v9" {
		report.Comparison = map[string]ReplaySummary{}
		for name, c := range map[string]Config{"v5_mean_reversion": V5MeanReversionConfig(), "v6": V6RegimeConfig()} {
			rs, e := runProfileReplayInRange(c, name, dayData, h4Data, windowStart, windowEnd)
			if e != nil {
				return nil, e
			}
			report.Comparison[name] = rs
		}
		if profile == "v7_1" {
			legacyCalib, e := calibrateRegime(dayData, h4Data, V7RegimeConfig(), windowStart, "v7")
			if e == nil {
				legacy, e2 := runV7Replay(V7RegimeConfig(), dayData, h4Data, windowStart, legacyCalib.SelectedTimeframe, calibToParams(legacyCalib))
				if e2 == nil {
					report.Comparison["v7"] = legacy
				}
			}
			report.Comparison["v7_1"] = sum
		} else if profile == "v8" || profile == "v9" {
			v71Calib, e := calibrateRegime(dayData, h4Data, V7RegimeConfig(), windowStart, "v7_1")
			if e == nil {
				v71, e2 := runV7Replay(V7RegimeConfig(), dayData, h4Data, windowStart, v71Calib.SelectedTimeframe, calibToParams(v71Calib))
				if e2 == nil {
					report.Comparison["v7_1"] = v71
				}
			}
			v8Calib, e := calibrateRegime(dayData, h4Data, V8Config(), windowStart, "v7_1")
			if e == nil {
				v8Sum, e2 := runV8Replay(V8Config(), dayData, h4Data, windowStart, windowEnd, calibToParams(v8Calib))
				if e2 == nil {
					report.Comparison["v8"] = v8Sum
				}
			}
			if profile == "v8" {
				report.Comparison["v8"] = sum
			} else {
				report.Comparison["v9"] = sum
			}
		} else {
			report.Comparison["v7"] = sum
		}
		if err := saveV7ComparisonReport(report); err != nil {
			return nil, err
		}
	}

	if err := saveReplayReport(report); err != nil {
		return nil, err
	}
	return report, nil
}

func runProfileReplay(cfg Config, profile string, dayData, h4Data []models.KLineRecord, start time.Time) (ReplaySummary, error) {
	return runProfileReplayInRange(cfg, profile, dayData, h4Data, start, time.Time{})
}

func runProfileReplayInRange(cfg Config, profile string, dayData, h4Data []models.KLineRecord, start, end time.Time) (ReplaySummary, error) {
	if profile == "v6" {
		return runV6Replay(cfg, dayData, h4Data, start)
	}
	if profile == "v8" || profile == "v9" {
		calib, err := calibrateRegime(dayData, h4Data, cfg, start, "v7_1")
		if err != nil {
			return ReplaySummary{}, err
		}
		if profile == "v9" {
			return runV9Replay(cfg, dayData, h4Data, start, end, calibToParams(calib))
		}
		return runV8Replay(cfg, dayData, h4Data, start, end, calibToParams(calib))
	}
	if profile == "v10" {
		return runV10Replay(cfg, dayData, h4Data, start, end)
	}
	if profile == "v11" || profile == "v11_a" || profile == "v11_b" || profile == "v12" || profile == "v12_a" || profile == "v12_b" || profile == "v13" || profile == "v13_a" || profile == "v13_b" {
		return runV11Replay(cfg, profile, dayData, h4Data, start, end)
	}
	var wins int
	var profits float64
	var lossesAbs float64
	var net float64
	var corePnL float64
	var tacticalPnL float64
	var peak float64
	var maxDD float64
	monthly := map[string]float64{}
	capital := cfg.RiskCapitalUSDT

	pos := replayPosition{}
	tradeCount := 0
	coreActive := false
	coreQty := 0.0
	coreEntry := 0.0
	coreAlloc := cfg.CoreAllocationPct
	if coreAlloc < 0 {
		coreAlloc = 0
	}
	if coreAlloc > 0.9 {
		coreAlloc = 0.9
	}
	tacticalCapital := capital * (1 - coreAlloc)
	if tacticalCapital <= 0 {
		tacticalCapital = capital
	}
	pauseBars := 0
	consecutiveLosses := 0
	dailyPnL := map[string]float64{}
	weeklyPnL := map[string]float64{}

	for i := range h4Data {
		cur := h4Data[i]
		t := cur.OpenTime.UTC()
		if t.Before(start) || (!end.IsZero() && t.After(end)) {
			continue
		}

		h4Slice := make([]OHLC, 0, i+1)
		for j := 0; j <= i; j++ {
			h4Slice = append(h4Slice, OHLC{Close: h4Data[j].Close, High: h4Data[j].High, Low: h4Data[j].Low, Volume: h4Data[j].Volume})
		}
		daySlice := make([]OHLC, 0, 256)
		for _, d := range dayData {
			if d.OpenTime.UTC().After(t) {
				break
			}
			daySlice = append(daySlice, OHLC{Close: d.Close, High: d.High, Low: d.Low, Volume: d.Volume})
		}

		if profile == "v4" && coreAlloc > 0 {
			trendOn := evaluateTrendOnly(cfg, daySlice, h4Slice)
			if trendOn && !coreActive {
				coreNotional := capital * coreAlloc
				if cur.Close > 0 {
					coreQty = coreNotional / cur.Close
					coreEntry = cur.Close
					coreActive = true
				}
			} else if !trendOn && coreActive {
				pnl := (cur.Close - coreEntry) * coreQty
				net += pnl
				corePnL += pnl
				month := t.Format("2006-01")
				monthly[month] += pnl / capital
				coreActive = false
				coreQty = 0
				coreEntry = 0
			}
		}

		if pauseBars > 0 {
			pauseBars--
		}

		if !pos.active {
			riskBlocked := false
			if profile == "v4" {
				dayKey := t.Format("2006-01-02")
				_, week := t.ISOWeek()
				weekKey := fmt.Sprintf("%d-W%02d", t.Year(), week)
				if -dailyPnL[dayKey] >= capital*cfg.MaxDailyLossPct || -weeklyPnL[weekKey] >= capital*cfg.MaxWeeklyLossPct || pauseBars > 0 {
					riskBlocked = true
				}
			}
			if riskBlocked {
				continue
			}
			sig, err := EvaluateSignal(cfg, daySlice, h4Slice)
			if err != nil {
				return ReplaySummary{}, err
			}
			if !sig.ShouldEnter {
				continue
			}
			tradeCfg := cfg
			if profile == "v4" {
				tradeCfg.RiskCapitalUSDT = tacticalCapital
			}
			plan := BuildPosition(tradeCfg, sig.EntryPrice, sig.StopLoss)
			if plan.Qty <= 0 {
				continue
			}
			pos = replayPosition{active: true, entryPrice: sig.EntryPrice, stopLoss: sig.StopLoss, riskPerUnit: plan.RiskPerUnit, qtyTotal: plan.Qty, qtyRemain: plan.Qty}
			continue
		}

		lastPrice := cur.Close
		pos.holdBars++
		tp1 := pos.entryPrice + pos.riskPerUnit
		tp2 := pos.entryPrice + 2*pos.riskPerUnit
		if !pos.tp1Done && lastPrice >= tp1 {
			qty := pos.qtyTotal * 0.4
			pos.qtyRemain -= qty
			pos.tp1Done = true
			pos.trailStop = pos.entryPrice
			pos.realizedPart += (lastPrice - pos.entryPrice) * qty
		}
		if !pos.tp2Done && lastPrice >= tp2 {
			qty := pos.qtyTotal * 0.3
			pos.qtyRemain -= qty
			pos.tp2Done = true
			if pos.trailStop < tp1 {
				pos.trailStop = tp1
			}
			pos.realizedPart += (lastPrice - pos.entryPrice) * qty
		}

		trail := pos.trailStop
		if cfg.TrailATRMultiplier > 0 && pos.tp1Done && len(h4Slice) >= cfg.ATRPeriod+1 {
			highs := make([]float64, 0, len(h4Slice))
			lows := make([]float64, 0, len(h4Slice))
			closes := make([]float64, 0, len(h4Slice))
			for _, bar := range h4Slice {
				highs = append(highs, bar.High)
				lows = append(lows, bar.Low)
				closes = append(closes, bar.Close)
			}
			atr := indicator.LastATR(highs, lows, closes, cfg.ATRPeriod)
			if atr > 0 {
				atrTrail := lastPrice - atr*cfg.TrailATRMultiplier
				if atrTrail > trail {
					trail = atrTrail
				}
			}
		}
		if trail <= 0 {
			trail = pos.stopLoss
		}
		exitByStop := lastPrice <= trail
		exitByTimeout := cfg.MaxHoldBars > 0 && pos.holdBars >= cfg.MaxHoldBars
		if exitByStop || exitByTimeout {
			pnl := pos.realizedPart + (lastPrice-pos.entryPrice)*pos.qtyRemain
			net += pnl
			tacticalPnL += pnl
			if pnl > 0 {
				wins++
				profits += pnl
				consecutiveLosses = 0
			} else if pnl < 0 {
				lossesAbs += -pnl
				consecutiveLosses++
				if profile == "v4" && consecutiveLosses >= 3 {
					pauseBars = 6
					consecutiveLosses = 0
				}
			}
			tradeCount++
			equity := net
			if coreActive {
				equity += (lastPrice - coreEntry) * coreQty
			}
			if equity > peak {
				peak = equity
			}
			dd := peak - equity
			if dd > maxDD {
				maxDD = dd
			}
			month := t.Format("2006-01")
			monthly[month] += pnl / capital
			if profile == "v4" {
				dayKey := t.Format("2006-01-02")
				_, week := t.ISOWeek()
				weekKey := fmt.Sprintf("%d-W%02d", t.Year(), week)
				dailyPnL[dayKey] += pnl
				weeklyPnL[weekKey] += pnl
			}
			pos = replayPosition{}
		}
	}

	if profile == "v4" && coreActive {
		last := h4Data[len(h4Data)-1]
		pnl := (last.Close - coreEntry) * coreQty
		net += pnl
		corePnL += pnl
		month := last.OpenTime.UTC().Format("2006-01")
		monthly[month] += pnl / capital
	}

	winRate := 0.0
	if tradeCount > 0 {
		winRate = float64(wins) / float64(tradeCount)
	}
	profitFactor := 0.0
	if lossesAbs > 0 {
		profitFactor = profits / lossesAbs
	}

	sharpe := 0.0
	calmar := 0.0
	if len(monthly) >= 2 {
		vals := make([]float64, 0, len(monthly))
		for _, v := range monthly {
			vals = append(vals, v)
		}
		mean, std := meanStd(vals)
		if std > 0 {
			sharpe = mean / std * math.Sqrt(12)
		}
	}
	if maxDD > 0 {
		calmar = net / maxDD
	}

	return ReplaySummary{
		Profile:        profile,
		TradeCount:     tradeCount,
		WinRate:        winRate,
		ProfitFactor:   profitFactor,
		MaxDrawdown:    maxDD,
		NetPnL:         net,
		MonthlyReturns: monthly,
		Sharpe:         sharpe,
		Calmar:         calmar,
		CorePnL:        corePnL,
		TacticalPnL:    tacticalPnL,
	}, nil
}

func runV6Replay(cfg Config, dayData, h4Data []models.KLineRecord, start time.Time) (ReplaySummary, error) {
	var wins int
	var profits float64
	var lossesAbs float64
	var net float64
	var peak float64
	var maxDD float64
	monthly := map[string]float64{}
	capital := cfg.RiskCapitalUSDT
	if cfg.BearLeverage <= 0 {
		cfg.BearLeverage = 1
	}
	if cfg.RegimeMinConfirmBars <= 0 {
		cfg.RegimeMinConfirmBars = 2
	}
	switchCounts := map[string]int{}
	switchTakeProfitPnL := 0.0

	var longPos replayPosition
	var shortPos replayShortPosition
	tradeCount := 0
	pauseBars := 0
	consecutiveLosses := 0
	dailyPnL := map[string]float64{}
	weeklyPnL := map[string]float64{}

	currentRegime := regimeRange
	pendingRegime := currentRegime
	pendingCnt := 0

	for i := range h4Data {
		cur := h4Data[i]
		t := cur.OpenTime.UTC()
		if t.Before(start) {
			continue
		}
		h4Slice := make([]OHLC, 0, i+1)
		for j := 0; j <= i; j++ {
			h4Slice = append(h4Slice, OHLC{Close: h4Data[j].Close, High: h4Data[j].High, Low: h4Data[j].Low, Volume: h4Data[j].Volume})
		}
		daySlice := make([]OHLC, 0, 256)
		for _, d := range dayData {
			if d.OpenTime.UTC().After(t) {
				break
			}
			daySlice = append(daySlice, OHLC{Close: d.Close, High: d.High, Low: d.Low, Volume: d.Volume})
		}
		if len(daySlice) < cfg.EMAFilterPeriod+cfg.EMA200SlopeLookback+2 {
			continue
		}

		candidate := detectRegime(cfg, daySlice)
		if candidate != currentRegime {
			if candidate != pendingRegime {
				pendingRegime = candidate
				pendingCnt = 1
			} else {
				pendingCnt++
			}
			if pendingCnt >= cfg.RegimeMinConfirmBars {
				prev := currentRegime
				currentRegime = candidate
				switchKey := fmt.Sprintf("%s->%s", prev, currentRegime)
				switchCounts[switchKey]++
				pendingCnt = 0
				if prev == regimeBull && currentRegime == regimeRange && longPos.active {
					tpPnL := applyLongSwitchTakeProfit(cfg, &longPos, cur.Close, h4Slice)
					net += tpPnL
					switchTakeProfitPnL += tpPnL
					month := t.Format("2006-01")
					monthly[month] += tpPnL / capital
				}
				if prev == regimeBear && currentRegime == regimeRange && shortPos.active {
					tpPnL := applyShortSwitchTakeProfit(cfg, &shortPos, cur.Close, h4Slice)
					net += tpPnL
					switchTakeProfitPnL += tpPnL
					month := t.Format("2006-01")
					monthly[month] += tpPnL / capital
				}
				if prev == regimeRange && currentRegime == regimeBull && shortPos.active {
					pnl := shortPos.realizedPart + (shortPos.entryPrice-cur.Close)*shortPos.qtyRemain
					net += pnl
					month := t.Format("2006-01")
					monthly[month] += pnl / capital
					if pnl > 0 {
						wins++
						profits += pnl
						consecutiveLosses = 0
					} else if pnl < 0 {
						lossesAbs += -pnl
						consecutiveLosses++
					}
					tradeCount++
					shortPos = replayShortPosition{}
				}
			}
		}

		if pauseBars > 0 {
			pauseBars--
		}
		dayKey := t.Format("2006-01-02")
		_, week := t.ISOWeek()
		weekKey := fmt.Sprintf("%d-W%02d", t.Year(), week)
		riskBlocked := -dailyPnL[dayKey] >= capital*cfg.MaxDailyLossPct || -weeklyPnL[weekKey] >= capital*cfg.MaxWeeklyLossPct || pauseBars > 0

		if !longPos.active && !shortPos.active && !riskBlocked {
			switch currentRegime {
			case regimeBull:
				sig, err := evalTrend(cfg, daySlice, h4Slice)
				if err != nil {
					return ReplaySummary{}, err
				}
				if sig.ShouldEnter {
					plan := BuildPosition(cfg, sig.EntryPrice, sig.StopLoss)
					if plan.Qty > 0 {
						longPos = replayPosition{active: true, entryPrice: sig.EntryPrice, stopLoss: sig.StopLoss, riskPerUnit: plan.RiskPerUnit, qtyTotal: plan.Qty, qtyRemain: plan.Qty}
					}
				}
			case regimeBear:
				sig, err := evalTrendShort(cfg, daySlice, h4Slice)
				if err != nil {
					return ReplaySummary{}, err
				}
				if sig.ShouldEnter {
					plan := buildShortPosition(cfg, sig.EntryPrice, sig.StopLoss)
					if plan.Qty > 0 {
						shortPos = replayShortPosition{active: true, entryPrice: sig.EntryPrice, stopLoss: sig.StopLoss, riskPerUnit: plan.RiskPerUnit, qtyTotal: plan.Qty, qtyRemain: plan.Qty}
					}
				}
			}
		}

		if longPos.active {
			pnl, closed := stepLongPosition(cfg, &longPos, cur.Close, h4Slice)
			if closed {
				net += pnl
				month := t.Format("2006-01")
				monthly[month] += pnl / capital
				dailyPnL[dayKey] += pnl
				weeklyPnL[weekKey] += pnl
				if pnl > 0 {
					wins++
					profits += pnl
					consecutiveLosses = 0
				} else if pnl < 0 {
					lossesAbs += -pnl
					consecutiveLosses++
					if consecutiveLosses >= 3 {
						pauseBars = 6
						consecutiveLosses = 0
					}
				}
				tradeCount++
			}
		}
		if shortPos.active {
			pnl, closed := stepShortPosition(cfg, &shortPos, cur.Close, h4Slice)
			if closed {
				net += pnl
				month := t.Format("2006-01")
				monthly[month] += pnl / capital
				dailyPnL[dayKey] += pnl
				weeklyPnL[weekKey] += pnl
				if pnl > 0 {
					wins++
					profits += pnl
					consecutiveLosses = 0
				} else if pnl < 0 {
					lossesAbs += -pnl
					consecutiveLosses++
					if consecutiveLosses >= 3 {
						pauseBars = 6
						consecutiveLosses = 0
					}
				}
				tradeCount++
			}
		}

		equity := net
		if longPos.active {
			equity += longPos.realizedPart + (cur.Close-longPos.entryPrice)*longPos.qtyRemain
		}
		if shortPos.active {
			equity += shortPos.realizedPart + (shortPos.entryPrice-cur.Close)*shortPos.qtyRemain
		}
		if equity > peak {
			peak = equity
		}
		dd := peak - equity
		if dd > maxDD {
			maxDD = dd
		}
	}

	winRate := 0.0
	if tradeCount > 0 {
		winRate = float64(wins) / float64(tradeCount)
	}
	profitFactor := 0.0
	if lossesAbs > 0 {
		profitFactor = profits / lossesAbs
	}
	sharpe := 0.0
	calmar := 0.0
	if len(monthly) >= 2 {
		vals := make([]float64, 0, len(monthly))
		for _, v := range monthly {
			vals = append(vals, v)
		}
		mean, std := meanStd(vals)
		if std > 0 {
			sharpe = mean / std * math.Sqrt(12)
		}
	}
	if maxDD > 0 {
		calmar = net / maxDD
	}
	return ReplaySummary{Profile: "v6", TradeCount: tradeCount, WinRate: winRate, ProfitFactor: profitFactor, MaxDrawdown: maxDD, NetPnL: net, MonthlyReturns: monthly, Sharpe: sharpe, Calmar: calmar, RegimeSwitches: switchCounts, RegimeSwitchTakeProfit: switchTakeProfitPnL}, nil
}

func runV8Replay(cfg Config, dayData, h4Data []models.KLineRecord, start, end time.Time, p RegimeCalibParams) (ReplaySummary, error) {
	weekly := aggregate1dTo1w(dayData)
	if len(weekly) < 120 {
		return ReplaySummary{}, fmt.Errorf("insufficient weekly bars for v8: %d", len(weekly))
	}
	points := classifySeries(weekly, p)
	if len(points) == 0 {
		return ReplaySummary{}, fmt.Errorf("no weekly regime points for v8")
	}
	if cfg.RegimeMinConfirmBars <= 0 {
		cfg.RegimeMinConfirmBars = 2
	}
	if cfg.RegimeMinStayBars <= 0 {
		cfg.RegimeMinStayBars = 3
	}
	if cfg.MaxEntriesPerWeek <= 0 {
		cfg.MaxEntriesPerWeek = 2
	}
	if cfg.CooldownBars < 0 {
		cfg.CooldownBars = 0
	}

	var wins int
	var profits, lossesAbs, net, peak, maxDD float64
	monthly := map[string]float64{}
	capital := cfg.RiskCapitalUSDT
	longPos := replayPosition{}
	shortPos := replayShortPosition{}
	tradeCount := 0
	pauseBars := 0
	cooldown := 0
	consecutiveLosses := 0
	dailyPnL := map[string]float64{}
	weeklyPnL := map[string]float64{}
	weeklyEntries := map[string]int{}

	idx := 0
	currentRegime := regimeRange
	pendingRegime := regimeRange
	pendingCnt := 0
	barsSinceSwitch := 999
	switchCounts := map[string]int{}

	for i := range h4Data {
		cur := h4Data[i]
		t := cur.OpenTime.UTC()
		if t.Before(start) || (!end.IsZero() && t.After(end)) {
			continue
		}
		for idx < len(points) && !points[idx].Time.After(t) {
			cand := points[idx].Regime
			delta := math.Abs(points[idx].Score)
			if cand != currentRegime && barsSinceSwitch >= cfg.RegimeMinStayBars && delta >= cfg.RegimeHysteresis {
				if cand != pendingRegime {
					pendingRegime = cand
					pendingCnt = 1
				} else {
					pendingCnt++
				}
				if pendingCnt >= cfg.RegimeMinConfirmBars {
					switchCounts[fmt.Sprintf("%s->%s", currentRegime, cand)]++
					currentRegime = cand
					pendingCnt = 0
					barsSinceSwitch = 0
				}
			} else if cand == currentRegime {
				pendingCnt = 0
			}
			idx++
		}
		barsSinceSwitch++

		h4Slice := make([]OHLC, 0, i+1)
		for j := 0; j <= i; j++ {
			h4Slice = append(h4Slice, OHLC{Close: h4Data[j].Close, High: h4Data[j].High, Low: h4Data[j].Low, Volume: h4Data[j].Volume})
		}
		daySlice := make([]OHLC, 0, 256)
		for _, d := range dayData {
			if d.OpenTime.UTC().After(t) {
				break
			}
			daySlice = append(daySlice, OHLC{Close: d.Close, High: d.High, Low: d.Low, Volume: d.Volume})
		}

		if pauseBars > 0 {
			pauseBars--
		}
		if cooldown > 0 {
			cooldown--
		}
		dayKey := t.Format("2006-01-02")
		y, w := t.ISOWeek()
		weekKey := fmt.Sprintf("%d-W%02d", y, w)
		riskBlocked := -dailyPnL[dayKey] >= capital*cfg.MaxDailyLossPct || -weeklyPnL[weekKey] >= capital*cfg.MaxWeeklyLossPct || pauseBars > 0

		if currentRegime == regimeRange {
			if longPos.active {
				pnl := longPos.realizedPart + (cur.Close-longPos.entryPrice)*longPos.qtyRemain
				net += pnl
				monthly[t.Format("2006-01")] += pnl / capital
				tradeCount++
				longPos = replayPosition{}
			}
			if shortPos.active {
				pnl := shortPos.realizedPart + (shortPos.entryPrice-cur.Close)*shortPos.qtyRemain
				net += pnl
				monthly[t.Format("2006-01")] += pnl / capital
				tradeCount++
				shortPos = replayShortPosition{}
			}
		}

		if !longPos.active && !shortPos.active && !riskBlocked && cooldown == 0 && weeklyEntries[weekKey] < cfg.MaxEntriesPerWeek {
			switch currentRegime {
			case regimeBull:
				sig, err := evalTrend(cfg, daySlice, h4Slice)
				if err != nil {
					return ReplaySummary{}, err
				}
				if sig.ShouldEnter && longSignalStrength(cfg, daySlice, h4Slice) >= cfg.MinSignalStrength {
					plan := BuildPosition(cfg, sig.EntryPrice, sig.StopLoss)
					if plan.Qty > 0 {
						longPos = replayPosition{active: true, entryPrice: sig.EntryPrice, stopLoss: sig.StopLoss, riskPerUnit: plan.RiskPerUnit, qtyTotal: plan.Qty, qtyRemain: plan.Qty}
						cooldown = cfg.CooldownBars
						weeklyEntries[weekKey]++
					}
				}
			case regimeBear:
				if !bearQualityFilter(cfg, daySlice, h4Slice) {
					break
				}
				sig, err := evalTrendShort(cfg, daySlice, h4Slice)
				if err != nil {
					return ReplaySummary{}, err
				}
				if sig.ShouldEnter && shortSignalStrength(cfg, daySlice, h4Slice) >= cfg.MinSignalStrength {
					plan := buildShortPosition(cfg, sig.EntryPrice, sig.StopLoss)
					if plan.Qty > 0 {
						shortPos = replayShortPosition{active: true, entryPrice: sig.EntryPrice, stopLoss: sig.StopLoss, riskPerUnit: plan.RiskPerUnit, qtyTotal: plan.Qty, qtyRemain: plan.Qty}
						cooldown = cfg.CooldownBars
						weeklyEntries[weekKey]++
					}
				}
			}
		}

		if longPos.active {
			pnl, closed := stepLongPositionV8(cfg, &longPos, cur.Close, h4Slice)
			if closed {
				net += pnl
				monthly[t.Format("2006-01")] += pnl / capital
				dailyPnL[dayKey] += pnl
				weeklyPnL[weekKey] += pnl
				if pnl > 0 {
					wins++
					profits += pnl
					consecutiveLosses = 0
				} else if pnl < 0 {
					lossesAbs += -pnl
					consecutiveLosses++
					if consecutiveLosses >= 3 {
						pauseBars = 6
						consecutiveLosses = 0
					}
				}
				tradeCount++
			}
		}
		if shortPos.active {
			pnl, closed := stepShortPositionV8(cfg, &shortPos, cur.Close, h4Slice)
			if closed {
				net += pnl
				monthly[t.Format("2006-01")] += pnl / capital
				dailyPnL[dayKey] += pnl
				weeklyPnL[weekKey] += pnl
				if pnl > 0 {
					wins++
					profits += pnl
					consecutiveLosses = 0
				} else if pnl < 0 {
					lossesAbs += -pnl
					consecutiveLosses++
					if consecutiveLosses >= 3 {
						pauseBars = 6
						consecutiveLosses = 0
					}
				}
				tradeCount++
			}
		}

		equity := net
		if longPos.active {
			equity += longPos.realizedPart + (cur.Close-longPos.entryPrice)*longPos.qtyRemain
		}
		if shortPos.active {
			equity += shortPos.realizedPart + (shortPos.entryPrice-cur.Close)*shortPos.qtyRemain
		}
		if equity > peak {
			peak = equity
		}
		if dd := peak - equity; dd > maxDD {
			maxDD = dd
		}
	}

	winRate := 0.0
	if tradeCount > 0 {
		winRate = float64(wins) / float64(tradeCount)
	}
	profitFactor := 0.0
	if lossesAbs > 0 {
		profitFactor = profits / lossesAbs
	}
	sharpe := 0.0
	calmar := 0.0
	if len(monthly) >= 2 {
		vals := make([]float64, 0, len(monthly))
		for _, v := range monthly {
			vals = append(vals, v)
		}
		mean, std := meanStd(vals)
		if std > 0 {
			sharpe = mean / std * math.Sqrt(12)
		}
	}
	if maxDD > 0 {
		calmar = net / maxDD
	}
	return ReplaySummary{Profile: "v8", TradeCount: tradeCount, WinRate: winRate, ProfitFactor: profitFactor, MaxDrawdown: maxDD, NetPnL: net, MonthlyReturns: monthly, Sharpe: sharpe, Calmar: calmar, RegimeSwitches: switchCounts, RegimeTimeframe: "1w"}, nil
}

func runV9Replay(cfg Config, dayData, h4Data []models.KLineRecord, start, end time.Time, p RegimeCalibParams) (ReplaySummary, error) {
	weekly := aggregate1dTo1w(dayData)
	if len(weekly) < 120 {
		return ReplaySummary{}, fmt.Errorf("insufficient weekly bars for v9: %d", len(weekly))
	}
	points := classifySeries(weekly, p)
	if len(points) == 0 {
		return ReplaySummary{}, fmt.Errorf("no weekly regime points for v9")
	}
	if cfg.RegimeMinConfirmBars <= 0 {
		cfg.RegimeMinConfirmBars = 2
	}
	if cfg.RegimeMinStayBars <= 0 {
		cfg.RegimeMinStayBars = 3
	}
	if cfg.MaxEntriesPerWeek <= 0 {
		cfg.MaxEntriesPerWeek = 3
	}
	if cfg.CooldownBars < 0 {
		cfg.CooldownBars = 0
	}
	if cfg.RangeLookbackDays <= 0 {
		cfg.RangeLookbackDays = 20
	}
	if cfg.RangeEntryATRBuffer <= 0 {
		cfg.RangeEntryATRBuffer = 0.6
	}
	if cfg.RangeBreakoutATRBuffer <= 0 {
		cfg.RangeBreakoutATRBuffer = 0.55
	}
	if cfg.RangeMidExitRatio <= 0 || cfg.RangeMidExitRatio >= 1 {
		cfg.RangeMidExitRatio = 0.6
	}
	if cfg.RangeMaxHoldBars <= 0 {
		cfg.RangeMaxHoldBars = 18
	}
	if cfg.RangeMinWidthATR <= 0 {
		cfg.RangeMinWidthATR = 2.4
	}
	if cfg.RangeMinSignalStrength <= 0 {
		cfg.RangeMinSignalStrength = cfg.MinSignalStrength
	}

	var wins int
	var profits, lossesAbs, net, peak, maxDD float64
	monthly := map[string]float64{}
	capital := cfg.RiskCapitalUSDT
	longPos := replayPosition{}
	shortPos := replayShortPosition{}
	tradeCount := 0
	pauseBars := 0
	cooldown := 0
	consecutiveLosses := 0
	dailyPnL := map[string]float64{}
	weeklyPnL := map[string]float64{}
	weeklyEntries := map[string]int{}

	idx := 0
	currentRegime := regimeRange
	pendingRegime := regimeRange
	pendingCnt := 0
	barsSinceSwitch := 999
	switchCounts := map[string]int{}

	for i := range h4Data {
		cur := h4Data[i]
		t := cur.OpenTime.UTC()
		if t.Before(start) || (!end.IsZero() && t.After(end)) {
			continue
		}
		for idx < len(points) && !points[idx].Time.After(t) {
			cand := points[idx].Regime
			delta := math.Abs(points[idx].Score)
			if cand != currentRegime && barsSinceSwitch >= cfg.RegimeMinStayBars && delta >= cfg.RegimeHysteresis {
				if cand != pendingRegime {
					pendingRegime = cand
					pendingCnt = 1
				} else {
					pendingCnt++
				}
				if pendingCnt >= cfg.RegimeMinConfirmBars {
					switchCounts[fmt.Sprintf("%s->%s", currentRegime, cand)]++
					currentRegime = cand
					pendingCnt = 0
					barsSinceSwitch = 0
				}
			} else if cand == currentRegime {
				pendingCnt = 0
			}
			idx++
		}
		barsSinceSwitch++

		h4Slice := make([]OHLC, 0, i+1)
		for j := 0; j <= i; j++ {
			h4Slice = append(h4Slice, OHLC{Close: h4Data[j].Close, High: h4Data[j].High, Low: h4Data[j].Low, Volume: h4Data[j].Volume})
		}
		daySlice := make([]OHLC, 0, 256)
		for _, d := range dayData {
			if d.OpenTime.UTC().After(t) {
				break
			}
			daySlice = append(daySlice, OHLC{Close: d.Close, High: d.High, Low: d.Low, Volume: d.Volume})
		}

		if pauseBars > 0 {
			pauseBars--
		}
		if cooldown > 0 {
			cooldown--
		}
		dayKey := t.Format("2006-01-02")
		y, w := t.ISOWeek()
		weekKey := fmt.Sprintf("%d-W%02d", y, w)
		riskBlocked := -dailyPnL[dayKey] >= capital*cfg.MaxDailyLossPct || -weeklyPnL[weekKey] >= capital*cfg.MaxWeeklyLossPct || pauseBars > 0

		rangeBand, rangeOK := detectRangeBand(cfg, daySlice, h4Slice)
		if currentRegime != regimeRange {
			if longPos.active && longPos.stopLoss == 0 {
				pnl := longPos.realizedPart + (cur.Close-longPos.entryPrice)*longPos.qtyRemain
				net += pnl
				monthly[t.Format("2006-01")] += pnl / capital
				dailyPnL[dayKey] += pnl
				weeklyPnL[weekKey] += pnl
				tradeCount++
				longPos = replayPosition{}
			}
			if shortPos.active && shortPos.stopLoss == 0 {
				pnl := shortPos.realizedPart + (shortPos.entryPrice-cur.Close)*shortPos.qtyRemain
				net += pnl
				monthly[t.Format("2006-01")] += pnl / capital
				dailyPnL[dayKey] += pnl
				weeklyPnL[weekKey] += pnl
				tradeCount++
				shortPos = replayShortPosition{}
			}
		}

		if !longPos.active && !shortPos.active && !riskBlocked && cooldown == 0 && weeklyEntries[weekKey] < cfg.MaxEntriesPerWeek {
			switch currentRegime {
			case regimeBull:
				sig, err := evalTrend(cfg, daySlice, h4Slice)
				if err != nil {
					return ReplaySummary{}, err
				}
				if sig.ShouldEnter && longSignalStrength(cfg, daySlice, h4Slice) >= cfg.MinSignalStrength {
					plan := BuildPosition(cfg, sig.EntryPrice, sig.StopLoss)
					if plan.Qty > 0 {
						longPos = replayPosition{active: true, entryPrice: sig.EntryPrice, stopLoss: sig.StopLoss, riskPerUnit: plan.RiskPerUnit, qtyTotal: plan.Qty, qtyRemain: plan.Qty}
						cooldown = cfg.CooldownBars
						weeklyEntries[weekKey]++
					}
				}
			case regimeBear:
				if !bearQualityFilter(cfg, daySlice, h4Slice) {
					break
				}
				sig, err := evalTrendShort(cfg, daySlice, h4Slice)
				if err != nil {
					return ReplaySummary{}, err
				}
				if sig.ShouldEnter && shortSignalStrength(cfg, daySlice, h4Slice) >= cfg.MinSignalStrength {
					plan := buildShortPosition(cfg, sig.EntryPrice, sig.StopLoss)
					if plan.Qty > 0 {
						shortPos = replayShortPosition{active: true, entryPrice: sig.EntryPrice, stopLoss: sig.StopLoss, riskPerUnit: plan.RiskPerUnit, qtyTotal: plan.Qty, qtyRemain: plan.Qty}
						cooldown = cfg.CooldownBars
						weeklyEntries[weekKey]++
					}
				}
			case regimeRange:
				if rangeOK {
					if score := rangeLongSignalStrength(cfg, cur.Close, rangeBand); score >= cfg.RangeMinSignalStrength {
						entry := cur.Close
						stop := rangeBand.Lower - rangeBand.ATR*cfg.RangeBreakoutATRBuffer
						plan := BuildPosition(cfg, entry, stop)
						if plan.Qty > 0 {
							longPos = replayPosition{active: true, entryPrice: entry, stopLoss: 0, riskPerUnit: plan.RiskPerUnit, qtyTotal: plan.Qty, qtyRemain: plan.Qty, trailStop: stop}
							cooldown = cfg.CooldownBars
							weeklyEntries[weekKey]++
						}
					} else if score := rangeShortSignalStrength(cfg, cur.Close, rangeBand); score >= cfg.RangeMinSignalStrength {
						entry := cur.Close
						stop := rangeBand.Upper + rangeBand.ATR*cfg.RangeBreakoutATRBuffer
						plan := buildShortPosition(cfg, entry, stop)
						if plan.Qty > 0 {
							shortPos = replayShortPosition{active: true, entryPrice: entry, stopLoss: 0, riskPerUnit: plan.RiskPerUnit, qtyTotal: plan.Qty, qtyRemain: plan.Qty, trailStop: stop}
							cooldown = cfg.CooldownBars
							weeklyEntries[weekKey]++
						}
					}
				}
			}
		}

		if longPos.active {
			pnl, closed := 0.0, false
			if longPos.stopLoss == 0 {
				pnl, closed = stepRangeLongPosition(cfg, &longPos, cur.Close, currentRegime, rangeBand, rangeOK)
			} else {
				pnl, closed = stepLongPositionV8(cfg, &longPos, cur.Close, h4Slice)
			}
			if closed {
				net += pnl
				monthly[t.Format("2006-01")] += pnl / capital
				dailyPnL[dayKey] += pnl
				weeklyPnL[weekKey] += pnl
				if pnl > 0 {
					wins++
					profits += pnl
					consecutiveLosses = 0
				} else if pnl < 0 {
					lossesAbs += -pnl
					consecutiveLosses++
					if consecutiveLosses >= 3 {
						pauseBars = 6
						consecutiveLosses = 0
					}
				}
				tradeCount++
			}
		}
		if shortPos.active {
			pnl, closed := 0.0, false
			if shortPos.stopLoss == 0 {
				pnl, closed = stepRangeShortPosition(cfg, &shortPos, cur.Close, currentRegime, rangeBand, rangeOK)
			} else {
				pnl, closed = stepShortPositionV8(cfg, &shortPos, cur.Close, h4Slice)
			}
			if closed {
				net += pnl
				monthly[t.Format("2006-01")] += pnl / capital
				dailyPnL[dayKey] += pnl
				weeklyPnL[weekKey] += pnl
				if pnl > 0 {
					wins++
					profits += pnl
					consecutiveLosses = 0
				} else if pnl < 0 {
					lossesAbs += -pnl
					consecutiveLosses++
					if consecutiveLosses >= 3 {
						pauseBars = 6
						consecutiveLosses = 0
					}
				}
				tradeCount++
			}
		}

		equity := net
		if longPos.active {
			equity += longPos.realizedPart + (cur.Close-longPos.entryPrice)*longPos.qtyRemain
		}
		if shortPos.active {
			equity += shortPos.realizedPart + (shortPos.entryPrice-cur.Close)*shortPos.qtyRemain
		}
		if equity > peak {
			peak = equity
		}
		if dd := peak - equity; dd > maxDD {
			maxDD = dd
		}
	}

	winRate := 0.0
	if tradeCount > 0 {
		winRate = float64(wins) / float64(tradeCount)
	}
	profitFactor := 0.0
	if lossesAbs > 0 {
		profitFactor = profits / lossesAbs
	}
	sharpe := 0.0
	calmar := 0.0
	if len(monthly) >= 2 {
		vals := make([]float64, 0, len(monthly))
		for _, v := range monthly {
			vals = append(vals, v)
		}
		mean, std := meanStd(vals)
		if std > 0 {
			sharpe = mean / std * math.Sqrt(12)
		}
	}
	if maxDD > 0 {
		calmar = net / maxDD
	}
	return ReplaySummary{Profile: "v9", TradeCount: tradeCount, WinRate: winRate, ProfitFactor: profitFactor, MaxDrawdown: maxDD, NetPnL: net, MonthlyReturns: monthly, Sharpe: sharpe, Calmar: calmar, RegimeSwitches: switchCounts, RegimeTimeframe: "1w"}, nil
}

func runV10Replay(cfg Config, dayData, h4Data []models.KLineRecord, start, end time.Time) (ReplaySummary, error) {
	capital := cfg.RiskCapitalUSDT
	if capital <= 0 {
		return ReplaySummary{}, fmt.Errorf("invalid capital for v10")
	}
	longNotional := capital * 0.70
	shortNotional := capital * 0.70 * 2.0

	var wins int
	var profits float64
	var lossesAbs float64
	var net float64
	var peak float64
	var maxDD float64
	monthly := map[string]float64{}
	tradeCount := 0

	currentPos := regimeRange
	entryPrice := 0.0
	qty := 0.0
	currentRegime := regimeRange
	lastRegimeDay := ""

	closePos := func(price float64, t time.Time) {
		if currentPos == regimeRange || qty <= 0 {
			return
		}
		pnl := 0.0
		if currentPos == regimeBull {
			pnl = (price - entryPrice) * qty
		} else if currentPos == regimeBear {
			pnl = (entryPrice - price) * qty
		}
		net += pnl
		monthly[t.Format("2006-01")] += pnl / capital
		tradeCount++
		if pnl > 0 {
			wins++
			profits += pnl
		} else if pnl < 0 {
			lossesAbs += -pnl
		}
		currentPos = regimeRange
		entryPrice = 0
		qty = 0
	}

	for i := range h4Data {
		cur := h4Data[i]
		t := cur.OpenTime.UTC()
		if t.Before(start) || (!end.IsZero() && t.After(end)) {
			continue
		}

		daySlice := make([]models.KLineRecord, 0, 512)
		for _, d := range dayData {
			if d.OpenTime.UTC().After(t) {
				break
			}
			daySlice = append(daySlice, d)
		}
		if len(daySlice) > 0 {
			dayKey := daySlice[len(daySlice)-1].OpenTime.UTC().Format("2006-01-02")
			if dayKey != lastRegimeDay {
				lastRegimeDay = dayKey
				regime, err := detectV10RegimeFromStartpoints(daySlice, t)
				if err != nil {
					return ReplaySummary{}, err
				}
				currentRegime = regime
			}
		}

		if currentRegime == regimeBull {
			if currentPos == regimeBear {
				closePos(cur.Close, t)
			}
			if currentPos != regimeBull {
				if cur.Close > 0 {
					currentPos = regimeBull
					entryPrice = cur.Close
					qty = longNotional / cur.Close
				}
			}
		} else if currentRegime == regimeBear {
			if currentPos == regimeBull {
				closePos(cur.Close, t)
			}
			if currentPos != regimeBear {
				if cur.Close > 0 {
					currentPos = regimeBear
					entryPrice = cur.Close
					qty = shortNotional / cur.Close
				}
			}
		} else {
			if currentPos != regimeRange {
				closePos(cur.Close, t)
			}
		}

		equity := net
		if currentPos == regimeBull {
			equity += (cur.Close - entryPrice) * qty
		} else if currentPos == regimeBear {
			equity += (entryPrice - cur.Close) * qty
		}
		if equity > peak {
			peak = equity
		}
		if dd := peak - equity; dd > maxDD {
			maxDD = dd
		}
	}

	if currentPos != regimeRange {
		last := h4Data[len(h4Data)-1]
		closePos(last.Close, last.OpenTime.UTC())
	}

	winRate := 0.0
	if tradeCount > 0 {
		winRate = float64(wins) / float64(tradeCount)
	}
	profitFactor := 0.0
	if lossesAbs > 0 {
		profitFactor = profits / lossesAbs
	}
	sharpe := 0.0
	calmar := 0.0
	if len(monthly) >= 2 {
		vals := make([]float64, 0, len(monthly))
		for _, v := range monthly {
			vals = append(vals, v)
		}
		mean, std := meanStd(vals)
		if std > 0 {
			sharpe = mean / std * math.Sqrt(12)
		}
	}
	if maxDD > 0 {
		calmar = net / maxDD
	}
	return ReplaySummary{Profile: "v10", TradeCount: tradeCount, WinRate: winRate, ProfitFactor: profitFactor, MaxDrawdown: maxDD, NetPnL: net, MonthlyReturns: monthly, Sharpe: sharpe, Calmar: calmar, RegimeTimeframe: "realtime_startpoints"}, nil
}

func runV11Replay(cfg Config, profile string, dayData, h4Data []models.KLineRecord, start, end time.Time) (ReplaySummary, error) {
	capital := cfg.RiskCapitalUSDT
	if capital <= 0 {
		return ReplaySummary{}, fmt.Errorf("invalid capital for v11")
	}
	if cfg.BullAllocMin <= 0 {
		cfg.BullAllocMin = 0.70
	}
	if cfg.BullAllocMax < cfg.BullAllocMin {
		cfg.BullAllocMax = cfg.BullAllocMin
	}
	if cfg.BullAllocMax > 0.95 {
		cfg.BullAllocMax = 0.95
	}
	if cfg.BullSignalFloor <= 0 {
		cfg.BullSignalFloor = 0.50
	}
	if cfg.RangeBaseAlloc <= 0 {
		cfg.RangeBaseAlloc = 0.20
	}
	if cfg.RangeTrendAllocBoost < 0 {
		cfg.RangeTrendAllocBoost = 0
	}
	if cfg.BearSignalFloor <= 0 {
		cfg.BearSignalFloor = 0.80
	}
	if cfg.BearLeverageCap <= 0 {
		cfg.BearLeverageCap = 1.5
	}
	if cfg.BearMomentumFloor == 0 {
		cfg.BearMomentumFloor = -0.03
	}
	if cfg.RegimeMinConfirmBars <= 0 {
		cfg.RegimeMinConfirmBars = 1
	}
	if cfg.RegimeMinStayBars <= 0 {
		cfg.RegimeMinStayBars = 1
	}
	if cfg.RegimeHysteresis < 0 {
		cfg.RegimeHysteresis = 0
	}
	if cfg.RangeCostGuardATR <= 0 {
		cfg.RangeCostGuardATR = 1.0
	}
	if cfg.CostPerSidePct < 0 {
		cfg.CostPerSidePct = 0
	}
	if cfg.ShortFundingAPR < 0 {
		cfg.ShortFundingAPR = 0
	}

	isV13 := profile == "v13" || profile == "v13_a" || profile == "v13_b"
	bullCoreExitFloor := cfg.BullSignalFloor - 0.10
	if bullCoreExitFloor < 0.40 {
		bullCoreExitFloor = 0.40
	}
	bullAddOnFloor := cfg.MinSignalStrength
	if bullAddOnFloor <= 0 {
		bullAddOnFloor = cfg.BullSignalFloor + 0.20
	}
	if bullAddOnFloor < cfg.BullSignalFloor+0.10 {
		bullAddOnFloor = cfg.BullSignalFloor + 0.10
	}
	if bullAddOnFloor > 0.92 {
		bullAddOnFloor = 0.92
	}

	var wins int
	var profits, lossesAbs, net, peak, maxDD float64
	monthly := map[string]float64{}
	tradeCount := 0
	entryPrice := 0.0
	qty := 0.0
	entryTime := time.Time{}
	posRegime := marketRegime("")
	stopPrice := 0.0
	trailPrice := 0.0
	bullCoreQty := 0.0

	currentRegime := regimeRange
	pendingRegime := regimeRange
	pendingRegimeDays := 0
	regimeDaysSinceSwitch := 999
	lastRegimeDay := ""
	weekEntries := map[string]int{}
	cooldown := 0

	closePos := func(price float64, t time.Time) {
		if posRegime == "" || qty <= 0 {
			return
		}
		pnl := 0.0
		if posRegime == regimeBull || posRegime == regimeRange {
			pnl = (price - entryPrice) * qty
		} else if posRegime == regimeBear {
			pnl = (entryPrice - price) * qty
		}
		openNotional := math.Abs(entryPrice * qty)
		closeNotional := math.Abs(price * qty)
		pnl -= (openNotional + closeNotional) * cfg.CostPerSidePct
		if posRegime == regimeBear && !entryTime.IsZero() && cfg.ShortFundingAPR > 0 {
			hours := t.Sub(entryTime).Hours()
			if hours > 0 {
				pnl -= openNotional * cfg.ShortFundingAPR * (hours / (24.0 * 365.0))
			}
		}
		net += pnl
		monthly[t.Format("2006-01")] += pnl / capital
		tradeCount++
		if pnl > 0 {
			wins++
			profits += pnl
		} else if pnl < 0 {
			lossesAbs += -pnl
		}
		entryPrice, qty, stopPrice, trailPrice = 0, 0, 0, 0
		entryTime = time.Time{}
		posRegime = ""
		bullCoreQty = 0
	}

	for i := range h4Data {
		cur := h4Data[i]
		t := cur.OpenTime.UTC()
		if t.Before(start) || (!end.IsZero() && t.After(end)) {
			continue
		}
		h4Slice := make([]OHLC, 0, i+1)
		for j := 0; j <= i; j++ {
			h4Slice = append(h4Slice, OHLC{Close: h4Data[j].Close, High: h4Data[j].High, Low: h4Data[j].Low, Volume: h4Data[j].Volume})
		}
		daySlice := make([]OHLC, 0, 512)
		dayBars := make([]models.KLineRecord, 0, 512)
		for _, d := range dayData {
			if d.OpenTime.UTC().After(t) {
				break
			}
			daySlice = append(daySlice, OHLC{Close: d.Close, High: d.High, Low: d.Low, Volume: d.Volume})
			dayBars = append(dayBars, d)
		}
		if len(daySlice) < cfg.EMAFilterPeriod+cfg.EMA200SlopeLookback+2 || len(h4Slice) < cfg.ATRPeriod+3 {
			continue
		}

		dayKey := dayBars[len(dayBars)-1].OpenTime.UTC().Format("2006-01-02")
		if dayKey != lastRegimeDay {
			lastRegimeDay = dayKey
			regimeDaysSinceSwitch++
			cand, err := detectV10RegimeFromStartpoints(dayBars, t)
			if err != nil {
				return ReplaySummary{}, err
			}
			if cand == currentRegime {
				pendingRegime = cand
				pendingRegimeDays = 0
			} else {
				strength := regimeSwitchStrength(cfg, cand, daySlice, h4Slice)
				if regimeDaysSinceSwitch >= cfg.RegimeMinStayBars && strength >= cfg.RegimeHysteresis {
					if pendingRegime != cand {
						pendingRegime = cand
						pendingRegimeDays = 1
					} else {
						pendingRegimeDays++
					}
					if pendingRegimeDays >= cfg.RegimeMinConfirmBars {
						currentRegime = cand
						pendingRegimeDays = 0
						regimeDaysSinceSwitch = 0
					}
				} else {
					pendingRegime = cand
					pendingRegimeDays = 0
				}
			}
		}

		y, w := t.ISOWeek()
		weekKey := fmt.Sprintf("%d-W%02d", y, w)
		if cooldown > 0 {
			cooldown--
		}
		if cfg.MaxEntriesPerWeek <= 0 {
			cfg.MaxEntriesPerWeek = 4
		}

		if posRegime == regimeBull {
			atr := lastATR(h4Slice, cfg.ATRPeriod)
			cand := cur.Close - atr*cfg.TrailATRMultiplier
			if cand > trailPrice {
				trailPrice = cand
			}
			bullScoreNow := longSignalStrength(cfg, daySlice, h4Slice)
			shouldExit := cur.Close <= trailPrice || cur.Close <= stopPrice
			if !isV13 {
				shouldExit = shouldExit || currentRegime != regimeBull
			} else {
				if currentRegime == regimeBear {
					shouldExit = true
				}
				if currentRegime == regimeRange && bullScoreNow < bullCoreExitFloor {
					shouldExit = true
				}
				if qty > bullCoreQty && bullScoreNow < bullAddOnFloor-0.06 {
					shouldExit = true
				}
				if bullScoreNow < bullCoreExitFloor-0.04 {
					shouldExit = true
				}
			}
			if shouldExit {
				closePos(cur.Close, t)
			}
		} else if posRegime == regimeBear {
			atr := lastATR(h4Slice, cfg.ATRPeriod)
			cand := cur.Close + atr*cfg.TrailATRMultiplier
			if trailPrice == 0 || cand < trailPrice {
				trailPrice = cand
			}
			if currentRegime != regimeBear || cur.Close >= trailPrice || cur.Close >= stopPrice {
				closePos(cur.Close, t)
			}
		} else if posRegime == regimeRange {
			if currentRegime != regimeRange {
				closePos(cur.Close, t)
			}
			if cfg.RangeExitEMABufferATR > 0 {
				emaFast := indicator.LastEMA(extractCloses(h4Slice), cfg.EMAPullback)
				atr := lastATR(h4Slice, cfg.ATRPeriod)
				if emaFast > 0 && atr > 0 && cur.Close < emaFast-atr*cfg.RangeExitEMABufferATR {
					closePos(cur.Close, t)
				}
			}
		}

		if posRegime != "" || cooldown > 0 || weekEntries[weekKey] >= cfg.MaxEntriesPerWeek {
			goto equity
		}

		switch currentRegime {
		case regimeBull:
			bullScore := longSignalStrength(cfg, daySlice, h4Slice)
			if bullScore < cfg.BullSignalFloor {
				break
			}
			alloc := cfg.BullAllocMin
			if alloc < 0 {
				alloc = 0
			}
			if alloc > cfg.BullAllocMax {
				alloc = cfg.BullAllocMax
			}
			if !isV13 {
				norm := (bullScore - cfg.BullSignalFloor) / (1 - cfg.BullSignalFloor)
				if norm < 0 {
					norm = 0
				}
				if norm > 1 {
					norm = 1
				}
				alloc = cfg.BullAllocMin + (cfg.BullAllocMax-cfg.BullAllocMin)*norm
			} else if bullScore >= bullAddOnFloor && cfg.BullAllocMax > cfg.BullAllocMin {
				norm := (bullScore - bullAddOnFloor) / (1 - bullAddOnFloor)
				if norm < 0 {
					norm = 0
				}
				if norm > 1 {
					norm = 1
				}
				alloc = cfg.BullAllocMin + (cfg.BullAllocMax-cfg.BullAllocMin)*norm
			}
			notional := capital * alloc
			if cur.Close <= 0 || notional <= 0 {
				break
			}
			entryPrice = cur.Close
			qty = notional / cur.Close
			if isV13 {
				bullCoreQty = (capital * cfg.BullAllocMin) / cur.Close
				if bullCoreQty > qty {
					bullCoreQty = qty
				}
			}
			entryTime = t
			posRegime = regimeBull
			atr := lastATR(h4Slice, cfg.ATRPeriod)
			stopPrice = cur.Close - atr*1.8
			trailPrice = cur.Close - atr*cfg.TrailATRMultiplier
			cooldown = cfg.CooldownBars
			weekEntries[weekKey]++
		case regimeRange:
			alloc := cfg.RangeBaseAlloc
			if isRangeTrendSupport(cfg, daySlice, h4Slice) {
				alloc += cfg.RangeTrendAllocBoost
			}
			if alloc <= 0 || alloc > 0.45 || cur.Close <= 0 {
				break
			}
			atr := lastATR(h4Slice, cfg.ATRPeriod)
			if atr <= 0 {
				break
			}
			if cfg.CostPerSidePct > 0 {
				estEdge := atr * cfg.RangeCostGuardATR
				minEdge := cur.Close * cfg.CostPerSidePct * 2.0 * 1.10
				if estEdge < minEdge {
					break
				}
			}
			entryPrice = cur.Close
			qty = (capital * alloc) / cur.Close
			entryTime = t
			posRegime = regimeRange
			stopPrice = cur.Close - atr*1.5
			trailPrice = stopPrice
			cooldown = cfg.CooldownBars
			weekEntries[weekKey]++
		case regimeBear:
			if cfg.BearRequireQuality && !bearQualityFilter(cfg, daySlice, h4Slice) {
				break
			}
			bearScore := shortSignalStrength(cfg, daySlice, h4Slice)
			if bearScore < cfg.BearSignalFloor {
				break
			}
			mom := recentMomentum(daySlice, 5)
			if mom > cfg.BearMomentumFloor {
				break
			}
			lev := cfg.BearLeverage
			if lev <= 0 {
				lev = 1
			}
			if lev > cfg.BearLeverageCap {
				lev = cfg.BearLeverageCap
			}
			alloc := math.Min(0.75, 0.35+(bearScore-cfg.BearSignalFloor)*1.4)
			notional := capital * alloc * lev
			if cur.Close <= 0 || notional <= 0 {
				break
			}
			entryPrice = cur.Close
			qty = notional / cur.Close
			entryTime = t
			posRegime = regimeBear
			atr := lastATR(h4Slice, cfg.ATRPeriod)
			stopPrice = cur.Close + atr*1.6
			trailPrice = cur.Close + atr*cfg.TrailATRMultiplier
			cooldown = cfg.CooldownBars
			weekEntries[weekKey]++
		}

	equity:
		equity := net
		if posRegime == regimeBull || posRegime == regimeRange {
			equity += (cur.Close - entryPrice) * qty
		} else if posRegime == regimeBear {
			equity += (entryPrice - cur.Close) * qty
		}
		if equity > peak {
			peak = equity
		}
		if dd := peak - equity; dd > maxDD {
			maxDD = dd
		}
	}

	if posRegime != "" {
		last := h4Data[len(h4Data)-1]
		closePos(last.Close, last.OpenTime.UTC())
	}

	winRate := 0.0
	if tradeCount > 0 {
		winRate = float64(wins) / float64(tradeCount)
	}
	profitFactor := 0.0
	if lossesAbs > 0 {
		profitFactor = profits / lossesAbs
	}
	sharpe := 0.0
	calmar := 0.0
	if len(monthly) >= 2 {
		vals := make([]float64, 0, len(monthly))
		for _, v := range monthly {
			vals = append(vals, v)
		}
		mean, std := meanStd(vals)
		if std > 0 {
			sharpe = mean / std * math.Sqrt(12)
		}
	}
	if maxDD > 0 {
		calmar = net / maxDD
	}
	return ReplaySummary{Profile: profile, TradeCount: tradeCount, WinRate: winRate, ProfitFactor: profitFactor, MaxDrawdown: maxDD, NetPnL: net, MonthlyReturns: monthly, Sharpe: sharpe, Calmar: calmar, RegimeTimeframe: "realtime_startpoints"}, nil
}

func extractCloses(h4 []OHLC) []float64 {
	out := make([]float64, 0, len(h4))
	for _, k := range h4 {
		out = append(out, k.Close)
	}
	return out
}

func recentMomentum(day []OHLC, lookback int) float64 {
	if len(day) < lookback+1 || lookback <= 0 {
		return 0
	}
	last := day[len(day)-1].Close
	base := day[len(day)-1-lookback].Close
	if base <= 0 {
		return 0
	}
	return last/base - 1
}

func regimeSwitchStrength(cfg Config, cand marketRegime, day, h4 []OHLC) float64 {
	switch cand {
	case regimeBull:
		return longSignalStrength(cfg, day, h4)
	case regimeBear:
		return shortSignalStrength(cfg, day, h4)
	case regimeRange:
		if len(day) < cfg.EMAFilterPeriod+1 {
			return 0
		}
		closes := make([]float64, 0, len(day))
		for _, d := range day {
			closes = append(closes, d.Close)
		}
		ema := indicator.LastEMA(closes, cfg.EMAFilterPeriod)
		if ema <= 0 {
			return 0
		}
		last := closes[len(closes)-1]
		emaDist := math.Abs(last-ema) / ema
		mom := math.Abs(recentMomentum(day, 5))
		raw := 1.0 - math.Min(1.0, mom*8.0+emaDist*6.0)
		if raw < 0 {
			return 0
		}
		return raw
	default:
		return 0
	}
}

func isRangeTrendSupport(cfg Config, day, h4 []OHLC) bool {
	if len(day) < cfg.EMAFilterPeriod+1 || len(h4) < cfg.EMAPullback+2 {
		return false
	}
	dayCloses := make([]float64, 0, len(day))
	for _, d := range day {
		dayCloses = append(dayCloses, d.Close)
	}
	emaDay := indicator.LastEMA(dayCloses, cfg.EMAFilterPeriod)
	emaFast := indicator.LastEMA(extractCloses(h4), cfg.EMAPullback)
	last := h4[len(h4)-1].Close
	return last > emaDay && last > emaFast
}

func detectV10RegimeFromStartpoints(dayData []models.KLineRecord, at time.Time) (marketRegime, error) {
	weekly := aggregate1dTo1w(dayData)
	if len(dayData) == 0 || len(weekly) == 0 {
		return regimeRange, nil
	}
	dailyBars := make([]indicator.RegimeBar, 0, len(dayData))
	for _, d := range dayData {
		dailyBars = append(dailyBars, indicator.RegimeBar{Time: d.OpenTime.UTC(), Close: d.Close})
	}
	weeklyBars := make([]indicator.RegimeBar, 0, len(weekly))
	for _, w := range weekly {
		weeklyBars = append(weeklyBars, indicator.RegimeBar{Time: w.OpenTime.UTC(), Close: w.Close})
	}
	points := indicator.ComputeRegimeStartpoints(dailyBars, weeklyBars)
	state := regimeRange
	for _, p := range points {
		if p.Time.After(at) {
			break
		}
		switch p.State {
		case "BULL":
			state = regimeBull
		case "BEAR":
			state = regimeBear
		default:
			state = regimeRange
		}
	}
	return state, nil
}

type rangeBand struct {
	Upper float64
	Lower float64
	Mid   float64
	ATR   float64
}

func detectRangeBand(cfg Config, day, h4 []OHLC) (rangeBand, bool) {
	if len(day) < cfg.RangeLookbackDays || len(h4) < cfg.ATRPeriod+2 {
		return rangeBand{}, false
	}
	upper := day[len(day)-cfg.RangeLookbackDays].High
	lower := day[len(day)-cfg.RangeLookbackDays].Low
	for _, d := range day[len(day)-cfg.RangeLookbackDays:] {
		if d.High > upper {
			upper = d.High
		}
		if d.Low < lower {
			lower = d.Low
		}
	}
	atr := lastATR(h4, cfg.ATRPeriod)
	if atr <= 0 || upper <= lower {
		return rangeBand{}, false
	}
	if (upper-lower)/atr < cfg.RangeMinWidthATR {
		return rangeBand{}, false
	}
	return rangeBand{Upper: upper, Lower: lower, Mid: (upper + lower) / 2, ATR: atr}, true
}

func rangeLongSignalStrength(cfg Config, price float64, b rangeBand) float64 {
	if b.ATR <= 0 {
		return 0
	}
	dist := (price - b.Lower) / b.ATR
	if dist > cfg.RangeEntryATRBuffer {
		return 0
	}
	proximity := 1 - math.Min(1, dist/(cfg.RangeEntryATRBuffer+1e-9))
	widthScore := math.Min(1, (b.Upper-b.Lower)/(b.ATR*4.0))
	return 0.7*proximity + 0.3*widthScore
}

func rangeShortSignalStrength(cfg Config, price float64, b rangeBand) float64 {
	if b.ATR <= 0 {
		return 0
	}
	dist := (b.Upper - price) / b.ATR
	if dist > cfg.RangeEntryATRBuffer {
		return 0
	}
	proximity := 1 - math.Min(1, dist/(cfg.RangeEntryATRBuffer+1e-9))
	widthScore := math.Min(1, (b.Upper-b.Lower)/(b.ATR*4.0))
	return 0.7*proximity + 0.3*widthScore
}

func stepRangeLongPosition(cfg Config, pos *replayPosition, lastPrice float64, regime marketRegime, b rangeBand, rangeOK bool) (float64, bool) {
	if !pos.active {
		return 0, false
	}
	pos.holdBars++
	if !pos.tp1Done && lastPrice >= b.Mid {
		qty := pos.qtyTotal * cfg.RangeMidExitRatio
		if qty > pos.qtyRemain {
			qty = pos.qtyRemain
		}
		pos.qtyRemain -= qty
		pos.realizedPart += (lastPrice - pos.entryPrice) * qty
		pos.tp1Done = true
	}
	exitByBreakout := lastPrice <= pos.trailStop
	exitByRegime := regime != regimeRange
	exitByOppEdge := rangeOK && lastPrice >= b.Upper-b.ATR*0.2
	exitByTimeout := pos.holdBars >= cfg.RangeMaxHoldBars
	if exitByBreakout || exitByRegime || exitByOppEdge || exitByTimeout {
		pnl := pos.realizedPart + (lastPrice-pos.entryPrice)*pos.qtyRemain
		*pos = replayPosition{}
		return pnl, true
	}
	return 0, false
}

func stepRangeShortPosition(cfg Config, pos *replayShortPosition, lastPrice float64, regime marketRegime, b rangeBand, rangeOK bool) (float64, bool) {
	if !pos.active {
		return 0, false
	}
	pos.holdBars++
	if !pos.tp1Done && lastPrice <= b.Mid {
		qty := pos.qtyTotal * cfg.RangeMidExitRatio
		if qty > pos.qtyRemain {
			qty = pos.qtyRemain
		}
		pos.qtyRemain -= qty
		pos.realizedPart += (pos.entryPrice - lastPrice) * qty
		pos.tp1Done = true
	}
	exitByBreakout := lastPrice >= pos.trailStop
	exitByRegime := regime != regimeRange
	exitByOppEdge := rangeOK && lastPrice <= b.Lower+b.ATR*0.2
	exitByTimeout := pos.holdBars >= cfg.RangeMaxHoldBars
	if exitByBreakout || exitByRegime || exitByOppEdge || exitByTimeout {
		pnl := pos.realizedPart + (pos.entryPrice-lastPrice)*pos.qtyRemain
		*pos = replayShortPosition{}
		return pnl, true
	}
	return 0, false
}

func longSignalStrength(cfg Config, day, h4 []OHLC) float64 {
	if len(day) < cfg.EMAFilterPeriod+cfg.EMA200SlopeLookback+1 || len(h4) < cfg.VolumeSMAPeriod+2 {
		return 0
	}
	dayCloses, h4Closes, h4Highs, h4Lows, h4Volumes := splitOHLC(day, h4)
	dayEMA := indicator.EMA(dayCloses, cfg.EMAFilterPeriod)
	slopeBase := dayEMA[len(dayEMA)-1-cfg.EMA200SlopeLookback]
	if slopeBase <= 0 {
		return 0
	}
	slope := (dayEMA[len(dayEMA)-1] - slopeBase) / slopeBase
	atr := indicator.LastATR(h4Highs, h4Lows, h4Closes, cfg.ATRPeriod)
	volRatio := h4Volumes[len(h4Volumes)-1] / indicator.LastSMA(h4Volumes, cfg.VolumeSMAPeriod)
	atrPct := 0.0
	if last := h4[len(h4)-1].Close; last > 0 {
		atrPct = atr / last
	}
	score := 0.0
	if slope >= cfg.EMA200MinSlope {
		score += 0.4
	}
	if volRatio >= cfg.VolumeMinRatio {
		score += 0.3
	}
	if atrPct >= cfg.ATRMinPct && atrPct <= cfg.ATRMaxPct {
		score += 0.3
	}
	return score
}

func shortSignalStrength(cfg Config, day, h4 []OHLC) float64 {
	if len(day) < cfg.EMAFilterPeriod+cfg.EMA200SlopeLookback+1 || len(h4) < cfg.VolumeSMAPeriod+2 {
		return 0
	}
	dayCloses, h4Closes, h4Highs, h4Lows, h4Volumes := splitOHLC(day, h4)
	dayEMA := indicator.EMA(dayCloses, cfg.EMAFilterPeriod)
	slopeBase := dayEMA[len(dayEMA)-1-cfg.EMA200SlopeLookback]
	if slopeBase <= 0 {
		return 0
	}
	slope := (dayEMA[len(dayEMA)-1] - slopeBase) / slopeBase
	atr := indicator.LastATR(h4Highs, h4Lows, h4Closes, cfg.ATRPeriod)
	volRatio := h4Volumes[len(h4Volumes)-1] / indicator.LastSMA(h4Volumes, cfg.VolumeSMAPeriod)
	atrPct := 0.0
	if last := h4[len(h4)-1].Close; last > 0 {
		atrPct = atr / last
	}
	score := 0.0
	if slope <= -cfg.EMA200MinSlope {
		score += 0.4
	}
	if volRatio >= cfg.VolumeMinRatio {
		score += 0.3
	}
	if atrPct >= cfg.ATRMinPct && atrPct <= cfg.ATRMaxPct {
		score += 0.3
	}
	return score
}

func bearQualityFilter(cfg Config, day, h4 []OHLC) bool {
	if len(day) < cfg.EMAFilterPeriod+cfg.EMA200SlopeLookback+3 || len(h4) < 30 {
		return false
	}
	dayCloses := make([]float64, 0, len(day))
	for _, d := range day {
		dayCloses = append(dayCloses, d.Close)
	}
	ema := indicator.EMA(dayCloses, cfg.EMAFilterPeriod)
	last := dayCloses[len(dayCloses)-1]
	prev := dayCloses[len(dayCloses)-6]
	slopeBase := ema[len(ema)-1-cfg.EMA200SlopeLookback]
	if slopeBase <= 0 {
		return false
	}
	slope := (ema[len(ema)-1] - slopeBase) / slopeBase
	mom := last/prev - 1
	return last < ema[len(ema)-1] && slope <= -cfg.EMA200MinSlope*0.8 && mom < -0.03
}

func stepLongPositionV8(cfg Config, pos *replayPosition, lastPrice float64, h4 []OHLC) (float64, bool) {
	if !pos.active {
		return 0, false
	}
	pos.holdBars++
	tp1 := pos.entryPrice + pos.riskPerUnit
	if !pos.tp1Done && lastPrice >= tp1 {
		qty := pos.qtyTotal * 0.25
		pos.qtyRemain -= qty
		pos.tp1Done = true
		pos.realizedPart += (lastPrice - pos.entryPrice) * qty
		pos.trailStop = pos.entryPrice
	}
	trail := pos.trailStop
	if len(h4) >= cfg.ATRPeriod+2 {
		closes := make([]float64, 0, len(h4))
		highs := make([]float64, 0, len(h4))
		lows := make([]float64, 0, len(h4))
		for _, b := range h4 {
			closes = append(closes, b.Close)
			highs = append(highs, b.High)
			lows = append(lows, b.Low)
		}
		emaFast := indicator.LastEMA(closes, cfg.EMAPullback)
		atr := indicator.LastATR(highs, lows, closes, cfg.ATRPeriod)
		if atr > 0 && emaFast > 0 {
			cand := emaFast - atr*cfg.TrailATRMultiplier
			if cand > trail {
				trail = cand
			}
		}
	}
	if trail <= 0 {
		trail = pos.stopLoss
	}
	if lastPrice <= trail || (cfg.MaxHoldBars > 0 && pos.holdBars >= cfg.MaxHoldBars) {
		pnl := pos.realizedPart + (lastPrice-pos.entryPrice)*pos.qtyRemain
		*pos = replayPosition{}
		return pnl, true
	}
	pos.trailStop = trail
	return 0, false
}

func stepShortPositionV8(cfg Config, pos *replayShortPosition, lastPrice float64, h4 []OHLC) (float64, bool) {
	if !pos.active {
		return 0, false
	}
	pos.holdBars++
	tp1 := pos.entryPrice - pos.riskPerUnit
	if !pos.tp1Done && lastPrice <= tp1 {
		qty := pos.qtyTotal * 0.25
		pos.qtyRemain -= qty
		pos.tp1Done = true
		pos.realizedPart += (pos.entryPrice - lastPrice) * qty
		pos.trailStop = pos.entryPrice
	}
	trail := pos.trailStop
	if len(h4) >= cfg.ATRPeriod+2 {
		closes := make([]float64, 0, len(h4))
		highs := make([]float64, 0, len(h4))
		lows := make([]float64, 0, len(h4))
		for _, b := range h4 {
			closes = append(closes, b.Close)
			highs = append(highs, b.High)
			lows = append(lows, b.Low)
		}
		emaFast := indicator.LastEMA(closes, cfg.EMAPullback)
		atr := indicator.LastATR(highs, lows, closes, cfg.ATRPeriod)
		if atr > 0 && emaFast > 0 {
			cand := emaFast + atr*cfg.TrailATRMultiplier
			if trail == 0 || cand < trail {
				trail = cand
			}
		}
	}
	if trail <= 0 {
		trail = pos.stopLoss
	}
	if lastPrice >= trail || (cfg.MaxHoldBars > 0 && pos.holdBars >= cfg.MaxHoldBars) {
		pnl := pos.realizedPart + (pos.entryPrice-lastPrice)*pos.qtyRemain
		*pos = replayShortPosition{}
		return pnl, true
	}
	pos.trailStop = trail
	return 0, false
}

func detectRegime(cfg Config, day []OHLC) marketRegime {
	closes := make([]float64, 0, len(day))
	highs := make([]float64, 0, len(day))
	lows := make([]float64, 0, len(day))
	for _, d := range day {
		closes = append(closes, d.Close)
		highs = append(highs, d.High)
		lows = append(lows, d.Low)
	}
	emaSeries := indicator.EMA(closes, cfg.EMAFilterPeriod)
	if len(emaSeries) < cfg.EMA200SlopeLookback+1 {
		return regimeRange
	}
	ema := emaSeries[len(emaSeries)-1]
	base := emaSeries[len(emaSeries)-1-cfg.EMA200SlopeLookback]
	if base <= 0 {
		return regimeRange
	}
	slope := (ema - base) / base
	atr := indicator.LastATR(highs, lows, closes, cfg.RegimeVolPeriod)
	if atr <= 0 || closes[len(closes)-1] <= 0 {
		return regimeRange
	}
	atrPct := atr / closes[len(closes)-1]
	if atrPct <= cfg.RegimeVolConvergePct {
		return regimeRange
	}
	price := closes[len(closes)-1]
	if price > ema && slope >= cfg.EMA200MinSlope {
		return regimeBull
	}
	if price < ema && slope <= -cfg.EMA200MinSlope {
		return regimeBear
	}
	return regimeRange
}

func evalTrendShort(cfg Config, day []OHLC, h4 []OHLC) (SignalResult, error) {
	if len(day) < cfg.EMAFilterPeriod+cfg.EMA200SlopeLookback || len(h4) < cfg.ATRPeriod+5 {
		return SignalResult{Reason: "insufficient_kline"}, nil
	}
	dayCloses, h4Closes, h4Highs, h4Lows, h4Volumes := splitOHLC(day, h4)
	dayEMA := indicator.EMA(dayCloses, cfg.EMAFilterPeriod)
	dEMA := dayEMA[len(dayEMA)-1]
	slopeBase := dayEMA[len(dayEMA)-1-cfg.EMA200SlopeLookback]
	if slopeBase <= 0 {
		return SignalResult{Reason: "invalid_slope_base"}, nil
	}
	emaSlope := (dEMA - slopeBase) / slopeBase
	h4EMA := indicator.LastEMA(h4Closes, cfg.EMAFilterPeriod)
	emaFast := indicator.EMA(h4Closes, cfg.EMAPullback)
	atr := indicator.LastATR(h4Highs, h4Lows, h4Closes, cfg.ATRPeriod)
	volSMA := indicator.LastSMA(h4Volumes, cfg.VolumeSMAPeriod)
	if len(emaFast) < 2 || atr <= 0 || volSMA <= 0 {
		return SignalResult{Reason: "ind_invalid"}, nil
	}
	last, prev := h4[len(h4)-1], h4[len(h4)-2]
	emaNow, emaPrev := emaFast[len(emaFast)-1], emaFast[len(emaFast)-2]
	atrPct := atr / last.Close
	volRatio := last.Volume / volSMA

	trendOK := last.Close < dEMA && last.Close < h4EMA
	slopeOK := emaSlope <= -cfg.EMA200MinSlope
	pullback := prev.Close >= emaPrev && prev.High >= emaPrev*(1-cfg.PullbackTol)
	confirm := last.Close < emaNow
	entryTrigger := pullback && confirm
	volatilityOK := atrPct >= cfg.ATRMinPct && atrPct <= cfg.ATRMaxPct
	volumeOK := volRatio >= cfg.VolumeMinRatio
	if !(trendOK && slopeOK && entryTrigger && volatilityOK && volumeOK) {
		return SignalResult{Reason: "bear_filter_fail"}, nil
	}
	stop := last.Close + atr*cfg.TrailATRMultiplier
	if stop <= last.Close {
		return SignalResult{Reason: "invalid_stop"}, nil
	}
	return SignalResult{ShouldEnter: true, EntryPrice: last.Close, StopLoss: stop, Reason: "bear_pullback_short"}, nil
}

func buildShortPosition(cfg Config, entry, stop float64) PositionPlan {
	riskAmount := cfg.RiskCapitalUSDT * cfg.RiskPerTrade
	riskPerUnit := stop - entry
	if riskPerUnit <= 0 {
		return PositionPlan{}
	}
	qty := (riskAmount / riskPerUnit) * cfg.BearLeverage
	notional := qty * entry
	capLimit := cfg.RiskCapitalUSDT * cfg.BearLeverage
	if notional > capLimit {
		qty = capLimit / entry
		notional = qty * entry
	}
	return PositionPlan{RiskAmount: riskAmount, RiskPerUnit: riskPerUnit, Qty: math.Floor(qty*1e6) / 1e6, Notional: notional}
}

func stepLongPosition(cfg Config, pos *replayPosition, lastPrice float64, h4 []OHLC) (float64, bool) {
	if !pos.active {
		return 0, false
	}
	pos.holdBars++
	tp1 := pos.entryPrice + pos.riskPerUnit
	tp2 := pos.entryPrice + 2*pos.riskPerUnit
	if !pos.tp1Done && lastPrice >= tp1 {
		qty := pos.qtyTotal * 0.4
		pos.qtyRemain -= qty
		pos.tp1Done = true
		pos.trailStop = pos.entryPrice
		pos.realizedPart += (lastPrice - pos.entryPrice) * qty
	}
	if !pos.tp2Done && lastPrice >= tp2 {
		qty := pos.qtyTotal * 0.3
		pos.qtyRemain -= qty
		pos.tp2Done = true
		if pos.trailStop < tp1 {
			pos.trailStop = tp1
		}
		pos.realizedPart += (lastPrice - pos.entryPrice) * qty
	}
	trail := pos.trailStop
	if cfg.TrailATRMultiplier > 0 && pos.tp1Done && len(h4) >= cfg.ATRPeriod+1 {
		highs := make([]float64, 0, len(h4))
		lows := make([]float64, 0, len(h4))
		closes := make([]float64, 0, len(h4))
		for _, bar := range h4 {
			highs = append(highs, bar.High)
			lows = append(lows, bar.Low)
			closes = append(closes, bar.Close)
		}
		atr := indicator.LastATR(highs, lows, closes, cfg.ATRPeriod)
		if atr > 0 {
			atrTrail := lastPrice - atr*cfg.TrailATRMultiplier
			if atrTrail > trail {
				trail = atrTrail
			}
		}
	}
	if trail <= 0 {
		trail = pos.stopLoss
	}
	exitByStop := lastPrice <= trail
	exitByTimeout := cfg.MaxHoldBars > 0 && pos.holdBars >= cfg.MaxHoldBars
	if exitByStop || exitByTimeout {
		pnl := pos.realizedPart + (lastPrice-pos.entryPrice)*pos.qtyRemain
		*pos = replayPosition{}
		return pnl, true
	}
	return 0, false
}

func stepShortPosition(cfg Config, pos *replayShortPosition, lastPrice float64, h4 []OHLC) (float64, bool) {
	if !pos.active {
		return 0, false
	}
	pos.holdBars++
	tp1 := pos.entryPrice - pos.riskPerUnit
	tp2 := pos.entryPrice - 2*pos.riskPerUnit
	if !pos.tp1Done && lastPrice <= tp1 {
		qty := pos.qtyTotal * 0.4
		pos.qtyRemain -= qty
		pos.tp1Done = true
		pos.trailStop = pos.entryPrice
		pos.realizedPart += (pos.entryPrice - lastPrice) * qty
	}
	if !pos.tp2Done && lastPrice <= tp2 {
		qty := pos.qtyTotal * 0.3
		pos.qtyRemain -= qty
		pos.tp2Done = true
		if pos.trailStop == 0 || pos.trailStop > tp1 {
			pos.trailStop = tp1
		}
		pos.realizedPart += (pos.entryPrice - lastPrice) * qty
	}
	trail := pos.trailStop
	if cfg.TrailATRMultiplier > 0 && pos.tp1Done && len(h4) >= cfg.ATRPeriod+1 {
		highs := make([]float64, 0, len(h4))
		lows := make([]float64, 0, len(h4))
		closes := make([]float64, 0, len(h4))
		for _, bar := range h4 {
			highs = append(highs, bar.High)
			lows = append(lows, bar.Low)
			closes = append(closes, bar.Close)
		}
		atr := indicator.LastATR(highs, lows, closes, cfg.ATRPeriod)
		if atr > 0 {
			atrTrail := lastPrice + atr*cfg.TrailATRMultiplier
			if trail == 0 || atrTrail < trail {
				trail = atrTrail
			}
		}
	}
	if trail <= 0 {
		trail = pos.stopLoss
	}
	exitByStop := lastPrice >= trail
	exitByTimeout := cfg.MaxHoldBars > 0 && pos.holdBars >= cfg.MaxHoldBars
	if exitByStop || exitByTimeout {
		pnl := pos.realizedPart + (pos.entryPrice-lastPrice)*pos.qtyRemain
		*pos = replayShortPosition{}
		return pnl, true
	}
	return 0, false
}

func applyLongSwitchTakeProfit(cfg Config, pos *replayPosition, lastPrice float64, h4 []OHLC) float64 {
	if !pos.active || pos.qtyRemain <= 0 {
		return 0
	}
	pnl := 0.0
	q1 := pos.qtyRemain * cfg.SwitchTP1Pct
	q2 := pos.qtyRemain * cfg.SwitchTP2Pct
	q3 := pos.qtyRemain * cfg.SwitchTP3Pct
	for _, q := range []float64{q1, q2} {
		if q <= 0 || pos.qtyRemain <= 0 {
			continue
		}
		if q > pos.qtyRemain {
			q = pos.qtyRemain
		}
		pos.qtyRemain -= q
		pnl += (lastPrice - pos.entryPrice) * q
	}
	if q3 > pos.qtyRemain {
		q3 = pos.qtyRemain
	}
	if q3 > 0 {
		pos.qtyRemain -= q3
		trail := lastPrice - lastATR(h4, cfg.ATRPeriod)*cfg.SwitchTrailATRMultiplier
		if trail <= 0 {
			trail = pos.stopLoss
		}
		if lastPrice <= trail {
			pnl += (lastPrice - pos.entryPrice) * q3
		} else {
			pos.qtyRemain += q3
		}
	}
	if pos.qtyRemain <= 1e-9 {
		*pos = replayPosition{}
	}
	return pnl
}

func applyShortSwitchTakeProfit(cfg Config, pos *replayShortPosition, lastPrice float64, h4 []OHLC) float64 {
	if !pos.active || pos.qtyRemain <= 0 {
		return 0
	}
	pnl := 0.0
	q1 := pos.qtyRemain * cfg.SwitchTP1Pct
	q2 := pos.qtyRemain * cfg.SwitchTP2Pct
	q3 := pos.qtyRemain * cfg.SwitchTP3Pct
	for _, q := range []float64{q1, q2} {
		if q <= 0 || pos.qtyRemain <= 0 {
			continue
		}
		if q > pos.qtyRemain {
			q = pos.qtyRemain
		}
		pos.qtyRemain -= q
		pnl += (pos.entryPrice - lastPrice) * q
	}
	if q3 > pos.qtyRemain {
		q3 = pos.qtyRemain
	}
	if q3 > 0 {
		pos.qtyRemain -= q3
		trail := lastPrice + lastATR(h4, cfg.ATRPeriod)*cfg.SwitchTrailATRMultiplier
		if trail <= 0 {
			trail = pos.stopLoss
		}
		if lastPrice >= trail {
			pnl += (pos.entryPrice - lastPrice) * q3
		} else {
			pos.qtyRemain += q3
		}
	}
	if pos.qtyRemain <= 1e-9 {
		*pos = replayShortPosition{}
	}
	return pnl
}

func lastATR(h4 []OHLC, period int) float64 {
	if len(h4) < period+1 {
		return 0
	}
	highs := make([]float64, 0, len(h4))
	lows := make([]float64, 0, len(h4))
	closes := make([]float64, 0, len(h4))
	for _, bar := range h4 {
		highs = append(highs, bar.High)
		lows = append(lows, bar.Low)
		closes = append(closes, bar.Close)
	}
	return indicator.LastATR(highs, lows, closes, period)
}

func calcBuyHoldBenchmark(capital float64, h4Data []models.KLineRecord, start, end time.Time) ReplayBenchmark {
	series := make([]models.KLineRecord, 0, len(h4Data))
	for _, k := range h4Data {
		kt := k.OpenTime.UTC()
		if kt.Before(start) {
			continue
		}
		if !end.IsZero() && kt.After(end) {
			continue
		}
		series = append(series, k)
	}
	if len(series) < 2 {
		return ReplayBenchmark{Name: "buy_and_hold_10k", InitialCapital: capital, FinalEquity: capital, MonthlyReturns: map[string]float64{}}
	}
	entry := series[0].Close
	if entry <= 0 {
		entry = 1
	}
	qty := capital / entry
	peak := capital
	maxDD := 0.0
	monthly := map[string]float64{}
	monthStartEquity := map[string]float64{}
	for _, k := range series {
		eq := qty * k.Close
		if eq > peak {
			peak = eq
		}
		dd := peak - eq
		if dd > maxDD {
			maxDD = dd
		}
		m := k.OpenTime.UTC().Format("2006-01")
		if _, ok := monthStartEquity[m]; !ok {
			monthStartEquity[m] = eq
		}
		monthly[m] = (eq - monthStartEquity[m]) / capital
	}
	finalEq := qty * series[len(series)-1].Close
	net := finalEq - capital
	sharpe := 0.0
	if len(monthly) >= 2 {
		vals := make([]float64, 0, len(monthly))
		for _, v := range monthly {
			vals = append(vals, v)
		}
		mean, std := meanStd(vals)
		if std > 0 {
			sharpe = mean / std * math.Sqrt(12)
		}
	}
	return ReplayBenchmark{
		Name:           "buy_and_hold_10k",
		InitialCapital: capital,
		FinalEquity:    finalEq,
		NetPnL:         net,
		MaxDrawdown:    maxDD,
		Sharpe:         sharpe,
		MonthlyReturns: monthly,
	}
}

func evaluateTrendOnly(cfg Config, day []OHLC, h4 []OHLC) bool {
	if len(day) < cfg.EMAFilterPeriod+cfg.EMA200SlopeLookback || len(h4) < cfg.EMAFilterPeriod+2 {
		return false
	}
	dayCloses := make([]float64, 0, len(day))
	h4Closes := make([]float64, 0, len(h4))
	for _, k := range day {
		dayCloses = append(dayCloses, k.Close)
	}
	for _, k := range h4 {
		h4Closes = append(h4Closes, k.Close)
	}
	dayEMA := indicator.EMA(dayCloses, cfg.EMAFilterPeriod)
	if len(dayEMA) < cfg.EMA200SlopeLookback+1 {
		return false
	}
	dEMA := dayEMA[len(dayEMA)-1]
	slopeBase := dayEMA[len(dayEMA)-1-cfg.EMA200SlopeLookback]
	if slopeBase <= 0 {
		return false
	}
	emaSlope := (dEMA - slopeBase) / slopeBase
	h4EMA200 := indicator.LastEMA(h4Closes, cfg.EMAFilterPeriod)
	last := h4[len(h4)-1]
	return last.Close > dEMA && last.Close > h4EMA200 && emaSlope >= cfg.EMA200MinSlope
}

func meanStd(vals []float64) (float64, float64) {
	if len(vals) == 0 {
		return 0, 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	mean := sum / float64(len(vals))
	if len(vals) < 2 {
		return mean, 0
	}
	varSq := 0.0
	for _, v := range vals {
		d := v - mean
		varSq += d * d
	}
	return mean, math.Sqrt(varSq / float64(len(vals)-1))
}

func saveReplayReport(r *ReplayReport) error {
	reportDir := "strategies/spot_btc_conservative_v1/reports"
	if err := os.MkdirAll(reportDir, 0o775); err != nil {
		return err
	}
	ts := r.GeneratedAtUTC.Format("20060102_150405")
	capitalTag := fmt.Sprintf("%.0fk", r.CapitalUSDT/1000)
	windowTag := fmt.Sprintf("%dd", r.Days)
	if !r.StartTimeUTC.IsZero() && !r.EndTimeUTC.IsZero() {
		windowTag = fmt.Sprintf("%s_%s", r.StartTimeUTC.Format("20060102"), r.EndTimeUTC.Format("20060102"))
	}
	jsonPath := filepath.Join(reportDir, fmt.Sprintf("replay_%s_%s_%s_%s.json", r.Profile, capitalTag, windowTag, ts))
	mdPath := filepath.Join(reportDir, fmt.Sprintf("replay_%s_%s_%s_%s.md", r.Profile, capitalTag, windowTag, ts))

	r.ReportPathJSON = jsonPath
	r.ReportPathMD = mdPath
	buf, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(jsonPath, buf, 0o664); err != nil {
		return err
	}
	if err := os.WriteFile(mdPath, []byte(renderMarkdownReport(r)), 0o664); err != nil {
		return err
	}
	if err := updateReplayRegistry(reportDir); err != nil {
		return err
	}
	return nil
}

func saveV6ComparisonReport(r *ReplayReport) error {
	reportDir := "strategies/reports"
	if err := os.MkdirAll(reportDir, 0o775); err != nil {
		return err
	}
	ts := r.GeneratedAtUTC.Format("20060102_150405")
	mdPath := filepath.Join(reportDir, fmt.Sprintf("v6_10k_360d_comparison_%s.md", ts))
	v4 := r.Comparison["v4"]
	v5 := r.Comparison["v5_mean_reversion"]
	v6 := r.Comparison["v6"]
	content := fmt.Sprintf("# v6 360d Backtest Comparison (10k)\n\n| Strategy | Trades | WinRate | PF | NetPnL | MaxDD | Sharpe | Calmar | Excess vs B&H |\n|---|---:|---:|---:|---:|---:|---:|---:|---:|\n| Buy&Hold | - | - | - | %.2f | %.2f | %.3f | - | 0.00 |\n| v4 | %d | %.2f%% | %.3f | %.2f | %.2f | %.3f | %.3f | %.2f |\n| v5_mean_reversion | %d | %.2f%% | %.3f | %.2f | %.2f | %.3f | %.3f | %.2f |\n| v6 | %d | %.2f%% | %.3f | %.2f | %.2f | %.3f | %.3f | %.2f |\n\n## v6 Regime Switches\n\n- Bull->Range: %d\n- Bear->Range: %d\n- Range->Bull: %d\n- Switch take-profit contribution: %.2f USDT\n",
		r.Benchmark.NetPnL, r.Benchmark.MaxDrawdown, r.Benchmark.Sharpe,
		v4.TradeCount, v4.WinRate*100, v4.ProfitFactor, v4.NetPnL, v4.MaxDrawdown, v4.Sharpe, v4.Calmar, v4.NetPnL-r.Benchmark.NetPnL,
		v5.TradeCount, v5.WinRate*100, v5.ProfitFactor, v5.NetPnL, v5.MaxDrawdown, v5.Sharpe, v5.Calmar, v5.NetPnL-r.Benchmark.NetPnL,
		v6.TradeCount, v6.WinRate*100, v6.ProfitFactor, v6.NetPnL, v6.MaxDrawdown, v6.Sharpe, v6.Calmar, v6.NetPnL-r.Benchmark.NetPnL,
		v6.RegimeSwitches["BULL->RANGE"], v6.RegimeSwitches["BEAR->RANGE"], v6.RegimeSwitches["RANGE->BULL"], v6.RegimeSwitchTakeProfit,
	)
	return os.WriteFile(mdPath, []byte(content), 0o664)
}

func saveV7ComparisonReport(r *ReplayReport) error {
	reportDir := "strategies/reports"
	if err := os.MkdirAll(reportDir, 0o775); err != nil {
		return err
	}
	ts := r.GeneratedAtUTC.Format("20060102_150405")
	prefix := "v7"
	if r.Profile == "v7_1" {
		prefix = "v7_1"
	} else if r.Profile == "v8" {
		prefix = "v8"
	} else if r.Profile == "v9" {
		prefix = "v9"
	}
	windowTag := fmt.Sprintf("%dd", r.Days)
	if !r.StartTimeUTC.IsZero() && !r.EndTimeUTC.IsZero() {
		windowTag = fmt.Sprintf("%s_%s", r.StartTimeUTC.Format("20060102"), r.EndTimeUTC.Format("20060102"))
	}
	mdPath := filepath.Join(reportDir, fmt.Sprintf("%s_10k_%s_comparison_%s.md", prefix, windowTag, ts))
	content := fmt.Sprintf("# %s Regime Backtest Comparison (10k)\n\n- Regime timeframe selected: %s\n- Calibration report: %s\n\n| Strategy | Trades | WinRate | PF | NetPnL | MaxDD | Sharpe | Calmar | Excess vs B&H |\n|---|---:|---:|---:|---:|---:|---:|---:|---:|\n| Buy&Hold | - | - | - | %.2f | %.2f | %.3f | - | 0.00 |\n",
		r.Profile, r.RegimeTimeframe, r.CalibrationReportPath, r.Benchmark.NetPnL, r.Benchmark.MaxDrawdown, r.Benchmark.Sharpe)
	order := []string{"v5_mean_reversion", "v6", "v7", "v7_1", "v8", "v9"}
	for _, name := range order {
		v, ok := r.Comparison[name]
		if !ok {
			continue
		}
		content += fmt.Sprintf("| %s | %d | %.2f%% | %.3f | %.2f | %.2f | %.3f | %.3f | %.2f |\n", name, v.TradeCount, v.WinRate*100, v.ProfitFactor, v.NetPnL, v.MaxDrawdown, v.Sharpe, v.Calmar, v.NetPnL-r.Benchmark.NetPnL)
	}
	return os.WriteFile(mdPath, []byte(content), 0o664)
}

func aggregate1hTo4h(h1 []models.KLineRecord) []models.KLineRecord {
	if len(h1) == 0 {
		return nil
	}
	out := make([]models.KLineRecord, 0, len(h1)/4)
	for i := 0; i+3 < len(h1); i += 4 {
		c := models.KLineRecord{
			OpenTime:  h1[i].OpenTime,
			CloseTime: h1[i+3].CloseTime,
			Open:      h1[i].Open,
			Close:     h1[i+3].Close,
			High:      h1[i].High,
			Low:       h1[i].Low,
			Volume:    0,
		}
		for j := i; j < i+4; j++ {
			if h1[j].High > c.High {
				c.High = h1[j].High
			}
			if h1[j].Low < c.Low {
				c.Low = h1[j].Low
			}
			c.Volume += h1[j].Volume
		}
		out = append(out, c)
	}
	return out
}

func formatWindow(r *ReplayReport) string {
	if r == nil {
		return "N/A"
	}
	if !r.StartTimeUTC.IsZero() && !r.EndTimeUTC.IsZero() {
		return fmt.Sprintf("%s to %s", r.StartTimeUTC.Format("2006-01-02"), r.EndTimeUTC.Format("2006-01-02"))
	}
	return fmt.Sprintf("recent %d days", r.Days)
}

func renderMarkdownReport(r *ReplayReport) string {
	months := make([]string, 0, len(r.MonthlyReturns))
	for m := range r.MonthlyReturns {
		months = append(months, m)
	}
	sort.Strings(months)

	out := fmt.Sprintf("# Replay Report\n\n- Strategy: %s\n- Profile: %s\n- Symbol: %s\n- Days: %d\n- Window: %s\n- Interval: %s\n\n## Capital & Risk\n\n- Capital: %.2f USDT\n- Risk / Trade: %.2f%% (%.2f USDT)\n- Max Daily Loss: %.2f%% (%.2f USDT)\n- Max Weekly Loss: %.2f%% (%.2f USDT)\n\n## Cost Model\n\n- Cost / Side: %.4f%%\n- Short Funding APR: %.2f%%\n\n## Strategy Metrics\n\n- Trade Count: %d\n- Win Rate: %.2f%%\n- Profit Factor: %.4f\n- Max Drawdown: %.4f USDT\n- Net PnL: %.4f USDT\n- Sharpe (monthly): %.4f\n- Calmar: %.4f\n",
		r.StrategyID, r.Profile, r.Symbol, r.Days, formatWindow(r), r.Interval,
		r.CapitalUSDT, r.RiskPerTradePct*100, r.RiskPerTradeAmt, r.MaxDailyLossPct*100, r.MaxDailyLossAmt, r.MaxWeeklyLossPct*100, r.MaxWeeklyLossAmt,
		r.CostPerSidePct*100, r.ShortFundingAPR*100,
		r.TradeCount, r.WinRate*100, r.ProfitFactor, r.MaxDrawdown, r.NetPnL, r.Sharpe, r.Calmar,
	)
	if r.CorePnL != 0 || r.TacticalPnL != 0 {
		out += fmt.Sprintf("- Core PnL: %.4f USDT\n- Tactical PnL: %.4f USDT\n", r.CorePnL, r.TacticalPnL)
	}
	if len(r.RegimeSwitches) > 0 {
		out += "- Regime switches:\n"
		keys := make([]string, 0, len(r.RegimeSwitches))
		for k := range r.RegimeSwitches {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			out += fmt.Sprintf("  - %s: %d\n", k, r.RegimeSwitches[k])
		}
		out += fmt.Sprintf("- Switch take-profit contribution: %.4f USDT\n", r.RegimeSwitchTakeProfit)
	}
	out += fmt.Sprintf("\n## Benchmark (Buy & Hold)\n\n- Net PnL: %.4f USDT\n- Max Drawdown: %.4f USDT\n- Sharpe (monthly): %.4f\n- Excess Return (Strategy - B&H): %.4f USDT\n\n## Strategy Monthly Returns\n\n", r.Benchmark.NetPnL, r.Benchmark.MaxDrawdown, r.Benchmark.Sharpe, r.ExcessReturn)
	for _, m := range months {
		out += fmt.Sprintf("- %s: %.2f%%\n", m, r.MonthlyReturns[m]*100)
	}
	if len(r.Comparison) > 0 {
		keys := []string{"baseline", "v3", "v4"}
		out += "\n## Comparison (same window)\n\n"
		for _, k := range keys {
			v, ok := r.Comparison[k]
			if !ok {
				continue
			}
			out += fmt.Sprintf("- %s: trades=%d, win=%.2f%%, pf=%.4f, net=%.4f, maxdd=%.4f, sharpe=%.4f, calmar=%.4f\n", k, v.TradeCount, v.WinRate*100, v.ProfitFactor, v.NetPnL, v.MaxDrawdown, v.Sharpe, v.Calmar)
		}
		out += fmt.Sprintf("- buy_and_hold_10k: net=%.4f, maxdd=%.4f, sharpe=%.4f\n", r.Benchmark.NetPnL, r.Benchmark.MaxDrawdown, r.Benchmark.Sharpe)
		if v4, ok := r.Comparison["v4"]; ok {
			out += fmt.Sprintf("- v4 excess vs buy_and_hold_10k: %.4f USDT\n", v4.NetPnL-r.Benchmark.NetPnL)
		}
	}
	return out
}
