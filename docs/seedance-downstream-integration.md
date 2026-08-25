# Seedance 2.0 下游对接说明

## 1. 对接结论

下游不需要、也不应直接配置 `TokenMartSeedance` 渠道。下游只需把本站当作视频生成 API 服务，通过本站签发的 API Token 调用：

- 创建视频任务：`POST /v1/video/generations`
- 查询视频任务：`GET /v1/video/generations/{task_id}`

本站收到请求后，会根据模型和令牌分组路由到内部渠道。渠道 ID `265` 仅是本站内部配置，不是下游请求参数，不能通过请求体指定。

如果请求包含公网图片 URL，本站会自动创建上游素材并把图片转换为 `asset://{asset_id}`，下游通常不需要单独调用素材接口。

本文示例使用本站当前服务地址：

```text
https://cf-api.o1key.com
```

美国直连入口也可使用 `https://api.o1key.com`。同一套接口路径和请求格式保持不变。

## 2. 已核对任务

任务 ID：`task_ewRJSMfagKBf46U2kpQbqc9qddmRombH`

核对结果：

| 项目 | 值 |
| --- | --- |
| 本站创建接口 | `POST /v1/video/generations` |
| 本站公开任务 ID | `task_ewRJSMfagKBf46U2kpQbqc9qddmRombH` |
| 内部渠道 | `265` |
| 渠道类型 | `TokenMartSeedance`，类型编号 `60` |
| 请求模型 | `seedance-2-0-260128-d-ep` |
| 上游模型 | `dreamina-seedance-2-0-ep` |
| 时长 | `8` 秒 |
| 最终状态 | `SUCCESS` |
| 上游任务 ID | `mvt-669f56af71874eff` |

任务审计记录不会保存图片、音频和视频正文或原始媒体 URL，因此无法从该任务记录还原当时的图片 URL 和素材 ID。这不会影响接口及请求方法的确认。

## 3. 认证

所有下游请求都使用本站签发的 API Token：

```http
Authorization: Bearer <本站 API Token>
Content-Type: application/json
Accept: application/json
```

不要把渠道 265 的上游密钥交给下游，也不要让下游直接访问渠道 265 的上游地址。

## 4. 推荐方式：由本站自动创建素材

### 4.1 创建任务

```http
POST https://cf-api.o1key.com/v1/video/generations
```

图生视频请求示例：

```json
{
  "model": "seedance-2-0-260128-d-ep",
  "content": [
    {
      "type": "text",
      "text": "让人物自然向前走一步，镜头平滑推进"
    },
    {
      "type": "image_url",
      "image_url": {
        "url": "https://cdn.example.com/reference.jpg"
      },
      "role": "reference_image"
    }
  ],
  "duration": 8,
  "resolution": "720p",
  "ratio": "3:4",
  "generate_audio": true,
  "watermark": false,
  "return_last_frame": false
}
```

请求参数：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `model` | string | 是 | 使用本站公开的模型名，不要使用上游模型名 |
| `content` | array | 是 | 文本和参考媒体列表 |
| `content[].type` | string | 是 | `text`、`image_url`、`video_url` 或 `audio_url` |
| `content[].text` | string | 文本项必填 | 提示词 |
| `content[].image_url.url` | string | 图片项必填 | 公网可访问的 `http://` 或 `https://` 图片 URL，也支持已创建的 `asset://{asset_id}` |
| `content[].role` | string | 否 | 资源角色，例如 `reference_image`、`first_frame` |
| `duration` | integer | 否 | 视频时长，单位为秒 |
| `resolution` | string | 否 | 例如 `720p`、`1080p`、`4k`，以模型实际支持范围为准 |
| `ratio` | string | 否 | 例如 `16:9`、`9:16`、`3:4`、`1:1` |
| `generate_audio` | boolean | 否 | 是否生成音频 |
| `watermark` | boolean | 否 | 是否添加水印 |
| `return_last_frame` | boolean | 否 | 是否返回最后一帧 |

创建成功后，本站返回公开任务 ID。示例：

