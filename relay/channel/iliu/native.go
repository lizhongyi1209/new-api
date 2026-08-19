package iliu

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const maxImageBytes = 20 << 20

var supportedModes = map[string]bool{"RELAX": true, "FAST": true, "TURBO": true}
var supportedBotTypes = map[string]bool{"MID_JOURNEY": true, "NIJI_JOURNEY": true}
var supportedDimensions = map[string]bool{"PORTRAIT": true, "SQUARE": true, "LANDSCAPE": true}

type NativeRequest struct {
	Action  string
	Payload any
}

type TaskAdaptor struct {
	taskcommon.BaseBilling
}

func ModelForPath(path string) string {
	switch {
	case strings.HasSuffix(path, "/submit/imagine"):
		return "mj_imagine"
	case strings.HasSuffix(path, "/submit/blend"):
		return "mj_blend"
	case strings.HasSuffix(path, "/submit/describe"):
		return "mj_describe"
	case strings.HasSuffix(path, "/submit/shorten"):
		return "mj_shorten"
	case strings.HasSuffix(path, "/submit/edits"):
		return "mj_edits"
	case strings.HasSuffix(path, "/submit/video"):
		return "mj_video"
	case strings.HasSuffix(path, "/insight-face/swap"):
		return "swap_face"
	case strings.HasSuffix(path, "/submit/upload-discord-images"):
		return "mj_upload"
	case strings.HasSuffix(path, "/submit/action"):
		return "mj_action"
	case strings.HasSuffix(path, "/submit/modal"):
		return "mj_modal"
	default:
		return ""
	}
}

func (a *TaskAdaptor) Init(_ *relaycommon.RelayInfo) {}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	request, err := ParseNativeRequest(c)
	if err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	info.Action = request.Action
	c.Set("iliu_native_request", request.Payload)
	return nil
}

func ParseNativeRequest(c *gin.Context) (*NativeRequest, error) {
	path := c.Request.URL.Path
	modelName := ModelForPath(path)
	if modelName == "" {
		return nil, fmt.Errorf("unsupported iLiu Midjourney endpoint")
	}

	var payload any
	var err error
	switch modelName {
	case "mj_imagine":
		request := &ImagineRequest{}
		err = common.UnmarshalBodyReusable(c, request)
		if err == nil {
			err = validateCommonOptions(request.CommonOptions)
		}
		if err == nil && strings.TrimSpace(request.Prompt) == "" {
			err = fmt.Errorf("prompt is required")
		}
		if err == nil {
			err = validateImages(request.Base64Array, 5)
		}
		payload = request
	case "mj_action":
		request := &ActionRequest{}
		err = common.UnmarshalBodyReusable(c, request)
		if err == nil && strings.TrimSpace(request.CustomID) == "" {
			err = fmt.Errorf("customId is required")
		}
		if err == nil && strings.TrimSpace(request.TaskID) == "" {
			err = fmt.Errorf("taskId is required")
		}
		if err == nil {
			err = validateNotifyHook(request.NotifyHook)
		}
		payload = request
	case "mj_blend":
		request := &BlendRequest{}
		err = common.UnmarshalBodyReusable(c, request)
		if err == nil {
			err = validateCommonOptions(request.CommonOptions)
		}
		if err == nil && (len(request.Base64Array) < 2 || len(request.Base64Array) > 5) {
			err = fmt.Errorf("base64Array must contain between 2 and 5 images")
		}
		if err == nil {
			err = validateImages(request.Base64Array, 5)
		}
		if err == nil && request.Dimensions != nil && !supportedDimensions[*request.Dimensions] {
			err = fmt.Errorf("dimensions must be PORTRAIT, SQUARE, or LANDSCAPE")
		}
		payload = request
	case "mj_modal":
		request := &ModalRequest{}
		err = common.UnmarshalBodyReusable(c, request)
		if err == nil && strings.TrimSpace(request.TaskID) == "" {
			err = fmt.Errorf("taskId is required")
		}
		if err == nil && request.MaskBase64 != nil {
			err = validateImage(*request.MaskBase64)
		}
		payload = request
	case "mj_describe":
		request := &DescribeRequest{}
		err = common.UnmarshalBodyReusable(c, request)
		if err == nil {
			err = validateCommonOptions(request.CommonOptions)
		}
		if err == nil {
			err = validateImage(request.Base64)
		}
		payload = request
	case "mj_shorten":
		request := &ShortenRequest{}
		err = common.UnmarshalBodyReusable(c, request)
		if err == nil {
			err = validateCommonOptions(request.CommonOptions)
		}
		if err == nil && strings.TrimSpace(request.Prompt) == "" {
			err = fmt.Errorf("prompt is required")
		}
		payload = request
	case "swap_face":
		request := &FaceSwapRequest{}
		err = common.UnmarshalBodyReusable(c, request)
		if err == nil {
			err = validateCommonOptions(request.CommonOptions)
		}
		if err == nil {
			err = validateImage(request.SourceBase64)
		}
		if err == nil {
			err = validateImage(request.TargetBase64)
		}
		payload = request
	case "mj_upload":
		request := &UploadRequest{}
		err = common.UnmarshalBodyReusable(c, request)
		if err == nil && len(request.Base64Array) == 0 {
			err = fmt.Errorf("base64Array is required")
		}
		if err == nil {
			err = validateImages(request.Base64Array, 10)
		}
		payload = request
	case "mj_edits":
		request := &EditsRequest{}
		err = common.UnmarshalBodyReusable(c, request)
		if err == nil && strings.TrimSpace(request.Prompt) == "" {
			err = fmt.Errorf("prompt is required")
		}
		if err == nil {
			err = validateImage(request.ImageBase64)
		}
		payload = request
	case "mj_video":
		request := &VideoRequest{}
		err = common.UnmarshalBodyReusable(c, request)
		if err == nil && strings.TrimSpace(request.Prompt) == "" {
			err = fmt.Errorf("prompt is required")
		}
		if err == nil {
			err = validatePublicURL(request.Image, "image")
		}
		if err == nil && request.EndImage != nil {
			err = validatePublicURL(*request.EndImage, "endImage")
		}
		payload = request
	}
	if err != nil {
		return nil, err
	}
	return &NativeRequest{Action: strings.TrimPrefix(modelName, "mj_"), Payload: payload}, nil
}

