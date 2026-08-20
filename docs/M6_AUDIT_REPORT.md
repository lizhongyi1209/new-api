# M6 官方差异审计报告

> 审计日期：2026-08-19  
> 审计人：Claude (Opus 5)  
> 官方目标 commit：`2d23cdf29` (2025-01-12)  
> 当前基线：M5 完成状态 (`b198b4b545153db25b77c1753a168af03db8e99a`)  
> 审计范围：New API、Sub2API、Alpha Search、统一工具计费

---

## 执行摘要

### 🔴 阻塞级冲突（必须先决策）

**渠道编号冲突**：官方要占用的编号已被生产数据使用

| 编号 | 官方用途 | 当前生产用途 | 冲突状态 |
|------|---------|-------------|---------|
| 57 | Codex | Codex | ✅ 无冲突 |
| 58 | AdvancedCustom | **TencentVideo** | 🔴 冲突 |
| 59 | Sub2API | **AdvancedCustom** | 🔴 冲突 |
| 60 | — | **ServiceInferenceVideo** | ⚠️ 当前独有 |
| 61 | — | **xinhankr** | ⚠️ 当前独有 |
| 62 | — | **iLiuMidjourney** | ⚠️ 当前独有 |

**影响面**：
- 生产数据库 `channels` 表已有 type 58-62 的记录
- 用户配置、计费日志、能力映射都依赖这些编号
- 不能简单覆盖，否则会破坏现有渠道

### 🟠 高风险改动

**工具计费架构重构** (+542/-118 行)：
- 删除 `service/tool_billing.go` (86 行)
- 新增 `relay/common/tool_usage.go` (194 行)
- 重构 `service/text_quota.go` (245 行改动)
- 新增 348 行测试用例

**潜在冲突点**：
1. 当前 tiered billing 已有 `OtherRatios` 机制（异步任务、工具附加费）
2. 官方新增 `ToolSurchargeItems` 机制（工具附加费率）
3. 两者是否会重复计费？需详细分析

### 📊 代码规模

**后端**：65 文件，+3203/-424 行
- 核心：10 个文件，约 1500 行改动
- 测试：10 个新测试文件，约 1000 行
- 新渠道：Sub2API (121行)、Alpha Search (136行)

**前端**：Default 有改动，Classic 未同步
- 工具价格设置 UI：101 行
- 日志页面工具费显示：144 行
- 新增 3 个 Vitest 测试：399 行

---

## 一、渠道编号冲突详细分析

### 1.1 冲突矩阵

```
官方布局（commit 2d23cdf29）:
├─ 57: Codex (已有)
├─ 58: AdvancedCustom (新占用)
└─ 59: Sub2API (新增)

当前生产布局:
├─ 57: Codex ✅
├─ 58: TencentVideo 🔴 (云视频推理，有生产数据)
├─ 59: AdvancedCustom 🔴 (高级自定义，有生产数据)
├─ 60: ServiceInferenceVideo 🟡 (Seedance，有生产数据)
├─ 61: xinhankr 🟡 (套娃网关，有生产数据)
└─ 62: iLiuMidjourney 🟡 (MJ代理，有生产数据)
```

### 1.2 三个解决方案对比

#### 方案 A：官方新渠道改编号（推荐 ✅）

**做法**：
- Sub2API 改用新编号 **63**
- 官方 AdvancedCustom 的增强功能移植到当前 **59 号位**
- 保持 58-62 生产编号不变

**优点**：
- ✅ 零数据迁移
- ✅ 零停机时间
- ✅ 现有渠道配置完全不受影响
- ✅ 实施风险最低

**缺点**：
- ⚠️ 与官方编号不一致（但本项目本就是定制版）
- ⚠️ 未来同步官方时需注意编号映射

**实施成本**：极低（只改 `constant/channel.go` 一处）

---

#### 方案 B：数据迁移腾出 58-59

**做法**：
1. 新增编号 63-67
2. 数据库迁移：TencentVideo 58→63、AdvancedCustom 59→64、ServiceInferenceVideo 60→65、xinhankr 61→66、iLiu 62→67
3. 腾出 58-59 给官方 AdvancedCustom 和 Sub2API

**优点**：
- ✅ 与官方编号完全一致

**缺点**：
- 🔴 需写数据库迁移脚本（跨 SQLite/MySQL/PostgreSQL）
- 🔴 需迁移 `channels`、`abilities`、`logs` 等多表
- 🔴 需停机或灰度发布
- 🔴 回滚复杂

**实施成本**：高（需 3-5 天开发+测试）

---

#### 方案 C：不引入 Sub2API，只移植工具计费

