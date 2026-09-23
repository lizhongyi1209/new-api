# UI 更新第 2 批验收（2026-09-14）

第 2 批渠道配置功能已完成代码及本地验收，未部署生产。基准仍为生产 `6cb299e801e902dacffffbd7c42006a736deddb6` 与已全面核查的上游 `7fd06381976cbea126d1f20973548bf60f20e37d`，不将此固定快照称为今天重新查询的 GitHub 最新版本。此前阶段记录见 [进展报告](./ui-batch-2-progress-2026-09-13.md)。第 3–8 批继续待实施。

## 本批交付范围

| 功能 | 最终行为 | 影响 |
| --- | --- | --- |
| 提供商选择 | 创建先选择，编辑直接配置；名称/译名/编号搜索，分类及键盘选择；保留全部本地选项和未知正整数编号 | 默认主题渠道抽屉；本地 58/59/60/61/63/64 不改号；插件提供商归第 7 批 |
| 四页配置 | 连接与模型、路由与映射、请求与响应、其他设置；已访问页保持草稿，选择提供商时也不卸载；校验定位所属页并聚焦 | 表单布局及焦点；保留原控件、权限、风险确认和最终保存流程 |
| 草稿模型发现 | 使用草稿连接、请求头、代理及高级自定义模型路由；空 key 在后端使用已保存的启用 key；不推进轮询索引、不修复写入坏 JSON、不自动刷新保存的 Codex 凭证 | POST 预览构造和模型请求；POST/GET 原有权限分别保留 |
| 内联分类选模 | 分类、搜索、分组选择及新增/已有/移除区分；仅更新草稿；连接变化使旧结果失效，保留已选模型；失败可内联重试 | 模型选择；取消不保存，迟到响应不覆盖新状态 |
| 映射辅助 | 明确请求名→上游名及 JSON 方向；帮助可点击打开并通过 Escape 返回焦点；可视化/JSON 编辑继续复用原编辑器 | 修复自身 onChange 回传导致未完成行被重建的问题；外部重置仍加载新映射 |
| 默认地址提示 | 新增 read 权限 GET `/api/channel/default_base_urls`，取本地后端默认值；无默认值不返回；显示为占位提示，接口失败回退原提示 | 新只读接口及抽屉查询；不把提示自动填入保存地址；保留原提供商特殊地址控件 |
| 保存兼容 | 修正 AdvancedCustom 重复编号，保留 setting 未知字段，选择默认传输时清除旧覆盖 | 渠道表单序列化；其他定制业务不随上游编号重解释 |

继续复用现有 Command、Tabs、Popover、Button、Input、JsonCodeEditor 和原保存 hook。新提供商选择组件用于独立分类步骤，现有单下拉 Combobox 无该步骤能力；配置组件组织原字段 JSX，没有新增一套保存逻辑。没有增加前端依赖。

## 验证结果

| 验证 | 权威结果及范围 |
| --- | --- |
| 前端类型/规范 | `bun run typecheck` 通过；19 个涉及 TS/TSX 文件的 oxlint 和保留版权头 scoped oxfmt 通过；`git diff --check` 通过 |
| 渠道测试 | `bun run test src/features/channels`：12 个文件、43 项通过；覆盖发现/迟到响应、分类、编号、保存/权限、跨页草稿、校验焦点、地址提示、映射方向/未完成行/重复别名/外部重置 |
| 后端专项 | `go test ./controller ./router -run 'TestChannelDefault\|TestChannelStatusRoutesRegister\|TestFetchModels\|TestModelDiscoveryDraft' -count=1` 通过；预览、saved key/轮询、坏 JSON、不匹配类型/禁用 key、未知编号及默认地址 read 权限 |
| 原有真实浏览器流程 | [9 组检查](./artifacts/ui-batch-2-final/configuration/report.json)通过，包含真实候选后端、受控上游、七语言 320px、取消及真实保存后重新读取 |
| 映射及地址真实交互 | [6 组检查](./artifacts/ui-batch-2-final/mapping-hints/report.json)通过：手机点击帮助/焦点返回、目标先填的未完成行跨页保留、JSON 方向、默认值不写入地址、提示 HTTP 500 回退及取消无写入；无 pageerror |
| 全本地类型保存 | [浏览器矩阵](./artifacts/ui-batch-2-final/matrix/report.json)通过全部 52 种当前可选择本地类型及随机/轮询两个多 key 样本，共 54 次真实 PUT；保存请求均不含空 key |
| 实际数据库核对 | 对停服一致性快照运行 [核对脚本](./artifacts/ui-batch-2-final/verify-channel-matrix.py)，[54 个样本通过](./artifacts/ui-batch-2-final/matrix/database-verification.json)。检查每个存储列及原有 JSON 字段；密钥仅内存比较，不进入报告；多 key 状态/大小/索引/策略保持不变，R2、Gemini/Vertex fileData、原映射/覆盖参数、未知 0/false 保留 |
| 多语言 | 3 个映射新增键经临时 add-missing-keys.mjs 及 i18n:sync 写入七语言，逐个确认非空；临时脚本已删除；此前选择器和分页翻译保留 |
| 构建 | 候选后端二进制构建成功，前端候选生产构建成功；[构建指纹](./artifacts/ui-batch-2-final/build-manifest.json)及 [40 个源文件指纹](./artifacts/ui-batch-2-final/source-sha256.json)已归档，HTML 引用的 8 个本地资源实际存在 |

