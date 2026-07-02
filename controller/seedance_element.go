package controller

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// seedance_element.go provides Seedance (ServiceInference) subject management
// for Seedance 2.0 digital human feature. When a subject is created, the image
// is immediately uploaded to ServiceInference as an Asset, and the Asset ID is
// stored for reuse in video generation.

const seedanceElementImageFolder = "subject"

// CreateSeedanceElementRequest is the inbound JSON for creating a Seedance subject.
type CreateSeedanceElementRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	ImageURL    string `json:"image_url" binding:"required"`
	ChannelID   int    `json:"channel_id"` // Optional: specify which ServiceInference channel to use
}

// CreateSeedanceElement creates a new Seedance digital human subject.
// It uploads the image to ServiceInference, creates an Asset, and stores the Asset ID.
func CreateSeedanceElement(c *gin.Context) {
	userId := c.GetInt("id")
	tokenId := c.GetInt("token_id")
	tokenName := c.GetString("token_name")

	var req CreateSeedanceElementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	// Validate image URL format
	imageURL := strings.TrimSpace(req.ImageURL)
	if imageURL == "" {
		common.ApiErrorMsg(c, "image_url 不能为空")
		return
	}
	if !strings.HasPrefix(imageURL, "http://") && !strings.HasPrefix(imageURL, "https://") {
		common.ApiErrorMsg(c, "image_url 必须是 http:// 或 https:// 开头的 URL")
		return
	}

	// Get a ServiceInference channel for this user
	channel, err := getServiceInferenceChannel(userId, req.ChannelID)
	if err != nil {
		common.ApiErrorMsg(c, fmt.Sprintf("无法获取 ServiceInference 渠道: %v", err))
		return
	}

	// Create asset service
	baseURL := ""
	if channel.BaseURL != nil {
		baseURL = *channel.BaseURL
	}

	// Try to get user's personal API key first
	apiKey := channel.Key
	userSetting, err := model.GetUserAPIKeySetting(userId)
	if err == nil && userSetting != nil && userSetting.ServiceInferenceAPIKey != "" {
		// Use user's personal API key
		apiKey = userSetting.ServiceInferenceAPIKey
		fmt.Printf("[INFO] Using user's personal ServiceInference API key for user %d\n", userId)
	} else {
		// Use channel's API key
		fmt.Printf("[INFO] Using channel ServiceInference API key for user %d (key length: %d, prefix: %s)\n", userId, len(apiKey), apiKey[:min(15, len(apiKey))])
	}

	assetService := service.NewSeedanceAssetService(baseURL, apiKey)

	// Create or get asset group
	ctx, cancel := context.WithTimeout(c.Request.Context(), 90*time.Second)
	defer cancel()

	groupID, err := ensureAssetGroupForUser(ctx, assetService, userId, tokenId)
	if err != nil {
		common.ApiErrorMsg(c, fmt.Sprintf("创建素材组失败: %v", err))
		return
	}

	// Upload image and create asset (this is the digital human!)
	fmt.Printf("[INFO] Creating Seedance asset for user %d, image: %s\n", userId, imageURL)
	assetID, err := assetService.CreateAsset(ctx, groupID, imageURL, req.Name)
	if err != nil {
		common.ApiErrorMsg(c, fmt.Sprintf("创建数字人素材失败: %v", err))
		return
	}

	fmt.Printf("[INFO] Seedance asset created successfully: %s\n", assetID)

	// Create the subject record with Asset ID
	element := &model.AigcElement{
		UserId:       userId,
		TokenId:      tokenId,
		TokenName:    tokenName,
		Platform:     model.AigcElementPlatformSeedance,
		Name:         strings.TrimSpace(req.Name),
		Description:  strings.TrimSpace(req.Description),
		FrontalImage: imageURL,
		ElementId:    assetID, // Store the Asset ID here!
		JobId:        groupID, // Store the Asset Group ID for reuse
		Status:       "succeed",
	}

	if err := element.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, element)
}

