# UI 更新 1B：模型广场实现与本地验收（2026-09-13）

本报告覆盖 [批次规划](./upstream-ui-implementation-batches-2026-09-13.md) 的 1B。候选代码位于工作区，尚未提交或部署生产。基准为当前定制版 `6cb299e80` 与已核查上游 `7fd063819`，并未整包覆盖上游前端。

## 完成功能与影响

| 功能 | 实际结果 | 影响范围 |
| --- | --- | --- |
| 卡片与布局 | 模型名称/厂商、描述/标签、价格、分组/端点、健康状态分区；名称可换行，详情位于底部；侧栏收窄至 280 px 并避开固定页头，网格在 md 两列、xl 三列 | 默认前端模型广场卡片、网格、侧栏 |
| 手机价格切换 | Standard/Recharge 与 K/M 控件在手机可用；复用 ToggleGroup 和已有表格视图切换，筛选抽屉保持右侧打开 | 模型广场工具栏；只改变显示，不修改定价配置 |
| 显式零价 | 免费输入、输出、缓存和按次价格保留 0；缺失按次价格不再通过 `|| 0` 伪装成免费，卡片显示 Unset price | 卡片和现有价格格式化函数；表格/详情复用该函数，无扣费行为修改 |
| 24 小时健康条 | 新接口返回 24 个带时间戳小时槽；无流量为 null，0% 为真实失败；分钟/五分钟采样按请求数加权。末格是当前未结束小时 | 性能汇总 API/类型、模型卡片；无需新表或迁移 |
| 兼容与失败处理 | 保留旧成功率、延迟、吞吐和最近三个采样字段，数量仍不公开；使用服务器快照时间防止客户端时钟偏差。旧后端显示灰条；可选性能请求 500 不跳转全页错误或通知失败，切换视图后可恢复 | 模型广场的可选诊断查询；其他分析页保持原请求错误行为 |

风险为**低至中**。界面部分风险较低；后端增加了小时聚合与返回数据，需要前后端配套发布，并关注汇总响应大小和查询成本。常用 24 小时请求仍为一次原有批量查询，没有每卡片独立查询；请求小于 24 小时时读取窗口扩大以提供健康序列，但旧统计总额仍保持原窗口。模型广场每分钟共享刷新一次汇总查询。

影响还包括消费同一汇总接口的概览与模型分析性能面板，已在真实测试后端验证显示数值一致。当前渠道保存、密钥、多 key 策略、本地渠道编号、视频/图像转发、任务结算和计费表达式不在本批改动范围。模型广场的复杂表达式预览、插件筛选及多提供商用量价格继续分别归第 5、7 批。

## 复用审查

- 卡片使用现有 Card、Button、CopyButton；工具栏移除本地手写 SegmentedControl，使用 ToggleGroup 与 DataTableViewModeToggle。
- 继续复用 PricingSidebar、现有详情抽屉、币种格式化和成功率分级工具。
- 既有 UptimeSparkline 面向 30 天逐日数据，并按严重程度调整条高/计算每日平均，不能表达本批逐小时、空流量和请求数加权合约。因此小时条继续由已有 ModelPerfBadge 承载业务数据，不另建公共交互组件。
- 时间标题复用 `toIntlLocale`，避免项目内部 `zhCN`/`zhTW` 语言代码导致 Intl RangeError。

## 隔离环境与验证

