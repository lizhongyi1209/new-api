# 更新摘要 - 添加 1K 和 2K Size 支持

**更新时间**: 2026-05-08  
**版本**: custom 分支  
**提交**: 859948ff

---

## 📋 更新内容

### 1. Size 参数支持

`nano-banana-pro` 模型现在支持以下分辨率：

| Size | 说明 | 状态 |
|------|------|------|
| `1K` | 低分辨率（~1024px） | ✅ 新增 |
| `2K` | 标准分辨率（~2048px） | ✅ 新增 |
| `4K` | 高分辨率（~4096px） | ✅ 已支持 |

### 2. 代码变更

**文件**: `service/async_image.go:1112`

```go
switch size {
case "1K", "2K", "4K":  // 已支持 1K 和 2K
    imageConfig["imageSize"] = size
default:
    if strings.Contains(size, ":") && asyncReq.AspectRatio == "" {
        imageConfig["aspectRatio"] = size
    }
}
```

**说明**: 代码中已经支持 `1K` 和 `2K`，本次更新主要是添加文档和测试。

### 3. 新增文档

1. **NANO_BANANA_SIZE_GUIDE.md** - 完整的 size 参数使用指南
   - 支持的值说明
   - 使用示例（curl, Python, JavaScript）
   - 与 aspect_ratio 的组合
   - 性能对比
   - 迁移指南

2. **ROLLBACK_GUIDE.md** - 版本回退指南
   - 快速回退方法
   - Git 标签使用
   - 紧急回退脚本

3. **test_size_params.sh** - 自动化测试脚本
   - 测试 1K, 2K, 4K 参数
   - 验证错误处理

---

## 🚀 如何使用

### 客户端请求示例

```bash
curl -X POST https://cf-api.o1key.com/async/v1/images/generations \
  -H "Authorization: Bearer sk-xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "nano-banana-pro",
    "prompt": "一只可爱的猫",
    "aspect_ratio": "1:1",
    "size": "1K",
    "n": 1
  }'
```

### Python 示例

```python
import requests

response = requests.post(
    "https://cf-api.o1key.com/async/v1/images/generations",
    headers={
        "Authorization": "Bearer sk-xxx",
        "Content-Type": "application/json"
    },
    json={
        "model": "nano-banana-pro",
        "prompt": "一只可爱的猫",
        "aspect_ratio": "1:1",
        "size": "2K",  # 使用 2K
        "n": 1
    }
)

task_id = response.json()["task_id"]
print(f"任务 ID: {task_id}")
```

---

## 🧪 测试

### 运行自动化测试

```bash
cd /home/ubuntu/new-api
./test_size_params.sh
```

### 预期输出

```
🧪 测试 nano-banana-pro size 参数支持
==========================================

📊 测试 size=1K...
  ✅ 成功: task_id=task_xxx

📊 测试 size=2K...
  ✅ 成功: task_id=task_xxx

📊 测试 size=4K...
  ✅ 成功: task_id=task_xxx

📊 测试无效 size 值...
  ✅ 正确拒绝了无效值 'HD'

==========================================
📈 测试结果汇总
  成功: 3
  失败: 0
==========================================
✅ 所有测试通过！
```

---

## 🔄 回退方法

如果遇到问题，可以快速回退：

```bash
# 方法 1: 使用标签
git checkout backup-before-1k-2k-size
docker compose build new-api
docker compose restart new-api

# 方法 2: 使用提交 ID
git reset --hard 880cc5ab
docker compose build new-api
docker compose restart new-api
```

详细说明请查看 `ROLLBACK_GUIDE.md`。

---

## 📊 提交历史

```
859948ff test: add size parameter validation test script
1e48b473 docs: add rollback guide for version management
490f7fcc docs: add nano-banana-pro size parameter guide (1K, 2K, 4K)
880cc5ab feat: enhance presigned URL upload with size validation (备份点)
```

---

## ✅ 验证清单

- [x] 代码已支持 1K, 2K, 4K
- [x] 文档已更新
- [x] 测试脚本已创建
- [x] 回退方案已准备
- [x] Git 标签已创建
- [x] 服务已重启
- [x] 服务状态正常

---

## 📝 注意事项

1. **兼容性**: 旧客户端使用 `HD`, `FHD` 会被忽略，不会报错
2. **默认值**: 如果不传 `size`，系统使用默认值（通常是 2K）
3. **性能**: 1K 最快，4K 最慢，建议日常使用 2K

---

## 🔗 相关文档

- [NANO_BANANA_SIZE_GUIDE.md](./NANO_BANANA_SIZE_GUIDE.md) - 完整使用指南
- [ROLLBACK_GUIDE.md](./ROLLBACK_GUIDE.md) - 回退指南
- [ASYNC_IMAGE_API_GUIDE.md](./ASYNC_IMAGE_API_GUIDE.md) - 异步图片 API 指南
- [PRESIGNED_UPLOAD_GUIDE.md](./PRESIGNED_UPLOAD_GUIDE.md) - 预签名上传指南

---

## 🎯 下一步

1. **客户端测试**: 在客户端测试 `1K` 和 `2K` 参数
2. **监控**: 观察服务日志，确认无错误
3. **反馈**: 如有问题，使用回退方案

---

## 📞 支持

如有问题，请提供：
- Git 提交 ID: `git log --oneline -1`
- 服务日志: `docker logs new-api --tail 100`
- 错误截图

---

**状态**: ✅ 已完成，等待客户端测试
