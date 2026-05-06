// Package service 提供业务逻辑层实现
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"makejob-backend/internal/ai"
	"makejob-backend/internal/common"
	"makejob-backend/internal/model"
	"makejob-backend/internal/repository"
)

// SubmitAnswerRequest 提交答案请求DTO
type SubmitAnswerRequest struct {
	Answer    string `json:"answer" binding:"required"`
	TimeSpent int    `json:"time_spent"` // 答题用时(秒)
}

// SubmitAnswerResponse 提交答案响应DTO
type SubmitAnswerResponse struct {
	IsCorrect     bool   `json:"is_correct"`
	CorrectAnswer string `json:"correct_answer"`
	Explanation   string `json:"explanation"`
	AIAnalysis    string `json:"ai_analysis,omitempty"`
}

// PracticeRecommendationItem 表示一条对症练习推荐结果。
type PracticeRecommendationItem struct {
	Question            QuestionDetail `json:"question"`
	FocusTag            string         `json:"focus_tag"`
	TopicCode           string         `json:"topic_code,omitempty"`
	TopicTitle          string         `json:"topic_title,omitempty"`
	TopicProblemPattern string         `json:"topic_problem_pattern,omitempty"`
	RelatedQuestionSets []string       `json:"related_question_sets"`
	RecommendedActions  []string       `json:"recommended_actions"`
	PrimaryQuestionSet  string         `json:"primary_question_set,omitempty"`
	RecommendationMode  string         `json:"recommendation_mode"`
	Reason              string         `json:"reason"`
	SourceType          string         `json:"source_type"`
	Priority            int            `json:"priority"`
	OccurrenceCount     int            `json:"occurrence_count"`
	PriorityExplanation string         `json:"priority_explanation"`
}

// PracticeRecommendationResponse 表示当前用户基于学习档案生成的推荐题单。
type PracticeRecommendationResponse struct {
	FocusTags []string                     `json:"focus_tags"`
	Items     []PracticeRecommendationItem `json:"items"`
}

// practiceFocusTagStat 描述学习档案中某个高频错因标签的统计结果。
type practiceFocusTagStat struct {
	Tag       string
	Count     int
	TopicCode string
}

// QuestionDetail 题目详情DTO
type QuestionDetail struct {
	model.Question
	IsFavorited    bool                        `json:"is_favorited"`
	UserNote       *model.UserNote             `json:"user_note,omitempty"`
	TagList        []string                    `json:"tag_list"`
	Solution       *QuestionStructuredSolution `json:"solution,omitempty"`
	AnswerTemplate *QuestionAnswerTemplate     `json:"answer_template,omitempty"`
}

// RandomExamRequest 随机组卷请求DTO
type RandomExamRequest struct {
	IndustryID *uint  `json:"industry_id"`
	CategoryID *uint  `json:"category_id"`
	Difficulty string `json:"difficulty"`
	Count      int    `json:"count" binding:"required,min=1,max=100"`
}

// TimedExamRequest 限时模拟请求DTO
type TimedExamRequest struct {
	IndustryID       *uint  `json:"industry_id"`
	CategoryID       *uint  `json:"category_id"`
	Difficulty       string `json:"difficulty"`
	Count            int    `json:"count" binding:"required,min=1,max=100"`
	TimeLimitMinutes int    `json:"time_limit_minutes" binding:"required,min=1,max=180"`
}

// ExamResponse 试卷响应DTO
type ExamResponse struct {
	ExamID    string           `json:"exam_id"`
	Questions []QuestionDetail `json:"questions"`
	TimeLimit int              `json:"time_limit"` // 分钟
}

// ExamAnswer 试卷答案DTO
type ExamAnswer struct {
	QuestionID uint   `json:"question_id" binding:"required"`
	Answer     string `json:"answer"`
	TimeSpent  int    `json:"time_spent"`
}

// SubmitExamRequest 提交试卷请求DTO
type SubmitExamRequest struct {
	ExamID  string       `json:"exam_id" binding:"required"`
	Answers []ExamAnswer `json:"answers" binding:"required"`
}

// AnswerDetail 答案详情DTO
type AnswerDetail struct {
	QuestionID    uint   `json:"question_id"`
	IsCorrect     bool   `json:"is_correct"`
	UserAnswer    string `json:"user_answer"`
	CorrectAnswer string `json:"correct_answer"`
	Explanation   string `json:"explanation"`
}

// ExamResult 考试结果DTO
type ExamResult struct {
	TotalScore   float64        `json:"total_score"`
	CorrectCount int            `json:"correct_count"`
	TotalCount   int            `json:"total_count"`
	Details      []AnswerDetail `json:"details"`
}

// CreateNoteRequest 创建笔记请求DTO
type CreateNoteRequest struct {
	QuestionID *uint  `json:"question_id"`
	Title      string `json:"title" binding:"required,max=200"`
	Content    string `json:"content" binding:"required"`
}

// UpdateNoteRequest 更新笔记请求DTO
type UpdateNoteRequest struct {
	Title   string `json:"title,omitempty" binding:"omitempty,max=200"`
	Content string `json:"content,omitempty"`
}

// CategoryTree 分类树DTO (从repository导入)
type CategoryTree = repository.CategoryTree

// UserPracticeStats 用户练习统计 (从repository导入)
type UserPracticeStats = repository.UserPracticeStats

