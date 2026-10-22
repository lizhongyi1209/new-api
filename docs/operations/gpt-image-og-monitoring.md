# gpt-image-og 参数错误监控

`GPT_IMAGE_PARAM_CAPTURE_UNTIL` 启用临时诊断。到达 UTC 截止时间后，服务自动停止记录。只对 `gpt-image-og` 分组的 GPT Image 4xx 上游错误写入 `logs.other.admin_info.gpt_image_param_capture`。

采集内容包括最终发送给上游的请求路径、模型、请求体大小、状态码，以及安全的标量参数（例如 `n`、`size`、`quality`、`background`、`moderation`、`output_format`、`stream` 和 `parameters.n`）。提示词、图片、用户字段、凭据和结构化参数不写入日志，只保留字段名、长度和哈希摘要。

## 查询最近错误

```sql
SELECT
  request_id,
  model_name,
  other::jsonb->>'request_path' AS request_path,
  content,
  other::jsonb->'admin_info'->'gpt_image_param_capture' AS params
FROM logs
WHERE type = 5
  AND created_at >= extract(epoch FROM now() - interval '24 hours')::bigint
  AND (NULLIF(other, '')::jsonb->'admin_info'->'request_policy'->0->>'group') = 'gpt-image-og'
  AND (NULLIF(other, '')::jsonb->'admin_info') ? 'gpt_image_param_capture'
ORDER BY id DESC;
```

`503` 容量错误不会写入 GPT Image 专用的 `gpt_image_param_capture`；它们仍会写入通用的
`request_snapshot`，同时应单独统计，避免把上游容量问题误判为参数问题。

除这个临时的 GPT Image 专用字段外，所有 AI relay、任务、异步生图和
Midjourney 请求都会写入通用的 `other.admin_info.request_snapshot`。通用快照
同样过滤 Base64、文件、凭据和超大字段，详细字段限制与查询方式见
[request-body-audit.md](request-body-audit.md)。
