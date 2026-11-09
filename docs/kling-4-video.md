# Kling 4.0 / 4.0 Flash 视频接口

下游调用时保留 `/kling` 前缀，其余路径、请求体、状态和查询方式与可灵官方一致。客户说明见 `docs/api-doc.html` 的 Kling 4.0 一节。

| 操作 | 提交 | 单任务查询 |
| --- | --- | --- |
| 文生视频 | `POST /kling/text-to-video/kling-4.0` | `GET /kling/text-to-video/kling-4.0/{task_id}` |
| 图生视频 | `POST /kling/image-to-video/kling-4.0` | `GET /kling/image-to-video/kling-4.0/{task_id}` |
| Omni | `POST /kling/omni-video/kling-4.0` | `GET /kling/omni-video/kling-4.0/{task_id}` |
| Flash 文生视频 | `POST /kling/text-to-video/kling-4.0-flash` | `GET /kling/text-to-video/kling-4.0-flash/{task_id}` |
| Flash 图生视频 | `POST /kling/image-to-video/kling-4.0-flash` | `GET /kling/image-to-video/kling-4.0-flash/{task_id}` |
| Flash Omni | `POST /kling/omni-video/kling-4.0-flash` | `GET /kling/omni-video/kling-4.0-flash/{task_id}` |

批量查询：`GET /kling/tasks?task_ids=` 或 `external_task_ids=`。游标查询：`POST /kling/tasks`。

创建响应为 `{code,message,request_id,data:{id,status,create_time,update_time,external_id}}`。查询响应的 `data` 为任务数组（游标查询为 `data.result`），状态为 `submitted`、`processing`、`succeeded`、`failed`。计费秒数使用 `usage.output_duration`，输入视频时长使用 `usage.input_duration`。

Omni 的 `settings.audio` 为 `native`（默认）或 `original`，会原样转发。`original` 不能与 `voice` 同时使用，且至少要有 `base_video` 或 `feature_video`。

Kling 4.0：720p $0.157/秒，1080p $0.235/秒（无输入视频）；有输入视频时 720p $0.094/秒，1080p $0.141/秒。输入时长不超过输出时长时，计费时长为输出时长的 2 倍；否则为输入时长加输出时长。时长 3–30 秒。

Kling 4.0 Flash：仅 720p。无输入视频 $0.094/秒；有输入视频 $0.063/秒。时长规则与 4.0 相同，范围为 3–20 秒。

分组倍率仍乘在上述单价上。保存到模型定价的 4.0 表达式：

```text
u("video_input") == "video" ? (u("resolution") == "1080p" ? tier("1080p_video", max(u("input_video_seconds") + u("seconds"), u("seconds") * 2) * 0.141) : tier("720p_video", max(u("input_video_seconds") + u("seconds"), u("seconds") * 2) * 0.094)) : (u("resolution") == "1080p" ? tier("1080p", u("seconds") * 0.235) : tier("720p", u("seconds") * 0.157))
```

Flash 表达式：

```text
u("video_input") == "video" ? tier("720p_video", max(u("input_video_seconds") + u("seconds"), u("seconds") * 2) * 0.063) : tier("720p", u("seconds") * 0.094)
```

`seconds` 在任务完成时优先取 `usage.output_duration`。未返回该字段时，才使用视频输出的 `duration`。
