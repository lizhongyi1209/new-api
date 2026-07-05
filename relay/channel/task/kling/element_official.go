package kling

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/pkg/errors"
)

// ============================
// Kling Official Element Management (2024 New Version)
// ============================

// Reference types for Kling official elements
const (
	OfficialReferenceTypeVideo = "video_refer" // Video Character Elements
	OfficialReferenceTypeImage = "image_refer" // Multi-Image Elements
)

// Tag IDs for official element classification
const (
	OfficialTagHottest   = "o_101"
	OfficialTagCharacter = "o_102"
	OfficialTagAnimal    = "o_103"
	OfficialTagItemID    = "o_104"
	OfficialTagCostume   = "o_105"
	OfficialTagScene     = "o_106"
	OfficialTagEffect    = "o_107"
	OfficialTagOthers    = "o_108"
)

// OfficialElementImageList represents frontal and reference images for multi-image elements
type OfficialElementImageList struct {
	FrontalImage string                   `json:"frontal_image"`
	ReferImages  []OfficialReferImageItem `json:"refer_images,omitempty"`
}

type OfficialReferImageItem struct {
	ImageUrl string `json:"image_url"`
}

// OfficialElementVideoList represents reference videos for video character elements
type OfficialElementVideoList struct {
	ReferVideos []OfficialReferVideoItem `json:"refer_videos"`
}

type OfficialReferVideoItem struct {
	VideoUrl string `json:"video_url"`
}

// OfficialElementVoiceInfo represents voice binding information
type OfficialElementVoiceInfo struct {
	VoiceId   string `json:"voice_id,omitempty"`
	VoiceName string `json:"voice_name,omitempty"`
	TrialUrl  string `json:"trial_url,omitempty"`
	OwnedBy   string `json:"owned_by,omitempty"`
}

// OfficialTagItem represents an element tag
type OfficialTagItem struct {
	TagId string `json:"tag_id"`
}

// CreateOfficialElementRequest represents the request to create a custom element
type CreateOfficialElementRequest struct {
	ElementName        string                        `json:"element_name"`
	ElementDescription string                        `json:"element_description"`
	ReferenceType      string                        `json:"reference_type"`
	ElementImageList   *OfficialElementImageList     `json:"element_image_list,omitempty"`
	ElementVideoList   *OfficialElementVideoList     `json:"element_video_list,omitempty"`
	ElementVoiceId     string                        `json:"element_voice_id,omitempty"`
	TagList            []OfficialTagItem             `json:"tag_list,omitempty"`
	CallbackUrl        string                        `json:"callback_url,omitempty"`
	ExternalTaskId     string                        `json:"external_task_id,omitempty"`
}

// OfficialElementResult represents a single element in the response
type OfficialElementResult struct {
	ElementId          int64                         `json:"element_id"`
	ElementName        string                        `json:"element_name"`
	ElementDescription string                        `json:"element_description"`
	ReferenceType      string                        `json:"reference_type"`
	ElementImageList   *OfficialElementImageList     `json:"element_image_list,omitempty"`
	ElementVideoList   *OfficialElementVideoList     `json:"element_video_list,omitempty"`
	ElementVoiceInfo   *OfficialElementVoiceInfo     `json:"element_voice_info,omitempty"`
	TagList            []OfficialTagItem             `json:"tag_list,omitempty"`
	OwnedBy            string                        `json:"owned_by"`
	Status             string                        `json:"status"`
}

// CreateOfficialElementResponse represents the response from creating an element
type CreateOfficialElementResponse struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	RequestId string `json:"request_id"`
	Data      struct {
		TaskId     string `json:"task_id"`
		TaskInfo   struct {
			ExternalTaskId string `json:"external_task_id"`
		} `json:"task_info"`
		TaskStatus string `json:"task_status"`
		CreatedAt  int64  `json:"created_at"`
		UpdatedAt  int64  `json:"updated_at"`
	} `json:"data"`
}

// QueryOfficialElementResponse represents the response from querying an element
type QueryOfficialElementResponse struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	RequestId string `json:"request_id"`
	Data      struct {
		TaskId        string `json:"task_id"`
		TaskStatus    string `json:"task_status"`
		TaskStatusMsg string `json:"task_status_msg"`
		TaskInfo      struct {
			ExternalTaskId string `json:"external_task_id"`
		} `json:"task_info"`
		TaskResult struct {
			Elements []OfficialElementResult `json:"elements"`
		} `json:"task_result"`
		FinalUnitDeduction string `json:"final_unit_deduction"`
		FinalBalanceDeduction struct {
			Quota string `json:"quota"`
		} `json:"final_balance_deduction"`
		CreatedAt int64 `json:"created_at"`
		UpdatedAt int64 `json:"updated_at"`
	} `json:"data"`
}

// ListOfficialElementsResponse represents the response from listing elements
type ListOfficialElementsResponse struct {
	Code      int                            `json:"code"`
	Message   string                         `json:"message"`
	RequestId string                         `json:"request_id"`
	Data      []QueryOfficialElementResponse `json:"data"`
}

// DeleteOfficialElementRequest represents the request to delete an element
type DeleteOfficialElementRequest struct {
	ElementId string `json:"element_id"`
}

// DeleteOfficialElementResponse represents the response from deleting an element
type DeleteOfficialElementResponse struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	RequestId string `json:"request_id"`
	Data      struct {
		TaskId     string `json:"task_id"`
		TaskStatus string `json:"task_status"`
	} `json:"data"`
}

