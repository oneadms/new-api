package billingexpr

import "math"

// QuotaRound converts a float64 quota value to a non-negative int using
// half-away-from-zero rounding. Every billing path that converts a calculated
// quota into int MUST use this function to avoid +-1 discrepancies and
// overflow-induced negative credits.
func QuotaRound(f float64) int {
	if math.IsNaN(f) || f <= 0 {
		return 0
	}
	if math.IsInf(f, 1) {
		return math.MaxInt
	}
	rounded := math.Round(f)
	if rounded >= float64(math.MaxInt) {
		return math.MaxInt
	}
	if rounded <= 0 {
		return 0
	}
	return int(rounded)
}
