package middleware

import (
	"bytes"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"

	"github.com/gin-gonic/gin"
)

func KlingRequestConvert() func(c *gin.Context) {
	return func(c *gin.Context) {
		var originalReq map[string]interface{}
		if err := common.UnmarshalBodyReusable(c, &originalReq); err != nil {
			c.Next()
			return
		}

		// Support both model_name and model fields
		model, _ := originalReq["model_name"].(string)
		if model == "" {
			model, _ = originalReq["model"].(string)
		}
		prompt, _ := originalReq["prompt"].(string)

		// Extract common fields if present (parity with KlingOmniVideoConvert)
		mode, _ := originalReq["mode"].(string)
		duration, _ := originalReq["duration"].(string)
		aspectRatio, _ := originalReq["aspect_ratio"].(string)

		unifiedReq := map[string]interface{}{
			"model":    model,
			"prompt":   prompt,
			"metadata": originalReq,
		}

		// Promote optional params to top level so they populate TaskSubmitReq
		// (used for usage-log detail: mode/size/duration).
		if mode != "" {
			unifiedReq["mode"] = mode
		}
		if duration != "" {
			unifiedReq["duration"] = duration
		}
		if aspectRatio != "" {
			unifiedReq["size"] = aspectRatio
		}

		jsonData, err := common.Marshal(unifiedReq)
		if err != nil {
			c.Next()
			return
		}

		// Rewrite request body and path
		c.Request.Body = io.NopCloser(bytes.NewBuffer(jsonData))
		c.Request.URL.Path = "/v1/video/generations"
		if image, ok := originalReq["image"]; !ok || image == "" {
			c.Set("action", constant.TaskActionTextGenerate)
		}

		// Overwrite both caches so all subsequent UnmarshalBodyReusable calls see the converted body.
		// KeyBodyStorage takes priority over KeyRequestBody in GetRequestBody, so both must be updated.
		if bs, err := common.CreateBodyStorage(jsonData); err == nil {
			c.Set(common.KeyBodyStorage, bs)
		}
		c.Set(common.KeyRequestBody, jsonData)
		c.Next()
	}
}

// KlingMotionControlConvert converts Kling-native motion-control requests to the unified
// TaskSubmitReq format (model/prompt/metadata) without rewriting the URL path or
// pre-setting the action, so that ValidateRequestAndSetAction can detect the route correctly.
func KlingMotionControlConvert() func(c *gin.Context) {
	return func(c *gin.Context) {
		var originalReq map[string]interface{}
		if err := common.UnmarshalBodyReusable(c, &originalReq); err != nil {
			c.Next()
			return
		}

		model, _ := originalReq["model_name"].(string)
		if model == "" {
			model, _ = originalReq["model"].(string)
		}
		prompt, _ := originalReq["prompt"].(string)

		unifiedReq := map[string]interface{}{
			"model":    model,
			"prompt":   prompt,
			"metadata": originalReq,
		}

		jsonData, err := common.Marshal(unifiedReq)
		if err != nil {
			c.Next()
			return
		}

		c.Request.Body = io.NopCloser(bytes.NewBuffer(jsonData))
		if bs, err := common.CreateBodyStorage(jsonData); err == nil {
			c.Set(common.KeyBodyStorage, bs)
		}
		c.Set(common.KeyRequestBody, jsonData)
		// Do NOT rewrite URL path — ValidateRequestAndSetAction relies on it to detect motion-control.
		// Do NOT set action — let ValidateRequestAndSetAction set TaskActionMotionControl.
		c.Next()
	}
}

