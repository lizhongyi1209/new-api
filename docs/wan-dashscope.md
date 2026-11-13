# 百炼官方格式接口

> https://console.service-inference.ai/docs/wan-dashscope

## 万相 wan3.0 视频生成 —— 阿里云百炼官方格式接口

本平台按**阿里云百炼（DashScope）官方 API 的原样格式**提供万相 3.0 视频生成接口：路径、请求体、响应、任务状态和错误格式都与官方一致。已经按百炼官方文档写好调用代码（或在用 DashScope SDK）的，**只需把 base URL 换成本平台、API Key 换成本平台的 Key**，其余代码不用改。

支持的模型：`wan3.0-video`（标准版）和 `wan3.0-video-prime`（高速版）。两者能力、参数完全相同，`wan3.0-video-prime` 出片更快、单价更高。

视频生成为**异步接口**：提交任务拿到 `output.task_id`，轮询查询接口直到任务进入终态，再从 `output.video_url` 下载视频。单次任务通常 1–5 分钟完成。

> 想用本平台统一的 `content` 数组写法，见同组的「视频生成」文档（`/v1/video/generate`）。两套接口模型、计费完全一样，任务互不可见。

## 通用约定

- **BASE_URL**：`https://model.service-inference.ai`
- **API_KEY**：本平台的 API Key，通过请求头 `Authorization: Bearer $API_KEY` 传递
- 请求与响应均为 JSON（`Content-Type: application/json`）
- 客户端超时建议设置为 300 秒以上

```bash
export BASE_URL="https://model.service-inference.ai"
export API_KEY="<your-api-key>"
```

### 接口一览

| 操作 | 方法 | 端点 |
| --- | --- | --- |
| 创建任务 | `POST` | `/api/v1/services/aigc/video-generation/video-synthesis` |
| 查询任务 | `GET` | `/api/v1/tasks/{task_id}` |

使用 DashScope SDK 时，把 SDK 的 HTTP 地址指向本平台即可：

```python
import dashscope
dashscope.base_http_api_url = "https://model.service-inference.ai/api/v1"
dashscope.api_key = "<your-api-key>"
```

---

## 1. 创建任务

`POST $BASE_URL/api/v1/services/aigc/video-generation/video-synthesis`

### 请求头

| 请求头 | 说明 |
| --- | --- |
| `Authorization` | `Bearer $API_KEY` |
| `Content-Type` | `application/json` |
| `X-DashScope-Async` | `enable`。官方要求必带；本平台带不带都可以，建议照官方带上 |

### 请求体

| 字段 | 必选 | 说明 |
| --- | --- | --- |
| `model` | 是 | `wan3.0-video` 或 `wan3.0-video-prime` |
| `input.prompt` | 是 | 提示词。本平台要求必填（官方允许只传素材）；只用素材时也请写一句描述 |
| `input.media` | 否 | 素材数组，每项 `{ "type": "...", "url": "..." }`，见下 |
| `parameters.resolution` | 否 | `480P`、`720P`、`1080P`。**不传按官方默认 `1080P` 生成并计费**，建议显式传 |
| `parameters.ratio` | 否 | `adaptive`（按素材和提示词自动选）、`16:9`、`9:16`、`1:1`、`4:3`、`3:4`、`21:9`（宽银幕，如 480P 为 980×420） |
| `parameters.duration` | 否 | 整数秒，2–30；`-1` 为智能时长（由模型决定）。不传为 5。带参考视频时，参考视频总时长 + 输出时长不超过 30 秒 |
| `parameters.audio` | 否 | 是否带音轨，默认 `true` |
| `parameters.watermark` | 否 | 是否在画面右下角加「AI 生成」水印，默认 `false` |
| `parameters.seed` | 否 | 随机种子，整数。相同 seed 也不保证结果完全一致 |
| `parameters.prompt_extend` | 否 | 是否让模型先智能改写提示词，默认 `true`。改写后短提示词的画面更丰富，但生成更慢；传 `false` 严格按原提示词生成。见「关闭提示词智能改写」 |

`parameters` 里的字段类型不对时直接返回 400 并指出字段（例如 `duration` 写成字符串 `"10"`），不会悄悄换成默认值。

### `input.media` 素材类型

