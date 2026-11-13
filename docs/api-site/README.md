# API 对接文档站

公开文档的源文件，线上地址 `/docs/`。同一份源文件生成两种读者的版本：

- 网页：`/docs/`（概览）、`/docs/<page>`（每个模型一页），有「简明说明 / 完整参考」两种阅读模式。
- Agent：`/docs/<page>.md`、`/docs/llms.txt`（索引）、`/docs/llms-full.txt`（全部合并），`/docs/download` 下载全文。

## 目录

| 路径 | 内容 |
|---|---|
| `site.json` | 站点标题、Base URL、鉴权说明，以及侧栏分组和页面顺序 |
| `pages/index.html` | 概览页；`<doc-endpoint-index>` 处自动生成接口一览 |
| `pages/<slug>.html` | 每个模型一页，文件名即 URL |
| `assets/` | 全站共用的 `docs.css`、`docs.js` |
| `build.py` | 构建脚本，只用 Python 标准库 |

## 页面格式

```html
<doc-page slug="wan3" nav="侧栏名称" title="页面标题" kind="async">
<doc-summary>一两句概述，也用作 llms.txt 的说明。</doc-summary>
<doc-flow><step title="提交任务">…</step>…</doc-flow>
<doc-simple>简明说明模式的内容</doc-simple>
<doc-agent><ol><li>Agent 调用规则</li></ol></doc-agent>

<doc-endpoint id="wan3-create" title="接口标题" returns="接口一览的「返回」列">
<doc-route method="POST" path="/v1/video/generations"></doc-route>
<doc-main>参数说明、表格</doc-main>
<doc-examples>右侧示例和响应</doc-examples>
</doc-endpoint>
</doc-page>
```

- `kind="async"` 会在标题下显示「异步任务」标签，`kind="reference"` 不显示。
- `curl` 示例会自动生成 Python 和 Node.js 版本，只支持 `-X`、`-H`、`-d`、`-F`、`--output`。其他写法只显示 cURL。
- 在响应示例的 `<pre>` 上标 `data-sim="submit"` 或 `data-sim="done"`，「模拟一次调用」会按这两段响应演示流程。
- `href="#id"` 指向其他页面的元素时，构建时会自动改成跨页链接。所有页面的 `id` 必须唯一。

## 构建与发布

```bash
python3 -I docs/api-site/build.py --out /tmp/docs-preview   # 本地构建预览
scripts/deploy-api-doc.sh                                   # 构建并发布到 Nginx
```
