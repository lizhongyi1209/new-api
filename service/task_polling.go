package service

import (
	"context"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/samber/lo"
)

// TaskPollingAdaptor 定义轮询所需的最小适配器接口，避免 service -> relay 的循环依赖
type TaskPollingAdaptor interface {
	Init(info *relaycommon.RelayInfo)
	FetchTask(baseURL string, key string, params map[string]any, proxy string) (*http.Response, error)
	ParseTaskResult(body []byte) (*relaycommon.TaskInfo, error)
	// AdjustBillingOnComplete 在任务到达终态（成功/失败）时由轮询循环调用。
	// 返回正数触发差额结算（补扣/退还），返回 0 保持预扣费金额不变。
	AdjustBillingOnComplete(task *model.Task, taskResult *relaycommon.TaskInfo) int
}

type BatchTaskPollingAdaptor interface {
	TaskPollingAdaptor
	FetchMode() string
	FetchBatchTasks(baseURL, key string, tasks []*model.Task, proxy string) (*http.Response, error)
	ParseBatchResult(tasks []*model.Task, resp *http.Response, body []byte) (map[string]*BatchTaskResult, error)
}

const (
	pollClassOK           = "ok"
	pollClassOtherClient  = "other_client"
	pollClassNotFound     = "not_found"
	pollClassAuth         = "auth"
	pollClassTransient    = "transient"
	pollClassUnrecognized = "unrecognized"
	pollClassHookError    = "hook_error"
	pollClassTransport    = "transport_error"
)

// TaskContextPollingAdaptor lets plugins poll with the persisted task data and
// state. Legacy adaptors retain the map-based polling contract above.
type TaskContextPollingAdaptor interface {
	FetchTaskWithContext(baseURL, key string, task *model.Task, proxy string) (*http.Response, error)
	ParseTaskResultWithContext(task *model.Task, resp *http.Response, body []byte) (*relaycommon.TaskInfo, error)
}

// BatchTaskResult is the parsed result of one plugin batch-poll item.
// The legacy polling path continues to use TaskPollingAdaptor.
type BatchTaskResult struct {
	TaskInfo   relaycommon.TaskInfo
	Action     string
	SubmitTime int64
	StartTime  int64
	FinishTime int64
	Data       any
}

// GetTaskAdaptorFunc 由 main 包注入，用于获取指定平台的任务适配器。
// 打破 service -> relay -> relay/channel -> service 的循环依赖。
var GetTaskAdaptorFunc func(platform constant.TaskPlatform) TaskPollingAdaptor

// sweepTimedOutTasks 在主轮询之前独立清理超时任务。
// 每次最多处理 100 条，剩余的下个周期继续处理。
// 使用 per-task CAS (UpdateWithStatus) 防止覆盖被正常轮询已推进的任务。
func sweepTimedOutTasks(ctx context.Context) {
	if constant.TaskTimeoutMinutes <= 0 {
		return
	}
	cutoff := time.Now().Unix() - int64(constant.TaskTimeoutMinutes)*60
	tasks := model.GetTimedOutUnfinishedTasks(cutoff, 100)
	if len(tasks) == 0 {
		return
	}

	reason := fmt.Sprintf("任务超时（%d分钟）", constant.TaskTimeoutMinutes)
	legacyReason := "任务超时（旧系统遗留任务，不进行退款，请联系管理员）"
	now := time.Now().Unix()
	timedOutCount := 0

	for _, task := range tasks {
		isLegacy := task.SubmitTime > 0 && task.SubmitTime < model.TaskRefundLegacyCutoff

		oldStatus := task.Status
		task.Status = model.TaskStatusFailure
		task.Progress = "100%"
		task.FinishTime = now
		if isLegacy {
			task.FailReason = legacyReason
			// 旧系统任务明确不退款，随终态 CAS 一并清掉 quota，避免被后续对账误判。
			task.Quota = 0
		} else {
			task.FailReason = reason
		}
		if task.Action == "generateContent" {
			SetGenerateContentRequestOmittedData(task, "timeout")
		}

		won, err := task.UpdateWithStatus(oldStatus)
		if err != nil {
			logger.LogError(ctx, fmt.Sprintf("sweepTimedOutTasks CAS update error for task %s: %v", task.TaskID, err))
			continue
		}
		if !won {
			logger.LogInfo(ctx, fmt.Sprintf("sweepTimedOutTasks: task %s already transitioned, skip", task.TaskID))
			continue
		}
		timedOutCount++
		if !isLegacy && task.Quota != 0 {
			RefundTaskQuota(ctx, task, reason)
		}
	}

	if timedOutCount > 0 {
		logger.LogInfo(ctx, fmt.Sprintf("sweepTimedOutTasks: timed out %d tasks", timedOutCount))
	}
}

// TaskPollSummary is the result recorded on an async_task_poll system task row,
// summarizing one polling pass.
type TaskPollSummary struct {
	UnfinishedTasks  int `json:"unfinished_tasks"`
	PlatformsScanned int `json:"platforms_scanned"`
	NullTasksFailed  int `json:"null_tasks_failed"`
}

