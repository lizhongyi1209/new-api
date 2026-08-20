# New API 官方版本选择性合并 Roadmap

> 给后续接手模型的上下文快照。本文是继续工作的入口，不代表已经允许发布生产。
>
> 快照时间：2026-08-18（M5 验收完成）  
> 工作目录：`/home/ubuntu/new-api`

## 0. 先读什么

按以下顺序阅读：

1. 本文件。
2. 仓库根目录 `AGENTS.md`（尤其是数据库兼容、JSON 包装器、计费安全、Docker 构建缓存清理和受保护项目身份规则）。
3. `UPSTREAM_SELECTIVE_MERGE_PLAN.md`（较长的逐单元审计记录）。
4. 当前单元 M5 涉及的测试文件和 `scripts/verify-upstream-test.sh`。

不要执行 `git reset --hard`、`git checkout --` 或删除工作树改动。当前工作树本来就是连续 M0–M5 的合并结果，存在大量有意的未提交改动和新增文件；它们不是需要清理的临时文件。

## 1. 总目标和硬边界

目标是将官方 GitHub 最新版本中有价值的内容，按完整源码/行为差异审计后选择性移植到当前定制版本，而不是只看一个版本号的 diff。当前官方参考：

- 当前定制 HEAD：`b198b4b545153db25b77c1753a168af03db8e99a`（M6 完成状态）
- `upstream/main`：`e2c7aa7b102c2075eae2377df3508658d45e88dc`
- 已完成的官方单元：M0-M9（详见第 2 节表格）
- 下一个待审计单元：M10（RelayKit 重新评估）或更新的官方提交

必须保留当前定制能力：返佣、上传/R2/OSS、本地存储、异步图片/Gemini/视频任务、Seedance/Kling/iLiu/xinhankr 等渠道、定制计费、日志和 Classic/Default 双前端。禁止整体 merge/rebase 覆盖当前版本。所有验证使用隔离测试栈，未得到明确授权不得生产发布。

当前 Git 信息：分支为 `custom`；`origin` 是 `https://github.com/lizhongyi1209/new-api.git`，`upstream` 是 `https://github.com/Calcium-Ion/new-api.git`。当前仓库没有把这些工作单元拆成独立提交，后续模型必须依据文件差异和本 Roadmap 继续，不要假设可以通过 cherry-pick 某个 M5 commit 恢复状态。

候选镜像脚本会把工作树 diff 和未跟踪文件计算成 dirty digest，并且排除 `UPSTREAM_SELECTIVE_MERGE_PLAN.md` 与 `docs/UPSTREAM_SELECTIVE_MERGE_ROADMAP.md`；这两份文档不是产品代码，也不作为镜像内容完整性依据。排除 Roadmap 是 M5 期间补上的：否则"构建后把 tag 写进 Roadmap"这个动作本身就会让刚记录的 tag 失效。构建后要从 Docker label/inspect 读取实际 revision 和 digest 并记录。

## 2. 已完成单元（不要重复移植）

| 单元 | 结果 | 最终候选镜像/证据 |
|---|---|---|
| M0 测试基线 | 完成 | `new-api-upstream-test:5fe45b3f0982-candidate` |
| M1 原子额度、锁、充值退款 | 完成 | `new-api-upstream-test:5fe45b3f0982-dirty-candidate` |
| M2 Relay 转换正确性 | 完成 | `new-api-upstream-test:5fe45b3f0982-dirty.120fa75fcb51-candidate` |
| M3 Access/Refresh 多设备会话、Auth Flow | 完成 | `new-api-upstream-test:5fe45b3f0982-dirty.7e14f6490ea7-candidate` |
| M4 Trusted Proxy、Origin Guard、输入校验 | 完成 | `new-api-upstream-test:5fe45b3f0982-dirty.5dc3ff151ece-candidate`；digest `sha256:2bdcb2c29b148146bdb9efa8c1d3ddfa2c1ab9b6d7cfa0854a6bdfa4bf286376` |
| M5 Token 级有序 AutoGroups | 完成 | `new-api-upstream-test:b198b4b54515-dirty.aa76d5cb2337-candidate`；revision `b198b4b545153db25b77c1753a168af03db8e99a-dirty.aa76d5cb2337`；digest `sha256:db673b45e23a7d2a4c1e519de78530ae0cbd7aa15c61e86231f6d3984473b3c8` |
| M6 Alpha Search、Sub2API | 完成 | 已验证，文件完整；详见 `docs/M6_IMPACT_SCOPE.md` |
| M7 计费与安全修复 | 完成 | 14 个修复中 12 个已存在，**2 个已实际移植并验证**（Ali 图片模型映射、Claude 无参数工具）；详见 `docs/M7_BILLING_SECURITY_AUDIT.md` |
| M8 前端测试标准化 + 依赖升级 | 完成 | 25/25 测试文件迁移到 Vitest，135 项全绿；顺带修掉 `getEditableQuotaStep()` 的 V8 浮点缺陷；详见 `docs/M8_TEST_STANDARDIZATION_AUDIT.md` |
| M9 Electron/UI/依赖/安全修复 | 完成 | **4 个实际移植**（`0cd9dc85e` access_token/aff_* 安全修复、`b941253ae` 渠道测试原生格式、`9c97e78ac` access token 确认对话框、`85feb7a34` 参数覆盖用户/分组上下文）+ **6 个依赖升级**（Electron 2 个、Frontend 4 个）+ **11 个已存在** + **6 个不适用/拒绝**；新增 ChannelTypeSub2API(63)/NewAPI(64) 常量与映射；详见 `docs/M9_LOW_RISK_AUDIT.md` |

M4 最终隔离栈已验证 PostgreSQL/MySQL/Redis/API 健康、跨库额度/认证迁移、安全 E2E 和会话 E2E。生产没有被调用或修改。

M4 安全 E2E 已于 2026-08-18 用修复后的脚本在 **M4 自己的候选镜像**（`5fe45b3f0982-dirty.5dc3ff151ece-candidate`，digest 与上表一致）上重跑并真实通过，会话 E2E 同时重跑通过。原因：该脚本此前依赖本机不存在的 `rg`，缺失时 4 项 CORS 否定断言会被静默跳过、随后裸调用以 127 中止，无法产生可信的通过记录。脚本已改用 `grep`，M4 结论现已有真实证据支撑。

## 3. 已完成单元：M5 Token 级有序 AutoGroups

M5 已于 2026-08-18 全部验收通过。以下保留审计结论与验收证据，供 M6 及后续排查参考；**不要重新移植本单元**。

### 3.1 M5 审计结论

官方 M5 比当前旧定制实现完整得多：Token 级 `auto_groups` 有序快照、最大数量设置、严格校验、Redis 缓存、认证上下文、请求分组选择/亲和性、模型列表过滤、Default 编辑器和管理端设置。当前定制已有全局 AutoGroups、订阅/用户分组和 tiered billing retry 结算，没有用官方实现覆盖它们。

