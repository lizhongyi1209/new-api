package videoexpr

import (
	"math"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNativeVideoUsageContracts(t *testing.T) {
	for _, tc := range []struct {
		name       string
		channel    int
		model      string
		request    relaycommon.TaskSubmitReq
		body       string
		result     relaycommon.TaskInfo
		actual     map[string]any
		expression string
	}{
		{"tokenmart seedance", 60, "dreamina-seedance-2-0-fast-hc", relaycommon.TaskSubmitReq{Duration: 5, Metadata: map[string]any{"content": []any{map[string]any{"type": "video_url"}}}}, `{"metadata":{"usage":{"completion_tokens":100,"total_tokens":120}}}`, relaycommon.TaskInfo{}, map[string]any{"tokens": float64(100)}, `u("video_input") == "video" ? tier("video", u("tokens") * 22 / 1000000) : tier("text", u("tokens") * 37 / 1000000)`},
		{"doubao seedance", 54, "doubao-seedance-2-0-260128", relaycommon.TaskSubmitReq{Duration: 5}, `{"usage":{"total_tokens":123}}`, relaycommon.TaskInfo{}, map[string]any{"tokens": float64(123)}, `tier("video", u("tokens") * 46 / 1000000)`},
		{"xinhankr seedance", 61, "seedance-2.0-mini", relaycommon.TaskSubmitReq{Duration: 5}, `{"usage":{"completion_tokens":0,"total_tokens":12}}`, relaycommon.TaskInfo{}, map[string]any{"tokens": float64(0)}, `tier("video", u("tokens") * 23 / 1000000)`},
		{"kling credits", 50, "kling-v3-omni", relaycommon.TaskSubmitReq{Duration: 5}, `{"data":{"final_unit_deduction":"1.235"}}`, relaycommon.TaskInfo{TotalTokens: 2}, map[string]any{"units": 1.235}, `tier("video", u("units") * 1)`},
		{"unified kling credits", 50, "kling-v2-6-std-5s-novoice", relaycommon.TaskSubmitReq{Duration: 5}, `{"status":"completed","usage":{"completion_tokens":3,"total_tokens":100}}`, relaycommon.TaskInfo{CompletionTokens: 3, TotalTokens: 100}, map[string]any{"units": float64(3)}, `tier("video", u("units") * 1)`},
		{"kling 3 billing", 50, "kling-v3", relaycommon.TaskSubmitReq{Duration: 5}, `{"data":[{"billing":[{"charge_type":"unit","amount":"1.25"},{"charge_type":"cash","amount":"0.5"},{"charge_type":"coupon","amount":"9"}]}]}`, relaycommon.TaskInfo{}, map[string]any{"units": 1.75}, `tier("video", u("units") * 1)`},
		{"tencent credits", 58, "kling-v3-t", relaycommon.TaskSubmitReq{Duration: 5}, `{"Response":{"FinalUnitDeduction":"0"}}`, relaycommon.TaskInfo{}, map[string]any{"units": float64(0)}, `tier("video", u("units") * 1)`},
		{"minimax h3", 35, "MiniMax-H3", relaycommon.TaskSubmitReq{Duration: 5, ImageCount: 9}, `{"task":{"usage":{"output_seconds":5.5,"input_seconds":0,"input_image_count":0},"resolution":"2K"}}`, relaycommon.TaskInfo{}, map[string]any{"seconds": 5.5, "input_video_seconds": float64(0), "input_images": float64(0), "resolution": "2K"}, `tier("video", (u("seconds") + u("input_video_seconds")) * 0.5 + max(u("input_images") - 5, 0) * 0.2)`},
		{"tokenmart h3 max", 60, "MiniMax-H3-MAX", relaycommon.TaskSubmitReq{Duration: 5}, `{"task":{"metadata":{"usage":{"output_seconds":6,"total_seconds":6,"input_image_count":1}}}}`, relaycommon.TaskInfo{}, map[string]any{"seconds": float64(6), "input_video_seconds": float64(0), "input_images": float64(1)}, `tier("video", u("seconds") * 0.5)`},
		{"tokenmart h3 max turbo 1080p", 60, "minimax-h3-max-turbo", relaycommon.TaskSubmitReq{Duration: 5, Resolution: "1080P"}, `{"task":{"resolution":"1080P","usage":{"output_seconds":6,"input_image_count":1}}}`, relaycommon.TaskInfo{}, map[string]any{"seconds": float64(6), "input_images": float64(1), "resolution": "1080P"}, `u("resolution") == "480P" ? tier("480p", u("seconds") * 0.025) : u("resolution") == "768P" ? tier("768p", u("seconds") * 0.04) : tier("1080p", u("seconds") * 0.08)`},
		{"native grok", 48, "grok-imagine-video-1.5", relaycommon.TaskSubmitReq{Duration: 8}, `{"video":{"duration":4.75,"resolution":"720p"}}`, relaycommon.TaskInfo{}, map[string]any{"seconds": 4.75, "resolution": "720p"}, `tier("video", u("seconds") * 0.08 + u("input_images") * 0.01)`},
		{"tokenmart grok", 60, "grok-imagine-video-1.5", relaycommon.TaskSubmitReq{Duration: 8}, `{"metadata":{"video":{"duration":5.04}}}`, relaycommon.TaskInfo{}, map[string]any{"seconds": 5.04}, `tier("video", u("seconds") * 0.08)`},
		{"openai grok", 1, "grok-imagine-video-1.5-preview", relaycommon.TaskSubmitReq{Seconds: "8"}, `{"seconds":"8"}`, relaycommon.TaskInfo{}, map[string]any{"seconds": float64(8)}, `tier("video", u("seconds") * 0.08)`},
		{"omni modalities", 24, "gemini-omni-flash-preview", relaycommon.TaskSubmitReq{}, `{"usage":{"total_input_tokens":100,"total_output_tokens":300,"total_thought_tokens":25}}`, relaycommon.TaskInfo{PromptTokens: 100, CompletionTokens: 325, ThoughtTokens: 25, OutputTokensByModality: map[string]int{"video": 200}}, map[string]any{"input_tokens": 100, "text_output_tokens": 125, "video_output_tokens": 200}, `tier("video", (u("input_tokens") * 1.5 + u("text_output_tokens") * 9 + u("video_output_tokens") * 17.5) / 1000000)`},
		{"veo requested duration", 1, "veo-3.1", relaycommon.TaskSubmitReq{Duration: 8}, `{}`, relaycommon.TaskInfo{}, map[string]any{}, `tier("video", u("seconds") * 0.4)`},
		{"omni via openai", 1, "omni_flash_abra_edit", relaycommon.TaskSubmitReq{Seconds: "10"}, `{"seconds":"10"}`, relaycommon.TaskInfo{}, map[string]any{"seconds": float64(10)}, `tier("video", u("seconds") * 0.1)`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			meta := ForModel(tc.channel, tc.model)
			require.NotEmpty(t, meta.Meter)
			estimate, err := Estimate(meta, tc.request, constant.TaskActionGenerate)
			require.NoError(t, err)
			require.NotEmpty(t, estimate)
			require.NoError(t, billing_setting.SmokeTestTaskExpr(tc.expression, meta.Schema))
			actual, err := Complete(meta.Meter, estimate, []byte(tc.body), &tc.result)
			require.NoError(t, err)
			assert.Equal(t, tc.actual, actual)
		})
	}
}

