package spot_btc_conservative_v1

import (
	"fmt"

	"github.com/aiLeonardo/cryptotips/indicator"
)

type SignalResult struct {
	ShouldEnter bool
	EntryPrice  float64
	StopLoss    float64
	Reason      string
}

type OHLC struct {
	Close  float64
	High   float64
	Low    float64
	Volume float64
}

func EvaluateSignal(cfg Config, day []OHLC, h4 []OHLC) (SignalResult, error) {
	switch cfg.StrategyStyle {
	case "mean_reversion":
		return evalMeanReversion(cfg, day, h4)
	case "momentum_factor":
		return evalMomentumFactor(cfg, day, h4)
	default:
		return evalTrend(cfg, day, h4)
	}
}

func evalTrend(cfg Config, day []OHLC, h4 []OHLC) (SignalResult, error) {
	minBars := cfg.EMAFilterPeriod + 2
	if cfg.VolumeSMAPeriod+2 > minBars {
		minBars = cfg.VolumeSMAPeriod + 2
	}
	if cfg.ATRPeriod+2 > minBars {
		minBars = cfg.ATRPeriod + 2
	}
	if len(day) < cfg.EMAFilterPeriod+cfg.EMA200SlopeLookback || len(h4) < minBars {
		return SignalResult{Reason: "insufficient_kline"}, nil
	}
	dayCloses, h4Closes, h4Highs, h4Lows, h4Volumes := splitOHLC(day, h4)
	dayEMA := indicator.EMA(dayCloses, cfg.EMAFilterPeriod)
	if len(dayEMA) < cfg.EMA200SlopeLookback+1 {
		return SignalResult{Reason: "day_ema_insufficient"}, nil
	}
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

	trendOK := last.Close > dEMA && last.Close > h4EMA
	slopeOK := emaSlope >= cfg.EMA200MinSlope
	pullback := prev.Close <= emaPrev && prev.Low <= emaPrev*(1+cfg.PullbackTol)
	confirm := last.Close > emaNow
	secondaryBreakout := false
	if cfg.SecondaryBreakoutLookback > 1 && len(h4Closes) > cfg.SecondaryBreakoutLookback {
		start := len(h4Closes) - 1 - cfg.SecondaryBreakoutLookback
		highest := h4Highs[start]
		for i := start; i < len(h4Highs)-1; i++ {
			if h4Highs[i] > highest {
				highest = h4Highs[i]
			}
		}
		secondaryBreakout = prev.Close > emaPrev && last.Close >= highest*(1+cfg.SecondaryBreakoutBufferPct)
	}
	entryTrigger := (pullback && confirm) || secondaryBreakout
	volatilityOK := atrPct >= cfg.ATRMinPct && atrPct <= cfg.ATRMaxPct
	volumeOK := volRatio >= cfg.VolumeMinRatio
	if !(trendOK && slopeOK && entryTrigger && volatilityOK && volumeOK) {
		return SignalResult{Reason: fmt.Sprintf("trend=%v slope=%.4f trigger=%v atr=%.4f vol=%.2f", trendOK, emaSlope, entryTrigger, atrPct, volRatio)}, nil
	}
	stop := calcStop(cfg, h4, emaNow, atr)
	if stop <= 0 || last.Close <= stop {
		return SignalResult{Reason: "invalid_stop"}, nil
	}
	reason := "trend_pullback"
	if cfg.StrategyStyle == "trend_breakout" {
		reason = "trend_breakout"
	}
	return SignalResult{ShouldEnter: true, EntryPrice: last.Close, StopLoss: stop, Reason: reason}, nil
}

