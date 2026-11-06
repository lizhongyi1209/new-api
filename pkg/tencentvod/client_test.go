package tencentvod

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Real responses captured from the live VOD API. Keeping them verbatim pins
// the field layout: a rename on our side or a shape change upstream fails here
// instead of silently producing a task with no images.

// oneKTaskDetail is a plain text-to-image task without "--hd".
const oneKTaskDetail = `{
  "Status": "FINISH",
  "TaskType": "AigcImageTask",
  "CreateTime": "2026-11-06T07:15:16Z",
  "BeginProcessTime": "2026-11-06T07:15:16Z",
  "FinishTime": "2026-11-06T07:16:33Z",
  "RequestId": "5ca72b99-3173-4f6a-94ec-fc9972a1e1b3",
  "AigcImageTask": {
    "TaskId": "1500070444-AigcImageTask-e139b572ef6f7cb8a847b27b5e055ec2t",
    "Status": "FINISH",
    "ErrCode": 0,
    "ErrCodeExt": "",
    "Message": "",
    "Progress": 100,
    "SessionId": "mj-newapi-d941c93475df45e58d855969",
    "Input": {
      "ModelName": "MJ",
      "ModelVersion": "v8.2",
      "FileInfos": [],
      "Prompt": "A small orange cat beside a sunlit window --ar 1:1 --q 1",
      "OutputConfig": {
        "StorageMode": "Temporary",
        "Resolution": "",
        "AspectRatio": ""
      }
    },
    "Output": {
      "FileInfos": [
        {"StorageMode": "Temporary", "ExpireTime": "2026-11-13T07:16:35Z", "FileType": "", "FileId": "",
         "FileUrl": "https://store.vod-qcloud.com/x/aigcImageGenFile_1.png",
         "MetaData": {"Width": 1024, "Height": 1024, "Size": 1831523, "Container": "png_pipe"}},
        {"StorageMode": "Temporary", "ExpireTime": "2026-11-13T07:16:35Z", "FileType": "", "FileId": "",
         "FileUrl": "https://store.vod-qcloud.com/x/aigcImageGenFile_2.png",
         "MetaData": {"Width": 1024, "Height": 1024, "Size": 1629776, "Container": "png_pipe"}},
        {"StorageMode": "Temporary", "ExpireTime": "2026-11-13T07:16:35Z", "FileType": "", "FileId": "",
         "FileUrl": "https://store.vod-qcloud.com/x/aigcImageGenFile_3.png",
         "MetaData": {"Width": 1024, "Height": 1024, "Size": 1634055, "Container": "png_pipe"}},
        {"StorageMode": "Temporary", "ExpireTime": "2026-11-13T07:16:35Z", "FileType": "", "FileId": "",
         "FileUrl": "https://store.vod-qcloud.com/x/aigcImageGenFile.png",
         "MetaData": {"Width": 1024, "Height": 1024, "Size": 1612517, "Container": "png_pipe"}}
      ],
      "Usage": {
        "SpecificationUsages": [
          {"Specification": "MJv8.2_1k", "Category": "aigc_image_mjv82_1k"},
          {"Specification": "MJv8.2_1k", "Category": "aigc_image_mjv82_1k"},
          {"Specification": "MJv8.2_1k", "Category": "aigc_image_mjv82_1k"},
          {"Specification": "MJv8.2_1k", "Category": "aigc_image_mjv82_1k"}
        ]
      }
    }
  }
}`

