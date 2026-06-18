package service

import (
	"context"

	"google.golang.org/protobuf/types/known/timestamppb"

	archivev1 "makejob/api/makejob/learning_archive/v1"
	"makejob/app/learning_archive/internal/biz"
)

type ArchiveService struct {
	archivev1.UnimplementedLearningArchiveServiceServer
	uc *biz.ArchiveUseCase
}

func NewArchiveService(uc *biz.ArchiveUseCase) *ArchiveService {
	return &ArchiveService{uc: uc}
}

func (s *ArchiveService) WriteEntry(ctx context.Context, req *archivev1.WriteArchiveEntryRequest) (*archivev1.ArchiveEntry, error) {
	entry := &biz.ArchiveEntry{
		UserID:          req.UserId,
		SourceType:      req.SourceType,
		SourceRef:       req.SourceRef,
		InterviewID:     req.InterviewId,
		QuestionIndex:   req.QuestionIndex,
		IndustryCode:    req.IndustryCode,
		PlanPhase:       req.PlanPhase,
		PlanPhaseGoal:   req.PlanPhaseGoal,
		EntryPhase:      req.EntryPhase,
		TaskPhase:       req.TaskPhase,
		TaskPhaseGoal:   req.TaskPhaseGoal,
		Language:        req.Language,
		MistakeTags:     req.MistakeTags,
		StrengthTags:    req.StrengthTags,
		Suggestions:     req.Suggestions,
		EvidenceSummary: req.EvidenceSummary,
	}
	if req.OccurredAt != nil {
		entry.OccurredAt = req.OccurredAt.AsTime()
	}

	if err := s.uc.WriteEntry(ctx, entry); err != nil {
		return nil, err
	}
	return toProtoEntry(entry), nil
}

func (s *ArchiveService) BatchWriteEntries(ctx context.Context, req *archivev1.BatchWriteRequest) (*archivev1.BatchWriteResponse, error) {
	entries := make([]*biz.ArchiveEntry, len(req.Entries))
	for i, e := range req.Entries {
		entry := &biz.ArchiveEntry{
			UserID:          e.UserId,
			SourceType:      e.SourceType,
			SourceRef:       e.SourceRef,
			InterviewID:     e.InterviewId,
			QuestionIndex:   e.QuestionIndex,
			IndustryCode:    e.IndustryCode,
			PlanPhase:       e.PlanPhase,
			PlanPhaseGoal:   e.PlanPhaseGoal,
			EntryPhase:      e.EntryPhase,
			TaskPhase:       e.TaskPhase,
			TaskPhaseGoal:   e.TaskPhaseGoal,
			Language:        e.Language,
			MistakeTags:     e.MistakeTags,
			StrengthTags:    e.StrengthTags,
			Suggestions:     e.Suggestions,
			EvidenceSummary: e.EvidenceSummary,
		}
		if e.OccurredAt != nil {
			entry.OccurredAt = e.OccurredAt.AsTime()
		}
		entries[i] = entry
	}

	written, err := s.uc.BatchWriteEntries(ctx, entries)
	if err != nil {
		return nil, err
	}
	return &archivev1.BatchWriteResponse{Written: int32(written)}, nil
}

func (s *ArchiveService) ListByUser(ctx context.Context, req *archivev1.ListByUserRequest) (*archivev1.ArchiveEntryList, error) {
	entries, err := s.uc.ListByUser(ctx, req.UserId, req.Limit)
	if err != nil {
		return nil, err
	}
	items := make([]*archivev1.ArchiveEntry, len(entries))
	for i, e := range entries {
		items[i] = toProtoEntry(e)
	}
	return &archivev1.ArchiveEntryList{Entries: items}, nil
}

func (s *ArchiveService) GetWeakTopics(ctx context.Context, req *archivev1.GetWeakTopicsRequest) (*archivev1.WeakTopicList, error) {
	topics, err := s.uc.GetWeakTopics(ctx, req.UserId, req.Limit)
	if err != nil {
		return nil, err
	}
	return &archivev1.WeakTopicList{Topics: topics}, nil
}

func (s *ArchiveService) GetFocusSignals(ctx context.Context, req *archivev1.GetFocusSignalsRequest) (*archivev1.FocusSignalList, error) {
	signals, trendSummary, err := s.uc.GetFocusSignals(ctx, req.UserId, req.Limit, req.IndustryCode)
	if err != nil {
		return nil, err
	}

	items := make([]*archivev1.FocusSignal, len(signals))
	for i, sig := range signals {
		items[i] = toProtoFocusSignal(sig)
	}

	resp := &archivev1.FocusSignalList{Signals: items}
	if trendSummary != nil {
		resp.TrendSummary = &archivev1.GrowthTrendSummary{
			DominantSource:      trendSummary.DominantSource,
			DominantSourceLabel: trendSummary.DominantSourceLabel,
			TopFocusTag:         trendSummary.TopFocusTag,
			TopTopicCode:        trendSummary.TopTopicCode,
			TopTopicTitle:       trendSummary.TopTopicTitle,
			Summary:             trendSummary.Summary,
		}
	}
	return resp, nil
}

