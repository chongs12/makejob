package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	kratoserr "github.com/go-kratos/kratos/v2/errors"
	"makejob/pkg/mq"
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
	ListScraperSources(ctx context.Context) ([]*ScraperSourceRecord, error)

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
	GetQuestion(ctx context.Context, id uint64) (*QuestionRecord, error)
	CreateQuestion(ctx context.Context, q *QuestionRecord) error
	UpdateQuestion(ctx context.Context, q *QuestionRecord) error
	DeleteQuestion(ctx context.Context, id uint64) error
	GetQuestionStats(ctx context.Context) (totalQuestions int64, err error)
}

// InterviewClient 下游面试服务客户端接口，通过 gRPC 调用 interview 微服务
type InterviewClient interface {
	GetInterviewStats(ctx context.Context) (totalInterviews int64, err error)
}

// AIGatewayClient 下游 AI 网关客户端接口，通过 gRPC 调用 AI Gateway 的 admin 调试 RPC
// PipelineStreamEvent 描述题目流水线流式事件。
type PipelineStreamEvent struct {
	Event            string
	Message          string
	TraceID          string
	RawOutput        string
	FailureStage     string
	CandidateExcerpt string
	RepairAttempted  bool
	SupplementAttempted bool
	SlotIndex        int32
	RetryIndex       int32
	Card             *QuestionCandidate
	Response         *GenerateQuestionCandidatesResult
}

// PipelineStreamEmitter 描述题目流水线流式推送回调。
type PipelineStreamEmitter func(event *PipelineStreamEvent) error

type AIGatewayClient interface {
	// RenderPrompt 渲染 Prompt 模板预览
	RenderPrompt(ctx context.Context, scene, templateText string, variables map[string]string, runWithLLM bool) (*RenderPromptResult, error)
	// DebugAI 执行 AI 调试调用
	DebugAI(ctx context.Context, scene, prompt string, params map[string]string, modelOverride string) (*DebugAIResult, error)
	// GenerateQuestionCandidates 同步生成题目候选
	GenerateQuestionCandidates(ctx context.Context, industryCode, requirement, agentPrompt string, candidateCount int32, generationMode string, includeScraped, includeGenerated bool, sources []string, industryName string, categories []string) (*GenerateQuestionCandidatesResult, error)
	// GenerateQuestionCandidatesStream 流式生成题目候选
	GenerateQuestionCandidatesStream(ctx context.Context, industryCode, requirement, agentPrompt string, candidateCount int32, generationMode string, includeScraped, includeGenerated bool, sources []string, industryName string, categories []string, emit PipelineStreamEmitter) error
}

// RenderPromptResult Prompt 渲染结果
type RenderPromptResult struct {
	RenderedPrompt    string
	ResolvedVariables map[string]string
	LLMResponse       string
	Model             string
	LatencyMs         int64
}

// DebugAIResult AI 调试结果
type DebugAIResult struct {
	RenderedPrompt string
	Response       string
	Model          string
	InputTokens    int
	OutputTokens   int
	LatencyMs      int64
	Error          string
}

// GenerateQuestionCandidatesResult 同步候选题生成结果
type GenerateQuestionCandidatesResult struct {
	IndustryCode string
	Requirement  string
	Candidates   []*QuestionCandidate
	Warnings     []string
}

// QuestionCandidate 题目候选（FIX H6: 补齐与 PipelineCard 对齐的字段）
type QuestionCandidate struct {
	ID          string
	Title       string
	Content     string
	Type        string
	Difficulty  string
	Category    string
	Answer      string
	Explanation string
	Tags        []string
	SourceType  string
	Confidence  float64
	Solution    string
	JudgeConfig string
	SourceLabel string
	SourceTitle string
	SourceURL   string
}

