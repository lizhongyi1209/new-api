# 预签名 URL 上传指南

## 概述

使用预签名 URL 可以让客户端直接上传图片到 R2 存储，无需经过服务器中转，提高上传速度并减少服务器带宽压力。这是**业界标准做法**（AWS S3、阿里云 OSS、腾讯云 COS 都采用此方案）。

## 优势

✅ **安全**：客户端无需永久凭证，只获得临时上传权限  
✅ **高效**：直连 R2，无需服务器中转  
✅ **可控**：服务器控制上传大小、类型、过期时间  
✅ **标准**：符合 RESTful 设计，资源创建权由服务器授权

## 完整流程

```
1. 客户端 → 服务器：请求上传凭证
   POST /v1/storage/presign
   
2. 服务器 → 客户端：返回预签名 URL（15 分钟有效）
   {upload_url, public_url, expires_at}
   
3. 客户端 → R2：直接上传图片
   PUT {upload_url}
   
4. 客户端 → 服务器：提交异步任务（使用 public_url）
   POST /async/v1/images/generations
```

---

## API 详细说明

### 1. 获取预签名 URL

**请求**：

```http
POST /v1/storage/presign
Authorization: Bearer sk-xxx
Content-Type: application/json

{
  "filename": "reference.png",
  "content_type": "image/png",
  "size": 5242880  // 可选：文件大小（字节），用于提前验证
}
```

**参数说明**：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `filename` | string | ✅ | 文件名（会加 UUID 前缀避免冲突） |
| `content_type` | string | ✅ | MIME 类型，必须是 `image/*` |
| `size` | int64 | ❌ | 文件大小（字节），超过 50 MB 会被拒绝 |

**响应**：

```json
{
  "upload_url": "https://xxx.r2.cloudflarestorage.com/bucket/uploads/abc123_reference.png?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=...&X-Amz-Signature=...",
  "public_url": "https://r2.example.com/uploads/abc123_reference.png",
  "expires_at": 1234567890
}
```

**字段说明**：

| 字段 | 说明 |
|------|------|
| `upload_url` | 预签名上传 URL，15 分钟内有效，只能上传一次 |
| `public_url` | 上传成功后的公网访问 URL |
| `expires_at` | 过期时间（Unix 时间戳） |

**错误响应**：

```json
// 文件类型错误
{
  "error": "只支持图片类型 (image/*)"
}

// 文件过大
{
  "error": "文件大小 55.00 MB 超过限制 50 MB"
}
```

---

### 2. 上传图片到 R2

**请求**：

```http
PUT {upload_url}
Content-Type: image/png
Content-Length: 5242880

[二进制图片数据]
```

**注意事项**：

- ⚠️ 必须使用 `PUT` 方法（不是 `POST`）
- ⚠️ `Content-Type` 必须与申请时一致
- ⚠️ 15 分钟内必须完成上传
- ⚠️ 每个 URL 只能上传一次

**成功响应**：

```
HTTP/1.1 200 OK
```

**失败响应**：

```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>SignatureDoesNotMatch</Code>
  <Message>The request signature we calculated does not match the signature you provided.</Message>
</Error>
```

---

### 3. 提交异步图片任务

**请求**：

```http
POST /async/v1/images/generations
Authorization: Bearer sk-xxx
Content-Type: application/json

{
  "model": "gemini-3-pro-image",
  "prompt": "make it blue",
  "image": "https://r2.example.com/uploads/abc123_reference.png",
  "size": "1024x1024"
}
```

**说明**：

- `image` 字段使用步骤 1 返回的 `public_url`
- 服务器会下载并验证图片（受 50 MB 限制）
- 也可以使用 `images` 数组上传多张图片

---

## 完整示例

### Bash / cURL