// RunTaskPollingOnce performs one async-task (Suno/video) polling pass
// synchronously. It honors ctx cancellation (the system-task runner cancels it
// when the lease is lost) and, when report is non-nil, reports progress as
// (processedPlatforms, totalPlatforms). It returns immediately if the task
// adaptor factory has not been wired yet, to avoid a nil call during startup.
func RunTaskPollingOnce(ctx context.Context, report func(processed, total int)) TaskPollSummary {
	summary := TaskPollSummary{}
	if GetTaskAdaptorFunc == nil {
		return summary
	}
	if ctx == nil {
		ctx = context.Background()
	}

	common.SysLog("任务进度轮询开始")
	sweepTimedOutTasks(ctx)
	allTasks := model.GetAllUnFinishSyncTasks(constant.TaskQueryLimit)
	summary.UnfinishedTasks = len(allTasks)
	platformTask := make(map[constant.TaskPlatform][]*model.Task)
	for _, t := range allTasks {
		platformTask[t.Platform] = append(platformTask[t.Platform], t)
	}

	totalPlatforms := len(platformTask)
	processedPlatforms := 0
	for platform, tasks := range platformTask {
		if ctx.Err() != nil {
			break
		}
		if report != nil {
			report(processedPlatforms, totalPlatforms)
		}
		processedPlatforms++
		if len(tasks) == 0 {
			continue
		}
		summary.PlatformsScanned++
		pluginTasks := make([]*model.Task, 0)
		legacyTasks := make([]*model.Task, 0, len(tasks))
		for _, task := range tasks {
			if task.PrivateData.Execution != nil && task.PrivateData.Execution.TaskPlugin != nil {
				pluginTasks = append(pluginTasks, task)
			} else {
				legacyTasks = append(legacyTasks, task)
			}
		}
		if len(pluginTasks) > 0 {
			if err := UpdateTaskPluginTasks(ctx, platform, pluginTasks); err != nil {
				logger.LogError(ctx, fmt.Sprintf("Task plugin %s polling failed: %s", platform, err))
			}
		}
		if len(legacyTasks) == 0 {
			continue
		}
		taskChannelM := make(map[int][]string)
		taskM := make(map[string]*model.Task)
		nullTaskIds := make([]int64, 0)
		for _, task := range legacyTasks {
			upstreamID := task.GetUpstreamTaskID()
			if upstreamID == "" {
				// 统计失败的未完成任务
				nullTaskIds = append(nullTaskIds, task.ID)
				continue
			}
			taskM[upstreamID] = task
			taskChannelM[task.ChannelId] = append(taskChannelM[task.ChannelId], upstreamID)
		}
		if len(nullTaskIds) > 0 {
			summary.NullTasksFailed += len(nullTaskIds)
			err := model.TaskBulkUpdateByID(nullTaskIds, map[string]any{
				"status":   "FAILURE",
				"progress": "100%",
			})
			if err != nil {
				logger.LogError(ctx, fmt.Sprintf("Fix null task_id task error: %v", err))
			} else {
				logger.LogInfo(ctx, fmt.Sprintf("Fix null task_id task success: %v", nullTaskIds))
			}
		}
		if len(taskChannelM) == 0 {
			continue
		}

		DispatchPlatformUpdate(ctx, platform, taskChannelM, taskM)
	}
	if report != nil && ctx.Err() == nil {
		report(totalPlatforms, totalPlatforms)
	}
	common.SysLog("任务进度轮询完成")
	return summary
}

// UpdateTaskPluginTasks keeps each task's persisted plugin state attached to
// its poll and does not use upstream task IDs as map keys. Providers may reuse
// those IDs across channels, while public tasks remain distinct.
func UpdateTaskPluginTasks(ctx context.Context, platform constant.TaskPlatform, tasks []*model.Task) error {
	byChannel := make(map[int][]*model.Task)
	for _, task := range tasks {
		byChannel[task.ChannelId] = append(byChannel[task.ChannelId], task)
	}
	channelIDs := make([]int, 0, len(byChannel))
	for channelID := range byChannel {
		channelIDs = append(channelIDs, channelID)
	}
	sort.Ints(channelIDs)
	for _, channelID := range channelIDs {
		if err := ctx.Err(); err != nil {
			return err
		}
		ch, err := model.CacheGetChannel(channelID)
		if err != nil {
			logger.LogError(ctx, fmt.Sprintf("Task plugin channel #%d unavailable: %s", channelID, err))
			continue
		}
		if ch.Type != constant.ChannelTypeTaskPlugin || ch.GetSetting().TaskPluginKey != string(platform) {
			logger.LogError(ctx, fmt.Sprintf("Task plugin channel #%d identity changed; skipping pending tasks", channelID))
			continue
		}
		adaptor := GetTaskAdaptorFunc(platform)
		if adaptor == nil {
			logger.LogError(ctx, fmt.Sprintf("Task plugin %s adaptor is unavailable", platform))
			continue
		}
		if _, ok := adaptor.(TaskContextPollingAdaptor); !ok {
			logger.LogError(ctx, fmt.Sprintf("Task plugin %s adaptor cannot poll persisted tasks", platform))
			continue
		}
		info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelBaseUrl: ch.GetBaseURL(), ApiKey: ch.Key}}
		adaptor.Init(info)
		for index, task := range byChannel[channelID] {
			if err := ctx.Err(); err != nil {
				return err
			}
			if task.GetUpstreamTaskID() == "" {
				logger.LogError(ctx, fmt.Sprintf("Task plugin task %s has no upstream task ID", task.TaskID))
				continue
			}
			if err := updateVideoSingleTask(ctx, adaptor, ch, task); err != nil {
				logger.LogError(ctx, fmt.Sprintf("Task plugin task %s poll failed: %s", task.TaskID, err))
			}
			if ch.GetOtherSettings().DisableTaskPollingSleep || index == len(byChannel[channelID])-1 {
				continue
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Second):
			}
		}
	}
	return nil
}

