package data

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	ragv1 "makejob/api/makejob/rag/v1"
	"makejob/app/realtime/internal/biz"
	"makejob/app/realtime/internal/conf"
	"makejob/pkg/middleware"
)

// ragClient 通过 gRPC 调用 RAG 检索服务
type ragClient struct {
	client ragv1.RAGServiceClient
	conn   *grpc.ClientConn
}

// NewRAGClient 创建 RAG 检索服务客户端，返回接口实现和可选的关闭函数
func NewRAGClient(cfg *conf.DependentServices) (biz.RAGClient, *ragClient, error) {
	conn, err := grpc.NewClient(cfg.RAGAddr, middleware.CommonDialOptions()...)
	if err != nil {
		return nil, nil, fmt.Errorf("连接 RAG 服务失败 (%s): %w", cfg.RAGAddr, err)
	}
	c := &ragClient{
		client: ragv1.NewRAGServiceClient(conn),
		conn:   conn,
	}
	return c, c, nil
}

// Close 关闭 gRPC 连接
func (c *ragClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Retrieve 根据查询文本检索相关文档
func (c *ragClient) Retrieve(ctx context.Context, query string, topK int32) ([]*biz.RAGDocument, error) {
	resp, err := c.client.Retrieve(ctx, &ragv1.RetrieveRequest{
		Query: query,
		TopK:  topK,
	})
	if err != nil {
		return nil, fmt.Errorf("RAG Retrieve gRPC 调用失败: %w", err)
	}

	docs := make([]*biz.RAGDocument, len(resp.Documents))
	for i, doc := range resp.Documents {
		docs[i] = &biz.RAGDocument{
			ID:      doc.Id,
			Content: doc.Content,
			Score:   float64(doc.Score),
		}
	}
	return docs, nil
}
