# 可灵 (Kling) AI 视频生成接口开发指南

## 概述

本文档提供可灵 AI 视频生成接口的完整开发示例，帮助开发者快速集成视频生成功能。

### 支持的接口

| 接口名称 | 端点 | 功能描述 |
|---------|------|---------|
| 图生视频 | `/kling/v1/videos/image2video` | 将静态图片转换为动态视频 |
| 文生视频 | `/kling/v1/videos/text2video` | 根据文本描述直接生成视频 |
| 运动控制 | `/kling/v1/videos/motion-control` | 对现有视频进行运动控制处理 |
| 全能视频 | `/kling/v1/videos/omni-video` | 高级功能：多图片、自定义元素、声音绑定等 |
| 3.0 Omni | `/kling/omni-video/kling-3.0-omni` | 最新官方多模态接口，支持图片、视频、Element、原生音频、4K 与 1:1 |

---

## 一、图生视频接口示例

### 1.1 接口信息

**端点：** `POST /kling/v1/videos/image2video`

**功能：** 将静态图片转换为动态视频，支持首帧和尾帧控制。

**适用场景：**
- 电商产品展示动画
- 静态海报转动态宣传片
- 照片动画化处理
- 基于图片的创意视频制作

### 1.2 认证方式

使用 Bearer Token 认证，在 HTTP 请求头中添加：

```
Authorization: Bearer YOUR_API_KEY
```

### 1.3 请求参数

#### 必填参数

| 参数名 | 类型 | 说明 |
|--------|------|------|
| `prompt` | string | 视频生成的提示词描述（必填） |
| `image` | string | 输入图片 URL（与 image 字段二选一） |

#### 可选参数

| 参数名 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `model_name` | string | `kling-v2-6` | 模型版本：`kling-v1`, `kling-v1-6`, `kling-v2-6`, `kling-v2-master`, `kling-v3` |
| `negative_prompt` | string | - | 负面提示词，描述不希望出现的内容 |
| `mode` | string | `std` | 生成模式：`std`（标准模式，快速经济）或 `pro`（专业模式，高质量） |
| `duration` | string | `5` | 视频时长：`5` 或 `10` 秒 |
| `aspect_ratio` | string | `16:9` | 画幅比例：`1:1`（方形）、`16:9`（横屏）、`9:16`（竖屏） |
| `cfg_scale` | float | `0.7` | 提示词相关性（0-1），值越高越贴近提示词 |
| `camera_control` | object | - | 镜头控制参数（高级功能） |
| `callback_url` | string | - | 任务完成后的回调地址 |
| `external_task_id` | string | - | 外部任务 ID，用于关联业务系统 |

#### 镜头控制参数 (camera_control)

```json
{
  "type": "simple",
  "config": {
    "horizontal": 2.5,  // 水平移动（-10 到 10）
    "vertical": 0,      // 垂直移动（-10 到 10）
    "pan": 0,           // 水平旋转（-10 到 10）
    "tilt": 0,          // 垂直旋转（-10 到 10）
    "roll": 0,          // 画面旋转（-10 到 10）
    "zoom": 0           // 缩放（-10 到 10）
  }
}
```

### 1.4 请求示例

#### 示例 1：基础图生视频

```bash
curl -X POST https://your-api-domain.com/kling/v1/videos/image2video \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model_name": "kling-v2-6",
    "image": "https://example.com/images/cat.jpg",
    "prompt": "一只可爱的橘猫在阳光明媚的花园里悠闲地散步，蝴蝶在周围飞舞，微风轻拂草地",
    "negative_prompt": "模糊，低质量，扭曲，重影",
    "mode": "pro",
    "duration": "5",
    "aspect_ratio": "16:9"
  }'
```

#### 示例 2：使用镜头控制

```bash
curl -X POST https://your-api-domain.com/kling/v1/videos/image2video \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model_name": "kling-v2-master",
    "image": "https://example.com/images/landscape.jpg",
    "prompt": "镜头从左向右缓慢平移，展现壮丽的山景",
    "mode": "pro",
    "duration": "10",
    "aspect_ratio": "16:9",
    "camera_control": {
      "type": "simple",
      "config": {
        "horizontal": 5.0,
        "vertical": 0,
        "pan": 0,
        "tilt": 0,
        "roll": 0,
        "zoom": 0
      }
    }
  }'
```

