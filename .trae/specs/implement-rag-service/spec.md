# RAG 服务完整实现 Spec (P1-2~P1-4)

## Why
RAG 服务是微服务架构中的基础设施服务，负责向量检索能力。当前只有空壳骨架，需要实现 Retrieve、IndexQuestions 三个 RPC 和 MQ Consumer，封装 Milvus 和 Embedding API，供 Interview 和其他服务调用。

## What Changes
- P1-2: 实现 Retrieve RPC（语义检索）
  - 实现 Embedder 接口调用 Volcengine Ark Embedding API
  - 实现 VectorStore 接口调用 Milvus 搜索
  - 实现 RetrieveUseCase 业务逻辑
- P1-3: 实现 IndexQuestions RPC（批量索引）
  - 实现 VectorStore.Upsert 方法
  - 实现 IndexUseCase 批量处理逻辑
- P1-4: 实现 MQ Consumer（题目变更同步）
  - 实现 VectorStore.Delete 方法
  - 实现 SyncHandler 处理题目变更事件
  - 实现 MQ Consumer 注册

## Impact
- Affected specs: P1-2, P1-3, P1-4 RAG Service
- Affected code:
  - `app/rag/internal/conf/conf.go` (修改)
  - `app/rag/internal/biz/rag.go` (重写)
  - `app/rag/internal/biz/errors.go` (新建)
  - `app/rag/internal/data/data.go` (修改)
  - `app/rag/internal/data/milvus_client.go` (新建)
  - `app/rag/internal/service/rag.go` (重写)
  - `app/rag/internal/server/grpc.go` (修改)
  - `app/rag/internal/server/mq.go` (新建)
  - `app/rag/cmd/server/main.go` (修改)

## ADDED Requirements

### Requirement: 语义检索 (Retrieve)
系统 SHALL 通过 Embedding 向量化查询并在 Milvus 中搜索最相关的文档。

#### Scenario: 成功检索
- **WHEN** 调用 Retrieve(query="Go 并发编程", top_k=5)
- **THEN** 返回相关文档列表，每条包含 id, content, score, metadata

#### Scenario: 空查询
- **WHEN** 调用 Retrieve(query="", top_k=5)
- **THEN** 返回 INVALID_QUERY 错误

### Requirement: 批量索引 (IndexQuestions)
系统 SHALL 支持批量将题目内容向量化并存入 Milvus。

#### Scenario: 成功索引
- **WHEN** 传入 3 条 items
- **THEN** 返回 indexed_count=3, failed_count=0

#### Scenario: 部分失败
- **WHEN** 某批 Embedding 失败
- **THEN** 返回 failed_ids，继续处理其他批次

### Requirement: MQ 消费 (question.changed)
系统 SHALL 监听题目变更事件并同步更新 Milvus 索引。

#### Scenario: 创建题目
- **WHEN** 收到 action="create" 消息
- **THEN** Embedding 内容并 Upsert 到 Milvus

#### Scenario: 删除题目
- **WHEN** 收到 action="delete" 消息
- **THEN** 从 Milvus 删除对应文档

## 全局规范遵循
- 错误处理：使用 kratos errors 包
- 构造函数：NewXxx(deps...) 模式
- 禁止全局变量和 init() 函数
- 使用 context 传播
- 使用中文注释
