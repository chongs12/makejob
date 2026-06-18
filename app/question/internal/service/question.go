package service

import (
	"context"
	"strings"

	kratosErr "github.com/go-kratos/kratos/v2/errors"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"

	questionv1 "makejob/api/makejob/question/v1"
	sharedv1 "makejob/api/makejob/shared/v1"
	"makejob/app/question/internal/biz"
	"makejob/pkg/auth"
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
		return nil, toGRPCError(err)
	}

	// 批量查询当前用户已答题的题目 ID
	answeredMap := make(map[uint64]bool)
	if userID := auth.GetUserIDFromContext(ctx); userID > 0 {
		questionIDs := make([]uint64, len(questions))
		for i, q := range questions {
			questionIDs[i] = q.ID
		}
		if m, err := s.uc.GetAnsweredQuestionIDs(ctx, userID, questionIDs); err == nil {
			answeredMap = m
		}
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
			CategoryId:   q.CategoryID,
			IndustryId:   q.IndustryID,
			Tags:         q.Tags,
			IsAnswered:   answeredMap[q.ID],
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
		return nil, toGRPCError(err)
	}

	// 查询当前用户是否收藏/已答该题目
	var isFavorited bool
	var isAnswered bool
	var userNote *questionv1.NoteResponse
	if userID := auth.GetUserIDFromContext(ctx); userID > 0 {
		isFavorited = s.uc.IsFavorited(ctx, userID, req.Id)
		// 查询是否已答
		if answeredMap, ansErr := s.uc.GetAnsweredQuestionIDs(ctx, userID, []uint64{req.Id}); ansErr == nil {
			isAnswered = answeredMap[req.Id]
		}
		// 查询用户笔记
		if note, noteErr := s.uc.GetUserNote(ctx, userID, req.Id); noteErr == nil && note != nil {
			var questionID uint64
			if note.QuestionID != nil {
				questionID = *note.QuestionID
			}
			userNote = &questionv1.NoteResponse{
				Id:        note.ID,
				QuestionId: questionID,
				Content:   note.Content,
				CreatedAt: timestamppb.New(note.CreatedAt),
				UpdatedAt: timestamppb.New(note.UpdatedAt),
			}
		}
	}

	return &questionv1.QuestionDetail{
		Id:                 q.ID,
		Title:              q.Title,
		Content:            q.Content,
		Difficulty:         q.Difficulty,
		Type:               q.Type,
		IndustryCode:       q.IndustryCode,
		Category:           &questionv1.CategoryInfo{Id: q.CategoryID, Name: q.CategoryName},
		Tags:               q.Tags,
		StarterCode:        q.StarterCode,
		Language:           q.Language,
		EvaluationMode:     q.EvaluationMode,
		ReferenceAnswer:    q.ReferenceAnswer,
		Explanation:        q.Explanation,
		CreatedAt:          timestamppb.New(q.CreatedAt),
		OptionsJson:        q.OptionsJSON,
		Answer:             q.Answer,
		SolutionJson:       q.SolutionJSON,
		JudgeConfigJson:    q.JudgeConfigJSON,
		AnswerTemplateJson: q.AnswerTemplateJSON,
		IsFavorited:        isFavorited,
		IsAnswered:         isAnswered,
		UserNote:           userNote,
	}, nil
}

