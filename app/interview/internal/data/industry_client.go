package data

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	industryv1 "makejob/api/makejob/industry/v1"
	"makejob/app/interview/internal/biz"
	"makejob/app/interview/internal/conf"
)

const industryCacheTTL = 30 * time.Minute

type cachedIndustry struct {
	industry  *biz.Industry
	cachedAt  time.Time
}

// industryClient 实现 biz.IndustryClient 接口
// 通过 gRPC 调用 Industry 服务，带本地缓存
type industryClient struct {
	client industryv1.IndustryServiceClient
	conn   *grpc.ClientConn
	cache  sync.Map // code -> *cachedIndustry
}

// NewIndustryClient 创建行业客户端（由 Wire 调用）
func NewIndustryClient(cfg *conf.Industry) (biz.IndustryClient, error) {
	conn, err := grpc.Dial(cfg.ServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to dial Industry service at %s: %w", cfg.ServiceAddr, err)
	}
	return &industryClient{
		client: industryv1.NewIndustryServiceClient(conn),
		conn:   conn,
	}, nil
}

// Close 关闭 gRPC 连接
func (c *industryClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *industryClient) GetIndustry(ctx context.Context, code string) (*biz.Industry, error) {
	// 先查缓存
	if v, ok := c.cache.Load(code); ok {
		ci := v.(*cachedIndustry)
		if time.Since(ci.cachedAt) < industryCacheTTL {
			return ci.industry, nil
		}
		// 缓存过期，删除
		c.cache.Delete(code)
	}

	// gRPC 调用
	resp, err := c.client.GetIndustry(ctx, &industryv1.GetIndustryRequest{
		Code: code,
	})
	if err != nil {
		return nil, fmt.Errorf("GetIndustry gRPC call failed for code=%s: %w", code, err)
	}

	industry := &biz.Industry{
		Code: resp.Code,
		Name: resp.Name,
	}

	// 写入缓存
	c.cache.Store(code, &cachedIndustry{
		industry: industry,
		cachedAt: time.Now(),
	})

	return industry, nil
}
