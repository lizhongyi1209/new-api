# 第 6B 批 OAuth、Chat 配置与凭证请求验收

**本子批代码及必要本地验证已完成，未部署生产。** 第 6C、第 7–8 批尚未完成。对照继续使用已全面核查的上游 `7fd06381976cbea126d1f20973548bf60f20e37d`。

## 功能、影响和风险

| 改动 | 影响范围 | 风险与保护 |
| --- | --- | --- |
| OAuth 访问策略和拒绝提示模板 | 管理员自定义 OAuth 表单 | 低：增加等级且活跃、组织或角色两种模板及对应拒绝提示示例；复用现有按钮、JSON 编辑器和保存接口，只写入当前表单草稿，不覆盖既有提供商策略 |
| 策略缺失字段与数字比较修复 | 通用 OAuth 服务端 userinfo 授权 | 中至高：实际验证发现缺失 trust_level 能通过原比较；现在缺失字段默认拒绝，只有明确 not_exists 允许缺失；有序比较要求两侧数字或两侧字符串，拒绝非数值、NaN/Inf 和混合类型，保留有效数字字符串及字符串比较 |
| OAuth 登录浏览器绑定 | 标准 OAuth 登录 state 创建和回调，包括 Telegram | 中至高：实际验证发现无浏览器 Cookie 也能完成登录；现在随机 state 对应的已有 auth-flow payload 保存浏览器随机 Cookie 的 SHA-256 摘要，回调在联系提供商前验证。Cookie 为 host-only、HttpOnly、SameSite=Lax、10 分钟；secure 模式使用 Secure 和 __Host- 前缀，并对 state 创建采用已有受信 Origin 守卫。绑定和敏感验证继续校验用户、SID、操作上下文及 proof |
| OAuth 初始化失败提示 | 登录前退出、各 OAuth 登录入口 | 低至中：退出失败不会清除身份或继续创建交易，业务错误经统一安全错误处理显示，保留 Telegram 配置检查、邀请码及原提供商参数 |
| AQBot 配置导入 | Chat 客户端链接解析 | 低至中：接通 {aqbotConfig}，按客户端协议编码名称、服务地址、密钥与 openai 类型；保留 Cherry、AionUI、DeepChat 的既有配置协议 |
| 凭证获取时机与身份隔离 | Chat2Link、外部 Chat 入口、当前用户密钥查询 | 中：无需密钥的预设不读取密钥；需要时才获取已有启用令牌，查询开始及两个等待边界核对用户/SID，防止退出或切换身份后的旧结果用于导入。手机入口原本已在用户操作时获取，无需重复修改 |
| 在线测试新鲜认证 | Playground 流式及普通生成请求 | 中：请求前使用现有认证预检；普通生成配置 single-use authorization，401 不自动重放可能计费的 POST；保留取消与旧请求隔离 |

本批没有数据库模型、表结构迁移、新 SQL、计费算法或新前端依赖。浏览器绑定摘要复用 auth_flows 的已有 payload。已有未绑定的待处理 OAuth 登录交易需重新发起，最长涉及原 10 分钟有效期；缺失字段、非法数字或混合类型的旧策略可能从放行变为拒绝，应检查提供商真实 userinfo。secure 模式沿用现有 SESSION_COOKIE_SECURE 与受信来源配置，没有自动修改生产开关。

## 本地环境与专注验证

- **127.0.0.1:3348**：`new-api-ui-batch-6b`，独立 SQLite 卷与受控合成 OAuth 提供商。**127.0.0.1:3352**：`new-api-ui-batch-6b-secure`，另一个独立卷，开启现有 Secure Cookie 配置。两者安装同一个最终二进制，选择 default 主题；复用已有镜像，没有 Docker 镜像构建。[最终服务指纹](./artifacts/ui-batch-6b/verified-local-stage.json)
- 前端 **7 个文件、27 项业务检查**通过：客户端导入编码/旧协议、无密钥入口不查询、停用密钥过滤、用户/SID 切换拒绝、流式等待认证、取消保护、普通 POST 不重放，以及 OAuth 退出失败和提供商参数。[前端检查](./artifacts/ui-batch-6b/focused-tests.log)
- 后端 **17 个测试函数、55 个子用例**通过：17 种真实 GetUserInfo 策略输入、浏览器 Cookie 缺失/错误/畸形/旧交易/过期、错误回调不能消耗其他浏览器交易、正确绑定登录/单次消费、Cookie 属性/并行待处理交易、受信 Origin、原登录追加验证，以及 Telegram PKCE/nonce、身份保留、过期/重放、绑定会话与并发归属。[最终后端检查](./artifacts/ui-batch-6b/backend-tests.jsonl)
- 初始策略阶段 **29 次实际 API 请求**验证策略持久化、管理员权限、对象密钥归属、三个拒绝资料和三个允许资料、提供商错配与已消费 state 拒绝，只有允许资料创建了 3 个测试会话，均已撤销。该阶段暴露了未绑定浏览器的安全缺口，不能作为最终浏览器绑定证明。[初始请求](./artifacts/ui-batch-6b/runtime-calls.json)、[初始数据结果](./artifacts/ui-batch-6b/runtime.json)
- 增加浏览器绑定后 **8 次实际 API 请求**验证候选服务启动、策略保存、无发起浏览器 Cookie 回调 403，以及 secure 环境无受信 Origin 创建交易 403。[最终安全请求](./artifacts/ui-batch-6b/final-runtime-calls.json)
- 最终内嵌 default 前端通过**一次实际浏览器 OAuth 登录业务验证**：正常接受本地协议，创建 state 和 HttpOnly Cookie，模拟受控提供商导航；另一全新浏览器访问同一回调被 403 拒绝，原浏览器成功登录已有用户 905 并离开回调页。受控提供商总计仅 1 次换 token、1 次 userinfo；随后撤销测试会话并禁用提供商。没有截图、布局或纯 UI 断言。[浏览器流程及清理](./artifacts/ui-batch-6b/browser-auth-runtime.json)
- 最终一致数据库快照 quick_check 均为 ok：普通环境仍有 **2 个用户、1 个既有 OAuth 绑定、4 个已撤销会话**，4 个已消费交易、4 个未消费交易；没有拒绝资料或另一浏览器新增用户。secure 环境 0 个交易、0 个会话。两个候选服务已恢复运行，同二进制且提供商不公开启用。[数据库及最后只读状态](./artifacts/ui-batch-6b/verified-database-runtime.json)

