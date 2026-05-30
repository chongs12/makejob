# 面试RAG系统实施文档

## 一、项目概述

本项目为MakeJob平台的AI面试功能引入RAG（Retrieval-Augmented Generation）技术，通过Milvus向量数据库和豆包Embedding模型，实现面试出题的语义检索增强。

### 技术选型

| 组件 | 技术方案 | 版本 |
|------|---------|------|
| 向量数据库 | Milvus Standalone | v2.5.8 |
| Embedding模型 | 豆包 (doubao-embedding) | 通过火山引擎Ark平台 |
| Go框架 | Eino (cloudwego/eino) + eino-ext | v0.8.9 |
| 消息队列 | RabbitMQ | 3-management |

### 设计原则

- **接口抽象 + 工厂模式**：遵循项目现有架构风格
- **微服务扩展预留**：RAG Service通过接口解耦，未来可独立部署
- **异步同步**：通过RabbitMQ实现题目变更的异步向量库同步
- **本地兜底**：RAG初始化失败时不影响核心功能

---

## 二、已完成功能

### Phase 1: 基础设施搭建

#### 1.1 Milvus服务部署

在`docker-compose.yml`中添加了etcd + minio + milvus三个服务：

```yaml
services:
  etcd:      # Milvus依赖的元数据存储
  minio:     # Milvus依赖的对象存储
  milvus:    # 向量数据库主服务
    ports:
      - "19530:19530"  # gRPC端口
      - "9091:9091"    # 健康检查端口
```

#### 1.2 配置管理

**新增配置项**：

```yaml
# backend/config.yaml
milvus:
  enabled: true
  addr: "localhost:19530"
  user: "root"
  password: "Milvus"

ai:
  rag_embed_endpoint: ""  # 豆包Embedding接入点ID
```

**新增配置结构体**：

```go
// backend/internal/config/config.go
type MilvusConfig struct {
    Enabled  bool   `mapstructure:"enabled"`
    Addr     string `mapstructure:"addr"`
    User     string `mapstructure:"user"`
    Password string `mapstructure:"password"`
}
```

#### 1.3 依赖添加

```go
// backend/go.mod
require (
    github.com/cloudwego/eino-ext/components/embedding/ark v0.1.2
    github.com/milvus-io/milvus/client/v2 v2.6.5
)
```

---

### Phase 2: RAG核心层实现

#### 2.1 Embedding层

**文件**：`backend/internal/ai/eino/embedding.go`

封装豆包Embedding模型，遵循项目现有的薄桥接层风格：

```go
func NewEmbedder(ctx context.Context, apiKey string, endpointID string) (embedding.Embedder, error)
```

- 使用`eino-ext/components/embedding/ark`包
- 参数校验遵循`fmt.Errorf`错误包装风格
- 支持通过火山引擎API Key和接入点ID创建

#### 2.2 RAG接口层

**文件**：`backend/internal/rag/types.go`

定义了RAG系统的核心接口：

```go
// Indexer 向量索引写入接口
type Indexer interface {
    Index(ctx context.Context, docs []IndexDocument) (ids []string, err error)
    Delete(ctx context.Context, ids []string) error
}

// Retriever 语义检索接口
type Retriever interface {
    Retrieve(ctx context.Context, query string, topK int) ([]Document, error)
}
```

#### 2.3 Milvus Collection管理

**文件**：`backend/internal/rag/collection.go`

Collection Schema设计：

| 字段 | 类型 | 说明 |
|------|------|------|
| id | VarChar(64) | 主键，格式"q-{question_id}" |
| content | VarChar(8192) | 题目内容 |
| vector | FloatVector(1536) | 豆包Embedding向量 |
| metadata | JSON | 元数据（题目ID、类型、难度、标签等） |

索引策略：
- 向量字段使用COSINE度量
- 采用AUTOINDEX，Milvus自动选择最优索引（HNSW/IVF_FLAT）

#### 2.4 Indexer实现

**文件**：`backend/internal/rag/indexer.go`

核心功能：
- 批量Embedding（每批最多64条，避免API超限）
- float64→float32向量转换（Milvus要求）
- JSON元数据序列化
- 批量插入Milvus

#### 2.5 Retriever实现

**文件**：`backend/internal/rag/retriever.go`

核心功能：
- 查询文本Embedding向量化
- Milvus COSINE相似度搜索
- 结果解析（ID、Content、Metadata、Score）

#### 2.6 题目索引构建器

**文件**：`backend/internal/rag/question_builder.go`

将`model.Question`转换为`IndexDocument`：

```go
func BuildQuestionDocuments(questions []model.Question) []IndexDocument
```

Content拼接规则：`Title + "\n" + Content`

MetaData字段：question_id, type, difficulty, tags, category_id, industry_id, answer

