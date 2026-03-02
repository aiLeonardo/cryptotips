package spot_btc_conservative_v1

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/aiLeonardo/cryptotips/indicator"
	"github.com/aiLeonardo/cryptotips/models"
)

type RegimeCalibParams struct {
	EMA200SlopeBullMin  float64 `json:"ema200_slope_bull_min"`
	EMA200SlopeBearMax  float64 `json:"ema200_slope_bear_max"`
	ReturnBullMin       float64 `json:"return_bull_min"`
	ReturnBearMax       float64 `json:"return_bear_max"`
	DrawdownBearMin     float64 `json:"drawdown_bear_min"`
	VolRangeMax         float64 `json:"vol_range_max"`
	RegimeMinConfirmBar int     `json:"regime_min_confirm_bars"`
}

type RegimeSegment struct {
	Start      string  `json:"start"`
	End        string  `json:"end"`
	Regime     string  `json:"regime"`
	Confidence float64 `json:"confidence"`
	Score      float64 `json:"score"`
}

type RegimeCalibCandidate struct {
	Timeframe     string            `json:"timeframe"`
	Params        RegimeCalibParams `json:"params"`
	Objective     float64           `json:"objective"`
	LabelScore    float64           `json:"label_score"`
	TradeScore    float64           `json:"trade_score"`
	Stability     float64           `json:"stability"`
	PriorPenalty  float64           `json:"prior_penalty"`
	Profitability float64           `json:"profitability"`
	Switches      int               `json:"switches"`
	BullShare     float64           `json:"bull_share"`
	BearShare     float64           `json:"bear_share"`
	RangeShare    float64           `json:"range_share"`
}

type RegimeCalibrationReport struct {
	GeneratedAtUTC        time.Time              `json:"generated_at_utc"`
	Version               string                 `json:"version"`
	SelectedTimeframe     string                 `json:"selected_timeframe"`
	SelectionReason       string                 `json:"selection_reason"`
	DailyBest             RegimeCalibCandidate   `json:"daily_best"`
	WeeklyBest            RegimeCalibCandidate   `json:"weekly_best"`
	TopCandidates         []RegimeCalibCandidate `json:"top_candidates"`
	DailySegments         []RegimeSegment        `json:"daily_segments"`
	WeeklySegments        []RegimeSegment        `json:"weekly_segments"`
	SelectedSegments      []RegimeSegment        `json:"selected_segments"`
	SearchSpaceSummary    map[string]any         `json:"search_space_summary"`
	CalibrationWindowDays int                    `json:"calibration_window_days"`
	BoundaryBufferDays    int                    `json:"boundary_buffer_days"`
	DailyLabelScore       float64                `json:"daily_label_score"`
	WeeklyLabelScore      float64                `json:"weekly_label_score"`
	SelectedLabelScore    float64                `json:"selected_label_score"`
	SegmentScores         []FuzzySegmentScore    `json:"segment_scores"`
	JSONPath              string                 `json:"json_path"`
	MDPath                string                 `json:"md_path"`
}

type regimeClassPoint struct {
	Time       time.Time
	Regime     marketRegime
	Confidence float64
	Score      float64
}

type FuzzyLabelSegment struct {
	Start  string `json:"start"`
	End    string `json:"end"`
	Regime string `json:"regime"`
}

type FuzzySegmentScore struct {
	Start string  `json:"start"`
	End   string  `json:"end"`
	Label string  `json:"label"`
	Score float64 `json:"score"`
}

type FuzzyLabelSet struct {
	BufferDays int                `json:"buffer_days"`
	Segments   []FuzzyLabelSegment `json:"segments"`
}

type FuzzyLabelEval struct {
	TotalScore float64            `json:"total_score"`
	PerSegment []FuzzySegmentScore `json:"per_segment"`
}

