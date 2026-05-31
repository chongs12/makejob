# 面试模块RAG集成里程碑文档

## 一、项目概述

### 1.1 目标

将RAG（Retrieval-Augmented Generation）技术集成到AI面试模块中，实现实时语音面试和非实时文本面试的知识增强，提升面试出题质量和评估准确性。

### 1.2 技术方案

| 组件 | 技术选型 | 说明 |
|------|---------|------|
| 向量数据库 | Milvus v2.5.8 | 存储知识库文档向量 |
| Embedding模型 | 豆包 doubao-embedding-large-text-240915 | 文本转向量 |
| 实时语音模型 | 火山引擎端到端语音大模型 | 支持外部RAG输入（事件502） |
| AI框架 | Eino (cloudwego/eino) | Go语言AI框架 |

---

## 二、实现架构

### 2.1 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│                     面试模块RAG架构                          │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────────┐     ┌──────────────┐     ┌──────────────┐│
│  │ 实时语音面试  │     │ 非实时文本面试│     │   知识库管理  ││
│  └──────┬───────┘     └──────┬───────┘     └──────┬───────┘│
│         │                    │                    │        │
│         ▼                    ▼                    ▼        │
│  ┌──────────────────────────────────────────────────────┐  │
│  │              InterviewRAGService                      │  │
│  │  - RetrieveForInterview()  - EnhanceQuestionPrompt()  │  │
│  │  - FormatForExternalRAG()  - EnhanceFeedbackPrompt()  │  │
│  └──────────────────────────┬───────────────────────────┘  │
│                             │                              │
│         ┌───────────────────┼───────────────────┐         │
│         ▼                   ▼                   ▼         │
│  ┌────────────┐     ┌────────────┐     ┌────────────┐    │
│  │   Milvus   │     │ 豆包Embed  │     │  Ark LLM   │    │
│  │  向量数据库 │     │  文本向量   │     │  对话模型   │    │
│  └────────────┘     └────────────┘     └────────────┘    │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 实时语音面试RAG流程

```
用户语音回答
    ↓
ASR识别 → 获取用户回答文本 (ASREnded 459)
    ↓
发送安抚话术 (ChatTTSText 500) ← 避免沉默
    ↓
RAG检索 → 根据回答内容+简历画像+当前话题检索相关知识
    ↓
格式化 → [{"title":"...","content":"..."}] (最大4K)
    ↓
ChatRAGText(502) → 注入外部RAG到火山引擎模型
    ↓
模型总结+口语化改写 → TTS输出增强后的回答
```

### 2.3 非实时文本面试RAG流程

```
用户请求下一题
    ↓
构建出题提示词
    ↓
RAG检索 → 根据当前主题+行业+薄弱点检索相关知识
    ↓
增强提示词 → 注入参考知识
    ↓
LLM生成题目 → 返回给用户
```

---

## 三、实现的文件清单

### 3.1 新增文件

| 文件 | 说明 |
|------|------|
| `backend/internal/rag/interview_rag.go` | 面试场景RAG服务 |
| `docs/milestone-rag-interview-integration.md` | 本文档 |

### 3.2 修改文件

| 文件 | 修改内容 |
|------|---------|
| `backend/internal/realtime/volcengine/client.go` | 新增事件常量500/502，新增SendChatTTSText/SendChatRAGText方法 |
| `backend/internal/handler/interview_handler.go` | InterviewHandler添加ragService字段，wsInterviewSession注入ragService |
| `backend/internal/handler/interview_handler_realtime.go` | handleRealtimeASREnded触发RAG注入，新增injectRAGContext方法 |
| `backend/internal/service/interview_realtime.go` | RealtimeInterviewContext添加CurrentTopic字段 |
| `backend/internal/ai/interview_agent.go` | 新增PromptEnhancer接口 |
| `backend/internal/ai/runtime/interview_agent.go` | providerInterviewAgent添加promptEnhancer字段，generateQuestion使用增强器 |
| `backend/internal/ai/runtime/builder.go` | newInterviewAgent接受可选的PromptEnhancer参数 |
| `backend/cmd/server/main.go` | 创建InterviewRAGService并注入到面试Agent |

