package common

import (
	"fmt"
	"math"
)

// Quota columns are 32-bit integers in every supported database. Billing
// conversions must saturate instead of allowing an oversized float-to-int
// conversion to wrap into a negative charge.
const (
	MaxQuota = math.MaxInt32
	MinQuota = math.MinInt32
)

// QuotaFromFloat converts a computed quota product by truncating toward zero
// and clamping it to the database-safe int32 range.
func QuotaFromFloat(value float64) int {
	switch {
	case math.IsNaN(value):
		SysError("quota conversion received NaN, falling back to 0")
		return 0
	case value >= MaxQuota:
		SysError(fmt.Sprintf("quota conversion overflow: %g, clamped to %d", value, MaxQuota))
		return MaxQuota
	case value <= MinQuota:
		SysError(fmt.Sprintf("quota conversion underflow: %g, clamped to %d", value, MinQuota))
		return MinQuota
	default:
		return int(value)
	}
}
