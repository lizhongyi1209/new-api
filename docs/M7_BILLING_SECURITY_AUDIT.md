# M7 计费与安全修复审计报告

**审计时间**：2026-08-19  
**当前 HEAD**：`b198b4b545153db25b77c1753a168af03db8e99a`（M6 完成状态）  
**官方参考**：`upstream/main` @ `e2c7aa7b102c`  
**审计范围**：RelayKit (86ac0f774) 之后的 14 个高优先级提交

---

## 一、执行摘要

**审计结论**：14 个高优先级提交中，**12 个已经存在于当前代码**，**2 个已实际移植**（`93d2df85f`、`3dda1d50c`）。

> **2026-08-19 更正**：本报告初版把全部 14 个都记为"已存在、无需移植"，这不准确。实际做了代码改动的两项见下方"三、已移植的提交"。第二节中 93d2df85f 和 3dda1d50c 的"已存在"结论已随之更正。

**已存在提交**（12 个）：
1. ✅ cfaba1dd6 - tiered billing group-switch 修复（已有）
2. ✅ df43f8015 - tiered billing final group 修复（已有）
3. ✅ 50e5377ea - concurrent quota/status 修复（已有）
4. ✅ d7992672a - topup atomic settlement（已有）
5. ✅ 2a0ce3475 - topup uncreditable rejection（已有）
6. ✅ 58d4e9bd3 - async task refund used_quota（已有）
7. ✅ f11641428 - Responses cached token usage（已有）
8. 🔧 3dda1d50c - Claude parameterless tools（**已移植**）
9. ✅ 4442bb302 - Claude empty tools injection（已有）
10. ✅ 253a74dd1 - Responses penalty preservation（已有）
11. ✅ 7d09c6954 - Responses prompt_cache_key（已有）
12. ✅ 116255f07 - OAuth custom binding response（已有）
13. 🔧 93d2df85f - Ali 图片模型映射修复（**已移植**）
14. ✅ d7992672a (OAuth 部分) - OAuth 绑定状态保护（已有）

**原因分析**：
- 当前定制版在 M0-M6 期间已经独立实现了绝大多数修复
- 特别是 tiered billing、quota atomicity、OAuth 安全等核心功能已经超前官方
- RelayKit 之后的"修复"提交，很多是在修复 RelayKit 重构本身引入的问题
- 例外：Ali 图片模型映射与 Claude 无参数工具两处确实存在缺陷，已在本单元修复

---

## 二、详细审计记录

### 1. cfaba1dd6 - tiered billing group-switch billing

**官方改动**：
- `service/tiered_settle.go`：修复 tiered retry 时切换分组后计费不正确
- 确保使用 final group 的 GroupRatioInfo 来结算

**当前代码状态**：✅ **已存在**
- 文件位置：`service/tiered_settle.go:111-125`
- 逻辑：`GroupRatioInfo` 在 `middleware/distributor.go:331` 刷新后，tiered 结算已经使用最终分组的快照
- 验证方法：
  ```bash
  grep -A10 "c.Set.*GroupRatioInfo" middleware/distributor.go
  grep -A15 "func SettleTieredBilling" service/tiered_settle.go
  ```

**结论**：无需移植。

---

### 2. df43f8015 - tiered billing final group settlement

**官方改动**：
- `service/tiered_settle.go`：确保 tiered retry 最终使用成功渠道的分组结算
- 防止中间失败渠道的计费泄漏

**当前代码状态**：✅ **已存在**
- 文件位置：`service/tiered_settle.go:111-125`
- 逻辑：`SettleTieredBilling` 从 context 读取 `GroupRatioInfo`，已经是最终分组快照
- 与 cfaba1dd6 同一机制

**结论**：无需移植。

---

### 3. 50e5377ea - concurrent quota and status updates

**官方改动**：
- `controller/relay.go`：使用 `BillingSession` 统一管理 quota 和 status 更新
- 防止并发更新导致的数据不一致

**当前代码状态**：✅ **已存在**
- 文件位置：`service/billing_session.go`（完整实现）
- 功能：
  - `BillingSession` 统一 quota、status、arrears 管理
  - `reserveFunding` 支持欠费语义（balance 可为负）
  - `SettleBilling` 原子结算差额
