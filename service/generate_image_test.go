package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testStringPtr(value string) *string {
	return &value
}

type passthroughImageAdaptor struct{}

func (passthroughImageAdaptor) Init(*relaycommon.RelayInfo) {}

func (passthroughImageAdaptor) ConvertImageRequest(_ *gin.Context, _ *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	return request, nil
}

func (passthroughImageAdaptor) DoRequest(_ *gin.Context, _ *relaycommon.RelayInfo, _ io.Reader) (any, error) {
	return nil, nil
}

func TestGenerateImageUpstreamTimingIsIncludedInAdminLogInfo(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	task := &model.Task{
		TaskID:    "task_timing_audit",
		ChannelId: 389,
		PrivateData: model.TaskPrivateData{
			UsedChannels: []string{"389"},
		},
	}
	relayInfo := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "test-image-model"},
	}

	response, err := traceGenerateImageUpstreamRequest(
		context.Background(),
		c,
		task,
		relayInfo,
		"test",
		4096,
		func() (any, error) {
			return &http.Response{StatusCode: http.StatusOK, Proto: "HTTP/1.1"}, nil
		},
	)

	require.NoError(t, err)
	require.IsType(t, &http.Response{}, response)
	require.NotNil(t, task.PrivateData.GenerateImageTiming)
	assert.Equal(t, int64(4096), task.PrivateData.GenerateImageTiming.UpstreamRequestBytes)
	assert.Equal(t, 1, task.PrivateData.GenerateImageTiming.UpstreamAttempts)
	assert.Equal(t, http.StatusOK, task.PrivateData.GenerateImageTiming.UpstreamStatus)

	adminInfo := imageTaskAdminInfo(task)
	require.Contains(t, adminInfo, "generate_image_timing")
	assert.Same(t, task.PrivateData.GenerateImageTiming, adminInfo["generate_image_timing"])

	encoded, err := common.Marshal(adminInfo)
	require.NoError(t, err)
	var decoded map[string]interface{}
	require.NoError(t, common.Unmarshal(encoded, &decoded))
	decodedTiming, ok := decoded["generate_image_timing"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, float64(4096), decodedTiming["upstream_request_bytes"])
	assert.Equal(t, float64(1), decodedTiming["upstream_attempts"])
	assert.Equal(t, float64(http.StatusOK), decodedTiming["upstream_status"])
}

func TestIsGeminiImageModelName(t *testing.T) {
	tests := []struct {
		model string
		want  bool
	}{
		{model: "nano-banana-pro", want: true},
		{model: "nano-banana-2-2k", want: true},
		{model: "gemini-3-pro-image-preview", want: true},
		{model: "gemini-3.1-flash-image-preview", want: true},
		{model: "gemini-2.5-flash-image", want: true},
		{model: "gemini-3.1-flash-lite-image-c-sd", want: true},
		{model: "gemini-3.1-flash-image-c-sp", want: true},
		{model: "gemini-3-pro-image-neil", want: true},
		{model: " GEMINI-3.1-FLASH-IMAGE-C-SP ", want: true},
		{model: "gemini-3.5-flash", want: false},
		{model: "gemini-omni-flash-preview", want: false},
		{model: "x-gemini-3.1-flash-image", want: false},
		{model: "gpt-image-1", want: false},
	}

	for _, test := range tests {
		t.Run(test.model, func(t *testing.T) {
			assert.Equal(t, test.want, isGeminiImageModelName(test.model))
		})
	}
}

func TestFitNanoBananaGenerateContentBodyResizesInlineImages(t *testing.T) {
	const width = 180
	const height = 180

	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.NRGBA{
				R: uint8((x*17 + y*31) % 256),
				G: uint8((x*29 + y*13) % 256),
				B: uint8((x*7 + y*19) % 256),
				A: 255,
			})
		}
	}

	var imageBuf bytes.Buffer
	if err := png.Encode(&imageBuf, img); err != nil {
		t.Fatalf("encode source image: %v", err)
	}

	inlineData := map[string]interface{}{
		"mimeType": "image/png",
		"data":     base64.StdEncoding.EncodeToString(imageBuf.Bytes()),
	}
	requestBody := map[string]interface{}{
		"contents": []interface{}{
			map[string]interface{}{
				"role": "user",
				"parts": []interface{}{
					map[string]interface{}{"text": "draw"},
					map[string]interface{}{"inlineData": inlineData},
				},
			},
		},
	}

	originalBody, err := common.Marshal(requestBody)
	if err != nil {
		t.Fatalf("marshal original body: %v", err)
	}
	limit := len(originalBody) / 2

	resizedBody, resized, err := fitNanoBananaGenerateContentBody(context.Background(), requestBody, limit)
	if err != nil {
		t.Fatalf("fitNanoBananaGenerateContentBody returned error: %v", err)
	}
	if !resized {
		t.Fatalf("fitNanoBananaGenerateContentBody resized = false, want true")
	}
	if len(resizedBody) > limit {
		t.Fatalf("resized body length = %d, want <= %d", len(resizedBody), limit)
	}

	resizedB64, ok := inlineData["data"].(string)
	if !ok || resizedB64 == "" {
		t.Fatalf("resized inline data missing: %#v", inlineData["data"])
	}
	raw, err := base64.StdEncoding.DecodeString(resizedB64)
	if err != nil {
		t.Fatalf("decode resized inline data: %v", err)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("decode resized image config: %v", err)
	}
	if format != "png" {
		t.Fatalf("resized image format = %q, want png", format)
	}
	if cfg.Width >= width || cfg.Height >= height {
		t.Fatalf("resized image dimensions = %dx%d, want smaller than %dx%d", cfg.Width, cfg.Height, width, height)
	}
}