```json
{
  "id": "task_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
  "task_id": "task_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
  "object": "video",
  "model": "seedance-2-0-260128-d-ep",
  "status": "queued",
  "progress": 0,
  "created_at": 1785768134
}
```

下游必须保存 `id` 或 `task_id`。两者当前相同；后续查询必须使用本站返回的 `task_...`，不要使用内部上游的 `mvt-...`。

### 4.2 查询任务

```http
GET https://cf-api.o1key.com/v1/video/generations/{task_id}
```

处理中响应示例：

```json
{
  "id": "task_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
  "task_id": "task_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
  "object": "video",
  "model": "seedance-2-0-260128-d-ep",
  "status": "in_progress",
  "progress": 50,
  "created_at": 1785768134
}
```

完成响应示例：

```json
{
  "id": "task_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
  "task_id": "task_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
  "object": "video",
  "model": "seedance-2-0-260128-d-ep",
  "status": "completed",
  "progress": 100,
  "created_at": 1785768134,
  "completed_at": 1785768366,
  "metadata": {
    "url": "https://cdn.example.com/result.mp4",
    "outputs": [
      "https://cdn.example.com/result.mp4"
    ],
    "usage": {
      "completion_tokens": 174794,
      "total_tokens": 174794
    }
  }
}
```

任务状态可能为：

- `queued`
- `in_progress`
- `completed`
- `failed`

建议每 2 至 5 秒查询一次，直到进入 `completed` 或 `failed` 终态。

## 5. 本站内部素材创建流程

当 `content` 中的 `image_url.url` 是公网 HTTP(S) URL 时，本站渠道 265 会先按模型选择素材工作流，再把图片转换为 `asset://{asset_id}`。

DF 模型使用直接素材接口：

```http
POST /v1/sd-5/assets
```

```json
{
  "URL": "https://cdn.example.com/reference.jpg",
  "Name": "reference_image",
  "AssetType": "Image"
}
```

素材未完成时查询：

```http
GET /v1/sd-5/assets/{asset_id}
```

以下四个上游模型归入 DF 素材工作流：

- `dreamina-seedance-2-0-260128-df`
- `dreamina-seedance-2-0-fast-260128-df`
- `dreamina-seedance-2-0-mini-260615-df`
- `dreamina-seedance-2-5-260628-df`

其他 ServiceInference Seedance 模型继续使用素材组工作流：

1. 检查或创建素材组。
2. 调用素材创建接口导入图片 URL。
3. 查询素材处理状态，直到完成。
4. 把视频请求中的图片 URL 替换为 `asset://{asset_id}`。
5. 提交上游视频生成任务。

素材创建的核心接口和请求方法是：

```http
POST /v1/assets
```

请求体：

```json
{
  "group_id": "group-xxxxxxxx",
  "url": "https://cdn.example.com/reference.jpg",
  "asset_type": "Image",
  "name": "reference_image"
}
```

典型响应：

```json
{
  "id": "asset-xxxxxxxx",
  "task_id": "asset-task-xxxxxxxx",
  "status": "processing"
}
```

若素材尚未完成，使用以下接口查询：

```http
POST /v1/assets/get
```

```json
{
  "asset_id": "asset-xxxxxxxx",
  "task_id": "asset-task-xxxxxxxx"
}
```

素材组相关接口：

```http
POST /v1/asset-groups
GET /v1/asset-groups/{group_id}
```

创建素材组的请求体：

```json
{
  "name": "downstream-seedance-assets",
  "description": "Seedance video reference assets"
}
```

## 6. 可选方式：下游手动创建并复用素材

本站公开了素材代理接口。下游如需复用同一张图片，可以使用本站 API Token 调用以下接口；本站会在服务端使用渠道 265 的凭据访问真实上游：

HC 和 DF 客户端可以使用统一入口。由于上游 HC 工作流已进入退役迁移，`type=hc` 作为兼容别名与 `type=df` 一样转发到 DF 素材接口：

```http
POST https://cf-api.o1key.com/v1/seedance/assets
```

```json
{
  "type": "df",
  "url": "https://cdn.example.com/reference.jpg",
  "name": "reference_image",
  "asset_type": "image"
}
```

