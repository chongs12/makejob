package data

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	companionv1 "makejob/api/makejob/companion/v1"
	"makejob/app/interview/internal/biz"
	"makejob/app/interview/internal/conf"
	"makejob/pkg/auth"
	"makejob/pkg/middleware"
)

// companionClient 实现 biz.TTSPrewarmClient 接口。
// 通过 gRPC 调用 Companion 服务的 SynthesizeSpeech 预热并缓存 TTS，注入内部服务 Token 绕过用户鉴权。
type companionClient struct {
	client companionv1.CompanionServiceClient
	conn   *grpc.ClientConn
}

// NewCompanionClient 创建 companion 客户端，注入内部服务 Token。
func NewCompanionClient(cfg *conf.Companion, serviceToken string) (biz.TTSPrewarmClient, error) {
	opts := append(middleware.CommonDialOptions(),
		grpc.WithUnaryInterceptor(auth.ServiceAuthInterceptor(serviceToken)),
	)
	conn, err := grpc.Dial(cfg.ServiceAddr, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to dial Companion service at %s: %w", cfg.ServiceAddr, err)
	}
	return &companionClient{
		client: companionv1.NewCompanionServiceClient(conn),
		conn:   conn,
	}, nil
}

// Close 关闭 gRPC 连接。
func (c *companionClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// PrewarmTTS 触发 companion 合成指定文本并写入其进程内缓存，voice 传空由 companion 使用默认音色。
func (c *companionClient) PrewarmTTS(ctx context.Context, text string) error {
	_, err := c.client.SynthesizeSpeech(ctx, &companionv1.SynthesizeSpeechRequest{
		Text:  text,
		Voice: "",
	})
	if err != nil {
		return fmt.Errorf("PrewarmTTS gRPC call failed: %w", err)
	}
	return nil
}
