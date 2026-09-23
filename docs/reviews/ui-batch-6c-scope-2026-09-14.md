# 第 6C 批多域名 Passkey 实施前核查

> 本文保留实施前/服务端阶段的历史状态；后续前端、三数据库和最终候选服务验收已完成，当前结果见 [6C 最终验收](./ui-batch-6c-validation-2026-09-14.md)。

**当前为实施前核查，尚未完成本批代码或验收。** 第 6B 已完成，见 [6B 验收](./ui-batch-6b-validation-2026-09-14.md)。继续对照既定上游 `7fd06381976cbea126d1f20973548bf60f20e37d`，不会调整到另一个更容易实现的功能范围。

## 已确认的缺口和实施合同

| 内容 | 当前版本 | 本批要接通的行为 | 影响与风险 |
| --- | --- | --- | --- |
| 多个 Passkey RP ID | 只有一个 rp_id | 主域名与兼容域名列表；域名校验、旧域名保留，按受信 Origin 限制可选择项 | 高：错误信任配置会导致登录失败或错误接受签名来源 |
| 凭证归属域名 | passkey_credentials 没有 rp_id | 增加可空 varchar(253) 字段；新注册保存 RP ID，旧记录不猜测归属，只在成功签名验证后补写 | 高：涉及凭证模型和三数据库迁移；不能重新生成或删除旧凭证 |
| 主登录、追加登录验证、敏感操作验证 | 全部调用单域名 BuildWebAuthn | begin 从服务端许可域名中选择；已知凭证 RP ID 优先于浏览器记忆；finish 只能用已保存交易的 RP ID，继续验证 challenge、Origin、UV、期限、用户/SID/操作上下文 | 高：三条认证路径都必须接通，不能只增加登录页选择器 |
| 登录与管理前端 | 没有域名选择或成功域名记忆 | 登录及验证使用共享选择行为；成功后才记忆；不可用域名给出原站点或替代验证方式提示 | 中至高：选择提示不能绕过服务端规则，不自动反复尝试同一签名 |
| 删除兼容域名 | 普通 option 保存，无凭证影响预览 | 服务端统计已知受影响凭证及未知旧凭证；预览不落库，确认与变更内容/当前配置/统计绑定，过时确认拒绝；保存保持事务原子性 | 高：配置与凭证写入并发时必须重新核对，不能依赖前端确认 |
| 隐式域名变化 | GetPasskeySettings 会将 ServerAddress 默认写入共享设置 | 独立设置快照；ServerAddress、origins 变化同样经过域名合同，保留适用旧信任配置 | 高：只保护 rp_id 表单会留下另一条域名修改入口 |

本地普通与快速迁移原本都已注册 PasskeyCredential；新增字段应沿用 GORM 跨数据库新增可空列，不添加有差异的布尔默认值或数据库专属 SQL。旧凭证必须保留原 RP ID 哈希输入的精确值，不能通过大小写或 IDNA 标准化改写历史签名合同。新的管理员输入可以校验/规范化。

当前已核对 `controller/passkey.go`、`controller/login_verification.go`、`service/passkey/service.go`、`service/passkey/session.go`、`model/passkey.go`、`setting/system_setting/passkey.go`，以及认证设置/登录/安全管理的前端入口。特别确认追加登录验证的 `/api/user/login/passkey/*` 也缺少 rp_id，不会遗漏。上游 `model/passkey_option.go` 已提供删除预览、确认新鲜度与配置/凭证事务序列化合同，需要按本地 option、安全 proof 和用户定制接入。

## 实施和验证顺序

1. 接通设置快照、许可域名解析与 WebAuthn 单交易域名选择；模型新增可空 RP ID，并保持注册/断言更新的身份和认证版本合同。
2. 接通全部三条认证路径、option 单项/批量更新及删除预览/确认，保存与凭证写入采用一致事务锁顺序。
3. 前端复用现有 SettingsForm、Select、ConfirmDialog 和安全验证流程；按已有 i18n 技能补齐文案，保留邀请码、微信邀请和已有追加验证。
4. 独立本地候选服务验证新/旧凭证登录及敏感操作，使用合成认证器/签名，不操作线上用户。三数据库核对新增列与旧数据、注册/补写、删除确认及事务原子性；现有测试数据库资源需先确认归属后使用独立数据库。
5. 必要负例集中在错误域名/Origin/签名、挑战过期和重放、跨用户/SID/操作、已删除域名与过时确认。前端只检查选择如何影响业务请求及失败后状态，不做布局、截图或纯视觉测试。

修改前继续采用 [OWASP Authentication](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html)、[Session Management](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html) 和 [MFA 指南](https://cheatsheetseries.owasp.org/cheatsheets/Multifactor_Authentication_Cheat_Sheet.html)，并按 [W3C WebAuthn](https://www.w3.org/TR/webauthn-2/) 核对 RP ID/Origin、challenge 与用户验证合同。安全控制由服务端执行，不以域名下拉框代替验签。本文件只记录实施要求，不能作为安全控制已实现或验证通过的证据。

26 个当前入口/翻译文件的实施前快照及 SHA-256 已保存至 `/tmp/ui-batch-6c-before`；13 个上游合同文件保存至 `/tmp/ui-batch-6c-audit`，用于保留前序批次和用户改动。该备份过程未修改业务源码。此阶段尚未启动 6C 候选服务，尚未新增字段或修改数据库。
