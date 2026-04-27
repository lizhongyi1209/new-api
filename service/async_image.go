package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
)

type ImageAdaptor interface {
	Init(info *relaycommon.RelayInfo)
	ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error)
	DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error)
}

type GeminiAdaptor interface {
	Init(info *relaycommon.RelayInfo)
	DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error)
}

var GetImageAdaptorFunc func(apiType int) ImageAdaptor
var GetGeminiAdaptorFunc func(apiType int) GeminiAdaptor

func applyModelMapping(originModelName string, modelMappingJSON *string) string {
	if modelMappingJSON == nil || *modelMappingJSON == "" || *modelMappingJSON == "{}" {
		return originModelName
	}

	var modelMap map[string]string
	if err := common.Unmarshal([]byte(*modelMappingJSON), &modelMap); err != nil {
		return originModelName
	}

	currentModel := originModelName
	visited := map[string]bool{currentModel: true}
	for {
		if mappedModel, exists := modelMap[currentModel]; exists && mappedModel != "" {
			if visited[mappedModel] {
				break
			}
			visited[mappedModel] = true
			currentModel = mappedModel
		} else {
			break
		}
	}
	return currentModel
}

func ProcessAsyncImageTask(ctx context.Context, task *model.Task) {
	var asyncReq dto.AsyncImageRequest
	if err := task.GetData(&asyncReq); err != nil {
		logger.LogError(ctx, fmt.Sprintf("async_image: failed to parse task data: %v", err))
		task.Status = model.TaskStatusFailure
		task.FailReason = "解析任务数据失败"
		task.Progress = "100%"
		task.FinishTime = time.Now().Unix()
		_ = task.Update()
		return
	}

	imageReq := &dto.ImageRequest{
		Model:          asyncReq.Model,
		Prompt:         asyncReq.Prompt,
		N:              asyncReq.N,
		Size:           asyncReq.Size,
		Quality:        asyncReq.Quality,
		ResponseFormat: asyncReq.ResponseFormat,
		Style:          asyncReq.Style,
		User:           asyncReq.User,
	}

	// Create a new gin context for async execution instead of reusing the submit context
	c := &gin.Context{
		Request: &http.Request{
			Method: "POST",
			Header: make(http.Header),
			Body:   http.NoBody,
		},
	}

	channel, err := model.CacheGetChannel(task.ChannelId)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("async_image: failed to get channel: %v", err))
		task.Status = model.TaskStatusFailure
		task.FailReason = fmt.Sprintf("获取渠道信息失败: %v", err)
		task.Progress = "100%"
		task.FinishTime = time.Now().Unix()
		_ = task.Update()
		return
	}

	apiType, _ := common.ChannelType2APIType(channel.Type)
	key, keyIndex, keyErr := channel.GetNextEnabledKey()
	if keyErr != nil {
		task.Status = model.TaskStatusFailure
		task.FailReason = fmt.Sprintf("获取渠道密钥失败: %v", keyErr.Error())
		task.Progress = "100%"
		task.FinishTime = time.Now().Unix()
		_ = task.Update()
		return
	}

	upstreamModelName := applyModelMapping(imageReq.Model, channel.ModelMapping)

	relayInfo := &relaycommon.RelayInfo{
		UserId:     task.UserId,
		UsingGroup: task.Group,
		Request:    imageReq,
		RelayMode:  relayconstant.RelayModeImagesGenerations,
		TaskRelayInfo: &relaycommon.TaskRelayInfo{
			PublicTaskID: task.TaskID,
		},
		OriginModelName: imageReq.Model,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:          channel.Type,
			ChannelId:            channel.Id,
			ChannelIsMultiKey:    channel.ChannelInfo.IsMultiKey,
			ChannelMultiKeyIndex: keyIndex,
			ChannelBaseUrl:       channel.GetBaseURL(),
			ApiType:              apiType,
			ApiKey:               key,
			UpstreamModelName:    upstreamModelName,
		},
	}

	task.Status = model.TaskStatusInProgress
	task.StartTime = time.Now().Unix()
	task.Progress = "50%"
	_ = task.Update()

	if GetImageAdaptorFunc == nil {
		task.Status = model.TaskStatusFailure
		task.FailReason = "内部错误：适配器未初始化"
		task.Progress = "100%"
		task.FinishTime = time.Now().Unix()
		_ = task.Update()
		return
	}

	adaptor := GetImageAdaptorFunc(relayInfo.ApiType)
	if adaptor == nil {
		task.Status = model.TaskStatusFailure
		task.FailReason = fmt.Sprintf("不支持的 API 类型: %d", relayInfo.ApiType)
		task.Progress = "100%"
		task.FinishTime = time.Now().Unix()
		_ = task.Update()
		return
	}
	adaptor.Init(relayInfo)

	convertedRequest, err := adaptor.ConvertImageRequest(c, relayInfo, *imageReq)
	if err != nil {
		task.Status = model.TaskStatusFailure
		task.FailReason = fmt.Sprintf("转换请求失败: %v", err)
		task.Progress = "100%"
		task.FinishTime = time.Now().Unix()
		_ = task.Update()
		return
	}

	jsonData, err := common.Marshal(convertedRequest)
	if err != nil {
		task.Status = model.TaskStatusFailure
		task.FailReason = fmt.Sprintf("序列化请求失败: %v", err)
		task.Progress = "100%"
		task.FinishTime = time.Now().Unix()
		_ = task.Update()
		return
	}

	resp, err := adaptor.DoRequest(c, relayInfo, bytes.NewReader(jsonData))
	if err != nil {
		task.Status = model.TaskStatusFailure
		task.FailReason = fmt.Sprintf("请求上游失败: %v", err)
		task.Progress = "100%"
		task.FinishTime = time.Now().Unix()
		_ = task.Update()
		return
	}

	httpResp := resp.(*http.Response)
	if httpResp.StatusCode != http.StatusOK {
		relayErr := RelayErrorHandler(ctx, httpResp, false)
		task.Status = model.TaskStatusFailure
		task.FailReason = fmt.Sprintf("上游返回错误: %s", relayErr.Error())
		task.Progress = "100%"
		task.FinishTime = time.Now().Unix()
		_ = task.Update()
		return
	}

	bodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		task.Status = model.TaskStatusFailure
		task.FailReason = fmt.Sprintf("读取响应失败: %v", err)
		task.Progress = "100%"
		task.FinishTime = time.Now().Unix()
		_ = task.Update()
		return
	}

	var imageResp dto.ImageResponse
	if err := common.Unmarshal(bodyBytes, &imageResp); err != nil {
		task.Status = model.TaskStatusFailure
		task.FailReason = fmt.Sprintf("解析响应失败: %v", err)
		task.Progress = "100%"
		task.FinishTime = time.Now().Unix()
		_ = task.Update()
		return
	}

	if len(imageResp.Data) == 0 {
		task.Status = model.TaskStatusFailure
		task.FailReason = "上游未返回图片数据"
		task.Progress = "100%"
		task.FinishTime = time.Now().Unix()
		_ = task.Update()
		return
	}

	var uploadedURLs []string
	for _, imgData := range imageResp.Data {
		if imgData.B64Json != "" {
			publicURL, err := UploadBase64ImageToR2("image/png", imgData.B64Json)
			if err != nil {
				logger.LogError(ctx, fmt.Sprintf("async_image: R2 upload failed: %v", err))
				task.Status = model.TaskStatusFailure
				task.FailReason = fmt.Sprintf("上传图片到 R2 失败: %v", err)
				task.Progress = "100%"
				task.FinishTime = time.Now().Unix()
				_ = task.Update()
				return
			}
			uploadedURLs = append(uploadedURLs, publicURL)
		} else if imgData.Url != "" {
			uploadedURLs = append(uploadedURLs, imgData.Url)
		}
	}

	resultData := map[string]interface{}{
		"urls": uploadedURLs,
	}
	task.SetData(resultData)
	task.Status = model.TaskStatusSuccess
	task.Progress = "100%"
	task.FinishTime = time.Now().Unix()
	_ = task.Update()

	logger.LogInfo(ctx, fmt.Sprintf("async_image: task %s completed, uploaded %d images", task.TaskID, len(uploadedURLs)))
}

