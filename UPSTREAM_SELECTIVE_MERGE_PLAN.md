# 官方全量差异选择性合并计划

> 建立日期：2026-08-17  
> 当前开发基线：`custom` / `5fe45b3f09822a2424e56c77f3c2ec5c8b0618f0`  
> 官方对照基线：`upstream/main` / `e2c7aa7b102c2075eae2377df3508658d45e88dc`  
> 对照方式：源码全树、目录迁移映射和行为语义对照，不以版本号差值代替审计

## 1. 目标与边界

目标是在保留当前定制功能、数据库数据和既有 API 合约的前提下，逐项移植官方实现中有价值的安全修复、稳定性修复和功能增强。

执行边界：

1. 每次只处理一个可独立验收的合并单元。
2. 不直接 merge、rebase 或整体覆盖 `upstream/main`。
3. 不删除或覆盖当前定制渠道、异步任务、存储、返佣、前端路由和 Classic 前端。
4. 不复用生产数据库、Redis、卷、端口、容器、镜像标签或密钥。
5. 每个单元必须完成代码审查、自动化测试、独立镜像构建和隔离环境验收。
6. 全部计划项通过之前不进入生产发布流程；发布还需要单独确认。

## 2. 必须保留的当前定制能力

- 推荐返佣和收入报表。
- 上传管理、R2 公共上传、本地存储和 Aliyun OSS 选择逻辑。
- 异步图片、Gemini 图片和 Gemini Omni 视频任务。
- Seedance 资产、计费和任务查询。
- Kling 元素、3.0 Omni、Motion Control。
- xAI、TencentVideo、ServiceInference、xinhankr 和 iLiu Midjourney 渠道。
- 日志导出、自定义任务详情、用户设置和定制前端路由。
- `web/classic` 与 `web/default` 双前端构建。

## 3. 隔离测试环境

测试栈固定使用 `docker-compose.upstream-test.yml`：

| 资源 | 测试环境 | 生产环境 |
|---|---|---|
| Compose project | `new-api-upstream-test` | 默认生产 project |
| API 镜像 | `new-api-upstream-test:<revision>` | `new-api-new-api:latest` |
| API 容器 | `new-api-upstream-test-api` | `new-api` |
| API 监听 | `127.0.0.1:3302` | `0.0.0.0:3000` |
| PostgreSQL | `new-api-upstream-test-postgres` / `127.0.0.1:35432` | `postgres` |
| MySQL | `new-api-upstream-test-mysql` / `127.0.0.1:33306` | 生产数据库 |
| Redis | `new-api-upstream-test-redis` | `redis` |
| 数据卷 | Compose 独立命名卷 | 生产卷/目录 |
| 凭据 | 文件内测试专用值 | 环境变量/生产密钥 |

约束：

- 测试脚本不得调用 `scripts/deploy-production.sh deploy`。
- 测试 Compose 不读取生产 `.env`，也不挂载 `data/`、`logs/` 或生产数据库卷。
- 镜像必须从候选工作树重新构建，并携带候选 Git revision 标签。
- 每次 Docker 构建结束后执行 `docker builder prune -af`。
- 测试数据默认保留供故障分析；只有明确执行测试栈清理时才删除测试卷。

## 4. 每个合并单元的固定流程

每一项都按以下顺序执行，任何一步失败都停止进入下一项：

1. **范围冻结**：列出官方相关提交、文件、当前等价实现和定制冲突。
2. **基线测试**：在修改前记录相关测试结果，确认不是已有故障。
3. **最小移植**：按当前架构重新实现，不机械 cherry-pick。
4. **代码级验证**：格式化、静态检查、目标包测试和回归测试。
5. **数据库验证**：涉及模型或事务时验证 SQLite、MySQL、PostgreSQL 兼容性。
6. **前端验证**：涉及前端时执行 i18n 同步、类型检查和生产构建。
7. **测试镜像**：构建独立候选镜像并启动隔离 PostgreSQL/Redis 栈。
8. **镜像验收**：健康检查、数据库迁移、相关 API、错误路径和并发路径验收。
9. **定制回归**：确认本计划第 2 节中的相关定制能力未受影响。
10. **记录结论**：在本文件状态表写入 revision、测试命令、结果和已知限制。

