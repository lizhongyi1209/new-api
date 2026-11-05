package dto

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	kitutil "github.com/QuantumNous/new-api/relaykit/relayconvert/kitutil"
	"github.com/QuantumNous/new-api/relaykit/types"
)

// MaxImageN caps the image generation count. Without this bound a huge or
// wrapped-negative n overflows quota calculation into a negative charge.
const MaxImageN = 128

// Image generation bounds taken from the GPT Image guide. They guard the
// multipliers and media sizes that reach upstream, so an invalid request fails
// locally instead of after a quota reservation.
const (
	MaxImageOutputCompression = 100
	MinImagePartialImages     = 0
	MaxImagePartialImages     = 3
	MinImageFrameCount        = 2
	MaxImageFrameCount        = 64
	MinImageFrameDurationMs   = 20
	MaxImageFrameDurationMs   = 2000

	imageMaxEdgePixels  = 3840
	imageMinTotalPixels = 655360
	imageMaxTotalPixels = 8294400
)

// ImageBillingParameters contains only the provider scalars parsed by request
// validation. Keep this separate from the complete provider request payload.
type ImageBillingParameters struct {
	N            *uint `json:"n,omitempty"`
	PromptExtend *bool `json:"prompt_extend,omitempty"`
}

type ImageRequest struct {
	Model              string          `json:"model"`
	Prompt             string          `json:"prompt" binding:"required"`
	N                  *uint           `json:"n,omitempty"`
	Size               string          `json:"size,omitempty"`
	AspectRatio        string          `json:"aspect_ratio,omitempty"`
	Quality            string          `json:"quality,omitempty"`
	ResponseFormat     string          `json:"response_format,omitempty"`
	Style              json.RawMessage `json:"style,omitempty"`
	User               json.RawMessage `json:"user,omitempty"`
	ExtraFields        json.RawMessage `json:"extra_fields,omitempty"`
	Background         json.RawMessage `json:"background,omitempty"`
	Moderation         json.RawMessage `json:"moderation,omitempty"`
	OutputFormat       json.RawMessage `json:"output_format,omitempty"`
	OutputCompression  json.RawMessage `json:"output_compression,omitempty"`
	PartialImages      json.RawMessage `json:"partial_images,omitempty"`
	Stream             *bool           `json:"stream,omitempty"`
	Images             json.RawMessage `json:"images,omitempty"`
	Mask               json.RawMessage `json:"mask,omitempty"`
	InputFidelity      json.RawMessage `json:"input_fidelity,omitempty"`
	Watermark          *bool           `json:"watermark,omitempty"`
	LayerDecomposition *bool           `json:"layer_decomposition,omitempty"`
	// GPT Image 2.5+ markup-guided edits: an annotation layer uploaded beside
	// the source image, or a JSON object carrying image_url / file_id.
	Markup json.RawMessage `json:"markup,omitempty"`
	// GPT Image 2.6 complexity hint controlling pre-render planning effort.
	Complexity json.RawMessage `json:"complexity,omitempty"`
	// GIF output shape. Only meaningful when output_format is gif.
	FrameCount      json.RawMessage `json:"frame_count,omitempty"`
	FrameDurationMs json.RawMessage `json:"frame_duration_ms,omitempty"`
	// SafetyIdentifier lets an application correlate abuse reports. Filtered
	// by the channel's allow_safety_identifier setting, like the text path.
	SafetyIdentifier json.RawMessage `json:"safety_identifier,omitempty"`
	// zhipu 4v
	WatermarkEnabled json.RawMessage `json:"watermark_enabled,omitempty"`
	UserId           json.RawMessage `json:"user_id,omitempty"`
	Image            json.RawMessage `json:"image,omitempty"`
	// 用匿名参数接收额外参数
	Extra             map[string]json.RawMessage `json:"-"`
	BillingParameters *ImageBillingParameters    `json:"-"`
}

// GenerationOptions projects the documented GPT Image parameters into the shape
// used for cross-field validation. Values that are absent stay nil.
func (i *ImageRequest) GenerationOptions(target ImageGenerationTarget) ImageGenerationOptions {
	return ImageGenerationOptions{
		Model:             i.Model,
		Target:            target,
		N:                 i.N,
		Size:              i.Size,
		Quality:           i.Quality,
		Background:        rawString(i.Background),
		OutputFormat:      rawString(i.OutputFormat),
		OutputCompression: rawInt(i.OutputCompression),
		PartialImages:     rawInt(i.PartialImages),
		Stream:            i.Stream != nil && *i.Stream,
		Markup:            rawPresent(i.Markup),
		Mask:              rawPresent(i.Mask),
		Complexity:        rawString(i.Complexity),
		FrameCount:        rawInt(i.FrameCount),
		FrameDurationMs:   rawInt(i.FrameDurationMs),
	}
}

// rawPresent reports whether a raw JSON parameter carries a usable value.
func rawPresent(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	return trimmed != "" && trimmed != "null"
}

