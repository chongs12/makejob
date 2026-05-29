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
	"makejob-backend/internal/mq"
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
	ID             uint                        `json:"id"`
	CreatedAt      time.Time                   `json:"created_at"`
	UpdatedAt      time.Time                   `json:"updated_at"`
	CategoryID     uint                        `json:"category_id"`
	CategoryName   string                      `json:"category_name"`
	IndustryID     uint                        `json:"industry_id"`
	Type           string                      `json:"type"`
	Difficulty     string                      `json:"difficulty"`
	Title          string                      `json:"title"`
	Content        string                      `json:"content"`
	Options        []string                    `json:"options"`
	Answer         string                      `json:"answer"`
	Explanation    string                      `json:"explanation"`
	Solution       *QuestionStructuredSolution `json:"solution,omitempty"`
	JudgeConfig    *QuestionJudgeConfigDetail  `json:"judge_config,omitempty"`
	AnswerTemplate *QuestionAnswerTemplate     `json:"answer_template,omitempty"`
	Tags           []string                    `json:"tags"`
	IsActive       bool                        `json:"is_active"`
}

type AdminCreateQuestionRequest struct {
	CategoryID     uint                        `json:"category_id" binding:"required"`
	IndustryID     uint                        `json:"industry_id" binding:"required"`
	Type           string                      `json:"type" binding:"required,oneof=choice multi code subjective"`
	Difficulty     string                      `json:"difficulty" binding:"required,oneof=easy medium hard"`
	Title          string                      `json:"title" binding:"required,max=500"`
	Content        string                      `json:"content" binding:"required"`
	OptionsJSON    string                      `json:"options_json,omitempty"`
	Answer         string                      `json:"answer" binding:"required"`
	Explanation    string                      `json:"explanation"`
	Solution       *QuestionStructuredSolution `json:"solution,omitempty"`
	JudgeConfig    *QuestionJudgeConfig        `json:"judge_config,omitempty"`
	AnswerTemplate *QuestionAnswerTemplate     `json:"answer_template,omitempty"`
	Tags           string                      `json:"tags"`
	IsActive       bool                        `json:"is_active"`
}

type AdminUpdateQuestionRequest struct {
	CategoryID     uint                        `json:"category_id,omitempty"`
	IndustryID     uint                        `json:"industry_id,omitempty"`
	Type           string                      `json:"type,omitempty" binding:"omitempty,oneof=choice multi code subjective"`
	Difficulty     string                      `json:"difficulty,omitempty" binding:"omitempty,oneof=easy medium hard"`
	Title          string                      `json:"title,omitempty" binding:"omitempty,max=500"`
	Content        string                      `json:"content,omitempty"`
	OptionsJSON    string                      `json:"options_json,omitempty"`
	Answer         string                      `json:"answer,omitempty"`
	Explanation    string                      `json:"explanation,omitempty"`
	Solution       *QuestionStructuredSolution `json:"solution,omitempty"`
	JudgeConfig    *QuestionJudgeConfig        `json:"judge_config,omitempty"`
	AnswerTemplate *QuestionAnswerTemplate     `json:"answer_template,omitempty"`
	Tags           string                      `json:"tags,omitempty"`
	IsActive       *bool                       `json:"is_active,omitempty"`
}

type BatchImportRequest struct {
	IndustryCode string               `json:"industry_code" binding:"required"`
	Questions    []ImportQuestionItem `json:"questions" binding:"required,min=1"`
}

type ImportQuestionItem struct {
	CategoryName   string                      `json:"category_name" binding:"required"`
	Type           string                      `json:"type" binding:"required"`
	Difficulty     string                      `json:"difficulty" binding:"required"`
	Title          string                      `json:"title" binding:"required"`
	Content        string                      `json:"content" binding:"required"`
	OptionsJSON    string                      `json:"options_json,omitempty"`
	Answer         string                      `json:"answer" binding:"required"`
	Explanation    string                      `json:"explanation"`
	Solution       *QuestionStructuredSolution `json:"solution,omitempty"`
	JudgeConfig    *QuestionJudgeConfig        `json:"judge_config,omitempty"`
	AnswerTemplate *QuestionAnswerTemplate     `json:"answer_template,omitempty"`
	Tags           string                      `json:"tags"`
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
	TTSConfigID  *uint  `json:"tts_config_id,omitempty"`
	IsActive     bool   `json:"is_active"`
}

type UpdateLive2DModelRequest struct {
	Name         string `json:"name,omitempty" binding:"omitempty,max=100"`
	IndustryID   *uint  `json:"industry_id,omitempty"`
	Scene        string `json:"scene,omitempty" binding:"omitempty,oneof=interview companion"`
	ModelURL     string `json:"model_url,omitempty" binding:"omitempty,max=500"`
	ThumbnailURL string `json:"thumbnail_url,omitempty"`
	ConfigJSON   string `json:"config_json,omitempty"`
	TTSConfigID  *uint  `json:"tts_config_id,omitempty"`
	IsActive     *bool  `json:"is_active,omitempty"`
}

