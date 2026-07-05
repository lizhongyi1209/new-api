## 本地存储部署清单

### ✅ 已完成的工作

#### 1. 代码实现
- [x] 添加 `ImageStorageProviderLocal` 常量
- [x] 修改 `SelectImageStorageProvider` 逻辑，api.o1key.cn 默认走本地存储
- [x] 实现 `GenerateLocalPresignedUploadURL` - 生成本地上传 URL
- [x] 实现 `UploadBase64ImageToLocalCompressed` - base64 图片上传
- [x] 添加 `UploadLocalFile` 控制器 - 处理文件上传
- [x] 注册路由 `/v1/storage/local/upload`
- [x] 添加静态文件路由 `router.Static("/upload", uploadDir)`

#### 2. Nginx 配置
- [x] 在 api.o1key.cn 的 443 server block 添加 `/upload/` location
- [x] 配置静态文件 serve：`alias /home/ubuntu/new-api/uploads/`
- [x] 设置 CDN 缓存：`expires 30d`
- [x] 添加 CORS 头：`Access-Control-Allow-Origin "*"`
- [x] 安全配置：禁止脚本执行
- [x] 日志配置：upload_access.log / upload_error.log

#### 3. 权限设置
- [x] 创建 uploads 目录：`mkdir -p uploads`
- [x] 设置目录权限：`chmod 755 uploads`
- [x] 修复父目录权限：`chmod o+x /home/ubuntu`
- [x] 添加 .gitignore：`uploads/`

#### 4. 部署
- [x] Docker 构建成功
- [x] 服务启动正常
- [x] Nginx reload 成功
- [x] 静态文件访问测试通过

#### 5. 文档
- [x] LOCAL_STORAGE.md - 完整使用文档
- [x] monitor-storage.sh - 监控脚本
- [x] 测试脚本：/tmp/test_upload.sh

#### 6. Git 提交
- [x] 代码提交：9995218df
- [x] 推送到 origin/custom

### 🎯 实现效果

**URL 格式：**
```
https://api.o1key.cn/upload/uploads/uuid_filename.jpg
```

**存储路径：**
```
/home/ubuntu/new-api/uploads/uploads/uuid_filename.jpg
```

**优势：**
- ✅ 利用 Tencent EdgeOne 全球加速
- ✅ 30天 CDN 缓存
- ✅ 统一域名
- ✅ 降低存储成本
- ✅ 支持跨域访问

### 📊 当前状态

```bash
磁盘使用: 32% (261G/878G)
uploads 目录: 12K
文件数: 1 (测试文件)
```

### 🔧 维护命令

```bash
# 监控存储
/home/ubuntu/new-api/scripts/monitor-storage.sh

# 查看上传日志
sudo tail -f /var/log/nginx/upload_access.log

# 清理旧文件（>90天）
find uploads -type f -mtime +90 -delete

# 查看磁盘
df -h /home/ubuntu
du -sh uploads
```

### ⚙️ 环境变量

```bash
# 可选配置（使用默认值即可）
LOCAL_PUBLIC_BASE_URL=https://api.o1key.cn
LOCAL_UPLOAD_DIR=uploads
LOCAL_STORAGE_HOSTS=api.o1key.cn
```

### 🔄 回滚方案

如需切换回 OSS/R2：

```bash
# 方法 1: 修改域名映射
LOCAL_STORAGE_HOSTS=""
R2_STORAGE_HOSTS="api.o1key.cn"

# 方法 2: 恢复 OSS
ALIYUN_OSS_STORAGE_HOSTS="api.o1key.cn"
DISABLE_ALIYUN_OSS=false

# 重启
docker compose up -d
```

### 📝 待办事项

- [ ] 监控磁盘使用，设置告警（建议 >100GB 时）
- [ ] 考虑定期备份策略（rsync 到 R2？）
- [ ] 可选：实现自动清理旧文件（cron job）
- [ ] 可选：压缩历史图片（从 png 转 jpg）

### 🧪 测试

```bash
# 运行完整测试
/tmp/test_upload.sh YOUR_API_TOKEN

# 手动测试静态访问
curl -I https://api.o1key.cn/upload/test.txt
```

### 📦 备份建议

```bash
# 定期备份 uploads 到远程
rsync -avz /home/ubuntu/new-api/uploads/ backup-server:/backup/uploads/

# 或者同步到 R2（异步）
# 可以写个定时任务，每天将本地文件上传到 R2 作为冷备份
```

---

**部署时间**: 2026-07-05  
**版本**: custom@9995218df  
**状态**: ✅ 生产环境运行中
