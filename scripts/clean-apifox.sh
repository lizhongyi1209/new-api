#!/bin/bash
# 删除所有非 Kling 接口和文件夹

set -e

APIFOX_TOKEN="afxp_391009tJvPtAOLalw4RP1V7LdITsiONHaUXR"
PROJECT_ID="8507615"
API_BASE="https://api.apifox.com/api/v1"

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${YELLOW}=== 清理 Apifox 接口 - 仅保留 Kling ===${NC}"
echo ""

# 获取所有接口树
echo -e "${YELLOW}步骤 1: 获取接口列表...${NC}"
tree_data=$(curl -s -X GET \
    "${API_BASE}/projects/${PROJECT_ID}/api-tree-list" \
    -H "Authorization: Bearer ${APIFOX_TOKEN}" \
    -H "X-Apifox-Version: 2024-03-28")

# 保存到临时文件以便调试
echo "$tree_data" > /tmp/apifox_tree.json

# 提取所有文件夹（排除 "图片" 和 "可灵"）
echo -e "${YELLOW}步骤 2: 删除非 Kling 文件夹...${NC}"
folder_count=0

# 使用 jq 递归查找所有文件夹
folders_data=$(echo "$tree_data" | jq -r '.. | select(.folder?.id? != null and .folder.name != "图片" and .folder.name != "可灵") | "\(.folder.id)|\(.folder.name)"' | sort -u)

if [ -n "$folders_data" ]; then
    while IFS='|' read -r folder_id folder_name; do
        if [ -n "$folder_id" ] && [ "$folder_id" != "null" ]; then
            echo -e "${BLUE}  删除文件夹: $folder_name (ID: $folder_id)${NC}"

            delete_response=$(curl -s -w "\n%{http_code}" -X DELETE \
                "${API_BASE}/projects/${PROJECT_ID}/api-tree-folders/${folder_id}" \
                -H "Authorization: Bearer ${APIFOX_TOKEN}" \
                -H "X-Apifox-Version: 2024-03-28")

            http_code=$(echo "$delete_response" | tail -n1)
            if [ "$http_code" -eq 200 ] || [ "$http_code" -eq 204 ]; then
                ((folder_count++))
            else
                body=$(echo "$delete_response" | sed '$d')
                echo -e "${RED}    删除失败 (HTTP $http_code): $body${NC}"
            fi
            sleep 0.1  # 避免请求过快
        fi
    done <<< "$folders_data"
    echo -e "${GREEN}✓ 已删除 $folder_count 个文件夹${NC}"
else
    echo -e "${BLUE}没有需要删除的文件夹${NC}"
fi

# 删除接口（排除 Kling 相关）
echo -e "${YELLOW}步骤 3: 删除非 Kling 接口...${NC}"
api_count=0

# 使用 jq 递归查找所有接口
apis_data=$(echo "$tree_data" | jq -r '.. | select(.api?.id? != null and (.api.name | test("图生视频|文生视频|运动控制|全能视频") | not)) | "\(.api.id)|\(.api.name)"' | sort -u)

if [ -n "$apis_data" ]; then
    while IFS='|' read -r api_id api_name; do
        if [ -n "$api_id" ] && [ "$api_id" != "null" ]; then
            echo -e "${BLUE}  删除接口: $api_name (ID: $api_id)${NC}"

            delete_response=$(curl -s -w "\n%{http_code}" -X DELETE \
                "${API_BASE}/projects/${PROJECT_ID}/apis/${api_id}" \
                -H "Authorization: Bearer ${APIFOX_TOKEN}" \
                -H "X-Apifox-Version: 2024-03-28")

            http_code=$(echo "$delete_response" | tail -n1)
            if [ "$http_code" -eq 200 ] || [ "$http_code" -eq 204 ]; then
                ((api_count++))
            else
                body=$(echo "$delete_response" | sed '$d')
                echo -e "${RED}    删除失败 (HTTP $http_code)${NC}"
            fi
            sleep 0.05  # 避免请求过快
        fi
    done <<< "$apis_data"
    echo -e "${GREEN}✓ 已删除 $api_count 个接口${NC}"
else
    echo -e "${BLUE}没有需要删除的接口${NC}"
fi

echo ""
echo -e "${GREEN}=== 清理完成 ===${NC}"
echo -e "${BLUE}现在项目中只保留 Kling 相关接口${NC}"
echo -e "查看文档: ${YELLOW}https://d3za3h3ks5.apifox.cn${NC}"

# 清理临时文件
rm -f /tmp/apifox_tree.json