// getServiceInferenceChannel retrieves a ServiceInference channel for the user
func getServiceInferenceChannel(userId, channelID int) (*model.Channel, error) {
	// If specific channel ID provided, try to get it
	if channelID > 0 {
		channel, err := model.GetChannelById(channelID, true)
		if err != nil {
			return nil, fmt.Errorf("渠道 %d 不存在", channelID)
		}
		if channel.Type != 60 { // ServiceInference channel type
			return nil, fmt.Errorf("渠道 %d 不是 ServiceInference 类型", channelID)
		}
		return channel, nil
	}

	// Otherwise, get any available ServiceInference channel for this user
	// GetChannelsByType omits the key field, so we need to get the full channel
	channels, err := model.GetChannelsByType(0, 100, false, 60)
	if err != nil || len(channels) == 0 {
		return nil, fmt.Errorf("没有可用的 ServiceInference 渠道，请先在后台添加类型 60 的渠道")
	}

	// Find the first enabled channel and get its full info including key
	for _, ch := range channels {
		if ch.Status == 1 { // enabled
			// Get full channel info with key using GetChannelById
			fullChannel, err := model.GetChannelById(ch.Id, true)
			if err == nil {
				return fullChannel, nil
			}
		}
	}

	return nil, fmt.Errorf("没有已启用的 ServiceInference 渠道")
}

// ensureAssetGroupForUser creates or retrieves an asset group for the user
func ensureAssetGroupForUser(ctx context.Context, svc *service.SeedanceAssetService, userId, tokenId int) (string, error) {
	// Try to find existing asset group from user's previous subjects
	existingElement, err := model.GetUserSeedanceElementWithAssetGroup(userId)
	if err == nil && existingElement != nil && existingElement.JobId != "" {
		// JobId stores the asset group ID
		assetGroupID := existingElement.JobId

		if strings.HasPrefix(assetGroupID, "group-") {
			_, err := svc.GetAssetGroup(ctx, assetGroupID)
			if err == nil {
				fmt.Printf("[INFO] Reusing existing asset group: %s\n", assetGroupID)
				return assetGroupID, nil
			}
			fmt.Printf("[WARN] Previous asset group %s no longer exists, creating new one\n", assetGroupID)
		}
	}

	// Create new asset group
	groupName := fmt.Sprintf("user-%d-token-%d", userId, tokenId)
	groupDesc := fmt.Sprintf("Seedance subjects for user %d", userId)

	fmt.Printf("[INFO] Creating new asset group: %s\n", groupName)
	groupID, err := svc.CreateAssetGroup(ctx, groupName, groupDesc)
	if err != nil {
		return "", err
	}

	fmt.Printf("[INFO] Asset group created: %s\n", groupID)

	// Store the group ID in JobId field for future reuse
	// (We'll update the element after it's created)

	return groupID, nil
}

// GetSeedanceElements lists Seedance subjects. Regular users see only their own;
// admins with ?all=true see everyone's (with usernames populated).
func GetSeedanceElements(c *gin.Context) {
	page, _ := strconv.Atoi(c.Query("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(c.Query("per_page"))
	if perPage <= 0 || perPage > 100 {
		perPage = 10
	}

	userId := c.GetInt("id")
	isAdmin := userId == 1 || c.GetBool("is_admin")
	showAll := c.Query("all") == "true"

	var elements []*model.AigcElement
	var total int64
	var err error

	if isAdmin && showAll {
		elements, total, err = model.GetAllAigcElements(
			model.AigcElementPlatformSeedance,
			(page-1)*perPage,
			perPage,
		)
	} else {
		elements, total, err = model.GetUserAigcElements(
			userId,
			model.AigcElementPlatformSeedance,
			(page-1)*perPage,
			perPage,
		)
	}

	if err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    elements,
		"total":   total,
		"page":    page,
		"perPage": perPage,
	})
}

// ListMySeedanceElements returns all Seedance subjects for the current user
// with status=succeed, without pagination. Designed for UI dropdowns/selection.
func ListMySeedanceElements(c *gin.Context) {
	userId := c.GetInt("id")

	elements, err := model.GetUserSuccessAigcElements(userId, model.AigcElementPlatformSeedance)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, elements)
}

// DeleteSeedanceElement deletes a Seedance subject by ID.
func DeleteSeedanceElement(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "无效的 ID")
		return
	}

	userId := c.GetInt("id")
	element, err := model.GetAigcElementById(id, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// Check ownership (unless admin)
	isAdmin := userId == 1 || c.GetBool("is_admin")
	if !isAdmin && element.UserId != userId {
		common.ApiErrorMsg(c, "无权删除此主体")
		return
	}

	// Check platform
	if element.Platform != model.AigcElementPlatformSeedance {
		common.ApiErrorMsg(c, "该主体不属于 Seedance 平台")
		return
	}

	if err := model.DeleteAigcElementById(id, userId); err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, gin.H{"message": "删除成功"})
}
