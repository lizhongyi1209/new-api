# Kling 图生视频适配器

Kling 官方图生视频 API 的完整适配实现，支持所有官方文档中的参数透传。

## 快速开始

### 基础图生视频

```bash
POST /v1/videos/image2video
{
  "model_name": "kling-v2-6",
  "image": "https://example.com/image.jpg",
  "prompt": "描述视频内容",
  "duration": "5",
  "mode": "pro"
}
```

### 带音色的图生视频

```bash
POST /v1/videos/image2video
{
  "model_name": "kling-v2-6",
  "image": "https://example.com/person.jpg",
  "prompt": "<<<voice_1>>>让图中人物说：\"你好世界\"",
  "voice_list": [
    {"voice_id": "your_voice_id"}
  ],
  "sound": "on",
  "duration": "5",
  "mode": "pro"
}
```

## 支持的功能

### ✅ 完整支持的官方 API 参数

| 功能分类 | 参数 | 说明 |
|---------|------|------|
| **基础参数** | `model_name`, `model` | 模型选择（v1/v1-5/v1-6/v2-6/v3等） |
| | `image`, `image_tail` | 首帧/尾帧图片 |
| | `prompt`, `negative_prompt` | 正向/负向提示词 |
| | `duration` | 时长（3-15秒） |
| | `mode` | 模式（std/pro/4k） |
| | `cfg_scale` | 自由度（0-1） |
| | `sound` | 是否生成声音（on/off） |
| **多镜头** | `multi_shot` | 是否多镜头 |
| | `shot_type` | 分镜方式（customize/intelligence） |
| | `multi_prompt` | 分镜列表（最多6个） |
| **主体&音色** | `element_list` | 主体列表（最多3个） |
| | `voice_list` | 音色列表（最多2个）⭐ **新增** |
| **运动控制** | `static_mask` | 静态笔刷蒙版 |
| | `dynamic_masks` | 动态笔刷列表（最多6组） |
| | `camera_control` | 摄像机运动控制 |
| **其他** | `watermark_info` | 水印配置 |
| | `callback_url` | 回调地址 |
| | `external_task_id` | 自定义任务ID |

## 文档

- 📖 [完整 API 文档](./API_DOCUMENTATION.md) - 所有参数的详细说明和示例
- 📝 [更新日志](./CHANGELOG.md) - 版本历史和新增功能说明
- 🔗 [Kling 官方文档](https://app.klingai.com/cn/dev/document-api/apiReference/model/imageToVideo)

## 代码结构

```
kling/
├── adaptor.go           # 主适配器实现
├── element.go           # 主体库相关功能
├── API_DOCUMENTATION.md # 完整 API 文档
├── CHANGELOG.md         # 更新日志
└── README.md            # 本文件
```

### 核心结构体

```go
// VoiceItem - 音色配置
type VoiceItem struct {
    VoiceId string `json:"voice_id"`
}

// ElementItem - 主体配置
type ElementItem struct {
    ElementId int64 `json:"element_id"`
}

// requestPayload - 发送给 Kling 的完整请求
type requestPayload struct {
    // 基础字段
    Prompt         string   `json:"prompt,omitempty"`
    Image          string   `json:"image,omitempty"`
    ModelName      string   `json:"model_name,omitempty"`
    Duration       string   `json:"duration,omitempty"`
    Mode           string   `json:"mode,omitempty"`
    
    // 音色&主体
    VoiceList      []VoiceItem   `json:"voice_list,omitempty"`
    ElementList    []ElementItem `json:"element_list,omitempty"`
    
    // 更多字段...
}
```

## 音色功能使用指南

### 1. 获取音色 ID

通过 Kling 音色定制接口获取 `voice_id`，或使用系统预置音色。

### 2. 在请求中引用音色

```json
{
  "prompt": "男人<<<voice_1>>>说：\"你好\"，女人<<<voice_2>>>回答：\"你好\"",
  "voice_list": [
    {"voice_id": "male_voice_id"},
    {"voice_id": "female_voice_id"}
  ],
  "sound": "on"
}
```

### 3. 音色引用规则

- 使用 `<<<voice_N>>>` 引用音色，N 从 1 开始
- N 对应 `voice_list` 数组的索引（1-based）
- 最多引用 2 个音色
- 必须设置 `sound: "on"`

### 4. 注意事项

⚠️ **互斥关系**：`element_list` 与 `voice_list` 不能同时使用

💰 **计费**：引用音色时按"有指定音色"标准计费

## 常见问题

### Q: voice_list 和 element_list 可以同时使用吗？
A: 不可以。根据官方文档，这两个参数互斥。

### Q: 如何获取 voice_id？
A: 通过 Kling 音色定制接口创建音色后获得，或使用系统预置音色 ID。

### Q: 最多可以使用几个音色？
A: 一次任务最多 2 个音色。

### Q: 音色必须在 prompt 中引用吗？
A: 是的，需要在 prompt 中使用 `<<<voice_N>>>` 语法引用。

### Q: 不使用音色时会怎样计费？
A: 按标准费率计费。只有在 `voice_list` 不为空且 prompt 中引用音色时才按"有指定音色"费率。

## 开发指南

### 添加新字段支持

1. 在 `relay/common/relay_info.go` 的 `TaskSubmitReq` 中添加字段
2. 在 `adaptor.go` 的 `requestPayload` 中添加对应字段
3. 在 `convertToRequestPayload` 中添加解析逻辑
4. 更新文档

### 测试

```bash
# 运行 Kling 相关测试
go test ./relay/channel/task/kling/...

# 格式化代码
gofmt -w relay/channel/task/kling/
```

## 版本历史

- **2026-06-28**: 添加 `voice_list` 支持，实现所有官方字段完整透传
- **2024-06**: 初始版本，支持基础图生视频功能

## 许可证

本项目遵循 new-api 主项目的许可证。