func TestValidateGenerateImageRequestNormalizesNewParameters(t *testing.T) {
	req := &dto.GenerateImageRequest{
		Model:           "gemini-3.1-flash-image-preview",
		Prompt:          "draw a cat",
		Size:            " AUTO ",
		Quality:         " HIGH ",
		OutputFormat:    testStringPtr(" WEBP "),
		MediaResolution: " MEDIA_RESOLUTION_HIGH ",
		ThinkingLevel:   testStringPtr(" high "),
		Mask: &dto.ImageReference{
			ImageURL: testStringPtr(" data:image/png;base64,AAAA "),
		},
		Images: []dto.GenerateImageInput{{Value: testStringPtr("data:image/png;base64,BBBB")}},
	}

	if err := ValidateGenerateImageRequest(req); err != nil {
		t.Fatalf("ValidateGenerateImageRequest returned error: %v", err)
	}
	if req.Size != "auto" {
		t.Fatalf("Size = %q, want auto", req.Size)
	}
	if req.Quality != "high" {
		t.Fatalf("Quality = %q, want high", req.Quality)
	}
	if req.OutputFormat == nil || *req.OutputFormat != "webp" {
		t.Fatalf("OutputFormat = %v, want webp", req.OutputFormat)
	}
	if req.MediaResolution != "MEDIA_RESOLUTION_HIGH" {
		t.Fatalf("MediaResolution = %q, want MEDIA_RESOLUTION_HIGH", req.MediaResolution)
	}
	if req.ThinkingLevel == nil || *req.ThinkingLevel != "high" {
		t.Fatalf("ThinkingLevel = %v, want high", req.ThinkingLevel)
	}
	if req.Mask == nil || req.Mask.ImageURL == nil || *req.Mask.ImageURL != "data:image/png;base64,AAAA" {
		t.Fatalf("Mask.ImageURL = %v, want trimmed data URL", req.Mask)
	}
}

func TestValidateGenerateImageRequestNormalizesGPTImageParameters(t *testing.T) {
	req := &dto.GenerateImageRequest{
		Model:      "gpt-image-1",
		Prompt:     "draw a cat",
		Background: testStringPtr(" AUTO "),
		Moderation: testStringPtr(" LOW "),
	}

	require.NoError(t, ValidateGenerateImageRequest(req))
	require.NotNil(t, req.Background)
	assert.Equal(t, "auto", *req.Background)
	require.NotNil(t, req.Moderation)
	assert.Equal(t, "low", *req.Moderation)
}

func TestValidateGenerateImageRequestEnforcesSeedreamContract(t *testing.T) {
	two := uint(2)
	enabled := true
	valid := &dto.GenerateImageRequest{
		Model:        "dola-seedream-5-0-pro-260628-ep",
		Prompt:       "draw",
		Size:         "2048x2048",
		OutputFormat: testStringPtr("PNG"),
		Images: []dto.GenerateImageInput{
			{Value: testStringPtr("https://example.com/one.png")},
			{Value: testStringPtr("https://example.com/two.png")},
		},
	}
	require.NoError(t, ValidateGenerateImageRequest(valid))
	assert.Equal(t, "png", *valid.OutputFormat)
	layerRequest := &dto.GenerateImageRequest{
		Model:              "dola-seedream-5-0-pro-260628-ep",
		Size:               "1.5K",
		LayerDecomposition: &enabled,
		Images: []dto.GenerateImageInput{
			{Value: testStringPtr("https://example.com/input.png")},
		},
	}
	require.NoError(t, ValidateGenerateImageRequest(layerRequest))

	tests := []struct {
		name    string
		request *dto.GenerateImageRequest
		message string
	}{
		{
			name: "multiple outputs",
			request: &dto.GenerateImageRequest{
				Model: "dola-seedream-5-0-pro-260628-ep", Prompt: "draw", N: &two,
			},
			message: "n must be 1",
		},
		{
			name: "too many references",
			request: &dto.GenerateImageRequest{
				Model: "dola-seedream-5-0-pro-260628-ep", Prompt: "draw", Images: make([]dto.GenerateImageInput, 11),
			},
			message: "at most 10",
		},
		{
			name: "unsupported output format",
			request: &dto.GenerateImageRequest{
				Model: "dola-seedream-5-0-pro-260628-ep", Prompt: "draw", OutputFormat: testStringPtr("webp"),
			},
			message: "png or jpeg",
		},
		{
			name: "too few pixels",
			request: &dto.GenerateImageRequest{
				Model: "dola-seedream-5-0-pro-260628-ep", Prompt: "draw", Size: "100x100",
			},
			message: "between 921600 and 4624220 pixels",
		},
		{
			name: "layer decomposition without an image",
			request: &dto.GenerateImageRequest{
				Model: "dola-seedream-5-0-pro-260628-ep", Prompt: "split", LayerDecomposition: &enabled,
			},
			message: "exactly 1 item",
		},
		{
			name: "layer decomposition with multiple images",
			request: &dto.GenerateImageRequest{
				Model: "dola-seedream-5-0-pro-260628-ep", Prompt: "split", LayerDecomposition: &enabled,
				Images: []dto.GenerateImageInput{
					{Value: testStringPtr("https://example.com/one.png")},
					{Value: testStringPtr("https://example.com/two.png")},
				},
			},
			message: "exactly 1 item",
		},
		{
			name: "layer decomposition with explicit pixel size",
			request: &dto.GenerateImageRequest{
				Model: "dola-seedream-5-0-pro-260628-ep", Prompt: "split", Size: "1024x1024", LayerDecomposition: &enabled,
				Images: []dto.GenerateImageInput{{Value: testStringPtr("https://example.com/input.png")}},
			},
			message: "size must be auto, 1K, 1.5K, or 2K",
		},
		{
			name: "layer decomposition on another model",
			request: &dto.GenerateImageRequest{
				Model: "gpt-image-1", Prompt: "split", LayerDecomposition: &enabled,
			},
			message: "only supported by Seedream 5.0 Pro",
		},
		{
			name: "provider-specific parameter",
			request: &dto.GenerateImageRequest{
				Model: "dola-seedream-5-0-pro-260628-ep", Prompt: "draw", Quality: "high",
			},
			message: "only supports prompt, images, n, size, output_format, watermark, and layer_decomposition",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateGenerateImageRequest(test.request)
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.message)
		})
	}
}

