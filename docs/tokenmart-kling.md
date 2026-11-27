# TokenMart Kling 任务插件

插件 key 是 `tokenmart-kling`。它把可灵官方 API 2.0 请求原样转发到 TokenMart 国内站，不改 `plugins/tasks/kling`（key `kling`，渠道类型 50），也不占用站点现有的 `/kling/...` 下游路由。

## 只启用这一个插件

生产环境应保持任务插件总开关按现有策略处理。需要单独打开本插件时：

1. 将 `TaskPluginEnabled` 设为 `true`。
2. 将 `TaskPluginDisabledFactoryKeys` 设成除 `tokenmart-kling` 以外的全部内置插件 key（至少包含 `kling`、`sora`、`hailuo`、`vidu`、`google`、`vertex-ai`、`jimeng`、`doubao`、`alibaba`、`sunoapi`）。不要把 `tokenmart-kling` 放进这个列表。
3. 新建渠道，类型选 **Task Plugin**（61），插件选 `tokenmart-kling`。Base URL 留空时使用插件默认值 `https://model.service-inference.ai`。密钥填 TokenMart 的 API Key，请求以 `Authorization: Bearer` 发送，不做 AK/SK 签名。
4. 在渠道里勾选要用的模型。视频用 `kling-4.0-cn`、`kling-4.0-flash-cn`、`kling-3.0-omni-cn`、`kling-3.0-cn`、`kling-3.0-turbo-cn`、`kling-o1-cn`、`kling-2.6-cn`、`kling-2.5-turbo-cn`。图片和长图用 `kling-image-4.0-cn`。主体创建用 `kling-element-cn`。音色、主体列表和 `POST /tasks` 游标查询用 `kling-resource-cn`。

下游路径带 `/tokenmart-kling` 前缀，例如 `POST /tokenmart-kling/text-to-video/kling-4.0-cn`。插件发往 TokenMart 时去掉该前缀，模型名仍保留 `-cn`。

## 计费维度

价格由后台按插件 key 和模型配置，插件只上报用量：

- 视频：`output_seconds`、`resolution`（`720p` / `1080p` / `4k`）、`audio`、`dynamic_range`（`sdr` / `hdr`）、`input_video`、`watermark`。4.0 / 4.0 Flash 在完成时若同时有输入和输出时长，按 `max(input + output, output × 2)` 回填秒数。
- 图片和长图：`image_count`、`resolution`、`refer_images`、`result_type`、`skill`、`length`、`watermark`。
- 主体和资源请求：`requests`。列表、删除、游标查询如不收费，把 `kling-resource-cn` 的单价设为 0。

## 透传说明

官方请求体（`prompt` / `contents`、`settings`、`options`，含 `callback_url`、`watermark_info`、`dynamic_range`、`element`、`voice`）会原样送出。动作控制、主体库、音色、`GET /tasks` 和 `POST /tasks` 同样会送出。TokenMart 文档里写的「不支持」只是上游可能返回 400/404，或忽略 `callback_url`、不返回 `billing[]`；插件不会提前拒绝。素材 URL 需要能从中国大陆直接下载。

`GET /tokenmart-kling/tasks?task_ids=` 查询的是本网关返回的任务 id。`external_task_ids` 和 `POST /tasks` 游标分页会转发给上游。
