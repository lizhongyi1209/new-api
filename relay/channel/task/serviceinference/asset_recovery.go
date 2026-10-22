package serviceinference

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
)

func (a *TaskAdaptor) captureSeedanceAssetRecovery(c *gin.Context, info *relaycommon.RelayInfo, body *requestPayload, originalImages []string, data []byte) {
	_, basePath, direct := seedanceDirectAssetWorkflow(body.Model)
	if (!direct && !usesSeedanceV2(body.Model)) || len(data) > 1<<20 {
		return
	}
	var ids []string
	for _, item := range body.Content {
		for _, media := range []*MediaURL{item.ImageURL, item.VideoURL, item.AudioURL} {
			if media == nil {
				continue
			}
			if strings.HasPrefix(media.URL, "data:") {
				return
			}
			if strings.HasPrefix(media.URL, "asset://") {
				ids = append(ids, strings.TrimPrefix(media.URL, "asset://"))
			}
		}
	}
	if len(ids) == 0 {
		return
	}
	sources, err := model.FindSeedanceAssetSources(c.Request.Context(), info.UserId, ids)
	if err != nil {
		common.SysError("seedance asset recovery: failed to load creation sources")
		return
	}
	// Images converted by this submission already have a known original URL.
	for i, originalURL := range originalImages {
		if !direct || (!strings.HasPrefix(originalURL, "https://") && !strings.HasPrefix(originalURL, "http://")) {
			continue
		}
		item := body.Content[i]
		if item.ImageURL == nil || !strings.HasPrefix(item.ImageURL.URL, "asset://") {
			continue
		}
		name := strings.TrimSpace(item.Role)
		if name == "" {
			name = "reference-image"
		}
		creation, marshalErr := common.Marshal(map[string]string{"URL": originalURL, "AssetType": "Image", "Name": name})
		if marshalErr != nil {
			return
		}
		sources[strings.TrimPrefix(item.ImageURL.URL, "asset://")] = model.SeedanceAssetRecreateRequest{Path: basePath, Body: creation}
	}
	if len(sources) > 0 {
		c.Set(model.SeedanceAssetRecoveryContextKey, &model.SeedanceAssetRecovery{Request: append([]byte(nil), data...), Sources: sources})
	}
}

// MissingSeedanceAsset only recognizes an explicit asset-not-found failure
// whose source was retained for this task. Other failures are never recovered.
func (a *TaskAdaptor) MissingSeedanceAsset(task *model.Task, reason string) bool {
	recovery := task.PrivateData.SeedanceAssetRecovery
	if recovery == nil {
		return false
	}
	lower := strings.ToLower(reason)
	if !strings.Contains(lower, "asset") || !strings.Contains(lower, "not found") {
		return false
	}
	_, ok := recovery.Sources[extractAssetIDFromMessage(reason)]
	return ok
}

// RecreateMissingSeedanceAsset replaces only the missing reference and submits
// on the original channel. submitted marks the point after which a transport
// error is ambiguous and must not cause another blind generation submission.
func (a *TaskAdaptor) RecreateMissingSeedanceAsset(ctx context.Context, task *model.Task, ch *model.Channel, reason string) (upstreamID string, responseBody []byte, submitted bool, err error) {
	recovery := task.PrivateData.SeedanceAssetRecovery
	missingID := extractAssetIDFromMessage(reason)
	source, ok := recovery.Sources[missingID]
	if !ok || (source.Path != "/v1/sd/assets" && source.Path != "/v1/sd-5/assets" && source.Path != "/v2/db-sd-max/assets") {
		return "", nil, false, fmt.Errorf("asset creation source unavailable")
	}
	var payload map[string]json.RawMessage
	var content []ContentItem
	if common.Unmarshal(recovery.Request, &payload) != nil || common.Unmarshal(payload["content"], &content) != nil {
		return "", nil, false, fmt.Errorf("asset recovery request unavailable")
	}
	referenced := false
	for _, item := range content {
		for _, media := range []*MediaURL{item.ImageURL, item.VideoURL, item.AudioURL} {
			if media != nil && media.URL == "asset://"+missingID {
				referenced = true
			}
		}
	}
	if !referenced {
		return "", nil, false, fmt.Errorf("missing asset is not referenced by the request")
	}
	key := ch.Key
	if task.PrivateData.Key != "" {
		key = task.PrivateData.Key
	}
	worker := &TaskAdaptor{}
	baseURL := ch.GetBaseURL()
	if baseURL == "" {
		baseURL = constant.ChannelBaseURLs[ch.Type]
	}
	worker.Init(&relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ChannelType: ch.Type, ChannelId: ch.Id, ChannelBaseUrl: baseURL, ApiKey: key,
		ChannelSetting: ch.GetSetting(), ChannelOtherSettings: ch.GetOtherSettings(),
	}})
	assetID, createErr := worker.ensureDirectAsset(ctx, "recovery", source.Path, source.Body)
	if createErr != nil {
		// Provider bodies and source URLs must not escape into logs or errors.
		return "", nil, false, fmt.Errorf("asset recreation failed")
	}
	for _, item := range content {
		for _, media := range []*MediaURL{item.ImageURL, item.VideoURL, item.AudioURL} {
			if media != nil && media.URL == "asset://"+missingID {
				media.URL = "asset://" + assetID
			}
		}
	}
	payload["content"], err = common.Marshal(content)
	if err != nil {
		return "", nil, false, err
	}
	recovery.Request, err = common.Marshal(payload)
	if err != nil {
		return "", nil, false, err
	}
	delete(recovery.Sources, missingID)
	recovery.Sources[assetID] = source
	path := "/v1/video/generate"
	if usesSeedanceV2(task.Properties.UpstreamModelName) {
		path = "/v2/video/generate"
	}
	var response taskResponse
	_, responseBody, err = worker.doJSON(ctx, http.MethodPost, path, recovery.Request, &response)
	if err != nil || strings.TrimSpace(response.Task.ID) == "" {
		return "", nil, true, fmt.Errorf("asset recovery resubmission failed")
	}
	return response.Task.ID, responseBody, true, nil
}
