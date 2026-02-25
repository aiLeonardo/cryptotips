/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"github.com/aiLeonardo/cryptotips/app"
	"github.com/aiLeonardo/cryptotips/lib"

	"github.com/spf13/cobra"
)

var isGoapiDaemon bool

func init() {
	goapiCmd.Flags().BoolVarP(
		&isGoapiDaemon,
		"daemon",
		"d",
		true,
		"run goapi in background (daemon mode)",
	)
}

// rootCmd represents the base command when called without any subcommands
var goapiCmd = &cobra.Command{
	Use:   "goapi",
	Short: "goapi web service",
	Long:  `goapi web service.提供接口服务`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {
		if isGoapiDaemon {
			daemonCtx := lib.DaemonStart("goapi")
			defer daemonCtx.Release()
		}

		lib.SetLogFilePath("./logs/goapi.log")
		logger := lib.LoadLogger()
		defer lib.RecoverLogMsg(logger)

		// cpu, memory 资源检测报告
		stats := lib.NewSystemStats()
		defer stats.StopTicker()

		mainEndSign := make(chan struct{})
		goapi := app.NewGoapi()
		go goapi.Start()
		logger.Infof("goapi启动完成.")

		// 等待退出信号
		lib.WaitSIGTERM(mainEndSign)
	},
}
