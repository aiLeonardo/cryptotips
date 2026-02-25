package models

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// KLineRecord K 线数据表
// 唯一索引 (symbol, interval, open_time) 保证不重复存储，支持 UPSERT
type KLineRecord struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	Md5       string    `gorm:"type:varchar(65);not null;"`
	Symbol    string    `gorm:"type:varchar(20);not null;uniqueIndex:idx_sym_int_time"`
	Interval  string    `gorm:"type:varchar(10);not null;uniqueIndex:idx_sym_int_time"`
	OpenTime  time.Time `gorm:"not null;uniqueIndex:idx_sym_int_time"`
	CloseTime time.Time `gorm:"not null"`
	Open      float64   `gorm:"type:decimal(20,8)"`
	High      float64   `gorm:"type:decimal(20,8)"`
	Low       float64   `gorm:"type:decimal(20,8)"`
	Close     float64   `gorm:"type:decimal(20,8)"`
	Volume      float64 `gorm:"type:decimal(30,8)"`
	QuoteVolume float64 `gorm:"type:decimal(30,8)"`
}

// TableName 指定 GORM 映射的表名
func (KLineRecord) TableName() string {
	return "crypto_kline"
}

func (m *KLineRecord) GetInfoByMd5(db *gorm.DB, Md5 string) error {
	dbSession := db.Session(&gorm.Session{NewDB: true})

	if err := dbSession.Where("md5 = ?", Md5).First(m).Error; err != nil {
		return err
	}
	return nil
}

func (m *KLineRecord) InsertOrIgnoreByMd5(db *gorm.DB) error {
	dbSession := db.Session(&gorm.Session{NewDB: true})

	var existing KLineRecord
	err := dbSession.Where("md5 = ?", m.Md5).First(&existing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return dbSession.Create(m).Error
		}
		return err
	}

	return nil
}

// 使用 ON DUPLICATE KEY UPDATE 更新 OHLCV，保证数据最新（防止未收盘 K 线旧数据残留）
func (m *KLineRecord) BatchUpsertKLines(db *gorm.DB, records []KLineRecord) error {
	if len(records) == 0 {
		return nil
	}

	dbSession := db.Session(&gorm.Session{NewDB: true})
	return dbSession.Clauses(clause.OnConflict{
		DoUpdates: clause.AssignmentColumns([]string{
			"close_time", "open", "high", "low", "close", "volume", "quote_volume",
		}),
	}).CreateInBatches(&records, 500).Error
}

// GetKLines 从 MySQL 查询指定范围的 K 线（按 open_time 升序）
// interval 是 MySQL 保留字，需用反引号转义
func (m *KLineRecord) GetKLines(db *gorm.DB, symbol, interval string, start, end time.Time) ([]KLineRecord, error) {
	var records []KLineRecord

	dbSession := db.Session(&gorm.Session{NewDB: true})
	q := dbSession.Where("symbol = ? AND `interval` = ?", symbol, interval)
	if !start.IsZero() {
		q = q.Where("open_time >= ?", start)
	}
	if !end.IsZero() {
		q = q.Where("open_time <= ?", end)
	}
	err := q.Order("open_time ASC").Find(&records).Error
	return records, err
}

// GetLatestKLineTime 查询本地某 symbol/interval 最新的 open_time
// 返回 nil 表示本地无数据
func (m *KLineRecord) GetLatestKLineTime(db *gorm.DB, symbol, interval string) (*time.Time, error) {
	var rec KLineRecord

	dbSession := db.Session(&gorm.Session{NewDB: true})
	err := dbSession.Where("symbol = ? AND `interval` = ?", symbol, interval).
		Order("open_time DESC").First(&rec).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	t := rec.OpenTime
	return &t, nil
}

// GetKLineCount 查询本地 K 线总条数（用于日志）
func (m *KLineRecord) GetKLineCount(db *gorm.DB, symbol, interval string) int64 {
	var count int64

	dbSession := db.Session(&gorm.Session{NewDB: true})
	dbSession.Model(&KLineRecord{}).Where("symbol = ? AND `interval` = ?", symbol, interval).Count(&count)
	return count
}
