package indicator

import (
	"math"
	"time"
)

// RegimeBar 用于 Regime 判定的基础 K 线（仅需时间与收盘价）
type RegimeBar struct {
	Time  time.Time
	Close float64
}

// RegimeStartpoint 牛/熊/震荡状态起点
type RegimeStartpoint struct {
	Time       time.Time
	State      string
	Confidence float64
}

type dayFeatures struct {
	Dates   []time.Time
	Close   []float64
	EMA200  []float64
	Slope20 []float64
	Ret90   []float64
	Ret180  []float64
	DD365   []float64
	Score   []float64
	State   []string
}

type weekFeatures struct {
	Dates  []time.Time
	Close  []float64
	EMA52  []float64
	Slope8 []float64
	Ret26  []float64
	DD52   []float64
	Vol8   []float64
	Score  []float64
	State  []string
}

type regimeSegment struct {
	Start      time.Time
	End        time.Time
	State      string
	Confidence float64
	Days       int
}

// ComputeRegimeStartpoints 复刻 python 脚本逻辑，输出 regime 起点。
func ComputeRegimeStartpoints(dailyBars, weeklyBars []RegimeBar) []RegimeStartpoint {
	if len(dailyBars) == 0 || len(weeklyBars) == 0 {
		return nil
	}

	dayf := classifyTrend1D(dailyBars)
	weekf := classifyCP1W(weeklyBars)
	finalState, confs := fuseStates(dayf, weekf)
	segments := buildSegments(dayf.Dates, finalState, confs)

	out := make([]RegimeStartpoint, 0, len(segments))
	for _, s := range segments {
		out = append(out, RegimeStartpoint{
			Time:       s.Start.UTC(),
			State:      s.State,
			Confidence: s.Confidence,
		})
	}
	return out
}

func classifyTrend1D(dailyBars []RegimeBar) dayFeatures {
	dates := make([]time.Time, 0, len(dailyBars))
	closeVals := make([]float64, 0, len(dailyBars))
	for _, b := range dailyBars {
		dates = append(dates, b.Time.UTC())
		closeVals = append(closeVals, b.Close)
	}

	ema200 := ema(closeVals, 200)
	slope20 := make([]float64, len(closeVals))
	for i := range closeVals {
		j := i - 20
		if j >= 0 && ema200[j] > 0 {
			slope20[i] = ema200[i]/ema200[j] - 1.0
		}
	}

	ret90 := pctReturn(closeVals, 90)
	ret180 := pctReturn(closeVals, 180)
	dd365 := drawdown(closeVals, 365)

	states := make([]string, 0, len(closeVals))
	scores := make([]float64, 0, len(closeVals))
	for i := range closeVals {
		bull := 0.0
		bear := 0.0
		rng := 0.0

		if closeVals[i] > ema200[i] {
			bull += 0.35
		} else {
			bear += 0.25
		}

		if slope20[i] >= 0.004 {
			bull += 0.30
		} else if slope20[i] <= -0.004 {
			bear += 0.30
		} else {
			rng += 0.20
		}

		if ret180[i] >= 0.18 {
			bull += 0.20
		} else if ret180[i] <= -0.18 {
			bear += 0.20
		} else {
			rng += 0.10
		}

		if dd365[i] >= 0.35 {
			bear += 0.15
		} else if dd365[i] <= 0.18 && ret90[i] > -0.05 {
			bull += 0.10
		} else {
			rng += 0.15
		}

		if math.Abs(slope20[i]) <= 0.0025 {
			rng += 0.30
		}

		if bull >= bear && bull >= rng {
			states = append(states, "BULL")
			scores = append(scores, math.Min(1.0, bull))
		} else if bear >= bull && bear >= rng {
			states = append(states, "BEAR")
			scores = append(scores, -math.Min(1.0, bear))
		} else {
			states = append(states, "RANGE")
			if rng == 0 {
				scores = append(scores, 0)
			} else if bull > bear {
				scores = append(scores, 0.2)
			} else {
				scores = append(scores, -0.2)
			}
		}
	}

	smoothed := append([]string(nil), states...)
	for i := 4; i < len(states); i++ {
		last := states[i]
		cnt := 0
		for _, s := range states[i-4 : i+1] {
			if s == last {
				cnt++
			}
		}
		if cnt >= 4 {
			smoothed[i] = last
		} else {
			smoothed[i] = smoothed[i-1]
		}
	}

	return dayFeatures{
		Dates:   dates,
		Close:   closeVals,
		EMA200:  ema200,
		Slope20: slope20,
		Ret90:   ret90,
		Ret180:  ret180,
		DD365:   dd365,
		Score:   scores,
		State:   smoothed,
	}
}

