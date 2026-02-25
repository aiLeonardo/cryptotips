package cmd

import (
	"github.com/aiLeonardo/cryptotips/app"
	"github.com/aiLeonardo/cryptotips/lib"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(feargreedCmd)
}

var feargreedCmd = &cobra.Command{
	Use:   "feargreed",
	Short: "定时收集比特币贪婪恐慌指数",
	Long: `定时任务：每4小时从 alternative.me 拉取比特币贪婪恐慌指数，
持久化保存到 MySQL fear_greed_index 表，形成历史数据供后续分析。`,
	Run: func(cmd *cobra.Command, args []string) {
		daemonCtx := lib.DaemonStart("feargreed")
		defer daemonCtx.Release()

		lib.SetLogFilePath("./logs/feargreed.log")
		logger := lib.LoadLogger()
		defer lib.RecoverLogMsg(logger)

		mainEndSign := make(chan struct{})

		fetcher := app.NewFearGreedFetcher()
		fetcher.Start()
		logger.Info("feargreed 服务已启动，按 Ctrl+C 或发送 SIGTERM 停止")

		lib.WaitSIGTERM(mainEndSign)
		fetcher.Stop()
	},
}
