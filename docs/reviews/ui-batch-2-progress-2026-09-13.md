# UI 更新第 2 批：渠道配置进展（2026-09-13）

本页保留第 2 批此前阶段的历史进展，不作为当前待办或最终构建证据。2026-09-14 已完成剩余映射辅助、默认地址提示及全部本地类型/多 key 保存核对，最终结果见 [第 2 批验收](./ui-batch-2-validation-2026-09-14.md)。未部署生产，第 3–8 批继续待实施。以下阶段测试数字及待办均为当时记录。

## 本次续做：提供商选择与配置分页

- 创建渠道先进入提供商选择，编辑渠道直接进入配置；可搜索名称、译名和编号，支持内置/网关/自定义筛选、键盘选择及未知正整数编号。全部现有本地选项保留，腾讯视频 58、高级自定义 59、网关 63/64 不采用上游冲突编号。
- 表单分为连接与模型、路由与映射、请求与响应、其他设置四页。连接页保留完整提供商凭证控件；映射和优先级/权重进入路由页；参数/请求头/状态码覆盖及字段透传进入请求页；备注、传输、系统提示、输出策略及上游检测进入其他页。
- 已访问页面保留挂载；打开提供商选择时隐藏配置而不销毁草稿。返回或更换提供商保留地址、已选模型及其他分页字段。创建取消后重新打开，回到选择步骤。
- 保存校验切到字段所属页再聚焦，保留现有 Form 验证、风险确认、权限与保存逻辑。修正密钥说明内原有 div 嵌套 p 的无效 HTML。
- 复用候选已检查：现有 Combobox 适合单个下拉选择，不提供独立提供商选择步骤及分类卡片；新业务组合复用 Command、Tabs、Button、Badge；配置页复用 Tabs 及原字段 JSX。原提供商图标渲染提取为共享 ChannelTypeLogo，抽屉和选择器共同使用。

本次前端 typecheck、17 个涉及文件的 scoped lint/保留版权头格式检查及候选构建通过。渠道测试 **11 个文件、35 项通过**，其中 5 项抽屉集成用例验证跨页保存、错误定位、操作员权限与提供商切换草稿；4 项选择器用例验证本地分类、键盘、未知编号和禁用选项。新增文案经临时 add-missing-keys.mjs 和 i18n:sync 写入七语言，临时脚本已删除；两新组件的 16 个字面量键七语言均存在。

本次真实浏览器最终报告通过 **9 组检查**，覆盖四页切换/字段定位、提供商选择与返回、实际草稿发现/取消、59 自定义路由、失败重试、七语言 320px 两类分页与提供商选择、真实保存并重新读取配置、键盘选择 58 和取消创建。发现代理地址变化后结果清空是已有的正确失效行为，不要求保留过期发现。重复运行曾触发隔离实例 POST /api/user/auth/refresh 的 429，确认日志后仅重启本批后端及内部上游清空内存计数，未放宽安全配置；重跑通过。

产物：[本次报告](./artifacts/ui-batch-2-configuration/report.json)、[路由页](./artifacts/ui-batch-2-configuration/tabs-routing-desktop.png)、[中文手机提供商选择](./artifacts/ui-batch-2-configuration/provider-320-zh.png)、[构建指纹](./artifacts/ui-batch-2-configuration/build-manifest.json)。以下早期发现报告为此前阶段的记录，不代表本次新组件指纹。

仍需完成映射辅助、默认地址提示及全部本地类型/多 key 的完整浏览器保存往返后，才能整批验收。本次未改后端或数据库结构，仍使用 3324/3325 隔离环境；生产 healthy、revision 6cb299e801e902dacffffbd7c42006a736deddb6 未替换。

## 本轮实现及影响

| 项目 | 实现 | 影响与风险 |
| --- | --- | --- |
| 草稿模型发现 | POST 接口接通 channel_id、自定义模型路由、请求头和代理；省略可选配置保留已保存值，显式空字符串清空草稿值；类型必须与原渠道一致 | 渠道创建/编辑的发现请求。保持原 ChannelSensitiveWrite 权限；不增加表或迁移 |
| 原密钥及多 key | 编辑时空 key 使用服务器保存的启用 key；草稿有新 key 时使用新 key；跳过禁用 key，轮询预览不推进保存的索引 | 仅 POST 草稿预览；不返回密钥，不修改数据库。Codex 草稿不能自动刷新已保存凭证，原有显式刷新入口保留 |
| 原有操作权限 | 只有操作权限、不能修改敏感配置的管理员，继续通过原 GET 接口发现已保存配置的模型 | GET 保持 ChannelOperate 权限及既有行为，不允许这类管理员提交敏感草稿 |
| 内联分类选模 | 上游模型按厂商/家族分类、搜索及批量选择；区分新增/已有/未返回模型；来源别名不计作上游移除，取消勾选的候选可重新选择 | 仅更新表单草稿，最终提交才保存。原弹窗的 onModelsSelected 模式已是填写表单，本次保留其语义并改为内联 |
| 失败及旧响应 | 连接字段变化/关闭时中止浏览器请求并使旧结果失效，保留已选模型；失败在内联区域展示并可重试，不触发全页 500 或重复通知 | Axios 的认证流程未改；取消和过时响应即使传输层迟到也不应用 |
| 本地编号修复 | advanced-custom.ts 原来重复定义 58，改为引用 constants.ts 的本地 59；TencentVideo 仍为 58 | 恢复高级自定义路由的界面、校验、序列化和发现请求，避免腾讯视频被套用高级自定义校验；不改数据库编号 |
| 通用 setting 保留 | 原序列化只重建已知字段，现先保留原 JSON 扩展字段，再更新已知字段；选择默认传输模式时清除旧传输选项 | 影响渠道创建/编辑的 setting 序列化，必须验证未知字段、0/false 及传输模式往返 |
| 手机分类标签 | 修正共享 TabsList 固定高度覆盖自动高度的问题 | 只调整内联分类选择区域；多行标签不再与分类内容重叠 |

