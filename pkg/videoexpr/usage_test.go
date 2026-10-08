package videoexpr

import (
	"math"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
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