// twoKTaskDetail is the same prompt with "--hd". The tier moves to 2k and the
// images double in size, while OutputConfig.Resolution stays empty: the tier
// comes from the prompt flag, never from the request parameter.
const twoKTaskDetail = `{
  "Status": "FINISH",
  "TaskType": "AigcImageTask",
  "CreateTime": "2026-11-06T00:00:00Z",
  "FinishTime": "2026-11-06T00:01:20Z",
  "RequestId": "req-2k",
  "AigcImageTask": {
    "TaskId": "task-2k",
    "Status": "FINISH",
    "ErrCode": 0,
    "ErrCodeExt": "",
    "Progress": 100,
    "Input": {
      "ModelName": "MJ",
      "ModelVersion": "v8.2",
      "Prompt": "A small orange cat beside a sunlit window --ar 1:1 --q 2 --hd",
      "OutputConfig": {"StorageMode": "Temporary", "Resolution": ""}
    },
    "Output": {
      "FileInfos": [
        {"FileUrl": "https://store.vod-qcloud.com/y/1.png", "MetaData": {"Width": 2048, "Height": 2048}}
      ],
      "Usage": {"SpecificationUsages": [{"Specification": "MJv8.2_2k", "Category": "aigc_image_mjv82_2k"}]}
    }
  }
}`

// resolutionEchoedButIgnored records the finding that OutputConfig.Resolution
// is accepted and echoed for "4K" while the task still bills and renders at 2k.
const resolutionEchoedButIgnored = `{
  "Status": "FINISH",
  "TaskType": "AigcImageTask",
  "AigcImageTask": {
    "TaskId": "task-4k-request",
    "Status": "FINISH",
    "ErrCode": 0,
    "ErrCodeExt": "",
    "Progress": 100,
    "Input": {
      "ModelName": "MJ",
      "ModelVersion": "v8.2",
      "OutputConfig": {"StorageMode": "Temporary", "Resolution": "4K"}
    },
    "Output": {
      "FileInfos": [
        {"FileUrl": "https://store.vod-qcloud.com/z/1.png", "MetaData": {"Width": 2048, "Height": 2048}}
      ],
      "Usage": {"SpecificationUsages": [{"Specification": "MJv8.2_2k", "Category": "aigc_image_mjv82_2k"}]}
    }
  }
}`

func decodeDetail(t *testing.T, raw string) TaskDetailResponse {
	t.Helper()
	var detail TaskDetailResponse
	require.NoError(t, common.Unmarshal([]byte(raw), &detail))
	return detail
}

// stubTransport answers every request with a fixed body and records the last
// request so the signed headers and routing host can be asserted.
type stubTransport struct {
	status  int
	body    string
	lastURL string
	lastHost string
	lastAction string
	lastPayload []byte
}

func (s *stubTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	s.lastURL = req.URL.String()
	s.lastHost = req.Header.Get("Host")
	s.lastAction = req.Header.Get("X-TC-Action")
	if req.Body != nil {
		s.lastPayload, _ = io.ReadAll(req.Body)
		req.Body.Close()
	}
	status := s.status
	if status == 0 {
		status = http.StatusOK
	}
	body := s.body
	if body == "" {
		body = `{"Response":{"TaskId":"task-1","RequestId":"req-1"}}`
	}
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
		Request:    req,
	}, nil
}

func newStubClient(stub *stubTransport) *Client {
	return NewClient(Credential{SecretID: "AKIDtest", SecretKey: "secret"},
		&http.Client{Transport: stub})
}

func TestTaskDetailParsesGeneratedImages(t *testing.T) {
	detail := decodeDetail(t, oneKTaskDetail)
	task := detail.AigcImageTask

	require.True(t, task.Succeeded())
	require.True(t, task.Terminal())
	assert.Equal(t, "MJ", task.Input.ModelName)
	assert.Equal(t, "v8.2", task.Input.ModelVersion)

	require.Len(t, task.Output.FileInfos, 4, "a four-image task must keep all four results")
	first := task.Output.FileInfos[0]
	assert.Equal(t, "https://store.vod-qcloud.com/x/aigcImageGenFile_1.png", first.FileURL)
	assert.Equal(t, "2026-11-13T07:16:35Z", first.ExpireTime)
	require.NotNil(t, first.MetaData)
	assert.Equal(t, 1024, first.MetaData.Width)
	assert.Equal(t, 1024, first.MetaData.Height)
	assert.Equal(t, int64(1831523), first.MetaData.Size)

	// Timings belong to the outer object; the nested task has no such fields.
	assert.Equal(t, "2026-11-06T07:15:16Z", detail.CreateTime)
	assert.Equal(t, "2026-11-06T07:16:33Z", detail.FinishTime)
}

