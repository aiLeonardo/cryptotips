package spot_btc_conservative_v1

import "math"

type PositionPlan struct {
	RiskAmount  float64
	RiskPerUnit float64
	Qty         float64
	Notional    float64
}

func BuildPosition(cfg Config, entry, stop float64) PositionPlan {
	riskAmount := cfg.RiskCapitalUSDT * cfg.RiskPerTrade
	riskPerUnit := entry - stop
	if riskPerUnit <= 0 {
		return PositionPlan{}
	}
	qty := riskAmount / riskPerUnit
	notional := qty * entry
	if notional > cfg.RiskCapitalUSDT {
		qty = cfg.RiskCapitalUSDT / entry
		notional = qty * entry
	}
	qty = math.Floor(qty*1e6) / 1e6
	return PositionPlan{RiskAmount: riskAmount, RiskPerUnit: riskPerUnit, Qty: qty, Notional: notional}
}
