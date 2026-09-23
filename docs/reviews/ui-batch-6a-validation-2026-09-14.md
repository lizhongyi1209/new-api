# 第 6A 批登录加密、会话和验证码验收

**本子批代码及必要本地验证已完成，未部署生产。** 第 6B/6C、第 7–8 批仍需实施。基准继续采用已核查上游 `7fd06381976cbea126d1f20973548bf60f20e37d`。

## 功能、影响和风险

| 改动 | 影响范围 | 风险与保护 |
| --- | --- | --- |
| 接通密码登录加密 | 登录表单、登录 API、公开状态类型 | 中：按后端公开开关采用已有 RSA-OAEP/SHA-256 或长密码 v2 AES-GCM 加密；开启时请求不携带 password，加密/公钥失败不回退明文、不自动重放登录；关闭时保留现有协议。登录失败清除公钥缓存，下一次手动提交重新取钥 |
| 密钥表初始化遗漏修复 | 普通及快速数据库迁移 | 中：独立环境开启加密时实际启动失败，原因是已有 LoginEncryptionKey 模型未注册迁移。现加入两条 GORM AutoMigrate 路径，缺表时创建 login_encryption_keys；沿用原模型与单槽持久化协议，没有重置既有密钥 |
| 登录页和受保护页面会话判定 | sign-in 和 authenticated 路由守卫 | 中：使用现有 bootstrapAuthentication 的有效 bundle/服务端刷新结果判定，不再只看残留 user 或 token；登录后回跳复用同源地址校验。保留现有跨标签刷新协调、SID 不匹配恢复和退出保护；本地公开启动路径原本已刷新会话，无需移植上游新增提示 Cookie 才能实现恢复 |
| 验证码状态 | 登录、注册发送邮件验证码、共享 Turnstile | 低至中：登录原有提交后重置保留；邮件验证码实际请求前消耗本次状态，失败也要求新挑战，未通过校验不消耗；过期/错误清空令牌，替换或卸载后的旧回调不能写入新状态，共享脚本加载中挂载的组件可正确订阅完成事件 |
| 登录和验证码失败提示 | 密码登录、发送邮件验证码 | 低至中：接入现有统一错误处理，业务失败及传输失败可见，复用提示归属避免重复。原请求层已不再统一通知普通传输错误，因此不能继续静默 catch |

保留邀请码必填、链接邀请码、微信邀请关联、现有 OAuth/MFA/Passkey 流程和本地会话安全控制。没有新增前端依赖、翻译键或管理员开关；没有自动开启生产密码加密。共享验证码组件的已有调用方会在过期时得到空令牌，这是本次期望行为。

## 专注验证与证据

- 独立加密开启环境：**127.0.0.1:3342**，容器 `new-api-ui-batch-6a-v2`，独立 SQLite 卷。加密关闭兼容环境：**127.0.0.1:3346**，容器 `new-api-ui-batch-6a-compat`，另一个独立卷。最终二进制内嵌本批前端，两者可直接打开本地页面；复用已有 Docker 镜像，没有镜像构建或生产操作。
- 开启环境 **10 次实际 API 请求**：公开状态和公钥、明文拒绝、失效 key ID/损坏密文/错误密码拒绝、加密登录成功、用户身份读取、退出及旧访问令牌 401。请求只通过技能 api.js，合成凭证保存在权限 600 的本地文件；报告不包含密码或可用令牌。[实际请求](./artifacts/ui-batch-6a/runtime.json)
- 最终构建 **8 次实际请求**：重启前后公钥标识一致、数据库密钥表恰有一个持久化槽；关闭环境返回 disabled、原明文协议登录成功，退出后令牌被拒绝。启动修复验证来自实际缺表数据库，没有手工预建密钥表。[最终运行检查](./artifacts/ui-batch-6a/final-runtime.json)
- 前端 **7 个文件、39 项专注检查**通过：真实加解密往返、长 Unicode 密码、forge 兼容加密、公钥失败不提交、失败后取新钥、验证码消耗/过期/旧回调/共享加载、登录与保护路由结果、同源回跳及既有会话协调/退出合同。[前端检查](./artifacts/ui-batch-6a/focused-tests.log)
- 后端 **7 个测试函数、3 个子测试**通过：长密码加密登录与篡改拒绝、退出 Cookie/SID 不匹配、创建/刷新/撤销、认证版本失效、刷新竞态与宽限后重用撤销、单会话撤销，以及绝对过期前/当时/之后的边界。过期拒绝不能改写刷新摘要或延长绝对期限。[后端检查](./artifacts/ui-batch-6a/backend-tests.jsonl)
- 类型、12 个涉及前端文件的 lint/保护版权块格式、前后端构建通过。最终源码与构建指纹已封存；根 modfiles 和 relaykit 未修改；没有扩展纯 UI 或视觉测试。[源码和构建指纹](./artifacts/ui-batch-6a/source-manifest.json)、[构建](./artifacts/ui-batch-6a/build.log)、[lint](./artifacts/ui-batch-6a/lint.log)、[格式](./artifacts/ui-batch-6a/format.log)

