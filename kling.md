# Kling 视频模型接口调用文档

> 适用模型：`kling-v3-omni`、`kling-v3`、`kling-v2.6`  
> 说明：本文不绑定具体服务域名，所有接口地址中的 `{BASE_URL}` 请替换为实际部署地址。  
> 依据：当前 `K3_video.py` 节点实现整理，调用方建议同时传 `model` 与 `model_name`，以兼容不同后端网关。

---

## 1. 总览

| 模型 | 实际请求模型名 | 创建任务接口 | 查询任务接口 | 核心能力 |
|---|---|---|---|---|
| Kling V3 Omni | `kling-v3-omni` | `POST {BASE_URL}/kling/v1/videos/omni-video` | `GET {BASE_URL}/kling/v1/videos/omni-video/{task_id}` | 文生视频、图生视频、首尾帧、多参考图、参考视频、视频编辑、主体引用、分镜 |
| Kling V3 | `kling-v3` | `POST {BASE_URL}/kling/v1/videos/image2video` | `GET {BASE_URL}/kling/v1/videos/image2video/{task_id}` | 标准图生视频、首尾帧、分镜 |
| Kling V2.6 | `kling-v2-6` | `POST {BASE_URL}/kling/v1/videos/image2video` | `GET {BASE_URL}/kling/v1/videos/image2video/{task_id}` | 标准图生视频、首尾帧、分镜 |

> 注意：用户常写作 `kling-v2.6`，但当前接口请求体中的模型名使用 `kling-v2-6`。

---

## 2. 通用调用规则

### 2.1 请求头

所有创建任务请求均使用 JSON 请求体。

```http
Authorization: Bearer <YOUR_API_KEY>
Content-Type: application/json
```

### 2.2 模型字段兼容

为兼容不同网关，创建任务时建议同时传 `model` 和 `model_name`，并保持两者值完全一致。

| 模型 | `model` | `model_name` |
|---|---|---|
| Kling V3 Omni | `kling-v3-omni` | `kling-v3-omni` |
| Kling V3 | `kling-v3` | `kling-v3` |
| Kling V2.6 | `kling-v2-6` | `kling-v2-6` |

### 2.3 清晰度模式

前端可展示为 `720p / 1080p / 4K`，实际接口字段 `mode` 使用下表值。

| 展示档位 | 请求字段 `mode` | 说明 |
|---|---|---|
| `720p` | `std` | 标准模式 |
| `1080p` | `pro` | 高清模式 |
| `4K` | `4k` | 4K 模式，仅 `kling-v3` 与 `kling-v3-omni` 使用 |

### 2.4 时长与模式支持

| 模型 | `duration` | `mode` | 音频 | 备注 |
|---|---|---|---|---|
| `kling-v3-omni` | `"3"` 到 `"15"` | `std`、`pro`、`4k` | `on` / `off` | 支持多模态输入 |
| `kling-v3` | `"3"` 到 `"15"` | `std`、`pro`、`4k` | `on` / `off` | 必须提供起始图 |
| `kling-v2-6` | `"5"` 或 `"10"` | `std`、`pro` | `on` / `off` | 不支持 `4k` |

> `duration` 在请求体中必须使用字符串，例如 `"5"`，不要传数字 `5`。

### 2.5 通用字段

| 字段 | 类型 | 是否必填 | 适用模型 | 说明 |
|---|---:|---:|---|---|
| `model` | string | 是 | 全部 | 模型名 |
| `model_name` | string | 推荐必填 | 全部 | 兼容字段，建议与 `model` 同值 |
| `mode` | string | 是 | 全部 | `std`、`pro`、`4k` |
| `duration` | string | 是 | 全部 | 视频时长，字符串格式 |
| `sound` | string | 是 | 全部 | `on` 或 `off` |
| `watermark_info` | object | 推荐 | `kling-v3-omni`、`kling-v3`、`kling-v2-6` | 当前实现默认 `{ "enabled": false }` |
| `prompt` | string | 单段模式必填 | 全部 | 正向提示词 |
| `negative_prompt` | string | 否 | 全部 | 负向提示词 |

---

## 3. 创建任务与查询任务流程

### 3.1 基本流程

