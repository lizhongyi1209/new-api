package controller

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAsyncTaskFetchSetsExactContentLengthForBase64Result(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Task{}))

	base64Result := strings.Repeat("A", 64*1024)
	task := &model.Task{
		TaskID:   "task-base64-content-length",
		Platform: constant.TaskPlatformGenerateImage,
		UserId:   17,
		Status:   model.TaskStatusSuccess,
		Progress: "100%",
	}
	task.SetData(dto.GenerateImageResult{
		Model:  "nano-banana-2-2k",
		Images: []dto.GenerateImageData{{B64Json: base64Result, MimeType: "image/png"}},
	})
	require.NoError(t, db.Create(task).Error)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Set("id", task.UserId)
	context.Params = gin.Params{{Key: "id", Value: task.TaskID}}
	context.Request = httptest.NewRequest(http.MethodGet, "/async/v1/tasks/"+task.TaskID, nil)

	AsyncTaskFetch(context)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "application/json; charset=utf-8", recorder.Header().Get("Content-Type"))
	assert.Equal(t, strconv.Itoa(recorder.Body.Len()), recorder.Header().Get("Content-Length"))

	var response dto.AsyncTaskFetchResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, task.TaskID, response.TaskID)
	assert.Equal(t, string(model.TaskStatusSuccess), response.Status)
	assert.Equal(t, task.Data, response.Data)
}