func TestSeedanceRequestEstimateIncludesReferenceVideo(t *testing.T) {
	meta := ForModel(60, "dreamina-seedance-2-0-hc")
	facts, err := Estimate(meta, relaycommon.TaskSubmitReq{Duration: 5, Metadata: map[string]any{"resolution": "720p", "videos": []any{map[string]any{"url": "https://example.com/v.mp4"}}}}, "generate")
	require.NoError(t, err)
	assert.Equal(t, "video", facts["video_input"])
	assert.Equal(t, float64(432000), facts["tokens"])
	facts, err = Estimate(meta, relaycommon.TaskSubmitReq{Duration: 5}, "generate")
	require.NoError(t, err)
	assert.Equal(t, float64(108000), facts["tokens"])
	assert.Equal(t, "none", facts["video_input"])
}

func TestKling40PreviewPricingFormula(t *testing.T) {
	const expression = `u("video_input") == "video" ? (u("resolution") == "1080p" ? tier("1080p_video", max(u("input_video_seconds") + u("seconds"), u("seconds") * 2) * 0.141) : tier("720p_video", max(u("input_video_seconds") + u("seconds"), u("seconds") * 2) * 0.094)) : (u("resolution") == "1080p" ? tier("1080p", u("seconds") * 0.235) : tier("720p", u("seconds") * 0.157))`
	meta := ForModel(constant.ChannelTypeKling, "kling-4.0")
	require.Equal(t, Kling40, meta.Meter)
	require.NoError(t, billing_setting.SmokeTestTaskExpr(expression, meta.Schema))
	for _, tc := range []struct {
		name           string
		request        relaycommon.TaskSubmitReq
		response       string
		wantReserved   float64
		wantFinalUSD   float64
		wantFinalFacts map[string]any
	}{
		{
			name:         "1080p text with inferred duration",
			request:      relaycommon.TaskSubmitReq{Model: "kling-4.0"},
			response:     `{"data":[{"outputs":[{"type":"video","duration":"10"}]}]}`,
			wantReserved: 7.05, wantFinalUSD: 2.35,
			wantFinalFacts: map[string]any{"seconds": float64(10)},
		},
		{
			name:         "720p without video input",
			request:      relaycommon.TaskSubmitReq{Model: "kling-4.0", Duration: 10, Metadata: map[string]any{"settings": map[string]any{"resolution": "720p", "duration": 10}}},
			response:     `{"data":[{"outputs":[{"type":"video","duration":"10"}]}]}`,
			wantReserved: 1.57, wantFinalUSD: 1.57,
			wantFinalFacts: map[string]any{"seconds": float64(10)},
		},
		{
			name:         "1080p input shorter than output",
			request:      relaycommon.TaskSubmitReq{Model: "kling-4.0", Duration: 10, Metadata: map[string]any{"contents": []any{map[string]any{"type": "feature_video"}}}},
			response:     `{"data":[{"outputs":[{"type":"video","duration":"10"}],"usage":{"input_duration":"8.00"}}]}`,
			wantReserved: 5.64, wantFinalUSD: 2.82,
			wantFinalFacts: map[string]any{"seconds": float64(10), "input_video_seconds": float64(8), "video_input": "video"},
		},
		{
			name:         "720p input shorter than output",
			request:      relaycommon.TaskSubmitReq{Model: "kling-4.0", Duration: 10, Metadata: map[string]any{"settings": map[string]any{"resolution": "720p"}, "contents": []any{map[string]any{"type": "feature_video"}}}},
			response:     `{"data":[{"outputs":[{"type":"video","duration":"10"}],"usage":{"input_duration":"8.00"}}]}`,
			wantReserved: 3.76, wantFinalUSD: 1.88,
			wantFinalFacts: map[string]any{"seconds": float64(10), "input_video_seconds": float64(8), "video_input": "video"},
		},
		{
			name:         "1080p input longer than output",
			request:      relaycommon.TaskSubmitReq{Model: "kling-4.0", Duration: 3, Metadata: map[string]any{"contents": []any{map[string]any{"type": "base_video"}}}},
			response:     `{"data":[{"outputs":[{"type":"video","duration":"3"}],"usage":{"input_duration":"30.00"}}]}`,
			wantReserved: 4.653, wantFinalUSD: 4.653,
			wantFinalFacts: map[string]any{"seconds": float64(3), "input_video_seconds": float64(30), "video_input": "video"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			estimate, err := Estimate(meta, tc.request, constant.TaskActionKling40Omni)
			require.NoError(t, err)
			reserved, _, err := billingexpr.RunExprWithRequest(expression, billingexpr.TokenParams{}, billingexpr.RequestInput{Usage: estimate})
			require.NoError(t, err)
			assert.InDelta(t, tc.wantReserved, reserved, 1e-9)
			actual, err := Complete(meta.Meter, estimate, []byte(tc.response), &relaycommon.TaskInfo{})
			require.NoError(t, err)
			assert.Equal(t, tc.wantFinalFacts, actual)
			for key, value := range actual {
				estimate[key] = value
			}
			final, _, err := billingexpr.RunExprWithRequest(expression, billingexpr.TokenParams{}, billingexpr.RequestInput{Usage: estimate})
			require.NoError(t, err)
			assert.InDelta(t, tc.wantFinalUSD, final, 1e-9)
		})
	}
}