| 步骤 | 动作 | 说明 |
|---|---|---|
| 1 | 准备素材 URL | 图片、视频需要先上传到可被服务端访问的 URL |
| 2 | 创建任务 | 调用对应模型的 `POST` 接口 |
| 3 | 获取任务 ID | 从响应中的 `task_id`、`id` 或 `data.task_id` 读取 |
| 4 | 轮询任务 | 调用 `GET .../{task_id}` 查询状态 |
| 5 | 获取成片 URL | 成功后从响应中读取视频 URL |
| 6 | 下载视频 | 根据返回的视频 URL 下载成品 |

### 3.2 创建任务响应格式

后端至少需要返回一个任务 ID。推荐格式：

```json
{
  "task_id": "task_xxx"
}
```

当前客户端兼容以下任务 ID 字段：

| 支持位置 | 示例 |
|---|---|
| 根字段 `task_id` | `{ "task_id": "task_xxx" }` |
| 根字段 `id` | `{ "id": "task_xxx" }` |
| 嵌套字段 `data.task_id` | `{ "data": { "task_id": "task_xxx" } }` |

### 3.3 查询任务响应格式

推荐返回：

```json
{
  "task_id": "task_xxx",
  "status": "succeed",
  "progress": 100,
  "video_url": "https://example.com/output.mp4"
}
```

当前客户端会从根对象、`data`、`data.data` 中读取状态字段。

| 类型 | 支持字段 |
|---|---|
| 状态字段 | `status`、`task_status`、`state`、`task_state` |
| 进度字段 | `progress` |
| 错误字段 | `error`、`fail_reason`、`failure_reason`、`task_status_msg`、`status_msg`、`error_message`、`message`、`msg`、`reason`、`detail`、`details` |

### 3.4 成功与失败状态

| 类型 | 状态值 |
|---|---|
| 成功 | `succeed`、`succeeded`、`success`、`completed`、`done`、`finished` |
| 失败 | `fail`、`failed`、`failure`、`error`、`expired`、`timeout`、`timed_out`、`cancel`、`canceled`、`cancelled`、`rejected` |

### 3.5 视频 URL 读取位置

成功后，客户端会尝试从以下位置提取视频地址：

| 支持位置 | 示例 |
|---|---|
| `video_url` | `{ "video_url": "https://..." }` |
| `result_url` | `{ "result_url": "https://..." }` |
| `url` | `{ "url": "https://..." }` |
| `download_url` | `{ "download_url": "https://..." }` |
| `result.video_url` | `{ "result": { "video_url": "https://..." } }` |
| `result.url` | `{ "result": { "url": "https://..." } }` |
| `content.video_url` | `{ "content": { "video_url": "https://..." } }` |
| `content.url` | `{ "content": { "url": "https://..." } }` |
| `task_result.videos[0].url` | `{ "task_result": { "videos": [{ "url": "https://..." }] } }` |
| `task_result.videos[0].video_url` | `{ "task_result": { "videos": [{ "video_url": "https://..." }] } }` |
| `metadata.url` | `{ "metadata": { "url": "https://..." } }` |
| `metadata.video_url` | `{ "metadata": { "video_url": "https://..." } }` |

---

## 4. Kling V3 Omni 接口

### 4.1 创建任务

```http
POST {BASE_URL}/kling/v1/videos/omni-video
```

### 4.2 查询任务

```http
GET {BASE_URL}/kling/v1/videos/omni-video/{task_id}
```

### 4.3 基础请求体

```json
{
  "model": "kling-v3-omni",
  "model_name": "kling-v3-omni",
  "mode": "std",
  "duration": "5",
  "sound": "off",
  "watermark_info": {
    "enabled": false
  },
  "prompt": "一名宇航员站在雨夜的霓虹街道中，电影感镜头"
}
```

### 4.4 字段说明

| 字段 | 类型 | 是否必填 | 说明 |
|---|---:|---:|---|
| `model` | string | 是 | 固定为 `kling-v3-omni` |
| `model_name` | string | 推荐必填 | 固定为 `kling-v3-omni` |
| `mode` | string | 是 | `std`、`pro`、`4k` |
| `duration` | string | 是 | `"3"` 到 `"15"` |
| `sound` | string | 是 | `on` 或 `off` |
| `watermark_info` | object | 推荐 | `{ "enabled": false }` 表示关闭水印 |
| `prompt` | string | 单段模式必填 | 正向提示词 |
| `negative_prompt` | string | 否 | 负向提示词 |
| `aspect_ratio` | string | 条件必填 | `16:9`、`9:16`、`1:1` |
| `image_list` | array | 否 | 起始帧、尾帧、参考图列表 |
| `video_list` | array | 否 | 参考视频或待编辑视频列表 |
| `element_list` | array | 否 | 主体引用列表 |
| `multi_shot` | boolean | 分镜模式必填 | 分镜模式传 `true` |
| `shot_type` | string | 分镜模式必填 | 固定为 `customize` |
| `multi_prompt` | array | 分镜模式必填 | 分镜提示词列表 |

