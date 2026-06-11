package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	kratoserr "github.com/go-kratos/kratos/v2/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	adminv1 "makejob/api/makejob/admin/v1"
	sharedv1 "makejob/api/makejob/shared/v1"
	"makejob/app/admin/internal/biz"
)

// AdminService 实现 gRPC AdminServiceServer
type AdminService struct {
	adminv1.UnimplementedAdminServiceServer
	uc                *biz.AdminUseCase
	pipelinePublisher asyncTaskPublisher
}

// asyncTaskPublisher 抽象 Admin 侧需要投递的异步任务发布能力。
type asyncTaskPublisher interface {
	PublishQuestionPipelineBuild(ctx context.Context, taskID uint64, req *adminv1.GenerateQuestionPipelineRequest) error
	PublishScraperImport(ctx context.Context, taskID uint64, payload []byte) error
}

const (
	questionPipelineTaskType  = "question_pipeline_build"
	questionPipelineSourceURL = "manual://question-pipeline"
	questionPipelineSource    = "admin"
)

// NewAdminService 创建管理后台服务，并注入异步任务发布器。
func NewAdminService(uc *biz.AdminUseCase, pipelinePublisher asyncTaskPublisher) *AdminService {
	return &AdminService{uc: uc, pipelinePublisher: pipelinePublisher}
}

// ==================== 仪表盘 ====================

func (s *AdminService) GetDashboard(ctx context.Context, _ *emptypb.Empty) (*adminv1.DashboardResponse, error) {
	d, err := s.uc.GetDashboard(ctx)
	if err != nil {
		return nil, err
	}
	return &adminv1.DashboardResponse{
		TotalUsers:       d.TotalUsers,
		TotalQuestions:   d.TotalQuestions,
		TotalInterviews:  d.TotalInterviews,
		TodayActiveUsers: d.TodayActiveUsers,
		ProMembers:       d.ProMembers,
		NewUsersToday:    d.NewUsersToday,
	}, nil
}

// ==================== 用户管理 ====================

func (s *AdminService) ListUsers(ctx context.Context, req *adminv1.ListUsersRequest) (*adminv1.ListUsersResponse, error) {
	var page, pageSize int32 = 1, 20
	if req.Page != nil {
		page = req.Page.Page
		pageSize = req.Page.PageSize
	}
	users, total, err := s.uc.ListUsers(ctx, page, pageSize)
	if err != nil {
		return nil, err
	}
	items := make([]*adminv1.AdminUserInfo, len(users))
	for i, u := range users {
		item := &adminv1.AdminUserInfo{
			Id:              u.ID,
			Username:        u.Username,
			Email:           u.Email,
			Role:            u.Role,
			Avatar:          u.Avatar,
			MembershipLevel: u.MembershipLevel,
			MembershipType:  u.MembershipType,
			IsDisabled:      u.IsDisabled,
			CreatedAt:       timestamppb.New(u.CreatedAt),
		}
		if u.MembershipExpireAt != nil {
			item.MembershipExpireAt = timestamppb.New(*u.MembershipExpireAt)
		}
		items[i] = item
	}
	return &adminv1.ListUsersResponse{
		Users: items,
		PageResult: &sharedv1.PageResult{
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		},
	}, nil
}

func (s *AdminService) UpdateUserRole(ctx context.Context, req *adminv1.UpdateUserRoleRequest) (*emptypb.Empty, error) {
	if err := s.uc.UpdateUserRole(ctx, req.UserId, req.Role); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *AdminService) DisableUser(ctx context.Context, req *adminv1.DisableUserRequest) (*emptypb.Empty, error) {
	if err := s.uc.DisableUser(ctx, req.UserId); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// ==================== 题库管理 ====================

func (s *AdminService) AdminListQuestions(ctx context.Context, req *adminv1.AdminListQuestionsRequest) (*adminv1.AdminListQuestionsResponse, error) {
	var page, pageSize int32 = 1, 20
	if req.Page != nil {
		page = req.Page.Page
		pageSize = req.Page.PageSize
	}
	questions, total, err := s.uc.AdminListQuestions(ctx, page, pageSize, req.Keyword, req.Difficulty, req.CategoryId, req.IndustryCode)
	if err != nil {
		return nil, err
	}
	items := make([]*adminv1.QuestionInfo, len(questions))
	for i, q := range questions {
		items[i] = &adminv1.QuestionInfo{
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
			IsActive:           q.IsActive,
			CreatedAt:          timestamppb.New(q.CreatedAt),
			UpdatedAt:          timestamppb.New(q.UpdatedAt),
			CategoryName:       q.CategoryName,
			IndustryName:       q.IndustryName,
		}
	}
	return &adminv1.AdminListQuestionsResponse{
		Questions: items,
		PageResult: &sharedv1.PageResult{
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		},
	}, nil
}

func (s *AdminService) CreateQuestion(ctx context.Context, req *adminv1.CreateQuestionRequest) (*adminv1.QuestionInfo, error) {
	q := &biz.QuestionRecord{
		CategoryID:         req.CategoryId,
		IndustryID:         req.IndustryId,
		Type:               req.Type,
		Difficulty:         req.Difficulty,
		Title:              req.Title,
		Content:            req.Content,
		OptionsJSON:        req.OptionsJson,
		Answer:             req.Answer,
		Explanation:        req.Explanation,
		SolutionJSON:       req.SolutionJson,
		JudgeConfigJSON:    req.JudgeConfigJson,
		AnswerTemplateJSON: req.AnswerTemplateJson,
		Tags:               req.Tags,
		IsActive:           req.IsActive,
	}
	if err := s.uc.CreateQuestion(ctx, q); err != nil {
		return nil, err
	}
	return &adminv1.QuestionInfo{Id: q.ID}, nil
}

func (s *AdminService) UpdateQuestion(ctx context.Context, req *adminv1.UpdateQuestionRequest) (*emptypb.Empty, error) {
	q := &biz.QuestionRecord{
		ID:                 req.Id,
		CategoryID:         req.CategoryId,
		IndustryID:         req.IndustryId,
		Type:               req.Type,
		Difficulty:         req.Difficulty,
		Title:              req.Title,
		Content:            req.Content,
		OptionsJSON:        req.OptionsJson,
		Answer:             req.Answer,
		Explanation:        req.Explanation,
		SolutionJSON:       req.SolutionJson,
		JudgeConfigJSON:    req.JudgeConfigJson,
		AnswerTemplateJSON: req.AnswerTemplateJson,
		Tags:               req.Tags,
	}
	if req.IsActive != nil {
		q.IsActive = *req.IsActive
		q.HasIsActive = true
	}
	if err := s.uc.UpdateQuestion(ctx, q); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *AdminService) DeleteQuestion(ctx context.Context, req *adminv1.DeleteQuestionRequest) (*emptypb.Empty, error) {
	if err := s.uc.DeleteQuestion(ctx, req.Id); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *AdminService) BatchImportQuestions(ctx context.Context, req *adminv1.BatchImportQuestionsRequest) (*adminv1.BatchImportQuestionsResponse, error) {
	// 获取行业ID
	industry, err := s.uc.GetIndustryByCode(ctx, req.IndustryCode)
	if err != nil {
		return nil, err
	}
	questions := make([]*biz.QuestionRecord, len(req.Questions))
	for i, q := range req.Questions {
		questions[i] = &biz.QuestionRecord{
			IndustryID:         industry.ID,
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
			IsActive:           true,
		}
	}
	success, fail, errors := s.uc.BatchImportQuestions(ctx, questions)
	return &adminv1.BatchImportQuestionsResponse{
		TotalCount:   int32(len(questions)),
		SuccessCount: int32(success),
		FailCount:    int32(fail),
		Errors:       errors,
	}, nil
}

func (s *AdminService) GetQuestionTagTaxonomy(ctx context.Context, _ *emptypb.Empty) (*adminv1.QuestionTagTaxonomyResponse, error) {
	groups, err := s.uc.GetQuestionTagTaxonomy(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]*adminv1.TagTaxonomyGroup, len(groups))
	for i, g := range groups {
		items[i] = &adminv1.TagTaxonomyGroup{
			Category: g.Category,
			Tags:     g.Tags,
		}
	}
	return &adminv1.QuestionTagTaxonomyResponse{Groups: items}, nil
}

// ==================== 题目流水线（简化实现） ====================

