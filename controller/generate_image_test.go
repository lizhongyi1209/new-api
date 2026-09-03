package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnmarshalGenerateImageBodyWithoutContentType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	c.Request, _ = http.NewRequest(http.MethodPost, "/async/v1/generateImage", strings.NewReader(`{"model":"nano-banana-pro","prompt":"draw","images":[]}`))

	var raw map[string]interface{}
	if err := unmarshalGenerateImageBody(c, &raw); err != nil {
		t.Fatalf("first unmarshal returned error: %v", err)
	}
	if raw["model"] != "nano-banana-pro" {
		t.Fatalf("raw model = %v, want nano-banana-pro", raw["model"])
	}

	var req dto.GenerateImageRequest
	if err := unmarshalGenerateImageBody(c, &req); err != nil {
		t.Fatalf("second unmarshal returned error: %v", err)
	}
	if req.Model != "nano-banana-pro" || req.Prompt != "draw" {
		t.Fatalf("req = %#v, want parsed model and prompt", req)
	}
}

func TestGenerateImageSubmitRejectsBackgroundForBanana(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/async/v1/generateImage", strings.NewReader(
		`{"model":"nano-banana-pro","prompt":"draw","background":"transparent"}`,
	))
	c.Request.Header.Set("Content-Type", "application/json")

	GenerateImageSubmit(c)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "background is not supported by Banana/Gemini image models")
}

func TestGenerateImageSubmitRejectsModerationForBanana(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/async/v1/generateImage", strings.NewReader(
		`{"model":"nano-banana-pro","prompt":"draw","moderation":"low"}`,
	))
	c.Request.Header.Set("Content-Type", "application/json")

	GenerateImageSubmit(c)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "moderation is not supported by Banana/Gemini image models")
}

func TestApplyGenerateImageGoogleSearchTool(t *testing.T) {
	enabled := true
	nativeReq := map[string]interface{}{}
	applyGenerateImageGoogleSearchTool(nativeReq, &enabled)

	tools, ok := nativeReq["tools"].([]interface{})
	if !ok || len(tools) != 1 {
		t.Fatalf("tools = %#v, want one googleSearch tool", nativeReq["tools"])
	}
	tool, ok := tools[0].(map[string]interface{})
	if !ok {
		t.Fatalf("tool = %#v, want object", tools[0])
	}
	if _, ok := tool["googleSearch"].(map[string]interface{}); !ok {
		t.Fatalf("tool = %#v, want googleSearch object", tool)
	}
}

func TestApplyGenerateImageGoogleSearchToolOmitted(t *testing.T) {
	nativeReq := map[string]interface{}{}
	applyGenerateImageGoogleSearchTool(nativeReq, nil)
	if _, ok := nativeReq["tools"]; ok {
		t.Fatalf("tools should be omitted by default, got %#v", nativeReq["tools"])
	}

	disabled := false
	applyGenerateImageGoogleSearchTool(nativeReq, &disabled)
	if _, ok := nativeReq["tools"]; ok {
		t.Fatalf("tools should be omitted when google_search is false, got %#v", nativeReq["tools"])
	}
}

func TestGenerateImageToAsyncRequestPreservesThinkingConfig(t *testing.T) {
	thinkingLevel := "high"
	includeThoughts := false
	asyncReq := generateImageToAsyncRequest(&dto.GenerateImageRequest{
		Model:           "gemini-3.1-flash-image-preview",
		Prompt:          "draw",
		MediaResolution: "MEDIA_RESOLUTION_HIGH",
		ThinkingLevel:   &thinkingLevel,
		IncludeThoughts: &includeThoughts,
	})

	if asyncReq.MediaResolution != "MEDIA_RESOLUTION_HIGH" {
		t.Fatalf("MediaResolution = %q, want MEDIA_RESOLUTION_HIGH", asyncReq.MediaResolution)
	}
	if asyncReq.ThinkingLevel == nil || *asyncReq.ThinkingLevel != "high" {
		t.Fatalf("ThinkingLevel = %v, want high", asyncReq.ThinkingLevel)
	}
	if asyncReq.IncludeThoughts == nil || *asyncReq.IncludeThoughts {
		t.Fatalf("IncludeThoughts = %v, want explicit false", asyncReq.IncludeThoughts)
	}
}