### 4.5 `aspect_ratio` 规则

| 场景 | 是否需要传 `aspect_ratio` |
|---|---|
| 有起始帧 | 可不传，由后端根据首帧推断 |
| 有待编辑视频，即 `video_list[].refer_type = "base"` | 可不传，由后端根据视频推断 |
| 纯文本生成 | 必须传 |
| 仅主体引用生成 | 必须传 |
| 仅参考图生成，但没有起始帧 | 必须传 |

支持值：

```json
"aspect_ratio": "16:9"
```

也可以传：

```json
"aspect_ratio": "9:16"
```

或：

```json
"aspect_ratio": "1:1"
```

### 4.6 图片输入 `image_list`

`kling-v3-omni` 的图片输入统一放在 `image_list` 中。图片需要先上传成 URL。

| 图片类型 | 请求格式 | 说明 |
|---|---|---|
| 起始帧 | `{ "image_url": "...", "type": "first_frame" }` | 作为视频第一帧 |
| 尾帧 | `{ "image_url": "...", "type": "end_frame" }` | 作为视频最后一帧 |
| 参考图 | `{ "image_url": "..." }` | 不带 `type`，用于风格、场景、主体参考 |

示例：

```json
{
  "image_list": [
    {
      "image_url": "https://example.com/first.png",
      "type": "first_frame"
    },
    {
      "image_url": "https://example.com/end.png",
      "type": "end_frame"
    },
    {
      "image_url": "https://example.com/ref-1.png"
    }
  ]
}
```

> 当使用“待编辑视频”模式，即 `video_list[].refer_type = "base"` 时，当前实现不会给起始帧或尾帧附加 `type` 字段。

### 4.7 参考视频 `video_list`

`kling-v3-omni` 支持参考视频和待编辑视频。视频需要先上传成 URL。

```json
{
  "video_list": [
    {
      "video_url": "https://example.com/reference.mp4",
      "refer_type": "feature",
      "keep_original_sound": "no"
    }
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---:|---:|---|
| `video_url` | string | 是 | 参考视频 URL |
| `refer_type` | string | 是 | `feature` 表示特征参考，`base` 表示待编辑视频 |
| `keep_original_sound` | string | 是 | `yes` 或 `no` |

参考视频类型映射：

| 业务含义 | `refer_type` |
|---|---|
| 特征参考 | `feature` |
| 待编辑视频 | `base` |

保留原声映射：

| 业务含义 | `keep_original_sound` |
|---|---|
| 保留原声 | `yes` |
| 不保留原声 | `no` |

### 4.8 主体引用 `element_list`

主体引用只支持 `kling-v3-omni`。调用方需要先取得主体 ID，然后放入 `element_list`。

```json
{
  "element_list": [
    {
      "element_id": "element_xxx"
    }
  ]
}
```

如果提示词中保留 `【@名称】` 文本，也可以同时传 `element_list`，让模型在文本中读到主体名称，在结构化字段中获得主体 ID。

### 4.9 分镜模式

分镜模式下不传普通 `prompt`，而是传 `multi_shot`、`shot_type`、`multi_prompt`。

```json
{
  "model": "kling-v3-omni",
  "model_name": "kling-v3-omni",
  "mode": "pro",
  "duration": "10",
  "sound": "on",
  "watermark_info": {
    "enabled": false
  },
  "aspect_ratio": "16:9",
  "multi_shot": true,
  "shot_type": "customize",
  "multi_prompt": [
    {
      "index": 1,
      "prompt": "镜头一：远景，未来城市夜景，雨水反射霓虹灯",
      "duration": "5"
    },
    {
      "index": 2,
      "prompt": "镜头二：角色走向镜头，金属服装细节清晰",
      "duration": "5"
    }
  ]
}
```

分镜规则：

| 规则 | 要求 |
|---|---|
| `multi_shot` | 固定为 `true` |
| `shot_type` | 固定为 `customize` |
| `multi_prompt[].index` | 从 `1` 开始递增 |
| `multi_prompt[].prompt` | 每段不能为空，建议不超过 512 字符 |
| `multi_prompt[].duration` | 字符串格式 |
| 分镜时长总和 | 必须等于总 `duration` |
| 尾帧 | 分镜模式下不支持尾帧 |

### 4.10 Kling V3 Omni 完整示例

```bash
curl -X POST "{BASE_URL}/kling/v1/videos/omni-video" \
  -H "Authorization: Bearer <YOUR_API_KEY>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "kling-v3-omni",
    "model_name": "kling-v3-omni",
    "mode": "std",
    "duration": "5",
    "sound": "off",
    "watermark_info": {
      "enabled": false
    },
    "aspect_ratio": "16:9",
    "prompt": "让【@女模特A】站在海边回头微笑，黄昏光线，电影感",
    "image_list": [
      {
        "image_url": "https://example.com/first.png",
        "type": "first_frame"
      },
      {
        "image_url": "https://example.com/style-ref.png"
      }
    ],
    "video_list": [
      {
        "video_url": "https://example.com/motion-ref.mp4",
        "refer_type": "feature",
        "keep_original_sound": "no"
      }
    ],
    "element_list": [
      {
        "element_id": "element_xxx"
      }
    ],
    "negative_prompt": "低清晰度，畸形，抖动，闪烁"
  }'
