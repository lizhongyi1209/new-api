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

- 当前定制 HEAD：`b198b4b545153db25b77c1753a168af03db8e99a`（M5 验收期间从 `5fe45b3f0982` 前进了一个定制提交 `feat(grok): adapt video requests for service inference`；M0–M4 表格里的 `5fe45b3f0982-*` tag 因此不会再复现）
- `upstream/main`：`e2c7aa7b102c2075eae2377df3508658d45e88dc`
- 已完成的官方单元：`0ab020206`（Auto group，官方 PR #6590）
- 下一个待审计单元：M6（New API / Sub2API / Alpha Search / 统一工具计费）

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

### M6：New API、Sub2API、Alpha Search、统一工具计费（当前单元，尚未开始差异审计）

先审计官方相关 commit 和当前渠道编号。严禁复用当前定制的 `TencentVideo`、`AdvancedCustom`、`ServiceInferenceVideo`、`xinhankr`、`iLiu` 编号；新增编号必须通过三数据库和前后端同步测试。计费改动必须先读 `pkg/billingexpr/expr.md`，沿 validation → EstimateBilling/OtherRatios → quota conversion → pre-consume → settle/refund 全链路验证，所有倍率和 quota 转换遵守 AGENTS.md 的饱和/审计规则。

M6 开工前请先读第 3.5 节的四条通用规则，那些是 M5 真实踩到的坑（尤其 MySQL migrator 与验证脚本假通过）。

### M7：RelayKit

先做技术决策和兼容矩阵：完整引入独立 relaykit，还是继续在现有转换层逐项移植。没有决策、provider 行为矩阵、请求体重放/流式/工具调用回归，不允许大规模目录替换。

### M8：测试、依赖、Electron、普通 UI

迁移当前 12 个 Vitest 无法打包的 `node:test` 文件，补真正的行为回归；依赖升级和业务改动分开；再处理 Electron 和低风险 UI。前端 locale 任何新 key 都必须遵守 `i18n-translate` skill 的脚本+sync流程。

当前这 12 个文件（`bun run test` 全量下失败、其余 13 文件 64 项通过）为：`json-code-editor-utils`、`json-code-editor`、`dropdown-menu`、`oauth-callback-mode`、`channel-field-update`、`channel-table-row-id`、`flow-selection`、`flow`、`redemption-form`、`tool-price-validation`、`cost-display`、`tool-surcharge`。失败原因统一是 `Cannot bundle Node.js built-in "node:test"`，与 M5 无关。

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

- 当前工作不是"从零开始"，M0–M5 已完成并各自有候选镜像证据，当前单元是 M6 且尚未开始差异审计。
- 不要清理 dirty tree，不要重新拉取覆盖当前分支，不要生产部署。
- 先执行第 6 节命令确认 M5 基线仍然成立。
- 任何新发现的官方差异先追加到本 Roadmap 和主计划，再改代码。
- 每完成一个验证步骤，记录实际命令、通过/失败、镜像 tag、revision、digest 和已知限制；不要沿用历史候选镜像的结果。
- 需要构建镜像或启动隔离栈时，先向用户取得授权。
