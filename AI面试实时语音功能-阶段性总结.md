# AI 面试实时语音功能 — 阶段性开发总结

> 截止日期：2026-05-24  
> 状态：开发中（未提交），核心链路已跑通，存在已知问题待验证

---

## 一、功能概述

本次修改为 AI 面试模块接入了**豆包（火山引擎）端到端实时语音大模型**，实现面试官与候选人之间的双向语音对话：

- 面试官通过实时 TTS 语音播报题目和反馈
- 候选人通过浏览器麦克风实时回答，音频流式推送到火山模型
- 火山模型完成 ASR + 理解 + 生成 + TTS 的端到端处理
- 支持语音打断（barge-in）、自动静默检测、push_to_talk 模式

---

## 二、架构设计

```
浏览器麦克风 → PCM 16kHz → base64 → WebSocket → 后端 Go 服务
                                                      ↓
                                              解码 base64 → 原始 PCM
                                                      ↓
                                        火山二进制协议封装 → wss://openspeech.bytedance.com
                                                      ↓
                                              火山返回 TTS 音频 + ASR 文本 + Chat 回复
                                                      ↓
                                        后端解析事件 → WebSocket 推送前端
                                                      ↓
                              前端 PCM 流式播放器播放 TTS ← base64 音频块
```

关键设计决策：
- **前端不直连火山 API**，所有音频经后端中转（便于鉴权、日志、业务逻辑编排）
- **push_to_talk 模式**：需要显式发送 EndASR 信号告知火山用户说完
- **音频格式**：上行 PCM 16kHz 单声道 16bit，下行 PCM 24kHz 单声道 16bit
- **分帧策略**：每 20ms 一帧（320 samples = 640 bytes），与火山文档要求一致

---

## 三、修改文件清单

### 新增文件

| 文件 | 用途 |
|------|------|
| `backend/internal/realtime/volcengine/client.go` | 火山实时语音 WebSocket 客户端，封装建连、发送音频、发送文本、接收事件 |
| `backend/internal/realtime/volcengine/protocol.go` | 火山二进制协议编解码（4 字节 header + event + session_id + payload） |
| `backend/internal/realtime/volcengine/client_test.go` | 协议编解码单元测试 |
| `backend/internal/handler/interview_handler_realtime.go` | 实时面试 WebSocket 会话编排（事件消费、轮次管理、消息落库） |
| `backend/internal/service/interview_realtime.go` | 实时面试业务逻辑（上下文恢复、消息持久化、兜底报告生成） |
| `backend/internal/service/live2d_local_support.go` | Live2D 本地模型资源发现与兜底支持 |
| `frontend-react/apps/web/src/shared/usePCMStreamPlayer.ts` | 浏览器端 PCM 流式音频播放器 Hook（Web Audio API） |
| `online.md` | 豆包端到端语音模型 API 接入文档（参考） |

### 修改文件

| 文件 | 修改内容 |
|------|----------|
| `backend/config.yaml` | 新增 `volcengine.realtime` 配置段（app_id, access_token, speaker, input_mode 等） |
| `backend/internal/config/config.go` | 新增 `VolcRealtimeDialogConfig` 结构体及默认值 |
| `backend/cmd/server/main.go` | 注入 `RealtimeInterviewServiceOption` 和 `realtimeConfig` 到 handler |
| `backend/internal/handler/interview_handler.go` | 扩展 WebSocket handler：新增消息类型、realtime 模式分支、状态推送 |
| `backend/internal/service/interview_service.go` | 新增 `IsRealtimeInterview`、创建面试时区分实时/HTTP 模式 |
| `backend/internal/service/live2d_directive_service.go` | 扩展面试问题的 Live2D 指令装饰逻辑 |
| `backend/internal/service/live2d_service.go` | Live2D 服务扩展，支持本地模型发现 |
| `backend/internal/service/live2d_service_test.go` | 对应测试更新 |
| `frontend-react/.../InterviewSessionPage.tsx` | 核心：实时录音、PCM 分帧、WebSocket 双向通信、自动录音控制 |
| `frontend-react/.../interviewHelpers.ts` | PCM 编码工具函数、重采样、WebSocket URL 构建、常量定义 |
| `frontend-react/.../interviewTypes.ts` | 新增实时面试相关 TypeScript 类型定义 |
| `.gitignore` | 新增忽略规则 |

