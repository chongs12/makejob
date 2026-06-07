package service

import (
	"context"

	emptypb "google.golang.org/protobuf/types/known/emptypb"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"

	questionv1 "makejob/api/makejob/question/v1"
	sharedv1 "makejob/api/makejob/shared/v1"
	"makejob/app/question/internal/biz"
)

type QuestionService struct {
	questionv1.UnimplementedQuestionServiceServer
	uc *biz.QuestionUseCase
}

func NewQuestionService(uc *biz.QuestionUseCase) *QuestionService {
	return &QuestionService{uc: uc}
}

func (s *QuestionService) ListQuestions(ctx context.Context, req *questionv1.ListQuestionsRequest) (*questionv1.ListQuestionsResponse, error) {
	filter := &biz.QuestionFilter{
		IndustryCode: req.IndustryCode,
		CategoryID:   req.CategoryId,
		Difficulty:   req.Difficulty,
		Keyword:      req.Keyword,
	}
	var page, pageSize int32 = 1, 20
	if req.Page != nil {
		page = req.Page.Page
		pageSize = req.Page.PageSize
	}

	questions, total, err := s.uc.ListQuestions(ctx, filter, page, pageSize)
	if err != nil {
		return nil, err
	}

	items := make([]*questionv1.QuestionSummary, len(questions))
	for i, q := range questions {
		items[i] = &questionv1.QuestionSummary{
			Id:           q.ID,
			Title:        q.Title,
			Difficulty:   q.Difficulty,
			Type:         q.Type,
			IndustryCode: q.IndustryCode,
			CategoryName: q.CategoryName,
		}
	}

	return &questionv1.ListQuestionsResponse{
		Questions: items,
		PageResult: &sharedv1.PageResult{
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		},
	}, nil
}

func (s *QuestionService) GetQuestion(ctx context.Context, req *questionv1.GetQuestionRequest) (*questionv1.QuestionDetail, error) {
	q, err := s.uc.GetQuestion(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &questionv1.QuestionDetail{
		Id:              q.ID,
		Title:           q.Title,
		Content:         q.Content,
		Difficulty:      q.Difficulty,
		Type:            q.Type,
		IndustryCode:    q.IndustryCode,
		ReferenceAnswer: q.ReferenceAnswer,
		Explanation:     q.Explanation,
	}, nil
}

func (s *QuestionService) ListCategories(ctx context.Context, req *questionv1.ListCategoriesRequest) (*questionv1.CategoryTreeResponse, error) {
	categories, err := s.uc.ListCategories(ctx, req.IndustryCode)
	if err != nil {
		return nil, err
	}

	// Build tree: top-level nodes (ParentID == 0) with children
	nodeMap := make(map[uint64]*questionv1.CategoryNode)
	for _, c := range categories {
		nodeMap[c.ID] = &questionv1.CategoryNode{
			Id:   c.ID,
			Name: c.Name,
		}
	}

	var roots []*questionv1.CategoryNode
	for _, c := range categories {
		node := nodeMap[c.ID]
		if c.ParentID == 0 {
			roots = append(roots, node)
		} else if parent, ok := nodeMap[c.ParentID]; ok {
			parent.Children = append(parent.Children, node)
		} else {
			roots = append(roots, node)
		}
	}

	return &questionv1.CategoryTreeResponse{
		Categories: roots,
	}, nil
}

func (s *QuestionService) ListIndustries(ctx context.Context, _ *emptypb.Empty) (*questionv1.IndustryListResponse, error) {
	industries, err := s.uc.ListIndustries(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]*questionv1.IndustryInfo, len(industries))
	for i, ind := range industries {
		items[i] = &questionv1.IndustryInfo{
			Code: ind.Code,
			Name: ind.Name,
			Icon: ind.Icon,
		}
	}

	return &questionv1.IndustryListResponse{
		Industries: items,
	}, nil
}

func (s *QuestionService) SubmitAnswer(ctx context.Context, req *questionv1.SubmitAnswerRequest) (*questionv1.SubmitAnswerResponse, error) {
	resp, err := s.uc.SubmitAnswer(ctx, req.QuestionId, req.UserId, req.Answer, req.Language)
	if err != nil {
		return nil, err
	}

	return &questionv1.SubmitAnswerResponse{
		IsCorrect:     resp.IsCorrect,
		Score:         resp.Score,
		Feedback:      resp.Feedback,
		CorrectAnswer: resp.CorrectAnswer,
		Explanation:   resp.Suggestions,
		KeyPoints:     resp.KeyPoints,
	}, nil
}

