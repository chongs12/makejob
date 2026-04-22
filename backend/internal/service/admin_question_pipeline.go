package service

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"makejob-backend/internal/common"
	"makejob-backend/internal/model"
	"makejob-backend/internal/scraper"
)

const (
	defaultQuestionPipelineCount = 8
	maxQuestionPipelineCount     = 20
	maxPipelineMaterialSources   = 6
	questionPipelineModePlanned  = "planned"
	questionPipelineModeDirect   = "direct_single"
)

// AdminQuestionPipelineGenerateRequest 描述后台题目流水线生成请求。
type AdminQuestionPipelineGenerateRequest struct {
	IndustryCode     string   `json:"industry_code" binding:"required"`
	Requirement      string   `json:"requirement" binding:"required"`
	AgentPrompt      string   `json:"agent_prompt"`
	GenerationMode   string   `json:"generation_mode"`
	CandidateCount   int      `json:"candidate_count"`
	IncludeScraped   bool     `json:"include_scraped"`
	IncludeGenerated bool     `json:"include_generated"`
	Sources          []string `json:"sources"`
}

// AdminQuestionPipelineCard 描述前端确认前的候选题卡。
type AdminQuestionPipelineCard struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Content     string   `json:"content"`
	Type        string   `json:"type"`
	Difficulty  string   `json:"difficulty"`
	Category    string   `json:"category"`
	Answer      string   `json:"answer"`
	Explanation string   `json:"explanation"`
	Tags        []string `json:"tags"`
	Confidence  float64  `json:"confidence"`
	SourceType  string   `json:"source_type"`
	SourceLabel string   `json:"source_label"`
	SourceTitle string   `json:"source_title"`
	SourceURL   string   `json:"source_url"`
}

// AdminQuestionPipelineStats 描述题目流水线执行摘要。
type AdminQuestionPipelineStats struct {
	SearchedCount   int `json:"searched_count"`
	FetchedCount    int `json:"fetched_count"`
	ScrapedCount    int `json:"scraped_count"`
	GeneratedCount  int `json:"generated_count"`
	CandidateCount  int `json:"candidate_count"`
	SelectedSources int `json:"selected_sources"`
}

// AdminQuestionPipelineGenerateResponse 描述后台题目流水线候选结果。
type AdminQuestionPipelineGenerateResponse struct {
	IndustryCode   string                      `json:"industry_code"`
	Requirement    string                      `json:"requirement"`
	GenerationMode string                      `json:"generation_mode"`
	Cards          []AdminQuestionPipelineCard `json:"cards"`
	Warnings       []string                    `json:"warnings,omitempty"`
	Stats          AdminQuestionPipelineStats  `json:"stats"`
}

// AdminQuestionPipelineStreamEvent 描述题目流水线 SSE 推送事件的统一结构。
type AdminQuestionPipelineStreamEvent struct {
	Event      string                                 `json:"event"`
	Message    string                                 `json:"message,omitempty"`
	TraceID    string                                 `json:"trace_id,omitempty"`
	RawOutput  string                                 `json:"raw_output,omitempty"`
	SlotIndex  int                                    `json:"slot_index,omitempty"`
	RetryIndex int                                    `json:"retry_index,omitempty"`
	Card       *AdminQuestionPipelineCard             `json:"card,omitempty"`
	Response   *AdminQuestionPipelineGenerateResponse `json:"response,omitempty"`
}

// AdminQuestionPipelineStreamEmitter 描述题目流水线流式推送回调。
type AdminQuestionPipelineStreamEmitter func(event AdminQuestionPipelineStreamEvent) error

// AdminQuestionPipelineImportRequest 描述后台题目流水线导入请求。
type AdminQuestionPipelineImportRequest struct {
	IndustryCode string                            `json:"industry_code" binding:"required"`
	Cards        []AdminQuestionPipelineImportCard `json:"cards" binding:"required,min=1"`
}

// AdminQuestionPipelineImportCard 描述前端确认后回传的题卡数据。
type AdminQuestionPipelineImportCard struct {
	Title       string   `json:"title" binding:"required"`
	Content     string   `json:"content" binding:"required"`
	Type        string   `json:"type" binding:"required"`
	Difficulty  string   `json:"difficulty" binding:"required"`
	Category    string   `json:"category" binding:"required"`
	Answer      string   `json:"answer" binding:"required"`
	Explanation string   `json:"explanation"`
	Tags        []string `json:"tags"`
}

type questionPipelineMaterial struct {
	SourceType string
	Source     string
	Title      string
	URL        string
	Content    string
}

// questionPipelineTopicPlan 描述模型在生成题卡前拆出的单个考点计划。
type questionPipelineTopicPlan struct {
	Topic      string   `json:"topic"`
	Focus      string   `json:"focus"`
	Difficulty string   `json:"difficulty"`
	Category   string   `json:"category"`
	Tags       []string `json:"tags"`
}

// questionPipelinePlanResponse 描述模型拆解出的题卡计划列表。
type questionPipelinePlanResponse struct {
	Topics []questionPipelineTopicPlan `json:"topics"`
}

// questionPipelineModelCard 描述模型直接生成的结构化题卡。
type questionPipelineModelCard struct {
	Title       string   `json:"title"`
	Content     string   `json:"content"`
	Type        string   `json:"type"`
	Difficulty  string   `json:"difficulty"`
	Category    string   `json:"category"`
	Answer      string   `json:"answer"`
	Explanation string   `json:"explanation"`
	Tags        []string `json:"tags"`
}

// questionPipelineCardsResponse 描述模型返回的题卡数组。
type questionPipelineCardsResponse struct {
	Cards []questionPipelineModelCard `json:"cards"`
}

// questionPipelineConstraintProfile 描述从岗位要求与智能体指令中提炼出的硬约束。
type questionPipelineConstraintProfile struct {
	CandidateCount      int
	RequireSubjective   bool
	PreferDistinctTopic bool
	ExcludeProjectCards bool
	GoFeatureOnly       bool
	ExactLanguageCounts map[string]int
	ExactLanguageOrder  []string
	RemainingLanguage   string
}

// GenerateQuestionPipeline 根据岗位要求生成待确认的题卡集合。
func (s *adminService) GenerateQuestionPipeline(ctx context.Context, req *AdminQuestionPipelineGenerateRequest) (*AdminQuestionPipelineGenerateResponse, error) {
	if req == nil {
		return nil, common.NewBusinessError(common.CodeBadRequest, "pipeline request cannot be empty")
	}

	requirement := strings.TrimSpace(req.Requirement)
	if requirement == "" {
		return nil, common.NewBusinessError(common.CodeBadRequest, "requirement is required")
	}

	includeScraped := req.IncludeScraped
	includeGenerated := req.IncludeGenerated
	if !includeScraped && !includeGenerated {
		includeScraped = true
		includeGenerated = true
	}

	industry, categories, err := s.loadPipelineIndustryContext(ctx, req.IndustryCode)
	if err != nil {
		return nil, err
	}

	response := &AdminQuestionPipelineGenerateResponse{
		IndustryCode:   strings.TrimSpace(req.IndustryCode),
		Requirement:    requirement,
		GenerationMode: normalizeQuestionPipelineGenerationMode(req.GenerationMode),
		Warnings:       make([]string, 0),
	}

	candidateLimit := normalizeQuestionPipelineCount(req.CandidateCount)
	constraints := buildQuestionPipelineConstraintProfile(requirement, strings.TrimSpace(req.AgentPrompt), candidateLimit)
	materials := make([]questionPipelineMaterial, 0)
	if includeScraped {
		scrapedMaterials, searchedCount, materialErr := s.collectQuestionPipelineMaterials(ctx, req, candidateLimit)
		response.Stats.SearchedCount = searchedCount
		response.Stats.FetchedCount = len(scrapedMaterials)
		if materialErr != nil {
			response.Warnings = append(response.Warnings, materialErr.Error())
		}
		materials = append(materials, scrapedMaterials...)
	}

	cards := make([]AdminQuestionPipelineCard, 0)
	if len(materials) > 0 {
		for _, material := range materials {
			cleanResult, cleanErr := s.questionCleaner.Clean(ctx, scraper.CleanRequest{
				Content:      material.Content,
				IndustryCode: industry.Code,
				Source:       material.Source,
				SourceURL:    material.URL,
			})
			if cleanErr != nil {
				response.Warnings = append(response.Warnings, fmt.Sprintf("清洗素材失败: %v", cleanErr))
				continue
			}
			cards = append(cards, s.buildQuestionPipelineCards(cleanResult, material, categories, requirement)...)
		}
	}
	cards = filterQuestionPipelineCardsByIntent(cards, requirement, strings.TrimSpace(req.AgentPrompt))
	cards, enforceWarnings := enforceQuestionPipelineCardConstraints(cards, constraints, requirement, strings.TrimSpace(req.AgentPrompt))
	response.Warnings = append(response.Warnings, enforceWarnings...)
	response.Stats.ScrapedCount = len(cards)

	if includeGenerated {
		var (
			generatedCards    []AdminQuestionPipelineCard
			generatedWarnings []string
			generateErr       error
		)
		switch response.GenerationMode {
		case questionPipelineModeDirect:
			generatedCards, generatedWarnings, generateErr = s.generateQuestionPipelineCardsDirectly(
				ctx,
				industry,
				categories,
				requirement,
				strings.TrimSpace(req.AgentPrompt),
				constraints,
				candidateLimit,
				materials,
				nil,
				nil,
			)
		default:
			generatedCards, generatedWarnings, generateErr = s.generateQuestionPipelineCardsWithAgent(
				ctx,
				industry,
				categories,
				requirement,
				strings.TrimSpace(req.AgentPrompt),
				constraints,
				candidateLimit,
				materials,
			)
		}
		response.Warnings = append(response.Warnings, generatedWarnings...)
		if generateErr != nil {
			response.Warnings = append(response.Warnings, generateErr.Error())
		} else {
			response.Stats.GeneratedCount = len(generatedCards)
			cards = append(cards, generatedCards...)
		}
	}

	cards = dedupeQuestionPipelineCards(cards)
	if len(cards) == 0 {
		return nil, common.NewBusinessError(common.CodeBadRequest, buildQuestionPipelineFailureMessage(response.Warnings))
	}
	if len(cards) > candidateLimit {
		cards = cards[:candidateLimit]
	}

	for index := range cards {
		cards[index].ID = fmt.Sprintf("pipeline-card-%d", index+1)
	}

	response.Cards = cards
	response.Stats.CandidateCount = len(cards)
	response.Stats.SelectedSources = len(resolveQuestionPipelineSources(s.scraperProvider, req.Sources))
	return response, nil
}

// GenerateQuestionPipelineStream 以 SSE 事件形式推送题目流水线进度与候选题卡。
func (s *adminService) GenerateQuestionPipelineStream(ctx context.Context, req *AdminQuestionPipelineGenerateRequest, emit AdminQuestionPipelineStreamEmitter) error {
	if emit == nil {
		return common.NewBusinessError(common.CodeBadRequest, "stream emitter is required")
	}
	if req == nil {
		return common.NewBusinessError(common.CodeBadRequest, "pipeline request cannot be empty")
	}

	mode := normalizeQuestionPipelineGenerationMode(req.GenerationMode)
	if mode != questionPipelineModeDirect {
		response, err := s.GenerateQuestionPipeline(ctx, req)
		if err != nil {
			return err
		}

		if err := emit(AdminQuestionPipelineStreamEvent{
			Event:   "status",
			Message: "两阶段规划模式已完成，正在同步候选题卡。",
		}); err != nil {
			return err
		}
		for _, card := range response.Cards {
			cardCopy := card
			if err := emit(AdminQuestionPipelineStreamEvent{
				Event: "card",
				Card:  &cardCopy,
			}); err != nil {
				return err
			}
		}
		return emit(AdminQuestionPipelineStreamEvent{
			Event:    "complete",
			Response: response,
		})
	}

	requirement := strings.TrimSpace(req.Requirement)
	if requirement == "" {
		return common.NewBusinessError(common.CodeBadRequest, "requirement is required")
	}

	includeScraped := req.IncludeScraped
	includeGenerated := req.IncludeGenerated
	if !includeScraped && !includeGenerated {
		includeScraped = true
		includeGenerated = true
	}

	industry, categories, err := s.loadPipelineIndustryContext(ctx, req.IndustryCode)
	if err != nil {
		return err
	}

	response := &AdminQuestionPipelineGenerateResponse{
		IndustryCode:   strings.TrimSpace(req.IndustryCode),
		Requirement:    requirement,
		GenerationMode: mode,
		Warnings:       make([]string, 0),
	}

	if err := emit(AdminQuestionPipelineStreamEvent{
		Event:   "status",
		Message: "已建立流式生成连接，准备加载行业、分类与约束。",
	}); err != nil {
		return err
	}

	candidateLimit := normalizeQuestionPipelineCount(req.CandidateCount)
	constraints := buildQuestionPipelineConstraintProfile(requirement, strings.TrimSpace(req.AgentPrompt), candidateLimit)
	materials := make([]questionPipelineMaterial, 0)
	cards := make([]AdminQuestionPipelineCard, 0)

	if includeScraped {
		if err := emit(AdminQuestionPipelineStreamEvent{
			Event:   "status",
			Message: "正在抓取并清洗参考素材。",
		}); err != nil {
			return err
		}

		scrapedMaterials, searchedCount, materialErr := s.collectQuestionPipelineMaterials(ctx, req, candidateLimit)
		response.Stats.SearchedCount = searchedCount
		response.Stats.FetchedCount = len(scrapedMaterials)
		if materialErr != nil {
			response.Warnings = append(response.Warnings, materialErr.Error())
			if emitErr := emit(AdminQuestionPipelineStreamEvent{
				Event:   "warning",
				Message: materialErr.Error(),
			}); emitErr != nil {
				return emitErr
			}
		}
		materials = append(materials, scrapedMaterials...)
	}

	if len(materials) > 0 {
		for _, material := range materials {
			cleanResult, cleanErr := s.questionCleaner.Clean(ctx, scraper.CleanRequest{
				Content:      material.Content,
				IndustryCode: industry.Code,
				Source:       material.Source,
				SourceURL:    material.URL,
			})
			if cleanErr != nil {
				warning := fmt.Sprintf("清洗素材失败: %v", cleanErr)
				response.Warnings = append(response.Warnings, warning)
				if emitErr := emit(AdminQuestionPipelineStreamEvent{
					Event:   "warning",
					Message: warning,
				}); emitErr != nil {
					return emitErr
				}
				continue
			}
			cards = append(cards, s.buildQuestionPipelineCards(cleanResult, material, categories, requirement)...)
		}
	}
	cards = filterQuestionPipelineCardsByIntent(cards, requirement, strings.TrimSpace(req.AgentPrompt))
	cards, enforceWarnings := enforceQuestionPipelineCardConstraints(cards, constraints, requirement, strings.TrimSpace(req.AgentPrompt))
	response.Warnings = append(response.Warnings, enforceWarnings...)
	response.Stats.ScrapedCount = len(cards)
	for _, warning := range enforceWarnings {
		if err := emit(AdminQuestionPipelineStreamEvent{
			Event:   "warning",
			Message: warning,
		}); err != nil {
			return err
		}
	}

	if includeGenerated {
		if err := emit(AdminQuestionPipelineStreamEvent{
			Event:   "status",
			Message: "正在逐张生成候选题卡，生成结果会实时显示。",
		}); err != nil {
			return err
		}

		generatedCards, generatedWarnings, generateErr := s.generateQuestionPipelineCardsDirectly(
			ctx,
			industry,
			categories,
			requirement,
			strings.TrimSpace(req.AgentPrompt),
			constraints,
			candidateLimit,
			materials,
			nil,
			emit,
		)
		response.Warnings = append(response.Warnings, generatedWarnings...)
		if generateErr != nil {
			response.Warnings = append(response.Warnings, generateErr.Error())
		} else {
			response.Stats.GeneratedCount = len(generatedCards)
			cards = append(cards, generatedCards...)
		}
	}

	cards = dedupeQuestionPipelineCards(cards)
	if len(cards) == 0 {
		return common.NewBusinessError(common.CodeBadRequest, buildQuestionPipelineFailureMessage(response.Warnings))
	}
	if len(cards) > candidateLimit {
		cards = cards[:candidateLimit]
	}
	for index := range cards {
		if strings.TrimSpace(cards[index].ID) == "" {
			cards[index].ID = fmt.Sprintf("pipeline-card-%d", index+1)
		}
	}

	response.Cards = cards
	response.Stats.CandidateCount = len(cards)
	response.Stats.SelectedSources = len(resolveQuestionPipelineSources(s.scraperProvider, req.Sources))
	return emit(AdminQuestionPipelineStreamEvent{
		Event:    "complete",
		Response: response,
	})
}

