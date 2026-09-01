# `/async/v1/generateImage` fileData 输入优化方案

> 状态：代码已部署生产，已完成自动化测试与单次本地实链路验证，等待人工实测
> 记录日期：2026-08-29
> 目标：在不影响现有 Base64/URL 客户端的前提下，为支持 Gemini `fileData` 的上游降低服务器上传请求体的耗时。

## 1. 背景与结论

现有 `/async/v1/generateImage` 的 `images` 参数只接受字符串：

- URL：本服务器先下载图片，再转换成 Base64 `inlineData` 发送给上游。
- Base64/data URL：本服务器直接构造 `inlineData` 发送给上游。

因此，即使客户端传入 URL，当前 Gemini 原生生图链路仍然会把图片内容放进上游 JSON 请求体。Base64 还会产生约 33% 的体积膨胀。

已观测到 `gaorui.cc` 的服务器上传速度稳定在约 3.8 Mbps，典型请求会额外产生 10～20 秒上传耗时，较大请求可达到约 37 秒。该上游支持 `fileData.fileUri` 后，可以将图片内容传输改为“上游主动拉取 URL”，使本服务器发给上游的 JSON 降至几 KB。

需要注意：图片传输并没有消失，而是从“本服务器向上游推送”变成“上游从 CF/R2/OSS 拉取”。最终是否改善端到端耗时，需要通过分阶段日志比较 `request_write_ms` 与 `upstream_wait_ms`。

## 2. 设计原则

1. 客户端输入格式只表达“客户端传来的是什么”。
2. 最终发给上游的是 `inlineData` 还是 `fileData`，必须由渠道能力开关决定。
3. 渠道开关默认关闭，所有现有渠道和现有客户行为保持不变。
4. 老客户端继续使用 `images: []string`，不要求修改。
5. 新客户端可以显式传入 `inlineData` 或 `fileData`，用于准确指定 MIME 类型和输入类型。
6. 初期仅在确认支持 URL 拉取的 Gemini 渠道启用，例如 `gaorui.cc`。
7. 输入临时文件使用现有 `/tmp/input` 体系，不与生成结果 `/tmp/output` 混用。

## 3. 推荐请求协议

### 3.1 老格式：继续兼容

Base64/data URL：

```json
{
  "model": "nano-banana-pro",
  "prompt": "修改参考图片",
  "images": [
    "data:image/jpeg;base64,/9j/4AAQ..."
  ]
}
```

公开 URL：

```json
{
  "model": "nano-banana-pro",
  "prompt": "修改参考图片",
  "images": [
    "https://example.com/reference.png"
  ]
}
```

老格式裸 Base64 继续兼容。没有可识别 MIME 信息时，维持当前默认 `image/png` 行为。

### 3.2 新格式：显式 inlineData

```json
{
  "model": "nano-banana-pro",
  "prompt": "修改参考图片",
  "images": [
    {
      "inlineData": {
        "mimeType": "image/jpeg",
        "data": "/9j/4AAQ..."
      }
    }
  ]
}
```

### 3.3 新格式：显式 fileData

```json
{
  "model": "nano-banana-pro",
  "prompt": "修改参考图片",
  "images": [
    {
      "fileData": {
        "mimeType": "image/jpeg",
        "fileUri": "https://cdn.example.com/input/abc.jpg"
      }
    }
  ]
}
```

### 3.4 混合输入

```json
{
  "model": "nano-banana-pro",
  "prompt": "参考这些图片生成新图片",
  "images": [
    "data:image/png;base64,...",
    {
      "fileData": {
        "mimeType": "image/jpeg",
        "fileUri": "https://example.com/a.jpg"
      }
    },
    {
      "inlineData": {
        "mimeType": "image/webp",
        "data": "UklGR..."
      }
    }
  ]
}
```

每个对象必须且只能包含 `inlineData`、`fileData` 中的一种。规范字段名为：

- `inlineData.mimeType`
- `inlineData.data`
- `fileData.mimeType`
- `fileData.fileUri`

## 4. 渠道能力开关

建议在渠道的其他设置中增加：

```json
{
  "gemini_file_data_enabled": true
}
```

语义：

- 未设置或 `false`：上游只按 `inlineData` 处理。
- `true`：该上游已确认支持 `fileData` 公共 URL。
- 开关是上游能力声明，不是客户端参数。
- 客户端不能通过请求强制把不支持 URL 的渠道切换为 `fileData`。

首批建议仅在 `gaorui.cc` 对应渠道开启。其他渠道必须先完成实际兼容性验证。

## 5. 完整兼容矩阵

