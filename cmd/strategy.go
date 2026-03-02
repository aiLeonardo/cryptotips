package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/aiLeonardo/cryptotips/lib"
	"github.com/aiLeonardo/cryptotips/strategies/engine"
	_ "github.com/aiLeonardo/cryptotips/strategies/spot_btc_conservative_v1"
	spotv1 "github.com/aiLeonardo/cryptotips/strategies/spot_btc_conservative_v1"
	"github.com/spf13/cobra"
)

var (
	strategyID              string
	strategyMode            string
	strategyDaemon          bool
	strategyInterval        int
	strategyReplayDays      int
	strategyReplayProfile   string
	strategyReplayStartDate string
	strategyReplayEndDate   string
	v11BullMin              float64
	v11BullMax              float64
	v11BullFloor            float64
	v11RangeAlloc           float64
	v11RangeBoost           float64
	v11BearFloor            float64
	v11BearMomentum         float64
	v11BearLev              float64
	v11Cooldown             int
	v11RegimeConfirm        int
	v11RegimeStay           int
	v11RegimeHyst           float64
	v11BullAddOnFloor       float64
	replayCostSidePct       float64
	replayShortFundingAPR   float64
)

var strategyCmd = &cobra.Command{
	Use:   "strategy",
	Short: "策略运行与状态查看",
}

var strategyRunCmd = &cobra.Command{
	Use:   "run",
	Short: "运行指定策略",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strategyDaemon {
			daemonCtx := lib.DaemonStart("strategy_" + strategyID)
			defer daemonCtx.Release()
		}
		lib.SetLogFilePath("./logs/strategy." + strategyID + ".log")
		logger := lib.LoadLogger()
		defer lib.RecoverLogMsg(logger)

		logrusAdapter := lib.NewLogrusAdapter()
		redisLogger := lib.NewRedisLogger()
		db := lib.LoadDB(logrusAdapter)
		rdb := lib.LoadRedis(redisLogger)

		s, err := engine.MustGet(strategyID)
		if err != nil {
			return err
		}
		runner := engine.NewRunner(s, engine.RuntimeDeps{DB: db, Redis: rdb, Logger: logger})

		if !strategyDaemon {
			return runner.RunOnce(context.Background(), strategyID, strategyMode)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() {
			mainEndSign := make(chan struct{})
			lib.WaitSIGTERM(mainEndSign)
			cancel()
		}()
		return runner.RunLoop(ctx, strategyID, strategyMode, time.Duration(strategyInterval)*time.Minute)
	},
}

var strategyStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "查看策略状态",
	RunE: func(cmd *cobra.Command, args []string) error {
		logger := lib.LoadLogger()
		logrusAdapter := lib.NewLogrusAdapter()
		redisLogger := lib.NewRedisLogger()
		db := lib.LoadDB(logrusAdapter)
		rdb := lib.LoadRedis(redisLogger)

		s, err := engine.MustGet(strategyID)
		if err != nil {
			return err
		}
		runner := engine.NewRunner(s, engine.RuntimeDeps{DB: db, Redis: rdb, Logger: logger})
		status, err := runner.Status(context.Background(), strategyID, strategyMode)
		if err != nil {
			return err
		}
		fmt.Printf("strategy status: %+v\n", status)
		return nil
	},
}

