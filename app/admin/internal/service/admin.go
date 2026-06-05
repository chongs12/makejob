package service

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"makejob-backend/bridge"
	adminv1 "makejob/api/makejob/admin/v1"
	sharedv1 "makejob/api/makejob/shared/v1"
	"makejob/app/admin/internal/biz"
)

// AdminService 实现 gRPC AdminServiceServer
type AdminService struct {
	adminv1.UnimplementedAdminServiceServer
	uc            *biz.AdminUseCase
	backendBridge *bridge.Runtime
}

// NewAdminService 创建管理后台服务
func NewAdminService(uc *biz.AdminUseCase, backendBridge *bridge.Runtime) *AdminService {
	return &AdminService{uc: uc, backendBridge: backendBridge}
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

// GenerateQuestionPipeline 调用 backend bridge 生成真实题目流水线结果。
func (s *AdminService) GenerateQuestionPipeline(ctx context.Context, req *adminv1.GenerateQuestionPipelineRequest) (*adminv1.GenerateQuestionPipelineResponse, error) {
	backendBridge, err := s.requireBackendBridge()
	if err != nil {
		return nil, err
	}
	resp, err := backendBridge.GenerateQuestionPipeline(ctx, buildBridgePipelineRequest(req))
	if err != nil {
		return nil, err
	}
	return buildPipelineResponse(resp)
}

// GenerateQuestionPipelineAsync 调用 backend bridge 创建异步题目流水线任务。
func (s *AdminService) GenerateQuestionPipelineAsync(ctx context.Context, req *adminv1.GenerateQuestionPipelineRequest) (*adminv1.PipelineTaskInfo, error) {
	backendBridge, err := s.requireBackendBridge()
	if err != nil {
		return nil, err
	}
	task, err := backendBridge.CreateQuestionPipelineTask(ctx, buildBridgePipelineRequest(req))
	if err != nil {
		return nil, err
	}
	return &adminv1.PipelineTaskInfo{TaskId: task.TaskID, Status: task.Status}, nil
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

// TestRenderPrompt 调用 backend bridge 渲染并可选执行真实提示词调试。
func (s *AdminService) TestRenderPrompt(ctx context.Context, req *adminv1.TestRenderPromptRequest) (*adminv1.TestRenderPromptResponse, error) {
	backendBridge, err := s.requireBackendBridge()
	if err != nil {
		return nil, err
	}
	resp, err := backendBridge.TestRenderPrompt(ctx, buildBridgeAIDebugRequest(req.GetAgentType(), req.GetPrompt(), req.GetParams()))
	if err != nil {
		return nil, err
	}
	return &adminv1.TestRenderPromptResponse{
		RenderedPrompt: resp.RenderedPrompt,
		Response:       resp.Response,
		Model:          resp.Model,
		LatencyMs:      resp.LatencyMS,
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
	baseConfigs := map[string]string{}
	if s.backendBridge != nil && s.backendBridge.Config() != nil {
		baseConfigs = s.backendBridge.Config().AIRuntimeDefaults()
	}
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
			Id:       p.ID,
			Name:     p.Name,
			Configs:  p.Configs,
			IsActive: p.IsActive,
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

// UpdateAIConfigs 复用 bridge 暴露的单体规则校验并保存 AI 运行配置。
func (s *AdminService) UpdateAIConfigs(ctx context.Context, req *adminv1.UpdateAIConfigsRequest) (*emptypb.Empty, error) {
	normalizedConfigs, err := normalizeAdminAIConfigs(req.Configs)
	if err != nil {
		return nil, grpcstatus.Error(codes.InvalidArgument, err.Error())
	}
	if err := s.uc.BatchUpsertConfigs(ctx, normalizedConfigs); err != nil {
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
		Id:       preset.ID,
		Name:     preset.Name,
		Configs:  preset.Configs,
		IsActive: preset.IsActive,
	}, nil
}

func (s *AdminService) UpdateAIPreset(ctx context.Context, req *adminv1.UpdateAIPresetRequest) (*adminv1.AIPreset, error) {
	currentPreset, err := s.uc.GetAIPresetByID(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	if currentPreset == nil {
		return nil, fmt.Errorf("ai preset %d not found", req.Id)
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
		Id:       updatedPreset.ID,
		Name:     updatedPreset.Name,
		Configs:  updatedPreset.Configs,
		IsActive: updatedPreset.IsActive,
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

// DebugAI 调用 backend bridge 执行真实大模型调试。
func (s *AdminService) DebugAI(ctx context.Context, req *adminv1.DebugAIRequest) (*adminv1.DebugAIResponse, error) {
	backendBridge, err := s.requireBackendBridge()
	if err != nil {
		return nil, err
	}
	resp, err := backendBridge.DebugAI(ctx, buildBridgeAIDebugRequest(req.GetAgentType(), req.GetPrompt(), req.GetParams()))
	if err != nil {
		return nil, err
	}
	return &adminv1.DebugAIResponse{
		Response:   resp.Response,
		Model:      resp.Model,
		TokensUsed: resp.TokensUsed,
		LatencyMs:  resp.LatencyMS,
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

func (s *AdminService) ListLive2DModels(ctx context.Context, _ *emptypb.Empty) (*adminv1.ListLive2DModelsResponse, error) {
	models, err := s.uc.ListLive2DModels(ctx)
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

func (s *AdminService) DeleteLive2DModel(ctx context.Context, req *adminv1.DeleteLive2DModelRequest) (*emptypb.Empty, error) {
	if err := s.uc.DeleteLive2DModel(ctx, req.Id); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// ImportLive2DPackage 调用 backend bridge 导入真实 Live2D 模型包。
func (s *AdminService) ImportLive2DPackage(ctx context.Context, req *adminv1.ImportLive2DPackageRequest) (*adminv1.ImportLive2DPackageResponse, error) {
	backendBridge, err := s.requireBackendBridge()
	if err != nil {
		return nil, err
	}
	resp, err := backendBridge.ImportLive2DPackage(ctx, req.GetFilename(), req.GetFileContent())
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

// ImportLive2DBackground 调用 backend bridge 导入真实 Live2D 背景资源。
func (s *AdminService) ImportLive2DBackground(ctx context.Context, req *adminv1.ImportLive2DBackgroundRequest) (*adminv1.ImportLive2DBackgroundResponse, error) {
	backendBridge, err := s.requireBackendBridge()
	if err != nil {
		return nil, err
	}
	resp, err := backendBridge.ImportLive2DBackground(ctx, req.GetFilename(), req.GetFileContent())
	if err != nil {
		return nil, err
	}
	return &adminv1.ImportLive2DBackgroundResponse{
		FileName: resp.FileName,
		AssetUrl: resp.AssetURL,
	}, nil
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

func (s *AdminService) GetRAGConfigs(ctx context.Context, _ *emptypb.Empty) (*adminv1.GetRAGConfigsResponse, error) {
	configs, err := s.uc.ListAdminConfigs(ctx)
	if err != nil {
		return nil, err
	}
	configMap := make(map[string]string)
	items := make([]*adminv1.AdminConfigItem, 0)
	for _, c := range configs {
		if len(c.Key) >= 4 && c.Key[:4] == "rag_" {
			configMap[c.Key] = c.Value
			items = append(items, &adminv1.AdminConfigItem{
				Key:         c.Key,
				Value:       c.Value,
				ConfigType:  c.ConfigType,
				Description: c.Description,
			})
		}
	}
	return &adminv1.GetRAGConfigsResponse{
		Configs: configMap,
		Items:   items,
	}, nil
}

func (s *AdminService) UpdateRAGConfigs(ctx context.Context, req *adminv1.UpdateRAGConfigsRequest) (*emptypb.Empty, error) {
	if err := s.uc.BatchUpsertConfigs(ctx, req.Configs); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// TestRAGConnection 调用 backend bridge 检查真实 RAG 依赖连接状态。
func (s *AdminService) TestRAGConnection(ctx context.Context, _ *emptypb.Empty) (*adminv1.TestRAGConnectionResponse, error) {
	backendBridge, err := s.requireBackendBridge()
	if err != nil {
		return nil, err
	}
	resp, err := backendBridge.TestRAGConnection(ctx)
	if err != nil {
		return nil, err
	}
	return &adminv1.TestRAGConnectionResponse{
		MilvusOk:    resp.MilvusOK,
		EmbeddingOk: resp.EmbeddingOK,
		Error:       resp.Error,
	}, nil
}

// ==================== RAG 索引管理 ====================

// IndexAllQuestions 调用 backend bridge 为题库建立真实 RAG 索引。
func (s *AdminService) IndexAllQuestions(ctx context.Context, req *adminv1.IndexAllQuestionsRequest) (*adminv1.IndexResult, error) {
	backendBridge, err := s.requireBackendBridge()
	if err != nil {
		return nil, err
	}
	resp, err := backendBridge.IndexAllQuestions(ctx, req.GetIndustryId())
	if err != nil {
		return nil, err
	}
	return &adminv1.IndexResult{Indexed: resp.Indexed, Deleted: resp.Deleted}, nil
}

// IndexQuestions 调用 backend bridge 为指定题目建立真实 RAG 索引。
func (s *AdminService) IndexQuestions(ctx context.Context, req *adminv1.IndexQuestionsRequest) (*adminv1.IndexResult, error) {
	backendBridge, err := s.requireBackendBridge()
	if err != nil {
		return nil, err
	}
	resp, err := backendBridge.IndexQuestions(ctx, req.GetQuestionIds())
	if err != nil {
		return nil, err
	}
	return &adminv1.IndexResult{Indexed: resp.Indexed, Deleted: resp.Deleted}, nil
}

// DeleteRAGIndex 调用 backend bridge 删除真实向量索引。
func (s *AdminService) DeleteRAGIndex(ctx context.Context, req *adminv1.DeleteRAGIndexRequest) (*adminv1.IndexResult, error) {
	backendBridge, err := s.requireBackendBridge()
	if err != nil {
		return nil, err
	}
	resp, err := backendBridge.DeleteRAGIndex(ctx, req.GetQuestionIds())
	if err != nil {
		return nil, err
	}
	return &adminv1.IndexResult{Indexed: resp.Indexed, Deleted: resp.Deleted}, nil
}

// SearchRAGQuestions 调用 backend bridge 执行真实 RAG 检索。
func (s *AdminService) SearchRAGQuestions(ctx context.Context, req *adminv1.SearchRAGQuestionsRequest) (*adminv1.SearchRAGQuestionsResponse, error) {
	backendBridge, err := s.requireBackendBridge()
	if err != nil {
		return nil, err
	}
	resp, err := backendBridge.SearchRAGQuestions(ctx, req.GetQuery(), req.GetTopK())
	if err != nil {
		return nil, err
	}
	results := make([]*adminv1.RAGSearchResult, 0, len(resp.Results))
	for _, item := range resp.Results {
		results = append(results, &adminv1.RAGSearchResult{
			DocId:    item.DocID,
			Title:    item.Title,
			Content:  item.Content,
			Score:    item.Score,
			Metadata: structFromMap(item.Metadata),
		})
	}
	return &adminv1.SearchRAGQuestionsResponse{Query: resp.Query, Results: results}, nil
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

// SyncRAGDocumentsToVectorDB 调用 backend bridge 同步指定文档到向量库。
func (s *AdminService) SyncRAGDocumentsToVectorDB(ctx context.Context, req *adminv1.SyncRAGDocumentsRequest) (*emptypb.Empty, error) {
	backendBridge, err := s.requireBackendBridge()
	if err != nil {
		return nil, err
	}
	if err := backendBridge.SyncRAGDocumentsToVectorDB(ctx, req.GetIds()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// SyncAllPendingRAGDocuments 调用 backend bridge 同步全部待处理 RAG 文档。
func (s *AdminService) SyncAllPendingRAGDocuments(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	backendBridge, err := s.requireBackendBridge()
	if err != nil {
		return nil, err
	}
	if err := backendBridge.SyncAllPendingRAGDocuments(ctx); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// ==================== 面经爬虫 ====================

// GetScraperSources 调用 backend bridge 返回真实爬虫源配置。
func (s *AdminService) GetScraperSources(ctx context.Context, _ *emptypb.Empty) (*adminv1.GetScraperSourcesResponse, error) {
	backendBridge, err := s.requireBackendBridge()
	if err != nil {
		return nil, err
	}
	sources, err := backendBridge.GetScraperSources(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]*adminv1.ScraperSource, 0, len(sources))
	for _, source := range sources {
		items = append(items, &adminv1.ScraperSource{Name: source.Name, Label: source.Label, BaseUrl: source.BaseURL, IsActive: source.IsActive})
	}
	return &adminv1.GetScraperSourcesResponse{Sources: items}, nil
}

// ScraperSearch 调用 backend bridge 执行真实外部搜索。
func (s *AdminService) ScraperSearch(ctx context.Context, req *adminv1.ScraperSearchRequest) (*adminv1.ScraperSearchResponse, error) {
	backendBridge, err := s.requireBackendBridge()
	if err != nil {
		return nil, err
	}
	results, err := backendBridge.ScraperSearch(ctx, bridge.ScraperSearchRequest{Keyword: req.GetKeyword(), Source: req.GetSource(), Page: req.GetPage(), PageSize: req.GetPageSize()})
	if err != nil {
		return nil, err
	}
	items := make([]*adminv1.ScraperSearchResult, 0, len(results))
	for _, item := range results {
		items = append(items, &adminv1.ScraperSearchResult{Title: item.Title, Url: item.URL, Source: item.Source, Snippet: item.Snippet})
	}
	return &adminv1.ScraperSearchResponse{Results: items}, nil
}

// ScraperFetch 调用 backend bridge 抓取真实外部详情页。
func (s *AdminService) ScraperFetch(ctx context.Context, req *adminv1.ScraperFetchRequest) (*adminv1.ScraperFetchResponse, error) {
	backendBridge, err := s.requireBackendBridge()
	if err != nil {
		return nil, err
	}
	resp, err := backendBridge.ScraperFetch(ctx, bridge.ScraperFetchRequest{URL: req.GetUrl(), Source: req.GetSource()})
	if err != nil {
		return nil, err
	}
	return &adminv1.ScraperFetchResponse{Title: resp.Title, Content: resp.Content, Source: resp.Source, Url: resp.URL}, nil
}

// ScraperClean 调用 backend bridge 清洗真实爬取文本。
func (s *AdminService) ScraperClean(ctx context.Context, req *adminv1.ScraperCleanRequest) (*adminv1.ScraperCleanResponse, error) {
	backendBridge, err := s.requireBackendBridge()
	if err != nil {
		return nil, err
	}
	resp, err := backendBridge.ScraperClean(ctx, bridge.ScraperCleanRequest{Content: req.GetContent(), IndustryCode: req.GetIndustryCode(), Source: req.GetSource(), SourceURL: req.GetSourceUrl()})
	if err != nil {
		return nil, err
	}
	items := make([]*adminv1.ScraperCleanedQuestion, 0, len(resp.Questions))
	for _, item := range resp.Questions {
		items = append(items, &adminv1.ScraperCleanedQuestion{CategoryName: item.CategoryName, Type: item.Type, Difficulty: item.Difficulty, Title: item.Title, Content: item.Content, OptionsJson: item.OptionsJSON, Answer: item.Answer, Explanation: item.Explanation, Tags: item.Tags})
	}
	return &adminv1.ScraperCleanResponse{Questions: items, TotalExtracted: resp.TotalExtracted}, nil
}

// ScraperImport 调用 backend bridge 导入清洗后的真实题目。
func (s *AdminService) ScraperImport(ctx context.Context, req *adminv1.ScraperImportRequest) (*adminv1.ScraperImportResponse, error) {
	backendBridge, err := s.requireBackendBridge()
	if err != nil {
		return nil, err
	}
	resp, err := backendBridge.ScraperImport(ctx, buildBridgeScraperImportRequest(req))
	if err != nil {
		return nil, err
	}
	return &adminv1.ScraperImportResponse{TotalCount: resp.TotalCount, SuccessCount: resp.SuccessCount, FailCount: resp.FailCount, Errors: resp.Errors}, nil
}

// ScraperImportAsync 调用 backend bridge 创建真实异步导入任务。
func (s *AdminService) ScraperImportAsync(ctx context.Context, req *adminv1.ScraperImportRequest) (*adminv1.ScraperTaskInfo, error) {
	backendBridge, err := s.requireBackendBridge()
	if err != nil {
		return nil, err
	}
	task, err := backendBridge.ScraperImportAsync(ctx, buildBridgeScraperImportRequest(req))
	if err != nil {
		return nil, err
	}
	return &adminv1.ScraperTaskInfo{TaskId: task.TaskID, Status: task.Status}, nil
}

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
		item := &adminv1.ScraperTaskDetail{
			Id:            t.ID,
			TaskType:      t.TaskType,
			SourceUrl:     t.SourceURL,
			SourceTitle:   t.SourceTitle,
			Source:        t.Source,
			Status:        t.Status,
			QuestionCount: int32(t.QuestionCount),
			ImportedCount: int32(t.ImportedCount),
			RetryCount:    int32(t.RetryCount),
			ErrorMsg:      t.ErrorMsg,
			CreatedAt:     timestamppb.New(t.CreatedAt),
			UpdatedAt:     timestamppb.New(t.UpdatedAt),
		}
		if t.StartedAt != nil {
			item.StartedAt = timestamppb.New(*t.StartedAt)
		}
		if t.FinishedAt != nil {
			item.FinishedAt = timestamppb.New(*t.FinishedAt)
		}
		items[i] = item
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

func (s *AdminService) GetScraperTask(ctx context.Context, req *adminv1.GetScraperTaskRequest) (*adminv1.ScraperTaskDetail, error) {
	t, err := s.uc.GetScraperTask(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	item := &adminv1.ScraperTaskDetail{
		Id:            t.ID,
		TaskType:      t.TaskType,
		SourceUrl:     t.SourceURL,
		SourceTitle:   t.SourceTitle,
		Source:        t.Source,
		Status:        t.Status,
		QuestionCount: int32(t.QuestionCount),
		ImportedCount: int32(t.ImportedCount),
		RetryCount:    int32(t.RetryCount),
		ErrorMsg:      t.ErrorMsg,
		CreatedAt:     timestamppb.New(t.CreatedAt),
		UpdatedAt:     timestamppb.New(t.UpdatedAt),
	}
	if t.StartedAt != nil {
		item.StartedAt = timestamppb.New(*t.StartedAt)
	}
	if t.FinishedAt != nil {
		item.FinishedAt = timestamppb.New(*t.FinishedAt)
	}
	return item, nil
}

func (s *AdminService) RetryScraperTask(ctx context.Context, req *adminv1.RetryScraperTaskRequest) (*adminv1.ScraperTaskInfo, error) {
	task, err := s.uc.GetScraperTask(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	task.Status = "pending"
	task.RetryCount++
	if err := s.uc.UpdateScraperTask(ctx, task); err != nil {
		return nil, err
	}
	return &adminv1.ScraperTaskInfo{TaskId: task.ID, Status: "pending"}, nil
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