```

### 4.11 Kling V3 Omni 约束

| 约束 | 要求 |
|---|---|
| 起始帧 | 可不传，支持纯文本或主体生成 |
| 图片数量 + 主体数量 | 无参考视频时最多 `7` 个；有参考视频时最多 `4` 个 |
| 主体数量 | 最多 `3` 个 |
| 尾帧与分镜 | 互斥，分镜模式不能传尾帧 |
| 尾帧与多参考图 | 互斥，使用参考图时不要同时传尾帧 |
| 参考图 | 仅 `kling-v3-omni` 支持 |
| 参考视频 | 仅 `kling-v3-omni` 支持 |
| 主体引用 | 仅 `kling-v3-omni` 支持 |

---

## 5. Kling V3 接口

### 5.1 创建任务

```http
POST {BASE_URL}/kling/v1/videos/image2video
```

### 5.2 查询任务

```http
GET {BASE_URL}/kling/v1/videos/image2video/{task_id}
```

### 5.3 基础请求体

`kling-v3` 是标准图生视频接口，必须提供起始图 URL。

```json
{
  "model": "kling-v3",
  "model_name": "kling-v3",
  "image": "https://example.com/first.png",
  "prompt": "让画面中的人物自然转身，镜头缓慢推进",
  "negative_prompt": "低清晰度，畸形，抖动",
  "duration": "5",
  "mode": "std",
  "sound": "off",
  "watermark_info": {
    "enabled": false
  }
}
```

### 5.4 字段说明

| 字段 | 类型 | 是否必填 | 说明 |
|---|---:|---:|---|
| `model` | string | 是 | 固定为 `kling-v3` |
| `model_name` | string | 推荐必填 | 固定为 `kling-v3` |
| `image` | string | 是 | 起始图 URL |
| `prompt` | string | 单段模式必填 | 正向提示词 |
| `negative_prompt` | string | 否 | 负向提示词，可为空字符串 |
| `duration` | string | 是 | `"3"` 到 `"15"` |
| `mode` | string | 是 | `std`、`pro`、`4k` |
| `sound` | string | 是 | `on` 或 `off` |
| `watermark_info` | object | 推荐 | `{ "enabled": false }` |
| `image_tail` | string | 否 | 尾帧 URL，非分镜模式使用 |
| `multi_shot` | boolean | 分镜模式必填 | 分镜模式传 `true` |
| `shot_type` | string | 分镜模式必填 | 固定为 `customize` |
| `multi_prompt` | array | 分镜模式必填 | 分镜提示词列表 |

### 5.5 首尾帧示例

```bash
curl -X POST "{BASE_URL}/kling/v1/videos/image2video" \
  -H "Authorization: Bearer <YOUR_API_KEY>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "kling-v3",
    "model_name": "kling-v3",
    "image": "https://example.com/first.png",
    "image_tail": "https://example.com/end.png",
    "prompt": "从第一帧自然过渡到尾帧，人物动作流畅，电影感",
    "negative_prompt": "低清晰度，畸形，闪烁",
    "duration": "5",
    "mode": "pro",
    "sound": "off",
    "watermark_info": {
      "enabled": false
    }
  }'
