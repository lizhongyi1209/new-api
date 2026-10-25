# Kling 4.0 video API

The gateway is prepared to route `kling-4.0` through a type-50 Kling channel. Kling currently marks the official 4.0 API as Coming Soon. The gateway accepts the documented request bodies at these paths:

| Operation | Submit | Query a public task |
| --- | --- | --- |
| Text to video | `POST /kling/text-to-video/kling-4.0` | `GET /kling/text-to-video/kling-4.0/{task_id}` |
| Image to video | `POST /kling/image-to-video/kling-4.0` | `GET /kling/image-to-video/kling-4.0/{task_id}` |
| Omni video | `POST /kling/omni-video/kling-4.0` | `GET /kling/omni-video/kling-4.0/{task_id}` |

Send `Authorization: Bearer <gateway API token>` and `Content-Type: application/json`. The model is selected by the path, so the body has no `model` field. Submission returns the gateway's public video task ID; query that ID at the matching path. The gateway polls Kling's `GET /tasks?task_ids=...` internally. Query returns the gateway's compact task result with `task_id`, `status`, `video_url`, `duration`, `model`, `cost`, and `request_id`; Omni tasks with video input can also include `usage.input_duration`.

## Requests

Text to video uses a top-level `prompt`:

```json
{"prompt":"A cinematic sunrise over a harbor","settings":{"duration":10,"resolution":"1080p","aspect_ratio":"16:9"}}
```

Image to video uses `contents` with a prompt and up to 10 first, middle, or last frame images:

```json
{"contents":[{"type":"prompt","text":"Move smoothly from @first to @middle to @last"},{"type":"first_frame","url":"https://example.com/first.png","id":"first"},{"type":"middle_frame","url":"https://example.com/middle.png","id":"middle"},{"type":"last_frame","url":"https://example.com/last.png","id":"last"}],"settings":{"duration":15,"resolution":"1080p"}}
```

Omni video additionally accepts `refer_image`, `feature_video`, `base_video`, `element`, and `voice` contents. It supports `settings.aspect_ratio` including `adaptive`:

```json
{"contents":[{"type":"prompt","text":"Follow the motion in @video_1 and use the appearance in @image_1"},{"type":"feature_video","url":"https://example.com/motion.mp4","id":"video_1"},{"type":"refer_image","url":"https://example.com/subject.png","id":"image_1"}],"settings":{"duration":15,"resolution":"1080p","aspect_ratio":"adaptive"}}
```

All three endpoints accept `options.callback_url`, `options.external_task_id`, and `options.watermark_info.enabled`. Explicit `false` and empty strings are preserved when forwarding. Duration is 3–30 seconds; resolution is `720p` or `1080p`. When duration is omitted, the upstream may infer it from the prompt or base video.

## Expression billing

Kling 4.0 has its own native video meter with `seconds` (output duration), `input_video_seconds` (combined duration of input videos and video-character Elements), `video_input` (`none` or `video`), and `resolution` (`720p` or `1080p`). It does not use the older Kling `units` meter. The preview pricing rule is:

```text
no video input: output seconds × no-video rate
with video input: max(input video seconds + output seconds, 2 × output seconds) × video-input rate
```

The following ready-to-save expression uses the [official preview USD prices](https://kling.ai/document-api/pricing/base/video). Save it for `kling-4.0` in the model-pricing editor after confirming your selling prices:

```text
u("video_input") == "video" ? (u("resolution") == "1080p" ? tier("1080p_video", max(u("input_video_seconds") + u("seconds"), u("seconds") * 2) * 0.141) : tier("720p_video", max(u("input_video_seconds") + u("seconds"), u("seconds") * 2) * 0.094)) : (u("resolution") == "1080p" ? tier("1080p", u("seconds") * 0.235) : tier("720p", u("seconds") * 0.157))
```

Submission reserves 30 seconds of output when duration is omitted. A video URL's duration and an Element's type are unavailable locally, so requests containing a video or Element reserve the maximum 30 seconds of input. On success, `outputs[].duration` and `usage.input_duration` replace those estimates. If Kling omits the output duration, or omits input duration when the request may have video input, the reservation is retained and a warning is logged; no duration is invented. The official API and these prices are still marked **Coming Soon**, so confirm the returned fields and final rates at launch.

The gateway validates the JSON structure, prompt length, duration, enum values, unique reference IDs, and documented material counts before submission. Media file size, dimensions, duration, format, and whether an Element is a video character require inspecting remote assets or upstream Element records; Kling validates those properties. The gateway does not fetch arbitrary client media URLs during request validation.

Official references: [text to video](https://kling.ai/document-api/api/video/4-0/text-to-video.md), [image to video](https://kling.ai/document-api/api/video/4-0/image-to-video.md), [Omni video](https://kling.ai/document-api/api/video/4-0/video-omni.md).