// ImportQuestionPipeline 将前端确认后的候选题卡批量写入题库。
func (s *adminService) ImportQuestionPipeline(ctx context.Context, req *AdminQuestionPipelineImportRequest) (*BatchImportResponse, error) {
	if req == nil {
		return nil, common.NewBusinessError(common.CodeBadRequest, "pipeline import request cannot be empty")
	}

	questions := make([]ImportQuestionItem, 0, len(req.Cards))
	for _, card := range req.Cards {
		questions = append(questions, ImportQuestionItem{
			CategoryName: strings.TrimSpace(card.Category),
			Type:         normalizeQuestionPipelineType(card.Type),
			Difficulty:   normalizeQuestionPipelineDifficulty(card.Difficulty),
			Title:        strings.TrimSpace(card.Title),
			Content:      strings.TrimSpace(card.Content),
			Answer:       strings.TrimSpace(card.Answer),
			Explanation:  strings.TrimSpace(card.Explanation),
			Tags:         strings.Join(dedupeQuestionPipelineStrings(card.Tags), ","),
		})
	}

	return s.BatchImportQuestions(ctx, &BatchImportRequest{
		IndustryCode: strings.TrimSpace(req.IndustryCode),
		Questions:    questions,
	})
}

// loadPipelineIndustryContext 加载题目流水线所需的行业与分类上下文。
func (s *adminService) loadPipelineIndustryContext(ctx context.Context, industryCode string) (*model.Industry, []model.Category, error) {
	industryCode = strings.TrimSpace(industryCode)
	industry, err := s.industryRepo.GetByCode(ctx, industryCode)
	if err != nil {
		return nil, nil, err
	}
	if industry == nil {
		return nil, nil, common.NewBusinessError(common.CodeNotFound, "industry not found")
	}

	categories, err := s.adminCategoryRepo.List(ctx)
	if err != nil {
		return nil, nil, err
	}

	filtered := make([]model.Category, 0)
	for _, category := range categories {
		if category.IndustryID == industry.ID {
			filtered = append(filtered, category)
		}
	}
	if len(filtered) == 0 {
		return nil, nil, common.NewBusinessError(common.CodeBadRequest, "the selected industry has no categories")
	}

	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].SortOrder == filtered[j].SortOrder {
			return filtered[i].ID < filtered[j].ID
		}
		return filtered[i].SortOrder < filtered[j].SortOrder
	})

	return industry, filtered, nil
}

// collectQuestionPipelineMaterials 采集题目流水线所需的外部面经素材。
func (s *adminService) collectQuestionPipelineMaterials(ctx context.Context, req *AdminQuestionPipelineGenerateRequest, candidateLimit int) ([]questionPipelineMaterial, int, error) {
	if s.scraperProvider == nil {
		return nil, 0, fmt.Errorf("抓取 Provider 未配置")
	}

	sources := resolveQuestionPipelineSources(s.scraperProvider, req.Sources)
	if len(sources) == 0 {
		return nil, 0, fmt.Errorf("没有可用的抓取来源")
	}

	searchPageSize := 2
	if candidateLimit > 10 {
		searchPageSize = 3
	}

	materials := make([]questionPipelineMaterial, 0)
	searchedCount := 0
	for _, source := range sources {
		if len(materials) >= maxPipelineMaterialSources {
			break
		}

		results, err := s.scraperProvider.Search(ctx, scraper.SearchRequest{
			Keyword:  strings.TrimSpace(req.Requirement),
			Source:   source,
			Page:     1,
			PageSize: searchPageSize,
		})
		if err != nil {
			return materials, searchedCount, fmt.Errorf("搜索来源 %s 失败: %w", source, err)
		}
		searchedCount += len(results)

		for _, result := range results {
			if len(materials) >= maxPipelineMaterialSources {
				break
			}

			fetched, fetchErr := s.scraperProvider.Fetch(ctx, scraper.FetchRequest{
				URL:    result.URL,
				Source: source,
			})
			if fetchErr != nil {
				continue
			}
			if strings.TrimSpace(fetched.Content) == "" {
				continue
			}

			materials = append(materials, questionPipelineMaterial{
				SourceType: "scraped",
				Source:     source,
				Title:      fetched.Title,
				URL:        fetched.URL,
				Content:    fetched.Content,
			})
		}
	}

	if len(materials) == 0 {
		return nil, searchedCount, fmt.Errorf("没有抓取到可用面经素材")
	}

	return materials, searchedCount, nil
}

// generateQuestionPipelineCardsWithAgent 通过两阶段智能体流程生成结构化题卡。
func (s *adminService) generateQuestionPipelineCardsWithAgent(
	ctx context.Context,
	industry *model.Industry,
	categories []model.Category,
	requirement string,
	agentPrompt string,
	constraints questionPipelineConstraintProfile,
	candidateLimit int,
	materials []questionPipelineMaterial,
) ([]AdminQuestionPipelineCard, []string, error) {
	plan, planTrace, err := s.generateQuestionPipelinePlan(ctx, industry, categories, requirement, agentPrompt, constraints, candidateLimit, materials)
	if err != nil {
		return nil, nil, err
	}

	cardsPayload, cardTrace, err := s.generateQuestionPipelineStructuredCards(
		ctx,
		industry,
		categories,
		requirement,
		agentPrompt,
		constraints,
		candidateLimit,
		materials,
		plan,
	)
	if err != nil {
		fallbackCards := buildQuestionPipelineCardsFromPlan(plan, categories, requirement, candidateLimit)
		if len(fallbackCards) > 0 {
			fallbackCards, fallbackWarnings := enforceQuestionPipelineCardConstraints(fallbackCards, constraints, requirement, agentPrompt)
			warnings := []string{
				err.Error(),
				"智能体题卡结构化解析失败，已根据规划阶段自动回退生成基础题卡。",
			}
			warnings = append(warnings, fallbackWarnings...)
			return fallbackCards, warnings, nil
		}
		return nil, nil, err
	}

	cards := make([]AdminQuestionPipelineCard, 0, len(cardsPayload.Cards))
	for _, card := range cardsPayload.Cards {
		normalizedCard, ok := buildQuestionPipelineGeneratedCard(card, categories, requirement)
		if !ok {
			continue
		}
		cards = append(cards, normalizedCard)
	}

	cards = dedupeQuestionPipelineCards(cards)
	cards = filterQuestionPipelineCardsByIntent(cards, requirement, agentPrompt)
	cards, enforceWarnings := enforceQuestionPipelineCardConstraints(cards, constraints, requirement, agentPrompt)
	warnings := make([]string, 0)
	warnings = append(warnings, enforceWarnings...)
	if len(cards) < candidateLimit {
		fallbackCards := buildQuestionPipelineCardsFromPlan(plan, categories, requirement, candidateLimit)
		fallbackCards = filterQuestionPipelineCardsByIntent(fallbackCards, requirement, agentPrompt)
		fallbackCards, fallbackWarnings := enforceQuestionPipelineCardConstraints(fallbackCards, constraints, requirement, agentPrompt)
		warnings = append(warnings, fallbackWarnings...)
		cards = mergeQuestionPipelineCards(cards, fallbackCards, candidateLimit)
	}
	if len(cards) < candidateLimit {
		warnings = append(warnings, fmt.Sprintf("智能体本轮只生成了 %d 张满足约束的有效题卡，少于目标 %d 张。", len(cards), candidateLimit))
	}
	if strings.TrimSpace(planTrace) != "" && strings.TrimSpace(cardTrace) != "" && len(cards) == 0 {
		warnings = append(warnings, "模型已返回内容，但未解析出有效结构化题卡。")
	}

	if len(cards) == 0 {
		return nil, warnings, fmt.Errorf("智能体未返回可用题卡，请检查 AI 配置或调整提示词")
	}

	return cards, warnings, nil
}