func (s *QuestionService) RunCode(ctx context.Context, req *questionv1.RunCodeRequest) (*questionv1.RunCodeResponse, error) {
	resp, err := s.uc.RunCode(ctx, req.QuestionId, req.Language, req.Code)
	if err != nil {
		return nil, err
	}
	return &questionv1.RunCodeResponse{
		Success:         resp.Success,
		Output:          resp.Output,
		Error:           resp.Error,
		TestCasesPassed: resp.TestCasesPassed,
		TotalTestCases:  resp.TotalTestCases,
		ExecutionTimeMs: resp.ExecutionTimeMs,
	}, nil
}

func (s *QuestionService) CreateFavorite(ctx context.Context, req *questionv1.CreateFavoriteRequest) (*emptypb.Empty, error) {
	if err := s.uc.CreateFavorite(ctx, req.UserId, req.QuestionId); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *QuestionService) DeleteFavorite(ctx context.Context, req *questionv1.DeleteFavoriteRequest) (*emptypb.Empty, error) {
	if err := s.uc.DeleteFavorite(ctx, req.UserId, req.QuestionId); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *QuestionService) ListFavorites(ctx context.Context, req *questionv1.ListFavoritesRequest) (*questionv1.FavoriteListResponse, error) {
	var page, pageSize int32 = 1, 20
	if req.Page != nil {
		page = req.Page.Page
		pageSize = req.Page.PageSize
	}

	questions, total, err := s.uc.ListFavorites(ctx, req.UserId, page, pageSize)
	if err != nil {
		return nil, err
	}

	items := make([]*questionv1.QuestionSummary, len(questions))
	for i, q := range questions {
		items[i] = &questionv1.QuestionSummary{
			Id:           q.ID,
			Title:        q.Title,
			Difficulty:   q.Difficulty,
			Type:         q.Type,
			IndustryCode: q.IndustryCode,
		}
	}

	return &questionv1.FavoriteListResponse{
		Questions: items,
		PageResult: &sharedv1.PageResult{
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		},
	}, nil
}

func (s *QuestionService) CreateNote(ctx context.Context, req *questionv1.CreateNoteRequest) (*questionv1.NoteResponse, error) {
	note, err := s.uc.CreateNote(ctx, req.UserId, req.QuestionId, req.Content)
	if err != nil {
		return nil, err
	}

	return &questionv1.NoteResponse{
		Id:         note.ID,
		QuestionId: note.QuestionID,
		Content:    note.Content,
		CreatedAt:  timestamppb.New(note.CreatedAt),
		UpdatedAt:  timestamppb.New(note.UpdatedAt),
	}, nil
}

func (s *QuestionService) UpdateNote(ctx context.Context, req *questionv1.UpdateNoteRequest) (*questionv1.NoteResponse, error) {
	note, err := s.uc.UpdateNote(ctx, req.Id, req.UserId, req.Content)
	if err != nil {
		return nil, err
	}

	return &questionv1.NoteResponse{
		Id:        note.ID,
		Content:   note.Content,
		UpdatedAt: timestamppb.New(note.UpdatedAt),
	}, nil
}

func (s *QuestionService) ListNotes(ctx context.Context, req *questionv1.ListNotesRequest) (*questionv1.NoteListResponse, error) {
	var page, pageSize int32 = 1, 20
	if req.Page != nil {
		page = req.Page.Page
		pageSize = req.Page.PageSize
	}

	notes, total, err := s.uc.ListNotes(ctx, req.UserId, req.QuestionId, page, pageSize)
	if err != nil {
		return nil, err
	}

	items := make([]*questionv1.NoteResponse, len(notes))
	for i, n := range notes {
		items[i] = &questionv1.NoteResponse{
			Id:         n.ID,
			QuestionId: n.QuestionID,
			Content:    n.Content,
			CreatedAt:  timestamppb.New(n.CreatedAt),
			UpdatedAt:  timestamppb.New(n.UpdatedAt),
		}
	}

	return &questionv1.NoteListResponse{
		Notes: items,
		PageResult: &sharedv1.PageResult{
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		},
	}, nil
}

