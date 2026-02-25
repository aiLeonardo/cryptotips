package cmd

import (
	"github.com/aiLeonardo/cryptotips/app"
	"github.com/aiLeonardo/cryptotips/lib"

	"github.com/spf13/cobra"
)

var fetchklinesCmd = &cobra.Command{
	Use:   "fetchklines",
	Short: "从 Binance 全量拉取 BTCUSDT K线数据入库",
	Long: `一次性任务：从 Binance 公开 API 拉取 BTCUSDT 的 1h / 1d / 1w K线数据，
从币安最早可查时间起，到今天为止，全量写入 MySQL crypto_kline 表。
支持断点续传：重复执行只补拉缺失部分。`,
	Run: func(cmd *cobra.Command, args []string) {
		lib.SetLogFilePath("./logs/fetchklines.log")
		logger := lib.LoadLogger()
		defer lib.RecoverLogMsg(logger)

		fetcher := app.NewKlineFetcher()
		fetcher.Run("BTCUSDT", []string{"1h", "1d", "1w"})
	},
}

func init() {
	rootCmd.AddCommand(fetchklinesCmd)
}
