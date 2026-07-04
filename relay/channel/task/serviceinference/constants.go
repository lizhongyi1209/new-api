package serviceinference

import (
	"strings"

	"github.com/QuantumNous/new-api/relay/channel/task/doubao"
)

const ChannelName = "TokenMartSeedance"

var ModelList = []string{
	"dreamina-seedance-2-0-260128",
}

const (
	defaultAssetGroupName        = "tokenmart-seedance-assets"
	defaultAssetGroupDescription = "TokenMartSeedance video assets"
	defaultAssetPollAttempts     = 30
	defaultAssetPollIntervalMS   = 1000
)

// doubaoPricingModel 把本渠道的 Seedance 2.0 模型名映射到官方 doubao 价格表的键。
// TokenMart 上游计费与官方（火山引擎）完全一致：fast 走 fast 档，其余（标准/mini 等）走标准档。
func doubaoPricingModel(model string) string {
	if strings.Contains(strings.ToLower(model), "fast") {
		return "doubao-seedance-2-0-fast-260128"
	}
	return "doubao-seedance-2-0-260128"
}

// videoInputRatio 返回给定模型在「输出分辨率 × 是否含视频输入」下相对基准价（720p/480p 不含视频）
// 的计费倍率。直接复用官方 doubao 的价格表作为唯一来源，不再单独维护一套折扣比率，避免与官方口径分叉。
func videoInputRatio(modelName, resolution string, hasVideo bool) (float64, bool) {
	return doubao.GetVideoInputRatio(doubaoPricingModel(modelName), resolution, hasVideo)
}
