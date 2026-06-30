# Billing Refund Policy - 计费退费策略

## 背景

本项目作为 AI API 网关，需要准确反映上游提供商的计费行为，避免出现"上游扣费，我们退费"导致的亏损。

## 核心原则

**信任上游返回的计费信息（Trust Upstream Usage Metadata）**

- 如果上游 API 返回了 `usage` 信息（包含 token 计数），说明上游已经处理并计费
- 我们应该按照上游返回的计费信息扣费，而不是基于假设进行退费

## 常见误解场景

### 场景1：安全过滤拦截（Safety Filter）

```
Request: 用户发送违禁内容
Response: {
  "error": "content_filter",
  "usage": {
    "prompt_tokens": 263,
    "completion_tokens": 0
  }
}
```

**错误理解**：输出为 0，应该退费  
**正确理解**：上游处理了输入（审核内容），已经消耗资源，会扣费  
**处理方式**：按 `prompt_tokens=263` 扣费，不退费

### 场景2：客户端中断（Client Disconnect）

```
Request: 用户请求流式响应
Response: 流式输出到一半，客户端断开连接
Usage: {
  "prompt_tokens": 547,
  "completion_tokens": 0
}
```

**错误理解**：没有输出，应该退费  
**正确理解**：上游已经处理输入并开始生成，消耗了资源  
**处理方式**：按上游返回的 token 数扣费，不退费

### 场景3：图片生成（Image Generation）

```
Request: 生成图片
Response: {
  "images": [...],
  "usage": {
    "prompt_tokens": 82,
    "completion_tokens": 0  // 图片不计入 completion_tokens
  }
}
```

**错误理解**：没有输出 token，应该退费  
**正确理解**：图片生成任务的输出不体现在 `completion_tokens` 中  
**处理方式**：按实际计费规则（可能基于图片数量、分辨率等）扣费

## 应该退费的场景

### 场景1：网络超时/上游错误

```
Response: 504 Gateway Timeout
Usage: null 或 { "prompt_tokens": 0, "completion_tokens": 0 }
```

**处理方式**：不扣费（或退费），因为上游未成功处理请求

### 场景2：无效请求

```
Response: 400 Bad Request
Usage: null 或全 0
```

**处理方式**：不扣费，请求未被处理

## 代码实现

### 主要修改

1. **Gemini Handler** (`relay/gemini_handler.go`)
   - 移除 `RefundIfZeroCompletionTokens` 调用
   - 完全信任 Gemini 的 `usageMetadata`

2. **Compatible Handler** (`relay/compatible_handler.go`)
   - 移除 OpenAI、Claude 等标准提供商的退费调用
   - 这些提供商都返回准确的计费信息

3. **Image Handler** (`relay/image_handler.go`)
   - 移除图片生成的退费调用
   - 图片输出本身不计入 `completion_tokens`

4. **RefundIfZeroCompletionTokens 函数优化** (`service/text_quota.go`)
   - 添加安全过滤检测：检查 `reject_reason` 是否包含 `block_reason`、`PROHIBITED_CONTENT` 等
   - 添加客户端中断检测：检查是否包含 `client_gone`、`context canceled`
   - 只在真正异常的情况下退费（无明确原因且 completion=0）

### 检测逻辑

```go
// 检查是否有明确的拦截原因
rejectReason := common.GetContextKeyString(ctx, constant.ContextKeyAdminRejectReason)

// 这些情况不退费（上游已处理并计费）
if strings.Contains(rejectReason, "block_reason") ||
   strings.Contains(rejectReason, "PROHIBITED_CONTENT") ||
   strings.Contains(rejectReason, "SAFETY") ||
   strings.Contains(rejectReason, "content_filter") ||
   strings.Contains(rejectReason, "client_gone") ||
   strings.Contains(rejectReason, "context canceled") {
    // 不退费
    return
}

// 只有无明确原因的异常情况才退费
```

## 数据验证

基于生产日志分析（5124 条 Gemini 日志）：

- **正常计费**: 2897 条 (56.5%) - 有完整的 prompt + completion tokens
- **网络错误**: 2139 条 (41.7%) - 504 超时，已经不扣费（quota=0）
- **仅输入 token**: 88 条 (1.8%) - **之前会错误退费，造成亏损**

### 亏损估算

假设这 88 条记录：
- 平均 quota: 3000
- 总计: 88 × 3000 = 264,000 quota
- 如果按 0.3 倍率计算，相当于原价 880,000 quota 的成本

**修复后预计减少的不必要退费**：约 26 万 quota

## 最佳实践

### 对于新增的 AI 提供商

1. **首先确认提供商是否返回计费信息**
   - 查看 API 文档中的 `usage` 字段定义
   - 测试实际返回的数据结构

2. **信任官方计费信息**
   - 如果提供商返回了 `usage`，直接使用，不做额外判断
   - 不要基于 `completion_tokens=0` 就自动退费

3. **只处理明确的失败情况**
   - HTTP 错误码（4xx/5xx）
   - 网络超时
   - 上游明确表示失败且未返回 usage

### 调试和监控

1. **记录退费日志**
   - 所有退费操作都应该有明确的日志
   - 包含 `model_name`、`user_id`、`quota`、`reason`

2. **定期审计**
   - 每周/月检查退费记录
   - 对比上游账单，确认计费准确性

3. **保存上游原始 usage**
   - 在 `other` 字段中保存上游返回的原始计费数据
   - 用于事后审计和对账

## 历史问题

### 2026-06-30 发现的问题

**问题**：Gemini 在安全拦截、客户端中断等场景下返回 `completion_tokens=0`，但系统错误地全额退费

**影响**：88 条日志记录，约 26 万 quota 的不必要退费

**根因**：`RefundIfZeroCompletionTokens` 函数逻辑过于激进，没有区分"上游已处理"和"请求失败"

**修复**：
1. Gemini 完全移除退费调用
2. 其他标准提供商也移除退费调用
3. `RefundIfZeroCompletionTokens` 函数增加安全检查，避免错误退费

## 相关文件

- `/home/ubuntu/new-api/relay/gemini_handler.go` - Gemini 计费处理
- `/home/ubuntu/new-api/relay/compatible_handler.go` - 通用提供商计费处理
- `/home/ubuntu/new-api/relay/image_handler.go` - 图片生成计费处理
- `/home/ubuntu/new-api/service/text_quota.go` - 退费逻辑核心函数
- `/home/ubuntu/new-api/service/quota.go` - 底层 quota 操作

## 更新日志

- **2026-06-30**: 初始版本，修复 Gemini 错误退费问题，优化退费策略