func evalMeanReversion(cfg Config, day []OHLC, h4 []OHLC) (SignalResult, error) {
	if len(h4) < 60 || len(day) < 50 {
		return SignalResult{Reason: "insufficient_kline"}, nil
	}
	_, h4Closes, h4Highs, h4Lows, h4Volumes := splitOHLC(day, h4)
	upper, mid, lower := indicator.LastBollinger(h4Closes, 20, 2)
	rsi := indicator.LastRSI(h4Closes, 14)
	atr := indicator.LastATR(h4Highs, h4Lows, h4Closes, cfg.ATRPeriod)
	volSMA := indicator.LastSMA(h4Volumes, cfg.VolumeSMAPeriod)
	if lower <= 0 || mid <= 0 || upper <= 0 || atr <= 0 || volSMA <= 0 {
		return SignalResult{Reason: "ind_invalid"}, nil
	}
	last := h4[len(h4)-1]
	prev := h4[len(h4)-2]
	atrPct := atr / last.Close
	volRatio := last.Volume / volSMA
	entry := prev.Close < lower && last.Close > lower && rsi > 30 && rsi < 48 && last.Close < mid
	if !(entry && atrPct >= cfg.ATRMinPct && atrPct <= cfg.ATRMaxPct && volRatio >= cfg.VolumeMinRatio) {
		return SignalResult{Reason: fmt.Sprintf("mr_fail rsi=%.2f atr=%.4f vol=%.2f", rsi, atrPct, volRatio)}, nil
	}
	stop := last.Close - 1.2*atr
	if stop <= 0 || stop >= last.Close {
		return SignalResult{Reason: "invalid_stop"}, nil
	}
	return SignalResult{ShouldEnter: true, EntryPrice: last.Close, StopLoss: stop, Reason: "bb_rsi_mean_reversion"}, nil
}

func evalMomentumFactor(cfg Config, day []OHLC, h4 []OHLC) (SignalResult, error) {
	if len(day) < 130 || len(h4) < 120 {
		return SignalResult{Reason: "insufficient_kline"}, nil
	}
	dayCloses, h4Closes, h4Highs, h4Lows, h4Volumes := splitOHLC(day, h4)
	mom30 := dayCloses[len(dayCloses)-1]/dayCloses[len(dayCloses)-31] - 1
	mom90 := dayCloses[len(dayCloses)-1]/dayCloses[len(dayCloses)-91] - 1
	ema21 := indicator.LastEMA(h4Closes, 21)
	ema100 := indicator.LastEMA(h4Closes, cfg.EMAFilterPeriod)
	rsi := indicator.LastRSI(h4Closes, 14)
	atr := indicator.LastATR(h4Highs, h4Lows, h4Closes, cfg.ATRPeriod)
	volRatio := h4Volumes[len(h4Volumes)-1] / indicator.LastSMA(h4Volumes, cfg.VolumeSMAPeriod)
	last := h4[len(h4)-1]
	atrPct := atr / last.Close
	score := 0
	if mom30 > 0.03 { score++ }
	if mom90 > 0.08 { score++ }
	if last.Close > ema21 && ema21 > ema100 { score++ }
	if rsi >= 50 && rsi <= 72 { score++ }
	if volRatio >= cfg.VolumeMinRatio { score++ }
	if !(score >= 4 && atrPct >= cfg.ATRMinPct && atrPct <= cfg.ATRMaxPct) {
		return SignalResult{Reason: fmt.Sprintf("factor_score=%d mom30=%.3f mom90=%.3f rsi=%.1f", score, mom30, mom90, rsi)}, nil
	}
	stop := last.Close - 1.5*atr
	if stop <= 0 || stop >= last.Close {
		return SignalResult{Reason: "invalid_stop"}, nil
	}
	return SignalResult{ShouldEnter: true, EntryPrice: last.Close, StopLoss: stop, Reason: "multi_factor_momentum"}, nil
}

func splitOHLC(day []OHLC, h4 []OHLC) (dayCloses, h4Closes, h4Highs, h4Lows, h4Volumes []float64) {
	dayCloses = make([]float64, 0, len(day))
	h4Closes = make([]float64, 0, len(h4))
	h4Highs = make([]float64, 0, len(h4))
	h4Lows = make([]float64, 0, len(h4))
	h4Volumes = make([]float64, 0, len(h4))
	for _, k := range day { dayCloses = append(dayCloses, k.Close) }
	for _, k := range h4 {
		h4Closes = append(h4Closes, k.Close)
		h4Highs = append(h4Highs, k.High)
		h4Lows = append(h4Lows, k.Low)
		h4Volumes = append(h4Volumes, k.Volume)
	}
	return
}

func calcStop(cfg Config, h4 []OHLC, emaNow, atr float64) float64 {
	last := h4[len(h4)-1]
	stop := last.Low
	start := len(h4) - 5
	if start < 0 { start = 0 }
	for i := start; i < len(h4); i++ {
		if h4[i].Low < stop { stop = h4[i].Low }
	}
	emaStop := emaNow * 0.98
	if emaStop < stop { stop = emaStop }
	atrStop := last.Close - atr*cfg.TrailATRMultiplier
	if atrStop < stop { stop = atrStop }
	return stop
}