---

## 四、核心实现细节

### 4.1 后端火山客户端 (`backend/internal/realtime/volcengine/`)

- 使用 gorilla/websocket 连接 `wss://openspeech.bytedance.com/api/v3/realtime/dialogue`
- 鉴权通过 HTTP header：`X-Api-Access-Key`, `X-Api-App-Key`, `X-Api-App-ID`
- 协议为自定义二进制帧格式（非标准 WebSocket 文本/JSON）
- 关键事件 ID：
  - 上行：`EventTaskRequest(200)` 发送音频, `EventEndASR(400)` 结束语音, `EventChatTextQuery(501)` 文本输入
  - 下行：`EventASRResponse(451)` 识别文本, `EventTTSResponse(352)` TTS 音频, `EventChatResponse(550)` 模型回复

### 4.2 后端会话编排 (`interview_handler_realtime.go`)

- `bootstrapRealtime()`：恢复上下文 → 发送 session_ready → 建立火山连接 → 启动事件消费协程
- `handleRealtimeAudioChunk()`：base64 解码 → `client.SendAudio()` 转发
- `handleRealtimeAudioEnd()`：发送 `EndASR` 告知火山用户说完
- `consumeRealtimeEvents()`：消费火山事件流，转发为前端可消费的 JSON 协议
- `finalizeRealtimeAssistantTurn()`：轮次收口，落库并推送最终结果

### 4.3 前端录音与通信 (`InterviewSessionPage.tsx`)

- `ensureMicrophonePermission()`：预授权麦克风，避免回答时才弹窗
- `startVoiceCapture()`：创建 AudioContext(16kHz) → ScriptProcessorNode → PCM 分帧 → 20ms 节奏发送
- `stopVoiceCapture()`：释放资源 → 排空帧队列 → 发送 audio_end
- `finishQueuedAudioFrames()`：等待队列排空后再发 audio_end，保证音频完整性
- 自动静默检测：RMS 能量低于阈值持续 1.8s 后自动提交
- 最大录音时长保护：60 秒强制结束

### 4.4 前端 PCM 播放器 (`usePCMStreamPlayer.ts`)

- 接收后端推送的 base64 PCM 音频块
- 使用 Web Audio API 的 AudioBuffer + AudioBufferSourceNode 顺序播放
- 实时计算振幅回传给 Live2D 驱动嘴型动画

---

## 五、配置说明

`backend/config.yaml` 中 `volcengine.realtime` 段：

```yaml
volcengine:
  realtime:
    enabled: true                    # 总开关，false 时走传统 HTTP 问答模式
    base_url: "wss://openspeech.bytedance.com/api/v3/realtime/dialogue"
    app_id: "xxx"                    # 火山实时语音控制台 AppID
    access_token: "xxx"              # 火山实时语音控制台 Access Key
    app_key: "PlgvMymc7f3tQnJ6"      # 协议固定值，代码会强制纠正
    resource_id: "volc.speech.dialog"
    speaker: "zh_female_vv_jupiter_bigtts"  # TTS 音色
    input_mode: "push_to_talk"       # push_to_talk | auto_detect
    audio_format: "pcm"              # 上行音频格式
    sample_rate: 16000               # 上行采样率
    tts_format: "pcm_s16le"          # 下行 TTS 格式
    tts_sample_rate: 24000           # 下行 TTS 采样率
    recv_timeout: 120                # 会话超时秒数
```

---

## 六、已知问题与注意事项

### 6.1 已修复的 Bug