// ImportLive2DPackageResponse 描述后台导入 Live2D 模型包后的自动识别结果。
type ImportLive2DPackageResponse struct {
	Name         string `json:"name"`
	AssetDir     string `json:"asset_dir"`
	ModelURL     string `json:"model_url"`
	ThumbnailURL string `json:"thumbnail_url,omitempty"`
	ModelID      uint   `json:"model_id,omitempty"`
	Created      bool   `json:"created"`
	IsActive     bool   `json:"is_active"`
}

// ImportLive2DBackgroundResponse 描述后台上传舞台背景图后的可访问资源结果。
type ImportLive2DBackgroundResponse struct {
	FileName string `json:"file_name"`
	AssetURL string `json:"asset_url"`
}

type CreateTTSConfigRequest struct {
	Name           string `json:"name" binding:"required,max=100"`
	Engine         string `json:"engine" binding:"required,max=32"`
	VoiceID        string `json:"voice_id" binding:"required,max=100"`
	AuthConfigJSON string `json:"auth_config_json,omitempty"`
	ParamsJSON     string `json:"params_json,omitempty"`
	IsActive       bool   `json:"is_active"`
	SortOrder      int    `json:"sort_order"`
}

type UpdateTTSConfigRequest struct {
	Name           string `json:"name,omitempty" binding:"omitempty,max=100"`
	Engine         string `json:"engine,omitempty" binding:"omitempty,max=32"`
	VoiceID        string `json:"voice_id,omitempty" binding:"omitempty,max=100"`
	AuthConfigJSON string `json:"auth_config_json,omitempty"`
	ParamsJSON     string `json:"params_json,omitempty"`
	IsActive       *bool  `json:"is_active,omitempty"`
	SortOrder      *int   `json:"sort_order,omitempty"`
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
	GetQuestionTagTaxonomy(ctx context.Context) ([]QuestionTagTaxonomyGroup, error)
	GenerateQuestionPipeline(ctx context.Context, req *AdminQuestionPipelineGenerateRequest) (*AdminQuestionPipelineGenerateResponse, error)
	CreateQuestionPipelineTask(ctx context.Context, req *AdminQuestionPipelineGenerateRequest) (*model.ScraperTask, error)
	GenerateQuestionPipelineStream(ctx context.Context, req *AdminQuestionPipelineGenerateRequest, emit AdminQuestionPipelineStreamEmitter) error
	ImportQuestionPipeline(ctx context.Context, req *AdminQuestionPipelineImportRequest) (*BatchImportResponse, error)
	RunNextPendingQuestionPipelineTask(ctx context.Context) (*model.ScraperTask, bool, error)

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
	ListAIPresets(ctx context.Context) ([]AIPresetSummary, error)
	CreateAIPreset(ctx context.Context, req *CreateAIPresetRequest) (*AIPresetSummary, error)
	UpdateAIPreset(ctx context.Context, id uint, req *UpdateAIPresetRequest) (*AIPresetSummary, error)
	DeleteAIPreset(ctx context.Context, id uint) error
	ApplyAIPreset(ctx context.Context, id uint) (*AIConfigResponse, error)
	DebugAIRuntime(ctx context.Context, req *AIDebugRequest) (*AIDebugResponse, error)
	ListAICallLogs(ctx context.Context, req *ListAICallLogsRequest) (*common.PageResult, error)
	GetAICallLog(ctx context.Context, id uint) (*model.AICallLog, error)

	ListLive2DModels(ctx context.Context) ([]model.Live2DModel, error)
	CreateLive2DModel(ctx context.Context, req *CreateLive2DModelRequest) (*model.Live2DModel, error)
	UpdateLive2DModel(ctx context.Context, id uint, req *UpdateLive2DModelRequest) error
	DeleteLive2DModel(ctx context.Context, id uint) error
	ImportLive2DPackage(ctx context.Context, filename string, content []byte) (*ImportLive2DPackageResponse, error)
	ImportLive2DBackground(ctx context.Context, filename string, content []byte) (*ImportLive2DBackgroundResponse, error)

	ListTTSConfigs(ctx context.Context) (*TTSConfigListResponse, error)
	CreateTTSConfig(ctx context.Context, req *CreateTTSConfigRequest) (*model.TTSConfig, error)
	UpdateTTSConfig(ctx context.Context, id uint, req *UpdateTTSConfigRequest) error
	DeleteTTSConfig(ctx context.Context, id uint) error
	UpdateTTSSceneDefaults(ctx context.Context, req *UpdateTTSSceneDefaultsRequest) error
}

type adminService struct {
	adminUserRepo     repository.AdminUserRepository
	adminQuestionRepo repository.AdminQuestionRepository
	industryRepo      repository.IndustryRepository
	adminCategoryRepo repository.AdminCategoryRepository
	promptRepo        repository.PromptTemplateRepository
	adminConfigRepo   repository.AdminConfigRepository
	aiPresetRepo      repository.AIPresetRepository
	aiCallLogRepo     repository.AICallLogRepository
	live2DRepo        repository.Live2DModelRepository
	ttsRepo           repository.TTSConfigRepository
	mockInterviewRepo repository.MockInterviewRepository
	scraperTaskRepo   repository.ScraperTaskRepository
	scraperProvider   scraper.ScraperProvider
	questionCleaner   scraper.QuestionCleaner
	baseAIConfig      map[string]string
	taskPublisher     mq.TaskPublisher
	asyncEnabled      bool
}

