package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aiLeonardo/cryptotips/lib"
	"github.com/aiLeonardo/cryptotips/models"
	"github.com/spf13/cobra"
)

type regimeStartpointsFile struct {
	Symbol      string `json:"symbol"`
	Startpoints []struct {
		Date       string            `json:"date"`
		State      string            `json:"state"`
		Confidence float64           `json:"confidence"`
		Basis      map[string]string `json:"basis"`
	} `json:"startpoints"`
}

var (
	regimeFilePath string
	regimeSource   string
)

var regimeCmd = &cobra.Command{
	Use:   "regime",
	Short: "regime 数据相关命令",
}

var regimeSyncCmd = &cobra.Command{
	Use:   "sync-startpoints",
	Short: "将识别出的牛熊震荡起点数据导入 MySQL（幂等 upsert）",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(regimeFilePath) == "" {
			return fmt.Errorf("--file 不能为空")
		}

		raw, err := os.ReadFile(regimeFilePath)
		if err != nil {
			return err
		}

		var payload regimeStartpointsFile
		if err := json.Unmarshal(raw, &payload); err != nil {
			return fmt.Errorf("解析 JSON 失败: %w", err)
		}
		if payload.Symbol == "" {
			return fmt.Errorf("JSON 中缺少 symbol")
		}

		source := strings.TrimSpace(regimeSource)
		if source == "" {
			source = filepath.Base(regimeFilePath)
		}

		recs := make([]models.RegimeStartpointRecord, 0, len(payload.Startpoints))
		for _, p := range payload.Startpoints {
			if p.Date == "" || p.State == "" {
				continue
			}
			t, err := time.ParseInLocation("2006-01-02", p.Date, time.UTC)
			if err != nil {
				return fmt.Errorf("解析日期失败 %s: %w", p.Date, err)
			}
			recs = append(recs, models.RegimeStartpointRecord{
				Symbol:     strings.ToUpper(payload.Symbol),
				StartTime:  t,
				State:      strings.ToUpper(p.State),
				Confidence: p.Confidence,
				BasisA:     p.Basis["methodA"],
				BasisB:     p.Basis["methodB"],
				Source:     source,
			})
		}

		db := lib.LoadDB(lib.NewLogrusAdapter())
		m := &models.RegimeStartpointRecord{}
		if err := m.BatchUpsert(db, recs); err != nil {
			return err
		}

		fmt.Printf("regime startpoints sync done: symbol=%s records=%d source=%s\n", strings.ToUpper(payload.Symbol), len(recs), source)
		return nil
	},
}

func init() {
	regimeSyncCmd.Flags().StringVar(&regimeFilePath, "file", "./strategies/reports/btc_regime_startpoints_20180101_20260228_20260227_145300.json", "startpoints json file path")
	regimeSyncCmd.Flags().StringVar(&regimeSource, "source", "", "source label, default file basename")

	regimeCmd.AddCommand(regimeSyncCmd)
	rootCmd.AddCommand(regimeCmd)
}
