package model

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVideoTaskPollingSelectionExcludesImageTasks(t *testing.T) {
	truncateTables(t)

	for _, platform := range []constant.TaskPlatform{
		constant.TaskPlatformAsyncImage,
		constant.TaskPlatformUnifiedImage,
		constant.TaskPlatformGenerateImage,
	} {
		insertTask(t, &Task{TaskID: "image_" + string(platform), Platform: platform, Status: TaskStatusInProgress, Progress: "0%"})
	}
	require.False(t, HasUnfinishedSyncTasks())
	assert.Empty(t, GetAllUnFinishSyncTasks(100))

	video := &Task{TaskID: "video_pending", Platform: constant.TaskPlatformSuno, Status: TaskStatusInProgress, Progress: "0%"}
	insertTask(t, video)
	require.True(t, HasUnfinishedSyncTasks())
	tasks := GetAllUnFinishSyncTasks(100)
	require.Len(t, tasks, 1)
	assert.Equal(t, video.ID, tasks[0].ID)
}
