package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gorm.io/gorm"

	aiRuntime "makejob-backend/internal/ai/runtime"
	asrfactory "makejob-backend/internal/asr/factory"
	"makejob-backend/internal/config"
	"makejob-backend/internal/executor"
	"makejob-backend/internal/handler"
	"makejob-backend/internal/live2dassets"
	"makejob-backend/internal/middleware"
	"makejob-backend/internal/model"
	"makejob-backend/internal/mq"
	"makejob-backend/internal/rag"
	"makejob-backend/internal/repository"
	"makejob-backend/internal/scraper"
	"makejob-backend/internal/service"
	ttsfactory "makejob-backend/internal/tts/factory"
)

type runtimeOptions struct {
	configPath string
}

// RuntimeOption 用于定制 bridge 运行时初始化行为。
type RuntimeOption func(*runtimeOptions)

// WithConfigPath 指定 backend 配置文件路径。
func WithConfigPath(path string) RuntimeOption {
	return func(options *runtimeOptions) {
		options.configPath = strings.TrimSpace(path)
	}
}

// Runtime 聚合 backend 可复用的服务与 HTTP handler。
type Runtime struct {
	cfg               *config.Config
	adminService      service.AdminService
	authHandler       *handler.AuthHandler
	adminHandler      *handler.AdminHandler
	questionHandler   *handler.QuestionHandler
	communityHandler  *handler.CommunityHandler
	planHandler       *handler.PlanHandler
	companionHandler  *handler.CompanionHandler
	growthHandler     *handler.GrowthHandler
	membershipHandler *handler.MembershipHandler
	interviewHandler  *handler.InterviewHandler
	live2DHandler     *handler.Live2DHandler
	scraperHandler    *handler.ScraperHandler
	adminRAGHandler   *handler.AdminRAGHandler
	ragDocHandler     *handler.AdminRAGDocumentHandler
	scraperService    service.ScraperService
	ragDocService     service.RAGDocumentService
	questionRepo      repository.QuestionRepository
	ragService        *rag.Service
	ragInitErr        error
	live2DAssetsDir   string
}

// NewRuntime 创建 bridge 运行时并初始化 backend 依赖。
func NewRuntime(db *gorm.DB, options ...RuntimeOption) (*Runtime, error) {
	if db == nil {
		return nil, fmt.Errorf("bridge runtime requires a database connection")
	}

	opts := runtimeOptions{}
	for _, option := range options {
		option(&opts)
	}

	configPath, err := resolveBackendConfigPath(opts.configPath)
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("load backend config failed: %w", err)
	}
	config.SetConfig(cfg)

	runtime := &Runtime{cfg: cfg}
	if err := runtime.build(db); err != nil {
		return nil, err
	}
	return runtime, nil
}

// Config 返回 bridge 当前使用的 backend 配置。
func (r *Runtime) Config() *config.Config {
	return r.cfg
}

// Live2DAssetsDir 返回 Live2D 静态资源目录。
func (r *Runtime) Live2DAssetsDir() string {
	return r.live2DAssetsDir
}

// GenerateQuestionPipeline 调用 backend 题目流水线生成能力。
func (r *Runtime) GenerateQuestionPipeline(ctx context.Context, req QuestionPipelineGenerateRequest) (*QuestionPipelineGenerateResponse, error) {
	result, err := r.adminService.GenerateQuestionPipeline(ctx, &service.AdminQuestionPipelineGenerateRequest{
		IndustryCode:     req.IndustryCode,
		Requirement:      req.Requirement,
		AgentPrompt:      req.AgentPrompt,
		GenerationMode:   req.GenerationMode,
		CandidateCount:   int(req.CandidateCount),
		IncludeScraped:   req.IncludeScraped,
		IncludeGenerated: req.IncludeGenerated,
		Sources:          req.Sources,
	})
	if err != nil {
		return nil, err
	}
	return convertPipelineGenerateResponse(result), nil
}