func calibrateRegime(dayData []models.KLineRecord, h4Data []models.KLineRecord, cfg Config, start time.Time, version string) (*RegimeCalibrationReport, error) {
	if len(dayData) < 450 {
		return nil, fmt.Errorf("insufficient 1d data for calibration: %d", len(dayData))
	}
	windowDays := 1080
	end := dayData[len(dayData)-1].OpenTime.UTC()
	calStart := end.AddDate(0, 0, -windowDays)
	if calStart.Before(dayData[0].OpenTime.UTC()) {
		calStart = dayData[0].OpenTime.UTC()
	}

	d1 := filterBarsByTime(dayData, calStart)
	w1 := aggregate1dTo1w(d1)
	if len(w1) < 120 {
		w1 = nil
	}

	fuzzy := defaultFuzzyLabelSet(end, 14)
	candsDaily, bestDaily, dailyEval := calibrateOnBars("1d", d1, dayData, h4Data, start, cfg, fuzzy, version == "v7_1")
	if len(candsDaily) == 0 {
		return nil, fmt.Errorf("no daily calibration candidate")
	}
	candsAll := append([]RegimeCalibCandidate{}, candsDaily...)
	bestWeekly := bestDaily
	var candsWeekly []RegimeCalibCandidate
	if len(w1) > 0 {
		candsWeekly, bestWeekly, _ = calibrateOnBars("1w", w1, dayData, h4Data, start, cfg, fuzzy, version == "v7_1")
		candsAll = append(candsAll, candsWeekly...)
	}
	sort.Slice(candsAll, func(i, j int) bool { return candsAll[i].Objective > candsAll[j].Objective })
	if len(candsAll) > 12 {
		candsAll = candsAll[:12]
	}

	dailyPoints := classifySeries(d1, bestDaily.Params)
	weeklyPoints := classifySeries(w1, bestWeekly.Params)
	dailySeg := buildSegments(dailyPoints)
	weeklySeg := buildSegments(weeklyPoints)

	selected := bestDaily
	reason := "日线切换稳定且可区分 bull/bear，采用日线主判定"
	if shouldPreferWeekly(bestDaily) || (len(candsWeekly) > 0 && (bestWeekly.Objective > bestDaily.Objective+0.05 || (version == "v7_1" && bestWeekly.LabelScore > bestDaily.LabelScore+0.03))) {
		selected = bestWeekly
		reason = "日线判定不稳定或区分度不足，自动切换周线主判定；日线仅用于执行级别"
	}
	selectedSeg := dailySeg
	if selected.Timeframe == "1w" {
		selectedSeg = weeklySeg
	}

	rep := &RegimeCalibrationReport{
		GeneratedAtUTC:        time.Now().UTC(),
		Version:               version,
		SelectedTimeframe:     selected.Timeframe,
		SelectionReason:       reason,
		DailyBest:             bestDaily,
		WeeklyBest:            bestWeekly,
		TopCandidates:         candsAll,
		DailySegments:         dailySeg,
		WeeklySegments:        weeklySeg,
		SelectedSegments:      selectedSeg,
		CalibrationWindowDays: int(end.Sub(calStart).Hours() / 24),
		BoundaryBufferDays:    fuzzy.BufferDays,
		DailyLabelScore:       bestDaily.LabelScore,
		WeeklyLabelScore:      bestWeekly.LabelScore,
		SelectedLabelScore:    selected.LabelScore,
		SegmentScores:         dailyEval.PerSegment,
		SearchSpaceSummary: map[string]any{
			"ema200_slope_bull_min": []float64{0.0002, 0.0004},
			"ema200_slope_bear_max": []float64{-0.0002},
			"return_bull_min":       []float64{0.10},
			"return_bear_max":       []float64{-0.10},
			"drawdown_bear_min":     []float64{0.20, 0.30},
			"vol_range_max":         []float64{0.03},
			"confirm_bars":          []int{2, 3},
		},
	}
	if err := saveCalibrationReport(rep); err != nil {
		return nil, err
	}
	return rep, nil
}

func defaultFuzzyLabelSet(end time.Time, bufferDays int) FuzzyLabelSet {
	if bufferDays <= 0 {
		bufferDays = 14
	}
	return FuzzyLabelSet{BufferDays: bufferDays, Segments: []FuzzyLabelSegment{
		{Start: "2018-11-10", End: "2019-01-30", Regime: "BEAR"},
		{Start: "2019-03-21", End: "2019-08-08", Regime: "BULL"},
		{Start: "2019-08-08", End: "2020-07-20", Regime: "RANGE"},
		{Start: "2020-09-20", End: "2021-05-15", Regime: "BULL"},
		{Start: "2021-06-15", End: "2022-04-28", Regime: "RANGE"},
		{Start: "2022-04-28", End: "2022-12-07", Regime: "BEAR"},
		{Start: "2022-12-07", End: "2023-08-23", Regime: "RANGE"},
		{Start: "2023-08-23", End: "2025-11-30", Regime: "BULL"},
		{Start: "2025-12-10", End: end.Format("2006-01-02"), Regime: "BEAR"},
	}}
}

