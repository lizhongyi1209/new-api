# 第 6C 批多域名 Passkey 实现与本地验收

**第 6C 批代码与必要本地验证已完成，未部署生产。** 对照既定上游 `7fd06381976cbea126d1f20973548bf60f20e37d`，接通[实施合同](./ui-batch-6c-scope-2026-09-14.md)中的全部三条认证路径、旧凭证迁移、域名设置及删除影响确认。前序[服务端阶段](./ui-batch-6c-backend-progress-2026-09-14.md)及其 3354 服务仍是历史阶段证据；最终前后端候选服务为 **127.0.0.1:3356**，使用专属新卷和两个合成用户。

## 功能、影响与风险

| 内容 | 完成的行为 | 影响与风险 |
| --- | --- | --- |
| 主域名、兼容域名 | 设置主 RP ID 与兼容列表；主域名变化保留原域名；Origin 与域名由服务端校验，历史大小写不改写 | 高：认证信任配置；配置错误仍可能使某站点的旧 Passkey 不可用 |
| 三条认证路径 | 主登录、登录追加验证、敏感操作验证均可选择许可域名；finish 只用交易保存的 RP ID，已知凭证归属优先 | 高：登录和安全 proof；没有用前端选择替代验签与授权 |
| 旧凭证兼容 | 原表新增可空 `rp_id varchar(253)`；新注册绑定域名，旧凭证仅在正确签名后补写 | 高：数据库迁移及凭证状态；不重新生成或删除旧凭证，不猜测未知归属 |
| 域名删除 | Root-only 预览显示已知受影响凭证和未知旧凭证；确认绑定当前配置、目标变更和影响统计；过时确认拒绝 | 高：单项/bulk/ServerAddress/origins 相关变更受同一事务规则保护，与凭证写入序列化 |
| 前端流程 | 登录与统一安全验证复用域名选择；成功后才记忆域名；旧记忆失效只在认证器提示前重建 begin；取消、失败和卸载不继续 finish | 中至高：认证请求生命周期；只保存域名，不保存挑战、凭证或会话令牌 |
| 设置及提示 | 主域名变化确认、兼容域名删除影响确认、当前网站填充及配置说明；七语言各补齐 43 键 | 低至中：复用 SettingsForm、SettingsControlGroup、Select、Dialog 和 ConfirmDialog；没有新增依赖或重复通用弹窗 |
| 公共状态与审计 | `/api/status` 提供选择所需 RP ID，不提供完整 origins；审计记录域名、影响数与结果 | 中：审计不保存确认值、挑战、签名或可用令牌 |

43 个本批文件包括 19 个服务端/后端翻译文件、17 个前端文件及 7 个 locale 文件。保留前序登录加密、OAuth 浏览器绑定、邀请码和微信邀请处理；没有修改根 go.mod/go.sum、relaykit、计费、渠道或生产配置。GPL/QuantumNous 头部逐字节校验通过。[最终源码指纹](./artifacts/ui-batch-6c/final-source-sha256.json)、[最终构建](./artifacts/ui-batch-6c/final-build.json)

## 专注验证与实际环境

| 验证 | 结果与证明范围 |
| --- | --- |
| 前端业务 | 3 个文件、33 项全部通过：域名进入正确 begin 请求、记忆不覆盖已知归属、失效提示重建交易、失败/取消/存储不可用/中止、操作上下文与追加登录验证隔离；没有布局、截图或纯 UI 测试。[日志](./artifacts/ui-batch-6c/frontend-tests.log) |
| SQLite | 服务端阶段 56 个函数/144 个子用例通过；最终服务再次从无 rp_id 的旧凭证表实际启动升级，并验证真实软件签名及持久化。[阶段回归](./artifacts/ui-batch-6c/backend-tests.jsonl)、[最终实际结果](./artifacts/ui-batch-6c/final-runtime.json) |
| MySQL | 隔离 MySQL **8.4.11**，同组 56 个函数/144 个子用例全部通过，包含旧表与重复迁移、原凭证/唯一性保留、三条签名路径、影响预览、过时确认、bulk 回滚、历史大小写与并发写入。[日志](./artifacts/ui-batch-6c/mysql-tests.jsonl) |
| PostgreSQL | 隔离 PostgreSQL **15.19**，同组 56 个函数/144 个子用例全部通过，验证合同同上。[日志](./artifacts/ui-batch-6c/postgres-tests.jsonl) |
| 数据库隔离和清理 | 测试夹具只允许 loopback，为主库/审计库逐例创建独立数据库；两种数据库各创建的 242 个数据库已按本次日志中精确名称清理，不操作原测试业务库。[版本、结果及清理](./artifacts/ui-batch-6c/cross-database-validation.json) |
| 最终真实 API | **14 次实际请求加重启后 1 次只读状态**全部符合预期：公开 RP ID 就绪、普通账号预览 403、未配置/已删除域名拒绝、历史 LOCALHOST 选择、错误 RP 哈希拒绝、正确签名登录旧用户、交易重放拒绝、退出后旧会话 401、已知影响 1/未知 0、普通 option 无确认 409、携确认保存。[请求日志](./artifacts/ui-batch-6c/final-runtime-calls.json) |
| 最终数据核对 | quick_check=ok；新增列可空，正确补写 LOCALHOST；原公钥与凭证 ID 保持不变、sign_count=1；仍为两个用户，一个测试 Passkey 会话已撤销，删除配置持久化且凭证保留。[数据证据](./artifacts/ui-batch-6c/final-runtime.json) |
| 构建与运行同产物 | 类型、涉及文件 lint/格式、翻译插值和构建均通过；封存 239 个前端文件，6 个 HTML 入口存在；浏览器实际加载的 11 个资源 SHA-256 与封存构建一致，无运行时错误，未执行登录或视觉验收。[构建元数据](./artifacts/ui-batch-6c/frontend-build-metadata.json)、[实际资源](./artifacts/ui-batch-6c/served-assets.json) |

