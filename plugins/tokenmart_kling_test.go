package plugins_test

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	builtinplugins "github.com/QuantumNous/new-api/plugins"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenMartKlingResponsesProtocol(t *testing.T) {
	testVideoResponsesProtocol(t, videoResponsesTestCase{
		pluginKey: "tokenmart-kling",
		model:     "kling-4.0-cn",
		requestBody: map[string]any{
			"model": "kling-4.0-cn",
			"input": []any{map[string]any{"role": "user", "content": []any{
				map[string]any{"type": "input_text", "text": "camera orbit"},
				map[string]any{"type": "input_image", "image_url": "https://cdn.example/frame.png"},
			}}},
			"seconds": 10,
		},
		wantAction: "image_to_video",
		wantRequest: map[string]any{
			"body": map[string]any{
				"contents": []any{
					map[string]any{"type": "prompt", "text": "camera orbit"},
					map[string]any{"type": "first_frame", "url": "https://cdn.example/frame.png"},
				},
				"settings": map[string]any{"duration": float64(10)},
			},
		},
		wantUsageKeys:  []string{"audio", "dynamic_range", "input_video", "output_seconds", "resolution", "watermark"},
		wantVendorName: "tokenmart-kling",
	})
}

func TestTokenMartKlingIsIndependentOfLegacyKling(t *testing.T) {
	generation := jsplugin.DefaultRegistry.Generation()
	require.NotNil(t, generation)

	plugin, found := generation.Get("tokenmart-kling")
	require.True(t, found)
	assert.Equal(t, "tokenmart-kling", plugin.Meta.Key)
	assert.Empty(t, plugin.Meta.ChannelTypes)
	assert.Equal(t, "https://model.service-inference.ai", plugin.Meta.BaseURL)
	assert.Equal(t, []string{"vendor", "new_api"}, plugin.Meta.Upstreams)
	assert.Contains(t, plugin.Meta.Models, "kling-4.0-cn")
	assert.Contains(t, plugin.Meta.Models, "kling-4.0-flash-cn")
	assert.Contains(t, plugin.Meta.Models, "kling-image-4.0-cn")
	assert.NotContains(t, plugin.Meta.Models, "kling-4.0")
	assert.NotContains(t, plugin.Meta.Models, "kling-4.0-flash")
	assert.NotContains(t, plugin.Meta.Models, "kling-image-4.0")
	assert.NotContains(t, plugin.Meta.Models, "kling-v1")

	legacy, ok := generation.GetByChannelType(50)
	require.True(t, ok)
	assert.Equal(t, "kling", legacy.Meta.Key)

	owned, found := generation.LookupDeclaredRoute("POST", "/kling/v1/videos/text2video")
	require.True(t, found)
	assert.Equal(t, "kling", owned.Plugin.Meta.Key)

	route, found := generation.LookupDeclaredRoute("POST", "/text-to-video/:model")
	require.True(t, found)
	assert.Equal(t, "tokenmart-kling", route.Plugin.Meta.Key)
	assert.Equal(t, "text_to_video", route.Route.Action)

	for _, path := range []string{
		"/image-to-video/:model",
		"/omni-video/:model",
		"/motion-control/:model",
		"/image-generation/:model",
		"/long-image-generation/:model",
	} {
		binding, ok := generation.LookupDeclaredRoute("POST", path)
		require.True(t, ok, path)
		assert.Equal(t, "tokenmart-kling", binding.Plugin.Meta.Key)
	}
	tasks, found := generation.LookupDeclaredRoute("GET", "/tasks")
	require.True(t, found)
	assert.Equal(t, jsplugin.RouteTypeDynamic, tasks.Route.Type)
	page, found := generation.LookupDeclaredRoute("POST", "/tasks")
	require.True(t, found)
	assert.Equal(t, "tokenmart-kling", page.Plugin.Meta.Key)

	clash, found := generation.LookupDeclaredRoute("POST", "/kling/text-to-video/:model")
	if found {
		assert.NotEqual(t, "tokenmart-kling", clash.Plugin.Meta.Key)
	}

	for _, binding := range plugin.Meta.Routes {
		assert.NotContains(t, binding.Path, "tokenmart-kling")
		assert.NotContains(t, binding.Path, "/kling/")
		assert.False(t, strings.HasPrefix(binding.Path, "/v1/videos"))
	}
	element, found := generation.LookupDeclaredRoute("POST", "/v1/general/advanced-custom-elements")
	require.True(t, found)
	assert.Equal(t, "tokenmart-kling", element.Plugin.Meta.Key)
	legacyVideos, found := generation.LookupDeclaredRoute("POST", "/v1/videos/text2video")
	if found {
		assert.NotEqual(t, "tokenmart-kling", legacyVideos.Plugin.Meta.Key)
	}
}