func (s *QuestionService) GetPracticeRecommendations(ctx context.Context, req *questionv1.PracticeRecommendationRequest) (*questionv1.PracticeRecommendationResponse, error) {
	// FIX Q4: 从 request 读取 interview_id，支持面试驱动推荐
	questions, reason, err := s.uc.GetPracticeRecommendations(ctx, req.GetUserId(), req.GetInterviewId())
	if err != nil {
		return nil, err
	}

	items := make([]*questionv1.RecommendedQuestion, len(questions))
	for i, q := range questions {
		items[i] = &questionv1.RecommendedQuestion{
			QuestionId: q.ID,
			Title:      q.Title,
			Difficulty: q.Difficulty,
		}
	}

	return &questionv1.PracticeRecommendationResponse{
		Questions: items,
		Reason:    reason,
	}, nil
}

func (s *QuestionService) GetWrongQuestions(ctx context.Context, req *questionv1.WrongQuestionRequest) (*questionv1.WrongQuestionListResponse, error) {
	var page, pageSize int32 = 1, 20
	if req.Page != nil {
		page = req.Page.Page
		pageSize = req.Page.PageSize
	}

	wrongQuestions, total, err := s.uc.GetWrongQuestions(ctx, req.UserId, page, pageSize)
	if err != nil {
		return nil, err
	}

	items := make([]*questionv1.WrongQuestionEntry, len(wrongQuestions))
	for i, w := range wrongQuestions {
		entry := &questionv1.WrongQuestionEntry{
			QuestionId: w.QuestionID,
			Title:      w.Title,
			WrongCount: w.WrongCount,
			LastAnswer: w.LastAnswer,
		}
		if !w.LastWrongAt.IsZero() {
			entry.LastWrongAt = timestamppb.New(w.LastWrongAt)
		}
		items[i] = entry
	}

	return &questionv1.WrongQuestionListResponse{
		Entries: items,
		PageResult: &sharedv1.PageResult{
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		},
	}, nil
}

func (s *QuestionService) GetUserPracticeStats(ctx context.Context, req *questionv1.UserIDRequest) (*questionv1.UserPracticeStats, error) {
	totalAnswered, totalCorrect, accuracy, categoryStats, err := s.uc.GetUserPracticeStats(ctx, req.UserId)
	if err != nil {
		return nil, err
	}

	items := make([]*questionv1.CategoryStat, len(categoryStats))
	for i, cs := range categoryStats {
		items[i] = &questionv1.CategoryStat{
			CategoryName: cs.CategoryName,
			Answered:     cs.Answered,
			Correct:      cs.Correct,
			Accuracy:     cs.Accuracy,
		}
	}

	return &questionv1.UserPracticeStats{
		TotalAnswered: totalAnswered,
		TotalCorrect:  totalCorrect,
		Accuracy:      accuracy,
		CategoryStats: items,
	}, nil
}

func (s *QuestionService) GetRandomExam(ctx context.Context, req *questionv1.RandomExamRequest) (*questionv1.ExamResponse, error) {
	questions, err := s.uc.GetRandomExam(ctx, req.IndustryCode, req.QuestionCount)
	if err != nil {
		return nil, err
	}

	items := make([]*questionv1.QuestionDetail, len(questions))
	for i, q := range questions {
		items[i] = &questionv1.QuestionDetail{
			Id:           q.ID,
			Title:        q.Title,
			Content:      q.Content,
			Difficulty:   q.Difficulty,
			Type:         q.Type,
			IndustryCode: q.IndustryCode,
		}
	}

	return &questionv1.ExamResponse{
		Questions:        items,
		TimeLimitMinutes: int32(len(questions)) * 5, // 5 minutes per question
	}, nil
}

