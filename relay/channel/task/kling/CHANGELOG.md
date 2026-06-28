# Kling 图生视频适配器更新日志

## 2026-06-28 - Omni Video API 完整适配（基于官方最新文档）

### 新增
- ✅ **添加官方完整 API 文档** `KLING_OMNI_VIDEO_API.md`
  - 包含所有 Omni Video API 参数说明
  - 6 种使用场景示例（图片/主体参考、视频编辑、视频参考、首尾帧、文生视频、多镜头）
  - FAQ 和最佳实践
  - 计费说明和能力地图

### 改进
- ✅ **multi_shot 字段增强**：现在同时支持 boolean 和 string 两种格式
  - 优先解析 boolean 类型（官方标准）
  - 回退到 string 解析（"true"/"false"）以保持向后兼容
  - 代码位置：`relay/channel/task/kling/adaptor.go:440-453`

### 完整功能清单（已验证与官方文档一致）

#### 核心功能
- ✅ **多镜头视频生成**
  - `multi_shot`: 是否生成多镜头视频
  - `shot_type`: 分镜方式（customize/intelligence）
  - `multi_prompt`: 各分镜信息（最多6个分镜）
  
- ✅ **图片参考列表** (`image_list`)
  - 支持主体、场景、风格参考
  - 支持首帧/尾帧视频生成（`type: first_frame/end_frame`）
  - 图片数量限制：根据主体类型和参考视频动态调整

- ✅ **主体库引用** (`element_list`)
  - 支持视频定制主体（视频角色主体）
  - 支持图片定制主体（多图主体）
  - 最多支持 3 个视频角色主体

- ✅ **视频参考/编辑** (`video_list`)
  - `refer_type: base` - 视频编辑（修改内容、切换视角）
  - `refer_type: feature` - 视频参考（生成上下镜头、参考风格运镜）
  - `keep_original_sound: yes/no` - 是否保留原声

- ✅ **音色定制** (`voice_list`)
  - 支持自定义音色（最多2个）
  - Prompt 中使用 `<<<voice_N>>>` 引用

#### 生成控制
- ✅ `sound`: 声音生成控制（on/off）
- ✅ `mode`: 生成模式（std/pro/4k）
- ✅ `aspect_ratio`: 画面比例（16:9/9:16/1:1）
- ✅ `duration`: 视频时长（3-15秒，根据模型不同）

#### 高级功能
- ✅ `watermark_info`: 水印控制
- ✅ `callback_url`: 任务回调通知
- ✅ `external_task_id`: 自定义任务ID

#### 计费
- ✅ **精确的差额结算**
  - 从 `final_unit_deduction` 字段获取实际成本（RMB）
  - 自动转换为配额单位，考虑汇率（USDExchangeRate）和组倍率（GroupRatio）
  - 保留小数精度，避免四舍五入损失
  - 代码位置：`relay/channel/task/kling/adaptor.go:662-718`

### 支持的模型
- `kling-video-o1`（默认）- 3-10秒视频
- `kling-v3-omni` - 3-15秒视频，增强多镜头功能
- `kling-v1`, `kling-v1-6`, `kling-v2-6`, `kling-v2-master`, `kling-v3`

### API 端点
- ✅ `POST /v1/videos/omni-video` - Omni 视频生成
- ✅ `GET /v1/videos/omni-video/{task_id}` - 查询单个任务
- ✅ `GET /v1/videos/omni-video?pageNum=1&pageSize=30` - 查询任务列表
- ✅ `POST /v1/videos/motion-control` - 运动控制
- ✅ `POST /v1/videos/image2video` - 图生视频
- ✅ `POST /v1/videos/text2video` - 文生视频

### 认证方式（支持三种格式）
1. **新官方格式**：`api-key-kling-xxxxx`（直接作为 Bearer token）
2. **传统格式**：`AccessKey|SecretKey`（生成 JWT token）
3. **代理格式**：`sk-xxxxx`（new-api 内部代理）

### 技术实现细节
- 使用 `json.RawMessage` 处理复杂嵌套字段（避免类型冲突）
- 智能字段映射：`model_name` 优先，回退到 `model`
- 兼容 metadata 覆盖机制（向后兼容性）
- 自动检测请求类型（根据 URL 路径）
- 响应解析支持视频和图片两种结果类型

### 相关文档
- [Kling Omni Video 官方 API 文档](./KLING_OMNI_VIDEO_API.md) ← **新增**
- [API 完整文档](./API_DOCUMENTATION.md)
- [实现总结](./IMPLEMENTATION_SUMMARY.md)
- [README](./README.md)

