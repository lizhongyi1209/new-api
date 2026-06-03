# 统一异步生图 API 开发指南

## 基础信息

| 项目 | 值 |
|------|-----|
| Base URL | `https://cf-api.o1key.com` |
| 认证 | `Authorization: Bearer <your-api-key>` |
| Content-Type | `application/json` |

---

## 1. 上传参考图（仅图生图需要）

如果参考图来自本地文件，先上传到 R2 拿公网 URL。

### 请求

```bash
curl -X POST "https://cf-api.o1key.com/v1/storage/presign" \
  -H "Authorization: Bearer YOUR_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "filename": "reference.png",
    "content_type": "image/png"
  }'
```

### 响应

```json
{
  "upload_url": "https://<bucket>.r2.cloudflarestorage.com/uploads/<id>_reference.png?...",
  "public_url": "https://<public-base-url>/uploads/<id>_reference.png"
}
```

### 上传文件到 URL

```bash
curl -X PUT "<upload_url>" \
  --data-binary @/path/to/your/image.png
```

---

## 2. 文生图

### 请求

```bash
curl -X POST "https://cf-api.o1key.com/async/v1/images/generations" \
  -H "Authorization: Bearer YOUR_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "nano-banana-pro",
    "prompt": "A cute cat sitting on a cloud in a magical sky, Studio Ghibli style",
    "n": 1,
    "aspect_ratio": "16:9",
    "image_compression": "webp"
  }'
```

### 参数说明

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| model | string | 是 | 模型名：`nano-banana-pro`、`gpt-image-1`、`imagen-4.0-generate-001` 等 |
| prompt | string | 是 | 提示词 |
| n | number | 否 | 生成数量，默认 1 |
| size | string | 否 | `"1024x1024"`、`"1792x1024"` 等，与 aspect_ratio 二选一 |
| aspect_ratio | string | 否 | `"1:1"`、`"16:9"`、`"9:16"`、`"4:3"` 等 |
| quality | string | 否 | `"standard"` / `"hd"` |
| response_format | string | 否 | `"url"` 返回 R2 链接（默认），`"b64_json"` 返回 base64 |
| image_compression | string | 否 | `"webp"` R2 存储时转 WebP 压缩 |
| image | mixed | 否 | 参考图 URL 或 base64（图生图用） |
| images | string[] | 否 | 多张参考图 URL 数组 |

### 响应

```json
{
  "task_id": "task_3aBcDeFgHiJkLmNoPqRsTuVwXyZ01234567",
  "status": "SUBMITTED"
}
```

---

## 3. 图生图

参考图支持两种方式：

### 方式 A：传公网 URL（推荐，国内上传 R2 后使用）

```bash
curl -X POST "https://cf-api.o1key.com/async/v1/images/generations" \
  -H "Authorization: Bearer YOUR_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-1",
    "prompt": "Turn this sketch into a photorealistic rendering",
    "n": 1,
    "image": "https://<r2-public>/uploads/xxx_reference.png"
  }'
```

### 方式 B：直接传 base64（小图适用，<20MB）

```bash
curl -X POST "https://cf-api.o1key.com/async/v1/images/generations" \
  -H "Authorization: Bearer YOUR_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-1",
    "prompt": "Turn this sketch into a photorealistic rendering",
    "n": 1,
    "image": "data:image/png;base64,iVBORw0KGgo..."
  }'
```

### 方式 C：多张参考图

```json
{
  "model": "gpt-image-1",
  "prompt": "...",
  "images": [
    "https://r2.example.com/ref1.png",
    "https://r2.example.com/ref2.png"
  ]
}
```

---

## 4. 轮询任务结果

### 请求

```bash
curl -X GET "https://cf-api.o1key.com/async/v1/tasks/task_3aBcDeFgHiJkLmNoPqRsTuVwXyZ01234567" \
  -H "Authorization: Bearer YOUR_KEY"
```

或 POST：

```bash
curl -X POST "https://cf-api.o1key.com/async/v1/tasks/fetch" \
  -H "Authorization: Bearer YOUR_KEY" \
  -H "Content-Type: application/json" \
  -d '{"task_id": "task_3aBcDeFgHiJkLmNoPqRsTuVwXyZ01234567"}'
```

### 进行中响应

```json
{
  "task_id": "task_3aBcDeFgHiJkLmNoPqRsTuVwXyZ01234567",
  "status": "IN_PROGRESS",
  "progress": "50%",
  "estimated_seconds": 4
}
```

### 成功响应

```json
{
  "task_id": "task_3aBcDeFgHiJkLmNoPqRsTuVwXyZ01234567",
  "status": "SUCCESS",
  "progress": "100%",
  "estimated_seconds": 0,
  "data": {
    "data": [
      {
        "url": "https://<r2-public>/images/<uuid>.webp"
      }
    ],
    "created": 1778172246
  }
}
```

### 失败响应

```json
{
  "task_id": "task_3aBcDeFgHiJkLmNoPqRsTuVwXyZ01234567",
  "status": "FAILURE",
  "progress": "100%",
  "estimated_seconds": 0,
  "error": "上游返回错误: 503 Service Unavailable"
}
```

---

## 5. 轮询策略

使用 `estimated_seconds` 指导轮询间隔：

```
status == "SUBMITTED"            → 2s 后重试
status == "IN_PROGRESS"
  estimated_seconds >= 5         → 5s 后重试
  estimated_seconds < 5          → 1s 后重试
status == "SUCCESS" / "FAILURE"  → 结束
```

伪代码：

```python
import time

def poll_task(task_id, timeout=120):
    deadline = time.time() + timeout
    while time.time() < deadline:
        resp = fetch_task(task_id)
        if resp["status"] in ("SUCCESS", "FAILURE"):
            return resp
        wait = resp.get("estimated_seconds", 5)
        time.sleep(min(max(wait, 1), 10))
    raise TimeoutError("Task timed out")
```

---

## 6. 完整流程示例（Python）

```python
import requests
import time

API = "https://cf-api.o1key.com"
KEY = "sk-xxxxxxxx"

# 1. 提交文生图任务
r = requests.post(
    f"{API}/async/v1/images/generations",
    headers={"Authorization": f"Bearer {KEY}"},
    json={
        "model": "nano-banana-pro",
        "prompt": "A cute cat",
        "aspect_ratio": "1:1",
        "image_compression": "webp"
    }
)
task_id = r.json()["task_id"]
print(f"任务已提交: {task_id}")

# 2. 轮询结果
deadline = time.time() + 120
while time.time() < deadline:
    r = requests.get(
        f"{API}/async/v1/tasks/{task_id}",
        headers={"Authorization": f"Bearer {KEY}"}
    )
    result = r.json()
    status = result["status"]

    if status == "SUCCESS":
        urls = [img["url"] for img in result["data"]["data"]]
        print(f"生成完成: {urls}")
        break
    elif status == "FAILURE":
        print(f"任务失败: {result['error']}")
        break
    else:
        wait = result.get("estimated_seconds", 5)
        print(f"{status} {result['progress']} 等待 {wait}s")
        time.sleep(min(max(wait, 1), 10))
```

---

## 7. 错误码

| HTTP | type | 说明 |
|------|------|------|
| 400 | `invalid_request_error` | 缺少必填参数或参数格式错误 |
| 400 | `billing_error` | 余额不足或计费失败 |
| 401 | `authentication_error` | API Key 无效或缺失 |
| 404 | `not_found_error` | 任务不存在 |
| 500 | `internal_error` | 服务端内部错误 |
