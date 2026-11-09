package kling

import (
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
)

func isKling40Action(action string) bool {
	switch action {
	case constant.TaskActionKling40Text, constant.TaskActionKling40Image, constant.TaskActionKling40Omni:
		return true
	default:
		return false
	}
}

func kling40UpstreamSlug(info *relaycommon.RelayInfo) string {
	if info == nil {
		return "kling-4.0"
	}
	for _, name := range []string{info.UpstreamModelName, info.OriginModelName} {
		if _, ok := kling40ModelFromPath("/" + name); ok {
			return name
		}
	}
	return "kling-4.0"
}

func externalTaskIDFromMetadata(metadata map[string]any) string {
	options, ok := metadata["options"].(map[string]any)
	if !ok {
		return ""
	}
	ext, _ := options["external_task_id"].(string)
	return strings.TrimSpace(ext)
}

func kling40CreateResponse(c *gin.Context, info *relaycommon.RelayInfo, responseBody []byte) map[string]any {
	var upstream struct {
		Message   string `json:"message"`
		RequestID string `json:"request_id"`
		Data      struct {
			Status     string `json:"status"`
			CreateTime int64  `json:"create_time"`
			UpdateTime int64  `json:"update_time"`
			ExternalID string `json:"external_id"`
		} `json:"data"`
	}
	_ = common.Unmarshal(responseBody, &upstream)
	status := upstream.Data.Status
	switch status {
	case "submitted", "processing", "succeeded", "failed":
	case "succeed":
		status = "succeeded"
	default:
		status = "submitted"
	}
	now := time.Now().UnixMilli()
	createTime := upstream.Data.CreateTime
	if createTime <= 0 {
		createTime = now
	}
	updateTime := upstream.Data.UpdateTime
	if updateTime <= 0 {
		updateTime = createTime
	}
	externalID := upstream.Data.ExternalID
	if externalID == "" && c != nil {
		externalID = c.GetString("kling_external_task_id")
	}
	requestID := upstream.RequestID
	if requestID == "" {
		requestID = common.NewRequestId()
	}
	publicID := ""
	if info != nil {
		publicID = info.PublicTaskID
	}
	return map[string]any{
		"code":       0,
		"message":    upstream.Message,
		"request_id": requestID,
		"data": map[string]any{
			"id":          publicID,
			"status":      status,
			"create_time": createTime,
			"update_time": updateTime,
			"external_id": externalID,
		},
	}
}
