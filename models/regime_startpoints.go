package models

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// RegimeStartpointRecord 市场状态起点（用于前端 K 线打点）
type RegimeStartpointRecord struct {
	ID         uint      `gorm:"primaryKey;autoIncrement"`
	Symbol     string    `gorm:"type:varchar(20);not null;uniqueIndex:idx_regime_symbol_start"`
	StartTime  time.Time `gorm:"not null;uniqueIndex:idx_regime_symbol_start;index:idx_regime_symbol_state_time,priority:2"`
	State      string    `gorm:"type:varchar(10);not null;index:idx_regime_symbol_state_time,priority:3"` // BULL / BEAR / RANGE
	Confidence float64   `gorm:"type:decimal(10,6);not null;default:0"`
	BasisA     string    `gorm:"type:text"`
	BasisB     string    `gorm:"type:text"`
	Source     string    `gorm:"type:varchar(120);not null;default:'';index:idx_regime_symbol_state_time,priority:1"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (RegimeStartpointRecord) TableName() string {
	return "crypto_regime_startpoints"
}

func (m *RegimeStartpointRecord) BatchUpsert(db *gorm.DB, records []RegimeStartpointRecord) error {
	if len(records) == 0 {
		return nil
	}

	dbSession := db.Session(&gorm.Session{NewDB: true})
	return dbSession.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "symbol"}, {Name: "start_time"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"state", "confidence", "basis_a", "basis_b", "source", "updated_at",
		}),
	}).CreateInBatches(&records, 200).Error
}

func (m *RegimeStartpointRecord) ListBySymbolAndRange(db *gorm.DB, symbol string, start, end time.Time) ([]RegimeStartpointRecord, error) {
	recs := make([]RegimeStartpointRecord, 0)

	dbSession := db.Session(&gorm.Session{NewDB: true})
	q := dbSession.Where("symbol = ?", symbol)
	if !start.IsZero() {
		q = q.Where("start_time >= ?", start)
	}
	if !end.IsZero() {
		q = q.Where("start_time <= ?", end)
	}
	err := q.Order("start_time ASC").Find(&recs).Error
	return recs, err
}
