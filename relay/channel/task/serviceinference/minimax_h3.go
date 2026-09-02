package serviceinference

import (
	"fmt"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
)

const (
	tokenMartH3MinDuration        = 4
	tokenMartH3MaxMinDuration     = 5
	tokenMartH3MaxDuration        = 15
	tokenMartH3MaxPromptRunes     = 7000
	tokenMartH3MaxImageCount      = 9
	tokenMartH3MaxMixedMediaCount = 12
	tokenMartH3MaxBodyBytes       = 64 << 20
	// Current USD rates returned by TokenMart's authenticated /v1/models
	// endpoint. Completion settlement uses the provider's returned usage.
	tokenMartH3Rate768PUSD    = 0.0624
	tokenMartH3Rate2KUSD      = 0.1014
	tokenMartH3MaxRate480PUSD = 0.039
	tokenMartH3MaxRate768PUSD = 0.0624
	tokenMartH3FreeImageCount = 5
	tokenMartH3ExtraImageUSD  = 0.0312
)

type miniMaxH3InputSummary struct {
	ImageCount          int
	ReferenceImageCount int
	ReferenceAudioCount int
	HasVideo            bool
}

func validateMiniMaxH3Payload(request *requestPayload) (*miniMaxH3InputSummary, error) {
	if request == nil {
		return nil, fmt.Errorf("request is required")
	}
	if !isMiniMaxH3Model(request.Model) {
		return nil, fmt.Errorf("unsupported MiniMax H3 model %s", request.Model)
	}
	if len(request.Content) == 0 {
		return nil, fmt.Errorf("content field is required")
	}

	isMaxModel := isMiniMaxH3MaxModel(request.Model)
	minDuration := tokenMartH3MinDuration
	if isMaxModel {
		minDuration = tokenMartH3MaxMinDuration
	}
	if request.Duration == nil || *request.Duration < minDuration || *request.Duration > tokenMartH3MaxDuration {
		return nil, fmt.Errorf("duration must be between %d and %d", minDuration, tokenMartH3MaxDuration)
	}
	if isMaxModel {
		if request.Resolution != "480P" && request.Resolution != "768P" {
			return nil, fmt.Errorf("resolution must be 480P or 768P for MiniMax-H3-Max")
		}
	} else if request.Resolution != "768P" && request.Resolution != "2K" {
		return nil, fmt.Errorf("resolution must be 768P or 2K for MiniMax-H3")
	}

	validRatios := map[string]bool{
		"adaptive": true,
		"21:9":     true,
		"16:9":     true,
		"4:3":      true,
		"1:1":      true,
		"3:4":      true,
		"9:16":     true,
	}
	if request.Ratio != "" && !validRatios[request.Ratio] {
		return nil, fmt.Errorf("invalid ratio")
	}

	summary := &miniMaxH3InputSummary{}
	textCount := 0
	firstFrameCount := 0
	lastFrameCount := 0
	referenceVideoCount := 0
	frameMode := false
	referenceMode := false

	for i, item := range request.Content {
		switch item.Type {
		case "text":
			text := strings.TrimSpace(item.Text)
			if text == "" {
				return nil, fmt.Errorf("content[%d].text is required", i)
			}
			if len([]rune(text)) > tokenMartH3MaxPromptRunes {
				return nil, fmt.Errorf("content[%d].text exceeds %d characters", i, tokenMartH3MaxPromptRunes)
			}
			textCount++
		case "image_url":
			if item.ImageURL == nil || !validMiniMaxH3MediaURL(item.ImageURL.URL, "image") {
				return nil, fmt.Errorf("content[%d].image_url.url is invalid", i)
			}
			summary.ImageCount++
			role := item.Role
			if role == "" {
				role = "first_frame"
			}
			switch role {
			case "first_frame":
				firstFrameCount++
				frameMode = true
			case "last_frame":
				lastFrameCount++
				frameMode = true
			case "reference_image":
				if isMaxModel {
					return nil, fmt.Errorf("MiniMax-H3-Max does not support multimodal reference inputs")
				}
				summary.ReferenceImageCount++
				referenceMode = true
			default:
				return nil, fmt.Errorf("content[%d].role is invalid for image_url", i)
			}
		case "video_url":
			if isMaxModel {
				return nil, fmt.Errorf("MiniMax-H3-Max does not support video inputs")
			}
			if item.VideoURL == nil || !validMiniMaxH3MediaURL(item.VideoURL.URL, "video") {
				return nil, fmt.Errorf("content[%d].video_url.url is invalid", i)
			}
			if item.Role != "reference_video" {
				return nil, fmt.Errorf("content[%d].role must be reference_video", i)
			}
			referenceVideoCount++
			summary.HasVideo = true
			referenceMode = true
		case "audio_url":
			if isMaxModel {
				return nil, fmt.Errorf("MiniMax-H3-Max does not support audio inputs")
			}
			if item.AudioURL == nil || !validMiniMaxH3MediaURL(item.AudioURL.URL, "audio") {
				return nil, fmt.Errorf("content[%d].audio_url.url is invalid", i)
			}
			if item.Role != "reference_audio" {
				return nil, fmt.Errorf("content[%d].role must be reference_audio", i)
			}
			summary.ReferenceAudioCount++
			referenceMode = true
		default:
			return nil, fmt.Errorf("content[%d].type is invalid", i)
		}
	}

	if textCount == 0 {
		return nil, fmt.Errorf("content must include a non-empty text item")
	}
	if firstFrameCount > 1 || lastFrameCount > 1 {
		return nil, fmt.Errorf("first_frame and last_frame inputs are invalid")
	}
	if summary.ReferenceImageCount > tokenMartH3MaxImageCount {
		return nil, fmt.Errorf("a maximum of %d reference images is supported", tokenMartH3MaxImageCount)
	}
	if referenceVideoCount > 3 {
		return nil, fmt.Errorf("a maximum of 3 reference videos is supported")
	}
	if summary.ReferenceAudioCount > 3 {
		return nil, fmt.Errorf("a maximum of 3 reference audios is supported")
	}
	if summary.ImageCount+referenceVideoCount+summary.ReferenceAudioCount > tokenMartH3MaxMixedMediaCount {
		return nil, fmt.Errorf("a maximum of %d mixed media files is supported", tokenMartH3MaxMixedMediaCount)
	}
	if frameMode && referenceMode {
		return nil, fmt.Errorf("frame inputs and reference inputs are mutually exclusive")
	}
	if !frameMode && !referenceMode && (request.Ratio == "" || request.Ratio == "adaptive") {
		return nil, fmt.Errorf("ratio is required for text-to-video and cannot be adaptive")
	}
	return summary, nil
}

