# M7 RelayKit 差异审计报告

**审计时间**：2026-08-19  
**当前 HEAD**：`b198b4b545153db25b77c1753a168af03db8e99a`（M6 完成状态）  
**官方参考**：`upstream/main` @ `e2c7aa7b102c`  
**RelayKit 提交**：`86ac0f774` (2026-07-27)

---

## 一、时间线澄清

**重要发现**：RelayKit 重构 (86ac0f774) **早于** M6 AutoGroups (0ab020206)。

```
官方提交顺序：
86ac0f774 (2026-07-27) refactor: extract protocol conversion layer into standalone relaykit module (#6369)
↓ 29 commits
0ab020206 (2026-08-??) Feat/auto group (#6590)
↓ 47 commits  
e2c7aa7b1 (当前 upstream/main) test(web): standardize frontend tests on Vitest (#6569)
```

**原因分析**：
- Roadmap 按功能逻辑分组（M6=新端点、M7=RelayKit、M8=测试/依赖）
- 但官方提交时间顺序是：RelayKit → AutoGroups → 后续修复
- **当前定制版已经跳过了 RelayKit，直接移植了 AutoGroups**

---

## 二、RelayKit 重构规模

### 改动统计

```
从当前 HEAD (b198b4b54515) 到 RelayKit (86ac0f774):
2231 files changed, 45008 insertions(+), 220854 deletions(-)
```

**核心改动**（86ac0f774 单次提交）：
- 删除 8929 行
- 新增 620 行
- 净减少约 8300 行

### 主要变更

#### 1. 新增 `relaykit/` 独立模块

**目录结构**：
```
relaykit/
├── dto/                        # 协议 DTO
│   ├── openai_request.go
│   ├── openai_response.go
│   ├── claude_request.go
│   └── gemini_request.go
├── relayconvert/               # 协议转换
│   ├── text_converter_registry.go
│   ├── response_registry.go
│   ├── internal/
│   │   ├── oai_chat/          # OpenAI → 其他协议
│   │   ├── claude_messages/    # Claude → 其他协议
│   │   ├── gemini_chat/        # Gemini → 其他协议
│   │   └── oai_responses/      # Responses → 其他协议
│   └── terminal_stream.go      # 流式处理
├── types/                      # 共享类型
│   ├── price_data.go
│   ├── rw_map.go
│   └── set.go
├── go.mod                      # 独立模块
└── README.md
```

#### 2. 删除原有转换代码

**移除文件**（部分列表）：
- `service/http_client.go` 中的 216 行 HTTP 传输策略代码
- `service/http_transport_policy.go` (91 行，完全删除)
- `service/http_transport_sharded.go` (131 行，完全删除)
- `service/http_client_transport_test.go` (491 行，完全删除)
- `service/tiered_settle_test.go` (247 行，完全删除)
- `service/channel_select_auto_groups_test.go` (129 行，完全删除)
- `service/group_auto_groups_test.go` (72 行，完全删除)

#### 3. 前端 AutoGroups UI 删除

**移除组件**（173 个文件改动）：
- `web/src/features/keys/components/auto-group-order-editor.tsx` (338 行)
- `web/src/features/keys/components/auto-group-visuals.tsx` (140 行)
- `web/src/features/keys/components/api-key-group-cell.tsx` (90 行)
- `web/src/features/keys/__tests__/auto-group-order-editor.test.tsx` (540 行)
- `web/src/features/keys/__tests__/api-key-group-combobox.test.tsx` (294 行)
- `web/src/features/keys/__tests__/api-keys-mutate-drawer.test.tsx` (371 行)
- `web/src/features/keys/lib/__tests__/auto-group-form.test.ts` (189 行)
- 删除 7 个语言的 i18n 条目（en/fr/ja/ru/vi/zh-TW/zh）

**影响分析**：
- **M5 刚移植的 AutoGroups 前端组件在 M7 被官方全部删除**
- 理由：设计系统重构，可能改用不同的组件架构
- 删除的不是功能本身，而是旧的 UI 实现

#### 4. 后端 AutoGroups 配置删除

**移除文件**：
- `setting/auto_group.go` (33 行)
- `setting/auto_group_test.go` (29 行)

**影响**：
- `MaxTokenAutoGroups` 配置逻辑被删除或移动
- 需要确认新的配置存储位置

#### 5. Import Path 更新

