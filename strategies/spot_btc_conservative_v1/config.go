package spot_btc_conservative_v1

type Config struct {
	StrategyID                 string
	StrategyStyle              string
	Symbol                     string
	RiskCapitalUSDT            float64
	RiskPerTrade               float64
	MaxDailyLossPct            float64
	MaxWeeklyLossPct           float64
	EMAFilterPeriod            int
	EMAPullback                int
	PullbackTol                float64
	EMA200SlopeLookback        int
	EMA200MinSlope             float64
	VolumeSMAPeriod            int
	VolumeMinRatio             float64
	ATRPeriod                  int
	ATRMinPct                  float64
	ATRMaxPct                  float64
	TrailATRMultiplier         float64
	SecondaryBreakoutLookback  int
	SecondaryBreakoutBufferPct float64
	MaxHoldBars                int
	CoreAllocationPct          float64

	// v6 regime routing
	RegimeVolPeriod          int
	RegimeVolConvergePct     float64
	RegimeMinConfirmBars     int
	BearLeverage             float64
	SwitchTP1Pct             float64
	SwitchTP2Pct             float64
	SwitchTP3Pct             float64
	SwitchTrailATRMultiplier float64

	// v7 calibrated regime routing
	RegimeTimeframe string

	// v8 frequency/quality controls
	CooldownBars      int
	MaxEntriesPerWeek int
	MinSignalStrength float64
	RegimeMinStayBars int
	RegimeHysteresis  float64

	// v9 range-active controls
	RangeLookbackDays      int
	RangeEntryATRBuffer    float64
	RangeBreakoutATRBuffer float64
	RangeMidExitRatio      float64
	RangeMaxHoldBars       int
	RangeMinWidthATR       float64
	RangeMinSignalStrength float64

	// v11 sprint controls (startpoint-driven regime)
	BullAllocMin          float64
	BullAllocMax          float64
	BullSignalFloor       float64
	RangeBaseAlloc        float64
	RangeTrendAllocBoost  float64
	RangeExitEMABufferATR float64
	RangeCostGuardATR     float64
	BearSignalFloor       float64
	BearMomentumFloor     float64
	BearLeverageCap       float64
	BearRequireQuality    bool

	// replay transaction cost model
	CostPerSidePct  float64
	ShortFundingAPR float64
}

func DefaultConfig() Config { return V3Config() }

func LegacyOptimizedConfig() Config {
	return Config{StrategyID: "spot_btc_conservative_v1", StrategyStyle: "trend_pullback", Symbol: "BTCUSDT", RiskCapitalUSDT: 10000, RiskPerTrade: 0.008, MaxDailyLossPct: 0.015, MaxWeeklyLossPct: 0.04, EMAFilterPeriod: 200, EMAPullback: 20, PullbackTol: 0.003, EMA200SlopeLookback: 5, EMA200MinSlope: 0.001, VolumeSMAPeriod: 20, VolumeMinRatio: 1.05, ATRPeriod: 14, ATRMinPct: 0.008, ATRMaxPct: 0.05, TrailATRMultiplier: 1.2, SecondaryBreakoutLookback: 0, SecondaryBreakoutBufferPct: 0, MaxHoldBars: 0, CoreAllocationPct: 0, RegimeVolPeriod: 20, RegimeVolConvergePct: 0.02, RegimeMinConfirmBars: 2, BearLeverage: 1.0, SwitchTP1Pct: 0.5, SwitchTP2Pct: 0.3, SwitchTP3Pct: 0.2, SwitchTrailATRMultiplier: 1.0}
}

