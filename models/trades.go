package models

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// TradeRecord 交易记录表
type TradeRecord struct {
	ID         uint    `gorm:"primaryKey;autoIncrement"`
	Md5        string  `gorm:"type:varchar(65);not null;"`
	StrategyID string  `gorm:"type:varchar(100);not null;default:'';index:idx_strategy_symbol_status,priority:1"`
	Platform   string  `gorm:"type:varchar(20);not null"` // binance
	Type       string  `gorm:"type:varchar(10);not null"` // spot / future
	Side       string  `gorm:"type:varchar(10);not null"` // buy / sell
	Symbol     string  `gorm:"type:varchar(20);not null;index:idx_strategy_symbol_status,priority:2"`
	Price      float64 `gorm:"type:decimal(20,8)"`
	Qty        float64 `gorm:"type:decimal(20,8)"`
	StopLoss   float64 `gorm:"type:decimal(20,8)"`
	TakeProfit float64 `gorm:"type:decimal(20,8)"`
	Status     string  `gorm:"type:varchar(20);index:idx_strategy_symbol_status,priority:3"` // open / closed / cancelled
	PnL        float64 `gorm:"type:decimal(20,8)"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// TableName 指定 GORM 映射的表名
func (TradeRecord) TableName() string {
	return "crypto_trades"
}

func (m *TradeRecord) GetInfoByMd5(db *gorm.DB, Md5 string) error {
	dbSession := db.Session(&gorm.Session{NewDB: true})

	if err := dbSession.Where("md5 = ?", Md5).First(m).Error; err != nil {
		return err
	}
	return nil
}

func (m *TradeRecord) InsertOrIgnoreByMd5(db *gorm.DB) error {
	dbSession := db.Session(&gorm.Session{NewDB: true})

	var existing TradeRecord
	err := dbSession.Where("md5 = ?", m.Md5).First(&existing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return dbSession.Create(m).Error
		}
		return err
	}

	return nil
}

// SaveTrade 存储交易记录
func (m *TradeRecord) SaveTrade(db *gorm.DB) error {
	dbSession := db.Session(&gorm.Session{NewDB: true})

	return dbSession.Save(m).Error
}

// GetOpenTrade 获取当前未平仓的合约单
func (m *TradeRecord) GetOpenTrade(db *gorm.DB, symbol, tradeType string) (*TradeRecord, error) {
	var rec TradeRecord

	dbSession := db.Session(&gorm.Session{NewDB: true})
	err := dbSession.Where("symbol = ? AND type = ? AND status = 'open'", symbol, tradeType).First(&rec).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &rec, err
}

// CloseOpenTrades 平仓所有未平仓记录
func (m *TradeRecord) CloseOpenTrades(db *gorm.DB, symbol string, pnl float64) error {
	dbSession := db.Session(&gorm.Session{NewDB: true})

	return dbSession.Model(&TradeRecord{}).
		Where("symbol = ? AND status = 'open'", symbol).
		Updates(map[string]interface{}{"status": "closed", "pnl": pnl}).Error
}

func (m *TradeRecord) GetOpenTradeByStrategy(db *gorm.DB, strategyID, symbol, tradeType string) (*TradeRecord, error) {
	var rec TradeRecord
	dbSession := db.Session(&gorm.Session{NewDB: true})
	err := dbSession.Where("strategy_id = ? AND symbol = ? AND type = ? AND status = 'open'", strategyID, symbol, tradeType).First(&rec).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &rec, err
}

func (m *TradeRecord) CloseOpenTradesByStrategy(db *gorm.DB, strategyID, symbol string, pnl float64) error {
	dbSession := db.Session(&gorm.Session{NewDB: true})
	return dbSession.Model(&TradeRecord{}).
		Where("strategy_id = ? AND symbol = ? AND status = 'open'", strategyID, symbol).
		Updates(map[string]interface{}{"status": "closed", "pnl": pnl}).Error
}