#### 2.7 RAG Service

**文件**：`backend/internal/rag/service.go`

编排层，串联Indexer + Retriever：

```go
type Service struct {
    indexer   Indexer
    retriever Retriever
    config    Config
}

func (s *Service) IndexQuestions(ctx context.Context, questions []model.Question) error
func (s *Service) RetrieveByQuery(ctx context.Context, query string, topK int) ([]Document, error)
func (s *Service) DeleteByIDs(ctx context.Context, ids []string) error
```

#### 2.8 初始化封装

**文件**：`backend/internal/rag/init.go`

一键初始化RAG系统：

```go
func Init(ctx context.Context, cfg Config) (*InitResult, error)
```

初始化流程：
1. 连接Milvus
2. 确保Collection存在（不存在则创建并建立索引）
3. 创建Embedder
4. 创建Indexer和Retriever
5. 组装Service

---

### Phase 3: 数据同步与管理API

#### 3.1 RabbitMQ消息定义

**文件**：`backend/internal/mq/message.go`

新增RAG同步消息类型：

```go
const TaskTypeRAGSync = "rag.sync.question"

type RAGSyncPayload struct {
    Action     string `json:"action"`      // index | update | delete
    QuestionID uint   `json:"question_id"`
}
```

#### 3.2 同步消费者

**文件**：`backend/internal/rag/sync_consumer.go`

处理题目增删改事件：

```go
type SyncConsumer struct {
    service *Service
    repo    repository.QuestionRepository
}

func (c *SyncConsumer) Handle(ctx context.Context, payload []byte) error
```

支持的操作：
- `index` / `update`：从数据库查题目 → 构建文档 → 写入Milvus
- `delete`：从payload提取question_id → 删除Milvus文档

#### 3.3 管理后台API

**文件**：`backend/internal/handler/admin_rag_handler.go`

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/admin/rag/index-all` | 全量索引所有题目 |
| POST | `/admin/rag/index` | 增量索引指定题目 |
| DELETE | `/admin/rag/index` | 删除指定题目索引 |
| GET | `/admin/rag/search?query=xxx&top_k=5` | 语义检索测试 |

---

### Phase 4: 依赖注入与集成

#### 4.1 Server端集成

**文件**：`backend/cmd/server/main.go`

在`initDependencies`中添加RAG初始化：

```go
if cfg.Milvus.Enabled {
    ragResult, ragErr := rag.Init(ctx, rag.Config{...})
    if ragErr != nil {
        applogger.Warn("RAG init failed, continuing without RAG", zap.Error(ragErr))
    } else {
        defer ragResult.Closer()
        deps.AdminRAGHandler = handler.NewAdminRAGHandler(ragResult.Service, deps.QuestionRepo)
    }
}
```

#### 4.2 Worker端集成

**文件**：`backend/cmd/worker/main.go`

在`runWorkerMQ`中注册RAG同步消费者：

```go
if cfg.Milvus.Enabled {
    ragResult, ragErr := rag.Init(ctx, rag.Config{...})
    if ragErr == nil {
        ragSyncConsumer := rag.NewSyncConsumer(ragResult.Service, questionRepo)
        handlers["makejob.async.rag.sync.question"] = mq.TaskHandlerFunc(func(ctx context.Context, message mq.TaskMessage) error {
            return ragSyncConsumer.Handle(ctx, message.Payload)
        })
    }
}
```

---

## 三、文件结构

```
backend/
├── internal/
│   ├── ai/
│   │   ├── eino/
│   │   │   ├── provider.go          # 已有 - ChatModel Provider
│   │   │   ├── embedding.go         # 新增 - 豆包Embedding封装
│   │   │   └── embedding_test.go    # 新增 - 单测
│   │   └── config.go                # 修改 - 新增RAG配置键
│   ├── rag/                         # 新增目录
│   │   ├── types.go                 # 接口定义 + 数据类型
│   │   ├── service.go               # RAG Service（编排层）
│   │   ├── collection.go            # Milvus Collection管理
│   │   ├── indexer.go               # Milvus Indexer实现
│   │   ├── retriever.go             # Milvus Retriever实现
│   │   ├── question_builder.go      # 题目→文档转换
│   │   ├── init.go                  # 初始化封装
│   │   └── sync_consumer.go         # RabbitMQ同步消费者
│   ├── handler/
│   │   └── admin_rag_handler.go     # 新增 - 管理后台RAG API
│   ├── mq/
│   │   └── message.go               # 修改 - 新增RAG消息类型
│   └── config/
│       └── config.go                # 修改 - 新增MilvusConfig
├── cmd/
│   ├── server/
│   │   └── main.go                  # 修改 - 注入RAG依赖
│   └── worker/
│       └── main.go                  # 修改 - 注册RAG消费者
├── config.yaml                      # 修改 - 新增milvus配置
└── docker-compose.yml               # 修改 - 新增Milvus服务
```

---

## 四、使用指南

### 4.1 环境准备

**1. 启动Milvus服务**

```bash
cd D:/gogogo/makejob
docker-compose up -d
```

等待所有服务启动完成（约1-2分钟），可通过以下命令检查：

```bash
# 检查Milvus健康状态
curl http://localhost:9091/healthz

