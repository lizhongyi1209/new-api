package serviceinference

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

// videoInputRatioMap 视频输入折扣比率（含视频单价 / 不含视频单价）。
// 管理员应将 ModelRatio 设置为"不含视频"的较高费率，
// 系统在检测到视频输入时自动乘以此折扣。
var videoInputRatioMap = map[string]float64{
	// Doubao 标准版 (720p 不含视频: 46, 含视频: 28)
	"doubao-seedance-2-0-260128":        28.0 / 46.0, // ~0.6087
	"doubao-seedance-2-0-260128-d":      14.0 / 23.0, // ~0.6087 (原始配置 ratio=23)
	"doubao-seedance-2-0-260128-d-ep":   14.0 / 23.0, // ~0.6087 (原始配置 ratio=23)

	// Doubao Fast版 (不含视频: 37, 含视频: 22)
	"doubao-seedance-2-0-fast-260128":   22.0 / 37.0, // ~0.5946

	// Seedance 标准版 (720p 不含视频: 46, 含视频: 28)
	"seedance-2-0-260128-d":             28.0 / 46.0, // ~0.6087
	"seedance-2-0-260128-d-ep":          28.0 / 46.0, // ~0.6087

	// Seedance Fast版 (不含视频: 37, 含视频: 22)
	"seedance-2-0-fast-260128-d":        22.0 / 37.0, // ~0.5946
	"seedance-2-0-fast-d-ep":            22.0 / 37.0, // ~0.5946

	// Seedance Mini版 (不含视频: 11.5, 含视频: 7)
	"seedance-2-0-mini-260615-d":        7.0 / 11.5,  // ~0.6087 (原始配置 ratio=11.5)
	"seedance-2-0-mini-260615-d-ep":     7.0 / 11.5,  // ~0.6087 (原始配置 ratio=11.5)

	// Dreamina 映射模型 (标准版)
	"dreamina-seedance-2-0-260128":      28.0 / 46.0, // ~0.6087
	"dreamina-seedance-2-0-ep":          28.0 / 46.0, // ~0.6087

	// Dreamina 映射模型 (Fast版)
	"dreamina-seedance-2-0-fast-260128": 22.0 / 37.0, // ~0.5946
	"dreamina-seedance-2-0-fast-ep":     22.0 / 37.0, // ~0.5946

	// Dreamina 映射模型 (Mini版)
	"dreamina-seedance-2-0-mini-260615": 14.0 / 23.0, // ~0.6087
	"dreamina-seedance-2-0-mini-ep":     14.0 / 23.0, // ~0.6087
}

func GetVideoInputRatio(modelName string) (float64, bool) {
	r, ok := videoInputRatioMap[modelName]
	return r, ok
}
