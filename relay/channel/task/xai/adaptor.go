package xai

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/task/grokvideo"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type TaskAdaptor struct {
	taskcommon.BaseBilling
	apiKey  string
	baseURL string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.apiKey = info.ApiKey
	a.baseURL = info.ChannelBaseUrl
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	_, taskErr := grokvideo.ParseAndValidate(c, info)
	return taskErr
}

func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	request, ok := grokvideo.GetRequest(c)
	if !ok {
		return nil
	}
	return grokvideo.EstimateBilling(request, info.Action)
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	path := "/v1/videos/generations"
	switch info.Action {
	case constant.TaskActionVideoEdit:
		path = "/v1/videos/edits"
	case constant.TaskActionVideoExtend:
		path = "/v1/videos/extensions"
	}
	return strings.TrimRight(a.baseURL, "/") + path, nil
}

func (a *TaskAdaptor) BuildRequestHeader(_ *gin.Context, request *http.Request, _ *relaycommon.RelayInfo) error {
	request.Header.Set("Authorization", "Bearer "+a.apiKey)
	request.Header.Set("Content-Type", "application/json")
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	request, ok := grokvideo.GetRequest(c)
	if !ok {
		return nil, fmt.Errorf("xAI video request not found")
	}
	request.Model = info.UpstreamModelName
	body, err := common.Marshal(request)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(body), nil
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, body io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, body)
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, response *http.Response, info *relaycommon.RelayInfo) (string, []byte, *dto.TaskError) {
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", nil, service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	_ = response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", nil, service.TaskErrorWrapper(fmt.Errorf("%s", body), "fail_to_fetch_task", response.StatusCode)
	}
	var result submitResponse
	if err := common.Unmarshal(body, &result); err != nil {
		return "", nil, service.TaskErrorWrapper(err, "unmarshal_response_body_failed", http.StatusInternalServerError)
	}
	if strings.TrimSpace(result.RequestID) == "" {
		return "", nil, service.TaskErrorWrapper(fmt.Errorf("request_id is empty"), "invalid_response", http.StatusInternalServerError)
	}
	c.JSON(response.StatusCode, submitResponse{RequestID: info.PublicTaskID})
	return result.RequestID, body, nil
}

func (a *TaskAdaptor) FetchTask(baseURL, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok || strings.TrimSpace(taskID) == "" {
		return nil, fmt.Errorf("invalid task_id")
	}
	request, err := http.NewRequest(http.MethodGet, strings.TrimRight(baseURL, "/")+"/v1/videos/"+taskID, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+key)
	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, err
	}
	return client.Do(request)
}

func (a *TaskAdaptor) ParseTaskResult(body []byte) (*relaycommon.TaskInfo, error) {
	var response videoResponse
	if err := common.Unmarshal(body, &response); err != nil {
		return nil, err
	}
	result := &relaycommon.TaskInfo{}
	switch response.Status {
	case "pending":
		result.Status = model.TaskStatusInProgress
	case "done":
		result.Status = model.TaskStatusSuccess
		if response.Video != nil {
			result.Url = response.Video.URL
			result.Metadata = map[string]any{"duration": response.Video.Duration}
		}
	case "failed", "expired":
		result.Status = model.TaskStatusFailure
		result.Reason = response.Status
		if response.Error != nil && response.Error.Message != "" {
			result.Reason = response.Error.Message
		}
	default:
		return nil, fmt.Errorf("unknown xAI video status %q", response.Status)
	}
	return result, nil
}

func (a *TaskAdaptor) GetModelList() []string {
	return []string{
		"grok-imagine-video",
		"grok-imagine-video-1.5",
		"grok-imagine-video-1.5-preview",
		"grok-imagine-video-1.5-2026-05-30",
	}
}
func (a *TaskAdaptor) GetChannelName() string { return "xai-video" }
