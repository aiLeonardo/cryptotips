package lib

import (
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/sirupsen/logrus" // 导入 Logrus 库
)

var logFilePath string
var rusLogger *logrus.Logger

func SetLogFilePath(filePath string) bool {
	if filePath == "" {
		fmt.Printf("请设置正确的日志文件%s.\n", filePath)
		return false
	}
	filePath = strings.ReplaceAll(filePath, ".go", ".log")
	logdir := path.Dir(filePath)
	// 创建所有上级目录（如果不存在）
	err := os.MkdirAll(logdir, 0755)
	if err != nil {
		fmt.Println("创建目录失败:", err)
		return false
	}

	logFilePath = filePath
	fmt.Printf("当前logger文件: %s\n", logFilePath)

	return true
}

func LoadLogger() *logrus.Logger {
	if logFilePath == "" {
		fmt.Printf("请设置正确的日志文件%s.\n", logFilePath)
		return nil
	}

	if rusLogger != nil {
		return rusLogger
	}

	// 1. 创建 Logrus 实例
	// 使用 logrus.New() 而不是直接使用 logrus.Logger 全局实例，
	// 这样可以创建独立的日志器，避免污染全局配置。
	rusLogger := logrus.New()

	// 2. 配置日志文件
	logFile, err := os.OpenFile(
		logFilePath,
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0644, // 文件权限
	)
	if err != nil {
		rusLogger.Fatalf("无法打开日志文件: %v", err) // 使用 rusLogger.Fatalf 记录错误并退出
		return nil
	}
	// defer logFile.Close() // 确保文件在程序退出时关闭

	// 3. 配置 Logrus 输出目标
	// 同时输出到控制台和文件。
	// 这里使用 io.MultiWriter 来实现多重输出。
	// 注意：logrus.SetOutput(io.MultiWriter(os.Stdout, logFile)) 也可以，
	// 但如果使用 logrus.New() 创建独立实例，应该设置其自身的 Output。
	rusLogger.SetOutput(logFile) // 将日志输出到文件
	// 如果你还想在控制台看到输出，可以添加一个额外的 hook 或者使用 MultiWriter：
	// rusLogger.SetOutput(io.MultiWriter(os.Stdout, logFile))

	// 4. 配置 Logrus 格式化器
	// 设置为 JSON 格式，便于机器解析。你也可以选择 TextFormatter。
	rusLogger.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: time.RFC3339, // 设置时间戳格式
	})
	// 如果你想使用更易读的文本格式，可以使用：
	// rusLogger.SetFormatter(&logrus.TextFormatter{
	// 	FullTimestamp:   true,
	// 	TimestampFormat: "2006-01-02 15:04:05", // 自定义时间格式
	// 	ForceColors:     true,                  // 在终端中强制彩色输出
	// })

	// 5. 配置日志级别
	// 设置为 Info 级别，低于 Info 的日志（如 Debug）将不会被记录。
	// rusLogger.SetLevel(logrus.InfoLevel)
	rusLogger.SetLevel(logrus.InfoLevel)
	// 如果想显示所有日志，包括 Debug，可以设置为 DebugLevel：
	// rusLogger.SetLevel(logrus.DebugLevel)
	// 添加 Hook：输出到第三方 API
	apiHook := &apiHook{
		Taskname: path.Base(strings.ReplaceAll(logFilePath, ".log", ".go")),
	}
	rusLogger.AddHook(apiHook)

	return rusLogger
}

type apiHook struct {
	Taskname string
}

func (hook *apiHook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.ErrorLevel,
		logrus.FatalLevel,
		logrus.PanicLevel,
		logrus.WarnLevel,
	}
}

func (hook *apiHook) Fire(entry *logrus.Entry) error {
	// 发送 HTTP POST 请求
	NoticeWhenError(hook.Taskname, entry.Message)
	return nil
}

func loggerTest() {
	SetLogFilePath("./files/xxx.log")
	logger := LoadLogger()
	// 6. 使用日志器记录日志
	logger.Info("这是一个信息级别的日志。")
	logger.Debug("这是一个调试级别的日志，如果级别设置为 Info，将不会显示。")
	logger.WithFields(logrus.Fields{
		"user_id": 456,
		"action":  "purchase_item",
		"item_id": 789,
	}).Warn("这是一个警告级别的日志，带有一些字段。")

	err := fmt.Errorf("处理订单时发生错误")
	logger.WithError(err).Error("这是一个错误级别的日志，包含一个 Go 错误。")

	fmt.Println("日志已写入 xxx.log 文件。")
	fmt.Println("请查看 xxx.log 文件获取日志详情。")
}