// CreateQuestionPipelineTask 调用 backend 创建题目流水线异步任务。
func (r *Runtime) CreateQuestionPipelineTask(ctx context.Context, req QuestionPipelineGenerateRequest) (*TaskInfo, error) {
	task, err := r.adminService.CreateQuestionPipelineTask(ctx, &service.AdminQuestionPipelineGenerateRequest{
		IndustryCode:     req.IndustryCode,
		Requirement:      req.Requirement,
		AgentPrompt:      req.AgentPrompt,
		GenerationMode:   req.GenerationMode,
		CandidateCount:   int(req.CandidateCount),
		IncludeScraped:   req.IncludeScraped,
		IncludeGenerated: req.IncludeGenerated,
		Sources:          req.Sources,
	})
	if err != nil {
		return nil, err
	}
	return &TaskInfo{TaskID: uint64(task.ID), Status: task.Status}, nil
}

// TestRenderPrompt 渲染提示词并在需要时执行真实模型调用。
func (r *Runtime) TestRenderPrompt(ctx context.Context, req AIDebugRequest) (*AIDebugResponse, error) {
	return r.runAIDebug(ctx, req)
}

// DebugAI 执行真实 AI 调试调用。
func (r *Runtime) DebugAI(ctx context.Context, req AIDebugRequest) (*AIDebugResponse, error) {
	return r.runAIDebug(ctx, req)
}

// ImportLive2DPackage 调用 backend 导入 Live2D 模型包。
func (r *Runtime) ImportLive2DPackage(ctx context.Context, filename string, content []byte) (*ImportLive2DPackageResult, error) {
	result, err := r.adminService.ImportLive2DPackage(ctx, filename, content)
	if err != nil {
		return nil, err
	}
	return &ImportLive2DPackageResult{
		Name:         result.Name,
		AssetDir:     result.AssetDir,
		ModelURL:     result.ModelURL,
		ThumbnailURL: result.ThumbnailURL,
		ModelID:      uint64(result.ModelID),
		Created:      result.Created,
		IsActive:     result.IsActive,
	}, nil
}

// ImportLive2DBackground 调用 backend 导入 Live2D 背景资源。
func (r *Runtime) ImportLive2DBackground(ctx context.Context, filename string, content []byte) (*ImportLive2DBackgroundResult, error) {
	result, err := r.adminService.ImportLive2DBackground(ctx, filename, content)
	if err != nil {
		return nil, err
	}
	return &ImportLive2DBackgroundResult{
		FileName: result.FileName,
		AssetURL: result.AssetURL,
	}, nil
}

// TestRAGConnection 调用 backend 检查 RAG 外部依赖连接状态。
func (r *Runtime) TestRAGConnection(ctx context.Context) (*RAGConnectionResult, error) {
	result, err := r.adminService.TestRAGConnection(ctx)
	if err != nil {
		return nil, err
	}
	return &RAGConnectionResult{
		MilvusOK:    result.MilvusOK,
		EmbeddingOK: result.EmbeddingOK,
		Error:       result.Error,
	}, nil
}

// IndexAllQuestions 为题库建立 RAG 索引。
func (r *Runtime) IndexAllQuestions(ctx context.Context, industryID uint64) (*RAGIndexResult, error) {
	if err := r.ensureRAGReady(); err != nil {
		return nil, err
	}

	params := repository.QuestionListParams{Page: 1, PageSize: 100}
	if industryID > 0 {
		id := uint(industryID)
		params.IndustryID = &id
	}

	var questions []model.Question
	for {
		pageItems, total, err := r.questionRepo.List(ctx, params)
		if err != nil {
			return nil, err
		}
		questions = append(questions, pageItems...)
		if int64(len(questions)) >= total || len(pageItems) == 0 {
			break
		}
		params.Page++
	}

	if len(questions) == 0 {
		return &RAGIndexResult{}, nil
	}
	if err := r.ragService.IndexQuestions(ctx, questions); err != nil {
		return nil, err
	}
	return &RAGIndexResult{Indexed: int32(len(questions))}, nil
}

