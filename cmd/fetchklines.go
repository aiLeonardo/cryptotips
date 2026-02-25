package cmd

import (
	"github.com/aiLeonardo/cryptotips/app"
	"github.com/aiLeonardo/cryptotips/lib"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	isFetchKlinesDaemon bool
	fetchKlinesOnce     bool
	fetchKlinesMinutes  int
)

var fetchklinesCmd = &cobra.Command{
	Use:   "fetchklines",
	Short: "从 Binance 拉取并定时增量同步 BTCUSDT K线数据",
	Long: `支持两种模式：
1) 一次性任务：全量/断点拉取 BTCUSDT 的 1h / 1d / 1w K线入库
2) 定时任务：首次执行后，每隔 N 分钟自动增量同步（默认 40 分钟）`,
	Run: func(cmd *cobra.Command, args []string) {
		if isFetchKlinesDaemon {
			daemonCtx := lib.DaemonStart("fetchklines")
			defer daemonCtx.Release()
		}

		lib.SetLogFilePath("./logs/fetchklines.log")
		logger := lib.LoadLogger()
		defer lib.RecoverLogMsg(logger)

		fetcher := app.NewKlineFetcher()
		symbol := "BTCUSDT"
		intervals := []string{"1h", "1d", "1w"}

		if fetchKlinesOnce {
			fetcher.Run(symbol, intervals)
			return
		}

		minutes := fetchKlinesMinutes
		if minutes <= 0 {
			minutes = viper.GetInt("kline_sync.interval_minutes")
		}
		if minutes <= 0 {
			minutes = 40
		}

		mainEndSign := make(chan struct{})
		fetcher.StartPeriodic(symbol, intervals, minutes)
		logger.Infof("fetchklines 定时增量同步已启动，每 %d 分钟执行一次", minutes)

		lib.WaitSIGTERM(mainEndSign)
		fetcher.Stop()
	},
}

func init() {
	fetchklinesCmd.Flags().BoolVarP(
		&isFetchKlinesDaemon,
		"daemon",
		"d",
		true,
		"run fetchklines in background (daemon mode)",
	)
	fetchklinesCmd.Flags().BoolVar(
		&fetchKlinesOnce,
		"once",
		false,
		"run once and exit (disable periodic sync)",
	)
	fetchklinesCmd.Flags().IntVar(
		&fetchKlinesMinutes,
		"interval-minutes",
		40,
		"periodic incremental sync interval in minutes",
	)

	rootCmd.AddCommand(fetchklinesCmd)
}
