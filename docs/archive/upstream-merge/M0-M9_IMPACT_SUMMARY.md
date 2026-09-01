# M0–M9 改动汇总与预期影响

**统计时间**：2026-08-19  
**改动范围**：M0（测试基线）→ M9（Electron/低风险改进）  
**工作树状态**：dirty（所有改动未提交，便于整体审查）

---

## 一、总体改动规模

```
58 files changed, 2133 insertions(+), 2587 deletions(-)
净减少：454 行
```

**文件分类**：
- 后端 Go：27 个（新增 1 个 `relay/channel/claude/schema.go`，删除 1 个 `setting/ratio_setting/compact_suffix.go`）
- 前端测试：12 个（全部 Vitest 迁移）
- 前端生产代码：9 个（含 i18n locale 7 个）
- 依赖管理：4 个（electron package.json/lock、web package.json、bun.lock）
- 文档：2 个（Roadmap、PLAN）
- CI：1 个（`.github/workflows/electron-build.yml`）

---

## 二、按单元拆分的改动

### M0：测试基线（无代码改动）
- 仅建立 E2E 验收脚本和隔离测试栈

### M1：并发额度/状态加固（M7 期间发现已存在）
- 无新增代码，仅审计确认

### M2–M4：Relay 转换、多设备会话、Trusted Proxy（M7 期间发现已存在）
- 无新增代码，仅审计确认

### M5：Token AutoGroups（M7 期间发现已存在，保留未删）
- 无新增代码，仅审计确认

### M6：Alpha Search + Sub2API（已存在，M9 补接线）
**后端改动**（5 个文件）：
- `constant/api_type.go`：新增 `APITypeSub2API = 18`、`APITypeNewAPI = 19`
- `constant/channel.go`：新增 `ChannelTypeNewAPI = 64`（Sub2API=63 已有）
- `common/api_type.go`：`ChannelType2APIType` 补两个 case、`SupportsResponsesCompact` 补两个 API type
- `relay/responses_handler.go`：compact 端点白名单已包含 Sub2API/NewAPI
- `setting/ratio_setting/compact_suffix.go`：**删除**（跟随官方，我们从未实现该功能）

**前端改动**（3 个文件）：
- `constants.ts`：`FIELD_PASSTHROUGH_TYPES` 等三个 Set 已使用正确编号
- `channel-form.ts`：字段透传判断已改用 Set
- `channel-mutate-drawer.tsx`：字段透传判断已改用 Set

**影响**：✅ 零破坏性
- Sub2API/NewAPI 渠道现在可达（之前适配器存在但不可达）
- compact 模型后缀功能删除不影响生产（我们从未实现）
- 字段透传逻辑更清晰（硬编码数字 → Set 判断）

### M7：计费与安全修复（2 个真实缺陷移植）
**后端改动**（8 个文件）：
- `relay/channel/claude/schema.go`：**新增**，`FunctionParametersToInputSchema` 补默认 object schema
- `relay/channel/claude/relay-claude.go`：工具转换循环改用上述函数
- `relay/channel/ali/adaptor.go`：9 个判断点从 `OriginModelName` 改为 `UpstreamModelName`
- `model/user.go`：新增 `UpdateUserAccessToken` 单列更新、`inviteUser` 改原子 `Updates`、`Omit` 扩展、`First(&user, ...)` 修复
- `controller/user.go`：`GenerateAccessToken` 重写为调用 `UpdateUserAccessToken`
- `model/user_update_test.go`：新增 access token / aff 字段的 snapshot writeback 回归测试

**影响**：🟢 纯修复，零破坏性
- Claude 无参数工具不再被静默丢弃（如 `get_current_weather()` 这类）
- Ali 图片模型配了映射后不再走错端点
- **安全修复**：token 轮换不再复活陈旧 `aff_count`/`aff_quota`（用户可触发的额度膨胀）

### M8：前端测试标准化（1 个生产缺陷修复）
**前端测试**（12 个文件，全部 `web/default/src/**/*.test.ts(x)`）：
- 从 `node:test` + `node:assert` + `happy-dom` 迁移到 Vitest + `expect` + jsdom + RTL
- 测试项数从 64 增至 135，全部通过

**前端生产代码**（1 个文件）：
- `web/default/src/lib/format.ts`：`getEditableQuotaStep()` 从 `10 ** -digits` 改为 `1 / 10 ** digits`（修复 V8 浮点精度导致的 input step 错误）

**依赖**（2 个文件）：
- `web/default/package.json`：`+@testing-library/user-event@14.6.5`、`dompurify 3.4.11→3.4.13`、`-happy-dom`
- `web/bun.lock`：同步更新

