// Package service 提供业务逻辑层实现
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
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

// QuestionDetail 题目详情DTO
type QuestionDetail struct {
	model.Question
	IsFavorited bool            `json:"is_favorited"`
	UserNote    *model.UserNote `json:"user_note,omitempty"`
}

// RandomExamRequest 随机组卷请求DTO
type RandomExamRequest struct {
	CategoryID *uint  `json:"category_id"`
	Difficulty string `json:"difficulty"`
	Count      int    `json:"count" binding:"required,min=1,max=100"`
}

// TimedExamRequest 限时模拟请求DTO
type TimedExamRequest struct {
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
	GetCategories(ctx context.Context, industryID uint) ([]CategoryTree, error)

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
}

// questionService 题目服务实现
type questionService struct {
	questionRepo QuestionRepository
	categoryRepo CategoryRepository
	recordRepo   QuestionRecordRepository
	favoriteRepo FavoriteRepository
	noteRepo     NoteRepository
	quizAnalyzer ai.QuizAnalyzer
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
) QuestionService {
	return &questionService{
		questionRepo: questionRepo,
		categoryRepo: categoryRepo,
		recordRepo:   recordRepo,
		favoriteRepo: favoriteRepo,
		noteRepo:     noteRepo,
		quizAnalyzer: quizAnalyzer,
	}
}

// ListQuestions 获取题目列表
func (s *questionService) ListQuestions(ctx context.Context, params repository.QuestionListParams) (*common.PageResult, error) {
	questions, total, err := s.questionRepo.List(ctx, params)
	if err != nil {
		return nil, err
	}

	return &common.PageResult{
		List:     questions,
		Total:    total,
		Page:     params.Page,
		PageSize: params.PageSize,
	}, nil
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
		Question:    *question,
		IsFavorited: false,
		UserNote:    nil,
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

// GetCategories 获取分类列表
func (s *questionService) GetCategories(ctx context.Context, industryID uint) ([]CategoryTree, error) {
	return s.categoryRepo.GetTree(ctx, industryID)
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
			}
		}
	}

	record.IsCorrect = resp.IsCorrect
	if err := s.recordRepo.Create(ctx, record); err != nil {
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
	favorites, total, err := s.favoriteRepo.ListByUser(ctx, userID, page, pageSize)
	if err != nil {
		return nil, err
	}

	return &common.PageResult{
		List:     favorites,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// GetWrongQuestions 获取错题列表
func (s *questionService) GetWrongQuestions(ctx context.Context, userID uint, page, pageSize int) (*common.PageResult, error) {
	records, total, err := s.recordRepo.GetWrongQuestions(ctx, userID, page, pageSize)
	if err != nil {
		return nil, err
	}

	return &common.PageResult{
		List:     records,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
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
	notes, total, err := s.noteRepo.ListByUser(ctx, userID, page, pageSize)
	if err != nil {
		return nil, err
	}

	return &common.PageResult{
		List:     notes,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
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
		IndustryID: 1, // 默认行业，后续可从用户配置获取
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
		IndustryID:  params.IndustryID,
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
		IndustryID: 1, // 默认行业
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
		IndustryID:  params.IndustryID,
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
