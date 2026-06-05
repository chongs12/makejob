# MakeJob 微服务改造 — 分阶段执行任务指令库 (Phase 1-7)

> 本文档为任务执行库，每个任务包含完整的 PROMPT，可直接复制粘贴给执行模型使用。
> 执行模型在写任何代码时，都必须严格遵守 `docs/task-execution-library.md` 中的全局约定。

---

## Phase 1: 基础设施服务

---

### 任务 P1-1: CodeRunner Service - Execute RPC

**目标**：实现代码执行微服务，通过 Piston API 运行用户代码并支持批量测试用例。

**文件路径**：
- `app/coderunner/internal/biz/coderunner.go` (新建)
- `app/coderunner/internal/data/piston_client.go` (新建)
- `app/coderunner/internal/service/coderunner.go` (新建)
- `app/coderunner/internal/server/grpc.go` (新建)
- `app/coderunner/internal/conf/conf.go` (新建)
- `app/coderunner/cmd/server/main.go` (新建)

**单体参考**：`backend/internal/executor/piston.go`

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目实现微服务。

【任务】实现 CodeRunner 微服务的 Execute RPC，该服务负责调用 Piston API 执行用户代码，支持多语言和批量测试用例。

【目标】提供独立的代码执行能力，支持 go/python/javascript/java/cpp 五种语言，支持单次执行和批量测试用例模式。

【需要创建的文件】
- app/coderunner/internal/conf/conf.go
- app/coderunner/internal/biz/coderunner.go
- app/coderunner/internal/data/piston_client.go
- app/coderunner/internal/service/coderunner.go
- app/coderunner/internal/server/grpc.go
- app/coderunner/cmd/server/main.go
- app/coderunner/configs/config.yaml

【Proto 接口】
service CodeRunnerService {
  rpc Execute(ExecuteRequest) returns (ExecuteResponse);
}

message ExecuteRequest {
  string language = 1;       // go, python, javascript, java, cpp
  string code = 2;
  string stdin = 3;
  repeated TestCase test_cases = 4;
  int32 timeout_ms = 5;      // 默认 30000
}

message TestCase {
  string input = 1;
  string expected_output = 2;
}

message ExecuteResponse {
  bool success = 1;
  string stdout = 2;
  string stderr = 3;
  int32 exit_code = 4;
  int64 execution_time_ms = 5;
  repeated TestResult test_results = 6;
  int32 passed_count = 7;
  int32 total_count = 8;
}

message TestResult {
  string input = 1;
  string expected_output = 2;
  string actual_output = 3;
  bool passed = 4;
  int64 execution_time_ms = 5;
}

