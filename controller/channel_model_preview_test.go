package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestFetchModelsAdvancedCustomDraftUsesRoutesAndHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/draft/models", r.URL.Path)
		assert.Equal(t, "Bearer draft-key", r.Header.Get("Authorization"))
		assert.Equal(t, "draft-key", r.Header.Get("X-Draft"))
		_, _ = w.Write([]byte(`{"data":[{"id":" custom-model "},{"id":"custom-model"}]}`))
	}))
	t.Cleanup(server.Close)
	config, err := common.Marshal(dto.AdvancedCustomConfig{Routes: []dto.AdvancedCustomRoute{{
		IncomingPath: dto.AdvancedCustomModelListPath,
		UpstreamPath: "/draft/models",
		Converter:    "none",
	}}})
	require.NoError(t, err)
	body, err := common.Marshal(fetchModelsRequest{
		Type: constant.ChannelTypeAdvancedCustom, Key: "draft-key\nsecond-key",
		BaseURL: common.GetPointer(server.URL), AdvancedCustom: common.GetPointer(string(config)),
		HeaderOverride: common.GetPointer(`{"X-Draft":"{api_key}"}`), Proxy: common.GetPointer(""),
	})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/channel/fetch_models", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	FetchModels(ctx)
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.JSONEq(t, `{"success":true,"message":"","data":["custom-model"]}`, recorder.Body.String())
}

func TestFetchModelsSavedDraftNeverWritesOrAdvancesPolling(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/models", r.URL.Path)
		assert.Equal(t, "Bearer enabled-key", r.Header.Get("Authorization"))
		assert.Empty(t, r.Header.Get("X-Saved"))
		_, _ = w.Write([]byte(`{"data":[{"id":"draft-model"}]}`))
	}))
	t.Cleanup(server.Close)
	channel := &model.Channel{
		Name: "saved", Type: constant.ChannelTypeOpenAI, Models: "original-model",
		Key: "disabled-key\nenabled-key", BaseURL: common.GetPointer("http://127.0.0.1:1"),
		HeaderOverride: common.GetPointer(`{"X-Saved":"original-header"}`),
		Setting:        common.GetPointer(`{"proxy":"http://127.0.0.1:1","http_protocol":"http1","custom_future":false}`),
		OtherSettings:  `{"image_output_strategy":"r2","gemini_file_data_enabled":false,"custom_future":0}`,
		ChannelInfo: model.ChannelInfo{IsMultiKey: true, MultiKeyMode: constant.MultiKeyModePolling,
			MultiKeyPollingIndex: 0, MultiKeyStatusList: map[int]int{0: common.ChannelStatusManuallyDisabled, 1: common.ChannelStatusEnabled}},
	}
	require.NoError(t, db.Create(channel).Error)
	before, err := model.GetChannelById(channel.Id, true)
	require.NoError(t, err)
	writes := 0
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("preview_write_check", func(tx *gorm.DB) { writes++ }))
	t.Cleanup(func() { _ = db.Callback().Update().Remove("preview_write_check") })
	req := fetchModelsRequest{ChannelID: channel.Id, Type: channel.Type,
		BaseURL: common.GetPointer(server.URL), HeaderOverride: common.GetPointer(""), Proxy: common.GetPointer("")}
	preview, err := buildModelDiscoveryPreviewChannel(req)
	require.NoError(t, err)
	assert.Zero(t, preview.Id)
	assert.Contains(t, *preview.Setting, `"custom_future":false`)
	assert.Equal(t, channel.OtherSettings, preview.OtherSettings)
	models, err := fetchChannelUpstreamModelIDs(preview)
	require.NoError(t, err)
	assert.Equal(t, []string{"draft-model"}, models)
	assert.Zero(t, writes)
	after, err := model.GetChannelById(channel.Id, true)
	require.NoError(t, err)
	assert.Equal(t, before, after)

	// Explicit replacement credentials belong to the draft, even if old keys
	// were disabled; they do not inherit the saved key status/rotation metadata.
	req.Key = "replacement-key"
	replacement, err := buildModelDiscoveryPreviewChannel(req)
	require.NoError(t, err)
	assert.Equal(t, "replacement-key", replacement.Key)
	assert.False(t, replacement.ChannelInfo.IsMultiKey)
	assert.Zero(t, writes)
}

func TestModelDiscoveryDraftRejectsInvalidSavedSettingsWithoutRepair(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	channel := &model.Channel{Type: constant.ChannelTypeOpenAI, Name: "invalid saved settings", Key: "saved-key",
		Setting: common.GetPointer(`{"proxy":`), OtherSettings: "{}"}
	require.NoError(t, db.Create(channel).Error)
	before, err := model.GetChannelById(channel.Id, true)
	require.NoError(t, err)
	for _, req := range []fetchModelsRequest{
		{ChannelID: channel.Id},
		{ChannelID: channel.Id, Proxy: common.GetPointer("")},
	} {
		_, err := buildModelDiscoveryPreviewChannel(req)
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "saved-key")
	}
	after, err := model.GetChannelById(channel.Id, true)
	require.NoError(t, err)
	assert.Equal(t, before, after)
}

func TestModelDiscoveryDraftRejectsTypeMismatchAndDisabledKeys(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	channel := &model.Channel{Type: constant.ChannelTypeOpenAI, Name: "disabled saved keys", Key: "disabled-key",
		ChannelInfo: model.ChannelInfo{IsMultiKey: true, MultiKeyStatusList: map[int]int{0: common.ChannelStatusManuallyDisabled}}}
	require.NoError(t, db.Create(channel).Error)
	for _, tc := range []struct {
		name string
		req  fetchModelsRequest
	}{
		{"type changed", fetchModelsRequest{ChannelID: channel.Id, Type: constant.ChannelTypeAnthropic}},
		{"all disabled", fetchModelsRequest{ChannelID: channel.Id}},
		{"invalid type", fetchModelsRequest{Type: -1}},
		{"missing type", fetchModelsRequest{}},
		{"invalid header array", fetchModelsRequest{Type: 1, HeaderOverride: common.GetPointer(`[]`)}},
		{"invalid header null", fetchModelsRequest{Type: 1, HeaderOverride: common.GetPointer(`null`)}},
		{"invalid proxy", fetchModelsRequest{Type: 1, Proxy: common.GetPointer("not-a-proxy")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildModelDiscoveryPreviewChannel(tc.req)
			require.Error(t, err)
			assert.NotContains(t, err.Error(), "disabled-key")
		})
	}
}

func TestFetchModelsUnknownTypeWithExplicitURLDoesNotPanic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"id":"custom-future-model"}]}`))
	}))
	t.Cleanup(server.Close)
	channel, err := buildModelDiscoveryPreviewChannel(fetchModelsRequest{Type: 100000, Key: "draft-key", BaseURL: common.GetPointer(server.URL)})
	require.NoError(t, err)
	models, err := fetchChannelUpstreamModelIDs(channel)
	require.NoError(t, err)
	assert.Equal(t, []string{"custom-future-model"}, models)
}