// IndexQuestions 为指定题目建立 RAG 索引。
func (r *Runtime) IndexQuestions(ctx context.Context, questionIDs []uint64) (*RAGIndexResult, error) {
	if err := r.ensureRAGReady(); err != nil {
		return nil, err
	}

	questions := make([]model.Question, 0, len(questionIDs))
	for _, id := range questionIDs {
		question, err := r.questionRepo.GetByID(ctx, uint(id))
		if err != nil {
			return nil, err
		}
		if question != nil {
			questions = append(questions, *question)
		}
	}

	if len(questions) == 0 {
		return &RAGIndexResult{}, nil
	}
	if err := r.ragService.IndexQuestions(ctx, questions); err != nil {
		return nil, err
	}
	return &RAGIndexResult{Indexed: int32(len(questions))}, nil
}

// DeleteRAGIndex 删除指定题目的向量索引。
func (r *Runtime) DeleteRAGIndex(ctx context.Context, questionIDs []uint64) (*RAGIndexResult, error) {
	if err := r.ensureRAGReady(); err != nil {
		return nil, err
	}

	docIDs := make([]string, 0, len(questionIDs))
	for _, id := range questionIDs {
		docIDs = append(docIDs, rag.QuestionIDToDocID(uint(id)))
	}
	if len(docIDs) == 0 {
		return &RAGIndexResult{}, nil
	}
	if err := r.ragService.DeleteByIDs(ctx, docIDs); err != nil {
		return nil, err
	}
	return &RAGIndexResult{Deleted: int32(len(docIDs))}, nil
}

// SearchRAGQuestions 执行真实 RAG 检索。
func (r *Runtime) SearchRAGQuestions(ctx context.Context, query string, topK int32) (*RAGSearchResponse, error) {
	if err := r.ensureRAGReady(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("rag search query cannot be empty")
	}
	if topK <= 0 {
		topK = 5
	}

	documents, err := r.ragService.RetrieveByQuery(ctx, query, int(topK))
	if err != nil {
		return nil, err
	}

	results := make([]RAGSearchResult, 0, len(documents))
	for _, document := range documents {
		results = append(results, RAGSearchResult{
			DocID:    document.ID,
			Title:    extractRAGDocumentTitle(document.MetaData),
			Content:  document.Content,
			Score:    document.Score,
			Metadata: document.MetaData,
		})
	}
	return &RAGSearchResponse{Query: query, Results: results}, nil
}

// SyncRAGDocumentsToVectorDB 同步指定 RAG 文档到向量库。
func (r *Runtime) SyncRAGDocumentsToVectorDB(ctx context.Context, ids []uint64) error {
	uintIDs := make([]uint, 0, len(ids))
	for _, id := range ids {
		uintIDs = append(uintIDs, uint(id))
	}
	return r.ragDocService.SyncToVectorDB(ctx, uintIDs)
}

// SyncAllPendingRAGDocuments 同步全部待处理 RAG 文档。
func (r *Runtime) SyncAllPendingRAGDocuments(ctx context.Context) error {
	return r.ragDocService.SyncAllPending(ctx)
}

// GetScraperSources 返回真实爬虫源配置。
func (r *Runtime) GetScraperSources(ctx context.Context) ([]ScraperSource, error) {
	sources, err := r.scraperService.GetSources(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]ScraperSource, 0, len(sources))
	for _, source := range sources {
		items = append(items, ScraperSource{
			Name:     source.Name,
			Label:    source.Label,
			BaseURL:  source.BaseURL,
			IsActive: source.IsActive,
		})
	}
	return items, nil
}

// ScraperSearch 执行真实外部搜索。
func (r *Runtime) ScraperSearch(ctx context.Context, req ScraperSearchRequest) ([]ScraperSearchResult, error) {
	results, err := r.scraperService.Search(ctx, scraper.SearchRequest{
		Keyword:  req.Keyword,
		Source:   req.Source,
		Page:     int(req.Page),
		PageSize: int(req.PageSize),
	})
	if err != nil {
		return nil, err
	}

	items := make([]ScraperSearchResult, 0, len(results))
	for _, result := range results {
		items = append(items, ScraperSearchResult{
			Title:   result.Title,
			URL:     result.URL,
			Source:  result.Source,
			Snippet: result.Summary,
		})
	}
	return items, nil
}

