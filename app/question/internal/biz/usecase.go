package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"

	kratosErr "github.com/go-kratos/kratos/v2/errors"
)

var (
	ErrQuestionNotFound     = kratosErr.NotFound("QUESTION_NOT_FOUND", "题目不存在")
	ErrAlreadyFavorited     = kratosErr.Conflict("ALREADY_FAVORITED", "已收藏")
	ErrFavoriteNotFound     = kratosErr.NotFound("FAVORITE_NOT_FOUND", "收藏不存在")
	ErrNoteNotFound         = kratosErr.NotFound("NOTE_NOT_FOUND", "笔记不存在")
	ErrExamNotFound         = kratosErr.NotFound("EXAM_NOT_FOUND", "考试不存在")
	ErrQuestionSetNotFound  = kratosErr.NotFound("QUESTION_SET_NOT_FOUND", "题集不存在")
	ErrExamAlreadyCompleted = kratosErr.Conflict("EXAM_COMPLETED", "考试已完成")
)

type QuestionUseCase struct {
	questionRepo    QuestionRepo
	recordRepo      RecordRepo
	favoriteRepo    FavoriteRepo
	noteRepo        NoteRepo
	categoryRepo    CategoryRepo
	industryRepo    IndustryRepo
	quizAnalyzer    QuizAnalyzerClient
	codeRunner      CodeRunnerClient
	examRepo        ExamRepo
	questionSetRepo QuestionSetRepo
	generator       QuestionGeneratorClient
}

// NewQuestionUseCase 创建题库业务用例
func NewQuestionUseCase(
	questionRepo QuestionRepo,
	recordRepo RecordRepo,
	favoriteRepo FavoriteRepo,
	noteRepo NoteRepo,
	categoryRepo CategoryRepo,
	industryRepo IndustryRepo,
	quizAnalyzer QuizAnalyzerClient,
	codeRunner CodeRunnerClient,
	examRepo ExamRepo,
	questionSetRepo QuestionSetRepo,
	generator QuestionGeneratorClient,
) *QuestionUseCase {
	return &QuestionUseCase{
		questionRepo:    questionRepo,
		recordRepo:      recordRepo,
		favoriteRepo:    favoriteRepo,
		noteRepo:        noteRepo,
		categoryRepo:    categoryRepo,
		industryRepo:    industryRepo,
		quizAnalyzer:    quizAnalyzer,
		codeRunner:      codeRunner,
		examRepo:        examRepo,
		questionSetRepo: questionSetRepo,
		generator:       generator,
	}
}

func (uc *QuestionUseCase) ListQuestions(ctx context.Context, filter *QuestionFilter, page, pageSize int32) ([]*Question, int64, error) {
	return uc.questionRepo.List(ctx, filter, page, pageSize)
}

func (uc *QuestionUseCase) GetQuestion(ctx context.Context, id uint64) (*Question, error) {
	q, err := uc.questionRepo.GetByID(ctx, id)
	if err != nil {
		return nil, ErrQuestionNotFound
	}
	return q, nil
}

func (uc *QuestionUseCase) SubmitAnswer(ctx context.Context, questionID, userID uint64, answer, language string) (*QuizAnalyzerResponse, error) {
	question, err := uc.questionRepo.GetByID(ctx, questionID)
	if err != nil {
		return nil, ErrQuestionNotFound
	}

	// 调用 AI 分析答案
	resp, err := uc.quizAnalyzer.Analyze(ctx, &QuizAnalyzerRequest{
		Question:   question.Content,
		Answer:     answer,
		Topic:      question.CategoryName,
		Difficulty: question.Difficulty,
	})
	if err != nil {
		return nil, kratosErr.InternalServer("AI_ANALYZE_FAILED", "AI 分析失败").WithCause(err)
	}

	// 保存答题记录
	record := &UserQuestionRecord{
		UserID:     userID,
		QuestionID: questionID,
		IsCorrect:  resp.IsCorrect,
		Answer:     answer,
		Language:   language,
		Score:      resp.Score,
	}
	_ = uc.recordRepo.Create(ctx, record)

	return resp, nil
}

func (uc *QuestionUseCase) CreateFavorite(ctx context.Context, userID, questionID uint64) error {
	exists, _ := uc.favoriteRepo.Exists(ctx, userID, questionID)
	if exists {
		return ErrAlreadyFavorited
	}
	return uc.favoriteRepo.Create(ctx, &UserFavorite{
		UserID:     userID,
		QuestionID: questionID,
	})
}

func (uc *QuestionUseCase) DeleteFavorite(ctx context.Context, userID, questionID uint64) error {
	return uc.favoriteRepo.Delete(ctx, userID, questionID)
}

