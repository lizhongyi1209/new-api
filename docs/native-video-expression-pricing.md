# 原生视频模型表达式定价

2026-10-07 完成本地实现和验证；该阶段未部署，也未更改线上计费配置。生产发布通过
`scripts/deploy-production.sh` 执行，模型价格仍需在管理界面显式保存。当天只读核对调用记录，找到
76 个不同视频模型名、81 个模型/渠道类型组合，覆盖以下 9 种原生渠道。失败调用也纳入范围。
未读取渠道密钥、用户身份或提示词。

| 渠道 | 有记录的模型族 | 表达式用量 |
| --- | --- | --- |
| 60 TokenMart / ServiceInference | Dreamina / Doubao Seedance 2.0、Fast、Mini、2.5、HC/MAX/EP 等别名 | `tokens`、`resolution`、`video_input` |
| 54 Doubao Video | doubao-seedance-2-0-260128、seedance-2.0-fast | 同上 |
| 61 Xinhankr | seedance-2.0、Fast、Mini | 同上 |
| 50 Kling | v2.6、v3、v3 Omni、3.0，标准/专业/音频/4K/时长/动作控制变体 | `units`，保留上游资源扣减的小数精度 |
| 58 Tencent Video | kling-v2-6-t、kling-v3-t、Omni、动作控制变体 | `units` |
| 35 MiniMax、60 TokenMart | MiniMax-H3 / MiniMax-H3-MAX | `seconds`、`input_video_seconds`、`input_images`、`resolution` |
| 48 xAI、60 TokenMart、1 OpenAI 兼容 | grok-imagine-video、1.0、1.5、1.5-preview | `seconds`、`input_images`、`resolution`、`operation` |
| 24 Gemini | gemini-omni-flash-preview | `input_tokens`、`text_output_tokens`、`video_output_tokens` |
| 1 OpenAI 兼容 | veo-3.1、omni_flash_abra_edit、omni_flash_10s | Veo：`seconds`、`resolution`；Omni 兼容入口：`seconds`、`size` |

同一公开模型名映射到不同用量体系时，保存会拒绝不兼容的配置，需要使用不同的计费模型名。
模型映射通过同一个渠道的映射链解析，价格仍保存到公开模型名。已有插件定价继续使用插件自己的配置。

## 从设置面板配置

1. 打开 **系统设置 → 计费与支付 → 模型定价**（`/system-settings/billing/model-pricing`）。
2. 选择视频模型，切换到 **表达式**。
3. 在价格矩阵里按分辨率、参考视频等维度填写单价，也可切换源码输入表达式。
4. 用“示例规格”和费用计算器核对金额，再保存。

Token 的矩阵单价单位是 **每百万 Token**；秒、图片数、资源点分别按其对应单位填写。
矩阵会按所选币种换算，表达式源码中的价格始终是 USD。当前站点 `USDExchangeRate=1`，
因此以下人民币数值与源码数值一致；将来汇率改变时，使用矩阵的站点币种输入，或先将人民币单价除以汇率再写源码。

视频旧计费无法统一自动转换，转换按钮仍返回“使用任务用量 schema 手动转换”的说明。
本次没有自动把已有模型切换模式，也没有自动调整任何原价、折扣或分组倍率。

## Seedance 恢复原价

Fast 的 HC/MAX/其他别名，在当前汇率为 1 时可直接保存：

```text
u("video_input") == "video" ? tier("reference_video", u("tokens") * 22 / 1000000) : tier("standard", u("tokens") * 37 / 1000000)
```

Mini 对应：

```text
u("video_input") == "video" ? tier("reference_video", u("tokens") * 14 / 1000000) : tier("standard", u("tokens") * 23 / 1000000)
```

