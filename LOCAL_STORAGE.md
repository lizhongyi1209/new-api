# 本地存储功能说明

## 概述

从本次更新开始，`api.o1key.cn` 域名的图片/视频上传已切换为**本地存储**，文件直接保存在服务器磁盘，并通过 Tencent EdgeOne 全球加速 CDN 分发，提高访问速度和成功率。

## 架构变化

### 之前：
```
客户端 → api.o1key.cn → 上传到阿里云 OSS → 返回 OSS URL
```

### 现在：
```
客户端 → api.o1key.cn → 保存到服务器本地 → 返回 https://api.o1key.cn/upload/xxx
                      ↓
            Tencent EdgeOne CDN 全球加速
```

## 域名存储策略

| 域名 | 存储提供商 | 说明 |
|------|-----------|------|
| `api.o1key.cn` | **本地存储** | 利用 EdgeOne 全球加速 |
| `cf-api.o1key.cn` | R2 | Cloudflare CDN |
| `cf-api.o1key.com` | R2 | Cloudflare CDN |
| `api.o1key.com` | R2 | 直连 |

## 配置说明

### 环境变量

```bash
# 本地存储公共访问基础 URL（默认值）
LOCAL_PUBLIC_BASE_URL=https://api.o1key.cn

# 本地存储目录（默认值）
LOCAL_UPLOAD_DIR=uploads

# 可选：覆盖默认域名映射
LOCAL_STORAGE_HOSTS=api.o1key.cn,custom.domain.com
```

### Nginx 配置

已在 `/etc/nginx/conf.d/api.o1key.com.conf` 中为 `api.o1key.cn` 添加：

```nginx
location /upload/ {
    alias /home/ubuntu/new-api/uploads/;
    
    # CDN 缓存：30天
    expires 30d;
    add_header Cache-Control "public, immutable";
    
    # 跨域支持
    add_header Access-Control-Allow-Origin "*";
    
    # 安全：禁止执行脚本
    location ~ \.(php|pl|py|jsp|asp|sh|cgi)$ {
        return 403;
    }
}
```

## API 使用示例

### 1. 获取预签名上传 URL

**请求：**
```bash
curl -X POST https://api.o1key.cn/v1/storage/presign \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "filename": "example.jpg",
    "content_type": "image/jpeg",
    "size": 102400
  }'
```

**响应（本地存储）：**
```json
{
  "method": "POST",
  "upload_url": "https://api.o1key.cn/v1/storage/local/upload?object_key=uploads/uuid_example.jpg",
  "headers": {
    "Content-Type": "image/jpeg"
  },
  "public_url": "https://api.o1key.cn/upload/uploads/uuid_example.jpg",
  "object_key": "uploads/uuid_example.jpg",
  "expires_at": 1234567890,
  "provider": "local"
}
```

### 2. 上传文件

```bash
curl -X POST "https://api.o1key.cn/v1/storage/local/upload?object_key=uploads/uuid_example.jpg" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: image/jpeg" \
  --data-binary @your-image.jpg
```

### 3. 访问文件

```bash
# 直接访问（通过 EdgeOne CDN）
https://api.o1key.cn/upload/uploads/uuid_example.jpg
```

## 优势

1. **全球加速**：利用 Tencent EdgeOne 的全球节点，提高访问速度
2. **降低成本**：减少对第三方存储的依赖
3. **统一域名**：所有资源都在 api.o1key.cn 下
4. **CDN 缓存**：30天缓存，减少源站压力

## 注意事项

### 磁盘空间管理

```bash
# 查看 uploads 目录大小
du -sh /home/ubuntu/new-api/uploads

# 查看磁盘使用情况
df -h /home/ubuntu
```

### 备份建议

本地文件建议定期备份。可以考虑：

1. **定期同步到 R2**（异步备份）
2. **数据库备份一起打包**
3. **使用 rsync 到远程服务器**

### 清理策略

如需定期清理旧文件：

```bash
# 查找超过 90 天的文件
find /home/ubuntu/new-api/uploads -type f -mtime +90

# 删除超过 90 天的文件（谨慎使用！）
find /home/ubuntu/new-api/uploads -type f -mtime +90 -delete
```

## 权限要求

确保目录权限正确：

```bash
# uploads 目录需要对 www-data 用户可读
chmod 755 /home/ubuntu/new-api/uploads

# /home/ubuntu 需要 others execute 权限（让 nginx 可以穿越）
chmod o+x /home/ubuntu

# 新上传的文件自动设置为 0644
```

## 回滚方案

如需切换回 OSS/R2：

**方法 1：修改环境变量**
```bash
# 在 .env 或 docker-compose.yml 中
LOCAL_STORAGE_HOSTS=""  # 清空，回退到 R2
# 或
ALIYUN_OSS_STORAGE_HOSTS="api.o1key.cn"  # 使用 OSS
```

**方法 2：使用 kill-switch**
```bash
DISABLE_ALIYUN_OSS=false  # 取消 OSS 封禁
```

重启服务：
```bash
docker compose up -d
```

## 监控

### 查看上传日志

```bash
# nginx 访问日志
sudo tail -f /var/log/nginx/upload_access.log

# nginx 错误日志
sudo tail -f /var/log/nginx/upload_error.log

# 应用日志
docker compose logs -f new-api | grep storage
```

### 性能指标

- **EdgeOne CDN 命中率**：通过腾讯云控制台查看
- **带宽使用**：监控服务器出站流量
- **磁盘 I/O**：`iostat -x 1`

## 常见问题

### Q: 文件访问返回 403？

检查权限：
```bash
namei -l /home/ubuntu/new-api/uploads/your-file.jpg
```

确保：
- `/home/ubuntu` 有 `o+x` 权限
- `uploads/` 目录是 `755`
- 文件是 `644`

### Q: 如何切换特定域名的存储？

修改环境变量：
```bash
# 让 api.o1key.cn 使用 R2
R2_STORAGE_HOSTS="api.o1key.cn,cf-api.o1key.cn"
LOCAL_STORAGE_HOSTS=""
```

### Q: 上传的图片没有压缩？

使用 `compression` 参数（在代码中调用 `UploadBase64ImageToLocalCompressed`）：
- `"jpg"` - JPEG 压缩（quality 95）
- `"webp"` - WebP 压缩
- `"origin"` - 保持原格式

## 测试

运行测试脚本：

```bash
/tmp/test_upload.sh YOUR_API_TOKEN
```

## 更新日志

- **2026-07-05**: 初始版本，api.o1key.cn 切换到本地存储