---

## 四、关键接口和方法

### 4.1 InterviewRAGService

```go
// backend/internal/rag/interview_rag.go

type InterviewRAGService struct {
    service *Service
}

// RetrieveForInterview 根据面试上下文检索相关知识
func (s *InterviewRAGService) RetrieveForInterview(
    ctx context.Context,
    query string,           // 用户回答文本
    industry string,        // 行业
    currentTopic string,    // 当前话题
    skills []string,        // 技术栈
) (*InterviewRAGResult, error)

// FormatForExternalRAG 格式化为火山引擎external_rag格式
// 返回: [{"title":"...","content":"..."}]
func (s *InterviewRAGService) FormatForExternalRAG(docs []Document, maxLen int) string

// GetSoothingPhrase 获取随机安抚话术
func (s *InterviewRAGService) GetSoothingPhrase() string

// EnhanceQuestionPrompt 增强出题提示词（实现ai.PromptEnhancer接口）
func (s *InterviewRAGService) EnhanceQuestionPrompt(
    ctx context.Context,
    originalPrompt string,
    topic string,
    industry string,
    skills []string,
) string

// EnhanceFeedbackPrompt 增强评估提示词（实现ai.PromptEnhancer接口）
func (s *InterviewRAGService) EnhanceFeedbackPrompt(
    ctx context.Context,
    originalPrompt string,
    question string,
    answer string,
) string
```

### 4.2 PromptEnhancer接口

```go
// backend/internal/ai/interview_agent.go

type PromptEnhancer interface {
    EnhanceQuestionPrompt(ctx context.Context, originalPrompt string, topic string, industry string, skills []string) string
    EnhanceFeedbackPrompt(ctx context.Context, originalPrompt string, question string, answer string) string
}
```

### 4.3 火山引擎新增方法

```go
// backend/internal/realtime/volcengine/client.go

// SendChatTTSText 发送安抚话术（事件500）
func (c *Client) SendChatTTSText(content string) error

// SendChatRAGText 发送外部RAG数据（事件502）
// externalRAG: [{"title":"...","content":"..."}]
func (c *Client) SendChatRAGText(externalRAG string) error
```

### 4.4 injectRAGContext方法

```go
// backend/internal/handler/interview_handler_realtime.go

func (s *wsInterviewSession) injectRAGContext(userAnswer string) {
    // 1. 获取简历画像中的技术栈
    // 2. 获取当前话题和行业
    // 3. RAG检索
    // 4. 格式化为external_rag格式
    // 5. 发送安抚话术
    // 6. 注入RAG数据
}
```

---

## 五、配置说明

### 5.1 RAG配置（后台管理页面）

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `ai_rag_enabled` | 是否启用RAG | false |
| `ai_rag_collection` | Milvus Collection名 | interview_questions |
| `ai_rag_top_k` | 默认返回数量 | 5 |
| `ai_rag_score_threshold` | 相似度阈值 | 0.5 |
| `ai_rag_milvus_addr` | Milvus地址 | localhost:19530 |
| `ai_rag_milvus_user` | Milvus用户名 | root |
| `ai_rag_milvus_password` | Milvus密码 | Milvus |
| `ai_rag_embed_api_key` | 火山引擎API Key | - |
| `ai_rag_embed_model` | Embedding模型ID | doubao-embedding-large-text-240915 |
| `ai_rag_embed_base_url` | Ark API端点 | https://ark.cn-beijing.volces.com/api/v3 |

### 5.2 火山引擎实时语音配置

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `volcengine.realtime.app_id` | 火山引擎AppID | - |
| `volcengine.realtime.access_token` | 访问令牌 | - |
| `volcengine.realtime.speaker` | TTS发音人 | zh_female_vv_jupiter_bigtts |

---

## 六、使用指南

### 6.1 前置条件

1. Milvus服务已启动：`docker-compose up -d`
2. 后端服务已启动
3. RAG已在后台配置页面启用
4. 知识库已有数据（技术文档、面经、岗位要求）

### 6.2 知识库准备

