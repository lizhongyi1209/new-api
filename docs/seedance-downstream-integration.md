# Seedance 2.0 下游对接说明

## 1. 对接结论

下游不需要、也不应直接配置 `TokenMartSeedance` 渠道。下游只需把本站当作视频生成 API 服务，通过本站签发的 API Token 调用：

- 创建视频任务：`POST /v1/video/generations`
- 查询视频任务：`GET /v1/video/generations/{task_id}`

本站收到请求后，会根据模型和令牌分组路由到内部渠道。渠道 ID `265` 仅是本站内部配置，不是下游请求参数，不能通过请求体指定。

MAX 视频请求中的公网素材 URL 由上游自动准备，原有 v1 模型仍由本站创建素材并转换为 `asset://{asset_id}`。下游通常不需要单独调用素材接口。

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
  "result_url": "https://cdn.example.com/result.mp4",
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

建议每 5 至 10 秒查询一次，直到进入 `completed` 或 `failed` 终态。开启 `return_last_frame` 时，完成响应中的 `metadata.last_frame_url` 是尾帧 PNG 地址。

## 5. 本站内部素材创建流程

### 5.1 MAX v2 自动素材准备

当公开模型通过渠道模型映射指向对应的 MAX 上游模型时，本站内部使用 v2 视频接口。

**主推简写模型名（新接入推荐）：**

| 本站公开模型 | MAX 上游模型 |
| --- | --- |
| `dreamina-seedance-2-0` | `dreamina-seedance-2-0-260128-max` |
| `dreamina-seedance-2-0-fast` | `dreamina-seedance-2-0-fast-260128-max` |
| `dreamina-seedance-2-0-mini` | `dreamina-seedance-2-0-mini-260615-max` |
| `dreamina-seedance-2-5` | `dreamina-seedance-2-5-260628-max` |
| `doubao-seedance-2-0` | `doubao-seedance-2-0-260128-max` |
| `doubao-seedance-2-0-fast` | `doubao-seedance-2-0-fast-260128-max` |
| `doubao-seedance-2-0-mini` | `doubao-seedance-2-0-mini-260615-max` |
| `doubao-seedance-2-5` | `doubao-seedance-2-5-260628-max` |

**兼容旧名（Dreamina `-hc`，给老客户保留）：**

| 本站公开模型 | MAX 上游模型 |
| --- | --- |
| `dreamina-seedance-2-0-hc` | `dreamina-seedance-2-0-260128-max` |
| `dreamina-seedance-2-0-fast-hc` | `dreamina-seedance-2-0-fast-260128-max` |
| `dreamina-seedance-2-0-mini-hc` | `dreamina-seedance-2-0-mini-260615-max` |
| `dreamina-seedance-2-5-hc` | `dreamina-seedance-2-5-260628-max` |

国内 Doubao 也可继续直接使用长名别名（无需映射）：`doubao-seedance-2-0-260128-max`、`doubao-seedance-2-0-fast-260128-max`、`doubao-seedance-2-0-mini-260615-max`、`doubao-seedance-2-5-260628-max`。

下游创建和查询路径仍然是 `/v1/video/generations` 与 `/v1/video/generations/{task_id}`。切换 Dreamina / Doubao 时只需修改请求中的 `model`。本站根据映射后的上游模型前缀分别应用 Dreamina 或 Doubao 请求适配，两者当前均提交到上游 `/v2/video/generate` 并通过 `/v2/video/tasks/{id}` 查询。

Doubao MAX 和 Dreamina MAX 使用相同的请求参数。本站完整转发以下可选参数；未填写时使用上游默认值，显式 `false` 和 `0` 会保留：