| `type` | 含义 |
| --- | --- |
| `first_frame` | 视频第一帧（最多 1 张） |
| `last_frame` | 视频最后一帧（最多 1 张，需与 `first_frame` 一起用） |
| `reference_image` | 参考图（最多 10 张） |
| `reference_video` | 参考视频（最多 5 段，总时长不超过 15 秒）；也用于视频编辑、视频延长 |
| `reference_audio` | 参考音频（最多 5 段，总时长不超过 15 秒） |

- **首尾帧与参考素材互斥**：同一请求里有 `first_frame` / `last_frame` 就不能再有 `reference_*`，反之亦然。参考图、参考视频、参考音频之间可以自由组合。
- 提示词里用「图1」「视频1」「音频1」引用参考素材，序号为该类型在 `media` 中出现的顺序，三类各自计数。
- 官方的 `file`（文档）、`link`（网页）两种素材本平台**不支持**，传了返回 400。
- 素材 URL 必须能被直接下载：公网 HTTP / HTTPS，不依赖登录态、Cookie 或 Referer。

| 素材 | 限制 |
| --- | --- |
| 图片 | JPEG / JPG / PNG（不支持透明通道）/ BMP / WEBP；单边 240–8000 像素；宽高比不超过 8:1；单张不超过 20 MB |
| 参考视频 | MP4 / MOV；单段 1–15 秒，总计不超过 15 秒；帧率不低于 16 fps；单边 240–4096 像素；宽高比不超过 8:1；单个不超过 100 MB |
| 参考音频 | WAV / MP3；单段 1–15 秒，总计不超过 15 秒；单个不超过 15 MB |

### 示例：文生视频

```bash
curl --location "$BASE_URL/api/v1/services/aigc/video-generation/video-synthesis" \
--header "Authorization: Bearer $API_KEY" \
--header 'Content-Type: application/json' \
--header 'X-DashScope-Async: enable' \
--data '{
    "model": "wan3.0-video",
    "input": {
        "prompt": "一只小猫在月光下的屋顶上奔跑，城市的霓虹灯在远处闪烁，电影级画质，流畅运镜"
    },
    "parameters": {
        "resolution": "720P",
        "ratio": "16:9",
        "duration": 5
    }
}'
```

### 示例：首帧生视频

```bash
curl --location "$BASE_URL/api/v1/services/aigc/video-generation/video-synthesis" \
--header "Authorization: Bearer $API_KEY" \
--header 'Content-Type: application/json' \
--header 'X-DashScope-Async: enable' \
--data '{
    "model": "wan3.0-video",
    "input": {
        "prompt": "涂鸦少年从墙上活过来，开始说唱",
        "media": [
            { "type": "first_frame", "url": "https://example.com/first.png" }
        ]
    },
    "parameters": {
        "resolution": "720P",
        "ratio": "adaptive",
        "duration": 5
    }
}'
```

### 示例：参考生视频（参考图 + 参考音频）

```bash
curl --location "$BASE_URL/api/v1/services/aigc/video-generation/video-synthesis" \
--header "Authorization: Bearer $API_KEY" \
--header 'Content-Type: application/json' \
--header 'X-DashScope-Async: enable' \
--data '{
    "model": "wan3.0-video",
    "input": {
        "prompt": "图1中的女孩在海边奔跑，用音频1的声音说：今天的阳光真好",
        "media": [
            { "type": "reference_image", "url": "https://example.com/girl.jpg" },
            { "type": "reference_audio", "url": "https://example.com/voice.mp3" }
        ]
    },
    "parameters": {
        "resolution": "720P",
        "ratio": "16:9",
        "duration": 5
    }
}'
```

### 示例：视频编辑

```bash
curl --location "$BASE_URL/api/v1/services/aigc/video-generation/video-synthesis" \
--header "Authorization: Bearer $API_KEY" \
--header 'Content-Type: application/json' \
--header 'X-DashScope-Async: enable' \
--data '{
    "model": "wan3.0-video",
    "input": {
        "prompt": "将整个画面转换为黏土风格",
        "media": [
            { "type": "reference_video", "url": "https://example.com/source.mp4" }
        ]
    },
    "parameters": {
        "resolution": "720P",
        "ratio": "adaptive"
    }
}'
```

### 响应