- 验证方法：
  ```bash
  grep -A20 "type BillingSession" service/billing_session.go
  grep -A10 "func.*reserveFunding" service/billing_session.go
  ```

**结论**：无需移植。当前实现更完整。

---

### 4. d7992672a - topup atomic settlement

**官方改动**：
- `controller/topup.go`：充值订单结算改为原子操作
- 防止重复充值或充值丢失

**当前代码状态**：✅ **已存在**
- 文件位置：`model/topup.go:88-108`
- 逻辑：`CompleteTopUp` 使用事务 + `SELECT FOR UPDATE` 锁定订单
- 验证方法：
  ```bash
  grep -A20 "func CompleteTopUp" model/topup.go
  ```

**OAuth 部分**：✅ **已存在**
- 官方同一提交还修复了 OAuth 绑定覆盖用户状态的问题
- 详见第 14 项，已实测确认无需移植

**结论**：充值与 OAuth 两部分均已存在，无需移植。

---

### 5. 2a0ce3475 - topup uncreditable order rejection

**官方改动**：
- `service/topup.go`：支付前检查订单是否可充值
- 防止已完成或已关闭订单被重复支付

**当前代码状态**：✅ **已存在**
- 文件位置：`model/topup.go:51-66`
- 逻辑：`RedeemTopUp` 在数据库层锁定并检查 `status`
- 验证方法：
  ```bash
  grep -A20 "func RedeemTopUp" model/topup.go
  ```

**结论**：无需移植。

---

### 6. 58d4e9bd3 - async task refund used_quota

**官方改动**：
- `service/task_billing.go`：异步任务退款时同步减少 `used_quota`
- 防止 `used_quota` 累积导致配额泄漏

**当前代码状态**：✅ **已存在**
- 文件位置：`service/task_billing.go:91-107`
- 逻辑：
  - `RefundTaskQuota` 调用 `model.DecreaseUserUsedQuota`
  - `DecreaseUserUsedQuota` 同时减少 `quota` 和 `used_quota`
- 验证方法：
  ```bash
  grep -A15 "func RefundTaskQuota" service/task_billing.go
  grep -A10 "func DecreaseUserUsedQuota" model/user.go
  ```

**结论**：无需移植。

---

### 7. f11641428 - Responses cached token usage

**官方改动**：
- `relay/responses_handler.go`：正确结算 Responses API 缓存命中的 token
- 防止缓存命中时 token 计费错误

**当前代码状态**：✅ **已存在**
- 文件位置：`relay/responses_handler.go:156-170`
- 逻辑：`DoResponsesRequest` 正确解析 `usage` 并传递给计费
- 验证方法：
  ```bash
  grep -A20 "func DoResponsesRequest" relay/responses_handler.go
  ```

**结论**：无需移植。

---

### 8. 3dda1d50c - Claude parameterless tools preservation

**官方改动**：
- RelayKit 转换层：保留没有 parameters 的 Claude tools
- 防止工具定义在转换时被错误删除

**当前代码状态**：🔧 **已移植**（2026-08-19）

移植前：`RequestOpenAI2ClaudeMessage` 里的工具转换循环用类型断言取 `tool.Function.Parameters.(map[string]any)`，断言失败就 `continue`，**静默丢弃整个工具定义**。无参数工具（`parameters` 为 `nil` 或 `{}`）因此不会出现在发往 Claude 的请求里，模型无法调用它。

移植内容：
- 新增 `relay/channel/claude/schema.go`，导出 `FunctionParametersToInputSchema`：拷贝原有键，并在缺失时补 `type: "object"` 与空 `properties`，使无参数工具得到一个合法的空对象 schema。
- 改写 `relay/channel/claude/relay-claude.go` 的转换循环：只在既不是 `map[string]any` 又不是 `type == "function"` 时跳过，其余一律通过 `FunctionParametersToInputSchema` 转换后加入 `claudeTools`。

验证：`go build ./relay/... ./service/...`、`go vet`、`gofmt`、`go test ./relay/... ./service/...` 全部通过。

---

### 9. 4442bb302 - Claude empty tools injection

**官方改动**：
- 停止向 Claude 请求注入空 tools 数组
- 防止 Claude API 拒绝空工具定义

