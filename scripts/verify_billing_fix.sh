#!/bin/bash
# 验证计费退费优化脚本
# 用于检查修复后的计费行为是否正确

echo "================================================"
echo "计费退费优化验证脚本"
echo "================================================"
echo ""

# 检查是否还有 RefundIfZeroCompletionTokens 的调用（除了函数定义本身）
echo "1. 检查代码中是否还有活跃的 RefundIfZeroCompletionTokens 调用..."
active_calls=$(grep -r "service\.RefundIfZeroCompletionTokens" /home/ubuntu/new-api/relay --include="*.go" 2>/dev/null | grep -v "^Binary" | grep -v "// " | wc -l)

if [ "$active_calls" -eq 0 ]; then
    echo "✅ 已成功移除所有主要 relay handler 中的 RefundIfZeroCompletionTokens 调用"
else
    echo "⚠️  仍有 $active_calls 个活跃调用"
    grep -r "service\.RefundIfZeroCompletionTokens" /home/ubuntu/new-api/relay --include="*.go" 2>/dev/null | grep -v "^Binary" | grep -v "// "
fi

echo ""
echo "2. 检查最近的 Gemini 日志（查看是否还有错误退费）..."

# 查询最近 24 小时内 completion_tokens=0 但 prompt_tokens>0 的 Gemini 记录
docker exec postgres psql -U root -d new-api -c "
SELECT
    COUNT(*) as total_records,
    SUM(CASE WHEN completion_tokens = 0 AND prompt_tokens > 0 THEN 1 ELSE 0 END) as zero_completion_with_prompt,
    SUM(CASE WHEN completion_tokens = 0 AND prompt_tokens > 0 THEN quota ELSE 0 END) as total_quota_at_risk
FROM logs
WHERE model_name LIKE '%gemini%'
AND created_at > EXTRACT(EPOCH FROM NOW() - INTERVAL '24 hours')::bigint
" 2>&1 | tail -5

echo ""
echo "3. 检查退费逻辑优化..."
if grep -q "block_reason\|PROHIBITED_CONTENT\|client_gone" /home/ubuntu/new-api/service/text_quota.go; then
    echo "✅ RefundIfZeroCompletionTokens 函数已添加安全检查"
else
    echo "⚠️  RefundIfZeroCompletionTokens 函数可能缺少安全检查"
fi

echo ""
echo "4. 检查文档..."
if [ -f "/home/ubuntu/new-api/docs/billing_refund_policy.md" ]; then
    echo "✅ 计费退费策略文档已创建"
else
    echo "⚠️  缺少策略文档"
fi

echo ""
echo "================================================"
echo "建议的后续监控："
echo "================================================"
echo "1. 观察接下来 24-48 小时的日志，确认没有异常退费"
echo "2. 对比上游账单（如 Google Cloud），确认计费准确性"
echo "3. 监控用户反馈，是否有计费异常的投诉"
echo ""
echo "查询命令："
echo "  # 查看今天的 Gemini completion=0 记录"
echo "  docker exec postgres psql -U root -d new-api -c \\"
echo "    \"SELECT id, model_name, prompt_tokens, completion_tokens, quota, content, other \\"
echo "    FROM logs WHERE model_name LIKE '%gemini%' AND completion_tokens = 0 \\"
echo "    AND created_at > EXTRACT(EPOCH FROM NOW() - INTERVAL '1 day')::bigint \\"
echo "    ORDER BY id DESC LIMIT 20;\""
echo ""
echo "================================================"