func evaluateFuzzyLabels(points []regimeClassPoint, set FuzzyLabelSet, timeframe string) FuzzyLabelEval {
	_ = timeframe
	if len(points) == 0 || len(set.Segments) == 0 {
		return FuzzyLabelEval{TotalScore: 0}
	}
	buf := time.Duration(set.BufferDays) * 24 * time.Hour
	total := 0.0
	count := 0.0
	per := make([]FuzzySegmentScore, 0, len(set.Segments))
	for _, seg := range set.Segments {
		st, err1 := time.Parse("2006-01-02", seg.Start)
		ed, err2 := time.Parse("2006-01-02", seg.End)
		if err1 != nil || err2 != nil || ed.Before(st) {
			continue
		}
		exp := marketRegime(seg.Regime)
		segSum := 0.0
		segCnt := 0.0
		for _, pt := range points {
			t := pt.Time
			inMain := (!t.Before(st) && !t.After(ed))
			inBuf := (!t.Before(st.Add(-buf)) && !t.After(ed.Add(buf))) && !inMain
			if !inMain && !inBuf {
				continue
			}
			base := fuzzyPairScore(exp, pt.Regime)
			if inBuf {
				base = 0.55 + 0.45*base
			}
			v := base * (0.7 + 0.3*pt.Confidence)
			segSum += v
			segCnt += 1
			total += v
			count += 1
		}
		if segCnt > 0 {
			per = append(per, FuzzySegmentScore{Start: seg.Start, End: seg.End, Label: seg.Regime, Score: segSum / segCnt})
		}
	}
	res := 0.0
	if count > 0 {
		res = total / count
	}
	return FuzzyLabelEval{TotalScore: res, PerSegment: per}
}

func fuzzyPairScore(expect, got marketRegime) float64 {
	if expect == got {
		return 1.0
	}
	if expect == regimeRange || got == regimeRange {
		return 0.62
	}
	return 0.12
}

func shouldPreferWeekly(d RegimeCalibCandidate) bool {
	if d.Switches > 18 {
		return true
	}
	if d.BullShare < 0.08 || d.BearShare < 0.05 {
		return true
	}
	if d.RangeShare > 0.75 {
		return true
	}
	return false
}

func calibrateOnBars(timeframe string, regimeBars []models.KLineRecord, dayData []models.KLineRecord, h4Data []models.KLineRecord, start time.Time, cfg Config, fuzzy FuzzyLabelSet, useFuzzy bool) ([]RegimeCalibCandidate, RegimeCalibCandidate, FuzzyLabelEval) {
	if len(regimeBars) < 80 {
		return nil, RegimeCalibCandidate{}, FuzzyLabelEval{}
	}
	slopesBull := []float64{0.0002, 0.0004}
	slopesBear := []float64{-0.0002}
	retsBull := []float64{0.10}
	retsBear := []float64{-0.10}
	dds := []float64{0.20, 0.30}
	volMax := []float64{0.03}
	confirms := []int{2, 3}

	cands := make([]RegimeCalibCandidate, 0, 256)
	best := RegimeCalibCandidate{Objective: -1e9}
	bestEval := FuzzyLabelEval{}
	for _, sb := range slopesBull {
		for _, sr := range slopesBear {
			for _, rb := range retsBull {
				for _, rr := range retsBear {
					for _, dd := range dds {
						for _, vm := range volMax {
							for _, cb := range confirms {
								p := RegimeCalibParams{EMA200SlopeBullMin: sb, EMA200SlopeBearMax: sr, ReturnBullMin: rb, ReturnBearMax: rr, DrawdownBearMin: dd, VolRangeMax: vm, RegimeMinConfirmBar: cb}
								pts := classifySeries(regimeBars, p)
								if len(pts) == 0 {
									continue
								}
								switches, bull, bear, rng := regimeStats(pts)
								stability := 1.0 / (1.0 + float64(switches)/8.0)
								priorPenalty := math.Abs(bull-0.75) + math.Abs(bear-0.25) + math.Max(0, rng-0.35)
								labelEval := evaluateFuzzyLabels(pts, fuzzy, timeframe)
								labelScore := labelEval.TotalScore
								tradeScore, profit := quickTradeScore(cfg, dayData, h4Data, start, timeframe, p)
								if !useFuzzy {
									labelScore = 1 - math.Min(1, priorPenalty)
								}
								regimeScore := 0.6*stability + 0.4*(1.0-math.Min(1, priorPenalty))
								obj := 0.45*labelScore + 0.35*tradeScore + 0.20*regimeScore
								cand := RegimeCalibCandidate{Timeframe: timeframe, Params: p, Objective: obj, LabelScore: labelScore, TradeScore: tradeScore, Stability: stability, PriorPenalty: priorPenalty, Profitability: profit, Switches: switches, BullShare: bull, BearShare: bear, RangeShare: rng}
								cands = append(cands, cand)
								if cand.Objective > best.Objective {
									best = cand
									bestEval = labelEval
								}
							}
						}
					}
				}
			}
		}
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].Objective > cands[j].Objective })
	if len(cands) > 10 {
		cands = cands[:10]
	}
	return cands, best, bestEval
}

