package hailuo

const (
	ChannelName = "hailuo-video"
)

var ModelList = []string{
	"MiniMax-H3",
	"MiniMax-H3-Max",
	"MiniMax-Hailuo-2.3",
	"MiniMax-Hailuo-2.3-Fast",
	"MiniMax-Hailuo-02",
	"T2V-01-Director",
	"T2V-01",
	"I2V-01-Director",
	"I2V-01-live",
	"I2V-01",
	"S2V-01",
}

const (
	TextToVideoEndpoint       = "/v1/video_generation"
	QueryTaskEndpoint         = "/v1/query/video_generation"
	VideoGenerationV2Endpoint = "/v2/video_generation"
	QueryTaskV2Endpoint       = "/v2/query/video_generation"
	MiniMaxV2BaseURL          = "https://api.minimaxi.com"
	MiniMaxLegacyBaseURL      = "https://api.minimax.chat"
)

const (
	StatusSuccess    = 0
	StatusRateLimit  = 1002
	StatusAuthFailed = 1004
	StatusNoBalance  = 1008
	StatusSensitive  = 1026
	StatusParamError = 2013
	StatusInvalidKey = 2049
)

const (
	TaskStatusPreparing  = "Preparing"
	TaskStatusQueueing   = "Queueing"
	TaskStatusProcessing = "Processing"
	TaskStatusSuccess    = "Success"
	TaskStatusFailed     = "Fail"
)

const (
	Resolution480P  = "480P"
	Resolution512P  = "512P"
	Resolution720P  = "720P"
	Resolution768P  = "768P"
	Resolution1080P = "1080P"
	Resolution2K    = "2K"
)

const (
	DefaultDuration   = 6
	DefaultResolution = Resolution720P
)