# 检查Milvus管理界面（可选）
# 访问 http://localhost:9091
```

**2. 配置豆包Embedding**

在火山引擎控制台：
1. 创建豆包Embedding模型接入点
2. 获取接入点ID（格式：`ep-xxxxxxxxxx`）
3. 在`backend/config.yaml`中配置：

```yaml
ai:
  rag_embed_endpoint: "ep-xxxxxxxxxx"  # 你的接入点ID
```

**3. 重启服务**

```bash
# 重启server
cd backend
go run cmd/server/main.go

# 重启worker（另一个终端）
go run cmd/worker/main.go
```

### 4.2 数据索引

**全量索引现有题目**

```bash
curl -X POST http://localhost:8082/api/admin/rag/index-all \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{}'
```

响应示例：
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "indexed": 150
  }
}
```

**增量索引指定题目**

```bash
curl -X POST http://localhost:8082/api/admin/rag/index \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "question_ids": [1, 2, 3]
  }'
```

**删除索引**

```bash
curl -X DELETE http://localhost:8082/api/admin/rag/index \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "question_ids": [1, 2, 3]
  }'
```

### 4.3 语义检索测试

```bash
curl "http://localhost:8082/api/admin/rag/search?query=Redis缓存穿透如何解决&top_k=5" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

响应示例：
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "query": "Redis缓存穿透如何解决",
    "results": [
      {
        "ID": "q-42",
        "Content": "什么是缓存穿透？如何解决？\n缓存穿透是指...",
        "Score": 0.92,
        "MetaData": {
          "question_id": 42,
          "type": "technical",
          "difficulty": "medium",
          "tags": ["Redis", "缓存"]
        }
      }
    ]
  }
}
```

### 4.4 自动同步

题目创建/更新/删除时，系统会通过RabbitMQ自动同步到Milvus向量库。无需手动操作。

---

## 五、技术细节

### 5.1 Embedding维度处理

豆包`doubao-embedding`输出1536维float64向量，Milvus要求float32：

```go
func convertToFloat32(src []float64) []float32 {
    dst := make([]float32, len(src))
    for i, v := range src {
        dst[i] = float32(v)
    }
    return dst
}
```

### 5.2 批量Embedding策略

豆包API单次最多处理64条文本，实现分批处理：

```go
const embedBatchSize = 64

func (i *milvusIndexer) batchEmbed(ctx context.Context, texts []string) ([][]float32, error) {
    var allVectors [][]float32
    for start := 0; start < len(texts); start += embedBatchSize {
        end := min(start+embedBatchSize, len(texts))
        batch := texts[start:end]
        vectors, err := i.embedder.EmbedStrings(ctx, batch)
        // ... 转换并追加
    }
    return allVectors, nil
}
```

### 5.3 错误处理风格

遵循项目现有模式：

```go
// Repository层：fmt.Errorf包装
return nil, fmt.Errorf("查询题目失败: %w", err)

// Service层：业务错误用common.BusinessError
return nil, common.NewBusinessError(common.CodeNotFound, "题目不存在")

// AI层：本地兜底
if err != nil {
    return buildLocalQuestion()  // 回退到本地模板
}
```

### 5.4 日志风格

使用`applogger` + zap.Field：

```go
applogger.Info("RAG索引写入成功",
    zap.Int("count", len(docs)),
    zap.String("collection", s.config.Collection),
)
```

---

## 六、接下来的实施计划

### Phase 3: 面试模块RAG集成（预计3-4天）

#### 6.1 修改面试Agent出题逻辑

**目标**：在`interview_agent.go`的`buildQuestionUserPrompt`中集成RAG检索

```go
func (a *Agent) buildQuestionUserPrompt(ctx context.Context, session *InterviewSession, ragService *rag.Service) string {
    // 原有逻辑...
    
    // RAG增强：检索相关题目作为参考
    ragContext, _ := ragService.RetrieveByQuery(ctx, topic, 5)
    
    return fmt.Sprintf(`
请生成第 %d/%d 道面试题。
行业: %s
难度要求: %s
候选主题: %s
已出题列表: %s

## 参考题目（语义相似）:
%s

要求：题目和已出题不重复，保持循序渐进，结合真实面试场景。
`, currentIndex+1, total, industry, difficulty, topic, asked, ragContext)
}
```