func classifySeries(bars []models.KLineRecord, p RegimeCalibParams) []regimeClassPoint {
	if len(bars) == 0 {
		return nil
	}
	out := make([]regimeClassPoint, 0, len(bars))
	pending := regimeRange
	pendingCnt := 0
	current := regimeRange
	emaPeriod := 200
	if len(bars) < 210 {
		emaPeriod = 52
	}
	minWarmup := emaPeriod + 8
	for i := range bars {
		if i < minWarmup {
			continue
		}
		b := bars[:i+1]
		cand, conf, score := detectRegimeCalibrated(b, p)
		if cand != current {
			if cand != pending {
				pending = cand
				pendingCnt = 1
			} else {
				pendingCnt++
			}
			if pendingCnt >= p.RegimeMinConfirmBar {
				current = cand
				pendingCnt = 0
			}
		} else {
			pending = cand
			pendingCnt = 0
		}
		out = append(out, regimeClassPoint{Time: bars[i].OpenTime.UTC(), Regime: current, Confidence: conf, Score: score})
	}
	return out
}

func detectRegimeCalibrated(day []models.KLineRecord, p RegimeCalibParams) (marketRegime, float64, float64) {
	closes := make([]float64, 0, len(day))
	highs := make([]float64, 0, len(day))
	lows := make([]float64, 0, len(day))
	for _, d := range day {
		closes = append(closes, d.Close)
		highs = append(highs, d.High)
		lows = append(lows, d.Low)
	}
	emaPeriod := 200
	if len(closes) < 210 {
		emaPeriod = 52
	}
	if len(closes) < emaPeriod+8 {
		return regimeRange, 0.2, 0
	}
	price := closes[len(closes)-1]
	ema := indicator.LastEMA(closes, emaPeriod)
	emaSeries := indicator.EMA(closes, emaPeriod)
	if len(emaSeries) < 8 {
		return regimeRange, 0.2, 0
	}
	base := emaSeries[len(emaSeries)-8]
	slope := 0.0
	if base > 0 {
		slope = (emaSeries[len(emaSeries)-1] - base) / base
	}
	ret180 := 0.0
	if len(closes) > 180 && closes[len(closes)-181] > 0 {
		ret180 = price/closes[len(closes)-181] - 1
	}
	hh := closes[len(closes)-1]
	start := len(closes) - 365
	if start < 0 {
		start = 0
	}
	for i := start; i < len(closes); i++ {
		if closes[i] > hh {
			hh = closes[i]
		}
	}
	dd := 0.0
	if hh > 0 {
		dd = (hh - price) / hh
	}
	atr := indicator.LastATR(highs, lows, closes, 14)
	atrPct := 0.0
	if price > 0 {
		atrPct = atr / price
	}

	bullScore := 0.0
	bearScore := 0.0
	rangeScore := 0.0
	if price > ema {
		bullScore += 0.35
	}
	if slope >= p.EMA200SlopeBullMin {
		bullScore += 0.30
	}
	if ret180 >= p.ReturnBullMin {
		bullScore += 0.25
	}
	if dd < p.DrawdownBearMin*0.5 {
		bullScore += 0.10
	}
	if price < ema {
		bearScore += 0.35
	}
	if slope <= p.EMA200SlopeBearMax {
		bearScore += 0.30
	}
	if ret180 <= p.ReturnBearMax {
		bearScore += 0.25
	}
	if dd >= p.DrawdownBearMin {
		bearScore += 0.10
	}
	if math.Abs(slope) <= math.Max(math.Abs(p.EMA200SlopeBullMin), math.Abs(p.EMA200SlopeBearMax))*0.7 {
		rangeScore += 0.40
	}
	if atrPct <= p.VolRangeMax {
		rangeScore += 0.30
	}
	if dd > p.DrawdownBearMin*0.4 && dd < p.DrawdownBearMin {
		rangeScore += 0.30
	}

	reg := regimeRange
	best := rangeScore
	second := math.Max(bullScore, bearScore)
	if bullScore >= bearScore && bullScore > best {
		reg = regimeBull
		best = bullScore
		second = math.Max(bearScore, rangeScore)
	}
	if bearScore > bullScore && bearScore > best {
		reg = regimeBear
		best = bearScore
		second = math.Max(bullScore, rangeScore)
	}
	conf := 0.5 + (best-second)*0.5
	if conf > 0.99 {
		conf = 0.99
	}
	if conf < 0.1 {
		conf = 0.1
	}
	return reg, conf, best
}

