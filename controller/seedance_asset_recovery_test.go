package controller

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/task/serviceinference"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedanceAssetRecoveryThroughProxyAndBackgroundPolling(t *testing.T) {
	db, dialect := openTaskDialectDatabase(t, &model.Channel{}, &model.Task{}, &model.SeedanceAssetSource{}, &model.User{}, &model.Log{})
	previousDB, previousLog := model.DB, model.LOG_DB
	previousMain, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	previousMemory, previousRedis, previousBatch := common.MemoryCacheEnabled, common.RedisEnabled, common.BatchUpdateEnabled
	previousFactory := service.GetTaskAdaptorFunc
	model.DB, model.LOG_DB = db, db
	common.SetDatabaseTypes(dialect, dialect)
	common.MemoryCacheEnabled, common.RedisEnabled, common.BatchUpdateEnabled = false, false, false
	service.GetTaskAdaptorFunc = func(constant.TaskPlatform) service.TaskPollingAdaptor { return &serviceinference.TaskAdaptor{} }
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLog
		common.SetDatabaseTypes(previousMain, previousLogType)
		common.MemoryCacheEnabled, common.RedisEnabled, common.BatchUpdateEnabled = previousMemory, previousRedis, previousBatch
		service.GetTaskAdaptorFunc = previousFactory
	})

	const sourceURL = "https://cdn.example.com/reference.png?signature=private"
	const missingReason = "The parameter content[1].image_url.url is not valid: The specified asset asset-old is not found"
	creations, generations, readinessChecks := 0, 0, 0
	var originalRequest []byte
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer fixture-key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "POST /v1/sd/assets":
			creations++
			body, err := io.ReadAll(r.Body)
			assert.NoError(t, err)
			assert.JSONEq(t, `{"URL":"`+sourceURL+`","AssetType":"Image","Name":"portrait"}`, string(body))
			if creations == 1 {
				_, _ = io.WriteString(w, `{"success":true,"data":{"Id":"asset-old","Status":"Active"}}`)
			} else {
				_, _ = io.WriteString(w, `{"success":true,"data":{"Id":"asset-new","Status":"Pending"}}`)
			}
		case "GET /v1/sd/assets/asset-new":
			readinessChecks++
			_, _ = io.WriteString(w, `{"success":true,"data":{"Id":"asset-new","Status":"Active"}}`)
		case "GET /v2/video/tasks/upstream-original":
			data, err := common.Marshal(map[string]any{"task": map[string]any{"id": "upstream-original", "status": "failed", "error": missingReason}})
			assert.NoError(t, err)
			_, _ = w.Write(data)
		case "POST /v2/video/generate":
			generations++
			assert.Equal(t, 1, readinessChecks, "must wait for the recreated asset to become active")
			body, err := io.ReadAll(r.Body)
			assert.NoError(t, err)
			assert.JSONEq(t, strings.ReplaceAll(string(originalRequest), "asset://asset-old", "asset://asset-new"), string(body))
			_, _ = io.WriteString(w, `{"task":{"id":"upstream-recovered","status":"queued"}}`)
		case "GET /v2/video/tasks/upstream-recovered":
			_, _ = io.WriteString(w, `{"task":{"id":"upstream-recovered","status":"completed","outputs":["https://cdn.example.com/result.mp4"]}}`)
		default:
			t.Errorf("unexpected upstream operation: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer upstream.Close()
	channel := &model.Channel{Id: 265, Name: "seedance", Type: 60, Status: 1, Key: "fixture-key", BaseURL: &upstream.URL}
	require.NoError(t, db.Create(channel).Error)
	require.NoError(t, db.Create(&model.User{Id: 7, Username: "asset-owner", Quota: 9000}).Error)
	createResponse := httptest.NewRecorder()
	createContext, _ := gin.CreateTestContext(createResponse)
	createContext.Request = httptest.NewRequest(http.MethodPost, "/v1/seedance/assets", strings.NewReader(`{"type":"hc","url":"`+sourceURL+`","name":"portrait"}`))
	createContext.Set("id", 7)
	CreateUnifiedSeedanceAsset(createContext)
	require.Equal(t, http.StatusOK, createResponse.Code)
	assert.JSONEq(t, `{"success":true,"data":{"Id":"asset-old","Status":"Active"}}`, createResponse.Body.String())
	sources, err := model.FindSeedanceAssetSources(context.Background(), 8, []string{"asset-old"})
	require.NoError(t, err)
	assert.Empty(t, sources, "another user must not recover this source")

	request := `{"model":"dreamina-seedance-2-0-hc","content":[{"type":"text","text":"animate"},{"type":"image_url","image_url":{"url":"asset://asset-old"},"role":"reference_image"}],"duration":5,"resolution":"480p","seed":0,"watermark":false,"generate_audio":false}`
	submitContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	submitContext.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", strings.NewReader(request))
	submitContext.Request.Header.Set("Content-Type", "application/json")
	info := &relaycommon.RelayInfo{UserId: 7, OriginModelName: "dreamina-seedance-2-0-hc", ChannelMeta: &relaycommon.ChannelMeta{
		ChannelId: 265, ChannelType: 60, ChannelBaseUrl: upstream.URL, ApiKey: "fixture-key", IsModelMapped: true, UpstreamModelName: "dreamina-seedance-2-0-260128-max",
	}}
	adaptor := &serviceinference.TaskAdaptor{}
	adaptor.Init(info)
	require.Nil(t, adaptor.ValidateRequestAndSetAction(submitContext, info))
	reader, err := adaptor.BuildRequestBody(submitContext, info)
	require.NoError(t, err)
	originalRequest, err = io.ReadAll(reader)
	require.NoError(t, err)
	recovery, ok := submitContext.MustGet(model.SeedanceAssetRecoveryContextKey).(*model.SeedanceAssetRecovery)
	require.True(t, ok)
	require.NotNil(t, recovery)

	task := &model.Task{TaskID: "task_public_stays_same", Platform: "60", UserId: 7, ChannelId: 265,
		Status: model.TaskStatusSubmitted, Quota: 1000, Properties: model.Properties{OriginModelName: info.OriginModelName, UpstreamModelName: info.UpstreamModelName},
		PrivateData: model.TaskPrivateData{UpstreamTaskID: "upstream-original", BillingSource: service.BillingSourceWallet, SeedanceAssetRecovery: recovery},
	}
	log := &model.Log{UserId: 7, Type: model.LogTypeConsume, Quota: 1000, Other: `{}`}
	require.NoError(t, db.Create(log).Error)
	task.PrivateData.SubmitLogID = log.Id
	require.NoError(t, db.Create(task).Error)
	var stale model.Task
	require.NoError(t, db.First(&stale, task.ID).Error)
	require.NoError(t, service.UpdateVideoTasks(context.Background(), "60", map[int][]string{265: {"upstream-original"}}, map[string]*model.Task{"upstream-original": task}))
	assert.Equal(t, "task_public_stays_same", task.TaskID)
	assert.Equal(t, "upstream-recovered", task.GetUpstreamTaskID())
	assert.Equal(t, model.TaskStatusQueued, string(task.Status))
	assert.Equal(t, 1000, task.Quota)
	assert.Equal(t, 2, creations)
	assert.Equal(t, 1, generations)
	require.NoError(t, service.ApplyVideoTaskPollingResult(context.Background(), adaptor, &stale, &relaycommon.TaskInfo{Status: model.TaskStatusFailure, Reason: missingReason}, []byte(`{}`)))
	require.NoError(t, db.First(task, task.ID).Error)
	assert.Equal(t, model.TaskStatusQueued, string(task.Status))
	require.NoError(t, service.UpdateVideoTasks(context.Background(), "60", map[int][]string{265: {"upstream-recovered"}}, map[string]*model.Task{"upstream-recovered": task}))
	assert.Equal(t, model.TaskStatusSuccess, string(task.Status))
	assert.Empty(t, task.PrivateData.SeedanceAssetRecovery.Request)
	assert.Empty(t, task.PrivateData.SeedanceAssetRecovery.Sources)
	public, err := adaptor.ConvertToOpenAIVideo(task)
	require.NoError(t, err)
	assert.Contains(t, string(public), "task_public_stays_same")
	assert.NotContains(t, string(public), sourceURL)
	assert.NotContains(t, string(public), "fixture-key")
	var user model.User
	require.NoError(t, db.First(&user, 7).Error)
	assert.Equal(t, 9000, user.Quota)
	require.NoError(t, db.First(log, log.Id).Error)
	assert.Equal(t, 1000, log.Quota)
	assert.Contains(t, log.Other, `"task_status":"SUCCESS"`)
}
