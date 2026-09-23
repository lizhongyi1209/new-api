package billing_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSmokeTestExprKeepsUnconfiguredUsageBillingDisabled(t *testing.T) {
	for _, expression := range []string{
		`tier("task", u("seconds"))`,
		`tier("task", u(param("usage_key")))`,
		`true ? tier("tokens", p*2) : tier("task", u("seconds"))`,
	} {
		assert.ErrorContains(t, SmokeTestExpr(expression), "configured usage schema", expression)
	}
	assert.NoError(t, SmokeTestExpr(`tier("image", p*2 + img*5 + img_cr*0.1)`))
	assert.NoError(t, SmokeTestExpr(`tier("request", fixed(0))`))
	assert.Error(t, SmokeTestExpr(`tier("image", p*2 - img_cr)`))
}