func TestKling40VideoInputRequiresAuthoritativeDuration(t *testing.T) {
	meta := ForModel(constant.ChannelTypeKling, "kling-4.0")
	for _, request := range []relaycommon.TaskSubmitReq{
		{Duration: 31},
		{Metadata: map[string]any{"settings": map[string]any{"duration": 31}}},
		{Metadata: map[string]any{"settings": map[string]any{"duration": 2}}},
		{Resolution: "4k"},
	} {
		_, err := Estimate(meta, request, constant.TaskActionKling40Omni)
		assert.Error(t, err, request)
	}
	estimate, err := Estimate(meta, relaycommon.TaskSubmitReq{Duration: 3, Metadata: map[string]any{"contents": []any{map[string]any{"type": "element"}}}}, constant.TaskActionKling40Omni)
	require.NoError(t, err)
	assert.Equal(t, float64(30), estimate["input_video_seconds"])
	_, err = Complete(meta.Meter, estimate, []byte(`{"data":[{"outputs":[{"type":"video","duration":"3"}]}]}`), &relaycommon.TaskInfo{})
	assert.Error(t, err)
	actual, err := Complete(meta.Meter, estimate, []byte(`{"data":[{"outputs":[{"type":"video","duration":"3"}],"usage":{"input_duration":"0.00"}}]}`), &relaycommon.TaskInfo{})
	require.NoError(t, err)
	assert.Equal(t, "none", actual["video_input"])
	for _, response := range []string{
		`{"data":[{"outputs":[{"type":"video"}],"usage":{"input_duration":"8"}}]}`,
		`{"data":[{"outputs":[{"type":"video","duration":"31"}],"usage":{"input_duration":"8"}}]}`,
		`{"data":[{"outputs":[{"type":"video","duration":"3"}],"usage":{"input_duration":"31"}}]}`,
		`{"data":[{"outputs":[{"type":"video","duration":"3"}],"usage":{"input_duration":"NaN"}}]}`,
	} {
		_, err = Complete(meta.Meter, estimate, []byte(response), &relaycommon.TaskInfo{})
		assert.Error(t, err, response)
	}
}

