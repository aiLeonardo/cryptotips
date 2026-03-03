package cmd

import (
	"context"
	"time"

	"github.com/aiLeonardo/cryptotips/app/tgclient"
	"github.com/aiLeonardo/cryptotips/lib"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	tgAppID       int
	tgAppHash     string
	tgPhone       string
	tgSessionFile string
	tgOutputDir   string
	tgChannel     string
	tgLimit       int
	tgPageSize    int
)

var tgsyncCmd = &cobra.Command{
	Use:   "tgsync",
	Short: "同步 Telegram 频道历史并生成静态页面",
	Run: func(cmd *cobra.Command, args []string) {
		lib.SetLogFilePath("./logs/tgsync.log")
		logger := lib.LoadLogger()
		defer lib.RecoverLogMsg(logger)

		if tgAppID <= 0 {
			tgAppID = viper.GetInt("tg_client.app_id")
		}
		if tgAppHash == "" {
			tgAppHash = viper.GetString("tg_client.app_hash")
		}
		if tgPhone == "" {
			tgPhone = viper.GetString("tg_client.phone")
		}
		if tgSessionFile == "" {
			tgSessionFile = viper.GetString("tg_client.session_file")
		}
		if tgOutputDir == "" {
			tgOutputDir = viper.GetString("tg_client.output_dir")
		}
		if tgChannel == "" {
			tgChannel = viper.GetString("tg_client.channel")
		}
		if tgLimit <= 0 {
			tgLimit = viper.GetInt("tg_client.limit")
		}
		if tgPageSize <= 0 {
			tgPageSize = viper.GetInt("tg_client.page_size")
		}

		cfg := tgclient.Config{
			AppID:       tgAppID,
			AppHash:     tgAppHash,
			Phone:       tgPhone,
			SessionFile: tgSessionFile,
			OutputDir:   tgOutputDir,
			Channel:     tgChannel,
			Limit:       tgLimit,
			PageSize:    tgPageSize,
		}
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Minute)
		defer cancel()

		stats, err := tgclient.SyncChannel(ctx, cfg, logger)
		if err != nil {
			logger.Fatalf("tgsync failed: %v", err)
			return
		}
		logger.Infof("tgsync success: total=%d kept=%d filtered=%d media=%d", stats.TotalMessages, stats.KeptMessages, stats.FilteredMessages, stats.DownloadedMedia)
	},
}

func init() {
	tgsyncCmd.Flags().IntVar(&tgAppID, "app-id", viper.GetInt("tg_client.app_id"), "Telegram app_id (config: tg_client.app_id)")
	tgsyncCmd.Flags().StringVar(&tgAppHash, "app-hash", viper.GetString("tg_client.app_hash"), "Telegram app_hash (config: tg_client.app_hash)")
	tgsyncCmd.Flags().StringVar(&tgPhone, "phone", viper.GetString("tg_client.phone"), "Telegram phone number (config: tg_client.phone)")
	tgsyncCmd.Flags().StringVar(&tgSessionFile, "session-file", viper.GetString("tg_client.session_file"), "Session file path (config: tg_client.session_file)")
	tgsyncCmd.Flags().StringVar(&tgOutputDir, "output-dir", viper.GetString("tg_client.output_dir"), "Output directory (config: tg_client.output_dir)")
	tgsyncCmd.Flags().StringVar(&tgChannel, "channel", viper.GetString("tg_client.channel"), "Telegram channel username (config: tg_client.channel)")
	tgsyncCmd.Flags().IntVar(&tgLimit, "limit", viper.GetInt("tg_client.limit"), "Optional max kept messages (0 means no limit)")
	tgsyncCmd.Flags().IntVar(&tgPageSize, "page-size", viper.GetInt("tg_client.page_size"), "messages.getHistory page size (1-100, config: tg_client.page_size)")

	rootCmd.AddCommand(tgsyncCmd)
}