func validMiniMaxH3MediaURL(rawURL string, mediaType string) bool {
	rawURL = strings.TrimSpace(rawURL)
	if strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") || strings.HasPrefix(rawURL, "mm_file://") {
		return true
	}
	switch mediaType {
	case "image":
		for _, format := range []string{"jpg", "jpeg", "png", "webp", "heic", "heif"} {
			if strings.HasPrefix(rawURL, "data:image/"+format+";base64,") {
				return true
			}
		}
	case "video":
		return strings.HasPrefix(rawURL, "data:video/mp4;base64,")
	case "audio":
		return strings.HasPrefix(rawURL, "data:audio/wav;base64,") || strings.HasPrefix(rawURL, "data:audio/mp3;base64,")
	}
	return false
}

func miniMaxH3RateUSD(modelName, resolution string) (float64, bool) {
	if isMiniMaxH3MaxModel(modelName) {
		switch strings.ToUpper(strings.TrimSpace(resolution)) {
		case "480P":
			return tokenMartH3MaxRate480PUSD, true
		case "768P":
			return tokenMartH3MaxRate768PUSD, true
		default:
			return 0, false
		}
	}
	switch strings.ToUpper(strings.TrimSpace(resolution)) {
	case "768P":
		return tokenMartH3Rate768PUSD, true
	case "2K":
		return tokenMartH3Rate2KUSD, true
	default:
		return 0, false
	}
}

