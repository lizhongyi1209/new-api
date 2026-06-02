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
