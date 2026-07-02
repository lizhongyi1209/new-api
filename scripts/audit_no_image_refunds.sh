#!/bin/bash
# 审计"上游未返回图片数据"失败任务的上游计费情况
# 用于判断是否存在"上游已扣费但本站退款"的双重损失

set -euo pipefail

# 默认查询最近24小时
HOURS_AGO=${1:-24}
TIMESTAMP_START=$(($(date +%s) - HOURS_AGO * 3600))

echo "=========================================="
echo "上游未返回图片数据 - 计费审计报告"
echo "时间范围: 最近 ${HOURS_AGO} 小时"
echo "=========================================="
echo ""

# 1. 总体统计
echo "1. 总体统计"
docker exec postgres psql -U root -d new-api -t -c "
SELECT
  COUNT(*) AS total_failures,
  COUNT(CASE WHEN (private_data::jsonb->'error_detail'->>'upstream_prompt_tokens')::int > 0
              OR (private_data::jsonb->'error_detail'->>'upstream_completion_tokens')::int > 0
         THEN 1 END) AS with_upstream_usage,
  SUM(quota) AS total_refunded_quota,
  SUM(CASE WHEN (private_data::jsonb->'error_detail'->>'upstream_prompt_tokens')::int > 0
               OR (private_data::jsonb->'error_detail'->>'upstream_completion_tokens')::int > 0
          THEN quota ELSE 0 END) AS refunded_with_usage
FROM tasks
WHERE platform = 'generate_image'
  AND status = 'FAILURE'
  AND fail_reason = '上游未返回图片数据'
  AND finish_time > ${TIMESTAMP_START};
" | awk 'NF' | sed 's/^/  /'

echo ""
echo "2. 疑似双重损失案例 (上游有token用量但本站全额退款)"
docker exec postgres psql -U root -d new-api -x -c "
SELECT
  task_id,
  properties::jsonb->>'origin_model_name' AS model,
  channel_id,
  quota,
  TO_TIMESTAMP(finish_time) AS finish_time,
  private_data::jsonb->'error_detail'->>'upstream_prompt_tokens' AS prompt_tokens,
  private_data::jsonb->'error_detail'->>'upstream_completion_tokens' AS completion_tokens,
  (COALESCE((private_data::jsonb->'error_detail'->>'upstream_prompt_tokens')::int, 0) +
   COALESCE((private_data::jsonb->'error_detail'->>'upstream_completion_tokens')::int, 0)) AS total_tokens
FROM tasks
WHERE platform = 'generate_image'
  AND status = 'FAILURE'
  AND fail_reason = '上游未返回图片数据'
  AND finish_time > ${TIMESTAMP_START}
  AND ((private_data::jsonb->'error_detail'->>'upstream_prompt_tokens')::int > 0
       OR (private_data::jsonb->'error_detail'->>'upstream_completion_tokens')::int > 0)
ORDER BY finish_time DESC
LIMIT 10;
"

echo ""
echo "3. 按渠道分组统计"
docker exec postgres psql -U root -d new-api -c "
SELECT
  t.channel_id,
  c.name AS channel_name,
  COUNT(*) AS failure_count,
  COUNT(CASE WHEN (t.private_data::jsonb->'error_detail'->>'upstream_prompt_tokens')::int > 0
              OR (t.private_data::jsonb->'error_detail'->>'upstream_completion_tokens')::int > 0
         THEN 1 END) AS with_usage_count,
  SUM(t.quota) AS total_refunded,
  ROUND(SUM(t.quota)::numeric / 500000, 2) AS refunded_usd
FROM tasks t
LEFT JOIN channels c ON t.channel_id = c.id
WHERE t.platform = 'generate_image'
  AND t.status = 'FAILURE'
  AND t.fail_reason = '上游未返回图片数据'
  AND t.finish_time > ${TIMESTAMP_START}
GROUP BY t.channel_id, c.name
ORDER BY with_usage_count DESC;
"

echo ""
echo "=========================================="
echo "说明:"
echo "- total_failures: 总失败次数"
echo "- with_upstream_usage: 上游有token用量的失败次数"
echo "- total_refunded_quota: 已退款总额度"
echo "- refunded_with_usage: 有上游用量但仍退款的总额度(疑似双重损失)"
echo ""
echo "建议: 将 with_usage_count 非零的渠道与上游账单核对"
echo "=========================================="
