package controller

import (
	"bytes"
	"encoding/base64"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUploadTemporaryInputAttachmentReturnsOSSURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	storageDir := t.TempDir()
	t.Setenv("TEMP_STORAGE_DIR", storageDir)
	uploaded := make(chan []byte, 1)
	ossServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || !strings.HasPrefix(r.URL.Path, "/test-bucket/tmp/input/") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		uploaded <- body
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(ossServer.Close)
	t.Setenv("ALIYUN_OSS_ACCESS_KEY_ID", "test-access-key")
	t.Setenv("ALIYUN_OSS_ACCESS_KEY_SECRET", "test-secret-key")
	t.Setenv("ALIYUN_OSS_REGION", "test-region")
	t.Setenv("ALIYUN_OSS_ENDPOINT", ossServer.URL)
	t.Setenv("ALIYUN_OSS_FORCE_PATH_STYLE", "true")
	t.Setenv("ALIYUN_OSS_BUCKET", "test-bucket")
	t.Setenv("ALIYUN_OSS_PUBLIC_BASE_URL", "https://media.example.com")
	t.Setenv("DISABLE_ALIYUN_OSS", "false")
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
	assert.True(t, strings.HasPrefix(publicURL, "https://media.example.com/tmp/input/"))
	assert.Equal(t, "reference.png", payload["filename"])
	assert.Equal(t, "image/png", payload["content_type"])
	assert.Equal(t, float64(len(pngBytes)), payload["size"])
	assert.NotZero(t, payload["expires_at"])
	assert.Equal(t, pngBytes, <-uploaded)
	assert.NoDirExists(t, filepath.Join(storageDir, service.TemporaryInputCategory))
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

func TestUploadTemporaryInputAttachmentRejectsMultipleFiles(t *testing.T) {
	gin.SetMode(gin.TestMode)
	pngBytes, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	require.NoError(t, err)

	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)
	for _, name := range []string{"first.png", "second.png"} {
		filePart, err := writer.CreateFormFile("file", name)
		require.NoError(t, err)
		_, err = filePart.Write(pngBytes)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/o1key/uploads", &requestBody)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	context, _ := gin.CreateTestContext(recorder)
	context.Request = request
	UploadTemporaryInputAttachment(context)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.JSONEq(t, `{"error":"multiple files are not supported"}`, recorder.Body.String())
}

func TestUploadTemporaryInputAttachmentRejectsFileOver20MiB(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)
	filePart, err := writer.CreateFormFile("file", "reference.png")
	require.NoError(t, err)
	_, err = filePart.Write(make([]byte, service.TemporaryInputMaxFileBytes+1))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/o1key/uploads", &requestBody)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	context, _ := gin.CreateTestContext(recorder)
	context.Request = request
	UploadTemporaryInputAttachment(context)

	assert.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "20 MiB")
}

func TestServeTemporaryInputDocumentForcesDownload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	storageDir := t.TempDir()
	t.Setenv("TEMP_STORAGE_DIR", storageDir)
	filename := "8045b62c-39b6-4a7d-a75e-15fdb83420c2.pdf"
	inputDir := filepath.Join(storageDir, service.TemporaryInputCategory)
	require.NoError(t, os.MkdirAll(inputDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(inputDir, filename), []byte("%PDF-1.7\n% temporary upload\n"), 0644))

	router := gin.New()
	router.GET("/tmp/input/:filename", ServeTemporaryInputAttachment)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/tmp/input/"+filename, nil)
	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "application/pdf", recorder.Header().Get("Content-Type"))
	assert.Contains(t, recorder.Header().Get("Content-Disposition"), "attachment")
	assert.Contains(t, recorder.Header().Get("Cache-Control"), "must-revalidate")
}
