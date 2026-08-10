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

func TestIsGeminiImageModelName(t *testing.T) {
	tests := []struct {
		model string
		want  bool
	}{
		{model: "nano-banana-pro", want: true},
		{model: "nano-banana-2-2k", want: true},
		{model: "gemini-3-pro-image-preview", want: true},
		{model: "gemini-3.1-flash-image-preview", want: true},
		{model: "gpt-image-1", want: false},
	}

	for _, test := range tests {
		t.Run(test.model, func(t *testing.T) {
			if got := isGeminiImageModelName(test.model); got != test.want {
				t.Fatalf("isGeminiImageModelName(%q) = %v, want %v", test.model, got, test.want)
			}
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
		Images: []string{"data:image/png;base64,BBBB"},
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
	req := &dto.ImageRequest{
		Model:        "gpt-image-2",
		Prompt:       "draw a cat",
		Size:         "auto",
		Quality:      "auto",
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
	for _, strategy := range []string{"", dto.ImageOutputStrategyPassthrough} {
		t.Run(strategy, func(t *testing.T) {
			images, err := prepareGenerateImageResultsWithStrategy([]dto.GenerateImageData{
				{B64Json: "AQID", MimeType: "image/png"},
				{Url: "https://upstream.example/image.png"},
			}, "origin", "api.o1key.cn", strategy)
			require.NoError(t, err)
			require.Len(t, images, 2)
			assert.Equal(t, "AQID", images[0].B64Json)
			assert.Equal(t, "image/png", images[0].MimeType)
			assert.Empty(t, images[0].Url)
			assert.Equal(t, "https://upstream.example/image.png", images[1].Url)
			assert.Empty(t, images[1].B64Json)
		})
	}
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