## 5. 合并顺序

### M0：建立测试基线和隔离镜像环境

状态：**已完成**

内容：

- 添加隔离测试 Compose 和统一验证脚本。
- 对当前未修改基线执行 Go 测试、前端生产构建和完整 Docker 构建。
- 启动独立 PostgreSQL/Redis/API 容器并等待健康检查通过。
- 记录当前已有失败，避免把历史问题错误归因于后续移植。

验收条件：

- 测试栈不连接或修改任何生产资源。
- 测试镜像与生产镜像名称不同。
- API 容器健康，数据库迁移成功。
- 基线失败项有明确记录和归属。

基线结果（2026-08-17）：

- `bun run --cwd web/default build`：通过。
- `/usr/local/go/bin/go test ./...`：各业务包通过；首次与前端构建并行时根包因 `web/default/dist/index.html` 尚未生成而失败，前端产物生成后 `/usr/local/go/bin/go test .` 通过。
- 隔离镜像：`new-api-upstream-test:5fe45b3f0982-candidate`，构建通过并已执行 `docker builder prune -af`。
- 隔离容器：PostgreSQL、Redis、API 均健康；API 镜像 revision 为 `5fe45b3f09822a2424e56c77f3c2ec5c8b0618f0`。
- 原定 `3012` 被现有容器占用，测试端口调整为仅回环监听的 `3302`。

### M1：原子额度、行锁、充值与退款安全

状态：**已完成**

内容：

- 引入 GORM v2 `clause.Locking`，SQLite 使用兼容分支。
- 替换无效的 `Set("gorm:query_option", "FOR UPDATE")`。
- 移植用户与 Token 额度原子预扣、缓存补偿和数据库降级逻辑。
- 修复充值订单原子结算、不可入账订单拦截和并发状态更新。
- 修复异步退款的 `used_quota` 同步调整和兑换码精度问题。
- 保持现有饱和转换、`QuotaClamp` 审计和 tiered billing snapshot 语义。

重点测试：

- 并发预扣不能产生负额度或超扣。
- Redis 命中、未命中、故障降级和补偿失败路径。
- 有限 Token、无限 Token、用户钱包、订阅额度和任务退款。
- 重复支付回调、重复任务结算和重复退款必须幂等。
- SQLite、MySQL、PostgreSQL 的锁与条件更新语义。

验收结果（2026-08-17）：

- 引入 GORM v2 行锁兼容层，替换兑换码、充值、订阅、用户转账中的无效旧式查询锁；SQLite 保持无锁兼容分支。
- 用户与 Token 预扣改为 Redis Lua 原子保留，含缓存未命中安全装载、数据库条件更新降级和 Redis/数据库补偿。
- Epay 及其他充值结算增加订单行锁、状态幂等、支付渠道校验、钱包容量上限和原子入账；兑换码增加 CAS 状态更新和并发幂等。
- 异步任务与 Midjourney 退款同步回退用户、Token、渠道 `used_quota`，且不错误减少请求计数；部分结算失败可按实际入账阶段退款。
- Default 前端兑换码编辑保留数据库中的精确 quota，只有字段被实际编辑时才重新换算；CNY 小数精度和异步加载竞态已有测试。
- `/usr/local/go/bin/go test ./...`：通过。
- Model/Service 关键并发路径 `go test -race`：通过。
- `bun test` 兑换码精度用例：3 项通过；`bun run typecheck`、目标 lint、`bun run i18n:sync`、`bun run build`：通过。
- 候选镜像：`new-api-upstream-test:5fe45b3f0982-dirty-candidate`；镜像 revision 为 `5fe45b3f09822a2424e56c77f3c2ec5c8b0618f0-dirty`。
- 独立 PostgreSQL、MySQL、Redis、API 均健康；跨库并发预扣及 5 路重复支付回调测试在 PostgreSQL/MySQL 均通过。
- Docker 构建缓存已按规则执行 `docker builder prune -af`，共回收 `7.517GB`。
- 未调用生产部署脚本，未连接生产数据库、Redis、卷、端口或镜像标签。

### M2：Relay 转换正确性修复

状态：**已完成**

