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

创建响应为 `{code,message,request_id,data:{id,status,create_time,update_time,external_id}}`。查询响应的 `data` 为任务数组（游标查询为 `data.result`），状态为 `submitted`、`processing`、`succeeded`、`failed`。计费秒数使用 `usage.output_duration`，输入视频时长使用 `usage.input_duration`。可灵返回的上游扣费字段（`billing`、`final_unit_deduction`、`final_balance_deduction`）不会返回给下游。

Omni 的 `settings.audio` 为 `native`（默认）或 `original`，会原样转发。`original` 不能与 `voice` 同时使用，且至少要有 `base_video` 或 `feature_video`。

价格与可灵官方人民币计价一致（1 积分 = ¥1，见 [视频计价](https://www.klingai.com/document-api/pricing/base/video)）：

| 型号 | 条件 | 720p | 1080p |
| --- | --- | --- | --- |
| Kling 4.0 | 无输入视频 | ¥1.00/秒（1.0 积分） | ¥1.50/秒（1.5 积分） |
| Kling 4.0 | 有输入视频 | ¥0.60/秒（0.6 积分） | ¥0.90/秒（0.9 积分） |
| Kling 4.0 Flash | 无输入视频 | ¥0.60/秒（0.6 积分） | — |
| Kling 4.0 Flash | 有输入视频 | ¥0.40/秒（0.4 积分） | — |

输入视频包含参考类视频（`feature_video` 特征参考视频、`base_video` 待编辑视频）和视频角色主体。两者都不支持 4K。

单价乘以计费时长 `billed_seconds`：无输入视频时为输出时长；有输入视频时，输入时长不超过输出时长按输出时长的 2 倍计，否则按输入时长加输出时长计，即 `max(输入 + 输出, 输出 × 2)`。该规则由网关在预扣和结算时计算，定价表达式只需写每秒单价。时长：4.0 为 3–30 秒，Flash 为 3–20 秒。

分组倍率仍乘在上述单价上。表达式是“分辨率 × 视频输入”的扁平分档，可直接在模型定价的可视化矩阵中编辑（输出时长、输入时长两列保持 0）。4.0 表达式：

```text
u("resolution") == "720p" && u("video_input") == "none" ? tier("720p·none", u("billed_seconds") * 1 + u("input_video_seconds") * 0 + u("seconds") * 0) : u("resolution") == "720p" && u("video_input") == "video" ? tier("720p·video", u("billed_seconds") * 0.6 + u("input_video_seconds") * 0 + u("seconds") * 0) : u("resolution") == "1080p" && u("video_input") == "none" ? tier("1080p·none", u("billed_seconds") * 1.5 + u("input_video_seconds") * 0 + u("seconds") * 0) : tier("1080p·video", u("billed_seconds") * 0.9 + u("input_video_seconds") * 0 + u("seconds") * 0)
```

Flash 表达式：

```text
u("resolution") == "720p" && u("video_input") == "none" ? tier("720p·none", u("billed_seconds") * 0.6 + u("input_video_seconds") * 0 + u("seconds") * 0) : tier("720p·video", u("billed_seconds") * 0.4 + u("input_video_seconds") * 0 + u("seconds") * 0)
```

`seconds` 在任务完成时优先取 `usage.output_duration`。未返回该字段时，才使用视频输出的 `duration`。
