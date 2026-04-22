package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"makejob-backend/internal/ai"
	"makejob-backend/internal/common"
	"makejob-backend/internal/live2dassets"
	"makejob-backend/internal/model"
	"makejob-backend/internal/repository"
	"makejob-backend/internal/scraper"
)

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

type AdminQuestionListItem struct {
	ID           uint      `json:"id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	CategoryID   uint      `json:"category_id"`
	CategoryName string    `json:"category_name"`
	IndustryID   uint      `json:"industry_id"`
	Type         string    `json:"type"`
	Difficulty   string    `json:"difficulty"`
	Title        string    `json:"title"`
	Content      string    `json:"content"`
	Options      []string  `json:"options"`
	Answer       string    `json:"answer"`
	Explanation  string    `json:"explanation"`
	Tags         []string  `json:"tags"`
	IsActive     bool      `json:"is_active"`
}

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

type BatchImportRequest struct {
	IndustryCode string               `json:"industry_code" binding:"required"`
	Questions    []ImportQuestionItem `json:"questions" binding:"required,min=1"`
}

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

type BatchImportResponse struct {
	TotalCount   int      `json:"total_count"`
	SuccessCount int      `json:"success_count"`
	FailCount    int      `json:"fail_count"`
	Errors       []string `json:"errors,omitempty"`
}

type CreateCategoryRequest struct {
	IndustryID  uint   `json:"industry_id" binding:"required"`
	Name        string `json:"name" binding:"required,max=100"`
	ParentID    *uint  `json:"parent_id,omitempty"`
	SortOrder   int    `json:"sort_order"`
	Icon        string `json:"icon,omitempty"`
	Description string `json:"description,omitempty"`
}

type UpdateCategoryRequest struct {
	IndustryID  uint   `json:"industry_id,omitempty"`
	Name        string `json:"name,omitempty" binding:"omitempty,max=100"`
	ParentID    *uint  `json:"parent_id,omitempty"`
	SortOrder   *int   `json:"sort_order,omitempty"`
	Icon        string `json:"icon,omitempty"`
	Description string `json:"description,omitempty"`
}

type CreateIndustryRequest struct {
	Code        string `json:"code" binding:"required,max=50"`
	Name        string `json:"name" binding:"required,max=100"`
	Description string `json:"description,omitempty"`
	Icon        string `json:"icon,omitempty"`
	SortOrder   int    `json:"sort_order"`
}

type UpdateIndustryRequest struct {
	Code        string `json:"code,omitempty" binding:"omitempty,max=50"`
	Name        string `json:"name,omitempty" binding:"omitempty,max=100"`
	Description string `json:"description,omitempty"`
	Icon        string `json:"icon,omitempty"`
	SortOrder   *int   `json:"sort_order,omitempty"`
	IsActive    *bool  `json:"is_active,omitempty"`
}

type CreatePromptRequest struct {
	IndustryID      *uint  `json:"industry_id"`
	Name            string `json:"name" binding:"required,max=100"`
	Scene           string `json:"scene" binding:"required,oneof=interview companion quiz plan"`
	TemplateContent string `json:"template_content" binding:"required"`
	Variables       string `json:"variables,omitempty"`
	IsActive        bool   `json:"is_active"`
}

type UpdatePromptRequest struct {
	IndustryID      *uint  `json:"industry_id,omitempty"`
	Name            string `json:"name,omitempty" binding:"omitempty,max=100"`
	Scene           string `json:"scene,omitempty" binding:"omitempty,oneof=interview companion quiz plan"`
	TemplateContent string `json:"template_content,omitempty"`
	Variables       string `json:"variables,omitempty"`
	IsActive        *bool  `json:"is_active,omitempty"`
}

type CreateLive2DModelRequest struct {
	Name         string `json:"name" binding:"required,max=100"`
	IndustryID   *uint  `json:"industry_id"`
	Scene        string `json:"scene" binding:"required,oneof=interview companion"`
	ModelURL     string `json:"model_url" binding:"required,max=500"`
	ThumbnailURL string `json:"thumbnail_url,omitempty"`
	ConfigJSON   string `json:"config_json,omitempty"`
	IsActive     bool   `json:"is_active"`
}

type UpdateLive2DModelRequest struct {
	Name         string `json:"name,omitempty" binding:"omitempty,max=100"`
	IndustryID   *uint  `json:"industry_id,omitempty"`
	Scene        string `json:"scene,omitempty" binding:"omitempty,oneof=interview companion"`
	ModelURL     string `json:"model_url,omitempty" binding:"omitempty,max=500"`
	ThumbnailURL string `json:"thumbnail_url,omitempty"`
	ConfigJSON   string `json:"config_json,omitempty"`
	IsActive     *bool  `json:"is_active,omitempty"`
}

// ImportLive2DPackageResponse 描述后台导入 Live2D 模型包后的自动识别结果。
type ImportLive2DPackageResponse struct {
	Name         string `json:"name"`
	AssetDir     string `json:"asset_dir"`
	ModelURL     string `json:"model_url"`
	ThumbnailURL string `json:"thumbnail_url,omitempty"`
}

type CreateTTSConfigRequest struct {
	Name       string `json:"name" binding:"required,max=100"`
	Engine     string `json:"engine" binding:"required,oneof=elevenlabs minimax aliyun xunfei"`
	VoiceID    string `json:"voice_id" binding:"required,max=100"`
	Scene      string `json:"scene" binding:"required,oneof=interview companion"`
	ParamsJSON string `json:"params_json,omitempty"`
	IsActive   bool   `json:"is_active"`
	SortOrder  int    `json:"sort_order"`
}

type UpdateTTSConfigRequest struct {
	Name       string `json:"name,omitempty" binding:"omitempty,max=100"`
	Engine     string `json:"engine,omitempty" binding:"omitempty,oneof=elevenlabs minimax aliyun xunfei"`
	VoiceID    string `json:"voice_id,omitempty" binding:"omitempty,max=100"`
	Scene      string `json:"scene,omitempty" binding:"omitempty,oneof=interview companion"`
	ParamsJSON string `json:"params_json,omitempty"`
	IsActive   *bool  `json:"is_active,omitempty"`
	SortOrder  *int   `json:"sort_order,omitempty"`
}

type AdminService interface {
	GetDashboard(ctx context.Context) (*DashboardResponse, error)

	ListUsers(ctx context.Context, page, pageSize int, keyword, role string) (*common.PageResult, error)
	UpdateUserRole(ctx context.Context, userID uint, role string) error
	DisableUser(ctx context.Context, userID uint) error

	ListQuestions(ctx context.Context, page, pageSize int, keyword, difficulty string, categoryID uint) (*common.PageResult, error)
	CreateQuestion(ctx context.Context, req *AdminCreateQuestionRequest) (*model.Question, error)
	UpdateQuestion(ctx context.Context, id uint, req *AdminUpdateQuestionRequest) error
	DeleteQuestion(ctx context.Context, id uint) error
	BatchImportQuestions(ctx context.Context, req *BatchImportRequest) (*BatchImportResponse, error)
	GenerateQuestionPipeline(ctx context.Context, req *AdminQuestionPipelineGenerateRequest) (*AdminQuestionPipelineGenerateResponse, error)
	GenerateQuestionPipelineStream(ctx context.Context, req *AdminQuestionPipelineGenerateRequest, emit AdminQuestionPipelineStreamEmitter) error
	ImportQuestionPipeline(ctx context.Context, req *AdminQuestionPipelineImportRequest) (*BatchImportResponse, error)

	ListCategories(ctx context.Context) ([]model.Category, error)
	CreateCategory(ctx context.Context, req *CreateCategoryRequest) (*model.Category, error)
	UpdateCategory(ctx context.Context, id uint, req *UpdateCategoryRequest) error
	DeleteCategory(ctx context.Context, id uint) error

	ListIndustries(ctx context.Context) ([]model.Industry, error)
	CreateIndustry(ctx context.Context, req *CreateIndustryRequest) (*model.Industry, error)
	UpdateIndustry(ctx context.Context, id uint, req *UpdateIndustryRequest) error

	ListPrompts(ctx context.Context, industryID *uint, scene string) ([]model.PromptTemplate, error)
	CreatePrompt(ctx context.Context, req *CreatePromptRequest) (*model.PromptTemplate, error)
	UpdatePrompt(ctx context.Context, id uint, req *UpdatePromptRequest) error
	DeletePrompt(ctx context.Context, id uint) error

	GetAIConfigs(ctx context.Context) (*AIConfigResponse, error)
	UpdateAIConfigs(ctx context.Context, configs map[string]string) error
	DebugAIRuntime(ctx context.Context, req *AIDebugRequest) (*AIDebugResponse, error)
	ListAICallLogs(ctx context.Context, req *ListAICallLogsRequest) (*common.PageResult, error)

	ListLive2DModels(ctx context.Context) ([]model.Live2DModel, error)
	CreateLive2DModel(ctx context.Context, req *CreateLive2DModelRequest) (*model.Live2DModel, error)
	UpdateLive2DModel(ctx context.Context, id uint, req *UpdateLive2DModelRequest) error
	DeleteLive2DModel(ctx context.Context, id uint) error
	ImportLive2DPackage(ctx context.Context, filename string, content []byte) (*ImportLive2DPackageResponse, error)

	ListTTSConfigs(ctx context.Context) ([]model.TTSConfig, error)
	CreateTTSConfig(ctx context.Context, req *CreateTTSConfigRequest) (*model.TTSConfig, error)
	UpdateTTSConfig(ctx context.Context, id uint, req *UpdateTTSConfigRequest) error
	DeleteTTSConfig(ctx context.Context, id uint) error
}

type adminService struct {
	adminUserRepo     repository.AdminUserRepository
	adminQuestionRepo repository.AdminQuestionRepository
	industryRepo      repository.IndustryRepository
	adminCategoryRepo repository.AdminCategoryRepository
	promptRepo        repository.PromptTemplateRepository
	adminConfigRepo   repository.AdminConfigRepository
	aiCallLogRepo     repository.AICallLogRepository
	live2DRepo        repository.Live2DModelRepository
	ttsRepo           repository.TTSConfigRepository
	mockInterviewRepo repository.MockInterviewRepository
	scraperProvider   scraper.ScraperProvider
	questionCleaner   scraper.QuestionCleaner
	baseAIConfig      map[string]string
}

// NewAdminService 创建后台管理服务。
func NewAdminService(
	adminUserRepo repository.AdminUserRepository,
	adminQuestionRepo repository.AdminQuestionRepository,
	industryRepo repository.IndustryRepository,
	adminCategoryRepo repository.AdminCategoryRepository,
	promptRepo repository.PromptTemplateRepository,
	adminConfigRepo repository.AdminConfigRepository,
	aiCallLogRepo repository.AICallLogRepository,
	live2DRepo repository.Live2DModelRepository,
	ttsRepo repository.TTSConfigRepository,
	mockInterviewRepo repository.MockInterviewRepository,
	scraperProvider scraper.ScraperProvider,
	questionCleaner scraper.QuestionCleaner,
	baseAIConfig map[string]string,
) AdminService {
	return &adminService{
		adminUserRepo:     adminUserRepo,
		adminQuestionRepo: adminQuestionRepo,
		industryRepo:      industryRepo,
		adminCategoryRepo: adminCategoryRepo,
		promptRepo:        promptRepo,
		adminConfigRepo:   adminConfigRepo,
		aiCallLogRepo:     aiCallLogRepo,
		live2DRepo:        live2DRepo,
		ttsRepo:           ttsRepo,
		mockInterviewRepo: mockInterviewRepo,
		scraperProvider:   scraperProvider,
		questionCleaner:   questionCleaner,
		baseAIConfig:      ai.NormalizeRuntimeConfig(baseAIConfig),
	}
}

func (s *adminService) GetDashboard(ctx context.Context) (*DashboardResponse, error) {
	userStats, err := s.adminUserRepo.GetStats(ctx)
	if err != nil {
		return nil, err
	}

	totalQuestions, err := s.adminQuestionRepo.Count(ctx)
	if err != nil {
		return nil, err
	}

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

	return &common.PageResult{List: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (s *adminService) UpdateUserRole(ctx context.Context, userID uint, role string) error {
	validRoles := map[string]bool{
		model.UserRoleAdmin:      true,
		model.UserRoleProMember:  true,
		model.UserRoleFreeMember: true,
	}
	if !validRoles[role] {
		return common.NewBusinessError(common.CodeBadRequest, "invalid role")
	}

	return s.adminUserRepo.UpdateRole(ctx, userID, role)
}

func (s *adminService) DisableUser(ctx context.Context, userID uint) error {
	return s.adminUserRepo.Disable(ctx, userID)
}

func (s *adminService) ListQuestions(ctx context.Context, page, pageSize int, keyword, difficulty string, categoryID uint) (*common.PageResult, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	questions, total, err := s.adminQuestionRepo.List(ctx, page, pageSize, keyword, difficulty, categoryID)
	if err != nil {
		return nil, err
	}

	items := make([]AdminQuestionListItem, 0, len(questions))
	for _, question := range questions {
		items = append(items, AdminQuestionListItem{
			ID:           question.ID,
			CreatedAt:    question.CreatedAt,
			UpdatedAt:    question.UpdatedAt,
			CategoryID:   question.CategoryID,
			CategoryName: question.Category.Name,
			IndustryID:   question.IndustryID,
			Type:         question.Type,
			Difficulty:   question.Difficulty,
			Title:        question.Title,
			Content:      question.Content,
			Options:      parseQuestionOptions(question.OptionsJSON),
			Answer:       question.Answer,
			Explanation:  question.Explanation,
			Tags:         parseQuestionTags(question.Tags),
			IsActive:     question.IsActive,
		})
	}

	return &common.PageResult{List: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (s *adminService) CreateQuestion(ctx context.Context, req *AdminCreateQuestionRequest) (*model.Question, error) {
	if err := s.validateQuestionRefs(ctx, req.IndustryID, req.CategoryID); err != nil {
		return nil, err
	}
	if err := validateQuestionPayload(req.Type, req.OptionsJSON); err != nil {
		return nil, err
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

func (s *adminService) UpdateQuestion(ctx context.Context, id uint, req *AdminUpdateQuestionRequest) error {
	question, err := s.adminQuestionRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if question == nil {
		return common.NewBusinessError(common.CodeNotFound, "question not found")
	}

	nextIndustryID := question.IndustryID
	if req.IndustryID != 0 {
		nextIndustryID = req.IndustryID
	}

	nextCategoryID := question.CategoryID
	if req.CategoryID != 0 {
		nextCategoryID = req.CategoryID
	}

	if err := s.validateQuestionRefs(ctx, nextIndustryID, nextCategoryID); err != nil {
		return err
	}

	nextType := question.Type
	if req.Type != "" {
		nextType = req.Type
	}

	nextOptionsJSON := question.OptionsJSON
	if req.OptionsJSON != "" || nextType == model.QuestionTypeCode || nextType == model.QuestionTypeSubjective {
		nextOptionsJSON = req.OptionsJSON
	}

	if err := validateQuestionPayload(nextType, nextOptionsJSON); err != nil {
		return err
	}

	question.IndustryID = nextIndustryID
	question.CategoryID = nextCategoryID
	question.Type = nextType
	question.OptionsJSON = nextOptionsJSON
	if req.Difficulty != "" {
		question.Difficulty = req.Difficulty
	}
	if req.Title != "" {
		question.Title = req.Title
	}
	if req.Content != "" {
		question.Content = req.Content
	}
	if req.Answer != "" {
		question.Answer = req.Answer
	}
	question.Explanation = req.Explanation
	question.Tags = req.Tags
	if req.IsActive != nil {
		question.IsActive = *req.IsActive
	}

	return s.adminQuestionRepo.Update(ctx, question)
}

func (s *adminService) DeleteQuestion(ctx context.Context, id uint) error {
	return s.adminQuestionRepo.Delete(ctx, id)
}

func (s *adminService) BatchImportQuestions(ctx context.Context, req *BatchImportRequest) (*BatchImportResponse, error) {
	industry, err := s.industryRepo.GetByCode(ctx, req.IndustryCode)
	if err != nil {
		return nil, err
	}
	if industry == nil {
		return nil, common.NewBusinessError(common.CodeNotFound, "industry not found")
	}

	categories, err := s.adminCategoryRepo.List(ctx)
	if err != nil {
		return nil, err
	}

	categoryMap := make(map[string]uint, len(categories))
	for _, category := range categories {
		if category.IndustryID == industry.ID {
			categoryMap[category.Name] = category.ID
		}
	}

	response := &BatchImportResponse{
		TotalCount: len(req.Questions),
		Errors:     make([]string, 0),
	}

	questionsToImport := make([]model.Question, 0, len(req.Questions))
	for index, item := range req.Questions {
		categoryID, ok := categoryMap[item.CategoryName]
		if !ok {
			response.Errors = append(response.Errors, fmt.Sprintf("question %d: category %s not found", index+1, item.CategoryName))
			continue
		}

		if err := validateQuestionPayload(item.Type, item.OptionsJSON); err != nil {
			response.Errors = append(response.Errors, fmt.Sprintf("question %d: %v", index+1, err))
			continue
		}

		questionsToImport = append(questionsToImport, model.Question{
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
		})
	}

	if len(questionsToImport) > 0 {
		if err := s.adminQuestionRepo.BatchCreate(ctx, questionsToImport); err != nil {
			return nil, err
		}
	}

	response.SuccessCount = len(questionsToImport)
	response.FailCount = response.TotalCount - response.SuccessCount
	return response, nil
}

func (s *adminService) ListCategories(ctx context.Context) ([]model.Category, error) {
	return s.adminCategoryRepo.List(ctx)
}

func (s *adminService) CreateCategory(ctx context.Context, req *CreateCategoryRequest) (*model.Category, error) {
	parentID := normalizedParentID(req.ParentID)
	if err := s.validateCategoryRefs(ctx, req.IndustryID, parentID, 0); err != nil {
		return nil, err
	}

	category := &model.Category{
		IndustryID:  req.IndustryID,
		Name:        req.Name,
		ParentID:    parentID,
		SortOrder:   req.SortOrder,
		Icon:        req.Icon,
		Description: req.Description,
	}

	if err := s.adminCategoryRepo.Create(ctx, category); err != nil {
		return nil, err
	}

	return category, nil
}

func (s *adminService) UpdateCategory(ctx context.Context, id uint, req *UpdateCategoryRequest) error {
	category, err := s.adminCategoryRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if category == nil {
		return common.NewBusinessError(common.CodeNotFound, "category not found")
	}

	nextIndustryID := category.IndustryID
	if req.IndustryID != 0 {
		nextIndustryID = req.IndustryID
	}

	nextParentID := category.ParentID
	if req.ParentID != nil {
		nextParentID = normalizedParentID(req.ParentID)
	}

	if err := s.validateCategoryRefs(ctx, nextIndustryID, nextParentID, id); err != nil {
		return err
	}

	category.IndustryID = nextIndustryID
	category.ParentID = nextParentID
	if req.Name != "" {
		category.Name = req.Name
	}
	if req.SortOrder != nil {
		category.SortOrder = *req.SortOrder
	}
	if req.Icon != "" {
		category.Icon = req.Icon
	}
	if req.Description != "" {
		category.Description = req.Description
	}

	return s.adminCategoryRepo.Update(ctx, category)
}

func (s *adminService) DeleteCategory(ctx context.Context, id uint) error {
	return s.adminCategoryRepo.Delete(ctx, id)
}

func (s *adminService) ListIndustries(ctx context.Context) ([]model.Industry, error) {
	return s.industryRepo.List(ctx)
}

func (s *adminService) CreateIndustry(ctx context.Context, req *CreateIndustryRequest) (*model.Industry, error) {
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

func (s *adminService) UpdateIndustry(ctx context.Context, id uint, req *UpdateIndustryRequest) error {
	industry, err := s.industryRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if industry == nil {
		return common.NewBusinessError(common.CodeNotFound, "industry not found")
	}

	if req.Code != "" {
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

func (s *adminService) ListPrompts(ctx context.Context, industryID *uint, scene string) ([]model.PromptTemplate, error) {
	return s.promptRepo.List(ctx, industryID, scene)
}

func (s *adminService) CreatePrompt(ctx context.Context, req *CreatePromptRequest) (*model.PromptTemplate, error) {
	industryID, err := s.normalizeOptionalIndustryID(ctx, req.IndustryID)
	if err != nil {
		return nil, err
	}

	tpl := &model.PromptTemplate{
		IndustryID:      industryID,
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

func (s *adminService) UpdatePrompt(ctx context.Context, id uint, req *UpdatePromptRequest) error {
	tpl, err := s.promptRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if tpl == nil {
		return common.NewBusinessError(common.CodeNotFound, "prompt not found")
	}

	if req.IndustryID != nil {
		industryID, err := s.normalizeOptionalIndustryID(ctx, req.IndustryID)
		if err != nil {
			return err
		}
		tpl.IndustryID = industryID
	}
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

	return s.promptRepo.Update(ctx, tpl)
}

func (s *adminService) DeletePrompt(ctx context.Context, id uint) error {
	return s.promptRepo.Delete(ctx, id)
}

func (s *adminService) GetAIConfigs(ctx context.Context) (*AIConfigResponse, error) {
	items, err := s.adminConfigRepo.List(ctx)
	if err != nil {
		return nil, err
	}

	return buildAIConfigResponse(items, s.baseAIConfig), nil
}

func (s *adminService) UpdateAIConfigs(ctx context.Context, configs map[string]string) error {
	if len(configs) == 0 {
		return common.NewBusinessError(common.CodeBadRequest, "configs cannot be empty")
	}

	adminConfigs := buildAIConfigItems(ai.NormalizeRuntimeConfig(configs))
	if len(adminConfigs) == 0 {
		return common.NewBusinessError(common.CodeBadRequest, "no valid ai configs provided")
	}

	return s.adminConfigRepo.BatchUpsert(ctx, adminConfigs)
}

// ListLive2DModels 返回后台可维护的 Live2D 模型列表。
func (s *adminService) ListLive2DModels(ctx context.Context) ([]model.Live2DModel, error) {
	return s.live2DRepo.List(ctx)
}

// CreateLive2DModel 创建一条新的 Live2D 模型配置记录。
func (s *adminService) CreateLive2DModel(ctx context.Context, req *CreateLive2DModelRequest) (*model.Live2DModel, error) {
	industryID, err := s.normalizeOptionalIndustryID(ctx, req.IndustryID)
	if err != nil {
		return nil, err
	}

	live2d := &model.Live2DModel{
		Name:         req.Name,
		IndustryID:   industryID,
		Scene:        req.Scene,
		ModelURL:     req.ModelURL,
		ThumbnailURL: req.ThumbnailURL,
		ConfigJSON:   req.ConfigJSON,
		IsActive:     req.IsActive,
	}

	if err := s.live2DRepo.Create(ctx, live2d); err != nil {
		return nil, err
	}

	return live2d, nil
}

// UpdateLive2DModel 更新指定的 Live2D 模型，并支持切换为通用模型。
func (s *adminService) UpdateLive2DModel(ctx context.Context, id uint, req *UpdateLive2DModelRequest) error {
	live2d, err := s.live2DRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if live2d == nil {
		return common.NewBusinessError(common.CodeNotFound, "live2d model not found")
	}

	if req.IndustryID != nil {
		industryID, err := s.normalizeOptionalIndustryID(ctx, req.IndustryID)
		if err != nil {
			return err
		}
		live2d.IndustryID = industryID
	}
	if req.Name != "" {
		live2d.Name = req.Name
	}
	if req.Scene != "" {
		live2d.Scene = req.Scene
	}
	if req.ModelURL != "" {
		live2d.ModelURL = req.ModelURL
	}
	if req.ThumbnailURL != "" {
		live2d.ThumbnailURL = req.ThumbnailURL
	}
	if req.ConfigJSON != "" {
		live2d.ConfigJSON = req.ConfigJSON
	}
	if req.IsActive != nil {
		live2d.IsActive = *req.IsActive
	}

	return s.live2DRepo.Update(ctx, live2d)
}

// DeleteLive2DModel 删除指定的 Live2D 模型记录。
func (s *adminService) DeleteLive2DModel(ctx context.Context, id uint) error {
	return s.live2DRepo.Delete(ctx, id)
}

// ImportLive2DPackage 导入后台上传的 Live2D ZIP 包，并返回可直接回填的资源地址。
func (s *adminService) ImportLive2DPackage(ctx context.Context, filename string, content []byte) (*ImportLive2DPackageResponse, error) {
	_ = ctx

	importedPackage, err := live2dassets.ImportZip(filename, content)
	if err != nil {
		return nil, common.NewBusinessError(common.CodeBadRequest, "导入Live2D模型包失败: "+err.Error())
	}

	return &ImportLive2DPackageResponse{
		Name:         importedPackage.Name,
		AssetDir:     importedPackage.AssetDir,
		ModelURL:     importedPackage.ModelURL,
		ThumbnailURL: importedPackage.ThumbnailURL,
	}, nil
}

func (s *adminService) ListTTSConfigs(ctx context.Context) ([]model.TTSConfig, error) {
	return s.ttsRepo.List(ctx)
}

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

func (s *adminService) UpdateTTSConfig(ctx context.Context, id uint, req *UpdateTTSConfigRequest) error {
	cfg, err := s.ttsRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if cfg == nil {
		return common.NewBusinessError(common.CodeNotFound, "tts config not found")
	}

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

func (s *adminService) DeleteTTSConfig(ctx context.Context, id uint) error {
	return s.ttsRepo.Delete(ctx, id)
}

func parseQuestionOptions(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}

	var options []string
	if err := json.Unmarshal([]byte(raw), &options); err == nil {
		return options
	}

	return []string{}
}

func parseQuestionTags(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}

	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '，'
	})

	tags := make([]string, 0, len(parts))
	for _, part := range parts {
		tag := strings.TrimSpace(part)
		if tag == "" {
			continue
		}
		tags = append(tags, tag)
	}

	return tags
}

func validateQuestionPayload(questionType, optionsJSON string) error {
	if questionType == model.QuestionTypeChoice || questionType == model.QuestionTypeMulti {
		if strings.TrimSpace(optionsJSON) == "" {
			return common.NewBusinessError(common.CodeBadRequest, "choice questions require options_json")
		}

		var options []string
		if err := json.Unmarshal([]byte(optionsJSON), &options); err != nil {
			return common.NewBusinessError(common.CodeBadRequest, "options_json must be a valid JSON array")
		}
		if len(options) < 2 {
			return common.NewBusinessError(common.CodeBadRequest, "choice questions require at least two options")
		}
	}

	return nil
}

func (s *adminService) validateQuestionRefs(ctx context.Context, industryID, categoryID uint) error {
	if _, err := s.requireIndustry(ctx, industryID); err != nil {
		return err
	}

	category, err := s.adminCategoryRepo.GetByID(ctx, categoryID)
	if err != nil {
		return err
	}
	if category == nil {
		return common.NewBusinessError(common.CodeNotFound, "category not found")
	}
	if category.IndustryID != industryID {
		return common.NewBusinessError(common.CodeBadRequest, "category does not belong to the selected industry")
	}

	return nil
}

func (s *adminService) validateCategoryRefs(ctx context.Context, industryID uint, parentID *uint, currentCategoryID uint) error {
	if _, err := s.requireIndustry(ctx, industryID); err != nil {
		return err
	}

	if parentID == nil || *parentID == 0 {
		return nil
	}
	if currentCategoryID != 0 && *parentID == currentCategoryID {
		return common.NewBusinessError(common.CodeBadRequest, "category parent cannot be itself")
	}

	parent, err := s.adminCategoryRepo.GetByID(ctx, *parentID)
	if err != nil {
		return err
	}
	if parent == nil {
		return common.NewBusinessError(common.CodeNotFound, "parent category not found")
	}
	if parent.IndustryID != industryID {
		return common.NewBusinessError(common.CodeBadRequest, "parent category does not belong to the selected industry")
	}

	if currentCategoryID == 0 {
		return nil
	}

	categories, err := s.adminCategoryRepo.List(ctx)
	if err != nil {
		return err
	}

	categoryMap := make(map[uint]model.Category, len(categories))
	for _, category := range categories {
		categoryMap[category.ID] = category
	}

	currentParentID := parent.ParentID
	for currentParentID != nil && *currentParentID != 0 {
		if *currentParentID == currentCategoryID {
			return common.NewBusinessError(common.CodeBadRequest, "category hierarchy cannot contain cycles")
		}

		nextCategory, ok := categoryMap[*currentParentID]
		if !ok {
			break
		}
		currentParentID = nextCategory.ParentID
	}

	return nil
}

func (s *adminService) normalizeOptionalIndustryID(ctx context.Context, industryID *uint) (*uint, error) {
	if industryID == nil || *industryID == 0 {
		return nil, nil
	}

	if _, err := s.requireIndustry(ctx, *industryID); err != nil {
		return nil, err
	}

	return industryID, nil
}

func (s *adminService) requireIndustry(ctx context.Context, industryID uint) (*model.Industry, error) {
	industry, err := s.industryRepo.GetByID(ctx, industryID)
	if err != nil {
		return nil, err
	}
	if industry == nil {
		return nil, common.NewBusinessError(common.CodeNotFound, "industry not found")
	}

	return industry, nil
}

func normalizedParentID(parentID *uint) *uint {
	if parentID == nil || *parentID == 0 {
		return nil
	}
	return parentID
}
