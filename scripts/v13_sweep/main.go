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

type seg struct {
	Name       string  `json:"name"`
	NetPnL     float64 `json:"net_pnl"`
	BNetPnL    float64 `json:"benchmark_net_pnl"`
	Excess     float64 `json:"excess"`
	TradeCount int     `json:"trade_count"`
	WinRate    float64 `json:"win_rate"`
	PF         float64 `json:"pf"`
	MaxDD      float64 `json:"max_dd"`
	ReportJSON string  `json:"report_json"`
}

type row struct {
	RankScore      float64 `json:"rank_score"`
	WinsABC        int     `json:"wins_abc"`
	WinsAll        int     `json:"wins_all_windows"`
	BullCore       float64 `json:"bull_core"`
	BullMax        float64 `json:"bull_max"`
	BullFloor      float64 `json:"bull_floor"`
	BullAddOnFloor float64 `json:"bull_addon_floor"`
	RangeBase      float64 `json:"range_base"`
	RangeBoost     float64 `json:"range_boost"`
	RegimeConfirm  int     `json:"regime_confirm"`
	RegimeStay     int     `json:"regime_stay"`
	RegimeHyst     float64 `json:"regime_hysteresis"`
	Cooldown       int     `json:"cooldown"`
	MaxWeekEntries int     `json:"max_week_entries"`
	Segments       []seg   `json:"segments"`
}

func main() {
	logrusAdapter := lib.NewLogrusAdapter()
	db := lib.LoadDB(logrusAdapter)

	windows := []struct {
		Name       string
		Start, End time.Time
	}{
		{"A_2018_2021", time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2021, 12, 31, 23, 59, 59, 0, time.UTC)},
		{"B_2022_2023", time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC)},
		{"C_2024_2026", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 2, 28, 23, 59, 59, 0, time.UTC)},
		{"ALL_2018_2026", time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 2, 28, 23, 59, 59, 0, time.UTC)},
	}

	base := spot.V13AConfig()
	base.CostPerSidePct = 0.0015
	base.ShortFundingAPR = 0.05

	rows := make([]row, 0, 30)
	for _, core := range []float64{0.66, 0.70, 0.74} {
		for _, addFloor := range []float64{0.72, 0.75, 0.78} {
			for _, rangeBase := range []float64{0.06, 0.09, 0.12} {
				cfg := base
				cfg.BullAllocMin = core
				cfg.BullAllocMax = core + 0.20
				if cfg.BullAllocMax > 0.95 {
					cfg.BullAllocMax = 0.95
				}
				cfg.BullSignalFloor = 0.54
				cfg.MinSignalStrength = addFloor
				cfg.RangeBaseAlloc = rangeBase
				cfg.RangeTrendAllocBoost = 0.03
				cfg.RegimeMinConfirmBars = 3
				cfg.RegimeMinStayBars = 4
				cfg.RegimeHysteresis = 0.60 + (core-0.66)*0.10
				cfg.CooldownBars = 12
				cfg.MaxEntriesPerWeek = 2

				r := row{
					BullCore:       cfg.BullAllocMin,
					BullMax:        cfg.BullAllocMax,
					BullFloor:      cfg.BullSignalFloor,
					BullAddOnFloor: cfg.MinSignalStrength,
					RangeBase:      cfg.RangeBaseAlloc,
					RangeBoost:     cfg.RangeTrendAllocBoost,
					RegimeConfirm:  cfg.RegimeMinConfirmBars,
					RegimeStay:     cfg.RegimeMinStayBars,
					RegimeHyst:     cfg.RegimeHysteresis,
					Cooldown:       cfg.CooldownBars,
					MaxWeekEntries: cfg.MaxEntriesPerWeek,
					Segments:       make([]seg, 0, len(windows)),
				}

				winsABC := 0
				winsAll := 0
				score := 0.0
				for _, w := range windows {
					report, err := spot.RunReplayWithConfigAndRange(db, 0, cfg, "v13", w.Start, w.End)
					if err != nil {
						fmt.Printf("skip core=%.2f add=%.2f range=%.2f err=%v\n", core, addFloor, rangeBase, err)
						continue
					}
					ex := report.NetPnL - report.Benchmark.NetPnL
					if ex > 0 {
						winsAll++
						if w.Name != "ALL_2018_2026" {
							winsABC++
						}
					}
					if w.Name == "ALL_2018_2026" {
						score += ex * 0.35
						if report.MaxDrawdown > report.Benchmark.MaxDrawdown*1.15 {
							score -= (report.MaxDrawdown - report.Benchmark.MaxDrawdown*1.15) * 0.10
						}
					} else {
						score += ex
					}
					r.Segments = append(r.Segments, seg{
						Name:       w.Name,
						NetPnL:     report.NetPnL,
						BNetPnL:    report.Benchmark.NetPnL,
						Excess:     ex,
						TradeCount: report.TradeCount,
						WinRate:    report.WinRate,
						PF:         report.ProfitFactor,
						MaxDD:      report.MaxDrawdown,
						ReportJSON: report.ReportPathJSON,
					})
				}
				r.WinsABC = winsABC
				r.WinsAll = winsAll
				r.RankScore = score + float64(winsABC)*5000
				rows = append(rows, r)
			}
		}
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].RankScore > rows[j].RankScore })
	ts := time.Now().UTC().Format("20060102_150405")
	outDir := "strategies/reports"
	_ = os.MkdirAll(outDir, 0o775)
	jsonPath := filepath.Join(outDir, fmt.Sprintf("v13_sweep_%s.json", ts))
	mdPath := filepath.Join(outDir, fmt.Sprintf("v13_sweep_%s.md", ts))
	buf, _ := json.MarshalIndent(rows, "", "  ")
	_ = os.WriteFile(jsonPath, buf, 0o664)

	f, _ := os.Create(mdPath)
	defer f.Close()
	fmt.Fprintf(f, "# v13 Cost-B Sweep\n\nRuns: %d\n\n", len(rows))
	fmt.Fprintf(f, "| Rank | Wins(A/B/C) | Score | BullCore | BullMax | AddOnFloor | RangeBase | Hyst | Trades(ALL) | Excess(ALL) | Report(ALL) |\n")
	fmt.Fprintf(f, "|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|\n")
	for i, r := range rows {
		if i >= 20 {
			break
		}
		allTrades := 0
		allExcess := 0.0
		allReport := ""
		for _, s := range r.Segments {
			if s.Name == "ALL_2018_2026" {
				allTrades = s.TradeCount
				allExcess = s.Excess
				allReport = s.ReportJSON
				break
			}
		}
		fmt.Fprintf(f, "| %d | %d/3 | %.2f | %.2f | %.2f | %.2f | %.2f | %.2f | %d | %.2f | %s |\n",
			i+1, r.WinsABC, r.RankScore, r.BullCore, r.BullMax, r.BullAddOnFloor, r.RangeBase, r.RegimeHyst, allTrades, allExcess, allReport)
	}
	fmt.Printf("sweep done: runs=%d\njson=%s\nmd=%s\n", len(rows), jsonPath, mdPath)
}
