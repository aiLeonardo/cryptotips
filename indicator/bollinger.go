package indicator

import "math"

// BollingerResult 布林带计算结果
type BollingerResult struct {
	Middle []float64 // 中轨 = SMA(N)
	Upper  []float64 // 上轨 = Middle + k * σ
	Lower  []float64 // 下轨 = Middle - k * σ
}

// Bollinger 计算布林带
// period=20, k=2（2σ）
func Bollinger(closes []float64, period int, k float64) *BollingerResult {
	if len(closes) < period {
		return nil
	}

	n := len(closes)
	middle := SMA(closes, period)
	upper := make([]float64, n)
	lower := make([]float64, n)

	for i := period - 1; i < n; i++ {
		// 计算窗口内的标准差
		mean := middle[i]
		variance := 0.0
		for j := i - period + 1; j <= i; j++ {
			diff := closes[j] - mean
			variance += diff * diff
		}
		stdDev := math.Sqrt(variance / float64(period))
		upper[i] = mean + k*stdDev
		lower[i] = mean - k*stdDev
	}

	return &BollingerResult{
		Middle: middle,
		Upper:  upper,
		Lower:  lower,
	}
}

// LastBollinger 返回最新的布林带值
func LastBollinger(closes []float64, period int, k float64) (upper, middle, lower float64) {
	r := Bollinger(closes, period, k)
	if r == nil {
		return 0, 0, 0
	}
	n := len(r.Middle)
	return r.Upper[n-1], r.Middle[n-1], r.Lower[n-1]
}
