#!/bin/bash
# M6 功能验证脚本 - Alpha Search 端点和渠道类型测试

set -e

BASE_URL="http://localhost:3302"
ADMIN_TOKEN=""

echo "=== M6 功能验证测试 ==="
echo ""

# 1. 测试 Alpha Search 端点路由是否存在
echo "1. 测试 Alpha Search 端点路由..."
response=$(curl -s -w "\n%{http_code}" -X POST \
  "${BASE_URL}/v1/alpha/search" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4","input":[{"role":"user","content":"test"}]}')

http_code=$(echo "$response" | tail -n1)
body=$(echo "$response" | head -n-1)

if [ "$http_code" = "401" ] || [ "$http_code" = "403" ]; then
  echo "✓ Alpha Search 路由存在 (返回认证错误，符合预期)"
elif [ "$http_code" = "404" ]; then
  echo "✗ Alpha Search 路由不存在 (404)"
  exit 1
else
  echo "✓ Alpha Search 路由存在 (HTTP $http_code)"
fi

# 2. 检查渠道类型常量
echo ""
echo "2. 检查渠道类型定义..."
if docker compose -f docker-compose.upstream-test.yml exec -T upstream-test-api grep -q "ChannelTypeSub2API" /constant/channel.go 2>/dev/null; then
  echo "✓ Sub2API 渠道类型已定义"
else
  echo "✗ Sub2API 渠道类型未找到"
fi

# 3. 测试数据库迁移状态
echo ""
echo "3. 测试数据库迁移..."

# PostgreSQL
echo "  - PostgreSQL: 检查表结构..."
PG_TEST=$(docker compose -f docker-compose.upstream-test.yml exec -T upstream-test-postgres psql -U testuser -d testdb -c "\dt" 2>&1 | grep -c "channels\|tokens\|users" || true)
if [ "$PG_TEST" -ge 3 ]; then
  echo "    ✓ PostgreSQL 迁移成功"
else
  echo "    ✗ PostgreSQL 迁移失败"
fi

# MySQL
echo "  - MySQL: 检查表结构..."
MYSQL_TEST=$(docker compose -f docker-compose.upstream-test.yml exec -T upstream-test-mysql mysql -utestuser -ptestpass testdb -e "SHOW TABLES;" 2>&1 | grep -c "channels\|tokens\|users" || true)
if [ "$MYSQL_TEST" -ge 3 ]; then
  echo "    ✓ MySQL 迁移成功"
else
  echo "    ✗ MySQL 迁移失败"
fi

# 4. 验证 API 类型常量
echo ""
echo "4. 验证 API 类型常量..."
docker compose -f docker-compose.upstream-test.yml logs upstream-test-api 2>&1 | grep -q "started" && echo "✓ API 启动日志正常"

# 5. 检查工具计费相关文件
echo ""
echo "5. 检查核心文件存在性..."
files_to_check=(
  "relay/alpha_search_handler.go"
  "relay/channel/sub2api/adaptor.go"
  "dto/alpha_search_request.go"
  "constant/api_type.go"
)

all_exist=true
for file in "${files_to_check[@]}"; do
  if [ -f "$file" ]; then
    echo "  ✓ $file"
  else
    echo "  ✗ $file 不存在"
    all_exist=false
  fi
done

echo ""
echo "=== 测试完成 ==="
if [ "$all_exist" = true ]; then
  echo "状态: ✓ 所有核心文件已就位"
  exit 0
else
  echo "状态: ✗ 部分文件缺失"
  exit 1
fi
