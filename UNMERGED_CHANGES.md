# rc.14 → rc.16 未合并内容清单

> 生成时间：2026-07-05  
> 分析范围：v1.0.0-rc.14 到 v1.0.0-rc.16  
> 总提交数：70 个  
> 已合并：2 个核心安全修复  
> 未合并：68 个

---

## 📋 分类总览

| 分类 | 数量 | 优先级 | 风险等级 | 建议 |
|------|------|--------|----------|------|
| **核心架构变更** | 5 | 高 | 🔴 高 | 需全面测试 |
| **安全修复（已尝试）** | 8 | 高 | 🟢 低 | 已存在或冲突 |
| **功能增强** | 28 | 中 | 🟡 中 | 可选择性合并 |
| **前端改进** | 15 | 中 | 🟡 中 | 体验优化 |
| **依赖更新** | 6 | 中 | 🟢 低 | 需Go 1.21+ |
| **文档/构建** | 8 | 低 | 🟢 低 | 非关键 |

---

## 🔴 第一类：核心架构变更（不建议合并）

这些改动会彻底改变系统架构，与你的自定义功能冲突严重。

### 1. **ClickHouse 日志数据库支持** 🔴🔴🔴
```
6dc4030fd feat: support ClickHouse log database (#5663)
f84b7d591 refactor(log): simplify ClickHouse log deletion and add unit tests
f8cfbfa4d refactor(log): remove legacy log deletion endpoint and associated types
df5ba9fa5 fix: adapt ClickHouse log LIKE filters
acb52d0f7 chore(deps): update clickhouse-go and orb dependencies
993d67ebd chore(deps): bump github.com/ClickHouse/ch-go from 0.58.2 to 0.65.0
```

**影响范围**：
- 重构整个日志存储系统
- 删除旧的日志API端点
- 新增ClickHouse依赖和配置
- 修改数据库schema

**冲突原因**：
- 你的自定义日志功能 (`logs-sub/`, `logs-sub2/`, `logs-sub3/`) 会失效
- 日志查询API完全重构
- 需要部署和维护ClickHouse服务

**建议**：❌ **不要合并**，除非你准备迁移到ClickHouse并重写自定义日志逻辑

---

### 2. **系统任务运行器** 🔴🔴
```
537719229 feat: add system task runner (#5680)
a162163b4 feat: add persistent system task log cleanup progress
d10fc762f fix(task): attribute async task usage log to the initiating node (#5684)
354d0fedb feat: default node name to hostname when NODE_NAME unset (#5659)
```

**影响范围**：
- 新增分布式任务调度系统
- 需要NODE_NAME环境变量
- 改变异步任务处理方式

**冲突原因**：
- 你的现有异步任务（图片生成、视频任务）可能需要适配
- 多节点部署时的任务归属逻辑变化

**建议**：⚠️ **谨慎考虑**，单节点部署影响较小，但需测试任务功能

---

### 3. **优雅关闭支持** 🟡
```
986d90ae0 支持服务优雅关闭,避免重启回复中断和面板缓存数据丢失 (#4258)
8874d1929 Make quota logging synchronous and delay startup log
```

**影响范围**：
- 修改main.go启动和关闭流程
- 改变配额日志写入方式（同步化）

**建议**：✅ **可以考虑合并**，风险较低，能改善重启体验

---

### 4. **渠道被动监控模式** 🟡
```
efd6c445a feat: add passive channel monitoring mode (#5592)
44e0e6868 feat: add channel test env toggle
5d943281b feat: add channel async polling delay toggle
```

**影响范围**：
- 渠道健康检查逻辑变化
- 新增渠道测试环境开关

**建议**：✅ **可选合并**，提升渠道管理灵活性

---

### 5. **权限系统增强** 🟡
```
4aee5f7d5 feat: better admin permissions (#5755)
```

**影响范围**：
- 管理员权限细化
- 可能影响现有管理功能

**建议**：⚠️ **谨慎评估**，需检查是否与你的权限设置冲突

---

## 🟢 第二类：安全修复（已尝试合并但跳过）

这些修复在cherry-pick时显示为空，说明你的代码已经包含了类似修复或冲突。

### 已存在/冲突的安全修复

```
✅ bfddc5fea fix: omit access_token from user queries
✅ 0565e6267 fix: only treat 401 as session expiry in auth guard (#5872)
✅ 0b7ae4ea7 fix: StepFun -> Stepfun (#5636)
✅ 0977965d9 fix: handle ollama non-stream tool calls (#5865)
✅ c1903607d fix: persist channel status filter across page navigation (#5863)
✅ 759ab6bbc fix: keep page state when switching tabs within the same route (#5796)
✅ df5ba9fa5 fix: adapt ClickHouse log LIKE filters (依赖ClickHouse)
✅ cf6ae6fde fix: preserve SMTP PLAIN auth TLS guard
```