func regimeStats(points []regimeClassPoint) (int, float64, float64, float64) {
	if len(points) == 0 {
		return 999, 0, 0, 1
	}
	sw := 0
	bull := 0
	bear := 0
	rng := 0
	prev := points[0].Regime
	for _, p := range points {
		switch p.Regime {
		case regimeBull:
			bull++
		case regimeBear:
			bear++
		default:
			rng++
		}
		if p.Regime != prev {
			sw++
			prev = p.Regime
		}
	}
	n := float64(len(points))
	return sw, float64(bull) / n, float64(bear) / n, float64(rng) / n
}

func buildSegments(points []regimeClassPoint) []RegimeSegment {
	if len(points) == 0 {
		return nil
	}
	out := make([]RegimeSegment, 0, 32)
	start := points[0]
	accConf := 0.0
	accScore := 0.0
	cnt := 0.0
	for i := range points {
		accConf += points[i].Confidence
		accScore += points[i].Score
		cnt++
		isEnd := i == len(points)-1 || points[i+1].Regime != points[i].Regime
		if isEnd {
			out = append(out, RegimeSegment{Start: start.Time.Format("2006-01-02"), End: points[i].Time.Format("2006-01-02"), Regime: string(points[i].Regime), Confidence: accConf / cnt, Score: accScore / cnt})
			if i < len(points)-1 {
				start = points[i+1]
			}
			accConf, accScore, cnt = 0, 0, 0
		}
	}
	return out
}

func quickTradeScore(cfg Config, dayData, h4Data []models.KLineRecord, start time.Time, tf string, p RegimeCalibParams) (float64, float64) {
	if len(h4Data) > 0 {
		quickStart := h4Data[len(h4Data)-1].OpenTime.UTC().AddDate(0, 0, -540)
		if quickStart.After(start) {
			start = quickStart
		}
	}
	s, err := runV7Replay(cfg, dayData, h4Data, start, tf, p)
	if err != nil {
		return 0, 0
	}
	capital := math.Max(cfg.RiskCapitalUSDT, 1)
	ret := s.NetPnL / capital
	dd := s.MaxDrawdown / capital
	retScore := 0.5 + 0.5*math.Tanh(ret*2.0)
	ddScore := 1 - math.Min(1, dd/0.35)
	pfScore := math.Min(1, s.ProfitFactor/2.0)
	freqScore := 0.4
	if s.TradeCount >= 6 && s.TradeCount <= 90 {
		freqScore = 1
	} else if s.TradeCount > 0 {
		freqScore = 0.7
	}
	score := 0.4*retScore + 0.3*ddScore + 0.2*pfScore + 0.1*freqScore
	profit := ret - 0.6*dd
	return score, 0.5 + math.Tanh(profit)*0.5
}

