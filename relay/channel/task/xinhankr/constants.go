package xinhankr

import (
	"strings"

	"github.com/QuantumNous/new-api/relay/channel/task/doubao"
)

const ChannelName = "xinhankr"

var ModelList = []string{
	"doubao-seedance-2-0",
	"doubao-seedance-2-0-fast",
	"seedance-2.0",
	"seedance-2.0-fast",
}

// doubaoPricingModel 把本渠道的 Seedance 2.0 模型名映射到官方 doubao 价格表的键。
// xinhankr 上游计费与官方（火山引擎 DoubaoVideo）完全一致：fast 走 fast 档，其余走标准档。
func doubaoPricingModel(model string) string {
	if strings.Contains(strings.ToLower(model), "fast") {
		return "doubao-seedance-2-0-fast-260128"
	}
	return "doubao-seedance-2-0-260128"
}

// videoInputRatio 返回给定模型在「输出分辨率 × 是否含视频输入」下相对基准价
// （720p/480p 不含视频）的计费倍率。直接复用官方 doubao 的价格表作为唯一来源，
// 不单独维护一套比率，避免与官方口径分叉。
func videoInputRatio(modelName, resolution string, hasVideo bool) (float64, bool) {
	return doubao.GetVideoInputRatio(doubaoPricingModel(modelName), resolution, hasVideo)
}