**当前代码状态**：✅ **已存在**（与修复 8 相邻但不同的代码段）
- 文件位置：`relay/channel/claude/relay-claude.go:118-120`
- 逻辑：`if len(claudeTools) > 0 { claudeRequest.Tools = claudeTools }`，工具列表为空时不设置 `Tools` 字段
- 说明：移植修复 8 时确认此守卫已存在，无需改动

**结论**：无需移植。

---

### 10. 93d2df85f - Ali image model mapping fix

**官方改动**：
- `relay/channel/ali/adaptor.go`：将所有 `info.OriginModelName` 改为 `info.UpstreamModelName`
- 确保图片模型判断使用映射后的模型名，而不是原始模型名

**当前代码状态**：🔧 **已移植**（2026-08-19）

移植前：图片分支用 `info.OriginModelName`（客户端原始模型名）做协议判断。当渠道配了模型映射时，映射后的真实上游模型名在 `info.UpstreamModelName` 里，用原始名判断会走错端点、错设异步头。

移植内容（3 处编辑，共 9 个判断点）：
- `GetRequestURL` 图片分支：`isSyncImageModel` / `isOldWanModel` / `isWanModel` 改用 `info.UpstreamModelName`
- `SetupRequestHeader`：`X-DashScope-Async` 的设置判断改用 `info.UpstreamModelName`
- `ConvertImageRequest`：三处模型判断改用 `info.UpstreamModelName`

- 文件位置：`relay/channel/ali/adaptor.go:109-195`
- 移植后逻辑：所有图片模型判断（`isSyncImageModel`、`isWanModel`、`isOldWanModel`）都使用 `info.UpstreamModelName`
- 验证方法：
  ```bash
  grep -n "OriginModelName" relay/channel/ali/adaptor.go
  # 返回空，说明主逻辑不使用 OriginModelName
  
  grep -n "UpstreamModelName" relay/channel/ali/adaptor.go
  # 行109, 115, 117, 142, 149, 182, 191, 194, 195 - 全部正确
  ```
- 关键代码：
  ```go
  // 行109-113：图片生成 URL 判断
  if isSyncImageModel(info.UpstreamModelName) {
      fullRequestURL = fmt.Sprintf("%s/api/v1/services/aigc/multimodal-generation/generation", info.ChannelBaseUrl)
  } else {
      fullRequestURL = fmt.Sprintf("%s/api/v1/services/aigc/text2image/image-synthesis", info.ChannelBaseUrl)
  }
  
  // 行142-147：异步头设置
  if isSyncImageModel(info.UpstreamModelName) {
      // 同步模型不设置异步头
  } else {
      req.Set("X-DashScope-Async", "enable")
  }
  
  // 行182-200：图片请求转换
  if isSyncImageModel(info.UpstreamModelName) {
      a.IsSyncImageModel = true
  }
  if isOldWanModel(info.UpstreamModelName) {
      return oaiFormEdit2WanxImageEdit(c, info, request)
  }
  if isWanModel(info.UpstreamModelName) {
      a.IsSyncImageModel = false
  }
  ```

**结论**：已移植完成。验证：`go build ./relay/... ./service/...`、`go vet ./relay/channel/ali/...`、`gofmt`、`go test ./relay/channel/ali/...` 全部通过。注意 `adaptor_test.go:104` 仍保留 `OriginModelName` 作为夹具，属预期。

---

### 11. 253a74dd1 - Responses penalty preservation

**官方改动**：
- Chat → Responses 转换时保留 `frequency_penalty` 和 `presence_penalty`
- 防止参数在转换时丢失

**当前代码状态**：✅ **已存在**
- 文件位置：`service/relayconvert/chat_to_responses.go:376-380, 401-402`
- 逻辑：
  ```go
  if req.FrequencyPenalty != nil {
      frequencyPenaltyRaw, _ = common.Marshal(req.FrequencyPenalty)
  }
  if req.PresencePenalty != nil {
      presencePenaltyRaw, _ = common.Marshal(req.PresencePenalty)
  }
  ```

**结论**：无需移植。

---

### 12. 7d09c6954 - Responses prompt_cache_key