func saveCalibrationReport(r *RegimeCalibrationReport) error {
	dir := "strategies/reports"
	if err := os.MkdirAll(dir, 0o775); err != nil {
		return err
	}
	ts := r.GeneratedAtUTC.Format("20060102_150405")
	prefix := "v7"
	if r.Version == "v7_1" {
		prefix = "v7_1"
	}
	jsonPath := filepath.Join(dir, fmt.Sprintf("%s_regime_calibration_%s.json", prefix, ts))
	mdPath := filepath.Join(dir, fmt.Sprintf("%s_regime_calibration_%s.md", prefix, ts))
	r.JSONPath, r.MDPath = jsonPath, mdPath
	buf, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(jsonPath, buf, 0o664); err != nil {
		return err
	}
	if err := os.WriteFile(mdPath, []byte(renderCalibrationMarkdown(r)), 0o664); err != nil {
		return err
	}
	return nil
}

func renderCalibrationMarkdown(r *RegimeCalibrationReport) string {
	out := fmt.Sprintf("# %s Regime Calibration Report\n\n- Generated: %s\n- Calibration window: %d days\n- Boundary buffer: ±%d days\n- Selected timeframe: %s\n- Reason: %s\n- Label score(daily/weekly/selected): %.4f / %.4f / %.4f\n\n## Best Daily Candidate\n- objective=%.4f label=%.4f trade=%.4f stability=%.4f prior_penalty=%.4f profitability=%.4f switches=%d bull=%.2f bear=%.2f range=%.2f\n\n## Best Weekly Candidate\n- objective=%.4f label=%.4f trade=%.4f stability=%.4f prior_penalty=%.4f profitability=%.4f switches=%d bull=%.2f bear=%.2f range=%.2f\n\n## Top Candidates\n\n| rank | timeframe | objective | label | trade | stability | prior_penalty | profitability | switches | bull | bear | range |\n|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|\n",
		r.Version, r.GeneratedAtUTC.Format(time.RFC3339), r.CalibrationWindowDays, r.BoundaryBufferDays, r.SelectedTimeframe, r.SelectionReason, r.DailyLabelScore, r.WeeklyLabelScore, r.SelectedLabelScore,
		r.DailyBest.Objective, r.DailyBest.LabelScore, r.DailyBest.TradeScore, r.DailyBest.Stability, r.DailyBest.PriorPenalty, r.DailyBest.Profitability, r.DailyBest.Switches, r.DailyBest.BullShare, r.DailyBest.BearShare, r.DailyBest.RangeShare,
		r.WeeklyBest.Objective, r.WeeklyBest.LabelScore, r.WeeklyBest.TradeScore, r.WeeklyBest.Stability, r.WeeklyBest.PriorPenalty, r.WeeklyBest.Profitability, r.WeeklyBest.Switches, r.WeeklyBest.BullShare, r.WeeklyBest.BearShare, r.WeeklyBest.RangeShare,
	)
	for i, c := range r.TopCandidates {
		out += fmt.Sprintf("| %d | %s | %.4f | %.4f | %.4f | %.4f | %.4f | %.4f | %d | %.2f | %.2f | %.2f |\n", i+1, c.Timeframe, c.Objective, c.LabelScore, c.TradeScore, c.Stability, c.PriorPenalty, c.Profitability, c.Switches, c.BullShare, c.BearShare, c.RangeShare)
	}
	out += "\n## 日线方案分段\n\n| Start | End | Regime | Confidence | Score |\n|---|---|---|---:|---:|\n"
	for _, s := range r.DailySegments {
		out += fmt.Sprintf("| %s | %s | %s | %.3f | %.3f |\n", s.Start, s.End, s.Regime, s.Confidence, s.Score)
	}
	out += "\n## 周线方案分段\n\n| Start | End | Regime | Confidence | Score |\n|---|---|---|---:|---:|\n"
	for _, s := range r.WeeklySegments {
		out += fmt.Sprintf("| %s | %s | %s | %.3f | %.3f |\n", s.Start, s.End, s.Regime, s.Confidence, s.Score)
	}
	out += "\n## Fuzzy 标签分段匹配得分\n\n| Start | End | Label | Score |\n|---|---|---|---:|\n"
	for _, ss := range r.SegmentScores {
		out += fmt.Sprintf("| %s | %s | %s | %.4f |\n", ss.Start, ss.End, ss.Label, ss.Score)
	}
	return out
}

