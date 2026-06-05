package biz

import (
	"github.com/go-kratos/kratos/v2/errors"
)

// ErrRAGConnectionFailed 向量数据库连接失败
var ErrRAGConnectionFailed = errors.ServiceUnavailable("RAG_CONNECTION_FAILED", "向量数据库连接失败")

// ErrEmbeddingFailed 文本向量化失败
var ErrEmbeddingFailed = errors.New(502, "EMBEDDING_FAILED", "文本向量化失败")

// ErrNoResults 未找到相关文档
var ErrNoResults = errors.NotFound("NO_RESULTS", "未找到相关文档")

// ErrInvalidQuery 查询文本不能为空
var ErrInvalidQuery = errors.BadRequest("INVALID_QUERY", "查询文本不能为空")