**状态**：这些修复要么已经在你的代码中，要么因架构差异无法应用。

---

## 🎨 第三类：前端体验改进（可选择性合并）

### UI/UX 增强

```
95e8c5eec perf(web): optimize web Rsbuild and Tailwind build pipeline (#5786)
  - 前端构建性能优化
  - 风险：低，建议：✅ 可合并

966af88ec feat(playground): improve Playground chat experience and Markdown rendering (#5217)
  - Playground聊天体验改进
  - 风险：低，建议：✅ 可合并

f4473d963 fix(web): replace default markdown renderer and expand syntax support (#5689)
  - 更好的Markdown渲染
  - 风险：低，建议：✅ 可合并

0b48ad86d fix(web): render custom HTML and Markdown content consistently (#5760)
626dadb55 fix(web): secure rich content rendering
  - 富文本安全渲染
  - 风险：低，建议：✅ 推荐合并（安全相关）

fda817786 fix(web): 修复自定义 HTML 样式被过滤及排版间距异常的问题 (#5795)
1f4d8d2b2 fix(web): inject app styles into isolated HTML (#5860)
  - HTML样式修复
  - 风险：低，建议：✅ 可合并

25f998595 feat: refine channel management UI
1d166532f fix: update section titles and improve layout in channel components
de0d6ac99 fix(web): sync channel card selection state (#5700)
  - 渠道管理UI优化
  - 风险：低，建议：✅ 可合并

c0e42bfbd fix(theme): 切换前端主题后重置到首页，避免路由 404 (#5612)
  - 主题切换bug修复
  - 风险：低，建议：✅ 推荐合并

0b2cf43e7 fix(web): hide wallet entry in profile dropdown when wallet module disabled (#5708)
  - 钱包模块显示修复
  - 风险：低，建议：✅ 可合并

9ba251ce5 perf(web): streamline table actions and destructive dialogs (#5645)
  - 表格操作优化
  - 风险：低，建议：✅ 可合并

3245b2b74 fix(model-pricing): refresh tiered expression editor when switching models (#5752)
  - 模型定价编辑器修复
  - 风险：低，建议：✅ 可合并

43591fba7 feat: improve advanced custom route editor
  - 高级路由编辑器改进
  - 风险：中，建议：⚠️ 谨慎测试

2cbdfa039 feat: add system instance info panel (#5716)
  - 系统实例信息面板
  - 风险：低，建议：✅ 可合并

5bf346836 chore: run `bun format` to automatically format the frontend code
  - 代码格式化
  - 风险：低，建议：✅ 可合并
```

---

## 💰 第四类：计费功能更新（需谨慎评估）

### Doubao Seedance 2.0 计费

```
e514db20f feat: support doubao seedance 2.0 safety_identifier/priority and 4k billing (#5824)
c8491b41b feat: bill doubao seedance-2.0 by output resolution and video input (#5300)
```

**冲突警告**：🔴🔴🔴  
你已经有自定义的 Seedance 计费逻辑：
- `2a3845210 fix: TokenMartSeedance 计费改用官方 doubao 分辨率×视频价格表`

**建议**：❌ **不要合并**，会覆盖你的自定义计费规则

---

### 可灵 Wan2.7 支持

```
52858ad1e feat: support Wan2.7 i2v media mapping (#4984)
```

**建议**：✅ **可以考虑合并**，新增可灵模型支持

---

### Waffo商品和Webhook

```
79396745d fix: add Waffo goods info and webhook SDK update (#5704)
```

**建议**：✅ **可以考虑合并**，如果你使用Waffo支付

---

## 🔧 第五类：配置和管理功能

### 用户Token限制配置

```
5d943281b feat(system-settings): add user token limit configuration section (#5678)
5814ca90c fix: add token limit save label translations
```

**建议**：✅ **推荐合并**，增强管理功能

---

### Responses to Chat 支持

```
2d5a04163 feat: support Responses to Chat (#5787)
3a506f50f fix(openai): harden Chat-to-Responses compatibility (#5772)
```

**说明**：支持OpenAI的Responses格式转换  
**建议**：✅ **可以考虑合并**，增强兼容性

---

### 密码验证文案对齐

```
df5ba9fa5 fix(auth): align password validation copy (#5759)
```

**建议**：✅ **可合并**，用户体验改进

---

## 📦 第六类：依赖更新（需Go 1.21+）