func TestValidateGenerateImageRequestRejectsInvalidEnums(t *testing.T) {
	tests := []struct {
		name    string
		req     *dto.GenerateImageRequest
		wantErr string
	}{
		{
			name: "quality",
			req: &dto.GenerateImageRequest{
				Model:   "gpt-image-1",
				Prompt:  "draw a cat",
				Quality: "best",
			},
			wantErr: "quality must be one of",
		},
		{
			name: "output format",
			req: &dto.GenerateImageRequest{
				Model:        "gpt-image-1",
				Prompt:       "draw a cat",
				OutputFormat: testStringPtr("gif"),
			},
			wantErr: "output_format must be one of",
		},
		{
			name: "background value",
			req: &dto.GenerateImageRequest{
				Model:      "gpt-image-2",
				Prompt:     "draw a cat",
				Background: testStringPtr("opaque"),
			},
			wantErr: "background must be one of",
		},
		{
			name: "background on banana",
			req: &dto.GenerateImageRequest{
				Model:      "nano-banana-pro",
				Prompt:     "draw a cat",
				Background: testStringPtr("transparent"),
			},
			wantErr: "not supported by Banana/Gemini",
		},
		{
			name: "background on another model",
			req: &dto.GenerateImageRequest{
				Model:      "dall-e-3",
				Prompt:     "draw a cat",
				Background: testStringPtr("auto"),
			},
			wantErr: "gpt-image prefix",
		},
		{
			name: "moderation value",
			req: &dto.GenerateImageRequest{
				Model:      "gpt-image-2",
				Prompt:     "draw a cat",
				Moderation: testStringPtr("strict"),
			},
			wantErr: "moderation must be one of",
		},
		{
			name: "moderation on banana",
			req: &dto.GenerateImageRequest{
				Model:      "nano-banana-pro",
				Prompt:     "draw a cat",
				Moderation: testStringPtr("low"),
			},
			wantErr: "not supported by Banana/Gemini",
		},
		{
			name: "moderation on another model",
			req: &dto.GenerateImageRequest{
				Model:      "dall-e-3",
				Prompt:     "draw a cat",
				Moderation: testStringPtr("auto"),
			},
			wantErr: "gpt-image prefix",
		},
		{
			name: "transparent jpeg",
			req: &dto.GenerateImageRequest{
				Model:        "gpt-image-2-c-sd",
				Prompt:       "draw a cat",
				Background:   testStringPtr("transparent"),
				OutputFormat: testStringPtr("jpeg"),
			},
			wantErr: "png or webp",
		},
		{
			name: "empty thinking level",
			req: &dto.GenerateImageRequest{
				Model:         "nano-banana-pro",
				Prompt:        "draw a cat",
				ThinkingLevel: testStringPtr(" "),
			},
			wantErr: "thinking_level must not be empty",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateGenerateImageRequest(test.req)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("error = %v, want containing %q", err, test.wantErr)
			}
		})
	}
}

func TestValidateGenerateImageRequestValidatesMask(t *testing.T) {
	tests := []struct {
		name    string
		mask    *dto.ImageReference
		wantErr string
	}{
		{
			name:    "neither field",
			mask:    &dto.ImageReference{},
			wantErr: "exactly one",
		},
		{
			name: "both fields",
			mask: &dto.ImageReference{
				FileID:   testStringPtr("file-123"),
				ImageURL: testStringPtr("https://example.com/mask.png"),
			},
			wantErr: "exactly one",
		},
		{
			name: "invalid image url",
			mask: &dto.ImageReference{
				ImageURL: testStringPtr("not-a-url"),
			},
			wantErr: "mask.image_url",
		},
		{
			name: "empty file id",
			mask: &dto.ImageReference{
				FileID: testStringPtr(" "),
			},
			wantErr: "mask.file_id",
		},
		{
			name: "empty image url",
			mask: &dto.ImageReference{
				ImageURL: testStringPtr(" "),
			},
			wantErr: "mask.image_url",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := &dto.GenerateImageRequest{
				Model:  "gpt-image-1",
				Prompt: "draw a cat",
				Mask:   test.mask,
			}
			err := ValidateGenerateImageRequest(req)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("error = %v, want containing %q", err, test.wantErr)
			}
		})
	}
}