内容：

- Responses 转换保留 `presence_penalty` 和 `frequency_penalty`。
- Chat → Responses 保留 `prompt_cache_key`。
- Claude 请求不注入空 `tools`。
- Ali 请求未提供 `top_p` 时保持缺省。
- Ollama 保留 reasoning 和 tool-call 上下文。
- 修复 Gemini 风格 `/v1/models`。
- 支持 HTTP/2 流重置后的可重放请求体。

重点测试：显式 `0`/`false` 保留、字段缺省不注入、流式终止、工具调用、客户端断开和请求重放。

验收结果（2026-08-17）：

- 按官方提交 `253a74dd1`、`7d09c6954` 保留 Chat → Responses 的 `frequency_penalty`、`presence_penalty`、`prompt_cache_key`；显式 `0` 与字段缺省均有精确断言。
- 按 `4442bb302` 修复 Claude 空 `tools` 注入，同时保留函数工具和 Web Search 工具。
- 按 `2399de97d` 修复 Ali/DashScope 未提供 `top_p` 时被注入近似贪心值；显式边界值按两位小数约束收敛到 `0.01`/`0.99`。
- 按 `8ad159a3b` 保留 Ollama reasoning effort、assistant thinking、tool-call ID、tool result 关联和结构化输出 schema；非流请求显式发送 `stream:false`。
- 按 `3d5dc36f1` 修复 Gemini 风格 `/v1/models` 的 query/header API key 认证和模型列表响应格式，并保留 OpenAI Bearer 格式。
- 按 `d6b5ce99d` 为内存/磁盘 BodyStorage 增加独立重放 reader，给全部转换及透传路径绑定 `ContentLength`/`GetBody`，清理跨渠道尝试的失效元数据，并禁止自动跟随上游 3xx。
- 真实 HTTP/2 测试已验证：首次请求体完整写出后收到 `REFUSED_STREAM`，transport 自动重试且两次请求体完全一致；未设置 `GetBody` 的对照用例按预期无法重试。
- `/usr/local/go/bin/go test ./...`：通过；Relay/转换/路由目标包 `go test -race`：通过。
- Default 与 Classic 前端均在候选 Docker 多阶段构建中通过生产构建；Go 生产二进制构建通过。
- 候选镜像：`new-api-upstream-test:5fe45b3f0982-dirty.120fa75fcb51-candidate`；镜像 revision 为 `5fe45b3f09822a2424e56c77f3c2ec5c8b0618f0-dirty.120fa75fcb51`。
- 独立 PostgreSQL、MySQL、Redis、API 均健康；M1 跨库额度/重复支付回调回归继续通过。
- Docker 构建缓存已按规则执行 `docker builder prune -af`，本次回收 `7.517GB`。
- 未调用生产部署脚本，未连接或修改任何生产资源。

### M3：新认证与多设备会话体系

状态：**已完成**

内容：短期 Access JWT、Refresh Token 轮换与重用检测、会话列表/撤销、`auth_version`、OAuth Auth Flow、外部身份声明、前端会话同步及兼容迁移。

重点风险：现有 Passkey、2FA、OAuth 绑定、邀请注册、返佣注册链路和旧 Session 兼容。

验收结果（2026-08-17）：

