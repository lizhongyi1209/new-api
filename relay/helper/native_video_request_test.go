package helper

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNativeVideoBillingFreezesOnlyReferencedScalarConditions(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)
	c.Request.Header.Set("X-Billing-Tier", "standard")
	c.Set("task_request", relaycommon.TaskSubmitReq{
		Prompt: "private prompt", Duration: 5, EffectiveResolution: "720p",
		Metadata: map[string]any{"resolution": "720p", "generate_audio": false, "content": []any{map[string]any{"type": "image_url", "image_url": "https://example.com/private.png"}}},
	})
	expression := `param("duration") == 5 && param("resolution") == "720p" && param("generate_audio") == false && header("x-billing-tier") == "standard" ? tier("original", u("tokens") * 37 / 1000000) : tier("other", u("tokens") * 99 / 1000000)`
	input, err := FreezeTaskBillingExprRequestInput(c, expression)
	require.NoError(t, err)
	assert.Empty(t, input.Body)
	assert.Equal(t, map[string]any{"duration": float64(5), "resolution": "720p", "generate_audio": false}, input.Params)
	assert.Equal(t, map[string]string{"x-billing-tier": "standard"}, input.Headers)
	input.Usage = map[string]any{"tokens": 100000}
	cost, trace, err := billingexpr.RunExprWithRequest(expression, billingexpr.TokenParams{}, input)
	require.NoError(t, err)
	assert.InDelta(t, 3.7, cost, 0.000000001)
	assert.Equal(t, "original", trace.MatchedTier)
	for _, expression := range []string{
		`tier("bad", u("tokens") * param("content.0.image_url"))`,
		`tier("bad", u("tokens") * param("video_url"))`,
		`tier("bad", u("tokens") * param("prompt"))`,
		`tier("bad", u("tokens") * header("Authorization"))`,
	} {
		_, err := FreezeTaskBillingExprRequestInput(c, expression)
		assert.Error(t, err)
	}
}
