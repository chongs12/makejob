// Package service 提供业务逻辑层实现
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"makejob-backend/internal/common"
	"makejob-backend/internal/model"
	"makejob-backend/internal/repository"
	"makejob-backend/internal/scraper"
)

// ScraperService 爬虫服务接口
type ScraperService interface {
	Search(ctx context.Context, req scraper.SearchRequest) ([]scraper.SearchResult, error)
	Fetch(ctx context.Context, req scraper.FetchRequest) (*scraper.FetchResult, error)
	Clean(ctx context.Context, req scraper.CleanRequest) (*scraper.CleanResult, error)
	Import(ctx context.Context, req scraper.ImportRequest) (*scraper.ImportResult, error)
	CreateImportTask(ctx context.Context, req scraper.ImportRequest) (*model.ScraperTask, error)
	GetTask(ctx context.Context, taskID uint) (*scraper.TaskDetail, error)
	RetryTask(ctx context.Context, taskID uint) (*model.ScraperTask, error)
	ListTasks(ctx context.Context, page, pageSize int, filter scraper.TaskListFilter) (*common.PageResult, error)
	GetSources(ctx context.Context) ([]scraper.Source, error)
	RunNextPendingTask(ctx context.Context) (*model.ScraperTask, bool, error)
}

// scraperService 爬虫服务实现
type scraperService struct {
	provider     scraper.ScraperProvider
	cleaner      scraper.QuestionCleaner
	scraperRepo  repository.ScraperTaskRepository
	industryRepo repository.IndustryRepository
	categoryRepo repository.AdminCategoryRepository
	questionRepo repository.AdminQuestionRepository
}

// NewScraperService 创建爬虫服务实例
func NewScraperService(
	provider scraper.ScraperProvider,
	cleaner scraper.QuestionCleaner,
	scraperRepo repository.ScraperTaskRepository,
	industryRepo repository.IndustryRepository,
	categoryRepo repository.AdminCategoryRepository,
	questionRepo repository.AdminQuestionRepository,
) ScraperService {
	return &scraperService{
		provider:     provider,
		cleaner:      cleaner,
		scraperRepo:  scraperRepo,
		industryRepo: industryRepo,
		categoryRepo: categoryRepo,
		questionRepo: questionRepo,
	}
}

// Search 搜索面经
func (s *scraperService) Search(ctx context.Context, req scraper.SearchRequest) ([]scraper.SearchResult, error) {
	// 设置默认值
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	return s.provider.Search(ctx, req)
}

// Fetch 爬取面经内容
func (s *scraperService) Fetch(ctx context.Context, req scraper.FetchRequest) (*scraper.FetchResult, error) {
	result, err := s.provider.Fetch(ctx, req)
	if err != nil {
		return nil, err
	}

	// 创建爬取任务记录
	task := &model.ScraperTask{
		TaskType:    scraper.TaskTypeFetchSnapshot,
		SourceURL:   req.URL,
		SourceTitle: result.Title,
		Source:      req.Source,
		Status:      scraper.TaskStatusFetched,
		RawContent:  result.Content,
	}

	if err := s.scraperRepo.Create(ctx, task); err != nil {
		return nil, err
	}

	return result, nil
}

// Clean 清洗面经内容
func (s *scraperService) Clean(ctx context.Context, req scraper.CleanRequest) (*scraper.CleanResult, error) {
	return s.cleaner.Clean(ctx, req)
}

// Import 导入题目到题库
func (s *scraperService) Import(ctx context.Context, req scraper.ImportRequest) (*scraper.ImportResult, error) {
	return s.importQuestions(ctx, req)
}