// ScraperFetch 抓取真实外部正文。
func (r *Runtime) ScraperFetch(ctx context.Context, req ScraperFetchRequest) (*ScraperFetchResult, error) {
	result, err := r.scraperService.Fetch(ctx, scraper.FetchRequest{
		URL:    req.URL,
		Source: req.Source,
	})
	if err != nil {
		return nil, err
	}
	return &ScraperFetchResult{
		Title:   result.Title,
		Content: result.Content,
		Source:  result.Source,
		URL:     result.URL,
	}, nil
}

// ScraperClean 调用题目清洗链路。
func (r *Runtime) ScraperClean(ctx context.Context, req ScraperCleanRequest) (*ScraperCleanResult, error) {
	result, err := r.scraperService.Clean(ctx, scraper.CleanRequest{
		Content:      req.Content,
		IndustryCode: req.IndustryCode,
		Source:       req.Source,
		SourceURL:    req.SourceURL,
	})
	if err != nil {
		return nil, err
	}

	items := make([]ScraperCleanedQuestion, 0, len(result.Questions))
	for _, question := range result.Questions {
		items = append(items, ScraperCleanedQuestion{
			CategoryName: question.Category,
			Type:         question.Type,
			Difficulty:   question.Difficulty,
			Title:        question.Title,
			Content:      question.Content,
			Answer:       question.Answer,
			Explanation:  question.Explanation,
			Tags:         strings.Join(question.Tags, ","),
		})
	}
	return &ScraperCleanResult{
		Questions:      items,
		TotalExtracted: int32(result.TotalFound),
	}, nil
}

// ScraperImport 导入清洗后的题目。
func (r *Runtime) ScraperImport(ctx context.Context, req ScraperImportRequest) (*ScraperImportResult, error) {
	result, err := r.scraperService.Import(ctx, convertScraperImportRequest(req))
	if err != nil {
		return nil, err
	}
	return &ScraperImportResult{
		TotalCount:   int32(result.TotalCount),
		SuccessCount: int32(result.SuccessCount),
		FailCount:    int32(result.FailCount),
		Errors:       result.Errors,
	}, nil
}

// ScraperImportAsync 创建清洗题目的异步导入任务。
func (r *Runtime) ScraperImportAsync(ctx context.Context, req ScraperImportRequest) (*TaskInfo, error) {
	task, err := r.scraperService.CreateImportTask(ctx, convertScraperImportRequest(req))
	if err != nil {
		return nil, err
	}
	return &TaskInfo{TaskID: uint64(task.ID), Status: task.Status}, nil
}

