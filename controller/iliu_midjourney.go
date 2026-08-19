package controller

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/iliu"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

const maxILiuResponseBytes = 16 << 20

func RelayILiuMidjourneySubmit(c *gin.Context) {
	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatTask, nil, nil)
	if err != nil {
		respondILiuError(c, http.StatusInternalServerError, 5, err.Error())
		return
	}
	relayInfo.InitChannelMeta(c)
	if relayInfo.ChannelType != constant.ChannelTypeILiuMidjourney {
		respondILiuError(c, http.StatusBadRequest, 4, "selected channel is not an iLiu Midjourney channel")
		return
	}

	adaptor := &iliu.TaskAdaptor{}
	adaptor.Init(relayInfo)
	if taskErr := adaptor.ValidateRequestAndSetAction(c, relayInfo); taskErr != nil {
		respondILiuError(c, taskErr.StatusCode, 4, taskErr.Message)
		return
	}

	priceData, err := helper.ModelPriceHelperPerCall(c, relayInfo)
	if err != nil {
		respondILiuError(c, http.StatusBadRequest, 4, err.Error())
		return
	}
	relayInfo.PriceData = priceData
	if !priceData.FreeModel {
		relayInfo.ForcePreConsume = true
		if apiErr := service.PreConsumeBilling(c, priceData.Quota, relayInfo); apiErr != nil {
			respondILiuError(c, apiErr.StatusCode, 4, apiErr.Error())
			return
		}
	}

	settled := false
	defer func() {
		if !settled && relayInfo.Billing != nil {
			relayInfo.Billing.Refund(c)
		}
	}()

	requestBody, err := adaptor.BuildRequestBody(c, relayInfo)
	if err != nil {
		respondILiuError(c, http.StatusInternalServerError, 5, err.Error())
		return
	}
	resp, err := adaptor.DoRequest(c, relayInfo, requestBody)
	if err != nil {
		respondILiuError(c, http.StatusBadGateway, 5, err.Error())
		return
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, maxILiuResponseBytes+1))
	if err != nil {
		respondILiuError(c, http.StatusBadGateway, 5, "failed to read iLiu response")
		return
	}
	if len(responseBody) > maxILiuResponseBytes {
		respondILiuError(c, http.StatusBadGateway, 5, "iLiu response is too large")
		return
	}
	envelope, upstreamTaskID, parseErr := iliu.ParseSubmitResponse(responseBody)
	if parseErr != nil {
		writeILiuResponse(c, resp.StatusCode, responseBody)
		return
	}
	accepted := resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices &&
		(envelope.Code == 1 || envelope.Code == 21 || envelope.Code == 22)
	if !accepted {
		writeILiuResponse(c, resp.StatusCode, responseBody)
		return
	}

	isUpload := iliu.ModelForPath(c.Request.URL.Path) == "mj_upload"
	if isUpload {
		if _, err := iliu.ParseUploadResult(envelope.Result); err != nil {
			respondILiuError(c, http.StatusBadGateway, 5, err.Error())
			return
		}
	}
	if !isUpload && upstreamTaskID == "" {
		respondILiuError(c, http.StatusBadGateway, 5, "iLiu returned an empty task ID")
		return
	}
	if !isUpload {
		if existing, exists, lookupErr := model.GetByTaskId(relayInfo.UserId, upstreamTaskID); lookupErr == nil && exists && existing != nil {
			writeILiuResponse(c, resp.StatusCode, responseBody)
			return
		}
	}

	var task *model.Task
	if !isUpload {
		task = model.InitTask(constant.TaskPlatformILiuMidjourney, relayInfo)
		task.TaskID = upstreamTaskID
		task.PrivateData.UpstreamTaskID = upstreamTaskID
		task.PrivateData.Key = relayInfo.ApiKey
		task.PrivateData.RequestID = c.GetString(common.RequestIdKey)
		task.PrivateData.RequestSnapshot = service.BuildTaskRequestSnapshot(c, relayInfo)
		task.PrivateData.SubmitResponse = service.SanitizeTaskAuditResponse(responseBody)
		task.PrivateData.BillingSource = relayInfo.BillingSource
		task.PrivateData.SubscriptionId = relayInfo.SubscriptionId
		task.PrivateData.TokenId = relayInfo.TokenId
		task.PrivateData.NodeName = common.NodeName
		task.PrivateData.BillingContext = &model.TaskBillingContext{
			ModelPrice:      priceData.ModelPrice,
			GroupRatio:      priceData.GroupRatioInfo.GroupRatio,
			ModelRatio:      priceData.ModelRatio,
			OtherRatios:     priceData.OtherRatios,
			OriginModelName: relayInfo.OriginModelName,
			PerCallBilling:  true,
		}
		task.Quota = priceData.Quota
		task.Action = relayInfo.Action
		task.Data = responseBody
		if envelope.Code == 22 {
			task.Status = model.TaskStatusQueued
		} else {
			task.Status = model.TaskStatusSubmitted
		}
		if err := task.Insert(); err != nil {
			respondILiuError(c, http.StatusInternalServerError, 5, "failed to persist iLiu task")
			return
		}
	}

	if err := service.SettleBilling(c, relayInfo, priceData.Quota); err != nil {
		common.SysError("settle iLiu Midjourney billing error: " + err.Error())
	}
	settled = true
	logTask := task
	if logTask == nil {
		logTask = model.InitTask(constant.TaskPlatformILiuMidjourney, relayInfo)
	}
	service.LogTaskConsumption(c, relayInfo, logTask.TaskID)
	writeILiuResponse(c, resp.StatusCode, responseBody)
}