// QuestionService 题目服务接口
type QuestionService interface {
	// 题目
	ListQuestions(ctx context.Context, params repository.QuestionListParams) (*common.PageResult, error)
	GetQuestion(ctx context.Context, id uint, userID uint) (*QuestionDetail, error)
	ListIndustries(ctx context.Context) ([]model.Industry, error)
	GetCategories(ctx context.Context, industryID uint, industryCode string) ([]CategoryTree, error)

	// 答题
	SubmitAnswer(ctx context.Context, userID, questionID uint, req *SubmitAnswerRequest) (*SubmitAnswerResponse, error)

	// 收藏
	ToggleFavorite(ctx context.Context, userID, questionID uint) (bool, error)
	GetFavorites(ctx context.Context, userID uint, page, pageSize int) (*common.PageResult, error)

	// 错题本
	GetWrongQuestions(ctx context.Context, userID uint, page, pageSize int) (*common.PageResult, error)

	// 笔记
	CreateNote(ctx context.Context, userID uint, req *CreateNoteRequest) (*model.UserNote, error)
	UpdateNote(ctx context.Context, userID, noteID uint, req *UpdateNoteRequest) error
	DeleteNote(ctx context.Context, userID, noteID uint) error
	ListNotes(ctx context.Context, userID uint, page, pageSize int) (*common.PageResult, error)

	// 组卷与模拟
	GenerateRandomExam(ctx context.Context, userID uint, req *RandomExamRequest) (*ExamResponse, error)
	GenerateTimedExam(ctx context.Context, userID uint, req *TimedExamRequest) (*ExamResponse, error)
	SubmitExam(ctx context.Context, userID uint, req *SubmitExamRequest) (*ExamResult, error)

	// 统计
	GetPracticeStats(ctx context.Context, userID uint) (*UserPracticeStats, error)
	GetPracticeRecommendations(ctx context.Context, userID uint, interviewID *uint, limit int) (*PracticeRecommendationResponse, error)
	ListQuestionSets(ctx context.Context, industryID uint) ([]QuestionSetSummary, error)
	GetQuestionSetDetail(ctx context.Context, industryID uint, slug string) (*QuestionSetDetail, error)
	ListMistakeTopics(ctx context.Context, codes []string) ([]MistakeTopicCard, error)
	GetMistakeTopic(ctx context.Context, code string) (*MistakeTopicCard, error)
}

// questionService 题目服务实现
type questionService struct {
	questionRepo        QuestionRepository
	categoryRepo        CategoryRepository
	recordRepo          QuestionRecordRepository
	favoriteRepo        FavoriteRepository
	noteRepo            NoteRepository
	quizAnalyzer        ai.QuizAnalyzer
	learningArchiveRepo repository.LearningArchiveRepository
	industryRepo        repository.IndustryRepository
}

// QuestionRepository 题目仓库接口 (本地定义，避免循环导入)
type QuestionRepository interface {
	List(ctx context.Context, params repository.QuestionListParams) ([]model.Question, int64, error)
	GetByID(ctx context.Context, id uint) (*model.Question, error)
	GetByIDs(ctx context.Context, ids []uint) ([]model.Question, error)
	GetRandomByParams(ctx context.Context, params repository.RandomQuestionParams) ([]model.Question, error)
}

// CategoryRepository 分类仓库接口
type CategoryRepository interface {
	List(ctx context.Context, industryID uint) ([]model.Category, error)
	GetByID(ctx context.Context, id uint) (*model.Category, error)
	GetTree(ctx context.Context, industryID uint) ([]CategoryTree, error)
}

// QuestionRecordRepository 答题记录仓库接口
type QuestionRecordRepository interface {
	Create(ctx context.Context, record *model.UserQuestionRecord) error
	GetByUserAndQuestion(ctx context.Context, userID, questionID uint) ([]model.UserQuestionRecord, error)
	GetWrongQuestions(ctx context.Context, userID uint, page, pageSize int) ([]model.UserQuestionRecord, int64, error)
	GetUserStats(ctx context.Context, userID uint) (*UserPracticeStats, error)
	GetDailyCount(ctx context.Context, userID uint, date time.Time) (int64, error)
}

// FavoriteRepository 收藏仓库接口
type FavoriteRepository interface {
	Create(ctx context.Context, favorite *model.UserFavorite) error
	Delete(ctx context.Context, userID, questionID uint) error
	Exists(ctx context.Context, userID, questionID uint) (bool, error)
	ListByUser(ctx context.Context, userID uint, page, pageSize int) ([]model.UserFavorite, int64, error)
}

// NoteRepository 笔记仓库接口
type NoteRepository interface {
	Create(ctx context.Context, note *model.UserNote) error
	GetByID(ctx context.Context, id uint) (*model.UserNote, error)
	Update(ctx context.Context, note *model.UserNote) error
	Delete(ctx context.Context, id, userID uint) error
	ListByUser(ctx context.Context, userID uint, page, pageSize int) ([]model.UserNote, int64, error)
	GetByUserAndQuestion(ctx context.Context, userID, questionID uint) (*model.UserNote, error)
}

// 内存存储考试会话
var (
	examSessions = sync.Map{}
)

// ExamSession 考试会话
type ExamSession struct {
	ExamID      string
	UserID      uint
	QuestionIDs []uint
	TimeLimit   int // 分钟
	CreatedAt   time.Time
	IndustryID  uint
}

