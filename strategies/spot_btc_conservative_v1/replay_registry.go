package spot_btc_conservative_v1

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type ReplayRegistryEntry struct {
	GeneratedAtUTC    time.Time `json:"generated_at_utc"`
	StrategyID        string    `json:"strategy_id"`
	Profile           string    `json:"profile"`
	Symbol            string    `json:"symbol"`
	Interval          string    `json:"interval"`
	CapitalUSDT       float64   `json:"capital_usdt"`
	StartTimeUTC      time.Time `json:"start_time_utc"`
	EndTimeUTC        time.Time `json:"end_time_utc"`
	TradeCount        int       `json:"trade_count"`
	WinRate           float64   `json:"win_rate"`
	ProfitFactor      float64   `json:"profit_factor"`
	MaxDrawdown       float64   `json:"max_drawdown"`
	NetPnL            float64   `json:"net_pnl"`
	BenchmarkNetPnL   float64   `json:"benchmark_net_pnl"`
	ExcessReturn      float64   `json:"excess_return"`
	ReportPathJSON    string    `json:"report_path_json"`
	ReportPathMD      string    `json:"report_path_md"`
	WindowLabel       string    `json:"window_label,omitempty"`
	windowSortKeyHint string
}

func updateReplayRegistry(reportDir string) error {
	entries, err := collectReplayRegistryEntries(reportDir)
	if err != nil {
		return err
	}
	if err := writeReplayRegistryCSV(filepath.Join(reportDir, "replay_registry.csv"), entries); err != nil {
		return err
	}
	if err := writeReplayRegistryJSON(filepath.Join(reportDir, "replay_registry.json"), entries); err != nil {
		return err
	}
	if err := writeReplayComparisonMarkdown(filepath.Join(reportDir, "replay_comparison.md"), entries); err != nil {
		return err
	}
	return nil
}

func collectReplayRegistryEntries(reportDir string) ([]ReplayRegistryEntry, error) {
	files, err := filepath.Glob(filepath.Join(reportDir, "replay_*.json"))
	if err != nil {
		return nil, err
	}
	entries := make([]ReplayRegistryEntry, 0, len(files))
	seen := map[string]struct{}{}
	for _, fp := range files {
		entry, ok := parseReplayRegistryEntry(fp)
		if !ok {
			continue
		}
		key := entry.ReportPathJSON
		if key == "" {
			key = fp
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].StartTimeUTC.Equal(entries[j].StartTimeUTC) {
			if entries[i].EndTimeUTC.Equal(entries[j].EndTimeUTC) {
				if entries[i].GeneratedAtUTC.Equal(entries[j].GeneratedAtUTC) {
					return entries[i].Profile < entries[j].Profile
				}
				return entries[i].GeneratedAtUTC.Before(entries[j].GeneratedAtUTC)
			}
			return entries[i].EndTimeUTC.Before(entries[j].EndTimeUTC)
		}
		return entries[i].StartTimeUTC.Before(entries[j].StartTimeUTC)
	})
	return entries, nil
}

func parseReplayRegistryEntry(jsonPath string) (ReplayRegistryEntry, bool) {
	buf, err := os.ReadFile(jsonPath)
	if err != nil {
		return ReplayRegistryEntry{}, false
	}
	var report ReplayReport
	if err := json.Unmarshal(buf, &report); err != nil {
		return ReplayRegistryEntry{}, false
	}
	st, et := report.StartTimeUTC.UTC(), report.EndTimeUTC.UTC()
	if st.IsZero() || et.IsZero() {
		if fi, err := os.Stat(jsonPath); err == nil {
			end := fi.ModTime().UTC()
			if end.IsZero() {
				end = time.Now().UTC()
			}
			if report.Days > 0 {
				st = end.AddDate(0, 0, -report.Days)
			}
			if st.IsZero() {
				st = end
			}
			if et.IsZero() {
				et = end
			}
		}
	}
	generatedAt := report.GeneratedAtUTC.UTC()
	if generatedAt.IsZero() {
		if fi, err := os.Stat(jsonPath); err == nil {
			generatedAt = fi.ModTime().UTC()
		}
	}
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}
	benchmarkNet := report.Benchmark.NetPnL
	excess := report.ExcessReturn
	if excess == 0 && report.NetPnL != 0 {
		excess = report.NetPnL - benchmarkNet
	}
	reportJSON := report.ReportPathJSON
	if reportJSON == "" {
		reportJSON = jsonPath
	}
	reportMD := report.ReportPathMD
	if reportMD == "" {
		reportMD = strings.TrimSuffix(reportJSON, filepath.Ext(reportJSON)) + ".md"
	}
	entry := ReplayRegistryEntry{
		GeneratedAtUTC:  generatedAt,
		StrategyID:      report.StrategyID,
		Profile:         report.Profile,
		Symbol:          report.Symbol,
		Interval:        report.Interval,
		CapitalUSDT:     report.CapitalUSDT,
		StartTimeUTC:    st,
		EndTimeUTC:      et,
		TradeCount:      report.TradeCount,
		WinRate:         report.WinRate,
		ProfitFactor:    report.ProfitFactor,
		MaxDrawdown:     report.MaxDrawdown,
		NetPnL:          report.NetPnL,
		BenchmarkNetPnL: benchmarkNet,
		ExcessReturn:    excess,
		ReportPathJSON:  reportJSON,
		ReportPathMD:    reportMD,
	}
	entry.WindowLabel = buildWindowLabel(entry.StartTimeUTC, entry.EndTimeUTC)
	if entry.StrategyID == "" {
		entry.StrategyID = "spot_btc_conservative_v1"
	}
	if entry.Interval == "" {
		entry.Interval = "4h"
	}
	if entry.Symbol == "" {
		entry.Symbol = "BTCUSDT"
	}
	if entry.Profile == "" {
		entry.Profile = "unknown"
	}
	return entry, true
}

