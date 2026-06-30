package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	kratosErr "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
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

// RAGSyncPublisher 题目变更后发布 RAG 同步事件的接口
type RAGSyncPublisher interface {
	PublishQuestionChanged(ctx context.Context, questionID uint64, action string, content string, metadata map[string]string) error
}

type QuestionUseCase struct {
	questionRepo          QuestionRepo
	recordRepo            RecordRepo
	favoriteRepo          FavoriteRepo
	noteRepo              NoteRepo
	categoryRepo          CategoryRepo
	industryRepo          IndustryRepo
	quizAnalyzer          QuizAnalyzerClient
	codeRunner            CodeRunnerClient
	examRepo              ExamRepo
	questionSetRepo       QuestionSetRepo
	generator             QuestionGeneratorClient
	ragSyncPub            RAGSyncPublisher
	learningArchiveClient LearningArchiveClient
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
	learningArchiveClient LearningArchiveClient,
) *QuestionUseCase {
	return &QuestionUseCase{
		questionRepo:          questionRepo,
		recordRepo:            recordRepo,
		favoriteRepo:          favoriteRepo,
		noteRepo:              noteRepo,
		categoryRepo:          categoryRepo,
		industryRepo:          industryRepo,
		quizAnalyzer:          quizAnalyzer,
		codeRunner:            codeRunner,
		examRepo:              examRepo,
		questionSetRepo:       questionSetRepo,
		generator:             generator,
		learningArchiveClient: learningArchiveClient,
	}
}

// SetRAGSyncPublisher 注入 RAG 同步事件发布器
func (uc *QuestionUseCase) SetRAGSyncPublisher(pub RAGSyncPublisher) {
	uc.ragSyncPub = pub
}

func (uc *QuestionUseCase) ListQuestions(ctx context.Context, filter *QuestionFilter, page, pageSize int32) ([]*Question, int64, error) {
	return uc.questionRepo.List(ctx, filter, page, pageSize)
}

// CreateQuestion 管理后台创建题目，并在有分类时补齐行业与分类信息。
func (uc *QuestionUseCase) CreateQuestion(ctx context.Context, question *Question) error {
	if question.CategoryID > 0 {
		category, err := uc.categoryRepo.GetByID(ctx, question.CategoryID)
		if err != nil {
			return err
		}
		question.CategoryName = category.Name
		if question.IndustryCode == "" && category.IndustryID > 0 {
			if industry, err := uc.industryRepo.GetByID(ctx, category.IndustryID); err == nil {
				question.IndustryCode = industry.Code
			}
		}
	}
	if err := uc.questionRepo.Create(ctx, question); err != nil {
		return err
	}

	// 发布 RAG 同步事件
	uc.publishRAGSync(ctx, question.ID, "create", question.Title+"\n"+question.Content, buildRAGSyncMetadata(question))

	return nil
}

// UpdateQuestion 管理后台更新题目，并在有分类时同步分类与行业冗余字段。
func (uc *QuestionUseCase) UpdateQuestion(ctx context.Context, question *Question) error {
	if question.CategoryID > 0 {
		category, err := uc.categoryRepo.GetByID(ctx, question.CategoryID)
		if err != nil {
			return err
		}
		question.CategoryName = category.Name
		if question.IndustryCode == "" && category.IndustryID > 0 {
			if industry, err := uc.industryRepo.GetByID(ctx, category.IndustryID); err == nil {
				question.IndustryCode = industry.Code
			}
		}
	}
	if err := uc.questionRepo.Update(ctx, question); err != nil {
		return err
	}

	// 发布 RAG 同步事件
	uc.publishRAGSync(ctx, question.ID, "update", question.Title+"\n"+question.Content, buildRAGSyncMetadata(question))

	return nil
}

// DeleteQuestion 管理后台删除题目。
func (uc *QuestionUseCase) DeleteQuestion(ctx context.Context, id uint64) error {
	if err := uc.questionRepo.Delete(ctx, id); err != nil {
		return err
	}

	// 发布 RAG 同步事件（删除）
	uc.publishRAGSync(ctx, id, "delete", "", nil)

	return nil
}

