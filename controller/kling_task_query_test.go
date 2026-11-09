package controller

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseKlingTaskCursorDefaultsAndRejectsBothBounds(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.UnixMilli(1_781_516_352_968)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/kling/tasks", strings.NewReader(`{"limit":500,"filters":[{"key":"status","values":["succeeded"]},{"key":"product_type","values":["video"]}]}`))
	query, empty, err := parseKlingTaskCursor(c, now)
	require.NoError(t, err)
	assert.False(t, empty)
	assert.Equal(t, 500, query.Limit)
	assert.True(t, query.UseTime)
	assert.Equal(t, []model.TaskStatus{model.TaskStatusSuccess}, query.Statuses)

	c, _ = gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/kling/tasks", strings.NewReader(`{"cursor":"42","start_time":"1","end_time":"2"}`))
	query, empty, err = parseKlingTaskCursor(c, now)
	require.NoError(t, err)
	assert.False(t, empty)
	assert.Equal(t, int64(42), query.CursorID)
	assert.False(t, query.UseTime)

	c, _ = gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/kling/tasks", strings.NewReader(`{"filters":[{"key":"product_type","values":["image"]}]}`))
	_, empty, err = parseKlingTaskCursor(c, now)
	require.NoError(t, err)
	assert.True(t, empty)

	c, _ = gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/kling/tasks", strings.NewReader(`{"start_time":"20","end_time":"10"}`))
	_, _, err = parseKlingTaskCursor(c, now)
	require.EqualError(t, err, "start_time must be earlier than end_time")
}

func TestOrderKlingTasksFollowsRequestedIDs(t *testing.T) {
	tasks := []*model.Task{{TaskID: "b", ExternalTaskID: "eb"}, {TaskID: "a", ExternalTaskID: "ea"}}
	ordered := orderKlingTasks(tasks, []string{"a", "missing", "b"}, nil)
	require.Len(t, ordered, 2)
	assert.Equal(t, "a", ordered[0].TaskID)
	assert.Equal(t, "b", ordered[1].TaskID)
	ordered = orderKlingTasks(tasks, nil, []string{"eb"})
	require.Len(t, ordered, 1)
	assert.Equal(t, "b", ordered[0].TaskID)
}