// NewQuestionService 创建题目服务实例
func NewQuestionService(
	questionRepo repository.QuestionRepository,
	categoryRepo repository.CategoryRepository,
	recordRepo repository.QuestionRecordRepository,
	favoriteRepo repository.FavoriteRepository,
	noteRepo repository.NoteRepository,
	quizAnalyzer ai.QuizAnalyzer,
	learningArchiveRepo repository.LearningArchiveRepository,
	industryRepo ...repository.IndustryRepository,
) QuestionService {
	s := &questionService{
		questionRepo:        questionRepo,
		categoryRepo:        categoryRepo,
		recordRepo:          recordRepo,
		favoriteRepo:        favoriteRepo,
		noteRepo:            noteRepo,
		quizAnalyzer:        quizAnalyzer,
		learningArchiveRepo: learningArchiveRepo,
	}
	if len(industryRepo) > 0 {
		s.industryRepo = industryRepo[0]
	}
	return s
}

// ListQuestions 获取题目列表
func (s *questionService) ListQuestions(ctx context.Context, params repository.QuestionListParams) (*common.PageResult, error) {
	pageParam := common.PageParam{Page: params.Page, PageSize: params.PageSize}
	pageParam.Normalize()
	params.Page = pageParam.Page
	params.PageSize = pageParam.PageSize

	questions, total, err := s.questionRepo.List(ctx, params)
	if err != nil {
		return nil, err
	}

	return common.NewPageResult(questions, total, pageParam), nil
}

// GetQuestion 获取题目详情
func (s *questionService) GetQuestion(ctx context.Context, id uint, userID uint) (*QuestionDetail, error) {
	question, err := s.questionRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if question == nil {
		return nil, common.NewBusinessError(common.CodeNotFound, "题目不存在")
	}

	detail := &QuestionDetail{
		Question:       *question,
		IsFavorited:    false,
		UserNote:       nil,
		TagList:        parseQuestionTagsFromStorage(question.Tags),
		Solution:       parseQuestionStructuredSolution(question.SolutionJSON, question),
		AnswerTemplate: parseQuestionAnswerTemplate(question.AnswerTemplateJSON, question),
	}

	// 如果用户已登录，查询收藏状态和笔记
	if userID > 0 {
		isFavorited, _ := s.favoriteRepo.Exists(ctx, userID, id)
		detail.IsFavorited = isFavorited

		userNote, _ := s.noteRepo.GetByUserAndQuestion(ctx, userID, id)
		detail.UserNote = userNote
	}

	return detail, nil
}

// ListQuestionSets 返回当前行业下可直接消费的核心题单摘要。
func (s *questionService) ListQuestionSets(ctx context.Context, industryID uint) ([]QuestionSetSummary, error) {
	questions, _, err := s.questionRepo.List(ctx, repository.QuestionListParams{
		Page:       1,
		PageSize:   100,
		IndustryID: uintPointer(industryID),
		IsActive:   boolPointer(true),
	})
	if err != nil {
		return nil, err
	}

	return buildQuestionSetSummaries(questions), nil
}

// GetQuestionSetDetail 返回指定题单下可直接进入练习的完整题目集合。
func (s *questionService) GetQuestionSetDetail(ctx context.Context, industryID uint, slug string) (*QuestionSetDetail, error) {
	definition, ok := findQuestionSetDefinition(slug)
	if !ok {
		return nil, common.NewBusinessError(common.CodeNotFound, "题单不存在")
	}

	questions, _, err := s.questionRepo.List(ctx, repository.QuestionListParams{
		Page:       1,
		PageSize:   200,
		IndustryID: uintPointer(industryID),
		IsActive:   boolPointer(true),
	})
	if err != nil {
		return nil, err
	}

	detail := buildQuestionSetDetail(definition, questions)
	if detail == nil {
		return nil, common.NewBusinessError(common.CodeNotFound, "当前题单下暂无可用题目")
	}

	return detail, nil
}

// ListMistakeTopics 返回前台可展示的错因专题卡片列表。
func (s *questionService) ListMistakeTopics(ctx context.Context, codes []string) ([]MistakeTopicCard, error) {
	_ = ctx
	if len(codes) == 0 {
		return buildMistakeTopicCatalog(), nil
	}
	return listMistakeTopicsByCodes(codes), nil
}

// GetMistakeTopic 返回单个错因专题卡片详情。
func (s *questionService) GetMistakeTopic(ctx context.Context, code string) (*MistakeTopicCard, error) {
	_ = ctx
	topic, ok := resolveMistakeTopicByCode(code)
	if !ok {
		return nil, common.NewBusinessError(common.CodeNotFound, "错因专题不存在")
	}
	return topic, nil
}

// ListIndustries 获取前台可见的行业列表，仅返回已启用行业。
func (s *questionService) ListIndustries(ctx context.Context) ([]model.Industry, error) {
	if s.industryRepo == nil {
		return []model.Industry{}, nil
	}

	industries, err := s.industryRepo.List(ctx)
	if err != nil {
		return nil, err
	}

	activeIndustries := make([]model.Industry, 0, len(industries))
	for _, industry := range industries {
		if industry.IsActive {
			activeIndustries = append(activeIndustries, industry)
		}
	}

	return activeIndustries, nil
}

// GetCategories 获取分类列表。
func (s *questionService) GetCategories(ctx context.Context, industryID uint, industryCode string) ([]CategoryTree, error) {
	resolvedIndustryID, err := s.resolveCategoryIndustryID(ctx, industryID, industryCode)
	if err != nil {
		return nil, err
	}

	return s.categoryRepo.GetTree(ctx, resolvedIndustryID)
}

