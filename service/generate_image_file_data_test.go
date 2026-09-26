package service

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const onePixelPNGBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="

func TestValidateAsyncImageReferencesRejectsPrivateURL(t *testing.T) {
	err := ValidateAsyncImageReferences(&dto.AsyncImageRequest{
		Images: []string{"http://127.0.0.1/reference.png"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "不允许访问")
}

func TestValidateAsyncImageReferencesAllowsFormerRawSizeLimit(t *testing.T) {
	firstImage := base64.StdEncoding.EncodeToString(make([]byte, 7*1024*1024))
	secondImage := base64.StdEncoding.EncodeToString(make([]byte, 7*1024*1024+1))

	err := ValidateAsyncImageReferences(&dto.AsyncImageRequest{
		Images: []string{firstImage, secondImage},
	})
	require.NoError(t, err)
}

func TestPrepareGenerateImageGeminiNativeUsesFinalSerializedBodyLimit(t *testing.T) {
	firstImage := base64.StdEncoding.EncodeToString(make([]byte, 7*1024*1024))
	secondImage := base64.StdEncoding.EncodeToString(make([]byte, 7*1024*1024+1))
	request, _, err := PrepareGenerateImageGeminiNative(
		context.Background(),
		&dto.AsyncImageRequest{Prompt: "draw"},
		[]dto.GenerateImageInput{
			{InlineData: &dto.GenerateImageInlineData{MimeType: "image/png", Data: firstImage}},
			{InlineData: &dto.GenerateImageInlineData{MimeType: "image/png", Data: secondImage}},
		},
		GeminiFileDataOptions{},
	)
	require.NoError(t, err)
	require.NoError(t, ValidateGeminiGenerateContentRequestSize(request))

	thirdImage := base64.StdEncoding.EncodeToString(make([]byte, 1024*1024))
	_, _, err = PrepareGenerateImageGeminiNative(
		context.Background(),
		&dto.AsyncImageRequest{Prompt: "draw"},
		[]dto.GenerateImageInput{
			{InlineData: &dto.GenerateImageInlineData{MimeType: "image/png", Data: firstImage}},
			{InlineData: &dto.GenerateImageInlineData{MimeType: "image/png", Data: secondImage}},
			{InlineData: &dto.GenerateImageInlineData{MimeType: "image/png", Data: thirdImage}},
		},
		GeminiFileDataOptions{},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "超过 20 MiB 限制")
}

func TestValidateGeminiGenerateContentRequestSizeIncludesOtherParameters(t *testing.T) {
	request := map[string]interface{}{
		"contents": []interface{}{map[string]interface{}{
			"parts": []interface{}{
				map[string]interface{}{"text": strings.Repeat("x", geminiGenerateContentMaxBodyBytes)},
			},
		}},
	}

	err := ValidateGeminiGenerateContentRequestSize(request)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "超过 20 MiB 限制")
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

func TestPrepareGenerateImageGeminiNativePassesThroughSupportedURLs(t *testing.T) {
	ossUploads := make(chan struct{}, 3)
	configureTemporaryInputOSSTest(t, func(w http.ResponseWriter, r *http.Request) {
		ossUploads <- struct{}{}
		w.WriteHeader(http.StatusOK)
	})
	r2Uploads := make(chan struct{}, 3)
	configureTemporaryInputR2Test(t, func(w http.ResponseWriter, r *http.Request) {
		r2Uploads <- struct{}{}
		w.WriteHeader(http.StatusOK)
	})
	legacyURL := "https://oss.o1key.cn/tmp/input/reference.png?signature=secret"
	fileDataURL := "https://oss.o1key.cn/image?id=second"
	unknownExtensionURL := "https://oss.o1key.cn/image?id=third"
	inputs := []dto.GenerateImageInput{
		{Value: &legacyURL},
		{FileData: &dto.GenerateImageFileData{MimeType: "image/png", FileURI: fileDataURL}},
		{Value: &unknownExtensionURL},
	}
	var downloaded []string
	downloadImage := func(imageURL string, maxSizeMB int) (string, string, error) {
		assert.Equal(t, AsyncImageMaxURLSizeMB, maxSizeMB)
		downloaded = append(downloaded, imageURL)
		return "image/png", onePixelPNGBase64, nil
	}

	nativeRequest, preparation, err := PrepareGenerateImageGeminiNative(
		context.Background(),
		&dto.AsyncImageRequest{Prompt: "draw"},
		inputs,
		GeminiFileDataOptions{Enabled: true, downloadImage: downloadImage},
	)
	require.NoError(t, err)
	assert.Equal(t, []string{unknownExtensionURL}, downloaded)
	assert.Equal(t, "mixed", preparation.UpstreamFormat)
	assert.Equal(t, "mixed", preparation.Conversion)
	assert.Equal(t, "mime_unknown", preparation.Fallback)
	assert.Equal(t, 3, preparation.ImageCount)
	assert.Empty(t, ossUploads)
	assert.Empty(t, r2Uploads)
	legacyFileData := preparedGeminiImagePayload(t, nativeRequest, 1, "fileData")
	assert.Equal(t, legacyURL, legacyFileData["fileUri"])
	assert.Equal(t, "image/png", legacyFileData["mimeType"])
	explicitFileData := preparedGeminiImagePayload(t, nativeRequest, 2, "fileData")
	assert.Equal(t, fileDataURL, explicitFileData["fileUri"])
	assert.Equal(t, "image/png", explicitFileData["mimeType"])
	inlineData := preparedGeminiImagePayload(t, nativeRequest, 3, "inlineData")
	assert.Equal(t, onePixelPNGBase64, inlineData["data"])
}

func TestPrepareGenerateImageGeminiNativeKeepsURLInlineWhenDisabled(t *testing.T) {
	requestCount := 0
	downloadImage := func(_ string, maxSizeMB int) (string, string, error) {
		requestCount++
		assert.Equal(t, AsyncImageMaxURLSizeMB, maxSizeMB)
		return "image/png", onePixelPNGBase64, nil
	}
	imageURL := "https://oss.o1key.cn/tmp/input/reference.png"
	input := dto.GenerateImageInput{Value: &imageURL}

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

func TestPrepareGenerateImageGeminiNativeRejectsDownloadedFileDataMIMEMismatchWhenDisabled(t *testing.T) {
	fileDataURL := "https://oss.o1key.cn/tmp/input/reference.png"
	_, _, err := PrepareGenerateImageGeminiNative(
		context.Background(),
		&dto.AsyncImageRequest{Prompt: "draw"},
		[]dto.GenerateImageInput{{FileData: &dto.GenerateImageFileData{MimeType: "image/png", FileURI: fileDataURL}}},
		GeminiFileDataOptions{downloadImage: func(_ string, _ int) (string, string, error) {
			return "image/jpeg", onePixelPNGBase64, nil
		}},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match declared MIME type")
}

func TestPrepareGenerateImageGeminiNativeUploadsBase64ToR2(t *testing.T) {
	temporaryRoot := t.TempDir()
	t.Setenv("TEMP_STORAGE_DIR", temporaryRoot)
	ossUploads := make(chan struct{}, 1)
	configureTemporaryInputOSSTest(t, func(w http.ResponseWriter, r *http.Request) {
		ossUploads <- struct{}{}
		w.WriteHeader(http.StatusOK)
	})
	type uploadedObject struct {
		path string
		body []byte
	}
	r2Uploads := make(chan uploadedObject, 1)
	configureTemporaryInputR2Test(t, func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		r2Uploads <- uploadedObject{path: r.URL.Path, body: body}
		w.WriteHeader(http.StatusOK)
	})
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
	assert.True(t, strings.HasPrefix(fileURI, "https://r2.example.com/tmp/input/"))
	assert.Equal(t, "image/png", fileData["mimeType"])
	assert.Equal(t, "r2_to_file_data", preparation.Conversion)
	assert.Equal(t, "file_data", preparation.UpstreamFormat)
	assert.Greater(t, preparation.InputBytes, int64(0))
	pngBytes, err := base64.StdEncoding.DecodeString(onePixelPNGBase64)
	require.NoError(t, err)
	require.Len(t, r2Uploads, 1)
	uploaded := <-r2Uploads
	assert.Equal(t, "/r2-bucket/tmp/input/"+filepath.Base(fileURI), uploaded.path)
	assert.Equal(t, pngBytes, uploaded.body)
	assert.Empty(t, ossUploads)
	assert.NoDirExists(t, filepath.Join(temporaryRoot, TemporaryInputCategory))
}

func TestPrepareGenerateImageGeminiNativeFallsBackOnStorageFailure(t *testing.T) {
	t.Setenv("R2_BUCKET", "")
	t.Setenv("R2_PUBLIC_BASE_URL", "")
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

	imageURL := "https://oss.o1key.cn/tmp/input/reference.png"
	urlRequest, urlPreparation, err := PrepareGenerateImageGeminiNative(
		context.Background(),
		&dto.AsyncImageRequest{Prompt: "draw"},
		[]dto.GenerateImageInput{{Value: &imageURL}},
		GeminiFileDataOptions{Enabled: true, downloadImage: func(_ string, _ int) (string, string, error) {
			t.Fatal("supported URL should not be downloaded")
			return "", "", nil
		}},
	)
	require.NoError(t, err)
	urlFileData := preparedGeminiImagePayload(t, urlRequest, 1, "fileData")
	assert.Equal(t, imageURL, urlFileData["fileUri"])
	assert.Equal(t, "none", urlPreparation.Fallback)
	assert.Equal(t, "passthrough", urlPreparation.Conversion)
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
