# 安全合并计划 - rc.14 → rc.16 选择性更新

> 生成时间：2026-07-05  
> 当前版本：v1.0.0-rc.14  
> 目标：在保留所有自定义功能的前提下，合并安全修复和稳定性改进

---

## 📊 更新概况

- **可安全合并**: 11 个关键修复 + 3 个依赖更新
- **预计时间**: 30-60 分钟
- **停机时间**: < 2 分钟（重启容器）
- **回滚方案**: 备份分支 + Docker 镜像

---

## ✅ 第一批：核心安全修复（5 个，强烈推荐）

### 1. Token 只读访问修复
```bash
git cherry-pick 0d5995eb6
# fix(auth): allow read-only access for non-disabled tokens
# 文件：middleware/auth.go (+11 行)
```

### 2. 用户名输入验证
```bash
git cherry-pick bed4a3f91
# fix(user): trim whitespace from username and validate input
# 文件：controller/user.go (+10 行)
```

### 3. 用户查询隐私保护
```bash
git cherry-pick bfddc5fea
# fix: omit access_token from user queries
# 文件：model/user.go (3 行改动)
```

### 4. Session 过期判断优化
```bash
git cherry-pick 0565e6267
# fix: only treat 401 as session expiry in auth guard
# 文件：修复误判导致的意外登出
```

### 5. 品牌名修正
```bash
git cherry-pick 0b7ae4ea7
# fix: StepFun -> Stepfun
```

---

## ✅ 第二批：依赖安全更新（3 个，推荐）

### 6. 图片处理库安全更新
```bash
git cherry-pick 1dcb389d0
# chore(deps): bump golang.org/x/image from 0.38.0 to 0.41.0
```

### 7. 网络库安全更新
```bash
git cherry-pick 69c4d83df
# chore(deps): bump golang.org/x/net from 0.50.0 to 0.55.0
```

### 8. XSS 防护库更新
```bash
git cherry-pick 0bf42781d
# chore(deps): bump dompurify from 3.4.5 to 3.4.11
# 前端 XSS 防护加固
```

---

## ✅ 第三批：功能修复（3 个，可选）

### 9. Ollama Tool Calls 修复
```bash
git cherry-pick 0977965d9
# fix: handle ollama non-stream tool calls
# 文件：relay/channel/ollama/stream.go + 测试
```

### 10. 渠道过滤器持久化
```bash
git cherry-pick c1903607d
# fix: persist channel status filter across page navigation
# 前端用户体验改进
```

### 11. Tab 切换状态保持
```bash
git cherry-pick 759ab6bbc
# fix: keep page state when switching tabs within the same route
# 前端用户体验改进
```

---

## ⚠️ 第四批：进阶功能（需单独测试）

### 12. 服务优雅关闭（可选）
```bash
git cherry-pick 986d90ae0
# 支持服务优雅关闭,避免重启回复中断和面板缓存数据丢失
# 风险：改动 main.go，涉及进程管理
# 建议：先在测试环境验证重启流程
```

---

## 🚫 不合并的内容（会破坏自定义功能）

- ❌ ClickHouse 日志数据库（大规模重构）
- ❌ 系统任务运行器（架构变更）
- ❌ 所有删除自定义文件的提交
- ❌ Seedance 2.0 计费（与你的自定义计费冲突）

---

## 📋 执行步骤

### 准备工作

```bash
cd /home/ubuntu/new-api

# 1. 完整备份
cp -r /home/ubuntu/new-api /home/ubuntu/new-api-backup-$(date +%Y%m%d)
docker exec postgres pg_dump -U root new-api > /home/ubuntu/backup_safe_merge_$(date +%Y%m%d_%H%M%S).sql

# 2. 创建合并分支
git checkout custom
git checkout -b safe-merge-rc16
```

### 批量 Cherry-pick

```bash
# 第一批：核心安全修复
git cherry-pick 0d5995eb6  # token 只读访问
git cherry-pick bed4a3f91  # 用户名验证
git cherry-pick bfddc5fea  # 移除 access_token
git cherry-pick 0565e6267  # session 过期优化
git cherry-pick 0b7ae4ea7  # StepFun 品牌名

# 第二批：依赖更新
git cherry-pick 1dcb389d0  # x/image
git cherry-pick 69c4d83df  # x/net
git cherry-pick 0bf42781d  # dompurify

# 第三批：功能修复
git cherry-pick 0977965d9  # Ollama tool calls
git cherry-pick c1903607d  # 渠道过滤器
git cherry-pick 759ab6bbc  # Tab 状态
```

### 冲突处理

如果遇到冲突：

```bash
# 查看冲突文件
git status

# 手动解决冲突后
git add <冲突文件>
git cherry-pick --continue

# 如果某个提交冲突太复杂，可以跳过
git cherry-pick --skip
```

### 测试和部署

```bash
# 1. 本地构建测试
docker compose build

# 2. 运行测试（如果有）
go test ./...

# 3. 部署
docker compose up -d
docker compose logs -f new-api

# 4. 验证服务
curl http://localhost:3000/api/status
```

### 合并到 custom 分支

```bash
# 确认一切正常后
git checkout custom
git merge safe-merge-rc16 --no-ff -m "chore: merge safe fixes from rc.16"
git push origin custom

# 更新版本标记（可选，保持 rc.14 或标记为 rc.14+）
echo "v1.0.0-rc.14+" > VERSION
git add VERSION
git commit -m "chore: mark as rc.14+ with security patches"
git push origin custom
```

---

## 🔄 回滚方案

如果出现问题：

```bash
# 方案 1：切回原分支
git checkout custom
docker compose up --build -d

# 方案 2：恢复备份
rm -rf /home/ubuntu/new-api
mv /home/ubuntu/new-api-backup-$(date +%Y%m%d) /home/ubuntu/new-api
cd /home/ubuntu/new-api
docker compose up -d

# 方案 3：恢复数据库（如果需要）
docker exec -i postgres psql -U root new-api < /home/ubuntu/backup_safe_merge_*.sql
```

---

## ⚠️ 风险评估

| 批次 | 风险等级 | 潜在影响 | 建议 |
|------|----------|----------|------|
| 第一批 | 🟢 低 | 改进安全性，无副作用 | 必须合并 |
| 第二批 | 🟢 低 | 依赖更新，向后兼容 | 推荐合并 |
| 第三批 | 🟡 中 | 功能修复，可能与前端交互 | 测试后合并 |
| 第四批 | 🟡 中 | 改动进程管理，需充分测试 | 可选，谨慎 |

---

## 📝 注意事项

1. **不要直接 rebase upstream/main**，会删除所有自定义功能
2. **逐个 cherry-pick**，遇到冲突可以跳过
3. **优先合并第一、二批**，第三、四批可以后续再考虑
4. **每批合并后都测试一次**，确保服务正常
5. **保留备份至少 7 天**

---

## 🎯 预期收益

- ✅ 修复 5 个安全漏洞
- ✅ 更新 3 个依赖库（包含安全补丁）
- ✅ 修复 3 个用户体验问题
- ✅ 保留 100% 自定义功能
- ✅ 无需大规模代码重构

---

## 📞 后续建议

合并完成后：

1. **监控日志** 24 小时，观察异常
2. **通知用户**（如有）短暂重启
3. **更新文档**，记录合并的提交
4. **定期检查**官方新版本，重复此流程

---

生成工具：Claude Code  
维护者：请在每次合并后更新此文件