- 引入短期 Access JWT、HttpOnly Refresh Token、服务端多设备会话、Refresh Token 轮换/宽限窗口/重放撤销、会话上限、会话列表、单会话撤销、撤销其他会话、登出和定期清理；Default 与 Classic 均不持久化 Access Token。
- 引入 `auth_version` 与用户认证缓存栅栏；密码修改/重置、管理员降权/删除及关键安全设置变更会使既有凭证失效，并保留 Classic Gin Session 的受控兼容路径，迁移前旧 Cookie 不会绕过版本检查。
- OAuth 登录/绑定、2FA、Passkey、Telegram、WeChat 使用一次性 Auth Flow 或会话绑定声明；外部身份绑定通过唯一声明表防止并发抢占，安全操作使用带 scope 的短期 Security Proof。
- 保留当前定制邀请注册与返佣约束：密码、OAuth、WeChat 新注册仍强制校验邀请码；既有用户登录不受影响。Classic 与 Default 双前端均完成新会话协议适配。
- M3 仅合并认证会话核心；官方同批次的 Trusted Proxy、Origin Guard 与统一输入校验明确留在 M4，避免跨单元混合上线。官方认证文档也将在 M4 安全边界完成后按当前双前端兼容策略落地。
- Go 全仓编译通过；认证相关 model/service/controller/middleware/common 目标测试通过；会话轮换、缓存栅栏和并发路径的目标 `go test -race` 通过。
- Default：37 个 Vitest 回归测试、TypeScript 检查、生产构建、M3 目标 lint 与 i18n 同步检查通过；Classic：生产构建与 M3 目标格式检查通过。全量前端 lint 仍有非 M3 文件的既有基线问题，未把无关清理混入本单元。
- 最终候选镜像：`new-api-upstream-test:5fe45b3f0982-dirty.7e14f6490ea7-candidate`；镜像 revision 为 `5fe45b3f09822a2424e56c77f3c2ec5c8b0618f0-dirty.7e14f6490ea7`，与最终工作树内容摘要一致。
- 独立 PostgreSQL、MySQL、Redis、API 均健康；两种数据库的额度回归与认证迁移测试通过，PostgreSQL 已确认 `user_sessions`、`auth_flows`、`external_identity_claims` 迁移表存在。
- 隔离 API 端到端验证通过：登录、Refresh 轮换、宽限期内重放、宽限期后旧 Token 重用触发整会话撤销、Access Token 同步失效、撤销其他会话、登出及登出后刷新拒绝。
- Docker 构建缓存已按规则执行 `docker builder prune -af`，本次回收 `7.619GB`；未调用生产部署脚本，未连接或修改任何生产资源。

### M4：Trusted Proxy、Origin Guard 与输入校验

状态：**已完成**

内容：可信代理配置、Secure Cookie Origin 防护、统一后端字符长度校验、JSON 包装器合规和相关安全回归测试。

验收结果（2026-08-17）：

- 增加可信代理三态配置：缺省仅信任回环/私网代理，`none` 明确不信任任何代理，显式列表完全替换缺省值；非法配置在启动时失败，避免静默降级。
- Secure Cookie 的刷新与登出接口增加精确 Origin/Referer 校验；拒绝通配符、路径/查询、userinfo、后缀欺骗和多值来源，不信任客户端伪造的 `X-Forwarded-Proto`。保留当前更严格的定制 CORS 白名单，没有引入官方宽松 CORS 行为。
- Redis 关键限流改为原子 Lua 固定窗口并启用 v2 命名空间，返回 `Retry-After`；补齐用户级关键操作限流、邮件校验 Redis/内存安全降级及模型限流 UTC 时间语义。
- 敏感认证接口统一禁用缓存；Turnstile 不再把一次验证结果缓存在 Gin Session 中，每次受保护操作均独立校验。
- Console 设置校验按浏览器 UTF-16 字符计数，并统一使用项目 JSON 包装器；公告附加内容限制为 100 个字符。
- 新增完整认证说明与环境变量文档，覆盖 Access/Refresh、多设备会话、兼容 Gin Session、可信代理、Secure Origin 和部署配置。
- `/usr/local/go/bin/go test ./... -timeout 5m`：全仓通过；`go test -race ./middleware ./common ./setting/console_setting`：通过。同步修复了 M3 测试夹具在单连接 SQLite 事务中回查导致的真实死锁，保留原认证栅栏回归语义。
- Default 的 M3/M4 目标 Vitest 共 37 项通过；全量 Vitest 仍有 12 个既有 `node:test` 测试文件无法被 Vitest 打包，已明确留给 M8 统一迁移。Default、Classic 和 Go 生产构建均在 Docker 多阶段构建中通过。
- 最终候选镜像：`new-api-upstream-test:5fe45b3f0982-dirty.5dc3ff151ece-candidate`；镜像 revision 为 `5fe45b3f09822a2424e56c77f3c2ec5c8b0618f0-dirty.5dc3ff151ece`，manifest list digest 为 `sha256:2bdcb2c29b148146bdb9efa8c1d3ddfa2c1ab9b6d7cfa0854a6bdfa4bf286376`。
- 独立 PostgreSQL、MySQL、Redis、API 均健康；双数据库额度/结算与认证迁移回归通过。安全 E2E 已验证严格来源防护、禁止不可信 CORS 反射及 `TRUSTED_PROXIES=none` 下无法通过伪造转发 IP 绕过限流；M3 完整会话 E2E 再次通过。
- Docker 构建缓存已按规则执行 `docker builder prune -af`，本次回收 `7.619GB`；未调用生产部署脚本，未连接或修改任何生产资源。