func classifyCP1W(weeklyBars []RegimeBar) weekFeatures {
	dates := make([]time.Time, 0, len(weeklyBars))
	closeVals := make([]float64, 0, len(weeklyBars))
	for _, b := range weeklyBars {
		dates = append(dates, b.Time.UTC())
		closeVals = append(closeVals, b.Close)
	}

	ema52 := ema(closeVals, 52)
	slope8 := make([]float64, len(closeVals))
	for i := range closeVals {
		j := i - 8
		if j >= 0 && ema52[j] > 0 {
			slope8[i] = ema52[i]/ema52[j] - 1.0
		}
	}

	rets := make([]float64, len(closeVals))
	for i := 1; i < len(closeVals); i++ {
		if closeVals[i-1] > 0 {
			rets[i] = math.Log(closeVals[i] / closeVals[i-1])
		}
	}
	vol8 := rollingStd(rets, 8)
	dd52 := drawdown(closeVals, 52)
	ret26 := pctReturn(closeVals, 26)

	rawState := make([]string, 0, len(closeVals))
	rawScore := make([]float64, 0, len(closeVals))
	for i := range closeVals {
		bull := 0.0
		bear := 0.0
		rng := 0.0

		if closeVals[i] > ema52[i] {
			bull += 0.35
		} else {
			bear += 0.30
		}

		if slope8[i] >= 0.02 {
			bull += 0.25
		} else if slope8[i] <= -0.02 {
			bear += 0.25
		} else {
			rng += 0.25
		}

		if ret26[i] >= 0.15 {
			bull += 0.20
		} else if ret26[i] <= -0.15 {
			bear += 0.20
		} else {
			rng += 0.10
		}

		if dd52[i] >= 0.30 {
			bear += 0.20
		} else if dd52[i] <= 0.15 {
			bull += 0.10
		} else {
			rng += 0.15
		}

		if vol8[i] <= 0.035 {
			rng += 0.20
		}

		if bull >= bear && bull >= rng {
			rawState = append(rawState, "BULL")
			rawScore = append(rawScore, math.Min(1.0, bull))
		} else if bear >= bull && bear >= rng {
			rawState = append(rawState, "BEAR")
			rawScore = append(rawScore, -math.Min(1.0, bear))
		} else {
			rawState = append(rawState, "RANGE")
			rawScore = append(rawScore, 0)
		}
	}

	sm := append([]string(nil), rawState...)
	for i := 1; i < len(sm); i++ {
		if rawState[i] != sm[i-1] {
			if i+1 < len(sm) && rawState[i+1] == rawState[i] {
				sm[i] = rawState[i]
			} else {
				sm[i] = sm[i-1]
			}
		}
	}

	return weekFeatures{
		Dates:  dates,
		Close:  closeVals,
		EMA52:  ema52,
		Slope8: slope8,
		Ret26:  ret26,
		DD52:   dd52,
		Vol8:   vol8,
		Score:  rawScore,
		State:  sm,
	}
}

func fuseStates(dayf dayFeatures, weekf weekFeatures) ([]string, []float64) {
	ws, wsc := expandWeeklyToDaily(weekf, dayf.Dates)

	finalState := make([]string, 0, len(dayf.Dates))
	confidence := make([]float64, 0, len(dayf.Dates))

	for i := range dayf.Dates {
		s1 := dayf.State[i]
		s2 := ws[i]
		sc1 := dayf.Score[i]
		sc2 := wsc[i]

		v1 := stateToNum(s1) * (0.6 + 0.4*math.Min(1.0, math.Abs(sc1)))
		v2 := stateToNum(s2) * (0.6 + 0.4*math.Min(1.0, math.Abs(sc2)))
		combined := 0.55*v1 + 0.45*v2

		st := "RANGE"
		if combined >= 0.22 {
			st = "BULL"
		} else if combined <= -0.22 {
			st = "BEAR"
		}

		agree := 0.0
		if s1 == s2 {
			agree = 1.0
		}
		conf := 0.40 + 0.30*agree + 0.20*math.Min(1.0, math.Abs(combined)) + 0.10*(1.0-math.Abs(v1-v2)/2.0)
		conf = clamp(conf, 0.05, 0.99)

		finalState = append(finalState, st)
		confidence = append(confidence, conf)
	}

	for i := 1; i < len(finalState)-2; i++ {
		if finalState[i] != finalState[i-1] && finalState[i+1] == finalState[i-1] {
			finalState[i] = finalState[i-1]
			confidence[i] = math.Min(confidence[i], 0.62)
		}
	}

	return finalState, confidence
}

