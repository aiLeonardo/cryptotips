package indicator

import "math"

// QuoteVolumeLog 对 USDT 成交额做对数化，压缩极值尖峰。
// 为避免 log(0)，最小值钳制为 1。
func QuoteVolumeLog(quoteVolume []float64) []float64 {
	if len(quoteVolume) == 0 {
		return nil
	}
	out := make([]float64, len(quoteVolume))
	for i, v := range quoteVolume {
		if v < 1 {
			v = 1
		}
		out[i] = math.Log(v)
	}
	return out
}

// RollingStd 计算滚动标准差（总体标准差）。
// 当窗口不足 period 时返回 0。
func RollingStd(values []float64, period int) []float64 {
	if len(values) == 0 || period <= 1 {
		return nil
	}
	out := make([]float64, len(values))
	for i := range values {
		if i+1 < period {
			out[i] = 0
			continue
		}
		start := i + 1 - period
		mean := 0.0
		for j := start; j <= i; j++ {
			mean += values[j]
		}
		mean /= float64(period)

		variance := 0.0
		for j := start; j <= i; j++ {
			d := values[j] - mean
			variance += d * d
		}
		variance /= float64(period)
		out[i] = math.Sqrt(variance)
	}
	return out
}

// QuoteVolumeZScore 计算成交额异常强度：
// z = (ln(qv)-EMA(ln(qv), period)) / RollingStd(ln(qv)-EMA, period)
// 常用于识别异常放量（如 z > 2）。
func QuoteVolumeZScore(quoteVolume []float64, period int) (logQ []float64, emaLogQ []float64, z []float64) {
	if len(quoteVolume) == 0 || period <= 1 {
		return nil, nil, nil
	}
	logQ = QuoteVolumeLog(quoteVolume)
	emaLogQ = EMA(logQ, period)
	if len(emaLogQ) == 0 {
		return logQ, nil, nil
	}

	dev := make([]float64, len(logQ))
	for i := range logQ {
		dev[i] = logQ[i] - emaLogQ[i]
	}
	std := RollingStd(dev, period)
	z = make([]float64, len(logQ))
	for i := range logQ {
		if std[i] > 0 {
			z[i] = dev[i] / std[i]
		}
	}
	return logQ, emaLogQ, z
}