// CreateImportTask 创建一条待执行的异步导入任务，供后台先入库、再交由独立 worker 消费。
func (s *scraperService) CreateImportTask(ctx context.Context, req scraper.ImportRequest) (*model.ScraperTask, error) {
	payloadJSON, err := buildScraperImportTaskPayload(req)
	if err != nil {
		return nil, err
	}
	industry, err := s.industryRepo.GetByCode(ctx, strings.TrimSpace(req.IndustryCode))
	if err != nil {
		return nil, err
	}
	if industry == nil {
		return nil, common.NewBusinessError(common.CodeNotFound, "行业不存在")
	}

	task := &model.ScraperTask{
		TaskType:      scraper.TaskTypeImportQuestions,
		SourceURL:     strings.TrimSpace(req.SourceURL),
		SourceTitle:   strings.TrimSpace(req.SourceTitle),
		Source:        resolveScraperTaskSource(req.SourceURL),
		Status:        scraper.TaskStatusPending,
		PayloadJSON:   payloadJSON,
		QuestionCount: len(req.Questions),
	}
	if task.SourceURL == "" {
		task.SourceURL = "manual://question-import"
	}
	if err := s.scraperRepo.Create(ctx, task); err != nil {
		return nil, err
	}
	return task, nil
}

// RunNextPendingTask 领取并执行下一条待处理任务，当前优先支持题目导入任务。
func (s *scraperService) RunNextPendingTask(ctx context.Context) (*model.ScraperTask, bool, error) {
	task, err := s.scraperRepo.ClaimNextPending(ctx, scraper.TaskTypeImportQuestions)
	if err != nil {
		return nil, false, err
	}
	if task == nil {
		return nil, false, nil
	}

	executeErr := s.executeImportTask(ctx, task)
	return task, true, executeErr
}

// importQuestions 执行同步题目导入，并复用给异步 worker，避免导入规则分叉。
func (s *scraperService) importQuestions(ctx context.Context, req scraper.ImportRequest) (*scraper.ImportResult, error) {
	// 查找行业
	industry, err := s.industryRepo.GetByCode(ctx, req.IndustryCode)
	if err != nil {
		return nil, err
	}
	if industry == nil {
		return nil, common.NewBusinessError(common.CodeNotFound, "行业不存在")
	}

	// 获取所有分类
	categories, err := s.categoryRepo.List(ctx)
	if err != nil {
		return nil, err
	}

	// 构建分类名称到ID的映射
	categoryMap := make(map[string]uint)
	for _, cat := range categories {
		categoryMap[cat.Name] = cat.ID
		// 也支持小写匹配
		categoryMap[strings.ToLower(cat.Name)] = cat.ID
	}

	result := &scraper.ImportResult{
		TotalCount: len(req.Questions),
		Errors:     make([]string, 0),
	}

	var questionsToImport []model.Question

	for i, q := range req.Questions {
		// 查找分类ID
		categoryID, exists := categoryMap[q.Category]
		if !exists {
			// 尝试模糊匹配
			found := false
			for catName, id := range categoryMap {
				if strings.Contains(strings.ToLower(catName), strings.ToLower(q.Category)) ||
					strings.Contains(strings.ToLower(q.Category), strings.ToLower(catName)) {
					categoryID = id
					found = true
					break
				}
			}
			if !found {
				result.FailCount++
				result.Errors = append(result.Errors, fmt.Sprintf("第%d题: 分类'%s'不存在", i+1, q.Category))
				continue
			}
		}

		// 验证题目类型
		validTypes := map[string]bool{
			model.QuestionTypeChoice:     true,
			model.QuestionTypeMulti:      true,
			model.QuestionTypeCode:       true,
			model.QuestionTypeSubjective: true,
		}
		if !validTypes[q.Type] {
			q.Type = model.QuestionTypeSubjective // 默认主观题
		}

		// 验证难度
		validDifficulties := map[string]bool{
			model.QuestionDifficultyEasy:   true,
			model.QuestionDifficultyMedium: true,
			model.QuestionDifficultyHard:   true,
		}
		if !validDifficulties[q.Difficulty] {
			q.Difficulty = model.QuestionDifficultyMedium // 默认中等
		}

		question := model.Question{
			CategoryID:  categoryID,
			IndustryID:  industry.ID,
			Type:        q.Type,
			Difficulty:  q.Difficulty,
			Title:       q.Title,
			Content:     q.Content,
			Answer:      q.Answer,
			Explanation: q.Explanation,
			Tags:        strings.Join(q.Tags, ","),
			IsActive:    true,
		}

		questionsToImport = append(questionsToImport, question)
	}

	// 批量创建题目
	if len(questionsToImport) > 0 {
		if err := s.questionRepo.BatchCreate(ctx, questionsToImport); err != nil {
			return nil, err
		}
		result.SuccessCount = len(questionsToImport)
	}

	return result, nil
}

