package service

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	newapicommon "github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateTextOtherInfoIncludesResponsesTimingInAdminInfo(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	ctx.Set("use_channel", []string{"441"})

	startTime := time.Unix(100, 0)
	timing := &relaycommon.ResponsesTimingAudit{
		UpstreamRequestBytes:   6_183_000,
		UpstreamRequestWriteMs: 20_000,
		UpstreamWaitMs:         5_000,
		UpstreamTotalMs:        37_000,
		UpstreamAttempts:       1,
		UpstreamStatus:         http.StatusOK,
	}
	info := &relaycommon.RelayInfo{
		StartTime:         startTime,
		FirstResponseTime: startTime.Add(26 * time.Second),
		ResponsesTiming:   timing,
		ChannelMeta:       &relaycommon.ChannelMeta{},
	}

	other := GenerateTextOtherInfo(ctx, info, 1, 1, 1, 0, 0, 0, -1)
	adminInfo, ok := other["admin_info"].(map[string]interface{})
	require.True(t, ok)
	assert.Same(t, timing, adminInfo["responses_timing"])
	assert.Equal(t, "/v1/responses", other["request_path"])

	encoded, err := newapicommon.Marshal(other)
	require.NoError(t, err)
	var decoded map[string]interface{}
	require.NoError(t, newapicommon.Unmarshal(encoded, &decoded))
	decodedAdmin, ok := decoded["admin_info"].(map[string]interface{})
	require.True(t, ok)
	decodedTiming, ok := decodedAdmin["responses_timing"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, float64(20_000), decodedTiming["upstream_request_write_ms"])
	assert.Equal(t, float64(http.StatusOK), decodedTiming["upstream_status"])
}