func V3Config() Config {
	return Config{StrategyID: "spot_btc_conservative_v1", StrategyStyle: "trend_pullback", Symbol: "BTCUSDT", RiskCapitalUSDT: 10000, RiskPerTrade: 0.008, MaxDailyLossPct: 0.015, MaxWeeklyLossPct: 0.04, EMAFilterPeriod: 200, EMAPullback: 20, PullbackTol: 0.0045, EMA200SlopeLookback: 5, EMA200MinSlope: 0.0006, VolumeSMAPeriod: 20, VolumeMinRatio: 0.95, ATRPeriod: 14, ATRMinPct: 0.006, ATRMaxPct: 0.05, TrailATRMultiplier: 1.2, SecondaryBreakoutLookback: 6, SecondaryBreakoutBufferPct: 0.0008, MaxHoldBars: 24, CoreAllocationPct: 0, RegimeVolPeriod: 20, RegimeVolConvergePct: 0.02, RegimeMinConfirmBars: 2, BearLeverage: 1.0, SwitchTP1Pct: 0.5, SwitchTP2Pct: 0.3, SwitchTP3Pct: 0.2, SwitchTrailATRMultiplier: 1.0}
}

func V4Config() Config {
	return Config{StrategyID: "spot_btc_conservative_v1", StrategyStyle: "trend_pullback", Symbol: "BTCUSDT", RiskCapitalUSDT: 10000, RiskPerTrade: 0.008, MaxDailyLossPct: 0.015, MaxWeeklyLossPct: 0.04, EMAFilterPeriod: 200, EMAPullback: 20, PullbackTol: 0.004, EMA200SlopeLookback: 5, EMA200MinSlope: 0.0005, VolumeSMAPeriod: 20, VolumeMinRatio: 0.9, ATRPeriod: 14, ATRMinPct: 0.005, ATRMaxPct: 0.05, TrailATRMultiplier: 1.1, SecondaryBreakoutLookback: 8, SecondaryBreakoutBufferPct: 0.0005, MaxHoldBars: 30, CoreAllocationPct: 0.6, RegimeVolPeriod: 20, RegimeVolConvergePct: 0.02, RegimeMinConfirmBars: 2, BearLeverage: 1.0, SwitchTP1Pct: 0.5, SwitchTP2Pct: 0.3, SwitchTP3Pct: 0.2, SwitchTrailATRMultiplier: 1.0}
}

func V5TrendBreakoutConfig() Config {
	c := V4Config()
	c.StrategyID = "btc_trend_breakout_v5"
	c.StrategyStyle = "trend_breakout"
	c.CoreAllocationPct = 0
	c.SecondaryBreakoutLookback = 12
	c.SecondaryBreakoutBufferPct = 0.0015
	c.VolumeMinRatio = 1.0
	c.ATRMinPct = 0.006
	c.MaxHoldBars = 24
	return c
}

func V5MeanReversionConfig() Config {
	c := V3Config()
	c.StrategyID = "btc_mean_reversion_v5"
	c.StrategyStyle = "mean_reversion"
	c.EMAFilterPeriod = 120
	c.EMAPullback = 20
	c.TrailATRMultiplier = 1.0
	c.MaxHoldBars = 18
	c.ATRMinPct = 0.004
	c.ATRMaxPct = 0.04
	c.VolumeMinRatio = 0.85
	return c
}

func V5MomentumFactorConfig() Config {
	c := V4Config()
	c.StrategyID = "btc_momentum_factor_v5"
	c.StrategyStyle = "momentum_factor"
	c.CoreAllocationPct = 0
	c.EMAFilterPeriod = 100
	c.EMAPullback = 21
	c.EMA200MinSlope = 0.0003
	c.VolumeMinRatio = 0.9
	c.SecondaryBreakoutLookback = 10
	c.SecondaryBreakoutBufferPct = 0.0008
	c.MaxHoldBars = 26
	return c
}

func V6RegimeConfig() Config {
	c := V5MeanReversionConfig()
	c.StrategyID = "btc_regime_router_v6"
	c.StrategyStyle = "regime_router_v6"
	c.EMAFilterPeriod = 200
	c.EMA200SlopeLookback = 7
	c.EMA200MinSlope = 0.0004
	c.RegimeVolPeriod = 20
	c.RegimeVolConvergePct = 0.018
	c.RegimeMinConfirmBars = 2
	c.BearLeverage = 1.0
	c.SwitchTP1Pct = 0.5
	c.SwitchTP2Pct = 0.3
	c.SwitchTP3Pct = 0.2
	c.SwitchTrailATRMultiplier = 1.0
	c.MaxHoldBars = 28
	c.RegimeTimeframe = "1d"
	return c
}

