# Kling 图生视频 API 文档

本文档记录 Kling 官方图生视频接口的完整参数规范和当前实现状态。

## 接口概览

### 创建任务
**POST** `/v1/videos/image2video`

创建图生视频任务，支持多种模式（标准、专家、4K）和高级功能（多镜头、音色定制、运动控制等）。

### 查询任务（单个）
**GET** `/v1/videos/image2video/{task_id}`

根据任务ID或自定义任务ID查询单个任务状态和结果。

### 查询任务（列表）
**GET** `/v1/videos/image2video?pageNum=1&pageSize=30`

分页查询任务列表。

---

## 请求参数详解

### 基础参数

| 参数名 | 类型 | 必填 | 默认值 | 说明 | 透传支持 |
|--------|------|------|--------|------|---------|
| `model_name` | string | 否 | kling-v1 | 模型名称。可选值：`kling-v1`, `kling-v1-5`, `kling-v1-6`, `kling-v2-master`, `kling-v2-1`, `kling-v2-1-master`, `kling-v2-5-turbo`, `kling-v2-6`, `kling-v3` | ✅ |
| `model` | string | 否 | - | 模型名称（向前兼容字段）。优先使用 `model_name` | ✅ |
| `image` | string | 否 | - | 参考图像，支持 Base64 或 URL（确保可访问）。与 `image_tail` 至少二选一 | ✅ |
| `image_tail` | string | 否 | - | 参考图像 - 尾帧控制。与 `image` 至少二选一 | ✅ |
| `prompt` | string | 否 | - | 正向文本提示词，最多 2500 字符 | ✅ |
| `negative_prompt` | string | 否 | - | 负向文本提示词，最多 2500 字符 | ✅ |
| `duration` | string | 否 | "5" | 生成视频时长（秒）。可选值：`3`-`15` | ✅ |
| `mode` | string | 否 | std | 生成模式。可选值：`std`（标准720P）、`pro`（高品质1080P）、`4k`（4K） | ✅ |
| `cfg_scale` | float | 否 | 0.5 | 生成自由度，取值范围 [0, 1]。值越大，与提示词相关性越强。kling-v2.x 不支持 | ✅ |
| `sound` | string | 否 | off | 是否生成声音。可选值：`on`, `off` | ✅ |

### 多镜头相关

| 参数名 | 类型 | 必填 | 默认值 | 说明 | 透传支持 |
|--------|------|------|--------|------|---------|
| `multi_shot` | boolean | 否 | false | 是否生成多镜头视频。为 true 时 `prompt` 无效 | ✅ |
| `shot_type` | string | 否 | - | 分镜方式。可选值：`customize`（自定义）、`intelligence`（智能）。`multi_shot` 为 true 时必填 | ✅ |
| `multi_prompt` | array | 否 | - | 各分镜信息（提示词、时长等）。最多 6 个分镜，每个分镜最长 512 字符 | ✅ |

**multi_prompt 数组元素格式**：
```json
{
  "index": 1,
  "prompt": "镜头描述",
  "duration": "5"
}
```

### 主体和音色

| 参数名 | 类型 | 必填 | 默认值 | 说明 | 透传支持 |
|--------|------|------|--------|------|---------|
| `element_list` | array | 否 | - | 参考主体列表，最多 3 个。基于主体库中主体的 ID 配置 | ✅ |
| `voice_list` | array | 否 | - | 引用的音色列表，最多 2 个。通过音色定制接口获得 voice_id | ✅ |

**element_list 数组元素格式**：
```json
{
  "element_id": 123456789
}
```

**voice_list 数组元素格式**：
```json
{
  "voice_id": "voice_id_xxx"
}
```

**音色使用说明**：
- 在 `prompt` 中使用 `<<<voice_1>>>` 引用音色，序号对应 `voice_list` 数组顺序
- 一次任务最多引用 2 个音色
- 指定音色时，`sound` 参数必须为 `on`
- 语法示例：`男人<<<voice_1>>>说："你好"`
- 当 `voice_list` 不为空且 `prompt` 中引用音色时，按"有指定音色"计费
- `element_list` 与 `voice_list` 互斥，不能共存

**主体引用说明**：
- 在 `prompt` 中使用 `<<<element_1>>>` 引用主体，序号对应 `element_list` 数组顺序
- 可结合 `<<<image_1>>>` 引用图片，`<<<video_1>>>` 引用视频

### 运动控制

| 参数名 | 类型 | 必填 | 默认值 | 说明 | 透传支持 |
|--------|------|------|--------|------|---------|
| `static_mask` | string | 否 | - | 静态笔刷涂抹区域（mask 图片 Base64 或 URL） | ✅ |
| `dynamic_masks` | array | 否 | - | 动态笔刷配置列表，最多 6 组 | ✅ |
| `camera_control` | object | 否 | - | 摄像机运动控制 | ✅ |

**dynamic_masks 数组元素格式**：
```json
{
  "mask": "base64_or_url",
  "trajectories": [
    {"x": 100, "y": 200},
    {"x": 150, "y": 250}
  ]
}
```