实际数据库验证为隔离 SQLite；MySQL/PostgreSQL 未执行本批集成测试。迁移只注册已有跨数据库 GORM 模型，没有新增方言 SQL。快速迁移路径已注册，实际服务启动验证使用普通迁移路径。

## 适用 OWASP 控制与验证边界

修改前查阅 [Authentication Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html)、[Session Management Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html) 及稳定版 ASVS 5.0.0，采用以下与本批改动相关的控制：

- **v5.0.0-6.2.8、6.2.9**：密码原值和长密码兼容，通过实际加解密与服务端长 Unicode 登录验证；没有截断、改大小写或增加密码组成限制。[ASVS Authentication](https://github.com/OWASP/ASVS/blob/v5.0.0/5.0/en/0x15-V6-Authentication.md)
- **v5.0.0-6.3.1**：登录与邮件验证码保留服务端限流和 Turnstile 检查；前端令牌状态只用于交互，不能授权登录。本批验证状态失效和单次请求边界，没有调用真实 Cloudflare 验证服务或宣称完成抗自动化测试。
- **v5.0.0-7.2.1、7.3.2、7.4.1**：后端验证会话、绝对期限不被刷新延长、退出后拒绝旧会话。实际退出请求与服务/模型回归共同验证；登录路由根据刷新结果决定页面，不替代服务端授权。[ASVS Session Management](https://github.com/OWASP/ASVS/blob/v5.0.0/5.0/en/0x16-V7-Session-Management.md)
- **v5.0.0-12.2.1**：额外密码加密不能替代 HTTPS。本批 HTTP 仅绑定 loopback 的隔离测试服务，没有改变生产 TLS、Secure Cookie 配置，也未执行生产 TLS 验收。[ASVS Secure Communication](https://github.com/OWASP/ASVS/blob/v5.0.0/5.0/en/0x21-V12-Secure-Communication.md)

现有刷新 Cookie 的 HttpOnly/SameSite/路径和服务端会话逻辑已核对。api.js 无 Cookie/自定义请求头能力，因此实际请求脚本**不证明浏览器刷新 Cookie 的端到端传输**；刷新和 Cookie/SID 不匹配由现有后端及前端协调合同检查覆盖。登录审计沿用仅记录非秘密方法、用户和客户端上下文的实现。报告不宣称全站 ASVS 合规，生产 TLS、真实验证码及 MySQL/PostgreSQL 集成不在本次本地验证结果中。

## 回退和下一批

回退本子批具体差异，保留前序更新和用户修改。新增密钥表属于内部安全存储；回退应用时保留表和既有密钥，不删除或重新生成，避免其他已升级副本失去解密能力。若生产后来开启加密，回退前端须同时保证前后端登录协议一致，不能只撤销前端接线。

下一子批为 **6B：OAuth 策略模板、Chat 客户端配置与凭证请求时机**，继续独立本地验证，不部署生产。
