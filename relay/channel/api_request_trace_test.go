package channel

import (
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"sync/atomic"
	"testing"

	newapicommon "github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDoRequestAppliesConfiguredUpstreamHTTPTrace(t *testing.T) {
	var gotConnection atomic.Bool
	var wroteRequest atomic.Bool
	var gotFirstResponseByte atomic.Bool
	trace := &httptrace.ClientTrace{
		GotConn: func(httptrace.GotConnInfo) {
			gotConnection.Store(true)
		},
		WroteRequest: func(httptrace.WroteRequestInfo) {
			wroteRequest.Store(true)
		},
		GotFirstResponseByte: func() {
			gotFirstResponseByte.Store(true)
		},
	}

	gin.SetMode(gin.TestMode)
	c := &gin.Context{Request: httptest.NewRequest(http.MethodGet, "/", nil)}
	c.Set(newapicommon.UpstreamHTTPTraceKey, trace)
	req := httptest.NewRequest(http.MethodGet, "https://upstream.example", nil)
	tracedRequest := applyUpstreamHTTPTrace(c, req)
	appliedTrace := httptrace.ContextClientTrace(tracedRequest.Context())
	require.NotNil(t, appliedTrace)

	appliedTrace.GotConn(httptrace.GotConnInfo{})
	appliedTrace.WroteRequest(httptrace.WroteRequestInfo{})
	appliedTrace.GotFirstResponseByte()

	assert.True(t, gotConnection.Load())
	assert.True(t, wroteRequest.Load())
	assert.True(t, gotFirstResponseByte.Load())
}