func TestSeedanceResolutionSchemaMatchesModelFamily(t *testing.T) {
	tests := []struct {
		name  string
		model string
		want  []string
	}{
		{name: "2.0 standard", model: "dreamina-seedance-2-0-260128-max", want: []string{"480p", "720p", "1080p", "4k"}},
		{name: "2.0 fast", model: "dreamina-seedance-2-0-fast-260128-max", want: []string{"480p", "720p"}},
		{name: "2.0 mini", model: "dreamina-seedance-2-0-mini-260615-max", want: []string{"480p", "720p"}},
		{name: "2.5", model: "dreamina-seedance-2-5-260628-max", want: []string{"480p", "720p", "1080p"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := ForModel(constant.ChannelTypeServiceInferenceVideo, tt.model)
			assert.Equal(t, tt.want, meta.Schema["resolution"].Enum)
		})
	}
}

func TestSeedanceAutomaticDurationUsesMappedModelMaximum(t *testing.T) {
	for _, tc := range []struct {
		name       string
		model      string
		hasVideo   bool
		wantTokens float64
	}{
		{name: "2.0 max reserves 15 seconds", model: "dreamina-seedance-2-0-260128-max", wantTokens: 324000},
		{name: "2.5 max reserves 30 seconds", model: "dreamina-seedance-2-5-260628-max", wantTokens: 648000},
		{name: "2.5 max includes conservative reference input", model: "dreamina-seedance-2-5-260628-max", hasVideo: true, wantTokens: 972000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := relaycommon.TaskSubmitReq{Duration: -1, HasVideo: tc.hasVideo, Metadata: map[string]any{"duration": -1}}
			facts, err := Estimate(ForModel(constant.ChannelTypeServiceInferenceVideo, tc.model), request, constant.TaskActionGenerate)
			require.NoError(t, err)
			assert.Equal(t, tc.wantTokens, facts["tokens"])
		})
	}
}

