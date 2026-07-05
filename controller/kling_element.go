package controller

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/task/kling"

	"github.com/gin-gonic/gin"
)

// kling_element.go exposes the Kling Official Element Management APIs (2024 New Version).
// Elements are created against an enabled Kling channel and owned by the requesting user.

// resolveKlingChannel returns an enabled Kling channel and its parsed credential.
// When channelId > 0 that specific channel is used, otherwise the first enabled one is picked.
func resolveKlingChannel(channelId int) (*model.Channel, string, string, error) {
	var channel *model.Channel
	if channelId > 0 {
		ch, err := model.GetChannelById(channelId, true)
		if err != nil {
			return nil, "", "", err
		}
		channel = ch
	} else {
		var channels []*model.Channel
		err := model.DB.Where("type = ? AND status = ?",
			constant.ChannelTypeKling, common.ChannelStatusEnabled).
			Find(&channels).Error
		if err != nil {
			return nil, "", "", err
		}
		if len(channels) == 0 {
			return nil, "", "", fmt.Errorf("没有可用的 Kling 渠道")
		}
		channel = channels[0]
	}
	if channel.Type != constant.ChannelTypeKling {
		return nil, "", "", fmt.Errorf("渠道类型不是 Kling")
	}
	key, _, apiErr := channel.GetNextEnabledKey()
	if apiErr != nil {
		return nil, "", "", apiErr
	}
	return channel, key, channel.GetBaseURL(), nil
}

// klingOfficialElementClient builds a Kling official element management client for the given channel.
func klingOfficialElementClient(channel *model.Channel, key string) *kling.OfficialElementClient {
	return kling.NewOfficialElementClient(key, channel.GetBaseURL(), "")
}

// CreateKlingOfficialElementRequest is the inbound JSON for creating an official element.
type CreateKlingOfficialElementRequest struct {
	ChannelId          int      `json:"channel_id"`
	ElementName        string   `json:"element_name"`
	ElementDescription string   `json:"element_description"`
	ReferenceType      string   `json:"reference_type"`
	FrontalImage       string   `json:"frontal_image"`
	ReferImages        []string `json:"refer_images"`
	VideoList          []string `json:"video_list"`
	ElementVoiceId     string   `json:"element_voice_id"`
	TagIds             []string `json:"tag_ids"`
	CallbackUrl        string   `json:"callback_url"`
	ExternalTaskId     string   `json:"external_task_id"`
}

func CreateKlingOfficialElement(c *gin.Context) {
	userId := c.GetInt("id")
	var req CreateKlingOfficialElementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	req.ElementName = strings.TrimSpace(req.ElementName)
	req.ElementDescription = strings.TrimSpace(req.ElementDescription)
	if req.ElementName == "" || req.ElementDescription == "" {
		common.ApiErrorMsg(c, "element_name 和 element_description 不能为空")
		return
	}
	if req.ReferenceType == "" {
		req.ReferenceType = kling.OfficialReferenceTypeImage
	}

	// Validate reference assets
	if req.ReferenceType == kling.OfficialReferenceTypeImage {
		if msg := validateKlingOfficialImageRefs(&req); msg != "" {
			common.ApiErrorMsg(c, msg)
			return
		}
	}

	channel, key, _, err := resolveKlingChannel(req.ChannelId)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	klingReq := &kling.CreateOfficialElementRequest{
		ElementName:        req.ElementName,
		ElementDescription: req.ElementDescription,
		ReferenceType:      req.ReferenceType,
		ElementVoiceId:     req.ElementVoiceId,
		CallbackUrl:        req.CallbackUrl,
		ExternalTaskId:     req.ExternalTaskId,
	}

	if req.ReferenceType == kling.OfficialReferenceTypeImage {
		il := &kling.OfficialElementImageList{FrontalImage: req.FrontalImage}
		for _, u := range req.ReferImages {
			if strings.TrimSpace(u) != "" {
				il.ReferImages = append(il.ReferImages, kling.OfficialReferImageItem{ImageUrl: u})
			}
		}
		klingReq.ElementImageList = il
	} else if req.ReferenceType == kling.OfficialReferenceTypeVideo {
		vl := &kling.OfficialElementVideoList{}
		for _, u := range req.VideoList {
			if strings.TrimSpace(u) != "" {
				vl.ReferVideos = append(vl.ReferVideos, kling.OfficialReferVideoItem{VideoUrl: u})
			}
		}
		klingReq.ElementVideoList = vl
	}

	for _, id := range req.TagIds {
		if strings.TrimSpace(id) != "" {
			klingReq.TagList = append(klingReq.TagList, kling.OfficialTagItem{TagId: id})
		}
	}

	result, err := klingOfficialElementClient(channel, key).CreateElement(klingReq)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// Store reference images and videos for management
	referImagesJSON := ""
	if len(req.ReferImages) > 0 {
		if b, mErr := common.Marshal(req.ReferImages); mErr == nil {
			referImagesJSON = string(b)
		}
	}
	videoListJSON := ""
	if len(req.VideoList) > 0 {
		if b, mErr := common.Marshal(req.VideoList); mErr == nil {
			videoListJSON = string(b)
		}
	}

	element := &model.AigcElement{
		UserId:        userId,
		TokenId:       c.GetInt("token_id"),
		TokenName:     c.GetString("token_name"),
		ChannelId:     channel.Id,
		Platform:      model.AigcElementPlatformKlingOfficial,
		JobId:         result.Data.TaskId, // Use TaskId as JobId
		ElementId:     "",                 // Will be filled when querying task result
		Name:          req.ElementName,
		Description:   req.ElementDescription,
		ReferenceType: req.ReferenceType,
		Status:        defaultStatus(result.Data.TaskStatus),
		FrontalImage:  req.FrontalImage,
		ReferImages:   referImagesJSON,
		VideoList:     videoListJSON,
	}
	if err := element.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, element)
}