func TestResolutionTierFollowsTheHDFlag(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want string
	}{
		{name: "default prompt bills 1k", raw: oneKTaskDetail, want: ResolutionTier1K},
		{name: "hd flag bills 2k", raw: twoKTaskDetail, want: ResolutionTier2K},
		// Requesting 4K changes neither the rendered size nor the billed tier.
		{name: "requested 4K still bills 2k", raw: resolutionEchoedButIgnored, want: ResolutionTier2K},
	} {
		t.Run(tc.name, func(t *testing.T) {
			task := decodeDetail(t, tc.raw).AigcImageTask
			require.NotNil(t, task.Output.Usage)
			assert.Equal(t, tc.want, task.Output.Usage.ResolutionTier())
		})
	}
}

func TestResolutionTierRejectsUnknownShapes(t *testing.T) {
	for _, tc := range []struct {
		name  string
		usage OutputUsage
	}{
		{name: "no usages", usage: OutputUsage{}},
		{name: "empty specification", usage: OutputUsage{SpecificationUsages: []SpecificationUsage{{Category: "aigc_image_mjv82_4k"}}}},
		{name: "unrecognised tier", usage: OutputUsage{SpecificationUsages: []SpecificationUsage{{Specification: "MJv8.2_4k"}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Empty(t, tc.usage.ResolutionTier(),
				"an unknown specification must not be guessed into a priced tier")
		})
	}
}

func TestSucceededRequiresHealthyFields(t *testing.T) {
	base := func() AigcImageTask {
		return decodeDetail(t, oneKTaskDetail).AigcImageTask
	}

	t.Run("finish without files is not success", func(t *testing.T) {
		task := base()
		task.Output.FileInfos = nil
		assert.False(t, task.Succeeded())
		assert.Contains(t, task.FailureReason(), "上游未返回图片")
	})

	t.Run("non-zero ErrCode is not success", func(t *testing.T) {
		task := base()
		task.ErrCode = 40000
		assert.False(t, task.Succeeded())
		assert.Contains(t, task.FailureReason(), "ErrCode=40000")
	})

	t.Run("non-empty ErrCodeExt is not success", func(t *testing.T) {
		task := base()
		task.ErrCodeExt = "ContentModerationFailed"
		assert.False(t, task.Succeeded())
		assert.Contains(t, task.FailureReason(), "ContentModerationFailed")
	})

	t.Run("still processing is not terminal", func(t *testing.T) {
		task := base()
		task.Status = TaskStatusProcessing
		assert.False(t, task.Terminal())
		assert.False(t, task.Succeeded())
	})
}

// TestSubmitHostRouting pins the domain rule: Base64 input is only accepted on
// the gateway endpoint, so sending it to the API host would fail the upload.
func TestSubmitHostRouting(t *testing.T) {
	assert.Equal(t, HostAPI, SubmitHost(nil))
	assert.Equal(t, HostAPI, SubmitHost([]InputFileInfo{{Type: "Url", URL: "https://example.com/ref.png"}}))
	assert.Equal(t, HostGateway, SubmitHost([]InputFileInfo{{Type: "Base64", Base64: "aGVsbG8="}}))
}

// TestCreateImageTaskRoutesBase64ToTheGatewayHost proves the routing decision
// reaches the network call: the signed Host header and URL must both be the
// gateway domain.
func TestCreateImageTaskRoutesBase64ToTheGatewayHost(t *testing.T) {
	stub := &stubTransport{}
	client := newStubClient(stub)

	_, err := client.CreateImageTask(context.Background(), CreateImageTaskRequest{
		SubAppID: 1500070444, ModelName: ModelNameMJ, ModelVersion: ModelVersionV82,
		Prompt: "x", FileInfos: []InputFileInfo{{Type: "Base64", Base64: "aGVsbG8="}},
	})
	require.NoError(t, err)

	assert.Equal(t, "https://"+HostGateway+"/", stub.lastURL)
	assert.Equal(t, HostGateway, stub.lastHost, "the signature Host header must match the endpoint actually called")
	assert.Equal(t, ActionCreateImageTask, stub.lastAction)
	assert.Contains(t, string(stub.lastPayload), `"Base64":"aGVsbG8="`)
}

func TestCreateImageTaskRejectsMissingTaskID(t *testing.T) {
	stub := &stubTransport{body: `{"Response":{"RequestId":"req-1"}}`}
	_, err := newStubClient(stub).CreateImageTask(context.Background(), CreateImageTaskRequest{
		SubAppID: 1, ModelName: ModelNameMJ, ModelVersion: ModelVersionV82, Prompt: "x",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no TaskId")
}

func TestCreateImageTaskSurfacesProviderError(t *testing.T) {
	stub := &stubTransport{body: `{"Response":{"Error":{"Code":"AuthFailure.SignatureFailure","Message":"signature mismatch"},"RequestId":"req-bad"}}`}
	_, err := newStubClient(stub).CreateImageTask(context.Background(), CreateImageTaskRequest{
		SubAppID: 1, ModelName: ModelNameMJ, ModelVersion: ModelVersionV82, Prompt: "x",
	})
	require.Error(t, err, "an HTTP 200 body carrying Response.Error must not look like an accepted task")
	assert.Contains(t, err.Error(), "AuthFailure.SignatureFailure")
}

func TestDescribeTaskSurfacesProviderError(t *testing.T) {
	stub := &stubTransport{body: `{"Response":{"Error":{"Code":"InvalidParameterValue","Message":"task id invalid"}}}`}
	_, err := newStubClient(stub).DescribeTask(context.Background(), 1, "task-x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "InvalidParameterValue")
}

func TestHTTPFailureIsReportedWithBody(t *testing.T) {
	stub := &stubTransport{status: http.StatusForbidden, body: `denied`}
	_, err := newStubClient(stub).DescribeTask(context.Background(), 1, "task-x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 403")
	assert.Contains(t, err.Error(), "denied")
}

func TestSignSetsTC3Headers(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://"+HostAPI+"/", strings.NewReader(""))
	require.NoError(t, err)

	payload := []byte(`{"SubAppId":1500070444}`)
	now := time.Date(2026, 11, 6, 7, 15, 16, 0, time.UTC)
	sign(req, payload, Credential{SecretID: "AKIDtest", SecretKey: "secret"}, signTarget{
		Host: HostAPI, Action: ActionCreateImageTask, Version: VersionVOD,
	}, now)

	assert.Equal(t, ActionCreateImageTask, req.Header.Get("X-TC-Action"))
	assert.Equal(t, VersionVOD, req.Header.Get("X-TC-Version"))
	assert.Equal(t, "1793949316", req.Header.Get("X-TC-Timestamp"))
	assert.Equal(t, HostAPI, req.Host)
	auth := req.Header.Get("Authorization")
	assert.True(t, strings.HasPrefix(auth, signAlgorithm+" Credential=AKIDtest/2026-11-06/vod/tc3_request"),
		"credential scope must bind the signature to the date and service: %s", auth)
	assert.Contains(t, auth, "SignedHeaders=content-type;host;x-tc-action")
}

func TestParseCredential(t *testing.T) {
	cred, err := ParseCredential("AKIDabc|secret123")
	require.NoError(t, err)
	assert.Equal(t, "AKIDabc", cred.SecretID)
	assert.Equal(t, "secret123", cred.SecretKey)

	for _, bad := range []string{"", "onlyid", "AKID|", "|secret"} {
		_, err := ParseCredential(bad)
		assert.Error(t, err, "credential %q must be rejected", bad)
	}
}