// DispatchPlatformUpdate 按平台分发轮询更新
func DispatchPlatformUpdate(ctx context.Context, platform constant.TaskPlatform, taskChannelM map[int][]string, taskM map[string]*model.Task) {
	if ctx == nil {
		ctx = context.Background()
	}
	if platform == constant.TaskPlatformMidjourney {
		// MJ 轮询由其自身处理，这里预留入口
		return
	}
	adaptor := GetTaskAdaptorFunc(platform)
	if batchAdaptor, ok := adaptor.(BatchTaskPollingAdaptor); ok && batchAdaptor.FetchMode() == "batch" {
		if err := UpdateBatchTasks(ctx, batchAdaptor, taskChannelM, taskM); err != nil {
			common.SysLog(fmt.Sprintf("UpdateBatchTasks fail: %s", err))
		}
		return
	}
	if err := UpdateVideoTasks(ctx, platform, taskChannelM, taskM); err != nil {
		common.SysLog(fmt.Sprintf("UpdateVideoTasks fail: %s", err))
	}
}

func UpdateBatchTasks(ctx context.Context, adaptor BatchTaskPollingAdaptor, taskChannelM map[int][]string, taskM map[string]*model.Task) error {
	for channelId, taskIds := range taskChannelM {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		err := updateBatchTasks(ctx, adaptor, channelId, taskIds, taskM)
		if err != nil {
			logger.LogError(ctx, fmt.Sprintf("渠道 #%d 更新异步任务失败: %s", channelId, err.Error()))
		}
	}
	return nil
}

func updateBatchTasks(ctx context.Context, adaptor BatchTaskPollingAdaptor, channelId int, taskIds []string, taskM map[string]*model.Task) error {
	logger.LogInfo(ctx, fmt.Sprintf("渠道 #%d 未完成的任务有: %d", channelId, len(taskIds)))
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if len(taskIds) == 0 {
		return nil
	}
	ch, err := model.CacheGetChannel(channelId)
	if err != nil {
		common.SysLog(fmt.Sprintf("CacheGetChannel: %v", err))
		// Collect DB primary key IDs for bulk update (taskIds are upstream IDs, not task_id column values)
		var failedIDs []int64
		for _, upstreamID := range taskIds {
			if t, ok := taskM[upstreamID]; ok {
				failedIDs = append(failedIDs, t.ID)
			}
		}
		err = model.TaskBulkUpdateByID(failedIDs, map[string]any{
			"fail_reason": fmt.Sprintf("获取渠道信息失败，请联系管理员，渠道ID：%d", channelId),
			"status":      "FAILURE",
			"progress":    "100%",
		})
		if err != nil {
			common.SysLog(fmt.Sprintf("UpdateSunoTask error: %v", err))
		}
		return err
	}
	proxy := ch.GetSetting().Proxy
	baseURL := ch.GetBaseURL()
	if baseURL == "" {
		baseURL = constant.GetChannelBaseURL(ch.Type)
	}
	tasks := make([]*model.Task, 0, len(taskIds))
	for _, upstreamID := range taskIds {
		if task := taskM[upstreamID]; task != nil {
			tasks = append(tasks, task)
		}
	}
	// The channel type tells plugin adaptors whether the upstream is another
	// New API gateway, the same signal submission derives from the request.
	info := &relaycommon.RelayInfo{}
	info.ChannelMeta = &relaycommon.ChannelMeta{ChannelType: ch.Type, ChannelId: ch.Id, ChannelBaseUrl: baseURL}
	info.ApiKey = ch.Key
	adaptor.Init(info)
	resp, err := adaptor.FetchBatchTasks(baseURL, ch.Key, tasks, proxy)
	if err != nil {
		common.SysLog(fmt.Sprintf("Get Task Do req error: %v", err))
		return recordPollFailureForTasks(ctx, adaptor, tasks, pollClassTransport, 0, err.Error())
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		common.SysLog(fmt.Sprintf("Get Suno Task parse body error: %v", err))
		return recordPollFailureForTasks(ctx, adaptor, tasks, pollClassTransport, resp.StatusCode, err.Error())
	}
	switch classifyPollHTTP(resp.StatusCode) {
	case pollClassNotFound:
		return failTasksFromPoll(ctx, adaptor, tasks, fmt.Sprintf("upstream task not found (HTTP %d)", resp.StatusCode))
	case pollClassAuth:
		logger.LogWarn(ctx, fmt.Sprintf("task poll auth failure channel_id=%d http=%d", channelId, resp.StatusCode))
		return recordPollFailureForTasks(ctx, adaptor, tasks, pollClassAuth, resp.StatusCode, "")
	case pollClassTransient:
		return recordPollFailureForTasks(ctx, adaptor, tasks, pollClassTransient, resp.StatusCode, "")
	}
	responseItems, err := adaptor.ParseBatchResult(tasks, resp, responseBody)
	if err != nil {
		return recordPollFailureForTasks(ctx, adaptor, tasks, pollClassHookError, resp.StatusCode, err.Error())
	}
	for upstreamID, responseItem := range responseItems {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		task := taskM[upstreamID]
		if task == nil {
			logger.LogWarn(ctx, fmt.Sprintf("Batch task response ignored: unknown task_id=%s", upstreamID))
			continue
		}
		snap := task.Snapshot()
		parsedStatus := model.TaskStatus(responseItem.TaskInfo.Status)
		if parsedStatus == model.TaskStatusUnknown || parsedStatus == "" || !knownPollStatus(parsedStatus) {
			if err := recordPollFailure(ctx, adaptor, task, snap.Status, pollClassUnrecognized, resp.StatusCode, responseItem.TaskInfo.Reason); err != nil {
				common.SysLog("UpdateSunoTask task error: " + err.Error())
			}
			continue
		}

		prevStatus := task.Status
		task.Status = lo.If(parsedStatus != "", parsedStatus).Else(task.Status)
		task.FailReason = lo.If(responseItem.TaskInfo.Reason != "", responseItem.TaskInfo.Reason).Else(task.FailReason)
		task.SubmitTime = lo.If(responseItem.SubmitTime != 0, responseItem.SubmitTime).Else(task.SubmitTime)
		task.StartTime = lo.If(responseItem.StartTime != 0, responseItem.StartTime).Else(task.StartTime)
		task.FinishTime = lo.If(responseItem.FinishTime != 0, responseItem.FinishTime).Else(task.FinishTime)
		isFailure := responseItem.TaskInfo.Reason != "" || task.Status == model.TaskStatusFailure
		if isFailure {
			logger.LogInfo(ctx, task.TaskID+" 构建失败，"+task.FailReason)
			task.Status = model.TaskStatusFailure
			task.Progress = "100%"
		}
		if responseItem.TaskInfo.Status == model.TaskStatusSuccess {
			task.Progress = "100%"
		}
		if responseItem.Data != nil {
			task.SetData(responseItem.Data)
		} else if task.Status == model.TaskStatusSuccess || task.Status == model.TaskStatusFailure {
			logger.LogWarn(ctx, fmt.Sprintf(
				"Batch task %s reached terminal status without data; preserving existing task data",
				task.TaskID,
			))
		}
		if responseItem.TaskInfo.Url != "" {
			task.PrivateData.ResultURL = responseItem.TaskInfo.Url
		}

		// 持久化走 CAS，防止重叠轮询/sweep/多实例/持久化失败重试导致重复退款或覆盖终态。
		won, err := task.UpdateWithStatus(prevStatus)
		if err != nil {
			logger.LogError(ctx, fmt.Sprintf("UpdateSunoTask task %s error: %v", task.TaskID, err))
		} else if !won {
			logger.LogWarn(ctx, fmt.Sprintf("Task %s CAS lost or no-op update, skip billing", task.TaskID))
		} else if (task.Status == model.TaskStatusSuccess || task.Status == model.TaskStatusFailure) &&
			prevStatus != model.TaskStatusSuccess && prevStatus != model.TaskStatusFailure {
			finalizeTerminalTask(ctx, adaptor, task, &responseItem.TaskInfo)
		}
	}
	return nil
}