```bash
#!/bin/bash

API_BASE="https://api.example.com"
TOKEN="sk-xxx"
IMAGE_FILE="reference.png"

# Step 1: 获取预签名 URL
echo "Step 1: 获取上传凭证..."
PRESIGN_RESPONSE=$(curl -s -X POST "$API_BASE/v1/storage/presign" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"filename\": \"$IMAGE_FILE\",
    \"content_type\": \"image/png\",
    \"size\": $(stat -f%z "$IMAGE_FILE" 2>/dev/null || stat -c%s "$IMAGE_FILE")
  }")

UPLOAD_URL=$(echo "$PRESIGN_RESPONSE" | jq -r '.upload_url')
PUBLIC_URL=$(echo "$PRESIGN_RESPONSE" | jq -r '.public_url')

echo "Upload URL: $UPLOAD_URL"
echo "Public URL: $PUBLIC_URL"

# Step 2: 上传图片到 R2
echo -e "\nStep 2: 上传图片到 R2..."
curl -X PUT "$UPLOAD_URL" \
  -H "Content-Type: image/png" \
  --data-binary "@$IMAGE_FILE"

echo -e "\n上传完成！"

# Step 3: 提交异步任务
echo -e "\nStep 3: 提交异步图片任务..."
TASK_RESPONSE=$(curl -s -X POST "$API_BASE/async/v1/images/generations" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"model\": \"gemini-3-pro-image\",
    \"prompt\": \"make it blue\",
    \"image\": \"$PUBLIC_URL\",
    \"size\": \"1024x1024\"
  }")

TASK_ID=$(echo "$TASK_RESPONSE" | jq -r '.task_id')
echo "Task ID: $TASK_ID"

# Step 4: 查询任务状态
echo -e "\nStep 4: 查询任务状态..."
sleep 5
curl -s "$API_BASE/async/v1/task/$TASK_ID" \
  -H "Authorization: Bearer $TOKEN" | jq
```

---

### Python

```python
import requests
import os

API_BASE = "https://api.example.com"
TOKEN = "sk-xxx"
IMAGE_FILE = "reference.png"

headers = {
    "Authorization": f"Bearer {TOKEN}",
    "Content-Type": "application/json"
}

# Step 1: 获取预签名 URL
print("Step 1: 获取上传凭证...")
file_size = os.path.getsize(IMAGE_FILE)
presign_resp = requests.post(
    f"{API_BASE}/v1/storage/presign",
    headers=headers,
    json={
        "filename": IMAGE_FILE,
        "content_type": "image/png",
        "size": file_size
    }
)
presign_data = presign_resp.json()
upload_url = presign_data["upload_url"]
public_url = presign_data["public_url"]

print(f"Upload URL: {upload_url}")
print(f"Public URL: {public_url}")

# Step 2: 上传图片到 R2
print("\nStep 2: 上传图片到 R2...")
with open(IMAGE_FILE, "rb") as f:
    upload_resp = requests.put(
        upload_url,
        headers={"Content-Type": "image/png"},
        data=f
    )
print(f"上传状态: {upload_resp.status_code}")

# Step 3: 提交异步任务
print("\nStep 3: 提交异步图片任务...")
task_resp = requests.post(
    f"{API_BASE}/async/v1/images/generations",
    headers=headers,
    json={
        "model": "gemini-3-pro-image",
        "prompt": "make it blue",
        "image": public_url,
        "size": "1024x1024"
    }
)
task_data = task_resp.json()
task_id = task_data["task_id"]
print(f"Task ID: {task_id}")

# Step 4: 查询���务状态
import time
time.sleep(5)
print("\nStep 4: 查询任务状态...")
result_resp = requests.get(
    f"{API_BASE}/async/v1/task/{task_id}",
    headers={"Authorization": f"Bearer {TOKEN}"}
)
print(result_resp.json())
```

---

### JavaScript / Node.js

```javascript
const axios = require('axios');
const fs = require('fs');

const API_BASE = 'https://api.example.com';
const TOKEN = 'sk-xxx';
const IMAGE_FILE = 'reference.png';

const headers = {
  'Authorization': `Bearer ${TOKEN}`,
  'Content-Type': 'application/json'
};

(async () => {
  // Step 1: 获取预签名 URL
  console.log('Step 1: 获取上传凭证...');
  const fileSize = fs.statSync(IMAGE_FILE).size;
  const presignResp = await axios.post(
    `${API_BASE}/v1/storage/presign`,
    {
      filename: IMAGE_FILE,
      content_type: 'image/png',
      size: fileSize
    },
    { headers }
  );
  const { upload_url, public_url } = presignResp.data;
  console.log('Upload URL:', upload_url);
  console.log('Public URL:', public_url);

  // Step 2: 上传图片到 R2
  console.log('\nStep 2: 上传图片到 R2...');
  const imageData = fs.readFileSync(IMAGE_FILE);
  await axios.put(upload_url, imageData, {
    headers: { 'Content-Type': 'image/png' }
  });
  console.log('上传完成！');

  // Step 3: 提交异步任务
  console.log('\nStep 3: 提交异步图片任务...');
  const taskResp = await axios.post(
    `${API_BASE}/async/v1/images/generations`,
    {
      model: 'gemini-3-pro-image',
      prompt: 'make it blue',
      image: public_url,
      size: '1024x1024'
    },
    { headers }
  );
  const { task_id } = taskResp.data;
  console.log('Task ID:', task_id);

  // Step 4: 查询任务状态
  await new Promise(resolve => setTimeout(resolve, 5000));
  console.log('\nStep 4: 查询任务状态...');
  const resultResp = await axios.get(
    `${API_BASE}/async/v1/task/${task_id}`,
    { headers: { 'Authorization': `Bearer ${TOKEN}` } }
  );
  console.log(resultResp.data);
})();
```