func TestNativeVideoRequestBounds(t *testing.T) {
	meta := ForModel(60, "dreamina-seedance-2-0-hc")
	for _, req := range []relaycommon.TaskSubmitReq{
		{Duration: -1}, {Duration: relaycommon.MaxTaskDurationSeconds + 1}, {Seconds: "NaN"}, {Seconds: "18446744073686646784"},
		{Duration: 5, Metadata: map[string]any{"duration": -1}},
		{Metadata: map[string]any{"parameters": map[string]any{"durationSeconds": 3601}}},
		{Metadata: map[string]any{"frames": 3600*24 + 1}},
		{ImageCount: dto.MaxImageN + 1},
		{Metadata: map[string]any{"resolution": "arbitrary"}},
	} {
		_, err := Estimate(meta, req, "generate")
		assert.Error(t, err)
	}
	_, err := Estimate(ForModel(1, "gpt-4"), relaycommon.TaskSubmitReq{}, "generate")
	assert.Error(t, err)
}

func TestGrokExtensionBillsOnlyTheGeneratedPortion(t *testing.T) {
	meta := ForModel(48, "grok-imagine-video")
	facts, err := Estimate(meta, relaycommon.TaskSubmitReq{Duration: 5}, constant.TaskActionVideoExtend)
	require.NoError(t, err)
	assert.Equal(t, float64(5), facts["seconds"])
	assert.Equal(t, "extend", facts["operation"])
	actual, err := Complete(Grok, facts, []byte(`{"video":{"duration":15}}`), &relaycommon.TaskInfo{Metadata: map[string]any{"duration": float64(15)}})
	require.NoError(t, err)
	assert.NotContains(t, actual, "seconds")
}

func TestNativeVideoCompletionRejectsMissingOrInvalidUsage(t *testing.T) {
	for _, tc := range []struct{ meter, body string }{
		{Seedance, `{}`}, {Seedance, `{"usage":{"completion_tokens":-1}}`}, {Seedance, `{"usage":{"completion_tokens":2147483648}}`},
		{Seedance, `{"usage":{"completion_tokens":1.5}}`}, {Kling, `{}`}, {Kling, `{"data":{"final_unit_deduction":"NaN"}}`},
		{Kling, `{"data":{"final_unit_deduction":"-0.1"}}`}, {Kling, `{"data":[{"billing":[{"charge_type":"unit","amount":"2147483647"},{"charge_type":"cash","amount":"1"}]}]}`},
		{Grok, `{"video":{"duration":3601}}`}, {H3, `{}`}, {H3, `{"usage":{"input_image_count":129}}`},
		{Omni, `{}`},
	} {
		_, err := Complete(tc.meter, nil, []byte(tc.body), &relaycommon.TaskInfo{})
		assert.Error(t, err, tc.body)
	}
	assert.Error(t, Validate(ForMeter(Kling).Schema, map[string]any{"units": math.Inf(1)}))
	assert.Error(t, Validate(ForMeter(Seedance).Schema, map[string]any{"tokens": -1}))
}