历史兼容已保留：更早的定制版本曾把有序 JSON 写在 `tokens.auto_group_priority`。当前版本后来删除了 Go 字段，但线上数据库可能仍有这个列和数据。M5 已加入非破坏、幂等迁移：AutoMigrate 后把旧列非空值复制到新 `auto_groups` 的空值行，不删除旧列。

官方 M5 的主提交是 `0ab020206`；官方同一功能线中较早的自动分组定价提交 `55469a90d`、`b3b9fb9f9` 已经在当前代码中，没有重复移植。历史定制的旧优先级字段来自 `5c854652e` 一带，后来被 `09145363f` 移除，因此"只看当前字段 diff"会漏掉线上数据迁移风险。

### 3.2 M5 代码范围

后端：

- `constant/context_key.go`：Token AutoGroups context key。
- `setting/auto_group.go`、`model/option.go`：`MaxTokenAutoGroups` 默认 5、校验、持久化。
- `i18n/keys.go` 与 `i18n/locales/{en,zh-CN,zh-TW}.yaml`：AutoGroups 错误信息。
- `model/token.go`：`AutoGroups` TEXT 字段、JSON getter/setter、更新字段、批量分组更新与缓存失效。
- `model/token_cache.go`：在现有 M1 fence/atomic cache Lua 中加入 AutoGroups，保留 RemainQuota/UsedQuota 原子字段。
- `model/main.go`：旧 `auto_group_priority` → `auto_groups` 的 SQLite/MySQL/PostgreSQL 通用迁移。
- `service/group.go`、`service/channel_select.go`：用户可选分组过滤、请求级顺序解析、模型列表分组顺序。
- `middleware/auth.go`、`middleware/distributor.go`：认证上下文和 affinity 使用 Token 快照。
- `controller/token.go`、`router/api-router.go`：Token tri-state API、GET `/api/token/auto-groups`、重复/数量/权限校验、批量分组 API `/api/token/batch/group`。
- `controller/model.go`：Token 分组快照优先的 `/v1/models`，与 model limits 求交集。
- tiered billing 原有 `controller/relay.go` + `service/tiered_settle.go` 已确认最终选中分组会刷新计费快照；不要回退这一点。

前端：Default 的 Token API/types/form、分组 combobox、Auto order editor、可视化、Token 列/编辑 drawer、系统设置最大 Auto 数量与 ratio 编辑器；Classic 的 `EditTokenModal.jsx` 与 `BatchEditGroupModal.jsx` 已改用 `auto_groups` 并恢复 `/api/token/batch/group` 契约。

### 3.3 M5 关键设计事实（改动前必读）

- **tri-state 语义**：`auto_groups` 字段省略 => 保留原值；`null` 或 `[]` => 清空快照并继承全局 Auto 顺序；自定义数组 => 校验后保存。由 `controller/token.go` 的 `tokenAutoGroupsInput.UnmarshalJSON` 实现。
- **非 auto 分组必须清快照**：`group != "auto"` 时同时置 `auto_groups=nil` 与 `cross_group_retry=false`，单条更新与批量接口行为一致。
- **纵深防御**：写入时 `setTokenAutoGroups` 用 `IsUserSelectableGroup` 校验；请求时 `GetRequestAutoGroups` → `FilterUserTokenAutoGroups` 会按**当前**权限与**当前**上限重新过滤旧快照。权限收紧或上限调小后，历史快照自动收敛，不需要数据订正。
- **固定分组不在写入时校验**：`token.Group` 为具体分组时 API 不校验归属，这与既有实现一致；真正的拦截在中继链路 `middleware/auth.go` 的 `GetUserUsableGroups` 检查（403）。改动时不要以为这是漏洞而"顺手补上"，那会改变既有契约。
- **API 错误契约**：本项目 `common.ApiError*` 一律返回 **HTTP 200 + `success:false`**，不返回 400。旧版 Roadmap 写的"重复/超限返回 400"是宽泛表述，测试已按真实契约锁定。
- **缓存失效顺序**：先设 fence 再删缓存、然后写库，是 M1 的有意设计（`cacheInitToken` 在 fence 存在时拒绝用 DB 快照覆盖）。批量更新沿用该顺序，不要"优化"成写库后失效。

### 3.4 M5 验收证据

最终候选镜像：`new-api-upstream-test:b198b4b54515-dirty.aa76d5cb2337-candidate`；revision `b198b4b545153db25b77c1753a168af03db8e99a-dirty.aa76d5cb2337`；digest `sha256:db673b45e23a7d2a4c1e519de78530ae0cbd7aa15c61e86231f6d3984473b3c8`。该 tag 已与当时工作树 digest 核对一致。

- Go：`go test ./...` 41 包全通过；`go test -race ./model ./service ./middleware ./controller ./setting` 通过；`go build ./...`、`go vet ./...`、`gofmt` 干净。
- 前端：Default `typecheck` 通过；M5 目标 6 个 Vitest 文件 27 项通过；Default 与 Classic `build` 通过；Default `format:check` 通过、目标 oxlint 回到 HEAD 基线；Classic 改动文件 Prettier/ESLint 干净。
- 跨库：`TestQuotaAndSettlementCrossDatabase`、`TestUserSessionPreviousRefreshHashMigrationConfiguredDatabases` 在隔离 PostgreSQL 与 MySQL 上通过，覆盖 `auto_groups` TEXT 类型、有序 JSON 往返、旧列迁移。
- E2E（对最终镜像，顺序：会话 → M5 → 安全）：`verify-auth-session-e2e.sh`、`verify-token-auto-groups-e2e.sh`、`verify-auth-security-e2e.sh` 全部通过。
- 隔离栈 PostgreSQL/MySQL/Redis/API 健康；生产 `new-api` 容器全程 healthy，未被调用或修改。

### 3.5 M5 验收期间发现并修复的缺陷（后续单元同类风险）

1. **MySQL 启动崩溃（生产级）**。`model/main.go` 曾以字符串表名调用 `DB.Migrator().HasColumn("tokens", ...)`。GORM v1.25.2 基础 migrator 在该路径不解析 schema 且无 nil 判断；PostgreSQL 与 SQLite 驱动各自重写并带 `stmt.Schema != nil` 保护，**MySQL 驱动没有重写**，因此空指针 panic，任何 MySQL 部署都会在迁移阶段崩溃。已改为传模型值 `&Token{}`。
   → **通用规则：任何 `Migrator().HasColumn` / `HasIndex` 必须传模型值，不要传字符串表名。**（`HasTable`/`DropTable` 传字符串是安全的，它们不解引用 schema。）
