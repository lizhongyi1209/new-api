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
	"strings"
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
		{name: "api host uses local storage", host: "api.o1key.cn", want: ImageStorageProviderLocal},
		{name: "api host with port", host: "api.o1key.cn:443", want: ImageStorageProviderLocal},
		{name: "api URL", host: "https://api.o1key.cn/v1/images/generations", want: ImageStorageProviderLocal},
		{name: "cf api cn host", host: "cf-api.o1key.cn", want: ImageStorageProviderR2},
		{name: "cf api com host", host: "cf-api.o1key.com", want: ImageStorageProviderR2},
		{name: "unknown host defaults to local", host: "example.com", want: ImageStorageProviderLocal},
		{name: "empty host defaults to local", host: "", want: ImageStorageProviderLocal},
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

	if got := SelectImageStorageProvider("img-local.example.com"); got != ImageStorageProviderLocal {
		t.Fatalf("custom local host selected %q, want %q", got, ImageStorageProviderLocal)
	}
	if got := SelectImageStorageProvider("img-api.example.com:8443"); got != ImageStorageProviderAliyunOSS {
		t.Fatalf("custom OSS host selected %q, want %q", got, ImageStorageProviderAliyunOSS)
	}
	if got := SelectImageStorageProvider("cf.example.com"); got != ImageStorageProviderR2 {
		t.Fatalf("custom R2 host selected %q, want %q", got, ImageStorageProviderR2)
	}
}

// DISABLE_ALIYUN_OSS 只把 OSS 分支改写为 R2，不得影响本地存储路由（2e7292635 回归）。
func TestSelectImageStorageProviderOSSKillSwitch(t *testing.T) {
	t.Setenv("LOCAL_STORAGE_HOSTS", "")
	t.Setenv("ALIYUN_OSS_STORAGE_HOSTS", "oss-api.example.com")
	t.Setenv("R2_STORAGE_HOSTS", "")
	t.Setenv("DISABLE_ALIYUN_OSS", "true")

	if got := SelectImageStorageProvider("oss-api.example.com"); got != ImageStorageProviderR2 {
		t.Fatalf("kill-switch should redirect OSS host to R2, got %q", got)
	}
	if got := SelectImageStorageProvider("api.o1key.cn"); got != ImageStorageProviderLocal {
		t.Fatalf("kill-switch must not affect local storage routing, got %q", got)
	}
}
