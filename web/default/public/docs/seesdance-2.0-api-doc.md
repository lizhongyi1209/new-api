# Seedance 2.0 视频生成 API 文档

## 概述

Seedance 2.0 支持多模态参考、首帧、首尾帧和文生视频四种生成方式，可以基于文本、图片、视频和音频生成高质量视频内容。

**基础信息：**

- 基础 URL: `https://cf-api.o1key.com`
- 认证方式: Bearer Token
- 模型名称: `seedance-2-0-260128-d-ep`

---

## 接口列表

### 1. 创建视频生成任务

#### 请求

**端点:** `POST /v1/video/generations`

**Headers:**

```
Content-Type: application/json
Authorization: Bearer YOUR_API_KEY
```

**请求体参数:**

| 参数名            | 类型    | 必填 | 说明                                      |
| ----------------- | ------- | ---- | ----------------------------------------- |
| model             | string  | 是   | 模型名称，使用 `seedance-2-0-260128-d-ep` |
| content           | array   | 是   | 内容数组，包含文本提示、图片、音频等元素  |
| duration          | integer | 否   | 视频时长（秒），支持 4-15                 |
| resolution        | string  | 否   | 分辨率，例如：`480p`、`720p`、`1080p`     |
| ratio             | string  | 否   | 视频宽高比，例如：`16:9`、`9:16`、`1:1`   |
| generate_audio    | boolean | 否   | 是否生成同步音频                          |
| watermark         | boolean | 否   | 是否添加水印，默认 false                  |
| return_last_frame | boolean | 否   | 是否返回最后一帧图片，默认 false          |

**content 数组元素结构:**

每个 content 元素包含以下字段：

| 字段名    | 类型   | 说明                                                                                           |
| --------- | ------ | ---------------------------------------------------------------------------------------------- |
| type      | string | 内容类型：`text`、`image_url`、`video_url`、`audio_url`                                        |
| text      | string | 当 type 为 `text` 时，文本提示内容                                                             |
| image_url | object | 当 type 为 `image_url` 时，包含图片 URL                                                        |
| video_url | object | 当 type 为 `video_url` 时，包含视频 URL                                                        |
| audio_url | object | 当 type 为 `audio_url` 时，包含音频 URL                                                        |
| role      | string | 媒体用途：`reference_image`、`reference_video`、`reference_audio`、`first_frame`、`last_frame` |

**媒体 URL 对象结构:**

```json
{
  "url": "https://example.com/media.png"
}
```

**四种生成方式：**

| 生成方式   | `content` 组成                 | `role` 要求                                                                                                                        |
| ---------- | ------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------- |
| 多模态参考 | 文本（可选）+ 图片、视频或音频 | 图片、视频、音频分别使用 `reference_image`、`reference_video`、`reference_audio`；音频不能单独作为参考，至少还需一张图片或一个视频 |
| 首帧       | 文本（可选）+ 一张图片         | 图片使用 `first_frame`；只有一张图片时也可以省略 `role`                                                                            |
| 首尾帧     | 文本（可选）+ 两张图片         | 第一张使用 `first_frame`，第二张使用 `last_frame`，两个 `role` 均不可省略                                                          |
| 文生视频   | 一条文本                       | 不传媒体项和 `role`                                                                                                                |

首帧、首尾帧和多模态参考是互斥的生成方式，不要在同一个请求中混用 `first_frame`、`last_frame` 和 `reference_image`。

#### 请求示例

**示例 1：多模态参考生视频**