// rawString reads an optional string parameter, tolerating a non-string value by
// treating it as absent. The caller validates the resulting value.
func rawString(raw json.RawMessage) string {
	if !rawPresent(raw) {
		return ""
	}
	var value string
	if err := kitutil.Unmarshal(raw, &value); err != nil {
		return ""
	}
	return value
}

// rawInt reads an optional integer parameter. A non-numeric value yields nil so
// the caller reports the same "must be an integer" style error as the schema.
func rawInt(raw json.RawMessage) *int {
	if !rawPresent(raw) {
		return nil
	}
	var value int
	if err := kitutil.Unmarshal(raw, &value); err != nil {
		return nil
	}
	return &value
}

// ImageCount resolves the validated request quantity. Top-level zero retains
// its legacy default of one; an explicit provider count must be positive.
func (i *ImageRequest) ImageCount(useProviderParameters bool) (int, error) {
	n := uint(1)
	if i.N != nil && *i.N != 0 {
		n = *i.N
	}
	if n > MaxImageN {
		return 0, fmt.Errorf("n must be an integer between 1 and %d", MaxImageN)
	}
	if parameters := i.BillingParameters; parameters != nil && parameters.N != nil {
		if *parameters.N > MaxImageN || useProviderParameters && *parameters.N == 0 {
			return 0, fmt.Errorf("parameters.n must be an integer between 1 and %d", MaxImageN)
		}
		if useProviderParameters {
			n = *parameters.N
		}
	}
	return int(n), nil
}

func (i *ImageRequest) UnmarshalJSON(data []byte) error {
	// 先解析成 map[string]interface{}
	var rawMap map[string]json.RawMessage
	if err := kitutil.Unmarshal(data, &rawMap); err != nil {
		return err
	}

	// 用 struct tag 获取所有已定义字段名
	knownFields := GetJSONFieldNames(reflect.TypeFor[ImageRequest]())

	// 再正常解析已定义字段
	type Alias ImageRequest
	var known Alias
	if err := kitutil.Unmarshal(data, &known); err != nil {
		return err
	}
	*i = ImageRequest(known)

	// 提取多余字段
	i.Extra = make(map[string]json.RawMessage)
	for k, v := range rawMap {
		if _, ok := knownFields[k]; !ok {
			i.Extra[k] = v
		}
	}
	return nil
}

// 序列化时需要重新把字段平铺
func (r ImageRequest) MarshalJSON() ([]byte, error) {
	// 将已定义字段转为 map
	type Alias ImageRequest
	alias := Alias(r)
	base, err := kitutil.Marshal(alias)
	if err != nil {
		return nil, err
	}

	var baseMap map[string]json.RawMessage
	if err := kitutil.Unmarshal(base, &baseMap); err != nil {
		return nil, err
	}

	// 不能合并ExtraFields！！！！！！！！
	// 合并 ExtraFields
	//for k, v := range r.Extra {
	//	if _, exists := baseMap[k]; !exists {
	//		baseMap[k] = v
	//	}
	//}

	return kitutil.Marshal(baseMap)
}

func GetJSONFieldNames(t reflect.Type) map[string]struct{} {
	fields := make(map[string]struct{})
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// 跳过匿名字段（例如 ExtraFields）
		if field.Anonymous {
			continue
		}

		tag := field.Tag.Get("json")
		if tag == "-" || tag == "" {
			continue
		}

		// 取逗号前字段名（排除 omitempty 等）
		name := tag
		if commaIdx := indexComma(tag); commaIdx != -1 {
			name = tag[:commaIdx]
		}
		fields[name] = struct{}{}
	}
	return fields
}

func indexComma(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			return i
		}
	}
	return -1
}

func (i *ImageRequest) GetTokenCountMeta() *types.TokenCountMeta {
	imageN := uint(1)
	if i.N != nil && *i.N > 0 {
		imageN = *i.N
	}

	// Keep n separate from ImagePriceRatio so size/quality and count remain
	// independent billing dimensions. Fixed-price pre-consume stores this on
	// PriceData, and image settlement reuses or replaces the same "n" ratio.
	return &types.TokenCountMeta{
		CombineText:     i.Prompt,
		MaxTokens:       1584,
		ImagePriceRatio: i.legacyDallePriceRatio(),
		BillingRatios:   map[string]float64{"n": float64(imageN)},
	}
}

func (i *ImageRequest) IsStream(c *http.Request) bool {
	return i.Stream != nil && *i.Stream
}

func (i *ImageRequest) SetModelName(modelName string) {
	if modelName != "" {
		i.Model = modelName
	}
}

type ImageResponse struct {
	Data     []ImageData     `json:"data"`
	Created  int64           `json:"created"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}
type ImageData struct {
	Url           string `json:"url"`
	B64Json       string `json:"b64_json"`
	RevisedPrompt string `json:"revised_prompt"`
	Size          string `json:"size,omitempty"`
	OutputFormat  string `json:"output_format,omitempty"`
}