// build 初始化 bridge 所需的 backend 服务和 handler。
func (r *Runtime) build(db *gorm.DB) error {
	adminConfigRepo := repository.NewAdminConfigRepository(db)
	industryRepo := repository.NewIndustryRepository(db)
	promptRepo := repository.NewPromptTemplateRepository(db)
	aiCallLogRepo := repository.NewAICallLogRepository(db)
	userRepo := repository.NewUserRepository(db)
	membershipRepo := repository.NewMembershipRepository(db)
	interviewRepo := repository.NewInterviewRepository(db)
	interviewMessageRepo := repository.NewInterviewMessageRepository(db)
	interviewCodingRepo := repository.NewInterviewCodingAttemptRepository(db)
	learningArchiveRepo := repository.NewLearningArchiveRepository(db)
	live2DRepo := repository.NewLive2DModelRepository(db)
	adminUserRepo := repository.NewAdminUserRepository(db)
	adminQuestionRepo := repository.NewAdminQuestionRepository(db)
	adminCategoryRepo := repository.NewAdminCategoryRepository(db)
	aiPresetRepo := repository.NewAIPresetRepository(db)
	ttsRepo := repository.NewTTSConfigRepository(db)
	mockInterviewRepo := repository.NewMockInterviewRepository(db)
	scraperTaskRepo := repository.NewScraperTaskRepository(db)
	questionRepo := repository.NewQuestionRepository(db)
	ragDocRepo := repository.NewRAGDocumentRepository(db)
	categoryRepo := repository.NewCategoryRepository(db)
	recordRepo := repository.NewQuestionRecordRepository(db)
	favoriteRepo := repository.NewFavoriteRepository(db)
	noteRepo := repository.NewNoteRepository(db)
	planRepo := repository.NewPlanRepository(db)
	planTaskRepo := repository.NewPlanTaskRepository(db)
	planTaskFeedbackRepo := repository.NewPlanTaskFeedbackRepository(db)
	planTaskDiagnosisRepo := repository.NewPlanTaskDiagnosisRepository(db)
	studyLogRepo := repository.NewStudyLogRepository(db)

	publisher, publisherErr := mq.NewTaskPublisher(r.cfg.RabbitMQ, mq.DefaultQueueSpecs())
	if publisherErr != nil {
		_, _ = fmt.Fprintf(os.Stderr, "bridge: rabbitmq publisher init failed: %v\n", publisherErr)
	}
	asyncOption := service.AsyncDispatchOption{
		Enabled:       r.cfg.RabbitMQ.Enabled && publisher != nil,
		Publisher:     publisher,
		AsyncTaskRepo: repository.NewAsyncTaskRepository(db),
	}

	runtimeBuilder := aiRuntime.NewBuilder(adminConfigRepo, promptRepo, industryRepo, aiCallLogRepo, r.cfg.AIRuntimeDefaults())
	aiClient := aiRuntime.NewRuntimeManager(runtimeBuilder).BuildDynamicClient()
	live2DDirectiveService := service.NewLive2DDirectiveService(live2DRepo, aiClient.Live2DDirector)

	ragConfigs := r.cfg.AIRuntimeDefaults()
	if adminConfigs, err := adminConfigRepo.List(context.Background()); err == nil {
		for _, item := range adminConfigs {
			if strings.HasPrefix(item.ConfigKey, "rag_") {
				ragConfigs[item.ConfigKey] = item.ConfigValue
			}
		}
	}

	if rag.IsRAGEnabled(ragConfigs) {
		ragResult, err := rag.InitFromConfigs(context.Background(), ragConfigs)
		if err != nil {
			r.ragInitErr = fmt.Errorf("initialize rag failed: %w", err)
			_, _ = fmt.Fprintf(os.Stderr, "bridge: %v\n", r.ragInitErr)
		} else {
			r.ragService = ragResult.Service
		}
	}

	scraperProvider := scraper.NewHTTPProvider()
	questionCleaner := scraper.NewHeuristicCleaner()
	membershipService := service.NewMembershipService(membershipRepo, userRepo)
	authService := service.NewAuthService(userRepo, r.cfg)
	questionService := service.NewQuestionService(
		questionRepo,
		categoryRepo,
		recordRepo,
		favoriteRepo,
		noteRepo,
		aiClient.QuizAnalyzer,
		learningArchiveRepo,
		industryRepo,
	)
	service.SetCodeExecutor(questionService, &bridgeCodeExecutor{cfg: r.cfg})

	ttsProvider, err := ttsfactory.NewTTSProviderWithConfig("", r.cfg)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "bridge: tts provider init failed: %v\n", err)
	}
	ttsSceneService := service.NewSceneTTSService(ttsRepo, adminConfigRepo, live2DRepo, ttsProvider)

	asrProvider, err := asrfactory.NewASRProviderWithConfig("", r.cfg)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "bridge: asr provider init failed: %v\n", err)
	}

	interviewService := service.NewInterviewService(
		interviewRepo,
		interviewMessageRepo,
		interviewCodingRepo,
		learningArchiveRepo,
		aiClient.InterviewAgent,
		aiClient.QuizAnalyzer,
		industryRepo,
		live2DDirectiveService,
		service.RealtimeInterviewServiceOption{Enabled: r.cfg.Volcengine.Realtime.Enabled},
		aiClient.ResumeParser,
		asyncOption,
	)

	var interviewRAGService *rag.InterviewRAGService
	if r.ragService != nil {
		interviewRAGService = rag.NewInterviewRAGService(r.ragService)
		aiRuntime.SetPromptEnhancer(aiClient.InterviewAgent, interviewRAGService)
	}

	adminService := service.NewAdminService(
		adminUserRepo,
		adminQuestionRepo,
		industryRepo,
		adminCategoryRepo,
		promptRepo,
		adminConfigRepo,
		aiPresetRepo,
		aiCallLogRepo,
		live2DRepo,
		ttsRepo,
		mockInterviewRepo,
		scraperTaskRepo,
		scraperProvider,
		questionCleaner,
		r.cfg.AIRuntimeDefaults(),
		asyncOption,
	)
	planService := service.NewPlanService(
		planRepo,
		planTaskRepo,
		aiClient.PlanAgent,
		learningArchiveRepo,
		interviewRepo,
		planTaskFeedbackRepo,
		planTaskDiagnosisRepo,
		aiClient.QuizAnalyzer,
		industryRepo,
		asyncOption,
	)
	growthService := service.NewGrowthService(
		studyLogRepo,
		recordRepo,
		interviewRepo,
		planRepo,
		planTaskRepo,
		learningArchiveRepo,
	)
	companionService := service.NewCompanionService(
		aiClient.CompanionAgent,
		live2DDirectiveService,
		ttsSceneService,
		ttsProvider,
		learningArchiveRepo,
		interviewRepo,
		planRepo,
	)
	communityService := service.NewCommunityService(repository.NewCommunityRepository(db), userRepo)

	r.scraperService = service.NewScraperService(
		scraperProvider,
		questionCleaner,
		scraperTaskRepo,
		industryRepo,
		adminCategoryRepo,
		adminQuestionRepo,
		asyncOption,
	)
	r.ragDocService = service.NewRAGDocumentService(ragDocRepo, r.ragService)
	r.adminService = adminService
	r.authHandler = handler.NewAuthHandler(authService)
	r.adminHandler = handler.NewAdminHandler(adminService)
	r.questionHandler = handler.NewQuestionHandler(questionService)
	r.communityHandler = handler.NewCommunityHandler(communityService)
	r.planHandler = handler.NewPlanHandler(planService)
	r.companionHandler = handler.NewCompanionHandler(companionService)
	r.growthHandler = handler.NewGrowthHandler(growthService)
	r.membershipHandler = handler.NewMembershipHandler(membershipService)
	r.live2DHandler = handler.NewLive2DHandler(service.NewLive2DService(live2DRepo, industryRepo))
	r.scraperHandler = handler.NewScraperHandler(r.scraperService)
	r.interviewHandler = handler.NewInterviewHandler(
		interviewService,
		ttsSceneService,
		ttsProvider,
		asrProvider,
		r.cfg.Volcengine.Realtime,
		interviewRAGService,
	)
	if r.ragService != nil {
		r.adminRAGHandler = handler.NewAdminRAGHandler(r.ragService, questionRepo)
	}
	r.ragDocHandler = handler.NewAdminRAGDocumentHandler(r.ragDocService)
	r.questionRepo = questionRepo

	if _, err := middleware.InitCasbin(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "bridge: casbin init failed: %v\n", err)
	}

	assetsDir, err := live2dassets.EnsureAssetsDir()
	if err != nil {
		return fmt.Errorf("prepare live2d assets dir failed: %w", err)
	}
	r.live2DAssetsDir = assetsDir
	return nil
}

