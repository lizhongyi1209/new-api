# 异步 Gemini 格式图片处理流程

## 概述

当使用 `/async/v1/images/generations` 接口请求 Gemini 原生图片模型（如 `gemini-3-pro-image`、`nano-banana*` 等）时，系统会自动转换为 Gemini 原生格式并异步处理。

## 完整流程

### 1. 请求提交阶段

**入口**：`controller/async_image.go:AsyncImageSubmit`

```
客户端请求 (OpenAI 格式)
  ↓
判断是否 Gemini 原生模型 (第 98-105 行)
  ├─ 是 Gemini channel + 特定模型？
  │   ├─ nano-banana* 前缀
  │   ├─ gemini-3-pro-image
  │   └─ gemini-3.1-flash-image-preview
  ↓
转换为 Gemini 原生格式 (第 109 行)
  └─ service.ConvertAsyncImageToGeminiNative()
```

**转换逻辑**（`service/async_image.go:1023-1150`）：

```json
// OpenAI 格式输入
{
  "model": "gemini-3-pro-image",
  "prompt": "a blue cat",
  "image": "https://example.com/ref.png",
  "size": "1024x1024"
}

// 转换为 Gemini 原生格式
{
  "contents": [
    {
      "role": "user",
      "parts": [
        {"text": "a blue cat"},
        {
          "inlineData": {
            "mimeType": "image/png",
            "data": "iVBORw0KGgo..." // base64
          }
        }
      ]
    }
  ],
  "generationConfig": {
    "imageConfig": {
      "imageSize": "1024x1024"
    }
  }
}
```

### 2. 异步处理阶段

**处理器**：`service/async_image.go:ProcessUnifiedImageTask`

```
后台 goroutine 启动
  ↓
从 task.Data 读取 Gemini 原生请求
  ↓
调用上游 Gemini API (第 850 行)
  └─ adaptor.DoRequest(c, relayInfo, jsonData)
  ↓
上游返回 Gemini 原生响应
```

### 3. 图片提取与上传阶段

**位置**：`service/async_image.go:920-968`

```
解析 Gemini 响应
  ↓
遍历 candidates[].content.parts[]
  ↓
找到 inlineData 字段
  └─ {
       "inlineData": {
         "mimeType": "image/png",
         "data": "iVBORw0KGgo..."  // 上游返回的 base64
       }
     }
  ↓
上传到 R2 存储 (第 945 行)
  └─ UploadBase64ImageToR2(mimeType, base64Data)
  ↓
返回公网 URL
  └─ https://r2.example.com/images/uuid.png
  ↓
修改响应结构 (第 957-958 行)
  ├─ 删除 inlineData 字段
  └─ 添加 imageUrl 字段
```

**上传函数详解**（`service/storage.go:172-242`）：

```go
func UploadBase64ImageToR2(mimeType, base64Data string) (string, error)
```

图片字节不会重新编码，文件扩展名根据原始图片格式检测。

**上传流程**：

```
base64 解码并检测原始格式
  ↓
生成 UUID 文件名
  └─ images/{uuid}.{ext}
  ↓
上传到 R2 (S3 兼容)
  └─ PutObject(bucket, key, body, contentType)
  ↓
返回公网 URL
  └─ {R2_PUBLIC_BASE_URL}/images/{uuid}.{ext}
```

### 4. 结果存储阶段

**位置**：`service/async_image.go:980-990`

```
存储到 task.Data (第 983 行)
  └─ {
       "urls": [
         "https://r2.example.com/images/uuid1.png",
         "https://r2.example.com/images/uuid2.png"
       ]
     }
  ↓
同时存储到 task.PrivateData.ResultURL (第 985 行)
  └─ 第一张图片的 URL (向后兼容)
  ↓
更新任务状态
  └─ Status: SUCCESS
  └─ Progress: 100%
```

### 5. 客户端获取结果

**入口**：`controller/async_image.go:AsyncTaskFetch`

```
GET /async/v1/task/{task_id}
  ↓
读取 task.Data
  ↓
返回标准 OpenAI ImageResponse 格式 (第 369-397 行)
```

**响应格式**：

```json
{
  "task_id": "task_xxx",
  "status": "SUCCESS",
  "progress": "100%",
  "data": {
    "created": 1234567890,
    "data": [
      {"url": "https://r2.example.com/images/uuid1.png"},
      {"url": "https://r2.example.com/images/uuid2.png"}
    ]
  }
}
```

