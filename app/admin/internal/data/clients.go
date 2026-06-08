package data

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	interviewv1 "makejob/api/makejob/interview/v1"
	questionv1 "makejob/api/makejob/question/v1"
	sharedv1 "makejob/api/makejob/shared/v1"
	userv1 "makejob/api/makejob/user/v1"
	"makejob/app/admin/internal/biz"
	"makejob/pkg/auth"
)

// ==================== 用户服务 gRPC 客户端 ====================

// userClient 实现 biz.UserClient，通过 gRPC 调用 user 微服务的 admin RPC
type userClient struct {
	client userv1.UserServiceClient
	logger log.Logger
}

// NewUserClient 创建用户服务 gRPC 客户端
func NewUserClient(conn *grpc.ClientConn, logger log.Logger) biz.UserClient {
	return &userClient{
		client: userv1.NewUserServiceClient(conn),
		logger: logger,
	}
}

// ListUsers 调用 user 服务的 AdminListUsers RPC 获取用户列表
func (c *userClient) ListUsers(ctx context.Context, page, pageSize int32) ([]*biz.UserRecord, int64, error) {
	resp, err := c.client.AdminListUsers(forwardServiceAuth(ctx), &userv1.AdminListUsersRequest{
		Page: &sharedv1.PageParam{Page: page, PageSize: pageSize},
	})
	if err != nil {
		return nil, 0, err
	}
	users := make([]*biz.UserRecord, len(resp.Users))
	for i, u := range resp.Users {
		users[i] = &biz.UserRecord{
			ID:              u.Id,
			Username:        u.Username,
			Email:           u.Email,
			Role:            u.Role,
			Avatar:          u.Avatar,
			MembershipLevel: u.MembershipLevel,
			MembershipType:  u.MembershipType,
			IsDisabled:      u.IsDisabled,
			CreatedAt:       protoTimeToTime(u.CreatedAt),
		}
		if u.MembershipExpireAt != nil {
			t := protoTimeToTime(u.MembershipExpireAt)
			users[i].MembershipExpireAt = &t
		}
	}
	total := resp.PageResult.GetTotal()
	return users, total, nil
}

// UpdateUserRole 调用 user 服务的 AdminUpdateUserRole RPC 更新用户角色
func (c *userClient) UpdateUserRole(ctx context.Context, userID uint64, role string) error {
	_, err := c.client.AdminUpdateUserRole(forwardServiceAuth(ctx), &userv1.AdminUpdateUserRoleRequest{
		UserId: userID,
		Role:   role,
	})
	return err
}

// BanUser 调用 user 服务的 AdminBanUser RPC 封禁用户
func (c *userClient) BanUser(ctx context.Context, userID uint64) error {
	_, err := c.client.AdminBanUser(forwardServiceAuth(ctx), &userv1.AdminBanUserRequest{
		UserId: userID,
	})
	return err
}

// GetUserStats 调用 user 服务的 GetAdminUserStats RPC 获取用户统计
func (c *userClient) GetUserStats(ctx context.Context) (totalUsers, proMembers, newUsersToday, todayActiveUsers int64, err error) {
	resp, err := c.client.GetAdminUserStats(forwardServiceAuth(ctx), &userv1.GetAdminUserStatsRequest{})
	if err != nil {
		return 0, 0, 0, 0, err
	}
	return resp.TotalUsers, resp.ProMembers, resp.NewUsersToday, resp.TodayActiveUsers, nil
}

// ==================== 题目服务 gRPC 客户端 ====================

// industryCodeLookup 定义按行业编码查询权威行业记录的最小依赖。
type industryCodeLookup interface {
	GetIndustryByCode(ctx context.Context, code string) (*biz.IndustryRecord, error)
}

// questionClient 实现 biz.QuestionClient，通过 gRPC 调用 question 微服务的 admin RPC
type questionClient struct {
	client       questionv1.QuestionServiceClient
	industryRepo industryCodeLookup
	logger       log.Logger
}

// NewQuestionClient 创建题目服务 gRPC 客户端
func NewQuestionClient(conn *grpc.ClientConn, industryRepo industryCodeLookup, logger log.Logger) biz.QuestionClient {
	return &questionClient{
		client:       questionv1.NewQuestionServiceClient(conn),
		industryRepo: industryRepo,
		logger:       logger,
	}
}