func TestTokenMartKlingForwardsOfficialBodies(t *testing.T) {
	plugin := loadTokenMartPlugin(t)
	official := map[string]any{
		"contents": []any{
			map[string]any{"type": "prompt", "text": "参照 @video_1"},
			map[string]any{"type": "element", "element_id": "el_1", "id": "subject"},
			map[string]any{"type": "feature_video", "url": "https://cdn.example/ref.mp4", "id": "video_1"},
			map[string]any{"type": "voice", "voice_id": "voice_1", "id": "voice_a"},
		},
		"settings": map[string]any{"resolution": "4k", "duration": float64(5), "dynamic_range": "hdr", "audio": "native"},
		"options": map[string]any{
			"callback_url":     "https://caller.example/hook",
			"external_task_id": "order-1",
			"watermark_info":   map[string]any{"enabled": false},
		},
	}

	decoded := pluginObject(t, plugin, "native", []string{"decodeSubmit"}, map[string]any{
		"method": "POST",
		"path":   "/omni-video/:model",
		"params": map[string]any{"model": "kling-4.0-cn"},
		"body":   map[string]any{"kind": "json", "value": official},
	})
	assert.Equal(t, "submit", decoded["kind"])
	assert.Equal(t, "kling-4.0-cn", decoded["model"])
	assert.Equal(t, "omni_video", decoded["action"])

	requestBody := decoded["requestBody"].(map[string]any)
	assert.Equal(t, official, requestBody["body"])

	submit := pluginObject(t, plugin, "", nil, map[string]any{
		"action":        "omni_video",
		"model":         "kling-4.0-cn",
		"upstreamModel": "kling-4.0-cn",
		"baseUrl":       "https://upstream.example/",
		"apiKey":        "access|secret",
		"upstream":      map[string]any{"kind": "vendor"},
		"requestBody":   requestBody,
	}, "buildSubmitRequest")
	assert.Equal(t, "https://upstream.example/omni-video/kling-4.0-cn", submit["url"])
	assert.Equal(t, "POST", submit["method"])
	headers := submit["headers"].(map[string]any)
	assert.Equal(t, "Bearer access|secret", headers["Authorization"])
	assert.Equal(t, official, submit["body"])

	gateway := pluginObject(t, plugin, "", nil, map[string]any{
		"action":        "omni_video",
		"model":         "kling-4.0-cn",
		"upstreamModel": "kling-4.0-cn",
		"baseUrl":       "https://gateway.example",
		"apiKey":        "sk-gateway",
		"upstream":      map[string]any{"kind": "new_api"},
		"requestBody":   requestBody,
	}, "buildSubmitRequest")
	assert.Equal(t, "https://gateway.example/omni-video/kling-4.0-cn", gateway["url"])
	assert.Equal(t, "Bearer sk-gateway", gateway["headers"].(map[string]any)["Authorization"])

	facts := pluginObject(t, plugin, "", nil, map[string]any{
		"action":        "omni_video",
		"model":         "kling-4.0-cn",
		"upstreamModel": "kling-4.0-cn",
		"requestBody":   requestBody,
	}, "extractUsage")
	assert.Equal(t, map[string]any{
		"output_seconds": float64(5),
		"resolution":     "4k",
		"audio":          "native",
		"dynamic_range":  "hdr",
		"input_video":    "video",
		"watermark":      false,
	}, facts)

	image := pluginObject(t, plugin, "native", []string{"decodeSubmit"}, map[string]any{
		"method": "POST",
		"path":   "/image-generation/:model",
		"params": map[string]any{"model": "kling-4.0-cn"},
		"body": map[string]any{"kind": "json", "value": map[string]any{
			"contents": []any{
				map[string]any{"type": "prompt", "text": "poster"},
				map[string]any{"type": "refer_image", "url": "https://cdn.example/dog.png", "id": "dog"},
				map[string]any{"type": "element", "element_id": "el_9", "id": "hero"},
			},
			"settings": map[string]any{"result_type": "series", "series_amount": float64(4), "resolution": "4k"},
		}},
	})
	assert.Equal(t, "kling-image-4.0-cn", image["model"])
	assert.Equal(t, "image_generation", image["action"])
	imageSubmit := pluginObject(t, plugin, "", nil, map[string]any{
		"action":        "image_generation",
		"upstreamModel": "kling-image-4.0-cn",
		"baseUrl":       "https://upstream.example",
		"apiKey":        "tm-key",
		"upstream":      map[string]any{"kind": "vendor"},
		"requestBody":   image["requestBody"],
	}, "buildSubmitRequest")
	assert.Equal(t, "https://upstream.example/image-generation/kling-4.0-cn", imageSubmit["url"])
	assert.Equal(t, image["requestBody"].(map[string]any)["body"], imageSubmit["body"])

	imageFacts := pluginObject(t, plugin, "", nil, map[string]any{
		"action":        "image_generation",
		"upstreamModel": "kling-image-4.0-cn",
		"requestBody":   image["requestBody"],
	}, "extractUsage")
	assert.Equal(t, float64(4), imageFacts["image_count"])
	assert.Equal(t, "4k", imageFacts["resolution"])
	assert.Equal(t, float64(1), imageFacts["refer_images"])
	assert.Equal(t, "series", imageFacts["result_type"])

	future := pluginObject(t, plugin, "native", []string{"decodeSubmit"}, map[string]any{
		"path":   "/text-to-video/:model",
		"params": map[string]any{"model": "future-video-cn"},
		"body":   map[string]any{"kind": "json", "value": map[string]any{"prompt": "keep going"}},
	})
	assert.Equal(t, "future-video-cn", future["model"])
	assert.NotContains(t, plugin.Meta.Models, "future-video-cn")
	futureSubmit := pluginObject(t, plugin, "", nil, map[string]any{
		"action":      "text_to_video",
		"baseUrl":     "https://upstream.example",
		"apiKey":      "tm-key",
		"requestBody": future["requestBody"],
	}, "buildSubmitRequest")
	assert.Equal(t, "https://upstream.example/text-to-video/future-video-cn", futureSubmit["url"])
	assert.Equal(t, map[string]any{"prompt": "keep going"}, futureSubmit["body"])
}

