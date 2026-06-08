package biz

import (
	"context"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	kratoserr "github.com/go-kratos/kratos/v2/errors"
)

// AdminRepo data 层必须实现的接口
type AdminRepo interface {
	// 分类管理
	ListCategories(ctx context.Context) ([]*CategoryRecord, error)
	CreateCategory(ctx context.Context, c *CategoryRecord) error
	UpdateCategory(ctx context.Context, c *CategoryRecord) error
	DeleteCategory(ctx context.Context, id uint64) error

	// 行业管理
	ListIndustries(ctx context.Context) ([]*IndustryRecord, error)
	CreateIndustry(ctx context.Context, ind *IndustryRecord) error
	UpdateIndustry(ctx context.Context, ind *IndustryRecord) error
	GetIndustryByCode(ctx context.Context, code string) (*IndustryRecord, error)

	// AI 预设
	ListAIPresets(ctx context.Context) ([]*AIPreset, error)
	SaveAIPreset(ctx context.Context, preset *AIPreset) error
	CreateAIPreset(ctx context.Context, preset *AIPreset) error
	UpdateAIPreset(ctx context.Context, preset *AIPreset) error
	DeleteAIPreset(ctx context.Context, id uint64) error
	GetAIPresetByID(ctx context.Context, id uint64) (*AIPreset, error)
	ApplyAIPreset(ctx context.Context, id uint64) error

	// Prompt 模板
	ListPromptTemplates(ctx context.Context, industryCode string) ([]*PromptTemplate, error)
	SavePromptTemplate(ctx context.Context, tpl *PromptTemplate) error
	CreatePromptTemplate(ctx context.Context, tpl *PromptTemplate) error
	UpdatePromptTemplate(ctx context.Context, tpl *PromptTemplate) error
	DeletePromptTemplate(ctx context.Context, id uint64) error

	// 系统配置
	GetAdminConfig(ctx context.Context, key string) (string, error)
	SetAdminConfig(ctx context.Context, key, value string) error
	ListAdminConfigs(ctx context.Context) ([]*AdminConfigItem, error)
	BatchUpsertConfigs(ctx context.Context, configs map[string]string) error

	// AI 调用日志
	ListAICallLogs(ctx context.Context, filter AICallLogListFilter) ([]*AICallLog, int64, error)
	GetAICallLog(ctx context.Context, id uint64) (*AICallLogDetail, error)

	// Live2D 管理
	ListLive2DModels(ctx context.Context) ([]*Live2DModelRecord, error)
	CreateLive2DModel(ctx context.Context, m *Live2DModelRecord) error
	UpdateLive2DModel(ctx context.Context, m *Live2DModelRecord) error
	DeleteLive2DModel(ctx context.Context, id uint64) error

	// TTS 管理
	ListTTSConfigs(ctx context.Context) ([]*TTSConfigRecord, error)
	CreateTTSConfig(ctx context.Context, t *TTSConfigRecord) error
	UpdateTTSConfig(ctx context.Context, t *TTSConfigRecord) error
	DeleteTTSConfig(ctx context.Context, id uint64) error

	// Scraper 管理
	CreateScraperTask(ctx context.Context, task *ScraperTaskRecord) error
	ListScraperTasks(ctx context.Context, page, pageSize int32, status, taskType string) ([]*ScraperTaskRecord, int64, error)
	GetScraperTask(ctx context.Context, id uint64) (*ScraperTaskRecord, error)
	UpdateScraperTask(ctx context.Context, task *ScraperTaskRecord) error

	// RAG 文档管理
	ListRAGDocuments(ctx context.Context, page, pageSize int32, collection, docType, keyword, syncStatus string) ([]*RAGDocumentRecord, int64, error)
	GetRAGDocument(ctx context.Context, id uint64) (*RAGDocumentRecord, error)
	CreateRAGDocument(ctx context.Context, doc *RAGDocumentRecord) error
	UpdateRAGDocument(ctx context.Context, doc *RAGDocumentRecord) error
	DeleteRAGDocument(ctx context.Context, id uint64) error
	BatchCreateRAGDocuments(ctx context.Context, docs []*RAGDocumentRecord) (int, int, []string)
	GetRAGDocumentStats(ctx context.Context, collection string) (map[string]int64, error)
	GetPendingSyncRAGDocuments(ctx context.Context, limit int) ([]*RAGDocumentRecord, error)
	UpdateRAGDocumentSyncStatus(ctx context.Context, id uint64, status, vectorID string) error
}