#### 示例 3：Python 代码示例

```python
import requests
import json

# API 配置
API_KEY = "YOUR_API_KEY"
BASE_URL = "https://your-api-domain.com"

# 请求头
headers = {
    "Authorization": f"Bearer {API_KEY}",
    "Content-Type": "application/json"
}

# 请求参数
payload = {
    "model_name": "kling-v2-6",
    "image": "https://example.com/images/product.jpg",
    "prompt": "产品在旋转展示，背景有粒子特效飘动",
    "negative_prompt": "模糊，低质量",
    "mode": "pro",
    "duration": "5",
    "aspect_ratio": "1:1",
    "cfg_scale": 0.7
}

# 发送请求
response = requests.post(
    f"{BASE_URL}/kling/v1/videos/image2video",
    headers=headers,
    json=payload
)

# 处理响应
if response.status_code == 200:
    result = response.json()
    task_id = result.get("task_id")
    print(f"任务提交成功！任务ID: {task_id}")
    print(f"任务状态: {result.get('status')}")
else:
    print(f"请求失败: {response.status_code}")
    print(response.text)
```

#### 示例 4：JavaScript/Node.js 代码示例

```javascript
const axios = require('axios');

// API 配置
const API_KEY = 'YOUR_API_KEY';
const BASE_URL = 'https://your-api-domain.com';

// 图生视频函数
async function generateImageToVideo() {
  try {
    const response = await axios.post(
      `${BASE_URL}/kling/v1/videos/image2video`,
      {
        model_name: 'kling-v2-6',
        image: 'https://example.com/images/scene.jpg',
        prompt: '场景中的云朵缓慢飘动，树叶随风摇曳',
        mode: 'pro',
        duration: '5',
        aspect_ratio: '16:9'
      },
      {
        headers: {
          'Authorization': `Bearer ${API_KEY}`,
          'Content-Type': 'application/json'
        }
      }
    );

    console.log('任务提交成功！');
    console.log('任务ID:', response.data.task_id);
    console.log('任务状态:', response.data.status);
    
    return response.data.task_id;
  } catch (error) {
    console.error('请求失败:', error.response?.data || error.message);
    throw error;
  }
}

// 执行
generateImageToVideo();
```

### 1.5 响应示例

#### 成功响应（200 OK）

```json
{
  "task_id": "task_abc123xyz456",
  "status": "submitted",
  "created_at": 1704038400,
  "model": "kling-v2-6"
}
```

**响应字段说明：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `task_id` | string | 任务唯一标识符，用于查询任务状态 |
| `status` | string | 任务状态：`submitted`（已提交）、`processing`（处理中） |
| `created_at` | integer | 任务创建时间戳（Unix 时间戳，秒） |
| `model` | string | 实际使用的模型名称 |

#### 错误响应示例

```json
{
  "code": 400001,
  "message": "prompt is required"
}
```

常见错误码：

| 错误码 | 说明 |
|--------|------|
| 400001 | 参数错误（缺少必填参数） |
| 401001 | 认证失败（Token 无效或过期） |
| 403001 | 无权限访问 |
| 429001 | 请求频率超限 |
| 500001 | 服务器内部错误 |

### 1.6 查询任务状态

#### 接口信息

**端点：** `GET /kling/v1/videos/image2video/{task_id}`

**说明：** 视频生成是异步任务，提交后需要轮询此接口查询任务状态和获取结果。

#### 请求示例

```bash
curl -X GET https://your-api-domain.com/kling/v1/videos/image2video/task_abc123xyz456 \
  -H "Authorization: Bearer YOUR_API_KEY"
```

#### Python 轮询示例