func TestGenerateImageRawMessageHelpers(t *testing.T) {
	outputFormatRaw := stringPtrToRawMessage(testStringPtr("png"))
	var outputFormat string
	if err := common.Unmarshal(outputFormatRaw, &outputFormat); err != nil {
		t.Fatalf("unmarshal output_format: %v", err)
	}
	if outputFormat != "png" {
		t.Fatalf("outputFormat = %q, want png", outputFormat)
	}

	maskRaw := imageReferenceToRawMessage(&dto.ImageReference{FileID: testStringPtr("file-123")})
	var mask map[string]string
	if err := common.Unmarshal(maskRaw, &mask); err != nil {
		t.Fatalf("unmarshal mask: %v", err)
	}
	if mask["file_id"] != "file-123" {
		t.Fatalf("mask[file_id] = %q, want file-123", mask["file_id"])
	}
	if _, ok := mask["image_url"]; ok {
		t.Fatalf("mask should not include empty image_url: %v", mask)
	}
}

func TestMappedAsyncImageRequest(t *testing.T) {
	req := &dto.ImageRequest{Model: "gpt-image-2-c"}
	mappedReq := mappedAsyncImageRequest(req, &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-image-2"},
	})
	if req.Model != "gpt-image-2-c" {
		t.Fatalf("original Model = %q, want unchanged original model", req.Model)
	}
	if mappedReq == req {
		t.Fatalf("mapped request should be a copy, got same pointer")
	}
	if mappedReq.Model != "gpt-image-2" {
		t.Fatalf("mapped Model = %q, want mapped upstream model", mappedReq.Model)
	}

	mappedReq = mappedAsyncImageRequest(req, &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{},
	})
	if mappedReq.Model != "gpt-image-2-c" {
		t.Fatalf("empty upstream model should not overwrite request model, got %q", mappedReq.Model)
	}

	mappedReq = mappedAsyncImageRequest(req, &relaycommon.RelayInfo{})
	if mappedReq.Model != "gpt-image-2-c" {
		t.Fatalf("nil channel meta should not overwrite request model, got %q", mappedReq.Model)
	}

	mappedReq = mappedAsyncImageRequest(nil, &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-image-2"},
	})
	if mappedReq != nil {
		t.Fatalf("nil image request should return nil, got %#v", mappedReq)
	}
}

func TestNewAsyncOpenAIImageRequestPreservesGPTImageParameters(t *testing.T) {
	background := "transparent"
	moderation := "low"
	outputFormat := "png"
	imageReq := newAsyncOpenAIImageRequest(&dto.AsyncImageRequest{
		Model:        "gpt-image-2",
		Prompt:       "draw a cat",
		Background:   &background,
		Moderation:   &moderation,
		OutputFormat: &outputFormat,
	}, nil, nil)

	var gotBackground string
	require.NoError(t, common.Unmarshal(imageReq.Background, &gotBackground))
	assert.Equal(t, "transparent", gotBackground)
	var gotModeration string
	require.NoError(t, common.Unmarshal(imageReq.Moderation, &gotModeration))
	assert.Equal(t, "low", gotModeration)
	var gotOutputFormat string
	require.NoError(t, common.Unmarshal(imageReq.OutputFormat, &gotOutputFormat))
	assert.Equal(t, "png", gotOutputFormat)
}

func TestSeedreamRouteUsesSynchronousGenerationsWorker(t *testing.T) {
	action, isGeminiNative := ResolveImageRoute("dola-seedream-5-0-pro-260628-ep", 1)
	assert.Equal(t, "seedreamGenerate", action)
	assert.False(t, isGeminiNative)
}

func TestNewAsyncSeedreamImageRequestUsesImageArrayAndPreservesURLs(t *testing.T) {
	watermark := false
	layerDecomposition := true
	imageReq, err := newAsyncSeedreamImageRequest(&dto.AsyncImageRequest{
		Model:              "dola-seedream-5-0-pro-260628-ep",
		Prompt:             "combine",
		Images:             []string{"https://example.com/one.png", "https://example.com/two.png"},
		Watermark:          &watermark,
		LayerDecomposition: &layerDecomposition,
		OutputFormat:       testStringPtr("png"),
	})
	require.NoError(t, err)
	require.NotNil(t, imageReq)
	assert.Empty(t, imageReq.Images)
	assert.False(t, *imageReq.Watermark)

	var images []string
	require.NoError(t, common.Unmarshal(imageReq.Image, &images))
	assert.Equal(t, []string{"https://example.com/one.png", "https://example.com/two.png"}, images)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/async/v1/generateImage", nil)
	c.Request.Header.Set("Content-Type", "application/json")
	relayInfo := &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeImagesGenerations}
	body, _, err := buildAsyncOpenAIImageRequestBody(c, passthroughImageAdaptor{}, relayInfo, imageReq)
	require.NoError(t, err)
	encoded, err := io.ReadAll(body)
	require.NoError(t, err)
	assert.Contains(t, string(encoded), `"image":["https://example.com/one.png","https://example.com/two.png"]`)
	assert.NotContains(t, string(encoded), `"images"`)
	assert.Contains(t, string(encoded), `"watermark":false`)
	assert.Contains(t, string(encoded), `"layer_decomposition":true`)
}