**做法**：
- 只移植统一工具计费架构
- 不要 Sub2API 渠道（官方 59 号位）
- 不要 Alpha Search 端点
- 官方 AdvancedCustom 增强功能可选移植到当前 59

**优点**：
- ✅ 避开编号冲突
- ✅ 聚焦核心价值（工具计费）

**缺点**：
- ⚠️ Sub2API 和 Alpha Search 功能缺失（但生产暂无需求）

**实施成本**：中（需详细分析工具计费与 tiered billing 兼容性）

---

### 1.3 推荐方案

**推荐方案 A**，理由：
1. 本项目本就是深度定制版（有返佣、异步任务、Seedance 等官方没有的功能）
2. 渠道编号不一致不影响功能，只影响文档对照
3. 生产数据优先，零风险优先
4. 官方也在持续开发，未来可能再新增渠道占用其他编号

---

## 二、工具计费架构重构分析

### 2.1 官方改动概览

**删除文件**：
- `service/tool_billing.go` (86 行) — 旧的工具计费逻辑

**新增文件**：
- `relay/common/tool_usage.go` (194 行) — 新的统一工具使用跟踪
- `relay/common/tool_usage_test.go` (348 行) — 完整测试覆盖

**重构文件**：
- `service/text_quota.go` — 计费入口，+146/-99 行
- `setting/operation_setting/tools.go` — 工具费率配置，+34 行

### 2.2 新旧架构对比

#### 旧架构（service/tool_billing.go）

```go
// 在各个渠道适配器内部零散处理
func (a *Adaptor) DoResponse(...) {
    // Claude: 检测 web_search
    // Gemini: 无检测（缺失）
    // OpenAI: 检测 file_search/code_interpreter
    // 各自调用 service.CalculateToolUsageQuota()
}
```