// generateQuestionPipelineCardsDirectly 通过逐张直生模式生成题卡，降低批量结构化任务的偏航概率。
func (s *adminService) generateQuestionPipelineCardsDirectly(
	ctx context.Context,
	industry *model.Industry,
	categories []model.Category,
	requirement string,
	agentPrompt string,
	constraints questionPipelineConstraintProfile,
	candidateLimit int,
	materials []questionPipelineMaterial,
	onAccepted func(card AdminQuestionPipelineCard, index int) error,
	streamEmit AdminQuestionPipelineStreamEmitter,
) ([]AdminQuestionPipelineCard, []string, error) {
	targetLanguages := buildQuestionPipelineDirectTargetLanguages(constraints, candidateLimit)
	cards := make([]AdminQuestionPipelineCard, 0, candidateLimit)
	warnings := make([]string, 0)

	for slot := 0; slot < candidateLimit; slot++ {
		targetLanguage := ""
		if slot < len(targetLanguages) {
			targetLanguage = targetLanguages[slot]
		}
		if streamEmit != nil {
			message := fmt.Sprintf("正在生成第 %d / %d 张题卡。", slot+1, candidateLimit)
			if strings.TrimSpace(targetLanguage) != "" {
				message = fmt.Sprintf("正在生成第 %d / %d 张题卡，目标语言：%s。", slot+1, candidateLimit, strings.ToUpper(targetLanguage))
			}
			if err := streamEmit(AdminQuestionPipelineStreamEvent{
				Event:   "status",
				Message: message,
			}); err != nil {
				return nil, warnings, err
			}
		}

		slotSucceeded := false
		for retry := 0; retry < 3; retry++ {
			if streamEmit != nil && retry > 0 {
				if err := streamEmit(AdminQuestionPipelineStreamEvent{
					Event:   "status",
					Message: fmt.Sprintf("第 %d 张题卡进入第 %d 次重试。", slot+1, retry+1),
				}); err != nil {
					return nil, warnings, err
				}
			}

			generatedCards, trace, traceID, err := s.generateQuestionPipelineDirectSingleCard(
				ctx,
				industry,
				categories,
				requirement,
				agentPrompt,
				constraints,
				materials,
				cards,
				slot,
				targetLanguage,
			)
			if err != nil {
				warning := fmt.Sprintf("第 %d 张题卡生成失败: %v", slot+1, err)
				warnings = append(warnings, warning)
				if streamEmit != nil {
					if emitErr := streamEmit(AdminQuestionPipelineStreamEvent{
						Event:      "warning",
						Message:    warning,
						TraceID:    strings.TrimSpace(traceID),
						RawOutput:  strings.TrimSpace(trace),
						SlotIndex:  slot + 1,
						RetryIndex: retry + 1,
					}); emitErr != nil {
						return nil, warnings, emitErr
					}
				}
				continue
			}

			nextCards := append([]AdminQuestionPipelineCard{}, cards...)
			nextCards = append(nextCards, generatedCards...)
			nextCards = dedupeQuestionPipelineCards(nextCards)
			nextCards = filterQuestionPipelineCardsByIntent(nextCards, requirement, agentPrompt)
			constrainedCards, constraintWarnings := enforceQuestionPipelineCardConstraints(nextCards, constraints, requirement, agentPrompt)
			warnings = append(warnings, constraintWarnings...)
			if streamEmit != nil {
				for _, warning := range constraintWarnings {
					if emitErr := streamEmit(AdminQuestionPipelineStreamEvent{
						Event:   "warning",
						Message: warning,
					}); emitErr != nil {
						return nil, warnings, emitErr
					}
				}
			}
			if len(constrainedCards) <= len(cards) {
				if strings.TrimSpace(trace) != "" {
					warning := fmt.Sprintf("第 %d 张题卡未通过约束校验，已丢弃。", slot+1)
					warnings = append(warnings, warning)
					if streamEmit != nil {
						if emitErr := streamEmit(AdminQuestionPipelineStreamEvent{
							Event:   "warning",
							Message: warning,
						}); emitErr != nil {
							return nil, warnings, emitErr
						}
					}
				}
				continue
			}

			cards = constrainedCards
			if len(cards) > 0 {
				acceptedCard := cards[len(cards)-1]
				acceptedCard.ID = fmt.Sprintf("pipeline-card-%d", len(cards))
				cards[len(cards)-1].ID = acceptedCard.ID
				if streamEmit != nil {
					cardCopy := acceptedCard
					if err := streamEmit(AdminQuestionPipelineStreamEvent{
						Event:   "card",
						Message: fmt.Sprintf("第 %d 张题卡已生成。", len(cards)),
						Card:    &cardCopy,
					}); err != nil {
						return nil, warnings, err
					}
				}
				if streamEmit != nil {
					if err := streamEmit(AdminQuestionPipelineStreamEvent{
						Event:   "status",
						Message: fmt.Sprintf("已生成 %d / %d 张候选题卡。", len(cards), candidateLimit),
					}); err != nil {
						return nil, warnings, err
					}
				}
			}
			if onAccepted != nil && len(cards) > 0 {
				acceptedCard := cards[len(cards)-1]
				if err := onAccepted(acceptedCard, len(cards)-1); err != nil {
					return nil, warnings, err
				}
			}
			slotSucceeded = true
			break
		}

		if !slotSucceeded {
			warning := fmt.Sprintf("第 %d 张题卡在逐张直生模式下未能生成满足要求的结果。", slot+1)
			warnings = append(warnings, warning)
			if streamEmit != nil {
				if err := streamEmit(AdminQuestionPipelineStreamEvent{
					Event:   "warning",
					Message: warning,
				}); err != nil {
					return nil, warnings, err
				}
			}
		}
	}

	warnings = dedupeQuestionPipelineStrings(warnings)
	if len(cards) == 0 {
		if streamEmit != nil {
			if err := streamEmit(AdminQuestionPipelineStreamEvent{
				Event:   "warning",
				Message: "逐张直生模式未返回可用题卡，请检查 AI 配置或调整提示词。",
			}); err != nil {
				return nil, warnings, err
			}
		}
		return nil, warnings, fmt.Errorf("逐张直生模式未返回可用题卡，请检查 AI 配置或调整提示词")
	}
	if len(cards) < candidateLimit {
		warning := fmt.Sprintf("逐张直生模式只生成了 %d 张满足约束的有效题卡，少于目标 %d 张。", len(cards), candidateLimit)
		warnings = append(warnings, warning)
		if streamEmit != nil {
			if err := streamEmit(AdminQuestionPipelineStreamEvent{
				Event:   "warning",
				Message: warning,
			}); err != nil {
				return nil, warnings, err
			}
		}
	}
	return cards, dedupeQuestionPipelineStrings(warnings), nil
}

// generateQuestionPipelineDirectSingleCard 逐轮请求模型只生成一张题卡，并携带已有题卡摘要减少重复。
func (s *adminService) generateQuestionPipelineDirectSingleCard(
	ctx context.Context,
	industry *model.Industry,
	categories []model.Category,
	requirement string,
	agentPrompt string,
	constraints questionPipelineConstraintProfile,
	materials []questionPipelineMaterial,
	existingCards []AdminQuestionPipelineCard,
	slot int,
	targetLanguage string,
) ([]AdminQuestionPipelineCard, string, string, error) {
	categoryHints := make([]string, 0, len(categories))
	for _, category := range categories {
		categoryHints = append(categoryHints, category.Name)
	}

	result, err := s.DebugAIRuntime(ctx, &AIDebugRequest{
		Scene: model.PromptSceneInterview,
		TemplateContent: strings.TrimSpace(`
你是 MakeJob 的逐张题卡生成智能体。当前只允许生成 1 张中文面试题卡，请不要一次输出多张。

严格要求：
1. 只返回一个 JSON 对象，结构必须是 {"cards":[{...}]}。
2. cards 数组里只能有 1 张题卡，不允许额外解释、Markdown、代码块或前后缀文本。
3. 本轮题卡必须与已生成题卡考点明显不同，禁止同义改写。
4. 如果指定了目标语言，必须严格按目标语言出题；不要输出项目经历、职业规划、微服务治理等凑数题。
5. title、content、answer、explanation 都必须填写完整。

行业：
{{industry_name}}

用户目标：
{{requirement}}

智能体指令：
{{agent_prompt}}

硬约束摘要：
{{constraint_summary}}

本轮目标语言：
{{target_language}}

当前是第 {{slot_index}} / {{candidate_count}} 张题卡。

已生成题卡摘要：
{{existing_cards}}

现有分类：
{{category_hints}}

参考素材摘要：
{{material_excerpt}}
`),
		Variables: map[string]string{
			"industry_name":      industry.Name,
			"requirement":        requirement,
			"agent_prompt":       buildQuestionPipelineAgentPrompt(agentPrompt),
			"constraint_summary": buildQuestionPipelineConstraintSummary(constraints),
			"target_language":    buildQuestionPipelineDirectTargetLanguageLabel(targetLanguage),
			"slot_index":         fmt.Sprintf("%d", slot+1),
			"candidate_count":    fmt.Sprintf("%d", constraints.CandidateCount),
			"existing_cards":     buildQuestionPipelineExistingCardsExcerpt(existingCards),
			"category_hints":     strings.Join(categoryHints, "、"),
			"material_excerpt":   buildQuestionPipelineMaterialExcerpt(materials),
		},
		RunModel:  true,
		UserInput: "请严格返回只包含 1 张题卡的 JSON 对象。",
	})
	if err != nil {
		return nil, "", "", fmt.Errorf("逐张直生调用失败: %w", err)
	}
	traceOutput := buildQuestionPipelineDebugTrace(result)
	if isQuestionPipelineMockOutput(traceOutput) {
		return nil, traceOutput, result.TraceID, fmt.Errorf("当前调用实际返回了 Mock Provider 输出，请检查 AI 配置中的主 provider、fallback provider 与 API Key")
	}
	if strings.TrimSpace(result.ModelError) != "" {
		return nil, traceOutput, result.TraceID, fmt.Errorf("逐张直生模型调用失败: %s", strings.TrimSpace(result.ModelError))
	}

	payload, decodeErr := decodeQuestionPipelineCardsResponse(result.ModelOutput)
	if decodeErr != nil {
		if strings.EqualFold(strings.TrimSpace(result.Provider), "mock") {
			return nil, traceOutput, result.TraceID, fmt.Errorf("当前 AI provider 仍为 mock，无法生成真实题卡，请先在后台 AI 配置页切换到可用模型")
		}
		repairedPayload, repairedTrace, repairErr := s.repairQuestionPipelineCardResponse(ctx, result.ModelOutput)
		if repairErr != nil {
			return nil, traceOutput, result.TraceID, fmt.Errorf("逐张直生返回内容无法解析: %w", decodeErr)
		}
		payload = repairedPayload
		traceOutput = firstNonEmptyString(strings.TrimSpace(repairedTrace), traceOutput)
	}
	if payload == nil || len(payload.Cards) == 0 {
		return nil, traceOutput, result.TraceID, fmt.Errorf("逐张直生未返回有效题卡")
	}

	normalizedCards := make([]AdminQuestionPipelineCard, 0, 1)
	for _, card := range payload.Cards[:1] {
		normalized, ok := buildQuestionPipelineGeneratedCard(card, categories, requirement)
		if !ok {
			continue
		}
		normalizedCards = append(normalizedCards, normalized)
	}
	if len(normalizedCards) == 0 {
		return nil, traceOutput, result.TraceID, fmt.Errorf("逐张直生返回了内容，但未解析出可用题卡")
	}

	return normalizedCards, traceOutput, result.TraceID, nil
}

// repairQuestionPipelineCardResponse 在单卡输出解析失败时，追加一次轻量修复请求，尽量把原始内容整理成标准题卡 JSON。
func (s *adminService) repairQuestionPipelineCardResponse(ctx context.Context, raw string) (*questionPipelineCardsResponse, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, "", fmt.Errorf("raw model output is empty")
	}

	result, err := s.DebugAIRuntime(ctx, &AIDebugRequest{
		Scene: model.PromptSceneInterview,
		TemplateContent: strings.TrimSpace(`
你是 MakeJob 的题卡 JSON 修复助手。
你的任务不是重新出题，而是把已有内容整理成唯一合法的 JSON 对象。

严格要求：
1. 只能输出一个 JSON 对象，结构必须是 {"cards":[{...}]}。
2. cards 数组最多只保留 1 张题卡。
3. 不要输出解释、Markdown、代码块、前后缀、注释。
4. 只保留原内容里可以稳定提取的信息，不要凭空扩写。
5. 如果原内容里没有可靠答案，也必须尽量从原文提炼参考答案字段。
`),
		RunModel: true,
		UserInput: fmt.Sprintf(
			"目标 JSON 结构：\n%s\n\n待修复内容：\n%s",
			buildQuestionPipelineSingleCardRepairSchema(),
			raw,
		),
	})
	if err != nil {
		return nil, "", err
	}
	traceOutput := buildQuestionPipelineDebugTrace(result)
	if isQuestionPipelineMockOutput(traceOutput) {
		return nil, traceOutput, fmt.Errorf("当前修复链路实际返回了 Mock Provider 输出，请检查 AI 配置中的主 provider、fallback provider 与 API Key")
	}
	if strings.TrimSpace(result.ModelError) != "" {
		return nil, traceOutput, fmt.Errorf("题卡修复调用失败: %s", strings.TrimSpace(result.ModelError))
	}

	payload, decodeErr := decodeQuestionPipelineCardsResponse(result.ModelOutput)
	if decodeErr != nil {
		return nil, traceOutput, decodeErr
	}
	if payload == nil || len(payload.Cards) == 0 {
		return nil, traceOutput, fmt.Errorf("repair cards not found")
	}

	return payload, firstNonEmptyString(strings.TrimSpace(result.ModelOutput), traceOutput), nil
}

// generateQuestionPipelinePlan 先让模型拆出互不重复的考点计划。
func (s *adminService) generateQuestionPipelinePlan(
	ctx context.Context,
	industry *model.Industry,
	categories []model.Category,
	requirement string,
	agentPrompt string,
	constraints questionPipelineConstraintProfile,
	candidateLimit int,
	materials []questionPipelineMaterial,
) (*questionPipelinePlanResponse, string, error) {
	categoryHints := make([]string, 0, len(categories))
	for _, category := range categories {
		categoryHints = append(categoryHints, category.Name)
	}

	result, err := s.DebugAIRuntime(ctx, &AIDebugRequest{
		Scene: model.PromptSceneInterview,
		TemplateContent: strings.TrimSpace(`
你是 MakeJob 的题库智能体规划器。你的任务不是直接出题，而是先把用户要求拆成互不重复的考察主题计划。

要求：
1. 必须输出 {{candidate_count}} 个不同 topic，禁止重复或同义改写重复。
2. 每个 topic 必须明确一个聚焦点 focus，说明它要考察目标语言或目标技术中的哪个核心理解点。
3. category 必须优先从现有分类里选择最贴近的一项。
4. difficulty 只能是 easy、medium、hard 之一。
5. 只返回一个 JSON 对象，不要返回解释、Markdown、代码块或额外文本。
6. 如果硬约束与泛化描述冲突，始终以硬约束摘要为准；宁可减少跑偏内容，也不要用项目题或职业规划题凑数。

用户目标：
{{requirement}}

智能体指令：
{{agent_prompt}}

硬约束摘要：
{{constraint_summary}}

现有分类：
{{category_hints}}

参考素材摘要：
{{material_excerpt}}
`),
		Variables: map[string]string{
			"candidate_count":    fmt.Sprintf("%d", candidateLimit),
			"requirement":        requirement,
			"agent_prompt":       buildQuestionPipelineAgentPrompt(agentPrompt),
			"constraint_summary": buildQuestionPipelineConstraintSummary(constraints),
			"category_hints":     strings.Join(categoryHints, "、"),
			"material_excerpt":   buildQuestionPipelineMaterialExcerpt(materials),
		},
		RunModel:  true,
		UserInput: "请严格返回 JSON 计划对象，字段必须符合约定。",
	})
	if err != nil {
		return nil, "", fmt.Errorf("智能体规划阶段失败: %w", err)
	}
	traceOutput := buildQuestionPipelineDebugTrace(result)
	if strings.TrimSpace(result.ModelError) != "" {
		return nil, traceOutput, fmt.Errorf("智能体规划阶段模型调用失败: %s", strings.TrimSpace(result.ModelError))
	}

	payload, decodeErr := decodeQuestionPipelinePlanResponse(result.ModelOutput)
	if decodeErr != nil {
		if strings.EqualFold(strings.TrimSpace(result.Provider), "mock") {
			return nil, traceOutput, fmt.Errorf("当前 AI provider 仍为 mock，无法生成真实题卡，请先在后台 AI 配置页切换到可用模型")
		}
		return nil, traceOutput, fmt.Errorf("智能体规划阶段返回内容无法解析: %w", decodeErr)
	}

	payload.Topics = dedupeQuestionPipelinePlan(payload.Topics)
	if len(payload.Topics) == 0 {
		return nil, traceOutput, fmt.Errorf("智能体规划阶段未返回有效考点")
	}
	if len(payload.Topics) > candidateLimit {
		payload.Topics = payload.Topics[:candidateLimit]
	}

	return payload, traceOutput, nil
}

