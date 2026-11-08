package model

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
)

func serviceInferencePlatform() constant.TaskPlatform {
	return constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeServiceInferenceVideo))
}

// PassesThroughSeedanceVideo reports whether this task is a ServiceInference
// Seedance video whose completed result must stay on the provider URL.
// MiniMax and Grok tasks on the same channel type keep the channel output strategy.
func (t *Task) PassesThroughSeedanceVideo() bool {
	if t == nil || t.Platform != serviceInferencePlatform() {
		return false
	}
	if seedanceVideoModel(t.Properties.OriginModelName) || seedanceVideoModel(t.Properties.UpstreamModelName) {
		return true
	}
	return seedanceVideoModel(modelNameFromTaskData(t.Data))
}

func seedanceVideoModel(name string) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(name)), "seedance")
}

func modelNameFromTaskData(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	var payload map[string]any
	if err := common.Unmarshal(data, &payload); err != nil {
		return ""
	}
	taskPayload := seedanceTaskObject(payload)
	if name, ok := taskPayload["model"].(string); ok && strings.TrimSpace(name) != "" {
		return name
	}
	if metadata, ok := taskPayload["metadata"].(map[string]any); ok {
		if name, ok := metadata["model"].(string); ok {
			return name
		}
	}
	return ""
}

// seedanceProviderVideoURL reads the upstream video URL from a Seedance payload.
// Selection follows the poll parser: outputs, then content, then metadata.video,
// then metadata.content. An http(s) URL is required so a data URL is left for
// the existing proxy path.
func (t *Task) seedanceProviderVideoURL() string {
	if t == nil || !t.PassesThroughSeedanceVideo() || len(t.Data) == 0 {
		return ""
	}
	var payload map[string]any
	if err := common.Unmarshal(t.Data, &payload); err != nil {
		return ""
	}
	taskPayload := seedanceTaskObject(payload)
	if rawURL := httpMediaURL(firstOutputValue(taskPayload["outputs"])); rawURL != "" {
		return rawURL
	}
	if rawURL := httpMediaURL(taskPayload["content"]); rawURL != "" {
		return rawURL
	}
	metadata, _ := taskPayload["metadata"].(map[string]any)
	if metadata == nil {
		return ""
	}
	if video, ok := metadata["video"].(map[string]any); ok {
		if rawURL := httpMediaURL(video["url"]); rawURL != "" {
			return rawURL
		}
	}
	return httpMediaURL(metadata["content"])
}

func seedanceTaskObject(payload map[string]any) map[string]any {
	if taskPayload, ok := payload["task"].(map[string]any); ok {
		return taskPayload
	}
	return payload
}

func firstOutputValue(value any) any {
	items, ok := value.([]any)
	if !ok || len(items) == 0 {
		return nil
	}
	return items[0]
}

func httpMediaURL(value any) string {
	switch typed := value.(type) {
	case string:
		return directHTTPURL(typed)
	case map[string]any:
		if rawURL := httpMediaURL(typed["url"]); rawURL != "" {
			return rawURL
		}
		return httpMediaURL(typed["video_url"])
	default:
		return ""
	}
}

func directHTTPURL(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return value
	}
	return ""
}