func (uc *QuestionUseCase) ListCategories(ctx context.Context, industryCode string) ([]*Category, error) {
	return uc.categoryRepo.ListByIndustry(ctx, industryCode)
}

func (uc *QuestionUseCase) ListIndustries(ctx context.Context) ([]*Industry, error) {
	return uc.industryRepo.List(ctx)
}

func (uc *QuestionUseCase) GetUserPracticeStats(ctx context.Context, userID uint64) (int32, int32, float64, []*CategoryStat, error) {
	stats, err := uc.recordRepo.GetCategoryStats(ctx, userID)
	if err != nil {
		return 0, 0, 0, nil, err
	}

	var totalAnswered, totalCorrect int32
	for _, s := range stats {
		totalAnswered += s.Answered
		totalCorrect += s.Correct
	}

	accuracy := float64(0)
	if totalAnswered > 0 {
		accuracy = float64(totalCorrect) / float64(totalAnswered)
	}

	return totalAnswered, totalCorrect, accuracy, stats, nil
}

func (uc *QuestionUseCase) GetWrongQuestions(ctx context.Context, userID uint64, page, pageSize int32) ([]*WrongQuestion, int64, error) {
	return uc.recordRepo.GetWrongQuestions(ctx, userID, page, pageSize)
}

// GetPracticeRecommendations 增强推荐算法：面试驱动加权
// interviewID > 0 时，优先推荐面试相关分类的薄弱题目
func (uc *QuestionUseCase) GetPracticeRecommendations(ctx context.Context, userID uint64, interviewID uint64) ([]*Question, string, error) {
	// 基于错题推荐薄弱知识点的题目
	wrong, _, err := uc.recordRepo.GetWrongQuestions(ctx, userID, 1, 20)
	if err != nil {
		return nil, "", err
	}

	if len(wrong) == 0 {
		return nil, "暂无推荐", nil
	}

	// 获取错题对应的题目，并计算加权得分
	type scoredQuestion struct {
		question *Question
		score    float64
	}

	var scored []scoredQuestion
	for _, w := range wrong {
		q, err := uc.questionRepo.GetByID(ctx, w.QuestionID)
		if err != nil {
			continue
		}

		// 基础得分 = 错误次数加权
		score := float64(w.WrongCount) * 1.0

		// 面试驱动加权：如果指定了面试 ID，相关分类题目得分 ×2
		if interviewID > 0 {
			// 面试驱动策略：coding 类型和对应行业加权
			if q.IndustryCode != "" {
				score *= 2.0
			}
		}

		// 难度加权：hard > medium > easy
		switch q.Difficulty {
		case "hard":
			score *= 1.5
		case "medium":
			score *= 1.2
		}

		scored = append(scored, scoredQuestion{question: q, score: score})
	}

	// 按加权得分排序
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// 取前 10 题
	limit := 10
	if len(scored) < limit {
		limit = len(scored)
	}

	questions := make([]*Question, limit)
	for i := 0; i < limit; i++ {
		questions[i] = scored[i].question
	}

	reason := fmt.Sprintf("基于你最近 %d 道错题的薄弱知识点推荐", len(wrong))
	if interviewID > 0 {
		reason = "基于面试场景和错题薄弱点推荐"
	}
	return questions, reason, nil
}

func (uc *QuestionUseCase) ListFavorites(ctx context.Context, userID uint64, page, pageSize int32) ([]*Question, int64, error) {
	return uc.favoriteRepo.ListByUser(ctx, userID, page, pageSize)
}

func (uc *QuestionUseCase) CreateNote(ctx context.Context, userID, questionID uint64, content string) (*UserNote, error) {
	note := &UserNote{
		UserID:     userID,
		QuestionID: questionID,
		Content:    content,
	}
	if err := uc.noteRepo.Create(ctx, note); err != nil {
		return nil, err
	}
	return note, nil
}

func (uc *QuestionUseCase) UpdateNote(ctx context.Context, noteID, userID uint64, content string) (*UserNote, error) {
	note := &UserNote{
		ID:      noteID,
		UserID:  userID,
		Content: content,
	}
	if err := uc.noteRepo.Update(ctx, note); err != nil {
		return nil, err
	}
	return note, nil
}

// DeleteNote 删除笔记（校验归属后删除）
func (uc *QuestionUseCase) DeleteNote(ctx context.Context, noteID, userID uint64) error {
	note, err := uc.noteRepo.GetByID(ctx, noteID)
	if err != nil {
		return ErrNoteNotFound
	}
	if note.UserID != userID {
		return kratosErr.Forbidden("NOTE_FORBIDDEN", "无权删除该笔记")
	}
	return uc.noteRepo.Delete(ctx, noteID, userID)
}

func (uc *QuestionUseCase) ListNotes(ctx context.Context, userID, questionID uint64, page, pageSize int32) ([]*UserNote, int64, error) {
	return uc.noteRepo.ListByUser(ctx, userID, questionID, page, pageSize)
}