func (s *ArchiveService) GetPracticeRecommendations(ctx context.Context, req *archivev1.GetPracticeRecommendationsRequest) (*archivev1.PracticeRecommendationList, error) {
	var interviewID *uint64
	if req.InterviewId > 0 {
		interviewID = &req.InterviewId
	}
	recs, err := s.uc.GetPracticeRecommendations(ctx, req.UserId, req.Limit, interviewID)
	if err != nil {
		return nil, err
	}

	items := make([]*archivev1.PracticeRecommendation, len(recs))
	for i, rec := range recs {
		items[i] = &archivev1.PracticeRecommendation{
			FocusTag:        rec.FocusTag,
			TopicCode:       rec.TopicCode,
			TopicTitle:      rec.TopicTitle,
			Keywords:        rec.Keywords,
			OccurrenceCount: int32(rec.OccurrenceCount),
			Reason:          rec.Reason,
		}
	}
	return &archivev1.PracticeRecommendationList{Recommendations: items}, nil
}

func (s *ArchiveService) ListMistakeTopics(ctx context.Context, req *archivev1.ListMistakeTopicsRequest) (*archivev1.ListMistakeTopicsResponse, error) {
	topics := s.uc.ListMistakeTopics()
	items := make([]*archivev1.MistakeTopicSummary, len(topics))
	for i, t := range topics {
		items[i] = &archivev1.MistakeTopicSummary{
			Code:           t.Code,
			Tag:            t.Tag,
			Title:          t.Title,
			ProblemPattern: t.ProblemPattern,
		}
	}
	return &archivev1.ListMistakeTopicsResponse{Topics: items}, nil
}

func (s *ArchiveService) GetMistakeTopic(ctx context.Context, req *archivev1.GetMistakeTopicRequest) (*archivev1.MistakeTopicCard, error) {
	topic, ok := s.uc.GetMistakeTopic(req.Code)
	if !ok {
		return nil, nil
	}
	return &archivev1.MistakeTopicCard{
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

// --- 转换函数 ---

func toProtoEntry(e *biz.ArchiveEntry) *archivev1.ArchiveEntry {
	pb := &archivev1.ArchiveEntry{
		Id:              e.ID,
		UserId:          e.UserID,
		SourceType:      e.SourceType,
		SourceRef:       e.SourceRef,
		InterviewId:     e.InterviewID,
		QuestionIndex:   e.QuestionIndex,
		IndustryCode:    e.IndustryCode,
		PlanPhase:       e.PlanPhase,
		PlanPhaseGoal:   e.PlanPhaseGoal,
		EntryPhase:      e.EntryPhase,
		TaskPhase:       e.TaskPhase,
		TaskPhaseGoal:   e.TaskPhaseGoal,
		Language:        e.Language,
		MistakeTags:     e.MistakeTags,
		StrengthTags:    e.StrengthTags,
		Suggestions:     e.Suggestions,
		EvidenceSummary: e.EvidenceSummary,
	}
	if !e.OccurredAt.IsZero() {
		pb.OccurredAt = timestamppb.New(e.OccurredAt)
	}
	if !e.CreatedAt.IsZero() {
		pb.CreatedAt = timestamppb.New(e.CreatedAt)
	}
	return pb
}

func toProtoFocusSignal(sig *biz.TrainingFocusSignal) *archivev1.FocusSignal {
	pb := &archivev1.FocusSignal{
		Tag:                       sig.Tag,
		TopicCode:                 sig.TopicCode,
		TopicTitle:                sig.TopicTitle,
		TopicProblemPattern:       sig.TopicProblemPattern,
		OccurrenceCount:           int32(sig.OccurrenceCount),
		ArchiveOccurrenceCount:    int32(sig.ArchiveOccurrenceCount),
		InterviewOccurrenceCount:  int32(sig.InterviewOccurrenceCount),
		DominantArchivePhase:      sig.DominantArchivePhase,
		DominantArchivePhaseLabel: sig.DominantArchivePhaseLabel,
		RelatedQuestionSets:       sig.RelatedQuestionSets,
		RecommendedActions:        sig.RecommendedActions,
		PrimaryQuestionSet:        sig.PrimaryQuestionSet,
		Source:                    sig.Source,
		SourceLabel:               sig.SourceLabel,
		Reason:                    sig.Reason,
		SourceRef:                 sig.SourceRef,
		CollectionHint:            sig.CollectionHint,
		Topic:                     sig.Tag,
		Weight:                    float64(sig.OccurrenceCount),
	}
	return pb
}