var strategyReplayCmd = &cobra.Command{
	Use:   "replay",
	Short: "4h paper 回放并生成报告",
	RunE: func(cmd *cobra.Command, args []string) error {
		logrusAdapter := lib.NewLogrusAdapter()
		db := lib.LoadDB(logrusAdapter)
		allowed := map[string]bool{"optimized": true, "baseline": true, "v3": true, "v4": true, "v5_trend_breakout": true, "v5_mean_reversion": true, "v5_momentum_factor": true, "v6": true, "v7": true, "v7_1": true, "v8": true, "v9": true, "v10": true, "v11": true, "v11_a": true, "v11_b": true, "v12": true, "v12_a": true, "v12_b": true, "v13": true, "v13_a": true, "v13_b": true}
		if !allowed[strategyReplayProfile] {
			return fmt.Errorf("invalid --profile: %s (allowed: optimized|baseline|v3|v4|v5_trend_breakout|v5_mean_reversion|v5_momentum_factor|v6|v7|v7_1|v8|v9|v10|v11|v11_a|v11_b|v12|v12_a|v12_b|v13|v13_a|v13_b)", strategyReplayProfile)
		}
		profile := strategyReplayProfile
		cfg := spotv1.LegacyOptimizedConfig()
		switch profile {
		case "baseline":
			cfg = spotv1.BaselineConfigFrom(cfg)
		case "v3":
			cfg = spotv1.V3Config()
		case "v4":
			cfg = spotv1.V4Config()
		case "v5_trend_breakout":
			cfg = spotv1.V5TrendBreakoutConfig()
		case "v5_mean_reversion":
			cfg = spotv1.V5MeanReversionConfig()
		case "v5_momentum_factor":
			cfg = spotv1.V5MomentumFactorConfig()
		case "v6":
			cfg = spotv1.V6RegimeConfig()
		case "v7", "v7_1":
			cfg = spotv1.V7RegimeConfig()
		case "v8":
			cfg = spotv1.V8Config()
		case "v9":
			cfg = spotv1.V9Config()
		case "v10":
			cfg = spotv1.V10Config()
		case "v11":
			cfg = spotv1.V11Config()
		case "v11_a":
			cfg = spotv1.V11AConfig()
		case "v11_b":
			cfg = spotv1.V11BConfig()
		case "v12":
			cfg = spotv1.V12Config()
		case "v12_a":
			cfg = spotv1.V12AConfig()
		case "v12_b":
			cfg = spotv1.V12BConfig()
		case "v13":
			cfg = spotv1.V13Config()
		case "v13_a":
			cfg = spotv1.V13AConfig()
		case "v13_b":
			cfg = spotv1.V13BConfig()
		}
		if profile == "v11" || profile == "v11_a" || profile == "v11_b" || profile == "v12" || profile == "v12_a" || profile == "v12_b" || profile == "v13" || profile == "v13_a" || profile == "v13_b" {
			if v11BullMin > 0 {
				cfg.BullAllocMin = v11BullMin
			}
			if v11BullMax > 0 {
				cfg.BullAllocMax = v11BullMax
			}
			if v11BullFloor > 0 {
				cfg.BullSignalFloor = v11BullFloor
			}
			if v11RangeAlloc > 0 {
				cfg.RangeBaseAlloc = v11RangeAlloc
			}
			if v11RangeBoost >= 0 {
				cfg.RangeTrendAllocBoost = v11RangeBoost
			}
			if v11BearFloor > 0 {
				cfg.BearSignalFloor = v11BearFloor
			}
			if v11BearMomentum < 0 {
				cfg.BearMomentumFloor = v11BearMomentum
			}
			if v11BearLev > 0 {
				cfg.BearLeverage = v11BearLev
				cfg.BearLeverageCap = v11BearLev
			}
			if v11Cooldown >= 0 {
				cfg.CooldownBars = v11Cooldown
			}
			if v11RegimeConfirm > 0 {
				cfg.RegimeMinConfirmBars = v11RegimeConfirm
			}
			if v11RegimeStay > 0 {
				cfg.RegimeMinStayBars = v11RegimeStay
			}
			if v11RegimeHyst >= 0 {
				cfg.RegimeHysteresis = v11RegimeHyst
			}
			if v11BullAddOnFloor > 0 {
				cfg.MinSignalStrength = v11BullAddOnFloor
			}
		}
		if replayCostSidePct >= 0 {
			cfg.CostPerSidePct = replayCostSidePct
		}
		if replayShortFundingAPR >= 0 {
			cfg.ShortFundingAPR = replayShortFundingAPR
		}

		startAt, endAt, err := parseReplayRange(strategyReplayStartDate, strategyReplayEndDate)
		if err != nil {
			return err
		}
		report, err := spotv1.RunReplayWithConfigAndRange(db, strategyReplayDays, cfg, profile, startAt, endAt)
		if err != nil {
			return err
		}
		fmt.Printf("replay done: trades=%d win_rate=%.2f%% pf=%.4f max_dd=%.4f net=%.4f\n", report.TradeCount, report.WinRate*100, report.ProfitFactor, report.MaxDrawdown, report.NetPnL)
		fmt.Printf("reports: %s , %s\n", report.ReportPathJSON, report.ReportPathMD)
		return nil
	},
}

func parseReplayRange(startDate, endDate string) (time.Time, time.Time, error) {
	if startDate == "" && endDate == "" {
		return time.Time{}, time.Time{}, nil
	}
	if startDate == "" || endDate == "" {
		return time.Time{}, time.Time{}, fmt.Errorf("--start and --end must be provided together")
	}
	startAt, err := time.ParseInLocation("2006-01-02", startDate, time.UTC)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid --start format, expect YYYY-MM-DD: %w", err)
	}
	endDay, err := time.ParseInLocation("2006-01-02", endDate, time.UTC)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid --end format, expect YYYY-MM-DD: %w", err)
	}
	endAt := endDay.Add(24*time.Hour - time.Nanosecond)
	if endAt.Before(startAt) {
		return time.Time{}, time.Time{}, fmt.Errorf("--end must be >= --start")
	}
	return startAt, endAt, nil
}

