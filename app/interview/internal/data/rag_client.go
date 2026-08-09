package data

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	ragv1 "makejob/api/makejob/rag/v1"
	"makejob/app/interview/internal/biz"
	"makejob/app/interview/internal/conf"
	"makejob/pkg/middleware"
)

// ragClient 实现 biz.RAGClient 接口，通过 gRPC 调用 RAG 检索服务
type ragClient struct {
	client ragv1.RAGServiceClient
	conn   *grpc.ClientConn
}

// NewRAGClient 创建 RAG 检索服务客户端
func NewRAGClient(cfg *conf.RAG) (biz.RAGClient, error) {
	conn, err := grpc.NewClient(cfg.ServiceAddr, middleware.CommonDialOptions()...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RAG service at %s: %w", cfg.ServiceAddr, err)
	}
	return &ragClient{
		client: ragv1.NewRAGServiceClient(conn),
		conn:   conn,
	}, nil
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
		return nil, fmt.Errorf("RAG Retrieve gRPC call failed: %w", err)
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