`video_input` 的值是 `none` / `video`，表示提交时是否含参考视频。普通版和 2.5 可以同样按
`resolution` 和 `video_input` 分支设置；各分辨率的价格应按实际供应商账单填写。
表达式模式不会叠加旧的 `ModelRatio` 或 `video_input` 倍率。分组倍率仍作用一次，
包括当前三个特定用户分组对 `seedance-官方` 的 0.9 倍；若需要这些分组也按完全原价收费，
需要另行将对应分组覆盖改为 1。

## 其他模型表达式示例

以下示例用于说明单位和字段，保存前应确认自己的售卖单价。人民币计价供应商在源码中应先换算为 USD；
表达式运行期间不读取 Tencent markup、实时汇率或旧 ModelRatio。

Kling / Tencent：每资源点的售卖单价为 USD 1 时：

```text
tier("resource", u("units") * 1)
```

MiniMax H3：可计输出及参考视频秒数；前 5 张图片免费时：

```text
u("resolution") == "2K" ? tier("2K", (u("seconds") + u("input_video_seconds")) * 0.8 + max(u("input_images") - 5, 0) * 0.2) : tier("768P", (u("seconds") + u("input_video_seconds")) * 0.5 + max(u("input_images") - 5, 0) * 0.2)
```

Grok 1.5：

```text
u("resolution") == "1080p" ? tier("1080p", u("seconds") * 0.25 + u("input_images") * 0.01) : u("resolution") == "720p" ? tier("720p", u("seconds") * 0.14 + u("input_images") * 0.01) : tier("base", u("seconds") * 0.08 + u("input_images") * 0.01)
```

`operation` 是 `generate` / `edit` / `extend`。编辑在未知分辨率时使用 `auto`，可为这一行单独设置价格。
延长视频只按新生成部分的 `seconds` 计量，不把输入视频时长再次收费。

Gemini Omni（文本输出包含思考 Token，避免漏计或重复）：

```text
tier("omni", (u("input_tokens") * 1.5 + u("text_output_tokens") * 9 + u("video_output_tokens") * 17.5) / 1000000)
```

## 预扣与结算合同

- 提交按经校验的请求估算用量。Seedance 用分辨率与时长估算 Token，参考视频预留 15 秒输入；自动输出时长预留 15 秒。
  Kling 暂按每请求秒 2 个资源点预留；实际资源扣减完成后结算。MiniMax 参考视频同样预留 15 秒。
  这些是预扣估算，不是供应商的精确报价，也不能保证覆盖任意管理员定义的条件表达式。
- 请求里的 `usage` 不会成为用量；秒数、图片数、Token 和资源点先校验范围，再作为乘数。
- 提交时冻结表达式、分组倍率、额度换算参数、用量维度和引用的标量条件/允许的请求头/时间。
  修改模型价格不会改变已经提交的表达式任务。
- 成功后用上游实际 Token、资源点、时长和多模态用量替换估算，差额调整钱包/订阅与令牌并更新原消费日志。
  上游明确返回 0 的用量可以退回预扣；缺失或非法 Token/资源点用量保留预扣并记录警告。
- Veo / Sora 兼容入口若没有实际时长，保留请求计价时长；MiniMax 只覆盖实际上报的字段。
  旧任务、旧倍率/按次模式以及插件历史快照保持原有结算。

## 本地验证

- 只读调用记录核对：所有 81 个模型/渠道类型组合都有用量计量器。
- 后端：schema 保存、别名、版本冲突、旧插件兼容、预扣在调用上游前拦截余额不足、精准小数资源点、
  缺失/非法/零用量、钱包和令牌差额、重复轮询、失败退款、冻结条件、Grok 延长时长边界。
- 前端：55 个价格编辑器/任务表达式测试通过，含后台示例独立于公开目录的显示与 Token 金额预览。
  TypeScript、所改文件 lint、前端构建通过。
- 实现阶段完成根模块本地编译，未执行 Docker 构建、生产部署、线上价格写入或真实上游付费测试。
  发布阶段由 `scripts/deploy-production.sh` 执行测试、镜像构建、缓存清理、部署、健康检查及版本核验。
