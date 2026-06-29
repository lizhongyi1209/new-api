#!/bin/bash
# Apifox API 文档自动同步脚本（简化版）

set -e

# Apifox 配置
APIFOX_TOKEN="afxp_391009tJvPtAOLalw4RP1V7LdITsiONHaUXR"
PROJECT_ID="8507615"
API_BASE="https://api.apifox.com/api/v1"

# 颜色输出
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${YELLOW}=== Apifox API 文档同步工具 ===${NC}"
echo ""

# 检查文档文件是否存在
if [ ! -f "docs/openapi/relay.json" ]; then
    echo -e "${RED}错误: docs/openapi/relay.json 不存在${NC}"
    exit 1
fi

if [ ! -f "docs/openapi/api.json" ]; then
    echo -e "${RED}错误: docs/openapi/api.json 不存在${NC}"
    exit 1
fi

echo -e "${GREEN}✓ 找到 OpenAPI 文档文件${NC}"
echo ""

# 函数：导入 OpenAPI 文档
import_openapi() {
    local file=$1
    local name=$2

    echo -e "${YELLOW}正在导入 ${name}...${NC}"

    # 构造正确的请求体格式
    local payload=$(jq -n \
        --arg format "openapi" \
        --slurpfile data "$file" \
        '{
            importFormat: $format,
            data: ($data[0] | tostring),
            options: {
                mode: "merge",
                targetFolderId: null,
                enableUpdateApiPath: true,
                enableUpdateApiMethod: true
            }
        }')

    # 发送导入请求
    local response=$(curl -s -w "\n%{http_code}" -X POST \
        "${API_BASE}/projects/${PROJECT_ID}/import-data" \
        -H "Authorization: Bearer ${APIFOX_TOKEN}" \
        -H "Content-Type: application/json" \
        -H "X-Apifox-Version: 2024-03-28" \
        -d "$payload")

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | sed '$d')

    if [ "$http_code" -eq 200 ] || [ "$http_code" -eq 201 ]; then
        echo -e "${GREEN}✓ ${name} 导入成功${NC}"

        # 解析并显示导入统计
        local create_count=$(echo "$body" | jq -r '.data.apiCollection.item.createCount // 0' 2>/dev/null || echo "0")
        local update_count=$(echo "$body" | jq -r '.data.apiCollection.item.updateCount // 0' 2>/dev/null || echo "0")
        local error_count=$(echo "$body" | jq -r '.data.apiCollection.item.errorCount // 0' 2>/dev/null || echo "0")

        echo -e "${BLUE}  - 创建: ${create_count} 个接口${NC}"
        echo -e "${BLUE}  - 更新: ${update_count} 个接口${NC}"
        if [ "$error_count" -gt 0 ]; then
            echo -e "${RED}  - 错误: ${error_count} 个接口${NC}"
        fi
    else
        echo -e "${RED}✗ ${name} 导入失败 (HTTP ${http_code})${NC}"
        echo -e "${RED}响应: ${body}${NC}"

        # 显示详细错误信息
        if echo "$body" | jq -e '.errorMessage' >/dev/null 2>&1; then
            local error_msg=$(echo "$body" | jq -r '.errorMessage')
            echo -e "${RED}错误详情: ${error_msg}${NC}"
        fi

        return 1
    fi
    echo ""
}

# 导入两个文档
import_openapi "docs/openapi/relay.json" "AI模型接口文档"
import_openapi "docs/openapi/api.json" "后台管理接口文档"

echo -e "${GREEN}=== 同步完成 ===${NC}"
echo -e "查看文档: ${YELLOW}https://d3za3h3ks5.apifox.cn${NC}"