// runAIDebug 统一执行 bridge 暴露的 AI 调试逻辑。
func (r *Runtime) runAIDebug(ctx context.Context, req AIDebugRequest) (*AIDebugResponse, error) {
	debugRequest := buildRuntimeDebugRequest(req)
	result, err := r.adminService.DebugAIRuntime(ctx, debugRequest)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(result.ModelError) != "" {
		return nil, fmt.Errorf("ai debug failed: %s", result.ModelError)
	}
	if debugRequest.RunModel && strings.TrimSpace(result.ModelOutput) == "" {
		return nil, fmt.Errorf("ai debug failed: model returned empty output")
	}
	return &AIDebugResponse{
		Response:       result.ModelOutput,
		RenderedPrompt: result.RenderedPrompt,
		Model:          result.Model,
		TokensUsed:     0,
		LatencyMS:      result.LatencyMS,
	}, nil
}

// ensureRAGReady 确保 RAG 服务已正确初始化。
func (r *Runtime) ensureRAGReady() error {
	if r.ragService != nil {
		return nil
	}
	if r.ragInitErr != nil {
		return r.ragInitErr
	}
	return fmt.Errorf("rag service is not enabled or not initialized")
}

// buildRuntimeDebugRequest 将 bridge 调试请求转换为 backend runtime 请求。
func buildRuntimeDebugRequest(req AIDebugRequest) *service.AIDebugRequest {
	variables, runtimeOverrides := splitDebugParams(req.Params)
	runModel := req.RunModel
	if !runModel {
		runModel = true
	}
	return &service.AIDebugRequest{
		Scene:            normalizeAgentScene(req.AgentType),
		TemplateContent:  req.Prompt,
		Variables:        variables,
		RuntimeOverrides: runtimeOverrides,
		RunModel:         runModel,
		UserInput:        firstDebugVariable(variables),
	}
}