func validateCommonOptions(options CommonOptions) error {
	if options.Mode != nil && !supportedModes[*options.Mode] {
		return fmt.Errorf("mode must be RELAX, FAST, or TURBO")
	}
	if options.BotType != nil && !supportedBotTypes[*options.BotType] {
		return fmt.Errorf("botType must be MID_JOURNEY or NIJI_JOURNEY")
	}
	if options.AccountFilter != nil {
		for _, mode := range options.AccountFilter.Modes {
			if !supportedModes[mode] {
				return fmt.Errorf("accountFilter.modes contains an invalid mode")
			}
		}
	}
	return validateNotifyHook(options.NotifyHook)
}

func validateNotifyHook(rawURL *string) error {
	if rawURL == nil || strings.TrimSpace(*rawURL) == "" {
		return nil
	}
	parsed, err := url.Parse(strings.TrimSpace(*rawURL))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return fmt.Errorf("notifyHook must be a valid HTTPS URL")
	}
	if err := validatePublicNetworkHost(parsed); err != nil {
		return fmt.Errorf("notifyHook must be a valid HTTPS URL: %w", err)
	}
	return nil
}

func validatePublicURL(rawURL, field string) error {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" || parsed.User != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return fmt.Errorf("%s must be a valid public HTTP(S) URL", field)
	}
	if err := validatePublicNetworkHost(parsed); err != nil {
		return fmt.Errorf("%s must be a valid public HTTP(S) URL: %w", field, err)
	}
	return nil
}

// validatePublicNetworkHost rejects explicit local/private targets before they
// are forwarded to a provider that will fetch the supplied URL. Hostname DNS
// rebinding remains the upstream fetcher's responsibility because this gateway
// does not resolve or fetch these media URLs itself.
func validatePublicNetworkHost(parsed *url.URL) error {
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") ||
		strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return fmt.Errorf("private network host is not allowed")
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return nil
	}
	policy := common.SSRFProtection{AllowPrivateIp: false, IpFilterMode: false}
	if !policy.IsIPAccessAllowed(ip) {
		return fmt.Errorf("private network address is not allowed")
	}
	return nil
}

func validateImages(images []string, maxCount int) error {
	if len(images) > maxCount {
		return fmt.Errorf("too many images; maximum is %d", maxCount)
	}
	for _, image := range images {
		if err := validateImage(image); err != nil {
			return err
		}
	}
	return nil
}

func validateImage(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("image data is required")
	}
	encoded := raw
	declaredType := ""
	if strings.HasPrefix(raw, "data:") {
		comma := strings.IndexByte(raw, ',')
		if comma < 0 {
			return fmt.Errorf("invalid image data URL")
		}
		metadata := raw[5:comma]
		parts := strings.Split(metadata, ";")
		declaredType = parts[0]
		if len(parts) < 2 || parts[len(parts)-1] != "base64" {
			return fmt.Errorf("image data URL must use base64 encoding")
		}
		encoded = raw[comma+1:]
	}
	if base64.StdEncoding.DecodedLen(len(encoded)) > maxImageBytes {
		return fmt.Errorf("image exceeds the %d MiB limit", maxImageBytes>>20)
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return fmt.Errorf("invalid image base64 data")
	}
	if len(decoded) > maxImageBytes {
		return fmt.Errorf("image exceeds the %d MiB limit", maxImageBytes>>20)
	}
	detectedType := http.DetectContentType(decoded)
	if !isSupportedImageType(detectedType) {
		return fmt.Errorf("unsupported image MIME type %s", detectedType)
	}
	if declaredType != "" && !isSupportedImageType(declaredType) {
		return fmt.Errorf("unsupported declared image MIME type %s", declaredType)
	}
	return nil
}

