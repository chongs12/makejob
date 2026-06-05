package biz

import (
	"context"
	"fmt"

	kratosErr "github.com/go-kratos/kratos/v2/errors"
)

var (
	ErrQuestionNotFound = kratosErr.NotFound("QUESTION_NOT_FOUND", "题目不存在")
	ErrAlreadyFavorited = kratosErr.Conflict("ALREADY_FAVORITED", "已收藏")
	ErrFavoriteNotFound = kratosErr.NotFound("FAVORITE_NOT_FOUND", "收藏不存在")
	ErrNoteNotFound     = kratosErr.NotFound("NOTE_NOT_FOUND", "笔记不存在")
)

type QuestionUseCase struct {
	questionRepo  QuestionRepo
	recordRepo    RecordRepo
	favoriteRepo  FavoriteRepo
	noteRepo      NoteRepo
	categoryRepo  CategoryRepo
	industryRepo  IndustryRepo
	quizAnalyzer  QuizAnalyzerClient
}

func NewQuestionUseCase(
	questionRepo QuestionRepo,
	recordRepo RecordRepo,
	favoriteRepo FavoriteRepo,
	noteRepo NoteRepo,
	categoryRepo CategoryRepo,
	industryRepo IndustryRepo,
	quizAnalyzer QuizAnalyzerClient,
) *QuestionUseCase {
	return &QuestionUseCase{
		questionRepo: questionRepo,
		recordRepo:   recordRepo,
		favoriteRepo: favoriteRepo,
		noteRepo:     noteRepo,
		categoryRepo: categoryRepo,
		industryRepo: industryRepo,
		quizAnalyzer: quizAnalyzer,
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

func (uc *QuestionUseCase) GetPracticeRecommendations(ctx context.Context, userID uint64) ([]*Question, string, error) {
	// 基于错题推荐薄弱知识点的题目
	wrong, _, err := uc.recordRepo.GetWrongQuestions(ctx, userID, 1, 10)
	if err != nil {
		return nil, "", err
	}

	if len(wrong) == 0 {
		return nil, "暂无推荐", nil
	}

	// 获取错题对应的题目
	var questions []*Question
	for _, w := range wrong {
		q, err := uc.questionRepo.GetByID(ctx, w.QuestionID)
		if err == nil {
			questions = append(questions, q)
		}
	}

	reason := fmt.Sprintf("基于你最近 %d 道错题的薄弱知识点推荐", len(wrong))
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

func (uc *QuestionUseCase) ListNotes(ctx context.Context, userID, questionID uint64, page, pageSize int32) ([]*UserNote, int64, error) {
	return uc.noteRepo.ListByUser(ctx, userID, questionID, page, pageSize)
}

func (uc *QuestionUseCase) GetRandomExam(ctx context.Context, industryCode string, questionCount int32) ([]*Question, error) {
	if questionCount <= 0 {
		questionCount = 10
	}
	filter := &QuestionFilter{
		IndustryCode: industryCode,
	}
	questions, _, err := uc.questionRepo.List(ctx, filter, 1, questionCount)
	if err != nil {
		return nil, err
	}
	return questions, nil
}
