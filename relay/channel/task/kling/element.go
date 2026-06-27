package kling

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/service"
	"github.com/pkg/errors"
)

// ============================
// Element Management Structures
// ============================

// Reference types for elements
const (
	ReferenceTypeVideo = "video_refer" // Video Character Elements
	ReferenceTypeImage = "image_refer" // Multi-Image Elements
)

// Tag IDs for element classification
const (
	TagHottest   = "o_101"
	TagCharacter = "o_102"
	TagAnimal    = "o_103"
	TagItemID    = "o_104"
	TagCostume   = "o_105"
	TagScene     = "o_106"
	TagEffect    = "o_107"
	TagOthers    = "o_108"
)

// ElementImageList represents the frontal and reference images for multi-image elements
type ElementImageList struct {
	FrontalImage string            `json:"frontal_image"`
	ReferImages  []ReferImageItem `json:"refer_images,omitempty"`
}

type ReferImageItem struct {
	ImageUrl string `json:"image_url"`
}

// ElementVideoList represents reference videos for video character elements
type ElementVideoList struct {
	ReferVideos []ReferVideoItem `json:"refer_videos"`
}

type ReferVideoItem struct {
	VideoUrl string `json:"video_url"`
}

// ElementVoiceInfo represents voice binding information
type ElementVoiceInfo struct {
	VoiceId  string `json:"voice_id,omitempty"`
	VoiceName string `json:"voice_name,omitempty"`
	TrialUrl string `json:"trial_url,omitempty"`
	OwnedBy  string `json:"owned_by,omitempty"`
}

// TagItem represents an element tag
type TagItem struct {
	TagId string `json:"tag_id"`
}

// CreateElementRequest represents the request to create a custom element
type CreateElementRequest struct {
	ElementName        string            `json:"element_name"`
	ElementDescription string            `json:"element_description"`
	ReferenceType      string            `json:"reference_type"`
	ElementImageList   *ElementImageList `json:"element_image_list,omitempty"`
	ElementVideoList   *ElementVideoList `json:"element_video_list,omitempty"`
	ElementVoiceId     string            `json:"element_voice_id,omitempty"`
	TagList            []TagItem         `json:"tag_list,omitempty"`
	CallbackUrl        string            `json:"callback_url,omitempty"`
	ExternalTaskId     string            `json:"external_task_id,omitempty"`
}

// ElementResult represents a single element in the response
type ElementResult struct {
	ElementId          int64             `json:"element_id"`
	ElementName        string            `json:"element_name"`
	ElementDescription string            `json:"element_description"`
	ReferenceType      string            `json:"reference_type"`
	ElementImageList   *ElementImageList `json:"element_image_list,omitempty"`
	ElementVideoList   *ElementVideoList `json:"element_video_list,omitempty"`
	ElementVoiceInfo   *ElementVoiceInfo `json:"element_voice_info,omitempty"`
	TagList            []TagItem         `json:"tag_list,omitempty"`
	OwnedBy            string            `json:"owned_by"`
	Status             string            `json:"status"`
}

// CreateElementResponse represents the response from creating an element
type CreateElementResponse struct {
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

// QueryElementResponse represents the response from querying an element
type QueryElementResponse struct {
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
			Elements []ElementResult `json:"elements"`
		} `json:"task_result"`
		FinalUnitDeduction string `json:"final_unit_deduction"`
		CreatedAt          int64  `json:"created_at"`
		UpdatedAt          int64  `json:"updated_at"`
	} `json:"data"`
}

// ListElementsResponse represents the response from listing elements
type ListElementsResponse struct {
	Code      int                    `json:"code"`
	Message   string                 `json:"message"`
	RequestId string                 `json:"request_id"`
	Data      []QueryElementResponse `json:"data"`
}

// DeleteElementRequest represents the request to delete an element
type DeleteElementRequest struct {
	ElementId string `json:"element_id"`
}