// generateQuestionPipelineStructuredCards 根据拆解出的计划直接生成结构化题卡。
func (s *adminService) generateQuestionPipelineStructuredCards(
	ctx context.Context,
	industry *model.Industry,
	categories []model.Category,
	requirement string,
	agentPrompt string,
	constraints questionPipelineConstraintProfile,
	candidateLimit int,
	materials []questionPipelineMaterial,
	plan *questionPipelinePlanResponse,
) (*questionPipelineCardsResponse, string, error) {
	if plan == nil || len(plan.Topics) == 0 {
		return nil, "", fmt.Errorf("智能体计划为空")
	}

	categoryHints := make([]string, 0, len(categories))
	for _, category := range categories {
		categoryHints = append(categoryHints, category.Name)
	}

	planJSON, _ := json.Marshal(plan)
	result, err := s.DebugAIRuntime(ctx, &AIDebugRequest{
		Scene: model.PromptSceneInterview,
		TemplateContent: strings.TrimSpace(`
你是 MakeJob 的题卡生成智能体。请根据已经拆好的考点计划，生成高质量、互不重复的中文面试题卡。

严格要求：
1. 必须返回与 topics 等长的 cards 数组，不允许少题、不允许合并题目。
2. 每张卡都要和对应 topic 一一映射，不能偷懒复用同一题干。
3. title 要短、准、可读；content 要写完整题目；answer 要给出清晰标准答案；explanation 要说明考察意图。
4. 如果用户要求是问答题，type 请优先用 subjective。
5. category 必须尽量落入现有分类。
6. 只返回一个 JSON 对象，不要返回解释、Markdown、代码块或额外文本。
7. 如果硬约束与常规后端面试套路冲突，优先满足硬约束；不要输出项目经历、职业规划、微服务治理等凑数题。

行业：
{{industry_name}}

用户目标：
{{requirement}}

智能体指令：
{{agent_prompt}}

硬约束摘要：
{{constraint_summary}}

现有分类：
{{category_hints}}

考点计划：
{{plan_json}}

参考素材摘要：
{{material_excerpt}}
`),
		Variables: map[string]string{
			"industry_name":      industry.Name,
			"requirement":        requirement,
			"agent_prompt":       buildQuestionPipelineAgentPrompt(agentPrompt),
			"constraint_summary": buildQuestionPipelineConstraintSummary(constraints),
			"category_hints":     strings.Join(categoryHints, "、"),
			"plan_json":          string(planJSON),
			"material_excerpt":   buildQuestionPipelineMaterialExcerpt(materials),
			"candidate_count":    fmt.Sprintf("%d", candidateLimit),
		},
		RunModel:  true,
		UserInput: "请严格返回 JSON 题卡对象，字段必须符合约定。",
	})
	if err != nil {
		return nil, "", fmt.Errorf("智能体题卡生成阶段失败: %w", err)
	}
	traceOutput := buildQuestionPipelineDebugTrace(result)
	if strings.TrimSpace(result.ModelError) != "" {
		return nil, traceOutput, fmt.Errorf("智能体题卡生成阶段模型调用失败: %s", strings.TrimSpace(result.ModelError))
	}

	payload, decodeErr := decodeQuestionPipelineCardsResponse(result.ModelOutput)
	if decodeErr != nil {
		if strings.EqualFold(strings.TrimSpace(result.Provider), "mock") {
			return nil, traceOutput, fmt.Errorf("当前 AI provider 仍为 mock，无法生成真实题卡，请先在后台 AI 配置页切换到可用模型")
		}
		return nil, traceOutput, fmt.Errorf("智能体题卡生成阶段返回内容无法解析: %w", decodeErr)
	}

	if len(payload.Cards) == 0 {
		return nil, traceOutput, fmt.Errorf("智能体题卡生成阶段未返回有效题卡")
	}

	return payload, traceOutput, nil
}

// buildQuestionPipelineCards 将清洗结果转换为前端可编辑的候选题卡。
func (s *adminService) buildQuestionPipelineCards(
	result *scraper.CleanResult,
	material questionPipelineMaterial,
	categories []model.Category,
	requirement string,
) []AdminQuestionPipelineCard {
	if result == nil || len(result.Questions) == 0 {
		return []AdminQuestionPipelineCard{}
	}

	cards := make([]AdminQuestionPipelineCard, 0, len(result.Questions))
	for _, question := range result.Questions {
		title := strings.TrimSpace(question.Title)
		if title == "" {
			continue
		}

		content := strings.TrimSpace(question.Content)
		if content == "" || content == title {
			content = fmt.Sprintf("请围绕“%s”回答该题，并结合岗位要求“%s”说明关键技术点与适用边界。", title, requirement)
		}

		answer := strings.TrimSpace(question.Answer)
		if answer == "" {
			answer = fmt.Sprintf("回答时建议聚焦题目本身，说明相关原理、核心机制、常见误区，以及它与“%s”的关联。", title)
		}

		explanation := strings.TrimSpace(question.Explanation)
		if explanation == "" {
			explanation = buildQuestionPipelineExplanation(title)
		}

		sourceLabel := material.Source
		if material.SourceType == "generated" {
			sourceLabel = "AI 生成"
		}

		cards = append(cards, AdminQuestionPipelineCard{
			Title:       title,
			Content:     content,
			Type:        normalizeQuestionPipelineType(question.Type),
			Difficulty:  normalizeQuestionPipelineDifficulty(question.Difficulty),
			Category:    matchQuestionPipelineCategory(categories, question.Category, requirement),
			Answer:      answer,
			Explanation: explanation,
			Tags:        buildQuestionPipelineTags(question.Tags, requirement),
			Confidence:  question.Confidence,
			SourceType:  material.SourceType,
			SourceLabel: sourceLabel,
			SourceTitle: strings.TrimSpace(material.Title),
			SourceURL:   strings.TrimSpace(material.URL),
		})
	}

	return cards
}

// normalizeQuestionPipelineCount 规范化候选题卡数量。
func normalizeQuestionPipelineCount(count int) int {
	switch {
	case count <= 0:
		return defaultQuestionPipelineCount
	case count > maxQuestionPipelineCount:
		return maxQuestionPipelineCount
	default:
		return count
	}
}

// normalizeQuestionPipelineGenerationMode 规范化题目流水线生成模式，避免前端回传非法值。
func normalizeQuestionPipelineGenerationMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case questionPipelineModeDirect:
		return questionPipelineModeDirect
	default:
		return questionPipelineModePlanned
	}
}

// buildQuestionPipelineGeneratedCard 将模型返回的结构化题卡归一化为后台统一候选题卡。
func buildQuestionPipelineGeneratedCard(card questionPipelineModelCard, categories []model.Category, requirement string) (AdminQuestionPipelineCard, bool) {
	title := strings.TrimSpace(card.Title)
	content := strings.TrimSpace(card.Content)
	answer := strings.TrimSpace(card.Answer)
	if title == "" || content == "" || answer == "" {
		return AdminQuestionPipelineCard{}, false
	}

	return AdminQuestionPipelineCard{
		Title:       title,
		Content:     content,
		Type:        normalizeQuestionPipelineType(card.Type),
		Difficulty:  normalizeQuestionPipelineDifficulty(card.Difficulty),
		Category:    matchQuestionPipelineCategory(categories, card.Category, requirement),
		Answer:      answer,
		Explanation: firstNonEmptyString(strings.TrimSpace(card.Explanation), buildQuestionPipelineExplanation(title)),
		Tags:        buildQuestionPipelineTags(card.Tags, requirement),
		Confidence:  0.94,
		SourceType:  "generated",
		SourceLabel: "AI 智能体生成",
		SourceTitle: "智能体候选题卡",
		SourceURL:   "",
	}, true
}

// resolveQuestionPipelineSources 解析题目流水线允许使用的抓取来源。
func resolveQuestionPipelineSources(provider scraper.ScraperProvider, selected []string) []string {
	if provider == nil {
		return []string{}
	}

	activeSources := provider.GetSupportedSources()
	supported := make(map[string]bool, len(activeSources))
	ordered := make([]string, 0, len(activeSources))
	for _, source := range activeSources {
		if !source.IsActive {
			continue
		}
		supported[source.Name] = true
		ordered = append(ordered, source.Name)
	}

	if len(selected) == 0 {
		return ordered
	}

	filtered := make([]string, 0, len(selected))
	for _, source := range selected {
		source = strings.TrimSpace(source)
		if source == "" || !supported[source] {
			continue
		}
		filtered = append(filtered, source)
	}

	return dedupeQuestionPipelineStrings(filtered)
}

// buildQuestionPipelineMaterialExcerpt 压缩参考素材，避免 prompt 过长。
func buildQuestionPipelineMaterialExcerpt(materials []questionPipelineMaterial) string {
	if len(materials) == 0 {
		return "暂无参考素材，本轮主要根据用户要求直接生成。"
	}

	parts := make([]string, 0, len(materials))
	for index, material := range materials {
		if index >= 3 {
			break
		}

		content := strings.TrimSpace(material.Content)
		if len([]rune(content)) > 260 {
			content = string([]rune(content)[:260])
		}
		parts = append(parts, fmt.Sprintf("素材%d（%s）：%s", index+1, material.Title, content))
	}

	return strings.Join(parts, "\n")
}

// buildQuestionPipelineAgentPrompt 规范化前端传入的智能体指令。
func buildQuestionPipelineAgentPrompt(agentPrompt string) string {
	baseRules := "必须严格执行用户在智能体指令中的显式要求。凡是“必须包含”“必须排除”“至少/恰好几题”“指定语言或主题”的约束，都视为硬约束，不得忽略。若用户要求聚焦语言特性、语言机制或底层原理，默认禁止输出项目经历、职业规划、微服务治理、行为面试等泛项目题，除非智能体指令明确要求保留。"
	if strings.TrimSpace(agentPrompt) == "" {
		return "优先保证题卡之间互不重复，覆盖不同考点，避免模板化表述。\n" + baseRules
	}
	return strings.TrimSpace(agentPrompt) + "\n" + baseRules
}

// buildQuestionPipelineConstraintProfile 从需求与智能体指令中提炼题卡硬约束。
func buildQuestionPipelineConstraintProfile(requirement string, agentPrompt string, candidateLimit int) questionPipelineConstraintProfile {
	profile := questionPipelineConstraintProfile{
		CandidateCount:      candidateLimit,
		RequireSubjective:   strings.Contains(strings.ToLower(requirement+"\n"+agentPrompt), "问答题") || strings.Contains(strings.ToLower(requirement+"\n"+agentPrompt), "主观题"),
		PreferDistinctTopic: strings.Contains(strings.ToLower(requirement+"\n"+agentPrompt), "不同考点") || strings.Contains(strings.ToLower(requirement+"\n"+agentPrompt), "互不重复"),
		ExcludeProjectCards: shouldFilterQuestionPipelineProjectCards(requirement, agentPrompt),
		GoFeatureOnly:       shouldFilterQuestionPipelineGoFeatureOnly(requirement, agentPrompt),
		ExactLanguageCounts: make(map[string]int),
		ExactLanguageOrder:  make([]string, 0),
	}

	counts, order := extractQuestionPipelineLanguageCounts(agentPrompt)
	for _, language := range order {
		profile.ExactLanguageCounts[language] = counts[language]
		profile.ExactLanguageOrder = append(profile.ExactLanguageOrder, language)
	}
	profile.RemainingLanguage = extractQuestionPipelineRemainingLanguage(agentPrompt)
	return profile
}

// buildQuestionPipelineConstraintSummary 构造传给模型的硬约束摘要，减少自由发挥空间。
func buildQuestionPipelineConstraintSummary(profile questionPipelineConstraintProfile) string {
	parts := []string{
		fmt.Sprintf("必须返回 %d 张候选题卡。", profile.CandidateCount),
	}
	if profile.RequireSubjective {
		parts = append(parts, "题型优先使用问答题（subjective），不要退化成选择题。")
	}
	if profile.PreferDistinctTopic {
		parts = append(parts, "每张题卡必须覆盖不同考点，禁止同义改写凑数。")
	}
	if profile.ExcludeProjectCards {
		parts = append(parts, "禁止输出项目经历、职业规划、微服务治理、行为面试等泛项目题。")
	}
	if profile.GoFeatureOnly {
		parts = append(parts, "题目主体必须聚焦语言特性、语言机制、底层原理或标准库核心语义。")
	}
	for _, language := range profile.ExactLanguageOrder {
		parts = append(parts, fmt.Sprintf("必须包含 %d 张 %s 题卡。", profile.ExactLanguageCounts[language], strings.ToUpper(language)))
	}
	if profile.RemainingLanguage != "" {
		parts = append(parts, fmt.Sprintf("除上述显式配额外，其余题卡全部使用 %s 主题。", strings.ToUpper(profile.RemainingLanguage)))
	}
	parts = append(parts, "如果无法满足约束，宁可少输出，也不要补无关题凑数。")
	return strings.Join(parts, "\n")
}

// extractQuestionPipelineLanguageCounts 解析“必须有一个题是 Java”之类的显式语言配额。
func extractQuestionPipelineLanguageCounts(agentPrompt string) (map[string]int, []string) {
	counts := make(map[string]int)
	order := make([]string, 0)
	re := regexp.MustCompile(`(?i)(?:必须|至少|恰好|其中必须有|其中有|至少有|确保有)?\s*([0-9一二两三四五六七八九十]+)\s*(?:个|道|张)?(?:题|题卡)[^。\n]{0,12}?(?:是|为)?\s*(java|go|golang)`)
	matches := re.FindAllStringSubmatch(agentPrompt, -1)
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		count := parseQuestionPipelineCountToken(match[1])
		language := normalizeQuestionPipelineLanguage(match[2])
		if count <= 0 || language == "" {
			continue
		}
		if _, exists := counts[language]; !exists {
			order = append(order, language)
		}
		counts[language] += count
	}
	return counts, order
}