```json
{
    "output": {
        "task_status": "PENDING",
        "task_id": "c19072d3-7c58-410b-862a-b5426969a3af"
    },
    "request_id": "13bcd76b-865d-9b06-91e4-85d9fd499b49"
}
```

| 字段 | 说明 |
| --- | --- |
| `output.task_id` | 任务 ID，即阿里云百炼侧的官方任务号，查询时使用 |
| `output.task_status` | 刚创建的任务固定为 `PENDING` |
| `request_id` | 这个任务的请求 ID。同一个任务创建和之后每次查询返回的都是这个值（官方是每次请求各不相同），反馈问题时请附上 |

---

## 2. 查询任务

`GET $BASE_URL/api/v1/tasks/{task_id}`

```bash
curl --location "$BASE_URL/api/v1/tasks/c19072d3-7c58-410b-862a-b5426969a3af" \
--header "Authorization: Bearer $API_KEY"
```

### 任务状态

| `output.task_status` | 含义 | 终态 |
| --- | --- | --- |
| `PENDING` | 排队中 | 否 |
| `RUNNING` | 生成中 | 否 |
| `SUCCEEDED` | 成功，从 `output.video_url` 下载 | 是 |
| `FAILED` | 失败，见 `output.code` / `output.message` | 是 |
| `CANCELED` | 已取消 | 是 |
| `UNKNOWN` | 任务不存在或已过期 | 是 |

**轮询建议**：每 15 秒左右查一次，直到进入终态。

### 响应（成功）

```json
{
    "request_id": "13bcd76b-865d-9b06-91e4-85d9fd499b49",
    "output": {
        "task_id": "c19072d3-7c58-410b-862a-b5426969a3af",
        "task_status": "SUCCEEDED",
        "submit_time": "2026-10-15 15:44:06.853",
        "scheduled_time": "2026-10-15 15:44:06.890",
        "end_time": "2026-10-15 15:44:50.991",
        "orig_prompt": "一只猫",
        "video_url": "https://.../video.mp4"
    },
    "usage": {
        "video_count": 1,
        "duration": 2,
        "input_video_duration": 0,
        "output_video_duration": 2,
        "fps": 30,
        "SR": 480,
        "ratio": "16:9"
    }
}
```

### 响应（失败）

```json
{
    "request_id": "b8eedde4-b0fb-97af-9d28-168f7148c4ac",
    "output": {
        "task_id": "06343b94-831c-4d2e-9960-b3390bd4ef0e",
        "task_status": "FAILED",
        "submit_time": "2026-10-15 16:16:19.233",
        "scheduled_time": "2026-10-15 16:16:19.295",
        "end_time": "2026-10-15 16:16:19.382",
        "code": "InvalidParameter",
        "message": "duration must be -1 or in [2,30]"
    }
}
```

### 字段说明

| 字段 | 说明 |
| --- | --- |
| `output.submit_time` / `scheduled_time` / `end_time` | 提交 / 开始执行 / 完成时间，北京时间，`YYYY-MM-DD HH:mm:ss.SSS` |
| `output.orig_prompt` | 你提交的原始提示词（开了智能改写也只返回原文，不返回改写后的提示词） |
| `output.video_url` | 视频下载地址，仅 `SUCCEEDED` 时出现。**限时链接（约 24 小时）**，请及时下载转存 |
| `output.code` / `output.message` | 仅失败时出现：阿里云的错误码与原因 |
| `usage` | 仅成功时出现。`output_video_duration` 输出秒数、`input_video_duration` 输入视频秒数（两者都计费）、`SR` 分辨率档位、`fps` 帧率、`ratio` 实际比例 |

查询不存在、已过期或不属于你这个 API Key 的任务，返回 **200 + `task_status: UNKNOWN`**（与官方一致），不会返回 404。用本接口创建的任务请用本接口查询。

---

## 3. 关闭提示词智能改写（`prompt_extend`）

`prompt_extend` 默认开启：模型会先用大模型改写、扩写你的提示词再生成。提示词很短时画面会更丰富，但生成更慢，内容也可能超出你写的范围。需要严格按原提示词出片时，传 `"prompt_extend": false`，本平台会原样传给模型。

```json
{
    "model": "wan3.0-video",
    "input": { "prompt": "一只猫" },
    "parameters": {
        "resolution": "480P",
        "ratio": "16:9",
        "duration": 2,
        "seed": 42,
        "prompt_extend": false
    }
}
```

