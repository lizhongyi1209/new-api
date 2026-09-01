# Documentation index

`docs/` 只保留当前有效的产品、接口和运维资料。历史审计记录统一放在 `archive/`，不应作为当前实现契约使用。

## 接口与下游对接

- [`api-doc.html`](./api-doc.html)：下游 API 对接页面。
- [`seedance-downstream-integration.md`](./seedance-downstream-integration.md)：Seedance 下游接入说明。
- [`kling-api-guide.md`](./kling-api-guide.md)：Kling 视频接口指南。
- [`openapi/`](./openapi/)：OpenAPI JSON 定义。

## 系统与业务规则

- [`PROJECT_MAP.md`](./PROJECT_MAP.md)：功能入口索引；定位现有功能时优先阅读。
- [`authentication.md`](./authentication.md)：鉴权、会话和可信代理约束。
- [`billing_refund_policy.md`](./billing_refund_policy.md)：计费退费规则。
- [`channel/other_setting.md`](./channel/other_setting.md)：渠道其他设置。
- [`ionet-client.md`](./ionet-client.md)：IO.NET 客户端说明。

## 运维

- [`operations/production-release-process.md`](./operations/production-release-process.md)：生产发布流程。
- [`operations/generate-image-observability.md`](./operations/generate-image-observability.md)：异步生图全链路排查。
- [`operations/generate-image-filedata.md`](./operations/generate-image-filedata.md)：Gemini `fileData` 输入设计与验收。
- [`operations/server-bandwidth-check.md`](./operations/server-bandwidth-check.md)：服务器带宽测试与选型口径。

## 前端与安装资料

- [`installation/`](./installation/)：安装说明。
- [`translation-glossary.md`](./translation-glossary.md)、[`translation-glossary.fr.md`](./translation-glossary.fr.md)、[`translation-glossary.ru.md`](./translation-glossary.ru.md)：翻译术语表。
- [`images/`](./images/)：README 等文档使用的图片资源。

## 历史归档

- [`archive/upstream-merge/`](./archive/upstream-merge/)：M0–M9 选择性合并的历史审计、影响范围和 Roadmap。内容用于追溯决策，不代表当前运行状态。
