package app

import (
	"fmt"
	"os"
	"time"

	"github.com/aiLeonardo/cryptotips/lib"

	rdebug "runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

type goapi struct {
	logger  *logrus.Logger
	db      *gorm.DB
	redisDb *lib.RedisHandler

	resourceMap map[string]map[string]string
	resTypeMap  map[string]string
}

func NewGoapi() *goapi {
	logger := lib.LoadLogger()
	logrusAdapter := lib.NewLogrusAdapter()
	redisLogger := lib.NewRedisLogger()
	rdb := lib.LoadRedis(redisLogger)

	a := &goapi{
		logger:  logger,
		db:      lib.LoadDB(logrusAdapter),
		redisDb: lib.NewRedisHandler(rdb, logger),
	}

	return a
}

func (a *goapi) customRecovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// 打印堆栈信息
				stack := string(rdebug.Stack())
				a.logger.Errorf("程序运行异常: %v, stack: %s", err, stack)

				// 返回一个统一的错误响应
				lib.JsonError(c, fmt.Errorf("服务器内部发生错误 %v", err))
				// 停止请求链，不再调用后续的处理器
				c.Abort()
			}
		}()
		c.Next()
	}
}

func (a *goapi) loggerMiddler() func(*gin.Context) {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next() // 处理请求
		end := time.Now()
		latency := end.Sub(start)

		a.logger.Infof("[GoApi] %s %s %s %s",
			c.Request.Method,
			c.Request.RequestURI,
			c.ClientIP(),
			latency,
		)
	}
}

func (a *goapi) corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

func (a *goapi) Start() {
	var apiServicePort int = viper.GetInt("apiservice.port")
	var isLocalEnv bool = a.isLocalEnv()
	if !isLocalEnv {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(a.corsMiddleware())
	r.Use(a.loggerMiddler())
	r.Use(a.customRecovery())
	a.routers(r)

	r.Run(fmt.Sprintf(":%d", apiServicePort))
}

func (a *goapi) routers(r *gin.Engine) {
	// 路由分组，用于组织 API
	api := r.Group("/api")
	{
		// GET /api/list
		api.GET("/list", a.getDatalist)

		// K 线数据接口
		// GET /api/klines?symbol=BTCUSDT&interval=1d&limit=500
		// GET /api/klines?symbol=BTCUSDT&interval=1h&start=1609459200000&end=1640995200000
		api.GET("/klines", a.getKLines)

		// GET /api/klines/meta  — 返回数据库已有的 symbol / interval 列表
		api.GET("/klines/meta", a.getKLinesMeta)

		// GET /api/feargreed/history?limit=90  — 返回恐慌贪婪指数历史
		api.GET("/feargreed/history", a.getFearGreedHistory)
	}

}

func (a *goapi) getDatalist(c *gin.Context) {
	var req lib.DatalistReq
	if err := c.ShouldBindJSON(&req); err != nil {
		lib.JsonError(c, err)
		return
	}

	lib.JsonError(c, fmt.Errorf("this type: %s not found", req.Type))
}

func (a *goapi) isLocalEnv() bool {
	// 读取环境变量.
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "dev"
	}

	return env == "dev"
}

func (a *goapi) fileExists(inputPath string) bool {
	info, err := os.Stat(inputPath)
	return err == nil && !info.IsDir()
}

func (a *goapi) removeFile(inputPath string) bool {
	err := os.Remove(inputPath)
	if err != nil {
		a.logger.Errorf("删除文件失败: %s", err)
		return false
	}

	return true
}
