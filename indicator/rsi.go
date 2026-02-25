package indicator

// RSI 计算相对强弱指数（Wilder 平滑方法）
// RSI = 100 - 100 / (1 + RS)
// RS = 平均涨幅 / 平均跌幅
func RSI(closes []float64, period int) []float64 {
	if len(closes) < period+1 {
		return nil
	}
	result := make([]float64, len(closes))

	// 计算初始平均涨/跌
	var gains, losses float64
	for i := 1; i <= period; i++ {
		diff := closes[i] - closes[i-1]
		if diff > 0 {
			gains += diff
		} else {
			losses -= diff
		}
	}
	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)

	if avgLoss == 0 {
		result[period] = 100
	} else {
		rs := avgGain / avgLoss
		result[period] = 100 - 100/(1+rs)
	}

	// Wilder 平滑
	for i := period + 1; i < len(closes); i++ {
		diff := closes[i] - closes[i-1]
		gain, loss := 0.0, 0.0
		if diff > 0 {
			gain = diff
		} else {
			loss = -diff
		}
		avgGain = (avgGain*float64(period-1) + gain) / float64(period)
		avgLoss = (avgLoss*float64(period-1) + loss) / float64(period)

		if avgLoss == 0 {
			result[i] = 100
		} else {
			rs := avgGain / avgLoss
			result[i] = 100 - 100/(1+rs)
		}
	}
	return result
}

// LastRSI 返回最新 RSI 值
func LastRSI(closes []float64, period int) float64 {
	v := RSI(closes, period)
	if v == nil || len(v) == 0 {
		return 0
	}
	return v[len(v)-1]
}
