package indicator

// EMA 计算指数移动平均线
// α = 2 / (N + 1)
// EMA_t = Close_t * α + EMA_{t-1} * (1 - α)
func EMA(closes []float64, period int) []float64 {
	if len(closes) < period {
		return nil
	}
	result := make([]float64, len(closes))
	alpha := 2.0 / float64(period+1)

	// 用前 period 个收盘价的 SMA 作为第一个 EMA 值
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += closes[i]
	}
	result[period-1] = sum / float64(period)

	for i := period; i < len(closes); i++ {
		result[i] = closes[i]*alpha + result[i-1]*(1-alpha)
	}
	return result
}

// LastEMA 返回最新的 EMA 值（用于实时计算）
func LastEMA(closes []float64, period int) float64 {
	v := EMA(closes, period)
	if v == nil || len(v) == 0 {
		return 0
	}
	return v[len(v)-1]
}
