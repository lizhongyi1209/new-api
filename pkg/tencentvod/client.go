package tencentvod

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// Action names for the two VOD APIs this package uses.
const (
	ActionCreateImageTask    = "CreateAigcImageTask"
	ActionDescribeTaskDetail = "DescribeTaskDetail"
)

// maxResponseBytes bounds a provider reply so a malformed upstream cannot
// exhaust memory.
const maxResponseBytes = 8 << 20

// Client issues signed VOD requests. Timeout applies to each HTTP call, not to
// the whole polling cycle.
type Client struct {
	Cred    Credential
	HTTP    *http.Client
	Timeout time.Duration
	// Now is injectable so signature tests are deterministic.
	Now func() time.Time
}

// NewClient builds a client with sensible defaults.
func NewClient(cred Credential, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		Cred:    cred,
		HTTP:    httpClient,
		Timeout: 60 * time.Second,
		Now:     time.Now,
	}
}

// CreateImageTask submits a generation task. It returns the upstream task ID,
// which must be persisted before polling: a submit timeout does not prove the
// provider rejected the request, and re-submitting would bill twice.
func (c *Client) CreateImageTask(ctx context.Context, req CreateImageTaskRequest) (CreateImageTaskResponse, error) {
	var resp CreateImageTaskResponse
	if err := c.call(ctx, SubmitHost(req.FileInfos), ActionCreateImageTask, req, &resp); err != nil {
		return CreateImageTaskResponse{}, err
	}
	if resp.Error != nil {
		return CreateImageTaskResponse{}, resp.Error
	}
	if resp.TaskID == "" {
		return CreateImageTaskResponse{}, fmt.Errorf("tencent vod: create image task returned no TaskId")
	}
	return resp, nil
}

// DescribeTask returns the current state of a task.
func (c *Client) DescribeTask(ctx context.Context, subAppID int64, taskID string) (TaskDetailResponse, error) {
	var resp TaskDetailResponse
	body := DescribeTaskRequest{SubAppID: subAppID, TaskID: taskID}
	if err := c.call(ctx, HostAPI, ActionDescribeTaskDetail, body, &resp); err != nil {
		return TaskDetailResponse{}, err
	}
	if resp.Error != nil {
		return TaskDetailResponse{}, resp.Error
	}
	return resp, nil
}

// SubmitHost returns the submission endpoint for a request. Base64 reference
// images must go to the gateway host; the API host serves text-to-image and
// every query and rejects Base64 submissions.
func SubmitHost(files []InputFileInfo) string {
	for _, file := range files {
		if file.Type == "Base64" && file.Base64 != "" {
			return HostGateway
		}
	}
	return HostAPI
}

// call signs and performs one request. The payload is serialized once and the
// same bytes are both signed and sent; re-marshaling between signing and
// sending would invalidate the signature.
//
// VOD wraps every reply in a "Response" envelope, and reports API-level
// failures inside that envelope while still returning HTTP 200. Both facts are
// unwrapped here so callers only see the task payload or a real error.
func (c *Client) call(ctx context.Context, host, action string, requestBody, out any) error {
	payload, err := common.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("tencent vod: marshal %s request: %w", action, err)
	}

	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(callCtx, http.MethodPost, "https://"+host+"/", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("tencent vod: build %s request: %w", action, err)
	}
	now := time.Now
	if c.Now != nil {
		now = c.Now
	}
	sign(req, payload, c.Cred, signTarget{Host: host, Action: action, Version: VersionVOD}, now())

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("tencent vod: %s request failed: %w", action, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return fmt.Errorf("tencent vod: read %s response: %w", action, err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("tencent vod: %s returned HTTP %d: %s", action, resp.StatusCode, truncateForError(body))
	}

	var envelope struct {
		Response json.RawMessage `json:"Response"`
	}
	if err := common.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("tencent vod: parse %s response: %w", action, err)
	}
	if len(envelope.Response) == 0 {
		return fmt.Errorf("tencent vod: %s response missing Response envelope: %s", action, truncateForError(body))
	}

	var apiError struct {
		Error *APIError `json:"Error"`
	}
	if err := common.Unmarshal(envelope.Response, &apiError); err != nil {
		return fmt.Errorf("tencent vod: parse %s error envelope: %w", action, err)
	}
	if apiError.Error != nil {
		return apiError.Error
	}

	if err := common.Unmarshal(envelope.Response, out); err != nil {
		return fmt.Errorf("tencent vod: parse %s response: %w", action, err)
	}
	return nil
}

// truncateForError keeps error text readable without dumping a whole body into
// a log line or task fail_reason.
func truncateForError(body []byte) string {
	const limit = 512
	if len(body) <= limit {
		return string(body)
	}
	return string(body[:limit]) + "..."
}