```bash
curl -X POST https://cf-api.o1key.com/v1/video/generations \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "model": "seedance-2-0-260128-d-ep",
    "content": [
      {
        "type": "text",
        "text": "以图片1中的人物为主体，参考视频1的运镜和动作节奏，并使用音频1作为背景音乐，生成电影感街头短片"
      },
      {
        "type": "image_url",
        "image_url": {
          "url": "https://example.com/character.jpg"
        },
        "role": "reference_image"
      },
      {
        "type": "video_url",
        "video_url": {
          "url": "https://example.com/camera-motion.mp4"
        },
        "role": "reference_video"
      },
      {
        "type": "audio_url",
        "audio_url": {
          "url": "https://example.com/background-music.mp3"
        },
        "role": "reference_audio"
      }
    ],
    "duration": 8,
    "resolution": "720p",
    "ratio": "16:9",
    "generate_audio": true,
    "watermark": false,
    "return_last_frame": true
  }'
```

**示例 2：首帧生视频**

```bash
curl -X POST https://cf-api.o1key.com/v1/video/generations \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "model": "seedance-2-0-260128-d-ep",
    "content": [
      {
        "type": "text",
        "text": "人物缓慢转身看向镜头，微风吹动头发，镜头平滑向前推进"
      },
      {
        "type": "image_url",
        "image_url": {
          "url": "https://example.com/first-frame.jpg"
        },
        "role": "first_frame"
      }
    ],
    "duration": 5,
    "resolution": "720p",
    "ratio": "16:9",
    "generate_audio": true,
    "watermark": false,
    "return_last_frame": true
  }'
```

**示例 3：首尾帧生视频**

```bash
curl -X POST https://cf-api.o1key.com/v1/video/generations \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "model": "seedance-2-0-260128-d-ep",
    "content": [
      {
        "type": "text",
        "text": "从雨夜街头平滑过渡到清晨海边，人物动作和镜头运动保持自然连续"
      },
      {
        "type": "image_url",
        "image_url": {
          "url": "https://example.com/first-frame.jpg"
        },
        "role": "first_frame"
      },
      {
        "type": "image_url",
        "image_url": {
          "url": "https://example.com/last-frame.jpg"
        },
        "role": "last_frame"
      }
    ],
    "duration": 8,
    "resolution": "720p",
    "ratio": "16:9",
    "generate_audio": true,
    "watermark": false,
    "return_last_frame": true
  }'
```

**示例 4：文生视频**

```bash
curl -X POST https://cf-api.o1key.com/v1/video/generations \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "model": "seedance-2-0-260128-d-ep",
    "content": [
      {
        "type": "text",
        "text": "电影感航拍镜头掠过日出时的未来海滨城市，云层流动，暖色阳光映照玻璃建筑"
      }
    ],
    "duration": 8,
    "resolution": "720p",
    "ratio": "16:9",
    "generate_audio": true,
    "watermark": false,
    "return_last_frame": false
  }'
```

#### 响应

**成功响应 (200 OK):**

```json
{
  "id": "task_abc123def456",
  "task_id": "task_abc123def456",
  "status": "queued",
  "progress": 0,
  "created_at": 1735862400,
  "model": "seedance-2-0-260128-d-ep"
}
```

**响应字段说明:**

| 字段名     | 类型    | 说明                                                                                           |
| ---------- | ------- | ---------------------------------------------------------------------------------------------- |
| id         | string  | 任务 ID                                                                                        |
| task_id    | string  | 任务 ID（与 id 相同）                                                                          |
| status     | string  | 任务状态：`queued`（排队中）、`in_progress`（处理中）、`completed`（已完成）、`failed`（失败） |
| progress   | integer | 进度百分比 (0-100)                                                                             |
| created_at | integer | 创建时间戳（Unix 时间戳）                                                                      |
| model      | string  | 使用的模型名称                                                                                 |

**错误响应:**

```json
{
  "error": {
    "message": "错误描述信息",
    "type": "invalid_request_error",
    "code": "invalid_api_key"
  }
}
```

---

### 2. 查询任务状态

#### 请求

**端点:** `GET /v1/video/generations/{task_id}`

**Headers:**

```
Authorization: Bearer YOUR_API_KEY
Accept: application/json
```

**路径参数:**