【实现步骤】
1. conf/conf.go: 定义 Config 结构体，包含 piston_endpoint(string, 默认 http://localhost:2000/api/v2/execute)、timeout_sec(int, 默认30)、grpc_addr(string)。实现 Load(path string) (*Config, error)。
2. biz/coderunner.go: 定义 ExecuteUseCase 结构体和 CodeExecutor 接口：
   - CodeExecutor 接口方法: Execute(ctx, language, code, stdin string) (*RunResult, error)
   - ExecuteUseCase 方法: Run(ctx, req) 负责：校验语言 → 若无 test_cases 则单次执行 → 若有 test_cases 则循环执行每个用例
   - 定义 RunResult{Stdout, Stderr, ExitCode int, ExecutionTimeMS int64}
3. data/piston_client.go: 实现 CodeExecutor 接口：
   - 语言映射表: go→go(1.21.0)/main.go, python→python(3.11.0)/main.py, javascript→javascript(18.15.0)/index.js, java→java(17.0.0)/Main.java, cpp→c++(17)/main.cpp
   - HTTP POST 到 Piston endpoint, body: {"language": langID, "version": version, "files": [{"content": code}], "stdin": stdin, "run_timeout": timeout_ms}
   - 解析响应: {"run": {"stdout", "stderr", "code", "signal"}}
4. service/coderunner.go: 实现 gRPC handler:
   - 校验 language 是否在支持列表
   - 若无 test_cases: 调用 UseCase.Run，填充 stdout/stderr/exit_code/success
   - 若有 test_cases: 遍历调用 UseCase.Run(stdin=test_case.input)，比较 stdout.TrimSpace == expected_output.TrimSpace，汇总 passed_count/total_count
   - success = (所有测试通过) 或 (无测试且exit_code==0)
5. server/grpc.go: 创建 gRPC server，注册 CodeRunnerService
6. cmd/server/main.go: 加载配置 → 初始化 PistonClient → 初始化 UseCase → 初始化 Service → 启动 gRPC server

【错误处理】
- language 不在支持列表 → errors.BadRequest("UNSUPPORTED_LANGUAGE", "不支持的编程语言: %s", language)
- Piston 超时 → errors.New(408, "EXECUTION_TIMEOUT", "代码执行超时")
- Piston 服务不可用(连接失败/非200响应) → errors.New(503, "PISTON_UNAVAILABLE", "代码执行引擎不可用")

【数据库操作】
- 无数据库操作，纯计算服务

【依赖的外部服务/库】
- Piston API: HTTP POST http://localhost:2000/api/v2/execute
- net/http: 用于调用 Piston

【验证标准】
1. 单次执行 go 代码 fmt.Println("hello") 返回 stdout="hello\n", success=true
2. 传入 test_cases=[{input:"1\n2", expected:"3"}] 和加法代码，passed_count=1
3. 不支持的语言返回 UNSUPPORTED_LANGUAGE 错误
4. Piston 不可用时返回 503 错误

【禁止事项】
- 禁止在 service 层直接 import net/http，HTTP 调用必须在 data 层
- 禁止硬编码 Piston 地址，必须从配置读取
- 禁止使用 fmt.Errorf 返回给 gRPC，必须用 kratos errors
- 禁止使用全局变量存储客户端实例
```

---

### 任务 P1-2: RAG Service - Retrieve RPC

**目标**：实现 RAG 服务的语义检索能力，通过 Embedding 向量化查询并在 Milvus 中搜索最相关的文档。

**文件路径**：
- `app/rag/internal/biz/rag.go` (新建)
- `app/rag/internal/data/milvus_client.go` (新建)
- `app/rag/internal/service/rag.go` (新建)
- `app/rag/internal/server/grpc.go` (新建)
- `app/rag/internal/conf/conf.go` (新建)
- `app/rag/cmd/server/main.go` (新建)

**单体参考**：`backend/internal/rag/retriever.go`, `backend/internal/rag/service.go`

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目实现微服务。

【任务】实现 RAG 微服务的 Retrieve RPC，提供语义检索能力。

【目标】接收查询文本，通过 Volcengine Ark Embedding API 转为向量，在 Milvus 中搜索最相似的 top-K 文档并返回。

【需要创建的文件】
- app/rag/internal/conf/conf.go
- app/rag/internal/biz/rag.go
- app/rag/internal/data/milvus_client.go
- app/rag/internal/service/rag.go
- app/rag/internal/server/grpc.go
- app/rag/cmd/server/main.go
- app/rag/configs/config.yaml

【Proto 接口】
service RAGService {
  rpc Retrieve(RetrieveRequest) returns (RetrieveResponse);
}

message RetrieveRequest {
  string query = 1;
  int32 top_k = 2;           // 默认 5
  string collection = 3;      // 可选，默认使用配置中的 collection
  map<string, string> filters = 4;  // 可选元数据过滤
}

message RetrieveResponse {
  repeated Document documents = 1;
}

message Document {
  string id = 1;
  string content = 2;
  float score = 3;
  map<string, string> metadata = 4;
}

【实现步骤】
1. conf/conf.go: Config 结构体包含:
   - milvus_addr(string, 默认 localhost:19530)
   - collection_name(string, 默认 makejob_questions)
   - ark_api_key(string)
   - ark_base_url(string, 默认 https://ark.cn-beijing.volces.com/api/v3)
   - embed_model(string, 默认 doubao-embedding-large-text-240915)
   - top_k(int, 默认 5)
   - grpc_addr(string)
2. biz/rag.go: 定义接口和实体:
   - Document{ID, Content, Score float64, MetaData map[string]any}
   - Embedder 接口: EmbedStrings(ctx, texts []string) ([][]float64, error)
   - VectorStore 接口: Search(ctx, vector []float32, topK int, collection string) ([]Document, error)
   - RetrieveUseCase: 组合 Embedder + VectorStore
   - Retrieve(ctx, query, collection, topK, filters) → embed query → search → return docs
3. data/milvus_client.go: 实现两部分:
   a) Embedder 实现: 使用 cloudwego/eino-ext 中的 ark embedding 组件
      - 初始化: embedding.NewEmbedder(arkConfig) 
      - EmbedStrings: 调用 embedder.EmbedStrings(ctx, texts)
   b) VectorStore 实现: 使用 milvus-io/milvus/client/v2
      - 初始化: milvusclient.New(ctx, &milvusclient.ClientConfig{Address: addr})
      - Search: 构建 SearchOption, 执行 client.Search, 解析 ResultSet
      - 输出字段: id, content, metadata
4. service/rag.go: gRPC handler:
   - 校验 query 非空
   - top_k 若为 0 则使用默认值
   - 调用 RetrieveUseCase.Retrieve
   - 转换 biz.Document → proto Document
5. server/grpc.go: 注册 RAGService
6. cmd/server/main.go: 装配所有依赖

【错误处理】
- Milvus 连接失败 → errors.New(503, "RAG_CONNECTION_FAILED", "向量数据库连接失败")
- Embedding API 调用失败 → errors.New(502, "EMBEDDING_FAILED", "文本向量化失败: %s", err)
- 检索结果为空 → errors.New(404, "NO_RESULTS", "未找到相关文档")
- query 为空 → errors.BadRequest("INVALID_QUERY", "查询文本不能为空")

【数据库操作】
- 无关系型数据库操作
- Milvus 操作: Search on collection "makejob_questions"
- 搜索字段: vector (FloatVector, dim=1024)
- 输出字段: id, content, metadata

【依赖的外部服务/库】
- Milvus: github.com/milvus-io/milvus/client/v2
- Eino Embedding: github.com/cloudwego/eino-ext (ark embedding 组件)
- Volcengine Ark API: embedding endpoint

【验证标准】
1. 传入 query="Go 并发编程" 能返回相关文档列表
2. top_k=3 时最多返回 3 条结果
3. 每条 Document 包含 id, content, score, metadata
4. Milvus 不可用时返回 503
5. 空 query 返回 400

【禁止事项】
- 禁止在 service 层直接操作 Milvus client
- 禁止硬编码 API Key，必须从配置文件读取
- 禁止忽略 context 传播，所有外部调用必须传递 ctx
- 禁止返回原始 error，必须包装为 kratos errors
```

---

### 任务 P1-3: RAG Service - IndexQuestions RPC

**目标**：实现 RAG 服务的批量索引能力，将题目内容向量化后存入 Milvus。

**文件路径**：
- `app/rag/internal/biz/rag.go` (修改，添加 IndexUseCase)
- `app/rag/internal/data/milvus_client.go` (修改，添加 Upsert 方法)
- `app/rag/internal/service/rag.go` (修改，添加 IndexQuestions handler)

**单体参考**：`backend/internal/rag/indexer.go`

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目实现微服务。

【任务】为 RAG 服务添加 IndexQuestions RPC，支持批量将题目内容向量化并存入 Milvus。

【目标】接收一批题目(id+content+metadata)，分批调用 Embedding API 生成向量，然后批量 Upsert 到 Milvus。

【需要修改的文件】
- app/rag/internal/biz/rag.go (添加 IndexUseCase 和 VectorStore.Upsert 接口方法)
- app/rag/internal/data/milvus_client.go (实现 Upsert)
- app/rag/internal/service/rag.go (添加 IndexQuestions handler)

【Proto 接口】
service RAGService {
  rpc IndexQuestions(IndexQuestionsRequest) returns (IndexQuestionsResponse);
}

message IndexQuestionsRequest {
  repeated IndexItem items = 1;
}

message IndexItem {
  string id = 1;
  string content = 2;
  map<string, string> metadata = 3;
}

message IndexQuestionsResponse {
  int32 indexed_count = 1;
  int32 failed_count = 2;
  repeated string failed_ids = 3;
}

【实现步骤】
1. biz/rag.go 添加:
   - VectorStore 接口新增: Upsert(ctx, collection string, docs []VectorDocument) error
   - VectorDocument{ID string, Content string, Vector []float32, Metadata map[string]any}
   - IndexUseCase 方法: IndexQuestions(ctx, items []IndexItem) (indexed, failed int, failedIDs []string)
     - 将 items 分批(每批最多 16 条)
     - 每批调用 Embedder.EmbedStrings 获取向量
     - 组装 VectorDocument 列表
     - 调用 VectorStore.Upsert
     - 统计成功/失败数
2. data/milvus_client.go 添加 Upsert 实现:
   - Milvus collection: "makejob_questions"
   - Schema fields: id(varchar, PK, max_length=64), content(varchar, max_length=65535), vector(float_vector, dim=1024), metadata(json)
   - 使用 milvusclient.NewColumnBasedInsertOption 构建插入数据
   - 调用 client.Upsert(ctx, option)
3. service/rag.go 添加 IndexQuestions:
   - 校验 items 非空
   - 调用 IndexUseCase.IndexQuestions
   - 返回统计结果

【错误处理】
- items 为空 → errors.BadRequest("INVALID_REQUEST", "索引项不能为空")
- 单批 Embedding 失败 → 记录 failedIDs，继续下一批(不中断整体)
- Milvus Upsert 失败 → 记录 failedIDs，继续下一批
- 所有批次都失败 → errors.New(500, "INDEX_FAILED", "全部索引失败")

【数据库操作】
- Milvus collection: makejob_questions
- 操作: Upsert (插入或更新)
- Schema: id(varchar PK), content(varchar), vector(float_vector dim=1024), metadata(json)

【依赖的外部服务/库】
- Embedder (已在 P1-2 实现): EmbedStrings 批量文本向量化
- Milvus client (已在 P1-2 初始化): Upsert 操作

【验证标准】
1. 传入 3 条 items 返回 indexed_count=3, failed_count=0
2. 16 条以上 items 会分多批处理
3. Embedding 某批失败时不影响其他批次
4. Upsert 后通过 Retrieve 可以检索到新增文档

【禁止事项】
- 禁止单条逐个处理，必须分批(max 16)
- 禁止整体事务(任意一条失败不能回滚已成功的)
- 禁止忽略部分失败的情况，必须返回 failed_ids
- 禁止在 Embedding 调用中不传递 ctx
```

---

### 任务 P1-4: RAG Service - MQ Consumer (question.changed)

**目标**：实现 RAG 服务的消息队列消费者，监听题目变更事件并同步更新 Milvus 索引。

**文件路径**：
- `app/rag/internal/server/mq.go` (新建)
- `app/rag/internal/biz/rag.go` (修改，添加 Delete 方法到 VectorStore 接口)
- `app/rag/internal/data/milvus_client.go` (修改，添加 Delete 实现)

**单体参考**：`backend/internal/rag/sync_consumer.go`

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目实现微服务。

【任务】为 RAG 服务实现 MQ 消费者，监听 question.changed 事件，自动同步 Milvus 索引。

【目标】当题目被创建/更新/删除时，通过 MQ 消息触发 Milvus 索引的对应操作(upsert/delete)。

【需要创建/修改的文件】
- app/rag/internal/server/mq.go (新建)
- app/rag/internal/biz/rag.go (修改)
- app/rag/internal/data/milvus_client.go (修改)

【MQ 协议】
- Exchange: makejob.tasks (topic 类型)
- Queue: makejob.tasks.rag.sync.question
- Routing Key: rag.sync.question
- Payload (TaskMessage.Payload):
  {
    "question_id": 123,        // uint64
    "action": "create",        // "create" | "update" | "delete"
    "content": "题目内容...",   // create/update 时必填
    "metadata": {              // 可选
      "category": "golang",
      "difficulty": "medium"
    }
  }

【实现步骤】
1. biz/rag.go 添加:
   - VectorStore 接口新增: Delete(ctx, collection string, ids []string) error
   - SyncHandler 结构体: 组合 Embedder + VectorStore
   - HandleQuestionChanged(ctx, payload) error:
     - 解析 payload 获取 action/question_id/content/metadata
     - action="create"/"update": embed content → upsert 到 Milvus (ID = fmt.Sprintf("%d", question_id))
     - action="delete": 调用 VectorStore.Delete(ctx, collection, []string{id})
2. data/milvus_client.go 添加 Delete:
   - 使用 milvusclient.NewDeleteOption(collection).WithIDs(entity.NewColumnVarChar("id", ids))
   - 调用 client.Delete(ctx, option)
3. server/mq.go:
   - 实现 pkg/mq.TaskHandler 接口
   - HandleTask(ctx, msg TaskMessage) error:
     - 解码 msg.Payload 为 QuestionChangedPayload 结构体
     - 调用 SyncHandler.HandleQuestionChanged
   - 注册: consumer.Register("makejob.tasks.rag.sync.question", handler)
4. cmd/server/main.go 中启动 MQ consumer (在 gRPC server 之后)

【错误处理】
- payload 解析失败 → 返回 error (消息会被 nack)
- action 非法 → log.Warn + 返回 nil (丢弃消息)
- Embedding 失败 → 返回 error (消息重试)
- Milvus 操作失败 → 返回 error (消息重试)
- 重试次数超过 3 次 → pkg/mq 框架自动进入死信队列

【数据库操作】
- Milvus: Upsert (create/update), Delete (delete)
- Collection: makejob_questions
- ID 格式: question_id 转为字符串

【依赖的外部服务/库】
- pkg/mq: Consumer.Register(queue, handler)
- Embedder (已实现): 用于 create/update 时向量化
- VectorStore (已实现): Upsert + Delete

【验证标准】
1. 发送 action="create" 消息后，Milvus 中能检索到该文档
2. 发送 action="update" 消息后，文档内容被更新
3. 发送 action="delete" 消息后，文档被删除
4. payload 格式错误时消息被 nack
5. consumer 启动后正确订阅 queue

【禁止事项】
- 禁止在 mq handler 中直接操作 Milvus，必须通过 biz 层
- 禁止忽略 ctx，所有操作必须传递 context
- 禁止手动 ack/nack，由 pkg/mq 框架管理
- 禁止阻塞式处理，单条消息处理超时应由框架控制
```

---

### 任务 P1-5: AI Gateway Service - Structure + InterviewAgent RPC

**目标**：实现 AI Gateway 微服务骨架及 InterviewAgent RPC，作为所有 AI 能力的统一网关。

**文件路径**：
- `app/ai_gateway/internal/biz/ai.go` (新建)
- `app/ai_gateway/internal/biz/runtime_builder.go` (新建)
- `app/ai_gateway/internal/data/ai_config_repo.go` (新建)
- `app/ai_gateway/internal/data/prompt_repo.go` (新建)
- `app/ai_gateway/internal/data/call_log_repo.go` (新建)
- `app/ai_gateway/internal/data/data.go` (新建)
- `app/ai_gateway/internal/service/ai.go` (新建)
- `app/ai_gateway/internal/server/grpc.go` (新建)
- `app/ai_gateway/internal/conf/conf.go` (新建)
- `app/ai_gateway/cmd/server/main.go` (新建)

**单体参考**：`backend/internal/ai/interview_agent.go`, `backend/internal/ai/types.go`, `backend/internal/ai/config.go`

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目实现微服务。

【任务】实现 AI Gateway 微服务的完整骨架 + InterviewAgent RPC。AI Gateway 是所有 AI 调用的统一入口，负责配置管理、Prompt 渲染、LLM 调用、日志记录。

【目标】建立 AI Gateway 的基础架构(配置表、Prompt模板表、调用日志表)，并实现第一个 RPC: InterviewAgent（面试出题）。

【需要创建的文件】
- app/ai_gateway/internal/conf/conf.go
- app/ai_gateway/internal/biz/ai.go
- app/ai_gateway/internal/biz/runtime_builder.go
- app/ai_gateway/internal/data/data.go
- app/ai_gateway/internal/data/ai_config_repo.go
- app/ai_gateway/internal/data/prompt_repo.go
- app/ai_gateway/internal/data/call_log_repo.go
- app/ai_gateway/internal/service/ai.go
- app/ai_gateway/internal/server/grpc.go
- app/ai_gateway/cmd/server/main.go
- app/ai_gateway/configs/config.yaml

【Proto 接口】
service AIGatewayService {
  rpc InterviewAgent(InterviewAgentRequest) returns (InterviewAgentResponse);
}

message InterviewAgentRequest {
  repeated ChatMessage history = 1;
  string resume = 2;
  string jd = 3;
  string industry = 4;
  int32 question_count = 5;
  string mode = 6;            // "question" | "report" | "evaluate"
  string difficulty = 7;
  repeated string weak_topics = 8;
}

message ChatMessage {
  string role = 1;            // system/user/assistant
  string content = 2;
}

message InterviewAgentResponse {
  string content = 1;
  string question_type = 2;   // technical/behavioral/coding
  string difficulty = 3;
  repeated TestCaseProto test_cases = 4;
  Live2DDirectiveProto live2d_directive = 5;
  int32 input_tokens = 6;
  int32 output_tokens = 7;
  int64 latency_ms = 8;
}

message TestCaseProto {
  string input = 1;
  string expected_output = 2;
}

message Live2DDirectiveProto {
  string emotion = 1;
  string action = 2;
  string motion_key = 3;
  int32 duration_ms = 4;
}

【数据库表结构】
表1: ai_configs
  - id uint PK
  - scene varchar(50) NOT NULL (interview/plan/companion/quiz_analyzer/resume_parser/live2d_director)
  - provider varchar(50) NOT NULL (volcengine)
  - model varchar(100) NOT NULL (doubao-1.5-pro-256k)
  - temperature float DEFAULT 0.7
  - max_tokens int DEFAULT 4096
  - extra_params_json text
  - is_active bool DEFAULT true
  - BaseModel (created_at, updated_at, deleted_at)

表2: prompt_templates
  - id uint PK
  - scene varchar(50) NOT NULL
  - version int NOT NULL DEFAULT 1
  - template_text text NOT NULL
  - variables_json text (定义模板变量列表)
  - is_active bool DEFAULT true
  - BaseModel

表3: ai_call_logs
  - id uint PK
  - scene varchar(50) NOT NULL
  - model varchar(100)
  - input_tokens int
  - output_tokens int
  - latency_ms int64
  - status varchar(20) (success/failed)
  - error_msg text
  - created_at timestamp

【实现步骤】
1. conf/conf.go: Config 包含 db(dsn), ark_api_key, ark_base_url, grpc_addr
2. data/data.go: 初始化 GORM 连接 + AutoMigrate 三张表
3. biz/ai.go: 定义:
   - AIConfig 实体(对应 ai_configs 表)
   - PromptTemplate 实体(对应 prompt_templates 表)
   - AICallLog 实体(对应 ai_call_logs 表)
   - AIConfigRepo 接口: GetActiveConfig(ctx, scene) (*AIConfig, error)
   - PromptRepo 接口: GetActiveTemplate(ctx, scene) (*PromptTemplate, error)
   - CallLogRepo 接口: Create(ctx, log *AICallLog) error
   - InterviewAgentUseCase: 组合以上 Repo + LLMClient
   - GenerateQuestion(ctx, req) → 加载配置 → 渲染 Prompt → 调用 LLM → 记录日志 → 解析输出
4. biz/runtime_builder.go:
   - LLMClient 接口: Chat(ctx, messages []Message, config AIConfig) (*LLMResponse, error)
   - LLMResponse{Content string, InputTokens, OutputTokens int}
   - 使用 cloudwego/eino 的 ChatModel 接口调用 Volcengine Ark
   - RenderPrompt(template string, variables map[string]string) string: 使用 strings.ReplaceAll 或 text/template
5. data/ 各 repo: 标准 GORM 实现
   - ai_config_repo: WHERE scene=? AND is_active=true LIMIT 1
   - prompt_repo: WHERE scene=? AND is_active=true ORDER BY version DESC LIMIT 1
   - call_log_repo: 纯 Create
6. service/ai.go: InterviewAgent handler:
   - 调用 UseCase.GenerateQuestion
   - 解析 LLM 返回的 JSON 结构 (question_type, difficulty, test_cases, live2d_directive)
   - 转换为 proto response
7. server/grpc.go + cmd/main.go: 标准启动

【错误处理】
- scene 对应的 config 不存在 → errors.NotFound("AI_CONFIG_NOT_FOUND", "AI配置未找到: scene=%s", scene)
- LLM 调用失败 → errors.New(502, "LLM_CALL_FAILED", "大模型调用失败: %s", err)
- Prompt 模板渲染失败 → errors.New(500, "PROMPT_RENDER_FAILED", "Prompt渲染失败")
- 所有失败都要记录 ai_call_logs (status="failed", error_msg=err.Error())

【数据库操作】
- 表 ai_configs: SELECT WHERE scene=? AND is_active=true
- 表 prompt_templates: SELECT WHERE scene=? AND is_active=true ORDER BY version DESC
- 表 ai_call_logs: INSERT

【依赖的外部服务/库】
- Volcengine Ark LLM API (通过 eino ChatModel)
- GORM: 数据库操作
- text/template 或 strings.ReplaceAll: Prompt 渲染

【验证标准】
1. 配置表有 interview 记录时，调用 InterviewAgent 能返回结构化面试题
2. ai_call_logs 表记录了每次调用(含 tokens 和 latency)
3. 配置不存在时返回 404
4. LLM 调用超时返回 502

【禁止事项】
- 禁止硬编码 model/temperature 等参数，必须从 ai_configs 表读取
- 禁止跳过日志记录，无论成功失败都必须写 ai_call_logs
- 禁止在 service 层直接调用 LLM，必须通过 biz 层
- 禁止使用 fmt.Errorf 返回给 gRPC
- 禁止使用全局 DB 变量
```

---

### 任务 P1-6: AI Gateway - PlanAgent RPC

**目标**：实现 AI Gateway 的 PlanAgent RPC，为用户生成个性化学习计划。

**文件路径**：
- `app/ai_gateway/internal/biz/ai.go` (修改，添加 PlanAgentUseCase)
- `app/ai_gateway/internal/service/ai.go` (修改，添加 PlanAgent handler)

**单体参考**：`backend/internal/ai/plan_agent.go`

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目实现微服务。

【任务】为 AI Gateway 添加 PlanAgent RPC，生成结构化学习计划。

【目标】根据用户薄弱点、最近活动、水平等信息，调用 LLM 生成结构化的学习计划 JSON。

【需要修改的文件】
- app/ai_gateway/internal/biz/ai.go (添加 PlanAgentUseCase)
- app/ai_gateway/internal/service/ai.go (添加 PlanAgent handler)

【Proto 接口】
service AIGatewayService {
  rpc PlanAgent(PlanAgentRequest) returns (PlanAgentResponse);
}

message PlanAgentRequest {
  repeated string weak_topics = 1;
  repeated string recent_activities = 2;
  string level = 3;            // beginner/intermediate/advanced
  int32 duration_days = 4;
  string industry = 5;
  int32 daily_study_minutes = 6;
  string goal_description = 7;
}

message PlanAgentResponse {
  string title = 1;
  string description = 2;
  repeated PlanPhase phases = 3;
  repeated PlanTaskProto tasks = 4;
  int32 input_tokens = 5;
  int32 output_tokens = 6;
  int64 latency_ms = 7;
}

message PlanPhase {
  string name = 1;
  string goal = 2;
  int32 start_day = 3;
  int32 end_day = 4;
}

message PlanTaskProto {
  string title = 1;
  string description = 2;
  string task_type = 3;       // study/practice/interview/review
  string phase = 4;
  int32 day_number = 5;
  int32 duration_minutes = 6;
  string priority = 7;        // high/medium/low
}

【实现步骤】
1. biz/ai.go 添加 PlanAgentUseCase:
   - scene = "plan"
   - GeneratePlan(ctx, req PlanAgentRequest) (*PlanResult, error):
     a. 从 AIConfigRepo 获取 scene="plan" 的配置
     b. 从 PromptRepo 获取 plan 的 Prompt 模板
     c. 渲染 Prompt，变量: {weak_topics, level, duration, industry, daily_time, goal}
     d. 构造 messages: [system prompt, user request summary]
     e. 调用 LLMClient.Chat
     f. 解析返回的 JSON 为 PlanResult{Title, Description, Phases[], Tasks[]}
     g. 记录 ai_call_logs
2. service/ai.go 添加 PlanAgent handler:
   - 调用 PlanAgentUseCase.GeneratePlan
   - 转换结果为 proto PlanAgentResponse

【错误处理】
- 与 P1-5 相同的错误模式: AI_CONFIG_NOT_FOUND, LLM_CALL_FAILED, PROMPT_RENDER_FAILED
- LLM 返回非法 JSON → errors.New(500, "PARSE_FAILED", "计划生成结果解析失败")

【数据库操作】
- 表 ai_configs: SELECT WHERE scene='plan' AND is_active=true
- 表 prompt_templates: SELECT WHERE scene='plan' AND is_active=true
- 表 ai_call_logs: INSERT

【依赖的外部服务/库】
- LLMClient (已在 P1-5 实现)
- AIConfigRepo, PromptRepo, CallLogRepo (已在 P1-5 实现)

【验证标准】
1. 调用 PlanAgent 返回包含 title, phases, tasks 的结构化响应
2. tasks 中 day_number 不超过 duration_days
3. ai_call_logs 记录了调用

【禁止事项】
- 禁止复制 P1-5 的代码，复用已有的 LLMClient/Repo
- 禁止硬编码 Prompt 模板，必须从数据库读取
- 禁止跳过 call log 记录
```

---

### 任务 P1-7: AI Gateway - CompanionAgent RPC

**目标**：实现 AI Gateway 的 CompanionAgent RPC，提供 AI 陪伴聊天能力。

**文件路径**：
- `app/ai_gateway/internal/biz/ai.go` (修改，添加 CompanionAgentUseCase)
- `app/ai_gateway/internal/service/ai.go` (修改，添加 CompanionAgent handler)

**单体参考**：`backend/internal/ai/companion_agent.go`

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目实现微服务。

【任务】为 AI Gateway 添加 CompanionAgent RPC，提供带有情感和 Live2D 控制的 AI 陪伴对话。

【目标】根据用户消息、情绪状态和学习上下文，生成包含文字回复、情感标签、动作指令和 Live2D 控制的完整响应。

【需要修改的文件】
- app/ai_gateway/internal/biz/ai.go (添加 CompanionAgentUseCase)
- app/ai_gateway/internal/service/ai.go (添加 CompanionAgent handler)

【Proto 接口】
service AIGatewayService {
  rpc CompanionAgent(CompanionAgentRequest) returns (CompanionAgentResponse);
}

message CompanionAgentRequest {
  repeated ChatMessage messages = 1;
  string user_emotion = 2;     // happy/sad/frustrated/neutral/anxious
  string learning_state = 3;   // studying/practicing/idle/tired
  string current_topic = 4;
}

message CompanionAgentResponse {
  string reply = 1;
  string emotion = 2;          // happy/neutral/encouraging/thinking/excited
  string mood = 3;
  string action = 4;           // idle/wave/nod/celebrate/comfort
  Live2DDirectiveProto live2d_directive = 5;
  int32 input_tokens = 6;
  int32 output_tokens = 7;
  int64 latency_ms = 8;
}

【实现步骤】
1. biz/ai.go 添加 CompanionAgentUseCase:
   - scene = "companion"
   - Chat(ctx, req) (*CompanionResult, error):
     a. 加载 config (scene="companion")
     b. 加载 Prompt 模板
     c. 渲染 Prompt，变量: {user_emotion, learning_state, current_topic}
     d. 构造 messages: [system prompt] + req.messages
     e. 调用 LLMClient.Chat
     f. 解析 JSON 输出: {reply, emotion, mood, action, live2d_directive}
     g. 记录日志
2. service/ai.go 添加 CompanionAgent handler

【错误处理】
- 标准 AI Gateway 错误模式: AI_CONFIG_NOT_FOUND, LLM_CALL_FAILED, PROMPT_RENDER_FAILED
- messages 为空 → errors.BadRequest("INVALID_REQUEST", "消息列表不能为空")

【数据库操作】
- 与 P1-5/P1-6 相同模式

【依赖的外部服务/库】
- LLMClient, AIConfigRepo, PromptRepo, CallLogRepo (已实现)

【验证标准】
1. 传入消息列表返回包含 reply + emotion + action 的响应
2. user_emotion="frustrated" 时，AI 回复带有 encouraging emotion
3. live2d_directive 中 emotion/action 与回复内容一致

【禁止事项】
- 禁止硬编码情感映射规则，交给 LLM 决定
- 禁止忽略 user_emotion 和 learning_state 上下文
- 禁止复制代码，复用公共基础设施
```

---

### 任务 P1-8: AI Gateway - QuizAnalyzer RPC

**目标**：实现 AI Gateway 的 QuizAnalyzer RPC，用于分析用户答题结果。

**文件路径**：
- `app/ai_gateway/internal/biz/ai.go` (修改，添加 QuizAnalyzerUseCase)
- `app/ai_gateway/internal/service/ai.go` (修改，添加 QuizAnalyzer handler)

**单体参考**：`backend/internal/ai/quiz_analyzer.go`

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目实现微服务。

【任务】为 AI Gateway 添加 QuizAnalyzer RPC，对用户的答题进行 AI 分析评估。

【目标】传入题目、用户答案和正确答案，调用 LLM 返回结构化的评判结果(是否正确、评分、反馈、错误标签)。

【需要修改的文件】
- app/ai_gateway/internal/biz/ai.go
- app/ai_gateway/internal/service/ai.go

【Proto 接口】
service AIGatewayService {
  rpc QuizAnalyzer(QuizAnalyzerRequest) returns (QuizAnalyzerResponse);
}

message QuizAnalyzerRequest {
  string question = 1;
  string user_answer = 2;
  string correct_answer = 3;
  string question_type = 4;    // choice/short_answer/coding/essay
  string code_language = 5;    // coding 类型时的语言
  string context = 6;          // 附加上下文(可选)
}

message QuizAnalyzerResponse {
  bool is_correct = 1;
  float score = 2;             // 0-100
  string feedback = 3;
  repeated string mistake_tags = 4;
  repeated string suggestions = 5;
  string time_complexity = 6;   // coding 类型时
  string space_complexity = 7;  // coding 类型时
  int32 input_tokens = 8;
  int32 output_tokens = 9;
  int64 latency_ms = 10;
}

【实现步骤】
1. biz/ai.go 添加 QuizAnalyzerUseCase:
   - scene = "quiz_analyzer"
   - Analyze(ctx, req) (*QuizAnalysisResult, error):
     a. 加载 config (scene="quiz_analyzer")
     b. 加载 Prompt 模板
     c. 渲染 Prompt，变量: {question, user_answer, correct_answer, question_type, code_language}
     d. 调用 LLMClient.Chat
     e. 解析 JSON: {is_correct, score, feedback, mistake_tags[], suggestions[], time_complexity, space_complexity}
     f. 记录日志
2. service/ai.go 添加 QuizAnalyzer handler

【错误处理】
- 标准 AI Gateway 错误模式
- question 或 user_answer 为空 → errors.BadRequest("INVALID_REQUEST", "题目和答案不能为空")

【数据库操作】
- 与其他 AI Gateway RPC 相同

【依赖的外部服务/库】
- LLMClient, Repos (已实现)

【验证标准】
1. 选择题: 传入正确答案返回 is_correct=true, score=100
2. 编程题: 返回包含 time_complexity 和 space_complexity
3. mistake_tags 为非空数组(当答案错误时)
4. feedback 长度 > 0

【禁止事项】
- 禁止硬编码评分规则，交给 LLM 判断
- 禁止 question_type="coding" 时忽略 code_language 参数
- 禁止返回空 feedback
```

---

### 任务 P1-9: AI Gateway - ResumeParser RPC

**目标**：实现 AI Gateway 的 ResumeParser RPC，从简历文本中提取结构化信息。

**文件路径**：
- `app/ai_gateway/internal/biz/ai.go` (修改，添加 ResumeParserUseCase)
- `app/ai_gateway/internal/service/ai.go` (修改，添加 ResumeParser handler)

**单体参考**：`backend/internal/ai/resume_parser.go`

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目实现微服务。

【任务】为 AI Gateway 添加 ResumeParser RPC，通过 LLM 从简历文本中提取结构化信息。

【目标】输入简历原始文本，输出结构化的技能列表、工作经验、教育背景、项目经验和总结。

【需要修改的文件】
- app/ai_gateway/internal/biz/ai.go
- app/ai_gateway/internal/service/ai.go

【Proto 接口】
service AIGatewayService {
  rpc ResumeParser(ResumeParserRequest) returns (ResumeParserResponse);
}

message ResumeParserRequest {
  string resume_text = 1;
}

message ResumeParserResponse {
  repeated string skills = 1;
  repeated ExperienceItem experience = 2;
  repeated EducationItem education = 3;
  repeated ProjectItem projects = 4;
  string summary = 5;
  repeated string strengths = 6;
  repeated string weak_signals = 7;
  int32 input_tokens = 8;
  int32 output_tokens = 9;
  int64 latency_ms = 10;
}

message ExperienceItem {
  string company = 1;
  string title = 2;
  string duration = 3;
  string description = 4;
}

message EducationItem {
  string school = 1;
  string degree = 2;
  string major = 3;
  string year = 4;
}

message ProjectItem {
  string name = 1;
  string role = 2;
  string description = 3;
  repeated string tech_stack = 4;
}

【实现步骤】
1. biz/ai.go 添加 ResumeParserUseCase:
   - scene = "resume_parser"
   - Parse(ctx, resumeText string) (*ResumeProfile, error):
     a. 加载 config + prompt 模板
     b. 渲染 Prompt，变量: {resume_text}
     c. 调用 LLM (temperature 建议用 0.1 以保证稳定输出)
     d. 解析 JSON 为 ResumeProfile 结构体
     e. 记录日志
2. service/ai.go 添加 ResumeParser handler

【错误处理】
- resume_text 为空 → errors.BadRequest("INVALID_REQUEST", "简历文本不能为空")
- resume_text 超过 50000 字符 → errors.BadRequest("CONTENT_TOO_LONG", "简历内容过长")
- 标准 AI Gateway 错误模式

【数据库操作】
- 与其他 AI Gateway RPC 相同

【依赖的外部服务/库】
- LLMClient, Repos (已实现)

【验证标准】
1. 传入有效简历文本，返回非空 skills 列表
2. experience 中每条包含 company + title
3. summary 长度 > 0
4. 空文本返回 400

【禁止事项】
- 禁止使用正则表达式解析简历，必须全部交给 LLM
- 禁止忽略 temperature 配置(解析任务应低 temperature)
- 禁止截断简历文本，完整传递给 LLM
```

---

### 任务 P1-10: AI Gateway - Live2DDirector RPC

**目标**：实现 AI Gateway 的 Live2DDirector RPC，为 Live2D 角色生成表情和动作指令。

**文件路径**：
- `app/ai_gateway/internal/biz/ai.go` (修改，添加 Live2DDirectorUseCase)
- `app/ai_gateway/internal/service/ai.go` (修改，添加 Live2DDirector handler)

**单体参考**：`backend/internal/ai/types.go` (Live2DDirective 相关定义)

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目实现微服务。

【任务】为 AI Gateway 添加 Live2DDirector RPC，根据文本内容和场景生成 Live2D 角色控制指令。

【目标】输入当前情感状态、场景和要表达的文本，输出 Live2D 角色的表情、动作、手势和时长控制。

【需要修改的文件】
- app/ai_gateway/internal/biz/ai.go
- app/ai_gateway/internal/service/ai.go

【Proto 接口】
service AIGatewayService {
  rpc Live2DDirector(Live2DDirectorRequest) returns (Live2DDirectorResponse);
}

message Live2DDirectorRequest {
  string current_emotion = 1;
  string scene = 2;            // interview/companion/study
  string text_to_express = 3;
  string user_emotion = 4;
}

message Live2DDirectorResponse {
  string expression = 1;       // smile/neutral/thinking/surprised/sad
  string motion = 2;           // idle/nod/wave/celebrate/comfort
  string gesture = 3;          // point/thumbs_up/thinking_pose/none
  int32 duration_ms = 4;
  string motion_group = 5;
  string motion_priority = 6;  // normal/force
  float intensity = 7;         // 0.0-1.0
  int32 input_tokens = 8;
  int32 output_tokens = 9;
  int64 latency_ms = 10;
}

【实现步骤】
1. biz/ai.go 添加 Live2DDirectorUseCase:
   - scene = "live2d_director"
   - GenerateDirective(ctx, req) (*Live2DDirectiveResult, error):
     a. 加载 config + prompt
     b. 渲染 Prompt，变量: {current_emotion, scene, text_to_express, user_emotion}
     c. 调用 LLM (低 temperature=0.3)
     d. 解析 JSON
     e. 记录日志
2. service/ai.go 添加 handler

【错误处理】
- text_to_express 为空 → errors.BadRequest("INVALID_REQUEST", "表达文本不能为空")
- 标准 AI Gateway 错误模式

【数据库操作】
- 与其他 AI Gateway RPC 相同

【依赖的外部服务/库】
- LLMClient, Repos (已实现)

【验证标准】
1. 传入正面情绪文本返回 smile/celebrate 类表情
2. duration_ms > 0
3. intensity 在 0-1 范围内

【禁止事项】
- 禁止硬编码表情/动作映射，必须由 LLM 生成
- 禁止返回空的 expression 或 motion
- 禁止 duration_ms 为 0
```

---

## Phase 2: 用户 + 会员 + 社区

---

### 任务 P2-1: User Service - Fix RefreshToken Bug

**目标**：修复用户服务中 Login/Register 不返回 refresh_token 以及 RefreshToken RPC 逻辑错误的问题。

**文件路径**：
- `app/user/internal/service/user.go` (修改)

**单体参考**：`backend/internal/service/auth_service.go`

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目修复 Bug。

【任务】修复 User 微服务中 refresh_token 相关的两个 Bug:
1. Login/Register RPC 只生成了 access_token，未生成 refresh_token
2. RefreshToken RPC 复用了 access_token 的逻辑，未正确实现刷新机制

【目标】Login/Register 返回双 token（短期 access + 长期 refresh），RefreshToken 正确验证 refresh_token 后签发新的 token 对。

【需要修改的文件】
- app/user/internal/service/user.go

【Proto 接口】(已存在，只需修改实现)
message LoginResponse {
  string access_token = 1;
  string refresh_token = 2;
  int64 expires_in = 3;
}

message RefreshTokenRequest {
  string refresh_token = 1;
}

message RefreshTokenResponse {
  string access_token = 1;
  string refresh_token = 2;
  int64 expires_in = 3;
}

【实现步骤】
1. 在 Login/Register 方法中:
   - 生成 access_token: auth.GenerateToken(userID, 2*time.Hour)  // 短期 2 小时
   - 生成 refresh_token: auth.GenerateToken(userID, 7*24*time.Hour)  // 长期 7 天
   - 两个 token 都写入 response
   - expires_in = 7200 (秒)
2. 在 RefreshToken 方法中:
   - 调用 auth.ParseToken(req.RefreshToken) 验证 refresh_token
   - 如果验证失败 → 返回 UNAUTHORIZED
   - 从 token 中提取 userID
   - 生成新的 access_token (2h) + 新的 refresh_token (7d)
   - 返回新 token 对

【错误处理】
- refresh_token 为空 → errors.BadRequest("INVALID_TOKEN", "refresh_token 不能为空")
- refresh_token 无效/过期 → errors.Unauthorized("TOKEN_EXPIRED", "refresh_token 已失效，请重新登录")
- 生成 token 失败 → errors.New(500, "TOKEN_GENERATION_FAILED", "Token 生成失败")

【数据库操作】
- 无新增数据库操作

【依赖的外部服务/库】
- pkg/auth: GenerateToken(userID uint, expiry time.Duration) (string, error)
- pkg/auth: ParseToken(tokenString string) (*Claims, error)

【验证标准】
1. Login 返回两个不同的 token 字符串
2. access_token 有效期为 2 小时
3. refresh_token 有效期为 7 天
4. 用过期的 access_token 调用 RefreshToken 失败(因为它不是 refresh_token)
5. 用有效的 refresh_token 能获取新的 token 对

【禁止事项】
- 禁止 access_token 和 refresh_token 使用相同的过期时间
- 禁止在 RefreshToken 中不验证就签发新 token
- 禁止返回旧的 refresh_token（必须每次刷新都生成新的）
- 禁止硬编码 token 过期时间，建议从 conf 读取（但本次修复可以使用常量）
```

---

### 任务 P2-2: User Service - Add Logout RPC

**目标**：实现用户登出功能，通过 Redis Token 黑名单使已签发的 Token 立即失效。

**文件路径**：
- `app/user/internal/biz/user.go` (修改，添加 TokenBlacklist 接口)
- `app/user/internal/data/redis_client.go` (新建)
- `app/user/internal/service/user.go` (修改，添加 Logout handler)
- `app/user/internal/conf/conf.go` (修改，添加 Redis 配置)

**单体参考**：无直接对应(新功能)

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目实现微服务。

【任务】为 User 服务添加 Logout RPC，通过 Redis 黑名单机制使 token 立即失效。

【目标】用户调用 Logout 后，其当前 access_token 和 refresh_token 被加入黑名单，后续任何使用这些 token 的请求都会被拒绝。

【需要创建/修改的文件】
- app/user/internal/conf/conf.go (添加 redis_addr)
- app/user/internal/biz/user.go (添加 TokenBlacklist 接口)
- app/user/internal/data/redis_client.go (新建，实现黑名单)
- app/user/internal/service/user.go (添加 Logout handler)

【Proto 接口】
service UserService {
  rpc Logout(LogoutRequest) returns (google.protobuf.Empty);
}

message LogoutRequest {
  string access_token = 1;
  string refresh_token = 2;
}

【实现步骤】
1. conf/conf.go 添加 Redis 配置: redis_addr(string, 默认 localhost:6379)
2. biz/user.go 添加接口:
   - TokenBlacklist 接口:
     - Add(ctx, tokenJTI string, ttl time.Duration) error
     - IsBlacklisted(ctx, tokenJTI string) (bool, error)
3. data/redis_client.go 实现 TokenBlacklist:
   - 使用 github.com/redis/go-redis/v9
   - Add: SET "token_blacklist:{jti}" "1" EX ttl_seconds
   - IsBlacklisted: EXISTS "token_blacklist:{jti}"
4. service/user.go 添加 Logout:
   - 解析 access_token 获取 JTI 和过期时间
   - 解析 refresh_token 获取 JTI 和过期时间
   - 计算剩余 TTL = exp - time.Now()
   - 调用 TokenBlacklist.Add(ctx, accessJTI, accessTTL)
   - 调用 TokenBlacklist.Add(ctx, refreshJTI, refreshTTL)
   - 即使解析失败也返回成功(idempotent)
5. 【重要】Auth 中间件需要检查黑名单:
   - 在 gRPC interceptor 中, ParseToken 成功后, 调用 IsBlacklisted(jti)
   - 如果在黑名单中 → 返回 UNAUTHORIZED

【错误处理】
- Logout 本身永远返回成功(幂等性)
- Redis 不可用时 Logout → log.Error + 返回成功(降级)
- Redis 不可用时 Auth 检查 → log.Error + 放行(降级，不因缓存故障阻塞所有请求)

【数据库操作】
- Redis SET: key="token_blacklist:{jti}", value="1", EX=remaining_ttl
- Redis EXISTS: key="token_blacklist:{jti}"

【依赖的外部服务/库】
- github.com/redis/go-redis/v9
- pkg/auth: ParseToken 获取 JTI 和 Exp

【验证标准】
1. Logout 后用同一 access_token 请求被拒绝
2. Logout 后用同一 refresh_token 刷新被拒绝
3. 其他用户的 token 不受影响
4. TTL 过后黑名单自动清除
5. Redis 不可用时 Logout 不报错

【禁止事项】
- 禁止将整个 token 字符串存入 Redis(太长)，只存 JTI
- 禁止设置永久过期的黑名单条目(浪费内存)
- 禁止 Logout 失败时返回错误给用户
- 禁止在无 Redis 时让所有认证请求失败
```

---

### 任务 P2-3: Membership Service - Complete Implementation

**目标**：完整实现会员服务的 8 个 RPC，包括订单创建、支付回调处理和功能权限检查。

**文件路径**：
- `app/membership/internal/biz/membership.go` (修改/重写)
- `app/membership/internal/data/membership_repo.go` (修改/重写)
- `app/membership/internal/service/membership.go` (修改/重写)
- `app/membership/internal/server/grpc.go` (修改)
- `app/membership/internal/conf/conf.go` (修改)

**单体参考**：`backend/internal/service/membership_service.go`

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目实现微服务。

【任务】完整实现 Membership 微服务的所有 RPC，包括订单管理、支付处理和权限控制。

【目标】提供完整的会员服务能力：创建订单、处理支付回调、查询订单、检查功能访问权限、查询用户会员信息。

【需要修改的文件】
- app/membership/internal/biz/membership.go
- app/membership/internal/data/membership_repo.go
- app/membership/internal/service/membership.go
- app/membership/internal/server/grpc.go
- app/membership/internal/conf/conf.go

【Proto 接口】
service MembershipService {
  rpc CreateOrder(CreateOrderRequest) returns (CreateOrderResponse);
  rpc HandlePaymentCallback(PaymentCallbackRequest) returns (PaymentCallbackResponse);
  rpc GetOrder(GetOrderRequest) returns (OrderInfo);
  rpc ListOrders(ListOrdersRequest) returns (ListOrdersResponse);
  rpc CheckFeatureAccess(CheckFeatureAccessRequest) returns (CheckFeatureAccessResponse);
  rpc GetUserMembership(GetUserMembershipRequest) returns (UserMembershipInfo);
  rpc CancelOrder(CancelOrderRequest) returns (google.protobuf.Empty);
  rpc GetMembershipPlans(google.protobuf.Empty) returns (MembershipPlansResponse);
}

message CreateOrderRequest {
  uint64 user_id = 1;
  string plan_id = 2;          // monthly/quarterly/yearly
  string payment_method = 3;   // wechat/alipay
}

message CreateOrderResponse {
  string order_no = 1;
  float amount = 2;
  string payment_url = 3;
}

message PaymentCallbackRequest {
  string order_no = 1;
  string transaction_id = 2;
  string status = 3;           // success/failed
  float paid_amount = 4;
}

message CheckFeatureAccessRequest {
  uint64 user_id = 1;
  string feature = 2;          // daily_practice/daily_interview/advanced_ai
}

message CheckFeatureAccessResponse {
  bool allowed = 1;
  int32 remaining_count = 2;   // -1 表示无限
  string reason = 3;
}

【数据库表结构】
表: membership_orders
  - id uint PK
  - user_id uint NOT NULL (index)
  - order_no varchar(32) NOT NULL (unique)
  - plan_id varchar(20) NOT NULL
  - amount float NOT NULL
  - payment_method varchar(20)
  - transaction_id varchar(100)
  - status varchar(20) NOT NULL DEFAULT 'pending' (pending/paid/cancelled/refunded)
  - paid_at *time.Time
  - expires_at time.Time
  - BaseModel

表: user_memberships (如需新增)
  - id uint PK
  - user_id uint NOT NULL (unique)
  - level varchar(20) DEFAULT 'free' (free/monthly/quarterly/yearly)
  - expires_at time.Time
  - BaseModel

【实现步骤】
1. biz/membership.go:
   - 实体: MembershipOrder, UserMembership
   - Repo 接口: OrderRepo, MembershipRepo
   - UseCase 方法:
     a. CreateOrder: 生成 order_no = "MJ" + time.Now().Format("20060102150405") + 6位随机数字
     b. HandlePaymentCallback: 查询订单 → 验证 status==pending → 更新为 paid → 计算 expires_at → 更新 user_memberships
     c. CheckFeatureAccess: 查询 user_memberships → 根据 level 判断权限
        - free: daily_practice=20, daily_interview=2, advanced_ai=false
        - monthly+: 全部无限制
     d. GetUserMembership: 查询 user_memberships + 检查是否过期
2. data/membership_repo.go: GORM 实现
   - CreateOrder: INSERT membership_orders
   - GetOrderByNo: WHERE order_no=? AND deleted_at IS NULL
   - UpdateOrderStatus: UPDATE SET status=?, paid_at=?, transaction_id=?
   - GetUserMembership: WHERE user_id=?
   - UpsertMembership: 若存在则更新 level+expires_at，否则创建
   - CountTodayUsage: SELECT COUNT(*) FROM usage_logs WHERE user_id=? AND feature=? AND DATE(created_at)=CURDATE()
3. service/membership.go: 各 handler 实现

【错误处理】
- 订单不存在 → errors.NotFound("ORDER_NOT_FOUND", "订单不存在")
- 订单状态非 pending → errors.New(409, "ORDER_STATUS_CONFLICT", "订单状态不允许此操作")
- 功能不可用 → CheckFeatureAccess 返回 allowed=false + reason，不返回 error
- plan_id 非法 → errors.BadRequest("INVALID_PLAN", "无效的会员计划")

【数据库操作】
- membership_orders: INSERT, SELECT WHERE order_no=?, UPDATE SET status/paid_at
- user_memberships: SELECT WHERE user_id=?, INSERT/UPDATE (upsert)

【依赖的外部服务/库】
- GORM
- math/rand + time: 生成订单号
- User Service (可选): 更新用户 membership_level 字段

【验证标准】
1. CreateOrder 生成格式正确的 order_no (MJ + 14位时间 + 6位随机)
2. HandlePaymentCallback 将 pending 订单转为 paid
3. paid 订单不能再次支付(返回 409)
4. free 用户 daily_practice 检查第 21 次返回 allowed=false
5. 会员用户 daily_practice 返回 remaining_count=-1

【禁止事项】
- 禁止在 CreateOrder 时不检查 plan_id 合法性
- 禁止 HandlePaymentCallback 不验证订单当前状态
- 禁止使用 math/rand 不初始化种子
- 禁止直接修改 user 表，通过 gRPC 调用 User 服务(或内部更新 user_memberships 表)
- 禁止 CheckFeatureAccess 返回 gRPC error(应返回 allowed=false)
```

---

### 任务 P2-4: Community Service - Implement UpdatePost

**目标**：实现社区服务的更新帖子功能，确保只有作者本人可以修改。

**文件路径**：
- `app/community/internal/biz/community.go` (修改)
- `app/community/internal/service/community.go` (修改)

**单体参考**：无直接对应(新 RPC)

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目实现微服务。

【任务】为 Community 服务实现 UpdatePost RPC，支持作者修改帖子标题、内容和标签。

【目标】只有帖子作者可以更新帖子，更新时重新校验内容规则并重新计算摘要。

【需要修改的文件】
- app/community/internal/biz/community.go
- app/community/internal/service/community.go

【Proto 接口】
service CommunityService {
  rpc UpdatePost(UpdatePostRequest) returns (PostInfo);
}

message UpdatePostRequest {
  uint64 post_id = 1;
  string title = 2;
  string content = 3;
  repeated string tags = 4;
}

【实现步骤】
1. biz/community.go 添加 UpdatePost UseCase 方法:
   - 通过 PostRepo.GetByID(ctx, postID) 获取帖子
   - 若不存在 → POST_NOT_FOUND
   - 获取当前用户 ID (从 ctx): auth.GetUserIDFromContext(ctx)
   - 若 post.AuthorID != currentUserID → FORBIDDEN
   - 校验: title 非空且 <= 120 字符, content <= 5000 字符, tags <= 5 个
   - 更新 title, content, tags
   - 重新计算 summary: 取 content 前 200 字符(或去除 markdown 后前 200)
   - 调用 PostRepo.Update(ctx, post)
2. service/community.go 添加 UpdatePost handler:
   - 转换 request → biz 层调用
   - 转换结果为 proto PostInfo

【错误处理】
- 帖子不存在 → errors.NotFound("POST_NOT_FOUND", "帖子不存在")
- 非作者操作 → errors.New(403, "FORBIDDEN", "只有作者可以修改帖子")
- title 为空 → errors.BadRequest("INVALID_TITLE", "标题不能为空")
- content 超长 → errors.BadRequest("CONTENT_TOO_LONG", "内容不能超过5000字符")
- tags 超过 5 个 → errors.BadRequest("TOO_MANY_TAGS", "标签不能超过5个")

【数据库操作】
- 表: posts
- 查询: SELECT * FROM posts WHERE id=? AND deleted_at IS NULL
- 更新: UPDATE posts SET title=?, content=?, tags=?, summary=?, updated_at=NOW() WHERE id=?

【依赖的外部服务/库】
- pkg/auth: GetUserIDFromContext
- GORM

【验证标准】
1. 作者可以成功更新自己的帖子
2. 非作者更新返回 403
3. 帖子不存在返回 404
4. 超长 content 返回 400
5. 更新后 summary 被重新计算

【禁止事项】
- 禁止允许修改 post_type 和 author_id
- 禁止跳过权限验证
- 禁止不重新计算 summary
- 禁止从 request body 获取 user_id，必须从 ctx 获取
```

---

### 任务 P2-5: Community Service - Implement ToggleLike

**目标**：实现社区帖子的点赞/取消点赞功能，使用事务保证原子性。

**文件路径**：
- `app/community/internal/biz/community.go` (修改)
- `app/community/internal/data/community_repo.go` (修改)
- `app/community/internal/service/community.go` (修改)

**单体参考**：无直接对应

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目实现微服务。

【任务】为 Community 服务实现 ToggleLike RPC，点赞/取消点赞切换模式。

【目标】如果用户已点赞则取消(删除记录+count-1)，未点赞则添加(创建记录+count+1)，操作必须在事务中完成。

【需要修改的文件】
- app/community/internal/biz/community.go
- app/community/internal/data/community_repo.go
- app/community/internal/service/community.go

【Proto 接口】
service CommunityService {
  rpc ToggleLike(ToggleLikeRequest) returns (ToggleLikeResponse);
}

message ToggleLikeRequest {
  uint64 post_id = 1;
}

message ToggleLikeResponse {
  bool liked = 1;
  int32 like_count = 2;
}

【实现步骤】
1. biz/community.go:
   - 新增实体 PostLike{BaseModel, PostID uint, UserID uint} TableName="post_likes"
   - LikeRepo 接口:
     - GetByPostAndUser(ctx, postID, userID uint) (*PostLike, error)
     - Create(ctx, like *PostLike) error
     - Delete(ctx, postID, userID uint) error
   - PostRepo 接口添加:
     - IncrementLikeCount(ctx, postID uint, delta int) error
   - ToggleLikeUseCase(ctx, postID uint) (liked bool, likeCount int32, err error):
     - 获取 userID from ctx
     - 验证 post 存在
     - 在事务中:
       - 查询 PostLike 是否存在
       - 存在 → Delete + IncrementLikeCount(postID, -1) → liked=false
       - 不存在 → Create + IncrementLikeCount(postID, +1) → liked=true
     - 重新查询 post.like_count 返回
2. data/community_repo.go:
   - GetByPostAndUser: SELECT FROM post_likes WHERE post_id=? AND user_id=? AND deleted_at IS NULL
   - Create: INSERT INTO post_likes
   - Delete: 硬删除 DELETE FROM post_likes WHERE post_id=? AND user_id=?
   - IncrementLikeCount: UPDATE posts SET like_count = like_count + ? WHERE id=?
   - 事务: 使用 db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {...})