func buildSegments(dates []time.Time, states []string, confs []float64) []regimeSegment {
	if len(states) == 0 || len(dates) != len(states) || len(confs) != len(states) {
		return nil
	}

	segs := make([]regimeSegment, 0)
	st := 0
	for i := 1; i <= len(states); i++ {
		if i == len(states) || states[i] != states[st] {
			part := confs[st:i]
			sum := 0.0
			for _, v := range part {
				sum += v
			}
			avg := 0.0
			if len(part) > 0 {
				avg = sum / float64(len(part))
			}
			segs = append(segs, regimeSegment{
				Start:      dates[st].UTC(),
				End:        dates[i-1].UTC(),
				State:      states[st],
				Confidence: roundTo(avg, 3),
				Days:       i - st,
			})
			st = i
		}
	}

	merged := make([]regimeSegment, 0, len(segs))
	for _, s := range segs {
		if len(merged) > 0 && (s.Days < 14 || (s.Days < 21 && s.Confidence < 0.75)) {
			last := &merged[len(merged)-1]
			last.End = s.End
			last.Days += s.Days
			last.Confidence = roundTo((last.Confidence+s.Confidence)/2.0, 3)
			continue
		}
		merged = append(merged, s)
	}

	final := make([]regimeSegment, 0, len(merged))
	for _, s := range merged {
		if len(final) > 0 && final[len(final)-1].State == s.State {
			last := &final[len(final)-1]
			last.End = s.End
			last.Days += s.Days
			last.Confidence = roundTo((last.Confidence+s.Confidence)/2.0, 3)
		} else {
			final = append(final, s)
		}
	}

	return final
}

func expandWeeklyToDaily(weekf weekFeatures, dailyDates []time.Time) ([]string, []float64) {
	if len(weekf.Dates) == 0 {
		return make([]string, len(dailyDates)), make([]float64, len(dailyDates))
	}

	idx := 0
	outState := make([]string, 0, len(dailyDates))
	outScore := make([]float64, 0, len(dailyDates))
	for _, d := range dailyDates {
		for idx+1 < len(weekf.Dates) && !weekf.Dates[idx+1].After(d) {
			idx++
		}
		outState = append(outState, weekf.State[idx])
		outScore = append(outScore, weekf.Score[idx])
	}
	return outState, outScore
}

func ema(values []float64, period int) []float64 {
	out := make([]float64, len(values))
	if len(values) == 0 {
		return out
	}
	alpha := 2.0 / (float64(period) + 1.0)
	out[0] = values[0]
	for i := 1; i < len(values); i++ {
		out[i] = alpha*values[i] + (1-alpha)*out[i-1]
	}
	return out
}

func rollingStd(values []float64, window int) []float64 {
	out := make([]float64, len(values))
	for i := range values {
		if i+1 < window {
			continue
		}
		seg := values[i+1-window : i+1]
		sum := 0.0
		for _, v := range seg {
			sum += v
		}
		m := sum / float64(len(seg))
		varSum := 0.0
		for _, v := range seg {
			d := v - m
			varSum += d * d
		}
		out[i] = math.Sqrt(varSum / float64(len(seg)))
	}
	return out
}

func pctReturn(values []float64, lookback int) []float64 {
	out := make([]float64, len(values))
	for i := range values {
		j := i - lookback
		if j >= 0 && values[j] > 0 {
			out[i] = values[i]/values[j] - 1.0
		}
	}
	return out
}

func drawdown(values []float64, lookback int) []float64 {
	out := make([]float64, len(values))
	for i := range values {
		st := i - lookback + 1
		if st < 0 {
			st = 0
		}
		hh := values[st]
		for j := st + 1; j <= i; j++ {
			if values[j] > hh {
				hh = values[j]
			}
		}
		if hh > 0 {
			out[i] = (hh - values[i]) / hh
		}
	}
	return out
}

func stateToNum(st string) float64 {
	switch st {
	case "BULL":
		return 1
	case "BEAR":
		return -1
	default:
		return 0
	}
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func roundTo(v float64, digits int) float64 {
	mul := math.Pow(10, float64(digits))
	return math.Round(v*mul) / mul
}
