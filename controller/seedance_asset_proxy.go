package controller

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
)

// ProxySeedanceAssetAPI transparently forwards ServiceInference asset-management
// calls (/v1/asset-groups*, /v1/assets*, /v1/sd/assets*) to the upstream provider
// using this instance's type-60 channel credentials. Sub-stations point their
// type-60 channel at this gateway with a gateway token, so the real upstream key
// never leaves the main site.
func ProxySeedanceAssetAPI(c *gin.Context) {
	channel, err := getServiceInferenceChannel(0, 0)
	if err != nil {
		common.ApiErrorMsg(c, fmt.Sprintf("无法获取 ServiceInference 渠道: %v", err))
		return
	}

	baseURL := "https://model.service-inference.ai"
	if channel.BaseURL != nil && strings.TrimSpace(*channel.BaseURL) != "" {
		baseURL = strings.TrimRight(strings.TrimSpace(*channel.BaseURL), "/")
	}

	upstreamURL := baseURL + c.Request.URL.Path
	if c.Request.URL.RawQuery != "" {
		upstreamURL += "?" + c.Request.URL.RawQuery
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), c.Request.Method, upstreamURL, c.Request.Body)
	if err != nil {
		common.ApiErrorMsg(c, fmt.Sprintf("构造上游请求失败: %v", err))
		return
	}
	req.Header.Set("Authorization", "Bearer "+channel.Key)
	if contentType := c.GetHeader("Content-Type"); contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		common.ApiErrorMsg(c, fmt.Sprintf("请求 ServiceInference 失败: %v", err))
		return
	}
	defer resp.Body.Close()

	c.Status(resp.StatusCode)
	if contentType := resp.Header.Get("Content-Type"); contentType != "" {
		c.Header("Content-Type", contentType)
	}
	_, _ = io.Copy(c.Writer, resp.Body)
}