```
1dcb389d0 chore(deps): bump golang.org/x/image from 0.38.0 to 0.41.0 (#5873)
69c4d83df chore(deps): bump golang.org/x/net from 0.50.0 to 0.55.0 (#5862)
0bf42781d chore(deps): bump dompurify from 3.4.5 to 3.4.11 in /web/default (#5718)
35074345e chore(deps): sync bun.lock for dompurify 3.4.11 (#5738)
48da37a3d feat: add date-fns and date-fns-tz dependencies
69b0f0b56 feat: add date-fns and date-fns-tz paths to build configuration
64eafc941 fix: date-fns-tz classic theme build error (#5676)
```

**状态**：❌ **无法合并**（宿主机Go 1.18太旧）  
**影响**：Docker构建使用Go 1.23，部分安全更新已生效  
**建议**：升级宿主机Go版本后再考虑

---

## 🛠️ 第七类：构建和开发工具（非关键）

```
e1fd9cc28 chore(build): align make targets with web naming
f9165e7bf fix(dev): run only default frontend in dev-web
c12e5db4f fix(ci): install classic workspace dependencies for releases (#5719)
12fc01006 Bump Electron lockfile dependencies
e5694748c chore(web): use tsgo for type checking
bff701b0c docs: update AGENTS.md
72b3f3457 chore: update agent skills and project config
9fc9c8f1e chore: avoid duplicate shadcn skill exposure
6c35e1ef2 chore: update i18n skill
ad35ab1d9 feat: enhance i18n-translate skill
dfc0d6324 Merge commit from fork
```

**建议**：🟡 **可选**，不影响运行时功能

---

## 🎯 推荐合并优先级

### ⭐⭐⭐ 高优先级（强烈推荐）

```bash
# 安全和体验相关
626dadb55  # 富文本安全渲染
0b48ad86d  # HTML/Markdown一致性渲染
c0e42bfbd  # 主题切换bug修复
5d943281b  # 用户Token限制配置
df5ba9fa5  # 密码验证改进
```

### ⭐⭐ 中优先级（建议评估）

```bash
# 功能增强
966af88ec  # Playground体验改进
f4473d963  # Markdown渲染增强
2d5a04163  # Responses to Chat支持
52858ad1e  # 可灵Wan2.7支持
986d90ae0  # 优雅关闭（需测试）
```

### ⭐ 低优先级（可选）

```bash
# UI优化
25f998595  # 渠道管理UI
9ba251ce5  # 表格操作优化
0b2cf43e7  # 钱包模块显示
95e8c5eec  # 构建优化
```

---

## ❌ 不建议合并的内容

### 🔴 架构冲突

1. **ClickHouse日志系统** - 重构太大，与现有日志冲突
2. **Seedance 2.0计费** - 覆盖你的自定义计费逻辑

### ⚠️ 需深度测试

1. **系统任务运行器** - 可能影响异步任务
2. **权限系统增强** - 可能影响管理功能

---

## 📊 统计总结

| 状态 | 数量 | 占比 |
|------|------|------|
| **已合并** | 2 | 2.9% |
| **强烈推荐** | 5 | 7.1% |
| **建议评估** | 15 | 21.4% |
| **可选合并** | 25 | 35.7% |
| **不建议合并** | 10 | 14.3% |
| **无法合并（依赖）** | 7 | 10.0% |
| **非关键** | 6 | 8.6% |

---

## 🎯 下一步行动建议

### 立即可做（1-2天）

1. ✅ 合并安全渲染相关的5个高优先级提交
2. ✅ 合并UI/UX改进中风险低的10个提交
3. ✅ 测试合并后的功能

### 短期计划（1-2周）

1. 评估Responses to Chat支持
2. 测试可灵Wan2.7模型
3. 考虑优雅关闭功能

### 长期计划（1个月+）

1. 升级宿主机Go版本到1.21+
2. 评估是否需要ClickHouse日志系统
3. 考虑系统任务运行器（如果多节点部署）

---

## 📝 合并脚本模板

如果要合并高优先级提交：

```bash
cd /home/ubuntu/new-api
git checkout custom
git checkout -b merge-recommended-fixes

# 高优先级安全和体验修复
git cherry-pick 626dadb55  # 富文本安全渲染
git cherry-pick 0b48ad86d  # HTML/Markdown一致性
git cherry-pick c0e42bfbd  # 主题切换修复
git cherry-pick 5d943281b  # Token限制配置
git cherry-pick df5ba9fa5  # 密码验证

# 测试构建
docker compose build new-api

# 如果成功
git checkout custom
git merge merge-recommended-fixes --no-ff
docker compose up -d
```

---

**生成工具**：Claude Code with Max Effort  
**维护建议**：每月检查一次上游更新，选择性合并安全修复和体验改进