手动 New API 请求均使用技能 api.js，合成凭证只在本地权限 600 的私有文件或进程中使用；结果不保存 Cookie、state、可用密钥或认证令牌。密钥读取只验证已有合成令牌，没有创建后重新列出/取回密钥。前期脚本解析 token POST 的附加提示中断后，以已持久化的审计证据确认成功，没有重复密钥读取。

类型、15 个涉及前端文件的 lint/保护版权块格式、i18n 及前后端构建通过。源码 30 个文件与最终构建已封存，根 go.mod/go.sum 和 relaykit 未修改。[构建指纹](./artifacts/ui-batch-6b/build.json)、[源码指纹](./artifacts/ui-batch-6b/source-sha256.json)、[类型](./artifacts/ui-batch-6b/typecheck.log)、[lint](./artifacts/ui-batch-6b/lint.log)、[格式](./artifacts/ui-batch-6b/format-check.log)、[翻译与版权块](./artifacts/ui-batch-6b/i18n-and-headers.json)

## 本地构建问题及证据区分

浏览器验证最初发现 HTML 引用新 JS，而本地 dist/static 仍是旧产物。该生成目录由 UID 65534 拥有，构建无法写入；构建退出成功不足以证明前端可启动。旧静态目录已完整备份至 `/tmp/ui-batch-6b-static-before`，仅将该生成目录所有者调整到工作区用户，再按原配置重建。诊断期间尝试的哈希设置已完全撤销，rsbuild.config.ts 与本批开始前逐字节一致。

最终 **239 个新产物**封存于 `/tmp/newapi-ui-batch-6b-verified-dist`，6 个入口资源均存在且实际浏览器成功加载；初始含残留文件的 349 个产物及其日志/指纹改为 phase1-* 标记，不能当作最终前端健康证明。此修复只针对本地生成文件权限，没有改动线上文件、生产构建配置或静态服务代码。[入口资源](./artifacts/ui-batch-6b/build-entry-assets.json)、[最终资源指纹](./artifacts/ui-batch-6b/verified-frontend-dist-sha256.json)、[最终前端构建](./artifacts/ui-batch-6b/build.log)、[最终内嵌构建](./artifacts/ui-batch-6b/backend-build.log)

## OWASP 控制与验证边界

修改认证路径前已查阅 [Authentication Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html)、[Session Management Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html)、[OAuth2 Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/OAuth2_Cheat_Sheet.html) 及 [Authorization Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Authorization_Cheat_Sheet.html)，采用稳定 ASVS 5.0.0：

- **v5.0.0-8.2.1、8.2.2、8.3.1**：提供商写权限、令牌对象归属与策略均由服务端执行；实际拒绝请求和真实 userinfo 策略合同覆盖改动。[ASVS Authorization](https://github.com/OWASP/ASVS/blob/v5.0.0/5.0/en/0x17-V8-Authorization.md)
- **v5.0.0-10.1.2、10.2.1**：随机 state 与用户代理绑定，回调验证浏览器 Cookie，secure 模式创建交易检查受信来源；覆盖错误/过期/未绑定/跨浏览器/重放以及正常登录。现有 Telegram 的 PKCE/nonce 和其负例仍通过。[ASVS OAuth/OIDC](https://github.com/OWASP/ASVS/blob/v5.0.0/5.0/en/0x19-V10-OAuth-and-OIDC.md)
- **v5.0.0-10.2.2**：交易匹配提供商与意图，混用提供商或已消费交易不进入换 token；原绑定/敏感验证继续绑定认证 SID、操作和 proof。

本批本地运行是 loopback HTTP。实际验证了普通浏览器 Cookie 往返与跨浏览器拒绝；Secure/__Host- 属性及 Origin 拒绝由后端合同和独立 secure 环境验证，**没有验证生产 HTTPS 下 Secure Cookie 的真实浏览器传输**。也没有接入真实外部 OAuth 提供商、Cloudflare、执行生产 TLS 验收或完整浏览器刷新会话验收。SQLite 数据核对已完成，MySQL/PostgreSQL 本批未做集成执行；本批没有模型/迁移/方言 SQL。日志与报告仅保留非秘密方法、结果和对象上下文，不宣称全站 ASVS 合规。

## 回退和后续

仅回退本批具体差异，保留用户原修改及前序批次；前序文件快照在 `/tmp/ui-batch-6b-before`，不得整体 git reset。回退 OAuth 服务端浏览器绑定会恢复已发现的安全缺口，需作为单独风险决定；没有表结构回退或删除凭证数据的需要。原本待处理的未绑定交易重新发起即可，不修改现有账号绑定。

下一子批 **6C：多 Passkey 域名、登录/敏感验证域名选择、删除影响确认**，先确定旧凭证 RP ID 兼容和三数据库迁移，再实施并使用独立本地环境验证。