```bash
# 创建技术文档
curl -X POST http://localhost:8082/api/admin/rag-documents \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer TOKEN" \
  -d '{
    "collection": "interview_questions",
    "doc_type": "tech_doc",
    "title": "Redis缓存穿透解决方案",
    "content": "缓存穿透是指...",
    "metadata": {"tags": ["Redis", "缓存"]}
  }'

# 同步到向量库
curl -X POST http://localhost:8082/api/admin/rag-documents/sync-all \
  -H "Authorization: Bearer TOKEN"
```

### 6.3 面试流程

**实时语音面试**：
1. 创建面试（选择实时语音模式）
2. 用户通过语音回答问题
3. 系统自动RAG检索并注入相关知识
4. 面试官（AI）基于RAG知识给出更专业的追问

**非实时文本面试**：
1. 创建面试（选择文本模式）
2. 系统在出题时自动RAG检索相关知识
3. 生成更有针对性的面试题

---

## 七、验证检查清单

### 7.1 实时语音面试

- [ ] 用户回答后，事件502正确发送到火山引擎
- [ ] RAG内容格式符合`[{"title":"...","content":"..."}]`
- [ ] 内容长度不超过4K
- [ ] 安抚话术正常播放
- [ ] 模型回复包含RAG知识
- [ ] RAG失败不影响面试流程

### 7.2 非实时文本面试

- [ ] 出题时RAG检索正常执行
- [ ] 提示词包含参考知识
- [ ] 生成的题目与参考知识相关
- [ ] RAG失败不影响出题

### 7.3 知识库管理

- [ ] 文档创建和同步正常
- [ ] 语义检索返回相关结果
- [ ] 同义词查询能命中相关文档

---

## 八、后续优化方向

### 8.1 短期优化

1. **缓存优化**：缓存热门查询的Embedding结果，减少API调用
2. **异步优化**：RAG检索异步执行，不阻塞面试流程
3. **错误处理**：更完善的错误处理和降级策略

### 8.2 中期优化

1. **智能检索**：根据面试阶段动态调整检索策略
2. **多知识库**：支持按行业、岗位、技术栈分库检索
3. **评估增强**：使用RAG知识增强答案评估准确性

### 8.3 长期优化

1. **知识图谱**：构建技术知识图谱，支持更智能的检索
2. **个性化**：基于用户学习档案个性化检索
3. **实时更新**：知识库实时更新，保持知识新鲜度

---

## 九、技术参考

### 9.1 火山引擎外部RAG文档

- 事件ID: 502 (ChatRAGText)
- 格式: `{"external_rag": "[{\"title\":\"...\",\"content\":\"...\"}]"}`
- 长度限制: 4K字符
- 模型行为: 自动总结和口语化改写

### 9.2 相关文件

- 火山引擎客户端: `backend/internal/realtime/volcengine/client.go`
- 协议定义: `backend/internal/realtime/volcengine/protocol.go`
- RAG服务: `backend/internal/rag/interview_rag.go`
- 面试Agent: `backend/internal/ai/runtime/interview_agent.go`
- 面试Handler: `backend/internal/handler/interview_handler_realtime.go`

### 9.3 API参考

| API | 方法 | 说明 |
|-----|------|------|
| `/api/admin/rag-documents` | POST | 创建知识库文档 |
| `/api/admin/rag-documents/sync-all` | POST | 同步所有文档到向量库 |
| `/api/admin/rag/search` | GET | 语义检索测试 |
| `/api/admin/rag-configs` | GET/PUT | RAG配置管理 |
| `/api/admin/rag-configs/test` | POST | 测试RAG连接

---

## 十、里程碑总结

| 阶段 | 内容 | 状态 |
|------|------|------|
| Phase 1 | 基础设施搭建（Milvus、配置、Embedding） | ✅ 完成 |
| Phase 2 | RAG核心层实现（Indexer、Retriever、Service） | ✅ 完成 |
| Phase 3A | RAG配置管理后台 | ✅ 完成 |
| Phase 3B | 知识库数据管理 | ✅ 完成 |
| Phase 4 | 面试模块RAG集成 | ✅ 完成 |

**核心成果**：
- 实时语音面试支持外部RAG注入
- 非实时文本面试支持出题增强
- 完整的知识库管理后台
- 灵活的RAG配置管理
