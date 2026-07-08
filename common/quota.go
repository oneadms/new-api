package common

import "math"

func GetTrustQuota() int {
	return int(10 * QuotaPerUnit)
}

func SaturatingAddInt(a int, b int) int {
	if b > 0 && a > math.MaxInt-b {
		return math.MaxInt
	}
	if b < 0 && a < math.MinInt-b {
		return math.MinInt
	}
	return a + b
}
