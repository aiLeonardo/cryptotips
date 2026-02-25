package lib

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// LoadDB reads database config and initializes a GORM DB object
func LoadConfig() {
	// 加载 .env 文件
	err := godotenv.Load()
	if err != nil {
		fmt.Println(".env 文件未找到，使用默认环境")
	}

	// 读取环境变量.
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "dev"
	}

	viper.SetConfigName(fmt.Sprintf("config.%s", env))
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")

	viper.AutomaticEnv() // 支持从环境变量读取

	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("无法读取配置文件: %v \n", err)
	}

	fmt.Println("使用配置文件:", viper.ConfigFileUsed())
}
