# Realtime Service - Full Implementation Spec (P4-9)

## Why
Realtime 服务是微服务架构中实时面试的核心服务，负责 WebSocket 实时面试能力，集成 Volcengine 实时语音 API。当前只有空壳骨架，需要完整实现。

## What Changes
- P4-9: 完整实现 Realtime 微服务
  - WebSocket 处理器（客户端音频 ↔ Volcengine 双向中转）
  - Volcengine 实时语音 API 集成
  - ASR/Chat/TTS 事件处理
  - RAG 上下文定期注入
  - Interview 服务消息同步

## Impact
- Affected specs: P4-9
- Affected code:
  - `app/realtime/internal/conf/conf.go` (修改)
  - `app/realtime/internal/biz/realtime.go` (重写)
  - `app/realtime/internal/data/volcengine_client.go` (新建)
  - `app/realtime/internal/service/realtime.go` (重写)
  - `app/realtime/internal/server/http.go` (新建)
  - `app/realtime/cmd/server/main.go` (修改)

## ADDED Requirements

### Requirement: WebSocket 实时面试
系统 SHALL 提供 WebSocket 端点接受客户端音频连接。

#### Scenario: 客户端连接
- **WHEN** 客户端连接 ws://host:port/ws/interview/{interview_id}
- **THEN** 创建 Volcengine 会话，双向中转音频数据

### Requirement: Volcengine 实时语音集成
系统 SHALL 集成 Volcengine 实时语音 API。

#### Scenario: ASR 识别
- **WHEN** Volcengine 返回 ASR final text
- **THEN** 调用 Interview.AppendRealtimeUserAnswer 保存

#### Scenario: Chat 回复
- **WHEN** Volcengine 返回 Chat reply
- **THEN** 调用 Interview.AppendRealtimeAssistantReply 保存，转发 TTS 音频给客户端

### Requirement: RAG 上下文注入
系统 SHALL 定期注入 RAG 上下文。

#### Scenario: 定期注入
- **WHEN** 每 30 秒
- **THEN** 调用 RAG.Retrieve 获取相关知识，注入 Volcengine 会话

## 全局规范遵循
- 错误处理：使用 kratos errors 包
- 构造函数：NewXxx(deps...) 模式
- 禁止全局变量和 init() 函数
- 使用 context 传播
- 使用中文注释
- goroutine 必须在所有退出路径清理