// RAGClient 下游 RAG 服务客户端接口，通过 gRPC 调用 rag 微服务
type RAGClient interface {
	TestConnection(ctx context.Context) (milvusOk bool, embeddingOk bool, err error)
	GetConfig(ctx context.Context) (collectionName string, embedModel string, err error)
	UpdateConfig(ctx context.Context, collectionName string, embeddingDimension int32, embedModel string) (string, int32, string, error)
	IndexQuestions(ctx context.Context, items []*RAGIndexItem) (indexed int32, failedIDs []string, err error)
	DeleteIndex(ctx context.Context, ids []string) (deletedCount int32, err error)
	SearchQuestions(ctx context.Context, query string, topK int32) ([]*RAGSearchResult, error)
	GetDocumentStats(ctx context.Context) (totalDocuments int64, totalQuestions int64, err error)
	IndexDocuments(ctx context.Context, items []*RAGDocumentIndexItem) (indexed int32, failedIDs []string, err error)
}

// RAGIndexItem RAG 索引条目
type RAGIndexItem struct {
	QuestionID uint64
	Content    string
	Metadata   map[string]string
}

// RAGSearchResult RAG 搜索结果
type RAGSearchResult struct {
	DocID   string
	Title   string
	Content string
	Score   float64
}

// RAGDocumentIndexItem RAG 文档索引条目
type RAGDocumentIndexItem struct {
	ID       string
	Content  string
	Source   string
	Metadata map[string]string
}

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

type ScraperSourceRecord struct {
	Name     string
	Label    string
	BaseURL  string
	IsActive bool
}

type ScraperSearchResult struct {
	Title   string
	URL     string
	Source  string
	Snippet string
}

type ScraperFetchResult struct {
	Title   string
	Content string
	Source  string
	URL     string
}

type ScraperCleanedQuestionRecord struct {
	CategoryName string
	Type         string
	Difficulty   string
	Title        string
	Content      string
	OptionsJSON  string
	Answer       string
	Explanation  string
	Tags         string
}

type ScraperImportResult struct {
	TotalCount   int
	SuccessCount int
	FailCount    int
	Errors       []string
}

// ScraperProvider 爬虫提供者接口，负责搜索和抓取外部站点内容。
type ScraperProvider interface {
	GetSources() []ScraperSourceRecord
	Search(ctx context.Context, source, keyword string, page, pageSize int32) ([]*ScraperSearchResult, int32, error)
	Fetch(ctx context.Context, source, url string) (*ScraperFetchResult, error)
}

// ScraperCleaner 面经内容清洗器接口，从原始文本中提取结构化题目。
type ScraperCleaner interface {
	Clean(content, industryCode, source, sourceURL string) ([]*ScraperCleanedQuestionRecord, int)
}

