package models

import (
	"time"

	"gorm.io/gorm"
)

func (m *TradeRecord) GetOpenTradesByStrategy(db *gorm.DB, strategyID, symbol, tradeType string) ([]TradeRecord, error) {
	var records []TradeRecord
	err := db.Session(&gorm.Session{NewDB: true}).
		Where("strategy_id = ? AND symbol = ? AND type = ? AND status = 'open'", strategyID, symbol, tradeType).
		Order("id ASC").
		Find(&records).Error
	return records, err
}

func (m *TradeRecord) SumClosedPnLByRange(db *gorm.DB, strategyID string, start, end time.Time) (float64, error) {
	var sum float64
	err := db.Session(&gorm.Session{NewDB: true}).
		Model(&TradeRecord{}).
		Where("strategy_id = ? AND side = 'sell' AND status = 'closed' AND created_at >= ? AND created_at < ?", strategyID, start, end).
		Select("COALESCE(SUM(pnl),0)").
		Scan(&sum).Error
	return sum, err
}

func (m *TradeRecord) LatestLossStreak(db *gorm.DB, strategyID string, limit int) (int, error) {
	if limit <= 0 {
		limit = 50
	}
	var rows []TradeRecord
	err := db.Session(&gorm.Session{NewDB: true}).
		Where("strategy_id = ? AND side = 'sell' AND status = 'closed'", strategyID).
		Order("created_at DESC").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return 0, err
	}
	streak := 0
	for _, r := range rows {
		if r.PnL < 0 {
			streak++
			continue
		}
		if r.PnL > 0 {
			break
		}
	}
	return streak, nil
}