func RelayILiuMidjourneyAction(c *gin.Context) {
	relayILiuOriginSubmit(c, "mj_action")
}

func RelayILiuMidjourneyModal(c *gin.Context) {
	relayILiuOriginSubmit(c, "mj_modal")
}

func relayILiuOriginSubmit(c *gin.Context, modelName string) {
	var request struct {
		TaskID string `json:"taskId"`
	}
	if err := common.UnmarshalBodyReusable(c, &request); err != nil || request.TaskID == "" {
		respondILiuError(c, http.StatusBadRequest, 4, "taskId is required")
		return
	}
	task, exists, err := model.GetByTaskId(c.GetInt("id"), request.TaskID)
	if err != nil {
		respondILiuError(c, http.StatusInternalServerError, 5, err.Error())
		return
	}
	if !exists || task == nil || task.Platform != constant.TaskPlatformILiuMidjourney {
		respondILiuError(c, http.StatusNotFound, 4, "task not found")
		return
	}
	channelModel, err := model.GetChannelById(task.ChannelId, true)
	if err != nil || channelModel.Status != common.ChannelStatusEnabled || channelModel.Type != constant.ChannelTypeILiuMidjourney {
		respondILiuError(c, http.StatusServiceUnavailable, 4, "task channel is unavailable")
		return
	}
	if setupErr := middleware.SetupContextForSelectedChannel(c, channelModel, modelName, task.Group); setupErr != nil {
		respondILiuError(c, http.StatusServiceUnavailable, 4, setupErr.Error())
		return
	}
	if task.PrivateData.Key != "" {
		common.SetContextKey(c, constant.ContextKeyChannelKey, task.PrivateData.Key)
	}
	common.SetContextKey(c, constant.ContextKeyRequestStartTime, time.Now())
	RelayILiuMidjourneySubmit(c)
}

func RelayILiuMidjourneyTaskFetch(c *gin.Context) {
	task, channelModel, ok := getILiuTaskAndChannel(c, c.Param("id"))
	if !ok {
		return
	}
	resp, err := iliu.DoChannelRequest(c, channelModel, task.PrivateData.Key, http.MethodGet, "/v1/mj/task/"+url.PathEscape(task.TaskID)+"/fetch", nil)
	if err != nil {
		respondILiuError(c, http.StatusBadGateway, 5, err.Error())
		return
	}
	copyILiuUpstreamResponse(c, resp)
}