| 参数名  | 类型   | 必填 | 说明                        |
| ------- | ------ | ---- | --------------------------- |
| task_id | string | 是   | 任务 ID，从创建任务接口返回 |

#### 请求示例

```bash
curl -X GET https://cf-api.o1key.com/v1/video/generations/task_abc123def456 \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Accept: application/json"
```

**Python 示例:**

```python
import requests
import time

task_id = "task_abc123def456"
url = f"https://cf-api.o1key.com/v1/video/generations/{task_id}"
headers = {
    "Authorization": "Bearer YOUR_API_KEY",
    "Accept": "application/json"
}

# 轮询任务状态
while True:
    response = requests.get(url, headers=headers)
    result = response.json()

    status = result.get("status")
    print(f"任务状态: {status}, 进度: {result.get('progress')}%")

    if status == "completed":
        print(f"视频 URL: {result['metadata']['url']}")
        if "last_frame_url" in result.get("metadata", {}):
            print(f"最后一帧: {result['metadata']['last_frame_url']}")
        break
    elif status == "failed":
        print(f"任务失败: {result.get('error', {}).get('message')}")
        break

    time.sleep(3)  # 每 3 秒查询一次
```

#### 响应

**排队中 (queued):**

```json
{
  "id": "task_abc123def456",
  "task_id": "task_abc123def456",
  "status": "queued",
  "progress": 0,
  "created_at": 1735862400,
  "model": "seedance-2-0-260128-d-ep"
}
```

**处理中 (in_progress):**

```json
{
  "id": "task_abc123def456",
  "task_id": "task_abc123def456",
  "status": "in_progress",
  "progress": 50,
  "created_at": 1735862400,
  "model": "seedance-2-0-260128-d-ep"
}
```

**已完成 (completed):**

```json
{
  "id": "task_abc123def456",
  "task_id": "task_abc123def456",
  "status": "completed",
  "progress": 100,
  "created_at": 1735862400,
  "completed_at": 1735862480,
  "model": "seedance-2-0-260128-d-ep",
  "metadata": {
    "url": "https://cdn.example.com/output-video.mp4",
    "outputs": ["https://cdn.example.com/output-video.mp4"],
    "last_frame_url": "https://cdn.example.com/last-frame.jpg",
    "usage": {
      "completion_tokens": 40594,
      "total_tokens": 40594
    }
  }
}
```

**失败 (failed):**

```json
{
  "id": "task_abc123def456",
  "task_id": "task_abc123def456",
  "status": "failed",
  "progress": 100,
  "created_at": 1735862400,
  "completed_at": 1735862450,
  "model": "seedance-2-0-260128-d-ep",
  "error": {
    "message": "生成失败：图片格式不支持",
    "code": "upstream_error"
  }
}
```

**响应字段说明:**

| 字段名                  | 类型    | 说明                                                   |
| ----------------------- | ------- | ------------------------------------------------------ |
| id                      | string  | 任务 ID                                                |
| task_id                 | string  | 任务 ID                                                |
| status                  | string  | 任务状态                                               |
| progress                | integer | 进度百分比 (0-100)                                     |
| created_at              | integer | 创建时间戳                                             |
| completed_at            | integer | 完成时间戳（仅完成或失败时）                           |
| model                   | string  | 模型名称                                               |
| metadata                | object  | 元数据（仅完成时），包含视频 URL 等信息                |
| metadata.url            | string  | 主视频 URL                                             |
| metadata.outputs        | array   | 所有输出视频 URL 数组                                  |
| metadata.last_frame_url | string  | 最后一帧图片 URL（如果请求时设置了 return_last_frame） |
| metadata.usage          | object  | Token 使用情况                                         |
| error                   | object  | 错误信息（仅失败时）                                   |
| error.message           | string  | 错误描述                                               |
| error.code              | string  | 错误代码                                               |

---

## 使用说明

### 1. 图片要求

- **格式**: JPG、PNG
- **尺寸**: 建议 512x512 或更高
- **内容**: 清晰的人物正面照，面部特征明显
- **URL**: 必须是可公开访问的 HTTPS URL