// resolveCategoryIndustryID 解析分类查询使用的行业ID，兼容行业编码筛选。
func (s *questionService) resolveCategoryIndustryID(ctx context.Context, industryID uint, industryCode string) (uint, error) {
	if industryID > 0 {
		return industryID, nil
	}

	if industryCode == "" || s.industryRepo == nil {
		return 0, nil
	}

	industry, err := s.industryRepo.GetByCode(ctx, industryCode)
	if err != nil {
		return 0, err
	}
	if industry == nil {
		return 0, common.NewBusinessError(common.CodeNotFound, "行业不存在")
	}

	return industry.ID, nil
}

// SubmitAnswer 提交答案
func (s *questionService) SubmitAnswer(ctx context.Context, userID, questionID uint, req *SubmitAnswerRequest) (*SubmitAnswerResponse, error) {
	// 获取题目
	question, err := s.questionRepo.GetByID(ctx, questionID)
	if err != nil {
		return nil, err
	}
	if question == nil {
		return nil, common.NewBusinessError(common.CodeNotFound, "题目不存在")
	}

	// 判分
	isCorrect := s.judgeAnswer(question, req.Answer)

	// 保存答题记录
	record := &model.UserQuestionRecord{
		UserID:     userID,
		QuestionID: questionID,
		UserAnswer: req.Answer,
		IsCorrect:  isCorrect,
		TimeSpent:  req.TimeSpent,
	}

	// 构建响应
	resp := &SubmitAnswerResponse{
		IsCorrect:     isCorrect,
		CorrectAnswer: question.Answer,
		Explanation:   question.Explanation,
	}

	// 对于编程题和主观题，调用AI分析
	if question.Type == model.QuestionTypeCode || question.Type == model.QuestionTypeSubjective {
		if s.quizAnalyzer != nil {
			analysis, err := s.quizAnalyzer.AnalyzeCode(ctx, req.Answer, detectQuestionLanguage(question), question.Content)
			if err == nil {
				resp.IsCorrect = analysis.IsCorrect
				analysisJSON, _ := json.Marshal(analysis)
				resp.AIAnalysis = string(analysisJSON)
				record.AnalysisJSON = resp.AIAnalysis
			}
		}
	}

	record.IsCorrect = resp.IsCorrect
	if err := s.recordRepo.Create(ctx, record); err != nil {
		return nil, err
	}

	if err := s.syncPracticeLearningArchive(ctx, userID, question, record, resp.AIAnalysis); err != nil {
		return nil, err
	}

	return resp, nil
}

// judgeAnswer 判分逻辑
func (s *questionService) judgeAnswer(question *model.Question, userAnswer string) bool {
	userAnswer = strings.TrimSpace(userAnswer)
	correctAnswer := strings.TrimSpace(question.Answer)

	switch question.Type {
	case model.QuestionTypeChoice:
		// 单选题：直接比较
		return strings.EqualFold(userAnswer, correctAnswer)
	case model.QuestionTypeMulti:
		// 多选题：排序后比较
		userChoices := splitAndSort(userAnswer)
		correctChoices := splitAndSort(correctAnswer)
		return strings.EqualFold(userChoices, correctChoices)
	case model.QuestionTypeCode, model.QuestionTypeSubjective:
		// 编程题和主观题：预留AI判分，当前返回false
		return false
	default:
		return false
	}
}

// splitAndSort 分割并排序答案选项
func splitAndSort(answer string) string {
	parts := strings.Split(answer, ",")
	var choices []string
	for _, p := range parts {
		p = strings.TrimSpace(strings.ToUpper(p))
		if p != "" {
			choices = append(choices, p)
		}
	}
	sort.Strings(choices)
	return strings.Join(choices, ",")
}

// ToggleFavorite 切换收藏状态
func (s *questionService) ToggleFavorite(ctx context.Context, userID, questionID uint) (bool, error) {
	// 检查题目是否存在
	question, err := s.questionRepo.GetByID(ctx, questionID)
	if err != nil {
		return false, err
	}
	if question == nil {
		return false, common.NewBusinessError(common.CodeNotFound, "题目不存在")
	}

	// 检查是否已收藏
	exists, err := s.favoriteRepo.Exists(ctx, userID, questionID)
	if err != nil {
		return false, err
	}

	if exists {
		// 取消收藏
		if err := s.favoriteRepo.Delete(ctx, userID, questionID); err != nil {
			return false, err
		}
		return false, nil
	}

	// 添加收藏
	favorite := &model.UserFavorite{
		UserID:     userID,
		QuestionID: questionID,
	}
	if err := s.favoriteRepo.Create(ctx, favorite); err != nil {
		return false, err
	}
	return true, nil
}

// GetFavorites 获取收藏列表
func (s *questionService) GetFavorites(ctx context.Context, userID uint, page, pageSize int) (*common.PageResult, error) {
	pageParam := common.PageParam{Page: page, PageSize: pageSize}
	pageParam.Normalize()

	favorites, total, err := s.favoriteRepo.ListByUser(ctx, userID, pageParam.Page, pageParam.PageSize)
	if err != nil {
		return nil, err
	}

	return common.NewPageResult(favorites, total, pageParam), nil
}

