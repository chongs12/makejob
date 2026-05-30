// Package rag 提供基于向量检索的RAG（Retrieval-Augmented Generation）能力。
// 通过Milvus向量数据库存储文档向量，支持语义检索增强面试出题质量。
package rag

import "context"

// Config RAG系统配置
type Config struct {
	MilvusAddr     string // Milvus地址，如localhost:19530
	MilvusUser     string // Milvus用户名
	MilvusPassword string // Milvus密码
	Collection     string // Collection名称
	ArkAPIKey      string // 火山引擎API Key（用于Embedding）
	ArkBaseURL     string // Ark API端点
	EmbedModel     string // Embedding模型ID（如doubao-embedding-large-text-240915）
	TopK           int    // 默认返回数量
}

// Document 检索到的文档
type Document struct {
	ID       string         // 文档ID
	Content  string         // 文档内容
	Score    float64        // 相似度分数
	MetaData map[string]any // 元数据
}

// IndexDocument 待索引的文档
type IndexDocument struct {
	ID       string         // 文档ID
	Content  string         // 文档内容（用于向量化）
	MetaData map[string]any // 元数据
}

// Indexer 向量索引写入接口
type Indexer interface {
	// Index 批量写入文档到向量库，返回分配的ID列表
	Index(ctx context.Context, docs []IndexDocument) (ids []string, err error)
	// Delete 根据ID列表删除文档
	Delete(ctx context.Context, ids []string) error
}

// Retriever 语义检索接口
type Retriever interface {
	// Retrieve 根据查询语义检索相关文档，按相似度降序返回
	Retrieve(ctx context.Context, query string, topK int) ([]Document, error)
}