func RelayILiuMidjourneyTaskList(c *gin.Context) {
	var condition struct {
		IDs []string `json:"ids"`
	}
	if err := common.UnmarshalBodyReusable(c, &condition); err != nil || len(condition.IDs) == 0 {
		respondILiuError(c, http.StatusBadRequest, 4, "ids is required")
		return
	}
	if len(condition.IDs) > 100 {
		respondILiuError(c, http.StatusBadRequest, 4, "ids must contain at most 100 task IDs")
		return
	}
	ids := make([]any, len(condition.IDs))
	for i, id := range condition.IDs {
		ids[i] = id
	}
	tasks, err := model.GetByTaskIds(c.GetInt("id"), ids)
	if err != nil {
		respondILiuError(c, http.StatusInternalServerError, 5, err.Error())
		return
	}
	type taskGroup struct {
		ChannelID int
		Key       string
	}
	groupedTasks := make(map[taskGroup][]string)
	for _, task := range tasks {
		if task.Platform == constant.TaskPlatformILiuMidjourney {
			group := taskGroup{ChannelID: task.ChannelId, Key: task.PrivateData.Key}
			groupedTasks[group] = append(groupedTasks[group], task.TaskID)
		}
	}
	resultsByID := make(map[string]json.RawMessage, len(tasks))
	for group, taskIDs := range groupedTasks {
		channelModel, channelErr := model.GetChannelById(group.ChannelID, true)
		if channelErr != nil || channelModel.Status != common.ChannelStatusEnabled || channelModel.Type != constant.ChannelTypeILiuMidjourney {
			respondILiuError(c, http.StatusServiceUnavailable, 4, "task channel is unavailable")
			return
		}
		body, marshalErr := common.Marshal(map[string]any{"ids": taskIDs})
		if marshalErr != nil {
			respondILiuError(c, http.StatusInternalServerError, 5, marshalErr.Error())
			return
		}
		resp, requestErr := iliu.DoChannelRequest(c, channelModel, group.Key, http.MethodPost, "/v1/mj/task/list-by-condition", body)
		if requestErr != nil {
			respondILiuError(c, http.StatusBadGateway, 5, requestErr.Error())
			return
		}
		responseBody, readErr := io.ReadAll(io.LimitReader(resp.Body, maxILiuResponseBytes+1))
		resp.Body.Close()
		if readErr != nil || resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			respondILiuError(c, http.StatusBadGateway, 5, "failed to fetch iLiu tasks")
			return
		}
		var channelResults []json.RawMessage
		if err := common.Unmarshal(responseBody, &channelResults); err != nil {
			respondILiuError(c, http.StatusBadGateway, 5, "invalid iLiu task list response")
			return
		}
		for _, result := range channelResults {
			var item struct {
				ID string `json:"id"`
			}
			if err := common.Unmarshal(result, &item); err != nil || item.ID == "" {
				respondILiuError(c, http.StatusBadGateway, 5, "invalid iLiu task list item")
				return
			}
			resultsByID[item.ID] = result
		}
	}
	results := make([]json.RawMessage, 0, len(resultsByID))
	for _, taskID := range condition.IDs {
		if result, exists := resultsByID[taskID]; exists {
			results = append(results, result)
		}
	}
	c.JSON(http.StatusOK, results)
}

func RelayILiuMidjourneyImageSeed(c *gin.Context) {
	task, channelModel, ok := getILiuTaskAndChannel(c, c.Param("id"))
	if !ok {
		return
	}
	resp, err := iliu.DoChannelRequest(c, channelModel, task.PrivateData.Key, http.MethodGet, "/v1/mj/task/"+url.PathEscape(task.TaskID)+"/image-seed", nil)
	if err != nil {
		respondILiuError(c, http.StatusBadGateway, 5, err.Error())
		return
	}
	copyILiuUpstreamResponse(c, resp)
}

func getILiuTaskAndChannel(c *gin.Context, taskID string) (*model.Task, *model.Channel, bool) {
	task, exists, err := model.GetByTaskId(c.GetInt("id"), taskID)
	if err != nil {
		respondILiuError(c, http.StatusInternalServerError, 5, err.Error())
		return nil, nil, false
	}
	if !exists || task == nil || task.Platform != constant.TaskPlatformILiuMidjourney {
		respondILiuError(c, http.StatusNotFound, 4, "task not found")
		return nil, nil, false
	}
	channelModel, err := model.GetChannelById(task.ChannelId, true)
	if err != nil || channelModel.Status != common.ChannelStatusEnabled || channelModel.Type != constant.ChannelTypeILiuMidjourney {
		respondILiuError(c, http.StatusServiceUnavailable, 4, "task channel is unavailable")
		return nil, nil, false
	}
	return task, channelModel, true
}

func copyILiuUpstreamResponse(c *gin.Context, resp *http.Response) {
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxILiuResponseBytes+1))
	if err != nil {
		respondILiuError(c, http.StatusBadGateway, 5, "failed to read iLiu response")
		return
	}
	if len(body) > maxILiuResponseBytes {
		respondILiuError(c, http.StatusBadGateway, 5, "iLiu response is too large")
		return
	}
	writeILiuResponse(c, resp.StatusCode, body)
}

func writeILiuResponse(c *gin.Context, statusCode int, body []byte) {
	c.Header("Content-Type", "application/json")
	c.Status(statusCode)
	_, _ = c.Writer.Write(body)
}

func respondILiuError(c *gin.Context, statusCode, code int, description string) {
	c.JSON(statusCode, gin.H{
		"code":        code,
		"description": description,
		"properties":  gin.H{},
		"result":      "",
	})
}
