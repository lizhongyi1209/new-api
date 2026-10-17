package controller

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImageGenerationHistoryKeepsUserScopedURLsAfterTaskDataCleanup(t *testing.T) {
	previousDB, previousLogDB := model.DB, model.LOG_DB
	t.Cleanup(func() { model.DB, model.LOG_DB = previousDB, previousLogDB })
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Task{}))

	newTask := func(id string, userID int, gallery bool) *model.Task {
		task := &model.Task{
			TaskID: id, UserId: userID, Platform: constant.TaskPlatformGenerateImage,
			Status: model.TaskStatusSuccess, FinishTime: 100,
			Properties: model.Properties{OriginModelName: "gemini-nano-banana-2.1"},
			PrivateData: model.TaskPrivateData{
				ImageGallery: gallery,
				ImageGalleryResults: []model.TaskImageGalleryResult{{
					URL:      "https://images.example.com/images/" + id + ".png",
					MimeType: "image/png", OutputFormat: "png",
				}},
			},
		}
		task.SetData(dto.GenerateImageResult{Images: []dto.GenerateImageData{{B64Json: "AAAA"}}})
		require.NoError(t, db.Create(task).Error)
		return task
	}
	owned := newTask("owned", 17, true)
	newTask("other-user", 18, true)
	newTask("public-api", 17, false)

	cleared, err := model.ClearGenerateImageDataWindow(0, 100, 100)
	require.NoError(t, err)
	assert.EqualValues(t, 3, cleared)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Set("id", 17)
	context.Request = httptest.NewRequest(http.MethodGet, "/pg/images/history", nil)
	ImageGenerationHistory(context)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.NotContains(t, recorder.Body.String(), "b64_json")
	var history struct {
		Items []struct {
			TaskID string                         `json:"task_id"`
			Images []model.TaskImageGalleryResult `json:"images"`
		} `json:"items"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &history))
	require.Len(t, history.Items, 1)
	assert.Equal(t, owned.TaskID, history.Items[0].TaskID)
	assert.Equal(t, "https://images.example.com/images/owned.png", history.Items[0].Images[0].URL)

	fetchRecorder := httptest.NewRecorder()
	fetchContext, _ := gin.CreateTestContext(fetchRecorder)
	fetchContext.Set("id", 17)
	fetchContext.Params = gin.Params{{Key: "id", Value: owned.TaskID}}
	fetchContext.Request = httptest.NewRequest(http.MethodGet, "/pg/images/tasks/owned", nil)
	AsyncTaskFetch(fetchContext)

	assert.Equal(t, http.StatusOK, fetchRecorder.Code)
	assert.Contains(t, fetchRecorder.Body.String(), "https://images.example.com/images/owned.png")
	assert.NotContains(t, fetchRecorder.Body.String(), "b64_json")
}

func TestImageGenerationHistoryPagesWithoutRepeatingResults(t *testing.T) {
	previousDB, previousLogDB := model.DB, model.LOG_DB
	t.Cleanup(func() { model.DB, model.LOG_DB = previousDB, previousLogDB })
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Task{}))
	for index := 1; index <= 25; index++ {
		task := &model.Task{
			TaskID: "gallery-" + strconv.Itoa(index),
			UserId: 17, Platform: constant.TaskPlatformGenerateImage,
			Status: model.TaskStatusSuccess,
			PrivateData: model.TaskPrivateData{
				ImageGallery:        true,
				ImageGalleryResults: []model.TaskImageGalleryResult{{URL: "https://images.example.com/" + strconv.Itoa(index) + ".png"}},
			},
		}
		require.NoError(t, db.Create(task).Error)
	}

	getPage := func(path string) struct {
		Items []struct {
			TaskID string `json:"task_id"`
		} `json:"items"`
		NextBeforeID int64 `json:"next_before_id"`
	} {
		recorder := httptest.NewRecorder()
		context, _ := gin.CreateTestContext(recorder)
		context.Set("id", 17)
		context.Request = httptest.NewRequest(http.MethodGet, path, nil)
		ImageGenerationHistory(context)
		require.Equal(t, http.StatusOK, recorder.Code)
		var page struct {
			Items []struct {
				TaskID string `json:"task_id"`
			} `json:"items"`
			NextBeforeID int64 `json:"next_before_id"`
		}
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &page))
		return page
	}

	first := getPage("/pg/images/history")
	require.Len(t, first.Items, 24)
	require.Positive(t, first.NextBeforeID)
	second := getPage("/pg/images/history?before_id=" + strconv.FormatInt(first.NextBeforeID, 10))
	require.Len(t, second.Items, 1)
	assert.Zero(t, second.NextBeforeID)
	assert.NotEqual(t, first.Items[23].TaskID, second.Items[0].TaskID)
}
