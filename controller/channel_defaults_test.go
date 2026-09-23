package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannelDefaultBaseURLsExposeCurrentProviderDefaultsOnly(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/channel/default_base_urls", nil)
	GetChannelDefaultBaseURLs(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool           `json:"success"`
		Data    map[int]string `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	assert.Equal(t, constant.ChannelBaseURLs[constant.ChannelTypeDeepSeek], response.Data[constant.ChannelTypeDeepSeek])
	assert.Equal(t, "https://vclm.tencentcloudapi.com", response.Data[constant.ChannelTypeTencentVideo])
	assert.Equal(t, "https://model.service-inference.ai", response.Data[constant.ChannelTypeServiceInferenceVideo])
	assert.NotContains(t, response.Data, constant.ChannelTypeAdvancedCustom)
	assert.NotContains(t, response.Data, constant.ChannelTypeSub2API)
	assert.NotContains(t, response.Data, constant.ChannelTypeNewAPI)
	for _, address := range response.Data {
		assert.NotEmpty(t, address)
	}
}