// GenerateQuestionPipeline 同步生成题目候选，委托 AI Gateway 的 GenerateQuestionCandidates RPC。
func (s *AdminService) GenerateQuestionPipeline(ctx context.Context, req *adminv1.GenerateQuestionPipelineRequest) (*adminv1.GenerateQuestionPipelineResponse, error) {
	if s.uc.AIGatewayClient() == nil {
		return nil, kratoserr.ServiceUnavailable("AI_GATEWAY_NOT_CONFIGURED", "AI 网关客户端未配置")
	}

	normalized, err := normalizeQuestionPipelineRequest(req)
	if err != nil {
		return nil, err
	}

	result, err := s.uc.AIGatewayClient().GenerateQuestionCandidates(
		ctx,
		normalized.GetIndustryCode(),
		normalized.GetRequirement(),
		normalized.GetAgentPrompt(),
		normalized.GetCandidateCount(),
		normalized.GetGenerationMode(),
		normalized.GetIncludeScraped(),
		normalized.GetIncludeGenerated(),
		normalized.GetSources(),
	)
	if err != nil {
		return nil, err
	}

	cards := make([]*adminv1.PipelineCard, 0, len(result.Candidates))
	for _, c := range result.Candidates {
		cards = append(cards, &adminv1.PipelineCard{
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
			SourceUrl:   c.SourceURL,
		})
	}

	return &adminv1.GenerateQuestionPipelineResponse{
		IndustryCode:   result.IndustryCode,
		Requirement:    result.Requirement,
		GenerationMode: normalized.GetGenerationMode(),
		Cards:          cards,
		Warnings:       result.Warnings,
	}, nil
}

// GenerateQuestionPipelineStream 流式生成题目候选，实时推送事件。
func (s *AdminService) GenerateQuestionPipelineStream(req *adminv1.GenerateQuestionPipelineRequest, stream adminv1.AdminService_GenerateQuestionPipelineStreamServer) error {
	if s.uc.AIGatewayClient() == nil {
		return kratoserr.ServiceUnavailable("AI_GATEWAY_NOT_CONFIGURED", "AI 网关客户端未配置")
	}

	normalized, err := normalizeQuestionPipelineRequest(req)
	if err != nil {
		return err
	}

	// 创建事件回调
	emit := func(event *biz.PipelineStreamEvent) error {
		protoEvent := &adminv1.PipelineStreamEvent{
			Event:            event.Event,
			Message:          event.Message,
			TraceId:          event.TraceID,
			RawOutput:        event.RawOutput,
			FailureStage:     event.FailureStage,
			CandidateExcerpt: event.CandidateExcerpt,
			RepairAttempted:  event.RepairAttempted,
			SupplementAttempted: event.SupplementAttempted,
			SlotIndex:        event.SlotIndex,
			RetryIndex:       event.RetryIndex,
		}

		if event.Card != nil {
			protoEvent.Card = &adminv1.PipelineCard{
				Title:       event.Card.Title,
				Content:     event.Card.Content,
				Type:        event.Card.Type,
				Difficulty:  event.Card.Difficulty,
				Category:    event.Card.Category,
				Answer:      event.Card.Answer,
				Explanation: event.Card.Explanation,
				Tags:        event.Card.Tags,
				SourceType:  event.Card.SourceType,
				Confidence:  event.Card.Confidence,
				Solution:    event.Card.Solution,
				JudgeConfig: event.Card.JudgeConfig,
				SourceLabel: event.Card.SourceLabel,
				SourceTitle: event.Card.SourceTitle,
				SourceUrl:   event.Card.SourceURL,
			}
		}

		if event.Response != nil {
			resp := event.Response
			protoEvent.Response = &adminv1.GenerateQuestionPipelineResponse{
				IndustryCode: resp.IndustryCode,
				Requirement:  resp.Requirement,
				Warnings:     resp.Warnings,
			}
			for _, c := range resp.Candidates {
				protoEvent.Response.Cards = append(protoEvent.Response.Cards, &adminv1.PipelineCard{
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
					SourceUrl:   c.SourceURL,
				})
			}
		}

		return stream.Send(protoEvent)
	}

	return s.uc.AIGatewayClient().GenerateQuestionCandidatesStream(
		stream.Context(),
		normalized.GetIndustryCode(),
		normalized.GetRequirement(),
		normalized.GetAgentPrompt(),
		normalized.GetCandidateCount(),
		emit,
	)
}

// GenerateQuestionPipelineAsync 创建异步题目流水线任务并投递到 question 服务消费队列。
func (s *AdminService) GenerateQuestionPipelineAsync(ctx context.Context, req *adminv1.GenerateQuestionPipelineRequest) (*adminv1.PipelineTaskInfo, error) {
	normalized, err := normalizeQuestionPipelineRequest(req)
	if err != nil {
		return nil, err
	}
	if s.pipelinePublisher == nil {
		return nil, kratoserr.ServiceUnavailable("MQ_UNAVAILABLE", "题目流水线消息发布器未配置")
	}

	payloadBytes, err := json.Marshal(normalized)
	if err != nil {
		return nil, kratoserr.InternalServer("PIPELINE_PAYLOAD_ENCODE_FAILED", "题目流水线任务载荷编码失败")
	}

	task := &biz.ScraperTaskRecord{
		TaskType:      questionPipelineTaskType,
		SourceURL:     questionPipelineSourceURL,
		SourceTitle:   buildQuestionPipelineTaskTitle(normalized.GetRequirement()),
		Source:        questionPipelineSource,
		Status:        "pending",
		PayloadJSON:   string(payloadBytes),
		QuestionCount: int(normalized.GetCandidateCount()),
	}
	if err := s.uc.CreateScraperTask(ctx, task); err != nil {
		return nil, err
	}
	if err := s.pipelinePublisher.PublishQuestionPipelineBuild(ctx, task.ID, normalized); err != nil {
		task.Status = "failed"
		task.ErrorMsg = err.Error()
		finishedAt := time.Now()
		task.FinishedAt = &finishedAt
		_ = s.uc.UpdateScraperTask(ctx, task)
		return nil, kratoserr.InternalServer("PIPELINE_TASK_PUBLISH_FAILED", "题目流水线任务投递失败")
	}

	return &adminv1.PipelineTaskInfo{
		TaskId: task.ID,
		Status: task.Status,
	}, nil
}

func (s *AdminService) ImportQuestionPipeline(ctx context.Context, req *adminv1.ImportQuestionPipelineRequest) (*adminv1.BatchImportQuestionsResponse, error) {
	industry, err := s.uc.GetIndustryByCode(ctx, req.IndustryCode)
	if err != nil {
		return nil, err
	}
	questions := make([]*biz.QuestionRecord, len(req.Cards))
	for i, c := range req.Cards {
		questions[i] = &biz.QuestionRecord{
			IndustryID: industry.ID,
			Type:       c.Type,
			Difficulty: c.Difficulty,
			Title:      c.Title,
			Content:    c.Content,
			Answer:     c.Answer,
			Tags:       joinTags(c.Tags),
			IsActive:   true,
		}
	}
	success, fail, errors := s.uc.BatchImportQuestions(ctx, questions)
	return &adminv1.BatchImportQuestionsResponse{
		TotalCount:   int32(len(questions)),
		SuccessCount: int32(success),
		FailCount:    int32(fail),
		Errors:       errors,
	}, nil
}

func joinTags(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	result := tags[0]
	for i := 1; i < len(tags); i++ {
		result += "," + tags[i]
	}
	return result
}

// ==================== 分类管理 ====================

func (s *AdminService) AdminListCategories(ctx context.Context, _ *emptypb.Empty) (*adminv1.AdminListCategoriesResponse, error) {
	cats, err := s.uc.ListCategories(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]*adminv1.CategoryInfo, len(cats))
	for i, c := range cats {
		items[i] = &adminv1.CategoryInfo{
			Id:          c.ID,
			IndustryId:  c.IndustryID,
			Name:        c.Name,
			ParentId:    c.ParentID,
			SortOrder:   c.SortOrder,
			Icon:        c.Icon,
			Description: c.Description,
			CreatedAt:   timestamppb.New(c.CreatedAt),
		}
	}
	return &adminv1.AdminListCategoriesResponse{Categories: items}, nil
}

func (s *AdminService) CreateCategory(ctx context.Context, req *adminv1.CreateCategoryRequest) (*adminv1.CategoryInfo, error) {
	c := &biz.CategoryRecord{
		IndustryID:  req.IndustryId,
		Name:        req.Name,
		ParentID:    req.ParentId,
		SortOrder:   req.SortOrder,
		Icon:        req.Icon,
		Description: req.Description,
	}
	if err := s.uc.CreateCategory(ctx, c); err != nil {
		return nil, err
	}
	return &adminv1.CategoryInfo{Id: c.ID}, nil
}

func (s *AdminService) UpdateCategory(ctx context.Context, req *adminv1.UpdateCategoryRequest) (*emptypb.Empty, error) {
	c := &biz.CategoryRecord{
		ID:          req.Id,
		IndustryID:  req.IndustryId,
		Name:        req.Name,
		ParentID:    req.ParentId,
		SortOrder:   req.SortOrder,
		Icon:        req.Icon,
		Description: req.Description,
	}
	if err := s.uc.UpdateCategory(ctx, c); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *AdminService) DeleteCategory(ctx context.Context, req *adminv1.DeleteCategoryRequest) (*emptypb.Empty, error) {
	if err := s.uc.DeleteCategory(ctx, req.Id); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// ==================== 行业管理 ====================

func (s *AdminService) AdminListIndustries(ctx context.Context, _ *emptypb.Empty) (*adminv1.AdminListIndustriesResponse, error) {
	inds, err := s.uc.ListIndustries(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]*adminv1.IndustryInfo, len(inds))
	for i, ind := range inds {
		items[i] = &adminv1.IndustryInfo{
			Id:          ind.ID,
			Code:        ind.Code,
			Name:        ind.Name,
			Description: ind.Description,
			Icon:        ind.Icon,
			IsActive:    ind.IsActive,
			SortOrder:   ind.SortOrder,
			CreatedAt:   timestamppb.New(ind.CreatedAt),
		}
	}
	return &adminv1.AdminListIndustriesResponse{Industries: items}, nil
}