## 关键代码位置

| 功能 | 文件 | 行号 | 说明 |
|------|------|------|------|
| 判断是否 Gemini 原生 | `controller/async_image.go` | 98-105 | 检查 channel 类型和模型名 |
| 转换为 Gemini 格式 | `service/async_image.go` | 1023-1150 | ConvertAsyncImageToGeminiNative |
| 异步处理入口 | `service/async_image.go` | 705-1019 | ProcessUnifiedImageTask |
| 提取图片 | `service/async_image.go` | 920-968 | 遍历 candidates.content.parts |
| 上传 R2 | `service/storage.go` | 172-242 | UploadBase64ImageToR2 |
| 原图保存 | `service/storage.go` | 172-242 | 保持原始字节并检测扩展名 |
| 返回结果 | `controller/async_image.go` | 369-397 | AsyncTaskFetch |

## 图片保存规则

- 保持上游返回的原始图片字节，不做重新编码或压缩。
- 根据图片内容检测 PNG、JPEG、WebP 或 GIF 扩展名。
- 无法识别内容时使用 MIME 类型推断扩展名。

## 环境变量配置

```bash
# R2 存储配置（必需）
R2_ACCOUNT_ID=your_account_id
R2_ACCESS_KEY_ID=your_access_key
R2_SECRET_ACCESS_KEY=your_secret_key
R2_BUCKET=your_bucket_name
R2_PUBLIC_BASE_URL=https://your-r2-domain.com

# 图片大小限制（可选）
MAX_FILE_DOWNLOAD_MB=64  # 默认下载限制
MAX_REQUEST_BODY_MB=128  # 请求体限制
```

## 完整示例

### 1. 提交请求

```bash
curl -X POST https://api.example.com/async/v1/images/generations \
  -H "Authorization: Bearer sk-xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gemini-3-pro-image",
    "prompt": "a blue cat sitting on a red chair",
    "image": "https://example.com/reference.png",
    "size": "1024x1024",
    "n": 2
  }'
```

**响应**：

```json
{
  "task_id": "task_abc123",
  "status": "submitted"
}
```

### 2. 查询结果

```bash
curl https://api.example.com/async/v1/task/task_abc123 \
  -H "Authorization: Bearer sk-xxx"
```

**响应（处理中���**：

```json
{
  "task_id": "task_abc123",
  "status": "in_progress",
  "progress": "50%"
}
```

**响应（完成）**：

```json
{
  "task_id": "task_abc123",
  "status": "SUCCESS",
  "progress": "100%",
  "data": {
    "created": 1234567890,
    "data": [
      {"url": "https://r2.example.com/images/uuid1.png"},
      {"url": "https://r2.example.com/images/uuid2.png"}
    ]
  }
}
```

## 与标准异步图片接口的区别

| 特性 | 标准接口 (ProcessAsyncImageTask) | Gemini 原生 (ProcessUnifiedImageTask) |
|------|--------------------------------|-------------------------------------|
| 请求格式 | OpenAI 标准 | 转换为 Gemini 原生 |
| 上游调用 | ImageAdaptor | GeminiAdaptor |
| 响应解析 | OpenAI ImageResponse | Gemini candidates.content.parts |
| 图片位置 | `data[].url` 或 `data[].b64_json` | `inlineData.data` |
| 结果存储 | `{urls: [...]}` | `{urls: [...]}` (相同) |

## 故障排查

### 1. 图片未返回

**检查**：
- 上游响应中是否有 `candidates[].content.parts[].inlineData`
- 日志：`unified_image: upstream error status=xxx`

### 2. R2 上传失败

**检查**：
- 环境变量 `R2_*` 是否配置正确
- R2 bucket 权限是否正确
- 日志：`unified_image: R2 upload failed: xxx`

### 3. 图片过大

**检查**：
- 输入图片是否超过 50 MB（URL）或 20 MB（base64）
- 上游返回的图片是否超过默认限制（64 MB）

## 性能优化建议

1. **合理设置 `n` 参数**：避免一次生成过多图片
2. **使用 CDN**：在 R2 前加 Cloudflare CDN 加速访问
3. **定期清理**：删除过期的 R2 图片文件
