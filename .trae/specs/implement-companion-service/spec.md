# Companion Service - Complete Implementation Spec (P5-9)

## Why
Companion 服务是 AI 陪伴服务，提供聊天、状态管理和 TTS 语音合成功能。当前只有空壳骨架，需要完整实现。

## What Changes
- P5-9: 完整实现 Companion 微服务
  - Chat RPC：收集学习上下文 → 调用 AI Gateway.CompanionAgent → 调用 Live2DDirector → 合成语音
  - GetCompanionState RPC：查询会话状态
  - SynthesizeSpeech RPC：调用 Volcengine TTS

## Impact
- Affected specs: P5-9
- Affected code:
  - `app/companion/internal/biz/companion.go` (重写)
  - `app/companion/internal/data/companion_repo.go` (重写)
  - `app/companion/internal/data/tts_client.go` (新建)
  - `app/companion/internal/service/companion.go` (重写)
  - `app/companion/internal/conf/conf.go` (修改)
  - `app/companion/cmd/server/main.go` (修改)

## ADDED Requirements

### Requirement: Chat RPC
系统 SHALL 提供 AI 陪伴聊天功能。

#### Scenario: 正常聊天
- **WHEN** 调用 Chat(message, user_emotion)
- **THEN** 返回 reply + emotion + live2d_directive + audio_url

#### Scenario: 消息为空
- **WHEN** 调用 Chat(message="")
- **THEN** 返回 BadRequest 错误

### Requirement: GetCompanionState RPC
系统 SHALL 查询用户陪伴状态。

### Requirement: SynthesizeSpeech RPC
系统 SHALL 调用 TTS 合成语音。

## 全局规范遵循
- 错误处理：使用 kratos errors 包
- 构造函数：NewXxx(deps...) 模式
- 禁止全局变量和 init() 函数
- 使用 context 传播
- 使用中文注释
