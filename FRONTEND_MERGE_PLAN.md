# 前端完整合并策略（保留自定义功能）

## 🎯 目标
采用 rc.16 官方前端实现，同时保留所有自定义功能

---

## 📦 需要保留的自定义内容

### 后端自定义文件（完全保留）
```
controller/upload.go          # 上传管理控制器
controller/storage.go         # 存储管理控制器
service/storage.go            # 存储服务
service/storage_test.go       # 存储测试
service/r2_upload.go          # R2上传服务
router/api-router.go          # 路由配置（部分自定义）
```

### 前端自定义页面（完全保留）
```
web/classic/src/pages/UploadManagement/index.jsx    # 上传管理页面
web/classic/src/pages/UploadManagement/            # 整个目录
```

### 自定义路由（需合并）
```
/api/aigc/element/*           # 可灵元素管理
/api/aigc/seedance-element/*  # Seedance元素管理
/api/upload-management/*      # 上传管理API
```

---

## 🔧 合并策略

### 步骤 1：创建备份分支
```bash
git checkout custom
git branch custom-backup-$(date +%Y%m%d)
git checkout -b merge-rc16-frontend
```

### 步骤 2：合并 rc.16 前端改进
使用 merge 而不是 cherry-pick，选择性接受官方改动：

```bash
# 合并 rc.16，但不自动提交
git merge --no-commit --no-ff v1.0.0-rc.16
```

### 步骤 3：解决冲突
对于冲突文件：
- **共享组件**（html-content.tsx, rich-content.tsx 等）→ 采用官方版本
- **自定义页面**（UploadManagement）→ 保留我们的版本
- **路由配置**（api-router.go）→ 手动合并
- **翻译文件**（i18n）→ 合并双方内容

### 步骤 4：验证自定义功能
确保以下功能正常：
- ✅ 上传管理页面可访问
- ✅ 可灵元素API正常
- ✅ Seedance元素API正常
- ✅ 本地存储功能正常

---

## ⚠️ 预期冲突文件

### 高冲突文件（需要仔细处理）
```
router/api-router.go                    # 路由冲突（手动合并）
web/default/src/components/html-content.tsx    # 采用官方版
web/default/src/components/rich-content.tsx    # 采用官方版
web/default/src/i18n/locales/*.json           # 合并翻译
```

### 自定义文件（完全保留）
```
controller/upload.go            # 保留
controller/storage.go           # 保留
service/storage.go              # 保留
web/classic/src/pages/UploadManagement/*  # 保留
```

---

## 🎬 执行计划

我将按照以下步骤执行：

1. ✅ 创建备份和工作分支
2. 🔄 执行 merge v1.0.0-rc.16
3. 🔧 解决冲突：
   - 前端共享组件 → 采用官方
   - 自定义功能文件 → 保留
   - 路由配置 → 手动合并
4. 🏗️ 测试构建
5. ✅ 验证自定义功能
6. 📦 合并到 custom 分支

预计耗时：20-30 分钟
成功率：85%（可能需要手动调整路由）

---

## ❓ 确认执行？

这个操作会：
- ✅ 更新所有前端组件到 rc.16
- ✅ 保留你的上传管理功能
- ✅ 保留可灵/Seedance元素管理
- ✅ 保留本地存储功能
- ⚠️ 可能需要手动调整部分路由代码

是否开始执行？
