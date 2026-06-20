package tencentvideo

// element.go implements the Tencent VCLM "主体管理" (AIGC Element) management
// APIs: CreateAigcElement / DescribeAigcElement / DeleteAigcElement. Unlike the
// video-generation flow these are plain request/response calls (no unified
// async-task framework), so they expose a single signed-call helper that reuses
// the package's TC3-HMAC-SHA256 signer and credential parsing.

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
)

const (
	ActionCreateElement   = "CreateAigcElement"
	ActionDescribeElement = "DescribeAigcElement"
	ActionDeleteElement   = "DeleteAigcElement"

	// ReferenceTypeImage / ReferenceTypeVideo are the two ways a subject can be
	// defined when creating an element.
	ReferenceTypeImage = "image_refer"
	ReferenceTypeVideo = "video_refer"
)

// ============================
// Management request / response structures (PascalCase, like the rest of VCLM)
// ============================

// ReferImageItem is one non-frontal reference image.
type ReferImageItem struct {
	ImageUrl string `json:"ImageUrl,omitempty"`
}

// ElementImageList carries the frontal image plus 1~3 reference images for an
// image_refer subject.
type ElementImageList struct {
	FrontalImage string           `json:"FrontalImage,omitempty"`
	ReferImages  []ReferImageItem `json:"ReferImages,omitempty"`
}

// TagItem maps to Tencent's TagList entry (o_101 等).
type TagItem struct {
	TagId string `json:"TagId,omitempty"`
}

// CreateElementRequest is the CreateAigcElement request body.
type CreateElementRequest struct {
	Name             string            `json:"Name,omitempty"`
	Description      string            `json:"Description,omitempty"`
	ReferenceType    string            `json:"ReferenceType,omitempty"`
	ElementImageList *ElementImageList `json:"ElementImageList,omitempty"`
	VideoList        []string          `json:"VideoList,omitempty"`
	Provider         []string          `json:"Provider,omitempty"`
	TagList          []TagItem         `json:"TagList,omitempty"`
	ElementVoiceId   string            `json:"ElementVoiceId,omitempty"`
}

// ProviderDetail is one厂商聚合状态 entry returned by DescribeAigcElement.
type ProviderDetail struct {
	Provider     string `json:"Provider,omitempty"`
	Status       string `json:"Status,omitempty"`
	ErrorMessage string `json:"ErrorMessage,omitempty"`
}

// elementResponse is the {"Response":{...}} envelope shared by the three
// management actions; unused fields per action stay zero.
type elementResponse struct {
	Response struct {
		// Create
		JobId string `json:"JobId,omitempty"`
		// Common
		ElementId string   `json:"ElementId,omitempty"`
		Status    string   `json:"Status,omitempty"`
		Provider  []string `json:"Provider,omitempty"`
		CreatedAt string   `json:"CreatedAt,omitempty"`
		// Describe
		Name             string            `json:"Name,omitempty"`
		Description      string            `json:"Description,omitempty"`
		ReferenceType    string            `json:"ReferenceType,omitempty"`
		ElementImageList *ElementImageList `json:"ElementImageList,omitempty"`
		VideoList        []string          `json:"VideoList,omitempty"`
		TagList          []TagItem         `json:"TagList,omitempty"`
		ProviderDetails  []ProviderDetail  `json:"ProviderDetails,omitempty"`
		UpdatedAt        string            `json:"UpdatedAt,omitempty"`
		// Delete
		Deleted bool `json:"Deleted,omitempty"`

		RequestId string        `json:"RequestId,omitempty"`
		Error     *tencentError `json:"Error,omitempty"`
	} `json:"Response"`
}

// ElementResult is the normalized view returned to callers, flattening the
// Tencent envelope so controllers don't depend on VCLM's wire shape.
type ElementResult struct {
	JobId            string            `json:"job_id,omitempty"`
	ElementId        string            `json:"element_id,omitempty"`
	Status           string            `json:"status,omitempty"`
	Name             string            `json:"name,omitempty"`
	Description      string            `json:"description,omitempty"`
	ReferenceType    string            `json:"reference_type,omitempty"`
	Provider         []string          `json:"provider,omitempty"`
	ElementImageList *ElementImageList `json:"element_image_list,omitempty"`
	VideoList        []string          `json:"video_list,omitempty"`
	TagList          []TagItem         `json:"tag_list,omitempty"`
	ProviderDetails  []ProviderDetail  `json:"provider_details,omitempty"`
	Deleted          bool              `json:"deleted,omitempty"`
	CreatedAt        string            `json:"created_at,omitempty"`
	UpdatedAt        string            `json:"updated_at,omitempty"`
	RequestId        string            `json:"request_id,omitempty"`
}