| 客户端输入 | 开关关闭：上游仅支持 Base64 | 开关打开：上游支持 fileData |
| --- | --- | --- |
| 老格式 Base64 | 直接构造 `inlineData` | Base64 解码并写入本机临时文件，构造 CF `fileData` |
| 显式 `inlineData` | 直接构造 `inlineData` | 解码并写入本机临时文件，构造 CF `fileData` |
| 老格式 URL | 本服务器下载后转换为 `inlineData` | 不下载图片正文，直接构造 `fileData` |
| 显式 `fileData` | 本服务器下载后转换为 `inlineData` | 校验后原样转发 `fileData` |

这样可以保证：

- 老客户无需修改。
- 只支持 Base64 的上游继续收到 `inlineData`。
- 同时支持两种格式的上游优先收到 `fileData`。
- 客户端和上游格式可以独立演进。

## 6. 完整流程图

```text
客户端调用 POST /async/v1/generateImage
                    │
                    ▼
           解析 images 中每一项
                    │
         ┌──────────┴──────────┐
         │                     │
         ▼                     ▼
 Base64 / inlineData       URL / fileData
         │                     │
         ▼                     ▼
 读取渠道能力开关         读取渠道能力开关
         │                     │
    ┌────┴────┐           ┌────┴────┐
    │         │           │         │
 关闭        打开       关闭        打开
    │         │           │         │
    │         ▼           ▼         │
    │   解码 Base64    服务器下载    │
    │         │           │         │
    │         ▼           ▼         │
    │ 写入 /data/tmp/input 转 Base64 │
    │         │           │         │
    │         ▼           ▼         │
    │ 生成 CF 公共 URL  inlineData   │
    │         │                     │
    ▼         ▼                     ▼
inlineData  fileData             fileData
    │         │                     │
    └─────────┴──────────┬──────────┘
                        ▼
              序列化 Gemini JSON
                        │
                        ▼
        POST /v1beta/models/{model}:generateContent
                        │
                        ▼
                 上游生成并返回结果
```

## 7. Base64 转 fileData 示例

客户端提交：

```json
{
  "model": "nano-banana-pro",
  "prompt": "修改参考图片",
  "images": [
    {
      "inlineData": {
        "mimeType": "image/jpeg",
        "data": "/9j/4AAQ..."
      }
    }
  ]
}
```

渠道开启 `gemini_file_data_enabled` 后，本服务器：

1. 校验并解码 Base64。
2. 写入 `${TEMP_STORAGE_DIR}/input/<uuid>.jpg`。
3. 生成 `https://cf-api.o1key.com/tmp/input/<uuid>.jpg`。
4. 发给上游：

```json
{
  "contents": [
    {
      "role": "user",
      "parts": [
        {
          "text": "修改参考图片"
        },
        {
          "fileData": {
            "mimeType": "image/jpeg",
            "fileUri": "https://cf-api.o1key.com/tmp/input/00000000-0000-0000-0000-000000000000.jpg"
          }
        }
      ]
    }
  ]
}
```

## 8. 临时 CF 输入文件

应复用现有临时附件能力：

- 本机目录：`${TEMP_STORAGE_DIR:-tmp}/input`
- 生产默认目录：`/data/tmp/input`
- 公共 URL：`https://cf-api.o1key.com/tmp/input/<uuid>.<ext>`
- 文件名：随机 UUID
- 支持：`GET`、`HEAD`、Range
- 缓存：公开缓存，缓存时间随剩余有效期递减
- 有效期：24 小时
- 过期后：返回 404，并由清理任务删除

必须先完成文件原子落盘，再把公共 URL 发送给上游，避免上游拉取时文件尚未就绪。

## 9. 校验规则

### inlineData

- `mimeType` 必须以 `image/` 开头。
- `data` 必须是有效 Base64；新格式建议只接受纯 Base64。
- 老格式继续兼容 data URL 和裸 Base64。
- 解码后大小继续使用现有 Base64 图片上限。
- 文件真实类型必须与声明 MIME 类型匹配。

### fileData

- `fileUri` 必须是完整的 HTTP/HTTPS URL。
- 不接受 `file://`、本地路径或其他协议。
- `mimeType` 必须以 `image/` 开头。
- 继续执行 URL 长度和可选 `Content-Length` 校验。
- 开关打开时不下载图片正文，以免失去优化意义。
- 公共 URL 不能依赖 Cookie 或客户端登录状态。
- 签名 URL 必须在上游完成读取前持续有效，建议至少 30～60 分钟。

### 请求组合

- 一个输入对象只能包含一种数据类型。
- 空 `fileUri`、空 `data` 或空对象直接返回 400。
- `images` 允许字符串和显式对象混用。
- 初期仅作用于 Gemini 原生 `generateContent` 生图路径；非 Gemini 路径继续沿用原转换逻辑。

## 10. 异常与回退策略

### 本机临时文件写入失败

