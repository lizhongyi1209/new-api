package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// seedance_element.go provides Seedance (ServiceInference) subject management
// APIs. Unlike Kling which requires creating subjects on the provider side,
// Seedance subjects are simply metadata records; the actual asset upload to
// ServiceInference happens automatically during video generation by the
// serviceinference adaptor.

const seedanceElementImageFolder = "seedance"

// CreateSeedanceElementRequest is the inbound JSON for creating a Seedance subject.
type CreateSeedanceElementRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	ImageURL    string `json:"image_url" binding:"required"`
}

// CreateSeedanceElement creates a new Seedance subject record. The image URL
// is stored as-is; the serviceinference adaptor will automatically upload it
// to the asset-group during video generation.
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

	// Create the subject record
	element := &model.AigcElement{
		UserId:       userId,
		TokenId:      tokenId,
		TokenName:    tokenName,
		Platform:     model.AigcElementPlatformSeedance,
		Name:         strings.TrimSpace(req.Name),
		Description:  strings.TrimSpace(req.Description),
		FrontalImage: imageURL,
		Status:       "succeed", // Seedance subjects are immediately usable
	}

	if err := element.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, element)
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

	common.ApiSuccess(c, gin.H{
		"data":     elements,
		"total":    total,
		"page":     page,
		"per_page": perPage,
	})
}

// ListMySeedanceElements returns all of the current user's Seedance subjects
// without pagination, newest first. Only subjects with status=succeed are
// returned (ready to use in video generation).
func ListMySeedanceElements(c *gin.Context) {
	userId := c.GetInt("id")
	elements, err := model.ListUserAigcElements(
		userId,
		model.AigcElementPlatformSeedance,
		true, // onlySucceed
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, elements)
}

// DeleteSeedanceElement deletes a Seedance subject. Regular users can only
// delete their own; admins can delete any.
func DeleteSeedanceElement(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "无效的 ID")
		return
	}

	userId := c.GetInt("id")
	isAdmin := userId == 1 || c.GetBool("is_admin")

	// Scope: admin can delete any, regular user only their own
	scope := userId
	if isAdmin {
		scope = 0
	}

	// Verify the element exists and belongs to the correct platform
	element, err := model.GetAigcElementById(id, scope)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if element.Platform != model.AigcElementPlatformSeedance {
		common.ApiErrorMsg(c, "该主体不是 Seedance 平台的主体")
		return
	}

	if err := model.DeleteAigcElementById(id, scope); err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, gin.H{"id": id})
}