func TestAsyncOpenAIImageRelayMode(t *testing.T) {
	if got := asyncOpenAIImageRelayMode(&dto.ImageRequest{}); got != relayconstant.RelayModeImagesGenerations {
		t.Fatalf("empty request relay mode = %d, want generations", got)
	}

	imagesRaw, err := common.Marshal([]string{"data:image/png;base64,AAAA"})
	if err != nil {
		t.Fatalf("marshal images: %v", err)
	}
	if got := asyncOpenAIImageRelayMode(&dto.ImageRequest{Images: imagesRaw}); got != relayconstant.RelayModeImagesEdits {
		t.Fatalf("images relay mode = %d, want edits", got)
	}

	imageRaw, err := common.Marshal("data:image/png;base64,AAAA")
	if err != nil {
		t.Fatalf("marshal image: %v", err)
	}
	if got := asyncOpenAIImageRelayMode(&dto.ImageRequest{Image: imageRaw}); got != relayconstant.RelayModeImagesEdits {
		t.Fatalf("image relay mode = %d, want edits", got)
	}

	maskRaw, err := common.Marshal(map[string]string{"image_url": "data:image/png;base64,BBBB"})
	if err != nil {
		t.Fatalf("marshal mask: %v", err)
	}
	if got := asyncOpenAIImageRelayMode(&dto.ImageRequest{Mask: maskRaw}); got != relayconstant.RelayModeImagesEdits {
		t.Fatalf("mask relay mode = %d, want edits", got)
	}
}

func TestAsyncImageRequestURLPath(t *testing.T) {
	if got := asyncImageRequestURLPath(relayconstant.RelayModeImagesGenerations, "gpt-image-2"); got != "/v1/images/generations" {
		t.Fatalf("generations path = %q", got)
	}
	if got := asyncImageRequestURLPath(relayconstant.RelayModeImagesEdits, "gpt-image-2"); got != "/v1/images/edits" {
		t.Fatalf("edits path = %q", got)
	}
	if got := asyncImageRequestURLPath(relayconstant.RelayModeGemini, "gemini-3-pro-image"); got != "/v1beta/models/gemini-3-pro-image:generateContent" {
		t.Fatalf("gemini path = %q", got)
	}
}

func TestConvertAsyncImageToGeminiNativeMapsThinkingConfig(t *testing.T) {
	includeThoughts := false
	nativeReq, err := ConvertAsyncImageToGeminiNative(context.Background(), &dto.AsyncImageRequest{
		Model:              "gemini-3.1-flash-image-preview",
		Prompt:             "draw a cat",
		ResponseModalities: []string{"IMAGE"},
		MediaResolution:    "MEDIA_RESOLUTION_HIGH",
		ThinkingLevel:      testStringPtr(" minimal "),
		IncludeThoughts:    &includeThoughts,
	})
	if err != nil {
		t.Fatalf("ConvertAsyncImageToGeminiNative returned error: %v", err)
	}

	generationConfig, ok := nativeReq["generationConfig"].(map[string]interface{})
	if !ok {
		t.Fatalf("generationConfig = %#v, want object", nativeReq["generationConfig"])
	}
	if generationConfig["mediaResolution"] != "MEDIA_RESOLUTION_HIGH" {
		t.Fatalf("mediaResolution = %#v, want MEDIA_RESOLUTION_HIGH", generationConfig["mediaResolution"])
	}
	thinkingConfig, ok := generationConfig["thinkingConfig"].(map[string]interface{})
	if !ok {
		t.Fatalf("thinkingConfig = %#v, want object", generationConfig["thinkingConfig"])
	}
	if thinkingConfig["thinkingLevel"] != "minimal" {
		t.Fatalf("thinkingLevel = %#v, want minimal", thinkingConfig["thinkingLevel"])
	}
	if includeThoughtsValue, ok := thinkingConfig["includeThoughts"].(bool); !ok || includeThoughtsValue {
		t.Fatalf("includeThoughts = %#v, want explicit false", thinkingConfig["includeThoughts"])
	}
}

func TestPrepareAsyncOpenAIImageRequestMapsImagesToImage(t *testing.T) {
	imagesRaw, err := common.Marshal([]string{"data:image/png;base64,AAAA"})
	if err != nil {
		t.Fatalf("marshal images: %v", err)
	}
	req := &dto.ImageRequest{
		Model:  "gpt-image-2-c",
		Images: imagesRaw,
	}
	upstreamReq := prepareAsyncOpenAIImageRequest(req, &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeImagesEdits,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-image-2"},
	})

	if req.Model != "gpt-image-2-c" || len(req.Image) != 0 || len(req.Images) == 0 {
		t.Fatalf("original request should stay unchanged: %#v", req)
	}
	if upstreamReq.Model != "gpt-image-2" {
		t.Fatalf("upstream model = %q, want mapped model", upstreamReq.Model)
	}
	var image string
	if err := common.Unmarshal(upstreamReq.Image, &image); err != nil {
		t.Fatalf("unmarshal upstream image: %v", err)
	}
	if image != "data:image/png;base64,AAAA" {
		t.Fatalf("upstream image = %q", image)
	}
	if len(upstreamReq.Images) != 0 {
		t.Fatalf("upstream images should be cleared after mapping to image: %s", string(upstreamReq.Images))
	}
}