// ============================
// Official Element Client
// ============================

// OfficialElementClient handles Kling official element management API calls
type OfficialElementClient struct {
	apiKey  string
	baseURL string
	proxy   string
}

// NewOfficialElementClient creates a new official element management client
func NewOfficialElementClient(apiKey, baseURL, proxy string) *OfficialElementClient {
	if baseURL == "" {
		baseURL = "https://api.klingai.com"
	}
	return &OfficialElementClient{
		apiKey:  apiKey,
		baseURL: baseURL,
		proxy:   proxy,
	}
}

// createAuthToken generates the authentication token
func (c *OfficialElementClient) createAuthToken() (string, error) {
	adaptor := &TaskAdaptor{apiKey: c.apiKey}
	return adaptor.createJWTTokenWithKey(c.apiKey)
}

// doRequest performs an HTTP request with authentication
func (c *OfficialElementClient) doRequest(method, path string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := common.Marshal(body)
		if err != nil {
			return nil, errors.Wrap(err, "marshal request body failed")
		}
		reqBody = bytes.NewReader(data)
	}

	url := c.baseURL + path
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, errors.Wrap(err, "create request failed")
	}

	token, err := c.createAuthToken()
	if err != nil {
		return nil, errors.Wrap(err, "create auth token failed")
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "kling-sdk/1.0")

	client, err := service.GetHttpClientWithProxy(c.proxy)
	if err != nil {
		return nil, errors.Wrap(err, "get http client failed")
	}

	return client.Do(req)
}

// CreateElement creates a new custom element
func (c *OfficialElementClient) CreateElement(req *CreateOfficialElementRequest) (*CreateOfficialElementResponse, error) {
	resp, err := c.doRequest("POST", "/v1/general/advanced-custom-elements", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "read response body failed")
	}

	var result CreateOfficialElementResponse
	if err := common.Unmarshal(body, &result); err != nil {
		return nil, errors.Wrap(err, "unmarshal response failed")
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("create element failed: %s (code: %d)", result.Message, result.Code)
	}

	return &result, nil
}

// QueryElement queries a single element by task ID
func (c *OfficialElementClient) QueryElement(taskId string) (*QueryOfficialElementResponse, error) {
	path := fmt.Sprintf("/v1/general/advanced-custom-elements/%s", taskId)
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "read response body failed")
	}

	var result QueryOfficialElementResponse
	if err := common.Unmarshal(body, &result); err != nil {
		return nil, errors.Wrap(err, "unmarshal response failed")
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("query element failed: %s (code: %d)", result.Message, result.Code)
	}

	return &result, nil
}

// ListElements lists custom elements with pagination
func (c *OfficialElementClient) ListElements(pageNum, pageSize int) (*ListOfficialElementsResponse, error) {
	path := fmt.Sprintf("/v1/general/advanced-custom-elements?pageNum=%d&pageSize=%d", pageNum, pageSize)
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "read response body failed")
	}

	var result ListOfficialElementsResponse
	if err := common.Unmarshal(body, &result); err != nil {
		return nil, errors.Wrap(err, "unmarshal response failed")
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("list elements failed: %s (code: %d)", result.Message, result.Code)
	}

	return &result, nil
}

// ListPresetsElements lists preset elements from official library
func (c *OfficialElementClient) ListPresetsElements(pageNum, pageSize int) (*ListOfficialElementsResponse, error) {
	path := fmt.Sprintf("/v1/general/advanced-presets-elements?pageNum=%d&pageSize=%d", pageNum, pageSize)
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "read response body failed")
	}

	var result ListOfficialElementsResponse
	if err := common.Unmarshal(body, &result); err != nil {
		return nil, errors.Wrap(err, "unmarshal response failed")
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("list preset elements failed: %s (code: %d)", result.Message, result.Code)
	}

	return &result, nil
}

// DeleteElement deletes a custom element
func (c *OfficialElementClient) DeleteElement(elementId string) (*DeleteOfficialElementResponse, error) {
	req := &DeleteOfficialElementRequest{ElementId: elementId}
	resp, err := c.doRequest("POST", "/v1/general/delete-advanced-elements", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "read response body failed")
	}

	var result DeleteOfficialElementResponse
	if err := common.Unmarshal(body, &result); err != nil {
		return nil, errors.Wrap(err, "unmarshal response failed")
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("delete element failed: %s (code: %d)", result.Message, result.Code)
	}

	return &result, nil
}

// ValidateOfficialElementImageRefs validates image references for multi-image elements
func ValidateOfficialElementImageRefs(frontalImage string, referImages []string) error {
	if strings.TrimSpace(frontalImage) == "" {
		return errors.New("frontal_image is required")
	}

	if len(referImages) < 1 || len(referImages) > 3 {
		return fmt.Errorf("refer_images must have 1-3 images, got %d", len(referImages))
	}

	return nil
}

// ValidateOfficialElementVideoRefs validates video references for video character elements
func ValidateOfficialElementVideoRefs(videoList []string) error {
	if len(videoList) != 1 {
		return errors.New("element_video_list must contain exactly 1 video")
	}

	if strings.TrimSpace(videoList[0]) == "" {
		return errors.New("video_url cannot be empty")
	}

	return nil
}
