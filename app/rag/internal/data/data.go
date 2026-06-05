package data

// RAG 微服务不需要关系型数据库，仅依赖 Milvus 向量库和 Ark Embedding API。
// data 层通过 milvusClient 统一提供 Embedder 和 VectorStore 实现。
