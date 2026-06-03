package controller

import (
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/gin-gonic/gin"
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
	thinkingLevel := "High"
	includeThoughts := false
	asyncReq := generateImageToAsyncRequest(&dto.GenerateImageRequest{
		Model:           "nano-banana-pro",
		Prompt:          "draw",
		ThinkingLevel:   &thinkingLevel,
		IncludeThoughts: &includeThoughts,
	})

	if asyncReq.ThinkingLevel == nil || *asyncReq.ThinkingLevel != "High" {
		t.Fatalf("ThinkingLevel = %v, want High", asyncReq.ThinkingLevel)
	}
	if asyncReq.IncludeThoughts == nil || *asyncReq.IncludeThoughts {
		t.Fatalf("IncludeThoughts = %v, want explicit false", asyncReq.IncludeThoughts)
	}
}