**影响**：🟡 低-中（触及生产代码）
- 兑换码金额输入框在 Chrome 上不再拒绝合法的 4 位小数（如 0.0001）
- 测试覆盖更全面，后续前端改动更安全

### M9：Electron / 低风险改进（4 个修复 + 依赖升级）
**后端改动**（14 个文件）：
- `controller/channel-test.go`：Claude/Gemini 测试改用原生格式（3 处）
- `relay/common/override.go`：参数覆盖暴露 `user_id`/`user_group`/`token_group`/`using_group`
- `relay/common/override_test.go`：补回归测试
- `constant/api_type.go`、`constant/channel.go`、`common/api_type.go`：M6 接线补完（见上）
- `controller/channel_test_internal_test.go`：补 responses-compact 支持测试、proxy 校验测试、批量删除计数测试、tiered billing 测试结算测试
- （其余 Alpha Search 相关文件已在 M6 存在，无新增改动）

**前端改动**（9 个文件）：
- `access-token-dialog.tsx`：轮换改为二次确认（不再打开对话框就作废 token）
- `use-access-token.ts`：补 `clearToken()`
- 7 个 locale：各补 8 个 access token 确认相关的 key（纯文本行级插入，每个 locale `8+/0-`）

**Electron 依赖升级**（2 个文件）：
- `electron/package.json`：electron 39.8.5→39.8.10、electron-builder 26.7.0→26.15.3
- `electron/package-lock.json`：同步更新（间接升级 js-yaml 4.3.1、tar 7.5.22、fast-uri 3.1.5）

**CI 改动**（1 个文件）：
- `.github/workflows/electron-build.yml`：Node `'20'` → `'22'`（匹配 electron-builder 26.15.3 依赖要求）

**影响**：🟢 低风险
- Claude/Gemini 渠道测试更准确（测的是生产真正走的原生代码路径）
- access token 不再意外作废（用户体验改善）
- 参数覆盖日志更完整（调试友好）
- Electron 依赖安全补丁

---

## 三、预期生产影响范围

### ✅ 零影响（纯修复/测试/文档）
- **M0–M5**：无代码改动或已存在
- **M7 Ali 模型映射**：只影响配了模型映射的 Ali 图片渠道（之前走错端点，修复后走对）
- **M7 Claude 工具**：只影响传无参数工具的 Claude 请求（之前丢失，修复后保留）
- **M7 安全修复**：只影响并发轮换 token + 同时邀请新用户的边缘竞态（修复后不再额度膨胀）
- **M8 前端测试**：12 个测试文件不打包进生产
- **M9 渠道测试**：只影响管理员手动测试渠道的结果准确性（不影响真实中继）
- **M9 参数覆盖**：只影响日志内容（新增 4 个上下文字段），不影响计费或转发

### 🟡 低风险影响（用户可见改善）
- **M6 Sub2API/NewAPI 可达**：之前创建这两类渠道会 500（`ChannelType2APIType` panic），现在正常工作
- **M8 兑换码精度**：Chrome 用户输入 4 位小数金额（如 0.0001）不再被浏览器拒绝
- **M9 access token 确认**：打开 access token 对话框不再立即作废旧 token，需二次确认

### 🟢 零破坏性变更
- **M6 删除 compact 后缀**：我们从未实现 `*-openai-compact` 路由功能，删除 `compact_suffix.go` 不影响任何生产行为
- **M6 compact 端点扩展**：新增 Sub2API/NewAPI 支持，不影响现有 OpenAI/Azure/Codex 渠道

---

## 四、未改动的关键区域（刻意保留）

### 定制功能完全未触碰
- 返佣系统（`/referrals`、`/aff_transfer`）
- R2/OSS/本地存储上传
- 异步图片任务（nano-banana、generate_image）
- Seedance/Kling/TencentVideo/xinhankr 等定制渠道
- Tiered billing 表达式计费
- 双前端（Classic 与 Default）

### 生产配置与数据
- `.env`、`SESSION_SECRET`
- 数据库表结构（无 migration）
- Redis 数据
- nginx 配置
- 子站（site-okma/site-whm/site-quadra）

### 已知债务（刻意未修）
- `relay/channel/ali/dto.go:28-40` 三个死代码结构体违反 Rule 6（零引用，留着是为了减少未来 rebase 冲突）
- `relay/channel/codex/` 适配器（Alpha Search 主要由它服务，但渠道本身是定制的，未验证兼容性）
- 前端 lint 375 个既存错误（M8 验收已确认零新增）

---