```python
import requests
import time

def query_task_status(task_id, max_retries=60, interval=5):
    """
    轮询查询任务状态
    
    Args:
        task_id: 任务ID
        max_retries: 最大重试次数
        interval: 轮询间隔（秒）
    """
    headers = {
        "Authorization": f"Bearer {API_KEY}"
    }
    
    for i in range(max_retries):
        response = requests.get(
            f"{BASE_URL}/kling/v1/videos/image2video/{task_id}",
            headers=headers
        )
        
        if response.status_code == 200:
            result = response.json()
            status = result.get("status")
            
            print(f"[{i+1}/{max_retries}] 任务状态: {status}")
            
            if status == "succeeded":
                print("视频生成成功！")
                print(f"视频URL: {result.get('url')}")
                return result
            elif status == "failed":
                print(f"任务失败: {result.get('error', {}).get('message')}")
                return None
            
            # 继续等待
            time.sleep(interval)
        else:
            print(f"查询失败: {response.status_code}")
            time.sleep(interval)
    
    print("任务超时")
    return None

# 使用示例
task_id = "task_abc123xyz456"
result = query_task_status(task_id)
```

#### 响应示例（任务完成）

```json
{
  "task_id": "task_abc123xyz456",
  "status": "succeeded",
  "url": "https://cdn.example.com/videos/output_abc123.mp4",
  "format": "mp4",
  "metadata": {
    "duration": 5.0,
    "fps": 30,
    "width": 1920,
    "height": 1080
  }
}
```

**任务状态说明：**

| 状态 | 说明 |
|------|------|
| `submitted` | 任务已提交，等待处理 |
| `processing` | 正在生成中 |
| `succeeded` | 生成成功，可获取视频URL |
| `failed` | 生成失败 |

### 1.7 计费说明

**标准模式 (std)：**
- 5秒视频：约 0.6 元
- 10秒视频：约 1.2 元

**专业模式 (pro)：**
- 5秒视频：约 1.2 元
- 10秒视频：约 2.4 元

*注：实际费用以平台计费为准*

### 1.8 最佳实践

#### 提示词编写建议

✅ **好的提示词：**
```
"一只橘色的小猫在阳光明媚的花园里缓慢行走，周围有蝴蝶飞舞，绿色的草地上有露珠闪烁，背景是模糊的树木"
```

❌ **不好的提示词：**
```
"猫走"
```

**要点：**
- 详细描述场景、动作、氛围
- 包含具体的颜色、光线、背景信息
- 使用具体的动词（如"缓慢行走"而不是"动"）
- 中英文均可，但建议使用中文以获得更好的理解

#### 负面提示词建议

常用负面提示词：
```
"模糊，低质量，扭曲，重影，变形，失真，噪点，抖动，画面撕裂"
```

#### 图片要求

- **格式：** JPG、PNG
- **大小：** 建议不超过 10MB
- **分辨率：** 建议至少 512×512 像素
- **内容：** 清晰、主体明确的图片效果更好

#### 参数调优建议

1. **模型选择：**
   - `kling-v3`：最新模型，质量最高（推荐）
   - `kling-v2-6`：平衡质量与速度
   - `kling-v1`：快速生成，经济实惠

2. **模式选择：**
   - 测试阶段：使用 `std` 模式快速验证
   - 正式产品：使用 `pro` 模式获得最佳质量

3. **时长选择：**
   - 5秒：适合短视频、社交媒体
   - 10秒：适合完整的场景展示

4. **cfg_scale 调优：**
   - 0.3-0.5：生成结果更自然，但可能偏离提示词
   - 0.6-0.8：平衡自然度与准确性（推荐）
   - 0.9-1.0：严格遵循提示词，但可能不够自然

### 1.9 常见问题

**Q: 任务提交后多久能完成？**

A: 通常 5-10 分钟，取决于队列长度和视频时长。建议每 5 秒轮询一次任务状态。

**Q: 可以使用 base64 编码的图片吗？**

A: 可以，但建议使用 URL 方式，base64 会增加请求体积。

**Q: 生成的视频能保存多久？**

A: 视频 URL 通常 24 小时内有效，建议及时下载保存。

**Q: 如何获得更稳定的生成效果？**

A: 使用固定的 `seed` 参数（虽然当前接口未暴露，但后续可能支持）。

---

## 二、文生视频接口

### 2.1 接口信息

**端点：** `POST /kling/v1/videos/text2video`

**功能：** 纯文本生成视频，无需输入图片。

