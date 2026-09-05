package serviceinference

import (
	"strings"

	"github.com/QuantumNous/new-api/relay/channel/task/doubao"
)

const ChannelName = "TokenMartSeedance"

const seedance25VideoInputPriceRatio = 42.0 / 70.0

var ModelList = []string{
	"minimax-h3",
	"minimax-h3-max",
	"doubao-seedance-2-0-260128-max",
	"doubao-seedance-2-0-fast-260128-max",
	"doubao-seedance-2-0-mini-260615-max",
	"doubao-seedance-2-5-260628-max",
	"dreamina-seedance-2-0-260128",
	"dreamina-seedance-2-0-260128-df",
	"dreamina-seedance-2-0-fast-260128-df",
	"dreamina-seedance-2-0-mini-260615-df",
	"dreamina-seedance-2-5-260628-df",
	"grok-imagine-video-1.5",
}

func isMiniMaxH3Model(model string) bool {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case "minimax-h3", "minimax-h3-max":
		return true
	default:
		return false
	}
}

func isMiniMaxH3MaxModel(model string) bool {
	return strings.EqualFold(strings.TrimSpace(model), "minimax-h3-max")
}

const (
	defaultAssetGroupName        = "tokenmart-seedance-assets"
	defaultAssetGroupDescription = "TokenMartSeedance video assets"
	defaultAssetPollAttempts     = 30
	defaultAssetPollIntervalMS   = 1000
	seedanceHCAssetBasePath      = "/v1/sd/assets"
	seedanceDFAssetBasePath      = "/v1/sd-5/assets"
)

func isSeedanceHCModel(model string) bool {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case "dreamina-seedance-2-0-hc",
		"dreamina-seedance-2-0-fast-hc",
		"dreamina-seedance-2-0-mini-hc",
		"dreamina-seedance-2-5-hc":
		return true
	default:
		return false
	}
}

func isSeedanceDFModel(model string) bool {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case "dreamina-seedance-2-0-260128-df",
		"dreamina-seedance-2-0-fast-260128-df",
		"dreamina-seedance-2-0-mini-260615-df",
		"dreamina-seedance-2-5-260628-df":
		return true
	default:
		return false
	}
}

func isDreaminaSeedanceMaxModel(model string) bool {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case "dreamina-seedance-2-0-260128-max",
		"dreamina-seedance-2-0-fast-260128-max",
		"dreamina-seedance-2-0-mini-260615-max",
		"dreamina-seedance-2-5-260628-max":
		return true
	default:
		return false
	}
}

func isDoubaoSeedanceMaxModel(model string) bool {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case "doubao-seedance-2-0-260128-max",
		"doubao-seedance-2-0-fast-260128-max",
		"doubao-seedance-2-0-mini-260615-max",
		"doubao-seedance-2-5-260628-max":
		return true
	default:
		return false
	}
}

func usesSeedanceV2(model string) bool {
	return isDreaminaSeedanceMaxModel(model) || isDoubaoSeedanceMaxModel(model)
}

func seedanceDirectAssetWorkflow(model string) (string, string, bool) {
	if isSeedanceHCModel(model) {
		return "hc", seedanceHCAssetBasePath, true
	}
	if isSeedanceDFModel(model) {
		return "df", seedanceDFAssetBasePath, true
	}
	return "", "", false
}

// doubaoPricingModel 仅把既有 Seedance 2.0 模型族对应到 doubao 价格表。
// 未知模型不应默认套用 2.0 价格，避免后续新模型被错误计费。
func doubaoPricingModel(model string) (string, bool) {
	normalizedModel := strings.ToLower(strings.TrimSpace(model))
	if !strings.Contains(normalizedModel, "seedance-2-0") {
		return "", false
	}
	if strings.Contains(normalizedModel, "fast") {
		return "doubao-seedance-2-0-fast-260128", true
	}
	return "doubao-seedance-2-0-260128", true
}

// videoInputRatio 返回给定上游模型在「输出分辨率 × 是否含视频输入」下的计费倍率。
// Seedance 2.5 使用 TokenMart 独立价格；既有 2.0 模型继续复用 doubao 价格表。
func videoInputRatio(modelName, resolution string, hasVideo bool) (float64, bool) {
	normalizedModel := strings.ToLower(strings.TrimSpace(modelName))
	if strings.Contains(normalizedModel, "seedance-2-5") {
		if hasVideo {
			return seedance25VideoInputPriceRatio, true
		}
		return 1.0, true
	}
	pricingModel, ok := doubaoPricingModel(normalizedModel)
	if !ok {
		return 0, false
	}
	return doubao.GetVideoInputRatio(pricingModel, resolution, hasVideo)
}
