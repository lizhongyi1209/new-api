package iliu

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const onePixelPNG = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="

func newNativeTestContext(method, path, body string) *gin.Context {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(method, path, bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c
}

func TestAdaptorGetRequestURLAvoidsDuplicateVersionPrefix(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ChannelBaseUrl: "https://iliu.ai/v1",
	}}
	info.RequestURLPath = "/v1/chat/completions"

	requestURL, err := adaptor.GetRequestURL(info)
	require.NoError(t, err)
	assert.Equal(t, "https://iliu.ai/v1/chat/completions", requestURL)

	info.RequestURLPath = "/v1/images/generations"
	_, err = adaptor.GetRequestURL(info)
	require.ErrorContains(t, err, "only supports /v1/chat/completions")
}

func TestAdaptorDoRequestUsesProviderURLBuilder(t *testing.T) {
	var receivedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(server.Close)

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ChannelBaseUrl: server.URL + "/v1",
		ApiKey:         "test-key",
	}}
	info.RequestURLPath = "/v1/chat/completions"
	c := newNativeTestContext(http.MethodPost, info.RequestURLPath, `{}`)

	result, err := adaptor.DoRequest(c, info, bytes.NewBufferString(`{}`))
	require.NoError(t, err)
	resp, ok := result.(*http.Response)
	require.True(t, ok)
	defer resp.Body.Close()
	_, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "/v1/chat/completions", receivedPath)
}

func TestParseNativeActionPreservesOpaqueCustomID(t *testing.T) {
	c := newNativeTestContext(http.MethodPost, "/v1/mj/submit/action", `{
		"customId":"opaque-value-without-provider-format",
		"taskId":"task-123",
		"chooseSameChannel":false
	}`)

	request, err := ParseNativeRequest(c)
	require.NoError(t, err)
	action, ok := request.Payload.(*ActionRequest)
	require.True(t, ok)
	assert.Equal(t, "opaque-value-without-provider-format", action.CustomID)
	require.NotNil(t, action.ChooseSameChannel)
	assert.False(t, *action.ChooseSameChannel)

	encoded, err := common.Marshal(action)
	require.NoError(t, err)
	assert.Contains(t, string(encoded), `"customId":"opaque-value-without-provider-format"`)
	assert.Contains(t, string(encoded), `"chooseSameChannel":false`)
}

func TestParseNativeRequestValidatesImageAndCallbackBoundaries(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{
			name: "valid image",
			body: `{"prompt":"cat","base64Array":["` + onePixelPNG + `"]}`,
		},
		{
			name:    "non HTTPS callback",
			body:    `{"prompt":"cat","notifyHook":"http://127.0.0.1/callback"}`,
			wantErr: "notifyHook must be a valid HTTPS URL",
		},
		{
			name:    "non image payload",
			body:    `{"prompt":"cat","base64Array":["aGVsbG8="]}`,
			wantErr: "unsupported image MIME type",
		},
		{
			name:    "private callback address",
			body:    `{"prompt":"cat","notifyHook":"https://127.0.0.1/callback"}`,
			wantErr: "private network address is not allowed",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := newNativeTestContext(http.MethodPost, "/v1/mj/submit/imagine", test.body)
			_, err := ParseNativeRequest(c)
			if test.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, test.wantErr)
		})
	}
}

func TestParseSubmitResponseSupportsTaskAndUploadResults(t *testing.T) {
	taskResponse, taskID, err := ParseSubmitResponse([]byte(`{"code":1,"description":"ok","result":"task-123","properties":{}}`))
	require.NoError(t, err)
	assert.Equal(t, 1, taskResponse.Code)
	assert.Equal(t, "task-123", taskID)

	uploadResponse, uploadTaskID, err := ParseSubmitResponse([]byte(`{"code":1,"description":"ok","result":["https://example.com/a.png"],"properties":{}}`))
	require.NoError(t, err)
	assert.Equal(t, 1, uploadResponse.Code)
	assert.Empty(t, uploadTaskID)

	urls, err := ParseUploadResult(uploadResponse.Result)
	require.NoError(t, err)
	assert.Equal(t, []string{"https://example.com/a.png"}, urls)

	_, err = ParseUploadResult(json.RawMessage(`"not-an-array"`))
	require.ErrorContains(t, err, "string array")

	_, err = ParseUploadResult(json.RawMessage(`[]`))
	require.ErrorContains(t, err, "must not be empty")
}

func TestParseNativeVideoRejectsPrivateMediaURLs(t *testing.T) {
	c := newNativeTestContext(http.MethodPost, "/v1/mj/submit/video", `{
		"prompt":"move slowly",
		"image":"http://[::1]/image.png"
	}`)

	_, err := ParseNativeRequest(c)
	require.ErrorContains(t, err, "private network address is not allowed")
}

func TestParseTaskResultMapsTerminalStatuses(t *testing.T) {
	adaptor := &TaskAdaptor{}

	success, err := adaptor.ParseTaskResult([]byte(`{"id":"task-1","status":"SUCCESS","progress":"100%","imageUrl":"https://example.com/a.png"}`))
	require.NoError(t, err)
	assert.Equal(t, string(model.TaskStatusSuccess), success.Status)
	assert.Equal(t, "https://example.com/a.png", success.Url)

	cancelled, err := adaptor.ParseTaskResult([]byte(`{"id":"task-2","status":"CANCEL","progress":"100%"}`))
	require.NoError(t, err)
	assert.Equal(t, string(model.TaskStatusFailure), cancelled.Status)
	assert.Equal(t, "task cancelled", cancelled.Reason)
}