// GetWrongQuestions 获取错题列表
func (s *questionService) GetWrongQuestions(ctx context.Context, userID uint, page, pageSize int) (*common.PageResult, error) {
	pageParam := common.PageParam{Page: page, PageSize: pageSize}
	pageParam.Normalize()

	records, total, err := s.recordRepo.GetWrongQuestions(ctx, userID, pageParam.Page, pageParam.PageSize)
	if err != nil {
		return nil, err
	}

	return common.NewPageResult(records, total, pageParam), nil
}

// CreateNote 创建笔记
func (s *questionService) CreateNote(ctx context.Context, userID uint, req *CreateNoteRequest) (*model.UserNote, error) {
	note := &model.UserNote{
		UserID:     userID,
		QuestionID: req.QuestionID,
		Title:      req.Title,
		Content:    req.Content,
	}

	if err := s.noteRepo.Create(ctx, note); err != nil {
		return nil, err
	}

	return note, nil
}

// UpdateNote 更新笔记
func (s *questionService) UpdateNote(ctx context.Context, userID, noteID uint, req *UpdateNoteRequest) error {
	// 获取笔记
	note, err := s.noteRepo.GetByID(ctx, noteID)
	if err != nil {
		return err
	}
	if note == nil {
		return common.NewBusinessError(common.CodeNotFound, "笔记不存在")
	}

	// 检查权限
	if note.UserID != userID {
		return common.NewBusinessError(common.CodeForbidden, "无权修改此笔记")
	}

	// 更新字段
	if req.Title != "" {
		note.Title = req.Title
	}
	if req.Content != "" {
		note.Content = req.Content
	}

	return s.noteRepo.Update(ctx, note)
}

// DeleteNote 删除笔记
func (s *questionService) DeleteNote(ctx context.Context, userID, noteID uint) error {
	return s.noteRepo.Delete(ctx, noteID, userID)
}

// ListNotes 获取笔记列表
func (s *questionService) ListNotes(ctx context.Context, userID uint, page, pageSize int) (*common.PageResult, error) {
	pageParam := common.PageParam{Page: page, PageSize: pageSize}
	pageParam.Normalize()

	notes, total, err := s.noteRepo.ListByUser(ctx, userID, pageParam.Page, pageParam.PageSize)
	if err != nil {
		return nil, err
	}

	return common.NewPageResult(notes, total, pageParam), nil
}

// GenerateRandomExam 生成随机试卷
func (s *questionService) GenerateRandomExam(ctx context.Context, userID uint, req *RandomExamRequest) (*ExamResponse, error) {
	// 获取用户已做过的题目ID
	stats, err := s.recordRepo.GetUserStats(ctx, userID)
	if err != nil {
		stats = &UserPracticeStats{}
	}

	// 简化：不获取具体做过的题目ID，而是依赖随机算法
	_ = stats

	// 获取随机题目
	params := repository.RandomQuestionParams{
		IndustryID: s.resolveExamIndustryID(ctx, req.IndustryID, req.CategoryID),
		CategoryID: req.CategoryID,
		Difficulty: req.Difficulty,
		Count:      req.Count,
	}

	questions, err := s.questionRepo.GetRandomByParams(ctx, params)
	if err != nil {
		return nil, err
	}

	if len(questions) == 0 {
		return nil, common.NewBusinessError(common.CodeNotFound, "未找到符合条件的题目")
	}

	// 创建考试会话
	examID := generateExamID()
	questionIDs := make([]uint, len(questions))
	for i, q := range questions {
		questionIDs[i] = q.ID
	}

	session := &ExamSession{
		ExamID:      examID,
		UserID:      userID,
		QuestionIDs: questionIDs,
		TimeLimit:   0, // 随机组卷不限时
		CreatedAt:   time.Now(),
		IndustryID:  derefIndustryID(params.IndustryID),
	}
	examSessions.Store(examID, session)

	// 构建题目详情
	questionDetails := make([]QuestionDetail, len(questions))
	for i, q := range questions {
		detail, _ := s.GetQuestion(ctx, q.ID, userID)
		if detail != nil {
			questionDetails[i] = *detail
		} else {
			questionDetails[i] = QuestionDetail{Question: q}
		}
	}

	return &ExamResponse{
		ExamID:    examID,
		Questions: questionDetails,
		TimeLimit: 0,
	}, nil
}

// GenerateTimedExam 生成限时模拟试卷
func (s *questionService) GenerateTimedExam(ctx context.Context, userID uint, req *TimedExamRequest) (*ExamResponse, error) {
	// 获取随机题目
	params := repository.RandomQuestionParams{
		IndustryID: s.resolveExamIndustryID(ctx, req.IndustryID, req.CategoryID),
		CategoryID: req.CategoryID,
		Difficulty: req.Difficulty,
		Count:      req.Count,
	}

	questions, err := s.questionRepo.GetRandomByParams(ctx, params)
	if err != nil {
		return nil, err
	}

	if len(questions) == 0 {
		return nil, common.NewBusinessError(common.CodeNotFound, "未找到符合条件的题目")
	}

	// 创建考试会话
	examID := generateExamID()
	questionIDs := make([]uint, len(questions))
	for i, q := range questions {
		questionIDs[i] = q.ID
	}

	session := &ExamSession{
		ExamID:      examID,
		UserID:      userID,
		QuestionIDs: questionIDs,
		TimeLimit:   req.TimeLimitMinutes,
		CreatedAt:   time.Now(),
		IndustryID:  derefIndustryID(params.IndustryID),
	}
	examSessions.Store(examID, session)

	// 构建题目详情
	questionDetails := make([]QuestionDetail, len(questions))
	for i, q := range questions {
		detail, _ := s.GetQuestion(ctx, q.ID, userID)
		if detail != nil {
			questionDetails[i] = *detail
		} else {
			questionDetails[i] = QuestionDetail{Question: q}
		}
	}

	return &ExamResponse{
		ExamID:    examID,
		Questions: questionDetails,
		TimeLimit: req.TimeLimitMinutes,
	}, nil
}

