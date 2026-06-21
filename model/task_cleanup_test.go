package model

import (
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClearGenerateImageDataWindow protects the retention contract: only
// terminal generate_image tasks whose finish_time is inside the (since, cutoff]
// window get their `data` blanked. Recent tasks, unfinished tasks, other
// platforms, and tasks outside the window must be left untouched, and
// billing-relevant columns (quota/status) must survive the blanking.
func TestClearGenerateImageDataWindow(t *testing.T) {
	truncateTables(t)

	const payload = `{"images":[{"b64_json":"AAAA"}]}`
	now := time.Now().Unix()
	oldFinish := now - 48*3600  // past a 24h cutoff
	freshFinish := now - 1*3600 // within a 24h cutoff (newer than cutoff)
	cutoff := now - 24*3600
	since := int64(0)

	mk := func(taskID string, platform constant.TaskPlatform, status TaskStatus, finish int64, data string) *Task {
		task := &Task{
			TaskID:     taskID,
			Platform:   platform,
			Status:     status,
			Quota:      500,
			FinishTime: finish,
			Data:       json.RawMessage(data),
		}
		insertTask(t, task)
		return task
	}

	expiredSuccess := mk("img_old_success", constant.TaskPlatformGenerateImage, TaskStatusSuccess, oldFinish, payload)
	expiredFailure := mk("img_old_failure", constant.TaskPlatformGenerateImage, TaskStatusFailure, oldFinish, payload)
	freshSuccess := mk("img_fresh_success", constant.TaskPlatformGenerateImage, TaskStatusSuccess, freshFinish, payload)
	inProgress := mk("img_in_progress", constant.TaskPlatformGenerateImage, TaskStatusInProgress, 0, payload)
	otherPlatform := mk("video_old_success", constant.TaskPlatformSuno, TaskStatusSuccess, oldFinish, payload)

	cleared, err := ClearGenerateImageDataWindow(since, cutoff, 100)
	require.NoError(t, err)
	// Only the two expired terminal generate_image rows inside the window.
	assert.EqualValues(t, 2, cleared)

	dataOf := func(id int64) string {
		var reloaded Task
		require.NoError(t, DB.First(&reloaded, id).Error)
		return string(reloaded.Data)
	}

	// Blanked.
	assert.JSONEq(t, "{}", dataOf(expiredSuccess.ID))
	assert.JSONEq(t, "{}", dataOf(expiredFailure.ID))
	// Billing/audit columns survive on a blanked row.
	var reloaded Task
	require.NoError(t, DB.First(&reloaded, expiredSuccess.ID).Error)
	assert.EqualValues(t, 500, reloaded.Quota)
	assert.EqualValues(t, TaskStatusSuccess, reloaded.Status)

	// Untouched.
	assert.JSONEq(t, payload, dataOf(freshSuccess.ID), "task newer than cutoff must be kept")
	assert.JSONEq(t, payload, dataOf(inProgress.ID), "unfinished task must be kept")
	assert.JSONEq(t, payload, dataOf(otherPlatform.ID), "non generate_image platform must be kept")

	// Advancing the watermark to cutoff yields an empty window: nothing re-cleared.
	cleared, err = ClearGenerateImageDataWindow(cutoff, cutoff, 100)
	require.NoError(t, err)
	assert.EqualValues(t, 0, cleared)
}

// TestClearGenerateImageDataWindow_LowerBoundExclusive verifies the window is
// half-open (since, cutoff]: a task finished exactly at `since` (already covered
// by a prior run) is not re-blanked, while one finished exactly at `cutoff` is.
func TestClearGenerateImageDataWindow_LowerBoundExclusive(t *testing.T) {
	truncateTables(t)

	const payload = `{"images":[{"b64_json":"AAAA"}]}`
	since := int64(1_000_000)
	cutoff := int64(2_000_000)

	atSince := &Task{TaskID: "img_at_since", Platform: constant.TaskPlatformGenerateImage, Status: TaskStatusSuccess, FinishTime: since, Data: json.RawMessage(payload)}
	atCutoff := &Task{TaskID: "img_at_cutoff", Platform: constant.TaskPlatformGenerateImage, Status: TaskStatusSuccess, FinishTime: cutoff, Data: json.RawMessage(payload)}
	insertTask(t, atSince)
	insertTask(t, atCutoff)

	cleared, err := ClearGenerateImageDataWindow(since, cutoff, 100)
	require.NoError(t, err)
	assert.EqualValues(t, 1, cleared)

	var reloadedSince, reloadedCutoff Task
	require.NoError(t, DB.First(&reloadedSince, atSince.ID).Error)
	require.NoError(t, DB.First(&reloadedCutoff, atCutoff.ID).Error)
	assert.JSONEq(t, payload, string(reloadedSince.Data), "finish_time == since is excluded (prior window)")
	assert.JSONEq(t, "{}", string(reloadedCutoff.Data), "finish_time == cutoff is included")
}

// TestClearGenerateImageDataWindow_Batching verifies the id-cursor pagination
// clears every matching row when the batch size is smaller than the match set.
func TestClearGenerateImageDataWindow_Batching(t *testing.T) {
	truncateTables(t)

	const payload = `{"images":[{"b64_json":"AAAA"}]}`
	const n = 7
	finish := int64(1_500_000)
	for i := 0; i < n; i++ {
		insertTask(t, &Task{
			TaskID:     "img_batch_" + strconv.Itoa(i),
			Platform:   constant.TaskPlatformGenerateImage,
			Status:     TaskStatusSuccess,
			FinishTime: finish,
			Data:       json.RawMessage(payload),
		})
	}

	cleared, err := ClearGenerateImageDataWindow(0, 2_000_000, 2)
	require.NoError(t, err)
	assert.EqualValues(t, n, cleared, "all rows cleared across multiple batches")

	var remaining int64
	require.NoError(t, DB.Model(&Task{}).
		Where("platform = ?", constant.TaskPlatformGenerateImage).
		Where("data = ?", json.RawMessage(payload)).
		Count(&remaining).Error)
	assert.EqualValues(t, 0, remaining)
}