func (s *AdminService) CreateIndustry(ctx context.Context, req *adminv1.CreateIndustryRequest) (*adminv1.IndustryInfo, error) {
	ind := &biz.IndustryRecord{
		Code:        req.Code,
		Name:        req.Name,
		Description: req.Description,
		Icon:        req.Icon,
		SortOrder:   req.SortOrder,
	}
	if err := s.uc.CreateIndustry(ctx, ind); err != nil {
		return nil, err
	}
	return &adminv1.IndustryInfo{Id: ind.ID}, nil
}

func (s *AdminService) UpdateIndustry(ctx context.Context, req *adminv1.UpdateIndustryRequest) (*emptypb.Empty, error) {
	ind := &biz.IndustryRecord{
		ID:          req.Id,
		Code:        req.Code,
		Name:        req.Name,
		Description: req.Description,
		Icon:        req.Icon,
		SortOrder:   req.SortOrder,
	}
	if req.IsActive != nil {
		ind.IsActive = *req.IsActive
	}
	if err := s.uc.UpdateIndustry(ctx, ind); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// ==================== Prompt 模板管理 ====================

func (s *AdminService) ListPromptTemplates(ctx context.Context, req *adminv1.ListPromptTemplatesRequest) (*adminv1.ListPromptTemplatesResponse, error) {
	templates, err := s.uc.ListPromptTemplates(ctx, req.IndustryCode)
	if err != nil {
		return nil, err
	}
	items := make([]*adminv1.PromptTemplate, len(templates))
	for i, t := range templates {
		items[i] = &adminv1.PromptTemplate{
			Id:           t.ID,
			Name:         t.Name,
			IndustryCode: t.IndustryCode,
			TemplateType: t.TemplateType,
			Content:      t.TemplateContent,
			Scene:        t.Scene,
			Variables:    t.Variables,
			IsActive:     t.IsActive,
			UpdatedAt:    timestamppb.New(t.UpdatedAt),
		}
	}
	return &adminv1.ListPromptTemplatesResponse{Templates: items}, nil
}

func (s *AdminService) SavePromptTemplate(ctx context.Context, req *adminv1.SavePromptTemplateRequest) (*adminv1.PromptTemplate, error) {
	tpl := &biz.PromptTemplate{
		ID:              req.Id,
		Name:            req.Name,
		IndustryCode:    req.IndustryCode,
		TemplateType:    req.TemplateType,
		TemplateContent: req.Content,
	}
	if err := s.uc.SavePromptTemplate(ctx, tpl); err != nil {
		return nil, err
	}
	return &adminv1.PromptTemplate{Id: tpl.ID}, nil
}

func (s *AdminService) CreatePrompt(ctx context.Context, req *adminv1.CreatePromptRequest) (*adminv1.PromptTemplate, error) {
	tpl := &biz.PromptTemplate{
		IndustryID:      req.IndustryId,
		Name:            req.Name,
		Scene:           req.Scene,
		TemplateContent: req.TemplateContent,
		Variables:       req.Variables,
		IsActive:        req.IsActive,
	}
	if err := s.uc.CreatePromptTemplate(ctx, tpl); err != nil {
		return nil, err
	}
	return &adminv1.PromptTemplate{Id: tpl.ID}, nil
}

func (s *AdminService) UpdatePrompt(ctx context.Context, req *adminv1.UpdatePromptRequest) (*emptypb.Empty, error) {
	tpl := &biz.PromptTemplate{
		ID:              req.Id,
		IndustryID:      req.IndustryId,
		Name:            req.Name,
		Scene:           req.Scene,
		TemplateContent: req.TemplateContent,
		Variables:       req.Variables,
	}
	if req.IsActive != nil {
		tpl.IsActive = *req.IsActive
	}
	if err := s.uc.UpdatePromptTemplate(ctx, tpl); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *AdminService) DeletePrompt(ctx context.Context, req *adminv1.DeletePromptRequest) (*emptypb.Empty, error) {
	if err := s.uc.DeletePromptTemplate(ctx, req.Id); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// TestRenderPrompt 委托 AI Gateway 渲染 Prompt 模板预览。
func (s *AdminService) TestRenderPrompt(ctx context.Context, req *adminv1.TestRenderPromptRequest) (*adminv1.TestRenderPromptResponse, error) {
	if s.uc.AIGatewayClient() == nil {
		return nil, kratoserr.ServiceUnavailable("AI_GATEWAY_NOT_CONFIGURED", "AI 网关客户端未配置")
	}

	result, err := s.uc.AIGatewayClient().RenderPrompt(
		ctx,
		req.GetAgentType(),
		req.GetPrompt(),
		req.GetParams(),
		true, // 默认调用 LLM 试跑
	)
	if err != nil {
		return nil, err
	}

	return &adminv1.TestRenderPromptResponse{
		RenderedPrompt:    result.RenderedPrompt,
		Response:          result.LLMResponse,
		Model:             result.Model,
		LatencyMs:         result.LatencyMs,
		ResolvedVariables: result.ResolvedVariables,
	}, nil
}

// ==================== AI 配置 ====================

func (s *AdminService) GetAIConfigs(ctx context.Context, _ *emptypb.Empty) (*adminv1.GetAIConfigsResponse, error) {
	configs, err := s.uc.ListAdminConfigs(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]*adminv1.AdminConfigItem, len(configs))
	for i, c := range configs {
		items[i] = &adminv1.AdminConfigItem{
			Key:         c.Key,
			Value:       c.Value,
			ConfigType:  c.ConfigType,
			Description: c.Description,
		}
	}
	baseConfigs := defaultAIConfigValues()
	configMap := mergeAdminConfigItems(items, baseConfigs)

	// 获取预设
	presets, err := s.uc.ListAIPresets(ctx)
	if err != nil {
		return nil, err
	}
	presetItems := make([]*adminv1.AIPreset, len(presets))
	var activePresetID uint64
	for i, p := range presets {
		presetItems[i] = &adminv1.AIPreset{
			Id:        p.ID,
			Name:      p.Name,
			Configs:   p.Configs,
			IsActive:  p.IsActive,
			UpdatedAt: timestamppb.New(p.UpdatedAt),
		}
		if p.IsActive {
			activePresetID = p.ID
		}
	}
	return &adminv1.GetAIConfigsResponse{
		Configs:        configMap,
		Items:          items,
		Presets:        presetItems,
		ActivePresetId: activePresetID,
	}, nil
}

// UpdateAIConfigs 保存 AI 运行配置，并在 Admin 侧做兼容校验与默认值归一化。
func (s *AdminService) UpdateAIConfigs(ctx context.Context, req *adminv1.UpdateAIConfigsRequest) (*emptypb.Empty, error) {
	normalized, err := normalizeAIConfigInput(req.Configs)
	if err != nil {
		return nil, err
	}
	if err := s.uc.BatchUpsertConfigs(ctx, normalized); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// ==================== AI 预设管理 ====================

func (s *AdminService) ListAIPresets(ctx context.Context, _ *emptypb.Empty) (*adminv1.ListAIPresetsResponse, error) {
	presets, err := s.uc.ListAIPresets(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]*adminv1.AIPreset, len(presets))
	for i, p := range presets {
		items[i] = &adminv1.AIPreset{
			Id:        p.ID,
			Name:      p.Name,
			Configs:   p.Configs,
			IsActive:  p.IsActive,
			UpdatedAt: timestamppb.New(p.UpdatedAt),
		}
	}
	return &adminv1.ListAIPresetsResponse{Presets: items}, nil
}

func (s *AdminService) SaveAIPreset(ctx context.Context, req *adminv1.SaveAIPresetRequest) (*adminv1.AIPreset, error) {
	preset := &biz.AIPreset{
		ID:     req.Id,
		Name:   req.Name,
		Params: req.Params,
	}
	if err := s.uc.SaveAIPreset(ctx, preset); err != nil {
		return nil, err
	}
	return &adminv1.AIPreset{
		Id:   preset.ID,
		Name: preset.Name,
	}, nil
}

func (s *AdminService) CreateAIPreset(ctx context.Context, req *adminv1.CreateAIPresetRequest) (*adminv1.AIPreset, error) {
	preset := &biz.AIPreset{
		Name:    req.Name,
		Configs: req.Configs,
	}
	if err := s.uc.CreateAIPreset(ctx, preset); err != nil {
		return nil, err
	}
	return &adminv1.AIPreset{
		Id:        preset.ID,
		Name:      preset.Name,
		Configs:   preset.Configs,
		IsActive:  preset.IsActive,
		UpdatedAt: timestamppb.New(preset.UpdatedAt),
	}, nil
}

func (s *AdminService) UpdateAIPreset(ctx context.Context, req *adminv1.UpdateAIPresetRequest) (*adminv1.AIPreset, error) {
	currentPreset, err := s.uc.GetAIPresetByID(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	if currentPreset == nil {
		return nil, kratoserr.NotFound("AI_PRESET_NOT_FOUND", "AI 预设不存在")
	}

	updatedPreset := &biz.AIPreset{
		ID:       req.Id,
		Name:     currentPreset.Name,
		Configs:  currentPreset.Configs,
		IsActive: currentPreset.IsActive,
	}
	if req.Name != "" {
		updatedPreset.Name = req.Name
	}
	if len(req.Configs) > 0 {
		updatedPreset.Configs = req.Configs
	}

	if err := s.uc.UpdateAIPreset(ctx, updatedPreset); err != nil {
		return nil, err
	}
	return &adminv1.AIPreset{
		Id:        updatedPreset.ID,
		Name:      updatedPreset.Name,
		Configs:   updatedPreset.Configs,
		IsActive:  updatedPreset.IsActive,
		UpdatedAt: timestamppb.New(updatedPreset.UpdatedAt),
	}, nil
}

func (s *AdminService) DeleteAIPreset(ctx context.Context, req *adminv1.DeleteAIPresetRequest) (*emptypb.Empty, error) {
	if err := s.uc.DeleteAIPreset(ctx, req.Id); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *AdminService) ApplyAIPreset(ctx context.Context, req *adminv1.ApplyAIPresetRequest) (*adminv1.GetAIConfigsResponse, error) {
	if err := s.uc.ApplyAIPreset(ctx, req.Id); err != nil {
		return nil, err
	}
	// 返回更新后的配置
	return s.GetAIConfigs(ctx, &emptypb.Empty{})
}

// ==================== AI 调试 & 日志 ====================

// DebugAI 委托 AI Gateway 执行 AI 调试调用。
// DebugAI 委托 AI Gateway 执行 AI 调试调用（FIX C5: 透传所有调试字段，FIX L2: 透传 model_override）
func (s *AdminService) DebugAI(ctx context.Context, req *adminv1.DebugAIRequest) (*adminv1.DebugAIResponse, error) {
	if s.uc.AIGatewayClient() == nil {
		return nil, kratoserr.ServiceUnavailable("AI_GATEWAY_NOT_CONFIGURED", "AI 网关客户端未配置")
	}

	result, err := s.uc.AIGatewayClient().DebugAI(
		ctx,
		req.GetAgentType(),
		req.GetPrompt(),
		req.GetParams(),
		req.GetModelOverride(),
	)
	if err != nil {
		return nil, err
	}

	return &adminv1.DebugAIResponse{
		Response:       result.Response,
		Model:          result.Model,
		TokensUsed:     int32(result.InputTokens + result.OutputTokens),
		LatencyMs:      result.LatencyMs,
		RenderedPrompt: result.RenderedPrompt,
		InputTokens:    int32(result.InputTokens),
		OutputTokens:   int32(result.OutputTokens),
		Error:          result.Error,
	}, nil
}

// ListAICallLogs 按单体后台相同的筛选条件查询 AI 调用日志列表。
func (s *AdminService) ListAICallLogs(ctx context.Context, req *adminv1.ListAICallLogsRequest) (*adminv1.ListAICallLogsResponse, error) {
	var page, pageSize int32 = 1, 20
	if req.Page != nil {
		page = req.Page.Page
		pageSize = req.Page.PageSize
	}

	var taskID *uint
	if req.TaskId > 0 {
		normalizedTaskID := uint(req.TaskId)
		taskID = &normalizedTaskID
	}
	logs, total, err := s.uc.ListAICallLogs(ctx, biz.AICallLogListFilter{
		Page:      page,
		PageSize:  pageSize,
		AgentType: req.AgentType,
		Scene:     req.Scene,
		Source:    req.Source,
		Status:    req.Status,
		TraceID:   req.TraceId,
		TaskID:    taskID,
	})
	if err != nil {
		return nil, err
	}
	items := make([]*adminv1.AICallLog, len(logs))
	for i, l := range logs {
		items[i] = &adminv1.AICallLog{
			Id:         l.ID,
			AgentType:  l.AgentType,
			Model:      l.Model,
			TokensUsed: l.TokensUsed,
			LatencyMs:  l.LatencyMs,
			Status:     l.Status,
			CreatedAt:  timestamppb.New(l.CreatedAt),
		}
	}
	return &adminv1.ListAICallLogsResponse{
		Logs: items,
		PageResult: &sharedv1.PageResult{
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		},
	}, nil
}

func (s *AdminService) GetAICallLog(ctx context.Context, req *adminv1.GetAICallLogRequest) (*adminv1.AICallLogDetail, error) {
	l, err := s.uc.GetAICallLog(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &adminv1.AICallLogDetail{
		Id:              l.ID,
		TraceId:         l.TraceID,
		Source:          l.Source,
		Scene:           l.Scene,
		Provider:        l.Provider,
		Model:           l.Model,
		UserInput:       l.UserInput,
		ModelOutput:     l.ModelOutput,
		ModelError:      l.ModelError,
		LatencyMs:       l.LatencyMs,
		IsSuccess:       l.IsSuccess,
		InputTokens:     int32(l.InputTokens),
		OutputTokens:    int32(l.OutputTokens),
		RenderedPrompt:  l.RenderedPrompt,
		RequestMessages: l.RequestMessages,
		RuntimeConfig:   l.RuntimeConfig,
		CreatedAt:       timestamppb.New(l.CreatedAt),
	}, nil
}

// ==================== Live2D 管理 ====================

// ListLive2DModels 返回后台管理页可维护的 Live2D 模型列表，并先同步本地发现结果。
func (s *AdminService) ListLive2DModels(ctx context.Context, _ *emptypb.Empty) (*adminv1.ListLive2DModelsResponse, error) {
	models, err := s.uc.ListManagedLive2DModels(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]*adminv1.Live2DModelInfo, len(models))
	for i, m := range models {
		items[i] = &adminv1.Live2DModelInfo{
			Id:           m.ID,
			Name:         m.Name,
			IndustryId:   m.IndustryID,
			Scene:        m.Scene,
			ModelUrl:     m.ModelURL,
			ThumbnailUrl: m.ThumbnailURL,
			ConfigJson:   m.ConfigJSON,
			TtsConfigId:  m.TTSConfigID,
			IsActive:     m.IsActive,
			CreatedAt:    timestamppb.New(m.CreatedAt),
		}
	}
	return &adminv1.ListLive2DModelsResponse{Models: items}, nil
}

// CreateLive2DModel 创建一条后台维护的 Live2D 模型记录。
func (s *AdminService) CreateLive2DModel(ctx context.Context, req *adminv1.CreateLive2DModelRequest) (*adminv1.Live2DModelInfo, error) {
	m := &biz.Live2DModelRecord{
		Name:         req.Name,
		IndustryID:   req.IndustryId,
		Scene:        req.Scene,
		ModelURL:     req.ModelUrl,
		ThumbnailURL: req.ThumbnailUrl,
		ConfigJSON:   req.ConfigJson,
		TTSConfigID:  req.TtsConfigId,
		IsActive:     req.IsActive,
	}
	if err := s.uc.CreateLive2DModel(ctx, m); err != nil {
		return nil, err
	}
	return &adminv1.Live2DModelInfo{Id: m.ID}, nil
}

// UpdateLive2DModel 更新指定的 Live2D 模型配置。
func (s *AdminService) UpdateLive2DModel(ctx context.Context, req *adminv1.UpdateLive2DModelRequest) (*emptypb.Empty, error) {
	m := &biz.Live2DModelRecord{
		ID:           req.Id,
		Name:         req.Name,
		IndustryID:   req.IndustryId,
		Scene:        req.Scene,
		ModelURL:     req.ModelUrl,
		ThumbnailURL: req.ThumbnailUrl,
		ConfigJSON:   req.ConfigJson,
		TTSConfigID:  req.TtsConfigId,
	}
	if req.IsActive != nil {
		m.IsActive = *req.IsActive
	}
	if err := s.uc.UpdateLive2DModel(ctx, m); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// DeleteLive2DModel 删除后台模型记录，并在无复用时同步清理受管资源目录。
func (s *AdminService) DeleteLive2DModel(ctx context.Context, req *adminv1.DeleteLive2DModelRequest) (*emptypb.Empty, error) {
	if err := s.uc.DeleteManagedLive2DModel(ctx, req.Id); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// ListSelectableLive2DModels 返回前台指定场景下可切换的 Live2D 模型列表。
func (s *AdminService) ListSelectableLive2DModels(ctx context.Context, req *adminv1.ListSelectableLive2DModelsRequest) (*adminv1.ListSelectableLive2DModelsResponse, error) {
	models, err := s.uc.ListSelectableLive2DModels(ctx, req.Scene, req.IndustryCode)
	if err != nil {
		return nil, err
	}

	items := make([]*adminv1.SelectableLive2DModel, 0, len(models))
	for _, model := range models {
		if model == nil {
			continue
		}
		items = append(items, &adminv1.SelectableLive2DModel{
			Key:           model.Key,
			Name:          model.Name,
			Scene:         model.Scene,
			ModelUrl:      model.ModelURL,
			ThumbnailUrl:  model.ThumbnailURL,
			ConfigJson:    model.ConfigJSON,
			Source:        model.Source,
			MatchType:     model.MatchType,
			IsGeneric:     model.IsGeneric,
			IsRecommended: model.IsRecommended,
			Motions:       toAdminLive2DMotionInfos(model.Motions),
		})
	}
	return &adminv1.ListSelectableLive2DModelsResponse{Models: items}, nil
}

// GetCurrentLive2DModel 返回前台当前场景应默认使用的 Live2D 模型。
func (s *AdminService) GetCurrentLive2DModel(ctx context.Context, req *adminv1.GetCurrentLive2DModelRequest) (*adminv1.CurrentLive2DModelResponse, error) {
	model, err := s.uc.GetCurrentLive2DModel(ctx, req.Scene, req.IndustryCode)
	if err != nil {
		return nil, err
	}

	config, err := structpb.NewStruct(model.Config)
	if err != nil {
		return nil, kratoserr.InternalServer("LIVE2D_CONFIG_CONVERT_FAILED", "convert live2d config failed")
	}
	return &adminv1.CurrentLive2DModelResponse{
		Name:         model.Name,
		Scene:        model.Scene,
		IndustryCode: model.IndustryCode,
		Path:         model.Path,
		ModelUrl:     model.ModelURL,
		ThumbnailUrl: model.ThumbnailURL,
		Config:       config,
		Source:       model.Source,
	}, nil
}

// ImportLive2DPackage 导入管理员上传的 Live2D ZIP 包，并自动生成待确认模型记录。
func (s *AdminService) ImportLive2DPackage(ctx context.Context, req *adminv1.ImportLive2DPackageRequest) (*adminv1.ImportLive2DPackageResponse, error) {
	resp, err := s.uc.ImportLive2DPackage(ctx, req.Filename, req.FileContent)
	if err != nil {
		return nil, err
	}
	return &adminv1.ImportLive2DPackageResponse{
		Name:         resp.Name,
		AssetDir:     resp.AssetDir,
		ModelUrl:     resp.ModelURL,
		ThumbnailUrl: resp.ThumbnailURL,
		ModelId:      resp.ModelID,
		Created:      resp.Created,
		IsActive:     resp.IsActive,
	}, nil
}

// ImportLive2DBackground 导入管理员上传的舞台背景图，并返回可直接回填的静态地址。
func (s *AdminService) ImportLive2DBackground(ctx context.Context, req *adminv1.ImportLive2DBackgroundRequest) (*adminv1.ImportLive2DBackgroundResponse, error) {
	resp, err := s.uc.ImportLive2DBackground(ctx, req.Filename, req.FileContent)
	if err != nil {
		return nil, err
	}
	return &adminv1.ImportLive2DBackgroundResponse{
		FileName: resp.FileName,
		AssetUrl: resp.AssetURL,
	}, nil
}

// toAdminLive2DMotionInfos 将领域动作条目映射为 proto 响应结构。
func toAdminLive2DMotionInfos(items []*biz.Live2DMotionInfo) []*adminv1.Live2DMotionInfo {
	result := make([]*adminv1.Live2DMotionInfo, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		result = append(result, &adminv1.Live2DMotionInfo{
			Key:   item.Key,
			Group: item.Group,
			File:  item.File,
			Label: item.Label,
		})
	}
	return result
}

// ==================== TTS 管理 ====================

func (s *AdminService) ListTTSConfigs(ctx context.Context, _ *emptypb.Empty) (*adminv1.ListTTSConfigsResponse, error) {
	configs, err := s.uc.ListTTSConfigs(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]*adminv1.TTSConfigInfo, len(configs))
	for i, c := range configs {
		items[i] = &adminv1.TTSConfigInfo{
			Id:             c.ID,
			Name:           c.Name,
			Engine:         c.Engine,
			VoiceId:        c.VoiceID,
			AuthConfigJson: c.AuthConfigJSON,
			ParamsJson:     c.ParamsJSON,
			IsActive:       c.IsActive,
			SortOrder:      c.SortOrder,
			CreatedAt:      timestamppb.New(c.CreatedAt),
		}
	}
	return &adminv1.ListTTSConfigsResponse{Configs: items}, nil
}

func (s *AdminService) CreateTTSConfig(ctx context.Context, req *adminv1.CreateTTSConfigRequest) (*adminv1.TTSConfigInfo, error) {
	t := &biz.TTSConfigRecord{
		Name:           req.Name,
		Engine:         req.Engine,
		VoiceID:        req.VoiceId,
		AuthConfigJSON: req.AuthConfigJson,
		ParamsJSON:     req.ParamsJson,
		IsActive:       req.IsActive,
		SortOrder:      req.SortOrder,
	}
	if err := s.uc.CreateTTSConfig(ctx, t); err != nil {
		return nil, err
	}
	return &adminv1.TTSConfigInfo{Id: t.ID}, nil
}

func (s *AdminService) UpdateTTSConfig(ctx context.Context, req *adminv1.UpdateTTSConfigRequest) (*emptypb.Empty, error) {
	t := &biz.TTSConfigRecord{
		ID:             req.Id,
		Name:           req.Name,
		Engine:         req.Engine,
		VoiceID:        req.VoiceId,
		AuthConfigJSON: req.AuthConfigJson,
		ParamsJSON:     req.ParamsJson,
		SortOrder:      req.SortOrder,
	}
	if req.IsActive != nil {
		t.IsActive = *req.IsActive
	}
	if err := s.uc.UpdateTTSConfig(ctx, t); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *AdminService) DeleteTTSConfig(ctx context.Context, req *adminv1.DeleteTTSConfigRequest) (*emptypb.Empty, error) {
	if err := s.uc.DeleteTTSConfig(ctx, req.Id); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *AdminService) UpdateTTSSceneDefaults(ctx context.Context, req *adminv1.UpdateTTSSceneDefaultsRequest) (*emptypb.Empty, error) {
	// 将 scene→tts_config_id 映射写入 admin_configs
	for scene, configID := range req.DefaultBindings {
		key := "tts_default_" + scene
		if err := s.uc.SetAdminConfig(ctx, key, fmt.Sprintf("%d", configID)); err != nil {
			return nil, err
		}
	}
	return &emptypb.Empty{}, nil
}

// ==================== RAG 配置 ====================

// extractRAGRuntimeConfig 从后台配置表单中提取 RAG 运行时需要的关键字段。
func extractRAGRuntimeConfig(configs map[string]string) (collectionName string, embedModel string, embedDim int32, err error) {
	collectionName = strings.TrimSpace(configs["rag_collection_name"])
	embedModel = strings.TrimSpace(configs["rag_embedding_model"])
	if embedDimRaw := strings.TrimSpace(configs["rag_embedding_dimension"]); embedDimRaw != "" {
		parsed, parseErr := strconv.ParseInt(embedDimRaw, 10, 32)
		if parseErr != nil {
			return "", "", 0, kratoserr.BadRequest("INVALID_RAG_EMBEDDING_DIMENSION", "rag_embedding_dimension 必须是整数")
		}
		embedDim = int32(parsed)
	}
	return collectionName, embedModel, embedDim, nil
}

func (s *AdminService) GetRAGConfigs(ctx context.Context, _ *emptypb.Empty) (*adminv1.GetRAGConfigsResponse, error) {
	collectionName, embedModel, configErr := s.uc.GetRAGConfig(ctx)
	configMap := map[string]string{
		"rag_collection_name": collectionName,
		"rag_embedding_model": embedModel,
	}
	items := []*adminv1.AdminConfigItem{
		{
			Key:         "rag_collection_name",
			Value:       collectionName,
			ConfigType:  "string",
			Description: "RAG 向量集合名称",
		},
		{
			Key:         "rag_embedding_model",
			Value:       embedModel,
			ConfigType:  "string",
			Description: "RAG Embedding 模型名称",
		},
	}

	// 从 RAG 服务获取系统状态
	status := &adminv1.RAGSystemStatus{Enabled: true}
	if configErr == nil {
		status.Collection = collectionName
		status.EmbedModel = embedModel
	} else {
		status.Enabled = false
	}
	milvusOk, _, connErr := s.uc.TestRAGConnection(ctx)
	status.MilvusConnected = milvusOk
	if connErr != nil {
		status.Enabled = false
	}

	return &adminv1.GetRAGConfigsResponse{
		Configs: configMap,
		Items:   items,
		Status:  status,
	}, nil
}

// UpdateRAGConfigs 将管理后台提交的配置委托给 RAG 服务，并同步持久化权威快照。
func (s *AdminService) UpdateRAGConfigs(ctx context.Context, req *adminv1.UpdateRAGConfigsRequest) (*emptypb.Empty, error) {
	collectionName, embedModel, embedDim, err := extractRAGRuntimeConfig(req.GetConfigs())
	if err != nil {
		return nil, err
	}
	updatedCollection, updatedDim, updatedModel, err := s.uc.UpdateRAGConfig(ctx, collectionName, embedDim, embedModel)
	if err != nil {
		return nil, err
	}
	if err := s.uc.BatchUpsertConfigs(ctx, map[string]string{
		"rag_collection_name":     updatedCollection,
		"rag_embedding_model":     updatedModel,
		"rag_embedding_dimension": strconv.FormatInt(int64(updatedDim), 10),
	}); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// TestRAGConnection 委托 RAG 服务测试连接
func (s *AdminService) TestRAGConnection(ctx context.Context, _ *emptypb.Empty) (*adminv1.TestRAGConnectionResponse, error) {
	milvusOk, embeddingOk, err := s.uc.TestRAGConnection(ctx)
	if err != nil {
		return &adminv1.TestRAGConnectionResponse{
			MilvusOk:    false,
			EmbeddingOk: false,
			Error:       err.Error(),
		}, nil
	}
	return &adminv1.TestRAGConnectionResponse{
		MilvusOk:    milvusOk,
		EmbeddingOk: embeddingOk,
	}, nil
}

// ==================== RAG 索引管理 ====================

// IndexAllQuestions 全量索引题目到 RAG（FIX C4: 返回 failed 计数）
func (s *AdminService) IndexAllQuestions(ctx context.Context, req *adminv1.IndexAllQuestionsRequest) (*adminv1.IndexResult, error) {
	indexed, failed, err := s.uc.IndexAllQuestions(ctx, req.IndustryId)
	if err != nil {
		return &adminv1.IndexResult{Error: err.Error()}, nil
	}
	return &adminv1.IndexResult{
		Indexed: indexed,
		Failed:  failed,
	}, nil
}

// IndexQuestions 索引指定题目到 RAG（FIX H1: 用 failed 字段而非 deleted）
func (s *AdminService) IndexQuestions(ctx context.Context, req *adminv1.IndexQuestionsRequest) (*adminv1.IndexResult, error) {
	indexed, failed, err := s.uc.IndexQuestions(ctx, req.QuestionIds)
	if err != nil {
		return &adminv1.IndexResult{Error: err.Error()}, nil
	}
	return &adminv1.IndexResult{
		Indexed: indexed,
		Failed:  failed,
	}, nil
}

// DeleteRAGIndex 删除 RAG 向量索引
func (s *AdminService) DeleteRAGIndex(ctx context.Context, req *adminv1.DeleteRAGIndexRequest) (*adminv1.IndexResult, error) {
	deleted, err := s.uc.DeleteRAGIndex(ctx, req.QuestionIds)
	if err != nil {
		return &adminv1.IndexResult{Error: err.Error()}, nil
	}
	return &adminv1.IndexResult{
		Deleted: deleted,
	}, nil
}

// SearchRAGQuestions RAG 语义检索
func (s *AdminService) SearchRAGQuestions(ctx context.Context, req *adminv1.SearchRAGQuestionsRequest) (*adminv1.SearchRAGQuestionsResponse, error) {
	topK := req.TopK
	if topK <= 0 {
		topK = 10
	}
	results, err := s.uc.SearchRAGQuestions(ctx, req.Query, topK)
	if err != nil {
		return nil, err
	}

	protoResults := make([]*adminv1.RAGSearchResult, len(results))
	for i, r := range results {
		protoResults[i] = &adminv1.RAGSearchResult{
			DocId:   r.DocID,
			Title:   r.Title,
			Content: r.Content,
			Score:   r.Score,
		}
	}
	return &adminv1.SearchRAGQuestionsResponse{
		Query:   req.Query,
		Results: protoResults,
	}, nil
}

// ==================== RAG 文档管理 ====================

func (s *AdminService) ListRAGDocuments(ctx context.Context, req *adminv1.ListRAGDocumentsRequest) (*adminv1.ListRAGDocumentsResponse, error) {
	var page, pageSize int32 = 1, 20
	if req.Page != nil {
		page = req.Page.Page
		pageSize = req.Page.PageSize
	}
	docs, total, err := s.uc.ListRAGDocuments(ctx, page, pageSize, req.Collection, req.DocType, req.Keyword, req.SyncStatus)
	if err != nil {
		return nil, err
	}
	items := make([]*adminv1.RAGDocumentDetail, len(docs))
	for i, d := range docs {
		items[i] = &adminv1.RAGDocumentDetail{
			Id:         d.ID,
			Collection: d.Collection,
			DocType:    d.DocType,
			Title:      d.Title,
			Content:    d.Content,
			Metadata:   d.Metadata,
			VectorId:   d.VectorID,
			SyncStatus: d.SyncStatus,
			IsActive:   d.IsActive,
			CreatedAt:  timestamppb.New(d.CreatedAt),
			UpdatedAt:  timestamppb.New(d.UpdatedAt),
		}
	}
	return &adminv1.ListRAGDocumentsResponse{
		Documents: items,
		PageResult: &sharedv1.PageResult{
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		},
	}, nil
}

func (s *AdminService) GetRAGDocumentStats(ctx context.Context, req *adminv1.GetRAGDocumentStatsRequest) (*adminv1.RAGDocumentStatsResponse, error) {
	stats, err := s.uc.GetRAGDocumentStats(ctx, req.Collection)
	if err != nil {
		return nil, err
	}
	return &adminv1.RAGDocumentStatsResponse{Stats: stats}, nil
}

func (s *AdminService) GetRAGDocument(ctx context.Context, req *adminv1.GetRAGDocumentRequest) (*adminv1.RAGDocumentDetail, error) {
	d, err := s.uc.GetRAGDocument(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &adminv1.RAGDocumentDetail{
		Id:         d.ID,
		Collection: d.Collection,
		DocType:    d.DocType,
		Title:      d.Title,
		Content:    d.Content,
		Metadata:   d.Metadata,
		VectorId:   d.VectorID,
		SyncStatus: d.SyncStatus,
		IsActive:   d.IsActive,
		CreatedAt:  timestamppb.New(d.CreatedAt),
		UpdatedAt:  timestamppb.New(d.UpdatedAt),
	}, nil
}

func (s *AdminService) CreateRAGDocument(ctx context.Context, req *adminv1.CreateRAGDocumentRequest) (*adminv1.RAGDocumentDetail, error) {
	metadataJSON, err := metadataJSONFromStringMap(req.Metadata)
	if err != nil {
		return nil, err
	}
	doc := &biz.RAGDocumentRecord{
		Collection: req.Collection,
		DocType:    req.DocType,
		Title:      req.Title,
		Content:    req.Content,
		Metadata:   metadataJSON,
		IsActive:   true,
	}
	if err := s.uc.CreateRAGDocument(ctx, doc); err != nil {
		return nil, err
	}
	return &adminv1.RAGDocumentDetail{
		Id:         doc.ID,
		Collection: doc.Collection,
		DocType:    doc.DocType,
		Title:      doc.Title,
		Content:    doc.Content,
		Metadata:   metadataJSON,
	}, nil
}

func (s *AdminService) UpdateRAGDocument(ctx context.Context, req *adminv1.UpdateRAGDocumentRequest) (*emptypb.Empty, error) {
	metadataJSON, err := metadataJSONFromStringMap(req.Metadata)
	if err != nil {
		return nil, err
	}
	doc := &biz.RAGDocumentRecord{
		ID:         req.Id,
		Collection: req.Collection,
		DocType:    req.DocType,
		Title:      req.Title,
		Content:    req.Content,
		Metadata:   metadataJSON,
	}
	if req.IsActive != nil {
		doc.IsActive = *req.IsActive
	}
	if err := s.uc.UpdateRAGDocument(ctx, doc); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *AdminService) DeleteRAGDocument(ctx context.Context, req *adminv1.DeleteRAGDocumentRequest) (*emptypb.Empty, error) {
	if err := s.uc.DeleteRAGDocument(ctx, req.Id); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *AdminService) BatchImportRAGDocuments(ctx context.Context, req *adminv1.BatchImportRAGDocumentsRequest) (*adminv1.BatchImportRAGDocumentsResponse, error) {
	docs := make([]*biz.RAGDocumentRecord, len(req.Documents))
	for i, d := range req.Documents {
		docs[i] = &biz.RAGDocumentRecord{
			Collection: req.Collection,
			DocType:    req.DocType,
			Title:      d.Title,
			Content:    d.Content,
			IsActive:   true,
		}
	}
	success, fail, errors := s.uc.BatchImportRAGDocuments(ctx, docs)
	return &adminv1.BatchImportRAGDocumentsResponse{
		Imported: int32(success),
		Failed:   int32(fail),
		Errors:   errors,
	}, nil
}

// SyncRAGDocumentsToVectorDB 同步指定文档到向量库
func (s *AdminService) SyncRAGDocumentsToVectorDB(ctx context.Context, req *adminv1.SyncRAGDocumentsRequest) (*emptypb.Empty, error) {
	if err := s.uc.SyncRAGDocumentsToVectorDB(ctx, req.Ids); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// SyncAllPendingRAGDocuments 同步所有待处理文档到向量库
func (s *AdminService) SyncAllPendingRAGDocuments(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	if err := s.uc.SyncAllPendingRAGDocuments(ctx); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// ==================== 面经爬虫 ====================

// GetScraperSources 返回可用的爬虫数据源列表。
func (s *AdminService) GetScraperSources(ctx context.Context, _ *emptypb.Empty) (*adminv1.GetScraperSourcesResponse, error) {
	sources, err := s.uc.GetScraperSources(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]*adminv1.ScraperSource, len(sources))
	for i, src := range sources {
		items[i] = &adminv1.ScraperSource{
			Name:     src.Name,
			Label:    src.Label,
			BaseUrl:  src.BaseURL,
			IsActive: src.IsActive,
		}
	}
	return &adminv1.GetScraperSourcesResponse{Sources: items}, nil
}

// ScraperSearch 在指定数据源中搜索面经内容。
func (s *AdminService) ScraperSearch(ctx context.Context, req *adminv1.ScraperSearchRequest) (*adminv1.ScraperSearchResponse, error) {
	if strings.TrimSpace(req.GetKeyword()) == "" {
		return nil, kratoserr.BadRequest("INVALID_KEYWORD", "搜索关键词不能为空")
	}
	if strings.TrimSpace(req.GetSource()) == "" {
		return nil, kratoserr.BadRequest("INVALID_SOURCE", "数据源不能为空")
	}
	results, total, err := s.uc.ScraperSearch(ctx, req.GetSource(), req.GetKeyword(), req.GetPage(), req.GetPageSize())
	if err != nil {
		return nil, err
	}
	items := make([]*adminv1.ScraperSearchResult, len(results))
	for i, r := range results {
		items[i] = &adminv1.ScraperSearchResult{
			Title:   r.Title,
			Url:     r.URL,
			Source:  r.Source,
			Snippet: r.Snippet,
		}
	}
	return &adminv1.ScraperSearchResponse{Results: items, Total: total}, nil
}

// ScraperFetch 抓取指定 URL 的面经内容并留痕。
func (s *AdminService) ScraperFetch(ctx context.Context, req *adminv1.ScraperFetchRequest) (*adminv1.ScraperFetchResponse, error) {
	if strings.TrimSpace(req.GetUrl()) == "" {
		return nil, kratoserr.BadRequest("INVALID_URL", "抓取 URL 不能为空")
	}
	if strings.TrimSpace(req.GetSource()) == "" {
		return nil, kratoserr.BadRequest("INVALID_SOURCE", "数据源不能为空")
	}
	result, err := s.uc.ScraperFetch(ctx, req.GetSource(), req.GetUrl())
	if err != nil {
		return nil, err
	}
	return &adminv1.ScraperFetchResponse{
		Title:   result.Title,
		Content: result.Content,
		Source:  result.Source,
		Url:     result.URL,
	}, nil
}

// ScraperClean 清洗面经内容，提取结构化题目。
func (s *AdminService) ScraperClean(ctx context.Context, req *adminv1.ScraperCleanRequest) (*adminv1.ScraperCleanResponse, error) {
	if strings.TrimSpace(req.GetContent()) == "" {
		return nil, kratoserr.BadRequest("INVALID_CONTENT", "清洗内容不能为空")
	}
	questions, total := s.uc.ScraperClean(ctx, req.GetContent(), req.GetIndustryCode(), req.GetSource(), req.GetSourceUrl())
	items := make([]*adminv1.ScraperCleanedQuestion, len(questions))
	for i, q := range questions {
		items[i] = &adminv1.ScraperCleanedQuestion{
			CategoryName: q.CategoryName,
			Type:         q.Type,
			Difficulty:   q.Difficulty,
			Title:        q.Title,
			Content:      q.Content,
			OptionsJson:  q.OptionsJSON,
			Answer:       q.Answer,
			Explanation:  q.Explanation,
			Tags:         q.Tags,
		}
	}
	return &adminv1.ScraperCleanResponse{
		Questions:      items,
		TotalExtracted: int32(total),
	}, nil
}

// ScraperImport 同步导入清洗后的题目到题库。
func (s *AdminService) ScraperImport(ctx context.Context, req *adminv1.ScraperImportRequest) (*adminv1.ScraperImportResponse, error) {
	if strings.TrimSpace(req.GetIndustryCode()) == "" {
		return nil, kratoserr.BadRequest("INVALID_INDUSTRY_CODE", "行业编码不能为空")
	}
	if len(req.GetQuestions()) == 0 {
		return nil, kratoserr.BadRequest("INVALID_QUESTIONS", "至少需要一题才能导入")
	}
	questions := make([]*biz.ScraperCleanedQuestionRecord, len(req.GetQuestions()))
	for i, q := range req.GetQuestions() {
		questions[i] = &biz.ScraperCleanedQuestionRecord{
			CategoryName: q.GetCategoryName(),
			Type:         q.GetType(),
			Difficulty:   q.GetDifficulty(),
			Title:        q.GetTitle(),
			Content:      q.GetContent(),
			OptionsJSON:  q.GetOptionsJson(),
			Answer:       q.GetAnswer(),
			Explanation:  q.GetExplanation(),
			Tags:         q.GetTags(),
		}
	}
	result, err := s.uc.ScraperImport(ctx, req.GetIndustryCode(), questions)
	if err != nil {
		return nil, err
	}
	return &adminv1.ScraperImportResponse{
		TotalCount:   int32(result.TotalCount),
		SuccessCount: int32(result.SuccessCount),
		FailCount:    int32(result.FailCount),
		Errors:       result.Errors,
	}, nil
}

// ScraperImportAsync 异步导入清洗后的题目，创建任务记录并投递 MQ。
func (s *AdminService) ScraperImportAsync(ctx context.Context, req *adminv1.ScraperImportRequest) (*adminv1.ScraperTaskInfo, error) {
	if strings.TrimSpace(req.GetIndustryCode()) == "" {
		return nil, kratoserr.BadRequest("INVALID_INDUSTRY_CODE", "行业编码不能为空")
	}
	if len(req.GetQuestions()) == 0 {
		return nil, kratoserr.BadRequest("INVALID_QUESTIONS", "至少需要一题才能创建导入任务")
	}
	questions := make([]*biz.ScraperCleanedQuestionRecord, len(req.GetQuestions()))
	for i, q := range req.GetQuestions() {
		questions[i] = &biz.ScraperCleanedQuestionRecord{
			CategoryName: q.GetCategoryName(),
			Type:         q.GetType(),
			Difficulty:   q.GetDifficulty(),
			Title:        q.GetTitle(),
			Content:      q.GetContent(),
			OptionsJSON:  q.GetOptionsJson(),
			Answer:       q.GetAnswer(),
			Explanation:  q.GetExplanation(),
			Tags:         q.GetTags(),
		}
	}
	task, err := s.uc.ScraperImportAsync(ctx, "admin", req.GetIndustryCode(), req.GetSourceUrl(), req.GetSourceTitle(), questions)
	if err != nil {
		return nil, err
	}
	return &adminv1.ScraperTaskInfo{
		TaskId: task.ID,
		Status: task.Status,
	}, nil
}

// ListScraperTasks 分页返回 scraper_tasks 列表，供后台任务中心与轮询页面展示。
func (s *AdminService) ListScraperTasks(ctx context.Context, req *adminv1.ListScraperTasksRequest) (*adminv1.ListScraperTasksResponse, error) {
	var page, pageSize int32 = 1, 20
	if req.Page != nil {
		page = req.Page.Page
		pageSize = req.Page.PageSize
	}
	tasks, total, err := s.uc.ListScraperTasks(ctx, page, pageSize, req.Status, req.TaskType)
	if err != nil {
		return nil, err
	}
	items := make([]*adminv1.ScraperTaskDetail, len(tasks))
	for i, t := range tasks {
		items[i] = buildScraperTaskDetail(t)
	}
	return &adminv1.ListScraperTasksResponse{
		Tasks: items,
		PageResult: &sharedv1.PageResult{
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		},
	}, nil
}

// GetScraperTask 返回单条 scraper_tasks 详情，供 Gateway SSE 与后台任务详情页轮询。
func (s *AdminService) GetScraperTask(ctx context.Context, req *adminv1.GetScraperTaskRequest) (*adminv1.ScraperTaskDetail, error) {
	t, err := s.uc.GetScraperTask(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return buildScraperTaskDetail(t), nil
}

// UpdateQuestionPipelineTask 接收 question 服务的异步任务状态回写，并在终态后拒绝重复覆盖。
func (s *AdminService) UpdateQuestionPipelineTask(ctx context.Context, req *adminv1.UpdateQuestionPipelineTaskRequest) (*adminv1.UpdateQuestionPipelineTaskResponse, error) {
	if req.GetTaskId() == 0 {
		return nil, kratoserr.BadRequest("INVALID_TASK_ID", "task_id is required")
	}
	task, err := s.uc.GetScraperTask(ctx, req.GetTaskId())
	if err != nil {
		return nil, err
	}
	if !isManagedAsyncTaskType(task.TaskType) {
		return nil, status.Error(codes.FailedPrecondition, "任务类型不支持状态回写")
	}
	if isManagedAsyncTaskTerminalStatus(task.Status) {
		return &adminv1.UpdateQuestionPipelineTaskResponse{
			Applied: false,
			Task:    buildScraperTaskDetail(task),
		}, nil
	}

	if statusText := strings.TrimSpace(req.GetStatus()); statusText != "" {
		task.Status = statusText
	}
	if req.QuestionCount > 0 {
		task.QuestionCount = int(req.GetQuestionCount())
	}
	if req.ImportedCount >= 0 {
		task.ImportedCount = int(req.GetImportedCount())
	}
	if req.ErrorMsg != "" {
		task.ErrorMsg = req.GetErrorMsg()
	}
	if req.ResultJson != "" {
		task.ResultJSON = req.GetResultJson()
	}
	if req.StartedAt != nil {
		startedAt := req.GetStartedAt().AsTime()
		task.StartedAt = &startedAt
	}
	if req.FinishedAt != nil {
		finishedAt := req.GetFinishedAt().AsTime()
		task.FinishedAt = &finishedAt
	}
	if err := s.uc.UpdateScraperTask(ctx, task); err != nil {
		return nil, err
	}
	return &adminv1.UpdateQuestionPipelineTaskResponse{
		Applied: true,
		Task:    buildScraperTaskDetail(task),
	}, nil
}

// RetryScraperTask 将失败任务重置为 pending，并重新投递对应的异步任务。
func (s *AdminService) RetryScraperTask(ctx context.Context, req *adminv1.RetryScraperTaskRequest) (*adminv1.ScraperTaskInfo, error) {
	task, err := s.uc.GetScraperTask(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	task.Status = "pending"
	task.RetryCount++
	task.ImportedCount = 0
	task.ErrorMsg = ""
	task.ResultJSON = ""
	task.StartedAt = nil
	task.FinishedAt = nil
	if err := s.uc.UpdateScraperTask(ctx, task); err != nil {
		return nil, err
	}
	if s.pipelinePublisher == nil {
		return nil, kratoserr.ServiceUnavailable("MQ_UNAVAILABLE", "异步消息发布器未配置")
	}
	switch task.TaskType {
	case questionPipelineTaskType:
		pipelineReq := &adminv1.GenerateQuestionPipelineRequest{}
		if err := json.Unmarshal([]byte(task.PayloadJSON), pipelineReq); err != nil {
			return nil, kratoserr.InternalServer("PIPELINE_PAYLOAD_DECODE_FAILED", "题目流水线任务载荷解析失败")
		}
		if err := s.pipelinePublisher.PublishQuestionPipelineBuild(ctx, task.ID, pipelineReq); err != nil {
			task.Status = "failed"
			task.ErrorMsg = err.Error()
			finishedAt := time.Now()
			task.FinishedAt = &finishedAt
			_ = s.uc.UpdateScraperTask(ctx, task)
			return nil, kratoserr.InternalServer("PIPELINE_TASK_RETRY_FAILED", "题目流水线重试投递失败")
		}
	case "import_questions":
		if err := s.pipelinePublisher.PublishScraperImport(ctx, task.ID, []byte(task.PayloadJSON)); err != nil {
			task.Status = "failed"
			task.ErrorMsg = err.Error()
			finishedAt := time.Now()
			task.FinishedAt = &finishedAt
			_ = s.uc.UpdateScraperTask(ctx, task)
			return nil, kratoserr.InternalServer("SCRAPER_IMPORT_RETRY_FAILED", "爬虫导入任务重试投递失败")
		}
	default:
		return nil, status.Error(codes.FailedPrecondition, "任务类型不支持重试")
	}
	return &adminv1.ScraperTaskInfo{TaskId: task.ID, Status: "pending"}, nil
}

// buildScraperTaskDetail 将任务领域对象映射为对外 gRPC DTO。
func buildScraperTaskDetail(task *biz.ScraperTaskRecord) *adminv1.ScraperTaskDetail {
	item := &adminv1.ScraperTaskDetail{
		Id:            task.ID,
		TaskType:      task.TaskType,
		SourceUrl:     task.SourceURL,
		SourceTitle:   task.SourceTitle,
		Source:        task.Source,
		Status:        task.Status,
		QuestionCount: int32(task.QuestionCount),
		ImportedCount: int32(task.ImportedCount),
		RetryCount:    int32(task.RetryCount),
		ErrorMsg:      task.ErrorMsg,
		ResultJson:    task.ResultJSON,
		CreatedAt:     timestamppb.New(task.CreatedAt),
		UpdatedAt:     timestamppb.New(task.UpdatedAt),
	}
	if task.StartedAt != nil {
		item.StartedAt = timestamppb.New(*task.StartedAt)
	}
	if task.FinishedAt != nil {
		item.FinishedAt = timestamppb.New(*task.FinishedAt)
	}
	return item
}

// normalizeQuestionPipelineRequest 规范化题目流水线请求，并校验关键输入不为空。
func normalizeQuestionPipelineRequest(req *adminv1.GenerateQuestionPipelineRequest) (*adminv1.GenerateQuestionPipelineRequest, error) {
	if req == nil {
		return nil, kratoserr.BadRequest("INVALID_ARGUMENT", "request is required")
	}
	industryCode := strings.TrimSpace(req.GetIndustryCode())
	if industryCode == "" {
		return nil, kratoserr.BadRequest("INVALID_INDUSTRY_CODE", "industry_code is required")
	}
	requirement := strings.TrimSpace(req.GetRequirement())
	if requirement == "" {
		return nil, kratoserr.BadRequest("INVALID_REQUIREMENT", "requirement is required")
	}
	candidateCount := req.GetCandidateCount()
	if candidateCount <= 0 {
		candidateCount = 5
	}
	generationMode := strings.TrimSpace(req.GetGenerationMode())
	if generationMode == "" {
		generationMode = "standard"
	}
	sources := make([]string, 0, len(req.GetSources()))
	for _, source := range req.GetSources() {
		source = strings.TrimSpace(source)
		if source != "" {
			sources = append(sources, source)
		}
	}
	return &adminv1.GenerateQuestionPipelineRequest{
		IndustryCode:     industryCode,
		Requirement:      requirement,
		AgentPrompt:      strings.TrimSpace(req.GetAgentPrompt()),
		GenerationMode:   generationMode,
		CandidateCount:   candidateCount,
		IncludeScraped:   req.GetIncludeScraped(),
		IncludeGenerated: req.GetIncludeGenerated(),
		Sources:          sources,
	}, nil
}

// buildQuestionPipelineTaskTitle 生成后台任务列表展示用的简短标题。
func buildQuestionPipelineTaskTitle(requirement string) string {
	requirement = strings.TrimSpace(requirement)
	if requirement == "" {
		return "题目流水线生成任务"
	}
	runes := []rune(requirement)
	if len(runes) <= 24 {
		return requirement
	}
	return string(runes[:24]) + "..."
}

// isManagedAsyncTaskType 判断任务是否属于当前允许由 question 服务回写的异步任务类型。
func isManagedAsyncTaskType(taskType string) bool {
	switch strings.TrimSpace(taskType) {
	case questionPipelineTaskType, "import_questions":
		return true
	default:
		return false
	}
}

// isManagedAsyncTaskTerminalStatus 判断当前任务是否已经进入不可覆盖的终态。
func isManagedAsyncTaskTerminalStatus(statusText string) bool {
	switch strings.TrimSpace(statusText) {
	case "completed", "failed":
		return true
	default:
		return false
	}
}

// ==================== 系统配置 ====================

func (s *AdminService) GetAdminConfig(ctx context.Context, req *adminv1.GetAdminConfigRequest) (*adminv1.AdminConfigValue, error) {
	value, err := s.uc.GetAdminConfig(ctx, req.Key)
	if err != nil {
		return nil, err
	}
	return &adminv1.AdminConfigValue{
		Key:   req.Key,
		Value: value,
	}, nil
}

func (s *AdminService) SetAdminConfig(ctx context.Context, req *adminv1.SetAdminConfigRequest) (*emptypb.Empty, error) {
	if err := s.uc.SetAdminConfig(ctx, req.Key, req.Value); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