// buildRAGSyncMetadata 构造题目同步到 RAG 时需要的最小元数据集合。
func buildRAGSyncMetadata(question *Question) map[string]string {
	if question == nil {
		return nil
	}
	return map[string]string{
		"title":      question.Title,
		"type":       question.Type,
		"difficulty": question.Difficulty,
	}
}

// publishRAGSync 发布 RAG 同步事件（FIX C6: 使用结构化日志，不静默吞错）
func (uc *QuestionUseCase) publishRAGSync(ctx context.Context, questionID uint64, action string, content string, metadata map[string]string) {
	if uc.ragSyncPub == nil {
		return
	}
	if err := uc.ragSyncPub.PublishQuestionChanged(ctx, questionID, action, content, metadata); err != nil {
		log.Errorf("RAG sync publish failed: question_id=%d, action=%s, err=%v", questionID, action, err)
	}
}

// GetAdminQuestionStats 返回管理后台需要的题目总数。
func (uc *QuestionUseCase) GetAdminQuestionStats(ctx context.Context) (int64, error) {
	return uc.questionRepo.Count(ctx, nil)
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

	var resp *QuizAnalyzerResponse

	switch question.Type {
	case "choice", "multi":
		// 选择题：本地判分，不调 AI，返回题目自带解析
		isCorrect := judgeChoiceAnswer(question, answer)
		score := float64(0)
		if isCorrect {
			score = 100
		}
		resp = &QuizAnalyzerResponse{
			Score:          score,
			IsCorrect:      isCorrect,
			Feedback:       question.Explanation,
			CorrectAnswer:  question.Answer,
			EvaluationMode: "local",
		}
	default:
		// 编程题/主观题：调用 AI 分析
		resp, err = uc.quizAnalyzer.Analyze(ctx, &QuizAnalyzerRequest{
			Question:   question.Content,
			Answer:     answer,
			Topic:      question.CategoryName,
			Difficulty: question.Difficulty,
		})
		if err != nil {
			return nil, kratosErr.InternalServer("AI_ANALYZE_FAILED", "AI 分析失败").WithCause(err)
		}
		resp.EvaluationMode = ResolveEvaluationMode(question.JudgeConfig)

		// 编程题 testcase 模式：运行代码获取 judge_summary（使用隐藏用例）
		if question.JudgeConfig != nil && question.JudgeConfig.EvaluationMode == EvaluationModeTestcase {
			judgeSummary := uc.runCodeForJudgeSummary(ctx, question, answer, language)
			if judgeSummary != nil {
				resp.JudgeSummary = judgeSummary
				resp.IsCorrect = judgeSummary.AllPassed
				if judgeSummary.AllPassed {
					resp.Score = 100
				} else if judgeSummary.TotalCases > 0 {
					resp.Score = float64(judgeSummary.PassedCases) / float64(judgeSummary.TotalCases) * 100
				}
			}
		}
	}

	// 保存答题记录（Upsert 去重）
	record := &UserQuestionRecord{
		UserID:     userID,
		QuestionID: questionID,
		IsCorrect:  resp.IsCorrect,
		Answer:     answer,
		Language:   language,
		Score:      resp.Score,
	}
	if err := uc.recordRepo.Upsert(ctx, record); err != nil {
		log.Errorf("保存答题记录失败: question_id=%d, user_id=%d, err=%v", questionID, userID, err)
	}

	// 同步学习档案
	uc.syncPracticeLearningArchive(ctx, userID, question, record, resp)

	return resp, nil
}

// judgeChoiceAnswer 本地判分选择题，支持单选和多选
func judgeChoiceAnswer(question *Question, userAnswer string) bool {
	userAnswer = strings.TrimSpace(userAnswer)
	correctAnswer := strings.TrimSpace(question.Answer)

	switch question.Type {
	case "choice":
		return strings.EqualFold(userAnswer, correctAnswer)
	case "multi":
		return normalizeMultiAnswer(userAnswer) == normalizeMultiAnswer(correctAnswer)
	default:
		return false
	}
}