// UpdateVideoTasks 按渠道更新所有视频任务
func UpdateVideoTasks(ctx context.Context, platform constant.TaskPlatform, taskChannelM map[int][]string, taskM map[string]*model.Task) error {
	channelIDs := make([]int, 0, len(taskChannelM))
	for channelID := range taskChannelM {
		channelIDs = append(channelIDs, channelID)
	}
	sort.Ints(channelIDs)

	var wg sync.WaitGroup
	for _, channelId := range channelIDs {
		taskIds := taskChannelM[channelId]
		if len(taskIds) == 0 {
			continue
		}
		taskIds = append([]string(nil), taskIds...)

		wg.Add(1)
		gopool.Go(func() {
			defer wg.Done()
			if err := updateVideoTasks(ctx, platform, channelId, taskIds, taskM); err != nil {
				logger.LogError(ctx, fmt.Sprintf("Channel #%d failed to update video async tasks: %s", channelId, err.Error()))
			}
		})
	}
	wg.Wait()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return nil
}

func updateVideoTasks(ctx context.Context, platform constant.TaskPlatform, channelId int, taskIds []string, taskM map[string]*model.Task) error {
	logger.LogDebug(ctx, fmt.Sprintf("Channel #%d pending video tasks: %d", channelId, len(taskIds)))
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if len(taskIds) == 0 {
		return nil
	}
	cacheGetChannel, err := model.CacheGetChannel(channelId)
	if err != nil {
		// Collect DB primary key IDs for bulk update (taskIds are upstream IDs, not task_id column values)
		var failedIDs []int64
		for _, upstreamID := range taskIds {
			if t, ok := taskM[upstreamID]; ok {
				failedIDs = append(failedIDs, t.ID)
			}
		}
		errUpdate := model.TaskBulkUpdateByID(failedIDs, map[string]any{
			"fail_reason": fmt.Sprintf("Failed to get channel info, channel ID: %d", channelId),
			"status":      "FAILURE",
			"progress":    "100%",
		})
		if errUpdate != nil {
			common.SysLog(fmt.Sprintf("UpdateVideoTask error: %v", errUpdate))
		}
		return fmt.Errorf("CacheGetChannel failed: %w", err)
	}
	adaptor := GetTaskAdaptorFunc(platform)
	if adaptor == nil {
		return fmt.Errorf("video adaptor not found")
	}
	info := &relaycommon.RelayInfo{}
	info.ChannelMeta = &relaycommon.ChannelMeta{
		ChannelType:    cacheGetChannel.Type,
		ChannelId:      cacheGetChannel.Id,
		ChannelBaseUrl: cacheGetChannel.GetBaseURL(),
	}
	info.ApiKey = cacheGetChannel.Key
	adaptor.Init(info)
	disablePollingSleep := cacheGetChannel.GetOtherSettings().DisableTaskPollingSleep
	for i, taskId := range taskIds {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		task := taskM[taskId]
		if task == nil {
			logger.LogError(ctx, fmt.Sprintf("Task %s not found in taskM", taskId))
			continue
		}
		if err := updateVideoSingleTask(ctx, adaptor, cacheGetChannel, task); err != nil {
			logger.LogError(ctx, fmt.Sprintf("Failed to update video task %s: %s", taskId, err.Error()))
		}
		if disablePollingSleep || i == len(taskIds)-1 {
			continue
		}

		// sleep 1 second between tasks for this channel only.
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
		}
	}
	return nil
}

