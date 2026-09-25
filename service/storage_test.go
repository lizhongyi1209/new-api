package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrepareImageUploadPreservesOriginalBytesAndFormat(t *testing.T) {
	const pngBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="
	wantBytes, err := base64.StdEncoding.DecodeString(pngBase64)
	require.NoError(t, err)

	uploadBytes, ext, contentType, err := prepareImageUpload("image/jpeg", pngBase64)
	require.NoError(t, err)
	assert.Equal(t, wantBytes, uploadBytes)
	assert.Equal(t, "png", ext)
	assert.Equal(t, "image/png", contentType)
}

func TestPrepareImageUploadPreservesJPEGBytesAndFormat(t *testing.T) {
	var jpegBuffer bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 12, G: 34, B: 56, A: 255})
	require.NoError(t, jpeg.Encode(&jpegBuffer, img, &jpeg.Options{Quality: 95}))

	wantBytes := jpegBuffer.Bytes()
	uploadBytes, ext, contentType, err := prepareImageUpload(
		"image/jpeg",
		base64.StdEncoding.EncodeToString(wantBytes),
	)

	require.NoError(t, err)
	assert.Equal(t, wantBytes, uploadBytes)
	assert.Equal(t, "jpg", ext)
	assert.Equal(t, "image/jpeg", contentType)
}

func TestPrepareImageUploadRejectsInvalidImageInsteadOfUsingMIMEFallback(t *testing.T) {
	invalidBase64 := base64.StdEncoding.EncodeToString([]byte("not an image"))

	_, _, _, err := prepareImageUpload("image/png", invalidBase64)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported or invalid image data")
}

func TestUploadImageBytesToOSSPreservesFormatAndSetsImmutableCache(t *testing.T) {
	const pngBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="
	pngBytes, err := base64.StdEncoding.DecodeString(pngBase64)
	require.NoError(t, err)

	var requestPath string
	var requestContentType string
	var requestCacheControl string
	var requestBody []byte
	var requestBodyErr error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		requestContentType = r.Header.Get("Content-Type")
		requestCacheControl = r.Header.Get("Cache-Control")
		requestBody, requestBodyErr = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	previousClient := ossClient
	previousPresignClient := ossPresignClient
	ossClient = nil
	ossPresignClient = nil
	t.Cleanup(func() {
		ossClient = previousClient
		ossPresignClient = previousPresignClient
	})
	t.Setenv("ALIYUN_OSS_ACCESS_KEY_ID", "test-access-key")
	t.Setenv("ALIYUN_OSS_ACCESS_KEY_SECRET", "test-secret-key")
	t.Setenv("ALIYUN_OSS_REGION", "test-region")
	t.Setenv("ALIYUN_OSS_ENDPOINT", server.URL)
	t.Setenv("ALIYUN_OSS_FORCE_PATH_STYLE", "true")
	t.Setenv("ALIYUN_OSS_BUCKET", "test-bucket")
	t.Setenv("ALIYUN_OSS_PUBLIC_BASE_URL", "https://images.example.com")

	publicURL, err := UploadImageBytesToOSSContext(context.Background(), pngBytes)

	require.NoError(t, err)
	require.NoError(t, requestBodyErr)
	assert.True(t, strings.HasPrefix(requestPath, "/test-bucket/output/"), requestPath)
	assert.True(t, strings.HasSuffix(requestPath, ".png"), requestPath)
	assert.Equal(t, "image/png", requestContentType)
	assert.Equal(t, generatedImageCacheControl, requestCacheControl)
	assert.Equal(t, pngBytes, requestBody)
	assert.True(t, strings.HasPrefix(publicURL, "https://images.example.com/output/"), publicURL)
	assert.True(t, strings.HasSuffix(publicURL, ".png"), publicURL)
}

func TestSelectImageStorageProviderDefaultHosts(t *testing.T) {
	t.Setenv("LOCAL_STORAGE_HOSTS", "")
	t.Setenv("ALIYUN_OSS_STORAGE_HOSTS", "")
	t.Setenv("R2_STORAGE_HOSTS", "")
	t.Setenv("DISABLE_ALIYUN_OSS", "")

	tests := []struct {
		name string
		host string
		want string
	}{
		{name: "api host uses OSS", host: "api.o1key.cn", want: ImageStorageProviderAliyunOSS},
		{name: "api host with port", host: "api.o1key.cn:443", want: ImageStorageProviderAliyunOSS},
		{name: "api URL", host: "https://api.o1key.cn/v1/images/generations", want: ImageStorageProviderAliyunOSS},
		{name: "cf api cn host", host: "cf-api.o1key.cn", want: ImageStorageProviderR2},
		{name: "cf api com host", host: "cf-api.o1key.com", want: ImageStorageProviderR2},
		{name: "unknown host defaults to OSS", host: "example.com", want: ImageStorageProviderAliyunOSS},
		{name: "empty host defaults to OSS", host: "", want: ImageStorageProviderAliyunOSS},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := SelectImageStorageProvider(test.host); got != test.want {
				t.Fatalf("SelectImageStorageProvider(%q) = %q, want %q", test.host, got, test.want)
			}
		})
	}
}