// RunCode 调用代码运行服务执行用户代码
func (uc *QuestionUseCase) RunCode(ctx context.Context, questionID uint64, language, code string) (*CodeRunnerResponse, error) {
	question, err := uc.questionRepo.GetByID(ctx, questionID)
	if err != nil {
		return nil, ErrQuestionNotFound
	}

	// 从题目的 TestCasesJSON 字段解析测试用例
	var testCases []CodeTestCase
	if question.TestCasesJSON != "" {
		var cases []struct {
			Input          string `json:"input"`
			ExpectedOutput string `json:"expected_output"`
		}
		if err := json.Unmarshal([]byte(question.TestCasesJSON), &cases); err == nil {
			for _, tc := range cases {
				testCases = append(testCases, CodeTestCase{
					Input:          tc.Input,
					ExpectedOutput: tc.ExpectedOutput,
				})
			}
		}
	}

	resp, err := uc.codeRunner.Execute(ctx, &CodeRunnerRequest{
		Language:  language,
		Code:      code,
		TestCases: testCases,
		TimeoutMs: 10000,
	})
	if err != nil {
		return nil, kratosErr.InternalServer("CODE_RUNNER_FAILED", "代码运行服务调用失败").WithCause(err)
	}
	return resp, nil
}

// GenerateTimedExam 生成限时考试
func (uc *QuestionUseCase) GenerateTimedExam(ctx context.Context, userID uint64, industryCode string, questionCount, timeLimitMin int32) (*Exam, []*Question, error) {
	if questionCount <= 0 {
		questionCount = 10
	}
	if timeLimitMin <= 0 {
		timeLimitMin = questionCount * 5
	}

	// 随机选题
	filter := &QuestionFilter{IndustryCode: industryCode}
	questions, err := uc.questionRepo.RandomSelect(ctx, filter, questionCount)
	if err != nil {
		return nil, nil, err
	}
	if len(questions) == 0 {
		return nil, nil, kratosErr.NotFound("NO_QUESTIONS", "无可用题目")
	}

	// 提取题目 ID
	qIDs := make([]uint64, len(questions))
	for i, q := range questions {
		qIDs[i] = q.ID
	}

	exam := &Exam{
		UserID:       userID,
		IndustryCode: industryCode,
		QuestionIDs:  qIDs,
		TimeLimitMin: timeLimitMin,
		Status:       "pending",
	}
	if err := uc.examRepo.Create(ctx, exam); err != nil {
		return nil, nil, err
	}
	return exam, questions, nil
}

// SubmitExam 提交考试答案并评分
func (uc *QuestionUseCase) SubmitExam(ctx context.Context, examID, userID uint64, answers map[uint64]string) (*ExamResult, error) {
	exam, err := uc.examRepo.GetByID(ctx, examID)
	if err != nil {
		return nil, ErrExamNotFound
	}
	if exam.Status == "completed" {
		return nil, ErrExamAlreadyCompleted
	}
	if exam.UserID != userID {
		return nil, kratosErr.Forbidden("EXAM_FORBIDDEN", "无权提交该考试")
	}

	result := &ExamResult{
		ExamID:         examID,
		MaxScore:       float64(len(exam.QuestionIDs)) * 100,
		TotalQuestions: int32(len(exam.QuestionIDs)),
	}

	var totalScore float64
	var correctCount int32

	for _, qID := range exam.QuestionIDs {
		answer, ok := answers[qID]
		if !ok {
			continue
		}

		question, err := uc.questionRepo.GetByID(ctx, qID)
		if err != nil {
			continue
		}

		// 调用 AI 评分
		aiResp, err := uc.quizAnalyzer.Analyze(ctx, &QuizAnalyzerRequest{
			Question:   question.Content,
			Answer:     answer,
			Topic:      question.CategoryName,
			Difficulty: question.Difficulty,
		})
		if err != nil {
			// AI 评分失败时记录错误并标记该题为评分失败
			qr := &QuestionResult{
				QuestionID: qID,
				Score:      0,
				Feedback:   "评分服务暂时不可用，请稍后重试",
			}
			result.QuestionResults = append(result.QuestionResults, qr)
			continue
		}

		qr := &QuestionResult{
			QuestionID:    qID,
			IsCorrect:     aiResp.IsCorrect,
			Score:         aiResp.Score,
			Feedback:      aiResp.Feedback,
			CorrectAnswer: aiResp.CorrectAnswer,
		}
		result.QuestionResults = append(result.QuestionResults, qr)
		totalScore += aiResp.Score
		if aiResp.IsCorrect {
			correctCount++
		}

		// 保存答题记录
		if err := uc.recordRepo.Create(ctx, &UserQuestionRecord{
			UserID:     userID,
			QuestionID: qID,
			ExamID:     examID,
			IsCorrect:  aiResp.IsCorrect,
			Answer:     answer,
			Score:      aiResp.Score,
		}); err != nil {
			// 记录保存失败不影响整体流程，但需要记录日志
			fmt.Printf("保存答题记录失败: question_id=%d, err=%v\n", qID, err)
		}
	}

	result.TotalScore = totalScore
	result.CorrectCount = correctCount

	// FIX Q5: 更新考试状态，检查错误
	exam.Status = "completed"
	exam.TotalScore = totalScore
	if err := uc.examRepo.Update(ctx, exam); err != nil {
		return nil, kratosErr.InternalServer("EXAM_UPDATE_FAILED", "更新考试状态失败").WithCause(err)
	}

	return result, nil
}

