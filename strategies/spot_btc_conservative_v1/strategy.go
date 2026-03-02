package spot_btc_conservative_v1

import (
	"fmt"
	"time"

	"github.com/aiLeonardo/cryptotips/indicator"
	"github.com/aiLeonardo/cryptotips/models"
	"github.com/aiLeonardo/cryptotips/strategies/engine"
)

const stateKey = "runtime"

const (
	lossStreakLimit = 3
	pauseDuration   = 24 * time.Hour
)

type Strategy struct {
	cfg Config
}

func New() *Strategy {
	return &Strategy{cfg: DefaultConfig()}
}

func init() {
	engine.Register(New())
}

func (s *Strategy) ID() string { return s.cfg.StrategyID }

func (s *Strategy) Description() string {
	return "BTC 现货保守策略 v1 (paper): 1d/4h EMA200 过滤 + 4h EMA20 回踩确认"
}

func (s *Strategy) Status(ctx engine.RuntimeContext) (map[string]any, error) {
	st := StrategyState{}
	if err := ctx.StateStore.Load(ctx, stateKey, &st); err != nil {
		return nil, err
	}
	return map[string]any{"strategy_id": s.ID(), "mode": ctx.Mode, "state": st}, nil
}

func (s *Strategy) Run(ctx engine.RuntimeContext) error {
	if ctx.Mode != "paper" {
		return fmt.Errorf("only paper mode supported currently")
	}

	k := &models.KLineRecord{}
	dayData, err := k.GetKLines(ctx.Deps.DB, s.cfg.Symbol, "1d", time.Time{}, time.Time{})
	if err != nil {
		return err
	}
	h4Data, err := k.GetKLines(ctx.Deps.DB, s.cfg.Symbol, "4h", time.Time{}, time.Time{})
	if err != nil {
		return err
	}

	day := make([]OHLC, 0, len(dayData))
	h4 := make([]OHLC, 0, len(h4Data))
	for _, item := range dayData {
		day = append(day, OHLC{Close: item.Close, High: item.High, Low: item.Low, Volume: item.Volume})
	}
	for _, item := range h4Data {
		h4 = append(h4, OHLC{Close: item.Close, High: item.High, Low: item.Low, Volume: item.Volume})
	}

	state := StrategyState{}
	if err := ctx.StateStore.Load(ctx, stateKey, &state); err != nil {
		return err
	}
	if err := s.recoverPositionState(ctx, &state); err != nil {
		return err
	}
	if err := s.refreshRiskStateFromDB(ctx, &state); err != nil {
		return err
	}

	exec := NewPaperExecutor(s.cfg, ctx.Deps.DB)
	lastPrice := 0.0
	if len(h4) > 0 {
		lastPrice = h4[len(h4)-1].Close
	}

	if !state.HasPosition {
		if state.PauseUntil.After(ctx.Now) {
			ctx.Deps.Logger.Infof("[%s] skip entry: paused until %s", s.ID(), state.PauseUntil.Format(time.RFC3339))
			state.UpdatedAt = ctx.Now
			return ctx.StateStore.Save(ctx, stateKey, state, 30*24*time.Hour)
		}
		sig, err := EvaluateSignal(s.cfg, day, h4)
		if err != nil {
			return err
		}
		if !sig.ShouldEnter {
			ctx.Deps.Logger.Infof("[%s] no entry signal: %s", s.ID(), sig.Reason)
			return nil
		}
		plan := BuildPosition(s.cfg, sig.EntryPrice, sig.StopLoss)
		if plan.Qty <= 0 {
			ctx.Deps.Logger.Infof("[%s] invalid position plan", s.ID())
			return nil
		}
		if err := exec.Open(sig.EntryPrice, sig.StopLoss, plan.Qty, sig.Reason); err != nil {
			return err
		}
		state.HasPosition = true
		state.EntryPrice = sig.EntryPrice
		state.StopLoss = sig.StopLoss
		state.QtyTotal = plan.Qty
		state.QtyRemain = plan.Qty
		state.RiskPerUnit = plan.RiskPerUnit
		state.TP1Done = false
		state.TP2Done = false
		state.TrailStop = 0
		state.HoldBars = 0
		state.UpdatedAt = ctx.Now
		return ctx.StateStore.Save(ctx, stateKey, state, 30*24*time.Hour)
	}

	state.HoldBars++
	tp1 := state.EntryPrice + state.RiskPerUnit
	tp2 := state.EntryPrice + 2*state.RiskPerUnit
	if !state.TP1Done && lastPrice >= tp1 {
		qty := state.QtyTotal * 0.4
		state.QtyRemain -= qty
		state.TP1Done = true
		state.TrailStop = state.EntryPrice
		if err := exec.PartialClose(qty, lastPrice, "tp1_1R"); err != nil {
			return err
		}
	}
	if !state.TP2Done && lastPrice >= tp2 {
		qty := state.QtyTotal * 0.3
		state.QtyRemain -= qty
		state.TP2Done = true
		if state.TrailStop < tp1 {
			state.TrailStop = tp1
		}
		if err := exec.PartialClose(qty, lastPrice, "tp2_2R"); err != nil {
			return err
		}
	}

	trail := state.TrailStop
	if state.TP1Done && len(h4) >= s.cfg.ATRPeriod+1 {
		highs := make([]float64, 0, len(h4))
		lows := make([]float64, 0, len(h4))
		closes := make([]float64, 0, len(h4))
		for _, bar := range h4 {
			highs = append(highs, bar.High)
			lows = append(lows, bar.Low)
			closes = append(closes, bar.Close)
		}
		atr := indicator.LastATR(highs, lows, closes, s.cfg.ATRPeriod)
		if atr > 0 {
			atrTrail := lastPrice - atr*s.cfg.TrailATRMultiplier
			if atrTrail > trail {
				trail = atrTrail
			}
		}
	}
	if trail <= 0 {
		trail = state.StopLoss
	}
	state.TrailStop = trail
	exitByStop := lastPrice <= trail
	exitByTimeout := s.cfg.MaxHoldBars > 0 && state.HoldBars >= s.cfg.MaxHoldBars
	if exitByStop || exitByTimeout {
		pnl := (lastPrice - state.EntryPrice) * state.QtyRemain
		reason := "trailing_stop"
		if exitByTimeout && !exitByStop {
			reason = "max_hold_timeout"
		}
		if err := exec.CloseAll(lastPrice, pnl, reason); err != nil {
			return err
		}
		state.HasPosition = false
		state.EntryPrice = 0
		state.StopLoss = 0
		state.QtyTotal = 0
		state.QtyRemain = 0
		state.RiskPerUnit = 0
		state.TP1Done = false
		state.TP2Done = false
		state.TrailStop = 0
		state.HoldBars = 0
	}

	state.UpdatedAt = ctx.Now
	return ctx.StateStore.Save(ctx, stateKey, state, 30*24*time.Hour)
}

