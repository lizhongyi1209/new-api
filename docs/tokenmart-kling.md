# TokenMart Kling 任务插件

插件 key 是 `tokenmart-kling`。它把可灵官方 API 2.0 请求原样转发到 TokenMart 国内站。不改 `plugins/tasks/kling`（key `kling`，渠道类型 50），也不改站点现有的 `/kling/...` 路由。

下游路径和 TokenMart 根路径相同，没有 `/tokenmart-kling`，也没有 `/kling`。模型名保留 `-cn`。鉴权是 `Authorization: Bearer` 加渠道 API Key，不做 AK/SK 签名。

## 只启用这一个插件

生产环境保持任务插件总开关的现有策略。需要单独打开本插件时：

1. 将 `TaskPluginEnabled` 设为 `true`。
2. 将 `TaskPluginDisabledFactoryKeys` 设成除 `tokenmart-kling` 以外的全部内置插件 key（至少包含 `kling`、`sora`、`hailuo`、`vidu`、`google`、`vertex-ai`、`jimeng`、`doubao`、`alibaba`、`sunoapi`）。不要把 `tokenmart-kling` 放进这个列表。
3. 新建渠道，类型选 **Task Plugin**（62）。61 是 xinhankr，60 是 TokenMartSeedance。插件选 `tokenmart-kling`。Base URL 留空时使用 `https://model.service-inference.ai`。
4. 在渠道里勾选要用的模型。视频用 `kling-4.0-cn`、`kling-4.0-flash-cn`、`kling-3.0-omni-cn`、`kling-3.0-cn`、`kling-3.0-turbo-cn`、`kling-o1-cn`、`kling-2.6-cn`、`kling-2.5-turbo-cn`。图片和长图用 `kling-image-4.0-cn`。主体创建用 `kling-element-cn`。音色、主体列表和 `POST /tasks` 游标查询用 `kling-resource-cn`。

命中本插件的路由会钉住 key `tokenmart-kling`。分发只接受绑定了这个 key 的 Task Plugin（62）或 New API 渠道。渠道类型 50 仍只走 `/kling/...`。

## 下游路径

| 用途 | 方法与路径 |
| --- | --- |
| 文生视频 | `POST /text-to-video/{model}` |
| 图生视频 | `POST /image-to-video/{model}` |
| 全能参考 | `POST /omni-video/{model}` |
| 动作控制 | `POST /motion-control/{model}` |
| 图片 4.0 | `POST /image-generation/{model}` |
| 长图 | `POST /long-image-generation/{model}` |
| 按模型查一条任务 | `GET /{上面的段}/{model}/{task_id}` |
| 按 id 批量查询 | `GET /tasks?task_ids=` |
| 按外部 id 查询、游标分页 | `GET /tasks?external_task_ids=`、`POST /tasks` |
| 创建主体 | `POST /v1/general/advanced-custom-elements` |
| 主体列表、预设、删除 | `GET /v1/general/advanced-custom-elements`、`GET /v1/general/advanced-presets-elements`、`POST /v1/general/delete-advanced-elements` |
| 音色 | `POST /v1/general/custom-voices`、`GET /v1/general/custom-voices`、`GET /v1/general/custom-voices/{id}`、`GET /v1/general/presets-voices`、`POST /v1/general/delete-voices` |

发往 TokenMart 时路径不变。`{model}` 使用带 `-cn` 的名字。图片路径可以写 `kling-4.0-cn`，计费名是 `kling-image-4.0-cn`。

## 不能占用的路径

`POST /v1/videos/text2video`、`/v1/videos/image2video`、`GET /v1/videos/{task_id}` 与 OpenAI 兼容视频路由 `GET /v1/videos/:task_id` 相交，插件路由不能注册它们。旧版可灵接口继续用现有前缀：`POST /kling/v1/videos/text2video`，由渠道类型 50 处理。TokenMart 国内站官方 2.0 格式本身用的是上表里的根路径，不是 `/v1/videos/*`。

`POST /v1/images/generations` 和 `GET /v1/tasks/:key` 同样已被占用。图片 4.0 用 `/image-generation/{model}`，任务查询用 `GET /tasks`。

## 计费维度

价格由后台按插件 key 和模型配置，插件只上报用量：

- 视频：`output_seconds`、`resolution`（`720p` / `1080p` / `4k`）、`audio`、`dynamic_range`（`sdr` / `hdr`）、`input_video`、`watermark`。4.0 / 4.0 Flash 在完成时若同时有输入和输出时长，按 `max(input + output, output × 2)` 回填秒数。
- 图片和长图：`image_count`、`resolution`、`refer_images`、`result_type`、`skill`、`length`、`watermark`。
- 主体和资源请求：`requests`。列表、删除、游标查询如不收费，把 `kling-resource-cn` 的单价设为 0。

## 透传说明

官方请求体（`prompt` / `contents`、`settings`、`options`，含 `callback_url`、`watermark_info`、`dynamic_range`、`element`、`voice`）会原样送出。动作控制、主体库、音色、`GET /tasks` 和 `POST /tasks` 同样会送出。TokenMart 文档里写的「不支持」只是上游可能返回 400/404，或忽略 `callback_url`、不返回 `billing[]`；插件不会提前拒绝。素材 URL 需要能从中国大陆直接下载。

`GET /tasks?task_ids=` 查询的是本网关返回的任务 id。`external_task_ids` 和 `POST /tasks` 游标分页会转发给上游。