// DeleteNote 删除笔记（P3-2）
func (s *QuestionService) DeleteNote(ctx context.Context, req *questionv1.DeleteNoteRequest) (*emptypb.Empty, error) {
	if err := s.uc.DeleteNote(ctx, req.NoteId, req.UserId); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// GenerateTimedExam 生成限时考试（P3-3）
func (s *QuestionService) GenerateTimedExam(ctx context.Context, req *questionv1.GenerateTimedExamRequest) (*questionv1.GenerateTimedExamResponse, error) {
	exam, questions, err := s.uc.GenerateTimedExam(ctx, req.UserId, req.IndustryCode, req.QuestionCount, req.TimeLimitMinutes)
	if err != nil {
		return nil, err
	}

	items := make([]*questionv1.QuestionDetail, len(questions))
	for i, q := range questions {
		items[i] = &questionv1.QuestionDetail{
			Id:           q.ID,
			Title:        q.Title,
			Content:      q.Content,
			Difficulty:   q.Difficulty,
			Type:         q.Type,
			IndustryCode: q.IndustryCode,
		}
	}

	resp := &questionv1.GenerateTimedExamResponse{
		ExamId:           exam.ID,
		Questions:        items,
		TimeLimitMinutes: exam.TimeLimitMin,
	}
	if !exam.StartTime.IsZero() {
		resp.StartedAt = timestamppb.New(exam.StartTime)
	}
	if !exam.EndTime.IsZero() {
		resp.ExpiresAt = timestamppb.New(exam.EndTime)
	}
	return resp, nil
}

// SubmitExam 提交考试答案并评分（P3-4）
func (s *QuestionService) SubmitExam(ctx context.Context, req *questionv1.SubmitExamRequest) (*questionv1.SubmitExamResponse, error) {
	examResult, err := s.uc.SubmitExam(ctx, req.ExamId, req.UserId, req.Answers)
	if err != nil {
		return nil, err
	}

	results := make([]*questionv1.QuestionResult, len(examResult.QuestionResults))
	for i, qr := range examResult.QuestionResults {
		results[i] = &questionv1.QuestionResult{
			QuestionId: qr.QuestionID,
			IsCorrect:  qr.IsCorrect,
			Score:      qr.Score,
			Feedback:   qr.Feedback,
		}
	}

	return &questionv1.SubmitExamResponse{
		ExamId:          examResult.ExamID,
		TotalScore:      examResult.TotalScore,
		CorrectCount:    examResult.CorrectCount,
		TotalCount:      examResult.TotalQuestions,
		QuestionResults: results,
	}, nil
}

// ListQuestionSets 获取题集列表（P3-5）
func (s *QuestionService) ListQuestionSets(ctx context.Context, req *questionv1.ListQuestionSetsRequest) (*questionv1.ListQuestionSetsResponse, error) {
	var page, pageSize int32 = 1, 20
	if req.Page != nil {
		page = req.Page.Page
		pageSize = req.Page.PageSize
	}

	sets, total, err := s.uc.ListQuestionSets(ctx, req.IndustryCode, page, pageSize)
	if err != nil {
		return nil, err
	}

	items := make([]*questionv1.QuestionSetSummary, len(sets))
	for i, set := range sets {
		items[i] = &questionv1.QuestionSetSummary{
			Id:            set.ID,
			Title:         set.Name,
			Description:   set.Description,
			QuestionCount: set.QuestionCount,
		}
	}

	return &questionv1.ListQuestionSetsResponse{
		Items: items,
		PageResult: &sharedv1.PageResult{
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		},
	}, nil
}

// GetQuestionSetDetail 获取题集详情（P3-5）
func (s *QuestionService) GetQuestionSetDetail(ctx context.Context, req *questionv1.GetQuestionSetDetailRequest) (*questionv1.QuestionSetDetail, error) {
	set, questions, err := s.uc.GetQuestionSetDetail(ctx, req.SetId)
	if err != nil {
		return nil, err
	}

	info := &questionv1.QuestionSetSummary{
		Id:            set.ID,
		Title:         set.Name,
		Description:   set.Description,
		QuestionCount: set.QuestionCount,
	}

	items := make([]*questionv1.QuestionSummary, len(questions))
	for i, q := range questions {
		items[i] = &questionv1.QuestionSummary{
			Id:           q.ID,
			Title:        q.Title,
			Difficulty:   q.Difficulty,
			Type:         q.Type,
			IndustryCode: q.IndustryCode,
		}
	}

	return &questionv1.QuestionSetDetail{
		Info:      info,
		Questions: items,
	}, nil
}

// ListMistakeTopics 获取用户错题知识点聚合（P3-6）
func (s *QuestionService) ListMistakeTopics(ctx context.Context, req *questionv1.ListMistakeTopicsRequest) (*questionv1.ListMistakeTopicsResponse, error) {
	topics, err := s.uc.ListMistakeTopics(ctx, req.UserId)
	if err != nil {
		return nil, err
	}

	items := make([]*questionv1.MistakeTopic, len(topics))
	for i, t := range topics {
		var errorRate float64
		if t.TotalCount > 0 {
			errorRate = float64(t.WrongCount) / float64(t.TotalCount)
		}
		items[i] = &questionv1.MistakeTopic{
			Topic:         t.CategoryName,
			ErrorCount:    t.WrongCount,
			TotalAttempts: t.TotalCount,
			ErrorRate:     errorRate,
		}
	}

	return &questionv1.ListMistakeTopicsResponse{
		Topics: items,
	}, nil
}