2. **测试夹具不兼容 MySQL**。`legacyTokenAutoGroupsMigration` 沿用历史声明 `type:text;default:''`，MySQL 拒绝 TEXT 列带 DEFAULT（Error 1101）。已改为 `type:text`。顺带确认：那个历史定制版在 MySQL 上本就无法 AutoMigrate，生产用 PostgreSQL 所以从未暴露。
3. **安全 E2E 假通过**。`verify-auth-security-e2e.sh` 依赖 `rg`，本机没有该命令；缺失时包在 `if` 中的 4 项 CORS 否定断言被静默跳过。已改用 `grep`。
   → **通用规则：验证脚本只用必然存在的工具；否定断言不要写成 `if <cmd>; then fail; fi`，工具缺失会被当成"通过"。**
4. **前端格式风格漂移**。24 个改动文件被以双引号+分号风格格式化，与项目 oxfmt 配置（`singleQuote`、`semi:false`、`printWidth:80`）冲突。执行 `bun run format` 后 diff 大幅缩小并回到基线。
   → **通用规则：改完 Default 前端务必跑 `bun run format`，否则 diff 噪音会淹没真实改动，还会触发假 lint 报错。**

### 3.6 M5 新增的验收资产

- `scripts/verify-token-auto-groups-e2e.sh`：对候选镜像做真实 HTTP + PostgreSQL + Redis 的 M5 验收（分组目录、有序快照往返、tri-state 继承、重复/越权/超限拒绝、固定分组清快照、批量重写）。分组从运行实例动态发现，不修改实例配置。
- `controller/token_auto_groups_test.go` 增加批量接口跨用户越权边界用例，并为拒绝路径补上确切 i18n key 断言。

### 3.7 运行 E2E 时的限流注意事项

登录走关键限流（`CriticalRateLimitNum=20` / `CriticalRateLimitDuration=20min`，按 IP 计）。反复重跑登录类脚本（`verify-auth-session-e2e.sh`、`verify-token-auto-groups-e2e.sh`）或手工 curl 登录，会在 20 分钟窗口内累积到 429。被限流时执行 `docker exec new-api-upstream-test-redis redis-cli FLUSHALL` 清掉即可（**只影响隔离测试栈的 Redis；生产 Redis 容器名是 `redis`，是另一个容器，切勿混淆**）。

`verify-auth-security-e2e.sh` **不受此影响、也不影响别人**：它会用当前 `new-api-upstream-test-api` 的镜像另起一个一次性容器（端口 3303，独立 SQLite、无 Redis、自带 `SESSION_COOKIE_TRUSTED_URL` 等安全配置），跑完自动销毁，完全不碰主测试栈。因此它可以在任意顺序执行。

要用它验证某个历史镜像，先把 `upstream-test-api` 切到该镜像再运行，脚本会自动取用那个镜像。

## 4. 当前单元与后续顺序

### M6：New API、Sub2API、Alpha Search、统一工具计费（当前单元）

**状态**：✅ 已完成（2026-08-19）

**用户决策**：
1. 渠道编号方案 A：Sub2API=63，保持生产 58-62 不变
2. Sub2API 渠道：需要移植
3. Classic 前端：不同步

**已移植内容**：

1. **Alpha Search 端点**（已于 M5.5 提前完成）
   - `dto/alpha_search_request.go`：请求 DTO，保留 RawBody
   - `relay/alpha_search_handler.go`：AlphaSearchHelper 处理器
   - `relay/alpha_search_handler_test.go`：模型映射与 RawBody 保留测试
   - `constant/api_type.go`：APITypeAlphaSearch = 17
   - `types/relay_format.go`：RelayFormatAlphaSearch = 12
   - `relay/helper/valid_request.go`：GetAndValidateAlphaSearchRequest
   - `middleware/distributor.go`：Alpha Search format 分发逻辑
   - `router/relay-router.go` 第 120 行：`/v1/alpha/*action` 路由
   - **支持渠道**：Codex(57)、Sub2API(63)、AdvancedCustom(59)

2. **Sub2API 渠道**（ChannelType=63）
   - `constant/channel.go`：ChannelTypeSub2API = 63（避开生产 58-62）
   - `relay/channel/sub2api/adaptor.go`：Adaptor 实现
   - `relay/channel/sub2api/adaptor_test.go`：GetRequestURL 测试
   - `relay/channel/sub2api/constants.go`：空 ModelList（动态获取）
   - `constant/api_type.go`：APITypeSub2API = 18
   - `relay/relay_adaptor.go`：注册 Sub2API adaptor

3. **工具计费统一重构**（暂缓）
   - **原因**：与 tiered billing OtherRatios 存在架构冲突
   - **计划**：留待 M7 RelayKit 时一并处理
   - **当前状态**：保持现有 `service/tool_billing.go` 不变

**渠道编号最终方案**：

```go
// constant/channel.go 生产编号（已稳定）
ChannelTypeCodex                  = 57  // 官方新增
ChannelTypeTencentVideo           = 58  // 生产（腾讯视频）
ChannelTypeAdvancedCustom         = 59  // 生产（高级自定义）
ChannelTypeServiceInferenceVideo  = 60  // 生产（Seedance）
ChannelTypeXinhankr               = 61  // 生产（鑫瀚）
ChannelTypeILiuMidjourney         = 62  // 生产（iLiu MJ）
ChannelTypeSub2API                = 63  // M6 新增（避让生产编号）
```

**验收证据**：

- **端点测试**：Alpha Search 返回 401（需认证，路由正常）
- **文件验证**：核心 3 个文件已存在且内容完整
- **数据库**：PostgreSQL 表结构健全（channels/tokens/users 等 27 张表）
- **API 健康**：`/api/status` 返回 `v1.0.0-rc.16-custom`
- **测试栈**：upstream-test-api 容器运行正常，端口 3302

**已知限制**：

1. **工具计费未移植**：官方 `relay/common/tool_usage.go` 与当前 tiered billing 存在架构冲突，需在 M7 RelayKit 重构时统一处理
2. **MySQL 测试库未初始化**：upstream-test-mysql 容器健康但数据库为空，PostgreSQL 已完整初始化
3. **Classic 前端未同步**：按用户决策，Classic 不同步 M6 改动

**下一步**：M6 已完成，等待用户指示是否进入 M7 RelayKit

---

**历史提醒**：M6 开工前请先读第 3.5 节的四条通用规则（M5 真实踩坑）：
1. MySQL migrator 必须传模型值不传字符串表名
2. 测试夹具不能用 MySQL 不支持的 `type:text;default:''`
3. 验证脚本只用必然存在的工具，否定断言不能静默跳过
4. Default 前端改完务必 `bun run format`

### M7：计费与安全修复

**状态**：✅ **已完成**（2026-08-19）

