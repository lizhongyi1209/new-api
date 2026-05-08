# 异步图片接口大小限制说明

## 概述

`/async/v1/images/generations` 接口现在有独立的图片大小限制，与全局配置分离。

## 限制规则

| 上传方式 | 大小限制 | 说明 |
|---------|---------|------|
| **Base64 编码** | **10 MB** | 解码后的原始图片大小 |
| **URL 引用** | **50 MB** | 服务器下载的图片大小 |

## 代码位置

- **常量定义**：`service/async_image.go:67-72`
  ```go
  const (
      AsyncImageMaxBase64SizeMB = 10
      AsyncImageMaxURLSizeMB = 50
  )
  ```

- **验证函数**：`service/async_image.go:75-142`
  - `ValidateAsyncImageSize()` - 主验证入口
  - `validateSingleImageSize()` - 单张图片验证

- **调用位置**：`controller/async_image.go:33-41`

## 验证时机

### 1. 提交时验证（Base64）
- 在 `AsyncImageSubmit` 接收请求后立即验证
- 检查 `image` 和 `images` 字段中的 base64 数据
- 超限直接返回 400 错误，不消耗配额

### 2. 下载时验证（URL）
- 提交时进行 HEAD 请求预检（如果支持）
- 实际下载时强制限制（使用 `GetImageFromUrlWithLimit`）
- 超限任务标记为失败，已扣配额会退款

## 请求示例

### Base64 上传（限制 10 MB）

```bash
curl -X POST https://api.example.com/async/v1/images/generations \
  -H "Authorization: Bearer sk-xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gemini-3-pro-image",
    "prompt": "make it blue",
    "image": "data:image/png;base64,iVBORw0KGgo..."
  }'
```

### URL 引用（限制 50 MB）

```bash
curl -X POST https://api.example.com/async/v1/images/generations \
  -H "Authorization: Bearer sk-xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gemini-3-pro-image",
    "prompt": "make it blue",
    "image": "https://example.com/input.png"
  }'
```

### 多图上传

```bash
curl -X POST https://api.example.com/async/v1/images/generations \
  -H "Authorization: Bearer sk-xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gemini-3-pro-image",
    "prompt": "blend these",
    "images": [
      "data:image/png;base64,iVBORw0KGgo...",
      "https://example.com/input2.png"
    ]
  }'
```

## 错误响应

### Base64 超限

```json
{
  "error": {
    "message": "image 字段验证失败: base64 图片大小 12.50 MB 超过限制 10 MB",
    "type": "invalid_request_error"
  }
}
```

### URL 超限

```json
{
  "error": {
    "message": "images[0] 验证失败: URL 图片大小 52428800 字节超过限制 50 MB",
    "type": "invalid_request_error"
  }
}
```

## 修改限制

如需调整限制，修改 `service/async_image.go` 中的常量：

```go
const (
    AsyncImageMaxBase64SizeMB = 10  // 改为你需要的值
    AsyncImageMaxURLSizeMB = 50     // 改为你需要的值
)
```

然后重新构建：

```bash
docker compose up --build -d
```

## 与全局限制的关系

| 配置项 | 作用范围 | 默认值 | 说明 |
|-------|---------|-------|------|
| `MAX_REQUEST_BODY_MB` | 所有 HTTP 请求体 | 128 MB | 包括 JSON + base64 |
| `MAX_FILE_DOWNLOAD_MB` | 其他接口的 URL 下载 | 64 MB | 不影响异步图片接口 |
| `AsyncImageMaxBase64SizeMB` | 异步图片 base64 | **10 MB** | 独立限制 |
| `AsyncImageMaxURLSizeMB` | 异步图片 URL | **50 MB** | 独立限制 |

## 注意事项

1. **Base64 编码膨胀**：原始 10 MB 图片编码后约 13.3 MB，仍需满足 `MAX_REQUEST_BODY_MB` 限制
2. **多图累加**：`images` 数组中每张图片都单独验证，不累加计算
3. **Nginx 限制**：如果前端有 Nginx，需确保 `client_max_body_size` 足够大
4. **上游图片不受限**：从上游 provider 返回的生成图片下载使用默认限制（64 MB）

## 实现细节

### 验证流程

```
请求到达
  ↓
解析 JSON (受 MAX_REQUEST_BODY_MB 限制)
  ↓
ValidateAsyncImageSize()
  ├─ 检查 image 字段
  │   ├─ URL? → HEAD 请求预检 (50 MB)
  │   └─ Base64? → 解码验证 (10 MB)
  └─ 检查 images 数组
      └─ 逐个验证 (同上)
  ↓
通过 → 创建任务
  ↓
异步处理
  └─ URL 下载时再次强制限制 (50 MB)
```

### 代码改动

1. **新增验证函数**：`service/async_image.go`
   - `ValidateAsyncImageSize()`
   - `validateSingleImageSize()`

2. **新增带限制的下载函数**：`service/image.go`
   - `GetImageFromUrlWithLimit(maxSizeMB int)`
   - 原 `GetImageFromUrl()` 改为调用新函数（兼容性）

3. **调用点更新**：
   - `controller/async_image.go:33` - 提交时验证
   - `service/async_image.go:296,321` - ProcessAsyncImageTask 下载
   - `service/async_image.go:1038,1079` - ConvertAsyncImageToGeminiNative 下载

## 测试建议

```bash
# 1. 测试 base64 超限（生成 11 MB 图片）
dd if=/dev/urandom of=/tmp/test.bin bs=1M count=11
base64 /tmp/test.bin > /tmp/test.b64
# 用 test.b64 内容构造请求，应返回 400 错误

# 2. 测试 URL 超限（准备 51 MB 图片 URL）
# 应在提交时或下载时返回错误

# 3. 测试正常请求（9 MB base64 或 49 MB URL）
# 应成功创建任务
```