func updateVideoSingleTask(ctx context.Context, adaptor TaskPollingAdaptor, ch *model.Channel, task *model.Task) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if task == nil {
		return fmt.Errorf("task is required")
	}
	taskId := task.GetUpstreamTaskID()
	baseURL := constant.ChannelBaseURLs[ch.Type]
	if ch.GetBaseURL() != "" {
		baseURL = ch.GetBaseURL()
	}
	proxy := ch.GetSetting().Proxy

	key := ch.Key

	privateData := task.PrivateData
	snap := task.Snapshot()
	if privateData.Key != "" {
		key = privateData.Key
	}
	var resp *http.Response
	var err error
	contextPoller, pluginPoll := adaptor.(TaskContextPollingAdaptor)
	if pluginPoll {
		resp, err = contextPoller.FetchTaskWithContext(baseURL, key, task, proxy)
	} else {
		resp, err = adaptor.FetchTask(baseURL, key, map[string]any{
			"task_id": task.GetUpstreamTaskID(), "action": task.Action,
			"model": task.Properties.OriginModelName, "upstream_model": task.Properties.UpstreamModelName,
		}, proxy)
	}
	if err != nil {
		return recordPollFailure(ctx, adaptor, task, snap.Status, pollClassTransport, 0, err.Error())
	}
	defer resp.Body.Close()
	var responseReader io.Reader = resp.Body
	if pluginPoll {
		responseReader = io.LimitReader(resp.Body, (1<<20)+1)
	}
	responseBody, err := io.ReadAll(responseReader)
	if err != nil {
		return recordPollFailure(ctx, adaptor, task, snap.Status, pollClassTransport, resp.StatusCode, err.Error())
	}
	if pluginPoll && len(responseBody) > 1<<20 {
		return fmt.Errorf("plugin poll response exceeds size limit for task %s", taskId)
	}

	logger.LogDebug(ctx, "updateVideoSingleTask response: %s", responseBody)

	taskResult := &relaycommon.TaskInfo{}
	// try parse as New API response format
	var responseItems taskdto.TaskResponse[model.Task]
	if pluginPoll {
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			return fmt.Errorf("plugin poll returned HTTP %d for task %s", resp.StatusCode, taskId)
		}
		taskResult, err = contextPoller.ParseTaskResultWithContext(task, resp, responseBody)
		if err != nil {
			return fmt.Errorf("parse plugin task result failed for task %s: %w", taskId, err)
		}
	} else if err = common.Unmarshal(responseBody, &responseItems); err == nil && responseItems.IsSuccess() {
		logger.LogDebug(ctx, "updateVideoSingleTask parsed as new api response format: %+v", responseItems)
		t := responseItems.Data
		taskResult.TaskID = t.TaskID
		taskResult.Status = string(t.Status)
		taskResult.Url = t.GetResultURL()
		taskResult.Progress = t.Progress
		taskResult.Reason = t.FailReason
		task.Data = t.Data
	} else if taskResult, err = adaptor.ParseTaskResult(responseBody); err != nil {
		return recordPollFailure(ctx, adaptor, task, snap.Status, pollClassHookError, resp.StatusCode, err.Error())
	}

	if !pluginPoll {
		if err := ApplyVideoOutputStrategy(ctx, ch, task, taskResult); err != nil {
			return fmt.Errorf("apply video output strategy for task %s: %w", task.TaskID, err)
		}
	}
	return ApplyVideoTaskPollingResult(ctx, adaptor, task, taskResult, responseBody)
}

// ApplyVideoOutputStrategy moves a completed video into the durable object
// storage selected on its channel. Empty and passthrough strategies preserve
// the upstream result URL.
func ApplyVideoOutputStrategy(ctx context.Context, ch *model.Channel, task *model.Task, taskResult *relaycommon.TaskInfo) error {
	if ch == nil || task == nil || taskResult == nil {
		return fmt.Errorf("channel, task and task result are required")
	}
	if taskResult.Status != model.TaskStatusSuccess {
		return nil
	}

	strategy := ch.GetOtherSettings().ImageOutputStrategy
	if strategy == "" || strategy == taskdto.ImageOutputStrategyPassthrough {
		return nil
	}
	if strategy != taskdto.ImageOutputStrategyOSS && strategy != taskdto.ImageOutputStrategyR2 {
		return nil
	}

	source := strings.TrimSpace(taskResult.Url)
	usedRemoteURL := false
	if source == "" {
		source = strings.TrimSpace(taskResult.RemoteUrl)
		usedRemoteURL = source != ""
	}

	downloadHeaders := make(http.Header)
	if source == "" && (ch.Type == constant.ChannelTypeOpenAI || ch.Type == constant.ChannelTypeSora) {
		baseURL := strings.TrimRight(ch.GetBaseURL(), "/")
		if baseURL == "" {
			baseURL = strings.TrimRight(constant.ChannelBaseURLs[ch.Type], "/")
		}
		upstreamTaskID := strings.TrimSpace(task.GetUpstreamTaskID())
		if baseURL == "" || upstreamTaskID == "" {
			return fmt.Errorf("completed video has no downloadable output")
		}
		source = fmt.Sprintf("%s/v1/videos/%s/content", baseURL, url.PathEscape(upstreamTaskID))
		apiKey := strings.TrimSpace(task.PrivateData.Key)
		if apiKey == "" {
			apiKey = strings.TrimSpace(ch.Key)
		}
		if apiKey == "" {
			return fmt.Errorf("video download key is unavailable")
		}
		downloadHeaders.Set("Authorization", "Bearer "+apiKey)
	}
	if source == "" {
		return fmt.Errorf("completed video has no downloadable output")
	}

	// Gemini operation responses expose a download URI in RemoteUrl that needs
	// the task's upstream API key. Omni responses populate Url directly and must
	// not receive this query parameter because they may use a third-party host.
	if usedRemoteURL && ch.Type == constant.ChannelTypeGemini {
		apiKey := strings.TrimSpace(task.PrivateData.Key)
		if apiKey == "" {
			apiKey = strings.TrimSpace(ch.Key)
		}
		if apiKey == "" {
			return fmt.Errorf("Gemini video download key is unavailable")
		}
		parsedSource, err := url.Parse(source)
		if err != nil {
			return fmt.Errorf("parse Gemini video download URL: %w", err)
		}
		query := parsedSource.Query()
		if query.Get("key") == "" {
			query.Set("key", apiKey)
			parsedSource.RawQuery = query.Encode()
			source = parsedSource.String()
		}
	}

	downloadClient := GetSSRFProtectedHTTPClient()
	if proxy := strings.TrimSpace(ch.GetSetting().Proxy); proxy != "" {
		proxyClient, err := GetHttpClientWithProxy(proxy)
		if err != nil {
			return fmt.Errorf("create video output proxy client: %w", err)
		}
		downloadClient = proxyClient
	}

	var (
		storedURL string
		err       error
	)
	if strategy == taskdto.ImageOutputStrategyOSS {
		storedURL, err = cacheVideoOutputToOSS(ctx, source, downloadClient, downloadHeaders)
	} else {
		storedURL, err = cacheVideoOutputToR2(ctx, source, downloadClient, downloadHeaders)
	}
	if err != nil {
		return err
	}
	taskResult.Url = storedURL
	taskResult.RemoteUrl = ""
	return nil
}