// extractQuestionPipelineRemainingLanguage 解析“其他是 Go”之类的剩余题卡语言约束。
func extractQuestionPipelineRemainingLanguage(agentPrompt string) string {
	re := regexp.MustCompile(`(?i)(?:其他|其余|剩下|剩余)[^。\n]{0,10}?(?:是|为|都用|都为|都必须是)?\s*(java|go|golang)`)
	match := re.FindStringSubmatch(agentPrompt)
	if len(match) < 2 {
		return ""
	}
	return normalizeQuestionPipelineLanguage(match[1])
}

// parseQuestionPipelineCountToken 将阿拉伯数字或常见中文数字转换为数量。
func parseQuestionPipelineCountToken(token string) int {
	switch strings.TrimSpace(strings.ToLower(token)) {
	case "1", "一", "一个":
		return 1
	case "2", "二", "两", "两个":
		return 2
	case "3", "三", "三个":
		return 3
	case "4", "四", "四个":
		return 4
	case "5", "五", "五个":
		return 5
	case "6", "六", "六个":
		return 6
	case "7", "七", "七个":
		return 7
	case "8", "八", "八个":
		return 8
	case "9", "九", "九个":
		return 9
	case "10", "十", "十个":
		return 10
	default:
		return 0
	}
}

// normalizeQuestionPipelineLanguage 规范化题卡语言标识。
func normalizeQuestionPipelineLanguage(language string) string {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "go", "golang":
		return "go"
	case "java":
		return "java"
	default:
		return ""
	}
}

// buildQuestionPipelineDirectTargetLanguages 按语言配额构造逐张直生模式的目标语言序列。
func buildQuestionPipelineDirectTargetLanguages(profile questionPipelineConstraintProfile, candidateLimit int) []string {
	targets := make([]string, 0, candidateLimit)
	for _, language := range profile.ExactLanguageOrder {
		for count := 0; count < profile.ExactLanguageCounts[language] && len(targets) < candidateLimit; count++ {
			targets = append(targets, language)
		}
	}
	for profile.RemainingLanguage != "" && len(targets) < candidateLimit {
		targets = append(targets, profile.RemainingLanguage)
	}
	for len(targets) < candidateLimit {
		targets = append(targets, "")
	}
	return targets
}

// buildQuestionPipelineDirectTargetLanguageLabel 为逐张直生模式生成可读目标语言说明。
func buildQuestionPipelineDirectTargetLanguageLabel(language string) string {
	switch strings.TrimSpace(language) {
	case "go":
		return "本轮必须生成 Go 语言特性或 Go 核心机制相关题卡。"
	case "java":
		return "本轮必须生成 Java 相关题卡，用于满足显式语言配额。"
	default:
		return "未指定单独语言配额，本轮遵循整体需求与智能体指令。"
	}
}

// buildQuestionPipelineSingleCardRepairSchema 返回单张题卡修复阶段使用的目标 JSON 结构说明。
func buildQuestionPipelineSingleCardRepairSchema() string {
	return `{"cards":[{"title":"题目标题","content":"完整题干","type":"subjective","difficulty":"easy|medium|hard","category":"分类名称","answer":"参考答案","explanation":"考察意图","tags":["标签1","标签2"]}]}`
}

// isQuestionPipelineMockOutput 判断当前模型输出是否来自仓库内置的 Mock Provider，避免把假数据误判成真实模型格式问题。
func isQuestionPipelineMockOutput(raw string) bool {
	normalized := strings.TrimSpace(raw)
	if normalized == "" {
		return false
	}

	for _, marker := range []string{
		"作为一个Mock AI",
		"这是一个Mock流式响应",
		"实际集成后将连接真实的AI模型",
	} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

// buildQuestionPipelineExistingCardsExcerpt 压缩已生成题卡摘要，帮助模型避免重复考点。
func buildQuestionPipelineExistingCardsExcerpt(cards []AdminQuestionPipelineCard) string {
	if len(cards) == 0 {
		return "暂无已生成题卡，本轮可以从最重要的考点开始。"
	}

	parts := make([]string, 0, len(cards))
	for index, card := range cards {
		if index >= 6 {
			break
		}
		parts = append(parts, fmt.Sprintf("%d. [%s] %s", index+1, strings.ToUpper(firstNonEmptyString(detectQuestionPipelineCardLanguage(card), "unknown")), strings.TrimSpace(card.Title)))
	}
	return strings.Join(parts, "\n")
}

// buildQuestionPipelineCardsFromPlan 在题卡结构化解析失败时，根据规划阶段结果回退生成基础题卡。
func buildQuestionPipelineCardsFromPlan(
	plan *questionPipelinePlanResponse,
	categories []model.Category,
	requirement string,
	candidateLimit int,
) []AdminQuestionPipelineCard {
	if plan == nil || len(plan.Topics) == 0 {
		return nil
	}

	cards := make([]AdminQuestionPipelineCard, 0, len(plan.Topics))
	for _, topicPlan := range plan.Topics {
		topic := strings.TrimSpace(topicPlan.Topic)
		focus := firstNonEmptyString(strings.TrimSpace(topicPlan.Focus), topic)
		if topic == "" {
			continue
		}

		cards = append(cards, AdminQuestionPipelineCard{
			Title:       topic,
			Content:     buildQuestionPipelineFallbackContent(topic, focus, requirement),
			Type:        model.QuestionTypeSubjective,
			Difficulty:  normalizeQuestionPipelineDifficulty(topicPlan.Difficulty),
			Category:    matchQuestionPipelineCategory(categories, topicPlan.Category, requirement),
			Answer:      buildQuestionPipelineFallbackAnswer(topic, focus, requirement),
			Explanation: firstNonEmptyString(buildQuestionPipelineFallbackExplanation(focus), buildQuestionPipelineExplanation(topic)),
			Tags:        buildQuestionPipelineTags(topicPlan.Tags, requirement),
			Confidence:  0.9,
			SourceType:  "generated",
			SourceLabel: "AI 规划回退生成",
			SourceTitle: "规划阶段候选题卡",
			SourceURL:   "",
		})
	}

	cards = dedupeQuestionPipelineCards(cards)
	cards = filterQuestionPipelineCardsByIntent(cards, requirement, "")
	if len(cards) > candidateLimit {
		cards = cards[:candidateLimit]
	}

	return cards
}

// buildQuestionPipelineFallbackContent 基于规划出的考点主题拼装题干，避免回退题卡过于模板化。
func buildQuestionPipelineFallbackContent(topic string, focus string, requirement string) string {
	if strings.ContainsAny(topic, "？?") {
		return topic
	}

	return fmt.Sprintf("请围绕“%s”展开说明，重点解释%s，并结合岗位要求“%s”回答其核心机制、使用边界与常见误区。", topic, focus, requirement)
}

// buildQuestionPipelineFallbackAnswer 基于规划聚焦点生成较稳定的参考答案骨架。
func buildQuestionPipelineFallbackAnswer(topic string, focus string, requirement string) string {
	return fmt.Sprintf("参考答案应至少覆盖以下要点：1. 先定义“%s”的核心概念；2. 重点说明%s；3. 结合“%s”补充典型使用场景、性能权衡与常见误区；4. 给出面试中容易被继续追问的细节。", topic, focus, requirement)
}

// buildQuestionPipelineFallbackExplanation 生成回退题卡的考察意图说明。
func buildQuestionPipelineFallbackExplanation(focus string) string {
	focus = strings.TrimSpace(focus)
	if focus == "" {
		return ""
	}

	return fmt.Sprintf("该题用于重点考察候选人是否真正理解%s，而不是只会背诵结论。", focus)
}

// buildQuestionPipelineExplanation 生成题卡缺省解析，避免出现空解释。
func buildQuestionPipelineExplanation(title string) string {
	return fmt.Sprintf("该题用于检验候选人是否真正理解“%s”背后的核心机制、适用边界与常见误区。", title)
}

// dedupeQuestionPipelinePlan 对模型规划出的考点计划做去重与去空。
func dedupeQuestionPipelinePlan(items []questionPipelineTopicPlan) []questionPipelineTopicPlan {
	seen := make(map[string]bool, len(items))
	filtered := make([]questionPipelineTopicPlan, 0, len(items))
	for _, item := range items {
		topic := strings.TrimSpace(item.Topic)
		focus := strings.TrimSpace(item.Focus)
		if topic == "" || focus == "" {
			continue
		}

		key := strings.ToLower(topic) + "||" + strings.ToLower(focus)
		if seen[key] {
			continue
		}
		seen[key] = true
		item.Topic = topic
		item.Focus = focus
		item.Difficulty = normalizeQuestionPipelineDifficulty(item.Difficulty)
		filtered = append(filtered, item)
	}

	return filtered
}

// matchQuestionPipelineCategory 将推荐分类映射到当前行业分类。
func matchQuestionPipelineCategory(categories []model.Category, suggested string, requirement string) string {
	suggested = strings.TrimSpace(suggested)
	if suggested != "" {
		for _, category := range categories {
			if strings.EqualFold(strings.TrimSpace(category.Name), suggested) {
				return category.Name
			}
		}

		suggestedLower := strings.ToLower(suggested)
		for _, category := range categories {
			categoryLower := strings.ToLower(strings.TrimSpace(category.Name))
			if strings.Contains(categoryLower, suggestedLower) || strings.Contains(suggestedLower, categoryLower) {
				return category.Name
			}
		}
	}

	requirementLower := strings.ToLower(strings.TrimSpace(requirement))
	for _, category := range categories {
		categoryLower := strings.ToLower(strings.TrimSpace(category.Name))
		if strings.Contains(requirementLower, categoryLower) || strings.Contains(categoryLower, requirementLower) {
			return category.Name
		}
	}

	return categories[0].Name
}

// buildQuestionPipelineTags 合并标签并去重。
func buildQuestionPipelineTags(tags []string, requirement string) []string {
	merged := make([]string, 0, len(tags)+2)
	merged = append(merged, tags...)

	for _, topic := range extractQuestionPipelineTopics(requirement) {
		if len([]rune(topic)) > 18 {
			continue
		}
		merged = append(merged, topic)
		if len(merged) >= 6 {
			break
		}
	}

	return dedupeQuestionPipelineStrings(merged)
}

// extractQuestionPipelineTopics 从岗位要求中提炼若干关键词主题。
func extractQuestionPipelineTopics(requirement string) []string {
	parts := strings.FieldsFunc(requirement, func(r rune) bool {
		return r == '，' || r == ',' || r == '；' || r == ';' || r == '、' || r == '\n' || r == '\r'
	})

	topics := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		topics = append(topics, trimmed)
	}

	if len(topics) == 0 {
		return []string{strings.TrimSpace(requirement)}
	}

	return topics
}

// dedupeQuestionPipelineCards 按题目和答案对候选题卡做去重。
func dedupeQuestionPipelineCards(cards []AdminQuestionPipelineCard) []AdminQuestionPipelineCard {
	seen := make(map[string]bool, len(cards))
	filtered := make([]AdminQuestionPipelineCard, 0, len(cards))
	for _, card := range cards {
		key := strings.ToLower(strings.TrimSpace(card.Title)) + "||" + strings.ToLower(strings.TrimSpace(card.Answer))
		if key == "||" || seen[key] {
			continue
		}
		seen[key] = true
		filtered = append(filtered, card)
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		return filtered[i].Confidence > filtered[j].Confidence
	})
	return filtered
}

// mergeQuestionPipelineCards 合并两批题卡并按既有去重规则截断数量。
func mergeQuestionPipelineCards(primary []AdminQuestionPipelineCard, secondary []AdminQuestionPipelineCard, candidateLimit int) []AdminQuestionPipelineCard {
	merged := append([]AdminQuestionPipelineCard{}, primary...)
	merged = append(merged, secondary...)
	merged = dedupeQuestionPipelineCards(merged)
	if len(merged) > candidateLimit {
		return merged[:candidateLimit]
	}
	return merged
}

// dedupeQuestionPipelineStrings 对字符串切片去重并去空。
func dedupeQuestionPipelineStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, trimmed)
	}
	return result
}

// normalizeQuestionPipelineType 规范化题型值，避免前端回传非法题型。
func normalizeQuestionPipelineType(questionType string) string {
	switch strings.TrimSpace(questionType) {
	case model.QuestionTypeChoice:
		return model.QuestionTypeChoice
	case model.QuestionTypeMulti:
		return model.QuestionTypeMulti
	case model.QuestionTypeCode:
		return model.QuestionTypeCode
	default:
		return model.QuestionTypeSubjective
	}
}

// normalizeQuestionPipelineDifficulty 规范化难度值，避免前端回传非法难度。
func normalizeQuestionPipelineDifficulty(difficulty string) string {
	switch strings.TrimSpace(difficulty) {
	case model.QuestionDifficultyEasy:
		return model.QuestionDifficultyEasy
	case model.QuestionDifficultyHard:
		return model.QuestionDifficultyHard
	default:
		return model.QuestionDifficultyMedium
	}
}

// firstNonEmptyString 返回第一个非空字符串。
func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// decodeQuestionPipelinePlanResponse 解析模型输出中的考点计划，兼容数组根节点和常见别名字段。
func decodeQuestionPipelinePlanResponse(raw string) (*questionPipelinePlanResponse, error) {
	value, err := decodeQuestionPipelineJSONValue(raw)
	if err == nil {
		topics := normalizeQuestionPipelineTopics(value)
		if len(topics) > 0 {
			return &questionPipelinePlanResponse{
				Topics: topics,
			}, nil
		}
	}

	topics := parseQuestionPipelinePlanText(sanitizeQuestionPipelineModelOutput(raw))
	if len(topics) == 0 {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("plan topics not found")
	}

	return &questionPipelinePlanResponse{
		Topics: topics,
	}, nil
}

