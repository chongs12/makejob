# Checklist

## P1-2: Retrieve RPC
- [x] conf.go 包含 RAG 配置字段（MilvusAddr, CollectionName, ArkAPIKey, ArkBaseURL, EmbedModel, TopK）
- [x] biz/errors.go 定义四个领域错误
- [x] biz/rag.go 定义 Embedder 接口
- [x] biz/rag.go 定义 VectorStore.Search 接口
- [x] biz/rag.go 实现 RetrieveUseCase.Retrieve 方法
- [x] data/milvus_client.go 实现 Embedder.EmbedStrings 方法
- [x] data/milvus_client.go 实现 VectorStore.Search 方法
- [x] service/rag.go 实现 Retrieve 方法
- [x] 空 query 返回 INVALID_QUERY 错误
- [x] go build 编译通过

## P1-3: IndexQuestions RPC
- [x] biz/rag.go 定义 VectorStore.Upsert 接口
- [x] biz/rag.go 实现 IndexUseCase.IndexQuestions 方法
- [x] biz/rag.go 实现批量处理（max 16 条/批）
- [x] data/milvus_client.go 实现 VectorStore.Upsert 方法
- [x] service/rag.go 实现 IndexQuestions 方法
- [x] 部分失败返回 failed_ids
- [x] go build 编译通过

## P1-4: MQ Consumer
- [x] biz/rag.go 定义 VectorStore.Delete 接口
- [x] biz/rag.go 实现 SyncHandler.HandleQuestionChanged 方法
- [x] data/milvus_client.go 实现 VectorStore.Delete 方法
- [x] server/mq.go 实现 TaskHandler 接口
- [x] server/mq.go 注册 consumer（makejob.tasks.rag.sync.question）
- [x] main.go 启动 MQ consumer
- [x] go build 编译通过

## 通用
- [x] server/grpc.go 移除 JWT 认证
- [x] config.yaml 包含 rag 配置段
- [x] data/data.go 简化为无数据库版本
- [x] go vet 通过