### 2. 音频要求

- **格式**: MP3、WAV
- **时长**: 应与视频 duration 参数匹配
- **URL**: 必须是可公开访问的 HTTPS URL

### 3. 参数建议

- **duration**: 支持 4-15 秒；时长越长，生成耗时和费用通常越高
- **resolution**:
  - `480p`: 快速生成，适合预览
  - `720p`: 平衡质量与速度
  - `1080p`: 高质量，生成较慢
- **ratio**:
  - `16:9`: 横屏视频
  - `9:16`: 竖屏视频（适合短视频平台）
  - `1:1`: 正方形（适合社交媒体）

### 4. 轮询建议

- 建议轮询间隔: 2-5 秒
- 超时时间: 建议设置 5 分钟
- 状态检查: 判断 `status` 字段是否为 `completed` 或 `failed`

### 5. 错误处理

常见错误及解决方案：

| 错误信息                                                        | 原因              | 解决方案                                       |
| --------------------------------------------------------------- | ----------------- | ---------------------------------------------- |
| `invalid_api_key`                                               | API Key 无效      | 检查 Authorization header 是否正确             |
| `missing_model`                                                 | 缺少 model 参数   | 确保请求中包含 model 字段                      |
| `invalid_request`                                               | 请求格式错误      | 检查 JSON 格式和必填字段                       |
| `content text is required`                                      | 缺少文本提示      | content 数组中至少包含一个 type 为 text 的元素 |
| `service inference image asset url must be http(s) or asset://` | 图片 URL 格式错误 | 确保图片 URL 以 http:// 或 https:// 开头       |

---

## 完整示例代码

### Node.js / TypeScript

```typescript
import axios from 'axios'

const API_KEY = 'YOUR_API_KEY'
const BASE_URL = 'https://cf-api.o1key.com'

async function generateVideo() {
  try {
    // 1. 创建任务
    const createResponse = await axios.post(
      `${BASE_URL}/v1/video/generations`,
      {
        model: 'seedance-2-0-260128-d-ep',
        content: [
          {
            type: 'text',
            text: '一个人在微笑并挥手',
          },
          {
            type: 'image_url',
            image_url: {
              url: 'https://example.com/person.jpg',
            },
            role: 'reference_image',
          },
        ],
        duration: 4,
        resolution: '720p',
        ratio: '16:9',
        generate_audio: false,
        watermark: false,
        return_last_frame: true,
      },
      {
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${API_KEY}`,
        },
      }
    )

    const taskId = createResponse.data.task_id
    console.log('任务创建成功，task_id:', taskId)

    // 2. 轮询任务状态
    while (true) {
      const statusResponse = await axios.get(
        `${BASE_URL}/v1/video/generations/${taskId}`,
        {
          headers: {
            Authorization: `Bearer ${API_KEY}`,
            Accept: 'application/json',
          },
        }
      )

      const { status, progress, metadata, error } = statusResponse.data
      console.log(`状态: ${status}, 进度: ${progress}%`)

      if (status === 'completed') {
        console.log('视频生成成功！')
        console.log('视频 URL:', metadata.url)
        if (metadata.last_frame_url) {
          console.log('最后一帧:', metadata.last_frame_url)
        }
        console.log('Token 使用:', metadata.usage)
        break
      } else if (status === 'failed') {
        console.error('视频生成失败:', error.message)
        break
      }

      // 等待 3 秒后继续查询
      await new Promise((resolve) => setTimeout(resolve, 3000))
    }
  } catch (error) {
    console.error('发生错误:', error.response?.data || error.message)
  }
}

generateVideo()
```

### Python (详细版)

```python
import requests
import time
import json
from typing import Dict, Optional

