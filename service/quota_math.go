package service

import (
	"math"

	"github.com/shopspring/decimal"
)

func quotaDecimalRound(d decimal.Decimal) int {
	rounded := d.Round(0)
	if rounded.LessThanOrEqual(decimal.Zero) {
		return 0
	}
	if rounded.GreaterThan(decimal.NewFromInt(math.MaxInt)) {
		return math.MaxInt
	}
	return int(rounded.IntPart())
}