func TestPrepareAsyncOpenAIImageRequestKeepsMultipleImages(t *testing.T) {
	imagesRaw, err := common.Marshal([]string{
		"data:image/png;base64,AAAA",
		"data:image/png;base64,BBBB",
	})
	if err != nil {
		t.Fatalf("marshal images: %v", err)
	}
	upstreamReq := prepareAsyncOpenAIImageRequest(&dto.ImageRequest{Images: imagesRaw}, &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesEdits,
	})

	var images []string
	if err := common.Unmarshal(upstreamReq.Image, &images); err != nil {
		t.Fatalf("unmarshal upstream image array: %v", err)
	}
	if len(images) != 2 || images[0] != "data:image/png;base64,AAAA" || images[1] != "data:image/png;base64,BBBB" {
		t.Fatalf("upstream image array = %#v", images)
	}
	if !json.Valid(upstreamReq.Image) {
		t.Fatalf("upstream image should be valid json: %s", string(upstreamReq.Image))
	}
}

func TestBuildAsyncOpenAIImageEditMultipartBody(t *testing.T) {
	imageRaw, err := common.Marshal("data:image/png;base64,AQID")
	if err != nil {
		t.Fatalf("marshal image: %v", err)
	}
	outputFormatRaw, err := common.Marshal("png")
	if err != nil {
		t.Fatalf("marshal output format: %v", err)
	}
	backgroundRaw, err := common.Marshal("transparent")
	if err != nil {
		t.Fatalf("marshal background: %v", err)
	}
	moderationRaw, err := common.Marshal("low")
	if err != nil {
		t.Fatalf("marshal moderation: %v", err)
	}
	req := &dto.ImageRequest{
		Model:        "gpt-image-2",
		Prompt:       "draw a cat",
		Size:         "auto",
		Quality:      "auto",
		Background:   backgroundRaw,
		Moderation:   moderationRaw,
		OutputFormat: outputFormatRaw,
		Image:        imageRaw,
	}
	c := &gin.Context{Request: &http.Request{Header: make(http.Header)}}

	body, err := buildAsyncOpenAIImageEditMultipartBody(c, req)
	if err != nil {
		t.Fatalf("build multipart body: %v", err)
	}
	contentType := c.Request.Header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("parse content type %q: %v", contentType, err)
	}
	if mediaType != "multipart/form-data" {
		t.Fatalf("content type = %q, want multipart/form-data", mediaType)
	}

	form := readMultipartForm(t, body, params["boundary"])
	if got := form.Value["model"]; len(got) != 1 || got[0] != "gpt-image-2" {
		t.Fatalf("model field = %#v", got)
	}
	if got := form.Value["prompt"]; len(got) != 1 || got[0] != "draw a cat" {
		t.Fatalf("prompt field = %#v", got)
	}
	if got := form.Value["output_format"]; len(got) != 1 || got[0] != "png" {
		t.Fatalf("output_format field = %#v", got)
	}
	if got := form.Value["background"]; len(got) != 1 || got[0] != "transparent" {
		t.Fatalf("background field = %#v", got)
	}
	if got := form.Value["moderation"]; len(got) != 1 || got[0] != "low" {
		t.Fatalf("moderation field = %#v", got)
	}
	imageFiles := form.File["image"]
	if len(imageFiles) != 1 {
		t.Fatalf("image files = %d, want 1", len(imageFiles))
	}
	file, err := imageFiles[0].Open()
	if err != nil {
		t.Fatalf("open image file: %v", err)
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("read image file: %v", err)
	}
	if !bytes.Equal(data, []byte{1, 2, 3}) {
		t.Fatalf("image bytes = %v", data)
	}
}

func TestBuildAsyncOpenAIImageGenerationBodyPreservesGPTImageParameters(t *testing.T) {
	backgroundRaw, err := common.Marshal("transparent")
	require.NoError(t, err)
	outputFormatRaw, err := common.Marshal("webp")
	require.NoError(t, err)
	moderationRaw, err := common.Marshal("low")
	require.NoError(t, err)
	request := &dto.ImageRequest{
		Model:        "gpt-image-2",
		Prompt:       "draw a cat",
		Background:   backgroundRaw,
		Moderation:   moderationRaw,
		OutputFormat: outputFormatRaw,
	}
	relayInfo := &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeImagesGenerations}
	c := &gin.Context{Request: &http.Request{Header: make(http.Header)}}

	body, _, err := buildAsyncOpenAIImageRequestBody(c, passthroughImageAdaptor{}, relayInfo, request)
	require.NoError(t, err)
	bodyBytes, err := io.ReadAll(body)
	require.NoError(t, err)

	var payload map[string]json.RawMessage
	require.NoError(t, common.Unmarshal(bodyBytes, &payload))
	assert.JSONEq(t, `"transparent"`, string(payload["background"]))
	assert.JSONEq(t, `"low"`, string(payload["moderation"]))
	assert.JSONEq(t, `"webp"`, string(payload["output_format"]))
}

func readMultipartForm(t *testing.T, body *bytes.Buffer, boundary string) *multipart.Form {
	t.Helper()
	if boundary == "" {
		t.Fatalf("multipart boundary is empty")
	}
	reader := multipart.NewReader(bytes.NewReader(body.Bytes()), boundary)
	form, err := reader.ReadForm(1024 * 1024)
	if err != nil {
		t.Fatalf("read multipart form: %v", err)
	}
	t.Cleanup(func() { _ = form.RemoveAll() })
	return form
}

