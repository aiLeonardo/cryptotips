package app

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/aiLeonardo/cryptotips/lib"
	"github.com/aiLeonardo/cryptotips/models"

	"github.com/go-co-op/gocron"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

const fngAPIBase = "https://api.alternative.me/fng/"

// fngAPIResponse alternative.me API 响应结构
type fngAPIResponse struct {
	Data []struct {
		Value               string `json:"value"`
		ValueClassification string `json:"value_classification"`
		Timestamp           string `json:"timestamp"`
	} `json:"data"`
	Metadata struct {
		Error any `json:"error"`
	} `json:"metadata"`
}

// FearGreedFetcher 贪婪恐慌指数定时收集器
type FearGreedFetcher struct {
	db        *gorm.DB
	logger    *logrus.Logger
	scheduler *gocron.Scheduler
}

// NewFearGreedFetcher 初始化收集器，并自动建表
func NewFearGreedFetcher() *FearGreedFetcher {
	db := lib.LoadDB(lib.NewLogrusAdapter())
	logger := lib.LoadLogger()

	// 自动建表（表不存在时创建）
	if err := db.AutoMigrate(&models.FearGreedIndex{}); err != nil {
		logger.Errorf("[feargreed] AutoMigrate 失败: %v", err)
	}

	return &FearGreedFetcher{
		db:        db,
		logger:    logger,
		scheduler: gocron.NewScheduler(time.UTC),
	}
}

// Start 先全量同步历史数据，再按配置间隔启动定时任务（非阻塞）
// 间隔从配置项 feargreed.interval_hours 读取，未配置时默认 4 小时
func (f *FearGreedFetcher) Start() {
	intervalHours := viper.GetInt("feargreed.interval_hours")
	if intervalHours <= 0 {
		intervalHours = 4
	}

	// 启动时先全量同步历史数据，再走定期采集
	f.SyncAllHistory()

	_, err := f.scheduler.Every(intervalHours).Hours().Do(f.fetchAndSave)
	if err != nil {
		f.logger.Errorf("[feargreed] 注册定时任务失败: %v", err)
		return
	}

	f.scheduler.StartAsync()
	f.logger.Infof("[feargreed] 定时采集已启动，每 %d 小时收集一次恐慌贪婪指数", intervalHours)
}

// SyncAllHistory 一次性从 alternative.me 拉取全部历史数据并入库（去重写入）
func (f *FearGreedFetcher) SyncAllHistory() {
	f.logger.Info("[feargreed] 开始全量同步历史贪婪恐慌指数...")
	fmt.Println("[feargreed] 开始全量同步历史贪婪恐慌指数...")

	apiResp, err := f.fetchFromAPI(0)
	if err != nil {
		f.logger.Errorf("[feargreed] 全量同步失败: %v", err)
		return
	}

	total := len(apiResp.Data)
	inserted := 0
	for _, d := range apiResp.Data {
		value, err := strconv.Atoi(d.Value)
		if err != nil {
			f.logger.Warnf("[feargreed] 跳过无效 value=%s: %v", d.Value, err)
			continue
		}
		fngTs, err := strconv.ParseInt(d.Timestamp, 10, 64)
		if err != nil {
			f.logger.Warnf("[feargreed] 跳过无效 timestamp=%s: %v", d.Timestamp, err)
			continue
		}

		md5Key := fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("feargreed_%d", fngTs))))
		record := &models.FearGreedIndex{
			Md5:                 md5Key,
			Value:               value,
			ValueClassification: d.ValueClassification,
			FngTimestamp:        fngTs,
		}
		if err := record.InsertOrIgnoreByMd5(f.db); err != nil {
			f.logger.Errorf("[feargreed] 写入失败 fng_date=%s: %v",
				time.Unix(fngTs, 0).UTC().Format("2006-01-02"), err)
			continue
		}
		inserted++
	}

	msg := fmt.Sprintf("[feargreed] 全量同步完成: API返回 %d 条，新入库 %d 条", total, inserted)
	f.logger.Info(msg)
	fmt.Println(msg)
}

// Stop 停止定时任务
func (f *FearGreedFetcher) Stop() {
	f.scheduler.Stop()
	f.logger.Info("[feargreed] 定时采集已停止")
}

// fetchAndSave 从 alternative.me 拉取最新指数并写入 MySQL
func (f *FearGreedFetcher) fetchAndSave() {
	f.logger.Info("[feargreed] 开始拉取贪婪恐慌指数...")

	apiResp, err := f.fetchFromAPI(1)
	if err != nil {
		f.logger.Errorf("[feargreed] 拉取失败: %v", err)
		return
	}

	if len(apiResp.Data) == 0 {
		f.logger.Error("[feargreed] API 返回数据为空")
		return
	}

	d := apiResp.Data[0]

	value, err := strconv.Atoi(d.Value)
	if err != nil {
		f.logger.Errorf("[feargreed] 解析 value 失败: %v", err)
		return
	}

	fngTs, err := strconv.ParseInt(d.Timestamp, 10, 64)
	if err != nil {
		f.logger.Errorf("[feargreed] 解析 timestamp 失败: %v", err)
		return
	}

	md5Key := fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("feargreed_%d", fngTs))))

	record := &models.FearGreedIndex{
		Md5:                 md5Key,
		Value:               value,
		ValueClassification: d.ValueClassification,
		FngTimestamp:        fngTs,
	}

	if err := record.InsertOrIgnoreByMd5(f.db); err != nil {
		f.logger.Errorf("[feargreed] 写入数据库失败: %v", err)
		return
	}

	msg := fmt.Sprintf("[feargreed] 采集成功: value=%d (%s) fng_date=%s",
		value,
		d.ValueClassification,
		time.Unix(fngTs, 0).UTC().Format("2006-01-02"),
	)
	f.logger.Info(msg)
	fmt.Println(msg)
}

// fetchFromAPI 向 alternative.me 发请求，limit=0 表示获取全部历史
func (f *FearGreedFetcher) fetchFromAPI(limit int) (*fngAPIResponse, error) {
	url := fmt.Sprintf("%s?limit=%d", fngAPIBase, limit)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应内容失败: %w", err)
	}

	var apiResp fngAPIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %w, raw=%s", err, string(body))
	}

	return &apiResp, nil
}