```

### 5.6 分镜示例

```bash
curl -X POST "{BASE_URL}/kling/v1/videos/image2video" \
  -H "Authorization: Bearer <YOUR_API_KEY>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "kling-v3",
    "model_name": "kling-v3",
    "image": "https://example.com/first.png",
    "prompt": "镜头一：人物站在窗边，城市夜景作为背景",
    "negative_prompt": "低清晰度，畸形，闪烁",
    "duration": "8",
    "mode": "std",
    "sound": "off",
    "watermark_info": {
      "enabled": false
    },
    "multi_shot": true,
    "shot_type": "customize",
    "multi_prompt": [
      {
        "index": 1,
        "prompt": "镜头一：人物站在窗边，城市夜景作为背景",
        "duration": "4"
      },
      {
        "index": 2,
        "prompt": "镜头二：镜头缓慢推进，人物转身看向镜头",
        "duration": "4"
      }
    ]
  }'
```

> 标准图生视频分镜模式下，当前实现仍会传一个顶层 `prompt` 作为占位或兼容字段；主要分镜内容以 `multi_prompt` 为准。

### 5.7 Kling V3 约束

| 约束 | 要求 |
|---|---|
| 起始图 `image` | 必填 |
| 参考图2~5 | 不支持，请使用 `kling-v3-omni` |
| 参考视频 | 不支持，请使用 `kling-v3-omni` |
| 主体引用 | 不支持，请使用 `kling-v3-omni` |
| 尾帧与分镜 | 互斥 |
| 时长 | `3~15s` |
| 模式 | `std`、`pro`、`4k` |

---

## 6. Kling V2.6 接口

### 6.1 创建任务

```http
POST {BASE_URL}/kling/v1/videos/image2video
```

### 6.2 查询任务

```http
GET {BASE_URL}/kling/v1/videos/image2video/{task_id}
```

### 6.3 基础请求体

```json
{
  "model": "kling-v2-6",
  "model_name": "kling-v2-6",
  "image": "https://example.com/first.png",
  "prompt": "让画面中的人物轻轻挥手，背景保持稳定",
  "negative_prompt": "低清晰度，畸形，抖动",
  "duration": "5",
  "mode": "std",
  "sound": "off",
  "watermark_info": {
    "enabled": false
  }
}
```

### 6.4 字段说明

| 字段 | 类型 | 是否必填 | 说明 |
|---|---:|---:|---|
| `model` | string | 是 | 固定为 `kling-v2-6` |
| `model_name` | string | 推荐必填 | 固定为 `kling-v2-6` |
| `image` | string | 是 | 起始图 URL |
| `prompt` | string | 单段模式必填 | 正向提示词 |
| `negative_prompt` | string | 否 | 负向提示词，可为空字符串 |
| `duration` | string | 是 | 仅支持 `"5"` 或 `"10"` |
| `mode` | string | 是 | 仅支持 `std` 或 `pro` |
| `sound` | string | 是 | `on` 或 `off` |
| `watermark_info` | object | 推荐 | `{ "enabled": false }` |
| `image_tail` | string | 否 | 尾帧 URL，非分镜模式使用 |
| `multi_shot` | boolean | 分镜模式必填 | 分镜模式传 `true` |
| `shot_type` | string | 分镜模式必填 | 固定为 `customize` |
| `multi_prompt` | array | 分镜模式必填 | 分镜提示词列表 |

### 6.5 首尾帧示例

```bash
curl -X POST "{BASE_URL}/kling/v1/videos/image2video" \
  -H "Authorization: Bearer <YOUR_API_KEY>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "kling-v2-6",
    "model_name": "kling-v2-6",
    "image": "https://example.com/first.png",
    "image_tail": "https://example.com/end.png",
    "prompt": "从起始帧自然过渡到尾帧，保持人物身份一致",
    "negative_prompt": "低清晰度，畸形，闪烁",
    "duration": "5",
    "mode": "std",
    "sound": "off",
    "watermark_info": {
      "enabled": false
    }
  }'