func TestTokenMartKlingTaskQueryAndOfficialPaths(t *testing.T) {
	plugin := loadTokenMartPlugin(t)

	created := pluginObject(t, plugin, "", nil, map[string]any{"requestBody": map[string]any{}}, map[string]any{
		"body": map[string]any{
			"code": float64(0), "message": "SUCCEED",
			"data": map[string]any{"id": "950079555921253320", "status": "submitted", "external_id": "order-1"},
		},
	}, "parseSubmitResponse")
	assert.Equal(t, "950079555921253320", created["taskId"])

	_, err := plugin.Engine.Call(t.Context(), "parseSubmitResponse", map[string]any{}, map[string]any{
		"body": map[string]any{"code": float64(1201), "message": "video resolution value '1080P' is invalid"},
	})
	require.ErrorContains(t, err, "video resolution value '1080P' is invalid")

	query := pluginObject(t, plugin, "", nil, map[string]any{
		"taskId":   "950079555921253320",
		"action":   "omni_video",
		"model":    "kling-4.0-cn",
		"baseUrl":  "https://upstream.example",
		"apiKey":   "tm-key",
		"upstream": map[string]any{"kind": "vendor"},
	}, "buildQueryRequest")
	assert.Equal(t, "https://upstream.example/tasks?task_ids=950079555921253320", query["url"])
	assert.Equal(t, "GET", query["method"])

	gatewayQuery := pluginObject(t, plugin, "", nil, map[string]any{
		"taskId":   "task_public",
		"baseUrl":  "https://gateway.example",
		"apiKey":   "sk-gateway",
		"upstream": map[string]any{"kind": "new_api"},
	}, "buildQueryRequest")
	assert.Equal(t, "https://gateway.example/tasks?task_ids=task_public", gatewayQuery["url"])

	parsed := pluginObject(t, plugin, "", nil, map[string]any{"taskId": "950079555921253320", "upstreamModel": "kling-4.0-cn"}, map[string]any{
		"code": float64(0),
		"data": []any{map[string]any{
			"id": "950079555921253320", "status": "succeeded",
			"outputs": []any{map[string]any{"type": "video", "url": "https://cdn.example/out.mp4", "watermark_url": "https://cdn.example/wm.mp4", "duration": "5.04"}},
			"usage":   map[string]any{"output_duration": "5", "input_duration": "3"},
		}},
	}, "parseTaskResult")
	assert.Equal(t, "SUCCESS", parsed["status"])
	assert.Equal(t, "https://cdn.example/out.mp4", parsed["url"])

	pending := pluginObject(t, plugin, "", nil, map[string]any{"taskId": "missing"}, map[string]any{
		"code": float64(0), "data": []any{},
	}, "parseTaskResult")
	assert.Equal(t, "IN_PROGRESS", pending["status"])

	failed := pluginObject(t, plugin, "", nil, map[string]any{}, map[string]any{
		"code": float64(1201), "message": "model is not supported",
	}, "parseTaskResult")
	assert.Equal(t, "FAILURE", failed["status"])
	assert.Equal(t, "model is not supported", failed["reason"])

	four := pluginObject(t, plugin, "", nil, map[string]any{"upstreamModel": "kling-4.0-cn", "taskId": "950079555921253320"}, map[string]any{}, map[string]any{
		"data": []any{map[string]any{
			"id": "950079555921253320", "status": "succeeded",
			"usage": map[string]any{"output_duration": "5", "input_duration": "3"},
		}},
	}, "extractUsageOnComplete")
	assert.Equal(t, map[string]any{"output_seconds": float64(10)}, four)

	older := pluginObject(t, plugin, "", nil, map[string]any{"upstreamModel": "kling-3.0-cn", "action": "text_to_video"}, map[string]any{}, map[string]any{
		"data": []any{map[string]any{
			"status":  "succeeded",
			"outputs": []any{map[string]any{"type": "video", "url": "https://cdn.example/out.mp4", "duration": "5.04"}},
		}},
	}, "extractUsageOnComplete")
	assert.Equal(t, map[string]any{"output_seconds": float64(5)}, older)

	images := pluginObject(t, plugin, "", nil, map[string]any{"upstreamModel": "kling-image-4.0-cn", "action": "image_generation"}, map[string]any{}, map[string]any{
		"data": []any{map[string]any{
			"status": "succeeded",
			"outputs": []any{
				map[string]any{"type": "image", "url": "https://cdn.example/1.png"},
				map[string]any{"type": "image", "url": "https://cdn.example/2.png", "watermark_url": "https://cdn.example/2-wm.png"},
			},
		}},
	}, "extractUsageOnComplete")
	assert.Equal(t, map[string]any{"image_count": float64(2)}, images)

	motion := pluginObject(t, plugin, "native", []string{"decodeSubmit"}, map[string]any{
		"path":   "/motion-control/:model",
		"params": map[string]any{"model": "kling-3.0-cn"},
		"body": map[string]any{"kind": "json", "value": map[string]any{
			"contents": []any{
				map[string]any{"type": "image", "url": "https://cdn.example/person.png"},
				map[string]any{"type": "video", "url": "https://cdn.example/move.mp4"},
				map[string]any{"type": "element", "element_id": "el_1", "id": "hero"},
			},
			"settings": map[string]any{"character_orientation": "video", "resolution": "1080p", "audio": "original"},
			"options":  map[string]any{"callback_url": "https://caller.example/hook"},
		}},
	})
	assert.Equal(t, "motion_control", motion["action"])
	motionSubmit := pluginObject(t, plugin, "", nil, map[string]any{
		"action":      "motion_control",
		"baseUrl":     "https://upstream.example",
		"apiKey":      "tm-key",
		"requestBody": motion["requestBody"],
	}, "buildSubmitRequest")
	assert.Equal(t, "https://upstream.example/motion-control/kling-3.0-cn", motionSubmit["url"])
	assert.Equal(t, motion["requestBody"].(map[string]any)["body"], motionSubmit["body"])
	motionFacts := pluginObject(t, plugin, "", nil, map[string]any{
		"action": "motion_control", "upstreamModel": "kling-3.0-cn", "requestBody": motion["requestBody"],
	}, "extractUsage")
	assert.Equal(t, "video", motionFacts["input_video"])
	assert.Equal(t, "original", motionFacts["audio"])
	assert.Equal(t, "1080p", motionFacts["resolution"])

	longImage := pluginObject(t, plugin, "native", []string{"decodeSubmit"}, map[string]any{
		"path":   "/long-image-generation/:model",
		"params": map[string]any{"model": "kling-4.0-cn"},
		"body": map[string]any{"kind": "json", "value": map[string]any{
			"contents": []any{
				map[string]any{"type": "prompt", "text": "detail page"},
				map[string]any{"type": "refer_image", "url": "https://cdn.example/product.png", "id": "product"},
			},
			"settings": map[string]any{"skill": "ecommerce", "length": "medium", "resolution": "2k"},
		}},
	})
	assert.Equal(t, "kling-image-4.0-cn", longImage["model"])
	longFacts := pluginObject(t, plugin, "", nil, map[string]any{
		"action": "long_image", "upstreamModel": "kling-image-4.0-cn", "requestBody": longImage["requestBody"],
	}, "extractUsage")
	assert.Equal(t, float64(1), longFacts["image_count"])
	assert.Equal(t, "long", longFacts["result_type"])
	assert.Equal(t, "ecommerce", longFacts["skill"])
	assert.Equal(t, "medium", longFacts["length"])
	assert.Equal(t, float64(1), longFacts["refer_images"])

	element := pluginObject(t, plugin, "native", []string{"decodeElement"}, map[string]any{
		"body": map[string]any{"kind": "json", "value": map[string]any{
			"element_name": "cat", "element_description": "a small cat", "reference_type": "image_refer",
			"callback_url": "https://caller.example/hook", "element_voice_id": "voice_1",
		}},
	})
	assert.Equal(t, "kling-element-cn", element["model"])
	elementSubmit := pluginObject(t, plugin, "", nil, map[string]any{
		"action": "element", "baseUrl": "https://upstream.example", "apiKey": "tm-key", "requestBody": element["requestBody"],
	}, "buildSubmitRequest")
	assert.Equal(t, "https://upstream.example/v1/general/advanced-custom-elements", elementSubmit["url"])
	assert.Equal(t, element["requestBody"].(map[string]any)["body"], elementSubmit["body"])

	listed := pluginObject(t, plugin, "native", []string{"decodeTaskQuery"}, map[string]any{
		"query": map[string]any{"task_ids": []any{"task_public,task_other"}},
	})
	assert.Equal(t, "query", listed["kind"])
	assert.Equal(t, []any{"task_public", "task_other"}, listed["taskIds"])

	_, queryErr := plugin.Engine.CallPath(t.Context(), "native", []string{"decodeTaskQuery"}, map[string]any{
		"query": map[string]any{"task_ids": []any{"a"}, "external_task_ids": []any{"b"}},
	})
	require.ErrorContains(t, queryErr, "cannot be used together")

	external := pluginObject(t, plugin, "native", []string{"decodeTaskQuery"}, map[string]any{
		"query": map[string]any{"external_task_ids": []any{"order-1"}},
	})
	assert.Equal(t, "submit", external["kind"])
	assert.Equal(t, "kling-resource-cn", external["model"])
	externalSubmit := pluginObject(t, plugin, "", nil, map[string]any{
		"action": "resource", "baseUrl": "https://upstream.example", "apiKey": "tm-key", "upstream": map[string]any{"kind": "vendor"},
		"requestBody": external["requestBody"],
	}, "buildSubmitRequest")
	assert.Equal(t, "https://upstream.example/tasks?external_task_ids=order-1", externalSubmit["url"])
	assert.Equal(t, "GET", externalSubmit["method"])

	pageBody := map[string]any{"limit": float64(20), "cursor": "9", "filters": []any{map[string]any{"key": "status", "values": []any{"succeeded"}}}}
	page := pluginObject(t, plugin, "native", []string{"decodeTaskPage"}, map[string]any{
		"body": map[string]any{"kind": "json", "value": pageBody},
	})
	assert.Equal(t, "submit", page["kind"])
	pageSubmit := pluginObject(t, plugin, "", nil, map[string]any{
		"action": "resource", "baseUrl": "https://upstream.example", "apiKey": "tm-key", "requestBody": page["requestBody"],
	}, "buildSubmitRequest")
	assert.Equal(t, "https://upstream.example/tasks", pageSubmit["url"])
	assert.Equal(t, "POST", pageSubmit["method"])
	assert.Equal(t, pageBody, pageSubmit["body"])

	voices := pluginObject(t, plugin, "native", []string{"decodeResource"}, map[string]any{
		"method": "GET",
		"path":   "/v1/general/presets-voices",
		"query":  map[string]any{"pageNum": []any{"1"}},
	})
	voiceSubmit := pluginObject(t, plugin, "", nil, map[string]any{
		"action": "resource", "baseUrl": "https://upstream.example", "apiKey": "tm-key", "requestBody": voices["requestBody"],
	}, "buildSubmitRequest")
	assert.Equal(t, "https://upstream.example/v1/general/presets-voices?pageNum=1", voiceSubmit["url"])
	assert.NotContains(t, voiceSubmit, "body")

	presented := pluginObject(t, plugin, "native", []string{"taskCreated"}, map[string]any{}, map[string]any{
		"task_id": "task_public",
		"status":  "SUBMITTED",
		"data": map[string]any{
			"code": float64(0), "message": "SUCCEED",
			"data": map[string]any{"id": "950079555921253320", "status": "submitted", "external_id": "order-1"},
		},
	})
	data := presented["data"].(map[string]any)
	assert.Equal(t, "task_public", data["id"])
	assert.Equal(t, "order-1", data["external_id"])

	voiceBody := map[string]any{"code": float64(0), "data": map[string]any{"id": "voice_9", "name": "warm"}}
	unchanged := pluginObject(t, plugin, "native", []string{"taskCreated"}, map[string]any{}, map[string]any{
		"task_id": "task_resource", "data": voiceBody,
	})
	assert.Equal(t, "voice_9", unchanged["data"].(map[string]any)["id"])

	status := pluginObject(t, plugin, "native", []string{"taskStatus"}, map[string]any{}, map[string]any{
		"task_id": "task_public",
		"status":  "SUCCESS",
		"data": map[string]any{"data": []any{map[string]any{
			"id": "950079555921253320", "status": "processing",
			"outputs": []any{map[string]any{"type": "video", "url": "https://cdn.example/out.mp4", "watermark_url": "https://cdn.example/wm.mp4"}},
		}}},
	})
	items := status["data"].([]any)
	require.Len(t, items, 1)
	item := items[0].(map[string]any)
	assert.Equal(t, "task_public", item["id"])
	assert.Equal(t, "succeeded", item["status"])
	assert.Equal(t, "https://cdn.example/wm.mp4", item["outputs"].([]any)[0].(map[string]any)["watermark_url"])

	artifactsValue, artifactErr := plugin.Engine.Call(t.Context(), "listArtifacts", map[string]any{
		"status": "SUCCESS",
		"data": map[string]any{"data": []any{map[string]any{
			"status": "succeeded",
			"outputs": []any{
				map[string]any{"type": "video", "url": "https://cdn.example/out.mp4"},
				map[string]any{"type": "image", "url": "https://cdn.example/frame.png"},
			},
		}}},
	})
	require.NoError(t, artifactErr)
	artifacts := pluginValue(t, artifactsValue).([]any)
	require.Len(t, artifacts, 2)
	assert.Equal(t, "output-1", artifacts[0].(map[string]any)["key"])
	assert.Equal(t, "video", artifacts[0].(map[string]any)["type"])
	assert.Equal(t, "output-2", artifacts[1].(map[string]any)["key"])

	content := pluginObject(t, plugin, "", nil, map[string]any{
		"artifactKey":   "output-2",
		"clientRequest": map[string]any{"method": "GET"},
		"data": map[string]any{"data": []any{map[string]any{"outputs": []any{
			map[string]any{"type": "video", "url": "https://cdn.example/out.mp4"},
			map[string]any{"type": "image", "url": "https://cdn.example/frame.png"},
		}}}},
	}, "buildContentRequest")
	assert.Equal(t, "https://cdn.example/frame.png", content["url"])
	assert.Equal(t, true, content["credentialless"])
}

