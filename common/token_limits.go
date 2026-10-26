package common

import "math"

// MaxTokensLimit bounds caller-controlled generation limits before they reach
// quota arithmetic. Both relay validators and internal request producers use
// this value so their admission limits cannot diverge across dependency layers.
const MaxTokensLimit = math.MaxInt32 / 2
