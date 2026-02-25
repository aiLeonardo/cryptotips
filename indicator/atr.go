package indicator

import "math"

// ATR 计算平均真实范围（Wilder 平滑方法）
// TR = max(High-Low, |High-PrevClose|, |Low-PrevClose|)
// ATR = Wilder 平滑的 TR
func ATR(highs, lows, closes []float64, period int) []float64 {
	n := len(closes)
	if n < period+1 || len(highs) != n || len(lows) != n {
		return nil
	}

	tr := make([]float64, n)
	for i := 1; i < n; i++ {
		hl := highs[i] - lows[i]
		hpc := math.Abs(highs[i] - closes[i-1])
		lpc := math.Abs(lows[i] - closes[i-1])
		tr[i] = math.Max(hl, math.Max(hpc, lpc))
	}
	tr[0] = highs[0] - lows[0]

	result := make([]float64, n)
	// 初始 ATR = 前 period 个 TR 的均值
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += tr[i]
	}
	result[period-1] = sum / float64(period)

	for i := period; i < n; i++ {
		result[i] = (result[i-1]*float64(period-1) + tr[i]) / float64(period)
	}
	return result
}

// LastATR 返回最新 ATR 值
func LastATR(highs, lows, closes []float64, period int) float64 {
	v := ATR(highs, lows, closes, period)
	if v == nil || len(v) == 0 {
		return 0
	}
	return v[len(v)-1]
}
