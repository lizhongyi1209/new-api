package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const onePixelPNGBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="

func TestValidateAsyncImageSizeRejectsPrivateURLBeforeHead(t *testing.T) {
	err := ValidateAsyncImageSize(&dto.AsyncImageRequest{
		Images: []string{"http://127.0.0.1/reference.png"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "不允许访问")
}

func TestValidateGenerateImageRequestValidatesExplicitImageInputs(t *testing.T) {
	tests := []struct {
		name    string
		input   dto.GenerateImageInput
		wantErr string
	}{
		{
			name: "valid inline data",
			input: dto.GenerateImageInput{InlineData: &dto.GenerateImageInlineData{
				MimeType: " image/png ",
				Data:     onePixelPNGBase64,
			}},
		},
		{
			name: "inline MIME mismatch",
			input: dto.GenerateImageInput{InlineData: &dto.GenerateImageInlineData{
				MimeType: "image/jpeg",
				Data:     onePixelPNGBase64,
			}},
			wantErr: "does not match",
		},
		{
			name: "inline data URL rejected",
			input: dto.GenerateImageInput{InlineData: &dto.GenerateImageInlineData{
				MimeType: "image/png",
				Data:     "data:image/png;base64," + onePixelPNGBase64,
			}},
			wantErr: "raw base64",
		},
		{
			name: "file URI requires HTTP",
			input: dto.GenerateImageInput{FileData: &dto.GenerateImageFileData{
				MimeType: "image/png",
				FileURI:  "file:///tmp/image.png",
			}},
			wantErr: "http(s)",
		},
		{
			name: "file URI must be public",
			input: dto.GenerateImageInput{FileData: &dto.GenerateImageFileData{
				MimeType: "image/png",
				FileURI:  "http://127.0.0.1/reference.png",
			}},
			wantErr: "not allowed",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := &dto.GenerateImageRequest{
				Model:  "nano-banana-pro",
				Prompt: "draw",
				Images: []dto.GenerateImageInput{test.input},
			}
			err := ValidateGenerateImageRequest(request)
			if test.wantErr == "" {
				require.NoError(t, err)
				require.NotNil(t, request.Images[0].InlineData)
				assert.Equal(t, "image/png", request.Images[0].InlineData.MimeType)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.wantErr)
		})
	}
}

func TestPrepareGenerateImageGeminiNativePassesKnownURLAsFileData(t *testing.T) {
	imageURL := "https://example.com/reference.jpg?signature=secret"
	nativeRequest, preparation, err := PrepareGenerateImageGeminiNative(
		context.Background(),
		&dto.AsyncImageRequest{Prompt: "draw"},
		[]dto.GenerateImageInput{{Value: &imageURL}},
		GeminiFileDataOptions{Enabled: true},
	)
	require.NoError(t, err)

	fileData := preparedGeminiImagePayload(t, nativeRequest, 1, "fileData")
	assert.Equal(t, "image/jpeg", fileData["mimeType"])
	assert.Equal(t, imageURL, fileData["fileUri"])
	assert.Equal(t, "legacy_url", preparation.ClientFormat)
	assert.Equal(t, "file_data", preparation.UpstreamFormat)
	assert.Equal(t, "passthrough", preparation.Conversion)
	assert.Equal(t, "none", preparation.Fallback)
}

func TestPrepareGenerateImageGeminiNativeKeepsChannelCapabilityIsolated(t *testing.T) {
	requestCount := 0
	downloadImage := func(_ string, maxSizeMB int) (string, string, error) {
		requestCount++
		assert.Equal(t, AsyncImageMaxURLSizeMB, maxSizeMB)
		return "image/png", onePixelPNGBase64, nil
	}
	imageURL := "https://example.com/reference.png"
	input := dto.GenerateImageInput{Value: &imageURL}

	enabledRequest, enabledPreparation, err := PrepareGenerateImageGeminiNative(
		context.Background(),
		&dto.AsyncImageRequest{Prompt: "draw"},
		[]dto.GenerateImageInput{input},
		GeminiFileDataOptions{Enabled: true, downloadImage: downloadImage},
	)
	require.NoError(t, err)
	preparedGeminiImagePayload(t, enabledRequest, 1, "fileData")
	assert.Equal(t, 0, requestCount)
	assert.Equal(t, "file_data", enabledPreparation.UpstreamFormat)

	disabledRequest, disabledPreparation, err := PrepareGenerateImageGeminiNative(
		context.Background(),
		&dto.AsyncImageRequest{Prompt: "draw"},
		[]dto.GenerateImageInput{input},
		GeminiFileDataOptions{downloadImage: downloadImage},
	)
	require.NoError(t, err)
	preparedGeminiImagePayload(t, disabledRequest, 1, "inlineData")
	assert.Equal(t, 1, requestCount)
	assert.Equal(t, "inline_data", disabledPreparation.UpstreamFormat)
	assert.Equal(t, "download_to_inline", disabledPreparation.Conversion)
}

func TestPrepareGenerateImageGeminiNativeStoresBase64AsTemporaryFile(t *testing.T) {
	temporaryRoot := t.TempDir()
	t.Setenv("TEMP_STORAGE_DIR", temporaryRoot)
	input := dto.GenerateImageInput{InlineData: &dto.GenerateImageInlineData{
		MimeType: "image/png",
		Data:     onePixelPNGBase64,
	}}

	nativeRequest, preparation, err := PrepareGenerateImageGeminiNative(
		context.Background(),
		&dto.AsyncImageRequest{Prompt: "draw"},
		[]dto.GenerateImageInput{input},
		GeminiFileDataOptions{Enabled: true},
	)
	require.NoError(t, err)

	fileData := preparedGeminiImagePayload(t, nativeRequest, 1, "fileData")
	fileURI, ok := fileData["fileUri"].(string)
	require.True(t, ok)
	assert.True(t, strings.HasPrefix(fileURI, "https://cf-api.o1key.com/tmp/input/"))
	assert.Equal(t, "image/png", fileData["mimeType"])
	assert.Equal(t, "local_cf_to_file_data", preparation.Conversion)
	assert.Equal(t, "file_data", preparation.UpstreamFormat)
	assert.Greater(t, preparation.InputBytes, int64(0))

	entries, err := os.ReadDir(filepath.Join(temporaryRoot, TemporaryInputCategory))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.True(t, strings.HasSuffix(entries[0].Name(), ".png"))
}

func TestPrepareGenerateImageGeminiNativeFallsBackOnStorageFailure(t *testing.T) {
	temporaryRoot := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(temporaryRoot, TemporaryInputCategory), []byte("blocked"), 0600))
	t.Setenv("TEMP_STORAGE_DIR", temporaryRoot)
	input := dto.GenerateImageInput{InlineData: &dto.GenerateImageInlineData{
		MimeType: "image/png",
		Data:     onePixelPNGBase64,
	}}

	nativeRequest, preparation, err := PrepareGenerateImageGeminiNative(
		context.Background(),
		&dto.AsyncImageRequest{Prompt: "draw"},
		[]dto.GenerateImageInput{input},
		GeminiFileDataOptions{Enabled: true},
	)
	require.NoError(t, err)

	inlineData := preparedGeminiImagePayload(t, nativeRequest, 1, "inlineData")
	assert.Equal(t, onePixelPNGBase64, inlineData["data"])
	assert.Equal(t, "storage_error", preparation.Fallback)
	assert.Equal(t, "inline_data", preparation.UpstreamFormat)
}

func preparedGeminiImagePayload(
	t *testing.T,
	nativeRequest map[string]interface{},
	partIndex int,
	payloadKey string,
) map[string]interface{} {
	t.Helper()
	contents, ok := nativeRequest["contents"].([]interface{})
	require.True(t, ok)
	require.Len(t, contents, 1)
	content, ok := contents[0].(map[string]interface{})
	require.True(t, ok)
	parts, ok := content["parts"].([]interface{})
	require.True(t, ok)
	require.Greater(t, len(parts), partIndex)
	part, ok := parts[partIndex].(map[string]interface{})
	require.True(t, ok)
	payload, ok := part[payloadKey].(map[string]interface{})
	require.True(t, ok)
	return payload
}