// ApplyVideoTaskPollingResult atomically persists a parsed video-task poll,
// settles terminal usage, and finalizes the original consumption log. Both the
// background poller and request-time Gemini/Vertex refreshes use this path so a
// client status check cannot move a task to terminal state without billing it.
func ApplyVideoTaskPollingResult(ctx context.Context, adaptor TaskPollingAdaptor, task *model.Task, taskResult *relaycommon.TaskInfo, responseBody []byte) error {
	if task == nil || taskResult == nil {
		return fmt.Errorf("task and task result are required")
	}
	snap := task.Snapshot()
	parsedStatus := model.TaskStatus(taskResult.Status)
	if parsedStatus == model.TaskStatusUnknown || parsedStatus == "" || !knownPollStatus(parsedStatus) {
		return recordPollFailure(ctx, adaptor, task, snap.Status, pollClassUnrecognized, 0, unrecognizedPollDetail(taskResult.Reason, responseBody))
	}

	pluginTask := task.PrivateData.Execution != nil && task.PrivateData.Execution.TaskPlugin != nil
	if !pluginTask {
		task.Data = redactVideoResponseBody(responseBody)
	}
	if len(taskResult.PluginState) > 0 {
		task.PrivateData.PluginState = append([]byte(nil), taskResult.PluginState...)
	}
	task.PrivateData.PollFailures = 0

	now := time.Now().Unix()
	task.Status = parsedStatus
	switch parsedStatus {
	case model.TaskStatusSubmitted:
		task.Progress = taskcommon.ProgressSubmitted
	case model.TaskStatusQueued:
		task.Progress = taskcommon.ProgressQueued
	case model.TaskStatusInProgress:
		task.Progress = taskcommon.ProgressInProgress
		if task.StartTime == 0 {
			task.StartTime = now
		}
	case model.TaskStatusSuccess:
		task.Progress = taskcommon.ProgressComplete
		if task.FinishTime == 0 {
			task.FinishTime = now
		}
		if strings.HasPrefix(taskResult.Url, "data:") {
			task.PrivateData.ResultURL = taskcommon.BuildProxyURL(task.TaskID)
		} else if taskResult.Url != "" {
			task.PrivateData.ResultURL = taskResult.Url
		} else if !pluginTask {
			task.PrivateData.ResultURL = taskcommon.BuildProxyURL(task.TaskID)
		}
	case model.TaskStatusFailure:
		task.Progress = taskcommon.ProgressComplete
		if task.FinishTime == 0 {
			task.FinishTime = now
		}
		task.FailReason = taskResult.Reason
	}
	if taskResult.Progress != "" {
		task.Progress = taskResult.Progress
	}

	terminal := parsedStatus == model.TaskStatusSuccess || parsedStatus == model.TaskStatusFailure
	if terminal && snap.Status != parsedStatus {
		won, err := task.UpdateWithStatus(snap.Status)
		if err != nil {
			return fmt.Errorf("persist terminal task %s: %w", task.TaskID, err)
		}
		if !won {
			return nil
		}
		perfmetrics.RecordTaskResult(task, taskResult)
		settled := settleTaskBillingOnComplete(ctx, adaptor, task, taskResult)
		refundedQuota := 0
		refundStatus := "not_applicable"
		chargedQuota := task.Quota
		if parsedStatus == model.TaskStatusFailure {
			refundStatus = "not_required"
			if !settled && task.Quota != 0 {
				refundStatus = "failed"
				if RefundTaskQuota(ctx, task, task.FailReason) {
					refundedQuota = chargedQuota
					refundStatus = "succeeded"
				}
			}
		}
		FinalizeTaskConsumptionLog(task, chargedQuota, refundedQuota, refundStatus, taskResult)
		return nil
	}
	if !snap.Equal(task.Snapshot()) {
		if _, err := task.UpdateWithStatus(snap.Status); err != nil {
			return fmt.Errorf("persist task %s: %w", task.TaskID, err)
		}
	}
	return nil
}