func TestPrepareGenerateImageResultsPreservesUpstreamShapeWithoutStorageStrategy(t *testing.T) {
	zIndex := 1
	box := &dto.ImageBoundingBox{Absolute: []int{10, 20, 110, 220}, Normalized: []int{10, 20, 110, 220}}
	for _, strategy := range []string{"", dto.ImageOutputStrategyPassthrough} {
		t.Run(strategy, func(t *testing.T) {
			images, err := prepareGenerateImageResultsWithStrategy([]dto.GenerateImageData{
				{B64Json: "AQID", MimeType: "image/png", Size: "1024x1024", OutputFormat: "png", ZIndex: &zIndex, BoundingBox: box, Name: "subject", Description: "foreground subject"},
				{Url: "https://upstream.example/image.png"},
			}, "api.o1key.cn", strategy)
			require.NoError(t, err)
			require.Len(t, images, 2)
			assert.Equal(t, "AQID", images[0].B64Json)
			assert.Equal(t, "image/png", images[0].MimeType)
			assert.Equal(t, "1024x1024", images[0].Size)
			assert.Equal(t, "png", images[0].OutputFormat)
			assert.Empty(t, images[0].Url)
			require.NotNil(t, images[0].ZIndex)
			assert.Equal(t, 1, *images[0].ZIndex)
			assert.Equal(t, box, images[0].BoundingBox)
			assert.Equal(t, "subject", images[0].Name)
			assert.Equal(t, "foreground subject", images[0].Description)
			assert.Equal(t, "https://upstream.example/image.png", images[1].Url)
			assert.Empty(t, images[1].B64Json)
		})
	}
}

func TestGenerateImageBase64DecodedSize(t *testing.T) {
	tests := []struct {
		name string
		data string
		want int64
	}{
		{name: "raw padded", data: "aGVsbG8=", want: 5},
		{name: "data URL", data: "data:image/png;base64,aGVsbG8=", want: 5},
		{name: "double padding", data: "YQ==", want: 1},
		{name: "empty", data: "", want: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, generateImageBase64DecodedSize(test.data))
		})
	}
}

func TestPrepareGenerateImageResultsStoresLocalTemporaryOutput(t *testing.T) {
	t.Setenv("TEMP_STORAGE_DIR", t.TempDir())
	t.Setenv("TEMP_STORAGE_PUBLIC_BASE_URL", "https://api.o1key.cn")
	const pngBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="

	images, err := prepareGenerateImageResultsWithStrategy([]dto.GenerateImageData{
		{B64Json: pngBase64, MimeType: "image/png"},
	}, "api.o1key.cn", dto.ImageOutputStrategyLocalTempCF)
	require.NoError(t, err)
	require.Len(t, images, 1)
	assert.Empty(t, images[0].B64Json)
	assert.True(t, strings.HasPrefix(images[0].Url, "https://cf-api.o1key.com/tmp/output/"))
}

func TestExtractGeminiImagesSupportsInlineAndFileData(t *testing.T) {
	const fileURI = "https://flowapi.gaorui.cc/tmp/d357508ddebea305feeacad77a63e34a.jpg"
	geminiResp := map[string]interface{}{
		"candidates": []interface{}{
			map[string]interface{}{
				"content": map[string]interface{}{
					"parts": []interface{}{
						map[string]interface{}{
							"inlineData": map[string]interface{}{
								"mimeType": "image/png",
								"data":     "AQID",
							},
						},
						map[string]interface{}{
							"fileData": map[string]interface{}{
								"mimeType": "image/jpeg",
								"fileUri":  fileURI,
							},
						},
						map[string]interface{}{"text": fileURI},
						map[string]interface{}{
							"thought": true,
							"fileData": map[string]interface{}{
								"mimeType": "image/jpeg",
								"fileUri":  "https://example.com/thought.jpg",
							},
						},
					},
				},
			},
		},
	}

	images := extractGeminiImages(geminiResp)

	require.Len(t, images, 2)
	assert.Equal(t, dto.GenerateImageData{B64Json: "AQID", MimeType: "image/png"}, images[0])
	assert.Equal(t, dto.GenerateImageData{Url: fileURI, MimeType: "image/jpeg"}, images[1])
}

func TestBuildGenerateImageRelayInfoPreservesChannelImageOutputStrategy(t *testing.T) {
	channel := &model.Channel{
		Type:    1,
		Name:    "generate-image-output-strategy-test",
		Key:     "test-key",
		BaseURL: testStringPtr("https://upstream.example"),
		Models:  "banana2",
		Group:   "test",
		Status:  common.ChannelStatusEnabled,
	}
	channel.SetOtherSettings(dto.ChannelOtherSettings{ImageOutputStrategy: dto.ImageOutputStrategyOSS})
	require.NoError(t, model.DB.Create(channel).Error)
	t.Cleanup(func() {
		require.NoError(t, model.DB.Delete(&model.Channel{}, channel.Id).Error)
	})

	c, _ := gin.CreateTestContext(nil)
	relayInfo, err := buildGenerateImageRelayInfo(c, &model.Task{
		ChannelId: channel.Id,
		Group:     "test",
		Properties: model.Properties{
			OriginModelName: "banana2",
		},
	}, relayconstant.RelayModeImagesGenerations)
	require.NoError(t, err)
	assert.Equal(t, dto.ImageOutputStrategyOSS, relayInfo.ChannelOtherSettings.ImageOutputStrategy)
}