// resolveExamIndustryID 解析组卷所需的行业 ID，优先按分类归属推导，未指定分类时回退到显式行业筛选。
func (s *questionService) resolveExamIndustryID(ctx context.Context, industryID, categoryID *uint) *uint {
	if categoryID == nil || *categoryID == 0 || s.categoryRepo == nil {
		if industryID == nil || *industryID == 0 {
			return nil
		}
		return industryID
	}

	category, err := s.categoryRepo.GetByID(ctx, *categoryID)
	if err != nil || category == nil {
		return nil
	}

	resolvedIndustryID := category.IndustryID
	return &resolvedIndustryID
}

// derefIndustryID 将可选行业 ID 指针转换为可存储的数值，空指针时返回 0。
func derefIndustryID(industryID *uint) uint {
	if industryID == nil {
		return 0
	}
	return *industryID
}

// generateExamID 生成考试ID
func generateExamID() string {
	return fmt.Sprintf("exam_%d_%d", time.Now().Unix(), time.Now().Nanosecond())
}

// SubmitExam 提交试卷
func (s *questionService) SubmitExam(ctx context.Context, userID uint, req *SubmitExamRequest) (*ExamResult, error) {
	// 获取考试会话
	sessionValue, exists := examSessions.Load(req.ExamID)
	if !exists {
		return nil, common.NewBusinessError(common.CodeNotFound, "考试不存在或已过期")
	}
	session := sessionValue.(*ExamSession)

	// 验证用户
	if session.UserID != userID {
		return nil, common.NewBusinessError(common.CodeForbidden, "无权提交此考试")
	}

	// 获取题目信息
	questions, err := s.questionRepo.GetByIDs(ctx, session.QuestionIDs)
	if err != nil {
		return nil, err
	}

	questionMap := make(map[uint]*model.Question)
	for i := range questions {
		questionMap[questions[i].ID] = &questions[i]
	}

	// 处理答案
	answerMap := make(map[uint]ExamAnswer)
	for _, ans := range req.Answers {
		answerMap[ans.QuestionID] = ans
	}

	// 判分
	result := &ExamResult{
		TotalCount: len(session.QuestionIDs),
		Details:    []AnswerDetail{},
	}

	for _, qid := range session.QuestionIDs {
		question, exists := questionMap[qid]
		if !exists {
			continue
		}

		ans, answered := answerMap[qid]
		userAnswer := ""
		timeSpent := 0
		if answered {
			userAnswer = ans.Answer
			timeSpent = ans.TimeSpent
		}

		isCorrect := s.judgeAnswer(question, userAnswer)
		if isCorrect {
			result.CorrectCount++
		}

		// 保存答题记录
		record := &model.UserQuestionRecord{
			UserID:     userID,
			QuestionID: qid,
			UserAnswer: userAnswer,
			IsCorrect:  isCorrect,
			TimeSpent:  timeSpent,
		}
		s.recordRepo.Create(ctx, record)

		result.Details = append(result.Details, AnswerDetail{
			QuestionID:    qid,
			IsCorrect:     isCorrect,
			UserAnswer:    userAnswer,
			CorrectAnswer: question.Answer,
			Explanation:   question.Explanation,
		})
	}

	// 计算总分
	if result.TotalCount > 0 {
		result.TotalScore = float64(result.CorrectCount) / float64(result.TotalCount) * 100
	}

	// 清理会话
	examSessions.Delete(req.ExamID)

	return result, nil
}

// GetPracticeStats 获取练习统计
func (s *questionService) GetPracticeStats(ctx context.Context, userID uint) (*UserPracticeStats, error) {
	return s.recordRepo.GetUserStats(ctx, userID)
}