// finalizeTerminalTask 终态统一收尾（状态 CAS 赢家调用，恰好一次）：采样 + 结算 + 失败兜底退款。
func finalizeTerminalTask(ctx context.Context, adaptor TaskPollingAdaptor, task *model.Task, taskResult *relaycommon.TaskInfo) {
	perfmetrics.RecordTaskResult(task, taskResult)
	billingSettled := settleTaskBillingOnComplete(ctx, adaptor, task, taskResult)
	if task.Status == model.TaskStatusFailure && !billingSettled && task.Quota != 0 {
		RefundTaskQuota(ctx, task, task.FailReason)
	}
}

func redactVideoResponseBody(body []byte) []byte {
	var m map[string]any
	if err := common.Unmarshal(body, &m); err != nil {
		return body
	}
	resp, _ := m["response"].(map[string]any)
	if resp != nil {
		delete(resp, "bytesBase64Encoded")
		if v, ok := resp["video"].(string); ok {
			resp["video"] = truncateBase64(v)
		}
		if vs, ok := resp["videos"].([]any); ok {
			for i := range vs {
				if vm, ok := vs[i].(map[string]any); ok {
					delete(vm, "bytesBase64Encoded")
				}
			}
		}
	}
	b, err := common.Marshal(m)
	if err != nil {
		return body
	}
	return b
}

func truncateBase64(s string) string {
	const maxKeep = 256
	if len(s) <= maxKeep {
		return s
	}
	return s[:maxKeep] + "..."
}

// settleTaskBillingOnComplete 任务完成时的统一计费调整。
// 返回 true 表示用量结算路径已接管最终计费；失败任务仅在返回 false 时补做全额退款。
// 优先级：1. tiered snapshot → 2. adaptor 调整 → 3. token 重算。
//
//  2. taskResult.TotalTokens > 0 → 按 token 重算
//  3. 都不满足 → 保持预扣额度不变
func settleTaskBillingOnComplete(ctx context.Context, adaptor TaskPollingAdaptor, task *model.Task, taskResult *relaycommon.TaskInfo) bool {
	// 异步视频在提交时还不知道真实用量。完成轮询后先回填日志主字段，
	// 记录用量与是否需要差额结算是两个独立行为：按次计费、倍率缺失、
	// 或预扣恰好准确时，也必须在使用日志中展示输入/输出 tokens。
	promptTokens := taskResult.PromptTokens
	if promptTokens == 0 && taskResult.TotalTokens >= taskResult.CompletionTokens {
		promptTokens = taskResult.TotalTokens - taskResult.CompletionTokens
	}
	if task.PrivateData.SubmitLogID > 0 {
		model.UpdateConsumeLogQuotaAndOther(task.PrivateData.SubmitLogID, task.Quota, map[string]interface{}{
			"prompt_tokens":     promptTokens,
			"completion_tokens": taskResult.CompletionTokens,
			"use_time_seconds":  taskUseTimeSeconds(task),
		})
	}
	if task.Status == model.TaskStatusFailure {
		return false
	}
	if bc := task.PrivateData.BillingContext; bc != nil && len(bc.TieredSnapshot) > 0 {
		var snap billingexpr.BillingSnapshot
		if err := common.Unmarshal(bc.TieredSnapshot, &snap); err != nil {
			logger.LogError(ctx, fmt.Sprintf("task %s plugin billing snapshot is invalid: %v", task.TaskID, err))
			return true
		}
		if snap.TaskUsageBilling {
			usage := make(map[string]any, len(snap.UsageFacts)+len(taskResult.UsageFacts))
			maps.Copy(usage, snap.UsageFacts)
			maps.Copy(usage, taskResult.UsageFacts)
			result, err := billingexpr.ComputeTieredQuotaWithRequest(&snap, billingexpr.TokenParams{}, snap.TaskRequestInput(usage))
			if err != nil {
				logger.LogWarn(ctx, fmt.Sprintf("task %s usage settlement failed; retained reserved quota: %v", task.TaskID, err))
				return true
			}
			RecalculateTaskQuotaWithTier(ctx, task, result.ActualQuotaAfterGroup, "任务用量表达式结算", result.MatchedTier, result.Clamp)
			if task.Quota == result.ActualQuotaAfterGroup {
				snap.UsageFacts = usage
				snap.EstimatedTier = result.MatchedTier
				if encoded, marshalErr := common.Marshal(&snap); marshalErr == nil {
					bc.TieredSnapshot = encoded
					if task.ID > 0 {
						if updateErr := task.UpdatePrivateData(); updateErr != nil {
							logger.LogError(ctx, fmt.Sprintf("task %s billing snapshot update failed: %v", task.TaskID, updateErr))
						}
					}
				} else {
					logger.LogError(ctx, fmt.Sprintf("task %s billing snapshot encoding failed: %v", task.TaskID, marshalErr))
				}
			}
			if task.PrivateData.SubmitLogID > 0 {
				model.UpdateConsumeLogQuotaAndOther(task.PrivateData.SubmitLogID, task.Quota, map[string]interface{}{
					"usage_facts": usage,
				})
			}
			return true
		}
	}

	// 0. 按次计费的任务不做差额结算
	if bc := task.PrivateData.BillingContext; bc != nil && bc.PerCallBilling {
		logger.LogInfo(ctx, fmt.Sprintf("任务 %s 按次计费，跳过差额结算", task.TaskID))
		return false
	}
	// 优先让 adaptor 决定最终额度。
	if actualQuota := adaptor.AdjustBillingOnComplete(task, taskResult); actualQuota > 0 {
		RecalculateTaskQuota(ctx, task, actualQuota, "adaptor计费调整", taskResult.QuotaClamp)
		if taskResult.VideoBilling != nil && task.PrivateData.BillingContext != nil {
			taskResult.VideoBilling.FinalQuota = task.Quota
			taskResult.VideoBilling.SettlementDeltaQuota = task.Quota - taskResult.VideoBilling.PreConsumedQuota
			task.PrivateData.BillingContext.VideoBilling = taskResult.VideoBilling
			if err := task.UpdatePrivateData(); err != nil {
				logger.LogError(ctx, fmt.Sprintf("persist video billing audit failed for task %s: %s", task.TaskID, err.Error()))
			}
		}
		return true
	}
	// 回退到 token 重算。
	tokens := taskResult.TotalTokens
	if tokens == 0 && taskResult.CompletionTokens > 0 {
		tokens = taskResult.CompletionTokens
	}
	if tokens > 0 {
		return RecalculateTaskQuotaByTokens(ctx, task, tokens)
	}
	return false
}

