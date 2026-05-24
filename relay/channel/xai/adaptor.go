package xai

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/QuantumNous/new-api/relay/constant"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

type Adaptor struct {
}

func (a *Adaptor) ConvertGeminiRequest(*gin.Context, *relaycommon.RelayInfo, *dto.GeminiChatRequest) (any, error) {
	//TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertClaudeRequest(*gin.Context, *relaycommon.RelayInfo, *dto.ClaudeRequest) (any, error) {
	//TODO implement me
	//panic("implement me")
	return nil, errors.New("not available")
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	//not available
	return nil, errors.New("not available")
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	if info.RelayMode == constant.RelayModeImagesEdits {
		imageField, err := buildImageField(c, request)
		if err != nil {
			return nil, err
		}
		return ImageEditRequest{
			Model:          request.Model,
			Prompt:         request.Prompt,
			N:              int(lo.FromPtrOr(request.N, uint(1))),
			ResponseFormat: request.ResponseFormat,
			Image:          imageField,
		}, nil
	}
	var resolution string
	if raw, ok := request.Extra["resolution"]; ok {
		_ = common.Unmarshal(raw, &resolution)
	}
	return ImageRequest{
		Model:          request.Model,
		Prompt:         request.Prompt,
		N:              int(lo.FromPtrOr(request.N, uint(1))),
		ResponseFormat: request.ResponseFormat,
		AspectRatio:    request.AspectRatio,
		Resolution:     resolution,
	}, nil
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	return relaycommon.GetFullRequestURL(info.ChannelBaseUrl, info.RequestURLPath, info.ChannelType), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)
	req.Set("Authorization", "Bearer "+info.ApiKey)
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	if strings.HasSuffix(info.UpstreamModelName, "-search") {
		info.UpstreamModelName = strings.TrimSuffix(info.UpstreamModelName, "-search")
		request.Model = info.UpstreamModelName
		toMap := request.ToMap()
		toMap["search_parameters"] = map[string]any{
			"mode": "on",
		}
		return toMap, nil
	}
	if strings.HasPrefix(request.Model, "grok-3-mini") {
		if lo.FromPtrOr(request.MaxCompletionTokens, uint(0)) == 0 && lo.FromPtrOr(request.MaxTokens, uint(0)) != 0 {
			request.MaxCompletionTokens = request.MaxTokens
			request.MaxTokens = nil
		}
		if strings.HasSuffix(request.Model, "-high") {
			request.ReasoningEffort = "high"
			request.Model = strings.TrimSuffix(request.Model, "-high")
		} else if strings.HasSuffix(request.Model, "-low") {
			request.ReasoningEffort = "low"
			request.Model = strings.TrimSuffix(request.Model, "-low")
		}
		info.ReasoningEffort = request.ReasoningEffort
		info.UpstreamModelName = request.Model
	}
	return request, nil
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, nil
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	//not available
	return nil, errors.New("not available")
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	if request.Model == "" && info != nil {
		request.Model = info.UpstreamModelName
	}
	return request, nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	switch info.RelayMode {
	case constant.RelayModeImagesGenerations, constant.RelayModeImagesEdits:
		usage, err = openai.OpenaiImageHandler(c, info, resp)
	case constant.RelayModeResponses:
		if info.IsStream {
			usage, err = openai.OaiResponsesStreamHandler(c, info, resp)
		} else {
			usage, err = openai.OaiResponsesHandler(c, info, resp)
		}
	default:
		if info.IsStream {
			usage, err = xAIStreamHandler(c, info, resp)
		} else {
			usage, err = xAIHandler(c, info, resp)
		}
	}
	return
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

func buildImageField(c *gin.Context, request dto.ImageRequest) ([]byte, error) {
	contentType := c.Request.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "application/json") {
		return buildImageFieldFromJSON(request)
	}
	return buildImageFieldFromMultipart(c)
}

func buildImageFieldFromJSON(request dto.ImageRequest) ([]byte, error) {
	if len(request.Image) == 0 && len(request.Images) == 0 {
		return nil, errors.New("image field is required for image edit requests")
	}
	if len(request.Image) > 0 {
		return request.Image, nil
	}
	var urls []string
	if err := common.Unmarshal(request.Images, &urls); err != nil {
		return request.Images, nil
	}
	var images []ImageObject
	for _, url := range urls {
		images = append(images, ImageObject{Url: url, Type: "image_url"})
	}
	return common.Marshal(images)
}

func buildImageFieldFromMultipart(c *gin.Context) ([]byte, error) {
	mf := c.Request.MultipartForm
	if mf == nil {
		return nil, errors.New("image file is required for image edit requests")
	}

	var fileHeaders []*multipart.FileHeader
	if files, ok := mf.File["image"]; ok && len(files) > 0 {
		fileHeaders = files
	} else if files, ok := mf.File["image[]"]; ok && len(files) > 0 {
		fileHeaders = files
	} else {
		for fieldName, files := range mf.File {
			if strings.HasPrefix(fieldName, "image[") && len(files) > 0 {
				fileHeaders = append(fileHeaders, files...)
			}
		}
	}
	if len(fileHeaders) == 0 {
		return nil, errors.New("image file is required for image edit requests")
	}

	var images []ImageObject
	for _, fh := range fileHeaders {
		f, err := fh.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to open image file: %w", err)
		}
		data, err := io.ReadAll(f)
		_ = f.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read image file: %w", err)
		}
		mimeType := detectMimeType(fh.Filename)
		b64 := base64.StdEncoding.EncodeToString(data)
		dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, b64)
		images = append(images, ImageObject{Url: dataURI, Type: "image_url"})
	}

	if len(images) == 1 {
		return common.Marshal(images[0])
	}
	return common.Marshal(images)
}

func detectMimeType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		return "image/png"
	}
}