// GetPracticeRecommendations 基于最近学习档案中的错因标签返回一组可直接补练的题目。
func (s *questionService) GetPracticeRecommendations(ctx context.Context, userID uint, interviewID *uint, limit int) (*PracticeRecommendationResponse, error) {
	if s.learningArchiveRepo == nil {
		return &PracticeRecommendationResponse{
			FocusTags: []string{},
			Items:     []PracticeRecommendationItem{},
		}, nil
	}

	if limit <= 0 {
		limit = 6
	}

	entries, err := s.learningArchiveRepo.ListRecentByUser(ctx, userID, 20, interviewID)
	if err != nil {
		return nil, err
	}

	focusSignals := buildTrainingFocusSignals(entries, nil, defaultTrainingFocusSignalLimit)
	if len(focusSignals) == 0 {
		return &PracticeRecommendationResponse{
			FocusTags: []string{},
			Items:     []PracticeRecommendationItem{},
		}, nil
	}

	items := make([]PracticeRecommendationItem, 0, limit)
	seenQuestionIDs := make(map[uint]struct{}, limit)
	focusTags := make([]string, 0, len(focusSignals))
	sourceType := "learning_archive"
	if interviewID != nil && *interviewID > 0 {
		sourceType = "interview_archive"
	}
	for index, focusSignal := range focusSignals {
		focusTags = append(focusTags, focusSignal.Tag)
		recommendationMode := buildPracticeRecommendationMode(focusSignal)
		for _, keyword := range expandPracticeFocusTagKeywords(focusSignal.Tag) {
			questions, _, err := s.questionRepo.List(ctx, repository.QuestionListParams{
				Page:     1,
				PageSize: limit * 2,
				Keyword:  keyword,
				IsActive: boolPointer(true),
				Type:     model.QuestionTypeCode,
			})
			if err != nil {
				return nil, err
			}

			for _, question := range questions {
				if len(items) >= limit {
					break
				}
				if _, exists := seenQuestionIDs[question.ID]; exists {
					continue
				}
				seenQuestionIDs[question.ID] = struct{}{}
				items = append(items, PracticeRecommendationItem{
					Question: QuestionDetail{
						Question:       question,
						IsFavorited:    false,
						UserNote:       nil,
						TagList:        parseQuestionTagsFromStorage(question.Tags),
						Solution:       parseQuestionStructuredSolution(question.SolutionJSON, &question),
						AnswerTemplate: parseQuestionAnswerTemplate(question.AnswerTemplateJSON, &question),
					},
					FocusTag:            focusSignal.Tag,
					TopicCode:           focusSignal.TopicCode,
					TopicTitle:          focusSignal.TopicTitle,
					TopicProblemPattern: focusSignal.TopicProblemPattern,
					RelatedQuestionSets: append([]string(nil), focusSignal.RelatedQuestionSets...),
					RecommendedActions:  append([]string(nil), focusSignal.RecommendedActions...),
					PrimaryQuestionSet:  focusSignal.PrimaryQuestionSet,
					RecommendationMode:  recommendationMode,
					Reason:              buildPracticeRecommendationReason(focusSignal.Tag, focusSignal.OccurrenceCount, sourceType),
					SourceType:          sourceType,
					Priority:            index + 1,
					OccurrenceCount:     focusSignal.OccurrenceCount,
					PriorityExplanation: buildPracticeRecommendationPriorityExplanation(focusSignal, recommendationMode, sourceType),
				})
			}
			if len(items) >= limit {
				break
			}
		}
		if len(items) >= limit {
			break
		}
	}

	return &PracticeRecommendationResponse{
		FocusTags: focusTags,
		Items:     items,
	}, nil
}

// buildPracticeRecommendationMode 根据训练重点信号判断推荐结果更适合以哪种方式承接。
func buildPracticeRecommendationMode(signal trainingFocusSignal) string {
	if strings.TrimSpace(signal.PrimaryQuestionSet) != "" {
		return "question_set"
	}
	if strings.TrimSpace(signal.TopicCode) != "" || strings.TrimSpace(signal.TopicTitle) != "" {
		return "topic"
	}
	return "keyword"
}

// buildPracticeRecommendationPriorityExplanation 生成推荐结果在当前轮次中应如何消费的优先级说明。
func buildPracticeRecommendationPriorityExplanation(signal trainingFocusSignal, mode string, sourceType string) string {
	switch mode {
	case "question_set":
		return fmt.Sprintf("这个推荐优先级较高，建议先围绕题单“%s”做一轮集中补练，快速处理“%s”反复出现的问题。", signal.PrimaryQuestionSet, signal.Tag)
	case "topic":
		return fmt.Sprintf("这个推荐适合作为当前专题补练入口，先通过单题把“%s”对应的方法链路补齐。", signal.Tag)
	default:
		if sourceType == "interview_archive" {
			return "这个推荐来自单场面试后的即时补练，建议在记忆还新鲜时尽快完成。"
		}
		return fmt.Sprintf("这个推荐用于快速验证你是否已经修正“%s”这一类高频问题。", signal.Tag)
	}
}

// syncPracticeLearningArchive 将代码题或主观题的分析结果同步到学习档案中。
func (s *questionService) syncPracticeLearningArchive(
	ctx context.Context,
	userID uint,
	question *model.Question,
	record *model.UserQuestionRecord,
	analysisJSON string,
) error {
	if s.learningArchiveRepo == nil || question == nil || record == nil || strings.TrimSpace(analysisJSON) == "" {
		return nil
	}

	var analysis ai.CodeAnalysis
	if err := json.Unmarshal([]byte(analysisJSON), &analysis); err != nil {
		return nil
	}

	mistakeTags := normalizePracticeArchiveTags(analysis.MistakeTags, analysis.Issues, analysis.IsCorrect)
	if len(mistakeTags) == 0 {
		return nil
	}

	strengthTagsJSON, _ := json.Marshal(normalizePracticeArchiveStrengths(analysis.StrengthTags, analysis.IsCorrect))
	mistakeTagsJSON, _ := json.Marshal(mistakeTags)
	suggestionsJSON, _ := json.Marshal(normalizePracticeArchiveSuggestions(analysis.Improvements))
	sourceRef := fmt.Sprintf("practice:%d:%d", userID, question.ID)

	entry := &model.LearningArchiveEntry{
		UserID:           userID,
		SourceType:       model.LearningArchiveSourcePracticeQuestion,
		SourceRef:        sourceRef,
		QuestionIndex:    0,
		IndustryCode:     strconv.FormatUint(uint64(question.IndustryID), 10),
		Language:         detectQuestionLanguage(question),
		MistakeTagsJSON:  string(mistakeTagsJSON),
		StrengthTagsJSON: string(strengthTagsJSON),
		SuggestionsJSON:  string(suggestionsJSON),
		EvidenceSummary:  strings.Join(analysis.Issues, "；"),
		OccurredAt:       buildPracticeArchiveOccurredAt(record.CreatedAt),
	}
	return s.learningArchiveRepo.Upsert(ctx, entry)
}