| 项目 | 实际证据及边界 |
| --- | --- |
| 后端 | 新编译工作区二进制 `/tmp/newapi-ui-batch-1b-api`，以只读绑定运行于独立容器 `new-api-ui-batch-1b`，`127.0.0.1:3322`。底层使用现有镜像，运行二进制不是镜像内原二进制；测试版本显示 v0.0.0，不作为发布 revision |
| 数据 | 独立卷 `new-api-ui-batch-1b-data` 和 SQLite，Redis/外部数据库关闭；36 个合成模型、分钟/小时流量与已删除分组数据。渠道仅指向本机合成地址，未执行真实上游调用或付款 |
| 前端 | `bun run build --dist-path /tmp/newapi-ui-batch-1b-dist` 通过，最终产物在 `127.0.0.1:3323` preview 验收；独立代理保持 Host，同源配置未放宽。HTML 入口资源逐项核实存在 |
| 类型、格式与 lint | `bun run typecheck`、10 个涉及 TS/TSX 文件的 oxfmt 检查及 oxlint 通过；版权头原样保留；`git diff --check` 通过 |
| 前端回归 | 9 个文件、48 项相关测试通过，包含 1A 回归、1B 小时数据/时钟偏差/旧字段/空数据与价格边界，以及已有表达式测试；不将 48 项全部归为 1B 新测试 |
| 后端回归 | `go test ./pkg/perf_metrics` 通过：真实 SQLite 查询及本机未落盘 bucket 合并、分组过滤、分钟加权、空流量、跨小时、短窗口统计保留、JSON 隐私合约；1A 现有任务审计脱敏测试同时通过 |
| 浏览器 | 11 组检查通过，成功路径使用真实编译后端：36 模型分页、零价/缓存、复制/详情、K/M 与价格模式、表格/卡片、筛选重置；390/320 px、深浅主题、七种语言；两个既有分析页性能面板数值一致，无页面 JS 错误 |
| 故障与旧后端 | 仅这些路径注入固定响应：性能 HTTP 500 保留真实目录/价格且可恢复；旧汇总只有三个采样时不生成假小时数据。管理员更新检查使用固定 GitHub 发布响应，不声称本轮验证实时 GitHub 连通性 |
| 多语言 | 新增小时条说明 1 个键，经规定的 add-missing-keys.mjs 与 i18n:sync 补齐七种语言；涉及文件字面量键无缺失，临时翻译脚本已清理 |
| 生产 | 容器 `new-api` 仍 running/healthy，revision `6cb299e801e902dacffffbd7c42006a736deddb6`；未替换生产镜像、数据库或配置 |

重复浏览器运行曾触发测试实例原有关键请求限流。最终运行前只重启独立 1B 容器清空其本机内存计数，没有关闭限流或改动生产安全控制。

本轮数据库执行验证使用 SQLite；MySQL/PostgreSQL 未重新执行。本批没有 SQL、存储模型或迁移变化，继续使用现有跨数据库查询路径。未对生产规模模型数量进行负载测试，也未以隔离测试结果承诺生产性能。

命令与结果可按下列范围复核：

```sh
# web/default/
bun run typecheck
bun run test src/features/pricing/components/model-perf-badge.test.tsx src/features/pricing/lib/price.test.ts src/features/pricing/lib/billing-expr.test.ts src/features/system-update src/features/usage-logs/components/dialogs/task-detail-dialog.test.tsx src/features/usage-logs/task-audit-api.test.ts src/features/keys/components/__tests__/api-key-quota-cell.test.tsx src/components/data-table/toolbar/mobile-filter-panel.test.tsx
bun run build --dist-path /tmp/newapi-ui-batch-1b-dist

# 仓库根目录，实际使用 Go 1.24.0
env PATH=/usr/local/go/bin:/usr/bin:/bin /usr/local/go/bin/go test ./pkg/perf_metrics ./controller -run 'TestSummaryHourlyWindowMergesTrafficAndPreservesLegacyFields|TestGetTaskAuditReturnsSanitizedRetainedData' -count=1
env PATH=/usr/local/go/bin:/usr/bin:/bin CGO_ENABLED=0 /usr/local/go/bin/go build -o /tmp/newapi-ui-batch-1b-api .
```

证据：[浏览器报告](./artifacts/ui-batch-1b/report.json)、[真实汇总响应](./artifacts/ui-batch-1b/perf-response.json)、[构建日志](./artifacts/ui-batch-1b/build.log)、[构建指纹](./artifacts/ui-batch-1b/build-manifest.json)、[源码指纹](./artifacts/ui-batch-1b/source-sha256.json)。截图：[手机健康条](./artifacts/ui-batch-1b/pricing-mobile.png)、[免费模型](./artifacts/ui-batch-1b/pricing-free-desktop.png)、[中文 320 px 深色](./artifacts/ui-batch-1b/pricing-320-zh.png)、[请求失败保留价格](./artifacts/ui-batch-1b/pricing-metrics-failure.png)。浏览器脚本同目录 `pricing-regression.mjs`，复跑仅使用上述隔离实例与 `/tmp` 测试凭证。

## 发布与后续

本批无需数据库迁移，后端增加的 JSON 字段允许旧前端继续读取原字段；新前端遇到旧后端会显示灰色小时条。正式发布时仍需要构建并打包实际候选前后端；本轮独立 preview 和绑定二进制不能当作生产发布完成。

回退按本批文件差异撤回前后端，保留其他工作区修改；上线后使用正式发布前保存的镜像回退，不能 reset 整个工作区。经典主题没有同步本批新增界面能力，1A 已验证的主题切换未改变。

下一批为 **第 2 批渠道配置流程**，在独立环境验证旧渠道无修改保存、定制类型/字段、密钥、多 key 策略和显式零/false。第 3–8 批完整范围继续保留，不因 1B 验收而视作整体完成。
