#!/bin/bash
# 覆盖导入 Kling 接口文档（清空其他接口）

set -e

APIFOX_TOKEN="afxp_391009tJvPtAOLalw4RP1V7LdITsiONHaUXR"
PROJECT_ID="8507615"
API_BASE="https://api.apifox.com/api/v1"

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${YELLOW}=== Apifox 文档覆盖导入 - 仅保留 Kling 接口 ===${NC}"
echo ""

if [ ! -f "docs/openapi/kling.json" ]; then
    echo -e "${RED}错误: docs/openapi/kling.json 不存在${NC}"
    exit 1
fi

# 使用 overwrite 模式导入，这会清空所有接口并重新导入
echo -e "${YELLOW}正在以覆盖模式导入 Kling 文档...${NC}"

openapi_content=$(cat docs/openapi/kling.json)

request_body=$(cat <<EOF
{
  "importFormat": "openapi",
  "data": $(echo "$openapi_content" | jq -c .),
  "options": {
    "mode": "overwrite",
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
    echo -e "${GREEN}✓ Kling 接口文档导入成功（已清空其他接口）${NC}"

    create_count=$(echo "$body" | jq -r '.data.apiCollection.item.createCount // 0')
    update_count=$(echo "$body" | jq -r '.data.apiCollection.item.updateCount // 0')
    delete_count=$(echo "$body" | jq -r '.data.apiCollection.item.deleteCount // 0')

    echo -e "${BLUE}  - 创建: ${create_count} 个接口${NC}"
    echo -e "${BLUE}  - 更新: ${update_count} 个接口${NC}"
    echo -e "${BLUE}  - 删除: ${delete_count} 个接口${NC}"
else
    echo -e "${RED}✗ 导入失败 (HTTP ${http_code})${NC}"
    echo -e "${RED}响应: ${body}${NC}"
    exit 1
fi

echo ""
echo -e "${GREEN}=== 完成 ===${NC}"
echo -e "查看文档: ${YELLOW}https://d3za3h3ks5.apifox.cn${NC}"
