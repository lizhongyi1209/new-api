package serviceinference

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// decodeWrappedMessage extracts the human-readable message from the JSON envelope
// that WrapError stores in dto.TaskError.Message (`{"error":{"code","message","type"}}`).
func decodeWrappedMessage(t *testing.T, taskErrMessage string) string {
	t.Helper()
	var envelope struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
	}
	require.NoError(t, common.UnmarshalJsonStr(taskErrMessage, &envelope))
	return envelope.Error.Message
}

// TestWrapErrorProxyWrappedInvalidDurationIsNonRetryableClientError locks in the fix
// for the production incident where a client-side invalid `duration` (an upstream
// HTTP 400 InvalidParameter) was wrapped by an intermediate proxy as a 502
// proxy_error, then masked by the task retry loop as "可用渠道不存在". The proxy
// envelope carries an empty top-level error code and keeps the real error as an
// escaped string inside error.message, so structured parsing alone misses it.
// The classifier MUST still recognize it as a deterministic 400 client error that
// is not retried (LocalError=true) rather than a retryable 5xx upstream failure.
func TestWrapErrorProxyWrappedInvalidDurationIsNonRetryableClientError(t *testing.T) {
	// Exact shape observed in production (status_code=502 body):
	// the InvalidParameter/duration error is buried, escaped, inside error.message.
	upstreamBody := `{"error":{"message":"Failed to submit video generation job: Upstream submit failed (400): {\"code\":\"fail_to_fetch_task\",\"message\":\"{\\\"error\\\":{\\\"code\\\":\\\"InvalidParameter\\\",\\\"message\\\":\\\"the parameter duration specified in the request is not valid for model dreamina--2-0 in r2v\\\"}}\"}","type":"proxy_error"}}`

	taskErr := WrapError(fmt.Errorf("%s", upstreamBody), http.StatusBadGateway)
	require.NotNil(t, taskErr)

	// Non-retryable client error: must be a 400 marked LocalError so the shared task
	// retry loop stops instead of exhausting channels and reporting "可用渠道不存在".
	assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
	assert.True(t, taskErr.LocalError, "invalid duration is a deterministic client error and must not be retried")
	assert.Equal(t, ErrorCodeParamDurationInvalid, taskErr.Code)

	// Client-facing message names the duration parameter and the model, not a proxy_error.
	message := decodeWrappedMessage(t, taskErr.Message)
	assert.Contains(t, message, "时长")
	assert.Contains(t, message, "dreamina--2-0")
}

// TestWrapErrorGenuineUpstreamFailureStaysRetryable guards the opposite direction:
// a real transient upstream failure (no client-parameter signature) must remain a
// retryable, non-local 5xx so multi-channel retry still works.
func TestWrapErrorGenuineUpstreamFailureStaysRetryable(t *testing.T) {
	taskErr := WrapError(fmt.Errorf("upstream gateway timeout while contacting inference backend"), http.StatusBadGateway)
	require.NotNil(t, taskErr)

	assert.False(t, taskErr.LocalError, "a generic upstream failure must stay retryable")
	assert.Equal(t, http.StatusBadGateway, taskErr.StatusCode)
	assert.Equal(t, ErrorCodeUpstreamError, taskErr.Code)
}

// TestWrapErrorFailToFetchTaskWrappingInvalidResolution covers the structured
// fail_to_fetch_task wrapper path (top-level code populated) carrying an invalid
// resolution, ensuring it is likewise classified as a non-retryable 400.
func TestWrapErrorFailToFetchTaskWrappingInvalidResolution(t *testing.T) {
	upstreamBody := `{"code":"fail_to_fetch_task","message":"{\"error\":{\"code\":\"InvalidParameter\",\"message\":\"the parameter resolution specified in the request is not valid for model dreamina--2-0\"}}"}`

	taskErr := WrapError(fmt.Errorf("%s", upstreamBody), http.StatusBadGateway)
	require.NotNil(t, taskErr)

	assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
	assert.True(t, taskErr.LocalError)
	assert.Equal(t, ErrorCodeParamResolutionInvalid, taskErr.Code)
}
