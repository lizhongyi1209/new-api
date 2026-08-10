package controller

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
)

// UnifiedSeedanceAssetRequest is the stable downstream contract for creating
// Seedance assets. Type selects the provider-specific creation workflow.
type UnifiedSeedanceAssetRequest struct {
	Type      string `json:"type"`
	URL       string `json:"url"`
	Name      string `json:"name,omitempty"`
	AssetType string `json:"asset_type,omitempty"`
}

type serviceInferenceHCAssetRequest struct {
	URL       string `json:"URL"`
	Name      string `json:"Name,omitempty"`
	AssetType string `json:"AssetType"`
}

// CreateUnifiedSeedanceAsset dispatches a stable downstream request to the
// selected Seedance asset workflow. It is additive and does not replace any
// existing provider-specific asset endpoint.
func CreateUnifiedSeedanceAsset(c *gin.Context) {
	var request UnifiedSeedanceAssetRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": "invalid request body: " + err.Error(),
				"type":    "invalid_request_error",
			},
		})
		return
	}

	upstreamPath, body, err := buildUnifiedSeedanceAssetRequest(request)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": err.Error(),
				"type":    "invalid_request_error",
			},
		})
		return
	}

	proxySeedanceAssetAPI(c, upstreamPath, bytes.NewReader(body), "application/json")
}

func buildUnifiedSeedanceAssetRequest(request UnifiedSeedanceAssetRequest) (string, []byte, error) {
	assetWorkflow := strings.ToLower(strings.TrimSpace(request.Type))
	if assetWorkflow != "hc" {
		return "", nil, fmt.Errorf("unsupported type %q; currently supported: hc", request.Type)
	}

	sourceURL := strings.TrimSpace(request.URL)
	parsedURL, err := url.ParseRequestURI(sourceURL)
	if err != nil || parsedURL.Scheme != "https" || parsedURL.Host == "" {
		return "", nil, fmt.Errorf("url must be a public HTTPS URL")
	}

	assetType := strings.ToLower(strings.TrimSpace(request.AssetType))
	switch assetType {
	case "", "image":
		assetType = "Image"
	case "video":
		assetType = "Video"
	case "audio":
		assetType = "Audio"
	default:
		return "", nil, fmt.Errorf("asset_type must be image, video, or audio")
	}

	body, err := common.Marshal(serviceInferenceHCAssetRequest{
		URL:       sourceURL,
		Name:      strings.TrimSpace(request.Name),
		AssetType: assetType,
	})
	if err != nil {
		return "", nil, fmt.Errorf("marshal hc asset request: %w", err)
	}
	return "/v1/sd/assets", body, nil
}

// ProxySeedanceAssetAPI transparently forwards ServiceInference asset-management
// calls (/v1/asset-groups*, /v1/assets*, /v1/sd/assets*) to the upstream provider
// using this instance's type-60 channel credentials. Sub-stations point their
// type-60 channel at this gateway with a gateway token, so the real upstream key
// never leaves the main site.
func ProxySeedanceAssetAPI(c *gin.Context) {
	proxySeedanceAssetAPI(c, c.Request.URL.Path, c.Request.Body, c.GetHeader("Content-Type"))
}

func proxySeedanceAssetAPI(c *gin.Context, upstreamPath string, body io.Reader, contentType string) {
	channel, err := getServiceInferenceChannel(0, 0)
	if err != nil {
		common.ApiErrorMsg(c, fmt.Sprintf("无法获取 ServiceInference 渠道: %v", err))
		return
	}

	baseURL := "https://model.service-inference.ai"
	if channel.BaseURL != nil && strings.TrimSpace(*channel.BaseURL) != "" {
		baseURL = strings.TrimRight(strings.TrimSpace(*channel.BaseURL), "/")
	}

	upstreamURL := baseURL + upstreamPath
	if c.Request.URL.RawQuery != "" {
		upstreamURL += "?" + c.Request.URL.RawQuery
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), c.Request.Method, upstreamURL, body)
	if err != nil {
		common.ApiErrorMsg(c, fmt.Sprintf("构造上游请求失败: %v", err))
		return
	}
	req.Header.Set("Authorization", "Bearer "+channel.Key)
	if contentType != "" {
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