---

## 2026-06-28 - 添加 voice_list 支持

### 新增功能

✅ **完整支持 Kling 官方 `voice_list` 参数**

添加了音色定制功能的完整透传支持，用户现在可以通过 `voice_list` 参数指定自定义音色。

### 代码变更

#### 1. 新增 VoiceItem 结构体
**文件**: `relay/channel/task/kling/adaptor.go`

```go
type VoiceItem struct {
	VoiceId string `json:"voice_id"`
}
```

#### 2. requestPayload 添加 VoiceList 字段
**文件**: `relay/channel/task/kling/adaptor.go`

```go
type requestPayload struct {
    // ... 其他字段 ...
    VoiceList      []VoiceItem    `json:"voice_list,omitempty"`
    // ... 其他字段 ...
}
```

#### 3. TaskSubmitReq 添加 VoiceList 字段
**文件**: `relay/common/relay_info.go`

```go
type TaskSubmitReq struct {
    // ... 其他字段 ...
    VoiceList            json.RawMessage `json:"voice_list,omitempty"`
    // ... 其他字段 ...
}
```

#### 4. convertToRequestPayload 添加 VoiceList 处理逻辑
**文件**: `relay/channel/task/kling/adaptor.go`

```go
// Parse top-level RawMessage fields
if len(req.VoiceList) > 0 {
    if err := common.Unmarshal(req.VoiceList, &r.VoiceList); err != nil {
        return nil, errors.Wrap(err, "unmarshal voice_list failed")
    }
}
```

### 使用示例

#### 基础示例 - 单个音色

```bash
curl -X POST https://api.example.com/v1/videos/image2video \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "model_name": "kling-v2-6",
    "image": "https://example.com/person.jpg",
    "prompt": "<<<voice_1>>>让图中人物说出以下文字：\"热烈欢迎大家\"",
    "voice_list": [
      {"voice_id": "your_voice_id_here"}
    ],
    "duration": "5",
    "mode": "pro",
    "sound": "on"
  }'
```

#### 高级示例 - 多个音色 + 主体

```bash
curl -X POST https://api.example.com/v1/videos/image2video \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "model_name": "kling-v3",
    "image": "https://example.com/scene.jpg",
    "prompt": "The girl with <<<element_1>>> (using <<<voice_1>>>) talks to the boy (using <<<voice_2>>>)",
    "element_list": [
      {"element_id": 123456789}
    ],
    "voice_list": [
      {"voice_id": "voice_female_1"},
      {"voice_id": "voice_male_1"}
    ],
    "duration": "10",
    "mode": "pro",
    "sound": "on"
  }'
```

### 重要说明

1. **音色引用语法**：在 `prompt` 中使用 `<<<voice_N>>>` 引用音色，N 为音色在 `voice_list` 数组中的序号（从1开始）
2. **数量限制**：一次任务最多引用 2 个音色
3. **必需参数**：使用 `voice_list` 时，`sound` 参数必须设置为 `"on"`
4. **互斥关系**：`element_list` 与 `voice_list` 互斥，不能同时使用（根据官方文档）
5. **计费影响**：当 `voice_list` 不为空且 `prompt` 中引用音色时，按"有指定音色"计费

### 透传支持状态

✅ **所有官方 API 字段均已支持完整透传**

- ✅ 基础参数：`model_name`, `image`, `image_tail`, `prompt`, `negative_prompt`, `duration`, `mode`, `cfg_scale`, `sound`
- ✅ 多镜头：`multi_shot`, `shot_type`, `multi_prompt`
- ✅ 主体和音色：`element_list`, `voice_list` ← **新增**
- ✅ 运动控制：`static_mask`, `dynamic_masks`, `camera_control`
- ✅ 其他：`watermark_info`, `callback_url`, `external_task_id`

### 测试建议

建议测试以下场景：

1. ✅ 单个音色 + 简单台词
2. ✅ 两个音色 + 对话场景
3. ✅ 音色 + 主体（验证互斥关系）
4. ✅ 音色引用序号错误处理
5. ✅ 超过2个音色的错误处理

### 相关文档

- [API 完整文档](./API_DOCUMENTATION.md) - 包含所有字段说明和示例
- [Kling 官方文档](https://app.klingai.com/cn/dev/document-api/apiReference/model/imageToVideo)

---

## 历史版本

### 2024-06 - 初始版本
- 实现基础图生视频功能
- 支持多镜头、运动控制等高级功能