// NewAdminService 创建后台管理服务。
func NewAdminService(
	adminUserRepo repository.AdminUserRepository,
	adminQuestionRepo repository.AdminQuestionRepository,
	industryRepo repository.IndustryRepository,
	adminCategoryRepo repository.AdminCategoryRepository,
	promptRepo repository.PromptTemplateRepository,
	adminConfigRepo repository.AdminConfigRepository,
	aiPresetRepo repository.AIPresetRepository,
	aiCallLogRepo repository.AICallLogRepository,
	live2DRepo repository.Live2DModelRepository,
	ttsRepo repository.TTSConfigRepository,
	mockInterviewRepo repository.MockInterviewRepository,
	scraperTaskRepo repository.ScraperTaskRepository,
	scraperProvider scraper.ScraperProvider,
	questionCleaner scraper.QuestionCleaner,
	baseAIConfig map[string]string,
	deps ...interface{},
) AdminService {
	service := &adminService{
		adminUserRepo:     adminUserRepo,
		adminQuestionRepo: adminQuestionRepo,
		industryRepo:      industryRepo,
		adminCategoryRepo: adminCategoryRepo,
		promptRepo:        promptRepo,
		adminConfigRepo:   adminConfigRepo,
		aiPresetRepo:      aiPresetRepo,
		aiCallLogRepo:     aiCallLogRepo,
		live2DRepo:        live2DRepo,
		ttsRepo:           ttsRepo,
		mockInterviewRepo: mockInterviewRepo,
		scraperTaskRepo:   scraperTaskRepo,
		scraperProvider:   scraperProvider,
		questionCleaner:   questionCleaner,
		baseAIConfig:      ai.NormalizeRuntimeConfig(baseAIConfig),
	}
	for _, dep := range deps {
		if option, ok := dep.(AsyncDispatchOption); ok {
			service.asyncEnabled = option.Enabled
			service.taskPublisher = option.Publisher
		}
	}
	return service
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
	pageParam := common.PageParam{Page: page, PageSize: pageSize}
	pageParam.Normalize()

	users, total, err := s.adminUserRepo.List(ctx, pageParam.Page, pageParam.PageSize, keyword, role)
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

	return common.NewPageResult(items, total, pageParam), nil
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
	pageParam := common.PageParam{Page: page, PageSize: pageSize}
	pageParam.Normalize()

	questions, total, err := s.adminQuestionRepo.List(ctx, pageParam.Page, pageParam.PageSize, keyword, difficulty, categoryID)
	if err != nil {
		return nil, err
	}

	items := make([]AdminQuestionListItem, 0, len(questions))
	for _, question := range questions {
		items = append(items, AdminQuestionListItem{
			ID:             question.ID,
			CreatedAt:      question.CreatedAt,
			UpdatedAt:      question.UpdatedAt,
			CategoryID:     question.CategoryID,
			CategoryName:   question.Category.Name,
			IndustryID:     question.IndustryID,
			Type:           question.Type,
			Difficulty:     question.Difficulty,
			Title:          question.Title,
			Content:        question.Content,
			Options:        parseQuestionOptions(question.OptionsJSON),
			Answer:         question.Answer,
			Explanation:    question.Explanation,
			Solution:       parseQuestionStructuredSolution(question.SolutionJSON, &question),
			JudgeConfig:    buildQuestionJudgeConfigDetail(parseQuestionJudgeConfig(question.JudgeConfigJSON, &question), true),
			AnswerTemplate: parseQuestionAnswerTemplate(question.AnswerTemplateJSON, &question),
			Tags:           parseQuestionTagsFromStorage(question.Tags),
			IsActive:       question.IsActive,
		})
	}

	return common.NewPageResult(items, total, pageParam), nil
}

