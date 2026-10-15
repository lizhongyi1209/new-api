package sub2api

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRequestURLAlphaSearch(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    constant.ChannelTypeSub2API,
			ChannelBaseUrl: "https://sub2api.example",
		},
		RequestURLPath: "/v1/alpha/search",
		RelayMode:      relayconstant.RelayModeAlphaSearch,
	}

	url, err := adaptor.GetRequestURL(info)
	require.NoError(t, err)
	assert.Equal(t, "https://sub2api.example/v1/alpha/search", url)
}

func TestMultipartImageEditUsesOpenAICompatibleFormRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type receivedRequest struct {
		contentType           string
		authorization         string
		model                 string
		prompt                string
		quality               string
		responseFormatPresent bool
		imageCount            int
		image                 []byte
		err                   error
	}
	received := make(chan receivedRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := receivedRequest{
			contentType:   r.Header.Get("Content-Type"),
			authorization: r.Header.Get("Authorization"),
		}
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			result.err = err
			received <- result
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		result.model = r.PostForm.Get("model")
		result.prompt = r.PostForm.Get("prompt")
		result.quality = r.PostForm.Get("quality")
		result.responseFormatPresent = r.PostForm.Has("response_format")
		result.imageCount = len(r.MultipartForm.File["image"])
		if result.imageCount == 1 {
			file, err := r.MultipartForm.File["image"][0].Open()
			if err == nil {
				result.image, err = io.ReadAll(file)
				if closeErr := file.Close(); err == nil {
					err = closeErr
				}
			}
			result.err = err
		}
		received <- result
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	var clientBody bytes.Buffer
	clientWriter := multipart.NewWriter(&clientBody)
	require.NoError(t, clientWriter.WriteField("model", "gpt-image-2.5-flare-sp"))
	require.NoError(t, clientWriter.WriteField("prompt", "edit image"))
	require.NoError(t, clientWriter.WriteField("quality", "low"))
	require.NoError(t, clientWriter.WriteField("response_format", "b64_json"))
	imagePart, err := clientWriter.CreateFormFile("image", "input.png")
	require.NoError(t, err)
	_, err = imagePart.Write([]byte("fake image"))
	require.NoError(t, err)
	require.NoError(t, clientWriter.Close())

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", &clientBody)
	c.Request.Header.Set("Content-Type", clientWriter.FormDataContentType())
	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeImagesEdits,
		RequestURLPath:  "/v1/images/edits",
		OriginModelName: "gpt-image-2.5-flare-sp",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeSub2API,
			ChannelBaseUrl:    server.URL,
			ApiKey:            "test-key",
			UpstreamModelName: "gpt-image-2.5-flare",
		},
	}
	request := dto.ImageRequest{
		Model:          "gpt-image-2.5-flare",
		Prompt:         "edit image",
		ResponseFormat: "b64_json",
	}
	adaptor := &Adaptor{}
	adaptor.Init(info)

	converted, err := adaptor.ConvertImageRequest(c, info, request)
	require.NoError(t, err)
	requestBody, ok := converted.(*bytes.Buffer)
	require.True(t, ok)
	responseValue, err := adaptor.DoRequest(c, info, requestBody)
	require.NoError(t, err)
	response, ok := responseValue.(*http.Response)
	require.True(t, ok)
	require.NoError(t, response.Body.Close())

	upstreamRequest := <-received
	require.NoError(t, upstreamRequest.err)
	assert.Contains(t, upstreamRequest.contentType, "multipart/form-data; boundary=")
	assert.NotEqual(t, clientWriter.FormDataContentType(), upstreamRequest.contentType)
	assert.Equal(t, "Bearer test-key", upstreamRequest.authorization)
	assert.Equal(t, "gpt-image-2.5-flare", upstreamRequest.model)
	assert.Equal(t, "edit image", upstreamRequest.prompt)
	assert.Equal(t, "low", upstreamRequest.quality)
	assert.False(t, upstreamRequest.responseFormatPresent)
	assert.Equal(t, 1, upstreamRequest.imageCount)
	assert.Equal(t, []byte("fake image"), upstreamRequest.image)
}

func TestJSONImageRequestKeepsExistingSub2APIBehavior(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewBufferString(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesGenerations,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeSub2API,
		},
	}
	request := dto.ImageRequest{Model: "gpt-image-2.5-flare", Prompt: "draw image"}
	adaptor := &Adaptor{}
	adaptor.Init(info)

	converted, err := adaptor.ConvertImageRequest(c, info, request)
	require.NoError(t, err)
	assert.Equal(t, request, converted)

	encoded, err := common.Marshal(converted)
	require.NoError(t, err)
	assert.JSONEq(t, `{"model":"gpt-image-2.5-flare","prompt":"draw image"}`, string(encoded))
}