### 2.2 请求示例

```bash
curl -X POST https://your-api-domain.com/kling/v1/videos/text2video \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model_name": "kling-v3",
    "prompt": "一片宁静的湖泊，水面倒映着蓝天白云，微风轻拂湖面泛起涟漪，远处有几只天鹅在游动",
    "negative_prompt": "模糊，低质量，抖动",
    "mode": "pro",
    "duration": "5",
    "aspect_ratio": "16:9"
  }'
```

### 2.3 Python 示例

```python
def generate_text_to_video(prompt):
    """文生视频"""
    headers = {
        "Authorization": f"Bearer {API_KEY}",
        "Content-Type": "application/json"
    }
    
    payload = {
        "model_name": "kling-v3",
        "prompt": prompt,
        "mode": "pro",
        "duration": "5",
        "aspect_ratio": "16:9"
    }
    
    response = requests.post(
        f"{BASE_URL}/kling/v1/videos/text2video",
        headers=headers,
        json=payload
    )
    
    return response.json()

# 使用
result = generate_text_to_video("一只机器狗在未来城市中奔跑，霓虹灯闪烁，赛博朋克风格")
print(f"任务ID: {result['task_id']}")
```

### 2.4 查询任务状态

**端点：** `GET /kling/v1/videos/text2video/{task_id}`

用法与图生视频查询接口相同。

---

## 三、全能视频接口（高级功能）

### 3.1 接口信息

**端点：** `POST /kling/v1/videos/omni-video`

**功能：** 支持多图片、自定义元素、视频引用等高级功能。

### 3.2 特色功能

- **多图片输入：** 支持首帧和尾帧控制
- **自定义元素：** 引用预先创建的角色、物品等元素
- **视频引用：** 基于已有视频进行二次创作
- **声音控制：** 绑定音频或保留原声
- **水印控制：** 自定义是否显示水印

### 3.3 请求示例

#### 示例：多图片首尾帧控制

```python
payload = {
    "model_name": "kling-video-o1",
    "prompt": "从白天过渡到夜晚，城市天际线的颜色逐渐变化",
    "mode": "pro",
    "duration": "10",
    "aspect_ratio": "16:9",
    "image_list": [
        {
            "image_url": "https://example.com/city-day.jpg",
            "type": "first_frame"
        },
        {
            "image_url": "https://example.com/city-night.jpg",
            "type": "end_frame"
        }
    ],
    "watermark_info": {
        "enabled": False
    }
}

response = requests.post(
    f"{BASE_URL}/kling/v1/videos/omni-video",
    headers=headers,
    json=payload
)
```

### 3.4 查询任务状态

**端点：** `GET /kling/v1/videos/omni-video/{task_id}`

### 3.5 Kling 3.0 Omni 官方接口（额外支持）

该接口与上面的旧版 Omni Video 接口并存，不会替换或改变旧接口行为。

**创建端点：** `POST /kling/omni-video/kling-3.0-omni`

```json
{
  "contents": [
    {
      "type": "prompt",
      "text": "一款香水在镜面展台上缓慢旋转，柔和电影灯光"
    },
    {
      "type": "refer_image",
      "url": "https://example.com/product.png",
      "id": "image_1"
    }
  ],
  "settings": {
    "multi_shot": false,
    "audio": "native",
    "resolution": "1080p",
    "aspect_ratio": "1:1",
    "duration": 5
  },
  "options": {
    "callback_url": "https://example.com/callback",
    "external_task_id": "product-video-001",
    "watermark_info": {
      "enabled": false
    }
  }
}
```

`contents[].type` 支持 `prompt`、`first_frame`、`last_frame`、`refer_image`、`feature_video`、`base_video` 和 `element`。分辨率支持 `720p`、`1080p`、`4k`，画面比例支持 `16:9`、`9:16` 和 `1:1`。

**查询端点：** `GET /kling/omni-video/kling-3.0-omni/{task_id}`

查询时使用创建接口返回的 New API 任务 ID。网关在上游通过 Kling 官方 `GET /tasks?task_ids=...` 协议轮询任务，并继续沿用现有的任务归属校验、计费和结果代理机制。

