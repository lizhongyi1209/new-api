package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestIDRecordsRequestReceiptForDownstreamCorrelation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestId())
	router.GET("/trace", func(c *gin.Context) {
		receivedAt := common.GetContextKeyTime(c, constant.ContextKeyRequestReceivedTime)
		requestID := c.GetString(common.RequestIdKey)
		contextRequestID, _ := c.Request.Context().Value(common.RequestIdKey).(string)

		require.False(t, receivedAt.IsZero())
		assert.NotEmpty(t, requestID)
		assert.Equal(t, requestID, contextRequestID)
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/trace", nil))

	assert.Equal(t, http.StatusNoContent, recorder.Code)
	assert.NotEmpty(t, recorder.Header().Get(common.RequestIdKey))
}