### M5：Token 级有序 AutoGroups

状态：**已完成**

内容：Token `auto_groups`、顺序编辑、API、缓存、分组过滤、重试计费和前端状态显示。

重点风险：现有全局 AutoGroups、订阅分组、tiered retry 最终分组结算。

- 最终候选镜像：`new-api-upstream-test:b198b4b54515-dirty.aa76d5cb2337-candidate`；镜像 revision 为 `b198b4b545153db25b77c1753a168af03db8e99a-dirty.aa76d5cb2337`，image digest 为 `sha256:db673b45e23a7d2a4c1e519de78530ae0cbd7aa15c61e86231f6d3984473b3c8`。该 tag 与当前工作树 digest 已核对一致。
- 本单元起 HEAD 从 `5fe45b3f0982` 前进到 `b198b4b5451`（新增定制提交 `feat(grok): adapt video requests for service inference`），因此候选 tag 前缀与 M0–M4 不同。
- 隔离栈 PostgreSQL/MySQL/Redis/API 全部健康；`TestQuotaAndSettlementCrossDatabase` 与 `TestUserSessionPreviousRefreshHashMigrationConfiguredDatabases` 在独立 PostgreSQL 与 MySQL 上通过，覆盖 `auto_groups` TEXT 列类型、有序 JSON 往返和 `auto_group_priority` 旧列迁移。
- Go：`go test ./...` 41 包全通过；`go test -race ./model ./service ./middleware ./controller ./setting` 通过；`go build ./...`、`go vet ./...`、`gofmt` 干净。
- 前端：Default `typecheck` 通过；M5 目标 6 个 Vitest 文件 27 项通过；Default 与 Classic `build` 均通过；Default `format:check` 与目标 oxlint 回到 HEAD 基线；Classic 改动文件 Prettier 与 ESLint 干净。全量 Vitest 仍有 12 个既有 `node:test` 文件无法打包，属 M8 范围，与本单元无关。
- E2E（对最终候选镜像执行，顺序为会话 → M5 → 安全）：`verify-auth-session-e2e.sh`、`verify-token-auto-groups-e2e.sh`、`verify-auth-security-e2e.sh` 全部通过。
- Docker 构建缓存已按规则执行 `docker builder prune -af`；未调用生产部署脚本，未连接或修改任何生产资源，生产 `new-api` 容器全程保持 healthy。

本单元验收过程中发现并修复的缺陷见第 5.1 节。

#### 5.1 M5 验收期间发现的缺陷