```

### 6.6 分镜示例

```bash
curl -X POST "{BASE_URL}/kling/v1/videos/image2video" \
  -H "Authorization: Bearer <YOUR_API_KEY>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "kling-v2-6",
    "model_name": "kling-v2-6",
    "image": "https://example.com/first.png",
    "prompt": "镜头一：人物站在街角，轻微转头",
    "negative_prompt": "低清晰度，畸形，闪烁",
    "duration": "10",
    "mode": "pro",
    "sound": "off",
    "watermark_info": {
      "enabled": false
    },
    "multi_shot": true,
    "shot_type": "customize",
    "multi_prompt": [
      {
        "index": 1,
        "prompt": "镜头一：人物站在街角，轻微转头",
        "duration": "5"
      },
      {
        "index": 2,
        "prompt": "镜头二：镜头从侧面绕到正面，背景保持稳定",
        "duration": "5"
      }
    ]
  }'
```

### 6.7 Kling V2.6 约束

| 约束 | 要求 |
|---|---|
| 起始图 `image` | 必填 |
| 时长 | 仅支持 `5s` 或 `10s` |
| 模式 | 仅支持 `std`、`pro` |
| 4K | 不支持 |
| 参考图2~5 | 不支持，请使用 `kling-v3-omni` |
| 参考视频 | 不支持，请使用 `kling-v3-omni` |
| 主体引用 | 不支持，请使用 `kling-v3-omni` |
| 尾帧与分镜 | 互斥 |
| `pro` + 首尾帧 + 音频 | 当前实现禁止，建议关闭音频 |

---

## 7. 素材要求

### 7.1 图片素材

图片在接口请求体中以 URL 形式传递。

| 项目 | 要求 |
|---|---|
| 传输方式 | 先上传图片，得到公网或服务端可访问 URL，再放入请求体 |
| 标准图生视频起始图 | 使用字段 `image` |
| 标准图生视频尾帧 | 使用字段 `image_tail` |
| Omni 图片 | 使用 `image_list[].image_url` |
| 宽高比 | 建议在 `1:2.5 ~ 2.5:1` |
| 文件大小 | 建议不超过 `10MB` |
| 最小尺寸 | 建议宽高均不低于 `300px` |

### 7.2 视频素材

参考视频只适用于 `kling-v3-omni`。

| 项目 | 要求 |
|---|---|
| 格式 | `.mp4` 或 `.mov` |
| 时长 | `3~10s` |
| 文件大小 | `<= 200MB` |
| 宽高 | `720~2160px` |
| 帧率 | `24~60fps` |
| 宽高比 | `1:2.5 ~ 2.5:1` |
| 传输方式 | 先上传视频，得到公网或服务端可访问 URL，再放入 `video_list[].video_url` |

---

## 8. 常见请求模板

### 8.1 `kling-v3-omni` 纯文本生成

```json
{
  "model": "kling-v3-omni",
  "model_name": "kling-v3-omni",
  "mode": "std",
  "duration": "5",
  "sound": "off",
  "watermark_info": {
    "enabled": false
  },
  "aspect_ratio": "16:9",
  "prompt": "一辆红色跑车在雨夜城市道路上高速行驶，电影感，动态模糊"
}
```

### 8.2 `kling-v3-omni` 起始帧生成

```json
{
  "model": "kling-v3-omni",
  "model_name": "kling-v3-omni",
  "mode": "std",
  "duration": "5",
  "sound": "off",
  "watermark_info": {
    "enabled": false
  },
  "prompt": "让人物自然眨眼并微笑，头发轻微飘动",
  "image_list": [
    {
      "image_url": "https://example.com/first.png",
      "type": "first_frame"
    }
  ]
}
```

### 8.3 `kling-v3-omni` 待编辑视频

```json
{
  "model": "kling-v3-omni",
  "model_name": "kling-v3-omni",
  "mode": "std",
  "duration": "5",
  "sound": "off",
  "watermark_info": {
    "enabled": false
  },
  "prompt": "保持原视频动作，把场景改成赛博朋克风格",
  "video_list": [
    {
      "video_url": "https://example.com/base-video.mp4",
      "refer_type": "base",
      "keep_original_sound": "no"
    }
  ]
}
```

### 8.4 `kling-v3` 图生视频

```json
{
  "model": "kling-v3",
  "model_name": "kling-v3",
  "image": "https://example.com/first.png",
  "prompt": "让画面中的人物向镜头挥手，背景稳定，动作自然",
  "negative_prompt": "",
  "duration": "5",
  "mode": "std",
  "sound": "off",
  "watermark_info": {
    "enabled": false
  }
}
```

### 8.5 `kling-v2-6` 图生视频

```json
{
  "model": "kling-v2-6",
  "model_name": "kling-v2-6",
  "image": "https://example.com/first.png",
  "prompt": "让画面中的人物缓慢转身，动作连贯",
  "negative_prompt": "",
  "duration": "5",
  "mode": "std",
  "sound": "off",
  "watermark_info": {
    "enabled": false
  }
}
```

---

## 9. Python 调用示例

下面示例不指定具体域名，调用时把 `{BASE_URL}` 替换为实际地址。

```python
import time
import requests

