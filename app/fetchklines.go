package app

import (
	"context"
	"crypto/md5"
	"fmt"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/aiLeonardo/cryptotips/lib"
	"github.com/aiLeonardo/cryptotips/models"

	gobinance "github.com/adshao/go-binance/v2"
	"github.com/go-co-op/gocron"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

const (
	fetchBatchLimit = 1000
	fetchInterval   = 250 * time.Millisecond // 避免触发 Binance 限速
)

// 各 interval 对应币安最早可查询时间（UTC）
var klineEarliestTime = map[string]time.Time{
	"1h": time.Date(2017, 8, 17, 0, 0, 0, 0, time.UTC),
	"1d": time.Date(2017, 8, 17, 0, 0, 0, 0, time.UTC),
	"1w": time.Date(2017, 8, 14, 0, 0, 0, 0, time.UTC), // 那周的周一
}

// 增量同步回看窗口：每次从最近 N 根 K 线前开始回补，覆盖交易所对近期K线的修正
var klineLookbackBars = map[string]int{
	"1h": 12,
	"1d": 7,
	"1w": 2,
}

var klineIntervalDuration = map[string]time.Duration{
	"1h": time.Hour,
	"1d": 24 * time.Hour,
	"1w": 7 * 24 * time.Hour,
}

type KlineFetcher struct {
	db        *gorm.DB
	logger    *logrus.Logger
	client    *gobinance.Client
	scheduler *gocron.Scheduler
	syncing   atomic.Bool
}

func NewKlineFetcher() *KlineFetcher {
	return &KlineFetcher{
		db:        lib.LoadDB(lib.NewLogrusAdapter()),
		logger:    lib.LoadLogger(),
		client:    gobinance.NewClient("", ""), // 公开数据无需 API Key
		scheduler: gocron.NewScheduler(time.UTC),
	}
}

// Run 按顺序拉取指定 symbol 的各 interval K 线
func (f *KlineFetcher) Run(symbol string, intervals []string) {
	for _, interval := range intervals {
		fmt.Printf("\n======== 开始拉取 %s %s K线 ========\n", symbol, interval)
		f.logger.Infof("[fetchklines] 开始拉取 %s %s", symbol, interval)

		if err := f.fetchAll(symbol, interval); err != nil {
			f.logger.Errorf("[fetchklines] %s %s 拉取失败: %v", symbol, interval, err)
			fmt.Printf("[ERROR] %s %s 拉取失败: %v\n", symbol, interval, err)
		}
	}
	fmt.Println("\n======== 全部拉取完成 ========")
	f.logger.Infof("[fetchklines] 全部拉取完成")
}

// StartPeriodic 启动定时增量同步：先执行一次，再按 intervalMinutes 周期执行
func (f *KlineFetcher) StartPeriodic(symbol string, intervals []string, intervalMinutes int) {
	if intervalMinutes <= 0 {
		intervalMinutes = 40
	}

	f.runWithLock(symbol, intervals)

	_, err := f.scheduler.Every(intervalMinutes).Minutes().Do(func() {
		f.runWithLock(symbol, intervals)
	})
	if err != nil {
		f.logger.Errorf("[fetchklines] 注册定时任务失败: %v", err)
		return
	}

	f.scheduler.StartAsync()
	f.logger.Infof("[fetchklines] 定时增量同步已启动，每 %d 分钟执行一次", intervalMinutes)
}

// Stop 停止定时任务
func (f *KlineFetcher) Stop() {
	f.scheduler.Stop()
	f.logger.Info("[fetchklines] 定时增量同步已停止")
}

func (f *KlineFetcher) runWithLock(symbol string, intervals []string) {
	if !f.syncing.CompareAndSwap(false, true) {
		f.logger.Warn("[fetchklines] 上一轮同步尚未结束，跳过本轮")
		return
	}
	defer f.syncing.Store(false)

	f.Run(symbol, intervals)
}

// fetchAll 拉取单个 symbol/interval 的全量数据（支持断点续传）
func (f *KlineFetcher) fetchAll(symbol, interval string) error {
	model := &models.KLineRecord{}

	// 查询断点：数据库中最新 open_time
	latestTime, err := model.GetLatestKLineTime(f.db, symbol, interval)
	if err != nil {
		return fmt.Errorf("查询最新K线时间失败: %w", err)
	}

	var startTime time.Time
	earliest, ok := klineEarliestTime[interval]
	if !ok {
		earliest = time.Date(2017, 8, 17, 0, 0, 0, 0, time.UTC)
	}

	if latestTime != nil {
		lookbackBars := klineLookbackBars[interval]
		if lookbackBars <= 0 {
			lookbackBars = 3
		}
		intervalDur := klineIntervalDuration[interval]
		if intervalDur <= 0 {
			intervalDur = time.Hour
		}

		// 回看窗口：避免交易所回补/修正最近K线导致“断点续传漏更新”
		startTime = latestTime.Add(-time.Duration(lookbackBars) * intervalDur)
		if startTime.Before(earliest) {
			startTime = earliest
		}

		msg := fmt.Sprintf("[%s %s] 增量回补，从 %s 开始（最新 %s，回看 %d 根）",
			symbol, interval,
			startTime.Format("2006-01-02 15:04:05"),
			latestTime.Format("2006-01-02 15:04:05"),
			lookbackBars,
		)
		fmt.Println(msg)
		f.logger.Info(msg)
	} else {
		// 全量拉取：从币安最早时间开始
		startTime = earliest
		msg := fmt.Sprintf("[%s %s] 全量拉取，从 %s 开始", symbol, interval, startTime.Format("2006-01-02 15:04:05"))
		fmt.Println(msg)
		f.logger.Info(msg)
	}

	endTime := time.Now().UTC()
	totalInserted := int64(0)

	for {
		if startTime.After(endTime) {
			break
		}

		klines, err := f.fetchBatch(symbol, interval, startTime, endTime)
		if err != nil {
			return fmt.Errorf("拉取批次失败 startTime=%s: %w", startTime.Format("2006-01-02 15:04:05"), err)
		}

		if len(klines) == 0 {
			break
		}

		if err := model.BatchUpsertKLines(f.db, klines); err != nil {
			return fmt.Errorf("入库失败: %w", err)
		}

		totalInserted += int64(len(klines))
		last := klines[len(klines)-1]

		msg := fmt.Sprintf("[%s %s] 本批 %d 条 | 累计 %d 条 | 最新: %s",
			symbol, interval, len(klines), totalInserted, last.OpenTime.Format("2006-01-02 15:04:05"))
		fmt.Println(msg)
		f.logger.Info(msg)

		// 下一批从最后一条 CloseTime+1ms 开始
		startTime = last.CloseTime.Add(time.Millisecond)

		if len(klines) < fetchBatchLimit {
			// 数据已拉完（返回条数不足 limit）
			break
		}

		time.Sleep(fetchInterval)
	}

	total := model.GetKLineCount(f.db, symbol, interval)
	msg := fmt.Sprintf("[%s %s] 拉取完成，数据库共 %d 条", symbol, interval, total)
	fmt.Println(msg)
	f.logger.Info(msg)

	// 同步元数据表，避免 API 查询时全表 DISTINCT 扫描
	meta := &models.KLineMetaRecord{}
	if err := meta.Sync(f.db, symbol, interval); err != nil {
		f.logger.Warnf("[fetchklines] 元数据同步失败 %s %s: %v", symbol, interval, err)
	}

	return nil
}

// fetchBatch 调用 Binance SDK 拉取一批 K 线
func (f *KlineFetcher) fetchBatch(symbol, interval string, startTime, endTime time.Time) ([]models.KLineRecord, error) {
	raw, err := f.client.NewKlinesService().
		Symbol(symbol).
		Interval(interval).
		StartTime(startTime.UnixMilli()).
		EndTime(endTime.UnixMilli()).
		Limit(fetchBatchLimit).
		Do(context.Background())
	if err != nil {
		return nil, fmt.Errorf("Binance API请求失败: %w", err)
	}

	records := make([]models.KLineRecord, 0, len(raw))
	for _, k := range raw {
		openTime := time.UnixMilli(k.OpenTime).UTC()
		closeTime := time.UnixMilli(k.CloseTime).UTC()

		open, _ := strconv.ParseFloat(k.Open, 64)
		high, _ := strconv.ParseFloat(k.High, 64)
		low, _ := strconv.ParseFloat(k.Low, 64)
		close_, _ := strconv.ParseFloat(k.Close, 64)
		volume, _ := strconv.ParseFloat(k.Volume, 64)
		quoteVolume, _ := strconv.ParseFloat(k.QuoteAssetVolume, 64)

		md5Key := fmt.Sprintf("%x", md5.Sum([]byte(
			fmt.Sprintf("%s_%s_%d", symbol, interval, k.OpenTime),
		)))

		records = append(records, models.KLineRecord{
			Md5:         md5Key,
			Symbol:      symbol,
			Interval:    interval,
			OpenTime:    openTime,
			CloseTime:   closeTime,
			Open:        open,
			High:        high,
			Low:         low,
			Close:       close_,
			Volume:      volume,
			QuoteVolume: quoteVolume,
		})
	}

	return records, nil
}