**核心变更**：
```go
// 旧导入
import "github.com/QuantumNous/new-api/types"

// 新导入
import "github.com/QuantumNous/new-api/relaykit/types"
```

**影响文件**（部分）：
- `service/billing_usage.go`
- `service/channel_select.go`
- `service/group.go`
- `service/log_info_generate.go`
- `service/task_billing.go`
- `setting/ratio_setting/*.go`

#### 6. trusted_proxies 包重构

**变更**：
```
setting/system_setting/trusted_proxies.go → setting/trusted_proxies.go
```

**影响**：
- 包路径变化
- 导入路径需要全局更新

---

## 三、架构冲突分析

### 1. **与当前定制的冲突**

#### A. AutoGroups UI 冲突
- **M5 移植**：完整的 `auto-group-order-editor`、`auto-group-visuals` 组件
- **RelayKit 删除**：所有这些组件被官方删除
- **冲突性质**：功能保留但 UI 实现全部报废
- **解决方案**：
  - 方案 A：保留 M5 的旧 UI，不跟进官方新 UI
  - 方案 B：等官方新 UI 稳定后再次移植（可能是 M9/M10）
  - **推荐**：方案 A，等官方新设计稳定

#### B. HTTP 传输策略冲突
- **当前实现**：`service/http_client.go` 中的自定义传输策略
- **RelayKit 改动**：删除 216 行传输策略代码
- **影响**：
  - 分片传输 (sharded transport)
  - 传输策略 (transport policy)
  - 连接池管理
- **风险**：可能影响 Seedance、Kling 等自定义渠道的连接管理

#### C. Tiered Billing 测试冲突
- **删除文件**：`service/tiered_settle_test.go` (247 行)
- **影响**：M1-M6 期间依赖的 tiered billing 回归测试被删除
- **风险**：丢失测试覆盖
- **解决方案**：保留当前测试，不删除

### 2. **与 M6 工具计费的关系**

**关键发现**：RelayKit 重构**早于** M6 AutoGroups 和工具计费改动。

**时间线**：
1. RelayKit 重构 (86ac0f774) - 2026-07-27
2. 29 个提交（包括工具计费）
3. M6 AutoGroups (0ab020206) - 2026-08-??

**这意味着**：
- RelayKit 重构时，官方尚未引入 `relay/common/tool_usage.go`
- 工具计费统一是在 RelayKit **之后**才加入的
- M6 暂缓工具计费的决策是正确的，因为它依赖 RelayKit 架构

---

## 四、RelayKit 设计哲学

### 核心理念

**从 RelayKit README 推断**（需要读取完整文件确认）：
1. **协议转换独立化**：将 OpenAI ↔ Claude ↔ Gemini 等转换逻辑提取为独立模块
2. **去中心化转换**：不再在 `relay/` 中硬编码转换，而是通过 registry 注册
3. **类型安全**：独立的 DTO 包，明确的类型定义
4. **可测试性**：独立模块便于单元测试，不依赖整个网关环境

### 与当前架构的对比

**当前架构**（M0-M6 状态）：
```
relay/
├── relay-text.go               # 入口
├── relay_adaptor.go            # 适配器注册
├── channel/                    # 各渠道实现
│   ├── openai/
│   ├── claude/
│   ├── gemini/
│   └── ...
└── common/
    ├── relay_info.go
    └── relay_utils.go
```

**RelayKit 架构**：
```
relaykit/                       # 独立模块
├── dto/                        # 统一 DTO
├── relayconvert/               # 转换注册表
│   └── internal/               # 各协议转换实现
└── types/                      # 共享类型

relay/                          # 网关层（变薄）
├── relay-text.go               # 调用 relaykit
├── relay_adaptor.go
└── channel/                    # 渠道适配（简化）
```

**优势**：
- 协议转换逻辑可以独立测试
- 减少 `relay/` 的职责，专注于路由和计费
- 便于其他项目复用转换逻辑

**劣势**：
- 大规模目录移动，合并冲突风险高
- 需要全局更新 import path
- 与当前定制的渠道适配器可能不兼容

---

## 五、移植决策点

### 决策 1：是否完整引入 RelayKit 模块？

**选项 A：完整引入（高风险）**
- 创建 `relaykit/` 目录
- 移动所有相关代码
- 更新所有 import path
- 删除旧的转换代码
- **风险**：
  - 2231 个文件改动，极易引入 bug
  - 与 Seedance、Kling、Xinhankr 等定制渠道的兼容性未知
  - tiered billing 的 settle 逻辑可能被破坏
  - AutoGroups UI 被删除，需要重新实现或保留旧代码