// executeImportTask 执行已领取的异步导入任务，并把最终状态、结果摘要和错误信息写回任务表。
func (s *scraperService) executeImportTask(ctx context.Context, task *model.ScraperTask) error {
	req, err := decodeScraperImportTaskPayload(task.PayloadJSON)
	if err != nil {
		return s.failScraperTask(ctx, task, fmt.Errorf("解析导入任务载荷失败: %w", err))
	}

	result, runErr := s.importQuestions(ctx, req)
	if runErr != nil {
		return s.failScraperTask(ctx, task, runErr)
	}

	resultJSON, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		return s.failScraperTask(ctx, task, fmt.Errorf("序列化导入任务结果失败: %w", marshalErr))
	}

	now := time.Now()
	task.Status = scraper.TaskStatusImported
	task.FinishedAt = &now
	task.ResultJSON = string(resultJSON)
	task.ImportedCount = result.SuccessCount
	task.QuestionCount = result.TotalCount
	task.ErrorMsg = strings.Join(result.Errors, "\n")
	if err := s.scraperRepo.Update(ctx, task); err != nil {
		return err
	}
	return nil
}

// failScraperTask 将执行失败的任务写回失败状态，确保后台任务页可以直接看到失败原因。
func (s *scraperService) failScraperTask(ctx context.Context, task *model.ScraperTask, taskErr error) error {
	now := time.Now()
	task.Status = scraper.TaskStatusFailed
	task.FinishedAt = &now
	task.ErrorMsg = taskErr.Error()
	if err := s.scraperRepo.Update(ctx, task); err != nil {
		return err
	}
	return taskErr
}

// buildScraperImportTaskPayload 序列化异步导入任务载荷，并在入队前完成最基本的字段校验。
func buildScraperImportTaskPayload(req scraper.ImportRequest) (string, error) {
	if strings.TrimSpace(req.IndustryCode) == "" {
		return "", common.NewBusinessError(common.CodeBadRequest, "行业编码不能为空")
	}
	if len(req.Questions) == 0 {
		return "", common.NewBusinessError(common.CodeBadRequest, "至少需要一题才能创建导入任务")
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("序列化导入任务载荷失败: %w", err)
	}
	return string(payload), nil
}

// decodeScraperImportTaskPayload 反序列化异步导入任务载荷，供 worker 执行时恢复原始请求。
func decodeScraperImportTaskPayload(raw string) (scraper.ImportRequest, error) {
	var req scraper.ImportRequest
	if strings.TrimSpace(raw) == "" {
		return req, fmt.Errorf("empty payload")
	}
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		return req, err
	}
	return req, nil
}

// resolveScraperTaskSource 结合来源 URL 推断任务来源，便于后台任务页后续按来源聚合查看。
func resolveScraperTaskSource(sourceURL string) string {
	parsedURL, err := url.Parse(strings.TrimSpace(sourceURL))
	if err == nil {
		host := strings.ToLower(parsedURL.Host)
		switch {
		case strings.Contains(host, "niuke"), strings.Contains(host, "nowcoder"):
			return scraper.SourceNiuke
		case strings.Contains(host, "leetcode"):
			return scraper.SourceLeetCode
		case strings.Contains(host, "juejin"):
			return scraper.SourceJuejin
		}
	}

	return "manual"
}