BASE_URL = "{BASE_URL}"
API_KEY = "<YOUR_API_KEY>"

headers = {
    "Authorization": f"Bearer {API_KEY}",
    "Content-Type": "application/json",
}

body = {
    "model": "kling-v3",
    "model_name": "kling-v3",
    "image": "https://example.com/first.png",
    "prompt": "让画面中的人物自然微笑并轻轻挥手",
    "negative_prompt": "低清晰度，畸形，抖动",
    "duration": "5",
    "mode": "std",
    "sound": "off",
    "watermark_info": {"enabled": False},
}

create_resp = requests.post(
    f"{BASE_URL}/kling/v1/videos/image2video",
    headers=headers,
    json=body,
    timeout=60,
)
create_resp.raise_for_status()
create_data = create_resp.json()

task_id = (
    create_data.get("task_id")
    or create_data.get("id")
    or create_data.get("data", {}).get("task_id")
)

if not task_id:
    raise RuntimeError(f"任务创建失败，响应中没有 task_id: {create_data}")

while True:
    status_resp = requests.get(
        f"{BASE_URL}/kling/v1/videos/image2video/{task_id}",
        headers=headers,
        timeout=60,
    )
    status_resp.raise_for_status()
    data = status_resp.json()

    status = (
        data.get("status")
        or data.get("task_status")
        or data.get("state")
        or data.get("task_state")
        or data.get("data", {}).get("status")
        or data.get("data", {}).get("task_status")
    )

    if status:
        status = str(status).lower()

    if status in {"succeed", "succeeded", "success", "completed", "done", "finished"}:
        video_url = (
            data.get("video_url")
            or data.get("result_url")
            or data.get("url")
            or data.get("download_url")
            or data.get("data", {}).get("video_url")
            or data.get("data", {}).get("url")
        )
        print("video_url:", video_url)
        break

    if status in {"fail", "failed", "failure", "error", "expired", "timeout", "timed_out", "cancel", "canceled", "cancelled", "rejected"}:
        raise RuntimeError(f"任务失败: {data}")

    time.sleep(3)
```

---

## 10. 接入检查清单

| 检查项 | 要求 |
|---|---|
| 是否使用正确端点 | `kling-v3-omni` 用 `/omni-video`；`kling-v3`、`kling-v2-6` 用 `/image2video` |
| 是否同时传模型字段 | 建议同时传 `model` 与 `model_name` |
| `duration` 类型 | 必须是字符串 |
| `mode` 值 | 使用 `std`、`pro`、`4k`，不要传 `720p`、`1080p`、`4K` |
| 起始图 | `kling-v3`、`kling-v2-6` 必须传 `image` |
| 多模态能力 | 参考图、参考视频、主体引用只给 `kling-v3-omni` 使用 |
| 分镜时长 | `multi_prompt[].duration` 总和必须等于总 `duration` |
| 尾帧冲突 | 分镜模式不能传尾帧；`kling-v3-omni` 多参考图时不要传尾帧 |
| V2.6 限制 | 只支持 `5s/10s` 和 `std/pro` |
| 响应任务 ID | 创建任务响应必须返回 `task_id`、`id` 或 `data.task_id` |
| 查询结果 | 成功后必须返回可下载的视频 URL |