func ProcessAsyncGeminiTask(ctx context.Context, task *model.Task) {
	// Create a new gin context for async execution instead of reusing the submit context
	c := &gin.Context{
		Request: &http.Request{
			Method: "POST",
			Header: make(http.Header),
			Body:   http.NoBody,
		},
	}

	task.Status = model.TaskStatusInProgress
	task.StartTime = time.Now().Unix()
	task.Progress = "50%"
	_ = task.Update()

	var requestBody map[string]interface{}
	if err := task.GetData(&requestBody); err != nil {
		task.Status = model.TaskStatusFailure
		task.FailReason = fmt.Sprintf("解析请求数据失败: %v", err)
		task.Progress = "100%"
		task.FinishTime = time.Now().Unix()
		_ = task.Update()
		return
	}

	// Apply Gemini request normalization (set default role for first content)
	if contents, ok := requestBody["contents"].([]interface{}); ok && len(contents) > 0 {
		if firstContent, ok := contents[0].(map[string]interface{}); ok {
			if _, hasRole := firstContent["role"]; !hasRole {
				firstContent["role"] = "user"
			}
		}
	}

	jsonData, err := common.Marshal(requestBody)
	if err != nil {
		task.Status = model.TaskStatusFailure
		task.FailReason = fmt.Sprintf("序列化请求失败: %v", err)
		task.Progress = "100%"
		task.FinishTime = time.Now().Unix()
		_ = task.Update()
		return
	}

	channel, err := model.CacheGetChannel(task.ChannelId)
	if err != nil {
		task.Status = model.TaskStatusFailure
		task.FailReason = fmt.Sprintf("获取渠道信息失败: %v", err)
		task.Progress = "100%"
		task.FinishTime = time.Now().Unix()
		_ = task.Update()
		return
	}

	apiType, _ := common.ChannelType2APIType(channel.Type)
	key, keyIndex, keyErr := channel.GetNextEnabledKey()
	if keyErr != nil {
		task.Status = model.TaskStatusFailure
		task.FailReason = fmt.Sprintf("获取渠道密钥失败: %v", keyErr.Error())
		task.Progress = "100%"
		task.FinishTime = time.Now().Unix()
		_ = task.Update()
		return
	}

	upstreamModelName := applyModelMapping(task.Properties.OriginModelName, channel.ModelMapping)

	relayInfo := &relaycommon.RelayInfo{
		UserId:          task.UserId,
		UsingGroup:      task.Group,
		RelayMode:       relayconstant.RelayModeGemini,
		OriginModelName: task.Properties.OriginModelName,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:          channel.Type,
			ChannelId:            channel.Id,
			ChannelIsMultiKey:    channel.ChannelInfo.IsMultiKey,
			ChannelMultiKeyIndex: keyIndex,
			ChannelBaseUrl:       channel.GetBaseURL(),
			ApiType:              apiType,
			ApiVersion:           channel.Other,
			ApiKey:               key,
			UpstreamModelName:    upstreamModelName,
		},
	}

	if GetGeminiAdaptorFunc == nil {
		task.Status = model.TaskStatusFailure
		task.FailReason = "内部错误：Gemini 适配器未初始化"
		task.Progress = "100%"
		task.FinishTime = time.Now().Unix()
		_ = task.Update()
		return
	}

	adaptor := GetGeminiAdaptorFunc(relayInfo.ApiType)
	if adaptor == nil {
		task.Status = model.TaskStatusFailure
		task.FailReason = fmt.Sprintf("不支持的 API 类型: %d", relayInfo.ApiType)
		task.Progress = "100%"
		task.FinishTime = time.Now().Unix()
		_ = task.Update()
		return
	}
	adaptor.Init(relayInfo)

	// Debug: log request details
	logger.LogInfo(ctx, fmt.Sprintf("async_gemini: calling upstream with model=%s, baseUrl=%s, apiType=%d",
		relayInfo.ChannelMeta.UpstreamModelName, relayInfo.ChannelMeta.ChannelBaseUrl, relayInfo.ChannelMeta.ApiType))

	resp, err := adaptor.DoRequest(c, relayInfo, bytes.NewReader(jsonData))
	if err != nil {
		task.Status = model.TaskStatusFailure
		task.FailReason = fmt.Sprintf("请求上游失败: %v", err)
		task.Progress = "100%"
		task.FinishTime = time.Now().Unix()
		_ = task.Update()
		return
	}

	httpResp := resp.(*http.Response)
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		// Debug: log response details
		bodyPreview, _ := io.ReadAll(httpResp.Body)
		httpResp.Body = io.NopCloser(bytes.NewReader(bodyPreview))
		logger.LogError(ctx, fmt.Sprintf("async_gemini: upstream error status=%d, body=%s", httpResp.StatusCode, string(bodyPreview)))

		relayErr := RelayErrorHandler(ctx, httpResp, false)
		task.Status = model.TaskStatusFailure
		task.FailReason = fmt.Sprintf("上游返回错误: %s", relayErr.Error())
		task.Progress = "100%"
		task.FinishTime = time.Now().Unix()
		_ = task.Update()
		return
	}

	bodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		task.Status = model.TaskStatusFailure
		task.FailReason = fmt.Sprintf("读取响应失败: %v", err)
		task.Progress = "100%"
		task.FinishTime = time.Now().Unix()
		_ = task.Update()
		return
	}

	var geminiResp map[string]interface{}
	if err := common.Unmarshal(bodyBytes, &geminiResp); err != nil {
		task.Status = model.TaskStatusFailure
		task.FailReason = fmt.Sprintf("解析响应失败: %v", err)
		task.Progress = "100%"
		task.FinishTime = time.Now().Unix()
		_ = task.Update()
		return
	}

	task.SetData(geminiResp)
	task.Status = model.TaskStatusSuccess
	task.Progress = "100%"
	task.FinishTime = time.Now().Unix()
	_ = task.Update()

	logger.LogInfo(ctx, fmt.Sprintf("async_gemini: task %s completed", task.TaskID))
}