func aggregate1dTo1w(day []models.KLineRecord) []models.KLineRecord {
	if len(day) == 0 {
		return nil
	}
	out := make([]models.KLineRecord, 0, len(day)/7)
	var cur *models.KLineRecord
	for _, d := range day {
		y, w := d.OpenTime.UTC().ISOWeek()
		if cur == nil {
			t := d
			cur = &t
			continue
		}
		cy, cw := cur.OpenTime.UTC().ISOWeek()
		if y != cy || w != cw {
			out = append(out, *cur)
			t := d
			cur = &t
			continue
		}
		if d.High > cur.High {
			cur.High = d.High
		}
		if d.Low < cur.Low {
			cur.Low = d.Low
		}
		cur.Close = d.Close
		cur.CloseTime = d.CloseTime
		cur.Volume += d.Volume
	}
	if cur != nil {
		out = append(out, *cur)
	}
	return out
}

func filterBarsByTime(bars []models.KLineRecord, start time.Time) []models.KLineRecord {
	out := make([]models.KLineRecord, 0, len(bars))
	for _, b := range bars {
		if !b.OpenTime.UTC().Before(start) {
			out = append(out, b)
		}
	}
	return out
}

func calibToParams(r *RegimeCalibrationReport) RegimeCalibParams {
	if r == nil {
		return RegimeCalibParams{}
	}
	if r.SelectedTimeframe == "1w" {
		return r.WeeklyBest.Params
	}
	return r.DailyBest.Params
}