func (s *adminService) CreateQuestion(ctx context.Context, req *AdminCreateQuestionRequest) (*model.Question, error) {
	if err := s.validateQuestionRefs(ctx, req.IndustryID, req.CategoryID); err != nil {
		return nil, err
	}
	if err := validateQuestionPayload(req.Type, req.OptionsJSON); err != nil {
		return nil, err
	}

	normalizedQuestion := &model.Question{
		CategoryID:  req.CategoryID,
		IndustryID:  req.IndustryID,
		Type:        req.Type,
		Difficulty:  req.Difficulty,
		Title:       strings.TrimSpace(req.Title),
		Content:     strings.TrimSpace(req.Content),
		OptionsJSON: req.OptionsJSON,
		Answer:      strings.TrimSpace(req.Answer),
		Explanation: strings.TrimSpace(req.Explanation),
		Tags:        normalizeQuestionTagsForStorage(req.Tags),
		IsActive:    req.IsActive,
	}
	solutionJSON, err := marshalQuestionStructuredSolution(req.Solution, normalizedQuestion)
	if err != nil {
		return nil, common.NewBusinessError(common.CodeBadRequest, "solution 字段格式错误")
	}
	judgeConfigJSON, err := marshalQuestionJudgeConfig(req.JudgeConfig, normalizedQuestion)
	if err != nil {
		return nil, common.NewBusinessError(common.CodeBadRequest, "judge_config 字段格式错误")
	}
	if err := validateQuestionJudgeConfig(normalizedQuestion, normalizeQuestionJudgeConfig(req.JudgeConfig, normalizedQuestion)); err != nil {
		return nil, common.NewBusinessError(common.CodeBadRequest, err.Error())
	}
	answerTemplateJSON, err := marshalQuestionAnswerTemplate(req.AnswerTemplate, normalizedQuestion)
	if err != nil {
		return nil, common.NewBusinessError(common.CodeBadRequest, "answer_template 字段格式错误")
	}
	normalizedQuestion.SolutionJSON = solutionJSON
	normalizedQuestion.JudgeConfigJSON = judgeConfigJSON
	normalizedQuestion.AnswerTemplateJSON = answerTemplateJSON

	if err := s.adminQuestionRepo.Create(ctx, normalizedQuestion); err != nil {
		return nil, err
	}

	return normalizedQuestion, nil
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
	question.Tags = normalizeQuestionTagsForStorage(req.Tags)
	if req.Solution != nil || strings.TrimSpace(question.SolutionJSON) == "" {
		solutionJSON, err := marshalQuestionStructuredSolution(req.Solution, question)
		if err != nil {
			return common.NewBusinessError(common.CodeBadRequest, "solution 字段格式错误")
		}
		question.SolutionJSON = solutionJSON
	}
	if req.JudgeConfig != nil || (question.IsCode() && strings.TrimSpace(question.JudgeConfigJSON) == "") {
		judgeConfigJSON, err := marshalQuestionJudgeConfig(req.JudgeConfig, question)
		if err != nil {
			return common.NewBusinessError(common.CodeBadRequest, "judge_config 字段格式错误")
		}
		if err := validateQuestionJudgeConfig(question, normalizeQuestionJudgeConfig(req.JudgeConfig, question)); err != nil {
			return common.NewBusinessError(common.CodeBadRequest, err.Error())
		}
		question.JudgeConfigJSON = judgeConfigJSON
	}
	if req.AnswerTemplate != nil || (question.Type == model.QuestionTypeSubjective && strings.TrimSpace(question.AnswerTemplateJSON) == "") {
		answerTemplateJSON, err := marshalQuestionAnswerTemplate(req.AnswerTemplate, question)
		if err != nil {
			return common.NewBusinessError(common.CodeBadRequest, "answer_template 字段格式错误")
		}
		question.AnswerTemplateJSON = answerTemplateJSON
	}
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
			Title:       strings.TrimSpace(item.Title),
			Content:     strings.TrimSpace(item.Content),
			OptionsJSON: item.OptionsJSON,
			Answer:      strings.TrimSpace(item.Answer),
			Explanation: strings.TrimSpace(item.Explanation),
			Tags:        normalizeQuestionTagsForStorage(item.Tags),
			IsActive:    true,
		})
		lastQuestion := &questionsToImport[len(questionsToImport)-1]
		solutionJSON, err := marshalQuestionStructuredSolution(item.Solution, lastQuestion)
		if err != nil {
			response.Errors = append(response.Errors, fmt.Sprintf("question %d: solution 字段格式错误", index+1))
			questionsToImport = questionsToImport[:len(questionsToImport)-1]
			continue
		}
		judgeConfigJSON, err := marshalQuestionJudgeConfig(item.JudgeConfig, lastQuestion)
		if err != nil {
			response.Errors = append(response.Errors, fmt.Sprintf("question %d: judge_config 字段格式错误", index+1))
			questionsToImport = questionsToImport[:len(questionsToImport)-1]
			continue
		}
		if err := validateQuestionJudgeConfig(lastQuestion, normalizeQuestionJudgeConfig(item.JudgeConfig, lastQuestion)); err != nil {
			response.Errors = append(response.Errors, fmt.Sprintf("question %d: %s", index+1, err.Error()))
			questionsToImport = questionsToImport[:len(questionsToImport)-1]
			continue
		}
		answerTemplateJSON, err := marshalQuestionAnswerTemplate(item.AnswerTemplate, lastQuestion)
		if err != nil {
			response.Errors = append(response.Errors, fmt.Sprintf("question %d: answer_template 字段格式错误", index+1))
			questionsToImport = questionsToImport[:len(questionsToImport)-1]
			continue
		}
		lastQuestion.SolutionJSON = solutionJSON
		lastQuestion.JudgeConfigJSON = judgeConfigJSON
		lastQuestion.AnswerTemplateJSON = answerTemplateJSON
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

