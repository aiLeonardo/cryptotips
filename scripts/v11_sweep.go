package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/aiLeonardo/cryptotips/lib"
	spot "github.com/aiLeonardo/cryptotips/strategies/spot_btc_conservative_v1"
)

type row struct {
	Profile      string  `json:"profile"`
	BullMin      float64 `json:"bull_min"`
	BullMax      float64 `json:"bull_max"`
	RangeAlloc   float64 `json:"range_alloc"`
	RangeBoost   float64 `json:"range_boost"`
	BearFloor    float64 `json:"bear_floor"`
	BearMom      float64 `json:"bear_mom"`
	BearLev      float64 `json:"bear_lev"`
	CooldownBars int     `json:"cooldown_bars"`
	TradeCount   int     `json:"trade_count"`
	WinRate      float64 `json:"win_rate"`
	PF           float64 `json:"pf"`
	MaxDD        float64 `json:"max_dd"`
	NetPnL       float64 `json:"net_pnl"`
	Excess       float64 `json:"excess"`
	Score        float64 `json:"score"`
	ReportJSON   string  `json:"report_json"`
}

func main() {
	logrusAdapter := lib.NewLogrusAdapter()
	db := lib.LoadDB(logrusAdapter)
	start := time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 2, 28, 23, 59, 59, 0, time.UTC)

	rows := make([]row, 0, 40)

	for _, prof := range []string{"v11_a", "v11_b"} {
		base := spot.V11AConfig()
		if prof == "v11_b" {
			base = spot.V11BConfig()
		}
		for _, bullMax := range []float64{base.BullAllocMax - 0.05, base.BullAllocMax, base.BullAllocMax + 0.03, base.BullAllocMax + 0.06} {
			for _, bearFloor := range []float64{base.BearSignalFloor - 0.04, base.BearSignalFloor, base.BearSignalFloor + 0.03, base.BearSignalFloor + 0.06} {
				cfg := base
				cfg.BullAllocMax = bullMax
				if cfg.BullAllocMax < cfg.BullAllocMin {
					cfg.BullAllocMax = cfg.BullAllocMin
				}
				cfg.BearSignalFloor = bearFloor
				cfg.RangeBaseAlloc = base.RangeBaseAlloc
				if bullMax > base.BullAllocMax {
					cfg.RangeBaseAlloc *= 0.9
				}
				if bearFloor > base.BearSignalFloor {
					cfg.BearMomentumFloor = base.BearMomentumFloor - 0.004
				}
				report, err := spot.RunReplayWithConfigAndRange(db, 0, cfg, prof, start, end)
				if err != nil {
					fmt.Printf("skip %s bullMax=%.2f bearFloor=%.2f err=%v\n", prof, bullMax, bearFloor, err)
					continue
				}
				ddPenalty := 0.0
				if report.MaxDrawdown > report.Benchmark.MaxDrawdown*1.15 {
					ddPenalty = (report.MaxDrawdown - report.Benchmark.MaxDrawdown*1.15) * 0.20
				}
				score := report.ExcessReturn - ddPenalty
				rows = append(rows, row{
					Profile:      prof,
					BullMin:      cfg.BullAllocMin,
					BullMax:      cfg.BullAllocMax,
					RangeAlloc:   cfg.RangeBaseAlloc,
					RangeBoost:   cfg.RangeTrendAllocBoost,
					BearFloor:    cfg.BearSignalFloor,
					BearMom:      cfg.BearMomentumFloor,
					BearLev:      cfg.BearLeverage,
					CooldownBars: cfg.CooldownBars,
					TradeCount:   report.TradeCount,
					WinRate:      report.WinRate,
					PF:           report.ProfitFactor,
					MaxDD:        report.MaxDrawdown,
					NetPnL:       report.NetPnL,
					Excess:       report.ExcessReturn,
					Score:        score,
					ReportJSON:   report.ReportPathJSON,
				})
			}
		}
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].Score > rows[j].Score })
	ts := time.Now().UTC().Format("20060102_150405")
	outDir := "strategies/reports"
	_ = os.MkdirAll(outDir, 0o775)
	jsonPath := filepath.Join(outDir, fmt.Sprintf("v11_sprint_search_%s.json", ts))
	mdPath := filepath.Join(outDir, fmt.Sprintf("v11_sprint_search_%s.md", ts))
	buf, _ := json.MarshalIndent(rows, "", "  ")
	_ = os.WriteFile(jsonPath, buf, 0o664)

	f, _ := os.Create(mdPath)
	defer f.Close()
	fmt.Fprintf(f, "# v11 Sprint Search\n\nRuns: %d\n\n", len(rows))
	fmt.Fprintf(f, "| Rank | Profile | Trades | WinRate | PF | MaxDD | NetPnL | Excess | Score | Bull[Min,Max] | RangeAlloc | BearFloor | Report |\n")
	fmt.Fprintf(f, "|---|---|---:|---:|---:|---:|---:|---:|---:|---|---:|---:|---|\n")
	for i, r := range rows {
		if i >= 15 {
			break
		}
		fmt.Fprintf(f, "| %d | %s | %d | %.2f%% | %.3f | %.2f | %.2f | %.2f | %.2f | [%.2f, %.2f] | %.2f | %.2f | %s |\n",
			i+1, r.Profile, r.TradeCount, r.WinRate*100, r.PF, r.MaxDD, r.NetPnL, r.Excess, r.Score, r.BullMin, r.BullMax, r.RangeAlloc, r.BearFloor, r.ReportJSON)
	}
	fmt.Printf("search done: runs=%d\njson=%s\nmd=%s\n", len(rows), jsonPath, mdPath)
}