**问题**：
- 逻辑分散在多个 relay/channel/* 适配器
- Gemini grounding 检测缺失
- 难以扩展新工具类型

#### 新架构（relay/common/tool_usage.go）

```go
// 统一在 relay/common 层收集
type ToolUsage struct {
    ClaudeWebSearch      int  // Claude web_search 次数
    GeminiGoogleSearch   int  // Gemini grounding 次数
    OpenAIWebSearch      int  // OpenAI web_search_preview 次数
    OpenAIFileSearch     int  // OpenAI file_search 次数
    OpenAICodeInterpreter int // OpenAI code_interpreter 次数
}

// service/text_quota.go 统一计费
func calculateTextToolCallSurcharge(toolUsage *relaycommon.ToolUsage) int64 {
    // 从 setting.GetToolPrices() 读取费率
    // 统一计算附加费
}
```

**优点**：
- ✅ 逻辑集中，易维护
- ✅ Gemini grounding 检测补全
- ✅ 扩展新工具类型只需改一处
- ✅ 348 行测试覆盖

### 2.3 与当前定制功能的兼容性分析

#### 冲突点 1：tiered billing 的 OtherRatios

**当前实现**（pkg/billingexpr/）：
```go
// 表达式可以引用 OtherRatios
tier("base", p * 2.5 + c * 15) ||| when(param("tools")) * OtherRatios["web_search"]
```

**官方实现**（setting/operation_setting/tools.go）：
```go
type ToolSurchargeItem struct {
    ToolType string  `json:"tool_type"`  // "web_search", "file_search", ...
    Enabled  bool    `json:"enabled"`
    Price    float64 `json:"price"`      // 每次调用附加费（$/百万tokens）
}
```

**是否冲突？** 🟡 **可能重复计费**

- tiered billing 的 `OtherRatios` 是倍率（乘以基础价格）
- 官方的 `ToolSurchargeItem.Price` 是绝对价格（$/百万tokens）
- 如果同一个请求同时触发两套逻辑，会重复扣费

**解决方案**：
1. **方案 A**：废弃 `OtherRatios`，统一用官方 `ToolSurchargeItem`
2. **方案 B**：保留 `OtherRatios`，禁用官方工具计费
3. **方案 C**：在 `text_quota.go` 增加互斥检测，有 tiered billing 时跳过官方工具计费

---

#### 冲突点 2：异步任务退款

**当前实现**（service/task_billing.go）：
```go
// 异步任务失败时退款，包含工具附加费
func RefundTaskQuota(...) {
    // 退还 prompt + completion + 工具附加费（如果有）
}
```

**官方实现**：
- 没有异步任务概念
- 工具计费在同步流程内完成

**是否冲突？** 🟡 **需补充退款逻辑**

**解决方案**：
- 异步任务失败时，需同步退还 `ToolUsage` 统计的附加费
- 需在 `task_billing.go` 中增加工具费退款分支

---

#### 冲突点 3：Gemini 异步任务的 grounding 检测

**官方实现**（relay/channel/gemini/relay-gemini.go）：
```go
// 在同步响应流中检测 GroundingMetadata
func (a *Adaptor) markGeminiGoogleSearchCall(...) {
    if resp.GroundingMetadata != nil && len(resp.GroundingMetadata.WebSearchQueries) > 0 {
        relayInfo.ToolUsage.GeminiGoogleSearch = len(...)
    }
}
```

**当前定制**：
- Gemini 有异步图片/视频任务（`relay/channel/gemini/task.go`）
- 异步任务的响应不走 `DoResponse` 流程

**是否冲突？** 🟡 **异步路径缺失检测**

**解决方案**：
- 异步任务轮询结果时，需增加 `GroundingMetadata` 检测
- 或明确文档说明：异步任务不支持工具计费（因为无法实时检测）

---

### 2.4 测试覆盖情况

官方新增测试：
- `relay/common/tool_usage_test.go` (348 行)
  - `TestToolUsageAdd` — 合并逻辑
  - `TestToolUsageToQuota` — 费率计算
  - `TestGetToolTypeFromToolCall` — OpenAI 工具类型识别

**覆盖率**：✅ 核心逻辑全覆盖

**当前需补充的测试**：
- tiered billing 与工具计费互斥测试
- 异步任务工具费退款测试
- Gemini 异步 grounding 检测测试（或明确不支持）

---

## 三、新增功能详细分析

### 3.1 Alpha Search 端点

**官方新增**：
- `POST /v1/alpha/search` (类似 Perplexity 搜索)
- `dto/alpha_search_request.go` + `relay/alpha_search_handler.go` (共 136 行)

**功能**：
- 搜索引擎集成（需上游支持）
- 返回搜索结果 + AI 总结

**当前生产需求**：❓ **未知**
- 没有用户请求过此功能
- 没有支持此端点的上游渠道配置

**建议**：⏸️ **暂不移植**，理由：
1. 功能独立，不影响工具计费核心
2. 生产无需求
3. 减少 M6 复杂度

---

### 3.2 Sub2API 渠道

**官方新增**：
- `relay/channel/sub2api/` (121 行)
- 类似 Codex，基于订阅 token 的代理渠道

**功能**：
- 用户提供订阅 URL + token
- 网关透传请求到订阅服务

**当前生产**：
- 已有 Codex 渠道（功能类似）
- 未见 Sub2API 需求

**建议**：⏸️ **暂不移植**，理由：
1. 避开 59 号编号冲突
2. Codex 已满足订阅代理需求
3. 可在 M7（RelayKit）时再评估

---

### 3.3 Codex 渠道增强

**官方改动**：
- `relay/channel/codex/main.go` 增加 `SupportsStreamOptions()` 方法

**冲突**：❌ 无

**建议**：✅ **可直接移植**，只是小改进

---

## 四、前端改动分析

### 4.1 Default 前端改动

**新增文件**：
- `web/default/src/pages/Setting/Operation/components/ToolPrices.jsx` (101 行)
  - 工具价格设置 UI
  - 支持启用/禁用、设置单价

**修改文件**：
- `web/default/src/components/LogsTable.jsx` (+144 行)
  - 日志页面新增"工具附加费"列
  - 显示每条请求的工具使用明细

**测试**：
- `web/default/src/pages/Setting/Operation/__tests__/tool-price-validation.test.js` (185 行)
- `web/default/src/components/__tests__/cost-display.test.js` (114 行)
- `web/default/src/components/__tests__/tool-surcharge.test.js` (100 行)

**总计**：+544 行（含测试）

### 4.2 Classic 前端状态

**官方改动**：❌ **未同步**

**当前状态**：
- Classic 仍然只有旧的工具计费 UI（如果有的话）
- 无工具价格设置入口
- 日志页面不显示工具附加费

**需决策**：
- 🤔 是否需要手工移植到 Classic？
- 🤔 Classic 是否还在生产使用？

---

## 五、数据库 & 配置影响

### 5.1 数据库变更

**新增表**：❌ 无

**字段变更**：❌ 无

**配置存储**：
- 工具价格存储在现有 `options` 表（JSON）
- Key: `ToolPrices`
- Value: `[{"tool_type":"web_search","enabled":true,"price":0.5}, ...]`

### 5.2 环境变量

**新增**：❌ 无

**修改**：❌ 无

---

## 六、测试策略

### 6.1 必须通过的测试

按 Roadmap M6 流程：

1. **代码级验证**：
   ```bash
   go test ./relay/common -run ToolUsage
   go test ./service -run ToolCall
   go test ./setting/operation_setting -run Tool
   ```

2. **跨数据库验证**：
   ```bash
   # 测试工具价格配置在三数据库的读写
   go test ./model -run Option.*Tool
   ```

3. **前端验证**：
   ```bash
   cd web/default
   bun run typecheck
   bun run test -- tool
   bun run build
   ```

4. **隔离镜像验证**：
   - 启动测试栈（docker-compose.upstream-test.yml）
   - 设置工具价格
   - 发送带工具调用的请求
   - 检查日志中的工具附加费

### 6.2 回归测试

**必须确保不破坏**：
- M1-M5 已有测试（尤其是 tiered billing）
- 异步任务计费与退款
- Gemini 异步图片/视频任务

---

## 七、风险评估矩阵

| 风险项 | 等级 | 影响面 | 缓解措施 |
|--------|------|--------|----------|
| 渠道编号冲突 | 🔴 Critical | 生产数据 | 方案 A：官方新渠道改编号 |
| 工具计费重复扣费 | 🟠 High | 计费准确性 | 互斥检测 or 废弃 OtherRatios |
| 异步任务工具费退款 | 🟠 High | 退款准确性 | 补充退款逻辑 |
| Gemini 异步 grounding | 🟡 Medium | 功能完整性 | 补充检测 or 文档说明不支持 |
| Classic 前端未同步 | 🟡 Medium | 用户体验 | 确认 Classic 是否还在用 |
| Alpha Search 缺失 | 🟢 Low | 新功能 | 生产无需求，暂不移植 |
| Sub2API 缺失 | 🟢 Low | 新功能 | Codex 已覆盖，暂不移植 |

---

## 八、决策清单

### 必须决策（阻塞项）

#### 决策 1：渠道编号方案
- [ ] **方案 A**：Sub2API 用 63，保持 58-62 不变（推荐）
- [ ] **方案 B**：数据迁移腾出 58-59
- [ ] **方案 C**：不要 Sub2API，只移植工具计费

**我的推荐**：方案 A

---

#### 决策 2：工具计费与 tiered billing 兼容策略
- [ ] **方案 A**：废弃 `OtherRatios`，统一用 `ToolSurchargeItem`
- [ ] **方案 B**：保留 `OtherRatios`，禁用官方工具计费
- [ ] **方案 C**：互斥检测，有 tiered billing 时跳过官方计费

**我的推荐**：方案 C（保持向后兼容）

---

### 可选决策（不阻塞）

#### 决策 3：是否移植 Alpha Search？
- [ ] 移植（+136 行，需测试）
- [ ] 不移植（生产无需求）

**我的推荐**：不移植

---

#### 决策 4：是否移植 Sub2API？
- [ ] 移植（需解决编号冲突）
- [ ] 不移植（Codex 已覆盖）

**我的推荐**：不移植（除非方案 A 被采纳且有需求）

---

#### 决策 5：Classic 前端是否同步？
- [ ] 手工移植工具价格 UI 到 Classic
- [ ] Classic 不支持工具计费 UI（只在 Default 支持）

**需先确认**：Classic 是否还在生产使用？

---

## 九、实施建议

### 推荐实施路径（基于方案 A + 方案 C）

#### 阶段 1：最小可用移植（核心价值）
1. 渠道编号：Sub2API 改用 63（如果要移植）
2. 工具计费：
   - 移植 `relay/common/tool_usage.go` + 测试
   - 移植 `service/text_quota.go` 改动
   - 增加与 tiered billing 的互斥检测
3. Gemini grounding：
   - 移植同步路径检测
   - 异步路径文档说明不支持
4. 前端：移植 Default 的工具价格 UI
5. 测试：完整跑通 M6 测试流程

**预计工作量**：3-5 天

---

#### 阶段 2：补充完善（可选）
1. 异步任务工具费退款逻辑
2. Classic 前端同步（如需要）
3. Codex 增强功能移植
4. Alpha Search / Sub2API（如需要）

**预计工作量**：2-3 天

---

#### 阶段 3：镜像验收
按 Roadmap 标准流程：
- 构建候选镜像
- 启动隔离测试栈
- 跨数据库验证
- E2E 验收
- 定制回归测试

**预计工作量**：1-2 天

---

### 总预计时间
- **最小路径**：4-7 天
- **完整路径**：7-10 天

---

## 十、附录

### 附录 A：官方 commit 2d23cdf29 文件清单

```
commit 2d23cdf29 (2025-01-12)
Author: Calcium-Ion

65 files changed, 3203 insertions(+), 424 deletions(-)

删除文件:
- service/tool_billing.go (86 行)

新增文件:
- dto/alpha_search_request.go (50 行)
- dto/alpha_search_response.go (28 行)
- relay/alpha_search_handler.go (136 行)
- relay/channel/sub2api/adaptor.go (64 行)
- relay/channel/sub2api/main.go (57 行)
- relay/common/tool_usage.go (194 行)
- relay/common/tool_usage_test.go (348 行)
- setting/operation_setting/tools.go (34 行)
- web/default/src/pages/Setting/Operation/components/ToolPrices.jsx (101 行)
- web/default/src/pages/Setting/Operation/__tests__/tool-price-validation.test.js (185 行)
- web/default/src/components/__tests__/cost-display.test.js (114 行)
- web/default/src/components/__tests__/tool-surcharge.test.js (100 行)

重点修改文件:
- constant/channel.go (+3 行) — 新增 58/59 编号
- service/text_quota.go (+146/-99 行) — 工具计费重构
- relay/channel/gemini/relay-gemini.go (+27 行) — grounding 检测
- web/default/src/components/LogsTable.jsx (+144 行) — 工具费显示
- controller/relay.go (+2 行) — Alpha Search 路由
- middleware/distributor.go (+7 行) — 工具使用初始化
- dto/openai_response.go (+54 行) — Responses 内置工具
- dto/gemini.go (+47 行) — GroundingMetadata
```

### 附录 B：当前生产渠道配置

```
渠道编号使用情况（从生产数据库推断）:

57: Codex — 订阅代理渠道
58: TencentVideo — 腾讯云 VCLM 视频生成（有生产数据）
59: AdvancedCustom — 高级自定义渠道（有生产数据）
60: ServiceInferenceVideo — Seedance 视频服务（有生产数据）
61: xinhankr — 套娃网关（有生产数据）
62: iLiuMidjourney — MJ 代理（有生产数据）

已确认有生产数据的渠道不能改编号，否则会破坏:
- 渠道配置（channels 表）
- 能力映射（abilities 表）
- 计费日志（logs 表）
- 用户配置的渠道分组
```

### 附录 C：tiered billing OtherRatios 使用情况

当前 `pkg/billingexpr/` 支持的 OtherRatios：
```go
// 表达式示例（当前生产可能在用）
tier("base", p * 2.5 + c * 15) ||| when(param("tools")) * OtherRatios["web_search"]
```

**与官方 ToolSurchargeItem 的区别**：
- `OtherRatios` 是倍率（无量纲）
- `ToolSurchargeItem.Price` 是绝对价格（$/百万tokens）

**兼容方案**：
```go
// 在 service/text_quota.go 增加互斥检测
if relayInfo.IsTieredBilling() && relayInfo.HasOtherRatios() {
    // 使用 tiered billing 的工具计费
    // 跳过官方 ToolSurchargeItem 逻辑
} else {
    // 使用官方统一工具计费
    surcharge := calculateTextToolCallSurcharge(relayInfo.ToolUsage)
}
```

### 附录 D：Gemini 异步任务现状

当前定制的 Gemini 异步任务：
- `relay/channel/gemini/task.go` — 异步图片/视频生成
- 不走 `DoResponse` 流程
- 结果通过定时轮询获取

**grounding 检测问题**：
- 官方检测在 `DoResponse` 的响应流中
- 异步任务的响应不经过该流程
- 需要在轮询结果时单独检测 `GroundingMetadata`

**建议**：
1. 短期：文档说明异步任务不支持 grounding 计费
2. 长期：在异步结果轮询中增加检测逻辑

---

## 十一、后续行动

### 等待你的决策

请在以下 5 个决策点做出选择：

1. ✅ **渠道编号方案**：A / B / C？（推荐 A）
2. ✅ **工具计费兼容**：A / B / C？（推荐 C：互斥检测）
3. ⚪ **Alpha Search**：要 / 不要？（推荐不要）
4. ⚪ **Sub2API**：要 / 不要？（推荐不要）
5. ⚪ **Classic 同步**：要 / 不要？（需先确认 Classic 是否在用）

### 决策后的下一步

你决策完成后，我会：
1. 按你的选择制定详细实施计划
2. 开始逐文件移植代码
3. 编写必要的兼容层
4. 补充测试用例
5. 执行完整的 M6 验收流程

---

**审计报告完成日期**：2026-08-19  
**下次更新**：等待决策后开始实施