// GetQuestionTagTaxonomy 返回后台题库管理可复用的标准标签词典。
func (s *adminService) GetQuestionTagTaxonomy(ctx context.Context) ([]QuestionTagTaxonomyGroup, error) {
	return standardQuestionTagTaxonomy(), nil
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

// GetAIConfigs 返回后台 AI 配置页需要的配置、支持范围和当前告警信息。
func (s *adminService) GetAIConfigs(ctx context.Context) (*AIConfigResponse, error) {
	items, err := s.adminConfigRepo.List(ctx)
	if err != nil {
		return nil, err
	}

	response := buildAIConfigResponse(items, s.baseAIConfig)
	if s.aiPresetRepo == nil {
		return response, nil
	}

	presets, err := s.aiPresetRepo.List(ctx)
	if err != nil {
		return nil, err
	}

	summaries, activePresetID, err := buildAIPresetSummaries(presets)
	if err != nil {
		return nil, common.NewBusinessError(common.CodeInternalError, "parse ai presets failed: "+err.Error())
	}

	response.Presets = summaries
	response.ActivePresetID = activePresetID
	return response, nil
}

// UpdateAIConfigs 校验并持久化后台提交的 AI 运行配置。
func (s *adminService) UpdateAIConfigs(ctx context.Context, configs map[string]string) error {
	normalizedConfigs, err := normalizeStoredAIConfigs(configs)
	if err != nil {
		return common.NewBusinessError(common.CodeBadRequest, err.Error())
	}

	adminConfigs := buildAIConfigItems(normalizedConfigs)
	if len(adminConfigs) == 0 {
		return common.NewBusinessError(common.CodeBadRequest, "no valid ai configs provided")
	}

	if err := s.adminConfigRepo.BatchUpsert(ctx, adminConfigs); err != nil {
		return err
	}

	return s.syncActiveAIPresetSnapshot(ctx, normalizedConfigs)
}

// ListLive2DModels 返回后台可维护的 Live2D 模型列表。
func (s *adminService) ListLive2DModels(ctx context.Context) ([]model.Live2DModel, error) {
	if err := s.syncDiscoveredLive2DModels(ctx); err != nil {
		return nil, err
	}
	return s.live2DRepo.List(ctx)
}

// CreateLive2DModel 创建一条新的 Live2D 模型配置记录。
func (s *adminService) CreateLive2DModel(ctx context.Context, req *CreateLive2DModelRequest) (*model.Live2DModel, error) {
	industryID, err := s.normalizeOptionalIndustryID(ctx, req.IndustryID)
	if err != nil {
		return nil, err
	}
	ttsConfigID, err := s.normalizeOptionalTTSConfigID(ctx, req.TTSConfigID)
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
		TTSConfigID:  ttsConfigID,
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
	if req.TTSConfigID != nil {
		ttsConfigID, err := s.normalizeOptionalTTSConfigID(ctx, req.TTSConfigID)
		if err != nil {
			return err
		}
		live2d.TTSConfigID = ttsConfigID
	}
	if req.IsActive != nil {
		live2d.IsActive = *req.IsActive
	}

	return s.live2DRepo.Update(ctx, live2d)
}

// DeleteLive2DModel 删除指定的 Live2D 模型记录。
func (s *adminService) DeleteLive2DModel(ctx context.Context, id uint) error {
	if s.live2DRepo == nil {
		return common.NewBusinessError(common.CodeInternalError, "live2d repository not configured")
	}

	targetModel, err := s.live2DRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if targetModel == nil {
		return common.NewBusinessError(common.CodeNotFound, "live2d model not found")
	}

	if err := s.deleteManagedLive2DAssetsIfUnused(ctx, targetModel); err != nil {
		return err
	}

	return s.live2DRepo.Delete(ctx, id)
}

// ImportLive2DPackage 导入后台上传的 Live2D ZIP 包，并返回可直接回填的资源地址。
func (s *adminService) ImportLive2DPackage(ctx context.Context, filename string, content []byte) (*ImportLive2DPackageResponse, error) {
	importedPackage, err := live2dassets.ImportZip(filename, content)
	if err != nil {
		return nil, common.NewBusinessError(common.CodeBadRequest, "导入Live2D模型包失败: "+err.Error())
	}

	importedModel, created, err := s.ensureImportedLive2DModel(ctx, importedPackage)
	if err != nil {
		return nil, err
	}

	return &ImportLive2DPackageResponse{
		Name:         importedPackage.Name,
		AssetDir:     importedPackage.AssetDir,
		ModelURL:     importedPackage.ModelURL,
		ThumbnailURL: importedPackage.ThumbnailURL,
		ModelID:      importedModel.ID,
		Created:      created,
		IsActive:     importedModel.IsActive,
	}, nil
}

// ImportLive2DBackground 导入后台上传的舞台背景图，并返回可直接回填的静态地址。
func (s *adminService) ImportLive2DBackground(ctx context.Context, filename string, content []byte) (*ImportLive2DBackgroundResponse, error) {
	_ = ctx

	importedBackground, err := live2dassets.ImportBackgroundImage(filename, content)
	if err != nil {
		return nil, common.NewBusinessError(common.CodeBadRequest, "导入Live2D背景图失败: "+err.Error())
	}

	return &ImportLive2DBackgroundResponse{
		FileName: importedBackground.FileName,
		AssetURL: importedBackground.AssetURL,
	}, nil
}

// ListTTSConfigs 返回后台页维护 TTS 所需的完整配置视图。
func (s *adminService) ListTTSConfigs(ctx context.Context) (*TTSConfigListResponse, error) {
	configs, err := s.ttsRepo.List(ctx)
	if err != nil {
		return nil, err
	}
	return BuildTTSConfigListResponse(ctx, configs, s.adminConfigRepo)
}

// CreateTTSConfig 创建一条新的可运行 TTS 配置记录。
func (s *adminService) CreateTTSConfig(ctx context.Context, req *CreateTTSConfigRequest) (*model.TTSConfig, error) {
	normalizedAuthConfig, normalizedParamsConfig, err := ValidateTTSConfigInput(req.Engine, req.VoiceID, req.AuthConfigJSON, req.ParamsJSON)
	if err != nil {
		return nil, common.NewBusinessError(common.CodeBadRequest, err.Error())
	}

	cfg := &model.TTSConfig{
		Name:           req.Name,
		Engine:         strings.TrimSpace(req.Engine),
		VoiceID:        strings.TrimSpace(req.VoiceID),
		AuthConfigJSON: normalizedAuthConfig,
		ParamsJSON:     normalizedParamsConfig,
		IsActive:       req.IsActive,
		SortOrder:      req.SortOrder,
	}

	if err := s.ttsRepo.Create(ctx, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// UpdateTTSConfig 更新后台已有的 TTS 配置记录。
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
		cfg.Engine = strings.TrimSpace(req.Engine)
	}
	if req.VoiceID != "" {
		cfg.VoiceID = strings.TrimSpace(req.VoiceID)
	}
	if req.AuthConfigJSON != "" {
		cfg.AuthConfigJSON = req.AuthConfigJSON
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

	normalizedAuthConfig, normalizedParamsConfig, validateErr := ValidateTTSConfigInput(cfg.Engine, cfg.VoiceID, cfg.AuthConfigJSON, cfg.ParamsJSON)
	if validateErr != nil {
		return common.NewBusinessError(common.CodeBadRequest, validateErr.Error())
	}
	cfg.AuthConfigJSON = normalizedAuthConfig
	cfg.ParamsJSON = normalizedParamsConfig

	return s.ttsRepo.Update(ctx, cfg)
}

// DeleteTTSConfig 删除指定 TTS 配置。
func (s *adminService) DeleteTTSConfig(ctx context.Context, id uint) error {
	return s.ttsRepo.Delete(ctx, id)
}

// UpdateTTSSceneDefaults 更新后台按场景维度配置的默认 TTS 绑定。
func (s *adminService) UpdateTTSSceneDefaults(ctx context.Context, req *UpdateTTSSceneDefaultsRequest) error {
	if req == nil {
		return common.NewBusinessError(common.CodeBadRequest, "tts default bindings are required")
	}

	for scene, configID := range req.DefaultBindings {
		if _, err := normalizeLive2DScene(scene); err != nil {
			return common.NewBusinessError(common.CodeBadRequest, "invalid tts default scene")
		}
		if configID == 0 {
			continue
		}
		if _, err := s.loadActiveTTSConfig(ctx, configID); err != nil {
			return err
		}
	}

	adminConfigs, err := BuildTTSDefaultSceneConfigs(req.DefaultBindings)
	if err != nil {
		return common.NewBusinessError(common.CodeBadRequest, err.Error())
	}
	return s.adminConfigRepo.BatchUpsert(ctx, adminConfigs)
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
	return parseQuestionTagsFromStorage(raw)
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

// normalizeOptionalTTSConfigID 校验可选的 TTS 配置绑定并统一空值语义。
func (s *adminService) normalizeOptionalTTSConfigID(ctx context.Context, ttsConfigID *uint) (*uint, error) {
	if ttsConfigID == nil || *ttsConfigID == 0 {
		return nil, nil
	}

	if _, err := s.loadActiveTTSConfig(ctx, *ttsConfigID); err != nil {
		return nil, err
	}
	return ttsConfigID, nil
}

// loadActiveTTSConfig 加载一条存在且已启用的 TTS 配置，供绑定校验复用。
func (s *adminService) loadActiveTTSConfig(ctx context.Context, configID uint) (*model.TTSConfig, error) {
	if s.ttsRepo == nil {
		return nil, common.NewBusinessError(common.CodeInternalError, "tts repository not configured")
	}

	record, err := s.ttsRepo.GetByID(ctx, configID)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, common.NewBusinessError(common.CodeNotFound, "tts config not found")
	}
	if !record.IsActive {
		return nil, common.NewBusinessError(common.CodeBadRequest, "tts config is inactive")
	}
	return record, nil
}

// deleteManagedLive2DAssetsIfUnused 在没有其他模型复用同一资源目录时，删除对应的本地模型目录。
func (s *adminService) deleteManagedLive2DAssetsIfUnused(ctx context.Context, targetModel *model.Live2DModel) error {
	if s.live2DRepo == nil || targetModel == nil {
		return nil
	}

	managedAssetDir := live2dassets.ManagedModelAssetDirFromURL(targetModel.ModelURL)
	if managedAssetDir == "" {
		return nil
	}

	allModels, err := s.live2DRepo.List(ctx)
	if err != nil {
		return err
	}

	for _, currentModel := range allModels {
		if currentModel.ID == targetModel.ID {
			continue
		}
		if normalizeLive2DAssetURL(currentModel.ModelURL) == normalizeLive2DAssetURL(targetModel.ModelURL) {
			return nil
		}
		if live2dassets.ManagedModelAssetDirFromURL(currentModel.ModelURL) == managedAssetDir {
			return nil
		}
	}

	if err := live2dassets.DeleteManagedModelAssetDir(managedAssetDir); err != nil {
		return common.NewBusinessError(common.CodeInternalError, "删除Live2D模型资源失败: "+err.Error())
	}
	return nil
}

// syncDiscoveredLive2DModels 将本地资源目录中未入库的模型补登记到后台管理列表。
func (s *adminService) syncDiscoveredLive2DModels(ctx context.Context) error {
	if s.live2DRepo == nil {
		return nil
	}

	discoveredModels, err := live2dassets.DiscoverLocalModels()
	if err != nil {
		return common.NewBusinessError(common.CodeInternalError, "扫描本地Live2D资源失败: "+err.Error())
	}
	if len(discoveredModels) == 0 {
		return nil
	}

	existingModels, err := s.live2DRepo.List(ctx)
	if err != nil {
		return err
	}

	existingByModelURL := make(map[string]struct{}, len(existingModels))
	for _, existingModel := range existingModels {
		modelURL := normalizeLive2DAssetURL(existingModel.ModelURL)
		if modelURL == "" {
			continue
		}
		existingByModelURL[modelURL] = struct{}{}
	}

	for _, discoveredModel := range discoveredModels {
		modelURL := normalizeLive2DAssetURL(discoveredModel.ModelURL)
		if modelURL == "" {
			continue
		}
		if _, exists := existingByModelURL[modelURL]; exists {
			continue
		}

		newModel := buildPendingImportedLive2DModel(&discoveredModel)
		if err := s.live2DRepo.Create(ctx, newModel); err != nil {
			return err
		}
		existingByModelURL[modelURL] = struct{}{}
	}

	return nil
}

// ensureImportedLive2DModel 确保导入出来的模型资源同步存在于后台模型表中。
func (s *adminService) ensureImportedLive2DModel(
	ctx context.Context,
	importedPackage *live2dassets.ImportedPackage,
) (*model.Live2DModel, bool, error) {
	if s.live2DRepo == nil {
		return nil, false, common.NewBusinessError(common.CodeInternalError, "live2d repository not configured")
	}
	if importedPackage == nil {
		return nil, false, common.NewBusinessError(common.CodeBadRequest, "imported live2d package is required")
	}

	existingModels, err := s.live2DRepo.List(ctx)
	if err != nil {
		return nil, false, err
	}

	targetModelURL := normalizeLive2DAssetURL(importedPackage.ModelURL)
	for i := range existingModels {
		if normalizeLive2DAssetURL(existingModels[i].ModelURL) == targetModelURL {
			return &existingModels[i], false, nil
		}
	}

	newModel := buildPendingImportedLive2DModel(importedPackage)
	if err := s.live2DRepo.Create(ctx, newModel); err != nil {
		return nil, false, err
	}
	return newModel, true, nil
}

// buildPendingImportedLive2DModel 基于自动识别结果创建一条待后台确认的模型记录。
func buildPendingImportedLive2DModel(importedPackage *live2dassets.ImportedPackage) *model.Live2DModel {
	return &model.Live2DModel{
		Name:         strings.TrimSpace(importedPackage.Name),
		Scene:        model.Live2DSceneCompanion,
		ModelURL:     strings.TrimSpace(importedPackage.ModelURL),
		ThumbnailURL: strings.TrimSpace(importedPackage.ThumbnailURL),
		ConfigJSON:   "",
		IsActive:     false,
	}
}

// normalizeLive2DAssetURL 统一清洗资源地址，避免因空格或斜杠差异导致重复落库。
func normalizeLive2DAssetURL(value string) string {
	return strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
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

// ListAIPresets 返回后台 AI 预设列表。
func (s *adminService) ListAIPresets(ctx context.Context) ([]AIPresetSummary, error) {
	if s.aiPresetRepo == nil {
		return []AIPresetSummary{}, nil
	}

	presets, err := s.aiPresetRepo.List(ctx)
	if err != nil {
		return nil, err
	}

	summaries, _, err := buildAIPresetSummaries(presets)
	if err != nil {
		return nil, common.NewBusinessError(common.CodeInternalError, "parse ai presets failed: "+err.Error())
	}
	return summaries, nil
}

// CreateAIPreset 基于一份完整 AI 配置快照创建新预设。
func (s *adminService) CreateAIPreset(ctx context.Context, req *CreateAIPresetRequest) (*AIPresetSummary, error) {
	if s.aiPresetRepo == nil {
		return nil, common.NewBusinessError(common.CodeInternalError, "ai preset repository not configured")
	}

	presetName := normalizeAIPresetName(req.Name)
	if presetName == "" {
		return nil, common.NewBusinessError(common.CodeBadRequest, "preset name is required")
	}
	if err := s.ensureAIPresetNameAvailable(ctx, presetName, 0); err != nil {
		return nil, err
	}

	configJSON, err := serializeAIPresetConfigs(req.Configs)
	if err != nil {
		return nil, common.NewBusinessError(common.CodeBadRequest, err.Error())
	}

	preset := &model.AIPreset{
		Name:       presetName,
		ConfigJSON: configJSON,
		IsActive:   false,
	}
	if err := s.aiPresetRepo.Create(ctx, preset); err != nil {
		return nil, err
	}

	summary, err := buildAIPresetSummary(*preset)
	if err != nil {
		return nil, common.NewBusinessError(common.CodeInternalError, "parse ai preset failed: "+err.Error())
	}
	return &summary, nil
}

// UpdateAIPreset 更新指定 AI 预设的名称或配置快照。
func (s *adminService) UpdateAIPreset(ctx context.Context, id uint, req *UpdateAIPresetRequest) (*AIPresetSummary, error) {
	preset, err := s.getAIPresetOrNotFound(ctx, id)
	if err != nil {
		return nil, err
	}

	hasChanges := false
	if req.Name != nil {
		presetName := normalizeAIPresetName(*req.Name)
		if presetName == "" {
			return nil, common.NewBusinessError(common.CodeBadRequest, "preset name is required")
		}
		if err := s.ensureAIPresetNameAvailable(ctx, presetName, preset.ID); err != nil {
			return nil, err
		}
		preset.Name = presetName
		hasChanges = true
	}

	if req.Configs != nil {
		configJSON, err := serializeAIPresetConfigs(req.Configs)
		if err != nil {
			return nil, common.NewBusinessError(common.CodeBadRequest, err.Error())
		}
		preset.ConfigJSON = configJSON
		hasChanges = true
	}

	if !hasChanges {
		return nil, common.NewBusinessError(common.CodeBadRequest, "no preset changes provided")
	}
	if err := s.aiPresetRepo.Update(ctx, preset); err != nil {
		return nil, err
	}

	if preset.IsActive && req.Configs != nil {
		configs, err := parseAIPresetConfigs(preset.ConfigJSON)
		if err != nil {
			return nil, common.NewBusinessError(common.CodeInternalError, "parse ai preset failed: "+err.Error())
		}
		if err := s.adminConfigRepo.BatchUpsert(ctx, buildAIConfigItems(configs)); err != nil {
			return nil, err
		}
	}

	summary, err := buildAIPresetSummary(*preset)
	if err != nil {
		return nil, common.NewBusinessError(common.CodeInternalError, "parse ai preset failed: "+err.Error())
	}
	return &summary, nil
}

// DeleteAIPreset 删除指定 AI 预设，当前生效预设不允许直接删除。
func (s *adminService) DeleteAIPreset(ctx context.Context, id uint) error {
	preset, err := s.getAIPresetOrNotFound(ctx, id)
	if err != nil {
		return err
	}
	if preset.IsActive {
		return common.NewBusinessError(common.CodeBadRequest, "active preset cannot be deleted")
	}
	return s.aiPresetRepo.Delete(ctx, id)
}

// ApplyAIPreset 将指定预设覆盖到当前全局运行配置并标记为生效预设。
func (s *adminService) ApplyAIPreset(ctx context.Context, id uint) (*AIConfigResponse, error) {
	preset, err := s.getAIPresetOrNotFound(ctx, id)
	if err != nil {
		return nil, err
	}

	configs, err := parseAIPresetConfigs(preset.ConfigJSON)
	if err != nil {
		return nil, common.NewBusinessError(common.CodeInternalError, "parse ai preset failed: "+err.Error())
	}
	if err := s.adminConfigRepo.BatchUpsert(ctx, buildAIConfigItems(configs)); err != nil {
		return nil, err
	}
	if err := s.aiPresetRepo.SetActive(ctx, preset.ID); err != nil {
		return nil, err
	}

	return s.GetAIConfigs(ctx)
}

// syncActiveAIPresetSnapshot 在直接保存运行配置后同步更新当前生效预设快照。
func (s *adminService) syncActiveAIPresetSnapshot(ctx context.Context, configs map[string]string) error {
	if s.aiPresetRepo == nil {
		return nil
	}

	activePreset, err := s.aiPresetRepo.GetActive(ctx)
	if err != nil {
		return err
	}
	if activePreset == nil {
		return nil
	}

	configJSON, err := serializeAIPresetConfigs(configs)
	if err != nil {
		return common.NewBusinessError(common.CodeBadRequest, err.Error())
	}
	activePreset.ConfigJSON = configJSON
	return s.aiPresetRepo.Update(ctx, activePreset)
}

// ensureAIPresetNameAvailable 校验预设名称是否可用，并支持排除当前编辑中的记录。
func (s *adminService) ensureAIPresetNameAvailable(ctx context.Context, name string, excludeID uint) error {
	if s.aiPresetRepo == nil {
		return common.NewBusinessError(common.CodeInternalError, "ai preset repository not configured")
	}

	existingPreset, err := s.aiPresetRepo.GetByName(ctx, name)
	if err != nil {
		return err
	}
	if existingPreset != nil && existingPreset.ID != excludeID {
		return common.NewBusinessError(common.CodeBadRequest, "preset name already exists")
	}
	return nil
}

// getAIPresetOrNotFound 获取指定预设，不存在时返回标准业务错误。
func (s *adminService) getAIPresetOrNotFound(ctx context.Context, id uint) (*model.AIPreset, error) {
	if s.aiPresetRepo == nil {
		return nil, common.NewBusinessError(common.CodeInternalError, "ai preset repository not configured")
	}

	preset, err := s.aiPresetRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if preset == nil {
		return nil, common.NewBusinessError(common.CodeNotFound, "ai preset not found")
	}
	return preset, nil
}
