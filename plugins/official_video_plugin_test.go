package plugins_test

import (
	"encoding/json"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestKlingVideoPluginExposesPrices(t *testing.T) {
	var prices map[string]struct {
		Key        string `json:"plugin_key"`
		Expression string `json:"billing_expr"`
	}
	raw, err := os.ReadFile("../deploy/task-plugins/kling-video-plugin/prices.json")
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &prices))
	source, err := os.ReadFile("../deploy/task-plugins/kling-video-plugin/plugin.js")
	require.NoError(t, err)
	plugin, err := jsplugin.CompilePlugin(string(source), jsplugin.Options{Key: "kling-video-plugin"})
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"kling-3.0", "kling-3.0-omni", "kling-4.0", "kling-4.0-flash", "kling-image-4.0"}, plugin.Meta.Models)
	require.Len(t, prices, len(plugin.Meta.Models))
	for model, price := range prices {
		require.Equal(t, "kling-video-plugin", price.Key)
		require.Contains(t, plugin.Meta.Models, model)
		schema, _ := plugin.Meta.UsageForModel(model)
		require.NoError(t, billing_setting.SmokeTestTaskExpr(price.Expression, schema), model)
	}
}

func TestOfficialVideoPluginsExposePricesAndNormalizeProviderUsage(t *testing.T) {
	var prices map[string]struct {
		Key        string `json:"plugin_key"`
		Expression string `json:"billing_expr"`
	}
	raw, err := os.ReadFile("../deploy/task-plugins/official-video-prices.json")
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &prices))
	require.Len(t, prices, 9)
	plugins := map[string]*jsplugin.LoadedPlugin{}
	for model, price := range prices {
		plugin := plugins[price.Key]
		if plugin == nil {
			source, err := os.ReadFile("../deploy/task-plugins/" + price.Key + "/plugin.js")
			require.NoError(t, err)
			plugin, err = jsplugin.CompilePlugin(string(source), jsplugin.Options{Key: price.Key})
			require.NoError(t, err)
			plugins[price.Key] = plugin
		}
		require.Contains(t, plugin.Meta.Models, model)
		schema, _ := plugin.Meta.UsageForModel(model)
		require.NoError(t, billing_setting.SmokeTestTaskExpr(price.Expression, schema))
	}
	tests := []struct{ name, model, state, body, want string }{
		{"zero releases credit reservation", "kling-v3", "{}", `{"data":{"final_unit_deduction":"0"}}`, `{"units":0,"units_source":"reported"}`},
		{"missing credits retain reservation", "kling-v3", "{}", `{"data":{}}`, "null"},
		{"mixed charge types", "kling-3.0", "{}", `{"data":[{"id":"t","billing":[{"charge_type":"unit","amount":"1.5"},{"charge_type":"cash","amount":"2.1"},{"charge_type":"voucher","amount":"7"}]}]}`, `{"units":3.6,"units_source":"reported"}`},
		{"official input and output durations", "kling-4.0", `{"video_input":"video"}`, `{"data":[{"id":"t","usage":{"input_duration":3,"output_duration":5},"outputs":[{"type":"video","duration":"5.041"}]}]}`, `{"video_input":"video","billed_seconds":10}`},
		{"unknown reference duration retains price", "kling-4.0-flash", `{"video_input":"video"}`, `{"data":[{"id":"t","usage":{"output_duration":5}}]}`, "null"},
		{"missing wan reference duration", "wan3.0-video", `{"reference":true}`, `{"usage":{"output_video_duration":5,"SR":720}}`, "null"},
		{"wan prime actual resolution and duration", "wan3.0-video-prime", `{"reference":true}`, `{"usage":{"output_video_duration":5,"input_video_duration":3,"SR":1080}}`, `{"billed_seconds":8,"resolution":"1080P"}`},
	}
	for _, v := range tests {
		t.Run(v.name, func(t *testing.T) {
			var state, body map[string]any
			require.NoError(t, json.Unmarshal([]byte(v.state), &state))
			require.NoError(t, json.Unmarshal([]byte(v.body), &body))
			ctx := map[string]any{"taskId": "t", "model": v.model, "state": state}
			facts, err := plugins[prices[v.model].Key].Engine.Call(t.Context(), "extractUsageOnComplete", ctx, map[string]any{}, body)
			require.NoError(t, err)
			encoded, err := json.Marshal(facts)
			require.NoError(t, err)
			require.JSONEq(t, v.want, string(encoded))
		})
	}
}
