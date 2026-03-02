package lib

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aiLeonardo/cryptotips/models"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var dbConnect *gorm.DB
var redisConnect *redis.Client

// LoadDB reads database config and initializes a GORM DB object
func LoadDB(logger *LogrusAdapter) *gorm.DB {
	if dbConnect != nil {
		return dbConnect
	}
	enabledb := viper.GetBool("dbinfo.enabledb")
	if !enabledb {
		fmt.Printf("dbinfo.enabledb is false")
		return nil
	}
	// Construct DSN: user:password@tcp(host:port)/dbname?charset=utf8mb4&parseTime=True&loc=Local
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
		viper.GetString("dbinfo.user"),
		viper.GetString("dbinfo.password"),
		viper.GetString("dbinfo.host"),
		viper.GetInt("dbinfo.port"),
		viper.GetString("dbinfo.dbname"),
		viper.GetString("dbinfo.charset"),
	)

	var err error
	dbConnect, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger,
	})
	if err != nil {
		fmt.Printf("gorm open mysql error: %v\n", err)
		os.Exit(1)
	}

	if err := dbConnect.AutoMigrate(
		&models.KLineRecord{},
		&models.KLineMetaRecord{},
		&models.IndicatorRecord{},
		&models.StrategyLogRecord{},
		&models.TradeRecord{},
		&models.FearGreedIndex{},
		&models.RegimeStartpointRecord{},
	); err != nil {
		fmt.Printf("gorm auto migrate error: %v\n", err)
		os.Exit(1)
	}
	if err := models.EnsureStrategySchema(dbConnect); err != nil {
		fmt.Printf("ensure strategy schema error: %v\n", err)
		os.Exit(1)
	}

	return dbConnect
}

// LoadDB reads database config and initializes a GORM DB object
func LoadRedis(logger *RedisLogger) *redis.Client {
	if redisConnect != nil {
		return redisConnect
	}
	enabledb := viper.GetBool("redis.enabledb")
	if !enabledb {
		fmt.Printf("redis.enabledb is false")
		return nil
	}

	redis.SetLogger(logger)
	redis.SetLogLevel(2)

	redisHost := viper.GetString("redis.host")
	redisPort := viper.GetInt("redis.port")
	redisDB := viper.GetInt("redis.db")
	password := viper.GetString("redis.password")
	redisConnect = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", redisHost, redisPort),
		Password: password,
		DB:       redisDB,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := redisConnect.Ping(ctx).Err()
	if err != nil {
		fmt.Printf("redis ping error: %v\n", err)
		os.Exit(1)
	}

	return redisConnect
}
