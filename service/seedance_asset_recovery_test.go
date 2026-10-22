package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type seedanceRecoveryPollingAdaptor struct {
	taskPollingFetchAdaptor
	status    string
	reason    string
	calls     int
	createErr error
	submitErr error
	started   chan struct{}
	release   chan struct{}
}

func (a *seedanceRecoveryPollingAdaptor) FetchTask(string, string, map[string]any, string) (*http.Response, error) {
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
}

func (a *seedanceRecoveryPollingAdaptor) ParseTaskResult([]byte) (*relaycommon.TaskInfo, error) {
	return &relaycommon.TaskInfo{Status: a.status, Reason: a.reason}, nil
}

func (a *seedanceRecoveryPollingAdaptor) MissingSeedanceAsset(_ *model.Task, reason string) bool {
	return reason == "asset-missing not found"
}

func (a *seedanceRecoveryPollingAdaptor) RecreateMissingSeedanceAsset(context.Context, *model.Task, *model.Channel, string) (string, []byte, bool, error) {
	a.calls++
	if a.started != nil {
		close(a.started)
		<-a.release
	}
	if a.createErr != nil {
		return "", nil, false, a.createErr
	}
	if a.submitErr != nil {
		return "", nil, true, a.submitErr
	}
	return fmt.Sprintf("upstream-retry-%d", a.calls), []byte(`{"task":{"status":"queued"}}`), true, nil
}

func seedSeedanceRecoveryTask(t *testing.T) *model.Task {
	t.Helper()
	truncate(t)
	seedUser(t, 1, 900)
	task := makeTask(1, 265, 100, 0, BillingSourceWallet, 0)
	task.TaskID = "task_asset_recovery"
	task.PrivateData.UpstreamTaskID = "upstream-original"
	task.PrivateData.SeedanceAssetRecovery = &model.SeedanceAssetRecovery{
		Request: []byte(`{"content":[{"image_url":{"url":"asset://asset-missing"}}]}`),
		Sources: map[string]model.SeedanceAssetRecreateRequest{
			"asset-missing": {Path: "/v1/sd/assets", Body: []byte(`{"URL":"https://example.com/reference.png"}`)},
		},
	}
	task.PrivateData.SubmitLogID = seedConsumeLog(t, task, task.Quota, map[string]interface{}{"task_status": "SUBMITTED"})
	require.NoError(t, model.DB.Create(task).Error)
	return task
}

func TestSeedanceAssetRecoveryStopsAfterThreeAndRefundsOnce(t *testing.T) {
	task := seedSeedanceRecoveryTask(t)
	adaptor := &seedanceRecoveryPollingAdaptor{status: model.TaskStatusFailure, reason: "asset-missing not found"}
	channel := &model.Channel{Id: 265}
	for attempt := 1; attempt <= 3; attempt++ {
		require.NoError(t, updateVideoSingleTask(context.Background(), adaptor, channel, task))
		assert.Equal(t, model.TaskStatusQueued, string(task.Status))
		assert.Equal(t, "task_asset_recovery", task.TaskID)
		assert.Equal(t, 100, task.Quota)
		assert.Equal(t, attempt, task.PrivateData.SeedanceAssetRecovery.Attempts)
		assert.Equal(t, fmt.Sprintf("upstream-retry-%d", attempt), task.GetUpstreamTaskID())
		var user model.User
		require.NoError(t, model.DB.First(&user, 1).Error)
		assert.Equal(t, 900, user.Quota)
		// Reload between attempts to exercise restart-safe persisted counting.
		require.NoError(t, model.DB.First(task, task.ID).Error)
	}
	require.NoError(t, updateVideoSingleTask(context.Background(), adaptor, channel, task))
	assert.Equal(t, 3, adaptor.calls)
	assert.Equal(t, model.TaskStatusFailure, string(task.Status))
	assert.Zero(t, task.Quota)
	assert.Empty(t, task.PrivateData.SeedanceAssetRecovery.Request)
	assert.Empty(t, task.PrivateData.SeedanceAssetRecovery.Sources)
	require.NoError(t, updateVideoSingleTask(context.Background(), adaptor, channel, task))
	var user model.User
	require.NoError(t, model.DB.First(&user, 1).Error)
	assert.Equal(t, 1000, user.Quota)
	var log model.Log
	require.NoError(t, model.LOG_DB.First(&log, task.PrivateData.SubmitLogID).Error)
	assert.Contains(t, log.Other, `"refund_status":"succeeded"`)
}