class SeedanceClient:
    def __init__(self, api_key: str, base_url: str = "https://cf-api.o1key.com"):
        self.api_key = api_key
        self.base_url = base_url
        self.headers = {
            "Content-Type": "application/json",
            "Authorization": f"Bearer {api_key}"
        }

    def create_video_task(
        self,
        prompt: str,
        image_url: Optional[str] = None,
        audio_url: Optional[str] = None,
        duration: int = 4,
        resolution: str = "720p",
        ratio: str = "16:9",
        generate_audio: bool = False,
        watermark: bool = False,
        return_last_frame: bool = True
    ) -> Dict:
        """创建视频生成任务"""
        content = [{"type": "text", "text": prompt}]

        if image_url:
            content.append({
                "type": "image_url",
                "image_url": {"url": image_url},
                "role": "reference_image"
            })

        if audio_url:
            content.append({
                "type": "audio_url",
                "audio_url": {"url": audio_url},
                "role": "reference_audio"
            })

        payload = {
            "model": "seedance-2-0-260128-d-ep",
            "content": content,
            "duration": duration,
            "resolution": resolution,
            "ratio": ratio,
            "generate_audio": generate_audio,
            "watermark": watermark,
            "return_last_frame": return_last_frame
        }

        response = requests.post(
            f"{self.base_url}/v1/video/generations",
            headers=self.headers,
            json=payload
        )
        response.raise_for_status()
        return response.json()

    def get_task_status(self, task_id: str) -> Dict:
        """查询任务状态"""
        response = requests.get(
            f"{self.base_url}/v1/video/generations/{task_id}",
            headers={
                "Authorization": f"Bearer {self.api_key}",
                "Accept": "application/json"
            }
        )
        response.raise_for_status()
        return response.json()

    def wait_for_completion(
        self,
        task_id: str,
        poll_interval: int = 3,
        timeout: int = 300,
        callback=None
    ) -> Dict:
        """等待任务完成"""
        start_time = time.time()

        while True:
            if time.time() - start_time > timeout:
                raise TimeoutError(f"任务 {task_id} 超时")

            result = self.get_task_status(task_id)
            status = result.get("status")
            progress = result.get("progress", 0)

            if callback:
                callback(status, progress, result)

            if status == "completed":
                return result
            elif status == "failed":
                error_msg = result.get("error", {}).get("message", "未知错误")
                raise Exception(f"任务失败: {error_msg}")

            time.sleep(poll_interval)

# 使用示例
if __name__ == "__main__":
    client = SeedanceClient(api_key="YOUR_API_KEY")

    # 创建任务
    print("正在创建视频生成任务...")
    task = client.create_video_task(
        prompt="一个人在开心地唱歌跳舞",
        image_url="https://example.com/person.jpg",
        audio_url="https://example.com/music.mp3",
        duration=5,
        resolution="1080p",
        ratio="9:16",
        generate_audio=True,
        watermark=False,
        return_last_frame=True
    )

    task_id = task["task_id"]
    print(f"任务创建成功！task_id: {task_id}")

    # 等待完成
    def progress_callback(status, progress, result):
        print(f"[{status}] 进度: {progress}%")

    try:
        result = client.wait_for_completion(
            task_id,
            poll_interval=3,
            timeout=300,
            callback=progress_callback
        )

        print("\n视频生成成功！")
        print(f"视频 URL: {result['metadata']['url']}")
        if "last_frame_url" in result.get("metadata", {}):
            print(f"最后一帧: {result['metadata']['last_frame_url']}")
        print(f"Token 使用: {result['metadata']['usage']}")

    except Exception as e:
        print(f"发生错误: {e}")
```

---

## 计费说明

- 按 token 计费，具体价格请联系服务商
- 视频时长越长，分辨率越高，消耗的 token 越多
- 可在任务完成后通过 `metadata.usage` 查看实际消耗的 token 数量

---

## 技术支持

如有问题，请联系技术支持或查看完整 API 文档。

**文档版本**: v1.1

**更新日期**: 2026-08-12
**生成工具**: Claude Code