初期建议优先保证老客户可用性：

1. 如果回退后的 Gemini JSON 不超过现有 20 MiB 上限，回退为 `inlineData`。
2. 记录 `file_data_fallback=storage_error` 告警。
3. 如果回退后超过 20 MiB，则请求失败，不能发送超限请求。

### 上游无法下载 fileUri

- 不建议自动再次生成，避免上游已开始处理后形成重复调用或重复计费。
- 保存上游状态码和错误体摘要。
- 将其视为渠道能力或 URL 可达性故障。
- 连续出现时关闭该渠道 `gemini_file_data_enabled`。

### 原始 URL 无法确定 MIME

- 显式 `fileData` 使用客户端提供的 `mimeType`。
- 老格式 URL 可优先按扩展名推断。
- 无法可靠推断时可省略 `mimeType`（前提是已确认上游允许），否则维持旧流程下载并识别。
- 不建议为了获得 MIME 而完整下载图片。

## 11. 分阶段监控

建议增加输入准备阶段日志：

```text
generate_image_timing:
phase=input_prepare
task=<task_id>
request_id=<request_id>
channel=<channel_id>
client_format=legacy_url|legacy_base64|file_data|inline_data
upstream_format=file_data|inline_data
conversion=passthrough|download_to_inline|local_cf_to_file_data
image_count=<n>
input_bytes=<bytes>
download_ms=<ms>
decode_ms=<ms>
local_write_ms=<ms>
prepare_ms=<ms>
fallback=none|storage_error|mime_unknown
```

不要记录完整 Base64、完整签名 URL或鉴权查询参数。

继续结合现有上游日志字段：

- `request_bytes`
- `request_write_ms`
- `request_mbps`
- `upstream_wait_ms`
- `total_ms`
- `status`
- `request_error`

## 12. 上线后的验收方式

选择有代表性的 Base64、普通图片 URL 和显式 `fileData` 请求进行人工实测，并按客户端输入类型记录结果。不要求固定数量的批量对照请求。

预期变化：

| 指标 | 当前 inlineData | fileData 目标 |
| --- | ---: | ---: |
| 上游请求体 | 约 4～17 MiB | 通常几 KB |
| `request_write_ms` P50 | 约 10 秒 | 小于 500ms |
| `request_write_ms` 最大值 | 约 37 秒 | 小于 1 秒为理想目标 |
| 上传速度 | 约 3.8 Mbps | 不再是关键指标 |
| `upstream_wait_ms` | 包含生成等待 | 可能额外包含上游拉取 URL 时间 |

最终判断标准不是只看 `request_write_ms`，还要比较：

```text
input_prepare_ms + request_write_ms + upstream_wait_ms
```

如果 `request_write_ms` 明显下降，但 `upstream_wait_ms` 等量增加，说明耗时只是转移到上游下载阶段；如果总和下降，才能确认优化有效。

## 13. 推荐实施顺序

1. [x] 增加兼容字符串和对象的图片输入 DTO。
2. [x] 增加输入校验和标准化逻辑。
3. [x] 增加渠道 `gemini_file_data_enabled` 设置，默认关闭。
4. [x] 复用 `/tmp/input`，实现 Base64 → 本机临时 CF URL。
5. [x] 按兼容矩阵生成 `inlineData` 或 `fileData`。
6. [x] 增加 `input_prepare` 分阶段日志。
7. [x] 增加确定性的 DTO、转换、回退和渠道隔离测试。
8. [x] 部署生产代码，渠道开关保持默认关闭。
9. [ ] 在目标渠道开启开关并进行人工实测。
10. [ ] 根据端到端结果决定是否扩展到其他渠道。

## 14. 涉及的现有入口

- 统一异步生图路由：`router/relay-router.go`
- 请求控制器：`controller/generate_image.go`
- 统一请求 DTO：`dto/generate_image.go`
- 异步图片 DTO：`dto/async_image.go`
- Gemini 请求转换：`service/async_image.go`
- Gemini 异步处理及上游计时：`service/generate_image.go`
- 渠道其他设置：`dto/channel_settings.go`
- 临时输入存储：`service/temporary_upload.go`
- 临时输入公开响应：`controller/temporary_upload.go`
- 临时文件清理：`service/temporary_image_cleanup_task.go`
- 渠道编辑表单：`web/default/src/features/channels/`

## 15. 保持不变的事项

- 渠道开关默认关闭，是否启用由人工配置。
- 不改变现有客户端请求行为。
- 不改变图片输出策略。
- 不自动重试上游生成请求。

代码实现采用本文协议、`gemini_file_data_enabled` 开关、未知 MIME 下载回退和本机存储失败时的受限 `inlineData` 回退。目标渠道开启后由人工完成真实请求验证。
