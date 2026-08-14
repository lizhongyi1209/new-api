package controller

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/task/tencentvideo"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// Tencent VCLM image-reference limits (per the 主体管理 docs):
//   - format: .jpg / .jpeg / .png
//   - each image <= 10MB
//   - 1 frontal image + 1~3 additional reference images
const (
	elementMaxImageBytes  = 10 * 1024 * 1024
	elementMinReferImages = 1
	elementMaxReferImages = 3
)

// aigc_element.go exposes the Tencent VCLM "主体管理" (AIGC Element) APIs.
// A subject is created against an enabled TencentVideo channel and is owned by
// the requesting user (the token's main account). Admins may list and manage
// every user's subjects; regular users only see their own.

// resolveTencentVideoChannel returns an enabled TencentVideo channel and its
// parsed credential. When channelId > 0 that specific channel is used (and must
// be a TencentVideo channel), otherwise the first enabled one is picked.
func resolveTencentVideoChannel(channelId int) (*model.Channel, string, string, error) {
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
			constant.ChannelTypeTencentVideo, common.ChannelStatusEnabled).
			Find(&channels).Error
		if err != nil {
			return nil, "", "", err
		}
		if len(channels) == 0 {
			return nil, "", "", errors.New("没有可用的腾讯视频(TencentVideo)通道")
		}
		channel = channels[0]
	}
	if channel.Type != constant.ChannelTypeTencentVideo {
		return nil, "", "", errors.New("通道类型不是腾讯视频(TencentVideo)")
	}
	key, _, apiErr := channel.GetNextEnabledKey()
	if apiErr != nil {
		return nil, "", "", apiErr
	}
	return channel, key, channel.GetBaseURL(), nil
}

// elementClientForChannel builds a VCLM management client for the given channel.
func elementClientForChannel(channel *model.Channel, key string) *tencentvideo.ElementClient {
	return tencentvideo.NewElementClient(key, channel.GetBaseURL(), "", "")
}

// CreateAigcElementRequest is the inbound JSON for creating a subject. It is a
// thin, snake_case wrapper over the Tencent CreateAigcElement parameters.
type CreateAigcElementRequest struct {
	ChannelId      int      `json:"channel_id"`
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	ReferenceType  string   `json:"reference_type"`
	FrontalImage   string   `json:"frontal_image"`
	ReferImages    []string `json:"refer_images"`
	VideoList      []string `json:"video_list"`
	Provider       []string `json:"provider"`
	TagIds         []string `json:"tag_ids"`
	ElementVoiceId string   `json:"element_voice_id"`
}