将 `type` 设为 `hc` 或 `df` 都会转发到 `/v1/sd-5/assets`，已有客户端无需修改 `type=hc`。状态查询接口为：

```http
GET https://cf-api.o1key.com/v1/seedance/assets/{asset_id}?type=df
```

原有素材组接口保持可用：

```text
POST https://cf-api.o1key.com/v1/asset-groups
GET  https://cf-api.o1key.com/v1/asset-groups/{group_id}
POST https://cf-api.o1key.com/v1/assets
POST https://cf-api.o1key.com/v1/assets/get
```

素材完成后，在视频请求中传入：

```json
{
  "type": "image_url",
  "image_url": {
    "url": "asset://asset-xxxxxxxx"
  },
  "role": "reference_image"
}
```

只有需要跨多个任务复用素材时才建议手动创建。普通图生视频请求直接传公网图片 URL 即可。

## 7. 渠道 265 的公开模型映射

下游只能发送左侧的本站模型名，右侧上游模型名由本站内部转换：

| 本站模型名 | 内部上游模型名 |
| --- | --- |
| `seedance-2-0-260128-d` | `dreamina-seedance-2-0-260128` |
| `seedance-2-0-260128-d-ep` | `dreamina-seedance-2-0-ep` |
| `seedance-2-0-fast-260128-d` | `dreamina-seedance-2-0-fast-260128` |
| `seedance-2-0-fast-d-ep` | `dreamina-seedance-2-0-fast-ep` |
| `seedance-2-0-mini-260615-d` | `dreamina-seedance-2-0-mini-260615` |
| `seedance-2-0-mini-260615-d-ep` | `dreamina-seedance-2-0-mini-ep` |

## 8. 下游实现要求

- 下游没有 `TokenMartSeedance` 渠道类型时，直接实现上述 HTTP 客户端即可，不需要复制本站渠道类型。
- 下游配置的是本站域名和本站 API Token，不是渠道 265 的真实上游地址和密钥。
- 不要在请求体中传 `channel_id: 265`；渠道选择由本站完成。
- 图片 URL 必须能被本站和上游通过公网访问。短时签名 URL 必须保证在素材导入完成前有效。
- JSON 中的布尔值和数字必须使用原生类型，不要把 `false`、`0`、`8` 写成字符串。
- 对网络超时可以重试查询；创建请求发生超时时，先根据业务侧幂等记录确认是否已取得任务 ID，避免盲目重复创建并产生重复计费。
- 结果 URL 可能带有效期。任务完成后如需长期保存，应由下游及时转存。

## 9. 下游本身也是 New API 时

如果下游也是 New API，但其版本没有 `TokenMartSeedance` 类型，不要伪造类型编号 `60`，也不要把渠道 265 的上游地址填入普通 OpenAI 渠道。

下游可选择以下任一方式：

1. 由下游业务服务直接调用本站的 `POST /v1/video/generations` 和任务查询接口。
2. 在下游增加一个最小的异步视频适配器，把本站作为上游。

最小适配器只需遵守以下契约：

| 阶段 | 方法和路径 | 处理要求 |
| --- | --- | --- |
| 提交 | `POST https://cf-api.o1key.com/v1/video/generations` | 原样发送本文第 4 节的 `content` 数组请求 |
| 保存任务 | 读取响应的 `id` | 把本站返回的 `task_...` 保存为下游的上游任务 ID |
| 查询 | `GET https://cf-api.o1key.com/v1/video/generations/{id}` | 携带同一个本站 API Token |
| 成功 | `status == "completed"` | 从 `metadata.url` 读取视频地址；也可读取 `metadata.outputs[0]` |
| 失败 | `status == "failed"` | 读取 `error.message` 和 `error.code` |

下游适配器不需要实现素材上传逻辑。只要把公网图片 URL 放进 `content[].image_url.url`，素材组创建、`POST /v1/assets`、素材状态查询和 `asset://` 替换都由本站完成。

不要把本接口改写成真实上游的 `POST /v1/video/generate`。该路径属于渠道 265 后面的内部上游协议，不是下游访问本站的公开入口。
