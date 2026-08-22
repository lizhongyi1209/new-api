package controller

import (
	"bytes"
	"encoding/base64"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUploadAndServeTemporaryInputAttachment(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("TEMP_STORAGE_DIR", t.TempDir())
	pngBytes, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	require.NoError(t, err)

	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)
	filePart, err := writer.CreateFormFile("file", "reference.png")
	require.NoError(t, err)
	_, err = filePart.Write(pngBytes)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	router := gin.New()
	router.POST("/v1/o1key/uploads", UploadTemporaryInputAttachment)
	router.GET("/tmp/input/:filename", ServeTemporaryInputAttachment)
	uploadRecorder := httptest.NewRecorder()
	uploadRequest := httptest.NewRequest(http.MethodPost, "/v1/o1key/uploads", &requestBody)
	uploadRequest.Host = "cf-api.o1key.com"
	uploadRequest.Header.Set("Content-Type", writer.FormDataContentType())
	router.ServeHTTP(uploadRecorder, uploadRequest)

	require.Equal(t, http.StatusOK, uploadRecorder.Code)
	var payload map[string]any
	require.NoError(t, common.Unmarshal(uploadRecorder.Body.Bytes(), &payload))
	publicURL, ok := payload["url"].(string)
	require.True(t, ok)
	assert.True(t, strings.HasPrefix(publicURL, "https://cf-api.o1key.com/tmp/input/"))
	assert.Equal(t, "reference.png", payload["filename"])
	assert.Equal(t, "image/png", payload["content_type"])
	assert.Equal(t, float64(len(pngBytes)), payload["size"])
	assert.NotZero(t, payload["expires_at"])

	readRecorder := httptest.NewRecorder()
	readRequest := httptest.NewRequest(http.MethodGet, strings.TrimPrefix(publicURL, "https://cf-api.o1key.com"), nil)
	router.ServeHTTP(readRecorder, readRequest)
	assert.Equal(t, http.StatusOK, readRecorder.Code)
	assert.Equal(t, "image/png", readRecorder.Header().Get("Content-Type"))
	assert.Equal(t, "nosniff", readRecorder.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, pngBytes, readRecorder.Body.Bytes())
}

func TestUploadTemporaryInputAttachmentRejectsUnsupportedType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("TEMP_STORAGE_DIR", t.TempDir())

	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)
	filePart, err := writer.CreateFormFile("file", "page.html")
	require.NoError(t, err)
	_, err = filePart.Write([]byte("<!doctype html><script>alert(1)</script>"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/o1key/uploads", &requestBody)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	context, _ := gin.CreateTestContext(recorder)
	context.Request = request
	UploadTemporaryInputAttachment(context)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "unsupported attachment type")
}

func TestServeTemporaryInputDocumentForcesDownload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("TEMP_STORAGE_DIR", t.TempDir())
	attachment, err := service.StoreTemporaryInputAttachment(
		strings.NewReader("%PDF-1.7\n% temporary upload\n"),
		"brief.pdf",
		"cf-api.o1key.com",
	)
	require.NoError(t, err)

	router := gin.New()
	router.GET("/tmp/input/:filename", ServeTemporaryInputAttachment)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, strings.TrimPrefix(attachment.URL, "https://cf-api.o1key.com"), nil)
	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "application/pdf", recorder.Header().Get("Content-Type"))
	assert.Contains(t, recorder.Header().Get("Content-Disposition"), "attachment")
	assert.Contains(t, recorder.Header().Get("Cache-Control"), "must-revalidate")
}
