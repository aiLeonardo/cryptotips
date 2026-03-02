package spot_btc_conservative_v1

import "time"

type StrategyState struct {
	HasPosition bool      `json:"has_position"`
	EntryPrice  float64   `json:"entry_price"`
	StopLoss    float64   `json:"stop_loss"`
	QtyTotal    float64   `json:"qty_total"`
	QtyRemain   float64   `json:"qty_remain"`
	RiskPerUnit float64   `json:"risk_per_unit"`
	TP1Done     bool      `json:"tp1_done"`
	TP2Done     bool      `json:"tp2_done"`
	TrailStop   float64   `json:"trail_stop"`
	HoldBars    int       `json:"hold_bars"`
	PauseUntil  time.Time `json:"pause_until"`
	LossStreak  int       `json:"loss_streak"`
	UpdatedAt   time.Time `json:"updated_at"`
}
