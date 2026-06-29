#!/bin/bash
# 清空并导入 Kling 接口文档

set -e

APIFOX_TOKEN="afxp_391009tJvPtAOLalw4RP1V7LdITsiONHaUXR"
PROJECT_ID="8507615"
API_BASE="https://api.apifox.com/api/v1"

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${YELLOW}=== Apifox 文档重建 - Kling 接口 ===${NC}"
echo ""

# 步骤 1: 清空现有接口
echo -e "${YELLOW}步骤 1: 清空现有接口...${NC}"
clear_response=$(curl -s -X POST \
    "${API_BASE}/projects/${PROJECT_ID}/clear-data" \
    -H "Authorization: Bearer ${APIFOX_TOKEN}" \
    -H "Content-Type: application/json" \
    -H "X-Apifox-Version: 2024-03-28" \
    -d '{
        "clearApiCollection": true,
        "clearSchemaCollection": true,
        "clearTestCaseCollection": true
    }')

if echo "$clear_response" | jq -e '.success' >/dev/null 2>&1; then
    echo -e "${GREEN}✓ 接口已清空${NC}"
else
    echo -e "${RED}✗ 清空接口失败${NC}"
    echo "$clear_response" | jq .
    exit 1
fi
echo ""

# 步骤 2: 导入 Kling 文档
echo -e "${YELLOW}步骤 2: 导入 Kling OpenAPI 文档...${NC}"

if [ ! -f "docs/openapi/kling.json" ]; then
    echo -e "${RED}错误: docs/openapi/kling.json 不存在${NC}"
    exit 1
fi

# 读取文档并构造导入请求
openapi_content=$(cat docs/openapi/kling.json)

request_body=$(cat <<EOF
{
  "importFormat": "openapi",
  "data": $(echo "$openapi_content" | jq -c .),
  "options": {
    "mode": "merge",
    "targetFolderId": null,
    "enableUpdateApiPath": true,
    "enableUpdateApiMethod": true
  }
}
EOF
)

import_response=$(curl -s -w "\n%{http_code}" -X POST \
    "${API_BASE}/projects/${PROJECT_ID}/import-data" \
    -H "Authorization: Bearer ${APIFOX_TOKEN}" \
    -H "Content-Type: application/json" \
    -H "X-Apifox-Version: 2024-03-28" \
    -d "$request_body")

http_code=$(echo "$import_response" | tail -n1)
body=$(echo "$import_response" | sed '$d')

if [ "$http_code" -eq 200 ] || [ "$http_code" -eq 201 ]; then
    echo -e "${GREEN}✓ Kling 接口文档导入成功${NC}"

    # 解析并显示导入统计
    create_count=$(echo "$body" | jq -r '.data.apiCollection.item.createCount // 0')
    update_count=$(echo "$body" | jq -r '.data.apiCollection.item.updateCount // 0')

    echo -e "${BLUE}  - 创建: ${create_count} 个接口${NC}"
    echo -e "${BLUE}  - 更新: ${update_count} 个接口${NC}"
else
    echo -e "${RED}✗ 导入失败 (HTTP ${http_code})${NC}"
    echo -e "${RED}响应: ${body}${NC}"
    exit 1
fi

echo ""
echo -e "${GREEN}=== 文档重建完成 ===${NC}"
echo -e "查看文档: ${YELLOW}https://d3za3h3ks5.apifox.cn${NC}"
echo ""
echo -e "${BLUE}已创建的接口分类：${NC}"
echo -e "${BLUE}  • 图片 / 可灵${NC}"
echo -e "${BLUE}    - 图生视频 (image2video)${NC}"
echo -e "${BLUE}    - 文生视频 (text2video)${NC}"
echo -e "${BLUE}    - 运动控制 (motion-control)${NC}"
echo -e "${BLUE}    - 全能视频 (omni-video)${NC}"