**官方改动**：
- Chat → Responses 转换时保留 `prompt_cache_key`
- 支持缓存优化参数透传

**当前代码状态**：✅ **已存在**
- 文件位置：`service/relayconvert/chat_to_responses.go:384-385, 407`
- 逻辑：
  ```go
  if req.PromptCacheKey != "" {
      promptCacheKeyRaw, err = common.Marshal(req.PromptCacheKey)
  }
  ```

**结论**：无需移植。

---

### 13. 116255f07 - OAuth custom binding response alignment

**官方改动**：
- `controller/oauth.go`：对齐自定义 OAuth 绑定响应字段
- 统一绑定成功/失败的响应格式

**当前代码状态**：✅ **已存在**
- 文件位置：`controller/oauth.go:429-450`
- 逻辑：`OAuthBind` 返回标准化的 JSON 响应
- 验证方法：
  ```bash
  grep -A20 "func OAuthBind" controller/oauth.go
  ```

**结论**：无需移植。

---

### 14. d7992672a (OAuth 部分) - avoid overwriting user state

**官方改动**：
- `controller/oauth.go` / `controller/wechat.go`：绑定不再走"读完整用户 → 改一个字段 → `user.Update` 整体回写"，改为只 UPDATE 绑定列
- 新增 `model.UpdateUserBindColumn` + `userBindColumns` 白名单、`oauth.Provider.ProviderUserIDColumn()`
- 修复的实质是并发安全：读快照期间发生的封禁、降权、改分组会被旧快照整体覆盖恢复

**当前代码状态**：✅ **已存在**（2026-08-19 实测确认）

> **本节初版更正**：这里原写"⚠️ 需要检查 / 移植优先级 🟡 中"，与第一节摘要的"已存在"矛盾，且当时未实际查证。现已逐项 grep + 跑测试确认，摘要的结论是对的，本节的待办是初版遗留。

- `controller/oauth.go:280-296`：已是 `userId := pendingFlow.UserId` + `model.UpdateUserBindColumn(userId, provider.ProviderUserIDColumn(), ...)`，注释与官方一致
- `controller/wechat.go:175`：微信路径同样走 `UpdateUserBindColumn(userId, "wechat_id", ...)`
- `model/user.go:177-195`：`userBindColumns` 白名单（github/discord/oidc/linux_do/wechat）+ `UpdateUserBindColumn` 齐全
- 5 个 provider（discord/generic/github/linuxdo/oidc）均实现 `ProviderUserIDColumn()`，`oauth/provider.go:37` 有接口声明
- `model/user_update_test.go` 三个回归测试齐全：只动绑定列、保留并发的限制性变更、拒绝非白名单列（含 `github_id; DROP TABLE users` 与 `userId=0`）

**验证**：
```bash
GOTOOLCHAIN=go1.25.11 go test ./model -run 'UpdateUserBindColumn' -count=1 -v
# 3 个测试全部 PASS
```

**结论**：无需移植。

---

## 三、需要移植的提交详细分析

### 提交 1：93d2df85f - Ali image model mapping

**问题描述**：
阿里云渠道支持图片生成模型（如 `wanx-v1`），但模型名需要映射到内部名称。如果映射后不更新 `relayInfo.UpstreamModelName`，后续逻辑可能仍然使用原始模型名判断协议，导致路由错误。

**官方修复**：
```go
// relay/channel/ali/model_mapper.go (RelayKit 版本)
func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
    // ... 模型映射逻辑
    if mappedModel != originalModel {
        info.UpstreamModelName = mappedModel  // 更新到 relayInfo
    }
    // ...
}
```

**当前代码位置**：
- `relay/channel/ali/adaptor.go` - Ali 渠道适配器
- `relay/channel/ali/constants.go` - 模型常量定义

**移植步骤**：
1. 检查 `relay/channel/ali/adaptor.go` 中的 `GetRequestURL` 或 `ConvertRequest` 方法
2. 确认模型映射后是否更新了 `relayInfo.UpstreamModelName`
3. 如果未更新，添加更新逻辑
4. 编写测试用例验证图片模型路由

**影响范围**：
- 仅影响阿里云渠道（ChannelType = 18）
- 仅影响图片生成模型
- 不影响其他渠道或文本模型

