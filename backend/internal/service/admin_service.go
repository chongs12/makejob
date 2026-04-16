// Package service 提供业务逻辑层实现
package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"makejob-backend/internal/common"
	"makejob-backend/internal/model"
	"makejob-backend/internal/repository"
)

// ==================== DTO 定义 ====================

// DashboardResponse 仪表盘响应DTO
type DashboardResponse struct {
	TotalUsers       int64 `json:"total_users"`
	TotalQuestions   int64 `json:"total_questions"`
	TotalInterviews  int64 `json:"total_interviews"`
	TodayActiveUsers int64 `json:"today_active_users"`
	ProMembers       int64 `json:"pro_members"`
	NewUsersToday    int64 `json:"new_users_today"`
}

type AdminUserListItem struct {
	ID                 uint       `json:"id"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	Username           string     `json:"username"`
	Email              string     `json:"email"`
	Avatar             string     `json:"avatar"`
	Role               string     `json:"role"`
	MembershipLevel    string     `json:"membership_level"`
	MembershipType     string     `json:"membership_type"`
	MembershipExpireAt *time.Time `json:"membership_expire_at"`
	IsDisabled         bool       `json:"is_disabled"`
}

// AdminCreateQuestionRequest 创建题目请求DTO
type AdminCreateQuestionRequest struct {
	CategoryID  uint   `json:"category_id" binding:"required"`
	IndustryID  uint   `json:"industry_id" binding:"required"`
	Type        string `json:"type" binding:"required,oneof=choice multi code subjective"`
	Difficulty  string `json:"difficulty" binding:"required,oneof=easy medium hard"`
	Title       string `json:"title" binding:"required,max=500"`
	Content     string `json:"content" binding:"required"`
	OptionsJSON string `json:"options_json,omitempty"`
	Answer      string `json:"answer" binding:"required"`
	Explanation string `json:"explanation"`
	Tags        string `json:"tags"`
	IsActive    bool   `json:"is_active"`
}

// AdminUpdateQuestionRequest 更新题目请求DTO
type AdminUpdateQuestionRequest struct {
	CategoryID  uint   `json:"category_id,omitempty"`
	IndustryID  uint   `json:"industry_id,omitempty"`
	Type        string `json:"type,omitempty" binding:"omitempty,oneof=choice multi code subjective"`
	Difficulty  string `json:"difficulty,omitempty" binding:"omitempty,oneof=easy medium hard"`
	Title       string `json:"title,omitempty" binding:"omitempty,max=500"`
	Content     string `json:"content,omitempty"`
	OptionsJSON string `json:"options_json,omitempty"`
	Answer      string `json:"answer,omitempty"`
	Explanation string `json:"explanation,omitempty"`
	Tags        string `json:"tags,omitempty"`
	IsActive    *bool  `json:"is_active,omitempty"`
}

// BatchImportRequest 批量导入请求DTO
type BatchImportRequest struct {
	IndustryCode string               `json:"industry_code" binding:"required"`
	Questions    []ImportQuestionItem `json:"questions" binding:"required,min=1"`
}

// ImportQuestionItem 导入题目项DTO
type ImportQuestionItem struct {
	CategoryName string `json:"category_name" binding:"required"`
	Type         string `json:"type" binding:"required"`
	Difficulty   string `json:"difficulty" binding:"required"`
	Title        string `json:"title" binding:"required"`
	Content      string `json:"content" binding:"required"`
	OptionsJSON  string `json:"options_json,omitempty"`
	Answer       string `json:"answer" binding:"required"`
	Explanation  string `json:"explanation"`
	Tags         string `json:"tags"`
}

// BatchImportResponse 批量导入响应DTO
type BatchImportResponse struct {
	TotalCount   int      `json:"total_count"`
	SuccessCount int      `json:"success_count"`
	FailCount    int      `json:"fail_count"`
	Errors       []string `json:"errors,omitempty"`
}

// CreateCategoryRequest 创建分类请求DTO
type CreateCategoryRequest struct {
	IndustryID  uint   `json:"industry_id" binding:"required"`
	Name        string `json:"name" binding:"required,max=100"`
	ParentID    *uint  `json:"parent_id,omitempty"`
	SortOrder   int    `json:"sort_order"`
	Icon        string `json:"icon,omitempty"`
	Description string `json:"description,omitempty"`
}

// UpdateCategoryRequest 更新分类请求DTO
type UpdateCategoryRequest struct {
	IndustryID  uint   `json:"industry_id,omitempty"`
	Name        string `json:"name,omitempty" binding:"omitempty,max=100"`
	ParentID    *uint  `json:"parent_id,omitempty"`
	SortOrder   *int   `json:"sort_order,omitempty"`
	Icon        string `json:"icon,omitempty"`
	Description string `json:"description,omitempty"`
}

// CreateIndustryRequest 创建行业请求DTO
type CreateIndustryRequest struct {
	Code        string `json:"code" binding:"required,max=50"`
	Name        string `json:"name" binding:"required,max=100"`
	Description string `json:"description,omitempty"`
	Icon        string `json:"icon,omitempty"`
	SortOrder   int    `json:"sort_order"`
}

// UpdateIndustryRequest 更新行业请求DTO
type UpdateIndustryRequest struct {
	Code        string `json:"code,omitempty" binding:"omitempty,max=50"`
	Name        string `json:"name,omitempty" binding:"omitempty,max=100"`
	Description string `json:"description,omitempty"`
	Icon        string `json:"icon,omitempty"`
	SortOrder   *int   `json:"sort_order,omitempty"`
	IsActive    *bool  `json:"is_active,omitempty"`
}

// CreatePromptRequest 创建Prompt模板请求DTO
type CreatePromptRequest struct {
	IndustryID      *uint  `json:"industry_id"`
	Name            string `json:"name" binding:"required,max=100"`
	Scene           string `json:"scene" binding:"required,oneof=interview companion quiz plan"`
	TemplateContent string `json:"template_content" binding:"required"`
	Variables       string `json:"variables,omitempty"`
	IsActive        bool   `json:"is_active"`
}

// UpdatePromptRequest 更新Prompt模板请求DTO
type UpdatePromptRequest struct {
	IndustryID      *uint  `json:"industry_id,omitempty"`
	Name            string `json:"name,omitempty" binding:"omitempty,max=100"`
	Scene           string `json:"scene,omitempty" binding:"omitempty,oneof=interview companion quiz plan"`
	TemplateContent string `json:"template_content,omitempty"`
	Variables       string `json:"variables,omitempty"`
	IsActive        *bool  `json:"is_active,omitempty"`
}

// CreateLive2DModelRequest 创建Live2D模型请求DTO
type CreateLive2DModelRequest struct {
	Name         string `json:"name" binding:"required,max=100"`
	IndustryID   uint   `json:"industry_id"`
	Scene        string `json:"scene" binding:"required,oneof=interview companion"`
	ModelURL     string `json:"model_url" binding:"required,max=500"`
	ThumbnailURL string `json:"thumbnail_url,omitempty"`
	ConfigJSON   string `json:"config_json,omitempty"`
	IsActive     bool   `json:"is_active"`
}

// UpdateLive2DModelRequest 更新Live2D模型请求DTO
type UpdateLive2DModelRequest struct {
	Name         string `json:"name,omitempty" binding:"omitempty,max=100"`
	IndustryID   uint   `json:"industry_id,omitempty"`
	Scene        string `json:"scene,omitempty" binding:"omitempty,oneof=interview companion"`
	ModelURL     string `json:"model_url,omitempty" binding:"omitempty,max=500"`
	ThumbnailURL string `json:"thumbnail_url,omitempty"`
	ConfigJSON   string `json:"config_json,omitempty"`
	IsActive     *bool  `json:"is_active,omitempty"`
}

// CreateTTSConfigRequest 创建TTS配置请求DTO
type CreateTTSConfigRequest struct {
	Name       string `json:"name" binding:"required,max=100"`
	Engine     string `json:"engine" binding:"required,oneof=elevenlabs minimax aliyun xunfei"`
	VoiceID    string `json:"voice_id" binding:"required,max=100"`
	Scene      string `json:"scene" binding:"required,oneof=interview companion"`
	ParamsJSON string `json:"params_json,omitempty"`
	IsActive   bool   `json:"is_active"`
	SortOrder  int    `json:"sort_order"`
}

// UpdateTTSConfigRequest 更新TTS配置请求DTO
type UpdateTTSConfigRequest struct {
	Name       string `json:"name,omitempty" binding:"omitempty,max=100"`
	Engine     string `json:"engine,omitempty" binding:"omitempty,oneof=elevenlabs minimax aliyun xunfei"`
	VoiceID    string `json:"voice_id,omitempty" binding:"omitempty,max=100"`
	Scene      string `json:"scene,omitempty" binding:"omitempty,oneof=interview companion"`
	ParamsJSON string `json:"params_json,omitempty"`
	IsActive   *bool  `json:"is_active,omitempty"`
	SortOrder  *int   `json:"sort_order,omitempty"`
}

// ==================== Service 接口 ====================

// AdminService 管理员服务接口
type AdminService interface {
	// 仪表盘
	GetDashboard(ctx context.Context) (*DashboardResponse, error)

	// 用户管理
	ListUsers(ctx context.Context, page, pageSize int, keyword, role string) (*common.PageResult, error)
	UpdateUserRole(ctx context.Context, userID uint, role string) error
	DisableUser(ctx context.Context, userID uint) error

	// 题库管理
	CreateQuestion(ctx context.Context, req *AdminCreateQuestionRequest) (*model.Question, error)
	UpdateQuestion(ctx context.Context, id uint, req *AdminUpdateQuestionRequest) error
	DeleteQuestion(ctx context.Context, id uint) error
	BatchImportQuestions(ctx context.Context, req *BatchImportRequest) (*BatchImportResponse, error)

	// 分类管理
	CreateCategory(ctx context.Context, req *CreateCategoryRequest) (*model.Category, error)
	UpdateCategory(ctx context.Context, id uint, req *UpdateCategoryRequest) error
	DeleteCategory(ctx context.Context, id uint) error

	// 行业管理
	ListIndustries(ctx context.Context) ([]model.Industry, error)
	CreateIndustry(ctx context.Context, req *CreateIndustryRequest) (*model.Industry, error)
	UpdateIndustry(ctx context.Context, id uint, req *UpdateIndustryRequest) error

	// Prompt模板
	ListPrompts(ctx context.Context, industryID *uint, scene string) ([]model.PromptTemplate, error)
	CreatePrompt(ctx context.Context, req *CreatePromptRequest) (*model.PromptTemplate, error)
	UpdatePrompt(ctx context.Context, id uint, req *UpdatePromptRequest) error
	DeletePrompt(ctx context.Context, id uint) error

	// AI配置
	GetAIConfigs(ctx context.Context) ([]model.AdminConfig, error)
	UpdateAIConfigs(ctx context.Context, configs map[string]string) error

	// Live2D管理
	ListLive2DModels(ctx context.Context) ([]model.Live2DModel, error)
	CreateLive2DModel(ctx context.Context, req *CreateLive2DModelRequest) (*model.Live2DModel, error)
	UpdateLive2DModel(ctx context.Context, id uint, req *UpdateLive2DModelRequest) error
	DeleteLive2DModel(ctx context.Context, id uint) error

	// TTS管理
	ListTTSConfigs(ctx context.Context) ([]model.TTSConfig, error)
	CreateTTSConfig(ctx context.Context, req *CreateTTSConfigRequest) (*model.TTSConfig, error)
	UpdateTTSConfig(ctx context.Context, id uint, req *UpdateTTSConfigRequest) error
	DeleteTTSConfig(ctx context.Context, id uint) error
}

// adminService 管理员服务实现
type adminService struct {
	adminUserRepo     repository.AdminUserRepository
	adminQuestionRepo repository.AdminQuestionRepository
	industryRepo      repository.IndustryRepository
	adminCategoryRepo repository.AdminCategoryRepository
	promptRepo        repository.PromptTemplateRepository
	adminConfigRepo   repository.AdminConfigRepository
	live2DRepo        repository.Live2DModelRepository
	ttsRepo           repository.TTSConfigRepository
	mockInterviewRepo repository.MockInterviewRepository
}

// NewAdminService 创建管理员服务实例
func NewAdminService(
	adminUserRepo repository.AdminUserRepository,
	adminQuestionRepo repository.AdminQuestionRepository,
	industryRepo repository.IndustryRepository,
	adminCategoryRepo repository.AdminCategoryRepository,
	promptRepo repository.PromptTemplateRepository,
	adminConfigRepo repository.AdminConfigRepository,
	live2DRepo repository.Live2DModelRepository,
	ttsRepo repository.TTSConfigRepository,
	mockInterviewRepo repository.MockInterviewRepository,
) AdminService {
	return &adminService{
		adminUserRepo:     adminUserRepo,
		adminQuestionRepo: adminQuestionRepo,
		industryRepo:      industryRepo,
		adminCategoryRepo: adminCategoryRepo,
		promptRepo:        promptRepo,
		adminConfigRepo:   adminConfigRepo,
		live2DRepo:        live2DRepo,
		ttsRepo:           ttsRepo,
		mockInterviewRepo: mockInterviewRepo,
	}
}

// ==================== 仪表盘 ====================

// GetDashboard 获取仪表盘数据
func (s *adminService) GetDashboard(ctx context.Context) (*DashboardResponse, error) {
	// 获取用户统计
	userStats, err := s.adminUserRepo.GetStats(ctx)
	if err != nil {
		return nil, err
	}

	// 获取题目总数
	totalQuestions, err := s.adminQuestionRepo.Count(ctx)
	if err != nil {
		return nil, err
	}

	// 获取面试总数
	totalInterviews, err := s.mockInterviewRepo.Count(ctx)
	if err != nil {
		return nil, err
	}

	return &DashboardResponse{
		TotalUsers:       userStats.TotalUsers,
		TotalQuestions:   totalQuestions,
		TotalInterviews:  totalInterviews,
		TodayActiveUsers: userStats.TodayActiveUsers,
		ProMembers:       userStats.ProMembers,
		NewUsersToday:    userStats.NewUsersToday,
	}, nil
}

// ==================== 用户管理 ====================

// ListUsers 获取用户列表
func (s *adminService) ListUsers(ctx context.Context, page, pageSize int, keyword, role string) (*common.PageResult, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	users, total, err := s.adminUserRepo.List(ctx, page, pageSize, keyword, role)
	if err != nil {
		return nil, err
	}

	items := make([]AdminUserListItem, 0, len(users))
	for _, user := range users {
		isDisabled := user.Role == "disabled"
		roleValue := user.Role
		if isDisabled {
			roleValue = model.UserRoleFreeMember
		}

		items = append(items, AdminUserListItem{
			ID:                 user.ID,
			CreatedAt:          user.CreatedAt,
			UpdatedAt:          user.UpdatedAt,
			Username:           user.Username,
			Email:              user.Email,
			Avatar:             user.Avatar,
			Role:               roleValue,
			MembershipLevel:    user.MembershipLevel,
			MembershipType:     user.MembershipLevel,
			MembershipExpireAt: user.MembershipExpireAt,
			IsDisabled:         isDisabled,
		})
	}

	return &common.PageResult{
		List:     items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// UpdateUserRole 更新用户角色
func (s *adminService) UpdateUserRole(ctx context.Context, userID uint, role string) error {
	// 验证角色有效性
	validRoles := map[string]bool{
		model.UserRoleAdmin:      true,
		model.UserRoleProMember:  true,
		model.UserRoleFreeMember: true,
	}
	if !validRoles[role] {
		return common.NewBusinessError(common.CodeBadRequest, "无效的角色")
	}

	return s.adminUserRepo.UpdateRole(ctx, userID, role)
}

// DisableUser 禁用用户
func (s *adminService) DisableUser(ctx context.Context, userID uint) error {
	return s.adminUserRepo.Disable(ctx, userID)
}

// ==================== 题库管理 ====================

// CreateQuestion 创建题目
func (s *adminService) CreateQuestion(ctx context.Context, req *AdminCreateQuestionRequest) (*model.Question, error) {
	// 验证题目类型和选项
	if req.Type == model.QuestionTypeChoice || req.Type == model.QuestionTypeMulti {
		if req.OptionsJSON == "" {
			return nil, common.NewBusinessError(common.CodeBadRequest, "选择题必须提供选项")
		}
	}

	question := &model.Question{
		CategoryID:  req.CategoryID,
		IndustryID:  req.IndustryID,
		Type:        req.Type,
		Difficulty:  req.Difficulty,
		Title:       req.Title,
		Content:     req.Content,
		OptionsJSON: req.OptionsJSON,
		Answer:      req.Answer,
		Explanation: req.Explanation,
		Tags:        req.Tags,
		IsActive:    req.IsActive,
	}

	if err := s.adminQuestionRepo.Create(ctx, question); err != nil {
		return nil, err
	}

	return question, nil
}

// UpdateQuestion 更新题目
func (s *adminService) UpdateQuestion(ctx context.Context, id uint, req *AdminUpdateQuestionRequest) error {
	// 这里简化处理，实际应该先从数据库获取现有记录再更新
	// 由于repository层使用Save方法，需要完整对象
	// 实际项目中可能需要先查询再更新
	return common.NewBusinessError(common.CodeInternalError, "更新题目功能需要完整实现")
}

// DeleteQuestion 删除题目
func (s *adminService) DeleteQuestion(ctx context.Context, id uint) error {
	return s.adminQuestionRepo.Delete(ctx, id)
}

// BatchImportQuestions 批量导入题目
func (s *adminService) BatchImportQuestions(ctx context.Context, req *BatchImportRequest) (*BatchImportResponse, error) {
	// 查找行业
	industry, err := s.industryRepo.GetByCode(ctx, req.IndustryCode)
	if err != nil {
		return nil, err
	}
	if industry == nil {
		return nil, common.NewBusinessError(common.CodeNotFound, "行业不存在")
	}

	// 获取所有分类
	categories, err := s.adminCategoryRepo.List(ctx)
	if err != nil {
		return nil, err
	}

	// 构建分类名称到ID的映射
	categoryMap := make(map[string]uint)
	for _, cat := range categories {
		categoryMap[cat.Name] = cat.ID
	}

	response := &BatchImportResponse{
		TotalCount: len(req.Questions),
		Errors:     make([]string, 0),
	}

	var questionsToImport []model.Question

	for i, item := range req.Questions {
		// 查找分类ID
		categoryID, exists := categoryMap[item.CategoryName]
		if !exists {
			response.FailCount++
			response.Errors = append(response.Errors, fmt.Sprintf("第%d行: 分类'%s'不存在", i+1, item.CategoryName))
			continue
		}

		// 验证题目类型
		validTypes := map[string]bool{
			model.QuestionTypeChoice:     true,
			model.QuestionTypeMulti:      true,
			model.QuestionTypeCode:       true,
			model.QuestionTypeSubjective: true,
		}
		if !validTypes[item.Type] {
			response.FailCount++
			response.Errors = append(response.Errors, fmt.Sprintf("第%d行: 无效的题目类型'%s'", i+1, item.Type))
			continue
		}

		// 验证难度
		validDifficulties := map[string]bool{
			model.QuestionDifficultyEasy:   true,
			model.QuestionDifficultyMedium: true,
			model.QuestionDifficultyHard:   true,
		}
		if !validDifficulties[item.Difficulty] {
			response.FailCount++
			response.Errors = append(response.Errors, fmt.Sprintf("第%d行: 无效的难度'%s'", i+1, item.Difficulty))
			continue
		}

		question := model.Question{
			CategoryID:  categoryID,
			IndustryID:  industry.ID,
			Type:        item.Type,
			Difficulty:  item.Difficulty,
			Title:       item.Title,
			Content:     item.Content,
			OptionsJSON: item.OptionsJSON,
			Answer:      item.Answer,
			Explanation: item.Explanation,
			Tags:        item.Tags,
			IsActive:    true,
		}

		questionsToImport = append(questionsToImport, question)
	}

	// 批量创建题目
	if len(questionsToImport) > 0 {
		if err := s.adminQuestionRepo.BatchCreate(ctx, questionsToImport); err != nil {
			return nil, err
		}
		response.SuccessCount = len(questionsToImport)
	}

	return response, nil
}

// ==================== 分类管理 ====================

// CreateCategory 创建分类
func (s *adminService) CreateCategory(ctx context.Context, req *CreateCategoryRequest) (*model.Category, error) {
	category := &model.Category{
		IndustryID:  req.IndustryID,
		Name:        req.Name,
		ParentID:    req.ParentID,
		SortOrder:   req.SortOrder,
		Icon:        req.Icon,
		Description: req.Description,
	}

	if err := s.adminCategoryRepo.Create(ctx, category); err != nil {
		return nil, err
	}

	return category, nil
}

// UpdateCategory 更新分类
func (s *adminService) UpdateCategory(ctx context.Context, id uint, req *UpdateCategoryRequest) error {
	// 简化处理，实际应该先从数据库获取现有记录再更新
	return common.NewBusinessError(common.CodeInternalError, "更新分类功能需要完整实现")
}

// DeleteCategory 删除分类
func (s *adminService) DeleteCategory(ctx context.Context, id uint) error {
	return s.adminCategoryRepo.Delete(ctx, id)
}

// ==================== 行业管理 ====================

// ListIndustries 获取行业列表
func (s *adminService) ListIndustries(ctx context.Context) ([]model.Industry, error) {
	return s.industryRepo.List(ctx)
}

// CreateIndustry 创建行业
func (s *adminService) CreateIndustry(ctx context.Context, req *CreateIndustryRequest) (*model.Industry, error) {
	// 检查代码是否已存在
	existing, err := s.industryRepo.GetByCode(ctx, req.Code)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, common.NewBusinessError(common.CodeBadRequest, "行业代码已存在")
	}

	industry := &model.Industry{
		Code:        req.Code,
		Name:        req.Name,
		Description: req.Description,
		Icon:        req.Icon,
		SortOrder:   req.SortOrder,
		IsActive:    true,
	}

	if err := s.industryRepo.Create(ctx, industry); err != nil {
		return nil, err
	}

	return industry, nil
}

// UpdateIndustry 更新行业
func (s *adminService) UpdateIndustry(ctx context.Context, id uint, req *UpdateIndustryRequest) error {
	industry, err := s.industryRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if industry == nil {
		return common.NewBusinessError(common.CodeNotFound, "行业不存在")
	}

	// 更新字段
	if req.Code != "" {
		// 检查新代码是否与其他行业冲突
		existing, err := s.industryRepo.GetByCode(ctx, req.Code)
		if err != nil {
			return err
		}
		if existing != nil && existing.ID != id {
			return common.NewBusinessError(common.CodeBadRequest, "行业代码已存在")
		}
		industry.Code = req.Code
	}
	if req.Name != "" {
		industry.Name = req.Name
	}
	if req.Description != "" {
		industry.Description = req.Description
	}
	if req.Icon != "" {
		industry.Icon = req.Icon
	}
	if req.SortOrder != nil {
		industry.SortOrder = *req.SortOrder
	}
	if req.IsActive != nil {
		industry.IsActive = *req.IsActive
	}

	return s.industryRepo.Update(ctx, industry)
}

// ==================== Prompt模板管理 ====================

// ListPrompts 获取Prompt模板列表
func (s *adminService) ListPrompts(ctx context.Context, industryID *uint, scene string) ([]model.PromptTemplate, error) {
	return s.promptRepo.List(ctx, industryID, scene)
}

// CreatePrompt 创建Prompt模板
func (s *adminService) CreatePrompt(ctx context.Context, req *CreatePromptRequest) (*model.PromptTemplate, error) {
	tpl := &model.PromptTemplate{
		IndustryID:      req.IndustryID,
		Name:            req.Name,
		Scene:           req.Scene,
		TemplateContent: req.TemplateContent,
		Variables:       req.Variables,
		IsActive:        req.IsActive,
	}

	if err := s.promptRepo.Create(ctx, tpl); err != nil {
		return nil, err
	}

	return tpl, nil
}

// UpdatePrompt 更新Prompt模板
func (s *adminService) UpdatePrompt(ctx context.Context, id uint, req *UpdatePromptRequest) error {
	tpl, err := s.promptRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if tpl == nil {
		return common.NewBusinessError(common.CodeNotFound, "Prompt模板不存在")
	}

	// 更新字段
	if req.Name != "" {
		tpl.Name = req.Name
	}
	if req.Scene != "" {
		tpl.Scene = req.Scene
	}
	if req.TemplateContent != "" {
		tpl.TemplateContent = req.TemplateContent
	}
	if req.Variables != "" {
		tpl.Variables = req.Variables
	}
	if req.IsActive != nil {
		tpl.IsActive = *req.IsActive
	}
	// IndustryID 可以直接更新，nil表示通用
	if req.IndustryID != nil {
		tpl.IndustryID = req.IndustryID
	}

	return s.promptRepo.Update(ctx, tpl)
}

// DeletePrompt 删除Prompt模板
func (s *adminService) DeletePrompt(ctx context.Context, id uint) error {
	return s.promptRepo.Delete(ctx, id)
}

// ==================== AI配置管理 ====================

// GetAIConfigs 获取AI配置列表
func (s *adminService) GetAIConfigs(ctx context.Context) ([]model.AdminConfig, error) {
	return s.adminConfigRepo.List(ctx)
}

// UpdateAIConfigs 更新AI配置
func (s *adminService) UpdateAIConfigs(ctx context.Context, configs map[string]string) error {
	if len(configs) == 0 {
		return common.NewBusinessError(common.CodeBadRequest, "配置不能为空")
	}

	var adminConfigs []model.AdminConfig
	for key, value := range configs {
		// 只接受以 ai_ 开头的配置键
		if !strings.HasPrefix(key, "ai_") {
			continue
		}

		adminConfigs = append(adminConfigs, model.AdminConfig{
			ConfigKey:   key,
			ConfigValue: value,
			ConfigType:  model.ConfigTypeString,
			Description: "AI配置",
		})
	}

	if len(adminConfigs) == 0 {
		return common.NewBusinessError(common.CodeBadRequest, "没有有效的AI配置")
	}

	return s.adminConfigRepo.BatchUpsert(ctx, adminConfigs)
}

// ==================== Live2D模型管理 ====================

// ListLive2DModels 获取Live2D模型列表
func (s *adminService) ListLive2DModels(ctx context.Context) ([]model.Live2DModel, error) {
	return s.live2DRepo.List(ctx)
}

// CreateLive2DModel 创建Live2D模型
func (s *adminService) CreateLive2DModel(ctx context.Context, req *CreateLive2DModelRequest) (*model.Live2DModel, error) {
	m := &model.Live2DModel{
		Name:         req.Name,
		IndustryID:   req.IndustryID,
		Scene:        req.Scene,
		ModelURL:     req.ModelURL,
		ThumbnailURL: req.ThumbnailURL,
		ConfigJSON:   req.ConfigJSON,
		IsActive:     req.IsActive,
	}

	if err := s.live2DRepo.Create(ctx, m); err != nil {
		return nil, err
	}

	return m, nil
}

// UpdateLive2DModel 更新Live2D模型
func (s *adminService) UpdateLive2DModel(ctx context.Context, id uint, req *UpdateLive2DModelRequest) error {
	m, err := s.live2DRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if m == nil {
		return common.NewBusinessError(common.CodeNotFound, "Live2D模型不存在")
	}

	// 更新字段
	if req.Name != "" {
		m.Name = req.Name
	}
	if req.Scene != "" {
		m.Scene = req.Scene
	}
	if req.ModelURL != "" {
		m.ModelURL = req.ModelURL
	}
	if req.ThumbnailURL != "" {
		m.ThumbnailURL = req.ThumbnailURL
	}
	if req.ConfigJSON != "" {
		m.ConfigJSON = req.ConfigJSON
	}
	if req.IsActive != nil {
		m.IsActive = *req.IsActive
	}
	m.IndustryID = req.IndustryID

	return s.live2DRepo.Update(ctx, m)
}

// DeleteLive2DModel 删除Live2D模型
func (s *adminService) DeleteLive2DModel(ctx context.Context, id uint) error {
	return s.live2DRepo.Delete(ctx, id)
}

// ==================== TTS配置管理 ====================

// ListTTSConfigs 获取TTS配置列表
func (s *adminService) ListTTSConfigs(ctx context.Context) ([]model.TTSConfig, error) {
	return s.ttsRepo.List(ctx)
}

// CreateTTSConfig 创建TTS配置
func (s *adminService) CreateTTSConfig(ctx context.Context, req *CreateTTSConfigRequest) (*model.TTSConfig, error) {
	cfg := &model.TTSConfig{
		Name:       req.Name,
		Engine:     req.Engine,
		VoiceID:    req.VoiceID,
		Scene:      req.Scene,
		ParamsJSON: req.ParamsJSON,
		IsActive:   req.IsActive,
		SortOrder:  req.SortOrder,
	}

	if err := s.ttsRepo.Create(ctx, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// UpdateTTSConfig 更新TTS配置
func (s *adminService) UpdateTTSConfig(ctx context.Context, id uint, req *UpdateTTSConfigRequest) error {
	cfg, err := s.ttsRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if cfg == nil {
		return common.NewBusinessError(common.CodeNotFound, "TTS配置不存在")
	}

	// 更新字段
	if req.Name != "" {
		cfg.Name = req.Name
	}
	if req.Engine != "" {
		cfg.Engine = req.Engine
	}
	if req.VoiceID != "" {
		cfg.VoiceID = req.VoiceID
	}
	if req.Scene != "" {
		cfg.Scene = req.Scene
	}
	if req.ParamsJSON != "" {
		cfg.ParamsJSON = req.ParamsJSON
	}
	if req.IsActive != nil {
		cfg.IsActive = *req.IsActive
	}
	if req.SortOrder != nil {
		cfg.SortOrder = *req.SortOrder
	}

	return s.ttsRepo.Update(ctx, cfg)
}

// DeleteTTSConfig 删除TTS配置
func (s *adminService) DeleteTTSConfig(ctx context.Context, id uint) error {
	return s.ttsRepo.Delete(ctx, id)
}