func TestGenerateImageToAsyncRequestPreservesGPTImageOutputOptions(t *testing.T) {
	background := "transparent"
	moderation := "low"
	outputFormat := "webp"
	asyncReq := generateImageToAsyncRequest(&dto.GenerateImageRequest{
		Model:        "gpt-image-2-c-sp",
		Prompt:       "draw",
		Background:   &background,
		Moderation:   &moderation,
		OutputFormat: &outputFormat,
	})

	require.NotNil(t, asyncReq.Background)
	assert.Equal(t, "transparent", *asyncReq.Background)
	require.NotNil(t, asyncReq.Moderation)
	assert.Equal(t, "low", *asyncReq.Moderation)
	require.NotNil(t, asyncReq.OutputFormat)
	assert.Equal(t, "webp", *asyncReq.OutputFormat)
}

func TestGenerateImageToAsyncRequestPreservesSeedreamWatermark(t *testing.T) {
	watermark := false
	asyncReq := generateImageToAsyncRequest(&dto.GenerateImageRequest{
		Model:     "dola-seedream-5-0-pro-260628-ep",
		Prompt:    "draw",
		Watermark: &watermark,
	})

	require.NotNil(t, asyncReq.Watermark)
	assert.False(t, *asyncReq.Watermark)
}

func TestBuildGenerateImageBillingRequestInputPreservesCountsWithoutPayloads(t *testing.T) {
	first := "https://example.com/secret-first.png"
	second := "data:image/png;base64,SECRET_BASE64"
	input, err := buildGenerateImageBillingRequestInput(&dto.GenerateImageRequest{
		Model:  "dola-seedream-5-0-pro-260628-ep",
		Prompt: "private prompt",
		Size:   "2K",
		Images: []dto.GenerateImageInput{{Value: &first}, {Value: &second}},
	})
	require.NoError(t, err)
	assert.NotContains(t, string(input.Body), "private prompt")
	assert.NotContains(t, string(input.Body), "secret-first")
	assert.NotContains(t, string(input.Body), "SECRET_BASE64")

	cost, trace, err := billingexpr.RunExprWithRequest(
		`param("size") == "1K" ? tier("standard", 45000 + (param("image.#") - 1) * 3000) : tier("high_resolution", 90000 + (param("image.#") - 1) * 3000)`,
		billingexpr.TokenParams{},
		input,
	)
	require.NoError(t, err)
	assert.Equal(t, 93000.0, cost)
	assert.Equal(t, "high_resolution", trace.MatchedTier)
}

func TestGenerateImageToAsyncRequestNormalizesExplicitImageInputs(t *testing.T) {
	legacyValue := "https://example.com/legacy.png"
	request := &dto.GenerateImageRequest{
		Model:  "nano-banana-pro",
		Prompt: "draw",
		Images: []dto.GenerateImageInput{
			{Value: &legacyValue},
			{InlineData: &dto.GenerateImageInlineData{MimeType: "image/png", Data: "AAAA"}},
			{FileData: &dto.GenerateImageFileData{MimeType: "image/jpeg", FileURI: "https://example.com/file.jpg"}},
		},
	}

	asyncRequest := generateImageToAsyncRequest(request)
	require.Len(t, asyncRequest.Images, 3)
	assert.Equal(t, legacyValue, asyncRequest.Images[0])
	assert.Equal(t, "data:image/png;base64,AAAA", asyncRequest.Images[1])
	assert.Equal(t, "https://example.com/file.jpg", asyncRequest.Images[2])
}