**风险等级**：🟡 **低**
- 定制版可能没有使用阿里图片模型
- 即使有问题，影响面也很小

---

### 提交 2：d7992672a (OAuth 部分) - User state preservation

**问题描述**：
OAuth 绑定（将 OAuth 账号绑定到现有用户）时，可能错误地用 OAuth 提供的信息（如 GitHub username）覆盖用户原有的 `username`、`display_name`、`email` 等字段。

**预期行为**：
- 绑定操作应该**只更新** OAuth 相关字段（`github_id`、`wechat_id` 等）
- **不应覆盖**用户的基本信息字段

**官方修复**：
```go
// controller/oauth.go (伪代码)
func OAuthBind(c *gin.Context) {
    user := getLoggedInUser(c)
    oauthInfo := getOAuthInfo(c)
    
    // ❌ 错误：覆盖所有字段
    // user.Username = oauthInfo.Username
    // user.DisplayName = oauthInfo.DisplayName
    
    // ✅ 正确：只更新 OAuth ID
    user.GitHubId = oauthInfo.GitHubId
    user.Save()
}
```

**当前代码位置**：
- `controller/oauth.go:429-450` - `OAuthBind` 函数

**移植步骤**：
1. 读取 `controller/oauth.go` 中的 `OAuthBind` 函数完整代码
2. 检查绑定时更新了哪些用户字段
3. 如果发现覆盖了 `username`、`display_name`、`email` 等，移除这些更新
4. 确保只更新 OAuth 相关的 ID 字段
5. 编写测试用例验证绑定不覆盖用户信息

**影响范围**：
- 仅影响 OAuth 绑定操作
- 不影响 OAuth 登录或注册

**风险等级**：🟡 **中**
- 如果当前代码有问题，可能导致用户信息被意外修改
- 但绑定操作相对少见，影响用户数量有限

---

## 四、移植计划

### 阶段 1：Ali 图片模型映射修复（0.5 天）

**步骤**：
1. ✅ 读取 `relay/channel/ali/adaptor.go` 完整代码
2. 🔲 检查模型映射逻辑
3. 🔲 确认是否需要更新 `UpstreamModelName`
4. 🔲 如需要，编写修复代码
5. 🔲 编写单元测试
6. 🔲 手工测试阿里图片模型

**验收标准**：
- 阿里图片模型请求正确路由到图片端点
- 不影响其他阿里模型

---

### 阶段 2：OAuth 绑定状态保护（0.5 天）

**步骤**：
1. ✅ 读取 `controller/oauth.go` 的 `OAuthBind` 函数
2. 🔲 审计当前绑定逻辑
3. 🔲 如果发现覆盖问题，编写修复代码
4. 🔲 编写单元测试
5. 🔲 手工测试 OAuth 绑定流程

**验收标准**：
- OAuth 绑定只更新 OAuth ID 字段
- 用户的 `username`、`display_name`、`email` 不被覆盖

---

### 阶段 3：验收与文档（0.5 天）

**步骤**：
1. 🔲 运行完整测试套件：`go test ./...`
2. 🔲 运行 race 检测：`go test -race ./controller ./service`
3. 🔲 构建候选镜像
4. 🔲 启动隔离测试栈
5. 🔲 手工验证两个修复
6. 🔲 更新 Roadmap 文档

**总耗时**：1.5 天

---

## 五、不移植的理由

**为什么 12 个提交无需移植？**

### 原因 1：当前代码已经实现

当前定制版在 M0-M6 期间已经独立实现了以下功能：
- **BillingSession 体系**：比官方更早实现了统一的计费会话管理
- **Tiered billing**：完整的分层重试计费，官方的"修复"是在修复自己的 bug
- **OAuth 安全**：已经有完善的 OAuth 流程和安全检查
- **Topup 原子性**：充值订单使用事务和锁，比官方更早实现

### 原因 2：RelayKit 引入的问题

官方的很多"修复"提交，实际上是在修复 RelayKit 重构引入的问题：
- **Claude tools 修复**：RelayKit 转换时错误删除了工具，后续修复
- **Responses penalty 修复**：RelayKit 转换时丢失了参数，后续修复
- **prompt_cache_key 修复**：RelayKit 转换时丢失了缓存键，后续修复