// ListTasks 获取爬取任务列表
// normalizeScraperTaskListFilter 统一裁剪抓取任务筛选条件，避免空白字符造成筛选失效。
func normalizeScraperTaskListFilter(filter scraper.TaskListFilter) scraper.TaskListFilter {
	return scraper.TaskListFilter{
		Status:   strings.TrimSpace(filter.Status),
		TaskType: strings.TrimSpace(filter.TaskType),
	}
}

// buildScraperTaskDetail 将任务模型转换为后台详情 DTO，显式暴露异步载荷与执行结果，方便排障。
func buildScraperTaskDetail(task *model.ScraperTask) *scraper.TaskDetail {
	if task == nil {
		return nil
	}
	return &scraper.TaskDetail{
		ID:            task.ID,
		CreatedAt:     task.CreatedAt,
		UpdatedAt:     task.UpdatedAt,
		TaskType:      task.TaskType,
		SourceURL:     task.SourceURL,
		SourceTitle:   task.SourceTitle,
		Source:        task.Source,
		Status:        task.Status,
		QuestionCount: task.QuestionCount,
		ImportedCount: task.ImportedCount,
		RetryCount:    task.RetryCount,
		StartedAt:     task.StartedAt,
		FinishedAt:    task.FinishedAt,
		ErrorMsg:      task.ErrorMsg,
		PayloadJSON:   task.PayloadJSON,
		ResultJSON:    task.ResultJSON,
	}
}

// ListTasks 按分页和筛选条件返回抓取任务列表，供后台运行态任务页统一查看。
func (s *scraperService) ListTasks(ctx context.Context, page, pageSize int, filter scraper.TaskListFilter) (*common.PageResult, error) {
	pageParam := common.PageParam{Page: page, PageSize: pageSize}
	pageParam.Normalize()

	tasks, total, err := s.scraperRepo.List(ctx, pageParam.Page, pageParam.PageSize, normalizeScraperTaskListFilter(filter))
	if err != nil {
		return nil, err
	}

	return common.NewPageResult(tasks, total, pageParam), nil
}

// GetTask 按任务 ID 读取单条任务详情，供后台任务页查看异步导入结果与失败原因。
func (s *scraperService) GetTask(ctx context.Context, taskID uint) (*scraper.TaskDetail, error) {
	task, err := s.scraperRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, common.NewBusinessError(common.CodeNotFound, "任务不存在")
	}
	return buildScraperTaskDetail(task), nil
}

// isRetryableAsyncTaskType 判断当前任务类型是否允许在后台直接重试，避免把一次性同步留痕任务错误重投。
func isRetryableAsyncTaskType(taskType string) bool {
	switch strings.TrimSpace(taskType) {
	case scraper.TaskTypeImportQuestions, scraper.TaskTypeQuestionPipelineBuild:
		return true
	default:
		return false
	}
}

// RetryTask 将失败的异步任务重置为 pending，让 worker 可以重新消费。
func (s *scraperService) RetryTask(ctx context.Context, taskID uint) (*model.ScraperTask, error) {
	task, err := s.scraperRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, common.NewBusinessError(common.CodeNotFound, "任务不存在")
	}
	if !isRetryableAsyncTaskType(task.TaskType) {
		return nil, common.NewBusinessError(common.CodeBadRequest, "当前任务类型不支持直接重试")
	}
	if task.Status != scraper.TaskStatusFailed {
		return nil, common.NewBusinessError(common.CodeBadRequest, "只有失败任务才允许重试")
	}

	task.Status = scraper.TaskStatusPending
	task.StartedAt = nil
	task.FinishedAt = nil
	task.ResultJSON = ""
	task.ImportedCount = 0
	task.ErrorMsg = ""
	if err := s.scraperRepo.Update(ctx, task); err != nil {
		return nil, err
	}
	return task, nil
}

// GetSources 获取支持的数据源列表
func (s *scraperService) GetSources(ctx context.Context) ([]scraper.Source, error) {
	return s.provider.GetSupportedSources(), nil
}
