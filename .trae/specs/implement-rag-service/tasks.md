# Tasks

- [x] Task 1: 修改配置结构 conf.go - 添加 RAG 相关配置
  - [x] SubTask 1.1: 添加 RAG 结构体（MilvusAddr, CollectionName, ArkAPIKey, ArkBaseURL, EmbedModel, TopK）
  - [x] SubTask 1.2: 更新 Bootstrap 结构体包含 RAG 字段
  - [x] SubTask 1.3: 设置默认值

- [x] Task 2: 创建领域错误定义 biz/errors.go
  - [x] SubTask 2.1: 定义 ErrRAGConnectionFailed (ServiceUnavailable)
  - [x] SubTask 2.2: 定义 ErrEmbeddingFailed (BadGateway)
  - [x] SubTask 2.3: 定义 ErrNoResults (NotFound)
  - [x] SubTask 2.4: 定义 ErrInvalidQuery (BadRequest)

- [x] Task 3: 重写领域层 biz/rag.go - 定义接口和 UseCase
  - [x] SubTask 3.1: 定义 Document 实体
  - [x] SubTask 3.2: 定义 Embedder 接口
  - [x] SubTask 3.3: 定义 VectorStore 接口（Search, Upsert, Delete）
  - [x] SubTask 3.4: 实现 RetrieveUseCase
  - [x] SubTask 3.5: 实现 IndexUseCase
  - [x] SubTask 3.6: 实现 SyncHandler

- [x] Task 4: 创建 Milvus 客户端 data/milvus_client.go
  - [x] SubTask 4.1: 实现 milvusClient 结构体
  - [x] SubTask 4.2: 实现 Embedder 接口（EmbedStrings）
  - [x] SubTask 4.3: 实现 VectorStore.Search 方法
  - [x] SubTask 4.4: 实现 VectorStore.Upsert 方法
  - [x] SubTask 4.5: 实现 VectorStore.Delete 方法

- [x] Task 5: 更新 data 层 data.go - 简化为无数据库版本
  - [x] SubTask 5.1: 简化 Data 结构体
  - [x] SubTask 5.2: 更新 NewData 函数

- [x] Task 6: 重写 service 层 service/rag.go
  - [x] SubTask 6.1: 实现 Retrieve 方法
  - [x] SubTask 6.2: 实现 IndexQuestions 方法
  - [x] SubTask 6.3: 添加 proto 转换函数

- [x] Task 7: 更新 server 层 server/grpc.go
  - [x] SubTask 7.1: 移除 JWT 认证（内部服务）
  - [x] SubTask 7.2: 保留 Recovery + Logging 中间件

- [x] Task 8: 创建 MQ Consumer server/mq.go
  - [x] SubTask 8.1: 实现 TaskHandler 接口
  - [x] SubTask 8.2: 实现 HandleQuestionChanged 方法
  - [x] SubTask 8.3: 注册 consumer

- [x] Task 9: 更新 main.go 启动入口
  - [x] SubTask 9.1: 更新 wireApp 函数
  - [x] SubTask 9.2: 创建 MilvusClient
  - [x] SubTask 9.3: 启动 MQ consumer

- [x] Task 10: 更新配置文件 configs/config.yaml
  - [x] SubTask 10.1: 添加 rag 配置段

# Task Dependencies
- Task 1 无依赖
- Task 2 无依赖
- Task 3 依赖 Task 1, 2
- Task 4 依赖 Task 1, 3
- Task 5 依赖 Task 1
- Task 6 依赖 Task 3, 4
- Task 7 依赖 Task 1
- Task 8 依赖 Task 3, 4
- Task 9 依赖 Task 3, 4, 5, 6, 7, 8
- Task 10 依赖 Task 1