// DeleteElementResponse represents the response from deleting an element
type DeleteElementResponse struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	RequestId string `json:"request_id"`
}

// ============================
// Element Client
// ============================

// ElementClient handles element management API calls
type ElementClient struct {
	apiKey  string
	baseURL string
	proxy   string
}

// NewElementClient creates a new element management client
func NewElementClient(apiKey, baseURL, proxy string) *ElementClient {
	if baseURL == "" {
		baseURL = "https://api.klingai.com"
	}
	return &ElementClient{
		apiKey:  apiKey,
		baseURL: baseURL,
		proxy:   proxy,
	}
}

// createAuthToken generates the authentication token
func (c *ElementClient) createAuthToken() (string, error) {
	adaptor := &TaskAdaptor{apiKey: c.apiKey}
	return adaptor.createJWTTokenWithKey(c.apiKey)
}

// doRequest performs an HTTP request with authentication
func (c *ElementClient) doRequest(method, path string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
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
func (c *ElementClient) CreateElement(req *CreateElementRequest) (*CreateElementResponse, error) {
	resp, err := c.doRequest("POST", "/v1/general/advanced-custom-elements", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "read response body failed")
	}

	var result CreateElementResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, errors.Wrap(err, "unmarshal response failed")
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("create element failed: %s (code: %d)", result.Message, result.Code)
	}

	return &result, nil
}

// QueryElement queries a single element by task ID
func (c *ElementClient) QueryElement(taskId string) (*QueryElementResponse, error) {
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

	var result QueryElementResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, errors.Wrap(err, "unmarshal response failed")
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("query element failed: %s (code: %d)", result.Message, result.Code)
	}

	return &result, nil
}

// ListElements lists custom elements with pagination
func (c *ElementClient) ListElements(pageNum, pageSize int) (*ListElementsResponse, error) {
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

	var result ListElementsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, errors.Wrap(err, "unmarshal response failed")
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("list elements failed: %s (code: %d)", result.Message, result.Code)
	}

	return &result, nil
}

// ListPresetsElements lists preset elements from official library
func (c *ElementClient) ListPresetsElements(pageNum, pageSize int) (*ListElementsResponse, error) {
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

	var result ListElementsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, errors.Wrap(err, "unmarshal response failed")
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("list preset elements failed: %s (code: %d)", result.Message, result.Code)
	}

	return &result, nil
}

// DeleteElement deletes a custom element
func (c *ElementClient) DeleteElement(elementId string) (*DeleteElementResponse, error) {
	req := &DeleteElementRequest{ElementId: elementId}
	resp, err := c.doRequest("POST", "/v1/general/delete-elements", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "read response body failed")
	}

	var result DeleteElementResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, errors.Wrap(err, "unmarshal response failed")
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("delete element failed: %s (code: %d)", result.Message, result.Code)
	}

	return &result, nil
}

// ValidateElementImageRefs validates image references for multi-image elements
func ValidateElementImageRefs(frontalImage string, referImages []string) error {
	if strings.TrimSpace(frontalImage) == "" {
		return errors.New("frontal_image is required")
	}

	if len(referImages) < 1 || len(referImages) > 3 {
		return fmt.Errorf("refer_images must have 1-3 images, got %d", len(referImages))
	}

	// Validate all images
	allImages := append([]string{frontalImage}, referImages...)
	for _, imageUrl := range allImages {
		if strings.TrimSpace(imageUrl) == "" {
			continue
		}
		// TODO: Add image validation (format, size, etc.)
	}

	return nil
}

// ValidateElementVideoRefs validates video references for video character elements
func ValidateElementVideoRefs(videoList []string) error {
	if len(videoList) != 1 {
		return errors.New("element_video_list must contain exactly 1 video")
	}

	if strings.TrimSpace(videoList[0]) == "" {
		return errors.New("video_url cannot be empty")
	}

	// TODO: Add video validation (format, duration, size, etc.)

	return nil
}