func writeReplayRegistryCSV(path string, entries []ReplayRegistryEntry) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	header := []string{
		"generated_at_utc", "strategy_id", "profile", "symbol", "interval", "capital_usdt",
		"start_time_utc", "end_time_utc", "trade_count", "win_rate", "profit_factor", "max_drawdown",
		"net_pnl", "benchmark_net_pnl", "excess_return", "report_path_json", "report_path_md",
	}
	if err := w.Write(header); err != nil {
		return err
	}
	for _, e := range entries {
		rec := []string{
			e.GeneratedAtUTC.Format(time.RFC3339), e.StrategyID, e.Profile, e.Symbol, e.Interval,
			fmt.Sprintf("%.4f", e.CapitalUSDT), e.StartTimeUTC.Format(time.RFC3339), e.EndTimeUTC.Format(time.RFC3339),
			fmt.Sprintf("%d", e.TradeCount), fmt.Sprintf("%.8f", e.WinRate), fmt.Sprintf("%.8f", e.ProfitFactor), fmt.Sprintf("%.8f", e.MaxDrawdown),
			fmt.Sprintf("%.8f", e.NetPnL), fmt.Sprintf("%.8f", e.BenchmarkNetPnL), fmt.Sprintf("%.8f", e.ExcessReturn),
			e.ReportPathJSON, e.ReportPathMD,
		}
		if err := w.Write(rec); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func writeReplayRegistryJSON(path string, entries []ReplayRegistryEntry) error {
	buf, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, buf, 0o664)
}

func writeReplayComparisonMarkdown(path string, entries []ReplayRegistryEntry) error {
	groups := map[string][]ReplayRegistryEntry{}
	keys := make([]string, 0)
	for _, e := range entries {
		wk := buildWindowLabel(e.StartTimeUTC, e.EndTimeUTC)
		if _, ok := groups[wk]; !ok {
			keys = append(keys, wk)
		}
		groups[wk] = append(groups[wk], e)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	var b strings.Builder
	b.WriteString("# Replay Comparison Ledger\n\n")
	b.WriteString("按回测窗口分组，展示各 profile 关键指标（NetPnL / MaxDD / PF / WinRate / Excess vs B&H）。\n\n")
	for _, k := range keys {
		rows := groups[k]
		sort.Slice(rows, func(i, j int) bool {
			if rows[i].NetPnL == rows[j].NetPnL {
				return rows[i].GeneratedAtUTC.Before(rows[j].GeneratedAtUTC)
			}
			return rows[i].NetPnL > rows[j].NetPnL
		})
		best := rows[0].NetPnL
		b.WriteString("## Window: " + k + "\n\n")
		b.WriteString("| Profile | NetPnL | MaxDD | PF | WinRate | Excess vs B&H | Report JSON |\n")
		b.WriteString("|---|---:|---:|---:|---:|---:|---|\n")
		for _, r := range rows {
			profile := r.Profile
			if nearlyEqual(r.NetPnL, best) {
				profile += " ⭐"
			}
			b.WriteString(fmt.Sprintf("| %s | %.2f | %.2f | %.3f | %.2f%% | %.2f | %s |\n",
				profile, r.NetPnL, r.MaxDrawdown, r.ProfitFactor, r.WinRate*100, r.ExcessReturn, r.ReportPathJSON))
		}
		b.WriteString("\n")
	}
	return os.WriteFile(path, []byte(b.String()), 0o664)
}

func buildWindowLabel(start, end time.Time) string {
	if !start.IsZero() && !end.IsZero() {
		return fmt.Sprintf("%s~%s", start.UTC().Format("2006-01-02"), end.UTC().Format("2006-01-02"))
	}
	return "unknown"
}

func nearlyEqual(a, b float64) bool {
	const eps = 1e-9
	if a > b {
		return a-b <= eps
	}
	return b-a <= eps
}
