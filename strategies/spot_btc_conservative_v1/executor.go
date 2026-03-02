package spot_btc_conservative_v1

import (
	"fmt"
	"time"

	"github.com/aiLeonardo/cryptotips/lib"
	"github.com/aiLeonardo/cryptotips/models"
	"gorm.io/gorm"
)

func md5Text(input string) string {
	return lib.NewMyHash().Md532(input)
}

type PaperExecutor struct {
	cfg Config
	db  *gorm.DB
}

func NewPaperExecutor(cfg Config, db *gorm.DB) *PaperExecutor {
	return &PaperExecutor{cfg: cfg, db: db}
}

func (e *PaperExecutor) Open(entry, stop, qty float64, reason string) error {
	rec := models.TradeRecord{
		Md5:        md5Text(fmt.Sprintf("%s:%s:%d:open", e.cfg.StrategyID, e.cfg.Symbol, time.Now().UnixNano())),
		StrategyID: e.cfg.StrategyID,
		Platform:   "paper",
		Type:       "spot",
		Side:       "buy",
		Symbol:     e.cfg.Symbol,
		Price:      entry,
		Qty:        qty,
		StopLoss:   stop,
		Status:     "open",
	}
	if err := rec.SaveTrade(e.db); err != nil {
		return err
	}
	return e.log("BUY", reason)
}

func (e *PaperExecutor) PartialClose(qty, price float64, reason string) error {
	rec := models.TradeRecord{
		Md5:        md5Text(fmt.Sprintf("%s:%s:%d:partial", e.cfg.StrategyID, e.cfg.Symbol, time.Now().UnixNano())),
		StrategyID: e.cfg.StrategyID,
		Platform:   "paper",
		Type:       "spot",
		Side:       "sell",
		Symbol:     e.cfg.Symbol,
		Price:      price,
		Qty:        qty,
		Status:     "closed",
	}
	if err := rec.SaveTrade(e.db); err != nil {
		return err
	}
	return e.log("PARTIAL_SELL", reason)
}

func (e *PaperExecutor) CloseAll(price, pnl float64, reason string) error {
	if err := (&models.TradeRecord{}).CloseOpenTradesByStrategy(e.db, e.cfg.StrategyID, e.cfg.Symbol, pnl); err != nil {
		return err
	}
	rec := models.TradeRecord{
		Md5:        md5Text(fmt.Sprintf("%s:%s:%d:close", e.cfg.StrategyID, e.cfg.Symbol, time.Now().UnixNano())),
		StrategyID: e.cfg.StrategyID,
		Platform:   "paper",
		Type:       "spot",
		Side:       "sell",
		Symbol:     e.cfg.Symbol,
		Price:      price,
		Qty:        0,
		Status:     "closed",
		PnL:        pnl,
	}
	if err := rec.SaveTrade(e.db); err != nil {
		return err
	}
	return e.log("CLOSE_ALL", reason)
}

func (e *PaperExecutor) log(decision, reason string) error {
	log := models.StrategyLogRecord{
		Md5:        md5Text(fmt.Sprintf("%s:%s:%d:%s", e.cfg.StrategyID, e.cfg.Symbol, time.Now().UnixNano(), decision)),
		StrategyID: e.cfg.StrategyID,
		Symbol:     e.cfg.Symbol,
		Date:       time.Now().UTC(),
		Decision:   decision,
		Reason:     reason,
	}
	return log.SaveStrategyLog(e.db)
}
