# 回退指南

## 快速回退到添加 1K/2K 支持之前

如果新版本出现问题，可以快速回退：

### 方法 1: 使用 Git Tag（推荐）

```bash
# 回退到添加 1K/2K 之前的版本
git checkout backup-before-1k-2k-size

# 重新构建
docker compose build new-api

# 重启服务
docker compose restart new-api
```

### 方法 2: 使用 Git Reset

```bash
# 回退到上一个提交
git reset --hard 880cc5ab

# 重新构建
docker compose build new-api

# 重启服务
docker compose restart new-api
```

### 方法 3: 恢复到当前版本

```bash
# 如果已经回退，想恢复到最新版本
git checkout custom
git pull

# 重新构建
docker compose build new-api

# 重启服务
docker compose restart new-api
```

---

## 验证回退是否成功

```bash
# 查看当前提交
git log --oneline -3

# 测试服务
curl -s http://localhost:3000/api/status | jq '.success'
```

---

## 提交历史

| 提交 ID | 说明 | 标签 |
|---------|------|------|
| `490f7fcc` | 添加 1K/2K 文档 | 当前版本 |
| `880cc5ab` | 预签名 URL 增强 | `backup-before-1k-2k-size` |
| `42e82e32` | 异步图片大小限制 | - |

---

## 常见问题

### Q: 回退后数据会丢失吗？

A: 不会。回退只影响代码，不影响数据库和已上传的文件。

### Q: 回退后如何恢复？

A: 使用 `git checkout custom` 即可恢复到最新版本。

### Q: 如何查看所有可用的标签？

```bash
git tag -l
```

---

## 紧急回退脚本

创建一个快速回退脚本：

```bash
#!/bin/bash
# rollback.sh

echo "🔄 开始回退..."

# 回退代码
git checkout backup-before-1k-2k-size

# 重新构建
echo "🔨 重新构建镜像..."
docker compose build new-api

# 重启服务
echo "🚀 重启服务..."
docker compose restart new-api

# 等待服务启动
sleep 5

# 验证
echo "✅ 验证服务状态..."
curl -s http://localhost:3000/api/status | jq '.success'

echo "✅ 回退完成！"
```

使用方法：

```bash
chmod +x rollback.sh
./rollback.sh
```

---

## 联系支持

如果遇到问题，请提供以下信息：

1. 当前 Git 提交 ID: `git log --oneline -1`
2. 服务日志: `docker logs new-api --tail 100`
3. 错误信息截图
