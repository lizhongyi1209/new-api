package common

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQuotaFromFloatSaturatesBillingProducts(t *testing.T) {
	assert.Equal(t, 42, QuotaFromFloat(42.9))
	assert.Equal(t, MaxQuota, QuotaFromFloat(math.Inf(1)))
	assert.Equal(t, MinQuota, QuotaFromFloat(math.Inf(-1)))
	assert.Equal(t, 0, QuotaFromFloat(math.NaN()))
}
