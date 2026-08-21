package service

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/dto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const temporaryImageTestPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="

func TestUploadBase64ImageToTemporaryOutputStoresPublicImage(t *testing.T) {
	storageDir := t.TempDir()
	t.Setenv("TEMP_STORAGE_DIR", storageDir)
	t.Setenv("TEMP_STORAGE_PUBLIC_BASE_URL", "https://images.example.com/")

	publicURL, err := UploadBase64ImageToTemporaryOutput("image/jpeg", temporaryImageTestPNG, "")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(publicURL, "https://images.example.com/tmp/output/"))
	assert.True(t, strings.HasSuffix(publicURL, ".png"))

	filename := strings.TrimPrefix(publicURL, "https://images.example.com/tmp/output/")
	stored, err := os.ReadFile(filepath.Join(storageDir, TemporaryImageOutputCategory, filename))
	require.NoError(t, err)
	want, err := base64.StdEncoding.DecodeString(temporaryImageTestPNG)
	require.NoError(t, err)
	assert.Equal(t, want, stored)
}

func TestUploadBase64ImageWithOutputStrategySelectsTemporaryPublicDomain(t *testing.T) {
	t.Setenv("TEMP_STORAGE_DIR", t.TempDir())
	t.Setenv("TEMP_STORAGE_PUBLIC_BASE_URL", "https://legacy.example.com")

	tests := []struct {
		name       string
		strategy   string
		wantPrefix string
	}{
		{name: "legacy configurable domain", strategy: dto.ImageOutputStrategyLocalTemp, wantPrefix: "https://legacy.example.com/tmp/output/"},
		{name: "cloudflare", strategy: dto.ImageOutputStrategyLocalTempCF, wantPrefix: "https://cf-api.o1key.com/tmp/output/"},
		{name: "esa", strategy: dto.ImageOutputStrategyLocalTempESA, wantPrefix: "https://api.o1key.cn/tmp/output/"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			publicURL, err := UploadBase64ImageWithOutputStrategy("image/png", temporaryImageTestPNG, test.strategy, "request.example.com")
			require.NoError(t, err)
			assert.True(t, strings.HasPrefix(publicURL, test.wantPrefix))
		})
	}
}

func TestUploadBase64ImageToTemporaryOutputDefaultsToCloudflare(t *testing.T) {
	t.Setenv("TEMP_STORAGE_DIR", t.TempDir())
	t.Setenv("TEMP_STORAGE_PUBLIC_BASE_URL", "")
	t.Setenv("LOCAL_PUBLIC_BASE_URL", "")

	publicURL, err := UploadBase64ImageToTemporaryOutput("image/png", temporaryImageTestPNG, "")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(publicURL, "https://cf-api.o1key.com/tmp/output/"))
}

func TestUploadBase64ImageToTemporaryOutputRejectsNonImageData(t *testing.T) {
	t.Setenv("TEMP_STORAGE_DIR", t.TempDir())

	_, err := UploadBase64ImageToTemporaryOutput(
		"image/png",
		base64.StdEncoding.EncodeToString([]byte("not an image")),
		"",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a supported image")
}

func TestOpenTemporaryOutputImageRejectsExpiredFile(t *testing.T) {
	storageDir := t.TempDir()
	t.Setenv("TEMP_STORAGE_DIR", storageDir)
	t.Setenv("TEMP_STORAGE_PUBLIC_BASE_URL", "https://images.example.com")

	publicURL, err := UploadBase64ImageToTemporaryOutput("image/png", temporaryImageTestPNG, "")
	require.NoError(t, err)
	filename := strings.TrimPrefix(publicURL, "https://images.example.com/tmp/output/")
	path := filepath.Join(storageDir, TemporaryImageOutputCategory, filename)
	now := time.Now()
	expiredAt := now.Add(-TemporaryImageRetention)
	require.NoError(t, os.Chtimes(path, expiredAt, expiredAt))

	file, _, err := OpenTemporaryOutputImage(filename, now)
	if file != nil {
		_ = file.Close()
	}
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrTemporaryImageExpired)
	_, statErr := os.Stat(path)
	assert.True(t, errors.Is(statErr, os.ErrNotExist))
}

func TestCleanupExpiredTemporaryOutputImagesPreservesFreshFiles(t *testing.T) {
	storageDir := t.TempDir()
	t.Setenv("TEMP_STORAGE_DIR", storageDir)
	outputDir := filepath.Join(storageDir, TemporaryImageOutputCategory)
	require.NoError(t, os.MkdirAll(outputDir, 0755))

	now := time.Now()
	freshPath := filepath.Join(outputDir, "8045b62c-39b6-4a7d-a75e-15fdb83420c2.png")
	expiredPath := filepath.Join(outputDir, "20e21317-4a0a-4379-b438-3141d4d33af0.png")
	require.NoError(t, os.WriteFile(freshPath, []byte("fresh"), 0644))
	require.NoError(t, os.WriteFile(expiredPath, []byte("expired"), 0644))
	expiredAt := now.Add(-TemporaryImageRetention - time.Minute)
	require.NoError(t, os.Chtimes(expiredPath, expiredAt, expiredAt))

	stats, err := CleanupExpiredTemporaryOutputImages(now)
	require.NoError(t, err)
	assert.Equal(t, 1, stats.Deleted)
	assert.Equal(t, int64(len("expired")), stats.Bytes)
	require.FileExists(t, freshPath)
	assert.NoFileExists(t, expiredPath)
}