func (s *Strategy) refreshRiskStateFromDB(ctx engine.RuntimeContext, state *StrategyState) error {
	trade := &models.TradeRecord{}
	now := ctx.Now.UTC()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	weekStart := dayStart.AddDate(0, 0, -int((int(dayStart.Weekday())+6)%7))

	dailyPnL, err := trade.SumClosedPnLByRange(ctx.Deps.DB, s.cfg.StrategyID, dayStart, dayStart.Add(24*time.Hour))
	if err != nil {
		return err
	}
	weeklyPnL, err := trade.SumClosedPnLByRange(ctx.Deps.DB, s.cfg.StrategyID, weekStart, weekStart.Add(7*24*time.Hour))
	if err != nil {
		return err
	}
	lossStreak, err := trade.LatestLossStreak(ctx.Deps.DB, s.cfg.StrategyID, 50)
	if err != nil {
		return err
	}
	state.LossStreak = lossStreak

	dailyLossLimit := -s.cfg.RiskCapitalUSDT * s.cfg.MaxDailyLossPct
	weeklyLossLimit := -s.cfg.RiskCapitalUSDT * s.cfg.MaxWeeklyLossPct
	if dailyPnL <= dailyLossLimit {
		state.PauseUntil = now.Add(pauseDuration)
		ctx.Deps.Logger.Warnf("[%s] risk gate triggered: daily pnl %.4f <= %.2f, pause until %s", s.ID(), dailyPnL, dailyLossLimit, state.PauseUntil.Format(time.RFC3339))
	}
	if weeklyPnL <= weeklyLossLimit {
		state.PauseUntil = now.Add(pauseDuration)
		ctx.Deps.Logger.Warnf("[%s] risk gate triggered: weekly pnl %.4f <= %.2f, pause until %s", s.ID(), weeklyPnL, weeklyLossLimit, state.PauseUntil.Format(time.RFC3339))
	}
	if lossStreak >= lossStreakLimit {
		state.PauseUntil = now.Add(pauseDuration)
		ctx.Deps.Logger.Warnf("[%s] risk gate triggered: loss streak %d >= %d, pause until %s", s.ID(), lossStreak, lossStreakLimit, state.PauseUntil.Format(time.RFC3339))
	}
	if !state.PauseUntil.IsZero() && state.PauseUntil.Before(now) {
		ctx.Deps.Logger.Infof("[%s] risk gate recovered: pause lifted at %s", s.ID(), now.Format(time.RFC3339))
		state.PauseUntil = time.Time{}
	}
	return nil
}

func (s *Strategy) recoverPositionState(ctx engine.RuntimeContext, state *StrategyState) error {
	trade := &models.TradeRecord{}
	openTrades, err := trade.GetOpenTradesByStrategy(ctx.Deps.DB, s.cfg.StrategyID, s.cfg.Symbol, "spot")
	if err != nil {
		return err
	}
	if len(openTrades) == 0 {
		if state.HasPosition {
			ctx.Deps.Logger.Warnf("[%s] recover: redis has position but db has none, clear state", s.ID())
			state.HasPosition = false
		}
		return nil
	}

	if !state.HasPosition {
		t := openTrades[0]
		state.HasPosition = true
		state.EntryPrice = t.Price
		state.StopLoss = t.StopLoss
		state.QtyTotal = t.Qty
		state.QtyRemain = t.Qty
		state.RiskPerUnit = t.Price - t.StopLoss
		ctx.Deps.Logger.Infof("[%s] recover: rebuild position state from db open trade id=%d", s.ID(), t.ID)
	}
	if len(openTrades) > 1 {
		ctx.Deps.Logger.Warnf("[%s] recover: found %d open trades, skip new entry to avoid duplicate position", s.ID(), len(openTrades))
	}
	return nil
}