// splitDebugParams 拆分模板变量与运行时覆盖配置。
func splitDebugParams(params map[string]string) (map[string]string, map[string]string) {
	variables := make(map[string]string, len(params))
	runtimeOverrides := make(map[string]string)
	for key, value := range params {
		trimmedKey := strings.TrimSpace(key)
		trimmedValue := strings.TrimSpace(value)
		if trimmedKey == "" {
			continue
		}
		variables[trimmedKey] = trimmedValue
		if strings.HasPrefix(trimmedKey, "ai_") || strings.HasPrefix(trimmedKey, "rag_") {
			runtimeOverrides[trimmedKey] = trimmedValue
		}
	}
	return variables, runtimeOverrides
}

// firstDebugVariable 提取最适合映射到 user_input 的变量值。
func firstDebugVariable(variables map[string]string) string {
	for _, key := range []string{"user_input", "content", "question", "requirement"} {
		if value := strings.TrimSpace(variables[key]); value != "" {
			return value
		}
	}
	return ""
}

// bridgeCodeExecutor 为 bridge 侧的题目运行能力提供与单体一致的执行器。
type bridgeCodeExecutor struct {
	cfg *config.Config
}

// Execute 执行不带标准输入的代码片段。
func (e *bridgeCodeExecutor) Execute(ctx context.Context, language, code string) (*service.CodeExecResult, error) {
	client := executor.NewPistonClient(e.cfg.Piston.Endpoint, e.cfg.Piston.Timeout)
	result, err := client.Execute(ctx, language, code)
	if err != nil {
		return nil, err
	}
	return &service.CodeExecResult{Output: result.Output, Passed: result.Passed}, nil
}

// ExecuteWithInput 执行带标准输入的代码片段。
func (e *bridgeCodeExecutor) ExecuteWithInput(ctx context.Context, language, code string, stdin string) (*service.CodeExecResult, error) {
	client := executor.NewPistonClient(e.cfg.Piston.Endpoint, e.cfg.Piston.Timeout)
	result, err := client.ExecuteWithInput(ctx, language, code, stdin)
	if err != nil {
		return nil, err
	}
	return &service.CodeExecResult{Output: result.Output, Passed: result.Passed}, nil
}

// normalizeAgentScene 将外部 agent_type 适配为 backend runtime scene。
func normalizeAgentScene(agentType string) string {
	switch strings.ToLower(strings.TrimSpace(agentType)) {
	case "interview", "interview_agent":
		return "interview"
	case "plan", "plan_agent":
		return "plan"
	case "companion", "companion_agent":
		return "companion"
	case "quiz", "question", "question_pipeline", "quiz_analyzer", "":
		return "quiz"
	default:
		return strings.ToLower(strings.TrimSpace(agentType))
	}
}