| 参数 | MAX 支持范围与默认值 |
| --- | --- |
| `duration` | 2.0 / fast / mini 为 4–15 秒，2.5 为 4–30 秒；默认 5。2.5 编辑必须显式传 `-1`，输出时长基本跟随待编辑视频；其他任务的 `-1` 由模型自选 |
| `resolution` | 2.0 为 480p / 720p / 1080p / 4k，fast / mini 为 480p / 720p，2.5 为 480p / 720p / 1080p；默认 720p |
| `ratio` / `aspect_ratio` | 比例别名适用于两类 MAX；同时填写时以非空 `ratio` 为准，默认 adaptive；首尾帧模式跟随图片。2.5 首帧 / 首尾帧、编辑、延长仅支持 adaptive |
| `generate_audio` | 默认 true；false 关闭生成音频 |
| `watermark` | 默认 false；true 添加水印 |
| `seed` | 整数 [-1, 2147483647]，默认 -1；0 是有效种子；相同种子不保证完全一致 |
| `return_last_frame` | 默认 false；true 时从查询响应 `metadata.last_frame_url` 取尾帧 |
| `callback_url` | 状态变化时接收上游 POST 通知的地址 |
| `execution_expires_after` | 上游任务过期秒数 [3600, 259200]，默认 172800；本站任务超时策略独立生效 |
| `output_format` | 仅 2.5：mp4（默认）或 mov |
| `omni_reference_task_type` | 仅 2.5：auto（默认）、reference、edit 或 extend；提示词意图须与所选模式一致 |