func miniMaxH3CostUSD(modelName string, outputSeconds, inputVideoSeconds, inputImageCount int, resolution string) float64 {
	rate, ok := miniMaxH3RateUSD(modelName, resolution)
	if !ok {
		return 0
	}
	if outputSeconds < 0 {
		outputSeconds = 0
	}
	if inputVideoSeconds < 0 {
		inputVideoSeconds = 0
	}
	if inputImageCount < 0 {
		inputImageCount = 0
	}
	cost := float64(outputSeconds) * rate
	if isMiniMaxH3MaxModel(modelName) {
		return cost
	}
	cost += float64(inputVideoSeconds) * rate
	if inputImageCount > tokenMartH3FreeImageCount {
		cost += float64(inputImageCount-tokenMartH3FreeImageCount) * tokenMartH3ExtraImageUSD
	}
	return cost
}

func miniMaxH3BillingDetails(modelName string, outputSeconds, inputVideoSeconds, inputImageCount int, resolution string, groupRatio float64) *relaycommon.VideoBillingDetails {
	if groupRatio <= 0 {
		groupRatio = 1
	}
	rate, _ := miniMaxH3RateUSD(modelName, resolution)
	freeImageCount := tokenMartH3FreeImageCount
	billedImageCount := inputImageCount - freeImageCount
	imageUnitRate := tokenMartH3ExtraImageUSD
	if billedImageCount < 0 {
		billedImageCount = 0
	}
	if isMiniMaxH3MaxModel(modelName) {
		inputVideoSeconds = 0
		freeImageCount = inputImageCount
		billedImageCount = 0
		imageUnitRate = 0
	}
	outputCost := float64(outputSeconds) * rate
	referenceVideoCost := float64(inputVideoSeconds) * rate
	imageSurcharge := float64(billedImageCount) * imageUnitRate
	providerCost := outputCost + referenceVideoCost + imageSurcharge
	return &relaycommon.VideoBillingDetails{
		BillingMode:                "video_per_second",
		Currency:                   "USD",
		Resolution:                 resolution,
		OutputUnitRate:             rate,
		OutputSeconds:              outputSeconds,
		OutputCost:                 outputCost,
		ReferenceVideoInputSeconds: inputVideoSeconds,
		ReferenceVideoCost:         referenceVideoCost,
		ImageCount:                 inputImageCount,
		FreeImageCount:             freeImageCount,
		BilledImageCount:           billedImageCount,
		ImageUnitRate:              imageUnitRate,
		ImageSurcharge:             imageSurcharge,
		ProviderCost:               providerCost,
		GroupRatio:                 groupRatio,
		FinalCost:                  providerCost * groupRatio,
	}
}

func effectiveTaskUsage(task videoTask) *taskUsage {
	if task.Metadata != nil && task.Metadata.Usage != nil {
		return task.Metadata.Usage
	}
	return task.Usage
}

func (a *TaskAdaptor) estimateMiniMaxH3Billing(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	request, err := relaycommon.GetTaskRequest(c)
	if err != nil || info.PriceData.ModelRatio <= 0 {
		return nil
	}
	modelName := info.UpstreamModelName
	if !isMiniMaxH3Model(modelName) {
		modelName = info.OriginModelName
	}
	costUSD := miniMaxH3CostUSD(modelName, request.Duration, 0, request.ImageCount, request.EffectiveResolution)
	if costUSD <= 0 {
		return nil
	}
	baseCostUSD := info.PriceData.ModelRatio / 2
	ratio := costUSD / baseCostUSD
	if ratio <= 0 || math.IsNaN(ratio) || math.IsInf(ratio, 1) {
		return nil
	}
	info.VideoBilling = miniMaxH3BillingDetails(
		modelName,
		request.Duration,
		0,
		request.ImageCount,
		request.EffectiveResolution,
		info.PriceData.GroupRatioInfo.GroupRatio,
	)
	info.VideoBilling.Estimated = true
	return map[string]float64{"tokenmart_minimax_h3_cost": ratio}
}

