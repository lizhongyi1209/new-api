# Kling 4.0 / 4.0 Flash 视频接口

下游调用时保留 `/kling` 前缀，其余路径、请求体、状态和查询方式与可灵官方一致。客户说明见 `docs/api-site/pages/kling-4.html`（线上 `/docs/kling-4`）。

| 操作 | 提交 | 单任务查询 |
| --- | --- | --- |
| 文生视频 | `POST /kling/text-to-video/kling-4.0` | `GET /kling/text-to-video/kling-4.0/{task_id}` |
| 图生视频 | `POST /kling/image-to-video/kling-4.0` | `GET /kling/image-to-video/kling-4.0/{task_id}` |
| Omni | `POST /kling/omni-video/kling-4.0` | `GET /kling/omni-video/kling-4.0/{task_id}` |
| Flash 文生视频 | `POST /kling/text-to-video/kling-4.0-flash` | `GET /kling/text-to-video/kling-4.0-flash/{task_id}` |
| Flash 图生视频 | `POST /kling/image-to-video/kling-4.0-flash` | `GET /kling/image-to-video/kling-4.0-flash/{task_id}` |
| Flash Omni | `POST /kling/omni-video/kling-4.0-flash` | `GET /kling/omni-video/kling-4.0-flash/{task_id}` |

客户端使用任务 ID 轮询。提交带有回调地址的请求会返回 HTTP 400。

批量查询：`GET /kling/tasks?task_ids=` 或 `external_task_ids=`。游标查询：`POST /kling/tasks`。

创建响应为 `{code,message,request_id,data:{id,status,create_time,update_time,external_id}}`。查询响应的 `data` 为任务数组（游标查询为 `data.result`），状态为 `submitted`、`processing`、`succeeded`、`failed`。计费秒数使用 `usage.output_duration`，输入视频时长使用 `usage.input_duration`。可灵返回的上游扣费字段（`billing`、`final_unit_deduction`、`final_balance_deduction`）不会返回给下游。

Omni 的 `settings.audio` 为 `native`（默认）或 `original`，会原样转发。`original` 不能与 `voice` 同时使用，且至少要有 `base_video` 或 `feature_video`。

标准版支持 720p、1080p 和 4K，时长为 3–30 秒。1080p 和 4K 可设置 `settings.dynamic_range` 为 `hdr`。默认使用 SDR。Flash 支持 720p SDR、3–20 秒。

模型定价页按客户分组展示售价。标准版的 HDR 加价只按输出秒数计算。可视化编辑器分别设置各分辨率的基础单价和 HDR 输出单价。客户使用日志展示订单的实际金额。