**选项 B：逐项移植（低风险，但工作量大）**
- 不创建 `relaykit/` 模块
- 在现有 `relay/` 目录下采纳官方的转换逻辑改进
- 保留当前的目录结构
- 只移植有价值的转换改进（如 Gemini → OpenAI 流式优化）
- **优势**：
  - 可控的改动范围
  - 保持定制渠道兼容性
  - tiered billing 不受影响
- **劣势**：
  - 无法享受 RelayKit 的架构优势
  - 未来官方改动仍会基于 RelayKit，持续偏离

**选项 C：暂缓整个 M7（推荐）**
- 跳过 RelayKit 重构
- 直接移植 RelayKit **之后**的功能性改动（M6 → upstream/main 的 47 个提交）
- 等官方 RelayKit 生态稳定后再评估
- **优势**：
  - 避免架构重构风险
  - 当前定制功能不受影响
  - 可以优先移植安全修复和小功能
- **劣势**：
  - 与官方架构持续偏离
  - 未来某个时刻可能被迫重构

### 决策 2：AutoGroups UI 如何处理？

**现状**：
- M5 刚移植了完整的 AutoGroups UI（540 行测试 + 338 行编辑器）
- RelayKit 重构删除了所有这些组件
- 官方可能在设计系统重构中会重新实现

**选项 A：保留 M5 的旧 UI（推荐）**
- 不删除 `auto-group-order-editor` 等组件
- 继续维护旧的 UI 实现
- 等官方新 UI 稳定后再次评估
- **优势**：用户功能不受影响
- **劣势**：与官方前端逐渐偏离

**选项 B：删除并等待官方新 UI**
- 跟随官方，删除旧组件
- 临时移除 AutoGroups 编辑功能
- 等官方新 UI 出现后移植
- **劣势**：功能倒退，用户体验受损

### 决策 3：HTTP 传输策略如何处理？

**删除代码**：
- `service/http_transport_policy.go` (91 行)
- `service/http_transport_sharded.go` (131 行)
- `service/http_client.go` 中的 216 行代码

**问题**：
- 这些策略是否被 Seedance、Kling 等定制渠道使用？
- 删除后是否影响连接池性能？

**调查方向**：
- 搜索当前代码中对 `http_transport_sharded` 的引用
- 确认 Seedance (type 60) 是否依赖自定义传输策略

---

## 六、RelayKit 之后的 29 个提交

**时间范围**：86ac0f774 → 0ab020206 (M6 AutoGroups)

### 关键提交分类

