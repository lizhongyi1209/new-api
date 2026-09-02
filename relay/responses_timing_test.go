package relay

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	newapicommon "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnsureResponsesTimingScopesAuditToPublicResponsesEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	requestBody := `{"model":"gpt-test","input":"hello"}`
	requestReceivedAt := time.Unix(100, 0)
	requestBodyReadyAt := requestReceivedAt.Add(250 * time.Millisecond)

	t.Run("records exact request body size and receive duration", func(t *testing.T) {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(requestBody))
		storage, err := newapicommon.GetBodyStorage(ctx)
		require.NoError(t, err)
		t.Cleanup(func() { _ = storage.Close() })
		newapicommon.SetContextKey(ctx, constant.ContextKeyRequestReceivedTime, requestReceivedAt)
		newapicommon.SetContextKey(ctx, constant.ContextKeyRequestBodyReadyTime, requestBodyReadyAt)

		info := &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeResponses}
		ensureResponsesTiming(ctx, info)

		require.NotNil(t, info.ResponsesTiming)
		assert.Equal(t, int64(len(requestBody)), info.ResponsesTiming.ClientRequestBytes)
		assert.Equal(t, 250.0, info.ResponsesTiming.ClientBodyReceiveMs)
	})

	t.Run("does not record compact endpoint", func(t *testing.T) {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", strings.NewReader(requestBody))
		info := &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeResponsesCompact}

		ensureResponsesTiming(ctx, info)

		assert.Nil(t, info.ResponsesTiming)
	})
}