3. service/community.go 添加 ToggleLike handler

【错误处理】
- 帖子不存在 → errors.NotFound("POST_NOT_FOUND", "帖子不存在")
- 事务失败 → errors.New(500, "INTERNAL", "操作失败，请重试")

【数据库操作】
- 表 post_likes: SELECT/INSERT/DELETE
- 表 posts: UPDATE like_count = like_count + delta
- 唯一约束: post_likes (post_id, user_id) UNIQUE
- 必须在同一事务中

【依赖的外部服务/库】
- GORM (Transaction)
- pkg/auth: GetUserIDFromContext

【验证标准】
1. 首次点赞: liked=true, like_count +1
2. 再次调用: liked=false, like_count -1
3. 并发安全: like_count 不会变为负数
4. 帖子不存在返回 404

【禁止事项】
- 禁止不使用事务(非原子操作会导致 count 不一致)
- 禁止使用软删除 post_likes(浪费存储)
- 禁止先查后改不加事务(TOCTOU 问题)
- 禁止直接 SET like_count=N(必须用 like_count + delta 原子操作)
```

---

### 任务 P2-6: Community Service - Implement ListMyPosts + Enhance Existing RPCs

**目标**：新增 ListMyPosts RPC 并增强现有帖子列表/详情接口的功能。

**文件路径**：
- `app/community/internal/biz/community.go` (修改)
- `app/community/internal/data/community_repo.go` (修改)
- `app/community/internal/service/community.go` (修改)

**单体参考**：无

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目实现微服务。

【任务】为 Community 服务添加 ListMyPosts RPC，并增强 ListPosts、GetPost、CreatePost 的能力。

【目标】
1. ListMyPosts: 查询当前用户发布的帖子
2. ListPosts 增强: 支持按 post_type/keyword/tag 过滤，返回 is_liked/is_author 状态
3. GetPost 增强: 增加浏览量统计，返回 is_liked 状态
4. CreatePost 增强: 添加输入验证

【需要修改的文件】
- app/community/internal/biz/community.go
- app/community/internal/data/community_repo.go
- app/community/internal/service/community.go

【Proto 接口】
service CommunityService {
  rpc ListMyPosts(ListMyPostsRequest) returns (ListPostsResponse);
}

message ListMyPostsRequest {
  int32 page = 1;
  int32 page_size = 2;
}

// ListPosts 增强 request
message ListPostsRequest {
  int32 page = 1;
  int32 page_size = 2;
  string post_type = 3;        // 新增过滤: discussion/article/question
  string keyword = 4;          // 新增: 搜索标题/内容
  string tag = 5;              // 新增: 标签过滤
  string sort_by = 6;          // 新增: latest/popular/most_liked
}

// PostInfo 增强 response
message PostInfo {
  // ... existing fields ...
  bool is_liked = 20;          // 新增: 当前用户是否已点赞
  bool is_author = 21;         // 新增: 当前用户是否为作者
  int32 like_count = 22;
  int32 comment_count = 23;
  int32 view_count = 24;
}

【实现步骤】
1. ListMyPosts:
   - 获取 userID from ctx
   - 查询 posts WHERE author_id=userID, 分页, ORDER BY created_at DESC
   - 对每条帖子填充 is_liked=true, is_author=true
2. ListPosts 增强:
   - 动态构建查询条件:
     - post_type 非空 → WHERE post_type=?
     - keyword 非空 → WHERE (title LIKE '%keyword%' OR content LIKE '%keyword%')
     - tag 非空 → WHERE tags LIKE '%tag%' (或 JSON_CONTAINS if JSON field)
   - sort_by: latest→created_at DESC, popular→view_count DESC, most_liked→like_count DESC
   - 对每条帖子: 查询 post_likes 判断 is_liked, 比较 author_id 判断 is_author
3. GetPost 增强:
   - UPDATE posts SET view_count = view_count + 1 WHERE id=? (异步或直接)
   - 查询 is_liked 状态
4. CreatePost 增强验证:
   - post_type 必填且在 [discussion, article, question] 中
   - content 最大 5000 字符
   - title: article 类型最大 120 字符
   - tags 最多 5 个，每个最长 20 字符

【错误处理】
- CreatePost 验证失败 → errors.BadRequest("VALIDATION_FAILED", "具体原因")
- page_size > 50 → 强制设为 50(不报错)

【数据库操作】
- posts: SELECT with dynamic WHERE, UPDATE view_count
- post_likes: SELECT WHERE post_id=? AND user_id=? (批量: WHERE post_id IN (?) AND user_id=?)

【依赖的外部服务/库】
- pkg/auth: GetUserIDFromContext
- GORM

【验证标准】
1. ListMyPosts 只返回当前用户的帖子
2. ListPosts keyword 搜索能命中标题
3. GetPost 每次调用 view_count +1
4. is_liked 准确反映当前用户的点赞状态
5. CreatePost 空 post_type 被拒绝

【禁止事项】
- 禁止 N+1 查询(查询 is_liked 时应使用 IN 批量查询)
- 禁止 view_count 更新阻塞主查询(可直接 goroutine 异步)
- 禁止 keyword 搜索不转义 SQL 特殊字符(GORM 会自动处理)
- 禁止 page_size 大于 50 时直接报错
```

---

## Phase 3: 题目服务

---

### 任务 P3-1: Question Service - Implement RunCode (replace stub)

**目标**：将题目服务中的 RunCode 桩实现替换为真正调用 CodeRunner 服务的逻辑。

**文件路径**：
- `app/question/internal/service/question.go` (修改)
- `app/question/internal/conf/conf.go` (修改，添加 coderunner 地址)
- `app/question/cmd/server/main.go` (修改，注入 CodeRunner client)

**单体参考**：`backend/internal/executor/piston.go`

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目实现微服务。

【任务】将 Question 服务中 RunCode RPC 的 stub 实现替换为真正调用 CodeRunner 微服务的逻辑。

【目标】用户提交代码时，调用 CodeRunner.Execute gRPC，获取执行结果和测试用例通过情况。

【需要修改的文件】
- app/question/internal/service/question.go (替换 RunCode 方法实现)
- app/question/internal/conf/conf.go (添加 coderunner_addr 配置)
- app/question/cmd/server/main.go (注入 CodeRunnerServiceClient)

【Proto 接口】(已有，不需修改)
rpc RunCode(RunCodeRequest) returns (RunCodeResponse);

message RunCodeRequest {
  uint64 question_id = 1;
  string language = 2;
  string code = 3;
}

message RunCodeResponse {
  bool success = 1;
  string stdout = 2;
  string stderr = 3;
  int32 exit_code = 4;
  int64 execution_time_ms = 5;
  repeated TestResult test_results = 6;
  int32 passed_count = 7;
  int32 total_count = 8;
}