// KlingOmniVideoConvert converts Kling-native omni-video requests to the unified
// TaskSubmitReq format (model/prompt/metadata) without rewriting the URL path,
// so that ValidateRequestAndSetAction can detect the route correctly.
func KlingOmniVideoConvert() func(c *gin.Context) {
	return func(c *gin.Context) {
		var originalReq map[string]interface{}
		if err := common.UnmarshalBodyReusable(c, &originalReq); err != nil {
			c.Next()
			return
		}

		model, _ := originalReq["model_name"].(string)
		if model == "" {
			model, _ = originalReq["model"].(string)
		}
		prompt, _ := originalReq["prompt"].(string)

		// Extract common fields if present
		mode, _ := originalReq["mode"].(string)
		duration, _ := originalReq["duration"].(string)
		aspectRatio, _ := originalReq["aspect_ratio"].(string)

		unifiedReq := map[string]interface{}{
			"model":    model,
			"prompt":   prompt,
			"metadata": originalReq,
		}

		// Add optional fields to top level if present
		if mode != "" {
			unifiedReq["mode"] = mode
		}
		if duration != "" {
			unifiedReq["duration"] = duration
		}
		if aspectRatio != "" {
			unifiedReq["size"] = aspectRatio
		}

		jsonData, err := common.Marshal(unifiedReq)
		if err != nil {
			c.Next()
			return
		}

		c.Request.Body = io.NopCloser(bytes.NewBuffer(jsonData))
		if bs, err := common.CreateBodyStorage(jsonData); err == nil {
			c.Set(common.KeyBodyStorage, bs)
		}
		c.Set(common.KeyRequestBody, jsonData)
		// Do NOT rewrite URL path — ValidateRequestAndSetAction relies on it to detect omni-video.
		// Do NOT set action — let ValidateRequestAndSetAction set TaskActionOmniVideo.
		c.Next()
	}
}

// KlingOmniVideo30Convert accepts Kling's current contents/settings/options
// contract and promotes the prompt/model fields needed by the common task flow.
func KlingOmniVideo30Convert() func(c *gin.Context) {
	return func(c *gin.Context) {
		// The model is encoded in the route for Kling's 3.0 API. GET requests
		// have no body, but the distributor still needs the internal model name
		// to authorize the token and resolve the task's channel.
		if c.Request.Method == http.MethodGet {
			jsonData, err := common.Marshal(map[string]interface{}{
				"model": "kling-3.0-omni",
			})
			if err != nil {
				c.Next()
				return
			}
			c.Request.Body = io.NopCloser(bytes.NewBuffer(jsonData))
			c.Request.Header.Set("Content-Type", "application/json")
			if bs, err := common.CreateBodyStorage(jsonData); err == nil {
				c.Set(common.KeyBodyStorage, bs)
			}
			c.Set(common.KeyRequestBody, jsonData)
			c.Next()
			return
		}

		var originalReq struct {
			Contents []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"contents"`
		}
		if err := common.UnmarshalBodyReusable(c, &originalReq); err != nil {
			c.Next()
			return
		}

		prompt := ""
		for _, content := range originalReq.Contents {
			if content.Type == "prompt" {
				prompt = content.Text
				break
			}
		}

		var metadata map[string]interface{}
		if err := common.UnmarshalBodyReusable(c, &metadata); err != nil {
			c.Next()
			return
		}
		unifiedReq := map[string]interface{}{
			"model":    "kling-3.0-omni",
			"prompt":   prompt,
			"metadata": metadata,
		}
		if settings, ok := metadata["settings"].(map[string]interface{}); ok {
			if duration, ok := settings["duration"].(float64); ok {
				unifiedReq["duration"] = int(duration)
			}
			if aspectRatio, ok := settings["aspect_ratio"].(string); ok && aspectRatio != "" {
				unifiedReq["size"] = aspectRatio
			}
			if resolution, ok := settings["resolution"].(string); ok && resolution != "" {
				unifiedReq["mode"] = resolution
			}
		}
		jsonData, err := common.Marshal(unifiedReq)
		if err != nil {
			c.Next()
			return
		}

		c.Request.Body = io.NopCloser(bytes.NewBuffer(jsonData))
		if bs, err := common.CreateBodyStorage(jsonData); err == nil {
			c.Set(common.KeyBodyStorage, bs)
		}
		c.Set(common.KeyRequestBody, jsonData)
		c.Next()
	}
}