func TestTokenMartKlingForwardsCnModelNamesUnchanged(t *testing.T) {
	plugin := loadTokenMartPlugin(t)

	official := pluginObject(t, plugin, "native", []string{"decodeSubmit"}, map[string]any{
		"path":   "/text-to-video/:model",
		"params": map[string]any{"model": "kling-4.0-cn"},
		"body":   map[string]any{"kind": "json", "value": map[string]any{"prompt": "orbit"}},
	})
	assert.Equal(t, "kling-4.0-cn", official["model"])
	assert.Contains(t, plugin.Meta.Models, "kling-4.0-cn")
	channel := &model.Channel{Models: "kling-4.0-cn"}
	assert.Contains(t, channel.GetModels(), official["model"])

	kept := pluginObject(t, plugin, "", nil, map[string]any{
		"action":        "text_to_video",
		"model":         "kling-4.0-cn",
		"upstreamModel": "kling-4.0-cn",
		"baseUrl":       "https://upstream.example",
		"apiKey":        "tm-key",
		"requestBody":   official["requestBody"],
	}, "buildSubmitRequest")
	assert.Equal(t, "https://upstream.example/text-to-video/kling-4.0-cn", kept["url"])

	international := pluginObject(t, plugin, "native", []string{"decodeSubmit"}, map[string]any{
		"path":   "/text-to-video/:model",
		"params": map[string]any{"model": "kling-4.0"},
		"body":   map[string]any{"kind": "json", "value": map[string]any{"prompt": "orbit"}},
	})
	assert.Equal(t, "kling-4.0", international["model"])
	assert.NotContains(t, plugin.Meta.Models, international["model"])
	assert.NotContains(t, channel.GetModels(), international["model"])
	plain := pluginObject(t, plugin, "", nil, map[string]any{
		"action":      "text_to_video",
		"model":       "kling-4.0",
		"baseUrl":     "https://upstream.example",
		"apiKey":      "tm-key",
		"requestBody": international["requestBody"],
	}, "buildSubmitRequest")
	assert.Equal(t, "https://upstream.example/text-to-video/kling-4.0", plain["url"])
}