func V7RegimeConfig() Config {
	c := V6RegimeConfig()
	c.StrategyID = "btc_regime_router_v7_calibrated"
	c.StrategyStyle = "regime_router_v7"
	c.RegimeTimeframe = "auto"
	return c
}

func V8Config() Config {
	c := V7RegimeConfig()
	c.StrategyID = "btc_regime_router_v8_weekly"
	c.StrategyStyle = "regime_router_v8"
	c.RegimeTimeframe = "1w"
	c.RegimeMinConfirmBars = 2
	c.RegimeMinStayBars = 3
	c.RegimeHysteresis = 0.12
	c.CooldownBars = 4
	c.MaxEntriesPerWeek = 2
	c.MinSignalStrength = 0.58
	c.BearLeverage = 1.0
	c.SwitchTP1Pct = 0.25
	c.SwitchTP2Pct = 0.0
	c.SwitchTP3Pct = 0.75
	c.TrailATRMultiplier = 1.35
	c.SwitchTrailATRMultiplier = 1.1
	c.MaxHoldBars = 0
	c.CostPerSidePct = 0.001
	c.ShortFundingAPR = 0.05
	return c
}

func V9Config() Config {
	c := V8Config()
	c.StrategyID = "btc_regime_router_v9_range_active"
	c.StrategyStyle = "regime_router_v9"
	c.CooldownBars = 6
	c.MaxEntriesPerWeek = 3
	c.MinSignalStrength = 0.60
	c.RangeLookbackDays = 20
	c.RangeEntryATRBuffer = 0.60
	c.RangeBreakoutATRBuffer = 0.55
	c.RangeMidExitRatio = 0.60
	c.RangeMaxHoldBars = 18
	c.RangeMinWidthATR = 2.4
	c.RangeMinSignalStrength = 0.62
	return c
}

func V10Config() Config {
	c := V9Config()
	c.StrategyID = "btc_regime_router_v10_startpoint_only"
	c.StrategyStyle = "regime_router_v10_startpoint_only"
	c.BearLeverage = 2.0
	return c
}

func V11AConfig() Config {
	c := V10Config()
	c.StrategyID = "btc_regime_router_v11_sprint"
	c.StrategyStyle = "regime_router_v11"
	c.BullAllocMin = 0.70
	c.BullAllocMax = 0.95
	c.BullSignalFloor = 0.50
	c.RangeBaseAlloc = 0.22
	c.RangeTrendAllocBoost = 0.10
	c.RangeExitEMABufferATR = 0.55
	c.RangeCostGuardATR = 1.0
	c.BearSignalFloor = 0.78
	c.BearMomentumFloor = -0.028
	c.BearLeverage = 1.8
	c.BearLeverageCap = 1.8
	c.BearRequireQuality = true
	c.CooldownBars = 2
	c.MaxEntriesPerWeek = 4
	c.RegimeMinStayBars = 1
	c.RegimeHysteresis = 0
	return c
}

func V11BConfig() Config {
	c := V11AConfig()
	c.BullAllocMax = 0.85
	c.BullSignalFloor = 0.58
	c.RangeBaseAlloc = 0.16
	c.RangeTrendAllocBoost = 0.06
	c.RangeExitEMABufferATR = 0.45
	c.BearSignalFloor = 0.84
	c.BearMomentumFloor = -0.035
	c.BearLeverage = 1.2
	c.BearLeverageCap = 1.2
	c.CooldownBars = 4
	c.MaxEntriesPerWeek = 3
	return c
}

func V11Config() Config {
	c := V11AConfig()
	c.BearSignalFloor = 0.74
	return c
}

