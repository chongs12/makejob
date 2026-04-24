# Live2D AI 面试功能开发文档

## 1. 背景

当前仓库已经具备以下基础能力：

- 后端已有面试创建、答题、报告生成与基础 WebSocket 通道。
- 后端已有火山云 ASR/TTS provider 与 Live2D 模型配置能力。
- 前端已有面试入口页与陪伴页 Live2D 渲染实现。

但这些能力尚未形成真正可用的实时面试闭环：

- 面试页仍以文本提交为主。
- Live2D 模型没有进入面试主界面。
- TTS 没有驱动面试官播题与点评。
- ASR 没有接入实时识别。
- Live2D 还没有口型和表情联动。

## 2. 一期目标

一期目标是交付一个可实际使用的实时 AI 面试页，而不是只展示模型：

- 用户进入面试页后能看到 Live2D 面试官。
- 当前题目能实时显示，并在可用时自动播报。
- 用户可以直接文字作答，也可以开启麦克风做流式语音识别。
- 页面能实时展示 ASR partial / final 字幕。
- AI 点评和下一题通过 WebSocket 实时下发。
- Live2D 面试官会根据题目、点评状态切换表情。
- TTS 播放时 Live2D 使用前端音频振幅驱动基础嘴型开合。
- 链路错误能直出真实原因，不再静默回退 mock。

## 3. 当前代码基线

后端基线：

- 面试服务：`backend/internal/service/interview_service.go`
- 面试 WebSocket：`backend/internal/handler/interview_handler.go`
- Live2D 查询：`backend/internal/service/live2d_service.go`
- 火山云 TTS：`backend/internal/tts/volcengine/provider.go`
- 火山云 ASR：`backend/internal/asr/volcengine/provider.go`

前端基线：

- 面试页：`frontend-react/apps/web/src/features/interview/InterviewPage.tsx`
- 陪伴页 Live2D 舞台：`frontend-react/apps/web/src/features/companion/CompanionPage.tsx`

## 4. 一期范围

### 4.1 包含

- 面试页改为 WebSocket 实时驱动。
- 面试页接入 Live2D 模型选择与加载。
- 服务端扩展面试 WebSocket 事件协议。
- 服务端题目和反馈下发时同步触发 TTS。
- 前端使用音频分析器驱动 Live2D 嘴型。
- 前端录音并实时上传 PCM 数据到后端 ASR。
- 服务端下发 ASR partial / final 事件。
- WebSocket 事件统一透传 `trace_id` 与 `interview_id`。

### 4.2 不包含

- phoneme / viseme 级精细口型。
- 视频采集、摄像头表情捕捉。
- 多 AI 面试官或多人面试。
- 二期之前不做复杂动作编排系统。

## 5. 事件协议

### 5.1 前端 -> 后端

- `user_answer`
  - 直接提交文本答案。
- `audio_start`
  - 启动一轮流式 ASR。
  - 可带 `language`、`engine`。
- `audio_chunk`
  - 上传一段 base64 PCM 音频。
- `audio_end`
  - 结束当前音频识别。
- `ping`
  - 保持链路存活。

### 5.2 后端 -> 前端

- `connected`
- `session_ready`
- `interview_state`
- `ai_question`
- `ai_feedback`
- `asr_partial`
- `asr_final`
- `tts_audio`
- `live2d_expression`
- `error`
- `finished`

统一公共字段：

- `type`
- `timestamp`
- `trace_id`
- `interview_id`
- `content`
- `data`

## 6. 核心数据流

### 6.1 首次连接

1. 前端建立 `/api/interviews/:id/ws` WebSocket。
2. 后端校验用户身份，恢复当前面试详情。
3. 后端推送：
   - `connected`
   - `session_ready`
   - `live2d_expression`
   - `ai_question`
4. 若 TTS 可用，后端继续推送 `tts_audio`。
5. 前端收到音频后开始播放，并同步驱动 mouth open。

### 6.2 语音回答

1. 前端点击开始录音。
2. 浏览器创建 `AudioContext({ sampleRate: 16000 })`。
3. 前端将 PCM16 数据转为 base64，持续发送 `audio_chunk`。
4. 后端将 chunk 转发给火山云 ASR 流式会话。
5. 后端返回：
   - `asr_partial`
   - `asr_final`
6. 前端把 `asr_final` 写入回答框，允许用户确认后再提交。

### 6.3 提交回答

1. 前端发送 `user_answer`。
2. 后端调用现有面试服务完成存储、评分和下一题生成。
3. 后端依次推送：
   - `ai_feedback`
   - `live2d_expression`
   - `tts_audio`
   - `ai_question`
   - `tts_audio`
4. 若题目结束，推送 `finished`。

## 7. Live2D 联动策略

### 7.1 表情

表情采用状态映射，不依赖模型文件内置 expression 名称：

- `neutral`
- `serious`
- `thinking`
- `encourage`
- `praise`
- `warning`

前端通过常见 Cubism 参数做基础映射：

- `ParamMouthOpenY`
- `ParamMouthForm`
- `ParamEyeLOpen`
- `ParamEyeROpen`
- `ParamBrowLY`
- `ParamBrowRY`
- `ParamAngleX`
- `ParamBodyAngleX`

### 7.2 口型

一期采用前端音频振幅驱动：

- TTS 播放时创建 `AudioContext + AnalyserNode`
- 每帧计算平均振幅
- 将振幅归一化后写入 `ParamMouthOpenY`
- 播放结束后恢复到 `0`

该方案优先保证跨模型兼容和稳定性。

## 8. 配置与依赖约束

- ASR/TTS 不再允许静默回退 mock。
- 若 provider 未配置，必须直接报错并在前台可见。
- WebSocket 需要兼容浏览器 query token 透传。
- 前端默认采样率采用 `16000`，与当前 ASR 设计保持一致。

## 9. 降级策略

- TTS 不可用：
  - 继续显示文本题目与反馈。
  - Live2D 仍切表情，但不驱动嘴型。
- ASR 不可用：
  - 保留文本输入作答。
- Live2D 加载失败：
  - 不阻塞面试流程，页面继续保留问答和语音能力。
- WebSocket 断开：
  - 页面给出状态提示。
  - 可退回 HTTP 查询的历史消息与报告能力。

## 10. 验收标准

- 进入面试页后，用户能看到 Live2D 面试官与当前题目。
- WebSocket 建立后可收到首题事件。
- TTS 可用时首题能自动播报。
- 播放期间嘴型存在肉眼可见开合。
- 用户录音时页面能看到 partial / final 字幕变化。
- 提交答案后能实时收到点评与下一题。
- 评分不同档位会切换不同表情。
- provider 未配置时前台能收到明确错误，而不是 mock 文本。

## 11. 推荐实施顺序

1. 收口 ASR/TTS provider 选择策略，去掉静默 mock。
2. 扩展面试 WebSocket 协议与服务端事件编排。
3. 接入 TTS 题目播报与点评播报。
4. 面试页改为 WebSocket 驱动。
5. 抽取面试专用 Live2D 舞台组件。
6. 接入前端录音与流式 ASR。
7. 补齐调试面板、trace 展示与失败降级。
