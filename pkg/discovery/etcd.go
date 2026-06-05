package discovery

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// Registry etcd 服务注册
type Registry struct {
	client     *clientv3.Client
	leaseID    clientv3.LeaseID
	ttl        int64
	mu         sync.Mutex    // 保护 leaseID
	cancelFunc context.CancelFunc // 用于取消 keepalive goroutine
}

// NewRegistry 创建 etcd 注册中心
func NewRegistry(endpoints []string, ttl int64) (*Registry, error) {
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to etcd: %w", err)
	}

	return &Registry{
		client: client,
		ttl:    ttl,
	}, nil
}

// Register 注册服务实例
func (r *Registry) Register(ctx context.Context, serviceName, addr string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 创建租约
	resp, err := r.client.Grant(ctx, r.ttl)
	if err != nil {
		return fmt.Errorf("failed to create lease: %w", err)
	}
	r.leaseID = resp.ID

	// 注册服务
	key := fmt.Sprintf("/makejob/services/%s/%s", serviceName, addr)
	_, err = r.client.Put(ctx, key, addr, clientv3.WithLease(r.leaseID))
	if err != nil {
		return fmt.Errorf("failed to register service: %w", err)
	}

	// 使用可取消的 context 进行 keepalive
	keepAliveCtx, cancel := context.WithCancel(context.Background())
	r.cancelFunc = cancel

	ch, err := r.client.KeepAlive(keepAliveCtx, r.leaseID)
	if err != nil {
		cancel()
		return fmt.Errorf("failed to keep alive: %w", err)
	}

	// 消费 keepalive 响应（goroutine 随 context 取消而退出）
	go func() {
		for {
			select {
			case <-keepAliveCtx.Done():
				return
			case _, ok := <-ch:
				if !ok {
					return
				}
			}
		}
	}()

	return nil
}

// Deregister 注销服务
func (r *Registry) Deregister(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 取消 keepalive goroutine
	if r.cancelFunc != nil {
		r.cancelFunc()
	}

	if r.leaseID != 0 {
		_, err := r.client.Revoke(ctx, r.leaseID)
		if err != nil {
			return err
		}
	}
	return r.client.Close()
}

// GetServiceAddr 获取服务地址（辅助函数）
func GetServiceAddr(port int) string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return fmt.Sprintf("localhost:%d", port)
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return fmt.Sprintf("%s:%d", ipnet.IP.String(), port)
			}
		}
	}
	return fmt.Sprintf("localhost:%d", port)
}
