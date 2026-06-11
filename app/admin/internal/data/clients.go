package data

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	aiv1 "makejob/api/makejob/ai/v1"
	interviewv1 "makejob/api/makejob/interview/v1"
	questionv1 "makejob/api/makejob/question/v1"
	ragv1 "makejob/api/makejob/rag/v1"
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

// GetQuestion 调用 question 服务的公开题目详情 RPC，供 RAG 精确索引指定题目。
func (c *questionClient) GetQuestion(ctx context.Context, id uint64) (*biz.QuestionRecord, error) {
	resp, err := c.client.GetQuestion(forwardServiceAuth(ctx), &questionv1.GetQuestionRequest{Id: id})
	if err != nil {
		return nil, err
	}
	categoryID := uint64(0)
	categoryName := ""
	if resp.GetCategory() != nil {
		categoryID = resp.GetCategory().GetId()
		categoryName = resp.GetCategory().GetName()
	}
	return &biz.QuestionRecord{
		ID:           resp.GetId(),
		CategoryID:   categoryID,
		Type:         resp.GetType(),
		Difficulty:   resp.GetDifficulty(),
		Title:        resp.GetTitle(),
		Content:      resp.GetContent(),
		Answer:       resp.GetReferenceAnswer(),
		Explanation:  resp.GetExplanation(),
		Tags:         strings.Join(resp.GetTags(), ","),
		CreatedAt:    protoTimeToTime(resp.GetCreatedAt()),
		CategoryName: categoryName,
	}, nil
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

// ==================== AI 网关 gRPC 客户端 ====================

// aiGatewayClient 实现 biz.AIGatewayClient，通过 gRPC 调用 AI Gateway 的 admin 调试 RPC
type aiGatewayClient struct {
	client aiv1.AIServiceClient
	logger log.Logger
}

// NewAIGatewayClient 创建 AI 网关 gRPC 客户端
func NewAIGatewayClient(conn *grpc.ClientConn, logger log.Logger) biz.AIGatewayClient {
	return &aiGatewayClient{
		client: aiv1.NewAIServiceClient(conn),
		logger: logger,
	}
}

// noopAIGatewayClient 当 AI Gateway 未配置时的空实现
type noopAIGatewayClient struct{}

// NewAIGatewayClientNoop 创建空实现的 AI 网关客户端
func NewAIGatewayClientNoop() biz.AIGatewayClient {
	return &noopAIGatewayClient{}
}

func (c *noopAIGatewayClient) RenderPrompt(_ context.Context, _, _ string, _ map[string]string, _ bool) (*biz.RenderPromptResult, error) {
	return nil, fmt.Errorf("AI Gateway 服务未配置")
}

func (c *noopAIGatewayClient) DebugAI(_ context.Context, _, _ string, _ map[string]string, _ string) (*biz.DebugAIResult, error) {
	return nil, fmt.Errorf("AI Gateway 服务未配置")
}

func (c *noopAIGatewayClient) GenerateQuestionCandidates(_ context.Context, _, _, _ string, _ int32, _ string, _, _ bool, _ []string) (*biz.GenerateQuestionCandidatesResult, error) {
	return nil, fmt.Errorf("AI Gateway 服务未配置")
}

func (c *noopAIGatewayClient) GenerateQuestionCandidatesStream(_ context.Context, _, _, _ string, _ int32, _ biz.PipelineStreamEmitter) error {
	return fmt.Errorf("AI Gateway 服务未配置")
}

// RenderPrompt 调用 AI Gateway 的 RenderPrompt RPC
func (c *aiGatewayClient) RenderPrompt(ctx context.Context, scene, templateText string, variables map[string]string, runWithLLM bool) (*biz.RenderPromptResult, error) {
	resp, err := c.client.RenderPrompt(forwardServiceAuth(ctx), &aiv1.RenderPromptRequest{
		Scene:        scene,
		TemplateText: templateText,
		Variables:    variables,
		RunWithLlm:   runWithLLM,
	})
	if err != nil {
		return nil, err
	}
	return &biz.RenderPromptResult{
		RenderedPrompt:    resp.RenderedPrompt,
		ResolvedVariables: resp.ResolvedVariables,
		LLMResponse:       resp.LlmResponse,
		Model:             resp.Model,
		LatencyMs:         resp.LatencyMs,
	}, nil
}

// DebugAI 调用 AI Gateway 的 DebugAI RPC
func (c *aiGatewayClient) DebugAI(ctx context.Context, scene, prompt string, params map[string]string, modelOverride string) (*biz.DebugAIResult, error) {
	resp, err := c.client.DebugAI(forwardServiceAuth(ctx), &aiv1.DebugAIRequest{
		Scene:         scene,
		Prompt:        prompt,
		Params:        params,
		ModelOverride: modelOverride,
	})
	if err != nil {
		return nil, err
	}
	return &biz.DebugAIResult{
		RenderedPrompt: resp.RenderedPrompt,
		Response:       resp.Response,
		Model:          resp.Model,
		InputTokens:    int(resp.InputTokens),
		OutputTokens:   int(resp.OutputTokens),
		LatencyMs:      resp.LatencyMs,
		Error:          resp.Error,
	}, nil
}

// GenerateQuestionCandidates 调用 AI Gateway 的 GenerateQuestionCandidates RPC（FIX H5: 透传所有字段）
func (c *aiGatewayClient) GenerateQuestionCandidates(ctx context.Context, industryCode, requirement, agentPrompt string, candidateCount int32, generationMode string, includeScraped, includeGenerated bool, sources []string) (*biz.GenerateQuestionCandidatesResult, error) {
	// 先从原始 context 中提取认证信息
	accessToken := auth.GetAccessTokenFromContext(ctx)
	if accessToken == "" {
		accessToken = auth.GetAccessTokenFromMetadata(ctx)
	}

	// 创建全新的 context，不继承上游的 deadline
	// AI 调用耗时较长（通常 30-120 秒），不能被上游的短超时影响
	aiCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// 将认证信息添加到新 context 中
	if accessToken != "" {
		aiCtx = auth.WithOutgoingAccessToken(aiCtx, accessToken)
	}

	resp, err := c.client.GenerateQuestionCandidates(aiCtx, &aiv1.GenerateQuestionCandidatesRequest{
		IndustryCode:     industryCode,
		Requirement:      requirement,
		CandidateCount:   candidateCount,
		GenerationMode:   generationMode,
		Sources:          sources,
		AgentPrompt:      agentPrompt,
		IncludeScraped:   includeScraped,
		IncludeGenerated: includeGenerated,
	})
	if err != nil {
		return nil, err
	}

	candidates := make([]*biz.QuestionCandidate, 0, len(resp.Candidates))
	for _, c := range resp.Candidates {
		candidates = append(candidates, &biz.QuestionCandidate{
			Title:       c.Title,
			Content:     c.Content,
			Type:        c.Type,
			Difficulty:  c.Difficulty,
			Category:    c.Category,
			Answer:      c.Answer,
			Explanation: c.Explanation,
			Tags:        c.Tags,
			SourceType:  c.SourceType,
			Confidence:  c.Confidence,
			Solution:    c.Solution,
			JudgeConfig: c.JudgeConfig,
			SourceLabel: c.SourceLabel,
			SourceTitle: c.SourceTitle,
			SourceURL:   c.SourceUrl,
		})
	}

	return &biz.GenerateQuestionCandidatesResult{
		IndustryCode: resp.IndustryCode,
		Requirement:  resp.Requirement,
		Candidates:   candidates,
		Warnings:     resp.Warnings,
	}, nil
}

// GenerateQuestionCandidatesStream 调用 AI Gateway 的 GenerateQuestionCandidatesStream RPC（流式）
func (c *aiGatewayClient) GenerateQuestionCandidatesStream(ctx context.Context, industryCode, requirement, agentPrompt string, candidateCount int32, emit biz.PipelineStreamEmitter) error {
	// 先从原始 context 中提取认证信息
	accessToken := auth.GetAccessTokenFromContext(ctx)
	if accessToken == "" {
		accessToken = auth.GetAccessTokenFromMetadata(ctx)
	}

	// 创建全新的 context，不继承上游的 deadline
	aiCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// 将认证信息添加到新 context 中
	if accessToken != "" {
		aiCtx = auth.WithOutgoingAccessToken(aiCtx, accessToken)
	}

	stream, err := c.client.GenerateQuestionCandidatesStream(aiCtx, &aiv1.GenerateQuestionCandidatesRequest{
		IndustryCode:   industryCode,
		Requirement:    requirement,
		CandidateCount: candidateCount,
		AgentPrompt:    agentPrompt,
	})
	if err != nil {
		return err
	}

	for {
		event, err := stream.Recv()
		if err != nil {
			// EOF 表示流结束
			if err.Error() == "EOF" {
				break
			}
			return err
		}

		// 转换事件
		streamEvent := &biz.PipelineStreamEvent{
			Event:               event.GetEvent(),
			Message:             event.GetMessage(),
			TraceID:             event.GetTraceId(),
			RawOutput:           event.GetRawOutput(),
			FailureStage:        event.GetFailureStage(),
			CandidateExcerpt:    event.GetCandidateExcerpt(),
			RepairAttempted:     event.GetRepairAttempted(),
			SupplementAttempted: event.GetSupplementAttempted(),
			SlotIndex:           event.GetSlotIndex(),
			RetryIndex:          event.GetRetryIndex(),
		}

		if event.GetCard() != nil {
			streamEvent.Card = &biz.QuestionCandidate{
				Title:       event.GetCard().GetTitle(),
				Content:     event.GetCard().GetContent(),
				Type:        event.GetCard().GetType(),
				Difficulty:  event.GetCard().GetDifficulty(),
				Category:    event.GetCard().GetCategory(),
				Answer:      event.GetCard().GetAnswer(),
				Explanation: event.GetCard().GetExplanation(),
				Tags:        event.GetCard().GetTags(),
				SourceType:  event.GetCard().GetSourceType(),
				Confidence:  event.GetCard().GetConfidence(),
				Solution:    event.GetCard().GetSolution(),
				JudgeConfig: event.GetCard().GetJudgeConfig(),
				SourceLabel: event.GetCard().GetSourceLabel(),
				SourceTitle: event.GetCard().GetSourceTitle(),
				SourceURL:   event.GetCard().GetSourceUrl(),
			}
		}

		if event.GetResponse() != nil {
			resp := event.GetResponse()
			streamEvent.Response = &biz.GenerateQuestionCandidatesResult{
				IndustryCode: resp.GetIndustryCode(),
				Requirement:  resp.GetRequirement(),
				Warnings:     resp.GetWarnings(),
			}
			for _, c := range resp.GetCandidates() {
				streamEvent.Response.Candidates = append(streamEvent.Response.Candidates, &biz.QuestionCandidate{
					Title:       c.GetTitle(),
					Content:     c.GetContent(),
					Type:        c.GetType(),
					Difficulty:  c.GetDifficulty(),
					Category:    c.GetCategory(),
					Answer:      c.GetAnswer(),
					Explanation: c.GetExplanation(),
					Tags:        c.GetTags(),
					SourceType:  c.GetSourceType(),
					Confidence:  c.GetConfidence(),
					Solution:    c.GetSolution(),
					JudgeConfig: c.GetJudgeConfig(),
					SourceLabel: c.GetSourceLabel(),
					SourceTitle: c.GetSourceTitle(),
					SourceURL:   c.GetSourceUrl(),
				})
			}
		}

		if emit != nil {
			if err := emit(streamEvent); err != nil {
				return err
			}
		}
	}

	return nil
}

// ==================== RAG 服务 gRPC 客户端 ====================

// ragClient 实现 biz.RAGClient，通过 gRPC 调用 rag 微服务
type ragClient struct {
	client ragv1.RAGServiceClient
	logger log.Logger
}

// NewRAGClient 创建 RAG 服务 gRPC 客户端
func NewRAGClient(conn *grpc.ClientConn, logger log.Logger) biz.RAGClient {
	return &ragClient{
		client: ragv1.NewRAGServiceClient(conn),
		logger: logger,
	}
}

// TestConnection 调用 RAG 服务的 TestConnection RPC
func (c *ragClient) TestConnection(ctx context.Context) (milvusOk bool, embeddingOk bool, err error) {
	resp, err := c.client.TestConnection(forwardServiceAuth(ctx), &ragv1.TestConnectionRequest{})
	if err != nil {
		return false, false, err
	}
	return resp.Connected, resp.Connected, nil
}

// GetConfig 调用 RAG 服务的 GetConfig RPC
func (c *ragClient) GetConfig(ctx context.Context) (collectionName string, embedModel string, err error) {
	resp, err := c.client.GetConfig(forwardServiceAuth(ctx), &ragv1.GetConfigRequest{})
	if err != nil {
		return "", "", err
	}
	return resp.CollectionName, resp.EmbeddingModel, nil
}

// UpdateConfig 调用 RAG 服务的 UpdateConfig RPC 更新运行时配置。
func (c *ragClient) UpdateConfig(ctx context.Context, collectionName string, embeddingDimension int32, embedModel string) (string, int32, string, error) {
	resp, err := c.client.UpdateConfig(forwardServiceAuth(ctx), &ragv1.UpdateConfigRequest{
		CollectionName:     collectionName,
		EmbeddingDimension: embeddingDimension,
		EmbeddingModel:     embedModel,
	})
	if err != nil {
		return "", 0, "", err
	}
	return resp.GetCollectionName(), resp.GetEmbeddingDimension(), resp.GetEmbeddingModel(), nil
}

// IndexQuestions 调用 RAG 服务的 IndexQuestions RPC
func (c *ragClient) IndexQuestions(ctx context.Context, items []*biz.RAGIndexItem) (int32, []string, error) {
	protoItems := make([]*ragv1.IndexItem, len(items))
	for i, item := range items {
		protoItems[i] = &ragv1.IndexItem{
			QuestionId: item.QuestionID,
			Content:    item.Content,
			Metadata:   item.Metadata,
		}
	}

	resp, err := c.client.IndexQuestions(forwardServiceAuth(ctx), &ragv1.IndexQuestionsRequest{
		Items: protoItems,
	})
	if err != nil {
		return 0, nil, err
	}
	return resp.IndexedCount, resp.FailedIds, nil
}

// DeleteIndex 调用 RAG 服务的 DeleteIndex RPC
func (c *ragClient) DeleteIndex(ctx context.Context, ids []string) (int32, error) {
	resp, err := c.client.DeleteIndex(forwardServiceAuth(ctx), &ragv1.DeleteIndexRequest{
		Ids: ids,
	})
	if err != nil {
		return 0, err
	}
	return resp.DeletedCount, nil
}

// SearchQuestions 调用 RAG 服务的 Retrieve RPC
func (c *ragClient) SearchQuestions(ctx context.Context, query string, topK int32) ([]*biz.RAGSearchResult, error) {
	resp, err := c.client.Retrieve(forwardServiceAuth(ctx), &ragv1.RetrieveRequest{
		Query: query,
		TopK:  topK,
	})
	if err != nil {
		return nil, err
	}

	results := make([]*biz.RAGSearchResult, len(resp.Documents))
	for i, doc := range resp.Documents {
		title := ""
		if doc.Metadata != nil {
			title = doc.Metadata["title"]
		}
		results[i] = &biz.RAGSearchResult{
			DocID:   doc.Id,
			Title:   title,
			Content: doc.Content,
			Score:   float64(doc.Score),
		}
	}
	return results, nil
}

// GetDocumentStats 调用 RAG 服务的 GetDocumentStats RPC
func (c *ragClient) GetDocumentStats(ctx context.Context) (totalDocuments int64, totalQuestions int64, err error) {
	resp, err := c.client.GetDocumentStats(forwardServiceAuth(ctx), &ragv1.GetDocumentStatsRequest{})
	if err != nil {
		return 0, 0, err
	}
	return resp.TotalDocuments, resp.TotalQuestions, nil
}

// IndexDocuments 调用 RAG 服务的 IndexDocuments RPC
func (c *ragClient) IndexDocuments(ctx context.Context, items []*biz.RAGDocumentIndexItem) (int32, []string, error) {
	protoItems := make([]*ragv1.DocumentIndexItem, len(items))
	for i, item := range items {
		protoItems[i] = &ragv1.DocumentIndexItem{
			Id:       item.ID,
			Content:  item.Content,
			Source:   item.Source,
			Metadata: item.Metadata,
		}
	}

	resp, err := c.client.IndexDocuments(forwardServiceAuth(ctx), &ragv1.IndexDocumentsRequest{
		Items: protoItems,
	})
	if err != nil {
		return 0, nil, err
	}
	return resp.IndexedCount, resp.FailedIds, nil
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
