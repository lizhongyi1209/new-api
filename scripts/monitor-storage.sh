#!/bin/bash
#
# 本地存储监控脚本
# 用途：监控 uploads 目录大小，清理过期文件
#

UPLOAD_DIR="${LOCAL_UPLOAD_DIR:-uploads}"
MAX_DAYS=90  # 文件保留天数
ALERT_SIZE_GB=50  # 磁盘使用超过此值时告警

# 颜色输出
RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

echo "=== 本地存储监控 ==="
echo "时间: $(date)"
echo ""

# 1. 磁盘空间检查
echo "1. 磁盘空间："
df -h /home/ubuntu | tail -1
echo ""

# 2. uploads 目录大小
echo "2. uploads 目录："
UPLOAD_SIZE=$(du -sh $UPLOAD_DIR 2>/dev/null | cut -f1)
UPLOAD_SIZE_MB=$(du -sm $UPLOAD_DIR 2>/dev/null | cut -f1)
echo "  总大小: $UPLOAD_SIZE"

# 3. 文件统计
FILE_COUNT=$(find $UPLOAD_DIR -type f 2>/dev/null | wc -l)
echo "  文件数: $FILE_COUNT"
echo ""

# 4. 按类型统计
echo "3. 文件类型分布："
find $UPLOAD_DIR -type f 2>/dev/null | sed 's/.*\.//' | sort | uniq -c | sort -rn | head -10
echo ""

# 5. 检查旧文件
echo "4. 过期文件检查（>$MAX_DAYS 天）："
OLD_FILES=$(find $UPLOAD_DIR -type f -mtime +$MAX_DAYS 2>/dev/null | wc -l)
if [ $OLD_FILES -gt 0 ]; then
    OLD_SIZE=$(find $UPLOAD_DIR -type f -mtime +$MAX_DAYS -exec du -ch {} + 2>/dev/null | tail -1 | cut -f1)
    echo -e "  ${YELLOW}发现 $OLD_FILES 个过期文件，总大小 $OLD_SIZE${NC}"
    echo "  执行清理命令："
    echo "    find $UPLOAD_DIR -type f -mtime +$MAX_DAYS -delete"
else
    echo -e "  ${GREEN}没有过期文件${NC}"
fi
echo ""

# 6. 最大的文件
echo "5. 最大的 10 个文件："
find $UPLOAD_DIR -type f -exec ls -lh {} \; 2>/dev/null | sort -k5 -hr | head -10 | awk '{print "  " $5 "\t" $9}'
echo ""

# 7. 最近上传的文件
echo "6. 最近上传的 5 个文件："
find $UPLOAD_DIR -type f -printf '%TY-%Tm-%Td %TH:%TM\t%s\t%p\n' 2>/dev/null | sort -r | head -5 | awk '{printf "  %s %s\t%d bytes\t%s\n", $1, $2, $3, $4}'
echo ""

# 8. 告警检查
if [ $UPLOAD_SIZE_MB -gt $((ALERT_SIZE_GB * 1024)) ]; then
    echo -e "${RED}⚠ 告警：uploads 目录大小超过 ${ALERT_SIZE_GB}GB！${NC}"
    echo ""
fi

# 9. 建议
echo "7. 维护建议："
if [ $OLD_FILES -gt 100 ]; then
    echo -e "  ${YELLOW}→ 建议清理过期文件（>$MAX_DAYS 天）${NC}"
fi

if [ $UPLOAD_SIZE_MB -gt 10240 ]; then
    echo -e "  ${YELLOW}→ 考虑设置定期备份到 R2${NC}"
fi

DISK_USAGE=$(df /home/ubuntu | tail -1 | awk '{print $5}' | sed 's/%//')
if [ $DISK_USAGE -gt 80 ]; then
    echo -e "  ${RED}→ 磁盘使用率超过 80%，需要立即清理！${NC}"
fi

echo ""
echo "=== 监控完成 ==="