// ListQuestions 调用 question 服务查询题目列表；当需要分类/行业过滤时，在 Admin 侧补齐兼容筛选。
func (c *questionClient) ListQuestions(ctx context.Context, page, pageSize int32, keyword, difficulty string, categoryID uint64, industryCode string) ([]*biz.QuestionRecord, int64, error) {
	if categoryID == 0 && industryCode == "" {
		return c.listQuestionsPage(ctx, page, pageSize, keyword, difficulty)
	}

	filtered, err := c.listFilteredQuestions(ctx, keyword, difficulty, categoryID, industryCode)
	if err != nil {
		return nil, 0, err
	}

	total := int64(len(filtered))
	start := int((page - 1) * pageSize)
	if start >= len(filtered) {
		return []*biz.QuestionRecord{}, total, nil
	}
	end := start + int(pageSize)
	if end > len(filtered) {
		end = len(filtered)
	}
	return filtered[start:end], total, nil
}

// CreateQuestion 调用 question 服务的 AdminCreateQuestion RPC 创建题目
func (c *questionClient) CreateQuestion(ctx context.Context, q *biz.QuestionRecord) error {
	resp, err := c.client.AdminCreateQuestion(forwardServiceAuth(ctx), &questionv1.AdminCreateQuestionRequest{
		CategoryId:         q.CategoryID,
		IndustryId:         q.IndustryID,
		Type:               q.Type,
		Difficulty:         q.Difficulty,
		Title:              q.Title,
		Content:            q.Content,
		OptionsJson:        q.OptionsJSON,
		Answer:             q.Answer,
		Explanation:        q.Explanation,
		SolutionJson:       q.SolutionJSON,
		JudgeConfigJson:    q.JudgeConfigJSON,
		AnswerTemplateJson: q.AnswerTemplateJSON,
		Tags:               q.Tags,
		IsActive:           q.IsActive,
	})
	if err != nil {
		return err
	}
	// 回写下游生成的 ID
	q.ID = resp.Id
	return nil
}

// UpdateQuestion 调用 question 服务的 AdminUpdateQuestion RPC 更新题目
func (c *questionClient) UpdateQuestion(ctx context.Context, q *biz.QuestionRecord) error {
	req := &questionv1.AdminUpdateQuestionRequest{
		Id:                 q.ID,
		CategoryId:         q.CategoryID,
		IndustryId:         q.IndustryID,
		Type:               q.Type,
		Difficulty:         q.Difficulty,
		Title:              q.Title,
		Content:            q.Content,
		OptionsJson:        q.OptionsJSON,
		Answer:             q.Answer,
		Explanation:        q.Explanation,
		SolutionJson:       q.SolutionJSON,
		JudgeConfigJson:    q.JudgeConfigJSON,
		AnswerTemplateJson: q.AnswerTemplateJSON,
		Tags:               q.Tags,
	}
	if q.HasIsActive {
		req.IsActive = &q.IsActive
	}
	_, err := c.client.AdminUpdateQuestion(forwardServiceAuth(ctx), req)
	return err
}

// DeleteQuestion 调用 question 服务的 AdminDeleteQuestion RPC 删除题目
func (c *questionClient) DeleteQuestion(ctx context.Context, id uint64) error {
	_, err := c.client.AdminDeleteQuestion(forwardServiceAuth(ctx), &questionv1.AdminDeleteQuestionRequest{
		Id: id,
	})
	return err
}

// GetQuestionStats 调用 question 服务的 GetAdminQuestionStats RPC 获取题目统计
func (c *questionClient) GetQuestionStats(ctx context.Context) (totalQuestions int64, err error) {
	resp, err := c.client.GetAdminQuestionStats(forwardServiceAuth(ctx), &questionv1.GetAdminQuestionStatsRequest{})
	if err != nil {
		return 0, err
	}
	return resp.TotalQuestions, nil
}

// listQuestionsPage 调用 question 服务原生 admin 列表 RPC，保留完整字段返回。
func (c *questionClient) listQuestionsPage(ctx context.Context, page, pageSize int32, keyword, difficulty string) ([]*biz.QuestionRecord, int64, error) {
	resp, err := c.client.AdminListQuestions(forwardServiceAuth(ctx), &questionv1.AdminListQuestionsRequest{
		Page:       &sharedv1.PageParam{Page: page, PageSize: pageSize},
		Keyword:    keyword,
		Difficulty: difficulty,
	})
	if err != nil {
		return nil, 0, err
	}
	questions := make([]*biz.QuestionRecord, len(resp.Questions))
	for i, q := range resp.Questions {
		questions[i] = toBizQuestionRecord(q)
	}
	return questions, resp.GetPageResult().GetTotal(), nil
}

