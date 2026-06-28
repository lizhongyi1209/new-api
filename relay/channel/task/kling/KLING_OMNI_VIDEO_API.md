# Kling Omni Video API 官方文档

> 本文档来自可灵官方 API 文档，用于接口开发和排查问题
> 
> 文档日期：2026-06-28

## Omni 视频生成

### 创建任务

**接口：** `POST /v1/videos/omni-video`

Omni 模型可以通过 Prompt 结合元素、图片、视频等内容实现多种能力。

#### 请求头

- `Content-Type`: `application/json` (必填)
- `Authorization`: `Bearer <token>` (必填)

#### 请求参数

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| model_name | string | 可选 | kling-video-o1 | 模型名称，枚举值：`kling-video-o1`, `kling-v3-omni` |
| multi_shot | boolean | 可选 | false | 是否生成多镜头视频。为 true 时，prompt 参数无效；为 false 时，shot_type 和 multi_prompt 参数无效 |
| shot_type | string | 可选 | - | 分镜方式，枚举值：`customize`, `intelligence`。当 multi_shot 为 true 时必填 |
| prompt | string | 可选 | - | 文本提示词，可包含正向描述和负向描述。不能超过 2,500 个字符。通过 `<<<>>>` 格式指定主体、图片或视频，如：`<<<element_1>>>`、`<<<image_1>>>`、`<<<video_1>>>` |
| multi_prompt | array | 可选 | - | 各分镜信息，通过 index、prompt、duration 参数定义分镜序号及相应提示词和时长。最多支持 6 个分镜，最小支持 1 个分镜。每个分镜相关内容的最大长度不超过 512。当 multi_shot 为 true 且 shot_type 为 customize 时必填 |
| image_list | array | 可选 | - | 参考图列表，包括主体、场景、风格等参考图片，也可作为首帧或尾帧生成视频 |
| element_list | array | 可选 | - | 主体参考列表，基于主体库中主体的ID配置 |
| video_list | array | 可选 | - | 参考视频，可作为特征参考视频或待编辑视频 |
| sound | string | 可选 | off | 生成视频时是否同时生成声音，枚举值：`on`, `off` |
| mode | string | 可选 | pro | 生成视频的模式，枚举值：`std`, `pro`, `4k` |
| aspect_ratio | string | 可选 | - | 生成视频的画面纵横比（宽:高），枚举值：`16:9`, `9:16`, `1:1` |
| duration | string | 可选 | 5 | 生成视频时长，单位秒，枚举值：`3`-`15` |
| watermark_info | object | 可选 | - | 是否同时生成含水印的结果，格式：`{"enabled": boolean}` |
| callback_url | string | 可选 | - | 本次任务结果回调通知地址 |
| external_task_id | string | 可选 | - | 自定义任务ID，单用户下需保证唯一性 |

#### image_list 详细说明

```json
"image_list": [
  {
    "image_url": "图片 URL 或 Base64",
    "type": "first_frame" // 可选，枚举值：first_frame, end_frame
  }
]
```

**图片要求：**
- 支持传入图片Base64编码或图片URL（确保可访问）
- 格式：.jpg / .jpeg / .png
- 文件大小：≤10MB
- 尺寸：宽和高都不小于300px，宽高比1:2.5 ~ 2.5:1

**数量限制：**
- 无参考视频+仅有多图主体时，参考图片与多图主体数量之和不得超过7
- 无参考视频+有视频主体时，参考图片与多图主体数量之和不得超过4
- 有参考视频+仅有多图主体时，参考图片与多图主体数量之和不得超过4
- 使用kling-video-o1模型时，数组中超过2张图片时，不支持设置首尾帧

#### element_list 详细说明

```json
"element_list": [
  {
    "element_id": long // 主体库中的主体 ID
  }
]
```

主体分为视频定制主体（视频角色主体）和图片定制主体（多图主体），适用范围不同。

**数量限制：**
- 使用首帧或首尾帧生成视频时，kling-v3-omni模型最多支持3个主体
- 使用首尾帧生成视频时，kling-video-o1模型不支持主体
- 无参考视频+仅有多图主体时，参考图片与多图主体数量之和不得超过7
- 无参考视频+仅有视频角色主体时，视频角色主体数量不得超过3
- 无参考视频+同时有视频角色主体和多图主体时，视频角色主体数量不得超过3，参考图片与多图主体数量之和不得超过4
- 有参考视频+仅有多图主体时，参考图片与多图主体数量之和不得超过4
- 有参考视频时，不支持使用视频角色主体

#### video_list 详细说明

```json
"video_list": [
  {
    "video_url": "视频 URL",
    "refer_type": "base", // 可选，默认 base，枚举值：feature, base
    "keep_original_sound": "yes" // 可选，枚举值：yes, no
  }
]
```

- `base` - 视频编辑：修改视频，如增加/删除/修改内容（主体/背景/局部/视频风格/物体颜色/天气等），切换景别/视角
- `feature` - 视频参考：参考视频内容生成下一个镜头/上一个镜头，或者参考视频的风格/运镜方式进行视频生成

**视频要求：**
- 格式：仅支持 MP4/MOV
- 时长：不少于 3 秒
- 分辨率：720px-2160px（宽高尺寸）
- 帧率：24-60fps（生成视频时会输出为 24fps）
- 至多 1 段视频，大小≤200MB，宽高比 1:2.5 ~ 2.5:1

**使用限制：**
- 参考视频为待编辑视频（base）时，不能定义视频首尾帧；且不支持生成多镜头视频，multi_shot 参数需为 false
- 参考视频为特征参考视频（feature）时，可通过智能分镜的方式生成多镜头视频，多镜头通过 prompt 参数定义；此时，shot_type 参数需为 intelligence
- 有参考视频时，sound 参数值只能为 off

