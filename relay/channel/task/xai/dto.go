package xai

type submitResponse struct {
	RequestID string `json:"request_id"`
}

type videoResponse struct {
	Status string       `json:"status"`
	Video  *videoOutput `json:"video,omitempty"`
	Model  string       `json:"model,omitempty"`
	Error  *videoError  `json:"error,omitempty"`
}

type videoOutput struct {
	URL               string  `json:"url"`
	Duration          float64 `json:"duration"`
	RespectModeration bool    `json:"respect_moderation"`
}

type videoError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}
