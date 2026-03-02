package models

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// StrategyLogRecord 策略日志表
type StrategyLogRecord struct {
	ID          uint      `gorm:"primaryKey;autoIncrement"`
	Md5         string    `gorm:"type:varchar(65);not null;"`
	StrategyID  string    `gorm:"type:varchar(100);not null;default:'';index:idx_strategy_symbol_date,priority:1"`
	Symbol      string    `gorm:"type:varchar(20);not null;index:idx_strategy_symbol_date,priority:2"`
	Date        time.Time `gorm:"not null;index:idx_strategy_symbol_date,priority:3"`
	MarketState string    `gorm:"type:varchar(10)"` // BULL / BEAR / NEUTRAL
	Decision    string    `gorm:"type:varchar(50)"`
	Reason      string    `gorm:"type:text"`
	CreatedAt   time.Time
}

// TableName 指定 GORM 映射的表名
func (StrategyLogRecord) TableName() string {
	return "crypto_strategy_log"
}

func (m *StrategyLogRecord) GetInfoByMd5(db *gorm.DB, Md5 string) error {
	dbSession := db.Session(&gorm.Session{NewDB: true})

	if err := dbSession.Where("md5 = ?", Md5).First(m).Error; err != nil {
		return err
	}
	return nil
}

func (m *StrategyLogRecord) InsertOrIgnoreByMd5(db *gorm.DB) error {
	dbSession := db.Session(&gorm.Session{NewDB: true})

	var existing StrategyLogRecord
	err := dbSession.Where("md5 = ?", m.Md5).First(&existing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return dbSession.Create(m).Error
		}
		return err
	}

	return nil
}

// SaveStrategyLog 存储策略日志
func (m *StrategyLogRecord) SaveStrategyLog(db *gorm.DB) error {
	dbSession := db.Session(&gorm.Session{NewDB: true})

	return dbSession.Create(m).Error
}