func V12AConfig() Config {
	c := V11BConfig()
	c.StrategyID = "btc_regime_router_v12_cost_robust"
	c.StrategyStyle = "regime_router_v12"
	c.CooldownBars = 10
	c.MaxEntriesPerWeek = 2
	c.RegimeMinConfirmBars = 2
	c.RegimeMinStayBars = 3
	c.RegimeHysteresis = 0.56
	c.BullAllocMin = 0.60
	c.BullAllocMax = 0.80
	c.BullSignalFloor = 0.62
	c.RangeBaseAlloc = 0.08
	c.RangeTrendAllocBoost = 0.02
	c.RangeExitEMABufferATR = 0.48
	c.RangeCostGuardATR = 1.35
	c.BearSignalFloor = 0.90
	c.BearMomentumFloor = -0.045
	c.BearLeverage = 1.0
	c.BearLeverageCap = 1.0
	c.BearRequireQuality = true
	return c
}

func V12BConfig() Config {
	c := V12AConfig()
	c.BullAllocMax = 0.76
	c.RangeBaseAlloc = 0.06
	c.RangeTrendAllocBoost = 0.01
	c.BearSignalFloor = 0.92
	c.CooldownBars = 12
	c.MaxEntriesPerWeek = 1
	c.RegimeMinConfirmBars = 3
	c.RegimeHysteresis = 0.60
	c.RangeCostGuardATR = 1.55
	return c
}

func V12Config() Config {
	c := V12AConfig()
	c.BullAllocMax = 0.79
	c.RangeBaseAlloc = 0.07
	c.BearSignalFloor = 0.91
	c.CooldownBars = 11
	c.RegimeMinConfirmBars = 2
	c.RegimeMinStayBars = 3
	c.RegimeHysteresis = 0.58
	c.RangeCostGuardATR = 1.45
	return c
}

func V13AConfig() Config {
	c := V12Config()
	c.StrategyID = "btc_regime_router_v13_bull_core"
	c.StrategyStyle = "regime_router_v13"
	c.BullAllocMin = 0.68
	c.BullAllocMax = 0.90
	c.BullSignalFloor = 0.54
	c.MinSignalStrength = 0.74 // v13: bull add-on threshold (high confidence)
	c.RangeBaseAlloc = 0.10    // v13: min range participation
	c.RangeTrendAllocBoost = 0.03
	c.RangeCostGuardATR = 1.60
	c.RegimeMinConfirmBars = 3
	c.RegimeMinStayBars = 4
	c.RegimeHysteresis = 0.62
	c.CooldownBars = 12
	c.MaxEntriesPerWeek = 2
	c.BearSignalFloor = 0.92
	c.BearMomentumFloor = -0.045
	return c
}

func V13BConfig() Config {
	c := V13AConfig()
	c.BullAllocMin = 0.72
	c.BullAllocMax = 0.94
	c.BullSignalFloor = 0.56
	c.MinSignalStrength = 0.76
	c.RangeBaseAlloc = 0.08
	c.RangeTrendAllocBoost = 0.02
	c.RegimeMinConfirmBars = 3
	c.RegimeMinStayBars = 5
	c.RegimeHysteresis = 0.64
	c.CooldownBars = 13
	c.MaxEntriesPerWeek = 1
	c.BearSignalFloor = 0.93
	return c
}

func V13Config() Config {
	c := V13AConfig()
	c.BullAllocMin = 0.70
	c.BullAllocMax = 0.92
	c.BullSignalFloor = 0.55
	c.MinSignalStrength = 0.75
	c.RangeBaseAlloc = 0.09
	c.RangeTrendAllocBoost = 0.025
	c.RegimeMinConfirmBars = 3
	c.RegimeMinStayBars = 4
	c.RegimeHysteresis = 0.63
	c.CooldownBars = 12
	c.MaxEntriesPerWeek = 2
	c.BearSignalFloor = 0.92
	return c
}
