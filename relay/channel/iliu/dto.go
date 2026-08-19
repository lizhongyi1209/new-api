package iliu

import "encoding/json"

type AccountFilter struct {
	Modes      []string `json:"modes,omitempty"`
	ChannelID  *string  `json:"channelId,omitempty"`
	InstanceID *string  `json:"instanceId,omitempty"`
	Remark     *string  `json:"remark,omitempty"`
}

type CommonOptions struct {
	Mode          *string        `json:"mode,omitempty"`
	BotType       *string        `json:"botType,omitempty"`
	AccountFilter *AccountFilter `json:"accountFilter,omitempty"`
	NotifyHook    *string        `json:"notifyHook,omitempty"`
	State         *string        `json:"state,omitempty"`
}

type ImagineRequest struct {
	CommonOptions
	Prompt      string   `json:"prompt"`
	Base64Array []string `json:"base64Array,omitempty"`
}

type ActionRequest struct {
	CustomID          string         `json:"customId"`
	TaskID            string         `json:"taskId"`
	ChooseSameChannel *bool          `json:"chooseSameChannel,omitempty"`
	AccountFilter     *AccountFilter `json:"accountFilter,omitempty"`
	NotifyHook        *string        `json:"notifyHook,omitempty"`
	State             *string        `json:"state,omitempty"`
}

type BlendRequest struct {
	CommonOptions
	Base64Array []string `json:"base64Array"`
	Dimensions  *string  `json:"dimensions,omitempty"`
}

type ModalRequest struct {
	TaskID     string  `json:"taskId"`
	Prompt     *string `json:"prompt,omitempty"`
	MaskBase64 *string `json:"maskBase64,omitempty"`
}

type DescribeRequest struct {
	CommonOptions
	Base64 string `json:"base64"`
}

type ShortenRequest struct {
	CommonOptions
	Prompt string `json:"prompt"`
}

type FaceSwapRequest struct {
	CommonOptions
	SourceBase64 string `json:"sourceBase64"`
	TargetBase64 string `json:"targetBase64"`
}

type UploadRequest struct {
	Base64Array []string       `json:"base64Array"`
	Filter      *AccountFilter `json:"filter,omitempty"`
}

type EditsRequest struct {
	Prompt      string `json:"prompt"`
	ImageBase64 string `json:"imageBase64"`
}

type VideoRequest struct {
	Prompt   string  `json:"prompt"`
	Motion   *string `json:"motion,omitempty"`
	Image    string  `json:"image"`
	EndImage *string `json:"endImage,omitempty"`
	Loop     *bool   `json:"loop,omitempty"`
}

type SubmitResponse struct {
	Code        int             `json:"code"`
	Description string          `json:"description"`
	Result      json.RawMessage `json:"result"`
	Properties  json.RawMessage `json:"properties"`
}

type TaskResponse struct {
	ID         string          `json:"id"`
	Action     string          `json:"action"`
	Status     string          `json:"status"`
	Progress   string          `json:"progress"`
	ImageURL   string          `json:"imageUrl"`
	VideoURL   string          `json:"videoUrl"`
	FailReason string          `json:"failReason"`
	State      string          `json:"state"`
	SubmitTime int64           `json:"submitTime"`
	StartTime  int64           `json:"startTime"`
	FinishTime int64           `json:"finishTime"`
	Buttons    json.RawMessage `json:"buttons"`
	Properties json.RawMessage `json:"properties"`
}