func runV7Replay(cfg Config, dayData, h4Data []models.KLineRecord, start time.Time, timeframe string, p RegimeCalibParams) (ReplaySummary, error) {
	regimeBars := dayData
	if timeframe == "1w" {
		regimeBars = aggregate1dTo1w(dayData)
	}
	if len(regimeBars) < 220 {
		return ReplaySummary{}, fmt.Errorf("insufficient regime bars for v7: %d", len(regimeBars))
	}
	points := classifySeries(regimeBars, p)
	if len(points) == 0 {
		return ReplaySummary{}, fmt.Errorf("no regime points for v7")
	}
	idx := 0
	currentRegime := regimeRange
	switchCounts := map[string]int{}

	var wins int
	var profits float64
	var lossesAbs float64
	var net float64
	var peak float64
	var maxDD float64
	monthly := map[string]float64{}
	capital := cfg.RiskCapitalUSDT
	longPos := replayPosition{}
	shortPos := replayShortPosition{}
	tradeCount := 0
	pauseBars := 0
	consecutiveLosses := 0
	dailyPnL := map[string]float64{}
	weeklyPnL := map[string]float64{}

	for i := range h4Data {
		cur := h4Data[i]
		t := cur.OpenTime.UTC()
		if t.Before(start) {
			continue
		}
		for idx < len(points) && !points[idx].Time.After(t) {
			if points[idx].Regime != currentRegime {
				switchCounts[fmt.Sprintf("%s->%s", currentRegime, points[idx].Regime)]++
				currentRegime = points[idx].Regime
			}
			idx++
		}

		h4Slice := make([]OHLC, 0, i+1)
		for j := 0; j <= i; j++ {
			h4Slice = append(h4Slice, OHLC{Close: h4Data[j].Close, High: h4Data[j].High, Low: h4Data[j].Low, Volume: h4Data[j].Volume})
		}
		daySlice := make([]OHLC, 0, 256)
		for _, d := range dayData {
			if d.OpenTime.UTC().After(t) {
				break
			}
			daySlice = append(daySlice, OHLC{Close: d.Close, High: d.High, Low: d.Low, Volume: d.Volume})
		}
		if pauseBars > 0 {
			pauseBars--
		}
		dayKey := t.Format("2006-01-02")
		_, week := t.ISOWeek()
		weekKey := fmt.Sprintf("%d-W%02d", t.Year(), week)
		riskBlocked := -dailyPnL[dayKey] >= capital*cfg.MaxDailyLossPct || -weeklyPnL[weekKey] >= capital*cfg.MaxWeeklyLossPct || pauseBars > 0

		if currentRegime == regimeRange {
			if longPos.active {
				pnl := longPos.realizedPart + (cur.Close-longPos.entryPrice)*longPos.qtyRemain
				net += pnl
				monthly[t.Format("2006-01")] += pnl / capital
				tradeCount++
				longPos = replayPosition{}
			}
			if shortPos.active {
				pnl := shortPos.realizedPart + (shortPos.entryPrice-cur.Close)*shortPos.qtyRemain
				net += pnl
				monthly[t.Format("2006-01")] += pnl / capital
				tradeCount++
				shortPos = replayShortPosition{}
			}
		}

		if !longPos.active && !shortPos.active && !riskBlocked {
			switch currentRegime {
			case regimeBull:
				sig, err := evalTrend(cfg, daySlice, h4Slice)
				if err != nil {
					return ReplaySummary{}, err
				}
				if sig.ShouldEnter {
					plan := BuildPosition(cfg, sig.EntryPrice, sig.StopLoss)
					if plan.Qty > 0 {
						longPos = replayPosition{active: true, entryPrice: sig.EntryPrice, stopLoss: sig.StopLoss, riskPerUnit: plan.RiskPerUnit, qtyTotal: plan.Qty, qtyRemain: plan.Qty}
					}
				}
			case regimeBear:
				sig, err := evalTrendShort(cfg, daySlice, h4Slice)
				if err != nil {
					return ReplaySummary{}, err
				}
				if sig.ShouldEnter {
					plan := buildShortPosition(cfg, sig.EntryPrice, sig.StopLoss)
					if plan.Qty > 0 {
						shortPos = replayShortPosition{active: true, entryPrice: sig.EntryPrice, stopLoss: sig.StopLoss, riskPerUnit: plan.RiskPerUnit, qtyTotal: plan.Qty, qtyRemain: plan.Qty}
					}
				}
			}
		}

		if longPos.active {
			pnl, closed := stepLongPosition(cfg, &longPos, cur.Close, h4Slice)
			if closed {
				net += pnl
				monthly[t.Format("2006-01")] += pnl / capital
				dailyPnL[dayKey] += pnl
				weeklyPnL[weekKey] += pnl
				if pnl > 0 {
					wins++
					profits += pnl
					consecutiveLosses = 0
				} else if pnl < 0 {
					lossesAbs += -pnl
					consecutiveLosses++
					if consecutiveLosses >= 3 {
						pauseBars = 6
						consecutiveLosses = 0
					}
				}
				tradeCount++
			}
		}
		if shortPos.active {
			pnl, closed := stepShortPosition(cfg, &shortPos, cur.Close, h4Slice)
			if closed {
				net += pnl
				monthly[t.Format("2006-01")] += pnl / capital
				dailyPnL[dayKey] += pnl
				weeklyPnL[weekKey] += pnl
				if pnl > 0 {
					wins++
					profits += pnl
					consecutiveLosses = 0
				} else if pnl < 0 {
					lossesAbs += -pnl
					consecutiveLosses++
					if consecutiveLosses >= 3 {
						pauseBars = 6
						consecutiveLosses = 0
					}
				}
				tradeCount++
			}
		}

		equity := net
		if longPos.active {
			equity += longPos.realizedPart + (cur.Close-longPos.entryPrice)*longPos.qtyRemain
		}
		if shortPos.active {
			equity += shortPos.realizedPart + (shortPos.entryPrice-cur.Close)*shortPos.qtyRemain
		}
		if equity > peak {
			peak = equity
		}
		dd := peak - equity
		if dd > maxDD {
			maxDD = dd
		}
	}

	winRate := 0.0
	if tradeCount > 0 {
		winRate = float64(wins) / float64(tradeCount)
	}
	profitFactor := 0.0
	if lossesAbs > 0 {
		profitFactor = profits / lossesAbs
	}
	sharpe := 0.0
	calmar := 0.0
	if len(monthly) >= 2 {
		vals := make([]float64, 0, len(monthly))
		for _, v := range monthly {
			vals = append(vals, v)
		}
		mean, std := meanStd(vals)
		if std > 0 {
			sharpe = mean / std * math.Sqrt(12)
		}
	}
	if maxDD > 0 {
		calmar = net / maxDD
	}
	return ReplaySummary{Profile: "v7", TradeCount: tradeCount, WinRate: winRate, ProfitFactor: profitFactor, MaxDrawdown: maxDD, NetPnL: net, MonthlyReturns: monthly, Sharpe: sharpe, Calmar: calmar, RegimeSwitches: switchCounts, RegimeTimeframe: timeframe}, nil
}
