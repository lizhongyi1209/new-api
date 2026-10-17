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
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
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

func TestImageGenerationDownloadReturnsAttachmentOnlyForOwnedGalleryTask(t *testing.T) {
	previousDB, previousLogDB := model.DB, model.LOG_DB
	t.Cleanup(func() { model.DB, model.LOG_DB = previousDB, previousLogDB })
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Task{}))

	fetchSetting := system_setting.GetFetchSetting()
	previousFetchSetting := *fetchSetting
	previousWorkerURL := system_setting.WorkerUrl
	t.Cleanup(func() {
		*fetchSetting = previousFetchSetting
		system_setting.WorkerUrl = previousWorkerURL
	})
	fetchSetting.EnableSSRFProtection = false
	system_setting.WorkerUrl = ""
	service.InitHttpClient()

	imageBytes := []byte("saved-image-bytes")
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(imageBytes)
	}))
	t.Cleanup(imageServer.Close)

	newTask := func(taskID string, userID int, gallery bool) *model.Task {
		task := &model.Task{
			TaskID: taskID, UserId: userID, Platform: constant.TaskPlatformGenerateImage,
			Status: model.TaskStatusSuccess,
			PrivateData: model.TaskPrivateData{
				ImageGallery: gallery,
				ImageGalleryResults: []model.TaskImageGalleryResult{{
					URL: imageServer.URL + "/saved.png", MimeType: "image/png", OutputFormat: "png",
				}},
			},
		}
		require.NoError(t, db.Create(task).Error)
		return task
	}
	owned := newTask("owned-download", 17, true)
	newTask("other-download", 18, true)
	newTask("not-gallery", 17, false)

	tests := []struct {
		name   string
		taskID string
		index  string
		status int
	}{
		{name: "owned image", taskID: owned.TaskID, index: "0", status: http.StatusOK},
		{name: "other user", taskID: "other-download", index: "0", status: http.StatusNotFound},
		{name: "non-gallery task", taskID: "not-gallery", index: "0", status: http.StatusNotFound},
		{name: "missing image", taskID: owned.TaskID, index: "1", status: http.StatusNotFound},
		{name: "invalid index", taskID: owned.TaskID, index: "-1", status: http.StatusBadRequest},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Set("id", 17)
			context.Params = gin.Params{{Key: "id", Value: test.taskID}, {Key: "index", Value: test.index}}
			context.Request = httptest.NewRequest(http.MethodGet, "/pg/images/tasks/"+test.taskID+"/images/"+test.index+"/download", nil)
			ImageGenerationDownload(context)

			assert.Equal(t, test.status, recorder.Code)
			if test.status != http.StatusOK {
				return
			}
			assert.Equal(t, imageBytes, recorder.Body.Bytes())
			assert.Equal(t, "image/png", recorder.Header().Get("Content-Type"))
			assert.Equal(t, "attachment; filename=generated-image-"+strconv.FormatInt(owned.ID, 10)+"-1.png", recorder.Header().Get("Content-Disposition"))
			assert.Equal(t, "nosniff", recorder.Header().Get("X-Content-Type-Options"))
		})
	}
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
