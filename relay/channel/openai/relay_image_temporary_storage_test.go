package openai

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const temporaryStorageTestPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="

func TestUploadOpenAIImagesToLocalTemporaryStorage(t *testing.T) {
	t.Setenv("TEMP_STORAGE_DIR", t.TempDir())
	t.Setenv("TEMP_STORAGE_PUBLIC_BASE_URL", "https://api.o1key.cn")
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "https://api.o1key.cn/v1/images/generations", nil)
	body := []byte(`{"created":1,"data":[{"b64_json":"` + temporaryStorageTestPNG + `"}]}`)

	rewritten, err := uploadOpenAIImagesToStorage(c, body, dto.ImageOutputStrategyLocalTemp)
	require.NoError(t, err)
	url := gjson.GetBytes(rewritten, "data.0.url").String()
	assert.True(t, strings.HasPrefix(url, "https://api.o1key.cn/tmp/output/"))
	assert.Empty(t, gjson.GetBytes(rewritten, "data.0.b64_json").String())
}

func TestUploadOpenAIStreamImageToLocalTemporaryStorage(t *testing.T) {
	t.Setenv("TEMP_STORAGE_DIR", t.TempDir())
	t.Setenv("TEMP_STORAGE_PUBLIC_BASE_URL", "https://api.o1key.cn")
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "https://api.o1key.cn/v1/images/generations", nil)
	body := []byte(`{"type":"image_generation.completed","b64_json":"` + temporaryStorageTestPNG + `"}`)

	rewritten, err := uploadOpenAIStreamImageToStorage(c, body, dto.ImageOutputStrategyLocalTemp)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(gjson.GetBytes(rewritten, "url").String(), "https://api.o1key.cn/tmp/output/"))
	assert.False(t, gjson.GetBytes(rewritten, "b64_json").Exists())
}

func TestOpenAIImageStreamLocalTemporaryStrategyOnlyReturnsFinalLocalURL(t *testing.T) {
	t.Setenv("TEMP_STORAGE_DIR", t.TempDir())
	t.Setenv("TEMP_STORAGE_PUBLIC_BASE_URL", "https://api.o1key.cn")
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })
	body := strings.Join([]string{
		`data: {"type":"image_generation.partial_image","b64_json":"partial"}`,
		``,
		`data: {"type":"image_generation.completed","b64_json":"` + temporaryStorageTestPNG + `"}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")
	c, recorder, resp, info := newImageTestContext(t, body, "text/event-stream", true)
	info.ChannelOtherSettings.ImageOutputStrategy = dto.ImageOutputStrategyLocalTempCF

	_, relayErr := OpenaiImageStreamHandler(c, info, resp)
	require.Nil(t, relayErr)
	responseBody := recorder.Body.String()
	assert.NotContains(t, responseBody, "partial_image")
	assert.NotContains(t, responseBody, "b64_json")
	assert.Contains(t, responseBody, `"url":"https://cf-api.o1key.com/tmp/output/`)
	assert.Contains(t, responseBody, "data: [DONE]")
}