// ElementClient performs signed VCLM management calls. baseURL is optional and
// falls back to the default VCLM host; region falls back to ap-guangzhou.
type ElementClient struct {
	apiKey  string // "SecretId|SecretKey"
	baseURL string
	region  string
	proxy   string
}

// NewElementClient builds a client from a channel key. baseURL/region may be
// empty to use defaults.
func NewElementClient(apiKey, baseURL, region, proxy string) *ElementClient {
	if region == "" {
		region = tcDefaultRegion
	}
	return &ElementClient{apiKey: apiKey, baseURL: baseURL, region: region, proxy: proxy}
}

func (c *ElementClient) host() string {
	a := &TaskAdaptor{baseURL: c.baseURL}
	return a.host()
}

// call signs the payload for the given action, sends it, and decodes the
// {"Response":{...}} envelope. It returns an error when Tencent reports an
// Error block so callers see a meaningful message.
func (c *ElementClient) call(action string, payload []byte) (*elementResponse, error) {
	secretId, secretKey, err := splitCredential(c.apiKey)
	if err != nil {
		return nil, err
	}

	a := &TaskAdaptor{baseURL: c.baseURL}
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("https://%s/", c.host()), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	a.applyTC3Headers(req, action, payload, secretId, secretKey, c.region)

	client, err := service.GetHttpClientWithProxy(c.proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var out elementResponse
	if err := common.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("unmarshal response failed: %s", string(body))
	}
	if out.Response.Error != nil && out.Response.Error.Code != "" {
		return nil, fmt.Errorf("%s: %s", out.Response.Error.Code, out.Response.Error.Message)
	}
	return &out, nil
}

func toResult(r *elementResponse) *ElementResult {
	resp := r.Response
	return &ElementResult{
		JobId:            resp.JobId,
		ElementId:        resp.ElementId,
		Status:           resp.Status,
		Name:             resp.Name,
		Description:      resp.Description,
		ReferenceType:    resp.ReferenceType,
		Provider:         resp.Provider,
		ElementImageList: resp.ElementImageList,
		VideoList:        resp.VideoList,
		TagList:          resp.TagList,
		ProviderDetails:  resp.ProviderDetails,
		Deleted:          resp.Deleted,
		CreatedAt:        resp.CreatedAt,
		UpdatedAt:        resp.UpdatedAt,
		RequestId:        resp.RequestId,
	}
}

// CreateElement submits a CreateAigcElement task and returns the new ElementId
// plus its initial status.
func (c *ElementClient) CreateElement(reqBody *CreateElementRequest) (*ElementResult, error) {
	if len(reqBody.Provider) == 0 {
		reqBody.Provider = []string{"kling"}
	}
	payload, err := common.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	resp, err := c.call(ActionCreateElement, payload)
	if err != nil {
		return nil, err
	}
	return toResult(resp), nil
}

// DescribeElement queries the creation progress / detail of an element.
func (c *ElementClient) DescribeElement(elementId string) (*ElementResult, error) {
	payload, err := common.Marshal(map[string]string{"ElementId": elementId})
	if err != nil {
		return nil, err
	}
	resp, err := c.call(ActionDescribeElement, payload)
	if err != nil {
		return nil, err
	}
	return toResult(resp), nil
}

// DeleteElement deletes an element by id.
func (c *ElementClient) DeleteElement(elementId string) (*ElementResult, error) {
	payload, err := common.Marshal(map[string]string{"ElementId": elementId})
	if err != nil {
		return nil, err
	}
	resp, err := c.call(ActionDeleteElement, payload)
	if err != nil {
		return nil, err
	}
	return toResult(resp), nil
}
