# 上游 UI 页面核查与分批评估（2026-09-13）

本轮完成源码层面的页面、导航、设置子页与交互依赖核查，未修改应用代码、配置、数据库或部署。本文用于确认后续实施范围。

## 对比基准与核查边界

| 项目 | 核查基准 |
| --- | --- |
| 当前代码及运行容器版本 | `custom` / `6cb299e801e902dacffffbd7c42006a736deddb6`；容器 `new-api` 为 running / healthy，revision 与代码一致 |
| GitHub 最新发布 | [v1.0.0-rc.37](https://github.com/QuantumNous/new-api/releases/tag/v1.0.0-rc.37)，2026-09-11 发布，提交 `385d2dfd10d821b25c8a6766bd16eea248cb1652` |
| GitHub main 最新代码 | `7fd06381976cbea126d1f20973548bf60f20e37d`；本轮通过远端查询重新确认 |
| 前端路径对应 | 当前 `web/default/src/` ↔ 上游 `web/src/`；去除路径变化和引号、分号、空白差异后检查实际功能差异 |
| 页面覆盖 | 当前 65 个路由文件，上游 62 个路由文件；逐项对照导航、页面入口、弹窗/抽屉、桌面/移动分支，并核查当前 41 个系统设置 section |
| 运行验证边界 | 本轮为源码与容器版本核查，未逐页进行浏览器点击、截图或生产写入测试；“已有”表示源码中有可达入口及实现，不代表本轮重新执行全部业务流程 |
| 经典主题 | 当前存在 `web/classic` 及主题切换；本轮新增功能对照以默认主题为准。经典主题兼容及主题切换必须在实施时回归，不把默认主题结果当作经典主题已验证 |

路由文件包含布局、别名和重定向，数量不等于独立页面数量。上游路由较少并不说明当前缺少更多页面；上游独有的新页面入口是任务插件。

状态说明：**已有**＝核心能力已存在；**部分**＝已有基础功能，上游有扩展；**缺少**＝当前缺少对应入口或实现；**定制差异**＝当前能力与上游不同，需要保留现有业务行为。格式化、文件拆分、测试新增不计作缺失功能。

## 全部页面核查表

“批次”只表示规划归属，不代表已开始实施。跨页的统一错误提示列入第 3 批，涉及登录或安全的部分随第 6 批独立验证。

| 页面 / 入口 | 当前状态 | 上游新增或不同之处 | 规划批次 |
| --- | --- | --- | --- |
| 首页 `/` | 部分 / 定制差异 | 上游默认首页显示统计、功能介绍、使用流程；当前这些组件存在但未全部挂载。上游自定义首页 iframe 支持用户点击后跳转顶层页面。是否恢复默认展示需保留当前首页意图 | 3，可选首页项 |
| 关于 `/about` | 已有 | 内容读取的浏览器 ETag 重验证及失败提示改进；未发现独立新业务功能 | 3 |
| 隐私政策、用户协议 | 已有 | 公共文档 ETag 重验证改进；保留现有内容 | 3 |
| 排行榜 `/rankings` | 已有 | 请求失败处理改进；未发现独立新页面功能 | 3 |
| 初始化 `/setup` | 已有 | 初始化失败展示改进；不重新执行生产初始化 | 6，错误处理部分 |
| 登录 `/sign-in` | 部分 | 多 Passkey 域名选择、记住兼容域名；密码登录按后端开关调用已有加密能力；已有会话访问登录页的判定改进 | 6 |
| 注册 `/sign-up`、`/register` | 已有 / 定制差异 | 上游发送验证码后重置 Turnstile 状态；当前邀请码必填、链接邀请码和微信邀请关联为定制，不能被上游简化逻辑删除 | 6 |
| 忘记密码、重置密码 | 已有 | 失败提示改进；未发现全新的找回流程 | 6 |
| OTP 登录验证 `/otp` | 已有 | 失败提示改进；保留当前统一验证流程 | 6 |
| OAuth 回调、绑定回调 | 已有 / 定制差异 | 错误处理、会话恢复改进；保留现有绑定窗口及邀请关联 | 6 |
| 概览 `/dashboard/overview` | 部分 | 完成设置向导后移到页头展开入口，改进折叠后的布局、焦点返回；现有向导及完成状态已有 | 3 |
| 模型调用分析 `/dashboard/models` | 已有 | 查询失败处理改进；现有性能概览保留 | 3 |
| 流量分析 `/dashboard/flow` | 已有 | 查询失败处理改进；未发现新的分析页面 | 3 |
| 用户分析 `/dashboard/users` | 已有 | 查询失败处理改进；保持管理员权限 | 3 |
| API 密钥 `/keys` | 部分 | 额度详情弹层、创建/最近使用合并时间展示、继承用户分组说明、移动卡片展示模型/IP 限制、搜索防抖。密钥管理及原有额度进度/提示已存在 | 1A，部分完善项 3 |
| 普通日志 `/usage-logs/common` | 部分 | 移动端紧凑卡片/分页、公共可折叠筛选、月份快捷范围、按 self/admin/root 隔离查询和列偏好；图片缓存/按次/任务用量明细需要计费数据支持 | 1A、3、5、7 |
| 绘图日志 `/usage-logs/drawing` | 已有 / 部分 | 基本预览已有；共享筛选和布局改进 | 1A、3 |
| 任务日志 `/usage-logs/task` | 部分 | 结构化详情；统一多产物预览、下载及失败重试；插件身份/版本/运行诊断列。现有视频/音频预览、错误信息及管理员任务审计接口已有 | 1A 基础详情；7 完整产物/插件 |
| 操作审计 `/usage-logs/audit` | 已有 | 失败提示改进；不能误列为缺少审计功能 | 3 |
| 钱包 `/wallet` | 部分 | Waffo 付款方式先计算金额、再进入统一确认；订单搜索防抖及旧响应防覆盖；无有效订阅时更明确地区分 subscription_only 与 subscription_first 后果 | 4 |
| 推荐计划 `/referrals` | 定制差异 | 当前独立推荐计划及导航配置是本地能力，上游部分配置删除 referral 项；不按缺失处理 | 保留 |
| 个人资料 `/profile` | 已有 | 语言、通知、签到等失败处理改进；保留推荐计划相关配置 | 3 |
| 安全与访问 `/security` | 部分 | 会话、访问令牌、2FA、Passkey 管理已有；多 RP ID 的验证选择缺少 | 6 |
| 在线测试 `/playground` | 部分 | 回复及思考内容流式渐显；取消/切换请求后旧回调不覆盖新回复；批量刷新文本减少渲染；消息编辑使用站内离开确认及刷新丢失提醒 | 3，凭证刷新部分 6 |
| Chat、Chat2Link | 部分 | 新增 `{aqbotConfig}` 客户端配置链接；移动导航避免提前获取包含凭证的聊天链接。已有聊天预设和其他客户端链接保留 | 6 |
| 渠道 `/channels` | 部分 / 定制差异 | 提供商选择、统一分页配置、内联模型发现、模型选择分类、模型映射辅助、内置 Base URL 占位提示；插件参与提供商/模型选择另有后端依赖 | 2；插件部分 7 |
| 模型元数据 `/models/metadata` | 已有 | 厂商关联、元数据同步/冲突处理、模型广场可见性状态/筛选等已有；主要增量是错误处理、表单必填标记 | 3 |
| 厂商 `/models/vendors` | 已有 | 管理、合并及关联模型已有；主要为错误提示改进 | 3 |
| 模型部署 `/models/deployments` | 已有 | 名称检查、预估费用、搜索防抖和失败展示改进；未发现新的部署入口 | 3 |
| 用户 `/users` | 部分 | 额度详情弹层及余额/累计使用更清楚的展示；用户绑定、重置安全因素已有 | 1A；安全错误处理 6 |
| 兑换码 `/redemption-codes` | 已有 | 批量删除、文件导出已存在；表格和错误提示改进。保留只改名称时不改变精确额度的逻辑 | 3 |
| 订阅 `/subscriptions` | 已有 | 管理/购买/用户订阅已有；错误处理及表单标记改进 | 3；支付相关验证 4 |
| 上传管理 `/upload-management` | 定制能力 | 当前已有本地上传清单与清理界面，上游没有对应新入口 | 保留 |
| 系统信息 `/system-info` | 已有 | 没有过期实例时隐藏批量清理按钮；任务/实例加载失败状态改进 | 3 |
| 模型广场 `/pricing` | 部分 | 24 小时逐小时健康条、卡片重排、宽度/断点调整、移动价格模式与 K/M 切换、显式零价展示、搜索防抖；任务计费筛选与多提供商价格另有依赖 | 1B；3、5、7 |
| 模型详情 `/pricing/$modelId` | 部分 | 当前时间价格预览、复杂条件展示、文本/图片缓存分栏、按次/图片混合计费、用量示例、多插件提供商标签 | 5；插件部分 7 |
| 系统设置：站点与品牌 | 部分 / 定制差异 | Async Task Public Address 新设置缺少；当前默认/经典主题切换及备案页脚必须保留 | 8 |
| 系统设置：认证 | 部分 | 多 Passkey 域名及删除影响预览/确认；自定义 OAuth 访问策略和拒绝提示模板 | 6 |
| 系统设置：计费与支付 | 部分 | 定价币种输入、旧价格转表达式草稿/预览、缓存模式、增强条件编辑/请求模拟、任务用量价格矩阵、按插件定价 | 5；支付 4；插件 7 |
| 系统设置：模型与路由 | 部分 | 自动检测新增仅检测开启自动禁用渠道模式、1–32 并发设置；模型名排除规则的新语义/示例；Codex 亲和性请求头模板扩充 | 8；定价相关 5 |
| 系统设置：安全与限制 | 已有 / 定制差异 | SSRF、限流、敏感词、令牌限制已有；R2 公共上传白名单是本地定制，上游删除不适用于当前 | 保留；公共表单改进 3 |
| 系统设置：控制台内容 | 已有 | 公告、FAQ、API 信息、Kuma、聊天、绘图配置已有；主要为失败提示改进 | 3 |
| 系统设置：运维 | 部分 | 管理员更新提醒、忽略特定版本、统一版本详情；性能/清理接口失败处理完善 | 1A、3 |
| 任务插件 `/task-plugins` | 缺少 | 管理、上传/URL 导入、版本启用/禁用、源码差异、完整性检查、市场/来源配置、详情/本地化更新日志、沙箱、运行状态 | 7 |
| 错误页 401/403/404/500/503 | 已有 | 未发现独立新增页面能力；统一失败处理不能破坏当前错误路由 | 3 |
| 旧地址兼容 `/console/*`、`/login` 等 | 部分 | 上游统一旧地址映射覆盖渠道、令牌、模型、订阅、设置 tab 等；当前仅部分别名/跳转，需保留 query/hash 和主题差异 | 3，登录地址随 6 |

## 系统设置子页覆盖清单

以下来自当前 section registry，对比上游同名 registry。不能只核查七个设置大类的首页。

| 大类 | 已核查的当前 section ID | 增量位置 |
| --- | --- | --- |
| 站点与品牌（4） | system-info、notice、header-navigation、sidebar-modules | system-info 的任务媒体公共地址；保留主题切换与 referral 定制 |
| 认证（5） | basic-auth、oauth、passkey、bot-protection、custom-oauth | passkey、多域名验证；custom-oauth 的策略模板 |
| 计费与支付（6） | quota、currency、model-pricing、group-pricing、payment、checkin | model-pricing 的编辑器/计费扩展；payment 的付款确认改进 |
| 模型与路由（7） | global、routing-reliability、gemini、claude、grok、channel-affinity、model-deployment | global 规则语义、routing-reliability 检测模式/并发、channel-affinity 模板 |
| 安全与限制（5） | r2-upload-whitelist、rate-limit、sensitive-words、ssrf、token-limits | 保留本地 R2 白名单；其他没有独立新增 section |
| 控制台内容（7） | dashboard、announcements、api-info、faq、uptime-kuma、chat、drawing | 公共错误处理 |
| 运维（7） | behavior、alerts、email、worker、logs、performance、update-checker | update-checker 更新提醒；logs/performance 失败状态 |

## 批次、影响范围与风险

风险等级是按当前定制代码进行的工程评估，不是故障概率承诺。每一批单独形成可审查代码、验证结果和回退说明，再安排部署。

| 批次 | 具体范围 | 影响范围 / 数据变化 | 风险及主要原因 |
| --- | --- | --- | --- |
| 1A：原第一批核心 | 管理员更新提醒；用户/API 密钥额度详情；渠道/日志移动筛选；任务基础结构化详情 | 默认前端 header、users、keys、channels、usage-logs；复用当前数据及管理员任务审计；预计无需数据库迁移 | **低至中**：公共组件影响多个页面，详情必须遵守现有接口权限，更新提醒需识别定制版本而非武断判断已包含某发布全部功能 |
| 1B：模型广场补充，需确认新增范围 | 卡片/布局/响应式；手机价格模式/K/M；显式零价；24 小时健康条 | pricing、performance-metrics 类型、pkg/perf_metrics 汇总接口；保留旧成功率字段并新增带时间戳序列；预计无需表结构变化 | **低至中**：布局/零价低；健康条涉及聚合窗口、时区对齐、空流量及查询成本，需验证后端兼容 |
| 2：原第二批核心 | 非插件提供商选择；统一配置分页；内联发现和分类选模；映射辅助；Base URL 占位提示 | 渠道创建/编辑抽屉、表单序列化、模型发现/管理请求；若当前 API 不覆盖草稿发现则补最小接口；预计无需数据库迁移 | **中**：表单保存可能改变实际渠道配置，必须保留所有自定义字段、已有密钥、类型编号和显式零/false；发现结果由用户选择，不自动覆盖保存 |
| 3：全站交互与在线测试 | 抽屉内弹层交互；列显隐刷新；移动卡片/紧凑分页/活动时间；错误提示去重与真实失败状态；概览向导；在线测试取消/编辑/流式显示；公共文档缓存；旧地址兼容；首页展示可选 | 多页共用 components、QueryClient、http-client；playground、dashboard、内容页等。预计无需迁移；登录/验证/凭证路径的改动拆到第 6 批 | **中**：影响面广；统一错误处理需要逐接口保留业务错误，流式取消要防旧回调污染新会话，公共缓存只允许非用户特定内容 |
| 4：钱包与支付确认 | Waffo 金额计算/确认；订单搜索防抖及旧响应防覆盖；订阅偏好后果说明 | wallet、Waffo 现有 amount/payment API、订阅展示；当前后端已有 Waffo amount 接口；预计无需表结构变化 | **中**：付款方式索引、确认金额及重复提交可能影响付款；保留当前计算并发保护、余额上限提示，不能整文件覆盖 |
| 5A：定价输入及只读展示 | 输入币种选择/换算；编辑滚动布局；受支持表达式的条件/缓存展示；当前时段只读预览 | model-pricing、系统价格编辑页、pricing/详情；复用当前配置存储；币种仅作用于输入/显示，表达式仍按现有 USD 合约；预计无需迁移 | **中**：精度、价格往返、无意覆盖旧值、预览与实际结算一致性；当前时间不能用于重算历史日志 |
| 5B：计费行为扩展 | 旧价转换草稿与生效价格预览；可视条件树/完整请求模拟；fixed()、图片缓存及混合计费；日志用量明细；内置图片表达式默认值 | API、billingexpr、价格配置、预扣/结算/退款、异步任务快照及日志展示；转换接口需剥离上游插件依赖；是否迁移按最终兼容实现确定 | **高**：实际扣费路径及异步任务兼容；缓存 token 排除、按次单位、请求倍数、饱和保护和管理员覆盖必须全部保持正确 |
| 6：登录与安全 | 多 RP ID/域名选择及删除影响确认；接通现有密码登录加密；登录页会话恢复；验证码状态修复；OAuth 策略模板；Chat 客户端配置及凭证请求时机 | auth/security/自定义 OAuth、会话/验证接口、Passkey 配置、凭证字段；多 RP ID 预计增加可空 RPID 字段，需要三数据库迁移验证；其他子项可进一步拆开 | **中至高**：可能造成登录失败、旧 Passkey 无法使用或敏感操作验证绕过；按 OWASP 指导做失败/过期/重放/绕过测试，保留邀请码和微信邀请定制 |
| 7：任务插件完整能力 | 插件管理/市场/版本/沙箱；渠道绑定；流式任务；用量 schema/示例/矩阵/按插件价格；模型广场多提供商/任务筛选；统一产物预览/下载 | 新插件表、JS 运行时、relaykit、任务提交/轮询/输出/计费、channels、pricing、logs 多模块；需要架构适配及数据库迁移 | **高**：上游重构旧任务实现，与当前视频/异步图像定制冲突；渠道编号冲突，运行代码/来源及历史任务数据需要隔离兼容 |
| 8：路由检测与任务媒体地址 | 检测新增 auto_ban_only 与并发 1–32；模型名规则的新语义；亲和性模板扩充；Async Task Public Address | operation_setting、channel-test 调度、模型名解析/路由、参数覆盖；任务媒体地址还涉及输出 URL/代理/Nginx 兼容；预计用现有 options 存储，无新增业务表 | **中至高**：检测改变请求量/自动禁用恢复；模型名规则影响路由；媒体公共地址不能覆盖当前 OSS/R2/CF/ESA/直通输出契约。只新增模板展示风险较低，实际启用另行确认 |

1A/1B、5A/5B 是同类工作的可独立交付部分，不能为了凑成一批把高风险行为混进 UI 布局修改。第 8 批可与第 5–7 批独立安排，其编号不代表必须等插件完成。

原先得到确认的是第一、二批核心方向。本轮发现的模型广场补充和第 3–8 批仅完成评估；不将之前的同意扩大为全部新范围已获确认。

## 已确认的关键兼容风险

1. **渠道编号冲突**：当前 58＝TencentVideo、59＝AdvancedCustom、60＝ServiceInferenceVideo、61＝Xinhankr、63＝Sub2API、64＝NewAPI；上游 58＝AdvancedCustom、59＝Sub2API、60＝NewAPI、61＝TaskPlugin。不能原样替换枚举或按上游编号解释现有数据。插件需选择未占用编号，并统一前后端。
2. **本地任务与视频功能**：当前有 ServiceInference、Tencent/Xinhankr、Seedance、MiniMax、统一异步图像、输出存储和审计定制；上游任务层已采用 jsplugin/relaykit 架构。不能删除当前 adaptor，也不能把新的任务快照结构直接覆盖旧结构。
3. **任务详情权限**：当前 `/api/task/:task_id/audit` 是管理员接口，包含已脱敏请求/计费/响应。普通用户详情使用其允许访问的现有日志字段，不调用管理员接口；插件/root 诊断缺少数据时不伪造或仅靠前端隐藏。
4. **健康条不是纯前端**：当前汇总返回最多 3 个不带时间戳成功率；上游返回逐小时带时间戳序列。需要汇总 API 增量兼容，不能把旧 3 个点拉伸成 24 小时。
5. **计费部分已存在**：当前已有基础表达式、条件/阶梯与 hour/weekday 等时间条件、图片数量预扣与安全保护。缺少的是增强编辑、预览、图片缓存/按次等增量，不能说整个动态计费或时间条件都没有。
6. **登录加密部分已存在**：后端及 `password-encryption.ts` 已存在，但当前密码登录入口未按后端开关接通。实施时只接通支持逻辑；本轮未读取生产环境凭证/环境开关，不声称生产当前已经启用或受影响。
7. **渠道检测增量前后端都缺少**：当前只支持 scheduled_all / passive_recovery，没有 auto_ban_only 和可配置并发。不能以新增控件代替调度逻辑。
8. **付款安全定制保留**：当前已有金额计算请求序号防覆盖、余额上限失败说明等定制；上游新版拆分付款处理时部分删除了这些实现，移植新确认流程时仍须保留保护。
9. **不能把上游删除当作升级要求**：主题切换、推荐计划、R2 白名单、备案页脚、VideoCompletionRatio 等本地能力需要保留；项目 new-api / QuantumNous 标识及归属不修改。

上游 [rc.37 发布说明](https://github.com/QuantumNous/new-api/releases/tag/v1.0.0-rc.37) 将任务插件标为实验性架构，且说明此发布仍不推荐生产使用。这里作为第 7 批高风险评估的依据；不意味着必须放弃可隔离移植的前端改进。

## 各批交付前的验证范围

| 批次 | 必须验证的真实行为 |
| --- | --- |
| 1A/1B | 桌面/手机及深浅主题；额度有限/无限/零/负余额状态；筛选展开/重置后查询一致；普通用户与管理员详情权限；更新失败/忽略版本/定制版本；24 小时空流量和跨小时健康条，不更改计费配置 |
| 2 | 现有渠道打开→不修改→保存后的完整往返；空 key 保留原密钥；单 key/多 key 策略；自定义 Base URL；映射/覆盖参数；显式 0/false；全部本地类型/视频/图像输出字段；模型发现失败后草稿不丢失 |
| 3 | 抽屉中的 Select/Combobox 能鼠标/触屏/键盘交互；列显隐立即刷新；失败提示只报一次且业务错误不显示空成功结果；在线测试取消、重发、切换及卸载不被旧响应覆盖；编辑未保存离开确认；旧地址 query/hash 保留 |
| 4 | 各支付方式金额计算与确认、取消不下单、正确 Waffo 方式索引、重复提交控制；订单搜索乱序响应；订阅偏好在无有效订阅下展示准确。生产支付不作为本轮核查或未经确认的测试手段 |
| 5A/5B | 币种/精度往返、零价与未配置区分；转换只生成草稿；旧价/新价匹配；缓存排除及图片数量；实际结算与预览一致；预扣不足、重试、失败退款、异步旧快照；超大/NaN/溢出不生成负扣费；历史日志不按当前时间重算 |
| 6 | OWASP ASVS 5.0.0 与相关指南；登录/刷新/退出；现有和新 Passkey 域名；删除影响/确认/并发变化；挑战与 proof 过期、重放和跨会话/操作绕过；邀请码/微信邀请关联；加密开关开/关及失效公钥；模板授权逻辑服务端执行；审计不记录可用凭证 |
| 7 | 未启用插件时旧业务等价；编号兼容；旧/新任务提交/轮询/退款；插件版本切换与来源限制；产物权限和下载；schema 用量计价；三数据库迁移；`relaykit` 若引入必须独立 `GOWORK=off go build ./...` |
| 8 | 检测模式选择、并发边界与调度、不误启用额外检测；自动禁用/恢复；模型名与正则路由兼容；模板只影响新选用配置；不同媒体域名、端口和路径前缀及当前存储输出策略 |

所有实际 TS/TSX 变更按项目要求完成 typecheck、相关文件 lint/format 及构建；只为实际行为/契约回归增加必要测试。前三类数据库迁移仅在实际涉及表结构的批次执行。代码回退与数据回退分开说明；存在新数据或凭证迁移时不承诺仅换旧镜像即可完整回退。

安全工作引用：[OWASP ASVS 当前稳定版 5.0.0](https://owasp.org/www-project-application-security-verification-standard/)、[Authentication Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html)、[Session Management Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html)。本轮是范围评估，没有执行安全回归，因此不声明认证合规已完成。

## 可复核的代码入口

| 核查对象 | 当前入口 | 上游入口或提交 |
| --- | --- | --- |
| 全站页面 / 导航 | `web/default/src/routes/`、`hooks/use-sidebar-data.ts`、`components/layout/config/` | [main routes](https://github.com/QuantumNous/new-api/tree/7fd06381976cbea126d1f20973548bf60f20e37d/web/src/routes) |
| 设置子页 | `features/system-settings/*/section-registry.tsx` | [main system-settings](https://github.com/QuantumNous/new-api/tree/7fd06381976cbea126d1f20973548bf60f20e37d/web/src/features/system-settings) |
| 模型广场 / 性能 | `features/pricing/`、`features/performance-metrics/types.ts`、`pkg/perf_metrics/` | [模型健康逐小时序列提交](https://github.com/QuantumNous/new-api/commit/5c7cca015525212a7ac2741da7f3b51fa2e30db0) |
| 公共移动筛选 | `components/data-table/`、`features/channels/`、`features/usage-logs/` | [发布后 main 的移动筛选提交](https://github.com/QuantumNous/new-api/commit/2ba61576146f0583f789ee6845f2280cb86819ce) |
| 渠道地址占位提示 | `features/channels/components/drawers/`、`lib/channel-form.ts` | [发布后 main 的 Base URL 占位提交](https://github.com/QuantumNous/new-api/commit/c9a110190c5241c24d9d66de340431f5e9873db6) |
| 渠道类型及定制字段 | `constant/channel.go`、`dto/channel_settings.go`、`features/channels/lib/channel-form.ts` | [上游 constant/channel.go](https://github.com/QuantumNous/new-api/blob/7fd06381976cbea126d1f20973548bf60f20e37d/constant/channel.go) |
| 管理员任务审计 | `router/api-router.go`、`controller/task.go`、`controller/task_audit_test.go` | [上游任务详情 UI](https://github.com/QuantumNous/new-api/blob/7fd06381976cbea126d1f20973548bf60f20e37d/web/src/features/usage-logs/components/dialogs/task-details-dialog.tsx) |
| 在线测试 | `features/playground/hooks/use-chat-handler.ts`、`use-stream-request.ts`、`components/message/playground-message-editor.tsx`、`components/ai-elements/` | [上游 playground](https://github.com/QuantumNous/new-api/tree/7fd06381976cbea126d1f20973548bf60f20e37d/web/src/features/playground) |
| 登录加密与 OAuth 模板 | `controller/user.go`、`router/api-router.go`、`features/auth/api.ts`、`features/auth/lib/password-encryption.ts`、`oauth/generic.go` | [上游 auth](https://github.com/QuantumNous/new-api/tree/7fd06381976cbea126d1f20973548bf60f20e37d/web/src/features/auth) |
| 钱包付款 | `features/wallet/index.tsx`、`hooks/use-payment.ts`、`router/api-router.go` | [上游 wallet](https://github.com/QuantumNous/new-api/tree/7fd06381976cbea126d1f20973548bf60f20e37d/web/src/features/wallet) |
| 定价与结算契约 | `features/model-pricing/`、`features/system-settings/models/`、`pkg/billingexpr/expr.md`、`service/tiered_settle.go`、`service/task_billing.go` | [上游定价编辑](https://github.com/QuantumNous/new-api/tree/7fd06381976cbea126d1f20973548bf60f20e37d/web/src/features/model-pricing) |
| 任务插件 | 当前无完整 `pkg/jsplugin` / `relaykit` 与插件管理页面 | [上游 task-plugins](https://github.com/QuantumNous/new-api/tree/7fd06381976cbea126d1f20973548bf60f20e37d/web/src/features/task-plugins) |
| 路由检测 | `setting/operation_setting/monitor_setting.go`、`controller/channel-test.go` | [上游 monitor setting](https://github.com/QuantumNous/new-api/blob/7fd06381976cbea126d1f20973548bf60f20e37d/setting/operation_setting/monitor_setting.go) |

表中的当前前端相对路径以 `web/default/src/` 为根；后端路径以仓库根为根。发布后 main 的变化明确标注，不混称为 rc.37 已发布功能。
