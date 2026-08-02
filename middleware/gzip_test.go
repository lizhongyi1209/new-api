package middleware

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecompressRequestMiddlewareSupportsZstd(t *testing.T) {
	const payload = `{"model":"test","stream":false}`
	encoder, err := zstd.NewWriter(nil)
	require.NoError(t, err)
	compressed := encoder.EncodeAll([]byte(payload), nil)
	encoder.Close()

	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(compressed))
	request.Header.Set("Content-Encoding", "zstd")
	response := httptest.NewRecorder()

	router := gin.New()
	router.Use(DecompressRequestMiddleware())
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		body, readErr := io.ReadAll(c.Request.Body)
		require.NoError(t, readErr)
		assert.Empty(t, c.GetHeader("Content-Encoding"))
		assert.Equal(t, payload, string(body))
		c.Status(http.StatusNoContent)
	})

	router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusNoContent, response.Code)
}

func TestDecompressRequestMiddlewareLimitsDecompressedZstdBody(t *testing.T) {
	originalLimit := constant.MaxRequestBodyMB
	constant.MaxRequestBodyMB = 1
	t.Cleanup(func() { constant.MaxRequestBodyMB = originalLimit })

	encoder, err := zstd.NewWriter(nil)
	require.NoError(t, err)
	compressed := encoder.EncodeAll(bytes.Repeat([]byte("a"), (1<<20)+1), nil)
	encoder.Close()

	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(compressed))
	request.Header.Set("Content-Encoding", "zstd")
	response := httptest.NewRecorder()

	router := gin.New()
	router.Use(DecompressRequestMiddleware())
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		_, readErr := io.ReadAll(c.Request.Body)
		require.Error(t, readErr)
		c.Status(http.StatusRequestEntityTooLarge)
	})

	router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusRequestEntityTooLarge, response.Code)
}