【实现步骤】
1. conf/conf.go 添加 coderunner_addr string (默认 localhost:9001)
2. service/question.go:
   - 在 QuestionService struct 中添加字段: coderunnerClient coderunner.CodeRunnerServiceClient
   - 修改 NewQuestionService 构造函数接收 coderunnerClient 参数
   - RunCode 实现:
     a. 通过 question_id 查询题目获取 test_cases (JSON 字段)
     b. 构造 CodeRunner.ExecuteRequest{language, code, test_cases, timeout_ms: 30000}
     c. 调用 coderunnerClient.Execute(ctx, req)
     d. 将结果映射到 RunCodeResponse
3. cmd/server/main.go:
   - grpc.Dial(conf.CoderunnerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
   - coderunnerClient := coderunner.NewCodeRunnerServiceClient(conn)
   - 传入 NewQuestionService

【错误处理】
- 题目不存在 → errors.NotFound("QUESTION_NOT_FOUND", "题目不存在")
- 题目无 test_cases → 只执行代码不跑测试
- CodeRunner 不可用 → errors.New(503, "CODE_RUNNER_UNAVAILABLE", "代码执行服务不可用")
- CodeRunner 返回错误 → 透传具体错误信息

【数据库操作】
- 表 questions: SELECT WHERE id=? (获取 test_cases 字段)

【依赖的外部服务/库】
- CodeRunner Service (gRPC client): app/coderunner 的 proto 生成的 client
- GORM: 查询 question

【验证标准】
1. 替换后 RunCode 不再返回 "not implemented yet"
2. 提交正确代码 → success=true, passed_count == total_count
3. 提交错误代码 → success=false, stderr 非空
4. CodeRunner 服务不可用时返回 503

【禁止事项】
- 禁止在 service 层直接 grpc.Dial，必须通过 DI 注入
- 禁止忽略 question 的 test_cases，必须传递给 CodeRunner
- 禁止 hardcode CodeRunner 地址
```

---

### 任务 P3-2: Question Service - Implement DeleteNote

**目标**：为题目服务添加 DeleteNote RPC，支持用户删除自己的笔记。

**文件路径**：
- `app/question/internal/biz/question.go` (修改)
- `app/question/internal/data/question_repo.go` (修改)
- `app/question/internal/service/question.go` (修改)

**单体参考**：无

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目实现微服务。

【任务】为 Question 服务添加 DeleteNote RPC，允许用户软删除自己的笔记。

【目标】验证笔记属于当前用户后执行软删除。

【需要修改的文件】
- app/question/internal/biz/question.go
- app/question/internal/data/question_repo.go
- app/question/internal/service/question.go

【Proto 接口】(需添加到 proto 文件)
rpc DeleteNote(DeleteNoteRequest) returns (google.protobuf.Empty);

message DeleteNoteRequest {
  uint64 note_id = 1;
}

【实现步骤】
1. biz/question.go:
   - 添加 NoteRepo 接口方法: GetByID(ctx, noteID uint) (*Note, error), Delete(ctx, noteID uint) error
   - 添加 DeleteNote UseCase:
     a. 获取 userID from ctx
     b. GetByID(noteID)
     c. 若不存在 → NOTE_NOT_FOUND
     d. 若 note.UserID != userID → FORBIDDEN
     e. Delete(noteID) 软删除
2. data/question_repo.go:
   - GetByID: SELECT FROM notes WHERE id=? AND deleted_at IS NULL
   - Delete: db.WithContext(ctx).Delete(&Note{}, noteID) (GORM 软删除)
3. service/question.go:
   - 添加 DeleteNote handler, 调用 UseCase

【错误处理】
- 笔记不存在 → errors.NotFound("NOTE_NOT_FOUND", "笔记不存在")
- 非笔记所有者 → errors.New(403, "FORBIDDEN", "无权删除此笔记")

【数据库操作】
- 表 notes: SELECT WHERE id=? AND deleted_at IS NULL
- 表 notes: UPDATE SET deleted_at=NOW() WHERE id=? (软删除)

【依赖的外部服务/库】
- pkg/auth: GetUserIDFromContext
- GORM

【验证标准】
1. 用户可以删除自己的笔记
2. 非所有者删除返回 403
3. 不存在的笔记返回 404
4. 删除后再查询该笔记返回 404

【禁止事项】
- 禁止硬删除(必须软删除)
- 禁止跳过权限验证
- 禁止从 request body 获取 user_id
```

---

### 任务 P3-3: Question Service - Implement GenerateTimedExam

**目标**：实现限时考试生成功能，随机选题并创建考试记录。

**文件路径**：
- `app/question/internal/biz/question.go` (修改)
- `app/question/internal/data/question_repo.go` (修改)
- `app/question/internal/service/question.go` (修改)

**单体参考**：无

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目实现微服务。

【任务】为 Question 服务添加 GenerateTimedExam RPC，根据条件随机选题并创建限时考试记录。

【目标】根据分类、难度和题目数量随机选取题目，创建考试记录并返回题目列表和时间限制。

【需要修改的文件】
- app/question/internal/biz/question.go
- app/question/internal/data/question_repo.go
- app/question/internal/service/question.go

【Proto 接口】
rpc GenerateTimedExam(TimedExamRequest) returns (ExamResponse);

message TimedExamRequest {
  uint64 category_id = 1;
  string difficulty = 2;         // easy/medium/hard/mixed
  int32 question_count = 3;      // 默认 20
  int32 time_limit_minutes = 4;  // 默认 60
}

message ExamResponse {
  uint64 exam_id = 1;
  repeated ExamQuestion questions = 2;
  int32 time_limit_minutes = 3;
  google.protobuf.Timestamp started_at = 4;
  google.protobuf.Timestamp expires_at = 5;
}

message ExamQuestion {
  uint64 question_id = 1;
  string title = 2;
  string content = 3;
  string question_type = 4;
  string difficulty = 5;
  repeated string options = 6;
}

【实现步骤】
1. biz/question.go 添加:
   - Exam 实体:
     type Exam struct {
       BaseModel
       UserID           uint
       Type             string          // "timed"
       CategoryID       uint
       Difficulty       string
       QuestionIDsJSON  string          // JSON array of uint64
       TimeLimitMinutes int32
       StartedAt        time.Time
       FinishedAt       *time.Time
       Score            *float64
       Status           string          // "in_progress" / "submitted" / "expired"
     }
     TableName() = "exams"
   - ExamRepo 接口: Create(ctx, exam *Exam) error, GetByID(ctx, id uint) (*Exam, error)
   - QuestionRepo 接口添加: RandomSelect(ctx, categoryID uint, difficulty string, count int) ([]Question, error)
   - GenerateTimedExam UseCase:
     a. 获取 userID from ctx
     b. 校验 question_count(1-50)、time_limit_minutes(10-180)
     c. 调用 QuestionRepo.RandomSelect 获取题目
     d. 若题目不足 → 返回能获取到的(不报错)
     e. 创建 Exam 记录 (status="in_progress", started_at=now, question_ids_json)
     f. 返回 exam + questions
2. data/question_repo.go 添加:
   - RandomSelect: SELECT * FROM questions WHERE category_id=? AND difficulty=? AND deleted_at IS NULL ORDER BY RANDOM() LIMIT ?
     (若 difficulty="mixed" 则不加 difficulty 条件)
   - ExamRepo GORM 实现
3. service 添加 handler

【错误处理】
- category 不存在 → errors.NotFound("CATEGORY_NOT_FOUND", "分类不存在")
- 该条件下无题目 → errors.NotFound("NO_QUESTIONS", "该条件下没有可用题目")
- question_count > 50 → errors.BadRequest("INVALID_COUNT", "题目数量不能超过50")

【数据库操作】
- 表 questions: SELECT WHERE category_id=? AND difficulty=? ORDER BY RANDOM() LIMIT ?
- 表 exams: INSERT (新建考试记录)

【依赖的外部服务/库】
- pkg/auth: GetUserIDFromContext
- GORM
- encoding/json: 序列化 question_ids

【验证标准】
1. 返回指定数量的随机题目
2. 多次调用返回不同的题目顺序
3. exam 记录正确创建
4. expires_at = started_at + time_limit_minutes
5. difficulty="mixed" 时返回混合难度

【禁止事项】
- 禁止返回重复题目
- 禁止 time_limit 超过 180 分钟
- 禁止不创建 exam 记录就返回题目
- 禁止在内存中 shuffle(应在 SQL 中 ORDER BY RANDOM())
```

---

### 任务 P3-4: Question Service - Implement SubmitExam

**目标**：实现考试提交功能，自动批改并计算总分。

**文件路径**：
- `app/question/internal/biz/question.go` (修改)
- `app/question/internal/data/question_repo.go` (修改)
- `app/question/internal/service/question.go` (修改)

**单体参考**：无

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目实现微服务。

【任务】为 Question 服务添加 SubmitExam RPC，提交考试答案，调用 AI 批改并计算总分。

【目标】验证考试有效性(属于用户、未过期、未提交过)，对每道题调用 AI Gateway.QuizAnalyzer 批改，计算总分并更新考试记录。

【需要修改的文件】
- app/question/internal/biz/question.go
- app/question/internal/data/question_repo.go
- app/question/internal/service/question.go
- app/question/internal/conf/conf.go (添加 ai_gateway_addr)

【Proto 接口】
rpc SubmitExam(SubmitExamRequest) returns (ExamResult);

message SubmitExamRequest {
  uint64 exam_id = 1;
  repeated ExamAnswer answers = 2;
}

message ExamAnswer {
  uint64 question_id = 1;
  string answer = 2;
}

message ExamResult {
  uint64 exam_id = 1;
  float total_score = 2;
  int32 correct_count = 3;
  int32 total_count = 4;
  repeated QuestionResult question_results = 5;
}

message QuestionResult {
  uint64 question_id = 1;
  bool is_correct = 2;
  float score = 3;
  string feedback = 4;
  repeated string mistake_tags = 5;
}

【实现步骤】
1. biz/question.go:
   - ExamRepo 添加: Update(ctx, exam *Exam) error
   - SubmitExam UseCase:
     a. 获取 userID from ctx
     b. GetExamByID → 验证 exam.UserID == userID
     c. 验证 exam.Status == "in_progress"
     d. 验证 time.Now().Before(exam.StartedAt.Add(exam.TimeLimitMinutes * time.Minute)) (未过期)
     e. 对每个 answer:
        - 查询对应 question 获取 correct_answer, question_type
        - 调用 AI Gateway QuizAnalyzer RPC: {question, user_answer, correct_answer, question_type}
        - 收集 QuestionResult
     f. 计算 total_score = sum(scores) / len(answers) * 100
     g. 更新 exam: status="submitted", score=total_score, finished_at=now
     h. 保存每条 user_answers 记录
2. data 层实现
3. service 层 handler

【错误处理】
- 考试不存在 → errors.NotFound("EXAM_NOT_FOUND", "考试不存在")
- 考试已过期 → errors.New(400, "EXAM_EXPIRED", "考试已超时")
- 考试已提交 → errors.New(409, "EXAM_ALREADY_SUBMITTED", "考试已提交过")
- 非本人考试 → errors.New(403, "FORBIDDEN", "无权操作此考试")
- AI Gateway 调用失败 → 该题 score=0, feedback="评判服务暂时不可用"(不中断整体)

【数据库操作】
- 表 exams: SELECT WHERE id=?, UPDATE SET status/score/finished_at
- 表 questions: SELECT WHERE id IN (?) (批量获取正确答案)
- 表 user_answers: INSERT (记录用户答案和评判结果)

【依赖的外部服务/库】
- AI Gateway Service (gRPC): QuizAnalyzer RPC
- GORM
- pkg/auth

【验证标准】
1. 提交后 exam.status 变为 "submitted"
2. total_score 计算正确
3. 已过期的考试不能提交
4. 已提交的考试不能重复提交
5. AI Gateway 部分失败不影响整体提交

【禁止事项】
- 禁止不验证考试所有权
- 禁止不检查过期时间
- 禁止 AI 单题失败导致整体失败
- 禁止不记录 user_answers
```

---

### 任务 P3-5: Question Service - Implement ListQuestionSets + GetQuestionSetDetail

**目标**：实现题目集的列表查询和详情查询功能。

**文件路径**：
- `app/question/internal/biz/question.go` (修改)
- `app/question/internal/data/question_repo.go` (修改)
- `app/question/internal/service/question.go` (修改)

**单体参考**：无

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目实现微服务。

【任务】为 Question 服务实现题目集(QuestionSet)的列表和详情查询。

【目标】支持分页查询已发布的题目集列表，以及获取题目集详情（包含展开的题目列表）。

【需要修改的文件】
- app/question/internal/biz/question.go
- app/question/internal/data/question_repo.go
- app/question/internal/service/question.go

【Proto 接口】
rpc ListQuestionSets(ListQuestionSetsRequest) returns (ListQuestionSetsResponse);
rpc GetQuestionSetDetail(GetQuestionSetDetailRequest) returns (QuestionSetDetail);

message ListQuestionSetsRequest {
  int32 page = 1;
  int32 page_size = 2;
  uint64 category_id = 3;     // 可选过滤
}

message ListQuestionSetsResponse {
  repeated QuestionSetInfo items = 1;
  int32 total = 2;
}

message QuestionSetInfo {
  uint64 id = 1;
  string title = 2;
  string slug = 3;
  string description = 4;
  string cover_image = 5;
  int32 question_count = 6;
  string difficulty = 7;
  uint64 category_id = 8;
}

message GetQuestionSetDetailRequest {
  uint64 set_id = 1;
}

message QuestionSetDetail {
  QuestionSetInfo info = 1;
  repeated QuestionBrief questions = 2;
}

message QuestionBrief {
  uint64 id = 1;
  string title = 2;
  string difficulty = 3;
  string question_type = 4;
}

【实现步骤】
1. biz/question.go 添加:
   - QuestionSet 实体:
     type QuestionSet struct {
       BaseModel
       Title           string
       Slug            string  `gorm:"uniqueIndex"`
       Description     string
       CoverImage      string
       QuestionIDsJSON string  // JSON array of uint64
       QuestionCount   int32
       Difficulty      string
       CategoryID      uint
       SortOrder       int32
       IsPublished     bool
     }
     TableName() = "question_sets"
   - QuestionSetRepo 接口:
     - List(ctx, categoryID uint, page, pageSize int) ([]QuestionSet, int64, error)
     - GetByID(ctx, id uint) (*QuestionSet, error)
2. data/question_repo.go 实现:
   - List: SELECT FROM question_sets WHERE is_published=true [AND category_id=?] ORDER BY sort_order ASC, LIMIT/OFFSET
   - GetByID: SELECT WHERE id=? AND deleted_at IS NULL
3. service handler:
   - ListQuestionSets: 调用 repo.List → 转 proto
   - GetQuestionSetDetail:
     a. 获取 QuestionSet
     b. 解析 question_ids_json
     c. 批量查询 questions (SELECT WHERE id IN (?))
     d. 组装 QuestionSetDetail

【错误处理】
- set 不存在 → errors.NotFound("SET_NOT_FOUND", "题目集不存在")
- category 过滤无结果 → 返回空列表(不报错)

【数据库操作】
- 表 question_sets: SELECT (分页 + 过滤)
- 表 questions: SELECT WHERE id IN (?) (批量获取题目简要信息)

【依赖的外部服务/库】
- GORM
- encoding/json: 解析 question_ids_json

【验证标准】
1. 列表只返回 is_published=true 的集合
2. 按 sort_order 正序排列
3. 详情包含完整的题目列表
4. category_id 过滤有效

【禁止事项】
- 禁止返回未发布的题目集
- 禁止 GetDetail 中 N+1 查询(必须批量查询 questions)
- 禁止忽略 sort_order 排序
```

---

### 任务 P3-6: Question Service - Implement ListMistakeTopics

**目标**：实现错题主题聚合，按分类统计用户的错题情况。

**文件路径**：
- `app/question/internal/biz/question.go` (修改)
- `app/question/internal/data/question_repo.go` (修改)
- `app/question/internal/service/question.go` (修改)

**单体参考**：无

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目实现微服务。

【任务】为 Question 服务实现 ListMistakeTopics RPC，聚合用户的错题信息按分类统计。

【目标】查询用户的作答记录，按分类聚合错题数、总题数和错误率，并返回每个分类下最近的错题列表。

【需要修改的文件】
- app/question/internal/biz/question.go
- app/question/internal/data/question_repo.go
- app/question/internal/service/question.go

【Proto 接口】
rpc ListMistakeTopics(ListMistakeTopicsRequest) returns (ListMistakeTopicsResponse);

message ListMistakeTopicsRequest {
  int32 limit = 1;             // 返回 top N 个分类，默认 10
}

message ListMistakeTopicsResponse {
  repeated MistakeTopic topics = 1;
}

message MistakeTopic {
  string topic = 1;            // 分类名称
  int32 error_count = 2;
  int32 total_attempts = 3;
  float error_rate = 4;        // error_count / total_attempts
  repeated MistakeQuestion recent_wrong_questions = 5; // 最近 5 题
}

message MistakeQuestion {
  uint64 question_id = 1;
  string title = 2;
  string difficulty = 3;
  google.protobuf.Timestamp last_wrong_at = 4;
}

【实现步骤】
1. biz/question.go:
   - ListMistakeTopics UseCase:
     a. 获取 userID from ctx
     b. 调用 repo 聚合查询
     c. 对每个分类查询最近 5 道错题
     d. 计算 error_rate
2. data/question_repo.go:
   - 聚合查询 SQL:
     SELECT c.name as topic,
            COUNT(CASE WHEN ua.is_correct = false THEN 1 END) as error_count,
            COUNT(*) as total_attempts
     FROM user_answers ua
     JOIN questions q ON ua.question_id = q.id
     JOIN categories c ON q.category_id = c.id
     WHERE ua.user_id = ? AND ua.deleted_at IS NULL
     GROUP BY c.id, c.name
     HAVING error_count > 0
     ORDER BY error_count DESC
     LIMIT ?
   - 最近错题查询:
     SELECT q.id, q.title, q.difficulty, ua.created_at as last_wrong_at
     FROM user_answers ua JOIN questions q ON ua.question_id = q.id
     WHERE ua.user_id=? AND ua.is_correct=false AND q.category_id=?
     ORDER BY ua.created_at DESC LIMIT 5

【错误处理】
- 无错题记录 → 返回空列表(不报错)
- limit > 50 → 强制设为 50

【数据库操作】
- 表 user_answers: 聚合查询 (GROUP BY category)
- 表 questions: JOIN 获取题目信息
- 表 categories: JOIN 获取分类名
- 无新增表

【依赖的外部服务/库】
- GORM (Raw SQL for aggregation)
- pkg/auth: GetUserIDFromContext

【验证标准】
1. 返回按 error_count 降序的分类列表
2. error_rate = error_count / total_attempts (精度2位)
3. 每个 topic 最多返回 5 道最近错题
4. 无错题时返回空列表

【禁止事项】
- 禁止在内存中做聚合(必须 SQL GROUP BY)
- 禁止 N+1 查询最近错题(可以用一次批量查询再分组)
- 禁止返回已删除的记录
```

---

### 任务 P3-7: Question Service - MQ Consumers (pipeline.build + scraper.import)

**目标**：实现两个 MQ 消费者：题目生成流水线和爬虫导入。

**文件路径**：
- `app/question/internal/server/mq.go` (新建)
- `app/question/internal/biz/question.go` (修改)

**单体参考**：无

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目实现微服务。

【任务】为 Question 服务实现两个 MQ 消费者:
1. pipeline.build: 接收生成流水线配置，调用 AI 生成题目
2. scraper.import: 接收爬虫抓取的结构化题目，校验后批量插入

【目标】支持通过 MQ 异步触发题目的 AI 生成和批量导入。

【需要创建/修改的文件】
- app/question/internal/server/mq.go (新建)
- app/question/internal/biz/question.go (修改，添加 UseCase 方法)

【MQ 协议】
Consumer 1:
- Queue: makejob.tasks.question.pipeline.build
- Routing Key: question.pipeline.build
- Payload:
  {
    "category_id": 1,
    "difficulty": "medium",
    "count": 10,
    "question_type": "choice",
    "topic": "Go 并发编程",
    "industry": "backend"
  }

Consumer 2:
- Queue: makejob.tasks.scraper.import.questions
- Routing Key: scraper.import.questions
- Payload:
  {
    "questions": [
      {
        "title": "...",
        "content": "...",
        "answer": "...",
        "question_type": "choice",
        "difficulty": "medium",
        "category_id": 1,
        "options": ["A...", "B...", "C...", "D..."],
        "tags": ["go", "concurrency"]
      }
    ],
    "source": "leetcode_scraper"
  }

【实现步骤】
1. Consumer 1 (pipeline.build):
   a. 解析 payload 获取生成配置
   b. 调用 AI Gateway.InterviewAgent (mode="generate_questions"): 传入 topic + difficulty + count + question_type
   c. 解析 AI 返回的题目列表 JSON
   d. 逐条插入 questions 表
   e. 对每条新题目 publish "rag.sync.question" 消息 (action="create") 触发 RAG 索引
2. Consumer 2 (scraper.import):
   a. 解析 payload 获取 questions 数组
   b. 校验每条: title 非空, content 非空, category_id > 0
   c. 过滤掉不合法的条目
   d. 批量 INSERT 到 questions 表 (db.CreateInBatches, batch size=100)
   e. 对每条新题目 publish "rag.sync.question" (action="create")
3. server/mq.go:
   - PipelineBuildHandler 实现 TaskHandler
   - ScraperImportHandler 实现 TaskHandler
   - 注册两个 consumer

【错误处理】
- payload 解析失败 → return error (nack + 重试)
- AI Gateway 调用失败 → return error (重试)
- 单条题目插入失败 → log.Error + 跳过继续(不中断批量)
- MQ publish 失败 → log.Error (不阻塞主流程)

【数据库操作】
- 表 questions: INSERT (单条和批量)
- 字段: title, content, answer, question_type, difficulty, category_id, options_json, tags_json

【依赖的外部服务/库】
- AI Gateway Service (gRPC): InterviewAgent RPC
- pkg/mq: Publisher.Publish (发布 RAG 同步事件)
- GORM: 批量插入

【验证标准】
1. pipeline.build 消息触发后 questions 表新增对应数量题目
2. scraper.import 批量导入 50 条题目成功
3. 每条新题目都触发了 RAG 同步消息
4. 非法 payload 被正确 nack

【禁止事项】
- 禁止同步阻塞等待所有题目插入完成才 ack(应逐步处理)
- 禁止 pipeline.build 不调用 AI 直接造假数据
- 禁止 scraper.import 不校验就插入
- 禁止忽略 RAG 同步(每条新题目必须触发)
```

---

### 任务 P3-8: Question Service - Enhance GetPracticeRecommendations

**目标**：增强练习推荐算法，支持根据面试结果加权推荐。

**文件路径**：
- `app/question/internal/biz/question.go` (修改)
- `app/question/internal/service/question.go` (修改)

**单体参考**：`backend/internal/service/question_service.go` (GetPracticeRecommendations)

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目实现微服务。

【任务】增强 Question 服务的 GetPracticeRecommendations RPC，添加 interview_id 参数支持面试驱动的加权推荐。

【目标】当传入 interview_id 时，获取该面试的薄弱点信息，在推荐算法中对这些薄弱领域的题目提高权重。

【需要修改的文件】
- app/question/internal/biz/question.go
- app/question/internal/service/question.go

【Proto 接口】(修改现有)
message GetPracticeRecommendationsRequest {
  uint64 user_id = 1;         // 内部 RPC 可接受
  uint64 interview_id = 2;    // 新增: 可选，关联面试
  int32 limit = 3;            // 新增: 推荐数量，默认 10
}

【实现步骤】
1. service 层:
   - 若 interview_id > 0:
     a. 调用 Interview Service gRPC 获取面试报告 (GetReport)
     b. 从报告提取 weaknesses/weak_topics
     c. 传入 UseCase 作为加权因子
2. biz 层 GetPracticeRecommendations 增强:
   - 原逻辑: 根据错题率和最近练习情况推荐
   - 新增逻辑:
     a. 若有 weak_topics → 对这些分类的题目权重 x2
     b. 推荐策略: 70% 来自薄弱分类 + 30% 来自其他分类
     c. 优先推荐用户未做过的题目
     d. 排除最近 3 天做过的题目
   - limit 参数控制返回数量

【错误处理】
- Interview Service 不可用 → log.Warn + 退化为无面试加权的普通推荐(不报错)
- 无可推荐题目 → 返回空列表

【数据库操作】
- 表 questions: SELECT WHERE category_id IN (weak_categories) AND id NOT IN (recent_done) LIMIT ?
- 表 user_answers: SELECT question_id WHERE user_id=? AND created_at > 3天前

【依赖的外部服务/库】
- Interview Service (gRPC, 可选): GetReport
- GORM

【验证标准】
1. 无 interview_id 时正常推荐
2. 有 interview_id 时推荐偏向面试薄弱点
3. Interview Service 不可用时降级正常工作
4. 不推荐最近 3 天做过的题目

【禁止事项】
- 禁止 Interview Service 失败导致整个推荐失败
- 禁止推荐已删除的题目
- 禁止忽略 limit 参数
```

---

## Phase 4: 面试 + 实时服务

---

### 任务 P4-1: Interview Service - Implement GetNextQuestion

**目标**：实现面试获取下一题逻辑，结合 RAG 检索和 AI 生成。

**文件路径**：
- `app/interview/internal/biz/interview.go` (修改)
- `app/interview/internal/service/interview.go` (修改)
- `app/interview/internal/conf/conf.go` (修改，添加依赖服务地址)

**单体参考**：`backend/internal/service/interview_service.go`

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目实现微服务。

【任务】实现 Interview 服务的 GetNextQuestion RPC，根据面试上下文动态生成下一道面试题。

【目标】加载面试上下文(历史消息、简历、JD) → 调用 RAG 获取相关知识 → 调用 AI Gateway 生成下一题 → 保存消息记录 → 返回。

【需要修改的文件】
- app/interview/internal/biz/interview.go
- app/interview/internal/service/interview.go
- app/interview/internal/conf/conf.go

【Proto 接口】
rpc GetNextQuestion(GetNextQuestionRequest) returns (InterviewQuestion);

message GetNextQuestionRequest {
  uint64 interview_id = 1;
  string user_answer = 2;     // 上一题的回答(首题为空)
}

message InterviewQuestion {
  string content = 1;
  string question_type = 2;   // technical/behavioral/coding
  string difficulty = 3;
  repeated TestCaseProto test_cases = 4;
  Live2DDirectiveProto live2d_directive = 5;
  int32 question_index = 6;
}

【实现步骤】
1. biz/interview.go - GetNextQuestion UseCase:
   a. 获取 interview 记录, 验证 status == "ongoing"
   b. 如果 user_answer 非空:
      - 创建 interview_message (role="user", content=user_answer)
   c. 加载面试上下文:
      - 获取所有 interview_messages (最近 20 条)
      - 获取 interview.resume_text, interview.jd_text
   d. 调用 RAG.Retrieve(query=上一个问题主题, top_k=3) 获取相关知识点
   e. 构造 AI Gateway.InterviewAgent 请求:
      - history: 消息列表转为 ChatMessage[]
      - resume: interview.resume_text
      - jd: interview.jd_text
      - industry: interview.industry
      - question_count: interview.question_count
      - mode: "question"
      - weak_topics: interview.weak_topics (JSON字段)
   f. 调用 AI Gateway.InterviewAgent
   g. 解析返回，创建 interview_message (role="ai", content=question)
   h. 更新 interview.current_question_index++
   i. 返回 InterviewQuestion
2. service 层 handler: 调用 biz + proto 转换

【错误处理】
- 面试不存在 → errors.NotFound("INTERVIEW_NOT_FOUND", "面试不存在")
- 面试已结束 → errors.New(400, "INTERVIEW_FINISHED", "面试已结束")
- RAG 失败 → log.Warn + 跳过(不影响主流程)
- AI Gateway 失败 → errors.New(502, "AI_GENERATION_FAILED", "题目生成失败")

【数据库操作】
- 表 interviews: SELECT WHERE id=? AND deleted_at IS NULL
- 表 interview_messages: SELECT WHERE interview_id=? ORDER BY created_at, INSERT
- 表 interviews: UPDATE current_question_index

【依赖的外部服务/库】
- RAG Service (gRPC): Retrieve
- AI Gateway Service (gRPC): InterviewAgent
- GORM

【验证标准】
1. 首次调用(user_answer为空)返回第一题
2. 带 user_answer 调用保存答案并返回下一题
3. 面试已结束时返回 INTERVIEW_FINISHED
4. RAG 不可用时仍能生成题目(降级)

【禁止事项】
- 禁止不检查面试状态直接生成题目
- 禁止不保存消息记录
- 禁止 RAG 失败导致整体失败
- 禁止一次性加载所有历史消息(限制最近 20 条)
```

---

### 任务 P4-2: Interview Service - Implement FinishInterview

**目标**：实现面试结束逻辑，触发异步报告生成。

**文件路径**：
- `app/interview/internal/biz/interview.go` (修改)
- `app/interview/internal/service/interview.go` (修改)

**单体参考**：`backend/internal/service/interview_service.go`

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目实现微服务。

【任务】实现 Interview 服务的 FinishInterview RPC，结束面试并触发异步报告生成。

【目标】将面试状态改为 report_generating，发布 MQ 消息触发报告生成流程，返回占位报告信息。

【需要修改的文件】
- app/interview/internal/biz/interview.go
- app/interview/internal/service/interview.go

【Proto 接口】
rpc FinishInterview(FinishInterviewRequest) returns (FinishInterviewResponse);

message FinishInterviewRequest {
  uint64 interview_id = 1;
}

message FinishInterviewResponse {
  string status = 1;           // "report_generating"
  string message = 2;          // "面试报告正在生成中，请稍候查看"
}

【实现步骤】
1. biz/interview.go - FinishInterview UseCase:
   a. 获取 interview, 验证属于当前用户
   b. 验证 status == "ongoing"
   c. 更新 status = "report_generating"
   d. 更新 finished_at = time.Now()
   e. 发布 MQ 消息:
      - Routing Key: "interview.report.generate"
      - Payload: {interview_id: interview.ID}
   f. 返回 status + message
2. service 层 handler

【错误处理】
- 面试不存在 → errors.NotFound("INTERVIEW_NOT_FOUND", "面试不存在")
- 面试已结束(status != "ongoing") → errors.New(400, "INTERVIEW_FINISHED", "面试已经结束")
- MQ 发布失败 → log.Error + 仍然返回成功(后续可补偿)

【数据库操作】
- 表 interviews: SELECT WHERE id=?, UPDATE status + finished_at

【依赖的外部服务/库】
- pkg/mq: Publisher.Publish

【验证标准】
1. 面试状态变为 report_generating
2. finished_at 被设置
3. MQ 消息被成功发布
4. 已结束的面试不能重复结束

【禁止事项】
- 禁止同步生成报告(必须异步 MQ)
- 禁止 MQ 发布失败回滚状态变更
- 禁止不设置 finished_at
```

---

### 任务 P4-3: Interview Service - Report Generation MQ Consumer

**目标**：实现面试报告异步生成消费者。

**文件路径**：
- `app/interview/internal/server/mq.go` (新建)
- `app/interview/internal/biz/interview.go` (修改)

**单体参考**：`backend/internal/service/interview_service.go`

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目实现微服务。

【任务】为 Interview 服务实现 MQ 消费者，异步生成面试报告。

【目标】消费 report.generate 消息后：加载所有面试消息 → 调用 AI 生成报告 → 若有编程题则调用代码分析 → 保存报告 → 更新面试状态 → 发布完成事件。

【需要创建/修改的文件】
- app/interview/internal/server/mq.go (新建)
- app/interview/internal/biz/interview.go (修改)

【MQ 协议】
- Queue: makejob.tasks.interview.report.generate
- Routing Key: interview.report.generate
- Payload: {"interview_id": 123}

产出事件:
- Routing Key: interview.finished
- Payload: {"interview_id": 123, "user_id": 456, "score": 78.5, "weak_topics": ["goroutine", "channel"], "strength_topics": ["http"]}

【实现步骤】
1. biz/interview.go - GenerateReport UseCase:
   a. 加载 interview + 所有 interview_messages
   b. 构造 AI Gateway.InterviewAgent 请求:
      - history: 所有消息
      - mode: "report"
      - resume: interview.resume_text
      - industry: interview.industry
   c. 调用 AI Gateway.InterviewAgent → 获取报告 JSON
   d. 若有 interview_coding_attempts:
      - 对每个 coding attempt 调用 AI Gateway.QuizAnalyzer
      - 合并代码评审结果到报告
   e. 解析报告结构: {overall_score, dimension_scores, strengths, weaknesses, suggestions, summary}
   f. 创建 interview_reports 记录
   g. 更新 interview.status = "completed"
   h. 发布 "interview.finished" 事件 (包含 interview_id, user_id, score, weak/strength topics)
2. server/mq.go: 实现 TaskHandler, 注册 consumer

【错误处理】
- interview 不存在 → log.Error + return nil (丢弃消息)
- AI Gateway 调用失败 → return error (触发重试)
- 重试 3 次仍失败 → 进入死信 + 更新 interview.status = "report_failed"

【数据库操作】
- 表 interviews: SELECT + UPDATE status
- 表 interview_messages: SELECT WHERE interview_id=? ORDER BY created_at
- 表 interview_coding_attempts: SELECT WHERE interview_id=?
- 表 interview_reports: INSERT

interview_reports 表结构:
  - id uint PK
  - interview_id uint NOT NULL (unique)
  - overall_score float
  - dimension_scores_json text
  - strengths_json text
  - weaknesses_json text
  - suggestions_json text
  - summary text
  - coding_diagnostics_json text
  - BaseModel

【依赖的外部服务/库】
- AI Gateway (gRPC): InterviewAgent(mode=report), QuizAnalyzer
- pkg/mq: Consumer + Publisher

【验证标准】
1. 消费消息后报告被正确生成并保存
2. interview.status 变为 "completed"
3. interview.finished 事件被发布
4. 编程题诊断信息包含在报告中

【禁止事项】
- 禁止同步处理(此 handler 应在 MQ consumer goroutine 中运行)
- 禁止不发布 interview.finished 事件(下游依赖)
- 禁止报告生成失败但不更新状态
```

---

### 任务 P4-4: Interview Service - Implement GetReport

**目标**：实现面试报告查询功能。

**文件路径**：
- `app/interview/internal/biz/interview.go` (修改)
- `app/interview/internal/service/interview.go` (修改)

**单体参考**：无

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目实现微服务。

【任务】实现 Interview 服务的 GetReport RPC，查询面试报告。

【目标】根据 interview_id 查询报告，若正在生成则返回状态信息，若已完成则返回完整报告。

【需要修改的文件】
- app/interview/internal/biz/interview.go
- app/interview/internal/service/interview.go

【Proto 接口】
rpc GetReport(GetReportRequest) returns (InterviewReportResponse);

message GetReportRequest {
  uint64 interview_id = 1;
}

message InterviewReportResponse {
  string status = 1;           // "generating" | "completed" | "failed"
  float overall_score = 2;
  map<string, float> dimension_scores = 3;
  repeated string strengths = 4;
  repeated string weaknesses = 5;
  repeated string suggestions = 6;
  string summary = 7;
  repeated CodingDiagnostic coding_diagnostics = 8;
}

message CodingDiagnostic {
  int32 question_index = 1;
  float score = 2;
  repeated string mistake_tags = 3;
  repeated string strength_tags = 4;
  string process_summary = 5;
}

【实现步骤】
1. biz/interview.go - GetReport UseCase:
   a. 获取 interview，验证属于当前用户
   b. 检查 interview.status:
      - "report_generating" → 返回 {status: "generating"} 其他字段为空
      - "report_failed" → 返回 {status: "failed"}
      - "completed" → 查询 interview_reports 表
   c. 加载 interview_reports WHERE interview_id=?
   d. 解析 JSON 字段并填充响应
2. service 层: 转换为 proto 响应

【错误处理】
- 面试不存在 → errors.NotFound("INTERVIEW_NOT_FOUND", "面试不存在")
- 面试未结束(status=ongoing) → errors.New(400, "INTERVIEW_NOT_FINISHED", "面试尚未结束")
- 报告记录不存在(status=completed 但记录丢失) → errors.New(500, "REPORT_MISSING", "报告数据异常")

【数据库操作】
- 表 interviews: SELECT WHERE id=?
- 表 interview_reports: SELECT WHERE interview_id=?

【依赖的外部服务/库】
- GORM
- pkg/auth: GetUserIDFromContext

【验证标准】
1. report_generating 状态返回 status="generating"
2. completed 状态返回完整报告数据
3. 非本人面试返回 403
4. 未结束的面试返回 400

【禁止事项】
- 禁止不验证用户所有权
- 禁止 status=generating 时报错(应返回状态)
- 禁止不解析 JSON 字段直接返回原始字符串
```

---

### 任务 P4-5: Interview Service - Implement SubmitCodingAnswer

**目标**：实现面试中编程题的代码提交、执行和 AI 评审。

**文件路径**：
- `app/interview/internal/biz/interview.go` (修改)
- `app/interview/internal/service/interview.go` (修改)

**单体参考**：`backend/internal/service/interview_service.go`

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目实现微服务。

【任务】实现 Interview 服务的 SubmitCodingAnswer RPC，执行代码并进行 AI 代码评审。

【目标】接收用户的编程答案 → 调用 CodeRunner 执行 → 调用 AI Gateway 进行代码评审 → 保存尝试记录 → 返回综合结果。

【需要修改的文件】
- app/interview/internal/biz/interview.go
- app/interview/internal/service/interview.go

【Proto 接口】
rpc SubmitCodingAnswer(SubmitCodingAnswerRequest) returns (CodingAnswerResult);

message SubmitCodingAnswerRequest {
  uint64 interview_id = 1;
  int32 question_index = 2;
  string language = 3;
  string code = 4;
  repeated ProcessEvent process_events = 5;  // 编码过程事件
}

message ProcessEvent {
  string type = 1;
  int64 timestamp_ms = 2;
  string payload_json = 3;
}

message CodingAnswerResult {
  bool execution_success = 1;
  string stdout = 2;
  string stderr = 3;
  int32 passed_count = 4;
  int32 total_count = 5;
  float ai_score = 6;
  string ai_feedback = 7;
  repeated string mistake_tags = 8;
  repeated string suggestions = 9;
}

【实现步骤】
1. biz/interview.go - SubmitCodingAnswer UseCase:
   a. 获取 interview，验证 status="ongoing"
   b. 获取当前题目信息(从 interview_messages 中找到对应 question_index 的 AI 消息)
   c. 调用 CodeRunner.Execute:
      - language, code, test_cases (从题目元数据中获取)
   d. 调用 AI Gateway.QuizAnalyzer:
      - question: 题目内容
      - user_answer: code
      - question_type: "coding"
      - code_language: language
   e. 保存 interview_coding_attempts 记录:
      type InterviewCodingAttempt struct {
        BaseModel
        InterviewID    uint
        QuestionIndex  int32
        Language       string
        Code           string
        Stdout         string
        Stderr         string
        PassedCount    int32
        TotalCount     int32
        AIScore        float64
        AIFeedback     string
        ProcessEvents  string  // JSON
      }
   f. 返回综合结果
2. service handler

【错误处理】
- 面试不存在/已结束 → 标准错误
- CodeRunner 不可用 → 返回 execution_success=false, stderr="代码执行服务不可用"(不中断)
- AI Gateway 不可用 → ai_score=0, ai_feedback="AI评审服务暂不可用"(不中断)
- code 为空 → errors.BadRequest("INVALID_CODE", "代码不能为空")

【数据库操作】
- 表 interviews: SELECT
- 表 interview_messages: SELECT WHERE interview_id=? AND role='ai'
- 表 interview_coding_attempts: INSERT

【依赖的外部服务/库】
- CodeRunner Service (gRPC): Execute
- AI Gateway Service (gRPC): QuizAnalyzer
- GORM

【验证标准】
1. 正确代码返回 execution_success=true + ai_score > 0
2. 编译错误代码返回 stderr 非空
3. process_events 被正确保存
4. CodeRunner 不可用时仍返回 AI 评审结果

【禁止事项】
- 禁止任一下游服务失败导致整体失败
- 禁止不保存 attempt 记录
- 禁止不保存 process_events
```

---

### 任务 P4-6: Interview Service - Implement Realtime RPCs (5)

**目标**：实现面试服务中与实时面试相关的 5 个辅助 RPC。

**文件路径**：
- `app/interview/internal/biz/interview.go` (修改)
- `app/interview/internal/service/interview.go` (修改)

**单体参考**：`backend/internal/service/interview_service.go`

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目实现微服务。

【任务】为 Interview 服务实现 5 个实时面试相关 RPC，供 Realtime 服务调用。

【目标】提供实时面试场景所需的数据读写接口：判断是否实时面试、获取上下文、绑定会话、追加消息。

【需要修改的文件】
- app/interview/internal/biz/interview.go
- app/interview/internal/service/interview.go

【Proto 接口】
// 1. 判断是否为实时面试
rpc IsRealtimeInterview(IsRealtimeRequest) returns (IsRealtimeResponse);
message IsRealtimeRequest { uint64 interview_id = 1; }
message IsRealtimeResponse { bool is_realtime = 1; }

// 2. 获取实时面试上下文(供 Realtime 服务初始化)
rpc GetRealtimeContext(GetRealtimeContextRequest) returns (RealtimeContext);
message GetRealtimeContextRequest { uint64 interview_id = 1; }
message RealtimeContext {
  string resume = 1;
  string jd = 2;
  string industry = 3;
  int32 question_count = 4;
  string difficulty = 5;
  repeated ChatMessage recent_messages = 6;  // 最近 10 条
  int32 current_question_index = 7;
}

// 3. 绑定实时会话 ID
rpc BindRealtimeDialog(BindRealtimeDialogRequest) returns (google.protobuf.Empty);
message BindRealtimeDialogRequest {
  uint64 interview_id = 1;
  string dialog_id = 2;       // Volcengine realtime dialog ID
}

// 4. 追加用户回答
rpc AppendRealtimeUserAnswer(AppendUserAnswerRequest) returns (google.protobuf.Empty);
message AppendUserAnswerRequest {
  uint64 interview_id = 1;
  string content = 2;         // ASR 识别的用户回答
}

// 5. 追加 AI 回复并获取下一题元数据
rpc AppendRealtimeAssistantReply(AppendAssistantReplyRequest) returns (NextQuestionMeta);
message AppendAssistantReplyRequest {
  uint64 interview_id = 1;
  string content = 2;         // AI 的回复文本
}
message NextQuestionMeta {
  int32 question_index = 1;
  bool is_last_question = 2;
}

【实现步骤】
1. IsRealtimeInterview:
   - 查询 interview.mode 字段
   - 若 mode 包含 "realtime" → true，否则 false
2. GetRealtimeContext:
   - 加载 interview + 最近 10 条 messages
   - 组装 RealtimeContext
3. BindRealtimeDialog:
   - UPDATE interviews SET realtime_dialog_id=? WHERE id=?
4. AppendRealtimeUserAnswer:
   - INSERT interview_messages (role="user", content=content)
5. AppendRealtimeAssistantReply:
   - INSERT interview_messages (role="ai", content=content)
   - interview.current_question_index++
   - UPDATE interview
   - 返回 {question_index, is_last_question: index >= question_count}

【错误处理】
- 所有 RPC: interview 不存在 → errors.NotFound("INTERVIEW_NOT_FOUND", "面试不存在")
- BindRealtimeDialog: dialog_id 为空 → errors.BadRequest(...)

【数据库操作】
- 表 interviews: SELECT, UPDATE (dialog_id, current_question_index)
- 表 interview_messages: SELECT (最近10条), INSERT

【依赖的外部服务/库】
- GORM
- 无外部 gRPC 调用(这些是被其他服务调用的)

【验证标准】
1. IsRealtime 对 mode="realtime_voice" 返回 true
2. GetRealtimeContext 返回最近 10 条消息
3. Bind 后 interview 记录包含 dialog_id
4. Append 后消息正确保存
5. is_last_question 在最后一题时为 true

【禁止事项】
- 禁止不验证 interview 存在性
- 禁止 AppendRealtimeAssistantReply 不更新 question_index
- 禁止 GetRealtimeContext 加载所有消息(限制 10 条)
```

---

### 任务 P4-7: Interview Service - Resume Parse MQ Consumer

**目标**：实现简历解析 MQ 消费者，调用 AI 解析简历并存储结果。

**文件路径**：
- `app/interview/internal/server/mq.go` (修改，添加 consumer)
- `app/interview/internal/biz/interview.go` (修改)

**单体参考**：无

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目实现微服务。

【任务】为 Interview 服务添加简历解析 MQ 消费者，在面试创建时异步解析简历。

【目标】接收简历解析任务 → 调用 AI Gateway.ResumeParser → 将解析结果存入面试记录的 resume_parsed_json 字段。

【需要修改的文件】
- app/interview/internal/server/mq.go
- app/interview/internal/biz/interview.go

【MQ 协议】
- Queue: makejob.tasks.interview.resume.parse
- Routing Key: interview.resume.parse
- Payload: {"interview_id": 123, "resume_text": "简历全文..."}

【实现步骤】
1. biz/interview.go - ParseResume UseCase:
   a. 调用 AI Gateway.ResumeParser(resume_text)
   b. 将结果序列化为 JSON
   c. UPDATE interviews SET resume_parsed_json=? WHERE id=?
2. server/mq.go:
   - ResumeParseHandler 实现 TaskHandler
   - HandleTask: 解析 payload → 调用 UseCase
   - 注册: consumer.Register("makejob.tasks.interview.resume.parse", handler)

【错误处理】
- interview 不存在 → log.Warn + return nil (丢弃)
- resume_text 为空 → log.Warn + return nil (丢弃)
- AI Gateway 失败 → return error (重试)

【数据库操作】
- 表 interviews: UPDATE SET resume_parsed_json WHERE id=?

【依赖的外部服务/库】
- AI Gateway (gRPC): ResumeParser

【验证标准】
1. 消费消息后 interview.resume_parsed_json 非空
2. JSON 包含 skills, experience 等字段
3. AI 失败时消息重试

【禁止事项】
- 禁止同步解析(必须通过 MQ 异步)
- 禁止不保存解析结果
```

---

### 任务 P4-8: LearningArchive - MQ Consumer for interview.finished

**目标**：实现学习档案服务的面试完成事件消费者，记录面试结果到学习档案。

**文件路径**：
- `app/learning_archive/internal/server/mq.go` (新建或修改)
- `app/learning_archive/internal/biz/archive.go` (修改)

**单体参考**：无

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目实现微服务。

【任务】为 LearningArchive 服务实现 interview.finished 事件消费者，将面试薄弱点和优势写入学习档案。

【目标】监听面试完成事件 → 提取薄弱点和优势 → 创建/更新学习档案条目 → 发布 archive.written 事件。

【需要创建/修改的文件】
- app/learning_archive/internal/server/mq.go
- app/learning_archive/internal/biz/archive.go

【MQ 协议】
消费:
- Exchange: makejob.events (topic)
- Queue: makejob.events.learning_archive.interview_finished
- Routing Key: interview.finished
- Payload: {"interview_id": 123, "user_id": 456, "score": 78.5, "weak_topics": ["goroutine", "channel"], "strength_topics": ["http", "rest_api"]}

产出:
- Exchange: makejob.events
- Routing Key: archive.written
- Payload: {"user_id": 456, "source": "interview", "source_id": 123, "weak_topics_added": [...], "strength_topics_added": [...]}

【实现步骤】
1. biz/archive.go:
   - HandleInterviewFinished(ctx, event) error:
     a. 对每个 weak_topic: upsert learning_archive_entries (type="weak_topic", topic=name, source="interview", source_id=interview_id, frequency+1)
     b. 对每个 strength_topic: upsert learning_archive_entries (type="strength", topic=name, source="interview", source_id=interview_id)
     c. 更新 user_learning_profile: last_interview_score, last_interview_at
   d. 发布 archive.written 事件
2. server/mq.go: 实现 handler + 注册

【错误处理】
- user_id=0 → log.Warn + return nil (丢弃)
- 数据库写入失败 → return error (重试)
- 发布事件失败 → log.Error (不重试主流程)

【数据库操作】
- 表 learning_archive_entries: UPSERT (INSERT ON CONFLICT UPDATE frequency=frequency+1)
- 表 user_learning_profiles: UPDATE

【依赖的外部服务/库】
- pkg/mq: Consumer + Publisher
- GORM

【验证标准】
1. 面试完成后学习档案中出现对应的 weak_topics
2. 重复面试同一薄弱点，frequency 递增
3. archive.written 事件被发布

【禁止事项】
- 禁止丢失事件(必须可靠消费)
- 禁止覆盖已有 frequency(必须累加)
```

---

### 任务 P4-9: Realtime Service - Full Implementation

**目标**：完整实现实时面试服务，包括 WebSocket 处理和 Volcengine 实时语音集成。

**文件路径**：
- `app/realtime/internal/biz/realtime.go` (新建)
- `app/realtime/internal/data/volcengine_client.go` (新建)
- `app/realtime/internal/service/realtime.go` (新建)
- `app/realtime/internal/server/http.go` (新建，WebSocket handler)
- `app/realtime/internal/conf/conf.go` (新建)
- `app/realtime/cmd/server/main.go` (新建)

**单体参考**：`backend/internal/realtime/volcengine/`

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目实现微服务。

【任务】完整实现 Realtime 微服务，提供 WebSocket 实时面试能力，集成 Volcengine 实时语音 API。

【目标】
1. 接受客户端 WebSocket 连接
2. 创建 Volcengine 实时会话
3. 双向中转音频数据(客户端↔Volcengine)
4. 处理 ASR/Chat/TTS 事件，调用 Interview 服务同步消息
5. 定期注入 RAG 上下文

【需要创建的文件】
- app/realtime/internal/conf/conf.go
- app/realtime/internal/biz/realtime.go
- app/realtime/internal/data/volcengine_client.go
- app/realtime/internal/service/realtime.go
- app/realtime/internal/server/http.go
- app/realtime/cmd/server/main.go
- app/realtime/configs/config.yaml

【接口】
WebSocket endpoint: ws://host:port/ws/interview/{interview_id}
- 客户端发送: 二进制音频帧 (PCM 16kHz 16bit mono)
- 服务端发送: JSON 事件 + TTS 音频帧

【Volcengine 协议】
- 二进制协议: 4 字节 header + payload
- Header: [version(4bit) | header_size(4bit) | message_type(4bit) | flags(4bit) | serialization(4bit) | compression(4bit) | reserved(8bit) | payload_size(24bit)]
- 事件类型:
  - 501: ASR 文本(partial/final)
  - 502: Chat 回复(inject context 也用此 ID)
  - 503: TTS 音频
  - 100: 会话控制(start/stop)

【实现步骤】
1. conf/conf.go: volcengine_app_id, volcengine_token, volcengine_ws_url, interview_service_addr, rag_service_addr, http_addr
2. biz/realtime.go:
   - Session 结构体: interviewID, volcConn, clientConn, cancel context
   - SessionManager: 管理活跃会话
   - HandleSession(ctx, interviewID, clientConn):
     a. 调用 Interview.IsRealtimeInterview 验证
     b. 调用 Interview.GetRealtimeContext 获取初始上下文
     c. 连接 Volcengine WebSocket
     d. 调用 Interview.BindRealtimeDialog(dialog_id)
     e. 启动 3 个 goroutine:
        - clientToVolc: 读客户端音频 → 写 Volcengine
        - volcToClient: 读 Volcengine 事件 → 处理 + 转发客户端
        - ragInjector: 每 30s 调用 RAG.Retrieve 注入上下文(通过 event 502)
   - HandleVolcEvent(event):
     - ASR final text → Interview.AppendRealtimeUserAnswer
     - Chat reply → Interview.AppendRealtimeAssistantReply
     - TTS audio → 转发给客户端
3. data/volcengine_client.go:
   - VolcengineConn: 封装二进制协议 encode/decode
   - Connect(ctx, appID, token) error
   - SendAudio(data []byte) error
   - ReadEvent() (*VolcEvent, error)
   - InjectContext(text string) error  // 发送 event 502
   - Close() error
4. server/http.go:
   - WebSocket upgrade handler (gorilla/websocket)
   - 路由: /ws/interview/:interview_id
   - 认证: 从 query param 或 header 获取 token
5. cmd/main.go: 启动 HTTP server

【错误处理】
- 非实时面试 → WebSocket close 4001 "非实时面试"
- Volcengine 连接失败 → WebSocket close 4002 "语音服务不可用"
- 客户端断开 → 清理 Volcengine 连接 + 结束会话
- Volcengine 断开 → 尝试重连 1 次，失败则关闭客户端连接

【数据库操作】
- 无直接数据库操作(通过 Interview gRPC 间接)

【依赖的外部服务/库】
- Interview Service (gRPC): IsRealtimeInterview, GetRealtimeContext, BindRealtimeDialog, AppendRealtimeUserAnswer, AppendRealtimeAssistantReply
- RAG Service (gRPC): Retrieve
- Volcengine Realtime API (WebSocket)
- github.com/gorilla/websocket: WebSocket server

【验证标准】
1. 客户端连接 WebSocket 成功建立 Volcengine 会话
2. 客户端发送音频 → ASR 识别 → 消息被保存到 Interview
3. Volcengine Chat 回复 → 消息被保存 + TTS 音频转发客户端
4. RAG 上下文定期注入
5. 客户端断开后所有资源正确清理

【禁止事项】
- 禁止 goroutine 泄漏(必须在所有退出路径清理)
- 禁止不处理 Volcengine 断开的情况
- 禁止直接把 Volcengine 原始 bytes 发给客户端(需解析事件后分发)
- 禁止不验证面试是否为 realtime 模式
- 禁止不做 WebSocket 认证
```

---

## Phase 5: 计划 + 成长 + 陪伴

---

### 任务 P5-1: Plan Service - CreatePlan + MQ Consumer

**目标**：实现学习计划创建 + 异步生成流程。

**文件路径**：
- `app/plan/internal/biz/plan.go` (修改)
- `app/plan/internal/data/plan_repo.go` (修改)
- `app/plan/internal/service/plan.go` (修改)
- `app/plan/internal/server/mq.go` (新建)

**单体参考**：无

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目实现微服务。

【任务】实现 Plan 服务的 CreatePlan RPC 和对应的 MQ 消费者(异步生成计划内容)。

【目标】CreatePlan 同步创建计划骨架(status=generating) → 发布 MQ → 返回 ID。Consumer 收到消息后调用 AI 生成计划详情 → 创建任务列表 → 更新状态为 active。

【需要修改/创建的文件】
- app/plan/internal/biz/plan.go
- app/plan/internal/data/plan_repo.go
- app/plan/internal/service/plan.go
- app/plan/internal/server/mq.go

【Proto 接口】
rpc CreatePlan(CreatePlanRequest) returns (CreatePlanResponse);

message CreatePlanRequest {
  repeated string weak_topics = 1;
  string level = 2;             // beginner/intermediate/advanced
  int32 duration_days = 3;
  string industry = 4;
  int32 daily_study_minutes = 5;
  string goal_description = 6;
}

message CreatePlanResponse {
  uint64 plan_id = 1;
  string status = 2;            // "generating"
}

【MQ 协议】
- Queue: makejob.tasks.plan.generate
- Routing Key: plan.generate
- Payload: {"plan_id": 123, "user_id": 456, "weak_topics": [...], "level": "...", "duration_days": 30, ...}

【数据库表】
表 learning_plans:
  - id uint PK
  - user_id uint NOT NULL
  - title varchar(200)
  - description text
  - level varchar(20)
  - duration_days int
  - daily_study_minutes int
  - industry varchar(50)
  - status varchar(20) DEFAULT 'generating' (generating/active/completed/cancelled)
  - completed_tasks int DEFAULT 0
  - total_tasks int DEFAULT 0
  - BaseModel

表 learning_tasks:
  - id uint PK
  - plan_id uint NOT NULL (index)
  - title varchar(200)
  - description text
  - task_type varchar(20) (study/practice/interview/review)
  - phase varchar(50)
  - day_number int
  - duration_minutes int
  - priority varchar(10) (high/medium/low)
  - status varchar(20) DEFAULT 'pending' (pending/in_progress/completed/skipped)
  - completed_at *time.Time
  - sort_order int
  - BaseModel

【实现步骤】
1. CreatePlan RPC (biz):
   a. 获取 userID from ctx
   b. 校验: level 合法, duration_days(7-365), daily_study_minutes(15-480)
   c. 创建 LearningPlan 记录(status="generating")
   d. 发布 MQ 消息: plan.generate
   e. 返回 {plan_id, status: "generating"}
2. MQ Consumer (server/mq.go):
   a. 解析 payload
   b. 调用 AI Gateway.PlanAgent:
      - weak_topics, level, duration_days, industry, daily_study_minutes, goal_description
   c. 解析 AI 返回的 {title, description, phases[], tasks[]}
   d. 更新 plan: title, description, total_tasks
   e. 批量创建 learning_tasks
   f. 更新 plan.status = "active"

【错误处理】
- CreatePlan 参数非法 → errors.BadRequest(...)
- MQ 发布失败 → 删除已创建的 plan 记录 + 返回 500
- Consumer AI 调用失败 → return error (重试)
- Consumer 重试 3 次失败 → plan.status = "failed"

【数据库操作】
- learning_plans: INSERT (创建), UPDATE (状态变更)
- learning_tasks: BatchInsert

【依赖的外部服务/库】
- AI Gateway (gRPC): PlanAgent
- pkg/mq: Publisher + Consumer
- GORM

【验证标准】
1. CreatePlan 返回 plan_id + status="generating"
2. Consumer 消费后 plan 变为 active
3. learning_tasks 被正确创建
4. AI 失败时计划状态变为 failed

【禁止事项】
- 禁止同步调用 AI(必须异步)
- 禁止 CreatePlan 时不校验参数
- 禁止 Consumer 失败不更新 plan 状态
```

---

### 任务 P5-2: Plan Service - GetPlan, GetCurrentPlan, ListPlans

**目标**：实现计划的查询接口。

**文件路径**：
- `app/plan/internal/biz/plan.go` (修改)
- `app/plan/internal/data/plan_repo.go` (修改)
- `app/plan/internal/service/plan.go` (修改)

**单体参考**：无

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目实现微服务。

【任务】实现 Plan 服务的 GetPlan、GetCurrentPlan、ListPlans 三个查询 RPC。

【目标】支持查询单个计划详情(含任务列表)、获取当前活跃计划、分页列出所有计划。

【需要修改的文件】
- app/plan/internal/biz/plan.go
- app/plan/internal/data/plan_repo.go
- app/plan/internal/service/plan.go

【Proto 接口】
rpc GetPlan(GetPlanRequest) returns (PlanDetail);
rpc GetCurrentPlan(google.protobuf.Empty) returns (PlanDetail);
rpc ListPlans(ListPlansRequest) returns (ListPlansResponse);

message GetPlanRequest { uint64 plan_id = 1; }

message PlanDetail {
  uint64 id = 1;
  string title = 2;
  string description = 3;
  string status = 4;
  int32 duration_days = 5;
  int32 completed_tasks = 6;
  int32 total_tasks = 7;
  float progress = 8;          // completed_tasks / total_tasks * 100
  repeated PlanTaskInfo tasks = 9;
  google.protobuf.Timestamp created_at = 10;
}

message PlanTaskInfo {
  uint64 id = 1;
  string title = 2;
  string description = 3;
  string task_type = 4;
  string phase = 5;
  int32 day_number = 6;
  int32 duration_minutes = 7;
  string priority = 8;
  string status = 9;
  google.protobuf.Timestamp completed_at = 10;
}

message ListPlansRequest {
  int32 page = 1;
  int32 page_size = 2;
}

message ListPlansResponse {
  repeated PlanBrief items = 1;
  int32 total = 2;
}

message PlanBrief {
  uint64 id = 1;
  string title = 2;
  string status = 3;
  float progress = 4;
  google.protobuf.Timestamp created_at = 5;
}

【实现步骤】
1. GetPlan:
   a. 获取 userID from ctx
   b. PlanRepo.GetByID(planID) → 验证 plan.UserID == userID
   c. 加载关联的 learning_tasks (ORDER BY sort_order)
   d. 计算 progress
   e. 转换为 PlanDetail
2. GetCurrentPlan:
   a. 获取 userID from ctx
   b. PlanRepo.GetCurrentPlan(userID):
      SELECT FROM learning_plans WHERE user_id=? AND status='active' ORDER BY created_at DESC LIMIT 1
   c. 加载 tasks + 计算 progress
3. ListPlans:
   a. 获取 userID from ctx
   b. PlanRepo.List(userID, page, pageSize):
      SELECT FROM learning_plans WHERE user_id=? ORDER BY created_at DESC LIMIT ? OFFSET ?
   c. COUNT total

【错误处理】
- 计划不存在 → errors.NotFound("PLAN_NOT_FOUND", "学习计划不存在")
- 非本人计划 → errors.New(403, "FORBIDDEN", "无权查看此计划")
- 无当前活跃计划 → errors.NotFound("NO_ACTIVE_PLAN", "当前没有进行中的学习计划")

【数据库操作】
- 表 learning_plans: SELECT by ID, SELECT WHERE user_id AND status, SELECT with pagination
- 表 learning_tasks: SELECT WHERE plan_id=? ORDER BY sort_order

【依赖的外部服务/库】
- GORM
- pkg/auth

【验证标准】
1. GetPlan 返回完整计划 + 任务列表
2. GetCurrentPlan 返回最新的 active 计划
3. 无活跃计划时 GetCurrentPlan 返回 404
4. ListPlans 分页正确
5. progress 计算正确

【禁止事项】
- 禁止返回其他用户的计划
- 禁止不验证所有权
- 禁止 GetPlan 不加载任务列表
```

---

### 任务 P5-3: Plan Service - UpdateTaskStatus

**目标**：实现任务状态更新，带状态机校验和计划进度同步。

**文件路径**：
- `app/plan/internal/biz/plan.go` (修改)
- `app/plan/internal/data/plan_repo.go` (修改)
- `app/plan/internal/service/plan.go` (修改)

**单体参考**：无

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目实现微服务。

【任务】实现 Plan 服务的 UpdateTaskStatus RPC，支持任务状态流转并同步计划进度。

【目标】验证状态转换合法性 → 更新任务状态 → 同步计划的 completed_tasks 计数 → 判断计划是否全部完成。

【需要修改的文件】
- app/plan/internal/biz/plan.go
- app/plan/internal/data/plan_repo.go
- app/plan/internal/service/plan.go

【Proto 接口】
rpc UpdateTaskStatus(UpdateTaskStatusRequest) returns (UpdateTaskStatusResponse);

message UpdateTaskStatusRequest {
  uint64 plan_id = 1;
  uint64 task_id = 2;
  string status = 3;           // completed/skipped/in_progress
}

message UpdateTaskStatusResponse {
  string task_status = 1;
  string plan_status = 2;
  int32 completed_tasks = 3;
  int32 total_tasks = 4;
  float progress = 5;
}

【实现步骤】
1. biz/plan.go - UpdateTaskStatus:
   a. 获取 userID from ctx
   b. 获取 plan → 验证 plan.UserID == userID
   c. 获取 task → 验证 task.PlanID == planID
   d. 状态机校验(合法转换):
      - pending → in_progress ✓
      - pending → completed ✓
      - pending → skipped ✓
      - in_progress → completed ✓
      - 其他 → 非法
   e. 更新 task.status
   f. 若新状态为 completed: task.completed_at = time.Now()
   g. 重新计算 plan.completed_tasks = COUNT tasks WHERE status IN ('completed', 'skipped')
   h. 若 completed_tasks == total_tasks: plan.status = "completed"
   i. 保存 task + plan

【错误处理】
- 计划不存在/非本人 → errors.NotFound/Forbidden
- 任务不存在/不属于该计划 → errors.NotFound("TASK_NOT_FOUND", "任务不存在")
- 非法状态转换 → errors.BadRequest("INVALID_STATUS_TRANSITION", "不允许从 %s 转为 %s", oldStatus, newStatus)

【数据库操作】
- 表 learning_plans: SELECT, UPDATE (completed_tasks, status)
- 表 learning_tasks: SELECT, UPDATE (status, completed_at)

【依赖的外部服务/库】
- GORM (事务)
- pkg/auth

【验证标准】
1. pending→completed 成功，completed_at 被设置
2. completed→pending 被拒绝
3. 所有任务完成后 plan.status 变为 completed
4. progress 正确计算

【禁止事项】
- 禁止不做状态机校验
- 禁止不同步 plan.completed_tasks
- 禁止不在事务中操作(task 更新和 plan 更新必须原子)
```

---

### 任务 P5-4: Plan Service - SubmitTaskFeedback + Diagnosis Consumer

**目标**：实现任务反馈提交和 AI 诊断消费者。

**文件路径**：
- `app/plan/internal/biz/plan.go` (修改)
- `app/plan/internal/data/plan_repo.go` (修改)
- `app/plan/internal/service/plan.go` (修改)
- `app/plan/internal/server/mq.go` (修改)

**单体参考**：无

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目实现微服务。

【任务】实现 Plan 服务的 SubmitTaskFeedback RPC 和对应的 AI 诊断消费者。

【目标】用户对完成的任务提交反馈(难度感受、问题等) → 发布诊断请求 → 消费者调用 AI 分析 → 保存诊断结果。

【需要修改的文件】
- app/plan/internal/biz/plan.go
- app/plan/internal/data/plan_repo.go
- app/plan/internal/service/plan.go
- app/plan/internal/server/mq.go

【Proto 接口】
rpc SubmitTaskFeedback(SubmitTaskFeedbackRequest) returns (SubmitTaskFeedbackResponse);

message SubmitTaskFeedbackRequest {
  uint64 plan_id = 1;
  uint64 task_id = 2;
  string difficulty_feeling = 3;  // too_easy/just_right/too_hard
  string feedback_text = 4;
  int32 actual_duration_minutes = 5;
  repeated string problem_areas = 6;
}

message SubmitTaskFeedbackResponse {
  uint64 feedback_id = 1;
  string status = 2;              // "diagnosis_pending"
}

【MQ 协议】
- Queue: makejob.tasks.plan.feedback.diagnosis
- Routing Key: plan.feedback.diagnosis
- Payload: {"feedback_id": 789, "plan_id": 123, "task_id": 456, "user_id": 111, "feedback_text": "...", "difficulty_feeling": "too_hard", "problem_areas": [...]}

【数据库表】
表 learning_task_feedback:
  - id uint PK
  - plan_id uint NOT NULL
  - task_id uint NOT NULL
  - user_id uint NOT NULL
  - difficulty_feeling varchar(20)
  - feedback_text text
  - actual_duration_minutes int
  - problem_areas_json text
  - diagnosis_json text         // AI 诊断结果
  - diagnosis_status varchar(20) DEFAULT 'pending' (pending/completed/failed)
  - BaseModel

【实现步骤】
1. SubmitTaskFeedback RPC:
   a. 验证 plan 和 task 归属
   b. 创建 learning_task_feedback 记录
   c. 发布 plan.feedback.diagnosis MQ 消息
   d. 返回 feedback_id + status
2. Diagnosis Consumer:
   a. 解析 payload
   b. 调用 AI Gateway.QuizAnalyzer (mode=diagnosis):
      - question: "学习任务: " + task.title
      - user_answer: feedback_text + difficulty_feeling
      - context: problem_areas
   c. 保存诊断结果到 feedback.diagnosis_json
   d. 更新 diagnosis_status = "completed"

【错误处理】
- RPC: 标准 plan/task 验证错误
- Consumer AI 失败 → return error (重试), 最终失败 → diagnosis_status="failed"

【数据库操作】
- learning_task_feedback: INSERT, UPDATE (diagnosis_json, status)

【依赖的外部服务/库】
- AI Gateway (gRPC): QuizAnalyzer
- pkg/mq: Publisher + Consumer
- GORM

【验证标准】
1. 反馈提交后记录正确创建
2. 消费者执行后 diagnosis_json 非空
3. AI 失败时 status 变为 failed

【禁止事项】
- 禁止同步调用 AI 诊断
- 禁止不验证 plan/task 归属
```

---

### 任务 P5-5: Plan Service - AdjustPlan

**目标**：实现 AI 驱动的计划调整功能。

**文件路径**：
- `app/plan/internal/biz/plan.go` (修改)
- `app/plan/internal/data/plan_repo.go` (修改)
- `app/plan/internal/service/plan.go` (修改)

**单体参考**：无

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目实现微服务。

【任务】实现 Plan 服务的 AdjustPlan RPC，根据用户反馈和诊断结果，调用 AI 调整学习计划。

【目标】加载计划 + 所有反馈 + 诊断结果 → 调用 AI Gateway.PlanAgent(mode=adjust) → 应用调整(增删改任务) → 记录调整历史。

【需要修改的文件】
- app/plan/internal/biz/plan.go
- app/plan/internal/data/plan_repo.go
- app/plan/internal/service/plan.go

【Proto 接口】
rpc AdjustPlan(AdjustPlanRequest) returns (AdjustPlanResponse);

message AdjustPlanRequest {
  uint64 plan_id = 1;
  string reason = 2;           // 可选，用户主动说明调整原因
}

message AdjustPlanResponse {
  int32 tasks_added = 1;
  int32 tasks_removed = 2;
  int32 tasks_reordered = 3;
  string adjustment_summary = 4;
}

【实现步骤】
1. AdjustPlan UseCase:
   a. 获取 plan + 所有 tasks + 所有 feedback(含 diagnosis)
   b. 构造 AI Gateway.PlanAgent 请求:
      - mode: "adjust"
      - 额外 context: 当前 tasks 状态, feedback 摘要, difficulty_feelings 统计
   c. AI 返回调整方案: {add: [{title, ...}], remove: [task_id, ...], reorder: [{task_id, new_sort_order}], summary: "..."}
   d. 应用调整:
      - add: 批量 INSERT learning_tasks
      - remove: 批量 soft DELETE learning_tasks + 更新 plan.total_tasks
      - reorder: 批量 UPDATE sort_order
   e. 记录 plan_adjustments:
      type PlanAdjustment struct {
        BaseModel
        PlanID          uint
        Reason          string
        AddedCount      int
        RemovedCount    int
        ReorderedCount  int
        Summary         string
        DetailsJSON     string  // AI 原始返回
      }
   f. 返回统计信息

【错误处理】
- 计划不存在/非本人 → 标准错误
- 计划已完成 → errors.BadRequest("PLAN_COMPLETED", "已完成的计划不能调整")
- AI 调用失败 → errors.New(502, "AI_ADJUST_FAILED", "AI调整计划失败")

【数据库操作】
- learning_plans: SELECT + UPDATE total_tasks
- learning_tasks: SELECT all, INSERT (新增), DELETE (移除), UPDATE (重排序)
- learning_task_feedback: SELECT WHERE plan_id=?
- plan_adjustments: INSERT

【依赖的外部服务/库】
- AI Gateway (gRPC): PlanAgent (mode=adjust)
- GORM (事务)

【验证标准】
1. 调整后 tasks 列表符合 AI 建议
2. plan.total_tasks 正确更新
3. plan_adjustments 记录被创建
4. 已完成的计划不允许调整

【禁止事项】
- 禁止不使用事务(多表操作必须原子)
- 禁止删除已完成的 task(只删除 pending 状态的)
- 禁止不记录调整历史
```

---

### 任务 P5-6: Growth Service - Rewrite GetGrowthSummary

**目标**：重写成长总览，通过并发 gRPC 调用聚合多服务数据。

**文件路径**：
- `app/growth/internal/biz/growth.go` (修改/重写)
- `app/growth/internal/service/growth.go` (修改)
- `app/growth/internal/conf/conf.go` (修改)

**单体参考**：无

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目实现微服务。

【任务】重写 Growth 服务的 GetGrowthSummary RPC，通过并发 gRPC 调用多个下游服务聚合数据。

【目标】并发调用 Question/Plan/LearningArchive/Interview 服务获取各维度数据，汇总为用户成长总览。

【需要修改的文件】
- app/growth/internal/biz/growth.go
- app/growth/internal/service/growth.go
- app/growth/internal/conf/conf.go

【Proto 接口】
rpc GetGrowthSummary(google.protobuf.Empty) returns (GrowthSummaryResponse);

message GrowthSummaryResponse {
  // 练习统计
  int32 total_questions_done = 1;
  int32 correct_rate_percent = 2;
  int32 streak_days = 3;

  // 计划进度
  string current_plan_title = 4;
  float current_plan_progress = 5;
  int32 plan_completed_tasks = 6;
  int32 plan_total_tasks = 7;

  // 面试统计
  int32 total_interviews = 8;
  float avg_interview_score = 9;
  float latest_interview_score = 10;

  // 薄弱点
  repeated string top_weak_topics = 11;
  repeated string recent_focus_signals = 12;

  // 成就
  int32 level = 13;
  int32 experience_points = 14;
}

【实现步骤】
1. conf/conf.go 添加所有下游服务地址:
   - question_service_addr
   - plan_service_addr
   - learning_archive_service_addr
   - interview_service_addr
2. biz/growth.go - GetGrowthSummary UseCase:
   - 使用 golang.org/x/sync/errgroup 并发调用:
     a. g.Go: Question.GetUserPracticeStats(userID) → total_done, correct_rate, streak
     b. g.Go: Plan.GetCurrentPlan() → title, progress, completed/total tasks
     c. g.Go: LearningArchive.GetWeakTopics(userID, limit=5) → top_weak_topics
     d. g.Go: LearningArchive.GetFocusSignals(userID) → recent_focus_signals
     e. g.Go: Interview.GetInterviewStats(userID) → total, avg_score, latest_score
   - 设置超时: ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
   - 等待所有完成: g.Wait()
   - 对于失败的调用: 使用零值填充(不因单个服务失败阻塞整体)
3. service 层: 调用 UseCase + 组装响应

【错误处理】
- 任一下游服务失败 → 该部分使用零值，整体不失败
- 所有服务都失败 → errors.New(503, "SERVICE_UNAVAILABLE", "数据服务暂不可用")
- 超时(5s) → 返回已获取到的部分数据

【数据库操作】
- 无直接数据库操作(全部通过 gRPC)

【依赖的外部服务/库】
- Question Service (gRPC): GetUserPracticeStats
- Plan Service (gRPC): GetCurrentPlan
- LearningArchive Service (gRPC): GetWeakTopics, GetFocusSignals
- Interview Service (gRPC): GetInterviewStats
- golang.org/x/sync/errgroup

【验证标准】
1. 正常情况返回所有维度数据
2. 单个服务不可用时其他维度仍有数据
3. 总超时不超过 5 秒
4. 并发调用(非串行)

【禁止事项】
- 禁止串行调用(必须使用 errgroup 并发)
- 禁止任一服务失败导致整体 500
- 禁止不设置超时
- 禁止在 service 层直接 grpc.Dial
```

---

### 任务 P5-7: Growth Service - Rewrite GetWeeklyFocus

**目标**：重写周聚焦推荐，结合学习档案和题目集匹配。

**文件路径**：
- `app/growth/internal/biz/growth.go` (修改)
- `app/growth/internal/service/growth.go` (修改)

**单体参考**：无

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目实现微服务。

【任务】重写 Growth 服务的 GetWeeklyFocus RPC，根据学习档案推导本周聚焦方向。

【目标】从 LearningArchive 获取薄弱信号和聚焦方向 → 从当前计划推导阶段 → 匹配相关题目集 → 返回本周聚焦建议。

【需要修改的文件】
- app/growth/internal/biz/growth.go
- app/growth/internal/service/growth.go

【Proto 接口】
rpc GetWeeklyFocus(google.protobuf.Empty) returns (WeeklyFocusResponse);

message WeeklyFocusResponse {
  string dominant_phase = 1;           // 当前学习阶段(从计划推导)
  repeated string focus_topics = 2;    // 本周聚焦主题
  repeated string weak_topics = 3;     // 薄弱点(需加强)
  repeated QuestionSetBrief recommended_sets = 4;
  string recommendation_reason = 5;
}

message QuestionSetBrief {
  uint64 id = 1;
  string title = 2;
  int32 question_count = 3;
}

【实现步骤】
1. GetWeeklyFocus UseCase:
   a. 调用 LearningArchive.GetFocusSignals(userID) → focus_topics
   b. 调用 LearningArchive.GetWeakTopics(userID, limit=3) → weak_topics
   c. 调用 Plan.GetCurrentPlan() → 提取当前阶段(按 day_number 和 today 推算 dominant_phase)
   d. 合并 focus + weak → 作为关键词
   e. 调用 Question.ListQuestionSets(category 匹配关键词) → recommended_sets
   f. 生成 recommendation_reason (模板化文案)

【错误处理】
- LearningArchive 不可用 → 返回空 focus/weak，仅从 plan 推导
- Plan 不可用 → dominant_phase = "unknown"
- Question 不可用 → recommended_sets 为空

【数据库操作】
- 无直接数据库操作

【依赖的外部服务/库】
- LearningArchive Service (gRPC): GetFocusSignals, GetWeakTopics
- Plan Service (gRPC): GetCurrentPlan
- Question Service (gRPC): ListQuestionSets

【验证标准】
1. 返回非空的聚焦建议
2. recommended_sets 与 weak_topics 相关
3. 任一服务不可用时降级返回

【禁止事项】
- 禁止不做降级处理
- 禁止 recommendation_reason 为空字符串
```

---

### 任务 P5-8: Growth Service - Fix SyncStudyLog

**目标**：修复学习日志同步，合并单体和微服务两套字段。

**文件路径**：
- `app/growth/internal/biz/growth.go` (修改)
- `app/growth/internal/data/growth_repo.go` (修改)
- `app/growth/internal/service/growth.go` (修改)

**单体参考**：无

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目修复 Bug。

【任务】修复 Growth 服务的 SyncStudyLog RPC，合并单体遗留字段和微服务新增字段。

【目标】study_logs 表需要同时支持旧字段(date_key, plan_id, summary)和新字段(action, ref_id, duration, source)，确保完整记录。

【需要修改的文件】
- app/growth/internal/biz/growth.go
- app/growth/internal/data/growth_repo.go
- app/growth/internal/service/growth.go

【Proto 接口】
rpc SyncStudyLog(SyncStudyLogRequest) returns (google.protobuf.Empty);

message SyncStudyLogRequest {
  // 原有字段
  string date_key = 1;         // "2024-01-15" 格式
  uint64 plan_id = 2;
  string summary = 3;

  // 新增字段
  string action = 4;           // practice/interview/study/review
  uint64 ref_id = 5;           // 关联实体 ID
  string ref_type = 6;         // question/interview/plan_task
  int32 duration_minutes = 7;
  string source = 8;           // app/web/api
}

【数据库表】(合并后)
表 study_logs:
  - id uint PK
  - user_id uint NOT NULL
  - date_key varchar(10) NOT NULL  // YYYY-MM-DD
  - plan_id uint
  - summary text
  - action varchar(20)
  - ref_id uint
  - ref_type varchar(20)
  - duration_minutes int DEFAULT 0
  - source varchar(10)
  - BaseModel
  - 唯一约束: (user_id, date_key, action, ref_id) 防重复

【实现步骤】
1. biz/growth.go:
   - 更新 StudyLog 实体包含所有字段
   - SyncStudyLog UseCase:
     a. 获取 userID from ctx
     b. 若 date_key 为空 → 使用 today
     c. 构造 StudyLog 填充所有字段
     d. Upsert: 若 (user_id, date_key, action, ref_id) 已存在 → 更新 duration/summary
     e. 若不存在 → 创建
2. data 层: 实现 Upsert 逻辑 (GORM Clauses OnConflict)
3. service 层: 映射 proto → biz

【错误处理】
- action 非法 → errors.BadRequest("INVALID_ACTION", "action 必须为 practice/interview/study/review")
- duration < 0 → errors.BadRequest(...)

【数据库操作】
- study_logs: INSERT ON CONFLICT (user_id, date_key, action, ref_id) DO UPDATE SET duration_minutes=?, summary=?

【依赖的外部服务/库】
- GORM (Clauses)
- pkg/auth

【验证标准】
1. 新字段(action, ref_id, duration)正确保存
2. 旧字段(date_key, plan_id, summary)仍正常工作
3. 重复调用 upsert 而非报错
4. 缺少 date_key 自动用 today

【禁止事项】
- 禁止删除旧字段(向后兼容)
- 禁止重复记录(必须 upsert)
- 禁止 duration_minutes 为负数
```

---

### 任务 P5-9: Companion Service - Complete Implementation

**目标**：完整实现 AI 陪伴服务，包含聊天、状态管理和语音合成。

**文件路径**：
- `app/companion/internal/biz/companion.go` (新建)
- `app/companion/internal/data/companion_repo.go` (新建)
- `app/companion/internal/data/tts_client.go` (新建)
- `app/companion/internal/service/companion.go` (新建)
- `app/companion/internal/server/grpc.go` (新建)
- `app/companion/internal/conf/conf.go` (新建)
- `app/companion/cmd/server/main.go` (新建)

**单体参考**：`backend/internal/ai/companion_agent.go`

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目实现微服务。

【任务】完整实现 Companion(AI陪伴)微服务，提供聊天、状态管理和 TTS 语音合成。

【目标】
- Chat: 收集学习上下文 → 调用 AI Gateway.CompanionAgent → 调用 Live2DDirector → 合成语音 → 返回完整响应
- GetCompanionState: 从 session 表查询上次状态
- SynthesizeSpeech: 调用 Volcengine TTS 返回音频 URL

【需要创建的文件】
- app/companion/internal/conf/conf.go
- app/companion/internal/biz/companion.go
- app/companion/internal/data/companion_repo.go
- app/companion/internal/data/tts_client.go
- app/companion/internal/service/companion.go
- app/companion/internal/server/grpc.go
- app/companion/cmd/server/main.go

【Proto 接口】
service CompanionService {
  rpc Chat(CompanionChatRequest) returns (CompanionChatResponse);
  rpc GetCompanionState(google.protobuf.Empty) returns (CompanionState);
  rpc SynthesizeSpeech(SynthesizeSpeechRequest) returns (SynthesizeSpeechResponse);
}

message CompanionChatRequest {
  string message = 1;
  string user_emotion = 2;
}

message CompanionChatResponse {
  string reply = 1;
  string emotion = 2;
  string action = 3;
  Live2DDirectiveProto live2d_directive = 4;
  string audio_url = 5;        // TTS 音频 URL
}

message CompanionState {
  string last_emotion = 1;
  string last_topic = 2;
  int32 session_count = 3;
  google.protobuf.Timestamp last_chat_at = 4;
}

message SynthesizeSpeechRequest {
  string text = 1;
  string voice = 2;            // 默认 "zh_female_cancan"
}

message SynthesizeSpeechResponse {
  string audio_url = 1;
  int32 duration_ms = 2;
}

【数据库表】
表 companion_sessions:
  - id uint PK
  - user_id uint NOT NULL (index)
  - last_emotion varchar(20)
  - last_topic varchar(100)
  - session_count int DEFAULT 0
  - last_chat_at time.Time
  - messages_json text           // 最近 10 条消息
  - BaseModel

【实现步骤】
1. Chat UseCase:
   a. 获取 userID, 加载/创建 companion_session
   b. 获取学习上下文(调用 LearningArchive + Plan, 可选):
      - 当前学习状态
      - 最近薄弱点
   c. 构造 AI Gateway.CompanionAgent 请求:
      - messages: session.messages + new message
      - user_emotion
      - learning_state: 从上下文推导
   d. 调用 CompanionAgent → 获取 reply, emotion, action
   e. 调用 AI Gateway.Live2DDirector:
      - text_to_express: reply
      - current_emotion: emotion
      - scene: "companion"
   f. 调用 TTS (可选, 若配置开启):
      - text: reply
      - 返回 audio_url
   g. 更新 session: messages, last_emotion, last_topic, session_count++, last_chat_at
   h. 返回完整响应
2. GetCompanionState: 查询 companion_sessions WHERE user_id=?
3. SynthesizeSpeech (data/tts_client.go):
   - HTTP POST 到 Volcengine TTS API
   - 返回音频文件 URL

【错误处理】
- message 为空 → errors.BadRequest("INVALID_MESSAGE", "消息不能为空")
- AI Gateway 失败 → errors.New(502, "COMPANION_UNAVAILABLE", "AI陪伴暂不可用")
- TTS 失败 → audio_url 为空(不影响主流程)
- LearningArchive/Plan 不可用 → 跳过上下文增强

【数据库操作】
- companion_sessions: SELECT WHERE user_id=?, INSERT/UPDATE (upsert)

【依赖的外部服务/库】
- AI Gateway (gRPC): CompanionAgent, Live2DDirector
- LearningArchive Service (gRPC, 可选): GetWeakTopics
- Plan Service (gRPC, 可选): GetCurrentPlan
- Volcengine TTS API (HTTP)

【验证标准】
1. Chat 返回包含 reply + emotion + live2d_directive
2. session 记录正确更新
3. GetCompanionState 返回上次会话状态
4. TTS 失败不影响 Chat 响应

【禁止事项】
- 禁止不保存会话历史
- 禁止 TTS 失败导致 Chat 失败
- 禁止不限制 session.messages 长度(最多保留 10 条)
- 禁止不调用 Live2DDirector(陪伴场景必须有表情控制)
```

---

## Phase 6: Admin BFF + Bridge 移除

---

### 任务 P6-1: Admin - Refactor to delegate to domain services

**目标**：将 Admin 服务从直接操作数据库改为委托给各领域微服务。

**文件路径**：
- `app/admin/internal/biz/admin.go` (修改)
- `app/admin/internal/data/admin_repo.go` (修改)
- `app/admin/internal/service/admin.go` (修改)
- `app/admin/internal/conf/conf.go` (修改)

**单体参考**：无

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目重构微服务。

【任务】重构 Admin 服务，将直接数据库操作替换为 gRPC 调用各领域服务。

【目标】Admin 不再直接访问其他服务的数据库表，而是通过 gRPC 调用对应的领域服务来完成管理操作。

【需要修改的文件】
- app/admin/internal/biz/admin.go
- app/admin/internal/data/admin_repo.go
- app/admin/internal/service/admin.go
- app/admin/internal/conf/conf.go

【需要委托的操作】
1. 用户管理:
   - ListUsers → User.ListUsers (需在 User 服务添加)
   - UpdateUserRole → User.UpdateUserRole (需在 User 服务添加)
   - BanUser → User.BanUser (需在 User 服务添加)
2. 题目管理:
   - ListQuestions → Question.ListQuestions
   - CreateQuestion → Question.CreateQuestion
   - UpdateQuestion → Question.UpdateQuestion
   - DeleteQuestion → Question.DeleteQuestion
3. AI 配置管理:
   - ListAIConfigs → AI Gateway (新增 admin RPC)
   - UpdateAIConfig → AI Gateway
   - ListPromptTemplates → AI Gateway
   - UpdatePromptTemplate → AI Gateway
4. 数据统计:
   - GetDashboardStats → 并发调用各服务统计接口

【实现步骤】
1. conf/conf.go: 添加所有下游服务地址
2. biz/admin.go:
   - 移除直接 DB 操作的 Repo 接口
   - 添加各 gRPC client 注入
   - 重写 UseCase 方法为 gRPC 调用
3. data/admin_repo.go:
   - 移除对 users/questions/ai_configs 表的直接操作
   - 保留 Admin 自有表的操作(如 admin_logs)
4. service/admin.go:
   - 保持 handler 签名不变
   - 内部实现改为调用 biz 层(gRPC 委托)
5. 对于 User/AI Gateway 缺少的 admin RPC:
   - 在对应 proto 中定义(本任务只做 Admin 端改造)
   - 调用时若 RPC 不存在，做 TODO 标记 + 返回 "功能迁移中"

【错误处理】
- 下游服务不可用 → errors.New(503, "SERVICE_UNAVAILABLE", "%s 服务暂不可用", serviceName)
- 权限不足(非管理员调用) → errors.New(403, "FORBIDDEN", "需要管理员权限")

【数据库操作】
- 仅保留 admin_logs 表: INSERT (记录管理操作)
- 移除对其他表的直接操作

【依赖的外部服务/库】
- User Service (gRPC)
- Question Service (gRPC)
- AI Gateway Service (gRPC)
- Interview Service (gRPC)

【验证标准】
1. Admin 不再直接连接非 admin 的数据库表
2. 用户管理通过 User Service
3. 题目管理通过 Question Service
4. 管理操作日志仍正常记录

【禁止事项】
- 禁止保留对其他领域数据库表的直接操作
- 禁止删除管理日志功能
- 禁止改变 Admin API 的 proto 接口(保持前端兼容)
```

---

### 任务 P6-2: Admin - Implement GenerateQuestionPipelineStream SSE

**目标**：实现 Admin 端的题目生成流水线 SSE 流式推送。

**文件路径**：
- `app/admin/internal/service/admin.go` (修改)
- `app/admin/internal/server/http.go` (修改，添加 SSE 端点)
- `app/admin/internal/biz/admin.go` (修改)

**单体参考**：无

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目实现微服务。

【任务】实现 Admin 服务的题目生成流水线 SSE (Server-Sent Events) 端点，实时推送生成进度。

【目标】管理员触发题目批量生成 → 发布 MQ 任务 → 通过 SSE 实时推送进度事件到前端。

【需要修改的文件】
- app/admin/internal/service/admin.go
- app/admin/internal/server/http.go
- app/admin/internal/biz/admin.go

【HTTP 接口】
POST /admin/question-pipeline/generate
  Request: { category_id, difficulty, count, question_type, topic }
  Response: { task_id }

GET /admin/question-pipeline/generate/stream?task_id=xxx
  Response: SSE stream
  Events:
    - event: progress, data: {"current": 3, "total": 10, "status": "generating"}
    - event: question, data: {"question_id": 123, "title": "..."}
    - event: complete, data: {"total_generated": 10, "total_failed": 0}
    - event: error, data: {"message": "..."}

【数据库表】(任务追踪)
表 pipeline_tasks:
  - id uint PK
  - task_id varchar(36) NOT NULL (UUID, unique)
  - admin_user_id uint
  - category_id uint
  - difficulty varchar(20)
  - count int
  - topic varchar(200)
  - status varchar(20) (pending/running/completed/failed)
  - current_progress int DEFAULT 0
  - result_json text            // 完成后的统计
  - BaseModel

【实现步骤】
1. POST /generate:
   a. 生成 task_id (UUID)
   b. 创建 pipeline_tasks 记录 (status=pending)
   c. 发布 MQ: question.pipeline.build + task_id
   d. 返回 task_id
2. GET /generate/stream (SSE):
   a. 验证 task_id 存在
   b. 设置 SSE headers: Content-Type=text/event-stream, Cache-Control=no-cache
   c. 轮询 pipeline_tasks 记录(每 2 秒):
      - 若 status 变化 → 发送 progress 事件
      - 若 current_progress 变化 → 发送 progress 事件
      - 若 status=completed → 发送 complete 事件 + 关闭
      - 若 status=failed → 发送 error 事件 + 关闭
   d. 超时 5 分钟自动关闭
3. Question Service MQ Consumer (已在 P3-7 实现) 需要:
   - 每生成一题 → UPDATE pipeline_tasks SET current_progress=current_progress+1
   - 完成时 → UPDATE status=completed, result_json=...

【错误处理】
- task_id 不存在 → HTTP 404
- 非管理员 → HTTP 403
- 连接断开 → 清理资源

【数据库操作】
- pipeline_tasks: INSERT, SELECT (轮询), UPDATE (进度)

【依赖的外部服务/库】
- pkg/mq: Publisher
- GORM
- net/http (SSE, 使用 http.Flusher)

【验证标准】
1. 触发生成后返回 task_id
2. SSE 连接能接收到 progress 事件
3. 生成完成后收到 complete 事件
4. 连接超时后正确关闭

【禁止事项】
- 禁止使用 WebSocket(必须用 SSE)
- 禁止轮询间隔小于 1 秒(避免 DB 压力)
- 禁止不设置 SSE 超时
- 禁止 SSE 不处理客户端断开
```

---

### 任务 P6-3: Gateway - Remove all bridge code

**目标**：从网关服务中移除所有 bridge 兼容层代码。

**文件路径**：
- `app/gateway/internal/proxy/` (删除 bridge 相关文件)
- `backend/bridge/` (整个目录删除)
- `app/gateway/configs/config.yaml` (修改，移除 DB 配置)

**单体参考**：无

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目做清理工作。

【任务】从 Gateway 服务中移除所有 bridge (桥接层) 代码，完全切换到微服务架构。

【目标】删除 bridge 兼容层代码和数据库直连配置，Gateway 完全作为 gRPC 反向代理工作。

【需要操作的文件】
- 删除: app/gateway/internal/proxy/ 中与 bridge 相关的文件
- 删除: backend/bridge/ 整个目录
- 修改: app/gateway/configs/config.yaml (移除 database 配置段)
- 修改: app/gateway/internal/conf/conf.go (移除 DB 相关配置字段)
- 修改: app/gateway/cmd/server/main.go (移除 DB 初始化代码)

【实现步骤】
1. 识别 bridge 代码:
   - app/gateway/internal/proxy/ 中引用 backend/ 包的文件
   - backend/bridge/ 目录(整个删除)
   - 任何 import "makejob/backend/" 的代码行
2. 删除文件:
   - rm -rf backend/bridge/
   - rm bridge 相关的 proxy 文件
3. 清理配置:
   - config.yaml: 删除 database: 段
   - conf.go: 删除 DB DSN 字段
4. 清理代码:
   - main.go: 删除 gorm.Open / DB 初始化
   - 删除 bridge handler 的路由注册
5. 确保编译通过:
   - go build ./app/gateway/...
   - 修复所有编译错误

【错误处理】
- 若某些路由仍依赖 bridge → 记录 TODO + 暂时返回 501 Not Implemented

【数据库操作】
- 删除 Gateway 的所有数据库连接(Gateway 不应直连任何数据库)

【依赖的外部服务/库】
- 无(纯删除操作)

【验证标准】
1. go build ./app/gateway/... 编译成功
2. 无任何 import "makejob/backend/" 残留
3. Gateway config.yaml 无 database 配置
4. backend/bridge/ 目录不存在

【禁止事项】
- 禁止删除非 bridge 的正常 proxy 代码
- 禁止保留对 backend 包的任何导入
- 禁止删除 gRPC proxy 路由(只删 bridge 路由)
- 禁止在删除时不检查编译是否通过
```

---

### 任务 P6-4: Gateway - Register all missing HTTP routes

**目标**：为 Gateway 注册所有缺失的 HTTP 路由映射到对应微服务 RPC。

**文件路径**：
- `app/gateway/internal/server/http.go` (修改)
- `app/gateway/internal/conf/conf.go` (修改，添加所有服务地址)

**单体参考**：无

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目实现微服务。

【任务】在 Gateway 中注册所有前端需要的 HTTP 路由，映射到对应的微服务 gRPC RPC。

【目标】确保每个前端路由都能通过 Gateway 正确转发到对应的微服务，包括普通 REST、WebSocket 和 SSE。

【需要修改的文件】
- app/gateway/internal/server/http.go
- app/gateway/internal/conf/conf.go

【路由映射表】
// User Service
POST   /api/v1/auth/register       → User.Register
POST   /api/v1/auth/login          → User.Login
POST   /api/v1/auth/refresh        → User.RefreshToken
POST   /api/v1/auth/logout         → User.Logout
GET    /api/v1/user/profile        → User.GetProfile
PUT    /api/v1/user/profile        → User.UpdateProfile

// Question Service
GET    /api/v1/questions            → Question.ListQuestions
GET    /api/v1/questions/:id        → Question.GetQuestion
POST   /api/v1/questions/run-code   → Question.RunCode
POST   /api/v1/questions/submit     → Question.SubmitAnswer
GET    /api/v1/questions/recommendations → Question.GetPracticeRecommendations
GET    /api/v1/question-sets        → Question.ListQuestionSets
GET    /api/v1/question-sets/:id    → Question.GetQuestionSetDetail
GET    /api/v1/mistakes/topics      → Question.ListMistakeTopics
POST   /api/v1/exams/timed         → Question.GenerateTimedExam
POST   /api/v1/exams/:id/submit    → Question.SubmitExam
POST   /api/v1/notes               → Question.CreateNote
DELETE /api/v1/notes/:id           → Question.DeleteNote

// Interview Service
POST   /api/v1/interviews          → Interview.CreateInterview
GET    /api/v1/interviews/:id      → Interview.GetInterview
POST   /api/v1/interviews/:id/next-question → Interview.GetNextQuestion
POST   /api/v1/interviews/:id/finish → Interview.FinishInterview
GET    /api/v1/interviews/:id/report → Interview.GetReport
POST   /api/v1/interviews/:id/coding → Interview.SubmitCodingAnswer

// Realtime (WebSocket upgrade)
GET    /api/v1/interviews/:id/ws   → WebSocket proxy to Realtime Service

// Plan Service
POST   /api/v1/plans               → Plan.CreatePlan
GET    /api/v1/plans               → Plan.ListPlans
GET    /api/v1/plans/current       → Plan.GetCurrentPlan
GET    /api/v1/plans/:id           → Plan.GetPlan
PUT    /api/v1/plans/:id/tasks/:tid/status → Plan.UpdateTaskStatus
POST   /api/v1/plans/:id/tasks/:tid/feedback → Plan.SubmitTaskFeedback
POST   /api/v1/plans/:id/adjust    → Plan.AdjustPlan

// Growth Service
GET    /api/v1/growth/summary      → Growth.GetGrowthSummary
GET    /api/v1/growth/weekly-focus  → Growth.GetWeeklyFocus
POST   /api/v1/growth/study-log    → Growth.SyncStudyLog

// Companion Service
POST   /api/v1/companion/chat      → Companion.Chat
GET    /api/v1/companion/state     → Companion.GetCompanionState
POST   /api/v1/companion/tts       → Companion.SynthesizeSpeech

// Community Service
GET    /api/v1/community/posts     → Community.ListPosts
POST   /api/v1/community/posts     → Community.CreatePost
GET    /api/v1/community/posts/:id → Community.GetPost
PUT    /api/v1/community/posts/:id → Community.UpdatePost
POST   /api/v1/community/posts/:id/like → Community.ToggleLike
GET    /api/v1/community/my/posts  → Community.ListMyPosts

// Membership Service
POST   /api/v1/membership/orders   → Membership.CreateOrder
GET    /api/v1/membership/info     → Membership.GetUserMembership
POST   /api/v1/membership/check-access → Membership.CheckFeatureAccess

// Admin (SSE)
GET    /admin/question-pipeline/generate/stream → SSE proxy to Admin Service

【实现步骤】
1. conf/conf.go: 添加所有服务地址配置:
   user_addr, question_addr, interview_addr, realtime_addr, plan_addr, growth_addr, companion_addr, community_addr, membership_addr, admin_addr
2. http.go:
   - 使用 Kratos HTTP transport 注册路由
   - 对每个路由: gRPC-Gateway 风格转发或手写 handler
   - WebSocket 路由: 使用 gorilla/websocket upgrader + TCP proxy
   - SSE 路由: HTTP reverse proxy with flush
3. 中间件链:
   - CORS
   - Auth (除 register/login/refresh 外)
   - Rate limiting
   - Request logging

【错误处理】
- 下游服务不可用 → HTTP 503 {"error": "服务暂不可用"}
- 认证失败 → HTTP 401
- 路由不存在 → HTTP 404

【数据库操作】
- 无(Gateway 不连数据库)

【依赖的外部服务/库】
- 所有微服务 (gRPC clients)
- gorilla/websocket (WebSocket proxy)
- Kratos HTTP transport

【验证标准】
1. 每个路由都能正确转发到对应服务
2. WebSocket 升级成功
3. SSE 流正常推送
4. Auth 中间件正确拦截未认证请求
5. CORS 正确处理

【禁止事项】
- 禁止在 Gateway 中有任何业务逻辑
- 禁止 Gateway 连接数据库
- 禁止遗漏路由(前端所有路由都必须注册)
- 禁止 WebSocket 路由不做认证
```

---

## Phase 7: 优化与运维

---

### 任务 P7-1: Add health checks to all services

**目标**：为所有微服务添加标准健康检查端点。

**文件路径**：
- 每个服务的 `app/<service>/internal/server/grpc.go` (修改)
- `pkg/health/health.go` (新建，公共健康检查逻辑)

**单体参考**：无

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目实现运维基础设施。

【任务】为所有微服务添加 gRPC 和 HTTP 健康检查端点。

【目标】每个服务暴露标准的 gRPC Health Check (grpc.health.v1) 和 HTTP /healthz 端点，支持 Kubernetes 就绪/存活探针。

【需要创建/修改的文件】
- pkg/health/health.go (新建，公共健康检查逻辑)
- 每个服务的 server/grpc.go (修改，注册 Health Service)

服务列表: user, question, interview, plan, growth, companion, community, membership, admin, ai_gateway, coderunner, rag, realtime, learning_archive, gateway

【实现步骤】
1. pkg/health/health.go:
   - HealthChecker 接口: Check(ctx) error
   - DBHealthChecker: ping database
   - RedisHealthChecker: ping redis
   - GRPCHealthChecker: check downstream gRPC service
   - CompositeChecker: 组合多个 checker
2. 每个服务 server/grpc.go:
   - import "google.golang.org/grpc/health/grpc_health_v1"
   - 注册 Health Service:
     healthServer := health.NewServer()
     grpc_health_v1.RegisterHealthServer(srv, healthServer)
   - 启动后台 goroutine 定期检查依赖并更新 serving status
3. Gateway HTTP /healthz:
   - 返回 200 + {"status": "ok", "services": {"user": "serving", ...}}
   - 检查所有下游服务的 health

【健康检查维度】
每个服务检查自身的核心依赖:
- 有 DB 的服务: ping DB
- 有 Redis 的服务: ping Redis
- 有 MQ 的服务: check AMQP connection
- 有 Milvus 的服务: check Milvus connection
- 有下游 gRPC 的服务: check downstream health

【错误处理】
- 依赖不可用 → 设置 NOT_SERVING 状态
- 依赖恢复 → 自动恢复 SERVING 状态
- 检查超时 → 视为不健康

【数据库操作】
- db.Raw("SELECT 1").Error (ping check)

【依赖的外部服务/库】
- google.golang.org/grpc/health
- google.golang.org/grpc/health/grpc_health_v1

【验证标准】
1. grpcurl 调用 grpc.health.v1.Health/Check 返回 SERVING
2. HTTP GET /healthz 返回 200
3. DB 断开后状态变为 NOT_SERVING
4. DB 恢复后状态变回 SERVING

【禁止事项】
- 禁止健康检查阻塞主流程
- 禁止不检查核心依赖(只返回 OK)
- 禁止 health check goroutine 泄漏
- 禁止检查间隔小于 5 秒(避免压力)
```

---

### 任务 P7-2: Unified tracing propagation

**目标**：为所有微服务实现统一的分布式链路追踪。

**文件路径**：
- `pkg/trace/trace.go` (新建)
- 每个服务的 `cmd/server/main.go` (修改，初始化 tracer)
- 每个服务的 `server/grpc.go` (修改，添加 tracing interceptor)

**单体参考**：无

#### PROMPT

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目实现可观测性基础设施。

【任务】为所有微服务实现统一的 OpenTelemetry 分布式链路追踪。

【目标】请求从 Gateway 进入后，trace ID 在所有下游微服务间自动传播，最终可在 Jaeger/Tempo 中查看完整调用链。

【需要创建/修改的文件】
- pkg/trace/trace.go (新建，公共初始化逻辑)
- 每个服务的 cmd/server/main.go (初始化 TracerProvider)
- 每个服务的 server/grpc.go (添加 tracing interceptor)

【实现步骤】
1. pkg/trace/trace.go:
   - InitTracer(serviceName, endpoint string) (*sdktrace.TracerProvider, error):
     a. 创建 OTLP gRPC exporter (连接 Jaeger/OTEL Collector)
     b. 创建 TracerProvider:
        - Resource: service.name, service.version
        - Sampler: AlwaysSample (开发) / TraceIDRatioBased(0.1) (生产)
        - BatchSpanProcessor
     c. 设为全局 TraceProvider
     d. 返回 provider (用于 shutdown)
   - 提供 Shutdown(ctx) 方法
2. 每个服务 main.go:
   - tp, err := trace.InitTracer("makejob.{service}", conf.OTELEndpoint)
   - defer tp.Shutdown(context.Background())
3. 每个服务 grpc.go:
   - Server: grpc.UnaryInterceptor(otelgrpc.UnaryServerInterceptor())
   - Client: grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor())
4. MQ 传播:
   - Publish 时: 将 trace context 注入 AMQP headers
   - Consume 时: 从 AMQP headers 提取 trace context
5. HTTP (Gateway):
   - 使用 otelhttp 中间件

【配置】
每个服务 config.yaml 添加:
  otel:
    endpoint: "localhost:4317"     # OTEL Collector gRPC
    sample_rate: 1.0              # 开发环境采样率

【错误处理】
- OTEL Collector 不可用 → log.Warn + 服务正常启动(tracing 降级)
- 不应因 tracing 错误影响业务逻辑

【数据库操作】
- 无

【依赖的外部服务/库】
- go.opentelemetry.io/otel
- go.opentelemetry.io/otel/sdk/trace
- go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc
- go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc
- go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp

【验证标准】
1. Gateway 请求产生的 traceID 在所有下游服务中一致
2. Jaeger UI 中可看到完整调用链
3. MQ 消费者的 span 与发布者的 span 在同一 trace 中
4. OTEL Collector 不可用时服务正常运行

【禁止事项】
- 禁止生产环境 100% 采样(太贵)
- 禁止 tracing 初始化失败导致服务启动失败
- 禁止忘记在 MQ 中传播 trace context
- 禁止每个服务自己实现 trace 初始化(必须用 pkg/trace 公共逻辑)
```

---

### 任务 P7-3: CI/CD per-service build

**目标**：实现每个微服务的独立 CI/CD 构建流程。

**文件路径**：
- `Makefile` (修改，添加 per-service targets)
- `.github/workflows/service-build.yml` (新建)
- `deploy/docker/Dockerfile.service` (新建，通用 Dockerfile 模板)

**单体参考**：无

#### PROMPT

```
你是一个 DevOps 工程师，正在为 MakeJob 项目实现 CI/CD 基础设施。

【任务】实现每个微服务独立的 CI/CD 构建流程，支持变更检测和独立部署。

【目标】当某个服务的代码变更时，只构建和部署该服务，而非全量构建。

【需要创建/修改的文件】
- Makefile (添加 per-service build/test/docker targets)
- .github/workflows/service-build.yml (新建)
- deploy/docker/Dockerfile.service (新建)

【实现步骤】
1. Makefile 添加:
   ```makefile
   SERVICES := user question interview plan growth companion community membership admin ai_gateway coderunner rag realtime learning_archive gateway

   # 构建单个服务
   build-%:
     go build -o bin/$* ./app/$*/cmd/server/

   # 测试单个服务
   test-%:
     go test ./app/$*/...

   # Docker 构建单个服务
   docker-%:
     docker build --build-arg SERVICE=$* -f deploy/docker/Dockerfile.service -t makejob/$*:latest .

   # 构建所有服务
   build-all: $(addprefix build-,$(SERVICES))

   # 测试所有服务
   test-all: $(addprefix test-,$(SERVICES))
   ```

2. deploy/docker/Dockerfile.service:
   ```dockerfile
   FROM golang:1.22-alpine AS builder
   ARG SERVICE
   WORKDIR /app
   COPY go.mod go.sum ./
   RUN go mod download
   COPY . .
   RUN CGO_ENABLED=0 go build -o /server ./app/${SERVICE}/cmd/server/

   FROM alpine:3.19
   RUN apk --no-cache add ca-certificates tzdata
   COPY --from=builder /server /server
   COPY app/${SERVICE}/configs/ /configs/
   ENTRYPOINT ["/server", "-conf", "/configs/config.yaml"]
   ```

3. .github/workflows/service-build.yml:
   - trigger: push/PR to main
   - 变更检测: 使用 paths filter 或 dorny/paths-filter action
   - Matrix: 检测到变更的服务列表
   - Steps per service:
     a. go test ./app/{service}/...
     b. go build ./app/{service}/cmd/server/
     c. docker build (仅 main 分支 push)
     d. docker push to registry (仅 main 分支 push)

4. 变更检测逻辑:
   - app/{service}/ 变更 → 构建该服务
   - pkg/ 变更 → 构建所有服务
   - go.mod/go.sum 变更 → 构建所有服务
   - api/ (proto) 变更 → 构建所有服务

【错误处理】
- 单服务构建失败不影响其他服务的构建
- Docker push 失败 → retry 1 次

【数据库操作】
- 无

【依赖的外部服务/库】
- GitHub Actions
- Docker
- Go 1.22+

【验证标准】
1. make build-user 成功构建 user 服务
2. make docker-user 成功构建 Docker 镜像
3. 只修改 app/user/ 时，CI 只构建 user 服务
4. 修改 pkg/ 时，CI 构建所有服务
5. 各服务 Docker 镜像可独立运行

【禁止事项】
- 禁止全量构建(必须支持增量)
- 禁止 CI 中硬编码服务列表(应动态检测)
- 禁止 Dockerfile 不使用多阶段构建
- 禁止 Docker 镜像包含源代码
- 禁止不设置 CGO_ENABLED=0 (alpine 需要静态链接)
```