func CreateAigcElement(c *gin.Context) {
	userId := c.GetInt("id")
	var req CreateAigcElementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	if req.Name == "" || req.Description == "" {
		common.ApiErrorMsg(c, "name 和 description 不能为空")
		return
	}
	if req.ReferenceType == "" {
		req.ReferenceType = tencentvideo.ReferenceTypeImage
	}

	// Validate the reference assets up front so we fail fast with a clear
	// message instead of letting Tencent time out fetching an oversized image
	// (which surfaces as the opaque FailedOperation.RequestTimeout).
	if req.ReferenceType == tencentvideo.ReferenceTypeImage {
		if msg := validateElementImageRefs(&req); msg != "" {
			common.ApiErrorMsg(c, msg)
			return
		}
	}

	channel, key, _, err := resolveTencentVideoChannel(req.ChannelId)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	tcReq := &tencentvideo.CreateElementRequest{
		Name:           req.Name,
		Description:    req.Description,
		ReferenceType:  req.ReferenceType,
		VideoList:      req.VideoList,
		Provider:       req.Provider,
		ElementVoiceId: req.ElementVoiceId,
	}
	if req.ReferenceType == tencentvideo.ReferenceTypeImage {
		il := &tencentvideo.ElementImageList{FrontalImage: req.FrontalImage}
		for _, u := range req.ReferImages {
			if strings.TrimSpace(u) != "" {
				il.ReferImages = append(il.ReferImages, tencentvideo.ReferImageItem{ImageUrl: u})
			}
		}
		tcReq.ElementImageList = il
	}
	for _, id := range req.TagIds {
		if strings.TrimSpace(id) != "" {
			tcReq.TagList = append(tcReq.TagList, tencentvideo.TagItem{TagId: id})
		}
	}

	result, err := elementClientForChannel(channel, key).CreateElement(tcReq)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	provider := ""
	if len(result.Provider) > 0 {
		provider = strings.Join(result.Provider, ",")
	} else if len(req.Provider) > 0 {
		provider = strings.Join(req.Provider, ",")
	}

	// Snapshot the reference images and the creating token for visual + ownership
	// management. token_id/token_name are only present under TokenAuth (API
	// clients); console sessions leave them at zero/empty.
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
		Platform:      model.AigcElementPlatformKling,
		JobId:         result.JobId,
		ElementId:     result.ElementId,
		Name:          req.Name,
		Description:   req.Description,
		ReferenceType: req.ReferenceType,
		Provider:      provider,
		Status:        defaultStatus(result.Status),
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

func defaultStatus(s string) string {
	if s == "" {
		return "pending"
	}
	return s
}

// validateElementImageRefs checks the frontal + reference images for an
// image_refer subject against Tencent's documented limits (presence, count,
// format, and 10MB size). It returns a human-readable error message, or "" when
// everything passes. Each image is probed with checkRemoteImage, which reads
// only the response headers and closes the body without downloading the full
// payload.
func validateElementImageRefs(req *CreateAigcElementRequest) string {
	if strings.TrimSpace(req.FrontalImage) == "" {
		return "缺少正面参考图 (frontal_image)"
	}

	refers := make([]string, 0, len(req.ReferImages))
	for _, u := range req.ReferImages {
		if strings.TrimSpace(u) != "" {
			refers = append(refers, strings.TrimSpace(u))
		}
	}
	if len(refers) < elementMinReferImages || len(refers) > elementMaxReferImages {
		return fmt.Sprintf("其他参考图需 %d~%d 张，当前 %d 张", elementMinReferImages, elementMaxReferImages, len(refers))
	}

	all := append([]string{strings.TrimSpace(req.FrontalImage)}, refers...)
	for _, u := range all {
		if msg := checkRemoteImage(u); msg != "" {
			return msg
		}
	}
	return ""
}

// checkRemoteImage fetches the URL and inspects only its response headers to
// verify it is a reachable jpg/jpeg/png no larger than 10MB; the body is closed
// without being read. Returns "" when the image looks valid, otherwise a
// message. When the server does not report Content-Length the size check is
// skipped (Tencent will still enforce it), but format and reachability are
// always checked.
func checkRemoteImage(url string) string {
	resp, err := service.DoDownloadRequest(url, "aigc_element image precheck")
	if err != nil {
		return fmt.Sprintf("参考图无法访问: %s", url)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("参考图无法访问 (HTTP %d): %s", resp.StatusCode, url)
	}

	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if contentType != "" &&
		!strings.HasPrefix(contentType, "image/jpeg") &&
		!strings.HasPrefix(contentType, "image/jpg") &&
		!strings.HasPrefix(contentType, "image/png") &&
		contentType != "application/octet-stream" {
		return fmt.Sprintf("参考图格式需为 jpg/jpeg/png，当前为 %s: %s", contentType, url)
	}

	if resp.ContentLength > 0 && resp.ContentLength > elementMaxImageBytes {
		return fmt.Sprintf("参考图大小 %.1fMB 超过 10MB 限制: %s",
			float64(resp.ContentLength)/1024/1024, url)
	}
	return ""
}

// isAdmin reports whether the current request is from an admin/root user. Token
// auth sets only "id" (no role), so a missing/zero role means a regular client.
func isAdmin(c *gin.Context) bool {
	return c.GetInt("role") >= common.RoleAdminUser
}

// GetAigcElements lists subjects. Admins (with the all=true query) see every
// user's subjects; everyone else sees only their own.
func GetAigcElements(c *gin.Context) {
	userId := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)

	all := c.Query("all") == "true" && isAdmin(c)
	var (
		elements []*model.AigcElement
		total    int64
		err      error
	)
	if all {
		elements, total, err = model.GetAllAigcElements(model.AigcElementPlatformKling, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	} else {
		elements, total, err = model.GetUserAigcElements(userId, model.AigcElementPlatformKling, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(elements)
	common.ApiSuccess(c, pageInfo)
}

// ownerScope returns the userId to scope DB queries by: 0 for admins (any row),
// the caller's id otherwise.
func ownerScope(c *gin.Context) int {
	if isAdmin(c) {
		return 0
	}
	return c.GetInt("id")
}

// clientAigcElement is the slim shape returned to API clients picking a subject
// to reference in a video request: the display name (used as the 【@Name】 tag
// in the prompt), the element id (sent in ElementList), the frontal reference
// image (for a visual picker), and the status.
type clientAigcElement struct {
	ElementId     string `json:"element_id"`
	Name          string `json:"name"`
	FrontalImage  string `json:"frontal_image"`
	ReferenceType string `json:"reference_type"`
	Platform      string `json:"platform"`
	Status        string `json:"status"`
}

// ListMyAigcElements returns the subjects owned by the current token's account,
// so a client holding an API token can fetch its own subjects and reference
// them when generating a video. By default only succeeded subjects are
// returned; pass ?include_all=true to also see pending/failed ones.
//
// Usage flow for the client:
//  1. GET /api/aigc_element/mine  -> get {element_id, name, frontal_image}
//  2. when generating a video, put the element_id into metadata.ElementList and
//     reference the subject in the prompt as 【@<name>】.
func ListMyAigcElements(c *gin.Context) {
	userId := c.GetInt("id")
	onlySucceed := c.Query("include_all") != "true"

	elements, err := model.ListUserAigcElements(userId, model.AigcElementPlatformKling, onlySucceed)
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

// RefreshAigcElement re-queries Tencent for the element's latest status/detail
// and updates the local row.
func RefreshAigcElement(c *gin.Context) {
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
	if element.ElementId == "" {
		common.ApiErrorMsg(c, "该主体尚无 ElementId，无法查询")
		return
	}

	channel, key, _, err := resolveTencentVideoChannel(element.ChannelId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	result, err := elementClientForChannel(channel, key).DescribeElement(element.ElementId)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	if result.Status != "" {
		element.Status = result.Status
	}
	if len(result.Provider) > 0 {
		element.Provider = strings.Join(result.Provider, ",")
	}
	if result.Name != "" {
		element.Name = result.Name
	}
	if result.Description != "" {
		element.Description = result.Description
	}
	if result.ReferenceType != "" {
		element.ReferenceType = result.ReferenceType
	}
	element.FailReason = failReasonFromDetails(result)
	if err := element.Update(); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"element": element, "detail": result})
}

// failReasonFromDetails joins any per-provider error messages into a single
// reason string for a failed element.
func failReasonFromDetails(result *tencentvideo.ElementResult) string {
	var msgs []string
	for _, d := range result.ProviderDetails {
		if d.Status == "failed" && strings.TrimSpace(d.ErrorMessage) != "" {
			msgs = append(msgs, d.Provider+": "+d.ErrorMessage)
		}
	}
	return strings.Join(msgs, "; ")
}

// DeleteAigcElement deletes the subject on Tencent's side, then removes the
// local row. The remote delete is best-effort: a "not found" upstream error
// still lets the local row be removed.
func DeleteAigcElement(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	scope := ownerScope(c)
	element, err := model.GetAigcElementById(id, scope)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	if element.ElementId != "" {
		channel, key, _, cerr := resolveTencentVideoChannel(element.ChannelId)
		if cerr == nil {
			if _, derr := elementClientForChannel(channel, key).DeleteElement(element.ElementId); derr != nil {
				// Surface the remote error but allow forced local cleanup via ?force=true.
				if c.Query("force") != "true" {
					common.ApiError(c, derr)
					return
				}
			}
		}
	}

	if err := model.DeleteAigcElementById(id, scope); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"id": id})
}

// UploadAigcElementImage accepts a multipart image upload, auto-resizes it to
// stay within Tencent's 10MB limit, stores it on the site's object storage
// (R2/OSS per host routing), and returns the public URL the client can then
// pass as a reference image when creating a subject. This exists because end
// users typically cannot produce a public URL for a local image themselves.
func UploadAigcElementImage(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		common.ApiErrorMsg(c, "缺少上传文件 (form field: file)")
		return
	}

	f, err := fileHeader.Open()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	defer f.Close()

	raw, err := io.ReadAll(io.LimitReader(f, 100*1024*1024)) // hard cap 100MB read
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if len(raw) == 0 {
		common.ApiErrorMsg(c, "上传文件为空")
		return
	}

	// Shrink to <= 10MB if needed; jpg/jpeg/png are kept, others become jpeg.
	processed, mimeType, resized, err := service.CompressImageToLimit(raw, elementMaxImageBytes)
	if err != nil {
		common.ApiErrorMsg(c, "图片处理失败: "+err.Error())
		return
	}

	publicURL, err := service.UploadBase64ImageToHostStorage(
		mimeType,
		base64.StdEncoding.EncodeToString(processed),
		c.Request.Host,
	)
	if err != nil {
		common.ApiErrorMsg(c, "上传到对象存储失败: "+err.Error())
		return
	}

	common.ApiSuccess(c, gin.H{
		"url":        publicURL,
		"mime_type":  mimeType,
		"size":       len(processed),
		"resized":    resized,
		"size_human": fmt.Sprintf("%.2fMB", float64(len(processed))/1024/1024),
	})
}