---

## 技术细节

### 存储路径

- **格式**：`uploads/{uuid}_{filename}`
- **示例**：`uploads/abc123-def456-789_reference.png`
- **说明**：UUID 前缀避免文件名冲突

### 安全机制

1. **临时凭证**：预签名 URL 15 分钟后自动失效
2. **一次性**：每个 URL 只能上传一次，无法覆盖
3. **类型限制**：只允许 `image/*` MIME 类型
4. **大小限制**：最大 50 MB（与异步图片接口一致）
5. **认证要求**：获取预签名 URL 需要有效 Token

### 过期时间

- **预签名 URL**：15 分钟（从生成时开始计算）
- **上传文件**：永久存储（需手动清理）

### 清理策略（建议）

```bash
# 定期清理 uploads/ 目录中超过 24 小时的文件
# 可以用 cron 任务或 Cloudflare Workers 实现

# 示例：删除 24 小时前的文件
aws s3 ls s3://bucket/uploads/ --recursive | \
  awk '{if ($1 < "'$(date -d '24 hours ago' +%Y-%m-%d)'") print $4}' | \
  xargs -I {} aws s3 rm s3://bucket/{}
```

---

## 常见问题

### Q1: 为什么上传失败返回 403 Forbidden？

**原因**：
- 预签名 URL 已过期（超过 15 分钟）
- Content-Type 不匹配
- 文件大小超限

**解决**：重新获取预签名 URL

---

### Q2: 可以用同一个 URL 上传多次吗？

**不可以**。每个预签名 URL 只能上传一次，第二次上传会失败。如需重新上传，请重新获取预签名 URL。

---

### Q3: 上传的图片会永久保存吗？

**是的**。上传到 `uploads/` 目录的文件会永久保存，建议定期清理未使用的文件。

---

### Q4: 可以上传非图片文件吗？

**不可以**。服务器会验证 `content_type` 必须是 `image/*`，其他类型会被拒绝。

---

### Q5: 如何处理上传进度？

使用支持进度回调的 HTTP 客户端：

```javascript
// Axios 示例
await axios.put(upload_url, imageData, {
  onUploadProgress: (progressEvent) => {
    const percent = Math.round((progressEvent.loaded * 100) / progressEvent.total);
    console.log(`上传进度: ${percent}%`);
  }
});
```

---

## 环境变量配置

```bash
# R2 存储配置（必需）
R2_ACCOUNT_ID=your_account_id
R2_ACCESS_KEY_ID=your_access_key
R2_SECRET_ACCESS_KEY=your_secret_key
R2_BUCKET=your_bucket_name
R2_PUBLIC_BASE_URL=https://your-r2-domain.com
```

---

## 与直接 Base64 上传的对比

| 特性 | 预签名 URL 上传 | Base64 直接上传 |
|------|----------------|----------------|
| **服务器带宽** | 无压力（直连 R2） | 高压力（经过服务器） |
| **上传速度** | 快（直连） | 慢（两跳网络） |
| **文件大小限制** | 50 MB | 10 MB |
| **请求次数** | 3 次（获取凭证 + 上传 + 提交任务） | 1 次 |
| **实现复杂度** | 中等 | 简单 |
| **适用场景** | 大文件、高并发 | 小文件、简单场景 |

---

## 总结

预签名 URL 上传是**推荐方案**，特别适合：

✅ 大文件上传（10-50 MB）  
✅ 高并发场景  
✅ 需要减少服务器带宽成本  
✅ 需要更好的安全控制

对于小文件（< 5 MB）或简单场景，也可以继续使用 Base64 直接上传。
