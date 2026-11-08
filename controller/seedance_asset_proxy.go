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
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// UnifiedSeedanceAssetRequest is the stable downstream contract for creating
// Seedance assets. Type selects the provider-specific creation workflow.
type UnifiedSeedanceAssetRequest struct {
	Type      string `json:"type"`
	URL       string `json:"url"`
	Name      string `json:"name,omitempty"`
	AssetType string `json:"asset_type,omitempty"`
	Model     string `json:"model,omitempty"`
}

type serviceInferenceAssetRequest struct {
	URL       string `json:"URL"`
	Name      string `json:"Name,omitempty"`
	AssetType string `json:"AssetType"`
	Model     string `json:"Model,omitempty"`
}

func seedanceAssetBasePath(assetWorkflow string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(assetWorkflow)) {
	case "hc":
		return "/v1/sd/assets", true
	case "df":
		return "/v1/sd-5/assets", true
	case "doubao":
		return "/v2/db-sd-max/assets", true
	case "dreamina":
		return "/v2/sd-max/assets", true
	default:
		return "", false
	}
}

// CreateUnifiedSeedanceAsset dispatches a stable downstream request to the
// selected Seedance asset workflow. It is additive and does not replace any
// existing provider-specific asset endpoint.
func CreateUnifiedSeedanceAsset(c *gin.Context) {
	var request UnifiedSeedanceAssetRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		seedanceAssetInvalidRequest(c, "invalid request body: "+err.Error())
		return
	}

	if _, _, err := buildUnifiedSeedanceAssetRequest(request); err != nil {
		seedanceAssetInvalidRequest(c, err.Error())
		return
	}
	family := seedanceAssetFamilyFromWorkflow(request.Type)
	channel, err := selectSeedanceAssetChannel(seedanceAssetRoutingGroups(c), family, request.Model)
	if err != nil {
		common.ApiErrorMsg(c, fmt.Sprintf("无法获取 ServiceInference 渠道: %v", err))
		return
	}
	request.Model = mappedSeedanceAssetModel(channel, request.Model)
	upstreamPath, body, err := buildUnifiedSeedanceAssetRequest(request)
	if err != nil {
		seedanceAssetInvalidRequest(c, err.Error())
		return
	}

	proxySeedanceAssetAPIWithChannel(c, upstreamPath, "", bytes.NewReader(body), "application/json", channel)
}

// GetUnifiedSeedanceAsset dispatches a stable downstream status query to the
// selected Seedance asset workflow.
func GetUnifiedSeedanceAsset(c *gin.Context) {
	upstreamPath, err := buildUnifiedSeedanceAssetQuery(c.Query("type"), c.Param("asset_id"))
	if err != nil {
		seedanceAssetInvalidRequest(c, err.Error())
		return
	}
	family := seedanceAssetFamilyFromWorkflow(c.Query("type"))
	channel, err := selectSeedanceAssetChannel(seedanceAssetRoutingGroups(c), family, "")
	if err != nil {
		common.ApiErrorMsg(c, fmt.Sprintf("无法获取 ServiceInference 渠道: %v", err))
		return
	}

	proxySeedanceAssetAPIWithChannel(c, upstreamPath, "", nil, "", channel)
}

func seedanceAssetInvalidRequest(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, gin.H{
		"error": gin.H{
			"message": message,
			"type":    "invalid_request_error",
		},
	})
}