Seedance 2.5 编辑与延长有额外参数限制，不能直接套用普通参考生视频的配置。依据[火山引擎 Seedance 2.5 官方教程](https://docs.volcengine.com/docs/ark/seedance-2-5?lang=zh)：

| 场景 | 素材与提示词 | 参数 |
| --- | --- | --- |
| 视频编辑 | 至少一段 `reference_video`，待编辑视频 4–30 秒；提示词明确编辑动作，例如“修改 @Video1 的背景” | `omni_reference_task_type: "edit"`、`ratio: "adaptive"`、`duration: -1`；输出比例与时长基本跟随原视频 |
| 视频延长 | 至少一段 `reference_video`；提示词明确方向与目标，例如“向后延长 @Video1” | `omni_reference_task_type: "extend"`、`ratio: "adaptive"`；输出时长可设为 4–30 秒或 `-1`，比例跟随原视频 |

MP4 与 MOV 均可用于 2.5；官方推荐编辑、延长使用 MOV 输入和输出，以改善色彩与声画衔接。设置 `omni_reference_task_type` 后仍需明确提示词意图，模型会结合提示词判断实际场景。

`callback_url` 会原样转发。回调体为上游的 `{"task": {...}}`，其中 `task.id` 是上游任务标识，和本站公开的 `task_...` 不同。建议在回调 URL 中带自己的业务 ID，提交成功后保存它与本站任务 ID 的关联；收到通知后通过本站查询接口确认结果，轮询保留作兜底。

2.0 / fast / mini 允许最多 9 张参考图、3 段参考视频和 3 段参考音频；视频和音频各自总时长不超过 15 秒，音频不能作为唯一参考素材。2.5 对应为 30 张图、10 段视频、10 段音频，各自总时长不超过 30 秒，允许音频作为唯一参考素材；两类均需非空提示词。参考素材与首尾帧不能混用，`@Image1` / `@Video1` 分别按参考图 / 视频在数组中的出现顺序编号。

MAX v2 请求中的公网图片和视频 URL 会原样转发，由上游视频任务自动完成素材准备；本站不会再调用旧素材接口或把图片改写为 `asset://`。上游 `preparing` 状态对外归一化为 `in_progress`，同时保留在响应元数据中：

```json
{
  "status": "in_progress",
  "metadata": {
    "upstream_status": "preparing",
    "prep": {
      "total": 7,
      "active": 0,
      "failed": 0,
      "attempt": 1
    }
  }
}
```

下游仍然调用本站的 `/v1/video/generations`，并继续使用公开模型名（推荐简写）和公开 `task_...` ID，不需要感知 MAX 上游模型名或上游 v2 路径。

只使用一次的素材仍推荐直接传公网 URL。需要反复渲染时再预上传。Doubao MAX 与 Dreamina MAX 都可以预上传：

| 素材 | 统一入口 `type` | 同路径代理 |
| --- | --- | --- |
| Dreamina MAX | `dreamina` | `POST` / `GET /v2/sd-max/assets` |
| Doubao MAX | `doubao` | `POST` / `GET /v2/db-sd-max/assets` |
| HC（兼容旧名） | `hc` | `POST` / `GET /v1/sd/assets` |

创建和查询都会按 `type`，以及创建请求里的可选 `model`，在当前令牌分组里选择渠道密钥。Dreamina / HC 只在配置了 Dreamina 模型的国外渠道中选择（线上为渠道 265 一类）；Doubao 只在配置了豆包模型的国内渠道中选择（线上为渠道 465 一类）。不会再取任意一个优先级最高的类型 60 渠道。

`asset://` 句柄只对创建它时实际选中的那把上游密钥及其组织有效。Dreamina 与豆包不能混用。同一家族如果有多把密钥或多个组织，只传 `type` 会选该分组里优先级最高的匹配渠道；生成若落到另一把密钥，句柄不能用。预上传请带上与生成请求相同的公开模型名，使两边都在提供该模型的渠道中选择。同一模型仍可能对应多把密钥，不能把句柄当作同家族或同模型下的任意渠道都能复用。

Dreamina 的 `model` 可以省略，省略后适用于该渠道上的全部 Dreamina MAX 模型。如填写，可使用本站公开名（推荐简写如 `dreamina-seedance-2-0-mini`，兼容 `-hc` 旧名）或上游名 `dreamina-*-max`；统一入口会按渠道映射改写成上游模型名再转发。Doubao 的 `model` 仍建议省略；如填写，使用 `doubao-seedance-2-0` 或 `doubao-seedance-2-0-260128-max`。直接调用 `/v2/sd-max/assets` 或 `/v2/db-sd-max/assets` 时，`URL`、`AssetType`、`Name`、`Model` 原样转发。

本站对异步视频任务明确返回的“素材不存在”（`asset ... not found`）提供自动恢复：使用原创建参数重新创建该素材，等待素材可用后，在原渠道重新提交视频。每个任务最多恢复 3 次，公开任务 ID 保持不变，沿用原预扣额度，最终失败才退款。其他错误不进入此恢复流程。

通过本站直接素材代理创建的素材，其创建参数按用户隔离保留 24 小时；视频任务的私有恢复参数在任务结束后清除，不进入公共响应或审计请求快照。缺少原始来源、来源已过期或旧任务没有恢复参数时，仍按正常失败处理。上游直接发送的 `callback_url` 通知仍由上游控制，可能包含某次尝试的失败状态；本站任务查询接口反映恢复后的整体状态。

### 5.2 v1 素材创建流程

当 `content` 中的 `image_url.url` 是公网 HTTP(S) URL 时，本站渠道 265 会先按模型选择素材工作流，再把图片转换为 `asset://{asset_id}`。

> **对外说明**：`df` 素材类型与 DF 上游模型名**不再对外推荐**。代码侧仍识别 DF 路径以兼容存量，但公开文档与新接入请勿使用 `type: "df"` 或 `*-df` 模型名。Dreamina MAX / 简写模型请直接传公网 URL（走 MAX v2 自动素材准备）。

兼容旧名 HC 模型若仍走 v1 素材路径，使用直接素材接口 `POST /v1/sd/assets`，并通过 `GET /v1/sd/assets/{asset_id}` 查询状态：

- `dreamina-seedance-2-0-hc`
- `dreamina-seedance-2-0-fast-hc`
- `dreamina-seedance-2-0-mini-hc`
- `dreamina-seedance-2-5-hc`

其他 ServiceInference 模型继续使用素材组工作流：

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

本站公开了素材代理接口。下游如需复用同一份图片、视频或音频，可以使用本站 API Token 调用以下接口。本站按素材类型选择对应渠道的上游密钥，下游不需要、也不能指定渠道 ID。

统一入口按 `type` 转发。`dreamina` 转发到 `/v2/sd-max/assets`，`doubao` 转发到 `/v2/db-sd-max/assets`，`hc`（兼容旧名）转发到 `/v1/sd/assets`。内部仍识别 `df` 但不对外推荐：

```http
POST https://cf-api.o1key.com/v1/seedance/assets
```

国外 Dreamina 复用素材时，按下面四步调用。示例中的 `mva-example-mini-001`、`task_example_dreamina_mini` 和图片地址都是占位，不是线上记录。`asset_type` 使用 `image` / `video` / `audio`，默认 `image`。

1. 创建素材。`model` 填写即将用于生成的简写：

```json
{
  "type": "dreamina",
  "model": "dreamina-seedance-2-0-mini",
  "url": "https://cdn.example.com/reference-mini.png",
  "name": "窗台参考图",
  "asset_type": "image"
}
```

响应立即返回，`data.Status` 先是 `Processing`。记下 `data.Id` 和 `data.Ref`。统一入口会按渠道映射把公开简写改成上游名再转发，`data.Model` 可能回显该上游名。

2. 查询直到 `Active`：

```http
GET https://cf-api.o1key.com/v1/seedance/assets/mva-example-mini-001?type=dreamina
```

每 1–2 秒一次。`data.Status` 为 `Active` 后才能引用；`Failed` 时看 `data.Error`。

3. 用 `data.Ref` 作为 `image_url.url` 提交生成。`model` 与预上传相同，示例为 480p、4 秒、关闭生成音频：

```json
{
  "model": "dreamina-seedance-2-0-mini",
  "content": [
    {
      "type": "text",
      "text": "一只小猫在窗台上晒太阳，镜头缓慢推近。"
    },
    {
      "type": "image_url",
      "image_url": {
        "url": "asset://mva-example-mini-001"
      },
      "role": "reference_image"
    }
  ],
  "duration": 4,
  "resolution": "480p",
  "generate_audio": false
}
```

句柄已就绪时，创建响应的 `status` 通常是 `queued`，`id` / `task_id` 为本站 `task_...`。

4. 用该任务 ID 轮询 `GET /v1/video/generations/{task_id}`，直到 `status` 为 `completed` 或 `failed`。完成后从 `result_url` 或 `metadata.url` 取视频，`metadata.outputs` 是完整列表。本站状态是 `queued`、`in_progress`、`completed`、`failed`，不是上游的 `pending` / `processing`。

`url` 在 Dreamina 与 Doubao 上接受公网 `http://` 或 `https://`。`model` 可省略，但同一家族有多把密钥时建议填写，并与生成请求使用同一个公开名。把 `type` 换成 `doubao` 或兼容旧名 `hc` 即走对应素材接口。国内豆包创建示例：

```json
{
  "type": "doubao",
  "url": "https://cdn.example.com/reference.jpg",
  "name": "人物参考图",
  "asset_type": "image"
}
```

状态查询必须带上创建时的 `type`：

```http
GET https://cf-api.o1key.com/v1/seedance/assets/mva-example-doubao-001?type=doubao
```

已经按上游字段组装请求时，也可以直接调用同路径代理。Dreamina MAX 为 `POST /v2/sd-max/assets` 与 `GET /v2/sd-max/assets/{Id}`，请求体使用 `URL`、`AssetType`、`Name`、`Model`。Doubao MAX 仍是 `/v2/db-sd-max/assets`。上传是 POST 到不带 id 的路径；带 id 的路径只用于查询。

素材句柄不能跨组织混用。`type: "dreamina"` 或兼容旧名 `hc` 创建的 `asset://` 只能交给创建它的那把 Dreamina 密钥上的视频请求；`type: "doubao"` 的句柄只能交给对应的豆包密钥。同一家族的其他密钥不能仅凭家族名复用该句柄。只使用一次的素材不必预上传，继续在视频请求里传公网 URL 即可。

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

## 7. 渠道公开模型映射

下游只能发送左侧的本站模型名，右侧上游模型名由本站内部转换。

**主推（MAX 简写，见第 5.1 节）：** `dreamina-seedance-2-0` / `fast` / `mini` / `2-5`，以及 `doubao-seedance-2-0` / `fast` / `mini` / `2-5`。兼容旧名：`dreamina-*-hc`。

**历史非 MAX 映射（存量兼容，新接入勿用）：**

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
| 成功 | `status == "completed"` | 旧客户端可继续读取顶层 `result_url`；新客户端也可读取 `metadata.url` 或 `metadata.outputs[0]` |
| 失败 | `status == "failed"` | 读取 `error.message` 和 `error.code` |

下游适配器不需要实现素材上传逻辑。把公网素材 URL 放进 `content` 即可；MAX 由上游自动准备，既有 v1 模型由本站按对应素材流程处理。

不要把本接口改写成真实上游的 `POST /v1/video/generate`。该路径属于渠道 265 后面的内部上游协议，不是下游访问本站的公开入口。