func TestSelectImageStorageProviderCustomHosts(t *testing.T) {
	t.Setenv("LOCAL_STORAGE_HOSTS", "img-local.example.com")
	t.Setenv("ALIYUN_OSS_STORAGE_HOSTS", "api.example.com, https://img-api.example.com")
	t.Setenv("R2_STORAGE_HOSTS", "cf.example.com")
	t.Setenv("DISABLE_ALIYUN_OSS", "")

	if got := SelectImageStorageProvider("img-local.example.com"); got != ImageStorageProviderAliyunOSS {
		t.Fatalf("former local host selected %q, want %q", got, ImageStorageProviderAliyunOSS)
	}
	if got := SelectImageStorageProvider("img-api.example.com:8443"); got != ImageStorageProviderAliyunOSS {
		t.Fatalf("custom OSS host selected %q, want %q", got, ImageStorageProviderAliyunOSS)
	}
	if got := SelectImageStorageProvider("cf.example.com"); got != ImageStorageProviderR2 {
		t.Fatalf("custom R2 host selected %q, want %q", got, ImageStorageProviderR2)
	}
}

// DISABLE_ALIYUN_OSS redirects persistent uploads to R2, including former local hosts.
func TestSelectImageStorageProviderOSSKillSwitch(t *testing.T) {
	t.Setenv("LOCAL_STORAGE_HOSTS", "")
	t.Setenv("ALIYUN_OSS_STORAGE_HOSTS", "oss-api.example.com")
	t.Setenv("R2_STORAGE_HOSTS", "")
	t.Setenv("DISABLE_ALIYUN_OSS", "true")

	if got := SelectImageStorageProvider("oss-api.example.com"); got != ImageStorageProviderR2 {
		t.Fatalf("kill-switch should redirect OSS host to R2, got %q", got)
	}
	if got := SelectImageStorageProvider("api.o1key.cn"); got != ImageStorageProviderR2 {
		t.Fatalf("kill-switch must redirect former local storage to R2, got %q", got)
	}
}

func TestPersistentUploadWritesToOSSWithoutLocalFile(t *testing.T) {
	var requestPath, requestContentType string
	var requestBody []byte
	var requestMu sync.Mutex
	failUpload := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestMu.Lock()
		defer requestMu.Unlock()
		if failUpload {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		requestPath = r.URL.Path
		requestContentType = r.Header.Get("Content-Type")
		requestBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	previousClient, previousPresignClient := ossClient, ossPresignClient
	ossClient, ossPresignClient = nil, nil
	t.Cleanup(func() { ossClient, ossPresignClient = previousClient, previousPresignClient })
	t.Setenv("ALIYUN_OSS_ACCESS_KEY_ID", "test-access-key")
	t.Setenv("ALIYUN_OSS_ACCESS_KEY_SECRET", "test-secret-key")
	t.Setenv("ALIYUN_OSS_REGION", "test-region")
	t.Setenv("ALIYUN_OSS_ENDPOINT", server.URL)
	t.Setenv("ALIYUN_OSS_FORCE_PATH_STYLE", "true")
	t.Setenv("ALIYUN_OSS_BUCKET", "test-bucket")
	t.Setenv("ALIYUN_OSS_PUBLIC_BASE_URL", "https://images.example.com")
	localDir := t.TempDir()
	t.Setenv("LOCAL_UPLOAD_DIR", localDir)
	t.Setenv("DISABLE_ALIYUN_OSS", "false")

	publicURL, key, err := UploadPersistentFile(context.Background(), "example.png", "image/png", []byte("image bytes"))
	require.NoError(t, err)
	requestMu.Lock()
	uploadPath, uploadContentType, uploadedBody := requestPath, requestContentType, requestBody
	requestMu.Unlock()
	assert.True(t, strings.HasPrefix(key, "uploads/oss/"), key)
	assert.True(t, strings.HasSuffix(key, "_example.png"), key)
	assert.Equal(t, "/test-bucket/"+key, uploadPath)
	assert.Equal(t, "image/png", uploadContentType)
	assert.Equal(t, []byte("image bytes"), uploadedBody)
	assert.Equal(t, "https://images.example.com/"+key, publicURL)
	presigned, err := GeneratePresignedUploadURLForHost("unmapped.example.com", "example.png", "image/png", 11)
	require.NoError(t, err)
	ossPresigned, ok := presigned.(*OSSPresignResult)
	require.True(t, ok)
	assert.Equal(t, ImageStorageProviderAliyunOSS, ossPresigned.Provider)
	assert.Equal(t, http.MethodPut, ossPresigned.Method)
	assert.True(t, strings.HasPrefix(ossPresigned.ObjectKey, "uploads/oss/"))
	assert.Equal(t, "https://images.example.com/"+ossPresigned.ObjectKey, ossPresigned.PublicURL)

	entries, err := os.ReadDir(localDir)
	require.NoError(t, err)
	assert.Empty(t, entries)

	requestMu.Lock()
	failUpload = true
	requestMu.Unlock()
	failedURL, failedKey, err := UploadPersistentFile(context.Background(), "failed.png", "image/png", []byte("image bytes"))
	require.Error(t, err)
	assert.Empty(t, failedURL)
	assert.Empty(t, failedKey)
	entries, err = os.ReadDir(localDir)
	require.NoError(t, err)
	assert.Empty(t, entries)
}