func classifyPollHTTP(statusCode int) string {
	switch {
	case statusCode >= 200 && statusCode < 300:
		return pollClassOK
	case statusCode == http.StatusNotFound || statusCode == http.StatusGone:
		return pollClassNotFound
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden:
		return pollClassAuth
	case statusCode == http.StatusTooManyRequests || statusCode >= 500:
		return pollClassTransient
	case statusCode >= 400 && statusCode < 500:
		return pollClassOtherClient
	default:
		return pollClassTransient
	}
}

func knownPollStatus(status model.TaskStatus) bool {
	switch status {
	case model.TaskStatusNotStart, model.TaskStatusSubmitted, model.TaskStatusQueued, model.TaskStatusInProgress, model.TaskStatusSuccess, model.TaskStatusFailure:
		return true
	default:
		return false
	}
}

func isNonTerminalPollStatus(status model.TaskStatus) bool {
	switch status {
	case model.TaskStatusNotStart, model.TaskStatusSubmitted, model.TaskStatusQueued, model.TaskStatusInProgress:
		return true
	default:
		return false
	}
}

func pollFailureReason(class string, statusCode int, detail string) string {
	reason := fmt.Sprintf("poll failed: %s", class)
	if statusCode > 0 {
		reason = fmt.Sprintf("poll failed: %s (HTTP %d)", class, statusCode)
	}
	if detail != "" {
		reason = reason + ": " + detail
	}
	return reason
}

// unrecognizedPollDetail pairs the plugin's reason with a bounded copy of the
// upstream body so the WARN line is enough to diagnose a parser gap.
func unrecognizedPollDetail(reason string, body []byte) string {
	const maxBodyChars = 512
	redacted := string(redactVideoResponseBody(body))
	if len(redacted) > maxBodyChars {
		redacted = redacted[:maxBodyChars] + "…"
	}
	if strings.TrimSpace(reason) == "" {
		return "body=" + redacted
	}
	return reason + "; body=" + redacted
}

func recordPollFailure(ctx context.Context, adaptor TaskPollingAdaptor, task *model.Task, fromStatus model.TaskStatus, class string, statusCode int, detail string) error {
	task.PrivateData.PollFailures++
	if class == pollClassUnrecognized || class == pollClassHookError {
		// The redacted body is intentionally not persisted to Task.Data on these
		// paths, so the WARN line is the only operator-visible copy of what the
		// plugin could not interpret.
		logger.LogWarn(ctx, fmt.Sprintf("task %s poll %s (failures=%d, http=%d): %s", task.TaskID, class, task.PrivateData.PollFailures, statusCode, detail))
	}
	// TASK_POLL_MAX_FAILURES <= 0 disables the consecutive-failure cutoff, matching
	// TASK_TIMEOUT_MINUTES semantics; the 24h sweep remains the only backstop.
	if constant.TaskPollMaxFailures > 0 && task.PrivateData.PollFailures >= constant.TaskPollMaxFailures {
		return failTaskFromPoll(ctx, adaptor, task, fromStatus, pollFailureReason(class, statusCode, detail))
	}
	if _, err := task.UpdateWithStatus(fromStatus); err != nil {
		return err
	}
	return nil
}

func recordPollFailureForTasks(ctx context.Context, adaptor TaskPollingAdaptor, tasks []*model.Task, class string, statusCode int, detail string) error {
	var firstErr error
	for _, task := range tasks {
		if task == nil {
			continue
		}
		if err := recordPollFailure(ctx, adaptor, task, task.Status, class, statusCode, detail); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func failTaskFromPoll(ctx context.Context, adaptor TaskPollingAdaptor, task *model.Task, fromStatus model.TaskStatus, reason string) error {
	now := time.Now().Unix()
	task.Status = model.TaskStatusFailure
	task.Progress = taskcommon.ProgressComplete
	if task.FinishTime == 0 {
		task.FinishTime = now
	}
	task.FailReason = reason
	won, err := task.UpdateWithStatus(fromStatus)
	if err != nil {
		return err
	}
	if !won {
		return nil
	}
	finalizeTerminalTask(ctx, adaptor, task, relaycommon.FailTaskInfo(reason))
	return nil
}

func failTasksFromPoll(ctx context.Context, adaptor TaskPollingAdaptor, tasks []*model.Task, reason string) error {
	var firstErr error
	for _, task := range tasks {
		if task == nil {
			continue
		}
		if err := failTaskFromPoll(ctx, adaptor, task, task.Status, reason); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
