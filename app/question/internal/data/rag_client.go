package data

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	ragv1 "makejob/api/makejob/rag/v1"
	"makejob/app/question/internal/conf"
	"makejob/pkg/middleware"
)

// RAGClient RAG 服务客户端接口
type RAGClient interface {
	IndexQuestions(ctx context.Context, items []*ragv1.IndexItem) (int32, error)
	DeleteIndex(ctx context.Context, ids []string) (int32, error)
	Close() error
}

type ragClient struct {
	client ragv1.RAGServiceClient
	conn   *grpc.ClientConn
}

// NewRAGClient 创建 RAG 服务客户端
func NewRAGClient(cfg *conf.AI) (RAGClient, error) {
	conn, err := grpc.Dial(cfg.RAGAddr, middleware.CommonDialOptions()...)
	if err != nil {
		return nil, fmt.Errorf("failed to dial RAG service at %s: %w", cfg.RAGAddr, err)
	}
	return &ragClient{
		client: ragv1.NewRAGServiceClient(conn),
		conn:   conn,
	}, nil
}

func (c *ragClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// IndexQuestions 调用 RAG 服务索引题目
func (c *ragClient) IndexQuestions(ctx context.Context, items []*ragv1.IndexItem) (int32, error) {
	resp, err := c.client.IndexQuestions(ctx, &ragv1.IndexQuestionsRequest{
		Items: items,
	})
	if err != nil {
		return 0, fmt.Errorf("RAG IndexQuestions failed: %w", err)
	}
	return resp.GetIndexedCount(), nil
}

// DeleteIndex 调用 RAG 服务删除索引
func (c *ragClient) DeleteIndex(ctx context.Context, ids []string) (int32, error) {
	resp, err := c.client.DeleteIndex(ctx, &ragv1.DeleteIndexRequest{
		Ids: ids,
	})
	if err != nil {
		return 0, fmt.Errorf("RAG DeleteIndex failed: %w", err)
	}
	return resp.GetDeletedCount(), nil
}
