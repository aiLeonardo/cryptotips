package models

import (
	"fmt"

	"gorm.io/gorm"
)

// EnsureStrategySchema 幂等迁移 strategy_id 字段与复合索引
func EnsureStrategySchema(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}

	if err := ensureStrategyIDColumn(db, (&TradeRecord{}).TableName(), "varchar(100)"); err != nil {
		return err
	}
	if err := ensureStrategyIDColumn(db, (&StrategyLogRecord{}).TableName(), "varchar(100)"); err != nil {
		return err
	}

	if !db.Migrator().HasIndex(&TradeRecord{}, "idx_strategy_symbol_status") {
		if err := db.Migrator().CreateIndex(&TradeRecord{}, "idx_strategy_symbol_status"); err != nil {
			return err
		}
	}
	if !db.Migrator().HasIndex(&StrategyLogRecord{}, "idx_strategy_symbol_date") {
		if err := db.Migrator().CreateIndex(&StrategyLogRecord{}, "idx_strategy_symbol_date"); err != nil {
			return err
		}
	}

	return nil
}

func ensureStrategyIDColumn(db *gorm.DB, tableName, colType string) error {
	if !db.Migrator().HasColumn(tableName, "strategy_id") {
		if err := db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN strategy_id %s NOT NULL DEFAULT ''", tableName, colType)).Error; err != nil {
			return err
		}
	}
	if err := db.Exec(fmt.Sprintf("UPDATE %s SET strategy_id = '' WHERE strategy_id IS NULL", tableName)).Error; err != nil {
		return err
	}
	if err := db.Exec(fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN strategy_id %s NOT NULL DEFAULT ''", tableName, colType)).Error; err != nil {
		return err
	}
	return nil
}