1. **MySQL 启动崩溃（生产级）**：`model/main.go` 的 `migrateTokenAutoGroupsFromLegacyPriority` 以字符串表名调用 `DB.Migrator().HasColumn("tokens", ...)`。GORM v1.25.2 基础 migrator 在该路径不解析 schema 且未做 nil 判断，PostgreSQL 与 SQLite 驱动各自重写并带 `stmt.Schema != nil` 保护，MySQL 驱动没有重写，因此走基础实现时空指针 panic。任何 MySQL 部署都会在迁移阶段崩溃。已改为传模型值 `&Token{}`（旧列在 Go 结构体中已无字段，`LookUpField` 返回 nil，列名原样透传，三驱动行为一致）。已通过变异验证：改回字符串形式后 crossdb 的 mysql 子测试立即重现 panic。全仓审计确认仅此一处；`HasTable`/`DropTable` 传字符串不解引用 schema，安全。
2. **跨库测试夹具不兼容 MySQL**：`legacyTokenAutoGroupsMigration` 沿用历史声明 `type:text;default:''`，MySQL 拒绝 TEXT 列带 DEFAULT（Error 1101）。历史上该定制版本因此在 MySQL 上本就无法 AutoMigrate（生产使用 PostgreSQL，故从未暴露）。夹具改为 `type:text`——迁移只读值、不关心默认值来源，同时顺带覆盖旧列为 NULL 的路径。
3. **安全 E2E 存在假通过**：`verify-auth-security-e2e.sh` 依赖 `rg`。该命令缺失时，包在 `if` 中的 4 项 CORS 否定断言被静默跳过，随后裸调用因 `set -e` 以 127 中止。已改用 `grep`，断言现已真实执行并通过。受影响的 M4 结论已用修复后脚本在 M4 自己的候选镜像上重跑补证（见下）。
4. **前端格式风格漂移**：24 个改动文件被以双引号+分号风格格式化，与项目 oxfmt 配置（`singleQuote`、`semi:false`、`printWidth:80`）冲突，导致 `format:check` 27 个文件失败并触发一个新 oxlint `curly` 报错。执行 `bun run format` 后 diff 显著缩小（例如 `api-keys-columns.tsx` 由 116+/139- 降至 15+/39-），lint 回到 HEAD 基线。已确认受影响文件全部落在本次改动范围内。

补充的测试与脚本：`controller/token_auto_groups_test.go` 增加批量接口跨用户越权边界用例（经变异验证：移除 `user_id` 约束后失败），并为拒绝路径补上确切 i18n key 断言；新增 `scripts/verify-token-auto-groups-e2e.sh` 对候选镜像做真实 HTTP + PostgreSQL + Redis 的 M5 验收。

已知约束：`verify-auth-security-e2e.sh` 会故意耗尽登录关键限流窗口（20 次 / 20 分钟），必须最后执行；否则其后的登录类脚本会收到 429。

需要说明的契约事实：本项目 `common.ApiError*` 一律返回 HTTP 200 + `success:false`，不返回 400。Roadmap 中"重复/超限返回 400"为宽泛表述，测试按真实契约锁定，未改动 API 语义。

### M6：New API、Sub2API、Alpha Search 与统一工具计费

状态：**差异审计已完成（2026-08-19），等待决策**

内容：新增网关渠道、原生格式路由、字段透传、`/v1/alpha/search`、统一 BillingUsage 和工具调用计费。

审计结论（2026-08-19）：

- 官方 commit `2d23cdf29`，65 文件，+3203/-424 行。
- **阻塞级冲突**：官方 AdvancedCustom=58、Sub2API=59 与当前生产渠道编号冲突（TencentVideo=58、AdvancedCustom=59、ServiceInferenceVideo=60、xinhankr=61、iLiu=62 已有生产数据）。
- **高风险改动**：工具计费架构重构（删除 `service/tool_billing.go`，新增 `relay/common/tool_usage.go` + 348 行测试），与当前 tiered billing 的 `OtherRatios` 存在潜在重复计费风险。
- **新增功能**：Alpha Search 端点（136 行）、Sub2API 渠道（121 行）、Codex 增强、Gemini grounding 检测。
- **前端改动**：Default 有工具价格 UI（+544 行含测试），Classic 未同步。
- **完整审计报告**：`docs/M6_AUDIT_REPORT.md` (15000+ 字)

待决策问题：

1. **渠道编号方案**（阻塞项）：
   - 方案 A（推荐）：Sub2API 改用 63，保持 58-62 生产编号不变
   - 方案 B：数据迁移腾出 58-59
   - 方案 C：不要 Sub2API，只移植工具计费
2. **工具计费兼容策略**（阻塞项）：
   - 方案 A：废弃 `OtherRatios`，统一用 `ToolSurchargeItem`
   - 方案 B：保留 `OtherRatios`，禁用官方工具计费
   - 方案 C（推荐）：互斥检测，保持向后兼容
3. **Alpha Search**：是否移植？（推荐否，生产无需求）
4. **Sub2API**：是否移植？（推荐否，Codex 已覆盖）
5. **Classic 前端**：是否同步工具价格 UI？（需确认 Classic 是否在用）

强制约束：

