package models

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// FearGreedIndex 比特币贪婪恐慌指数历史记录表
// 每次定时收集生成一条记录，用于追踪各时间点的市场情绪
type FearGreedIndex struct {
	ID                  uint      `gorm:"primaryKey;autoIncrement"`
	Md5                 string    `gorm:"type:varchar(65);not null;uniqueIndex:idx_fng_md5;comment:基于FngTimestamp的MD5，防重复写入"`
	Value               int       `gorm:"not null;comment:恐慌贪婪指数值 0-100"`
	ValueClassification string    `gorm:"type:varchar(50);not null;comment:分类：Extreme Fear/Fear/Neutral/Greed/Extreme Greed"`
	FngTimestamp        int64     `gorm:"not null;comment:API数据的计算时间(Unix秒)，由alternative.me每日更新"`
	CreatedAt           time.Time `gorm:"index"`
}

// TableName 指定 GORM 映射的表名
func (FearGreedIndex) TableName() string {
	return "crypto_feargreed_index"
}

// Insert 插入一条恐慌指数记录（无去重，调用方需自行保证唯一性）
func (m *FearGreedIndex) Insert(db *gorm.DB) error {
	return db.Session(&gorm.Session{NewDB: true}).Create(m).Error
}

// InsertOrIgnoreByMd5 基于 Md5 去重写入：记录已存在则跳过，不存在则插入
func (m *FearGreedIndex) InsertOrIgnoreByMd5(db *gorm.DB) error {
	dbSession := db.Session(&gorm.Session{NewDB: true})

	var existing FearGreedIndex
	err := dbSession.Where("md5 = ?", m.Md5).First(&existing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return dbSession.Create(m).Error
		}
		return err
	}

	return nil
}

// GetLatest 获取最新一条收集记录
func (m *FearGreedIndex) GetLatest(db *gorm.DB) (*FearGreedIndex, error) {
	var rec FearGreedIndex
	err := db.Session(&gorm.Session{NewDB: true}).
		Order("fng_timestamp DESC").
		First(&rec).Error
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// GetHistory 按收集时间倒序查询历史记录
func (m *FearGreedIndex) GetHistory(db *gorm.DB, limit int) ([]FearGreedIndex, error) {
	var records []FearGreedIndex
	err := db.Session(&gorm.Session{NewDB: true}).
		Order("fng_timestamp DESC").
		Limit(limit).
		Find(&records).Error
	return records, err
}
