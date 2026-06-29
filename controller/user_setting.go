package controller

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// GetUserSettings retrieves user settings
func GetUserSettings(c *gin.Context) {
	userId := c.GetInt("id")

	setting, err := model.GetUserAPIKeySetting(userId)
	if err != nil {
		// Return empty settings if not found
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"serviceinference_api_key": "",
			},
		})
		return
	}

	// Mask the API key (only show first and last 4 characters)
	maskedKey := ""
	if len(setting.ServiceInferenceAPIKey) > 8 {
		maskedKey = setting.ServiceInferenceAPIKey[:4] + "****" + setting.ServiceInferenceAPIKey[len(setting.ServiceInferenceAPIKey)-4:]
	} else if setting.ServiceInferenceAPIKey != "" {
		maskedKey = "****"
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"serviceinference_api_key": maskedKey,
		},
	})
}

// UpdateUserSettings updates user settings
func UpdateUserSettings(c *gin.Context) {
	userId := c.GetInt("id")

	var req struct {
		ServiceInferenceAPIKey string `json:"serviceinference_api_key"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}

	// Validate API key format
	apiKey := strings.TrimSpace(req.ServiceInferenceAPIKey)
	if apiKey == "" {
		common.ApiErrorMsg(c, "API Key 不能为空")
		return
	}

	if !strings.HasPrefix(apiKey, "sk-inf-") {
		common.ApiErrorMsg(c, "API Key 格式不正确，应以 sk-inf- 开头")
		return
	}

	// Save settings
	err := model.UpdateUserAPIKeySetting(userId, apiKey)
	if err != nil {
		common.ApiErrorMsg(c, fmt.Sprintf("保存设置失败: %v", err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "设置保存成功",
	})
}

// TestServiceInferenceKey tests if a ServiceInference API key is valid
func TestServiceInferenceKey(c *gin.Context) {
	var req struct {
		APIKey string `json:"api_key"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}

	apiKey := strings.TrimSpace(req.APIKey)
	if apiKey == "" {
		common.ApiErrorMsg(c, "API Key 不能为空")
		return
	}

	// Test the API key by making a simple request to ServiceInference
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// Try to list asset groups (simple GET request to test auth)
	httpReq, err := http.NewRequestWithContext(ctx, "GET", "https://model.service-inference.ai/v1/asset-groups?limit=1", nil)
	if err != nil {
		common.ApiErrorMsg(c, fmt.Sprintf("创建请求失败: %v", err))
		return
	}

	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		common.ApiErrorMsg(c, fmt.Sprintf("请求失败: %v", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		common.ApiErrorMsg(c, "API Key 无效或已过期")
		return
	}

	if resp.StatusCode >= 400 {
		common.ApiErrorMsg(c, fmt.Sprintf("API 返回错误: HTTP %d", resp.StatusCode))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "API Key 验证成功",
	})
}