// decodeQuestionPipelineCardsResponse 解析模型输出中的题卡集合，兼容数组根节点和常见别名字段。
func decodeQuestionPipelineCardsResponse(raw string) (*questionPipelineCardsResponse, error) {
	value, err := decodeQuestionPipelineJSONValue(raw)
	if err == nil {
		cards := normalizeQuestionPipelineModelCards(value)
		if len(cards) > 0 {
			return &questionPipelineCardsResponse{
				Cards: cards,
			}, nil
		}
	}

	cards := parseQuestionPipelineCardsText(sanitizeQuestionPipelineModelOutput(raw))
	if len(cards) == 0 {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("cards not found")
	}

	return &questionPipelineCardsResponse{
		Cards: cards,
	}, nil
}

// sanitizeQuestionPipelineModelOutput 对模型原始输出做统一清洗，减少 think 标签、代码块和格式噪音对解析的干扰。
func sanitizeQuestionPipelineModelOutput(raw string) string {
	raw = strings.TrimSpace(strings.TrimPrefix(raw, "\ufeff"))
	raw = stripQuestionPipelineReasoningBlocks(stripQuestionPipelineCodeFence(raw))
	if raw == "" {
		return ""
	}

	lines := strings.Split(raw, "\n")
	if len(lines) == 0 {
		return raw
	}

	firstLine := normalizeQuestionPipelineFieldKey(lines[0])
	switch firstLine {
	case "json", "json输出", "outputjson", "resultjson":
		return strings.TrimSpace(strings.Join(lines[1:], "\n"))
	default:
		return strings.TrimSpace(raw)
	}
}

// buildQuestionPipelineDebugTrace 统一拼装题库流水线调试文本，优先保留模型原始输出，并在必要时补充模型错误。
func buildQuestionPipelineDebugTrace(result *AIDebugResponse) string {
	if result == nil {
		return ""
	}

	modelOutput := strings.TrimSpace(result.ModelOutput)
	modelError := strings.TrimSpace(result.ModelError)
	switch {
	case modelOutput != "" && modelError != "":
		return "[model_output]\n" + modelOutput + "\n\n[model_error]\n" + modelError
	case modelOutput != "":
		return modelOutput
	case modelError != "":
		return "[model_error]\n" + modelError
	default:
		return ""
	}
}

// decodeQuestionPipelineJSONValue 解析模型输出中的 JSON 值，兼容对象、数组和代码块包裹。
func decodeQuestionPipelineJSONValue(raw string) (any, error) {
	candidates := buildQuestionPipelineJSONCandidates(raw)
	for _, candidate := range candidates {
		var payload any
		if err := json.Unmarshal([]byte(candidate), &payload); err == nil {
			return payload, nil
		}
	}

	return nil, fmt.Errorf("json payload not found")
}

// buildQuestionPipelineJSONCandidates 构造若干候选 JSON 片段，提高模型输出解析成功率。
func buildQuestionPipelineJSONCandidates(raw string) []string {
	trimmed := sanitizeQuestionPipelineModelOutput(raw)
	candidates := make([]string, 0, 8)
	for _, candidate := range []string{
		trimmed,
		extractQuestionPipelineJSONObject(trimmed),
		extractQuestionPipelineJSONArray(trimmed),
	} {
		for _, expanded := range expandQuestionPipelineJSONCandidate(candidate) {
			if !containsQuestionPipelineString(candidates, expanded) {
				candidates = append(candidates, expanded)
			}
		}
	}

	for _, candidate := range extractQuestionPipelineBalancedJSONSegments(trimmed) {
		for _, expanded := range expandQuestionPipelineJSONCandidate(candidate) {
			if !containsQuestionPipelineString(candidates, expanded) {
				candidates = append(candidates, expanded)
			}
		}
	}

	return candidates
}

// expandQuestionPipelineJSONCandidate 展开单个候选 JSON 片段，并尝试处理被字符串包裹的 JSON 内容。
func expandQuestionPipelineJSONCandidate(candidate string) []string {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return nil
	}

	values := []string{candidate}
	if unquoted, err := strconv.Unquote(candidate); err == nil {
		unquoted = strings.TrimSpace(unquoted)
		if unquoted != "" && !containsQuestionPipelineString(values, unquoted) {
			values = append(values, unquoted)
		}
	}
	return values
}

// stripQuestionPipelineReasoningBlocks 清理模型输出中的 think/reasoning 片段，避免影响后续 JSON 或文本解析。
func stripQuestionPipelineReasoningBlocks(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	type blockMarker struct {
		start string
		end   string
	}

	blocks := []blockMarker{
		{start: "<think>", end: "</think>"},
		{start: "<reasoning>", end: "</reasoning>"},
	}

	lowered := strings.ToLower(raw)
	for _, block := range blocks {
		for {
			start := strings.Index(lowered, block.start)
			if start < 0 {
				break
			}
			end := strings.Index(lowered[start+len(block.start):], block.end)
			if end < 0 {
				raw = strings.TrimSpace(raw[:start])
				lowered = strings.ToLower(raw)
				break
			}

			realEnd := start + len(block.start) + end + len(block.end)
			raw = raw[:start] + raw[realEnd:]
			lowered = strings.ToLower(raw)
		}
	}

	raw = strings.ReplaceAll(raw, "<think>", "")
	raw = strings.ReplaceAll(raw, "</think>", "")
	raw = strings.ReplaceAll(raw, "<reasoning>", "")
	raw = strings.ReplaceAll(raw, "</reasoning>", "")
	return strings.TrimSpace(raw)
}

// stripQuestionPipelineCodeFence 去除模型常见的 Markdown 代码块包裹。
func stripQuestionPipelineCodeFence(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	if strings.HasPrefix(trimmed, "```") {
		if lineEnd := strings.Index(trimmed, "\n"); lineEnd >= 0 {
			trimmed = trimmed[lineEnd+1:]
		} else {
			trimmed = strings.TrimPrefix(trimmed, "```")
		}
	}
	trimmed = strings.TrimSuffix(strings.TrimSpace(trimmed), "```")
	return strings.TrimSpace(trimmed)
}

// extractQuestionPipelineJSONObject 提取文本中的 JSON 对象主体。
func extractQuestionPipelineJSONObject(raw string) string {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end < start {
		return ""
	}

	return strings.TrimSpace(raw[start : end+1])
}

// extractQuestionPipelineJSONArray 提取文本中的 JSON 数组主体。
func extractQuestionPipelineJSONArray(raw string) string {
	start := strings.Index(raw, "[")
	end := strings.LastIndex(raw, "]")
	if start < 0 || end < start {
		return ""
	}

	return strings.TrimSpace(raw[start : end+1])
}

// extractQuestionPipelineBalancedJSONSegments 从混杂文本中提取所有看起来完整闭合的 JSON 片段，并优先保留后出现的结果块。
func extractQuestionPipelineBalancedJSONSegments(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	segments := make([]string, 0)
	start := -1
	depth := 0
	inString := false
	escaping := false

	for index, char := range raw {
		if escaping {
			escaping = false
			continue
		}

		if char == '\\' && inString {
			escaping = true
			continue
		}

		if char == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}

		switch char {
		case '{', '[':
			if depth == 0 {
				start = index
			}
			depth++
		case '}', ']':
			if depth == 0 {
				continue
			}
			depth--
			if depth == 0 && start >= 0 {
				candidate := strings.TrimSpace(raw[start : index+1])
				if candidate != "" {
					segments = append(segments, candidate)
				}
				start = -1
			}
		}
	}

	reversed := make([]string, 0, len(segments))
	for index := len(segments) - 1; index >= 0; index-- {
		if !containsQuestionPipelineString(reversed, segments[index]) {
			reversed = append(reversed, segments[index])
		}
	}
	return reversed
}

// normalizeQuestionPipelineTopics 将任意 JSON 值尽量归一化为考点计划数组。
func normalizeQuestionPipelineTopics(value any) []questionPipelineTopicPlan {
	items := extractQuestionPipelineItemList(
		value,
		"topics", "plans", "plan", "topic_plans", "topicPlan", "items", "list", "data", "result",
		"考点", "考点计划", "规划", "计划", "主题", "主题规划",
	)
	if len(items) == 0 {
		return nil
	}

	topics := make([]questionPipelineTopicPlan, 0, len(items))
	for _, item := range items {
		normalized, ok := normalizeQuestionPipelineTopic(item)
		if !ok {
			continue
		}
		topics = append(topics, normalized)
	}

	return topics
}

// normalizeQuestionPipelineTopic 归一化单条考点计划，兼容常见字段别名。
func normalizeQuestionPipelineTopic(value any) (questionPipelineTopicPlan, bool) {
	item, ok := value.(map[string]any)
	if !ok {
		return questionPipelineTopicPlan{}, false
	}

	topic := firstNonEmptyString(
		readQuestionPipelineString(item, "topic"),
		readQuestionPipelineString(item, "title"),
		readQuestionPipelineString(item, "name"),
		readQuestionPipelineString(item, "subject"),
		readQuestionPipelineString(item, "考点"),
		readQuestionPipelineString(item, "主题"),
		readQuestionPipelineString(item, "标题"),
	)
	focus := firstNonEmptyString(
		readQuestionPipelineString(item, "focus"),
		readQuestionPipelineString(item, "point"),
		readQuestionPipelineString(item, "goal"),
		readQuestionPipelineString(item, "description"),
		readQuestionPipelineString(item, "focus_point"),
		readQuestionPipelineString(item, "考察重点"),
		readQuestionPipelineString(item, "聚焦点"),
		readQuestionPipelineString(item, "说明"),
		topic,
	)
	if topic == "" {
		return questionPipelineTopicPlan{}, false
	}

	return questionPipelineTopicPlan{
		Topic:      topic,
		Focus:      focus,
		Difficulty: normalizeQuestionPipelineDifficulty(readQuestionPipelineString(item, "difficulty", "level", "难度")),
		Category:   readQuestionPipelineString(item, "category", "classification", "domain", "类别", "分类"),
		Tags:       readQuestionPipelineStringSlice(item, "tags", "keywords", "points", "标签", "关键词"),
	}, true
}

// normalizeQuestionPipelineModelCards 将任意 JSON 值尽量归一化为题卡数组。
func normalizeQuestionPipelineModelCards(value any) []questionPipelineModelCard {
	items := extractQuestionPipelineItemList(
		value,
		"cards", "questions", "items", "list", "results", "data", "result",
		"题卡", "题目", "候选题卡", "候选题目",
	)
	if len(items) == 0 {
		return nil
	}

	cards := make([]questionPipelineModelCard, 0, len(items))
	for _, item := range items {
		normalized, ok := normalizeQuestionPipelineModelCard(item)
		if !ok {
			continue
		}
		cards = append(cards, normalized)
	}

	return cards
}

// normalizeQuestionPipelineModelCard 归一化单张题卡，兼容 question/reference_answer 等别名字段。
func normalizeQuestionPipelineModelCard(value any) (questionPipelineModelCard, bool) {
	item, ok := value.(map[string]any)
	if !ok {
		return questionPipelineModelCard{}, false
	}

	title := firstNonEmptyString(
		readQuestionPipelineString(item, "title"),
		readQuestionPipelineString(item, "name"),
		readQuestionPipelineString(item, "topic"),
		readQuestionPipelineString(item, "question_title"),
		readQuestionPipelineString(item, "标题"),
		readQuestionPipelineString(item, "题目标题"),
		readQuestionPipelineString(item, "考点"),
	)
	content := firstNonEmptyString(
		readQuestionPipelineString(item, "content"),
		readQuestionPipelineString(item, "question"),
		readQuestionPipelineString(item, "body"),
		readQuestionPipelineString(item, "prompt"),
		readQuestionPipelineString(item, "description"),
		readQuestionPipelineString(item, "题目"),
		readQuestionPipelineString(item, "题干"),
		readQuestionPipelineString(item, "问题"),
	)
	answer := firstNonEmptyString(
		readQuestionPipelineString(item, "answer"),
		readQuestionPipelineString(item, "reference_answer"),
		readQuestionPipelineString(item, "standard_answer"),
		readQuestionPipelineString(item, "sample_answer"),
		readQuestionPipelineString(item, "expected_answer"),
		readQuestionPipelineString(item, "solution"),
		readQuestionPipelineString(item, "analysis"),
		readQuestionPipelineString(item, "答案"),
		readQuestionPipelineString(item, "参考答案"),
		readQuestionPipelineString(item, "标准答案"),
		readQuestionPipelineString(item, "解析"),
	)
	if title == "" {
		title = summarizeQuestionPipelineTitle(content)
	}
	if content == "" {
		content = title
	}
	if title == "" || content == "" || answer == "" {
		return questionPipelineModelCard{}, false
	}

	return questionPipelineModelCard{
		Title:       title,
		Content:     content,
		Type:        normalizeQuestionPipelineType(readQuestionPipelineString(item, "type", "question_type", "kind", "题型", "类型")),
		Difficulty:  normalizeQuestionPipelineDifficulty(readQuestionPipelineString(item, "difficulty", "level", "难度")),
		Category:    readQuestionPipelineString(item, "category", "classification", "domain", "类别", "分类"),
		Answer:      answer,
		Explanation: firstNonEmptyString(readQuestionPipelineString(item, "explanation", "analysis", "rationale", "reasoning", "解释", "说明", "解析")),
		Tags:        readQuestionPipelineStringSlice(item, "tags", "keywords", "points", "标签", "关键词"),
	}, true
}

// extractQuestionPipelineItemList 递归提取题目流水线关注的列表字段，兼容嵌套对象和字符串化 JSON。
func extractQuestionPipelineItemList(value any, keys ...string) []any {
	switch typed := value.(type) {
	case []any:
		return typed
	case map[string]any:
		for _, key := range keys {
			if child, ok := typed[key]; ok {
				if list := extractQuestionPipelineItemList(child, keys...); len(list) > 0 {
					return list
				}
			}
		}
		if looksLikeQuestionPipelineItem(typed) {
			return []any{typed}
		}
		for _, child := range typed {
			if list := extractQuestionPipelineItemList(child, keys...); len(list) > 0 {
				return list
			}
		}
	case string:
		decoded, ok := decodeQuestionPipelineEmbeddedValue(typed)
		if ok {
			return extractQuestionPipelineItemList(decoded, keys...)
		}
	}

	return nil
}