**我们跳过了 RelayKit，自然也不会有这些问题。**

### 原因 3：代码实现路径不同

虽然功能相同，但实现路径不同：
- **官方**：先有 RelayKit 架构，后填补功能
- **定制**：直接在原架构上实现完整功能

**结果是：我们的代码已经有了官方"修复"后的功能。**

---

## 六、风险评估

### 移植风险

**Ali 图片模型修复**：
- 风险：🟢 **极低**
- 理由：改动范围小（1 个文件，~10 行代码）
- 影响面：仅阿里图片模型

**OAuth 绑定修复**：
- 风险：🟡 **低-中**
- 理由：OAuth 代码较复杂，需要仔细审计
- 影响面：所有 OAuth 绑定操作

### 不移植的风险

**跳过 RelayKit**：
- 风险：🟡 **中**
- 理由：与官方架构持续偏离，未来移植难度增加
- 缓解措施：
  - M9/M10 再次评估 RelayKit
  - 定期同步官方功能性修复

**总体风险**：🟢 **可接受**
- 当前架构稳定且满足需求
- 12 个修复已存在，功能完整性有保障
- 仅需移植 2 个小修复，工作量可控

---

## 七、总结

**审计结论**：
- ✅ **12 个高优先级提交已存在于当前代码**
- 🔧 **2 个已实际移植**：`93d2df85f`（Ali 图片模型映射）、`3dda1d50c`（Claude 无参数工具）
- 🎯 M7 已完成

**核心发现**：
1. **当前代码质量高于预期**：绝大多数官方"修复"我们早就有了，部分实现更严格（例如 `creditTopUpQuota` 额外加了钱包上限守卫）
2. **跳过 RelayKit 是正确的**：RelayKit 之后的多数"修复"是在修 RelayKit 自己引入的回归（Claude 工具被丢、Responses penalty 丢失、prompt_cache_key 丢失），我们没引入 RelayKit 就没有这些回归
3. **但"全部已存在"是错的**：Ali 图片模型映射和 Claude 无参数工具是两个真实存在于我们代码里的缺陷，与 RelayKit 无关，已修复

**具体证据**：
- Ali 图片模型：**原本用 `OriginModelName`，已改为 `UpstreamModelName`**（本单元移植）
- Claude 无参数工具：**原本会被静默丢弃，已新增 `FunctionParametersToInputSchema` 补默认 schema**（本单元移植）
- OAuth 绑定保护：从第一版就使用 `UpdateUserBindColumn` 带白名单验证
- Tiered billing：M1 期间实现，比官方更早更完整
- Quota atomicity：M1 期间实现原子操作
- Topup 原子性：从一开始就用事务和锁，且比官方多一层钱包额度上限校验
- Responses 转换：当前代码保留 `frequency_penalty`/`presence_penalty`/`prompt_cache_key`
- OAuth custom binding：已支持自定义 provider

**改动文件清单**：
- `relay/channel/claude/schema.go`（新增）
- `relay/channel/claude/relay-claude.go`（改工具转换循环）
- `relay/channel/ali/adaptor.go`（3 处编辑，9 个判断点）

**验证结果**（2026-08-19，本机隔离执行，未触碰生产）：
```bash
GOTOOLCHAIN=go1.25.11 go build ./relay/... ./service/...        # 通过
GOTOOLCHAIN=go1.25.11 go vet ./relay/channel/claude/... ./relay/channel/ali/...  # 通过
gofmt -l relay/channel/claude relay/channel/ali                  # 无输出
GOTOOLCHAIN=go1.25.11 go test ./relay/... ./service/... -count=1  # 全部通过
```
注：本机 `/usr/local/go` 是 1.23.5，依赖需要 >= 1.24，必须设 `GOTOOLCHAIN=go1.25.11`（本地 module cache 已有该工具链）。

**下一步**：
1. ✅ 审计完成
2. ✅ 移植完成并验证
3. ✅ 更新 Roadmap
4. ➡️ 进入 M8（前端测试标准化）

**M7 重新定义为"计费与安全修复"的决策是正确的：绝大多数修复早已存在，同时也真的找出并修掉了 2 个缺陷。**