数据库比较按有效语义核对：JSON 空白不计差异，原 nullable 空文本与空字符串等价；未启用 fileData 的 false 可按现有 omitempty 省略；表单补齐的中性默认值，以及后端已有 Vertex JSON/AWS AK-SK 默认类型不计作业务改变。所有其他原字段要求保留；参数覆盖的显式 0/false、未知扩展字段必须保留原值。这里不声称 JSON 字节完全不变。

全部类型来自当前 `CHANNEL_TYPES` 实际非注释且非 0 项，与 `CHANNEL_TYPE_OPTIONS` 的保留选项对应，共 52 项，包含旧本地类型 7/16 和高级自定义 59。没有将上游 TaskPlugin 61 套用到本地 Xinhankr。

连续矩阵测试曾触发隔离服务全局 429，46 种类型已完成；日志确认后保留通过记录，仅重启本批隔离后端与内部上游，继续剩余 6 种类型及 2 个多 key 样本。未修改限流/认证配置，未把限流失败算作通过。最终矩阵和数据库证据包含全部 54 项。此前浏览器流程也重新通过。

## 环境、风险与回退

- 本地后端 `new-api-ui-batch-2` / `127.0.0.1:3324`；独立卷 `new-api-ui-batch-2-data`、SQLite `/data/ui-batch-2.db`。数据及凭证均为合成测试样本，没有复制生产数据。
- 本地前端 `127.0.0.1:3325` preview；独立构建目录 `/tmp/newapi-ui-batch-2-dist`，同源代理只指向本批后端。内部受控上游只监听测试容器网络 `127.0.0.1:3398`，不访问真实付费供应商。
- 风险为 **中**，主要集中在渠道编辑保存、JSON 序列化和模型发现。没有数据库结构迁移，没有修改扣费/任务执行/媒体域名/登录机制。52 类型验证覆盖保存契约，不表示真实供应商的全部生成、余额、OAuth 刷新等流程都已重新执行。
- 本轮数据库执行验证为 SQLite；未重新运行 MySQL/PostgreSQL。没有引入数据库查询或迁移变化。默认主题更新不代表经典主题获得同样新界面。
- 使用现有镜像绑定候选二进制，没有构建 Docker 镜像。preview 加绑定二进制不等于正式镜像打包完成。独立目录构建会保留旧资源，已经核对当前 HTML 指纹及所引用资源，未靠目录存在证明新构建。
- 生产 `new-api` 只读检查仍为 running/healthy，revision `6cb299e801e902dacffffbd7c42006a736deddb6`，未替换生产镜像/数据/配置。
- 无数据迁移，回退按本批具体源码差异恢复，保留第 1 批及用户原修改；不能 reset 整个工作区。测试服务可以单独停止。正式部署和正式镜像回退另按授权安排。

后续按规划继续第 3 批，公共组件、失败处理、概览/在线测试分别交付，不把第 2 批通过视为整体目标完成。