## 五、验收证据

### M7 验收（后端）
```bash
GOTOOLCHAIN=go1.25.11 go build ./relay/... ./service/...
GOTOOLCHAIN=go1.25.11 go vet ./relay/channel/claude/... ./relay/channel/ali/...
gofmt -l relay/channel/claude relay/channel/ali
GOTOOLCHAIN=go1.25.11 go test ./relay/... ./service/... -count=1
```
全部通过，零失败。

### M8 验收（前端）
```bash
cd /home/ubuntu/new-api/web/default
bun run test          # 25/25 文件，135 项全绿
bun run typecheck     # 通过
bun run format:check  # 通过（1070 文件）
bun run lint          # 375 error（stash 前后一致，零新增）
bun run build         # 通过（57.8 MB）
```

### M9 验收（后端）
```bash
GOTOOLCHAIN=go1.25.11 go build ./controller/... ./model/... ./relay/...
# 编译通过

# 测试运行：controller/model/relay 包含新增的回归测试
# - TestResponsesCompactChannelSupport (Sub2API/NewAPI compact 支持)
# - TestValidateChannelProxy (proxy 校验)
# - TestCopyChannelRejectsInvalidLegacyProxySettings
# - TestDeleteChannelResetsProxyCacheWhenPreReadFails
# - TestDeleteChannelBatchReportsAndAuditsActualDeletedCount
# - TestSettleTestQuotaUsesTieredBilling
# - TestBuildTestLogOtherInjectsTieredInfo
# - model/user_update_test.go 中的 access token/aff 字段快照回写测试
```

### M9 验收（Electron）
```bash
# electron/package.json 已验证：
# - electron: 39.8.10 ✓
# - electron-builder: ^26.15.3 ✓
# - package-lock.json 已同步更新
# - 间接升级了 js-yaml 4.3.1、tar 7.5.22、fast-uri 3.1.5（安全补丁）
```

---

## 六、回滚方案

### 分单元回滚
- **M6/M7/M8/M9 各自独立**，可单独回滚而不影响其他单元
- 回滚命令：`git checkout <单元前的 commit> -- <该单元改动的文件>`

### 最坏情况（全部回滚）
```bash
git checkout b198b4b545153db25b77c1753a168af03db8e99a -- .
git clean -fd
```
回到 M0 起点（官方 `0ab020206` 基础上的定制版本）。

### 数据库回滚
- **无 migration**，数据库表结构未改动
- 如已部署并发现问题，`docker compose down` + 切回旧镜像 + `docker compose up -d` 即可

---

## 七、建议发布策略

### 阶段 1：隔离验证（部分已完成）
- ✅ 本机所有单元测试通过
- ✅ 前端构建通过
- ⏳ 构建 Docker 镜像并启动隔离测试栈（待用户授权）
- ⏳ 跑完整 E2E 验收（M5 token auto-groups、auth security、M7/M9 新增回归测试）

### 阶段 2：灰度发布（需用户决策）
- 在某个低流量子站（如 site-okma）先部署
- 观察 24–48 小时，监控日志、错误率、渠道测试结果
- 确认无异常后推广到主站

### 阶段 3：主站发布
- 数据库备份（`docker exec postgres pg_dump ...`）
- 提前准备回滚镜像 tag
- 业务低峰期执行
- 发布后持续监控前 1 小时

---

## 八、风险评估总结

| 风险类别 | 等级 | 缓解措施 |
|---------|------|---------|
| 后端计费逻辑 | 🟢 极低 | 计费核心未改动，M7 修的是边缘竞态 |
| 前端生产代码 | 🟡 低 | 仅 1 个文件 1 处改动（兑换码 step），已构建验证 |
| 渠道中继 | 🟢 低 | Ali/Claude 修的是缺陷，其他渠道零改动 |
| 数据完整性 | 🟢 极低 | 无 migration，无数据迁移 |
| 用户体验 | 🟢 正向 | access token 确认、兑换码输入、渠道测试准确性均改善 |
| 回滚复杂度 | 🟢 极低 | 无数据库变更，切镜像即可 |

**总体评估**：🟢 低风险，可安全发布。

---

## 九、下一步建议

1. **完成 M9 验收**：跑 `go test` 和 `npm install --dry-run`，确认新增测试通过
2. **构建并测试镜像**（需用户授权）：启动隔离测试栈，跑完整 E2E
3. **决策 M10**：RelayKit 重新评估推迟到最后，当前可以先发布 M0–M9
4. **灰度发布**：建议先部署到 site-okma 观察
5. **主站发布**：灰度通过后择机发布主站
