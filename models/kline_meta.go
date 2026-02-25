package models

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// KLineMetaRecord 记录每个 symbol/interval 的元数据，避免全表 DISTINCT 扫描
// 在每次 fetchAll 完成后 upsert，读时直接走主键索引
type KLineMetaRecord struct {
	ID           uint      `gorm:"primaryKey;autoIncrement"`
	Symbol       string    `gorm:"type:varchar(20);not null;uniqueIndex:idx_meta_sym_int"`
	Interval     string    `gorm:"type:varchar(10);not null;uniqueIndex:idx_meta_sym_int"`
	KlineCount   int64     `gorm:"not null;default:0"`
	EarliestTime time.Time `gorm:"not null"`
	LatestTime   time.Time `gorm:"not null"`
	UpdatedAt    time.Time `gorm:"not null"`
}

func (KLineMetaRecord) TableName() string {
	return "crypto_kline_meta"
}

// Sync 从 crypto_kline 聚合统计后 upsert 到 crypto_kline_meta
// 每次 fetchAll 结束时调用，保持元数据与主表同步
func (m *KLineMetaRecord) Sync(db *gorm.DB, symbol, interval string) error {
	type agg struct {
		KlineCount   int64
		EarliestTime time.Time
		LatestTime   time.Time
	}
	var a agg
	err := db.Session(&gorm.Session{NewDB: true}).
		Model(&KLineRecord{}).
		Select("COUNT(*) AS kline_count, MIN(open_time) AS earliest_time, MAX(open_time) AS latest_time").
		Where("symbol = ? AND `interval` = ?", symbol, interval).
		Scan(&a).Error
	if err != nil {
		return err
	}

	rec := KLineMetaRecord{
		Symbol:       symbol,
		Interval:     interval,
		KlineCount:   a.KlineCount,
		EarliestTime: a.EarliestTime,
		LatestTime:   a.LatestTime,
		UpdatedAt:    time.Now().UTC(),
	}

	return db.Session(&gorm.Session{NewDB: true}).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "symbol"}, {Name: "interval"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"kline_count", "earliest_time", "latest_time", "updated_at",
			}),
		}).
		Create(&rec).Error
}

// ListAll 返回全部元数据行，按 symbol/interval 升序
func (m *KLineMetaRecord) ListAll(db *gorm.DB) ([]KLineMetaRecord, error) {
	var rows []KLineMetaRecord
	err := db.Session(&gorm.Session{NewDB: true}).
		Order("symbol ASC, `interval` ASC").
		Find(&rows).Error
	return rows, err
}
