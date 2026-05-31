# RAG 实时语音面试 — 待修复清单

## 背景

RAG注入链路已接通（Milvus检索、502事件发送、model版本指定），但实时语音面试中模型仍未引用知识库内容。经排查，以下问题按优先级排列。

---

## P0：事件500时序错误（最大嫌疑）

**文件**: `backend/internal/handler/interview_handler_realtime.go` → `injectRAGContext`

**现状**: 先执行RAG检索（200-300ms），检索完成后才发送安抚话术（事件500），再发RAG数据（事件502）。用户在检索期间完全沉默。

**预期**: 事件500应在ASREnded后立即发送，用于"占住播报窗口"，让用户在检索等待期间有语音播放。RAG检索与安抚话术播放并行，检索完成后再发502。

**修改方案**:
```
当前:  检索 → 500 → 502
改为:  500 → 检索 → 502
```

将 `client.SendChatTTSText(soothingPhrase)` 移到 `RetrieveForInterview` 之前。

---

## P1：buildEnhancedQuery 未使用 industry 参数

**文件**: `backend/internal/rag/interview_rag.go:76`

**现状**: 函数接收 `industry` 参数但未拼入查询字符串，只用了 `currentTopic + userAnswer + skills[:3]`。

**修改方案**: 在查询拼接中加入 industry，与设计文档一致。

---

## P1：external_rag 的 title 使用了 doc.ID

**文件**: `backend/internal/rag/interview_rag.go:127`

**现状**: `FormatForExternalRAG` 中 `ragItem{Title: doc.ID}`，模型收到的是 `doc-12`、`q-5` 这种标识。

**修改方案**: 改为使用 `doc.MetaData` 中的标题信息，或在 `Document` 结构体中增加 `Title` 字段。对于 RAGDocument 来源的文档，ID 格式为 `doc-{id}`，可解析后查数据库获取标题；更简单的做法是同步时把标题写入 metadata，检索时取出。

---

## P2：话题提取按字节截断，有中文乱码风险

**文件**: `backend/internal/handler/interview_handler_realtime.go:815`

**现状**: `extractTopicFromReply` 直接 `reply[:50]` 按字节截断，中文字符可能被截成半截。

**修改方案**: 改为按 rune 截断，或使用 `utf8.DecodeLastRune` 确保不在多字节字符中间断开。

---

## P2：实时文本回答分支未走 RAG

**文件**: `backend/internal/handler/interview_handler_realtime.go:129` → `handleRealtimeUserAnswer`

**现状**: 实时模式下手动文本回答只调用 `AppendRealtimeUserAnswer` + `SendTextQuery`，不触发 `injectRAGContext`。

**修改方案**: 在 `handleRealtimeUserAnswer` 中加入与 `handleRealtimeASREnded` 相同的 RAG 注入逻辑。

---

## P3：RAG 配置未完全贯彻

**现状**:
- `interview_rag.go:19` 把 `defaultRetrieveTopK` 写死为 3，未读取配置项 `ai_rag_top_k`
- `retriever.go:35` 检索结果未按 `ai_rag_score_threshold` 做阈值过滤，低相似度结果也会被注入

**修改方案**:
- `InterviewRAGService` 初始化时接收 topK 和 scoreThreshold 配置
- `RetrieveForInterview` 返回结果后按 scoreThreshold 过滤

---

## 已完成的修复（本次会话）

| 修复项 | 文件 |
|--------|------|
| StartSession 添加 `model: 1.2.1.1` | `volcengine/client.go` |
| 事件500 添加 `start`/`end` 字段 | `volcengine/client.go` |
| RAG 改为同步注入（ASREnded 后阻塞等待） | `interview_handler_realtime.go` |
| TTS sentence start 日志增加 `tts_type` | `interview_handler_realtime.go` |
| `audio_chunk` 日志降级为不输出 | `interview_handler.go` |
| 前端 PCM 播放完成检测 + 手动按钮状态守卫 | `usePCMStreamPlayer.ts` / `InterviewSessionPage.tsx` |

---

## 验证方法

修复后重启 server，进行一次实时语音面试，检查：

1. 日志中 `tts_type` 是否出现 `external_rag`（证明模型识别了RAG数据）
2. 模型回复中是否出现知识库独有术语（如"梦境清理师"、"量子叠加态通道"等）
3. 用户等待期间是否能听到安抚话术（而非沉默）