func loadTokenMartPlugin(t *testing.T) *jsplugin.LoadedPlugin {
	t.Helper()
	source, err := builtinplugins.Source("tokenmart-kling")
	require.NoError(t, err)
	registry := jsplugin.NewRegistry()
	plugin, err := registry.RegisterFactory(source, jsplugin.Options{Key: "tokenmart-kling"})
	require.NoError(t, err)
	return plugin
}

func pluginObject(t *testing.T, plugin *jsplugin.LoadedPlugin, object string, path []string, args ...any) map[string]any {
	t.Helper()
	value := pluginCall(t, plugin, object, path, args...)
	decoded, ok := pluginValue(t, value).(map[string]any)
	require.True(t, ok)
	return decoded
}

func pluginCall(t *testing.T, plugin *jsplugin.LoadedPlugin, object string, path []string, args ...any) any {
	t.Helper()
	var value any
	var err error
	if object == "" {
		export := args[len(args)-1].(string)
		value, err = plugin.Engine.Call(t.Context(), export, args[:len(args)-1]...)
	} else {
		value, err = plugin.Engine.CallPath(t.Context(), object, path, args...)
	}
	require.NoError(t, err)
	return value
}

func pluginValue(t *testing.T, value any) any {
	t.Helper()
	encoded, err := common.Marshal(value)
	require.NoError(t, err)
	var decoded any
	require.NoError(t, common.Unmarshal(encoded, &decoded))
	return decoded
}
