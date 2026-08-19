#!/bin/bash
# test_size_params.sh - 测试 1K, 2K, 4K 参数

API_BASE="https://cf-api.o1key.com"
TOKEN="sk-GknXryJw9kQbiufQAJ4tNiqEWt3iEPUG2yjBjDN8FVzKYLxQ"

echo "🧪 测试 nano-banana-pro size 参数支持"
echo "=========================================="
echo ""

# 测试函数
test_size() {
    local size=$1
    echo "📊 测试 size=$size..."

    response=$(curl -s -X POST "$API_BASE/async/v1/images/generations" \
        -H "Authorization: Bearer $TOKEN" \
        -H "Content-Type: application/json" \
        -d "{
            \"model\": \"nano-banana-pro\",
            \"prompt\": \"一只可爱的猫\",
            \"aspect_ratio\": \"1:1\",
            \"size\": \"$size\",
            \"n\": 1
        }")

    # 检查响应
    if echo "$response" | jq -e '.task_id' > /dev/null 2>&1; then
        task_id=$(echo "$response" | jq -r '.task_id')
        echo "  ✅ 成功: task_id=$task_id"
        return 0
    else
        error=$(echo "$response" | jq -r '.error // .message // "未知错误"')
        echo "  ❌ 失败: $error"
        return 1
    fi
}

# 测试所有 size 值
sizes=("1K" "2K" "4K")
success_count=0
fail_count=0

for size in "${sizes[@]}"; do
    if test_size "$size"; then
        ((success_count++))
    else
        ((fail_count++))
    fi
    echo ""
    sleep 1
done

# 测试无效值
echo "📊 测试无效 size 值..."
response=$(curl -s -X POST "$API_BASE/async/v1/images/generations" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
        "model": "nano-banana-pro",
        "prompt": "一只可爱的猫",
        "aspect_ratio": "1:1",
        "size": "HD",
        "n": 1
    }')

if echo "$response" | jq -e '.error' > /dev/null 2>&1; then
    echo "  ✅ 正确拒绝了无效值 'HD'"
else
    echo "  ⚠️  未正确处理无效值"
fi

echo ""
echo "=========================================="
echo "📈 测试结果汇总"
echo "  成功: $success_count"
echo "  失败: $fail_count"
echo "=========================================="

if [ $fail_count -eq 0 ]; then
    echo "✅ 所有测试通过！"
    exit 0
else
    echo "❌ 部分测试失败"
    exit 1
fi
