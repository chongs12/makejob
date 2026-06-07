package data

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	archivev1 "makejob/api/makejob/learning_archive/v1"
	interviewv1 "makejob/api/makejob/interview/v1"
	planv1 "makejob/api/makejob/plan/v1"
	questionv1 "makejob/api/makejob/question/v1"
	"makejob/app/growth/internal/biz"
	"makejob/app/growth/internal/conf"
)

// --- QuestionClient 实现 ---

// questionClient 实现 biz.QuestionClient 接口，通过 gRPC 调用题目服务
type questionClient struct {
	client questionv1.QuestionServiceClient
	conn   *grpc.ClientConn
}

// NewQuestionClient 创建题目服务客户端
func NewQuestionClient(cfg *conf.DependentServices) (biz.QuestionClient, error) {
	conn, err := grpc.Dial(cfg.QuestionAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("连接题目服务失败(%s): %w", cfg.QuestionAddr, err)
	}
	return &questionClient{
		client: questionv1.NewQuestionServiceClient(conn),
		conn:   conn,
	}, nil
}

// Close 关闭 gRPC 连接
func (c *questionClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// GetUserPracticeStats 获取用户练习统计
func (c *questionClient) GetUserPracticeStats(ctx context.Context, userID uint64) (*biz.PracticeStats, error) {
	resp, err := c.client.GetUserPracticeStats(ctx, &questionv1.UserIDRequest{
		UserId: userID,
	})
	if err != nil {
		return nil, fmt.Errorf("GetUserPracticeStats 调用失败: %w", err)
	}
	return &biz.PracticeStats{
		TotalDone:   resp.GetTotalAnswered(),
		CorrectRate: int32(resp.GetAccuracy() * 100),
		StreakDays:  resp.GetStreakDays(),
	}, nil
}

// ListQuestionSets 获取题目集列表
func (c *questionClient) ListQuestionSets(ctx context.Context, industryCode string) ([]*biz.QuestionSetBrief, error) {
	resp, err := c.client.ListQuestionSets(ctx, &questionv1.ListQuestionSetsRequest{
		IndustryCode: industryCode,
	})
	if err != nil {
		return nil, fmt.Errorf("ListQuestionSets 调用失败: %w", err)
	}
	sets := make([]*biz.QuestionSetBrief, 0, len(resp.GetItems()))
	for _, item := range resp.GetItems() {
		sets = append(sets, &biz.QuestionSetBrief{
			ID:            item.GetId(),
			Title:         item.GetTitle(),
			Description:   item.GetDescription(),
			QuestionCount: item.GetQuestionCount(),
			Difficulty:    item.GetDifficulty(),
		})
	}
	return sets, nil
}

// --- PlanClient 实现 ---

// planClient 实现 biz.PlanClient 接口，通过 gRPC 调用计划服务
type planClient struct {
	client planv1.PlanServiceClient
	conn   *grpc.ClientConn
}

// NewPlanClient 创建计划服务客户端
func NewPlanClient(cfg *conf.DependentServices) (biz.PlanClient, error) {
	conn, err := grpc.Dial(cfg.PlanAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("连接计划服务失败(%s): %w", cfg.PlanAddr, err)
	}
	return &planClient{
		client: planv1.NewPlanServiceClient(conn),
		conn:   conn,
	}, nil
}

// Close 关闭 gRPC 连接
func (c *planClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// GetCurrentPlan 获取用户当前计划
func (c *planClient) GetCurrentPlan(ctx context.Context, userID uint64) (*biz.PlanInfo, error) {
	resp, err := c.client.GetCurrentPlan(ctx, &planv1.GetCurrentPlanRequest{
		UserId: userID,
	})
	if err != nil {
		return nil, fmt.Errorf("GetCurrentPlan 调用失败: %w", err)
	}
	return &biz.PlanInfo{
		Title:          resp.GetTitle(),
		Progress:       float64(resp.GetProgress()),
		CompletedTasks: resp.GetCompletedTasks(),
		TotalTasks:     resp.GetTotalTasks(),
	}, nil
}

// --- LearningArchiveClient 实现 ---

// archiveClient 实现 biz.LearningArchiveClient 接口，通过 gRPC 调用学习档案服务
type archiveClient struct {
	client archivev1.LearningArchiveServiceClient
	conn   *grpc.ClientConn
}

// NewArchiveClient 创建学习档案服务客户端
func NewArchiveClient(cfg *conf.DependentServices) (biz.LearningArchiveClient, error) {
	conn, err := grpc.Dial(cfg.LearningArchiveAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("连接学习档案服务失败(%s): %w", cfg.LearningArchiveAddr, err)
	}
	return &archiveClient{
		client: archivev1.NewLearningArchiveServiceClient(conn),
		conn:   conn,
	}, nil
}

// Close 关闭 gRPC 连接
func (c *archiveClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// GetWeakTopics 获取用户薄弱知识点列表
func (c *archiveClient) GetWeakTopics(ctx context.Context, userID uint64) ([]string, error) {
	resp, err := c.client.GetWeakTopics(ctx, &archivev1.UserIDRequest{
		UserId: userID,
	})
	if err != nil {
		return nil, fmt.Errorf("GetWeakTopics 调用失败: %w", err)
	}
	return resp.GetTopics(), nil
}

// GetFocusSignals 获取用户学习焦点信号
func (c *archiveClient) GetFocusSignals(ctx context.Context, userID uint64) ([]*biz.FocusSignal, error) {
	resp, err := c.client.GetFocusSignals(ctx, &archivev1.UserIDRequest{
		UserId: userID,
	})
	if err != nil {
		return nil, fmt.Errorf("GetFocusSignals 调用失败: %w", err)
	}
	signals := make([]*biz.FocusSignal, 0, len(resp.GetSignals()))
	for _, s := range resp.GetSignals() {
		signals = append(signals, &biz.FocusSignal{
			Topic:  s.GetTopic(),
			Weight: s.GetWeight(),
			Source: s.GetSource(),
		})
	}
	return signals, nil
}

// --- InterviewClient 实现 ---

// interviewClient 实现 biz.InterviewClient 接口，通过 gRPC 调用面试服务
type interviewClient struct {
	client interviewv1.InterviewServiceClient
	conn   *grpc.ClientConn
}

// NewInterviewClient 创建面试服务客户端
func NewInterviewClient(cfg *conf.DependentServices) (biz.InterviewClient, error) {
	conn, err := grpc.Dial(cfg.InterviewAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("连接面试服务失败(%s): %w", cfg.InterviewAddr, err)
	}
	return &interviewClient{
		client: interviewv1.NewInterviewServiceClient(conn),
		conn:   conn,
	}, nil
}

// Close 关闭 gRPC 连接
func (c *interviewClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// GetInterviewStats 获取用户面试统计
func (c *interviewClient) GetInterviewStats(ctx context.Context, userID uint64) (*biz.InterviewStats, error) {
	resp, err := c.client.GetInterviewStats(ctx, &interviewv1.UserIDRequest{
		UserId: userID,
	})
	if err != nil {
		return nil, fmt.Errorf("GetInterviewStats 调用失败: %w", err)
	}
	return &biz.InterviewStats{
		TotalInterviews: resp.GetTotalInterviews(),
		AvgScore:        resp.GetAvgScore(),
		LatestScore:     0, // 当前 proto 未提供 LatestScore 字段
	}, nil
}