func TestSeedanceAssetRecoveryCreationFailureIsBounded(t *testing.T) {
	task := seedSeedanceRecoveryTask(t)
	adaptor := &seedanceRecoveryPollingAdaptor{status: model.TaskStatusFailure, reason: "asset-missing not found", createErr: fmt.Errorf("creation failed")}
	for attempt := 0; attempt < 3; attempt++ {
		require.NoError(t, updateVideoSingleTask(context.Background(), adaptor, &model.Channel{Id: 265}, task))
	}
	assert.Equal(t, 3, adaptor.calls)
	assert.Equal(t, model.TaskStatusFailure, string(task.Status))
	assert.Equal(t, "upstream-original", task.GetUpstreamTaskID())
	assert.Zero(t, task.Quota)
}

func TestSeedanceAssetRecoveryDoesNotRetryOtherFailuresOrAmbiguousSubmission(t *testing.T) {
	for _, test := range []struct {
		name      string
		reason    string
		submitErr error
		wantCalls int
	}{
		{name: "content moderation", reason: "content policy rejected", wantCalls: 0},
		{name: "invalid duration", reason: "duration invalid", wantCalls: 0},
		{name: "ambiguous submit", reason: "asset-missing not found", submitErr: fmt.Errorf("connection lost after submit"), wantCalls: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			task := seedSeedanceRecoveryTask(t)
			adaptor := &seedanceRecoveryPollingAdaptor{status: model.TaskStatusFailure, reason: test.reason, submitErr: test.submitErr}
			require.NoError(t, updateVideoSingleTask(context.Background(), adaptor, &model.Channel{Id: 265}, task))
			assert.Equal(t, test.wantCalls, adaptor.calls)
			assert.Equal(t, model.TaskStatusFailure, string(task.Status))
			assert.Zero(t, task.Quota)
		})
	}
}

func TestSeedanceAssetRecoverySuccessRetainsSingleCharge(t *testing.T) {
	task := seedSeedanceRecoveryTask(t)
	adaptor := &seedanceRecoveryPollingAdaptor{status: model.TaskStatusFailure, reason: "asset-missing not found"}
	channel := &model.Channel{Id: 265}
	require.NoError(t, updateVideoSingleTask(context.Background(), adaptor, channel, task))
	adaptor.status, adaptor.reason = model.TaskStatusSuccess, ""
	require.NoError(t, updateVideoSingleTask(context.Background(), adaptor, channel, task))
	assert.Equal(t, model.TaskStatusSuccess, string(task.Status))
	assert.Equal(t, 1, adaptor.calls)
	assert.Equal(t, 100, task.Quota)
	assert.Empty(t, task.PrivateData.SeedanceAssetRecovery.Request)
	var user model.User
	require.NoError(t, model.DB.First(&user, 1).Error)
	assert.Equal(t, 900, user.Quota)
}

func TestSeedanceAssetRecoveryConcurrentPollCannotResubmit(t *testing.T) {
	task := seedSeedanceRecoveryTask(t)
	var other model.Task
	require.NoError(t, model.DB.First(&other, task.ID).Error)
	adaptor := &seedanceRecoveryPollingAdaptor{started: make(chan struct{}), release: make(chan struct{})}
	result := &relaycommon.TaskInfo{Status: model.TaskStatusFailure, Reason: "asset-missing not found"}
	done := make(chan error, 1)
	go func() {
		_, err := recoverMissingSeedanceAsset(context.Background(), adaptor, &model.Channel{Id: 265}, task, result)
		done <- err
	}()
	<-adaptor.started
	handled, err := recoverMissingSeedanceAsset(context.Background(), adaptor, &model.Channel{Id: 265}, &other, result)
	assert.NoError(t, err)
	assert.True(t, handled)
	close(adaptor.release)
	require.NoError(t, <-done)
	assert.Equal(t, 1, adaptor.calls)
	assert.Equal(t, model.TaskStatusQueued, string(task.Status))
	// The old terminal response must not overwrite the new task or refund it.
	require.NoError(t, ApplyVideoTaskPollingResult(context.Background(), adaptor, &other, result, []byte(`{}`)))
	var stored model.Task
	require.NoError(t, model.DB.First(&stored, task.ID).Error)
	assert.Equal(t, model.TaskStatusQueued, string(stored.Status))
	assert.Equal(t, 100, stored.Quota)
}

func TestSeedanceAssetRecoveryInterruptedWorkerDoesNotDuplicateGeneration(t *testing.T) {
	task := seedSeedanceRecoveryTask(t)
	task.PrivateData.SeedanceAssetRecovery.StartedAt = time.Now().Add(-6 * time.Minute).Unix()
	require.NoError(t, task.UpdatePrivateData())
	adaptor := &seedanceRecoveryPollingAdaptor{status: model.TaskStatusFailure, reason: "asset-missing not found"}
	require.NoError(t, updateVideoSingleTask(context.Background(), adaptor, &model.Channel{Id: 265}, task))
	assert.Zero(t, adaptor.calls)
	assert.Equal(t, model.TaskStatusFailure, string(task.Status))
	assert.Zero(t, task.Quota)
}