func (a *TaskAdaptor) AdjustBillingOnComplete(task *model.Task, taskResult *relaycommon.TaskInfo) int {
	if task == nil || (!isMiniMaxH3Model(task.Properties.OriginModelName) && !isMiniMaxH3Model(task.Properties.UpstreamModelName)) {
		return 0
	}
	responseTask, err := parseVideoTask(task.Data)
	if err != nil {
		return task.Quota
	}
	modelName := task.Properties.UpstreamModelName
	if !isMiniMaxH3Model(modelName) {
		modelName = task.Properties.OriginModelName
	}
	if isMiniMaxH3Model(responseTask.Model) {
		modelName = responseTask.Model
	}

	outputSeconds := responseTask.DurationSeconds
	inputVideoSeconds := 0
	inputImageCount := 0
	resolution := responseTask.Resolution
	ratio := responseTask.Ratio
	if responseTask.Metadata != nil {
		if responseTask.Metadata.Duration > 0 {
			outputSeconds = responseTask.Metadata.Duration
		}
		if responseTask.Metadata.Resolution != "" {
			resolution = responseTask.Metadata.Resolution
		}
		if responseTask.Metadata.Ratio != "" {
			ratio = responseTask.Metadata.Ratio
		}
	}
	if usage := effectiveTaskUsage(responseTask); usage != nil {
		if usage.OutputSeconds > 0 {
			outputSeconds = usage.OutputSeconds
		}
		inputVideoSeconds = usage.InputSeconds
		if inputVideoSeconds == 0 && usage.TotalSeconds > outputSeconds {
			inputVideoSeconds = usage.TotalSeconds - outputSeconds
		}
		inputImageCount = usage.InputImageCount
	}
	if snapshot := task.PrivateData.RequestSnapshot; snapshot != nil {
		if outputSeconds <= 0 {
			outputSeconds = snapshot.Duration
		}
		if inputImageCount == 0 {
			inputImageCount = snapshot.ImageCount
		}
		if resolution == "" {
			resolution = snapshot.ResolutionEffective
			if resolution == "" {
				resolution = snapshot.ResolutionRequested
			}
		}
		if ratio == "" {
			ratio = snapshot.AspectRatio
		}
	}
	if outputSeconds > tokenMartH3MaxDuration {
		outputSeconds = tokenMartH3MaxDuration
	}
	if inputVideoSeconds > tokenMartH3MaxDuration {
		inputVideoSeconds = tokenMartH3MaxDuration
	}
	if inputImageCount > tokenMartH3MaxImageCount {
		inputImageCount = tokenMartH3MaxImageCount
	}

	costUSD := miniMaxH3CostUSD(modelName, outputSeconds, inputVideoSeconds, inputImageCount, resolution)
	if costUSD <= 0 {
		return task.Quota
	}
	groupRatio := 1.0
	if billing := task.PrivateData.BillingContext; billing != nil {
		if billing.GroupRatio == 0 {
			return task.Quota
		}
		if billing.GroupRatio > 0 {
			groupRatio = billing.GroupRatio
		}
	}
	quota, clamp := common.QuotaRoundChecked(costUSD * common.QuotaPerUnit * groupRatio)
	if taskResult != nil && clamp != nil {
		taskResult.QuotaClamp = clamp
	}
	if quota <= 0 {
		return task.Quota
	}
	details := miniMaxH3BillingDetails(modelName, outputSeconds, inputVideoSeconds, inputImageCount, resolution, groupRatio)
	details.AspectRatio = ratio
	details.PreConsumedQuota = task.Quota
	details.SettlementDeltaQuota = quota - task.Quota
	details.FinalQuota = quota
	if taskResult != nil {
		taskResult.VideoBilling = details
	}
	if task.PrivateData.BillingContext != nil {
		task.PrivateData.BillingContext.VideoBilling = details
	}
	return quota
}