#### 1. AutoGroups 功能 (M6 已移植)
- `0ab020206` Feat/auto group (#6590)

#### 2. Billing 修复 (高优先级)
- `cfaba1dd6` fix(billing): harden tiered retry group-switch billing (#6570)
- `df43f8015` fix(billing): settle tiered retries with final group (#6518)

**影响**：
- Tiered billing 在 group 切换时的计费修复
- **必须移植**，因为涉及计费正确性

#### 3. Relay 修复 (中优先级)
- `8461e5339` fix(relay): preserve multipart image edits for New API channels (#6559)
- `66ee6b8f9` fix: preserve Qwen thinking_budget passthrough (#5836)

#### 4. OAuth 修复 (中优先级)
- `e78e1db1e` fix(oauth): stop treating a foreign window.opener as a bind flow (#6425)

#### 5. 流式日志 (低优先级)
- `84834eee8` feat(logs): expose stream status to log owners (#6558)

#### 6. OIDC 显示名 (低优先级)
- `cb4c8c02f` feat(oidc): 支持自定义 OIDC 登录显示名称 (#6012)

#### 7. 其他 (低优先级)
- `0f9f668c6` feat: support zstd request decompression (#6545)
- `c3db41407` fix: 慢查询/错误 SQL 日志参数化 (#6493)
- `e99a9bd86` feat: add per-channel HTTP transport controls

#### 8. RelayKit 内部改进 (依赖 RelayKit 模块)
- `b27b2b1d6` fix(web): detect iPad login sessions correctly
- `a043eef55` feat: implement Gemini to OpenAI chat stream conversion
- `8a7a49072` feat: add workflow to sync GitHub releases to GitCode
- `8aa5e754a` refactor: rename trusted_proxies package
- `b8bb3f40a` refactor: update import paths to use new types package
- `60a1acb70` refactor: update import paths to use relaykit module

---

## 七、M6 之后的 47 个提交 (当前 upstream/main)

**时间范围**：0ab020206 → e2c7aa7b102c

### 高优先级 (安全/计费)

1. **计费原子性修复**
   - `50e5377ea` fix: harden concurrent quota and status updates
   - `d7992672a` fix(topup): settle recharge orders atomically
   - `2a0ce3475` fix(topup): reject uncreditable orders before payment (#6845)
   - `58d4e9bd3` fix(billing): 异步任务退款时同步减少 used_quota (#6795)
   - `f11641428` fix: settle Responses cached token usage (#6892)

2. **OAuth 安全**
   - `d7992672a` fix(oauth): avoid overwriting user state when binding
   - `116255f07` fix(oauth): align custom binding response fields (#6818)

3. **Relay 正确性**
   - `3dda1d50c` fix(relaykit): preserve parameterless tools in Claude conversion (#6862)
   - `4442bb302` fix(relay): stop injecting empty tools into Claude requests
   - `7d09c6954` fix: prompt_cache_key openai chat -> openai responses (#6861)
   - `93d2df85f` fix(ali): 修复阿里图片模型映射后仍使用原始模型名判断协议的问题 (#6772)
   - `253a74dd1` fix(relay): preserve presence/frequency penalty in Responses conversion (#6654)

### 中优先级 (功能)

1. **渠道测试**
   - `4add708eb` feat: channel test (#6917)

2. **高级自定义渠道**
   - `2b0efd848` refactor: advanced custom channel route editor (#6865)
   - `e90a7c48e` feat: add field passthrough controls for gateway channels (#6847)

3. **前端改进**
   - `137d1171f` feat(web): fade in streamed response words and harden playground editor (#6895)
   - `15cfdedde` fix(web): keep fetched model selection in sync with form (#6841)
   - `ffeb1b24e` fix(web): refresh Turnstile token after login attempt (#6764)
   - `9c97e78ac` fix(web): require confirmation before rotating access token (#6749)
   - `4eaeefbdf` fix: mobile sidebar (#6760)

### 低优先级 (测试/重构)

1. **前端测试标准化**
   - `e2c7aa7b1` test(web): standardize frontend tests on Vitest (#6569)

2. **依赖升级**
   - `bbf67df04` chore(deps-dev): bump electron from 39.8.5 to 39.8.10
   - `cf38105a9` chore(deps-dev): bump js-yaml from 4.3.0 to 4.3.1
   - `e5efc73cd` chore(deps-dev): bump tar from 7.5.16 to 7.5.22
   - 其他依赖升级...

3. **代码清理**
   - `bb234ff41` refactor(responses): remove compact model suffix handling (#6770)

---

## 八、推荐的移植策略

### 策略 A：跳过 RelayKit，优先修复 (推荐)

**路线**：
1. **暂缓 M7 RelayKit 模块重构**
2. **创建 M7.5：计费与安全修复**
   - 移植 tiered billing 修复 (cfaba1dd6, df43f8015)
   - 移植计费原子性修复 (50e5377ea, d7992672a, 2a0ce3475, 58d4e9bd3, f11641428)
   - 移植 OAuth 安全修复 (d7992672a, 116255f07)
3. **创建 M7.6：Relay 正确性修复**
   - 移植 Claude tools 修复 (3dda1d50c, 4442bb302)
   - 移植 Ali 图片修复 (93d2df85f)
   - 移植 Responses 转换修复 (253a74dd1, 7d09c6954)
4. **M8 保持不变**：测试、依赖、Electron、普通 UI
5. **M9（未来）：评估是否引入 RelayKit**

**优势**：
- 避免 2231 文件的高风险重构
- 优先移植生产级修复（计费、安全）
- 保持定制功能（AutoGroups UI、tiered billing、定制渠道）
- 当前架构稳定，减少回归风险

**劣势**：
- 与官方架构持续偏离
- 未来如果官方功能强依赖 RelayKit，移植难度增加

### 策略 B：完整引入 RelayKit (高风险，不推荐)

**路线**：
1. 创建 `relaykit/` 模块
2. 移动所有协议转换代码
3. 更新 2231 个文件的 import path
4. 删除旧的 HTTP 传输策略
5. 删除 AutoGroups UI（或保留作为 legacy）
6. 验证所有定制渠道兼容性

**优势**：
- 与官方架构对齐
- 享受 RelayKit 的架构优势

**劣势**：
- 极高的合并冲突风险
- M5 AutoGroups UI 全部报废
- 定制渠道可能不兼容
- 回归测试工作量巨大
- 至少需要 2-3 周的完整验证

### 策略 C：混合策略 (折中)

**路线**：
1. **不创建** `relaykit/` 模块（保持当前目录结构）
2. **选择性引入** RelayKit 的转换改进
   - 例如：Gemini → OpenAI 流式优化
   - 保留在现有 `relay/channel/gemini/` 中实现
3. **移植** RelayKit 之后的高优先级修复
4. **保留** AutoGroups UI 和 HTTP 传输策略

**优势**：
- 享受部分 RelayKit 改进
- 避免大规模目录移动
- 保持定制功能

**劣势**：
- 代码风格与官方不一致
- 未来移植仍需手工对齐

---

## 九、决策建议

### 立即决策点

**问题 1**：是否完整引入 RelayKit 模块？
- **推荐答案**：❌ 否，暂缓
- **理由**：
  - 2231 文件改动风险过高
  - M5 AutoGroups UI 刚移植完就会被删除
  - 定制渠道兼容性未知
  - 当前架构稳定且满足需求

**问题 2**：M7 如何定义？
- **推荐答案**：重新定义 M7 为"计费与安全修复"，将 RelayKit 推迟到 M9
- **新 M7 范围**：
  - Tiered billing 修复（2 个提交）
  - 计费原子性修复（5 个提交）
  - OAuth 安全修复（2 个提交）
  - Relay 正确性修复（5 个提交）
- **总计**：约 14 个高优先级提交

**问题 3**：AutoGroups UI 如何处理？
- **推荐答案**：保留 M5 的实现，不跟随官方删除
- **理由**：
  - 用户功能不受影响
  - 官方新 UI 尚未稳定
  - 可以在 M9/M10 再次评估

### 需要用户确认的事项

1. **是否同意跳过 RelayKit 模块重构？**
   - 如果同意，M7 重新定义为"计费与安全修复"
   - 如果不同意，需要准备 2-3 周的完整验证周期

2. **是否保留 AutoGroups UI？**
   - 如果保留，与官方前端持续偏离
   - 如果删除，用户丢失编辑功能

3. **M7 的优先级顺序？**
   - 计费修复（最高）
   - 安全修复（高）
   - Relay 正确性（中）
   - 新功能（低）

---

## 十、下一步行动

### 如果用户同意策略 A（推荐）

1. **更新 Roadmap**
   - M7 重新定义为"计费与安全修复"
   - 新增 M9：RelayKit 评估（未来）
   - M8 保持不变

2. **开始 M7.5 差异审计**
   - 详细审计 14 个高优先级提交
   - 评估与定制功能的冲突
   - 编写 M7_BILLING_SECURITY_AUDIT.md

3. **准备验收环境**
   - 确认计费回归测试
   - 准备 OAuth 安全测试
   - 验证 tiered billing 场景

### 如果用户选择策略 B（高风险）

1. **创建 M7 风险评估文档**
   - 定制渠道兼容性矩阵
   - 目录移动计划
   - 回归测试清单

2. **准备隔离分支**
   - 在独立分支进行 RelayKit 移植
   - 完整验证后再合并到 custom

3. **预留 2-3 周验收时间**

---

## 十一、总结

**核心发现**：
1. RelayKit 是一次大规模架构重构，不是简单的功能增强
2. M5 AutoGroups UI 在 RelayKit 中被全部删除
3. RelayKit **早于** M6 AutoGroups，时间线与 Roadmap 逻辑顺序不同
4. 完整引入 RelayKit 风险极高，建议暂缓

**推荐路线**：
- **跳过 RelayKit 模块重构**
- **优先移植计费与安全修复**
- **保留 AutoGroups UI 和定制功能**
- **未来再评估 RelayKit**

**风险提示**：
- 与官方架构持续偏离
- 未来某个时刻可能被迫重构
- 但当前稳定性和可维护性更重要

**等待用户决策**：
1. 是否同意跳过 RelayKit？
2. 是否保留 AutoGroups UI？
3. M7 重新定义为"计费与安全修复"是否可接受？