#### 响应示例

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
    "task_status": "submitted", // 枚举值：submitted, processing, succeed, failed
    "created_at": 1722769557708,
    "updated_at": 1722769557708
  }
}
```

### 查询任务（单个）

**接口：** `GET /v1/videos/omni-video/{task_id}`

通过 ID 查询单个任务的状态和结果。

#### 路径参数

- `task_id`: 任务ID（系统生成）或通过 `external_task_id` 查询参数查询

#### 响应示例

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
    "final_unit_deduction": "1.5", // 任务最终扣减积分数值（RMB）
    "created_at": 1722769557708,
    "updated_at": 1722769557708
  }
}
```

### 查询任务（列表）

**接口：** `GET /v1/videos/omni-video?pageNum=1&pageSize=30`

分页查询任务列表。

#### 查询参数

- `pageNum`: 页码，默认 1，取值范围：[1, 1000]
- `pageSize`: 每页数据量，默认 30，取值范围：[1, 500]

## 使用场景示例

### 1. 图片/主体参考

参考图片/主体里的角色/道具/场景等多种元素，灵活生成视频：

```bash
curl --location 'https://api-beijing.klingai.com/v1/videos/omni-video' \
--header 'Authorization: Bearer <token>' \
--header 'Content-Type: application/json' \
--data '{
    "model_name": "kling-video-o1",
    "prompt": "<<<image_1>>>在东京的街头漫步，偶遇<<<element_1>>>和<<<element_2>>>，并跳到<<<element_2>>>的怀里。视频画面风格与<<<image_2>>>相同",
    "image_list": [
        {"image_url": "xxxxx"},
        {"image_url": "xxxxx"}
    ],
    "element_list": [
        {"element_id": 123},
        {"element_id": 456}
    ],
    "mode": "pro",
    "aspect_ratio": "1:1",
    "duration": "7"
}'
```

### 2. 视频编辑

修改视频，如增加/删除/修改内容：

```bash
curl --location 'https://api-beijing.klingai.com/v1/videos/omni-video' \
--header 'Authorization: Bearer <token>' \
--header 'Content-Type: application/json' \
--data '{
    "model_name": "kling-video-o1",
    "prompt": "给<<<video_1>>>中的穿蓝衣服的女孩，戴上<<<image_1>>>中的王冠",
    "image_list": [{"image_url": "xxx"}],
    "video_list": [{
        "video_url":"xxxxxxxx",
        "refer_type":"base",
        "keep_original_sound":"yes"
    }],
    "mode": "pro"
}'
```

### 3. 视频参考

参考视频内容生成下一个镜头/上一个镜头：

```bash
curl --location 'https://api-beijing.klingai.com/v1/videos/omni-video' \
--header 'Authorization: Bearer <token>' \
--header 'Content-Type: application/json' \
--data '{
    "model_name": "kling-video-o1",
    "prompt": "基于<<<video_1>>>，生成下一个镜头",
    "video_list": [{
        "video_url":"xxxxxxxx",
        "refer_type":"feature",
        "keep_original_sound":"yes"
    }],
    "mode": "pro"
}'
```

### 4. 首尾帧

图生视频首尾帧：

```bash
curl --location 'https://api-beijing.klingai.com/v1/videos/omni-video' \
--header 'Authorization: Bearer <token>' \
--header 'Content-Type: application/json' \
--data '{
    "model_name": "kling-video-o1",
    "prompt": "视频中的人跳舞",
    "image_list": [
        {"image_url": "xxx", "type": "first_frame"},
        {"image_url": "xxx", "type": "end_frame"}
    ],
    "mode": "pro"
}'
```

### 5. 文生视频

```bash
curl --location 'https://api-beijing.klingai.com/v1/videos/omni-video' \
--header 'Authorization: Bearer <token>' \
--header 'Content-Type: application/json' \
--data '{
    "model_name": "kling-video-o1",
    "prompt": "视频中的人跳舞",
    "mode": "pro",
    "aspect_ratio": "1:1",
    "duration": "7"
}'
```

### 6. 多镜头视频

自定义分镜：

```bash
curl --location 'https://api-beijing.klingai.com/v1/videos/omni-video' \
--header 'Authorization: Bearer <token>' \
--header 'Content-Type: application/json' \
--data '{
    "model_name": "kling-v3-omni",
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
    "mode": "pro",
    "aspect_ratio": "16:9",
    "duration": "5"
}'
```

## FAQ

### 1. 生成视频时长（duration）什么情况支持、什么情况不支持？

- 文生，图生（不含首尾帧）：可选3~10s
- 有视频输入（video_list不为空）且使用视频编辑功能（类型=base）时：不可指定时长，跟视频对齐
- 其他情况：可选时长与模型版本有关，kling-video-o1 为 3~10s，kling-v3-omni 为 3~15s

### 2. 怎么进行视频延长？

可以通过"视频参考"来实现，传入一段视频，通过prompt驱动模型"生成下一个镜头"或者"生成上一个镜头"。

### 3. 生成视频宽高比（aspect_ratio）什么情况支持、什么情况不支持？

- 不支持：视频编辑，图生视频（包括首尾帧）
- 支持：文生视频，图片/主体参考，视频参考

## 计费说明

任务成功后，响应中会包含 `final_unit_deduction` 字段，表示实际扣除的积分数值（单位：RMB）。系统会根据该值进行差额结算。

## 能力地图

不同模型版本、视频模式支持范围不同，详见官方文档中的能力地图章节。
