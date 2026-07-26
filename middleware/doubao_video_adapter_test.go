package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDoubaoVideoRequestConvertPreservesMini720pTextRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	request := httptest.NewRequest(http.MethodPost, "/api/v3/contents/generations/tasks", strings.NewReader(`{
		"model":"seedance-2.0-mini",
		"content":[{"type":"text","text":"paper boat at sunrise"}],
		"resolution":"720p",
		"ratio":"16:9",
		"duration":5
	}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = request

	DoubaoVideoRequestConvert()(context)

	body, err := io.ReadAll(context.Request.Body)
	require.NoError(t, err)
	var converted map[string]any
	require.NoError(t, common.Unmarshal(body, &converted))
	assert.Equal(t, "/v1/video/generations", context.Request.URL.Path)
	assert.Equal(t, "seedance-2.0-mini", converted["model"])
	assert.Equal(t, "paper boat at sunrise", converted["prompt"])
	assert.Equal(t, "720p", converted["resolution"])
	assert.Equal(t, "16:9", converted["ratio"])
	assert.EqualValues(t, 5, converted["duration"])
}

func TestDoubaoVideoRequestConvertPreservesMediaRoles(t *testing.T) {
	gin.SetMode(gin.TestMode)
	request := httptest.NewRequest(http.MethodPost, "/api/v3/contents/generations/tasks", strings.NewReader(`{
		"model":"seedance-2.0",
		"content":[
			{"type":"text","text":"animate references"},
			{"type":"image_url","image_url":{"url":"https://example.com/first.png"},"role":"first_frame"},
			{"type":"video_url","video_url":{"url":"https://example.com/ref.mp4"}},
			{"type":"audio_url","audio_url":{"url":"https://example.com/ref.mp3"}}
		]
	}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = request

	DoubaoVideoRequestConvert()(context)

	body, err := io.ReadAll(context.Request.Body)
	require.NoError(t, err)
	var converted struct {
		Images []map[string]any `json:"images"`
		Videos []string         `json:"videos"`
		Audios []string         `json:"audios"`
	}
	require.NoError(t, common.Unmarshal(body, &converted))
	require.Len(t, converted.Images, 1)
	assert.Equal(t, "https://example.com/first.png", converted.Images[0]["url"])
	assert.Equal(t, "first_frame", converted.Images[0]["role"])
	assert.Equal(t, []string{"https://example.com/ref.mp4"}, converted.Videos)
	assert.Equal(t, []string{"https://example.com/ref.mp3"}, converted.Audios)
}