实测对比（同一提示词「一只猫」、同一 seed，只改 `prompt_extend`）：

| `prompt_extend` | 出片耗时 | 画面 |
| --- | --- | --- |
| `false` | 约 44 秒 | 一只猫的近景特写，完全按原提示词 |
| `true`（默认） | 约 99 秒 | 提示词被扩写，画面变成客厅毛毯上的橘猫 |

开不开改写，计费方式相同（只按分辨率和秒数计）。

---

## 4. 错误处理

所有失败（包括没带 Key、Key 无效这类还没进入业务的请求）都用百炼官方的错误格式，HTTP 状态码表示错误类别：

```json
{
    "request_id": "01999b70-...",
    "code": "InvalidParameter",
    "message": "parameters.duration must be an integer"
}
```

| HTTP 状态码 | `code` | 场景 | 怎么处理 |
| --- | --- | --- | --- |
| 400 | `InvalidParameter` | 参数非法、`input.prompt` 为空、素材类型不支持等 | 按 `message` 修正 |
| 401 | `InvalidApiKey` | API Key 无效或没带 | 检查 `Authorization` 头 |
| 402 | `Arrearage` | 余额不足或超出用量限额 | 充值或联系我们 |
| 403 | `AccessDenied` | 模型不在你的套餐内 | 调用 `GET $BASE_URL/v1/models` 查看可用模型 |
| 404 | `NotFound` | 路径不存在（本平台在 `/api/v1/` 下只提供上面两个接口） | 检查路径 |
| 429 | `Throttling` / `Throttling.RateQuota` | 触发频率限制 | 退避后重试 |
| 5xx | `InternalError` | 暂时没有可用容量或服务异常 | 稍后重试 |

**参数取值超出模型范围时（如 `duration: 1`），可能在创建时直接返回 400，也可能先创建成功、随后任务变为 `FAILED`（`output.code` 为 `InvalidParameter`）——两种情况请都按失败处理。** 例如创建时直接返回：

```json
{
    "request_id": "01a13ea2-1232-71c2-abbd-fbc5d3396f3e",
    "code": "InvalidParameter",
    "message": "Failed to submit video generation job: Upstream submit failed (400): wan3.0-video duration must be -1 or an integer between 2 and 30"
}
```

任务创建成功后也可能在生成过程中失败（`task_status: FAILED`），常见 `output.code`：

| `code` | 含义 |
| --- | --- |
| `DataInspectionFailed` | 输入或输出内容未通过内容审核，请修改提示词或素材 |
| `IPInfringementSuspect` | 输入内容疑似涉及知识产权侵权 |
| `InvalidParameter` | 参数或素材组合不被模型接受（如首尾帧与参考素材混用），按 `message` 修正 |

失败任务不计费。

---

## 5. 与百炼官方的差异

| 项目 | 本平台 |
| --- | --- |
| 鉴权 | 使用本平台的 API Key |
| `X-DashScope-Async` | 可带可不带 |
| `input.prompt` | 必填 |
| `file` / `link` 素材 | 不支持 |
| `request_id` | 同一个任务固定为一个值（官方每次请求不同） |
| 回调、任务列表、取消任务 | 不提供，请轮询查询接口 |
| 其他百炼接口（文生图、文本生成等） | 不提供，返回 404 `NotFound` |

---

## 6. 计费

与「视频生成」文档的计费规则相同：

- **按秒、按分辨率计费**：每秒按输出分辨率（以 `usage.SR` 为准）的单价计。
- **输入视频和输出视频都计费**：计费秒数 = `usage.output_video_duration` + `usage.input_video_duration`（参考视频、视频编辑 / 延长时传入的视频），按同一分辨率单价。文本、首尾帧、参考图、参考音频不单独计费。
- **失败任务不计费**；带不带音轨、开不开 `prompt_extend` 价格相同。
- `wan3.0-video-prime` 单价高于 `wan3.0-video`；智能时长（`duration: -1`）按实际生成秒数计。
- 不传 `resolution` 时按 `1080P` 生成并计费。
- **单价以 `GET $BASE_URL/v1/models` 为准**：返回的 `pricing` 是你账号的实际费率，`video_seconds`（输出）和 `video_input_seconds`（输入视频）各按分辨率列出。