// convertPipelineGenerateResponse 转换 backend 题目流水线响应。
func convertPipelineGenerateResponse(response *service.AdminQuestionPipelineGenerateResponse) *QuestionPipelineGenerateResponse {
	if response == nil {
		return &QuestionPipelineGenerateResponse{}
	}

	cards := make([]QuestionPipelineCard, 0, len(response.Cards))
	for _, card := range response.Cards {
		cards = append(cards, QuestionPipelineCard{
			ID:          card.ID,
			Title:       card.Title,
			Content:     card.Content,
			Type:        card.Type,
			Difficulty:  card.Difficulty,
			Category:    card.Category,
			Answer:      card.Answer,
			Solution:    card.Solution,
			Explanation: card.Explanation,
			Tags:        card.Tags,
			JudgeConfig: marshalBridgeValue(card.JudgeConfig),
			Confidence:  card.Confidence,
			SourceType:  card.SourceType,
			SourceLabel: card.SourceLabel,
			SourceTitle: card.SourceTitle,
			SourceURL:   card.SourceURL,
		})
	}

	return &QuestionPipelineGenerateResponse{
		IndustryCode:   response.IndustryCode,
		Requirement:    response.Requirement,
		GenerationMode: response.GenerationMode,
		Warnings:       response.Warnings,
		Stats: map[string]any{
			"searched_count":   response.Stats.SearchedCount,
			"fetched_count":    response.Stats.FetchedCount,
			"scraped_count":    response.Stats.ScrapedCount,
			"generated_count":  response.Stats.GeneratedCount,
			"candidate_count":  response.Stats.CandidateCount,
			"selected_sources": response.Stats.SelectedSources,
		},
		Cards: cards,
	}
}

// convertScraperImportRequest 转换 bridge 导入请求为 backend scraper 请求。
func convertScraperImportRequest(req ScraperImportRequest) scraper.ImportRequest {
	questions := make([]scraper.CleanedQuestion, 0, len(req.Questions))
	for _, question := range req.Questions {
		questions = append(questions, scraper.CleanedQuestion{
			Category:    question.CategoryName,
			Type:        question.Type,
			Difficulty:  question.Difficulty,
			Title:       question.Title,
			Content:     question.Content,
			Answer:      question.Answer,
			Explanation: question.Explanation,
			Tags:        splitTags(question.Tags),
		})
	}
	return scraper.ImportRequest{
		IndustryCode: req.IndustryCode,
		SourceURL:    req.SourceURL,
		SourceTitle:  req.SourceTitle,
		Questions:    questions,
	}
}

// splitTags 将逗号分隔的标签字符串拆分为数组。
func splitTags(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

// extractRAGDocumentTitle 从 metadata 中提取可展示标题。
func extractRAGDocumentTitle(metadata map[string]any) string {
	for _, key := range []string{"title", "question_title", "name"} {
		if value, ok := metadata[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// marshalBridgeValue 将复杂结构编码为 JSON 字符串。
func marshalBridgeValue(value any) string {
	if value == nil {
		return ""
	}
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(data)
}

// resolveBackendConfigPath 解析可用的 backend 配置文件路径。
func resolveBackendConfigPath(explicitPath string) (string, error) {
	candidates := make([]string, 0, 4)
	if path := strings.TrimSpace(explicitPath); path != "" {
		candidates = append(candidates, path)
	}
	if path := strings.TrimSpace(os.Getenv("MAKEJOB_BACKEND_CONFIG")); path != "" {
		candidates = append(candidates, path)
	}
	candidates = append(candidates,
		filepath.Join("backend", "config.yaml"),
		"config.yaml",
		filepath.Join("..", "backend", "config.yaml"),
		filepath.Join("..", "..", "backend", "config.yaml"),
		filepath.Join("..", "..", "..", "backend", "config.yaml"),
	)

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("backend config file not found, tried: %s", strings.Join(candidates, ", "))
}
