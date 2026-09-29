package service

import (
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGPTImageEdit400CaptureRecordsSafeUpstreamParameters(t *testing.T) {
	t.Setenv(imageEdit400CaptureUntilEnv, time.Now().UTC().Add(time.Hour).Format(time.RFC3339))
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousRedisEnabled := common.RedisEnabled
	previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	previousErrorLogEnabled := constant.ErrorLogEnabled
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := database.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, database.AutoMigrate(&model.User{}, &model.Log{}))
	model.DB, model.LOG_DB = database, database
	common.RedisEnabled = false
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	constant.ErrorLogEnabled = true
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.RedisEnabled = previousRedisEnabled
		common.SetDatabaseTypes(previousMainType, previousLogType)
		constant.ErrorLogEnabled = previousErrorLogEnabled
		require.NoError(t, sqlDB.Close())
	})
	require.NoError(t, database.Create(&model.User{Id: 7, Username: "image-user", Group: "default"}).Error)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", nil)
	c.Request.MultipartForm = &multipart.Form{Value: map[string][]string{"response_format": {"b64_json"}}}
	c.Set("id", 7)
	c.Set("username", "image-user")
	c.Set("original_model", "gpt-image-2")
	c.Set("group", "default")
	c.Set(common.RequestIdKey, "image-400-capture")
	snapshot := relaycommon.NewUpstreamMultipartRequestSnapshot(http.MethodPost, "/v1/images/edits")
	snapshot.ContentLength = 1024
	snapshot.AddField("model", "gpt-image-2")
	snapshot.AddField("quality", "high")
	snapshot.AddField("prompt", "private drawing instructions")
	snapshot.AddField("custom", "private metadata")
	snapshot.AddField("api_key", "sk-private")
	snapshot.AddFile("image", "private-filename.png", "image/png", 300, strings.Repeat("a", 64))
	relaycommon.SetUpstreamRequestSnapshot(c, snapshot)
	relaycommon.SetUpstreamResponseSnapshot(c, http.StatusBadRequest, "application/json", []byte(`{"error":{"message":"invalid request"}}`))
	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeImagesEdits,
		OriginModelName: "gpt-image-2",
		ChannelMeta:     &relaycommon.ChannelMeta{UpstreamModelName: "gpt-image-2"},
	}
	apiErr := types.NewOpenAIError(errors.New("invalid request"), types.ErrorCodeBadResponseStatusCode, http.StatusBadRequest)

	ProcessChannelError(c, types.ChannelError{ChannelId: 282}, apiErr, info)

	var stored model.Log
	require.NoError(t, database.First(&stored).Error)
	assert.Equal(t, "image-400-capture", stored.RequestId)
	var other map[string]any
	require.NoError(t, common.Unmarshal([]byte(stored.Other), &other))
	adminInfo, ok := other["admin_info"].(map[string]any)
	require.True(t, ok)
	capture, ok := adminInfo["gpt_image_edit_400_capture"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, true, capture["client_response_format_present"])
	assert.Equal(t, "b64_json", capture["client_response_format"])
	assert.Equal(t, false, capture["outbound_response_format_found"])
	assert.Equal(t, float64(1024), capture["content_length"])
	encoded, err := common.Marshal(capture)
	require.NoError(t, err)
	assert.Contains(t, string(encoded), `"quality"`)
	assert.Contains(t, string(encoded), `"high"`)
	assert.Contains(t, string(encoded), `"image/png"`)
	for _, secret := range []string{"private drawing instructions", "private metadata", "sk-private", "private-filename.png"} {
		assert.NotContains(t, string(encoded), secret)
	}

	logs, total, err := model.GetUserLogs(7, model.LogTypeError, 0, 0, "", "", 0, 10, "", "", "")
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, logs, 1)
	assert.NotContains(t, logs[0].Other, "gpt_image_edit_400_capture")
}

func TestGPTImageEdit400CaptureRespectsScopeAndDeadline(t *testing.T) {
	t.Setenv(imageEdit400CaptureUntilEnv, time.Now().UTC().Add(time.Hour).Format(time.RFC3339))
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	relaycommon.SetUpstreamRequestSnapshot(c, relaycommon.NewUpstreamMultipartRequestSnapshot(http.MethodPost, "/v1/images/edits"))
	relaycommon.SetUpstreamResponseSnapshot(c, http.StatusInternalServerError, "application/json", []byte(`{}`))
	info := &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeImagesEdits, OriginModelName: "gpt-image-2"}
	other := model.NewLogOther()

	appendGPTImageEdit400Capture(c, info, other)
	assert.NotContains(t, other.JSONString(), "gpt_image_edit_400_capture")

	relaycommon.SetUpstreamResponseSnapshot(c, http.StatusBadRequest, "application/json", []byte(`{}`))
	info.OriginModelName = "dall-e-3"
	appendGPTImageEdit400Capture(c, info, other)
	assert.NotContains(t, other.JSONString(), "gpt_image_edit_400_capture")

	info.OriginModelName = "gpt-image-2"
	t.Setenv(imageEdit400CaptureUntilEnv, time.Now().UTC().Add(-time.Hour).Format(time.RFC3339))
	appendGPTImageEdit400Capture(c, info, other)
	assert.NotContains(t, other.JSONString(), "gpt_image_edit_400_capture")
}