// rankPracticeFocusTags 统计最近学习档案中的高频错因标签。
func rankPracticeFocusTags(entries []model.LearningArchiveEntry) []practiceFocusTagStat {
	counts := make(map[string]int)
	for _, entry := range entries {
		var tags []string
		if err := json.Unmarshal([]byte(entry.MistakeTagsJSON), &tags); err != nil {
			continue
		}
		for _, tag := range tags {
			trimmed := strings.TrimSpace(tag)
			if trimmed == "" {
				continue
			}
			counts[trimmed]++
		}
	}

	items := make([]practiceFocusTagStat, 0, len(counts))
	for tag, count := range counts {
		items = append(items, practiceFocusTagStat{
			Tag:       tag,
			Count:     count,
			TopicCode: resolveMistakeTopicCodeByTag(tag),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].Tag < items[j].Tag
		}
		return items[i].Count > items[j].Count
	})

	result := make([]practiceFocusTagStat, 0, minInt(len(items), 3))
	for _, item := range items {
		if len(result) >= 3 {
			break
		}
		result = append(result, item)
	}
	return result
}

// buildPracticeRecommendationReason 生成更可解释的推荐理由文案。
func buildPracticeRecommendationReason(focusTag string, occurrenceCount int, sourceType string) string {
	if sourceType == "interview_archive" {
		return fmt.Sprintf("这场面试里“%s”相关问题反复出现 %d 次，建议优先补这类题。", focusTag, occurrenceCount)
	}

	return fmt.Sprintf("你最近的学习档案里“%s”累计出现 %d 次，先用这题做对症补练。", focusTag, occurrenceCount)
}

// expandPracticeFocusTagKeywords 将错因标签扩展为更适合题库检索的关键词集合。
func expandPracticeFocusTagKeywords(tag string) []string {
	switch strings.TrimSpace(tag) {
	case "状态定义不清":
		return []string{tag, "状态", "动态规划"}
	case "边界条件生疏":
		return []string{tag, "边界", "数组"}
	case "循环/索引控制不稳":
		return []string{tag, "循环", "索引", "双指针"}
	case "数据结构选择不当":
		return []string{tag, "数据结构", "链表", "栈", "哈希"}
	case "复杂度意识薄弱":
		return []string{tag, "复杂度", "性能"}
	case "调试路径混乱":
		return []string{tag, "调试", "排错"}
	case "代码实现不完整":
		return []string{tag, "实现", "代码"}
	default:
		return []string{tag}
	}
}

// normalizePracticeArchiveTags 统一收敛练习题分析得到的错因标签。
func normalizePracticeArchiveTags(tags []string, issues []string, isCorrect bool) []string {
	if isCorrect {
		return []string{}
	}

	result := make([]string, 0, len(tags)+1)
	for _, tag := range tags {
		trimmed := strings.TrimSpace(tag)
		if trimmed == "" {
			continue
		}
		result = appendUniquePracticeStrings(result, trimmed)
	}
	if len(result) > 0 {
		return result
	}

	for _, issue := range issues {
		switch {
		case strings.Contains(issue, "边界"):
			result = appendUniquePracticeStrings(result, "边界条件生疏")
		case strings.Contains(issue, "复杂度"):
			result = appendUniquePracticeStrings(result, "复杂度意识薄弱")
		case strings.Contains(issue, "思路") || strings.Contains(issue, "实现"):
			result = appendUniquePracticeStrings(result, "状态定义不清")
		}
	}
	if len(result) == 0 {
		result = append(result, "状态定义不清")
	}
	return result
}

// normalizePracticeArchiveStrengths 统一收敛练习题分析中的正向标签。
func normalizePracticeArchiveStrengths(tags []string, isCorrect bool) []string {
	result := make([]string, 0, len(tags)+1)
	for _, tag := range tags {
		result = appendUniquePracticeStrings(result, tag)
	}
	if isCorrect {
		result = appendUniquePracticeStrings(result, "本题已形成可用解法")
	}
	return result
}

// normalizePracticeArchiveSuggestions 清理练习题分析中的改进建议。
func normalizePracticeArchiveSuggestions(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = appendUniquePracticeStrings(result, value)
	}
	return result
}

// buildPracticeArchiveOccurredAt 将 Unix 秒级时间戳转为 time 指针。
func buildPracticeArchiveOccurredAt(createdAt int64) *time.Time {
	if createdAt <= 0 {
		return nil
	}
	value := time.Unix(createdAt, 0)
	return &value
}

// appendUniquePracticeStrings 追加不重复的非空字符串。
func appendUniquePracticeStrings(values []string, next ...string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values)+len(next))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	for _, value := range next {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

// boolPointer 返回布尔值指针。
func boolPointer(value bool) *bool {
	return &value
}

// minInt 返回两个整数中的较小值。
func minInt(left int, right int) int {
	if left < right {
		return left
	}
	return right
}

// uintPointer 返回整数指针，便于构造可选查询参数。
func uintPointer(value uint) *uint {
	if value == 0 {
		return nil
	}
	return &value
}
