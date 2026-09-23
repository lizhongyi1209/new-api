# 第 6C 批多域名 Passkey 服务端阶段进展

> 本文保留实施前/服务端阶段的历史状态；后续前端、三数据库和最终候选服务验收已完成，当前结果见 [6C 最终验收](./ui-batch-6c-validation-2026-09-14.md)。

**服务端实现及隔离 SQLite 业务验证已完成，6C 整批尚未完成。** 前端域名选择/成功记忆、设置删除影响确认，以及 MySQL/PostgreSQL 集成验证仍需继续，不作为已验收功能交付。范围保持 [6C 实施合同](./ui-batch-6c-scope-2026-09-14.md) 和既定上游 `7fd06381976cbea126d1f20973548bf60f20e37d`。

## 已实现、影响和风险

| 内容 | 当前实现 | 影响与风险 |
| --- | --- | --- |
| 主域名与兼容域名 | 增加 legacy_rp_ids；不可变设置快照、域名校验、按受信 Origin 筛选许可域名；旧域名保留准确历史值 | 高：认证信任配置；不会将历史 RP 哈希输入统一成小写 |
| 凭证域名归属 | PasskeyCredential 增加可空 varchar(253) rp_id；新注册保存域名，旧凭证成功验签后补写；归属已知后禁止浏览器提示覆盖 | 高：新增凭证字段及持久化；原凭证 ID、公钥保持不变，无删除/重新生成旧密钥 |
| 全部认证路径 | 主登录、登录追加验证、敏感操作验证都接通域名选择；finish 使用交易保存的 RP ID；保留必需 UV、挑战期限/单次消费和用户/SID/操作 proof | 高：错误域名/签名或会话匹配不能创建会话或返回 proof |
| 域名变更与删除确认 | Root-only PUT /api/option/passkey/domains 支持不落库预览、已知/未知凭证统计和过时确认拒绝；普通 option、bulk、ServerAddress/origins 变化一起受保护 | 高：配置与凭证事务；配置锁先于用户/凭证锁，并核对数据库中的信任配置，防止旧节点缓存接受已删除域名 |
| 错误和审计 | 三种固定域名错误支持现有后端三语言；删除审计只包含域名、统计、结果及请求模板，不包含确认值/凭证/挑战 | 中：原有安全错误和审计体系继续负责返回与记录 |
| 公开状态 | Passkey 设置改为只读快照，不再公开完整受信 origins 列表 | 低至中：避免泄露私有站点列表；前端选择所需许可 RP ID 状态字段将在前端接线阶段补齐 |

配置正常/快速迁移原本都已注册 PasskeyCredential，因此沿用现有 GORM 增加可空列；没有新增表或数据库专属业务 SQL。本阶段所有生产源码变更仅在工作区。登录加密、OAuth 浏览器绑定、邀请码/微信邀请及既有认证版本控制保留；没有改动 relaykit、根 modfiles、计费或线上配置。

## 专注本地验证

- 服务端 **56 个测试函数、144 个子用例**通过，包括 17 个新增域名/旧凭证/删除确认合同、两个既有 Passkey 入口检查及相关原安全 proof/追加验证回归。实际签名覆盖三个认证路径、历史大小写、错误 RP 哈希/Origin/用户/签名/UV、期限/重放、删除后拒绝、配置快照、预览不落库、影响变化后拒绝旧确认、bulk 回滚、注册与删除序列化、旧节点缓存及新/旧结构迁移。共享认证夹具显式注册所需 Option 表；没有以缺表例外放宽生产检查。[回归](./artifacts/ui-batch-6c/backend-tests.jsonl)
- 新建独立 **127.0.0.1:3354** 服务 `new-api-ui-batch-6c` 与专属 SQLite 卷。采用不含 rp_id 的旧凭证表结构，包含两个合成用户与一个有真实软件公钥的旧 Passkey；服务实际启动迁移，没有提前在夹具中加列。复用已有镜像，无镜像构建。[环境](./artifacts/ui-batch-6c/backend-local-stage.json)
- **14 次实际 API 请求及重启后 1 次只读状态**通过：服务就绪，普通账号不能预览配置，未配置域名拒绝，历史 LOCALHOST 可选择，另一 RP 哈希签名被拒绝，新交易正确签名登录既有用户 905，重放拒绝，退出后旧会话 401，域名删除统计已知影响 1/未知 0，普通 option 路径无确认 409，审核后的确认真实保存，已删除域名不能再开始登录。[实际请求](./artifacts/ui-batch-6c/backend-runtime-calls.json)
- 一致数据库快照 quick_check=ok；新增 rp_id 为可空列，准确补写 LOCALHOST，原公钥/凭证 ID 与旧结构夹具逐字节一致，sign_count=1；仍有两个用户，恰有一个已撤销 Passkey 会话，删除配置已持久化，凭证记录仍保留。候选服务已恢复并只读确认就绪。[数据结果](./artifacts/ui-batch-6c/backend-runtime.json)
- 后端构建及 Go 格式通过，19 个本阶段文件和二进制指纹封存。[源码](./artifacts/ui-batch-6c/backend-stage-source-sha256.json)、[构建指纹](./artifacts/ui-batch-6c/backend-stage-build.json)、[构建日志](./artifacts/ui-batch-6c/backend-build.log)

最初一轮回归在旧 OAuth 测试启动 loopback 提供商时被沙箱禁止监听而中断；核实是环境限制，未修改 OAuth Cookie 夹具或生产实现，允许本地监听后重跑同一组通过。本地请求仅通过技能 api.js；合成软件认证器私钥、测试密码/令牌保留在权限 600 的本地私有文件或进程，不保存到公开证据。

本阶段二进制仍内嵌**上一批 6B 的已验证前端**，尚未装入 6C UI，不将服务端签名请求视作浏览器域名选择验收。没有前端源码变更、纯 UI/截图/布局测试。实际运行使用 loopback HTTP 和明确的本地不安全 Origin 开关，不改变生产 TLS 或 Secure Cookie 配置。

## 安全控制和剩余验证

修改前继续查阅 [OWASP Authentication](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html)、[Session Management](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html)、[MFA](https://cheatsheetseries.owasp.org/cheatsheets/Multifactor_Authentication_Cheat_Sheet.html) 及 [W3C WebAuthn](https://www.w3.org/TR/webauthn-2/)。使用服务端许可域名与保存交易单 RP ID 验签，前端提示不构成授权；敏感注册/删除仍需原 proof 与认证版本验证。

**尚需完成**：全部前端业务接线、对应文案、必要取消/域名选择/失败后记忆状态检查；独立 MySQL/PostgreSQL 的旧表重复启动迁移、凭证/唯一性保留、真实认证和删除确认事务检查；最终前后端构建与同产物运行核对。本阶段不宣称 6C 安全控制已完整验收或全站 ASVS 合规，最终报告补齐适用 ASVS 控制和验证边界。

后续只在本批候选环境推进，不部署生产。回退时仅恢复本批差异，保留前序批次、用户修改及新增字段/原凭证，不使用整体 git reset；实施前快照位于 `/tmp/ui-batch-6c-before`。