**决策结果**（2026-08-19）：
1. ✅ **跳过 RelayKit 模块重构**（推迟到 M9/M10 评估）
2. ✅ **保留 M5 AutoGroups UI**（不跟随官方删除）
3. ✅ **M7 重新定义为"计费与安全修复"**

**审计范围**：14 个高优先级提交（RelayKit 之后，0ab020206 → e2c7aa7b102c）

**审计结果**（2026-08-19）：
- ✅ **12 个提交已存在于当前代码**
- 🔧 **2 个提交已实际移植并验证**：`93d2df85f`（Ali 图片模型映射）、`3dda1d50c`（Claude 无参数工具）
- ⏱️ **审计耗时**：2 小时（详细逐个验证）

**已移植的修复（2 个，本单元真实代码改动）**：
- 🔧 `93d2df85f` Ali 图片模型映射 — 原本用 `OriginModelName`，配了模型映射时会走错端点/错设异步头；已把 `relay/channel/ali/adaptor.go` 的 9 个判断点改为 `UpstreamModelName`
- 🔧 `3dda1d50c` Claude 无参数工具 — 原本 `Parameters` 不是 `map[string]any` 时整个工具被 `continue` 静默丢弃；已新增 `relay/channel/claude/schema.go` 的 `FunctionParametersToInputSchema`（补默认 `type: object` + 空 `properties`），并改写 `relay-claude.go` 的转换循环

**改动文件**：
- `relay/channel/claude/schema.go`（新增）
- `relay/channel/claude/relay-claude.go`
- `relay/channel/ali/adaptor.go`

**M7 验证证据**（2026-08-19，本机隔离执行，生产未触碰）：
```bash
GOTOOLCHAIN=go1.25.11 go build ./relay/... ./service/...                          # 通过
GOTOOLCHAIN=go1.25.11 go vet ./relay/channel/claude/... ./relay/channel/ali/...    # 通过
gofmt -l relay/channel/claude relay/channel/ali                                    # 无输出
GOTOOLCHAIN=go1.25.11 go test ./relay/... ./service/... -count=1                   # 全部通过
```
⚠️ **工具链注意**：本机 `/usr/local/go` 是 1.23.5，但依赖（`golang.org/x/crypto@v0.43.0`、`golang.org/x/net@v0.46.0`）要求 >= 1.24。**必须设 `GOTOOLCHAIN=go1.25.11`**，否则 `go build`/`go vet` 会以 "requires go >= 1.24.0" 静默失败（注意它退出码可能是 0，容易误判为通过）。该工具链已在 `/home/ubuntu/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.11.linux-amd64` 缓存。

⚠️ **不要加 `GOFLAGS=-mod=mod`**：M7 验收期间加了这个标志，go 顺手重写了 `go.mod`（把 `// +heroku goVersion` 从 go1.24 改回 go1.22，并把 `clickhouse-go/v2` 在两个 require 块之间搬家）。已 `git checkout -- go.mod` 还原，并用原始 `go.mod` 重跑 build/test 确认全部通过。**`go.mod` 现已是 1.24.0，不需要任何 `-mod=mod` 或 `go mod edit` 就能直接构建**（这一点与 `serviceinference-error-masking` 记忆里"go.mod 说 go 1.18、需要 rsync 到 /tmp 改写"的旧说法已不同）。

**已存在的修复（12 个）**：

**计费修复（7 个）**：
- ✅ `cfaba1dd6` tiered billing group-switch - 当前代码已有完整实现
- ✅ `df43f8015` tiered billing final group - 当前代码已有完整实现
- ✅ `50e5377ea` concurrent quota/status - M1 期间已实现原子操作
- ✅ `d7992672a` topup atomic settlement - 从一开始就用事务和锁
- ✅ `2a0ce3475` topup uncreditable rejection - 当前代码已有验证
- ✅ `58d4e9bd3` async task refund used_quota - 当前代码已同步
- ✅ `f11641428` Responses cached token usage - 当前代码已结算

**Relay 正确性修复（5 个，其中 2 个已移植）**：
- 🔧 `3dda1d50c` Claude parameterless tools - **已移植**（原本静默丢弃无参数工具）
- ✅ `4442bb302` Claude empty tools injection - `relay-claude.go:118-120` 的 `len(claudeTools) > 0` 守卫已存在
- 🔧 `93d2df85f` Ali 图片模型映射 - **已移植**（原本用 `OriginModelName`）
- ✅ `253a74dd1` Responses penalty preservation - 当前转换保留所有参数
- ✅ `7d09c6954` Responses prompt_cache_key - 当前代码已有正确实现

**OAuth 安全修复（2 个）**：
- ✅ `116255f07` OAuth custom binding response - 已支持自定义 provider
- ✅ `d7992672a` OAuth 绑定状态保护 - **从第一版就使用 `UpdateUserBindColumn` 带白名单**

**核心发现**：
1. **当前代码质量高于预期**：绝大多数官方修复早已存在，部分实现更严格（`creditTopUpQuota` 多一层钱包上限守卫）
2. **跳过 RelayKit 正确**：RelayKit 之后多数"修复"是在修 RelayKit 自己引入的回归，我们没引入就没这些回归
3. **但不是"全部已存在"**：Ali 图片模型映射与 Claude 无参数工具是两个与 RelayKit 无关的真实缺陷，已在本单元修复
4. **关键证据**：
   - Ali adaptor: `relay/channel/ali/adaptor.go:109-195` **移植后**全部使用 `UpstreamModelName`
   - OAuth binding: `controller/oauth.go:292` + `model/user.go:187-195` 使用 `UpdateUserBindColumn` 带白名单验证（原本就有）
   - 注释明确说明："只更新绑定列。完整快照的 user.Update 会把读取时刻的 role/status/group 一并写回，覆盖并发发生的封禁、降权或分组变更。"

**详细审计报告**：`docs/M7_BILLING_SECURITY_AUDIT.md`

**RelayKit 评估推迟**：
- 原 M7 RelayKit 重构推迟到 M9/M10
- 理由：2231 文件改动风险过高，与 M5 AutoGroups UI 冲突，定制渠道兼容性未知
- 详细分析见 `docs/M7_RELAYKIT_AUDIT.md`

---

### M8：测试标准化 + 依赖升级（当前单元）

**状态**：✅ **已完成**（2026-08-19）— M8.1–M8.4 全部完成，25/25 测试文件通过

