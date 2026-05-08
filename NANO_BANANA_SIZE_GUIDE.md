# Nano Banana Pro Size 参数说明

## 概述

`nano-banana-pro` 模型支持多种分辨率选项，通过 `size` 参数控制生成图片的分辨率。

## 支持的 Size 值

| 值 | 说明 | 大致分辨率 | 适用场景 |
|----|------|-----------|---------|
| `1K` | 1K 分辨率 | ~1024 像素 | 快速预览、小图 |
| `2K` | 2K 分辨率 | ~2048 像素 | 标准质量 |
| `4K` | 4K 分辨率 | ~4096 像素 | 高质量、大图 |

**注意**：实际分辨率会根据 `aspect_ratio` 自动调整。

---

## 使用示例

### 示例 1: 1K 分辨率（快速）

```bash
curl -X POST https://cf-api.o1key.com/async/v1/images/generations \
  -H "Authorization: Bearer sk-xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "nano-banana-pro",
    "prompt": "一只可爱的猫",
    "aspect_ratio": "1:1",
    "size": "1K",
    "image_compression": "webp",
    "n": 1
  }'
```

### 示例 2: 2K 分辨率（标准）

```bash
curl -X POST https://cf-api.o1key.com/async/v1/images/generations \
  -H "Authorization: Bearer sk-xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "nano-banana-pro",
    "prompt": "一只可爱的猫",
    "aspect_ratio": "16:9",
    "size": "2K",
    "image_compression": "webp",
    "n": 1
  }'
```

### 示例 3: 4K 分辨率（高质量）

```bash
curl -X POST https://cf-api.o1key.com/async/v1/images/generations \
  -H "Authorization: Bearer sk-xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "nano-banana-pro",
    "prompt": "一只可爱的猫",
    "aspect_ratio": "16:9",
    "size": "4K",
    "image_compression": "webp",
    "n": 1
  }'
```

---

## 与 Aspect Ratio 的组合

### 1:1 正方形

| Size | 大致分辨率 |
|------|-----------|
| 1K | 1024x1024 |
| 2K | 2048x2048 |
| 4K | 4096x4096 |

### 16:9 横屏

| Size | 大致分辨率 |
|------|-----------|
| 1K | 1024x576 |
| 2K | 2048x1152 |
| 4K | 4096x2304 |

### 9:16 竖屏

| Size | 大致分辨率 |
|------|-----------|
| 1K | 576x1024 |
| 2K | 1152x2048 |
| 4K | 2304x4096 |

### 4:3 传统横屏

| Size | 大致分辨率 |
|------|-----------|
| 1K | 1024x768 |
| 2K | 2048x1536 |
| 4K | 4096x3072 |

### 3:4 传统竖屏

| Size | 大致分辨率 |
|------|-----------|
| 1K | 768x1024 |
| 2K | 1536x2048 |
| 4K | 3072x4096 |

---

## 性能与质量对比

| Size | 生成速度 | 图片质量 | 文件大小 | 推荐场景 |
|------|---------|---------|---------|---------|
| 1K | ⚡⚡⚡ 快 | ⭐⭐⭐ 标准 | 小 | 快速预览、测试 |
| 2K | ⚡⚡ 中等 | ⭐⭐⭐⭐ 良好 | 中等 | 日常使用、社交媒体 |
| 4K | ⚡ 较慢 | ⭐⭐⭐⭐⭐ 优秀 | 大 | 专业用途、打印 |

---

## 完整参数示例

```json
{
  "model": "nano-banana-pro",
  "prompt": "一只可爱的猫咪坐在窗台上，阳光洒在它身上",
  
  // 分辨率（必选其一）
  "size": "2K",              // 1K, 2K, 4K
  
  // 宽高比（可选）
  "aspect_ratio": "16:9",    // 1:1, 16:9, 9:16, 4:3, 3:4
  
  // 响应模式（可选）
  "response_modalities": ["IMAGE"],  // ["IMAGE"] 或 ["TEXT", "IMAGE"]
  
  // 压缩模式（可选）
  "image_compression": "webp",  // webp, jpg, origin
  
  // 生成数量（可选）
  "n": 1,
  
  // 参考图片（可选，图生图）
  "image": "https://example.com/ref.png",
  // 或多图
  "images": ["https://example.com/1.png", "https://example.com/2.png"]
}
```

---

## Python 完整示例

```python
import requests

API_BASE = "https://cf-api.o1key.com"
TOKEN = "sk-xxx"

# 测试不同分辨率
sizes = ["1K", "2K", "4K"]

for size in sizes:
    print(f"\n测试 {size} 分辨率...")
    
    response = requests.post(
        f"{API_BASE}/async/v1/images/generations",
        headers={
            "Authorization": f"Bearer {TOKEN}",
            "Content-Type": "application/json"
        },
        json={
            "model": "nano-banana-pro",
            "prompt": "一只可爱的猫",
            "aspect_ratio": "1:1",
            "size": size,
            "image_compression": "webp",
            "n": 1
        }
    )
    
    if response.status_code == 200:
        task_id = response.json()["task_id"]
        print(f"✅ 任务创建成功: {task_id}")
    else:
        print(f"❌ 失败: {response.text}")
```

---

## 注意事项

1. **分辨率越高，生成时间越长**
   - 1K: 约 10-20 秒
   - 2K: 约 20-40 秒
   - 4K: 约 40-80 秒

2. **文件大小**
   - 使用 `image_compression: "webp"` 可以显著减小文件大小
   - 4K 图片建议使用压缩

3. **配额消耗**
   - 不同分辨率可能消耗不同的配额
   - 具体以实际扣费为准

4. **兼容性**
   - 旧版本可能不支持 `1K` 和 `2K`
   - 如果遇到错误，请升级到最新版本

---

## 错误处理

### 错误 1: 不支持的 size 值

```json
{
  "error": "size 参数值错误: 'HD'，应为 1K、2K 或 4K"
}
```

**解决方案**：使用 `1K`, `2K`, `4K` 之一。

### 错误 2: 缺少 size 参数

如果不传 `size` 参数，系统会使用默认值（通常是 2K）。

---

## 迁移指南

### 从旧版本迁移

如果你之前使用的是 `HD`, `FHD`, `4K`：

| 旧值 | 新值 | 说明 |
|------|------|------|
| `HD` | `1K` | 低分辨率 |
| `FHD` | `2K` | 标准分辨率 |
| `4K` | `4K` | 保持不变 |

**迁移示例**：

```python
# 旧代码
payload = {"size": "HD"}

# 新代码
payload = {"size": "1K"}
```

---

## 总结

- ✅ 支持 `1K`, `2K`, `4K` 三种分辨率
- ✅ 可与 `aspect_ratio` 组合使用
- ✅ 推荐使用 `2K` 作为日常使用的默认值
- ✅ 使用 `webp` 压缩可以减小文件大小

如有问题，请查看完整文档或联系技术支持。
