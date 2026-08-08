package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFormatUserLogsStripsQuotaSaturation verifies the admin-only quota
// saturation marker (nested under other.admin_info) is removed for non-admin
// log views, since formatUserLogs strips the whole admin_info object.
func TestFormatUserLogsStripsQuotaSaturation(t *testing.T) {
	other := common.MapToJsonStr(map[string]interface{}{
		"model_price": 0.004,
		"admin_info": map[string]interface{}{
			"quota_saturation": map[string]interface{}{
				"op":      "QuotaFromDecimal",
				"kind":    "overflow",
				"clamped": common.MaxQuota,
			},
		},
	})
	logs := []*Log{{Other: other}}

	formatUserLogs(logs, 0)

	parsed, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	_, hasAdminInfo := parsed["admin_info"]
	require.False(t, hasAdminInfo, "admin_info (and nested quota_saturation) must be stripped for non-admin views")
	// Non-admin billing fields remain visible.
	require.Contains(t, parsed, "model_price")
}

func TestFormatUserLogsPreservesStreamStatus(t *testing.T) {
	streamStatus := map[string]interface{}{
		"status":      "error",
		"end_reason":  "upstream_error",
		"error_count": float64(1),
	}
	logs := []*Log{{
		Other: common.MapToJsonStr(map[string]interface{}{
			"stream_status": streamStatus,
			"admin_info":    map[string]interface{}{"channel_id": 12},
		}),
	}}

	formatUserLogs(logs, 0)

	parsed, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	require.NotContains(t, parsed, "admin_info")
	require.Equal(t, streamStatus, parsed["stream_status"])
}

func TestFormatUserLogsStripsUpstreamRequestID(t *testing.T) {
	logs := []*Log{{
		RequestId:         "request-public",
		UpstreamRequestId: "upstream-private",
	}}

	formatUserLogs(logs, 0)

	require.Equal(t, "request-public", logs[0].RequestId)
	require.Empty(t, logs[0].UpstreamRequestId)
}

func TestFormatUserLogsStripsTaskLifecycle(t *testing.T) {
	logs := []*Log{{
		Other: common.MapToJsonStr(map[string]interface{}{
			"task_lifecycle": map[string]interface{}{
				"status":                  "SUCCESS",
				"response_body_available": true,
			},
			"task_id": "task-public",
		}),
	}}

	formatUserLogs(logs, 0)

	parsed, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	assert.NotContains(t, parsed, "task_lifecycle")
	assert.Equal(t, "task-public", parsed["task_id"])
}

func TestFormatUserLogsStripsProviderVideoCosts(t *testing.T) {
	logs := []*Log{{
		Other: common.MapToJsonStr(map[string]interface{}{
			"video_billing": map[string]interface{}{
				"billing_mode":                  "video_per_second",
				"output_seconds":                5,
				"output_unit_rate":              0.5,
				"output_cost":                   2.5,
				"reference_video_input_seconds": 0,
				"reference_video_cost":          0,
				"image_count":                   1,
				"image_unit_rate":               0.2,
				"image_surcharge":               0,
				"provider_cost":                 2.5,
				"final_cost":                    2.5,
				"final_quota":                   1_250_000,
			},
		}),
	}}

	formatUserLogs(logs, 0)

	parsed, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	videoBilling, ok := parsed["video_billing"].(map[string]interface{})
	require.True(t, ok)
	assert.NotContains(t, videoBilling, "output_unit_rate")
	assert.NotContains(t, videoBilling, "output_cost")
	assert.NotContains(t, videoBilling, "reference_video_cost")
	assert.NotContains(t, videoBilling, "image_unit_rate")
	assert.NotContains(t, videoBilling, "image_surcharge")
	assert.NotContains(t, videoBilling, "provider_cost")
	assert.NotContains(t, videoBilling, "final_cost")
	assert.Equal(t, float64(5), videoBilling["output_seconds"])
	assert.Equal(t, float64(1), videoBilling["image_count"])
	assert.Equal(t, float64(1_250_000), videoBilling["final_quota"])
}

func TestStripServerOnlyLogSnapshotsPreservesOtherAdminInfo(t *testing.T) {
	logs := []*Log{{
		Other: common.MapToJsonStr(map[string]interface{}{
			"model_price": 0.004,
			"admin_info": map[string]interface{}{
				"use_channel":      []int{12, 18},
				"upstream_request": map[string]interface{}{"path": "/v1/images/edits"},
				"upstream_response": map[string]interface{}{
					"status_code": 400,
				},
			},
		}),
	}}

	stripServerOnlyLogSnapshots(logs)

	parsed, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	require.Equal(t, 0.004, parsed["model_price"])
	adminInfo, ok := parsed["admin_info"].(map[string]interface{})
	require.True(t, ok)
	require.NotContains(t, adminInfo, "upstream_request")
	require.NotContains(t, adminInfo, "upstream_response")
	require.Contains(t, adminInfo, "use_channel")
}