最初前端回归的一项旧测试缺少新增选择组件依赖的 QueryClientProvider；补齐真实查询上下文并预载公共 status，仍保留“不调用已登录用户验证接口”的断言后，33 项全部通过。数据库首次连接被沙箱禁止 socket；按授权在允许本地连接的环境重跑同一组通过，没有修改生产控制迎合测试。翻译经技能规定的 writer 和 i18n:sync 写入；现有 translation 命名空间下的七语言内容及插值均核对。

最终二进制 SHA-256 为 `d6cc8456e6423726cd1ec586b4077e7bb202eef37d2e0d14d01b1064580ef130`，内嵌本批前端，使用新 SQLite 卷。原服务端阶段二进制及卷未覆盖。没有 Docker 镜像构建。[最终环境](./artifacts/ui-batch-6c/final-local-stage.json)

## 安全控制与边界

沿用实施前查阅的 [OWASP Authentication](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html)、[Session Management](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html)、[MFA](https://cheatsheetseries.owasp.org/cheatsheets/Multifactor_Authentication_Cheat_Sheet.html) 及 [W3C WebAuthn](https://www.w3.org/TR/webauthn-2/)：由服务端验证 RP ID 哈希、Origin、challenge、签名、UV、有效期和单次消费，继续核对用户、会话及敏感操作上下文。

适用 ASVS **5.0.0** 控制映射限于本批修改：V6.1.3/V6.3.4 对应三条认证路径的登记与一致控制；错误签名、过期、重放、错误用户及已删除域名负例已覆盖。[ASVS V6](https://github.com/OWASP/ASVS/blob/v5.0.0/5.0/en/0x15-V6-Authentication.md) V7.2.1 对应服务端会话验证，V7.4.1 对应退出后的会话拒绝；V7.5.1/V7.5.3 对应既有凭证管理及敏感操作的再次验证与 scoped proof，相关期限、跨 SID/操作、授权变化及消费失败路径回归通过。[ASVS V7](https://github.com/OWASP/ASVS/blob/v5.0.0/5.0/en/0x16-V7-Session-Management.md)

本批受影响控制未发现未解决的验证失败；这不构成全站 ASVS 认证。实际 API 使用合成软件认证器、loopback HTTP 和明确的本地不安全 Origin 开关，未改变生产 TLS/Secure Cookie 或真实用户域名；未做真实硬件/同步 Passkey 厂商认证。跨库实测版本为上述 MySQL 8.4/PostgreSQL 15，未运行历史最低版本二进制矩阵。线上启用多个实际域名需管理员配置与域名/TLS条件满足，不能将现有域名下的 Passkey 自动迁移到另一域名。

## 回退和后续

风险集中在认证信任和数据库。部署前应备份凭证、配置及数据库；仅增加可空列且不批量回填旧凭证，回退代码时可保留新增列，但旧单域名版本不会使用兼容域名列表，须恢复适用的原主域名/Origin并继续保留旧站点。域名删除不删除凭证，重新允许原域名可恢复其使用；确认前仍应确保受影响用户有可用替代登录方式。

工作区回退只能恢复本批差异，并保留前序批次及用户修改；实施前快照在 `/tmp/ui-batch-6c-before`，不使用整体 reset。第 6C 完成后继续第 7A 插件基础架构；第 7–8 批仍待实施，生产部署仍未执行。