func (s *QuestionService) ListCategories(ctx context.Context, req *questionv1.ListCategoriesRequest) (*questionv1.CategoryTreeResponse, error) {
	// categories 表存的是 industry_id（整数外键），需要先将 code 解析为 ID
	var industryID uint64
	if req.IndustryCode != "" {
		industry, err := s.uc.GetIndustryByCode(ctx, req.IndustryCode)
		if err == nil && industry != nil {
			industryID = industry.ID
		}
	}
	categories, err := s.uc.ListCategories(ctx, industryID)
	if err != nil {
		return nil, toGRPCError(err)
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
		return nil, toGRPCError(err)
	}

	items := make([]*questionv1.IndustryInfo, len(industries))
	for i, ind := range industries {
		items[i] = &questionv1.IndustryInfo{
			Id:   ind.ID,
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
		return nil, toGRPCError(err)
	}

	return &questionv1.SubmitAnswerResponse{
		IsCorrect:      resp.IsCorrect,
		Score:          resp.Score,
		Feedback:       resp.Feedback,
		CorrectAnswer:  resp.CorrectAnswer,
		Explanation:    resp.Suggestions,
		KeyPoints:      resp.KeyPoints,
		EvaluationMode: resp.EvaluationMode,
		JudgeSummary:   toProtoJudgeSummary(resp.JudgeSummary),
	}, nil
}

func (s *QuestionService) RunCode(ctx context.Context, req *questionv1.RunCodeRequest) (*questionv1.RunCodeResponse, error) {
	resp, err := s.uc.RunCode(ctx, req.QuestionId, req.Language, req.Code)
	if err != nil {
		return nil, toGRPCError(err)
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

func (s *QuestionService) CreateFavorite(ctx context.Context, req *questionv1.CreateFavoriteRequest) (*questionv1.FavoriteResponse, error) {
	if err := s.uc.CreateFavorite(ctx, req.UserId, req.QuestionId); err != nil {
		return nil, toGRPCError(err)
	}
	return &questionv1.FavoriteResponse{IsFavorited: true}, nil
}

func (s *QuestionService) DeleteFavorite(ctx context.Context, req *questionv1.DeleteFavoriteRequest) (*questionv1.FavoriteResponse, error) {
	if err := s.uc.DeleteFavorite(ctx, req.UserId, req.QuestionId); err != nil {
		return nil, toGRPCError(err)
	}
	return &questionv1.FavoriteResponse{IsFavorited: false}, nil
}

func (s *QuestionService) ListFavorites(ctx context.Context, req *questionv1.ListFavoritesRequest) (*questionv1.FavoriteListResponse, error) {
	var page, pageSize int32 = 1, 20
	if req.Page != nil {
		page = req.Page.Page
		pageSize = req.Page.PageSize
	}

	questions, total, err := s.uc.ListFavorites(ctx, req.UserId, page, pageSize)
	if err != nil {
		return nil, toGRPCError(err)
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
		return nil, toGRPCError(err)
	}

	var questionID uint64
	if note.QuestionID != nil {
		questionID = *note.QuestionID
	}

	return &questionv1.NoteResponse{
		Id:         note.ID,
		QuestionId: questionID,
		Content:    note.Content,
		CreatedAt:  timestamppb.New(note.CreatedAt),
		UpdatedAt:  timestamppb.New(note.UpdatedAt),
	}, nil
}

func (s *QuestionService) UpdateNote(ctx context.Context, req *questionv1.UpdateNoteRequest) (*questionv1.NoteResponse, error) {
	note, err := s.uc.UpdateNote(ctx, req.Id, req.UserId, req.Content)
	if err != nil {
		return nil, toGRPCError(err)
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
		return nil, toGRPCError(err)
	}

	items := make([]*questionv1.NoteResponse, len(notes))
	for i, n := range notes {
		var questionID uint64
		if n.QuestionID != nil {
			questionID = *n.QuestionID
		}
		items[i] = &questionv1.NoteResponse{
			Id:         n.ID,
			QuestionId: questionID,
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
		return nil, toGRPCError(err)
	}

	items := make([]*questionv1.RecommendedQuestion, len(questions))
	focusTagSet := make(map[string]struct{})
	focusTags := make([]string, 0)
	for i, q := range questions {
		tagsCSV := strings.Join(q.Tags, ",")
		focusTag := ""
		if len(q.Tags) > 0 {
			focusTag = q.Tags[0]
		}
		if focusTag != "" {
			if _, ok := focusTagSet[focusTag]; !ok {
				focusTagSet[focusTag] = struct{}{}
				focusTags = append(focusTags, focusTag)
			}
		}
		items[i] = &questionv1.RecommendedQuestion{
			QuestionId:          q.ID,
			Title:               q.Title,
			Difficulty:          q.Difficulty,
			Type:                q.Type,
			CategoryId:          q.CategoryID,
			IndustryId:          q.IndustryID,
			CategoryName:        q.CategoryName,
			Tags:                tagsCSV,
			RecommendReason:     reason,
			FocusTag:            focusTag,
			TopicTitle:          focusTag,
			RelatedQuestionSets: []string{},
			RecommendedActions:  []string{},
			RecommendationMode:  "topic",
			SourceType:          "learning_archive",
			Priority:            int32(i + 1),
		}
	}

	return &questionv1.PracticeRecommendationResponse{
		Questions: items,
		Reason:    reason,
		FocusTags: focusTags,
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
		return nil, toGRPCError(err)
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
	totalAnswered, totalCorrect, accuracy, categoryStats, todayCount, err := s.uc.GetUserPracticeStats(ctx, req.UserId)
	if err != nil {
		return nil, toGRPCError(err)
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

	correctCount := totalCorrect
	wrongCount := totalAnswered - totalCorrect
	if wrongCount < 0 {
		wrongCount = 0
	}
	return &questionv1.UserPracticeStats{
		TotalAnswered: totalAnswered,
		TotalCorrect:  totalCorrect,
		Accuracy:      accuracy,
		CategoryStats: items,
		CorrectCount:  correctCount,
		WrongCount:    wrongCount,
		AccuracyRate:  accuracy,
		TodayCount:    todayCount,
	}, nil
}

// GetRandomExam 根据请求中的筛选条件随机返回一组题目。
func (s *QuestionService) GetRandomExam(ctx context.Context, req *questionv1.RandomExamRequest) (*questionv1.ExamResponse, error) {
	exam, questions, err := s.uc.GetRandomExam(ctx, &biz.GetRandomExamRequest{
		UserID:        req.UserId,
		IndustryID:    req.IndustryId,
		IndustryCode:  req.IndustryCode,
		CategoryID:    req.CategoryId,
		Difficulty:    req.Difficulty,
		QuestionCount: req.QuestionCount,
	})
	if err != nil {
		return nil, toGRPCError(err)
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
		TimeLimitMinutes: exam.TimeLimitMin,
		ExamId:           exam.ID,
	}, nil
}

// DeleteNote 删除笔记（P3-2）
func (s *QuestionService) DeleteNote(ctx context.Context, req *questionv1.DeleteNoteRequest) (*emptypb.Empty, error) {
	if err := s.uc.DeleteNote(ctx, req.NoteId, req.UserId); err != nil {
		return nil, toGRPCError(err)
	}
	return &emptypb.Empty{}, nil
}

// GenerateTimedExam 生成限时考试（P3-3）
func (s *QuestionService) GenerateTimedExam(ctx context.Context, req *questionv1.GenerateTimedExamRequest) (*questionv1.GenerateTimedExamResponse, error) {
	exam, questions, err := s.uc.GenerateTimedExam(ctx, &biz.GenerateTimedExamRequest{
		UserID:           req.UserId,
		IndustryID:       req.IndustryId,
		IndustryCode:     req.IndustryCode,
		CategoryID:       req.CategoryId,
		Difficulty:       req.Difficulty,
		QuestionCount:    req.QuestionCount,
		TimeLimitMinutes: req.TimeLimitMinutes,
	})
	if err != nil {
		return nil, toGRPCError(err)
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
		return nil, toGRPCError(err)
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
		return nil, toGRPCError(err)
	}

	items := make([]*questionv1.QuestionSetSummary, len(sets))
	for i, set := range sets {
		summary := &questionv1.QuestionSetSummary{
			Id:            set.ID,
			Title:         set.Name,
			Description:   set.Description,
			QuestionCount: set.QuestionCount,
			Slug:          slugify(set.Name),
			Questions:     []*questionv1.QuestionSetPreview{},
			FocusTags:     []string{},
		}

		// 加载题集内的题目预览，供前端列表卡片展示
		questions, err := s.uc.GetQuestionSetQuestions(ctx, set.ID)
		if err == nil && len(questions) > 0 {
			tagSet := make(map[string]struct{})
			previews := make([]*questionv1.QuestionSetPreview, 0, len(questions))
			for _, q := range questions {
				previews = append(previews, &questionv1.QuestionSetPreview{
					Id:         q.ID,
					Title:      q.Title,
					Type:       q.Type,
					Difficulty: q.Difficulty,
				})
				for _, tag := range q.Tags {
					if tag != "" {
						tagSet[tag] = struct{}{}
					}
				}
			}
			summary.Questions = previews
			focusTags := make([]string, 0, len(tagSet))
			for tag := range tagSet {
				focusTags = append(focusTags, tag)
			}
			summary.FocusTags = focusTags
		}

		items[i] = summary
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
		return nil, toGRPCError(err)
	}

	info := &questionv1.QuestionSetSummary{
		Id:            set.ID,
		Title:         set.Name,
		Description:   set.Description,
		QuestionCount: set.QuestionCount,
		Slug:          slugify(set.Name),
	}

	tagSet := make(map[string]struct{})
	items := make([]*questionv1.QuestionSummary, len(questions))
	for i, q := range questions {
		items[i] = &questionv1.QuestionSummary{
			Id:           q.ID,
			Title:        q.Title,
			Difficulty:   q.Difficulty,
			Type:         q.Type,
			IndustryCode: q.IndustryCode,
		}
		for _, tag := range q.Tags {
			if tag != "" {
				tagSet[tag] = struct{}{}
			}
		}
	}
	focusTags := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		focusTags = append(focusTags, tag)
	}
	info.FocusTags = focusTags

	return &questionv1.QuestionSetDetail{
		Info:      info,
		Questions: items,
	}, nil
}

// slugify 将标题转换为 URL 友好的 slug（小写、空格转连字符）。
func slugify(title string) string {
	s := strings.ToLower(strings.TrimSpace(title))
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")
	// 移除连续的连字符
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return s
}

// ListMistakeTopics 获取用户错题知识点聚合（P3-6）
func (s *QuestionService) ListMistakeTopics(ctx context.Context, req *questionv1.ListMistakeTopicsRequest) (*questionv1.ListMistakeTopicsResponse, error) {
	topics, err := s.uc.ListMistakeTopics(ctx, req.UserId)
	if err != nil {
		return nil, toGRPCError(err)
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

// GetMistakeTopic 根据错因专题编码查询单个专题卡片详情（委托 learning_archive 服务）。
func (s *QuestionService) GetMistakeTopic(ctx context.Context, req *questionv1.GetMistakeTopicRequest) (*questionv1.MistakeTopicCard, error) {
	topic, err := s.uc.GetMistakeTopicCard(ctx, req.GetCode())
	if err != nil {
		return nil, err
	}
	if topic == nil {
		return nil, kratosErr.NotFound("MISTAKE_TOPIC_NOT_FOUND", "错因专题不存在")
	}

	return &questionv1.MistakeTopicCard{
		Code:                topic.Code,
		Tag:                 topic.Tag,
		Title:               topic.Title,
		ProblemPattern:      topic.ProblemPattern,
		RootCauses:          topic.RootCauses,
		SelfCheckList:       topic.SelfCheckList,
		PracticeDirections:  topic.PracticeDirections,
		RecommendedActions:  topic.RecommendedActions,
		RelatedQuestionSets: topic.RelatedQuestionSets,
	}, nil
}

// AdminListQuestions 管理后台查询题目列表，复用现有题库过滤逻辑。
func (s *QuestionService) AdminListQuestions(ctx context.Context, req *questionv1.AdminListQuestionsRequest) (*questionv1.AdminListQuestionsResponse, error) {
	page, pageSize := int32(1), int32(20)
	if req.GetPage() != nil {
		page = req.GetPage().GetPage()
		pageSize = req.GetPage().GetPageSize()
	}

	questions, total, err := s.uc.ListQuestions(ctx, &biz.QuestionFilter{
		Keyword:    req.GetKeyword(),
		Difficulty: req.GetDifficulty(),
	}, page, pageSize)
	if err != nil {
		return nil, toGRPCError(err)
	}

	items := make([]*questionv1.AdminQuestionInfo, len(questions))
	for i, question := range questions {
		items[i] = toProtoAdminQuestion(question)
	}

	return &questionv1.AdminListQuestionsResponse{
		Questions: items,
		PageResult: &sharedv1.PageResult{
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		},
	}, nil
}

// AdminCreateQuestion 管理后台创建题目。
func (s *QuestionService) AdminCreateQuestion(ctx context.Context, req *questionv1.AdminCreateQuestionRequest) (*questionv1.AdminCreateQuestionResponse, error) {
	question := &biz.Question{
		CategoryID:         req.GetCategoryId(),
		IndustryID:         req.GetIndustryId(),
		Type:               req.GetType(),
		Difficulty:         req.GetDifficulty(),
		Title:              req.GetTitle(),
		Content:            req.GetContent(),
		OptionsJSON:        req.GetOptionsJson(),
		Answer:             req.GetAnswer(),
		ReferenceAnswer:    req.GetAnswer(),
		Explanation:        req.GetExplanation(),
		SolutionJSON:       req.GetSolutionJson(),
		JudgeConfigJSON:    req.GetJudgeConfigJson(),
		AnswerTemplateJSON: req.GetAnswerTemplateJson(),
		Tags:               splitProtoTags(req.GetTags()),
		IsActive:           req.GetIsActive(),
	}
	if err := s.uc.CreateQuestion(ctx, question); err != nil {
		return nil, toGRPCError(err)
	}
	return &questionv1.AdminCreateQuestionResponse{Id: question.ID}, nil
}

// AdminUpdateQuestion 管理后台更新题目。
func (s *QuestionService) AdminUpdateQuestion(ctx context.Context, req *questionv1.AdminUpdateQuestionRequest) (*questionv1.AdminUpdateQuestionResponse, error) {
	isActive := false
	if req.IsActive != nil {
		isActive = req.GetIsActive()
	} else {
		existing, err := s.uc.GetQuestion(ctx, req.GetId())
		if err != nil {
			return nil, toGRPCError(err)
		}
		isActive = existing.IsActive
	}

	question := &biz.Question{
		ID:                 req.GetId(),
		CategoryID:         req.GetCategoryId(),
		IndustryID:         req.GetIndustryId(),
		Type:               req.GetType(),
		Difficulty:         req.GetDifficulty(),
		Title:              req.GetTitle(),
		Content:            req.GetContent(),
		OptionsJSON:        req.GetOptionsJson(),
		Answer:             req.GetAnswer(),
		ReferenceAnswer:    req.GetAnswer(),
		Explanation:        req.GetExplanation(),
		SolutionJSON:       req.GetSolutionJson(),
		JudgeConfigJSON:    req.GetJudgeConfigJson(),
		AnswerTemplateJSON: req.GetAnswerTemplateJson(),
		Tags:               splitProtoTags(req.GetTags()),
		IsActive:           isActive,
	}
	if err := s.uc.UpdateQuestion(ctx, question); err != nil {
		return nil, toGRPCError(err)
	}
	return &questionv1.AdminUpdateQuestionResponse{}, nil
}

// AdminDeleteQuestion 管理后台删除题目。
func (s *QuestionService) AdminDeleteQuestion(ctx context.Context, req *questionv1.AdminDeleteQuestionRequest) (*questionv1.AdminDeleteQuestionResponse, error) {
	if err := s.uc.DeleteQuestion(ctx, req.GetId()); err != nil {
		return nil, toGRPCError(err)
	}
	return &questionv1.AdminDeleteQuestionResponse{}, nil
}

// GetAdminQuestionStats 管理后台读取题目总量统计。
func (s *QuestionService) GetAdminQuestionStats(ctx context.Context, _ *questionv1.GetAdminQuestionStatsRequest) (*questionv1.AdminQuestionStatsResponse, error) {
	totalQuestions, err := s.uc.GetAdminQuestionStats(ctx)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &questionv1.AdminQuestionStatsResponse{TotalQuestions: totalQuestions}, nil
}

// toProtoAdminQuestion 将题目实体转换为管理后台专用响应。
func toProtoAdminQuestion(question *biz.Question) *questionv1.AdminQuestionInfo {
	if question == nil {
		return nil
	}
	return &questionv1.AdminQuestionInfo{
		Id:                 question.ID,
		CategoryId:         question.CategoryID,
		IndustryId:         question.IndustryID,
		Type:               question.Type,
		Difficulty:         question.Difficulty,
		Title:              question.Title,
		Content:            question.Content,
		OptionsJson:        question.OptionsJSON,
		Answer:             question.Answer,
		Explanation:        question.Explanation,
		SolutionJson:       question.SolutionJSON,
		JudgeConfigJson:    question.JudgeConfigJSON,
		AnswerTemplateJson: question.AnswerTemplateJSON,
		Tags:               strings.Join(question.Tags, ","),
		IsActive:           question.IsActive,
		CreatedAt:          timestamppb.New(question.CreatedAt),
		UpdatedAt:          timestamppb.New(question.UpdatedAt),
		CategoryName:       question.CategoryName,
		IndustryName:       question.IndustryName,
	}
}

// AdminCreateQuestionSet 管理后台创建题单
func (s *QuestionService) AdminCreateQuestionSet(ctx context.Context, req *questionv1.AdminCreateQuestionSetRequest) (*questionv1.AdminCreateQuestionSetResponse, error) {
	set := &biz.QuestionSet{
		Name:         req.GetName(),
		Description:  req.GetDescription(),
		IndustryCode: req.GetIndustryCode(),
		CoverImage:   req.GetCoverImage(),
	}
	if err := s.uc.AdminCreateQuestionSet(ctx, set); err != nil {
		return nil, toGRPCError(err)
	}
	return &questionv1.AdminCreateQuestionSetResponse{Id: set.ID}, nil
}

// AdminUpdateQuestionSet 管理后台更新题单
func (s *QuestionService) AdminUpdateQuestionSet(ctx context.Context, req *questionv1.AdminUpdateQuestionSetRequest) (*questionv1.AdminUpdateQuestionSetResponse, error) {
	set := &biz.QuestionSet{
		ID:           req.GetId(),
		Name:         req.GetName(),
		Description:  req.GetDescription(),
		IndustryCode: req.GetIndustryCode(),
		CoverImage:   req.GetCoverImage(),
	}
	if err := s.uc.AdminUpdateQuestionSet(ctx, set); err != nil {
		return nil, toGRPCError(err)
	}
	return &questionv1.AdminUpdateQuestionSetResponse{}, nil
}

// AdminDeleteQuestionSet 管理后台删除题单
func (s *QuestionService) AdminDeleteQuestionSet(ctx context.Context, req *questionv1.AdminDeleteQuestionSetRequest) (*questionv1.AdminDeleteQuestionSetResponse, error) {
	if err := s.uc.AdminDeleteQuestionSet(ctx, req.GetId()); err != nil {
		return nil, toGRPCError(err)
	}
	return &questionv1.AdminDeleteQuestionSetResponse{}, nil
}

// AdminListQuestionSets 管理后台查询题单列表
func (s *QuestionService) AdminListQuestionSets(ctx context.Context, req *questionv1.AdminListQuestionSetsRequest) (*questionv1.AdminListQuestionSetsResponse, error) {
	var page, pageSize int32 = 1, 20
	if req.GetPage() != nil {
		page = req.GetPage().GetPage()
		pageSize = req.GetPage().GetPageSize()
	}

	sets, total, err := s.uc.ListQuestionSets(ctx, req.GetIndustryCode(), page, pageSize)
	if err != nil {
		return nil, toGRPCError(err)
	}

	items := make([]*questionv1.AdminQuestionSetInfo, len(sets))
	for i, set := range sets {
		info := &questionv1.AdminQuestionSetInfo{
			Id:            set.ID,
			Name:          set.Name,
			Description:   set.Description,
			IndustryCode:  set.IndustryCode,
			CoverImage:    set.CoverImage,
			QuestionCount: set.QuestionCount,
		}
		if !set.CreatedAt.IsZero() {
			info.CreatedAt = timestamppb.New(set.CreatedAt)
		}
		items[i] = info
	}

	return &questionv1.AdminListQuestionSetsResponse{
		Items: items,
		PageResult: &sharedv1.PageResult{
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		},
	}, nil
}

// AdminGetQuestionSet 管理后台获取题单详情（含关联题目）
func (s *QuestionService) AdminGetQuestionSet(ctx context.Context, req *questionv1.AdminGetQuestionSetRequest) (*questionv1.AdminQuestionSetDetail, error) {
	set, questions, err := s.uc.AdminGetQuestionSetDetail(ctx, req.GetId())
	if err != nil {
		return nil, toGRPCError(err)
	}

	info := &questionv1.AdminQuestionSetInfo{
		Id:            set.ID,
		Name:          set.Name,
		Description:   set.Description,
		IndustryCode:  set.IndustryCode,
		CoverImage:    set.CoverImage,
		QuestionCount: set.QuestionCount,
	}
	if !set.CreatedAt.IsZero() {
		info.CreatedAt = timestamppb.New(set.CreatedAt)
	}

	items := make([]*questionv1.AdminQuestionSetItem, len(questions))
	for i, q := range questions {
		items[i] = &questionv1.AdminQuestionSetItem{
			QuestionId: q.ID,
			Title:      q.Title,
			Difficulty: q.Difficulty,
			Type:       q.Type,
		}
	}

	return &questionv1.AdminQuestionSetDetail{
		Info:      info,
		Questions: items,
	}, nil
}

// AdminAddQuestionsToSet 管理后台向题单添加题目
func (s *QuestionService) AdminAddQuestionsToSet(ctx context.Context, req *questionv1.AdminAddQuestionsToSetRequest) (*questionv1.AdminAddQuestionsToSetResponse, error) {
	added, err := s.uc.AdminAddQuestionsToSet(ctx, req.GetSetId(), req.GetQuestionIds())
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &questionv1.AdminAddQuestionsToSetResponse{AddedCount: added}, nil
}

// AdminRemoveQuestionsFromSet 管理后台从题单移除题目
func (s *QuestionService) AdminRemoveQuestionsFromSet(ctx context.Context, req *questionv1.AdminRemoveQuestionsFromSetRequest) (*questionv1.AdminRemoveQuestionsFromSetResponse, error) {
	removed, err := s.uc.AdminRemoveQuestionsFromSet(ctx, req.GetSetId(), req.GetQuestionIds())
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &questionv1.AdminRemoveQuestionsFromSetResponse{RemovedCount: removed}, nil
}

// splitProtoTags 将后台透传的逗号标签转换为领域切片。
func splitProtoTags(raw string) []string {
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

// toProtoJudgeSummary 将 biz 层判题摘要转换为 proto 格式。
func toProtoJudgeSummary(summary *biz.JudgeSummary) *questionv1.JudgeSummary {
	if summary == nil {
		return nil
	}
	results := make([]*questionv1.JudgeCaseResult, 0, len(summary.Results))
	for _, r := range summary.Results {
		results = append(results, &questionv1.JudgeCaseResult{
			Input:          r.Input,
			ExpectedOutput: r.ExpectedOutput,
			ActualOutput:   r.ActualOutput,
			Passed:         r.Passed,
			Description:    r.Description,
		})
	}
	return &questionv1.JudgeSummary{
		AllPassed:   summary.AllPassed,
		TotalCases:  summary.TotalCases,
		PassedCases: summary.PassedCases,
		Results:     results,
	}
}

// toGRPCError 将业务错误转换为 gRPC 兼容的 Kratos 错误
func toGRPCError(err error) error {
	if err == nil {
		return nil
	}
	if kratosErr.FromError(err) != nil {
		return err
	}
	return kratosErr.InternalServer("INTERNAL", err.Error())
}