// ScraperPublisher 爬虫异步导入消息发布接口。
type ScraperPublisher interface {
	PublishScraperImport(ctx context.Context, taskID uint64, payload []byte) error
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
	repo             AdminRepo
	userClient       UserClient
	questionClient   QuestionClient
	interviewClient  InterviewClient
	aiGatewayClient  AIGatewayClient
	ragClient        RAGClient
	scraperProvider  ScraperProvider
	scraperCleaner   ScraperCleaner
	scraperPublisher ScraperPublisher
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

// SetScraperDeps 注入爬虫相关依赖（提供者、清洗器、异步发布器）。
func (uc *AdminUseCase) SetScraperDeps(provider ScraperProvider, cleaner ScraperCleaner, publisher ScraperPublisher) {
	uc.scraperProvider = provider
	uc.scraperCleaner = cleaner
	uc.scraperPublisher = publisher
}

// SetRAGClient 注入 RAG 服务客户端。
func (uc *AdminUseCase) SetRAGClient(client RAGClient) {
	uc.ragClient = client
}

// AIGatewayClient 返回 AI 网关客户端，供服务层委托调用。
func (uc *AdminUseCase) AIGatewayClient() AIGatewayClient {
	return uc.aiGatewayClient
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

func (uc *AdminUseCase) GetScraperSources(ctx context.Context) ([]*ScraperSourceRecord, error) {
	return uc.repo.ListScraperSources(ctx)
}

func (uc *AdminUseCase) ScraperSearch(ctx context.Context, source, keyword string, page, pageSize int32) ([]*ScraperSearchResult, int32, error) {
	if uc.scraperProvider == nil {
		return nil, 0, kratoserr.ServiceUnavailable("SCRAPER_UNAVAILABLE", "爬虫服务未配置")
	}
	return uc.scraperProvider.Search(ctx, source, keyword, page, pageSize)
}

func (uc *AdminUseCase) ScraperFetch(ctx context.Context, source, url string) (*ScraperFetchResult, error) {
	if uc.scraperProvider == nil {
		return nil, kratoserr.ServiceUnavailable("SCRAPER_UNAVAILABLE", "爬虫服务未配置")
	}
	result, err := uc.scraperProvider.Fetch(ctx, source, url)
	if err != nil {
		return nil, err
	}
	task := &ScraperTaskRecord{
		TaskType:    "fetch_snapshot",
		SourceURL:   url,
		SourceTitle: result.Title,
		Source:      source,
		Status:      "fetched",
		PayloadJSON: result.Content,
	}
	if err := uc.repo.CreateScraperTask(ctx, task); err != nil {
		return nil, err
	}
	return result, nil
}

// ScraperClean 清洗面经内容（FIX M4: 接受 context.Context）
func (uc *AdminUseCase) ScraperClean(ctx context.Context, content, industryCode, source, sourceURL string) ([]*ScraperCleanedQuestionRecord, int) {
	if uc.scraperCleaner == nil {
		return nil, 0
	}
	return uc.scraperCleaner.Clean(content, industryCode, source, sourceURL)
}

// ScraperImport 同步导入清洗后的题目到题库（FIX C1: 透传所有字段）
func (uc *AdminUseCase) ScraperImport(ctx context.Context, industryCode string, questions []*ScraperCleanedQuestionRecord) (*ScraperImportResult, error) {
	if len(questions) == 0 {
		return &ScraperImportResult{}, nil
	}
	industry, err := uc.repo.GetIndustryByCode(ctx, strings.TrimSpace(industryCode))
	if err != nil {
		return nil, err
	}
	categories, err := uc.repo.ListCategories(ctx)
	if err != nil {
		return nil, err
	}
	categoryIDs := buildCategoryIndex(categories, industry.ID)
	result := &ScraperImportResult{
		TotalCount: len(questions),
		Errors:     make([]string, 0),
	}
	for index, q := range questions {
		question, buildErr := uc.buildScraperImportedQuestion(industry, categoryIDs, q)
		if buildErr != nil {
			result.FailCount++
			result.Errors = append(result.Errors, fmt.Sprintf("第%d题: %v", index+1, buildErr))
			continue
		}
		if err := uc.questionClient.CreateQuestion(ctx, question); err != nil {
			result.FailCount++
			result.Errors = append(result.Errors, err.Error())
		} else {
			result.SuccessCount++
		}
	}
	return result, nil
}

// ScraperImportAsync 创建异步导入任务并发布 MQ 消息。
// FIX H2: MQ 发布失败标记 failed 并返回错误
// FIX H3: 先写 DB 获取 task ID，再发布 MQ
// FIX H4: 返回实际持久化状态，不篡改内存对象
func (uc *AdminUseCase) ScraperImportAsync(ctx context.Context, source, industryCode, sourceURL, sourceTitle string, questions []*ScraperCleanedQuestionRecord) (*ScraperTaskRecord, error) {
	if uc.scraperPublisher == nil {
		return nil, kratoserr.ServiceUnavailable("MQ_UNAVAILABLE", "异步消息发布器未配置")
	}

	mqQuestions := make([]mq.ScraperImportQuestion, len(questions))
	for i, q := range questions {
		mqQuestions[i] = mq.ScraperImportQuestion{
			CategoryName: q.CategoryName,
			Title:        q.Title,
			Content:      q.Content,
			Type:         q.Type,
			Difficulty:   q.Difficulty,
			OptionsJSON:  q.OptionsJSON,
			Answer:       q.Answer,
			Explanation:  q.Explanation,
			Tags:         q.Tags,
		}
	}

	payload := mq.ScraperImportPayload{
		Source:       source,
		SourceURL:    sourceURL,
		SourceTitle:  sourceTitle,
		IndustryCode: industryCode,
		Questions:    mqQuestions,
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, kratoserr.InternalServer("PAYLOAD_MARSHAL_FAILED", "序列化导入载荷失败")
	}

	// 先创建任务记录，状态为 pending，待消费者正式领取后再置为 running。
	task := &ScraperTaskRecord{
		TaskType:      "import_questions",
		SourceURL:     sourceURL,
		SourceTitle:   sourceTitle,
		Source:        source,
		Status:        "pending",
		PayloadJSON:   string(payloadJSON),
		QuestionCount: len(questions),
	}
	if task.SourceURL == "" {
		task.SourceURL = "manual://question-import"
	}
	if err := uc.repo.CreateScraperTask(ctx, task); err != nil {
		return nil, err
	}

	// 发布 MQ 消息（带 task ID）
	payload.TaskID = task.ID
	payloadJSONWithID, err := json.Marshal(payload)
	if err != nil {
		return nil, kratoserr.InternalServer("PAYLOAD_MARSHAL_FAILED", "序列化导入载荷失败")
	}
	task.PayloadJSON = string(payloadJSONWithID)
	if err := uc.repo.UpdateScraperTask(ctx, task); err != nil {
		return nil, err
	}
	if err := uc.scraperPublisher.PublishScraperImport(ctx, task.ID, payloadJSONWithID); err != nil {
		// FIX H2: MQ 发布失败，标记任务为 failed 并记录错误
		now := time.Now()
		task.Status = "failed"
		task.ErrorMsg = fmt.Sprintf("MQ 发布失败: %v", err)
		task.FinishedAt = &now
		if updateErr := uc.repo.UpdateScraperTask(ctx, task); updateErr != nil {
			return nil, kratoserr.InternalServer("SCRAPER_IMPORT_PUBLISH_FAILED", fmt.Sprintf("异步导入任务投递失败，且任务状态更新失败: %v / %v", err, updateErr))
		}
		return nil, kratoserr.InternalServer("SCRAPER_IMPORT_PUBLISH_FAILED", fmt.Sprintf("异步导入任务投递失败: %v", err))
	}

	// FIX H4: 直接返回，不篡改内存中的状态
	return task, nil
}

// buildCategoryIndex 为指定行业构造“分类名 -> 分类 ID”的快速索引。
func buildCategoryIndex(categories []*CategoryRecord, industryID uint64) map[string]uint64 {
	index := make(map[string]uint64)
	for _, category := range categories {
		if category == nil || category.IndustryID != industryID {
			continue
		}
		normalized := normalizeCategoryName(category.Name)
		if normalized != "" {
			index[normalized] = category.ID
		}
	}
	return index
}

// normalizeCategoryName 统一分类名匹配口径，避免大小写和空白差异导致导入失败。
func normalizeCategoryName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// buildScraperImportedQuestion 将清洗后的题目转换为下游 question 服务可接受的结构。
func (uc *AdminUseCase) buildScraperImportedQuestion(industry *IndustryRecord, categoryIDs map[string]uint64, question *ScraperCleanedQuestionRecord) (*QuestionRecord, error) {
	if industry == nil {
		return nil, fmt.Errorf("行业不存在")
	}
	if question == nil {
		return nil, fmt.Errorf("题目不能为空")
	}
	categoryName := strings.TrimSpace(question.CategoryName)
	if categoryName == "" {
		return nil, fmt.Errorf("分类不能为空")
	}
	categoryID, ok := categoryIDs[normalizeCategoryName(categoryName)]
	if !ok {
		return nil, fmt.Errorf("分类 %q 不存在或不属于行业 %s", categoryName, industry.Code)
	}
	return &QuestionRecord{
		CategoryID:   categoryID,
		IndustryID:   industry.ID,
		Type:         question.Type,
		Difficulty:   question.Difficulty,
		Title:        question.Title,
		Content:      question.Content,
		OptionsJSON:  question.OptionsJSON,
		Answer:       question.Answer,
		Explanation:  question.Explanation,
		Tags:         question.Tags,
		IsActive:     true,
		HasIsActive:  true,
		CategoryName: categoryName,
	}, nil
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

// --- RAG 服务委托 ---

// TestRAGConnection 委托 RAG 服务测试连接
func (uc *AdminUseCase) TestRAGConnection(ctx context.Context) (milvusOk bool, embeddingOk bool, err error) {
	if uc.ragClient == nil {
		return false, false, kratoserr.ServiceUnavailable("RAG_NOT_CONFIGURED", "RAG 服务未配置")
	}
	return uc.ragClient.TestConnection(ctx)
}

// GetRAGConfig 委托 RAG 服务获取配置
func (uc *AdminUseCase) GetRAGConfig(ctx context.Context) (collectionName string, embedModel string, err error) {
	if uc.ragClient == nil {
		return "", "", kratoserr.ServiceUnavailable("RAG_NOT_CONFIGURED", "RAG 服务未配置")
	}
	return uc.ragClient.GetConfig(ctx)
}

// UpdateRAGConfig 委托 RAG 服务更新运行时配置，并返回更新后的权威值。
func (uc *AdminUseCase) UpdateRAGConfig(ctx context.Context, collectionName string, embeddingDimension int32, embedModel string) (string, int32, string, error) {
	if uc.ragClient == nil {
		return "", 0, "", kratoserr.ServiceUnavailable("RAG_NOT_CONFIGURED", "RAG 服务未配置")
	}
	return uc.ragClient.UpdateConfig(ctx, collectionName, embeddingDimension, embedModel)
}

// IndexAllQuestions 拉取题目列表并批量索引到 RAG
func (uc *AdminUseCase) IndexAllQuestions(ctx context.Context, industryID uint64) (indexed int32, failed int32, err error) {
	if uc.ragClient == nil {
		return 0, 0, kratoserr.ServiceUnavailable("RAG_NOT_CONFIGURED", "RAG 服务未配置")
	}

	// 分页拉取所有题目
	const batchSize int32 = 100
	page := int32(1)
	var totalIndexed, totalFailed int32

	for {
		questions, total, err := uc.questionClient.ListQuestions(ctx, page, batchSize, "", "", 0, "")
		if err != nil {
			return totalIndexed, totalFailed, err
		}

		// 构建索引条目
		items := make([]*RAGIndexItem, 0, len(questions))
		for _, q := range questions {
			if industryID > 0 && q.IndustryID != industryID {
				continue
			}
			if q.Content == "" {
				continue
			}
			items = append(items, &RAGIndexItem{
				QuestionID: q.ID,
				Content:    q.Title + "\n" + q.Content,
				Metadata: map[string]string{
					"title":      q.Title,
					"type":       q.Type,
					"difficulty": q.Difficulty,
				},
			})
		}

		if len(items) > 0 {
			indexed, failedIDs, err := uc.ragClient.IndexQuestions(ctx, items)
			if err != nil {
				return totalIndexed, totalFailed, err
			}
			totalIndexed += indexed
			totalFailed += int32(len(failedIDs))
		}

		if int64(page*batchSize) >= total {
			break
		}
		page++
	}

	return totalIndexed, totalFailed, nil
}

// IndexQuestions 委托 RAG 服务索引指定题目
func (uc *AdminUseCase) IndexQuestions(ctx context.Context, questionIDs []uint64) (indexed int32, failed int32, err error) {
	if uc.ragClient == nil {
		return 0, 0, kratoserr.ServiceUnavailable("RAG_NOT_CONFIGURED", "RAG 服务未配置")
	}

	// 逐个获取题目内容并索引
	items := make([]*RAGIndexItem, 0, len(questionIDs))
	for _, qid := range questionIDs {
		question, err := uc.questionClient.GetQuestion(ctx, qid)
		if err != nil {
			failed++
			continue
		}
		if question == nil || question.Content == "" {
			failed++
			continue
		}
		items = append(items, &RAGIndexItem{
			QuestionID: question.ID,
			Content:    question.Title + "\n" + question.Content,
			Metadata: map[string]string{
				"title":      question.Title,
				"type":       question.Type,
				"difficulty": question.Difficulty,
			},
		})
	}

	if len(items) == 0 {
		return 0, failed, nil
	}

	indexedCount, failedIDs, err := uc.ragClient.IndexQuestions(ctx, items)
	return indexedCount, failed + int32(len(failedIDs)), err
}

// DeleteRAGIndex 委托 RAG 服务删除索引
func (uc *AdminUseCase) DeleteRAGIndex(ctx context.Context, questionIDs []uint64) (deleted int32, err error) {
	if uc.ragClient == nil {
		return 0, kratoserr.ServiceUnavailable("RAG_NOT_CONFIGURED", "RAG 服务未配置")
	}

	ids := make([]string, len(questionIDs))
	for i, qid := range questionIDs {
		ids[i] = fmt.Sprintf("%d", qid)
	}

	deletedCount, err := uc.ragClient.DeleteIndex(ctx, ids)
	return deletedCount, err
}

// SearchRAGQuestions 委托 RAG 服务检索题目
func (uc *AdminUseCase) SearchRAGQuestions(ctx context.Context, query string, topK int32) ([]*RAGSearchResult, error) {
	if uc.ragClient == nil {
		return nil, kratoserr.ServiceUnavailable("RAG_NOT_CONFIGURED", "RAG 服务未配置")
	}
	return uc.ragClient.SearchQuestions(ctx, query, topK)
}

// SyncRAGDocumentsToVectorDB 同步指定文档到向量库
func (uc *AdminUseCase) SyncRAGDocumentsToVectorDB(ctx context.Context, ids []uint64) error {
	if uc.ragClient == nil {
		return kratoserr.ServiceUnavailable("RAG_NOT_CONFIGURED", "RAG 服务未配置")
	}

	// 获取待同步的文档
	var docs []*RAGDocumentRecord
	for _, id := range ids {
		doc, err := uc.repo.GetRAGDocument(ctx, id)
		if err != nil {
			continue
		}
		docs = append(docs, doc)
	}

	if len(docs) == 0 {
		return nil
	}

	// 构建索引条目
	items := make([]*RAGDocumentIndexItem, len(docs))
	for i, doc := range docs {
		items[i] = &RAGDocumentIndexItem{
			ID:       fmt.Sprintf("%d", doc.ID),
			Content:  doc.Content,
			Source:   doc.DocType,
			Metadata: map[string]string{"title": doc.Title, "collection": doc.Collection},
		}
	}

	_, failedIDs, err := uc.ragClient.IndexDocuments(ctx, items)
	if err != nil {
		return err
	}
	failedSet := make(map[string]struct{}, len(failedIDs))
	for _, failedID := range failedIDs {
		failedSet[failedID] = struct{}{}
	}

	// 更新同步状态
	for _, doc := range docs {
		statusText := "synced"
		vectorID := fmt.Sprintf("%d", doc.ID)
		if _, failed := failedSet[vectorID]; failed {
			statusText = "failed"
			vectorID = ""
		}
		if updateErr := uc.repo.UpdateRAGDocumentSyncStatus(ctx, doc.ID, statusText, vectorID); updateErr != nil {
			return updateErr
		}
	}

	return nil
}

// SyncAllPendingRAGDocuments 同步所有待处理文档到向量库
func (uc *AdminUseCase) SyncAllPendingRAGDocuments(ctx context.Context) error {
	if uc.ragClient == nil {
		return kratoserr.ServiceUnavailable("RAG_NOT_CONFIGURED", "RAG 服务未配置")
	}

	// 分批获取待同步文档
	const batchSize = 100
	for {
		docs, err := uc.repo.GetPendingSyncRAGDocuments(ctx, batchSize)
		if err != nil {
			return err
		}
		if len(docs) == 0 {
			break
		}

		items := make([]*RAGDocumentIndexItem, len(docs))
		for i, doc := range docs {
			items[i] = &RAGDocumentIndexItem{
				ID:       fmt.Sprintf("%d", doc.ID),
				Content:  doc.Content,
				Source:   doc.DocType,
				Metadata: map[string]string{"title": doc.Title, "collection": doc.Collection},
			}
		}

		_, failedIDs, err := uc.ragClient.IndexDocuments(ctx, items)
		if err != nil {
			return err
		}
		failedSet := make(map[string]struct{}, len(failedIDs))
		for _, failedID := range failedIDs {
			failedSet[failedID] = struct{}{}
		}

		for _, doc := range docs {
			statusText := "synced"
			vectorID := fmt.Sprintf("%d", doc.ID)
			if _, failed := failedSet[vectorID]; failed {
				statusText = "failed"
				vectorID = ""
			}
			if updateErr := uc.repo.UpdateRAGDocumentSyncStatus(ctx, doc.ID, statusText, vectorID); updateErr != nil {
				return updateErr
			}
		}
	}

	return nil
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