func validateKlingOfficialImageRefs(req *CreateKlingOfficialElementRequest) string {
	if strings.TrimSpace(req.FrontalImage) == "" {
		return "缺少正面参考图 (frontal_image)"
	}

	refers := make([]string, 0, len(req.ReferImages))
	for _, u := range req.ReferImages {
		if strings.TrimSpace(u) != "" {
			refers = append(refers, strings.TrimSpace(u))
		}
	}
	if len(refers) < 1 || len(refers) > 3 {
		return fmt.Sprintf("其他参考图需 1~3 张，当前 %d 张", len(refers))
	}

	all := append([]string{strings.TrimSpace(req.FrontalImage)}, refers...)
	for _, u := range all {
		if msg := checkRemoteImage(u); msg != "" {
			return msg
		}
	}
	return ""
}

// GetKlingOfficialElements lists Kling official elements (deprecated, use ListKlingOfficialElements).
func GetKlingOfficialElements(c *gin.Context) {
	ListKlingOfficialElements(c)
}

// ListKlingOfficialElements lists Kling official elements with pagination.
func ListKlingOfficialElements(c *gin.Context) {
	userId := c.GetInt("id")
	pageNum, _ := strconv.Atoi(c.DefaultQuery("pageNum", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "30"))

	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 || pageSize > 500 {
		pageSize = 30
	}

	// Check if user wants to query from Kling API directly
	channelId, _ := strconv.Atoi(c.Query("channel_id"))
	if c.Query("from_api") == "true" && channelId > 0 {
		channel, key, _, err := resolveKlingChannel(channelId)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		result, err := klingOfficialElementClient(channel, key).ListElements(pageNum, pageSize)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		common.ApiSuccess(c, result)
		return
	}

	// Query from local database
	all := c.Query("all") == "true" && isAdmin(c)
	var (
		elements []*model.AigcElement
		total    int64
		err      error
	)
	startIdx := (pageNum - 1) * pageSize
	if all {
		elements, total, err = model.GetAllAigcElements(model.AigcElementPlatformKlingOfficial, startIdx, pageSize)
	} else {
		elements, total, err = model.GetUserAigcElements(userId, model.AigcElementPlatformKlingOfficial, startIdx, pageSize)
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// Return in Kling API format
	common.ApiSuccess(c, gin.H{
		"code":       0,
		"message":    "success",
		"request_id": c.GetString("X-Request-Id"),
		"data":       elements,
		"total":      total,
		"pageNum":    pageNum,
		"pageSize":   pageSize,
	})
}

// ListKlingOfficialPresetsElements lists preset elements from official library.
func ListKlingOfficialPresetsElements(c *gin.Context) {
	pageNum, _ := strconv.Atoi(c.DefaultQuery("pageNum", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "30"))

	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 || pageSize > 500 {
		pageSize = 30
	}

	channelId, _ := strconv.Atoi(c.Query("channel_id"))
	channel, key, _, err := resolveKlingChannel(channelId)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	result, err := klingOfficialElementClient(channel, key).ListPresetsElements(pageNum, pageSize)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

// ListMyKlingOfficialElements returns the Kling official elements owned by the current token's account.
func ListMyKlingOfficialElements(c *gin.Context) {
	userId := c.GetInt("id")
	onlySucceed := c.Query("include_all") != "true"

	elements, err := model.ListUserAigcElements(userId, model.AigcElementPlatformKlingOfficial, onlySucceed)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	items := make([]clientAigcElement, 0, len(elements))
	for _, e := range elements {
		items = append(items, clientAigcElement{
			ElementId:     e.ElementId,
			Name:          e.Name,
			FrontalImage:  e.FrontalImage,
			ReferenceType: e.ReferenceType,
			Platform:      e.Platform,
			Status:        e.Status,
		})
	}
	common.ApiSuccess(c, items)
}

// QueryKlingOfficialElement queries a single Kling official element by task_id.
func QueryKlingOfficialElement(c *gin.Context) {
	taskId := c.Param("task_id")
	if taskId == "" {
		common.ApiErrorMsg(c, "task_id 不能为空")
		return
	}

	// Try to find the element in local database first
	var element *model.AigcElement
	err := model.DB.Where("job_id = ? AND platform = ?", taskId, model.AigcElementPlatformKlingOfficial).
		First(&element).Error
	if err == nil {
		// Found in local DB, query from Kling API to get latest status
		channel, key, _, cerr := resolveKlingChannel(element.ChannelId)
		if cerr != nil {
			common.ApiError(c, cerr)
			return
		}
		result, err := klingOfficialElementClient(channel, key).QueryElement(taskId)
		if err != nil {
			common.ApiError(c, err)
			return
		}

		// Update local database with latest info
		if result.Data.TaskStatus != "" {
			element.Status = result.Data.TaskStatus
		}
		if len(result.Data.TaskResult.Elements) > 0 {
			firstElem := result.Data.TaskResult.Elements[0]
			element.ElementId = fmt.Sprintf("%d", firstElem.ElementId)
			element.Status = firstElem.Status
		}
		if result.Data.TaskStatusMsg != "" {
			element.FailReason = result.Data.TaskStatusMsg
		}
		element.Update()

		common.ApiSuccess(c, result)
		return
	}

	// Not found in local DB, query directly from Kling API (requires channel_id)
	channelId, _ := strconv.Atoi(c.Query("channel_id"))
	if channelId == 0 {
		common.ApiErrorMsg(c, "元素不存在，需要提供 channel_id 参数")
		return
	}

	channel, key, _, err := resolveKlingChannel(channelId)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	result, err := klingOfficialElementClient(channel, key).QueryElement(taskId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

// RefreshKlingOfficialElement re-queries Kling for the element's latest status and updates the local row.
func RefreshKlingOfficialElement(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	element, err := model.GetAigcElementById(id, ownerScope(c))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if element.JobId == "" {
		common.ApiErrorMsg(c, "该主体尚无 TaskId，无法查询")
		return
	}

	channel, key, _, err := resolveKlingChannel(element.ChannelId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	result, err := klingOfficialElementClient(channel, key).QueryElement(element.JobId)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// Update element info from task result
	if result.Data.TaskStatus != "" {
		element.Status = result.Data.TaskStatus
	}
	if len(result.Data.TaskResult.Elements) > 0 {
		firstElem := result.Data.TaskResult.Elements[0]
		element.ElementId = fmt.Sprintf("%d", firstElem.ElementId)
		if firstElem.ElementName != "" {
			element.Name = firstElem.ElementName
		}
		if firstElem.ElementDescription != "" {
			element.Description = firstElem.ElementDescription
		}
		if firstElem.ReferenceType != "" {
			element.ReferenceType = firstElem.ReferenceType
		}
		element.Status = firstElem.Status
	}
	if result.Data.TaskStatusMsg != "" {
		element.FailReason = result.Data.TaskStatusMsg
	}

	if err := element.Update(); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"element": element, "detail": result})
}

// DeleteKlingOfficialElementByBody deletes a Kling official element (element_id in request body).
func DeleteKlingOfficialElementByBody(c *gin.Context) {
	var req struct {
		ElementId string `json:"element_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}

	// Find element in local database
	var element *model.AigcElement
	err := model.DB.Where("element_id = ? AND platform = ?", req.ElementId, model.AigcElementPlatformKlingOfficial).
		First(&element).Error
	if err != nil {
		// Not found locally, still try to delete from Kling API
		channelId, _ := strconv.Atoi(c.Query("channel_id"))
		if channelId == 0 {
			common.ApiErrorMsg(c, "元素不存在")
			return
		}
		channel, key, _, cerr := resolveKlingChannel(channelId)
		if cerr != nil {
			common.ApiError(c, cerr)
			return
		}
		result, err := klingOfficialElementClient(channel, key).DeleteElement(req.ElementId)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		common.ApiSuccess(c, result)
		return
	}

	// Check ownership
	userId := c.GetInt("id")
	if !isAdmin(c) && element.UserId != userId {
		common.ApiErrorMsg(c, "无权删除此元素")
		return
	}

	// Delete from Kling API
	channel, key, _, err := resolveKlingChannel(element.ChannelId)
	if err == nil {
		_, derr := klingOfficialElementClient(channel, key).DeleteElement(req.ElementId)
		if derr != nil && c.Query("force") != "true" {
			common.ApiError(c, derr)
			return
		}
	}

	// Delete from local database
	if err := model.DB.Delete(element).Error; err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"task_id":     element.JobId,
			"task_status": "succeed",
		},
	})
}