func (uc *QuestionUseCase) GetRandomExam(ctx context.Context, industryCode string, questionCount int32) ([]*Question, error) {
	if questionCount <= 0 {
		questionCount = 10
	}
	filter := &QuestionFilter{IndustryCode: industryCode}
	return uc.questionRepo.RandomSelect(ctx, filter, questionCount)
}

// ListQuestionSets 获取题集列表
func (uc *QuestionUseCase) ListQuestionSets(ctx context.Context, industryCode string, page, pageSize int32) ([]*QuestionSet, int64, error) {
	return uc.questionSetRepo.List(ctx, industryCode, page, pageSize)
}

// GetQuestionSetDetail 获取题集详情（含题目列表）
func (uc *QuestionUseCase) GetQuestionSetDetail(ctx context.Context, setID uint64) (*QuestionSet, []*Question, error) {
	set, err := uc.questionSetRepo.GetByID(ctx, setID)
	if err != nil {
		return nil, nil, ErrQuestionSetNotFound
	}
	questions, err := uc.questionSetRepo.GetQuestions(ctx, setID)
	if err != nil {
		return nil, nil, err
	}
	return set, questions, nil
}

// ListMistakeTopics 获取用户错题知识点聚合
func (uc *QuestionUseCase) ListMistakeTopics(ctx context.Context, userID uint64) ([]*MistakeTopic, error) {
	topics, err := uc.recordRepo.GetMistakeTopics(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 按错误率降序排列，错误率相同时按错误次数降序
	sort.Slice(topics, func(i, j int) bool {
		if math.Abs(topics[i].Accuracy-topics[j].Accuracy) < 0.001 {
			return topics[i].WrongCount > topics[j].WrongCount
		}
		return topics[i].Accuracy < topics[j].Accuracy
	})

	return topics, nil
}

// ImportQuestions 从 scraper 导入题目（批量创建，FIX Q3: 幂等去重）
func (uc *QuestionUseCase) ImportQuestions(ctx context.Context, questions []*Question) (int, error) {
	var imported int
	for _, q := range questions {
		// 基本校验
		if q.Title == "" || q.Content == "" {
			continue
		}

		// FIX Q3: 检查同名同行业题目是否已存在，避免重复导入
		exists, err := uc.questionRepo.ExistsByTitleAndIndustry(ctx, q.Title, q.IndustryCode)
		if err != nil {
			fmt.Printf("去重检查失败: title=%s, err=%v\n", q.Title, err)
			continue
		}
		if exists {
			continue
		}

		// 调用 repo 创建题目
		if err := uc.questionRepo.Create(ctx, q); err != nil {
			fmt.Printf("导入题目失败: title=%s, err=%v\n", q.Title, err)
			continue
		}
		imported++
	}
	return imported, nil
}

// PipelineGenerateQuestions 流水线生成题目：调用 AI 生成并写入题库
func (uc *QuestionUseCase) PipelineGenerateQuestions(ctx context.Context, req *GenerateQuestionsRequest) (int, error) {
	if uc.generator == nil {
		return 0, kratosErr.ServiceUnavailable("GENERATOR_NOT_CONFIGURED", "题目生成服务未配置")
	}

	// 1. 调用 AI 生成题目
	questions, err := uc.generator.GenerateQuestions(ctx, req)
	if err != nil {
		return 0, kratosErr.InternalServer("GENERATE_FAILED", "AI 生成题目失败").WithCause(err)
	}

	if len(questions) == 0 {
		return 0, nil
	}

	// 2. 逐个写入题库
	var created int
	for _, q := range questions {
		if q.Title == "" || q.Content == "" {
			continue
		}
		if err := uc.questionRepo.Create(ctx, q); err != nil {
			// 单条失败不中断整体，记录后继续
			fmt.Printf("pipeline 创建题目失败: title=%s, err=%v\n", q.Title, err)
			continue
		}
		created++
	}

	return created, nil
}