// UserClient 下游用户服务客户端接口，通过 gRPC 调用 user 微服务
type UserClient interface {
	ListUsers(ctx context.Context, page, pageSize int32) ([]*UserRecord, int64, error)
	UpdateUserRole(ctx context.Context, userID uint64, role string) error
	BanUser(ctx context.Context, userID uint64) error
	GetUserStats(ctx context.Context) (totalUsers, proMembers, newUsersToday, todayActiveUsers int64, err error)
}

// QuestionClient 下游题目服务客户端接口，通过 gRPC 调用 question 微服务
type QuestionClient interface {
	ListQuestions(ctx context.Context, page, pageSize int32, keyword, difficulty string, categoryID uint64, industryCode string) ([]*QuestionRecord, int64, error)
	CreateQuestion(ctx context.Context, q *QuestionRecord) error
	UpdateQuestion(ctx context.Context, q *QuestionRecord) error
	DeleteQuestion(ctx context.Context, id uint64) error
	GetQuestionStats(ctx context.Context) (totalQuestions int64, err error)
}

// InterviewClient 下游面试服务客户端接口，通过 gRPC 调用 interview 微服务
type InterviewClient interface {
	GetInterviewStats(ctx context.Context) (totalInterviews int64, err error)
}

// AIGatewayClient 下游 AI 网关客户端接口（UNIMPLEMENTED：预留，待 AI Gateway 实现 admin RPC）
type AIGatewayClient interface{}

// --- 领域实体 ---

type Dashboard struct {
	TotalUsers       int64
	TotalQuestions   int64
	TotalInterviews  int64
	TodayActiveUsers int64
	ProMembers       int64
	NewUsersToday    int64
}

type UserRecord struct {
	ID                 uint64
	Username           string
	Email              string
	Role               string
	Avatar             string
	MembershipLevel    string
	MembershipType     string
	MembershipExpireAt *time.Time
	IsDisabled         bool
	CreatedAt          time.Time
}

type QuestionRecord struct {
	ID                 uint64
	CategoryID         uint64
	IndustryID         uint64
	Type               string
	Difficulty         string
	Title              string
	Content            string
	OptionsJSON        string
	Answer             string
	Explanation        string
	SolutionJSON       string
	JudgeConfigJSON    string
	AnswerTemplateJSON string
	Tags               string
	IsActive           bool
	HasIsActive        bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
	CategoryName       string
	IndustryName       string
}

type CategoryRecord struct {
	ID          uint64
	IndustryID  uint64
	Name        string
	ParentID    uint64
	SortOrder   int32
	Icon        string
	Description string
	CreatedAt   time.Time
	Children    []*CategoryRecord
}

type IndustryRecord struct {
	ID          uint64
	Code        string
	Name        string
	Description string
	Icon        string
	IsActive    bool
	SortOrder   int32
	CreatedAt   time.Time
}

type AIPreset struct {
	ID        uint64
	Name      string
	Provider  string
	Model     string
	Params    map[string]string
	Configs   map[string]string
	IsDefault bool
	IsActive  bool
	UpdatedAt time.Time
	CreatedAt time.Time
}

type PromptTemplate struct {
	ID              uint64
	IndustryID      uint64
	Name            string
	IndustryCode    string
	TemplateType    string
	Scene           string
	Content         string
	TemplateContent string
	Variables       string
	IsActive        bool
	UpdatedAt       time.Time
}

type AdminConfigItem struct {
	Key         string
	Value       string
	ConfigType  string
	Description string
}

type AICallLog struct {
	ID         uint64
	AgentType  string
	Model      string
	TokensUsed int32
	LatencyMs  int64
	Status     string
	CreatedAt  time.Time
}

// AICallLogListFilter 描述 AI 调用日志列表支持的筛选条件，保持与单体后台一致。
type AICallLogListFilter struct {
	Page      int32
	PageSize  int32
	AgentType string
	Scene     string
	Source    string
	Status    string
	TraceID   string
	TaskID    *uint
}