func init() {
	strategyRunCmd.Flags().StringVar(&strategyID, "id", "spot_btc_conservative_v1", "strategy id")
	strategyRunCmd.Flags().StringVar(&strategyMode, "mode", "paper", "strategy mode")
	strategyRunCmd.Flags().BoolVar(&strategyDaemon, "daemon", true, "run in daemon mode")
	strategyRunCmd.Flags().IntVar(&strategyInterval, "interval-minutes", 240, "daemon loop interval in minutes")

	strategyStatusCmd.Flags().StringVar(&strategyID, "id", "spot_btc_conservative_v1", "strategy id")
	strategyStatusCmd.Flags().StringVar(&strategyMode, "mode", "paper", "strategy mode")

	strategyReplayCmd.Flags().IntVar(&strategyReplayDays, "days", 360, "replay recent days (used when --start/--end not provided)")
	strategyReplayCmd.Flags().StringVar(&strategyReplayProfile, "profile", "optimized", "replay profile: optimized|baseline|v3|v4|v5_trend_breakout|v5_mean_reversion|v5_momentum_factor|v6|v7|v7_1|v8|v9|v10|v11|v11_a|v11_b|v12|v12_a|v12_b|v13|v13_a|v13_b")
	strategyReplayCmd.Flags().StringVar(&strategyReplayStartDate, "start", "", "replay start date (YYYY-MM-DD)")
	strategyReplayCmd.Flags().StringVar(&strategyReplayEndDate, "end", "", "replay end date (YYYY-MM-DD)")
	strategyReplayCmd.Flags().Float64Var(&v11BullMin, "v11-bull-min", -1, "v11 override: bull min allocation")
	strategyReplayCmd.Flags().Float64Var(&v11BullMax, "v11-bull-max", -1, "v11 override: bull max allocation")
	strategyReplayCmd.Flags().Float64Var(&v11BullFloor, "v11-bull-floor", -1, "v11 override: bull signal floor")
	strategyReplayCmd.Flags().Float64Var(&v11RangeAlloc, "v11-range-alloc", -1, "v11 override: range base allocation")
	strategyReplayCmd.Flags().Float64Var(&v11RangeBoost, "v11-range-boost", -1, "v11 override: range trend boost allocation")
	strategyReplayCmd.Flags().Float64Var(&v11BearFloor, "v11-bear-floor", -1, "v11 override: bear signal floor")
	strategyReplayCmd.Flags().Float64Var(&v11BearMomentum, "v11-bear-mom", 0, "v11 override: bear momentum floor (negative)")
	strategyReplayCmd.Flags().Float64Var(&v11BearLev, "v11-bear-lev", -1, "v11 override: bear leverage")
	strategyReplayCmd.Flags().IntVar(&v11Cooldown, "v11-cooldown", -1, "v11/v12/v13 override: cooldown bars")
	strategyReplayCmd.Flags().IntVar(&v11RegimeConfirm, "v11-regime-confirm", -1, "v11/v12/v13 override: regime min confirm bars")
	strategyReplayCmd.Flags().IntVar(&v11RegimeStay, "v11-regime-stay", -1, "v11/v12/v13 override: regime min stay bars")
	strategyReplayCmd.Flags().Float64Var(&v11RegimeHyst, "v11-regime-hyst", -1, "v11/v12/v13 override: regime hysteresis")
	strategyReplayCmd.Flags().Float64Var(&v11BullAddOnFloor, "v11-bull-addon-floor", -1, "v13 override: bull add-on signal floor (maps to min signal strength)")
	strategyReplayCmd.Flags().Float64Var(&replayCostSidePct, "cost-side-pct", -1, "replay trading cost per side (e.g. 0.001 for 0.10%)")
	strategyReplayCmd.Flags().Float64Var(&replayShortFundingAPR, "short-funding-apr", -1, "replay short funding annualized rate (e.g. 0.05 for 5%)")

	strategyCmd.AddCommand(strategyRunCmd)
	strategyCmd.AddCommand(strategyStatusCmd)
	strategyCmd.AddCommand(strategyReplayCmd)
	rootCmd.AddCommand(strategyCmd)
}
