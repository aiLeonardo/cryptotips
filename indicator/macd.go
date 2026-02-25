package indicator

// MACDResult MACD 计算结果
type MACDResult struct {
	MACD      []float64 // DIF = EMA(fast) - EMA(slow)
	Signal    []float64 // DEA = EMA(MACD, signal)
	Histogram []float64 // MACD 柱 = (MACD - Signal) * 2
}

// MACD 计算 MACD 指标
// fast=12, slow=26, signal=9
func MACD(closes []float64, fast, slow, signal int) *MACDResult {
	if len(closes) < slow+signal {
		return nil
	}

	emaFast := EMA(closes, fast)
	emaSlow := EMA(closes, slow)
	if emaFast == nil || emaSlow == nil {
		return nil
	}

	n := len(closes)
	dif := make([]float64, n)
	for i := slow - 1; i < n; i++ {
		dif[i] = emaFast[i] - emaSlow[i]
	}

	// Signal line = EMA(DIF, signal period)
	// 只取有效部分来计算 EMA
	dea := EMA(dif[slow-1:], signal)
	if dea == nil {
		return nil
	}

	// 对齐到原始长度
	deaFull := make([]float64, n)
	offset := slow - 1
	for i, v := range dea {
		deaFull[offset+i] = v
	}

	hist := make([]float64, n)
	for i := offset + signal - 1; i < n; i++ {
		hist[i] = (dif[i] - deaFull[i]) * 2
	}

	return &MACDResult{
		MACD:      dif,
		Signal:    deaFull,
		Histogram: hist,
	}
}

// LastMACD 返回最新的 MACD 值
func LastMACD(closes []float64, fast, slow, signal int) (macd, sig, hist float64) {
	r := MACD(closes, fast, slow, signal)
	if r == nil {
		return 0, 0, 0
	}
	n := len(r.MACD)
	return r.MACD[n-1], r.Signal[n-1], r.Histogram[n-1]
}