type AICallLogDetail struct {
	ID              uint64
	TraceID         string
	Source          string
	Scene           string
	Provider        string
	Model           string
	UserInput       string
	ModelOutput     string
	ModelError      string
	LatencyMs       int64
	IsSuccess       bool
	InputTokens     int
	OutputTokens    int
	RenderedPrompt  string
	RequestMessages string
	RuntimeConfig   string
	CreatedAt       time.Time
}

type Live2DModelRecord struct {
	ID           uint64
	Name         string
	IndustryID   uint64
	Scene        string
	ModelURL     string
	ThumbnailURL string
	ConfigJSON   string
	TTSConfigID  uint64
	IsActive     bool
	CreatedAt    time.Time
}

type TTSConfigRecord struct {
	ID             uint64
	Name           string
	Engine         string
	VoiceID        string
	AuthConfigJSON string
	ParamsJSON     string
	IsActive       bool
	SortOrder      int32
	CreatedAt      time.Time
}

type TagTaxonomyGroup struct {
	Category string
	Tags     []string
}

type ScraperTaskRecord struct {
	ID            uint64
	TaskType      string
	SourceURL     string
	SourceTitle   string
	Source        string
	Status        string
	PayloadJSON   string
	ResultJSON    string
	QuestionCount int
	ImportedCount int
	RetryCount    int
	StartedAt     *time.Time
	FinishedAt    *time.Time
	ErrorMsg      string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type RAGDocumentRecord struct {
	ID         uint64
	Collection string
	DocType    string
	Title      string
	Content    string
	Metadata   string
	VectorID   string
	SyncStatus string
	IsActive   bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// AdminUseCase 管理后台业务用例
type AdminUseCase struct {
	repo            AdminRepo
	userClient      UserClient
	questionClient  QuestionClient
	interviewClient InterviewClient
	aiGatewayClient AIGatewayClient
}

// NewAdminUseCase 创建管理后台用例，注入仓库和下游服务客户端
func NewAdminUseCase(repo AdminRepo, userClient UserClient, questionClient QuestionClient, interviewClient InterviewClient, aiGatewayClient AIGatewayClient) *AdminUseCase {
	return &AdminUseCase{
		repo:            repo,
		userClient:      userClient,
		questionClient:  questionClient,
		interviewClient: interviewClient,
		aiGatewayClient: aiGatewayClient,
	}
}

// --- 仪表盘 ---

// GetDashboard 并发调用用户/题目服务的统计接口，聚合仪表盘数据
func (uc *AdminUseCase) GetDashboard(ctx context.Context) (*Dashboard, error) {
	d := &Dashboard{}

	g, gctx := errgroup.WithContext(ctx)

	// 并发获取用户统计数据
	g.Go(func() error {
		totalUsers, proMembers, newUsersToday, todayActiveUsers, err := uc.userClient.GetUserStats(gctx)
		if err != nil {
			return kratoserr.InternalServer("USER_STATS_FAILED", "获取用户统计失败")
		}
		d.TotalUsers = totalUsers
		d.ProMembers = proMembers
		d.NewUsersToday = newUsersToday
		d.TodayActiveUsers = todayActiveUsers
		return nil
	})

	// 并发获取题目统计数据
	g.Go(func() error {
		totalQuestions, err := uc.questionClient.GetQuestionStats(gctx)
		if err != nil {
			return kratoserr.InternalServer("QUESTION_STATS_FAILED", "获取题目统计失败")
		}
		d.TotalQuestions = totalQuestions
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	if uc.interviewClient != nil {
		totalInterviews, err := uc.interviewClient.GetInterviewStats(ctx)
		if err != nil {
			return nil, kratoserr.InternalServer("INTERVIEW_STATS_FAILED", "获取面试统计失败")
		}
		d.TotalInterviews = totalInterviews
	}
	return d, nil
}

// --- 用户管理 ---

// ListUsers 委托给下游用户服务获取用户列表
func (uc *AdminUseCase) ListUsers(ctx context.Context, page, pageSize int32) ([]*UserRecord, int64, error) {
	return uc.userClient.ListUsers(ctx, page, pageSize)
}

// UpdateUserRole 委托给下游用户服务更新用户角色
func (uc *AdminUseCase) UpdateUserRole(ctx context.Context, userID uint64, role string) error {
	return uc.userClient.UpdateUserRole(ctx, userID, role)
}

// DisableUser 委托给下游用户服务封禁用户
func (uc *AdminUseCase) DisableUser(ctx context.Context, userID uint64) error {
	return uc.userClient.BanUser(ctx, userID)
}

// --- 题库管理 ---

// AdminListQuestions 委托给下游题目服务获取题目列表
func (uc *AdminUseCase) AdminListQuestions(ctx context.Context, page, pageSize int32, keyword, difficulty string, categoryID uint64, industryCode string) ([]*QuestionRecord, int64, error) {
	return uc.questionClient.ListQuestions(ctx, page, pageSize, keyword, difficulty, categoryID, industryCode)
}

// CreateQuestion 委托给下游题目服务创建题目
func (uc *AdminUseCase) CreateQuestion(ctx context.Context, q *QuestionRecord) error {
	return uc.questionClient.CreateQuestion(ctx, q)
}

// UpdateQuestion 委托给下游题目服务更新题目
func (uc *AdminUseCase) UpdateQuestion(ctx context.Context, q *QuestionRecord) error {
	return uc.questionClient.UpdateQuestion(ctx, q)
}

// DeleteQuestion 委托给下游题目服务删除题目
func (uc *AdminUseCase) DeleteQuestion(ctx context.Context, id uint64) error {
	return uc.questionClient.DeleteQuestion(ctx, id)
}

func (uc *AdminUseCase) BatchImportQuestions(ctx context.Context, questions []*QuestionRecord) (int, int, []string) {
	success, fail := 0, 0
	errors := make([]string, 0)
	for _, question := range questions {
		if err := uc.questionClient.CreateQuestion(ctx, question); err != nil {
			fail++
			errors = append(errors, err.Error())
			continue
		}
		success++
	}
	return success, fail, errors
}

func (uc *AdminUseCase) GetQuestionTagTaxonomy(ctx context.Context) ([]*TagTaxonomyGroup, error) {
	const pageSize int32 = 200
	page := int32(1)
	tagMap := make(map[string]map[string]bool)
	for {
		questions, total, err := uc.questionClient.ListQuestions(ctx, page, pageSize, "", "", 0, "")
		if err != nil {
			return nil, err
		}
		for _, question := range questions {
			category := question.Type
			if category == "" {
				category = "unknown"
			}
			if tagMap[category] == nil {
				tagMap[category] = make(map[string]bool)
			}
			for _, tag := range splitAdminTags(question.Tags) {
				tagMap[category][tag] = true
			}
		}
		if int64(page*pageSize) >= total || len(questions) == 0 {
			break
		}
		page++
	}

	groups := make([]*TagTaxonomyGroup, 0, len(tagMap))
	for category, tags := range tagMap {
		tagList := make([]string, 0, len(tags))
		for tag := range tags {
			tagList = append(tagList, tag)
		}
		groups = append(groups, &TagTaxonomyGroup{Category: category, Tags: tagList})
	}
	return groups, nil
}

// --- 分类管理 ---

func (uc *AdminUseCase) ListCategories(ctx context.Context) ([]*CategoryRecord, error) {
	return uc.repo.ListCategories(ctx)
}

func (uc *AdminUseCase) CreateCategory(ctx context.Context, c *CategoryRecord) error {
	return uc.repo.CreateCategory(ctx, c)
}

func (uc *AdminUseCase) UpdateCategory(ctx context.Context, c *CategoryRecord) error {
	return uc.repo.UpdateCategory(ctx, c)
}

func (uc *AdminUseCase) DeleteCategory(ctx context.Context, id uint64) error {
	return uc.repo.DeleteCategory(ctx, id)
}

// --- 行业管理 ---

func (uc *AdminUseCase) ListIndustries(ctx context.Context) ([]*IndustryRecord, error) {
	return uc.repo.ListIndustries(ctx)
}

func (uc *AdminUseCase) CreateIndustry(ctx context.Context, ind *IndustryRecord) error {
	return uc.repo.CreateIndustry(ctx, ind)
}

func (uc *AdminUseCase) UpdateIndustry(ctx context.Context, ind *IndustryRecord) error {
	return uc.repo.UpdateIndustry(ctx, ind)
}

func (uc *AdminUseCase) GetIndustryByCode(ctx context.Context, code string) (*IndustryRecord, error) {
	return uc.repo.GetIndustryByCode(ctx, code)
}

// --- AI 预设 ---

func (uc *AdminUseCase) ListAIPresets(ctx context.Context) ([]*AIPreset, error) {
	return uc.repo.ListAIPresets(ctx)
}

func (uc *AdminUseCase) SaveAIPreset(ctx context.Context, preset *AIPreset) error {
	return uc.repo.SaveAIPreset(ctx, preset)
}

func (uc *AdminUseCase) CreateAIPreset(ctx context.Context, preset *AIPreset) error {
	return uc.repo.CreateAIPreset(ctx, preset)
}

// GetAIPresetByID 返回指定 AI 预设详情，供服务层做部分更新时复用现有快照。
func (uc *AdminUseCase) GetAIPresetByID(ctx context.Context, id uint64) (*AIPreset, error) {
	return uc.repo.GetAIPresetByID(ctx, id)
}

func (uc *AdminUseCase) UpdateAIPreset(ctx context.Context, preset *AIPreset) error {
	return uc.repo.UpdateAIPreset(ctx, preset)
}

func (uc *AdminUseCase) DeleteAIPreset(ctx context.Context, id uint64) error {
	return uc.repo.DeleteAIPreset(ctx, id)
}

func (uc *AdminUseCase) ApplyAIPreset(ctx context.Context, id uint64) error {
	return uc.repo.ApplyAIPreset(ctx, id)
}

// --- Prompt 模板 ---

func (uc *AdminUseCase) ListPromptTemplates(ctx context.Context, industryCode string) ([]*PromptTemplate, error) {
	return uc.repo.ListPromptTemplates(ctx, industryCode)
}

func (uc *AdminUseCase) SavePromptTemplate(ctx context.Context, tpl *PromptTemplate) error {
	return uc.repo.SavePromptTemplate(ctx, tpl)
}

func (uc *AdminUseCase) CreatePromptTemplate(ctx context.Context, tpl *PromptTemplate) error {
	return uc.repo.CreatePromptTemplate(ctx, tpl)
}

func (uc *AdminUseCase) UpdatePromptTemplate(ctx context.Context, tpl *PromptTemplate) error {
	return uc.repo.UpdatePromptTemplate(ctx, tpl)
}

func (uc *AdminUseCase) DeletePromptTemplate(ctx context.Context, id uint64) error {
	return uc.repo.DeletePromptTemplate(ctx, id)
}

// --- 系统配置 ---

func (uc *AdminUseCase) GetAdminConfig(ctx context.Context, key string) (string, error) {
	return uc.repo.GetAdminConfig(ctx, key)
}

func (uc *AdminUseCase) SetAdminConfig(ctx context.Context, key, value string) error {
	return uc.repo.SetAdminConfig(ctx, key, value)
}

func (uc *AdminUseCase) ListAdminConfigs(ctx context.Context) ([]*AdminConfigItem, error) {
	return uc.repo.ListAdminConfigs(ctx)
}

func (uc *AdminUseCase) BatchUpsertConfigs(ctx context.Context, configs map[string]string) error {
	return uc.repo.BatchUpsertConfigs(ctx, configs)
}

// --- AI 调用日志 ---

// ListAICallLogs 按筛选条件查询 AI 调用日志列表。
func (uc *AdminUseCase) ListAICallLogs(ctx context.Context, filter AICallLogListFilter) ([]*AICallLog, int64, error) {
	return uc.repo.ListAICallLogs(ctx, filter)
}

func (uc *AdminUseCase) GetAICallLog(ctx context.Context, id uint64) (*AICallLogDetail, error) {
	return uc.repo.GetAICallLog(ctx, id)
}

// --- Live2D 管理 ---

func (uc *AdminUseCase) ListLive2DModels(ctx context.Context) ([]*Live2DModelRecord, error) {
	return uc.repo.ListLive2DModels(ctx)
}

func (uc *AdminUseCase) CreateLive2DModel(ctx context.Context, m *Live2DModelRecord) error {
	return uc.repo.CreateLive2DModel(ctx, m)
}

func (uc *AdminUseCase) UpdateLive2DModel(ctx context.Context, m *Live2DModelRecord) error {
	return uc.repo.UpdateLive2DModel(ctx, m)
}

func (uc *AdminUseCase) DeleteLive2DModel(ctx context.Context, id uint64) error {
	return uc.repo.DeleteLive2DModel(ctx, id)
}

// --- TTS 管理 ---

func (uc *AdminUseCase) ListTTSConfigs(ctx context.Context) ([]*TTSConfigRecord, error) {
	return uc.repo.ListTTSConfigs(ctx)
}

func (uc *AdminUseCase) CreateTTSConfig(ctx context.Context, t *TTSConfigRecord) error {
	return uc.repo.CreateTTSConfig(ctx, t)
}

func (uc *AdminUseCase) UpdateTTSConfig(ctx context.Context, t *TTSConfigRecord) error {
	return uc.repo.UpdateTTSConfig(ctx, t)
}

func (uc *AdminUseCase) DeleteTTSConfig(ctx context.Context, id uint64) error {
	return uc.repo.DeleteTTSConfig(ctx, id)
}

// --- Scraper 管理 ---

func (uc *AdminUseCase) CreateScraperTask(ctx context.Context, task *ScraperTaskRecord) error {
	return uc.repo.CreateScraperTask(ctx, task)
}

func (uc *AdminUseCase) ListScraperTasks(ctx context.Context, page, pageSize int32, status, taskType string) ([]*ScraperTaskRecord, int64, error) {
	return uc.repo.ListScraperTasks(ctx, page, pageSize, status, taskType)
}

func (uc *AdminUseCase) GetScraperTask(ctx context.Context, id uint64) (*ScraperTaskRecord, error) {
	return uc.repo.GetScraperTask(ctx, id)
}

func (uc *AdminUseCase) UpdateScraperTask(ctx context.Context, task *ScraperTaskRecord) error {
	return uc.repo.UpdateScraperTask(ctx, task)
}

// --- RAG 文档管理 ---

func (uc *AdminUseCase) ListRAGDocuments(ctx context.Context, page, pageSize int32, collection, docType, keyword, syncStatus string) ([]*RAGDocumentRecord, int64, error) {
	return uc.repo.ListRAGDocuments(ctx, page, pageSize, collection, docType, keyword, syncStatus)
}

func (uc *AdminUseCase) GetRAGDocument(ctx context.Context, id uint64) (*RAGDocumentRecord, error) {
	return uc.repo.GetRAGDocument(ctx, id)
}

func (uc *AdminUseCase) CreateRAGDocument(ctx context.Context, doc *RAGDocumentRecord) error {
	return uc.repo.CreateRAGDocument(ctx, doc)
}

func (uc *AdminUseCase) UpdateRAGDocument(ctx context.Context, doc *RAGDocumentRecord) error {
	return uc.repo.UpdateRAGDocument(ctx, doc)
}

func (uc *AdminUseCase) DeleteRAGDocument(ctx context.Context, id uint64) error {
	return uc.repo.DeleteRAGDocument(ctx, id)
}

func (uc *AdminUseCase) BatchImportRAGDocuments(ctx context.Context, docs []*RAGDocumentRecord) (int, int, []string) {
	return uc.repo.BatchCreateRAGDocuments(ctx, docs)
}

func (uc *AdminUseCase) GetRAGDocumentStats(ctx context.Context, collection string) (map[string]int64, error) {
	return uc.repo.GetRAGDocumentStats(ctx, collection)
}

func (uc *AdminUseCase) GetPendingSyncRAGDocuments(ctx context.Context, limit int) ([]*RAGDocumentRecord, error) {
	return uc.repo.GetPendingSyncRAGDocuments(ctx, limit)
}

func (uc *AdminUseCase) UpdateRAGDocumentSyncStatus(ctx context.Context, id uint64, status, vectorID string) error {
	return uc.repo.UpdateRAGDocumentSyncStatus(ctx, id, status, vectorID)
}

// splitAdminTags 将后台持久化的逗号标签拆分为列表。
func splitAdminTags(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	tags := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			tags = append(tags, trimmed)
		}
	}
	return tags
}