**camera_control 格式**：
```json
{
  "type": "camera_type",
  "config": {
    "horizontal": 0.5,
    "vertical": 0.5,
    "pan": 0.5,
    "tilt": 0.5,
    "roll": 0.5,
    "zoom": 0.5
  }
}
```

### 其他参数

| 参数名 | 类型 | 必填 | 默认值 | 说明 | 透传支持 |
|--------|------|------|--------|------|---------|
| `watermark_info` | object | 否 | - | 是否生成含水印的结果。格式：`{"enabled": true/false}` | ✅ |
| `callback_url` | string | 否 | - | 任务结果回调通知地址 | ✅ |
| `external_task_id` | string | 否 | - | 自定义任务 ID。单用户下需保证唯一性 | ✅ |

---

## 响应格式

### 创建任务响应

```json
{
  "code": 0,
  "message": "string",
  "request_id": "string",
  "data": {
    "task_id": "string",
    "task_info": {
      "external_task_id": "string"
    },
    "task_status": "submitted",
    "created_at": 1722769557708,
    "updated_at": 1722769557708
  }
}
```

**task_status 状态值**：
- `submitted`：已提交
- `processing`：处理中
- `succeed`：成功
- `failed`：失败

### 查询任务响应

```json
{
  "code": 0,
  "message": "string",
  "request_id": "string",
  "data": {
    "task_id": "string",
    "task_status": "succeed",
    "task_status_msg": "string",
    "watermark_info": {
      "enabled": false
    },
    "task_result": {
      "videos": [
        {
          "id": "string",
          "url": "string",
          "watermark_url": "string",
          "duration": "5"
        }
      ]
    },
    "task_info": {
      "external_task_id": "string"
    },
    "final_unit_deduction": "1.5",
    "created_at": 1722769557708,
    "updated_at": 1722769557708
  }
}
```

**重要说明**：
- 生成的图片/视频会在 **30 天后被清理**，请及时转存
- `final_unit_deduction` 为任务最终扣减的积分数值（人民币），用于精确计费

---

## 场景调用示例

### 1. 多镜头效果的图生视频

```bash
curl --location 'https://api-beijing.klingai.com/v1/videos/image2video' \
--header 'Authorization: Bearer xxx' \
--header 'Content-Type: application/json' \
--data '{
    "model_name": "kling-v3",
    "image": "https://example.com/image.jpg",
    "prompt": "",
    "multi_shot": true,
    "shot_type": "customize",
    "multi_prompt": [
        {
            "index": 1,
            "prompt": "Two friends talking under a streetlight at night.",
            "duration": "2"
        },
        {
            "index": 2,
            "prompt": "A runner sprinting through a forest, leaves flying.",
            "duration": "3"
        }
    ],
    "negative_prompt": "",
    "duration": "5",
    "mode": "pro",
    "sound": "on"
}'
```

### 2. 引用主体及主体音色的图生视频

```bash
curl --location 'https://api-beijing.klingai.com/v1/videos/image2video' \
--header 'Authorization: Bearer xxx' \
--header 'Content-Type: application/json' \
--data '{
    "model_name": "kling-v3",
    "image": "https://example.com/image1.jpg",
    "image_tail": "https://example.com/image2.jpg",
    "prompt": "The girl with <<<element_1>>> (using <<<voice_1>>>) communicates with the girl with <<<image_1>>> (using <<<voice_2>>>)",
    "element_list": [
        {"element_id": 123456789}
    ],
    "voice_list": [
        {"voice_id": "voice_id_1"},
        {"voice_id": "voice_id_2"}
    ],
    "negative_prompt": "",
    "duration": "9",
    "mode": "std",
    "sound": "on"
}'
```

### 3. 指定音色生成视频

```bash
curl --location 'https://api-beijing.klingai.com/v1/videos/image2video' \
--header 'Authorization: Bearer xxx' \
--header 'Content-Type: application/json' \
--data '{
    "model_name": "kling-v2-6",
    "image": "https://example.com/person.jpg",
    "prompt": "<<<voice_1>>>让图中人物说出以下文字：'\''热烈欢迎大家'\''",
    "voice_list": [
        {"voice_id": "your_voice_id"}
    ],
    "duration": "5",
    "mode": "pro",
    "sound": "on"
}'
```

**注意**：指定台词时需要加引号。

---

## 能力地图

不同模型版本对各功能的支持情况详见 [Kling 官方能力地图](https://app.klingai.com/cn/dev/document-api/capabilities)。

---

## 透传支持状态

✅ **完全支持透传** - 所有官方文档中的字段均已支持，包括：

- ✅ 基础参数：`model_name`, `image`, `image_tail`, `prompt`, `negative_prompt`, `duration`, `mode`, `cfg_scale`, `sound`
- ✅ 多镜头：`multi_shot`, `shot_type`, `multi_prompt`
- ✅ 主体和音色：`element_list`, `voice_list`
- ✅ 运动控制：`static_mask`, `dynamic_masks`, `camera_control`
- ✅ 其他：`watermark_info`, `callback_url`, `external_task_id`

---

## 更新日期

2026-06-28：添加 `voice_list` 支持，完成所有官方字段的透传实现。