#### 6.2 修改简历驱动面试Prompt

**目标**：在`interview_handler_realtime.go`的`buildResumeDrivenSystemPrompt`中注入RAG上下文

```go
func buildResumeDrivenSystemPrompt(profile *ResumeProfile, ragContext *rag.InterviewContext) string {
    basePrompt := `...原有五阶段面试指令...`
    
    if ragContext != nil && len(ragContext.SimilarQuestions) > 0 {
        basePrompt += fmt.Sprintf(`
## 相关面试题参考
%s

## 岗位技术要求
%s

请参考以上内容，设计更有针对性的面试问题。
`, ragContext.SimilarQuestionsPrompt(), ragContext.JobRequirementsPrompt())
    }
    
    return basePrompt
}
```

#### 6.3 面试上下文构建器

**新增文件**：`backend/internal/rag/interview_context.go`

```go
type InterviewContext struct {
    SimilarQuestions []Document
    JobRequirements  []Document
}

func (s *Service) BuildInterviewContext(ctx context.Context, query string, profile *ResumeProfile) (*InterviewContext, error) {
    // 1. 构建增强查询（结合简历画像）
    enhancedQuery := enhanceQueryWithProfile(query, profile)
    
    // 2. 语义检索
    docs, err := s.RetrieveByQuery(ctx, enhancedQuery, 5)
    
    // 3. 构建面试上下文
    return &InterviewContext{
        SimilarQuestions: docs,
    }, nil
}
```

---

### Phase 4: 知识库扩展（预计5-7天）

#### 6.4 面经知识库

**目标**：将爬取的面经数据也纳入RAG检索

- 新增Collection：`interview_experiences`
- 修改爬虫导入流程，同步到Milvus
- 面试时检索相似面经作为参考

#### 6.5 技术文档库

**目标**：引入技术知识文档，增强出题深度

- 手动维护或爬取技术文档
- 新增Collection：`tech_documents`
- 面试时检索相关技术知识点

#### 6.6 岗位要求库

**目标**：基于JD深度解析，精准匹配岗位要求

- 用户上传JD时自动解析并索引
- 新增Collection：`job_requirements`
- 面试时检索目标岗位的技术要求

---

### Phase 5: 智能评估增强（预计3-4天）

#### 6.7 答案评估RAG增强

**目标**：在`EvaluateAnswer`中检索标准答案和评分标准

```go
func (a *Agent) EvaluateAnswer(ctx context.Context, session *InterviewSession, answer string) (*AnswerFeedback, error) {
    // 1. 检索相关题目和标准答案
    ragDocs, _ := a.ragService.RetrieveByQuery(ctx, session.CurrentQuestion, 3)
    
    // 2. 构建评估上下文
    evalContext := buildEvaluationContext(ragDocs)
    
    // 3. 调用LLM评估
    feedback, err := a.callStructuredJSON[AnswerFeedback](ctx, messages)
    
    return feedback, nil
}
```

#### 6.8 面试报告RAG增强

**目标**：在`GenerateReport`中检索行业标准和评估基准

- 检索相似岗位的面试评估标准
- 检索行业最佳实践
- 生成更专业的面试报告

---

### Phase 6: 性能优化与监控（预计2-3天）

#### 6.9 缓存优化

- Embedding结果缓存（Redis）
- 热门查询结果缓存
- 向量索引预热

#### 6.10 监控指标

- RAG检索延迟
- Embedding API调用次数和耗时
- Milvus查询性能
- 索引同步延迟

#### 6.11 管理后台增强

- 知识库管理界面
- 检索效果评估工具
- 向量库状态监控

---

## 七、已知限制与注意事项

### 7.1 当前限制

1. **Embedding模型绑定**：当前绑定豆包Embedding，切换模型需修改代码
2. **单Collection设计**：所有题目存储在同一Collection，未来需按行业/类型拆分
3. **无缓存层**：每次检索都调用Milvus，高并发场景需优化
4. **同步延迟**：RabbitMQ异步同步存在秒级延迟

### 7.2 注意事项

1. **API Key安全**：`ai_rag_embed_endpoint`包含敏感信息，不要提交到版本控制
2. **Milvus资源**：Milvus Standalone模式适合开发测试，生产环境需部署集群
3. **Embedding成本**：豆包Embedding按调用次数计费，批量索引时注意成本
4. **向量维度一致性**：Collection创建后维度不可修改，切换Embedding模型需重建Collection

---

## 八、参考文档

- [Eino框架文档](https://github.com/cloudwego/eino)
- [Milvus官方文档](https://milvus.io/docs)
- [火山引擎Ark平台文档](https://www.volcengine.com/docs/82379)
- [豆包Embedding API](https://www.volcengine.com/docs/82379/1298454)