- 不复用官方渠道编号 58、59、60。
- 当前 `TencentVideo`、`AdvancedCustom`、`ServiceInferenceVideo`、`xinhankr` 和 `iLiu` 编号保持兼容。
- 新编号必须附数据库兼容检查和前后端同步测试。
- 工具计费改动必须先读 `pkg/billingexpr/expr.md`，全链路验证 validation → pre-consume → settle/refund。

### M7：RelayKit 架构迁移评估与实施

状态：**待开始**

先完成技术决策：完整引入独立 `relaykit` 模块，或继续在当前转换层逐项移植。未经决策和兼容矩阵验证，不进行大规模目录替换。

### M8：测试体系、依赖、Electron 与普通 UI 更新

状态：**待开始**

内容：扩展 Vitest 覆盖、补齐缺失回归测试、Go/前端依赖安全升级、Electron 更新和低风险 UI 修复。M3 已因认证回归需要引入最小 Vitest 测试基线；M8 仍负责广泛测试迁移与其他依赖/UI 工作，其中包括将当前 12 个使用 `node:test` 的既有测试文件迁移到 Vitest。依赖升级与业务改动分开提交和验证。

## 6. 状态与验收记录

| 单元 | 状态 | 候选 revision | 自动化测试 | 测试镜像 | 隔离环境 | 结论 |
|---|---|---|---|---|---|---|
| M0 测试基线 | 已完成 | `5fe45b3f0982` | Go/前端通过 | `new-api-upstream-test:5fe45b3f0982-candidate` | 健康 | 通过 |
| M1 额度与事务安全 | 已完成 | `5fe45b3f0982-dirty` | Go/前端/race/三数据库通过 | `new-api-upstream-test:5fe45b3f0982-dirty-candidate` | PostgreSQL/MySQL/Redis/API 健康 | 通过 |
| M2 Relay 正确性 | 已完成 | `5fe45b3f0982-dirty.120fa75fcb51` | Go/race/HTTP2/双前端通过 | `new-api-upstream-test:5fe45b3f0982-dirty.120fa75fcb51-candidate` | PostgreSQL/MySQL/Redis/API 健康 | 通过 |
| M3 认证会话 | 已完成 | `5fe45b3f0982-dirty.7e14f6490ea7` | Go/race/Vitest/双前端/E2E/跨库迁移通过 | `new-api-upstream-test:5fe45b3f0982-dirty.7e14f6490ea7-candidate` | PostgreSQL/MySQL/Redis/API 健康 | 通过 |
| M4 代理与输入安全 | 已完成 | `5fe45b3f0982-dirty.5dc3ff151ece` | Go 全仓/race/双前端/跨库通过；安全 E2E 与会话 E2E 已于 2026-08-18 用修复后脚本在该镜像上重跑补证 | `new-api-upstream-test:5fe45b3f0982-dirty.5dc3ff151ece-candidate` | PostgreSQL/MySQL/Redis/API 健康 | 通过 |
| M5 Token AutoGroups | 已完成 | `b198b4b54515-dirty.aa76d5cb2337` | Go 全仓/race/双前端/跨库/会话 E2E/安全 E2E/M5 API E2E 通过 | `new-api-upstream-test:b198b4b54515-dirty.aa76d5cb2337-candidate` | PostgreSQL/MySQL/Redis/API 健康 | 通过 |
| M6 网关与工具计费 | 审计完成 | `b198b4b54515` (M5基线) | — | — | — | 审计报告已完成，等待决策 |
| M7 RelayKit | 待开始 | — | — | — | — | — |
| M8 测试/依赖/UI | 待开始 | — | — | — | — | — |

## 7. 生产发布门禁

只有同时满足以下条件，才进入生产发布讨论：

- M1–M8 中选定的范围全部验收通过。
- 候选分支工作树干净，提交历史可审计。
- 完整 Go 测试、前端构建和候选镜像构建通过。
- 独立测试栈完成迁移、功能、并发、错误路径和定制回归。
- 已生成数据库备份与回滚方案，并验证旧镜像可恢复。
- 候选 revision 与测试镜像 revision 完全一致。
- 获得明确的生产发布授权。

生产发布只能使用现有 `scripts/deploy-production.sh` 的检查和确认门禁；本计划执行期间不得绕过该脚本直接替换线上容器。