// looksLikeQuestionPipelineItem 判断当前对象是否已经像单条计划项或单张题卡。
func looksLikeQuestionPipelineItem(item map[string]any) bool {
	for _, key := range []string{
		"topic", "title", "name", "subject", "question", "content",
		"考点", "主题", "标题", "题目", "题干",
	} {
		if strings.TrimSpace(readQuestionPipelineString(item, key)) != "" {
			return true
		}
	}

	return false
}

// decodeQuestionPipelineEmbeddedValue 尝试解析字段里的字符串化 JSON，兼容模型把数组再包成字符串。
func decodeQuestionPipelineEmbeddedValue(raw string) (any, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, false
	}

	if !(strings.HasPrefix(raw, "{") || strings.HasPrefix(raw, "[") || strings.Contains(raw, "\"topics\"") || strings.Contains(raw, "\"questions\"")) {
		return nil, false
	}

	value, err := decodeQuestionPipelineJSONValue(raw)
	if err != nil {
		return nil, false
	}

	return value, true
}

// parseQuestionPipelinePlanText 将普通文本、Markdown 或 YAML 风格的规划结果解析为考点计划。
func parseQuestionPipelinePlanText(raw string) []questionPipelineTopicPlan {
	lines := splitQuestionPipelineLines(stripQuestionPipelineCodeFence(raw))
	if len(lines) == 0 {
		return nil
	}

	topics := make([]questionPipelineTopicPlan, 0)
	current := questionPipelineTopicPlan{}
	for _, line := range lines {
		normalizedLine := trimQuestionPipelineListMarker(line)
		if normalizedLine == "" || isQuestionPipelinePlanNoiseLine(normalizedLine) {
			continue
		}

		key, value, ok := splitQuestionPipelineKeyValue(normalizedLine)
		if ok {
			field := normalizeQuestionPipelineFieldKey(key)
			if isQuestionPipelineTopicField(field) && current.Topic != "" {
				topics = appendQuestionPipelineTopicPlan(topics, current)
				current = questionPipelineTopicPlan{}
			}
			applyQuestionPipelinePlanField(&current, field, value)
			continue
		}

		if current.Topic == "" {
			current.Topic = normalizedLine
			current.Focus = normalizedLine
			continue
		}

		if current.Focus == "" || current.Focus == current.Topic {
			current.Focus = normalizedLine
			continue
		}

		current.Focus = strings.TrimSpace(current.Focus + "；" + normalizedLine)
	}

	topics = appendQuestionPipelineTopicPlan(topics, current)
	return dedupeQuestionPipelinePlan(topics)
}

// splitQuestionPipelineLines 将原始文本拆成去空后的逐行内容。
func splitQuestionPipelineLines(raw string) []string {
	parts := strings.Split(raw, "\n")
	lines := make([]string, 0, len(parts))
	for _, part := range parts {
		line := strings.TrimSpace(strings.TrimSuffix(part, "\r"))
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}

	return lines
}

// trimQuestionPipelineListMarker 去除列表项常见的序号和项目符号前缀。
func trimQuestionPipelineListMarker(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}

	for _, prefix := range []string{"- ", "* ", "• ", "+ "} {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}

	index := 0
	for index < len(line) && line[index] >= '0' && line[index] <= '9' {
		index++
	}
	if index > 0 && index < len(line) {
		switch {
		case strings.HasPrefix(line[index:], "."),
			strings.HasPrefix(line[index:], ")"),
			strings.HasPrefix(line[index:], "、"),
			strings.HasPrefix(line[index:], "-"),
			strings.HasPrefix(line[index:], ":"):
			return strings.TrimSpace(line[index+1:])
		}
	}

	return line
}

// isQuestionPipelinePlanNoiseLine 过滤模型在计划文本前后常见的说明性噪音行。
func isQuestionPipelinePlanNoiseLine(line string) bool {
	normalized := normalizeQuestionPipelineFieldKey(line)
	for _, marker := range []string{
		"以下是", "下面是", "规划结果", "计划如下", "输出如下",
	} {
		if strings.HasPrefix(line, marker) {
			return true
		}
	}

	for _, prefix := range []string{"json", "```", "topics", "plans", "plan", "result"} {
		if strings.HasPrefix(normalized, prefix) && !strings.Contains(line, ":") && !strings.Contains(line, "：") {
			return true
		}
	}

	return false
}

// splitQuestionPipelineKeyValue 拆分单行中的键值对，兼容中英文冒号。
func splitQuestionPipelineKeyValue(line string) (string, string, bool) {
	for _, sep := range []string{"：", ":"} {
		if idx := strings.Index(line, sep); idx >= 0 {
			key := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+len(sep):])
			if key != "" && value != "" {
				return key, value, true
			}
		}
	}

	return "", "", false
}

// normalizeQuestionPipelineFieldKey 统一字段名，便于匹配英文别名和中文别名。
func normalizeQuestionPipelineFieldKey(key string) string {
	replacer := strings.NewReplacer(" ", "", "_", "", "-", "", "\t", "", "（", "", "）", "", "(", "", ")", "")
	return strings.ToLower(strings.TrimSpace(replacer.Replace(key)))
}

// isQuestionPipelineTopicField 判断当前字段是否表示新的一条考点主题。
func isQuestionPipelineTopicField(field string) bool {
	switch field {
	case "topic", "title", "name", "subject", "考点", "主题", "标题":
		return true
	default:
		return false
	}
}

// applyQuestionPipelinePlanField 将文本键值对写入当前考点计划对象。
func applyQuestionPipelinePlanField(plan *questionPipelineTopicPlan, field string, value string) {
	if plan == nil {
		return
	}

	switch field {
	case "topic", "title", "name", "subject", "考点", "主题", "标题":
		plan.Topic = strings.TrimSpace(value)
		if strings.TrimSpace(plan.Focus) == "" {
			plan.Focus = strings.TrimSpace(value)
		}
	case "focus", "point", "goal", "description", "focuspoint", "考察重点", "聚焦点", "说明":
		plan.Focus = strings.TrimSpace(value)
	case "difficulty", "level", "难度":
		plan.Difficulty = normalizeQuestionPipelineDifficulty(strings.TrimSpace(value))
	case "category", "classification", "domain", "类别", "分类":
		plan.Category = strings.TrimSpace(value)
	case "tags", "keywords", "points", "标签", "关键词":
		plan.Tags = dedupeQuestionPipelineStrings(strings.FieldsFunc(value, func(r rune) bool {
			return r == ',' || r == '，' || r == '、' || r == ';' || r == '；'
		}))
	}
}

// appendQuestionPipelineTopicPlan 仅在主题非空时追加计划项，并补齐缺省聚焦点。
func appendQuestionPipelineTopicPlan(items []questionPipelineTopicPlan, plan questionPipelineTopicPlan) []questionPipelineTopicPlan {
	plan.Topic = strings.TrimSpace(plan.Topic)
	if plan.Topic == "" {
		return items
	}
	if strings.TrimSpace(plan.Focus) == "" {
		plan.Focus = plan.Topic
	}
	plan.Difficulty = normalizeQuestionPipelineDifficulty(plan.Difficulty)
	return append(items, plan)
}

// parseQuestionPipelineCardsText 将普通文本、Markdown 或 YAML 风格的题卡内容解析为结构化题卡。
func parseQuestionPipelineCardsText(raw string) []questionPipelineModelCard {
	lines := splitQuestionPipelineLines(stripQuestionPipelineCodeFence(raw))
	if len(lines) == 0 {
		return nil
	}

	cards := make([]questionPipelineModelCard, 0)
	current := questionPipelineModelCard{}
	for _, line := range lines {
		normalizedLine := trimQuestionPipelineListMarker(line)
		if normalizedLine == "" || isQuestionPipelineCardNoiseLine(normalizedLine) {
			continue
		}

		key, value, ok := splitQuestionPipelineKeyValue(normalizedLine)
		if ok {
			field := normalizeQuestionPipelineFieldKey(key)
			if isQuestionPipelineCardBoundaryField(field) && hasQuestionPipelineCardContent(current) {
				cards = appendQuestionPipelineModelCard(cards, current)
				current = questionPipelineModelCard{}
			}
			applyQuestionPipelineCardField(&current, field, value)
			continue
		}

		if strings.TrimSpace(current.Title) == "" {
			current.Title = normalizedLine
			if strings.TrimSpace(current.Content) == "" {
				current.Content = normalizedLine
			}
			continue
		}

		if strings.TrimSpace(current.Content) == "" || strings.TrimSpace(current.Content) == strings.TrimSpace(current.Title) {
			current.Content = normalizedLine
			continue
		}

		if strings.TrimSpace(current.Answer) == "" {
			current.Answer = normalizedLine
			continue
		}

		if strings.TrimSpace(current.Explanation) == "" {
			current.Explanation = normalizedLine
			continue
		}

		current.Explanation = strings.TrimSpace(current.Explanation + "；" + normalizedLine)
	}

	cards = appendQuestionPipelineModelCard(cards, current)
	return dedupeQuestionPipelineModelCards(cards)
}

// isQuestionPipelineCardNoiseLine 过滤题卡文本前后的说明性噪音行。
func isQuestionPipelineCardNoiseLine(line string) bool {
	for _, marker := range []string{"以下是", "下面是", "输出结果", "生成结果", "候选题卡", "题卡如下"} {
		if strings.HasPrefix(line, marker) {
			return true
		}
	}

	normalized := normalizeQuestionPipelineFieldKey(line)
	for _, prefix := range []string{"json", "```", "cards", "questions", "result"} {
		if strings.HasPrefix(normalized, prefix) && !strings.Contains(line, ":") && !strings.Contains(line, "：") {
			return true
		}
	}

	return false
}

// isQuestionPipelineCardBoundaryField 判断某个字段是否意味着一张新题卡开始。
func isQuestionPipelineCardBoundaryField(field string) bool {
	switch field {
	case "title", "name", "topic", "questiontitle", "标题", "题目标题", "考点":
		return true
	default:
		return false
	}
}

// hasQuestionPipelineCardContent 判断当前题卡是否已积累足够内容，可在遇到下一个边界时落盘。
func hasQuestionPipelineCardContent(card questionPipelineModelCard) bool {
	return strings.TrimSpace(card.Title) != "" || strings.TrimSpace(card.Content) != "" || strings.TrimSpace(card.Answer) != ""
}

// applyQuestionPipelineCardField 将文本键值对写入当前题卡对象。
func applyQuestionPipelineCardField(card *questionPipelineModelCard, field string, value string) {
	if card == nil {
		return
	}

	switch field {
	case "title", "name", "topic", "questiontitle", "标题", "题目标题", "考点":
		card.Title = strings.TrimSpace(value)
		if strings.TrimSpace(card.Content) == "" {
			card.Content = strings.TrimSpace(value)
		}
	case "content", "question", "body", "prompt", "description", "题目", "题干", "问题":
		card.Content = strings.TrimSpace(value)
	case "answer", "referenceanswer", "standardanswer", "sampleanswer", "expectedanswer", "solution", "答案", "参考答案", "标准答案":
		card.Answer = strings.TrimSpace(value)
	case "explanation", "analysis", "rationale", "reasoning", "解释", "说明", "解析":
		if strings.TrimSpace(card.Answer) == "" && (field == "analysis" || field == "解析") {
			card.Answer = strings.TrimSpace(value)
			return
		}
		card.Explanation = strings.TrimSpace(value)
	case "type", "questiontype", "kind", "题型", "类型":
		card.Type = normalizeQuestionPipelineType(strings.TrimSpace(value))
	case "difficulty", "level", "难度":
		card.Difficulty = normalizeQuestionPipelineDifficulty(strings.TrimSpace(value))
	case "category", "classification", "domain", "类别", "分类":
		card.Category = strings.TrimSpace(value)
	case "tags", "keywords", "points", "标签", "关键词":
		card.Tags = dedupeQuestionPipelineStrings(strings.FieldsFunc(value, func(r rune) bool {
			return r == ',' || r == '，' || r == '、' || r == ';' || r == '；'
		}))
	}
}

// appendQuestionPipelineModelCard 在题卡字段完整度达标时追加到结果集合。
func appendQuestionPipelineModelCard(cards []questionPipelineModelCard, card questionPipelineModelCard) []questionPipelineModelCard {
	card.Title = strings.TrimSpace(card.Title)
	card.Content = strings.TrimSpace(card.Content)
	card.Answer = strings.TrimSpace(card.Answer)
	if card.Title == "" && card.Content != "" {
		card.Title = summarizeQuestionPipelineTitle(card.Content)
	}
	if card.Content == "" {
		card.Content = card.Title
	}
	if card.Title == "" || card.Content == "" || card.Answer == "" {
		return cards
	}
	card.Type = normalizeQuestionPipelineType(card.Type)
	card.Difficulty = normalizeQuestionPipelineDifficulty(card.Difficulty)
	card.Explanation = firstNonEmptyString(card.Explanation, buildQuestionPipelineExplanation(card.Title))
	return append(cards, card)
}

// dedupeQuestionPipelineModelCards 对文本解析出的题卡做去重，避免同一题卡重复落入结果。
func dedupeQuestionPipelineModelCards(cards []questionPipelineModelCard) []questionPipelineModelCard {
	seen := make(map[string]bool, len(cards))
	filtered := make([]questionPipelineModelCard, 0, len(cards))
	for _, card := range cards {
		key := strings.ToLower(strings.TrimSpace(card.Title)) + "||" + strings.ToLower(strings.TrimSpace(card.Answer))
		if key == "||" || seen[key] {
			continue
		}
		seen[key] = true
		filtered = append(filtered, card)
	}

	return filtered
}

