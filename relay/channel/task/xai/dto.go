package xai

type submitResponse struct {
	RequestID string `json:"request_id"`
}

type videoResponse struct {
	Status   string       `json:"status"`
	Video    *videoOutput `json:"video,omitempty"`
	Model    string       `json:"model,omitempty"`
	Error    *videoError  `json:"error,omitempty"`
	Usage    *videoUsage  `json:"usage,omitempty"`
	Progress *int         `json:"progress,omitempty"`
}

type videoOutput struct {
	URL               string           `json:"url"`
	Duration          float64          `json:"duration"`
	RespectModeration bool             `json:"respect_moderation"`
	FileOutput        *videoFileOutput `json:"file_output,omitempty"`
	StorageError      *string          `json:"storage_error,omitempty"`
}

type videoFileOutput struct {
	ExpiresAt          *int64  `json:"expires_at,omitempty"`
	FileID             string  `json:"file_id"`
	Filename           string  `json:"filename"`
	PublicURL          *string `json:"public_url,omitempty"`
	PublicURLError     *string `json:"public_url_error,omitempty"`
	PublicURLExpiresAt *int64  `json:"public_url_expires_at,omitempty"`
}

type videoUsage struct {
	CostInUSDTicks      int64                    `json:"cost_in_usd_ticks"`
	InputTokens         *int                     `json:"input_tokens,omitempty"`
	InputTokensDetails  *videoInputTokenDetails  `json:"input_tokens_details,omitempty"`
	OutputTokens        *int                     `json:"output_tokens,omitempty"`
	OutputTokensDetails *videoOutputTokenDetails `json:"output_tokens_details,omitempty"`
	TotalTokens         *int                     `json:"total_tokens,omitempty"`
}

type videoInputTokenDetails struct {
	CachedTokens int `json:"cached_tokens"`
	ImageTokens  int `json:"image_tokens"`
	TextTokens   int `json:"text_tokens"`
}

type videoOutputTokenDetails struct {
	ImageTokens     int `json:"image_tokens"`
	ReasoningTokens int `json:"reasoning_tokens"`
	TextTokens      int `json:"text_tokens"`
}

type videoError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}
