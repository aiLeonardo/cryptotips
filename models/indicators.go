package models

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// IndicatorRecord 指标数据表
type IndicatorRecord struct {
	ID         uint      `gorm:"primaryKey;autoIncrement"`
	Md5        string    `gorm:"type:varchar(65);not null;"`
	Symbol     string    `gorm:"type:varchar(20);not null;index:idx_sym_date,priority:1"`
	Date       time.Time `gorm:"not null;index:idx_sym_date,priority:2"`
	MA200      float64   `gorm:"type:decimal(20,8)"`
	SlopeMA200 float64   `gorm:"type:decimal(20,8)"`
	EMA20      float64   `gorm:"type:decimal(20,8)"`
	EMA50      float64   `gorm:"type:decimal(20,8)"`
	EMA100     float64   `gorm:"type:decimal(20,8)"`
	EMA200     float64   `gorm:"type:decimal(20,8)"`
	RSI        float64   `gorm:"type:decimal(10,4)"`
	MACD       float64   `gorm:"type:decimal(20,8)"`
	SignalLine float64   `gorm:"type:decimal(20,8)"`
	ATR        float64   `gorm:"type:decimal(20,8)"`
	BollUpper  float64   `gorm:"type:decimal(20,8)"`
	BollLower  float64   `gorm:"type:decimal(20,8)"`
	CreatedAt  time.Time
}

// TableName 指定 GORM 映射的表名
func (IndicatorRecord) TableName() string {
	return "crypto_indicators"
}

func (m *IndicatorRecord) GetInfoByMd5(db *gorm.DB, Md5 string) error {
	dbSession := db.Session(&gorm.Session{NewDB: true})

	if err := dbSession.Where("md5 = ?", Md5).First(m).Error; err != nil {
		return err
	}
	return nil
}

func (m *IndicatorRecord) InsertOrIgnoreByMd5(db *gorm.DB) error {
	dbSession := db.Session(&gorm.Session{NewDB: true})

	var existing IndicatorRecord
	err := dbSession.Where("md5 = ?", m.Md5).First(&existing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return dbSession.Create(m).Error
		}
		return err
	}

	return nil
}

// SaveIndicator 存储或更新指标数据
func (m *IndicatorRecord) SaveIndicator(db *gorm.DB) error {
	dbSession := db.Session(&gorm.Session{NewDB: true})
	return dbSession.Save(m).Error
}
