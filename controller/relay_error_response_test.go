package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	ktypes "github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRespondRelayErrorReturnsRawUpstreamResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	rawBody := []byte(`{"error":{"message":"invalid response_format","param":"response_format"}}`)
	relayErr := types.NewOpenAIError(errors.New("normalized internal error"), types.ErrorCodeBadResponseStatusCode, http.StatusBadGateway)
	relayErr.SetRawUpstreamResponse(http.StatusBadRequest, "application/json", rawBody)

	respondRelayError(c, ktypes.RelayFormatOpenAI, nil, relayErr, "request-id-must-not-be-appended")

	result := recorder.Result()
	defer result.Body.Close()
	require.Equal(t, http.StatusBadRequest, result.StatusCode)
	assert.Equal(t, "application/json", result.Header.Get("Content-Type"))
	assert.Equal(t, string(rawBody), recorder.Body.String())
}

func TestRespondRelayErrorMasksConfiguredAddressInRawUpstreamResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, "https://private.example.com")
	rawBody := []byte(`{"error":{"message":"request to https://private.example.com/v1/images failed; see https://docs.example.org/errors"}}`)
	relayErr := types.NewOpenAIError(errors.New("normalized internal error"), types.ErrorCodeBadResponseStatusCode, http.StatusBadGateway)
	relayErr.SetRawUpstreamResponse(http.StatusBadRequest, "application/json", rawBody)

	respondRelayError(c, ktypes.RelayFormatOpenAI, nil, relayErr, "request-id")

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.JSONEq(t, `{"error":{"message":"request to https://***.com/***/*** failed; see https://docs.example.org/errors"}}`, recorder.Body.String())
}
