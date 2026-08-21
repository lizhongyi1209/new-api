package controller

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServeTemporaryOutputImageReturnsStoredImage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("TEMP_STORAGE_DIR", t.TempDir())
	t.Setenv("TEMP_STORAGE_PUBLIC_BASE_URL", "https://api.o1key.cn")
	const pngBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="

	publicURL, err := service.UploadBase64ImageToTemporaryOutput("image/png", pngBase64, "")
	require.NoError(t, err)
	requestPath := strings.TrimPrefix(publicURL, "https://api.o1key.cn")

	router := gin.New()
	router.GET("/tmp/output/:filename", ServeTemporaryOutputImage)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, requestPath, nil)
	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "image/png", recorder.Header().Get("Content-Type"))
	assert.Contains(t, recorder.Header().Get("Cache-Control"), "must-revalidate")
	want, err := base64.StdEncoding.DecodeString(pngBase64)
	require.NoError(t, err)
	assert.Equal(t, want, recorder.Body.Bytes())
}

func TestServeTemporaryOutputImageRejectsInvalidFilename(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("TEMP_STORAGE_DIR", t.TempDir())
	router := gin.New()
	router.GET("/tmp/output/:filename", ServeTemporaryOutputImage)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/tmp/output/not-an-image.png", nil)
	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusNotFound, recorder.Code)
}