// listFilteredQuestions 在下游尚未补齐过滤字段前，分页拉取后在 Admin 侧做兼容筛选。
func (c *questionClient) listFilteredQuestions(ctx context.Context, keyword, difficulty string, categoryID uint64, industryCode string) ([]*biz.QuestionRecord, error) {
	var targetIndustryID uint64
	if industryCode != "" {
		industry, err := c.loadIndustryByCode(ctx, industryCode)
		if err != nil {
			return nil, err
		}
		targetIndustryID = industry.ID
	}

	const batchSize int32 = 200
	page := int32(1)
	filtered := make([]*biz.QuestionRecord, 0)
	for {
		questions, total, err := c.listQuestionsPage(ctx, page, batchSize, keyword, difficulty)
		if err != nil {
			return nil, err
		}
		for _, question := range questions {
			if matchesQuestionFilters(question, categoryID, targetIndustryID) {
				filtered = append(filtered, question)
			}
		}
		if len(questions) == 0 || int64(page*batchSize) >= total {
			break
		}
		page++
	}
	return filtered, nil
}

// loadIndustryByCode 按行业编码读取权威行业记录，用于后台题目过滤对齐真实行业主键。
func (c *questionClient) loadIndustryByCode(ctx context.Context, industryCode string) (*biz.IndustryRecord, error) {
	if c.industryRepo == nil {
		return nil, fmt.Errorf("industry repo not configured")
	}
	return c.industryRepo.GetIndustryByCode(ctx, industryCode)
}

// toBizQuestionRecord 将 question 服务返回的管理题目信息转换为 Admin 领域对象。
func toBizQuestionRecord(q *questionv1.AdminQuestionInfo) *biz.QuestionRecord {
	if q == nil {
		return nil
	}
	return &biz.QuestionRecord{
		ID:                 q.Id,
		CategoryID:         q.CategoryId,
		IndustryID:         q.IndustryId,
		Type:               q.Type,
		Difficulty:         q.Difficulty,
		Title:              q.Title,
		Content:            q.Content,
		OptionsJSON:        q.OptionsJson,
		Answer:             q.Answer,
		Explanation:        q.Explanation,
		SolutionJSON:       q.SolutionJson,
		JudgeConfigJSON:    q.JudgeConfigJson,
		AnswerTemplateJSON: q.AnswerTemplateJson,
		Tags:               q.Tags,
		IsActive:           q.IsActive,
		HasIsActive:        true,
		CreatedAt:          protoTimeToTime(q.CreatedAt),
		UpdatedAt:          protoTimeToTime(q.UpdatedAt),
		CategoryName:       q.CategoryName,
		IndustryName:       q.IndustryName,
	}
}

// matchesQuestionFilters 判断题目是否满足 Admin 列表的补充筛选条件。
func matchesQuestionFilters(question *biz.QuestionRecord, categoryID uint64, industryID uint64) bool {
	if question == nil {
		return false
	}
	if categoryID > 0 && question.CategoryID != categoryID {
		return false
	}
	if industryID > 0 && question.IndustryID != industryID {
		return false
	}
	return true
}

// ==================== 面试服务 gRPC 客户端 ====================

// interviewClient 实现 biz.InterviewClient，通过 gRPC 调用 interview 微服务的 admin RPC。
type interviewClient struct {
	client interviewv1.InterviewServiceClient
	logger log.Logger
}

// NewInterviewClient 创建面试服务 gRPC 客户端。
func NewInterviewClient(conn *grpc.ClientConn, logger log.Logger) biz.InterviewClient {
	return &interviewClient{
		client: interviewv1.NewInterviewServiceClient(conn),
		logger: logger,
	}
}

// GetInterviewStats 调用 interview 服务统计全站面试总量。
func (c *interviewClient) GetInterviewStats(ctx context.Context) (int64, error) {
	resp, err := c.client.GetAdminInterviewStats(forwardServiceAuth(ctx), &interviewv1.GetAdminInterviewStatsRequest{})
	if err != nil {
		return 0, err
	}
	return resp.GetTotalInterviews(), nil
}

// ==================== AI 网关客户端（UNIMPLEMENTED） ====================

// aiGatewayClient 实现 biz.AIGatewayClient（UNIMPLEMENTED：预留，待 AI Gateway 实现 admin RPC）
type aiGatewayClient struct{}

// NewAIGatewayClient 创建 AI 网关客户端占位实例
func NewAIGatewayClient() biz.AIGatewayClient {
	return &aiGatewayClient{}
}

// ==================== 辅助函数 ====================

// forwardServiceAuth 透传当前请求的访问令牌，确保受保护的下游管理 RPC 能通过鉴权。
func forwardServiceAuth(ctx context.Context) context.Context {
	return auth.ForwardAccessToken(ctx)
}

// protoTimeToTime 将 protobuf Timestamp 安全转换为 time.Time
func protoTimeToTime(ts *timestamppb.Timestamp) time.Time {
	if ts == nil {
		return time.Time{}
	}
	return ts.AsTime()
}