// normalizeMultiAnswer 将多选答案排序去重后标准化比较
func normalizeMultiAnswer(answer string) string {
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

// runCodeForJudgeSummary 运行代码并构建判题摘要（SubmitAnswer 时使用隐藏用例）
func (uc *QuestionUseCase) runCodeForJudgeSummary(ctx context.Context, question *Question, code, language string) *JudgeSummary {
	if uc.codeRunner == nil {
		return nil
	}

	config := question.JudgeConfig
	if config == nil || config.EvaluationMode != EvaluationModeTestcase {
		return nil
	}

	// SubmitAnswer 使用隐藏用例
	hiddenCases := SelectTestCases(config, false)
	if len(hiddenCases) == 0 {
		return nil
	}

	lang := ResolveJudgeLanguage(language, config)

	testCases := make([]CodeTestCase, 0, len(hiddenCases))
	for _, tc := range hiddenCases {
		testCases = append(testCases, CodeTestCase{
			Input:          tc.Input,
			ExpectedOutput: tc.ExpectedOutput,
		})
	}

	resp, err := uc.codeRunner.Execute(ctx, &CodeRunnerRequest{
		Language:  lang,
		Code:      code,
		TestCases: testCases,
		TimeoutMs: int32(config.TimeLimitMS),
	})
	if err != nil {
		log.Warnf("代码运行失败: question_id=%d, err=%v", question.ID, err)
		return nil
	}

	// 用规范化逻辑重新判定每条用例
	results := make([]JudgeCaseResult, 0, len(resp.TestResults))
	var passedCount int32
	for i, tr := range resp.TestResults {
		actualNorm := NormalizeJudgeOutputText(tr.ActualOutput)
		expectedNorm := NormalizeJudgeOutputText(tr.ExpectedOutput)
		passed := actualNorm == expectedNorm
		if passed {
			passedCount++
		}
		var desc string
		if i < len(hiddenCases) {
			desc = hiddenCases[i].Description
		}
		results = append(results, JudgeCaseResult{
			Input:          tr.Input,
			ExpectedOutput: expectedNorm,
			ActualOutput:   actualNorm,
			Passed:         passed,
			Description:    desc,
		})
	}

	return &JudgeSummary{
		AllPassed:   passedCount == int32(len(hiddenCases)),
		TotalCases:  int32(len(hiddenCases)),
		PassedCases: passedCount,
		Results:     results,
	}
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

// IsFavorited 查询当前用户是否收藏了指定题目。
func (uc *QuestionUseCase) IsFavorited(ctx context.Context, userID, questionID uint64) bool {
	exists, _ := uc.favoriteRepo.Exists(ctx, userID, questionID)
	return exists
}

// GetUserNote 获取用户对指定题目的笔记（对齐单体：GetQuestion 填充 user_note）
func (uc *QuestionUseCase) GetUserNote(ctx context.Context, userID, questionID uint64) (*UserNote, error) {
	notes, _, err := uc.noteRepo.ListByUser(ctx, userID, questionID, 1, 1)
	if err != nil {
		return nil, err
	}
	if len(notes) == 0 {
		return nil, nil
	}
	return notes[0], nil
}

func (uc *QuestionUseCase) ListCategories(ctx context.Context, industryID uint64) ([]*Category, error) {
	return uc.categoryRepo.ListByIndustry(ctx, industryID)
}

func (uc *QuestionUseCase) ListIndustries(ctx context.Context) ([]*Industry, error) {
	return uc.industryRepo.List(ctx)
}

func (uc *QuestionUseCase) GetIndustryByCode(ctx context.Context, code string) (*Industry, error) {
	return uc.industryRepo.GetByCode(ctx, code)
}

func (uc *QuestionUseCase) GetUserPracticeStats(ctx context.Context, userID uint64) (int32, int32, float64, []*CategoryStat, int32, error) {
	stats, err := uc.recordRepo.GetCategoryStats(ctx, userID)
	if err != nil {
		return 0, 0, 0, nil, 0, err
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

	todayCount, _ := uc.recordRepo.GetTodayCount(ctx, userID)

	return totalAnswered, totalCorrect, accuracy, stats, todayCount, nil
}

func (uc *QuestionUseCase) GetWrongQuestions(ctx context.Context, userID uint64, page, pageSize int32) ([]*WrongQuestion, int64, error) {
	return uc.recordRepo.GetWrongQuestions(ctx, userID, page, pageSize)
}

// GetAnsweredQuestionIDs 批量查询用户已答题的题目 ID 集合
func (uc *QuestionUseCase) GetAnsweredQuestionIDs(ctx context.Context, userID uint64, questionIDs []uint64) (map[uint64]bool, error) {
	return uc.recordRepo.GetAnsweredQuestionIDs(ctx, userID, questionIDs)
}

// GetFavoritedQuestionIDs 批量查询用户已收藏的题目 ID 集合
func (uc *QuestionUseCase) GetFavoritedQuestionIDs(ctx context.Context, userID uint64, questionIDs []uint64) (map[uint64]bool, error) {
	return uc.favoriteRepo.GetFavoritedQuestionIDs(ctx, userID, questionIDs)
}

// GetMistakeTopicCard 通过 gRPC 从 learning_archive 服务查询错因专题详情。
func (uc *QuestionUseCase) GetMistakeTopicCard(ctx context.Context, code string) (*MistakeTopicCard, error) {
	if uc.learningArchiveClient == nil {
		topic, found := ResolveMistakeTopicByCode(code)
		if !found {
			return nil, nil
		}
		return topic, nil
	}
	topic, found := uc.learningArchiveClient.GetMistakeTopic(ctx, code)
	if !found {
		return nil, nil
	}
	return topic, nil
}

// GetPracticeRecommendations 增强推荐算法：基于学习档案的错因标签驱动推荐
// 通过 gRPC 从 learning_archive 服务获取焦点信号，搜索匹配题目推荐给用户
func (uc *QuestionUseCase) GetPracticeRecommendations(ctx context.Context, userID uint64, interviewID uint64) ([]*Question, string, error) {
	// 尝试从 learning_archive 服务获取焦点信号生成增强推荐
	if uc.learningArchiveClient != nil {
		signals, err := uc.learningArchiveClient.GetFocusSignals(ctx, userID, defaultTrainingFocusSignalLimit)
		if err == nil && len(signals) > 0 {
			questions, reason := uc.buildRecommendationsFromSignals(ctx, signals, interviewID)
			if len(questions) > 0 {
				return questions, reason, nil
			}
		}
	}

	// 回退到基于错题的简单推荐
	return uc.buildRecommendationsFromWrongQuestions(ctx, userID, interviewID)
}

// buildRecommendationsFromSignals 基于训练重点信号推荐题目
func (uc *QuestionUseCase) buildRecommendationsFromSignals(ctx context.Context, signals []FocusSignalData, interviewID uint64) ([]*Question, string) {
	type scoredQuestion struct {
		question *Question
		score    float64
	}

	var scored []scoredQuestion
	seenIDs := make(map[uint64]struct{})
	reasonParts := make([]string, 0, len(signals))

	for _, signal := range signals {
		// 使用信号的推荐动作作为关键词搜索题目
		keywords := []string{signal.Tag}
		if signal.TopicTitle != "" {
			keywords = append(keywords, signal.TopicTitle)
		}

		for _, keyword := range keywords {
			questions, _, err := uc.questionRepo.List(ctx, &QuestionFilter{
				Keyword: keyword,
			}, 1, 5)
			if err != nil {
				continue
			}

			for _, q := range questions {
				if _, exists := seenIDs[q.ID]; exists {
					continue
				}
				seenIDs[q.ID] = struct{}{}

				score := float64(signal.OccurrenceCount) * 1.0
				if interviewID > 0 {
					score *= 2.0
				}
				switch q.Difficulty {
				case "hard":
					score *= 1.5
				case "medium":
					score *= 1.2
				}

				scored = append(scored, scoredQuestion{question: q, score: score})
			}
		}

		if signal.Reason != "" {
			reasonParts = append(reasonParts, signal.Reason)
		}
	}

	if len(scored) == 0 {
		return nil, ""
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	limit := 10
	if len(scored) < limit {
		limit = len(scored)
	}

	questions := make([]*Question, limit)
	for i := 0; i < limit; i++ {
		questions[i] = scored[i].question
	}

	reason := "基于学习档案中的高频薄弱点推荐"
	if len(reasonParts) > 0 {
		reason = reasonParts[0]
	}
	if interviewID > 0 {
		reason = "基于面试场景和学习档案薄弱点推荐"
	}

	return questions, reason
}

// buildRecommendationsFromWrongQuestions 基于错题记录推荐（回退方案）
func (uc *QuestionUseCase) buildRecommendationsFromWrongQuestions(ctx context.Context, userID uint64, interviewID uint64) ([]*Question, string, error) {
	wrong, _, err := uc.recordRepo.GetWrongQuestions(ctx, userID, 1, 20)
	if err != nil {
		return nil, "", err
	}

	if len(wrong) == 0 {
		return nil, "暂无推荐", nil
	}

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

		score := float64(w.WrongCount) * 1.0
		if interviewID > 0 {
			if q.IndustryCode != "" {
				score *= 2.0
			}
		}
		switch q.Difficulty {
		case "hard":
			score *= 1.5
		case "medium":
			score *= 1.2
		}

		scored = append(scored, scoredQuestion{question: q, score: score})
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

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
		QuestionID: &questionID,
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

	if uc.codeRunner == nil {
		return nil, kratosErr.ServiceUnavailable("CODE_RUNNER_UNAVAILABLE", "代码运行服务未配置")
	}

	config := question.JudgeConfig
	evalMode := ResolveEvaluationMode(config)
	lang := ResolveJudgeLanguage(language, config)

	if evalMode == EvaluationModeTestcase && config != nil {
		// 测试用例判题模式：RunCode 只执行公开用例（最多3条）
		publicCases := SelectTestCases(config, true)
		if len(publicCases) == 0 {
			return &CodeRunnerResponse{
				Success: true,
				Output:  "无公开测试用例",
			}, nil
		}

		testCases := make([]CodeTestCase, 0, len(publicCases))
		for _, tc := range publicCases {
			testCases = append(testCases, CodeTestCase{
				Input:          tc.Input,
				ExpectedOutput: tc.ExpectedOutput,
			})
		}

		resp, err := uc.codeRunner.Execute(ctx, &CodeRunnerRequest{
			Language:  lang,
			Code:      code,
			TestCases: testCases,
			TimeoutMs: int32(config.TimeLimitMS),
		})
		if err != nil {
			return nil, kratosErr.InternalServer("CODE_RUNNER_FAILED", "代码运行服务调用失败").WithCause(err)
		}

		// 用规范化逻辑重新判定结果
		for i := range resp.TestResults {
			resp.TestResults[i].ActualOutput = NormalizeJudgeOutputText(resp.TestResults[i].ActualOutput)
			resp.TestResults[i].ExpectedOutput = NormalizeJudgeOutputText(resp.TestResults[i].ExpectedOutput)
			resp.TestResults[i].Passed = resp.TestResults[i].ActualOutput == resp.TestResults[i].ExpectedOutput
		}

		return resp, nil
	}

	// 非 testcase 模式：直接运行代码
	resp, err := uc.codeRunner.Execute(ctx, &CodeRunnerRequest{
		Language:  lang,
		Code:      code,
		TimeoutMs: 10000,
	})
	if err != nil {
		return nil, kratosErr.InternalServer("CODE_RUNNER_FAILED", "代码运行服务调用失败").WithCause(err)
	}
	return resp, nil
}

// GenerateTimedExamRequest 定义限时考试生成所需的筛选参数。
type GenerateTimedExamRequest struct {
	UserID           uint64
	IndustryID       uint64
	IndustryCode     string
	CategoryID       uint64
	Difficulty       string
	QuestionCount    int32
	TimeLimitMinutes int32
}

// GetRandomExamRequest 定义随机组卷所需的筛选参数。
type GetRandomExamRequest struct {
	UserID        uint64
	IndustryID    uint64
	IndustryCode  string
	CategoryID    uint64
	Difficulty    string
	QuestionCount int32
}

// GenerateTimedExam 生成限时考试
func (uc *QuestionUseCase) GenerateTimedExam(ctx context.Context, req *GenerateTimedExamRequest) (*Exam, []*Question, error) {
	if req.QuestionCount <= 0 {
		req.QuestionCount = 10
	}
	if req.TimeLimitMinutes <= 0 {
		req.TimeLimitMinutes = req.QuestionCount * 5
	}

	// 随机选题
	filter := &QuestionFilter{
		IndustryID:   req.IndustryID,
		IndustryCode: req.IndustryCode,
		CategoryID:   req.CategoryID,
		Difficulty:   req.Difficulty,
	}
	questions, err := uc.questionRepo.RandomSelect(ctx, filter, req.QuestionCount)
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
		UserID:       req.UserID,
		IndustryCode: req.IndustryCode,
		QuestionIDs:  qIDs,
		TimeLimitMin: req.TimeLimitMinutes,
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

		// 保存答题记录（Upsert 去重）
		if err := uc.recordRepo.Upsert(ctx, &UserQuestionRecord{
			UserID:     userID,
			QuestionID: qID,
			ExamID:     examID,
			IsCorrect:  aiResp.IsCorrect,
			Answer:     answer,
			Score:      aiResp.Score,
		}); err != nil {
			// 记录保存失败不影响整体流程，但需要记录日志
			log.Errorf("保存答题记录失败: question_id=%d, err=%v", qID, err)
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

// GetRandomExam 按筛选条件随机组装一套题卡，并创建考试记录以支持后续提交。
func (uc *QuestionUseCase) GetRandomExam(ctx context.Context, req *GetRandomExamRequest) (*Exam, []*Question, error) {
	if req.QuestionCount <= 0 {
		req.QuestionCount = 10
	}
	filter := &QuestionFilter{
		IndustryID:   req.IndustryID,
		IndustryCode: req.IndustryCode,
		CategoryID:   req.CategoryID,
		Difficulty:   req.Difficulty,
	}
	questions, err := uc.questionRepo.RandomSelect(ctx, filter, req.QuestionCount)
	if err != nil {
		return nil, nil, err
	}
	if len(questions) == 0 {
		return nil, nil, kratosErr.NotFound("NO_QUESTIONS", "无可用题目")
	}
	qIDs := make([]uint64, len(questions))
	for i, q := range questions {
		qIDs[i] = q.ID
	}
	exam := &Exam{
		UserID:       req.UserID,
		IndustryCode: req.IndustryCode,
		QuestionIDs:  qIDs,
		TimeLimitMin: int32(len(questions)) * 5,
		Status:       "pending",
	}
	if err := uc.examRepo.Create(ctx, exam); err != nil {
		return nil, nil, err
	}
	return exam, questions, nil
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

// GetQuestionSetQuestions 获取题集内的题目列表（不含题集元数据）。
func (uc *QuestionUseCase) GetQuestionSetQuestions(ctx context.Context, setID uint64) ([]*Question, error) {
	return uc.questionSetRepo.GetQuestions(ctx, setID)
}

// AdminCreateQuestionSet 管理后台创建题单
func (uc *QuestionUseCase) AdminCreateQuestionSet(ctx context.Context, set *QuestionSet) error {
	if set.Name == "" {
		return kratosErr.BadRequest("INVALID_NAME", "题单名称不能为空")
	}
	return uc.questionSetRepo.Create(ctx, set)
}

// AdminUpdateQuestionSet 管理后台更新题单
func (uc *QuestionUseCase) AdminUpdateQuestionSet(ctx context.Context, set *QuestionSet) error {
	if set.ID == 0 {
		return kratosErr.BadRequest("INVALID_ID", "题单 ID 不能为空")
	}
	existing, err := uc.questionSetRepo.GetByID(ctx, set.ID)
	if err != nil || existing == nil {
		return ErrQuestionSetNotFound
	}
	return uc.questionSetRepo.Update(ctx, set)
}

// AdminDeleteQuestionSet 管理后台删除题单（级联删除关联项）
func (uc *QuestionUseCase) AdminDeleteQuestionSet(ctx context.Context, id uint64) error {
	if id == 0 {
		return kratosErr.BadRequest("INVALID_ID", "题单 ID 不能为空")
	}
	return uc.questionSetRepo.Delete(ctx, id)
}

// AdminGetQuestionSetDetail 管理后台获取题单详情（含关联题目）
func (uc *QuestionUseCase) AdminGetQuestionSetDetail(ctx context.Context, setID uint64) (*QuestionSet, []*Question, error) {
	set, err := uc.questionSetRepo.GetByID(ctx, setID)
	if err != nil || set == nil {
		return nil, nil, ErrQuestionSetNotFound
	}
	questions, err := uc.questionSetRepo.GetQuestions(ctx, setID)
	if err != nil {
		return nil, nil, err
	}
	return set, questions, nil
}

// AdminAddQuestionsToSet 管理后台向题单添加题目
func (uc *QuestionUseCase) AdminAddQuestionsToSet(ctx context.Context, setID uint64, questionIDs []uint64) (int32, error) {
	if setID == 0 {
		return 0, kratosErr.BadRequest("INVALID_ID", "题单 ID 不能为空")
	}
	if len(questionIDs) == 0 {
		return 0, nil
	}
	existing, err := uc.questionSetRepo.GetByID(ctx, setID)
	if err != nil || existing == nil {
		return 0, ErrQuestionSetNotFound
	}
	return uc.questionSetRepo.AddQuestions(ctx, setID, questionIDs)
}

// AdminRemoveQuestionsFromSet 管理后台从题单移除题目
func (uc *QuestionUseCase) AdminRemoveQuestionsFromSet(ctx context.Context, setID uint64, questionIDs []uint64) (int32, error) {
	if setID == 0 {
		return 0, kratosErr.BadRequest("INVALID_ID", "题单 ID 不能为空")
	}
	if len(questionIDs) == 0 {
		return 0, nil
	}
	return uc.questionSetRepo.RemoveQuestions(ctx, setID, questionIDs)
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
			log.Errorf("去重检查失败: title=%s, err=%v", q.Title, err)
			continue
		}
		if exists {
			continue
		}

		// 调用 repo 创建题目
		if err := uc.questionRepo.Create(ctx, q); err != nil {
			log.Errorf("导入题目失败: title=%s, err=%v", q.Title, err)
			continue
		}
		uc.publishRAGSync(ctx, q.ID, "create", q.Title+"\n"+q.Content, buildRAGSyncMetadata(q))
		imported++
	}
	return imported, nil
}

// PipelineGenerateQuestions 流水线生成题目：调用 AI 生成并写入题库，并在每次成功写库后回调进度。
func (uc *QuestionUseCase) PipelineGenerateQuestions(ctx context.Context, req *GenerateQuestionsRequest, onProgress func(created int, question *Question) error) (int, error) {
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
			log.Errorf("pipeline 创建题目失败: title=%s, err=%v", q.Title, err)
			continue
		}
		uc.publishRAGSync(ctx, q.ID, "create", q.Title+"\n"+q.Content, buildRAGSyncMetadata(q))
		created++
		if onProgress != nil {
			if err := onProgress(created, q); err != nil {
				return created, err
			}
		}
	}

	return created, nil
}

// syncPracticeLearningArchive 将代码题或主观题的分析结果同步到学习档案中（通过 gRPC 写入 learning_archive 服务）
func (uc *QuestionUseCase) syncPracticeLearningArchive(
	ctx context.Context,
	userID uint64,
	question *Question,
	record *UserQuestionRecord,
	resp *QuizAnalyzerResponse,
) {
	if uc.learningArchiveClient == nil || question == nil || record == nil || resp == nil {
		return
	}

	// 仅对编程题和主观题同步学习档案
	if question.Type != "code" && question.Type != "subjective" {
		return
	}

	analysisJSON := resp.Feedback
	if analysisJSON == "" && resp.Suggestions == "" {
		return
	}

	// 构建错因标签
	mistakeTags := normalizePracticeArchiveTags(resp.KeyPoints, resp.IsCorrect)
	if len(mistakeTags) == 0 {
		return
	}

	strengthTagsJSON, _ := json.Marshal(normalizePracticeArchiveStrengths(resp.IsCorrect))
	mistakeTagsJSON, _ := json.Marshal(mistakeTags)
	suggestionsJSON, _ := json.Marshal(normalizePracticeArchiveSuggestions(resp.Suggestions))
	sourceRef := fmt.Sprintf("practice:%d:%d", userID, question.ID)

	entry := &LearningArchiveEntry{
		UserID:           userID,
		SourceType:       LearningArchiveSourcePracticeQuestion,
		SourceRef:        sourceRef,
		QuestionIndex:    0,
		IndustryCode:     strconv.FormatUint(question.IndustryID, 10),
		TaskPhase:        LearningPhaseDrill,
		TaskPhaseGoal:    BuildLearningPhaseGoal(LearningPhaseDrill),
		Language:         DetectQuestionLanguage(question),
		MistakeTagsJSON:  string(mistakeTagsJSON),
		StrengthTagsJSON: string(strengthTagsJSON),
		SuggestionsJSON:  string(suggestionsJSON),
		EvidenceSummary:  strings.Join(resp.KeyPoints, "；"),
	}
	if record.CreatedAt.IsZero() {
		entry.OccurredAt = nil
	} else {
		entry.OccurredAt = &record.CreatedAt
	}

	if err := uc.learningArchiveClient.WriteEntry(ctx, entry); err != nil {
		log.Warnf("同步学习档案失败: user_id=%d, question_id=%d, err=%v", userID, question.ID, err)
	}
}

// normalizePracticeArchiveTags 统一收敛练习题分析得到的错因标签
func normalizePracticeArchiveTags(keyPoints []string, isCorrect bool) []string {
	if isCorrect {
		return []string{}
	}

	result := make([]string, 0, len(keyPoints)+1)
	for _, kp := range keyPoints {
		trimmed := strings.TrimSpace(kp)
		if trimmed == "" {
			continue
		}
		// 根据关键词推断错因标签
		switch {
		case strings.Contains(trimmed, "边界"):
			result = appendUniquePracticeStrings(result, "边界条件生疏")
		case strings.Contains(trimmed, "复杂度"):
			result = appendUniquePracticeStrings(result, "复杂度意识薄弱")
		case strings.Contains(trimmed, "状态") || strings.Contains(trimmed, "定义"):
			result = appendUniquePracticeStrings(result, "状态定义不清")
		case strings.Contains(trimmed, "循环") || strings.Contains(trimmed, "索引"):
			result = appendUniquePracticeStrings(result, "循环/索引控制不稳")
		case strings.Contains(trimmed, "数据结构"):
			result = appendUniquePracticeStrings(result, "数据结构选择不当")
		case strings.Contains(trimmed, "调试"):
			result = appendUniquePracticeStrings(result, "调试路径混乱")
		case strings.Contains(trimmed, "实现") || strings.Contains(trimmed, "不完整"):
			result = appendUniquePracticeStrings(result, "代码实现不完整")
		}
	}
	if len(result) == 0 {
		result = append(result, "状态定义不清")
	}
	return result
}

// normalizePracticeArchiveStrengths 统一收敛练习题分析中的正向标签
func normalizePracticeArchiveStrengths(isCorrect bool) []string {
	result := make([]string, 0, 1)
	if isCorrect {
		result = append(result, "本题已形成可用解法")
	}
	return result
}

// normalizePracticeArchiveSuggestions 清理练习题分析中的改进建议
func normalizePracticeArchiveSuggestions(feedback string) []string {
	result := make([]string, 0)
	if trimmed := strings.TrimSpace(feedback); trimmed != "" {
		result = append(result, trimmed)
	}
	return result
}

// appendUniquePracticeStrings 追加不重复的非空字符串
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