风险仍为**中**：表单保存影响实际渠道配置；编号修复和序列化保留需要和剩余配置分页共同回归。没有计费、任务执行、媒体域名、登录机制或数据库结构修改。

## 隔离环境

- 后端：`new-api-ui-batch-2`，`127.0.0.1:3324`，新编译二进制只读绑定 `/tmp/newapi-ui-batch-2-api`；使用现有镜像，没有构建 Docker 镜像。
- 数据：独立卷 `new-api-ui-batch-2-data`、SQLite `/data/ui-batch-2.db`，来自上一批合成测试数据，新增 OpenAI 和本地 59 高级自定义合成渠道。未复制生产数据或凭证。
- 前端：`/tmp/newapi-ui-batch-2-dist`，`127.0.0.1:3325` preview，独立同源代理到本批后端；实际 HTML 入口资源存在。
- 上游：`new-api-ui-batch-2-mock` 与隔离后端共享网络，仅监听内部 `127.0.0.1:3398`，返回固定合成模型列表，不记录请求头。主机测试端口连接未完成后，改用隔离网络，不放宽生产网络限制。
- 生产：现有 `new-api` 仍 healthy，revision `6cb299e801e902dacffffbd7c42006a736deddb6`，镜像、数据及配置未替换。

## 验证及边界

| 检查 | 结果 |
| --- | --- |
| 后端 | controller 专项测试通过；6 个主用例及参数分支，覆盖自定义路由/请求头、原 key、多 key 不推进、不修复写入坏 JSON、禁用 key、类型不匹配、显式清空、未知编号不越界，以及旧 Claude 首行 key 行为 |
| 前端 | `bun run test src/features/channels`：9 个文件、26 项通过，包含现有密钥披露/保存/字段更新/输出策略/Seedance 默认值回归，以及新发现/分类/编号/扩展字段保留用例 |
| 静态检查 | typecheck、11 个涉及 TS/TSX 文件的 oxlint、保留版权头的 scoped oxfmt 通过；git diff --check 通过 |
| 多语言 | 新增 3 个键通过 add-missing-keys.mjs 写入七种语言，并运行 i18n:sync；涉及组件字面量键无缺失；临时脚本已清理 |
| 构建 | 独立前端构建和后端二进制构建通过，记录入口脚本和源文件指纹 |

浏览器成功路径使用真实候选后端和受控上游；HTTP 500 仅用于故障路径注入。报告与截图见 [本轮验证产物](./artifacts/ui-batch-2-discovery/report.json)。七语言手机布局以真实模型发现结果验证。JSON 中未启用且带 omitempty 的布尔字段可以省略，判断其有效 false 语义；参数覆盖中的显式 0/false 则要求原值保留。

最终浏览器报告通过 6 组检查：草稿地址及原密钥发现、选模/取消/新读取配置对照、本地 59 自定义路由、失败/重试、七语言 320 px 换行边界，以及真实保存后重新读取路由/模型/地址/R2/扩展字段/0/false。空 key 未进入保存请求。截图：[桌面发现](./artifacts/ui-batch-2-discovery/discovery-openai-desktop.png)、[手机失败](./artifacts/ui-batch-2-discovery/discovery-failure-mobile.png)、[中文手机分类](./artifacts/ui-batch-2-discovery/discovery-320-zh.png)。

可复核命令（Go 命令在仓库根执行，Bun 命令在 web/default 执行）：

```sh
env PATH=/usr/local/go/bin:/usr/bin:/bin /usr/local/go/bin/go test ./controller -run 'TestFetchModels|TestModelDiscoveryDraft' -count=1
bun run typecheck
bun run test src/features/channels
bun run build --dist-path /tmp/newapi-ui-batch-2-dist
```

构建和源文件指纹见 [build-manifest.json](./artifacts/ui-batch-2-discovery/build-manifest.json)、[source-sha256.json](./artifacts/ui-batch-2-discovery/source-sha256.json)。浏览器脚本使用 /tmp 中的合成测试登录文件，不打印凭证；重跑需保持独立测试实例和受控上游运行。

本轮执行数据库验证使用 SQLite；未重新运行 MySQL/PostgreSQL。preview 加绑定二进制不等于生产打包或发布完成。尚未覆盖所有本地类型的完整浏览器保存往返，须随剩余第 2 批完成；不以本轮结果宣称整批已验收。

## 后续及回退

继续实现映射辅助及默认地址提示，保留全部本地提供商和定制字段。已接通提供商选择和配置分页并重跑发现回归，再检查多 key 策略及全部本地类型保存往返，形成整批验收报告。

本轮无需数据迁移。代码回退按本轮具体文件差异处理，保留 1A/1B 和用户原有修改，不能 reset 整个工作区；测试环境可以单独停止，不影响生产容器。正式上线与正式镜像回退另行记录。