// filterQuestionPipelineCardsByIntent 按岗位要求和智能体命令过滤明显跑偏的泛项目题。
func filterQuestionPipelineCardsByIntent(cards []AdminQuestionPipelineCard, requirement string, agentPrompt string) []AdminQuestionPipelineCard {
	if !shouldFilterQuestionPipelineProjectCards(requirement, agentPrompt) {
		return cards
	}

	filtered := make([]AdminQuestionPipelineCard, 0, len(cards))
	for _, card := range cards {
		if shouldDropQuestionPipelineProjectCard(card, requirement, agentPrompt) {
			continue
		}
		filtered = append(filtered, card)
	}

	if len(filtered) == 0 {
		if hasQuestionPipelineHardConstraints(requirement, agentPrompt) {
			return filtered
		}
		return cards
	}

	return filtered
}

// hasQuestionPipelineHardConstraints 判断当前输入是否包含明确到可执行层面的硬约束。
func hasQuestionPipelineHardConstraints(requirement string, agentPrompt string) bool {
	constraints := buildQuestionPipelineConstraintProfile(requirement, agentPrompt, defaultQuestionPipelineCount)
	return constraints.GoFeatureOnly || constraints.ExcludeProjectCards || len(constraints.ExactLanguageCounts) > 0 || constraints.RemainingLanguage != ""
}

// shouldFilterQuestionPipelineProjectCards 判断当前需求是否明显聚焦语言特性，应压制项目八股题。
func shouldFilterQuestionPipelineProjectCards(requirement string, agentPrompt string) bool {
	combined := strings.ToLower(strings.TrimSpace(requirement + "\n" + agentPrompt))
	for _, marker := range []string{"保留项目题", "允许项目题", "包含项目题", "加入项目题"} {
		if strings.Contains(combined, marker) {
			return false
		}
	}

	for _, marker := range []string{
		"go语言特性", "语言特性", "语言机制", "核心特性", "底层原理", "底层机制", "聚焦go", "聚焦go语言", "聚焦于go语言特性",
	} {
		if strings.Contains(combined, marker) {
			return true
		}
	}

	return false
}

// shouldDropQuestionPipelineProjectCard 判断单张题卡是否属于与当前意图明显不符的泛项目题。
func shouldDropQuestionPipelineProjectCard(card AdminQuestionPipelineCard, requirement string, agentPrompt string) bool {
	text := strings.ToLower(strings.TrimSpace(card.Title + "\n" + card.Content))
	if shouldKeepQuestionPipelineGoFeatureCard(text, requirement, agentPrompt) {
		return false
	}

	if shouldFilterQuestionPipelineGoFeatureOnly(requirement, agentPrompt) {
		return true
	}

	genericProjectMarkers := []string{
		"项目", "职业规划", "服务发现", "幂等", "微服务", "通信", "最大的挑战", "项目中", "你是如何",
	}
	matchedGenericProject := false
	for _, marker := range genericProjectMarkers {
		if strings.Contains(text, marker) {
			matchedGenericProject = true
			break
		}
	}
	if !matchedGenericProject {
		return false
	}

	combined := strings.ToLower(strings.TrimSpace(requirement + "\n" + agentPrompt))
	for _, keepMarker := range []string{
		"java", "go", "golang", "channel", "goroutine", "gmp", "slice", "map", "interface", "defer", "panic", "recover",
	} {
		if strings.Contains(text, keepMarker) && strings.Contains(combined, keepMarker) {
			return false
		}
	}

	return true
}

// shouldFilterQuestionPipelineGoFeatureOnly 判断当前需求是否应仅保留 Go 语言特性相关题目。
func shouldFilterQuestionPipelineGoFeatureOnly(requirement string, agentPrompt string) bool {
	combined := strings.ToLower(strings.TrimSpace(requirement + "\n" + agentPrompt))
	for _, marker := range []string{
		"go语言特性", "聚焦与go语言特性", "聚焦go语言特性", "go 语言特性", "golang特性", "重点考察八股", "go八股", "go 语言机制",
	} {
		if strings.Contains(combined, marker) {
			return true
		}
	}

	return false
}

// shouldKeepQuestionPipelineGoFeatureCard 判断题卡文本是否属于 Go 语言特性/八股范围。
func shouldKeepQuestionPipelineGoFeatureCard(text string, requirement string, agentPrompt string) bool {
	combined := strings.ToLower(strings.TrimSpace(requirement + "\n" + agentPrompt))
	goMarkers := []string{
		"go语言", "go 语言", "golang", "goroutine", "gmp", "channel", "select", "context", "slice", "map", "interface",
		"defer", "panic", "recover", "mutex", "rwmutex", "逃逸", "gc", "垃圾回收", "调度", "并发", "并行",
		"内存模型", "内存逃逸", "协程", "锁", "原子", "sync", "unsafe", "反射", "nil", "make", "new",
	}
	for _, marker := range goMarkers {
		if strings.Contains(text, marker) && (strings.Contains(combined, "go") || strings.Contains(combined, "golang")) {
			return true
		}
	}

	javaMarkers := []string{"java", "jvm", "java内存模型", "spring", "hashmap", "concurrenthashmap"}
	for _, marker := range javaMarkers {
		if strings.Contains(text, marker) && strings.Contains(combined, "java") {
			return true
		}
	}

	return false
}

// enforceQuestionPipelineCardConstraints 对候选题卡执行语言配额与偏题过滤等硬约束校验。
func enforceQuestionPipelineCardConstraints(cards []AdminQuestionPipelineCard, profile questionPipelineConstraintProfile, requirement string, agentPrompt string) ([]AdminQuestionPipelineCard, []string) {
	if len(cards) == 0 {
		return cards, nil
	}

	normalized := make([]AdminQuestionPipelineCard, 0, len(cards))
	for _, card := range cards {
		if profile.RequireSubjective {
			card.Type = model.QuestionTypeSubjective
		}
		if profile.GoFeatureOnly && !matchesQuestionPipelineLanguageFocus(card, profile, requirement, agentPrompt) {
			continue
		}
		if profile.ExcludeProjectCards && isQuestionPipelineGenericProjectCard(card) && !matchesQuestionPipelineLanguageFocus(card, profile, requirement, agentPrompt) {
			continue
		}
		normalized = append(normalized, card)
	}

	warnings := make([]string, 0)
	if len(normalized) == 0 {
		if len(cards) > 0 && hasQuestionPipelineHardConstraints(requirement, agentPrompt) {
			warnings = append(warnings, "生成结果没有满足硬约束的题卡，已全部丢弃。")
		}
		return normalized, warnings
	}

	limited, quotaWarnings := applyQuestionPipelineLanguageQuotas(normalized, profile)
	warnings = append(warnings, quotaWarnings...)
	return limited, warnings
}

// applyQuestionPipelineLanguageQuotas 按语言配额裁剪题卡，优先满足显式语言数量约束。
func applyQuestionPipelineLanguageQuotas(cards []AdminQuestionPipelineCard, profile questionPipelineConstraintProfile) ([]AdminQuestionPipelineCard, []string) {
	if len(profile.ExactLanguageCounts) == 0 && profile.RemainingLanguage == "" {
		return cards, nil
	}

	buckets := make(map[string][]AdminQuestionPipelineCard)
	for _, card := range cards {
		language := detectQuestionPipelineCardLanguage(card)
		buckets[language] = append(buckets[language], card)
	}

	selected := make([]AdminQuestionPipelineCard, 0, len(cards))
	used := make(map[string]bool)
	warnings := make([]string, 0)

	appendCards := func(items []AdminQuestionPipelineCard, limit int) {
		for _, item := range items {
			if len(selected) >= profile.CandidateCount || limit <= 0 {
				return
			}
			key := strings.ToLower(strings.TrimSpace(item.Title)) + "||" + strings.ToLower(strings.TrimSpace(item.Answer))
			if used[key] {
				continue
			}
			used[key] = true
			selected = append(selected, item)
			limit--
		}
	}

	for _, language := range profile.ExactLanguageOrder {
		need := profile.ExactLanguageCounts[language]
		before := len(selected)
		appendCards(buckets[language], need)
		got := len(selected) - before
		if got < need {
			warnings = append(warnings, fmt.Sprintf("需要 %d 张 %s 题卡，但当前仅生成 %d 张满足条件的题卡。", need, strings.ToUpper(language), got))
		}
	}

	if profile.RemainingLanguage != "" && len(selected) < profile.CandidateCount {
		before := len(selected)
		appendCards(buckets[profile.RemainingLanguage], profile.CandidateCount-len(selected))
		got := len(selected) - before
		if got < profile.CandidateCount-before {
			warnings = append(warnings, fmt.Sprintf("其余题卡要求为 %s，但当前不足以填满目标数量。", strings.ToUpper(profile.RemainingLanguage)))
		}
	}

	if len(selected) == 0 {
		return selected, warnings
	}
	if len(selected) > profile.CandidateCount {
		selected = selected[:profile.CandidateCount]
	}
	return selected, warnings
}

// detectQuestionPipelineCardLanguage 基于题卡文本推断其主要语言归属。
func detectQuestionPipelineCardLanguage(card AdminQuestionPipelineCard) string {
	text := strings.ToLower(strings.TrimSpace(card.Title + "\n" + card.Content + "\n" + card.Answer))
	goMarkers := []string{
		"go语言", "go 语言", "golang", "goroutine", "gmp", "channel", "slice", "map", "interface", "defer",
		"panic", "recover", "mutex", "rwmutex", "逃逸", "gc", "垃圾回收", "调度", "sync", "unsafe", "反射",
	}
	javaMarkers := []string{
		"java", "jvm", "spring", "hashmap", "concurrenthashmap", "volatile", "synchronized", "java内存模型",
	}

	hasGo := false
	for _, marker := range goMarkers {
		if strings.Contains(text, marker) {
			hasGo = true
			break
		}
	}
	hasJava := false
	for _, marker := range javaMarkers {
		if strings.Contains(text, marker) {
			hasJava = true
			break
		}
	}

	switch {
	case hasJava:
		return "java"
	case hasGo:
		return "go"
	default:
		return ""
	}
}

// matchesQuestionPipelineLanguageFocus 判断题卡是否仍然落在当前语言特性聚焦范围内。
func matchesQuestionPipelineLanguageFocus(card AdminQuestionPipelineCard, profile questionPipelineConstraintProfile, requirement string, agentPrompt string) bool {
	language := detectQuestionPipelineCardLanguage(card)
	if language == "" {
		return false
	}
	if profile.RemainingLanguage != "" && language == profile.RemainingLanguage {
		return true
	}
	if profile.ExactLanguageCounts[language] > 0 {
		return true
	}
	return shouldKeepQuestionPipelineGoFeatureCard(strings.ToLower(strings.TrimSpace(card.Title+"\n"+card.Content+"\n"+card.Answer)), requirement, agentPrompt)
}

// isQuestionPipelineGenericProjectCard 判断题卡是否属于项目经历或泛后端流程类问题。
func isQuestionPipelineGenericProjectCard(card AdminQuestionPipelineCard) bool {
	text := strings.ToLower(strings.TrimSpace(card.Title + "\n" + card.Content))
	for _, marker := range []string{
		"项目", "职业规划", "服务发现", "幂等", "微服务", "通信", "最大的挑战", "项目中", "你是如何", "行为面试",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

// readQuestionPipelineString 从对象中按顺序读取首个非空字符串字段。
func readQuestionPipelineString(item map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := item[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				return strings.TrimSpace(typed)
			}
		case []any:
			parts := make([]string, 0, len(typed))
			for _, child := range typed {
				text := strings.TrimSpace(fmt.Sprint(child))
				if text != "" {
					parts = append(parts, text)
				}
			}
			if len(parts) > 0 {
				return strings.Join(parts, "\n")
			}
		default:
			text := strings.TrimSpace(fmt.Sprint(typed))
			if text != "" && text != "<nil>" {
				return text
			}
		}
	}

	return ""
}

// readQuestionPipelineStringSlice 从对象中读取标签数组或逗号分隔字符串。
func readQuestionPipelineStringSlice(item map[string]any, keys ...string) []string {
	for _, key := range keys {
		value, ok := item[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case []any:
			items := make([]string, 0, len(typed))
			for _, child := range typed {
				text := strings.TrimSpace(fmt.Sprint(child))
				if text != "" && text != "<nil>" {
					items = append(items, text)
				}
			}
			if len(items) > 0 {
				return dedupeQuestionPipelineStrings(items)
			}
		case []string:
			if len(typed) > 0 {
				return dedupeQuestionPipelineStrings(typed)
			}
		case string:
			parts := strings.FieldsFunc(typed, func(r rune) bool {
				return r == ',' || r == '，' || r == '、' || r == '\n' || r == '\r'
			})
			if len(parts) > 0 {
				return dedupeQuestionPipelineStrings(parts)
			}
		}
	}

	return nil
}

// summarizeQuestionPipelineTitle 从较长题干中截取一个可读短标题。
func summarizeQuestionPipelineTitle(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}

	runes := []rune(content)
	if len(runes) <= 28 {
		return content
	}

	return strings.TrimSpace(string(runes[:28]))
}

// containsQuestionPipelineString 判断候选字符串是否已存在，避免重复尝试同一 JSON 片段。
func containsQuestionPipelineString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

// buildQuestionPipelineFailureMessage 生成更可定位的失败提示，优先回传真实链路错误。
func buildQuestionPipelineFailureMessage(warnings []string) string {
	unique := dedupeQuestionPipelineStrings(warnings)
	if len(unique) == 0 {
		return "当前没有生成出可用题卡，请检查 AI 配置、提示词要求或抓取素材后重试"
	}

	if len(unique) == 1 {
		return unique[0]
	}

	if len(unique) > 2 {
		unique = unique[:2]
	}

	return strings.Join(unique, "；")
}