func isSupportedImageType(contentType string) bool {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/jpeg", "image/png", "image/webp", "image/gif":
		return true
	default:
		return false
	}
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if info == nil {
		return "", fmt.Errorf("relay info is nil")
	}
	return buildNativeURL(info.ChannelBaseUrl, info.RequestURLPath), nil
}

func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	req.Header.Set("Authorization", "Bearer "+info.ApiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, _ *relaycommon.RelayInfo) (io.Reader, error) {
	payload, exists := c.Get("iliu_native_request")
	if !exists {
		return nil, fmt.Errorf("validated iLiu request is missing")
	}
	body, err := common.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(body), nil
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

func (a *TaskAdaptor) DoResponse(_ *gin.Context, resp *http.Response, _ *relaycommon.RelayInfo) (string, []byte, *dto.TaskError) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	envelope, taskID, err := ParseSubmitResponse(body)
	if err != nil {
		return "", body, service.TaskErrorWrapper(err, "invalid_upstream_response", http.StatusBadGateway)
	}
	if envelope.Code != 1 && envelope.Code != 21 && envelope.Code != 22 {
		return "", body, service.TaskErrorWrapper(fmt.Errorf("%s", envelope.Description), fmt.Sprintf("%d", envelope.Code), resp.StatusCode)
	}
	return taskID, body, nil
}

func ParseSubmitResponse(body []byte) (*SubmitResponse, string, error) {
	response := &SubmitResponse{}
	if err := common.Unmarshal(body, response); err != nil {
		return nil, "", err
	}
	var taskID string
	if len(response.Result) > 0 && string(response.Result) != "null" {
		_ = common.Unmarshal(response.Result, &taskID)
	}
	return response, taskID, nil
}

func ParseUploadResult(result json.RawMessage) ([]string, error) {
	var urls []string
	if err := common.Unmarshal(result, &urls); err != nil {
		return nil, fmt.Errorf("upload result must be a string array")
	}
	if len(urls) == 0 {
		return nil, fmt.Errorf("upload result must not be empty")
	}
	for _, rawURL := range urls {
		parsed, err := url.Parse(strings.TrimSpace(rawURL))
		if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
			return nil, fmt.Errorf("upload result contains an invalid image URL")
		}
	}
	return urls, nil
}

func (a *TaskAdaptor) GetModelList() []string { return ModelList }
func (a *TaskAdaptor) GetChannelName() string { return ChannelName }

func (a *TaskAdaptor) FetchTask(baseURL, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, _ := body["task_id"].(string)
	if taskID == "" {
		return nil, fmt.Errorf("task_id is required")
	}
	requestURL := buildNativeURL(baseURL, "/v1/mj/task/"+url.PathEscape(taskID)+"/fetch")
	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Accept", "application/json")
	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

func DoChannelRequest(ctx *gin.Context, channelModel *model.Channel, key, method, path string, body []byte) (*http.Response, error) {
	if channelModel == nil {
		return nil, fmt.Errorf("channel is nil")
	}
	if key == "" {
		selectedKey, _, apiErr := channelModel.GetNextEnabledKey()
		if apiErr != nil {
			return nil, apiErr
		}
		key = selectedKey
	}
	var reader io.Reader
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx.Request.Context(), method, buildNativeURL(channelModel.GetBaseURL(), path), reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Accept", "application/json")
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	settings := channelModel.GetSetting()
	client, err := service.GetHttpClientWithProxySettings(settings.Proxy, settings)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

func buildNativeURL(baseURL, path string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	if strings.HasPrefix(path, "/v1/") && strings.HasSuffix(baseURL, "/v1") {
		path = strings.TrimPrefix(path, "/v1")
	}
	return baseURL + path
}

func (a *TaskAdaptor) ParseTaskResult(body []byte) (*relaycommon.TaskInfo, error) {
	response := &TaskResponse{}
	if err := common.Unmarshal(body, response); err != nil {
		return nil, err
	}
	status := response.Status
	switch response.Status {
	case "NOT_START", "SUBMITTED":
		status = string(model.TaskStatusSubmitted)
	case "MODAL", "IN_PROGRESS":
		status = string(model.TaskStatusInProgress)
	case "SUCCESS":
		status = string(model.TaskStatusSuccess)
	case "FAILURE", "CANCEL":
		status = string(model.TaskStatusFailure)
	}
	resultURL := response.ImageURL
	if resultURL == "" {
		resultURL = response.VideoURL
	}
	reason := response.FailReason
	if response.Status == "CANCEL" && reason == "" {
		reason = "task cancelled"
	}
	return &relaycommon.TaskInfo{
		TaskID:   response.ID,
		Status:   status,
		Reason:   reason,
		Url:      resultURL,
		Progress: response.Progress,
	}, nil
}
