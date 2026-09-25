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

// TestClearAsyncImageDataWindow protects the async_image retention contract:
// only terminal async_image generateContent tasks whose finish_time is inside
// the (since, cutoff] window get their `data` blanked. Other platforms, other
// actions, unfinished tasks, and tasks outside the window stay untouched, and
// billing-relevant columns survive the blanking.
func TestClearAsyncImageDataWindow(t *testing.T) {
	truncateTables(t)

	const payload = `{"candidates":[{"content":{"parts":[{"inlineData":{"mimeType":"image/png","data":"iVBORw0KGgo="}}]}}]}`
	now := time.Now().Unix()
	oldFinish := now - 48*3600  // past a 24h cutoff
	freshFinish := now - 1*3600 // within a 24h cutoff (newer than cutoff)
	cutoff := now - 24*3600
	since := int64(0)

	mk := func(taskID string, platform constant.TaskPlatform, action string, status TaskStatus, finish int64, data string, resultURL string) *Task {
		task := &Task{
			TaskID:     taskID,
			Platform:   platform,
			Action:     action,
			Status:     status,
			Quota:      500,
			FinishTime: finish,
			Data:       json.RawMessage(data),
			PrivateData: TaskPrivateData{
				ResultURL: resultURL,
			},
		}
		insertTask(t, task)
		return task
	}

	expiredSuccess := mk("async_old_success", constant.TaskPlatformAsyncImage, "generateContent", TaskStatusSuccess, oldFinish, payload, "https://cdn.example.com/a.png")
	expiredFailure := mk("async_old_failure", constant.TaskPlatformAsyncImage, "generateContent", TaskStatusFailure, oldFinish, payload, "https://cdn.example.com/b.png")
	freshSuccess := mk("async_fresh_success", constant.TaskPlatformAsyncImage, "generateContent", TaskStatusSuccess, freshFinish, payload, "https://cdn.example.com/c.png")
	inProgress := mk("async_in_progress", constant.TaskPlatformAsyncImage, "generateContent", TaskStatusInProgress, 0, payload, "https://cdn.example.com/d.png")
	otherPlatform := mk("async_video_old", constant.TaskPlatformSuno, "generateContent", TaskStatusSuccess, oldFinish, payload, "https://cdn.example.com/e.png")
	otherAction := mk("async_edit_old", constant.TaskPlatformAsyncImage, "editImage", TaskStatusSuccess, oldFinish, payload, "https://cdn.example.com/f.png")

	cleared, err := ClearAsyncImageDataWindow(since, cutoff, 100)
	require.NoError(t, err)
	// Only the two expired terminal generateContent async_image rows in window.
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
	assert.JSONEq(t, payload, dataOf(otherPlatform.ID), "non async_image platform must be kept")
	assert.JSONEq(t, payload, dataOf(otherAction.ID), "non generateContent action must be kept")

	// Advancing the watermark to cutoff yields an empty window: nothing re-cleared.
	cleared, err = ClearAsyncImageDataWindow(cutoff, cutoff, 100)
	require.NoError(t, err)
	assert.EqualValues(t, 0, cleared)
}

// TestClearAsyncImageDataWindow_KeepsRowsWithoutResultURL protects the data-loss
// boundary: an async_image row whose storage upload failed carries the only copy
// of the generated image inside `data`, so it must never be blanked even though
// it is a terminal async_image generateContent task inside the window.
func TestClearAsyncImageDataWindow_KeepsRowsWithoutResultURL(t *testing.T) {
	truncateTables(t)

	const payload = `{"candidates":[{"content":{"parts":[{"inlineData":{"mimeType":"image/png","data":"iVBORw0KGgo="}}]}}]}`
	finish := int64(1_500_000)

	withResultURL := &Task{
		TaskID:      "async_with_url",
		Platform:    constant.TaskPlatformAsyncImage,
		Action:      "generateContent",
		Status:      TaskStatusSuccess,
		FinishTime:  finish,
		Data:        json.RawMessage(payload),
		PrivateData: TaskPrivateData{ResultURL: "https://cdn.example.com/kept.png"},
	}
	withoutResultURL := &Task{
		TaskID:     "async_without_url",
		Platform:   constant.TaskPlatformAsyncImage,
		Action:     "generateContent",
		Status:     TaskStatusSuccess,
		FinishTime: finish,
		Data:       json.RawMessage(payload),
	}
	insertTask(t, withResultURL)
	insertTask(t, withoutResultURL)

	cleared, err := ClearAsyncImageDataWindow(0, 2_000_000, 100)
	require.NoError(t, err)
	assert.EqualValues(t, 1, cleared)

	var reloadedWith, reloadedWithout Task
	require.NoError(t, DB.First(&reloadedWith, withResultURL.ID).Error)
	require.NoError(t, DB.First(&reloadedWithout, withoutResultURL.ID).Error)
	assert.JSONEq(t, "{}", string(reloadedWith.Data), "row with a durable result URL is blanked")
	assert.JSONEq(t, payload, string(reloadedWithout.Data),
		"row without a result URL holds the only image copy and must be kept")
}

// TestClearAsyncImageDataWindow_Batching verifies the id-cursor pagination clears
// every matching row when the batch size is smaller than the match set.
func TestClearAsyncImageDataWindow_Batching(t *testing.T) {
	truncateTables(t)

	const payload = `{"candidates":[{"content":{"parts":[{"inlineData":{"data":"iVBORw0KGgo="}}]}}]}`
	const n = 7
	finish := int64(1_500_000)
	for i := 0; i < n; i++ {
		insertTask(t, &Task{
			TaskID:      "async_batch_" + strconv.Itoa(i),
			Platform:    constant.TaskPlatformAsyncImage,
			Action:      "generateContent",
			Status:      TaskStatusSuccess,
			FinishTime:  finish,
			Data:        json.RawMessage(payload),
			PrivateData: TaskPrivateData{ResultURL: "https://cdn.example.com/batch.png"},
		})
	}

	cleared, err := ClearAsyncImageDataWindow(0, 2_000_000, 2)
	require.NoError(t, err)
	assert.EqualValues(t, n, cleared, "all rows cleared across multiple batches")

	var remaining int64
	require.NoError(t, DB.Model(&Task{}).
		Where("platform = ?", constant.TaskPlatformAsyncImage).
		Where("data = ?", json.RawMessage(payload)).
		Count(&remaining).Error)
	assert.EqualValues(t, 0, remaining)
}
