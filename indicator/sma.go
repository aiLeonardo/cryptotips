package indicator

// SMA 计算简单移动平均线
func SMA(closes []float64, period int) []float64 {
	if len(closes) < period {
		return nil
	}
	result := make([]float64, len(closes))
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += closes[i]
	}
	result[period-1] = sum / float64(period)

	for i := period; i < len(closes); i++ {
		sum += closes[i] - closes[i-period]
		result[i] = sum / float64(period)
	}
	return result
}

// LastSMA 返回最新 SMA 值
func LastSMA(closes []float64, period int) float64 {
	v := SMA(closes, period)
	if v == nil || len(v) == 0 {
		return 0
	}
	return v[len(v)-1]
}

// SMASlope 计算 SMA 的斜率（相邻两个值的差 / 前一个值，百分比变化）
// 返回斜率序列，与 SMA 序列等长（第一个有效值位于 period 处）
func SMASlope(closes []float64, period int) []float64 {
	sma := SMA(closes, period)
	if sma == nil {
		return nil
	}
	slope := make([]float64, len(closes))
	for i := period; i < len(closes); i++ {
		if sma[i-1] != 0 {
			slope[i] = (sma[i] - sma[i-1]) / sma[i-1]
		}
	}
	return slope
}

// LastSMASlope 返回最新的 SMA 斜率
func LastSMASlope(closes []float64, period int) float64 {
	v := SMASlope(closes, period)
	if v == nil || len(v) == 0 {
		return 0
	}
	return v[len(v)-1]
}