func buildUnifiedSeedanceAssetRequest(request UnifiedSeedanceAssetRequest) (string, []byte, error) {
	upstreamPath, ok := seedanceAssetBasePath(request.Type)
	if !ok {
		return "", nil, fmt.Errorf("unsupported type %q; currently supported: hc, df, doubao, dreamina", request.Type)
	}

	assetWorkflow := strings.ToLower(strings.TrimSpace(request.Type))
	sourceURL := strings.TrimSpace(request.URL)
	parsedURL, err := url.ParseRequestURI(sourceURL)
	validScheme := parsedURL != nil && parsedURL.Scheme == "https"
	if assetWorkflow == "doubao" || assetWorkflow == "dreamina" {
		validScheme = parsedURL != nil && (parsedURL.Scheme == "http" || parsedURL.Scheme == "https")
	}
	if err != nil || !validScheme || parsedURL.Host == "" {
		if assetWorkflow == "doubao" || assetWorkflow == "dreamina" {
			return "", nil, fmt.Errorf("url must be a public HTTP(S) URL")
		}
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

	assetModel := ""
	if assetWorkflow == "doubao" || assetWorkflow == "dreamina" {
		assetModel = strings.TrimSpace(request.Model)
	}
	body, err := common.Marshal(serviceInferenceAssetRequest{
		URL:       sourceURL,
		Name:      strings.TrimSpace(request.Name),
		AssetType: assetType,
		Model:     assetModel,
	})
	if err != nil {
		return "", nil, fmt.Errorf("marshal seedance asset request: %w", err)
	}
	return upstreamPath, body, nil
}

func buildUnifiedSeedanceAssetQuery(assetWorkflow string, assetID string) (string, error) {
	upstreamPath, ok := seedanceAssetBasePath(assetWorkflow)
	if !ok {
		return "", fmt.Errorf("unsupported type %q; currently supported: hc, df, doubao, dreamina", assetWorkflow)
	}

	assetID = strings.TrimSpace(assetID)
	if assetID == "" {
		return "", fmt.Errorf("asset_id is required")
	}
	return upstreamPath + "/" + url.PathEscape(assetID), nil
}

// ProxySeedanceAssetAPI transparently forwards ServiceInference asset-management
// calls (/v1/asset-groups*, /v1/assets*, /v1/sd/assets*, /v1/sd-5/assets*,
// /v2/db-sd-max/assets*, /v2/sd-max/assets*) to the upstream provider.
// Family-specific asset routes use the type-60 channel that serves the same
// models as video generation. Legacy grouped asset routes still use the first
// enabled type-60 channel.
func ProxySeedanceAssetAPI(c *gin.Context) {
	proxySeedanceAssetAPIWithChannel(c, c.Request.URL.Path, c.Request.URL.RawQuery, c.Request.Body, c.GetHeader("Content-Type"), nil)
}

func proxySeedanceAssetAPI(c *gin.Context, upstreamPath string, rawQuery string, body io.Reader, contentType string) {
	proxySeedanceAssetAPIWithChannel(c, upstreamPath, rawQuery, body, contentType, nil)
}

func isRememberedSeedanceAssetPath(path string) bool {
	switch path {
	case "/v1/sd/assets", "/v1/sd-5/assets", "/v2/db-sd-max/assets", "/v2/sd-max/assets":
		return true
	default:
		return false
	}
}

func proxySeedanceAssetAPIWithChannel(c *gin.Context, upstreamPath string, rawQuery string, body io.Reader, contentType string, channel *model.Channel) {
	var creationBody []byte
	routingModel := ""
	if c.Request.Method == http.MethodPost && body != nil && isRememberedSeedanceAssetPath(upstreamPath) {
		prefix, readErr := io.ReadAll(io.LimitReader(body, 16*1024+1))
		if readErr != nil {
			common.ApiErrorMsg(c, "读取素材创建请求失败")
			return
		}
		body = io.MultiReader(bytes.NewReader(prefix), body)
		if len(prefix) <= 16*1024 {
			creationBody = prefix
			routingModel = seedanceAssetModelFromBody(prefix)
		}
	}

	if channel == nil {
		var err error
		if family := seedanceAssetFamilyFromPath(upstreamPath); family != "" {
			channel, err = selectSeedanceAssetChannel(seedanceAssetRoutingGroups(c), family, routingModel)
		} else {
			channel, err = getServiceInferenceChannel(0, 0)
		}
		if err != nil {
			common.ApiErrorMsg(c, fmt.Sprintf("无法获取 ServiceInference 渠道: %v", err))
			return
		}
	}

	baseURL := "https://model.service-inference.ai"
	if channel.BaseURL != nil && strings.TrimSpace(*channel.BaseURL) != "" {
		baseURL = strings.TrimRight(strings.TrimSpace(*channel.BaseURL), "/")
	}

	upstreamURL := baseURL + upstreamPath
	if rawQuery != "" {
		upstreamURL += "?" + rawQuery
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
	if requestID := resp.Header.Get("x-request-id"); requestID != "" {
		c.Header("x-request-id", requestID)
	}
	if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
		c.Header("Retry-After", retryAfter)
	}
	if len(creationBody) > 0 && resp.StatusCode >= 200 && resp.StatusCode < 300 {
		prefix, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024+1))
		if len(prefix) <= 64*1024 {
			rememberSeedanceAssetCreation(c, upstreamPath, creationBody, prefix)
		}
		_, _ = io.Copy(c.Writer, io.MultiReader(bytes.NewReader(prefix), resp.Body))
		return
	}
	_, _ = io.Copy(c.Writer, resp.Body)
}

func rememberSeedanceAssetCreation(c *gin.Context, path string, requestBody, responseBody []byte) {
	var request serviceInferenceAssetRequest
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			ID string `json:"Id"`
		} `json:"data"`
	}
	if common.Unmarshal(requestBody, &request) != nil || common.Unmarshal(responseBody, &response) != nil || !response.Success || response.Data.ID == "" {
		return
	}
	sourceURL, err := url.Parse(request.URL)
	if err != nil || sourceURL.Host == "" || (sourceURL.Scheme != "http" && sourceURL.Scheme != "https") {
		return
	}
	// Store only the documented creation fields, never arbitrary caller data.
	data, err := common.Marshal(request)
	if err != nil {
		return
	}
	if err := model.RememberSeedanceAssetSource(c.Request.Context(), c.GetInt("id"), response.Data.ID, model.SeedanceAssetRecreateRequest{Path: path, Body: data}); err != nil {
		common.SysError("seedance asset recovery: failed to retain creation source")
	}
}