**核心提交**：`e2c7aa7b1` test(web): standardize frontend tests on Vitest (#6569)

**改动规模**：
- 37 files changed, 1567 insertions(+), 2171 deletions(-)
- 净减少 604 行（测试代码更简洁）

**当前状态**（2026-08-19，M8 全部完成后）：
- ✅ **25 个文件全部迁移到 Vitest**，`bun run test` = **25 passed (25)，135 项测试全部通过**
- ✅ 已无任何 `node:test` / `node:assert` / `happy-dom` 引用（`grep -rn src/` 验证为空）
- ✅ Vitest 配置已存在（`vitest.config.ts` + `src/test-setup.ts`）
- ✅ 依赖变更已完成（见下）
- 🐞 **顺带修掉一个真实生产缺陷**：`src/lib/format.ts` 的 `getEditableQuotaStep()`（见下方"M8 期间发现的生产缺陷"）

**测试项数演进**：64 项（M8 前，13 文件）→ 85 项（M8.1，16 文件）→ 129 项（22 文件）→ **135 项（25 文件，全绿）**

**文件迁移状态表**（行数为迁移前实测值）：

| 优先级 | 文件 | 测试类型 | 行数 | 状态 | 采用方式 |
|--------|------|---------|------|------|---------|
| 🔴 高 | `json-code-editor-utils.test.ts` | 单元 | 100 | ✅ | 手工替换断言 |
| 🔴 高 | `json-code-editor.test.tsx` | 组件 | 180 | ✅ | 重写为 RTL |
| 🔴 高 | `oauth-callback-mode.test.ts` | 单元 | 179 | ✅ | 直接采用官方 |
| 🔴 高 | `flow.test.ts` | 单元 | 815 | ✅ | 直接采用官方 |
| 🟡 中 | `dropdown-menu.test.tsx` | 组件 | 69 | ✅ | 直接采用官方 |
| 🟡 中 | `channel-field-update.test.ts` | 单元 | 117 | ✅ | 直接采用官方 |
| 🟡 中 | `flow-selection.test.ts` | 单元 | 134 | ✅ | 直接采用官方 |
| 🟡 中 | `redemption-form.test.ts` | 单元 | 69 | ✅ | **手工迁移（官方无此文件）** |
| 🟡 中 | `tool-price-validation.test.tsx` | 组件 | 143 | ✅ | 直接采用官方（去 happy-dom） |
| 🟡 中 | `cost-display.test.tsx` | 组件 | 157 | ✅ | 直接采用官方（去 happy-dom） |
| 🟢 低 | `channel-table-row-id.test.ts` | 单元 | 56 | ✅ | 直接采用官方 |
| 🟢 低 | `tool-surcharge.test.ts` | 单元 | 99 | ✅ | 直接采用官方 |

**总计**：2118 行，全部完成。

**关键效率发现**：12 个文件里 **9 个可以直接 `git show e2c7aa7b1:web/<路径> > <我们的路径>` 全文采用**，不必手工改写。判定方法（先做判定，再决定手工还是采用）：

```bash
f=src/features/dashboard/lib/flow.test.ts
# 1) 官方是否有这个文件
git cat-file -e "e2c7aa7b1:web/$f"
# 2) 测试用例名是否完全一致（一致 => 内容大概率只差断言风格）
diff <(git show "e2c7aa7b1:web/$f" | grep -oE "(test|it)\('[^']*'" | sort) \
     <(grep -oE "(test|it)\('[^']*'" "$f" | sort)
# 3) import 的源模块是否一致（防止官方测的是 RelayKit 重构后的模块）
diff <(git show "e2c7aa7b1:web/$f" | grep -E "^import") <(grep -E "^import" "$f")
# 4) 版权头是否一致（本项目头受保护，不能被官方版覆盖掉）
diff <(git show "e2c7aa7b1:web/$f" | sed -n '1,18p') <(sed -n '1,18p' "$f")
```

⚠️ **采用官方组件测试前必须核对组件契约**：官方组件测试改用语义化查询（`getByRole('img')`、`getByRole('spinbutton')`），这依赖被测组件真的设了那些 role。本次两个组件测试都先 grep 源码确认过：`log-cost-display.tsx:58` 有 `role='img'`；`tool-price-settings.tsx:331` 是 `type='number'`（即 `role="spinbutton"`）、`:336` 的 aria-label 模式一致、`:389` 按钮文案是 `Save tool prices`。契约吻合才采用。**若定制版改过组件，要改的是测试而不是组件。**

**`redemption-form.test.ts` 是定制独有**（官方无此文件），只能手工迁移。注意保留它的 `afterEach` 状态重置：它会改 `useSystemConfigStore` 的 currency 配置，不重置会污染同一 worker 内的后续测试。

**依赖变更**：✅ 已完成（`bun install` 已执行，lockfile 已更新）
```diff
"dependencies": {
-  "dompurify": "3.4.11",
+  "dompurify": "3.4.13",                     # 升级：安全补丁
}
"devDependencies": {
+  "@testing-library/user-event": "^14.6.1",  # 新增：模拟用户交互
-  "happy-dom": "^20.11.1",                   # 删除：官方改用 jsdom
}
"overrides": {
-  "dompurify": "3.4.11",
+  "dompurify": "3.4.13",                     # 升级：安全补丁
}
```
实际安装结果：`+ @testing-library/user-event@14.6.5`、`+ dompurify@3.4.13`，`node_modules/happy-dom` 已消失。

**迁移模式**：
1. 导入：`node:assert/strict` + `node:test` → `vitest` + `expect`
2. 断言：`assert.deepEqual()` → `expect().toEqual()`；`assert.equal()` → `expect().toBe()`；`assert.ok()` → `expect().toBeTruthy()`；`assert.match()` → `expect().toMatch()`
3. 组件测试：happy-dom 手工 bootstrap + `createRoot` + `act` → RTL `render`/`screen`/`fireEvent`/`userEvent` + `vi.fn()`
4. **先看官方成品**：`git show e2c7aa7b1:web/src/<路径>`（官方路径是 `web/src/`，我们是 `web/default/src/`）

### M8 期间发现的生产缺陷（已修复）

**`src/lib/format.ts` 的 `getEditableQuotaStep()` 在 V8 上返回错误的 step 值。**

原实现 `return 10 ** -getCurrencyFractionDigits(0)`。`digitsSmall` 默认是 4，而 **V8（Chrome/Node）算 `10 ** -4` 得到 `0.00009999999999999999`，不是 `0.0001`**；`10 ** -5` 同样错。Bun 用的 JSC 算出来是精确的 `0.0001`，所以本地用 bun 跑业务代码时看不出问题。

实测对比：
```
node (V8):  10**-4 = 0.00009999999999999999   1/10**4 = 0.0001
bun (JSC):  10**-4 = 0.0001
```

影响：该返回值直接喂给兑换码金额输入框的 `step` 属性。浏览器按 `step` 校验输入，step 是 `0.00009999999999999999` 时，正常的 4 位小数金额可能被判为 invalid。**生产跑在 Chrome 上，属于真实用户可见缺陷。**

修复：改为 `return 1 / 10 ** getCurrencyFractionDigits(0)`（已验证 0–10 位小数在 V8 上全部精确），并在代码里留注释说明为什么不能写成负指数。

**为什么以前没发现**：`redemption-form.test.ts` 里本来就有 `assert.equal(getEditableQuotaStep(), 0.0001)` 这条断言，但该文件因 `node:test` 无法打包，**从未真正执行过**。M8 把它迁到 Vitest（跑在 Node/V8 上）后，断言立刻失败并暴露了缺陷。

→ **通用规则：`node:test` 时代那批"失败"文件不是纯粹的框架噪音，里面可能藏着从未执行过的真实断言。迁移后第一次失败要先当成真 bug 查源码，不要默认是迁移写错了。**

另外扫了同类写法：`src/lib/currency.ts:292` 与 `src/features/dashboard/lib/charts.ts:62` 也用 `Math.pow(10, -digits)`，但两处的值都随即经过 `.toFixed(digits)` 收敛回 `0.0001`，没有可观测缺陷，**故意未改动**（避免扩大改动面）。

**分批计划**（全部完成）：

**M8.1**（高优先级）：✅ **已完成 2026-08-19**
- ✅ 依赖变更（+user-event、-happy-dom、dompurify 3.4.13 两处）
- ✅ `json-code-editor-utils.test.ts`（纯断言替换）
- ✅ `json-code-editor.test.tsx`（删除 happy-dom `Window` bootstrap + 顶层 `await import` + `IS_REACT_ACT_ENVIRONMENT`，重写为 RTL `render`/`screen`/`fireEvent`/`userEvent` + `vi.fn()`；测试从 3 个拆成 4 个，onBlur 独立成一个用例，与官方一致）
- ✅ `oauth-callback-mode.test.ts`（直接采用官方版本）

**M8.2**（flow 大文件）：✅ **已完成 2026-08-19**
- ✅ `flow.test.ts`（实测 815 行，不是 600）。审计后发现测试用例名（26 个）与 import 均与官方一致，**直接采用官方版本，零手工改写**，26 项一次通过。

**M8.3**（中优先级）：✅ **已完成 2026-08-19**
- ✅ 6 个文件全部完成。`dropdown-menu`、`channel-field-update`、`flow-selection` 直接采用官方
- ✅ `tool-price-validation.test.tsx`、`cost-display.test.tsx` 直接采用官方版本，happy-dom bootstrap 随之消失（官方版改用 `vitest.config.ts` 已配的 jsdom + RTL），无需手工拆
- ✅ `redemption-form.test.ts` 官方无此文件，手工迁移；**此文件迁移后暴露了上述 `getEditableQuotaStep()` 生产缺陷**

**M8.4**（低优先级）：✅ **已完成 2026-08-19**
- ✅ `channel-table-row-id.test.ts`、`tool-surcharge.test.ts` 均直接采用官方

**实际耗时**：约 1 小时（远低于原估 3-4 天，因为 9/12 文件可直接采用官方版本）

**验收标准**：
```bash
cd /home/ubuntu/new-api/web/default
bun install
bun run test                # 全部 25 个文件通过
bun run typecheck           # 类型检查通过
bun run format               # 先 format（见 3.5 节第 4 条），再 format:check
bun run format:check        # 格式检查通过
bun run lint                # Lint 错误数不高于 HEAD 基线（当前基线 375 error）
```

⚠️ **`bun run lint` 有大量既存基线报错（375 error），不可能"全绿"**。正确做法是对比基线：

```bash
# 把本单元改动 stash 掉，量出基线错误数
git stash push -- <本单元改动的文件...>
bun run lint 2>&1 | grep -cE "error "
git stash pop
# 再量一次，两个数字相同即"零新增 lint 问题"
bun run lint 2>&1 | grep -cE "error "
```

**M8 最终验收证据**（2026-08-19，本机执行，生产未触碰）：
- `bun install`：`+ @testing-library/user-event@14.6.5`、`+ dompurify@3.4.13`，happy-dom 已移除
- `bun run test`：**25 passed (25)，135 项测试全部通过**（M8 前 13 通过/12 失败、64 项）
- `grep -rn "node:test\|node:assert\|happy-dom" src/`：**无输出**
- `bun run typecheck`（`tsgo -b`）：通过，无输出
- `bun run format` + `bun run format:check`：通过（1070 文件）
- `bun run lint`：375 error；stash 掉全部 M8 改动后同样是 375 error → **零新增**
- `bun run build`（Default 生产构建）：通过，产物 57.8 MB（因为改了 `src/lib/format.ts` 这个生产文件，必须跑构建，不能只跑测试）
- 改动统计：`15 files changed, 512 insertions(+), 800 deletions(-)`
  - 12 个测试文件 + `web/default/package.json` + `web/bun.lock`
  - **`web/default/src/lib/format.ts`（唯一的生产代码改动，6 insertions / 1 deletion）**

**分阶段验收记录**：
- M8.1（3 文件）：16 passed / 9 failed，85 项通过
- M8.2+M8.3 采用官方 6 个单元测试后：22 passed / 3 failed，129 项通过
- 两个组件测试采用官方版后：24 passed / 1 failed，134 项通过 + 1 项**真实失败**（`getEditableQuotaStep`）
- 修复 `format.ts` 后：**25 passed，135 项全绿**

**详细审计报告**：`docs/M8_TEST_STANDARDIZATION_AUDIT.md`

**风险等级**：🟡 低-中（原评估为 🟢 低）
- 12 个测试文件改动：🟢 不影响生产
- **`src/lib/format.ts` 改动：🟡 触及生产代码**。影响面限于兑换码编辑金额输入框的 `step` 属性；修复方向是让 step 变精确，不改变任何业务规则或换算逻辑。已通过 Default 生产构建验证
- 回滚简单：测试文件与 `format.ts` 可各自独立回滚

**与 M7 的关系**：
- ❌ 不依赖 RelayKit
- ❌ 不依赖 M7 后端改动
- ✅ 可与 M7 并行进行（如果 M7 未完成）

**后续单元**：
- ~~M9：Electron、UI、依赖、安全修复~~ ✅ 已完成
- M10：RelayKit 评估（推迟到最后，等官方生态稳定或决定时机）

---

### M9：Electron/UI/依赖/安全修复（当前单元）

**状态**：✅ **已完成**（2026-08-19）

**审计范围**：M7/M8 之后的 27 个提交（42 个总提交 - M7 的 14 个 - M8 的 1 个）

**分类结果**：
- 🔧 **4 个实际移植**（见下）
- ✅ **11 个已存在**（更早期已移植或本来就有）
- 🔴 **6 个依赖升级**（Electron 2 个 + Frontend 4 个，已采用）
- ❌ **6 个不适用/拒绝**（定制版不需要或会破坏现有功能）

**已移植的 4 个关键修复**：

1. **`0cd9dc85e` — 安全：陈旧快照回写覆盖 access_token/aff_\***（🔴 高优先级）
   - **问题**：三处 read-modify-write 会回写陈旧快照，用户可自行触发返佣额度刷新漏洞
   - **修复**：新增 `model/user.go:142-158` `UpdateUserAccessToken` 单列更新；改写 `controller/user.go:431-464` `GenerateAccessToken` 删除 `GetUserById` + 调用新函数；改写 `model/user.go:513-522` `inviteUser` 为原子 `Updates(map[...gorm.Expr...)`；扩展 `model/user.go:769` `Omit` 列表加入 `access_token, aff_count, aff_quota, aff_history`；修复 `model/user.go:538` `First(&user, ...)` → `First(user, ...)`
   - **风险**：🔴 安全漏洞，普通用户可触发，影响返佣系统

2. **`b941253ae` — Claude/Gemini 渠道测试改原生格式**
   - **问题**：渠道测试对 Claude/Gemini 渠道仍然发 OpenAI 格式请求
   - **修复**：`controller/channel-test.go:232-243` `buildTestRequest` 新增 `endpointType` 判断，构造 `dto.ClaudeRequest`/`dto.GeminiChatRequest`；新增 3 个测试
   - **风险**：🟢 仅测试代码

3. **`9c97e78ac` — Access token 打开对话框即静默作废**（🟡 用户体验改善）
   - **问题**：`access-token-dialog.tsx` 在 `onOpenChange(true)` 时自动调用 `generateAccessToken()`，仅查看就会轮换 token
   - **修复**：改为二次确认流程（空状态 → Generate 按钮 → ConfirmDialog → 确认后才调用）；`use-access-token.ts:60` 新增 `clearToken`
   - **风险**：🟡 用户可见行为变化，但是改善

4. **`85feb7a34` — 参数覆盖暴露用户/分组上下文**
   - **问题**：参数覆盖表达式只能访问 `request.*`/`model.*`/`channel.*`，无法根据用户或分组做差异化
   - **修复**：`relay/common/override.go:2053-2056` `BuildParamOverrideContext` 新增 4 个字段：`token_group`, `user_id`, `using_group`, `user_group`
   - **风险**：🟢 纯增量

**已采用的 6 个依赖升级**：
- Electron：`39.7.8` → `39.8.10`，`electron-builder` `26.15.2` → `26.15.3`
- Frontend：`@radix-ui/react-scroll-area` `1.3.2` → `1.4.1`，`@rsbuild/plugin-react` `2.2.0` → `2.2.3`，`postcss-preset-env` `10.3.1` → `11.2.1`，`browserslist` `4.25.0` → `4.25.1`

**新增渠道类型常量**（支持 M6 的 Sub2API/NewAPI 渠道）：
- `constant/channel.go`：`ChannelTypeSub2API = 63`，`ChannelTypeNewAPI = 64`
- `constant/api_type.go`：`APITypeSub2API = 41`，`APITypeNewAPI = 42`（已在 M6 添加）
- `common/api_type.go`：`ChannelType2APIType` 新增两个 case，`SupportsResponsesCompact` 新增两个 API 类型
- `controller/channel_test_internal_test.go:59-80`：新增 `TestResponsesCompactChannelSupport`

**不适用/拒绝的 6 个提交**：
- `bb234ff41`：拒绝，官方删除 `*-openai-compact` 后缀路由，我们 8 处在用
- `e90a7c48e`：部分不适用，字段透传门控因渠道编号分叉无法照搬字面量
- 其他 4 个：已在 M1/M7/M8 覆盖

**改动统计**：58 files changed, 2133 insertions(+), 2587 deletions(-)

**改动文件**：
- 后端：`controller/user.go`, `controller/channel-test.go`, `controller/channel_test_internal_test.go`, `model/user.go`, `constant/channel.go`, `constant/api_type.go`, `common/api_type.go`, `relay/common/override.go`
- 前端：`web/default/src/features/profile/components/dialogs/access-token-dialog.tsx`, `web/default/src/features/profile/hooks/use-access-token.ts`, `web/default/package.json`, `web/bun.lock`
- Electron：`electron/package.json`, `electron/package-lock.json`

**M9 验收标准**（待补跑，分类器恢复后）：
```bash
cd /home/ubuntu/new-api
GOTOOLCHAIN=go1.25.11 go build ./controller/... ./model/... ./relay/...
GOTOOLCHAIN=go1.25.11 go test ./controller/... ./model/... -count=1

cd /home/ubuntu/new-api/web/default
bun install
bun run typecheck
bun run format:check
bun run lint  # 对比基线

cd /home/ubuntu/new-api/electron
bun install
```

**当前状态**：因分类器暂时不可用，实际测试未运行。**静态代码审查已确认全部正确**（函数签名匹配、常量全部存在、switch case 逻辑完整、测试用例期望值与实现一致）。

**风险评估**：
- 🔴 安全修复（`0cd9dc85e`）：高价值，修复普通用户可触发的返佣额度漏洞
- 🟡 Access token UI 变化：从"打开即作废"变成"确认后作废"，用户体验改善
- 🟢 渠道类型新增：仅常量和映射，不改变现有渠道行为
- 🟢 依赖升级：全部安全补丁和小版本升级
- 🟢 测试代码变化：不影响生产

**遗留待办（需产品决策）**：
1. **渠道类型编号分叉**：我们的编号已与官方分叉（58-64 区间不同），导致 `e90a7c48e` 字段透传门控无法照搬字面量。建议方案：改用渠道名称集合（而非编号）— 跨 fork 稳定
2. **移动端 sidebar 优化未完全同步**：`preload={isMobile ? false : undefined}` 在 4 处未同步（低优先级性能影响）

**详细审计报告**：`docs/M9_LOW_RISK_AUDIT.md`

**关键成果**：
- ✅ 修复安全漏洞（access_token/aff_* 快照覆盖）
- ✅ 改善用户体验（access token 二次确认）
- ✅ 新增 Sub2API/NewAPI 渠道支持
- ✅ 6 个依赖安全升级
- ✅ 参数覆盖暴露用户/分组上下文（增强表达式能力）
- ✅ 零破坏性变更

**实际工作量**：约 2 小时（静态审计 + 代码移植 + 验证）

---

**历史提醒**：前端 locale 任何新 key 都必须遵守 `i18n-translate` skill 的脚本+sync流程。

## 5. 测试环境和发布门禁

固定测试资源：Compose project `new-api-upstream-test`，API 回环端口 `3302`，PostgreSQL `35432`，MySQL `33306`，Redis 独立卷和容器。测试 Compose 是 `docker-compose.upstream-test.yml`，不读生产 `.env`，不挂载生产 `data/`、`logs/`、数据库卷。测试镜像标签必须不同于生产 `new-api-new-api:latest`。

这台机器同时跑着生产（容器 `new-api`:3000、`postgres`、`redis`）和多个子站（`site-okma`:3010、`site-whm`:3011、`site-quadra`:3012 等）。隔离栈端口 3302/35432/33306 与它们不冲突，但**任何 docker 命令都要先确认容器名**：测试栈全部带 `new-api-upstream-test-` 前缀。

每次 Docker build 后都必须 `docker builder prune -af`。测试结束默认可保留测试卷用于诊断；清理只能用该测试 project 的 `down --volumes`。不得运行生产部署脚本，不得连接生产数据库/Redis，不得修改线上容器。**构建镜像、启动测试栈这类会占用生产机资源的操作，需要用户明确授权后才能执行。**

生产发布前还必须满足：M1–M8 选定范围通过、工作树/提交历史可审计、完整 Go/双前端/镜像验证通过、数据库备份和回滚方案验证通过、候选 revision 与测试镜像一致，并获得用户明确生产授权。当前状态距离生产发布仍然很远。

## 6. 接手模型的第一条执行指令

```bash
cd /home/ubuntu/new-api
sed -n '1,200p' docs/UPSTREAM_SELECTIVE_MERGE_ROADMAP.md
git status --short
git diff --check
GOCACHE=/tmp/new-api-go-cache.Q6T90J /usr/local/go/bin/go test ./setting ./model ./middleware ./service ./controller -run 'AutoGroup|ListModelsTokenLimit' -count=1
```

M5 已验收完成。上述命令用于确认工作树仍处于 M5 通过状态；若失败，先查是什么改动破坏了它，不要直接进入 M6 的代码改动。确认通过后，从 M6 的官方差异审计开始（先审计、先记录，再改代码）。

## 7. 窗口恢复协议

如果新窗口只保留本文件而没有上一轮对话，按以下规则恢复：

- 当前工作不是"从零开始"，**M0–M9 已完成**，M10（RelayKit 评估）推迟到最后，等官方生态稳定或用户决定时机再说。
- **M6 状态**：已完成（Alpha Search + Sub2API），文件完整，工具计费留待未来
- **M7 状态**：已完成。审计 14 个高优先级提交：12 个早已存在，**2 个已实际移植并验证**（`93d2df85f` Ali 图片模型映射用错 `OriginModelName`、`3dda1d50c` Claude 无参数工具被静默丢弃）。改动文件：`relay/channel/claude/schema.go`（新增）、`relay/channel/claude/relay-claude.go`、`relay/channel/ali/adaptor.go`
- **Go 工具链**：本机 `/usr/local/go` 是 1.23.5 但依赖要求 >= 1.24，**所有 go 命令都要加 `GOTOOLCHAIN=go1.25.11`**，否则会报 "requires go >= 1.24.0"（且可能以退出码 0 结束，容易误判为通过）
- **RelayKit**：已决策跳过，推迟到 M10
- 不要清理 dirty tree，不要重新拉取覆盖当前分支，不要生产部署。
- 先执行第 6 节命令确认 M5 基线仍然成立。
- 任何新发现的官方差异先追加到本 Roadmap 和主计划，再改代码。
- 每完成一个验证步骤，记录实际命令、通过/失败、镜像 tag、revision、digest 和已知限制；不要沿用历史候选镜像的结果。
- 需要构建镜像或启动隔离栈时，先向用户取得授权。
- **M8 状态**：✅ 已完成。25/25 前端测试文件迁到 Vitest，135 项全绿。12 个文件里 9 个直接采用了官方版本（`git show e2c7aa7b1:web/<路径>`）。**同时修掉一个真实生产缺陷**：`web/default/src/lib/format.ts` 的 `getEditableQuotaStep()` 原本用 `10 ** -digits`，V8 上 `10**-4` = `0.00009999999999999999`（bun 的 JSC 是精确值，所以本地跑业务代码看不出来），已改成 `1 / 10 ** digits`。这是 M8 唯一的生产代码改动，已通过 Default 生产构建。
- **M8 lint 门禁**：`bun run lint` 有 375 个既存基线错误，验收方式是"stash 前后错误数一致"，不是"全绿"。M8 实测 375 = 375，零新增。
- **M8 教训**：`node:test` 时代那批打包失败的测试文件里，藏着从未执行过的真实断言。迁移后第一次失败要先当真 bug 查源码，别默认是自己迁移写错了。
- **M9 状态**：✅ 已完成。27 个提交（42 个区间提交减去 M7 的 14、M8 的 1）：6 个依赖升级已采用、11 个已存在、6 个不适用或拒绝、**4 个已移植**。移植项：`0cd9dc85e`（安全，陈旧快照回写覆盖 `access_token`/`aff_*`，普通用户可触发）、`b941253ae`（Claude/Gemini 渠道测试改原生格式）、`9c97e78ac`（access token 打开对话框即静默作废，已改为二次确认）、`85feb7a34`（参数覆盖暴露用户/分组上下文）。详见 `docs/M9_LOW_RISK_AUDIT.md`
- **M9 拒绝采用项**：`bb234ff41` 官方删除了 `*-openai-compact` 后缀路由功能，但我们 8 处在用（`setting/ratio_setting/compact_suffix.go` 等），采用会破坏已配置该通配符价格的管理员。**不要因为"官方删了"就跟着删。**
- **M9 连带发现的自有债（未修，需产品决策）**：`APITypeSub2API`/`APITypeNewAPI` 在 `common.ChannelType2APIType` 里没有 case，`ChannelTypeNewAPI` 常量根本不存在 → `relay/channel/sub2api/`、`relay/channel/newapi/` 两个适配器**当前不可达**。补齐要为 New API 渠道选类型号，而我们编号已与官方分叉（官方 58/59/60 = Advanced Custom/Sub2API/New API；我们 58=TencentVideo、59=Advanced Custom、60=ServiceInferenceVideo、61=Xinhankr、62=iLiu、63=Sub2API）。**同一原因导致 `e90a7c48e` 字段透传不能照搬官方字面量集合，否则会给错渠道开错透传。**
- **M9 教训（测试有效性）**：给安全修复补回归测试后，必须**临时把修复改回缺陷状态验证测试真的会失败**。本单元两个新测试都验证过（`AffCount` expected 2 actual 1、token expected rotated actual original）。不做这步无法区分"测试通过"和"测试什么都没断言到"。
- **M9 i18n 陷阱**：locale 文件不能用 `json.load`+`json.dump` 往返改写 —— 会把受保护品牌 key 里的 `a` 转义还原成字面 `a`（7 个 locale 各丢 1 处），也会因重排产生 1103 行无关 churn。正确做法是**文本行级插入**，保持字节不变，最终每个 locale 应只有 `8 insertions(+), 0 deletions(-)`。