func TestImageUpstreamUsageDetail(t *testing.T) {
	cases := []struct {
		name             string
		promptTokens     int
		completionTokens int
	}{
		// 方案A：调用点均在 200 校验之后，无论上游是否回显 token，都标记已计费。
		{name: "both zero still billed", promptTokens: 0, completionTokens: 0},
		{name: "prompt tokens only", promptTokens: 120, completionTokens: 0},
		{name: "completion tokens only", promptTokens: 0, completionTokens: 30},
		{name: "both non-zero", promptTokens: 120, completionTokens: 30},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			detail := imageUpstreamUsageDetail(tc.promptTokens, tc.completionTokens)
			require.NotNil(t, detail)
			assert.True(t, detail.UpstreamBilled)
			assert.Equal(t, tc.promptTokens, detail.UpstreamPromptTokens)
			assert.Equal(t, tc.completionTokens, detail.UpstreamCompletionTokens)
		})
	}
}

func TestBuildGenerateImageTaskErrorDetailUsesRelayStatusAndRetryAfter(t *testing.T) {
	relayErr := types.WithOpenAIError(types.OpenAIError{
		Message: "too many requests",
		Type:    "rate_limit_error",
		Code:    "rate_limit_exceeded",
	}, http.StatusTooManyRequests)
	headers := http.Header{"Retry-After": []string{"45"}}

	detail := buildGenerateImageTaskErrorDetail(relayErr, headers)
	if detail == nil {
		t.Fatalf("detail is nil")
	}
	if detail.UpstreamStatus != http.StatusTooManyRequests {
		t.Fatalf("UpstreamStatus = %d", detail.UpstreamStatus)
	}
	if detail.UpstreamCode != "rate_limit_exceeded" || detail.UpstreamType != "rate_limit_error" {
		t.Fatalf("upstream code/type = (%q, %q)", detail.UpstreamCode, detail.UpstreamType)
	}
	if detail.RetryAfterSeconds != 45 || detail.RetryAction != imageRetryActionResubmit {
		t.Fatalf("retry hint = (%d, %q)", detail.RetryAfterSeconds, detail.RetryAction)
	}
}

func TestBuildGenerateImageTaskErrorDetailUsesDefaultRetryAfter(t *testing.T) {
	relayErr := types.NewOpenAIError(
		fmt.Errorf("service unavailable"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusServiceUnavailable,
	)

	detail := buildGenerateImageTaskErrorDetail(relayErr, nil)
	if detail == nil {
		t.Fatalf("detail is nil")
	}
	if detail.UpstreamStatus != http.StatusServiceUnavailable {
		t.Fatalf("UpstreamStatus = %d", detail.UpstreamStatus)
	}
	if detail.RetryAfterSeconds != 60 || detail.RetryAction != imageRetryActionResubmit {
		t.Fatalf("retry hint = (%d, %q)", detail.RetryAfterSeconds, detail.RetryAction)
	}
}

func TestGenerateImageStatusRetryUsesGlobalPolicy(t *testing.T) {
	originalRetryTimes := common.RetryTimes
	originalRanges := operation_setting.AutomaticRetryStatusCodeRanges
	t.Cleanup(func() {
		common.RetryTimes = originalRetryTimes
		operation_setting.AutomaticRetryStatusCodeRanges = originalRanges
	})
	common.RetryTimes = 2
	operation_setting.AutomaticRetryStatusCodeRanges = []operation_setting.StatusCodeRange{
		{Start: http.StatusTooManyRequests, End: http.StatusTooManyRequests},
		{Start: http.StatusBadGateway, End: http.StatusBadGateway},
		{Start: http.StatusGatewayTimeout, End: http.StatusGatewayTimeout},
	}

	tests := []struct {
		name         string
		statuses     []int
		wantAttempts int
		wantStatus   int
		wantRelayErr bool
	}{
		{
			name:         "retryable status eventually succeeds",
			statuses:     []int{http.StatusBadGateway, http.StatusTooManyRequests, http.StatusOK},
			wantAttempts: 3,
			wantStatus:   http.StatusOK,
			wantRelayErr: false,
		},
		{
			name:         "retry budget is exhausted",
			statuses:     []int{http.StatusBadGateway, http.StatusBadGateway, http.StatusBadGateway},
			wantAttempts: 3,
			wantStatus:   http.StatusBadGateway,
			wantRelayErr: true,
		},
		{
			name:         "status outside configured ranges is not retried",
			statuses:     []int{http.StatusBadRequest},
			wantAttempts: 1,
			wantStatus:   http.StatusBadRequest,
			wantRelayErr: true,
		},
		{
			name:         "always skipped timeout is not retried",
			statuses:     []int{http.StatusGatewayTimeout},
			wantAttempts: 1,
			wantStatus:   http.StatusGatewayTimeout,
			wantRelayErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			attempts := 0
			task := &model.Task{ChannelId: 292, TaskID: "task-retry-test"}
			resp, relayErr, err := doGenerateImageRequestWithStatusRetry(context.Background(), task, func() (any, error) {
				status := tc.statuses[attempts]
				attempts++
				return &http.Response{
					StatusCode: status,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"upstream error"}}`)),
				}, nil
			})

			require.NoError(t, err)
			require.NotNil(t, resp)
			assert.Equal(t, tc.wantAttempts, attempts)
			assert.Equal(t, tc.wantAttempts, len(task.PrivateData.UsedChannels))
			for _, channelID := range task.PrivateData.UsedChannels {
				assert.Equal(t, "292", channelID)
			}
			assert.Equal(t, tc.wantStatus, resp.StatusCode)
			assert.Equal(t, tc.wantRelayErr, relayErr != nil)
		})
	}
}
