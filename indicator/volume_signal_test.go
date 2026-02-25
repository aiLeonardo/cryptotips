package indicator

import "testing"

func TestQuoteVolumeZScore(t *testing.T) {
	qv := []float64{100, 120, 130, 125, 128, 150, 400, 180, 170, 160, 155, 152}
	logQ, emaLogQ, z := QuoteVolumeZScore(qv, 5)
	if len(logQ) != len(qv) || len(emaLogQ) != len(qv) || len(z) != len(qv) {
		t.Fatalf("unexpected length: log=%d ema=%d z=%d", len(logQ), len(emaLogQ), len(z))
	}
	if z[6] <= 0 {
		t.Fatalf("expected spike z-score > 0 at index 6, got %f", z[6])
	}
}