| 问题 | 根因 | 修复 |
|------|------|------|
| 麦克风权限弹窗从未出现 | `ensureMicrophonePermission()` 中 `await ensureRealtimeAudioPlaybackReady()` 在无用户交互时会永远 pending（Chrome 自动播放策略导致 `AudioContext.resume()` 挂起） | 改为 `void ensureRealtimeAudioPlaybackReady()`（fire-and-forget） |
| `stopVoiceCapture` 中有 `if (false && ...)` 死代码 | 调试残留，`audio_end` 发送逻辑已移至 `finishQueuedAudioFrames` | 删除死代码 |
| 录音永远不停止 | 自动停止阈值 0.018 过高 + 无最大时长保护 | 阈值降为 0.008，增加 60s 强制停止 |

### 6.2 待验证 / 潜在风险

1. **push_to_talk 模式下的 EndASR 时机**
   - 当前依赖前端静默检测或手动停止来触发 `audio_end` → 后端发 `EndASR`
   - 如果网络延迟导致最后几帧和 `audio_end` 乱序到达，可能丢失尾部语音
   - 建议后续考虑在 `audio_end` 前加短暂 flush 等待

2. **ScriptProcessorNode 已废弃**
   - 当前使用 `createScriptProcessor`，现代浏览器仍支持但已标记 deprecated
   - 长期应迁移到 `AudioWorklet`（需要单独的 worklet 文件和更复杂的消息通道）

3. **React StrictMode 双重执行**
   - 开发模式下 WebSocket effect 会执行两次（mount → unmount → mount）
   - 第一次连接会被立即关闭，后端会看到短暂的连接/断开日志
   - 生产环境无此问题，但开发时可能导致混淆

4. **火山连接生命周期**
   - 当前无心跳保活机制，长时间无交互可能被火山服务端超时断开（recv_timeout: 120s）
   - 前端有 ping 心跳，但后端未将其转发给火山
   - 建议后续添加定时 keepalive 或在超时断开后自动重连

5. **音频格式兼容性**
   - `new AudioContext({ sampleRate: 16000 })` 不是所有浏览器都支持指定采样率
   - 代码中通过 `resampleFloat32ToPCM16(data, inputSampleRate, 16000)` 做了兜底重采样
   - 但如果 `event.inputBuffer.sampleRate` 返回异常值，重采样可能产生噪音

6. **并发安全**
   - 后端 `realtimeClient` 的 `writeBinary` 有 mutex 保护
   - 但如果火山连接断开后 `readLoop` 触发 `Close()`，而此时 `handleRealtimeAudioChunk` 正在写入，需要确认 gorilla/websocket 的并发写安全性

### 6.3 重构建议

- **前端录音逻辑过于集中**：`InterviewSessionPage.tsx` 已超 1200 行，建议将录音采集抽为独立 Hook（如 `useVoiceCapture`）
- **后端事件消费可抽象**：`consumeRealtimeEvents` 中的 switch-case 可用事件处理器 map 替代，便于扩展新事件
- **协议层可复用**：`backend/internal/realtime/volcengine/` 可作为独立 package 供其他模块使用（如陪伴模块的语音交互）

---

## 七、测试验证清单

- [ ] 创建实时面试 → 确认后端日志 `realtime_mode: true`
- [ ] 面试官第一题 TTS 语音正常播放
- [ ] 用户说话后自动停止 → 后端收到 `audio_end` → 面试官回复
- [ ] 手动点击"手动停止并提交" → 同上
- [ ] 用户打断面试官播报（barge-in） → TTS 停止 → 切换为收听状态
- [ ] 60 秒最大录音时长保护生效
- [ ] 页面刷新后恢复实时会话（dialog_id 持久化）
- [ ] 面试结束后正确生成兜底报告
- [ ] 非 localhost 环境需 HTTPS 才能使用麦克风

---

## 八、参考资源

- `online.md`：豆包端到端实时语音 API 官方文档
- `realtime_dialog/go1.24/`：官方 Go 参考实现（包含完整的协议交互示例）
- 火山控制台：实时语音应用管理、Access Token 获取
- Chrome 自动播放策略：https://developer.chrome.com/blog/autoplay/