---

## 四、完整工作流示例

### 4.1 端到端 Python 完整示例

```python
import requests
import time

class KlingVideoAPI:
    def __init__(self, api_key, base_url):
        self.api_key = api_key
        self.base_url = base_url
        self.headers = {
            "Authorization": f"Bearer {api_key}",
            "Content-Type": "application/json"
        }
    
    def image_to_video(self, image_url, prompt, **kwargs):
        """图生视频"""
        payload = {
            "image": image_url,
            "prompt": prompt,
            "model_name": kwargs.get("model_name", "kling-v2-6"),
            "mode": kwargs.get("mode", "pro"),
            "duration": kwargs.get("duration", "5"),
            "aspect_ratio": kwargs.get("aspect_ratio", "16:9"),
        }
        
        response = requests.post(
            f"{self.base_url}/kling/v1/videos/image2video",
            headers=self.headers,
            json=payload
        )
        response.raise_for_status()
        return response.json()
    
    def query_task(self, task_id, task_type="image2video"):
        """查询任务状态"""
        response = requests.get(
            f"{self.base_url}/kling/v1/videos/{task_type}/{task_id}",
            headers=self.headers
        )
        response.raise_for_status()
        return response.json()
    
    def wait_for_completion(self, task_id, task_type="image2video", 
                           max_wait=600, check_interval=5):
        """等待任务完成"""
        start_time = time.time()
        
        while time.time() - start_time < max_wait:
            result = self.query_task(task_id, task_type)
            status = result.get("status")
            
            print(f"任务状态: {status}")
            
            if status == "succeeded":
                return result
            elif status == "failed":
                raise Exception(f"任务失败: {result.get('error')}")
            
            time.sleep(check_interval)
        
        raise TimeoutError("任务超时")

# 使用示例
def main():
    # 初始化客户端
    client = KlingVideoAPI(
        api_key="YOUR_API_KEY",
        base_url="https://your-api-domain.com"
    )
    
    # 提交图生视频任务
    print("提交视频生成任务...")
    task = client.image_to_video(
        image_url="https://example.com/cat.jpg",
        prompt="一只可爱的小猫在花园里玩耍，蝴蝶围绕飞舞",
        mode="pro",
        duration="5"
    )
    
    task_id = task["task_id"]
    print(f"任务已提交，ID: {task_id}")
    
    # 等待完成
    print("等待视频生成...")
    result = client.wait_for_completion(task_id)
    
    # 获取结果
    video_url = result.get("url")
    print(f"✅ 视频生成成功！")
    print(f"视频URL: {video_url}")
    print(f"视频信息: {result.get('metadata')}")
    
    # 下载视频（可选）
    video_response = requests.get(video_url)
    with open("output.mp4", "wb") as f:
        f.write(video_response.content)
    print("视频已保存到 output.mp4")

if __name__ == "__main__":
    main()
```

---

## 五、附录

### 5.1 支持的模型列表

| 模型名称 | 说明 | 推荐场景 |
|---------|------|---------|
| `kling-v1` | 第一代模型 | 快速测试、成本敏感场景 |
| `kling-v1-6` | v1 改进版 | 日常使用 |
| `kling-v2-6` | 第二代模型 | 通用推荐 |
| `kling-v2-master` | v2 大师版 | 高质量需求 |
| `kling-v3` | 最新模型 | 最佳质量（推荐） |
| `kling-video-o1` | Omni Video 专用 | 多模态、高级功能 |

### 5.2 画幅比例选择指南

| 比例 | 分辨率示例 | 适用场景 |
|------|-----------|---------|
| `1:1` | 1080×1080 | Instagram 帖子、朋友圈 |
| `16:9` | 1920×1080 | YouTube、B站、横屏视频 |
| `9:16` | 1080×1920 | 抖音、快手、竖屏短视频 |

### 5.3 技术支持

如有疑问，请联系：
- 技术文档：https://your-api-domain.com/docs
- 开发者社区：https://community.example.com
- 技术支持：support@example.com

---

**文档版本：** v1.0.0  
**更新日期：** 2024-01-01  
**适用平台：** new-api v1.0.0+
