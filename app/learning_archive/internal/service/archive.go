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

func (s *ArchiveService) GetWeakTopics(ctx context.Context, req *archivev1.UserIDRequest) (*archivev1.WeakTopicList, error) {
	topics, err := s.uc.GetWeakTopics(ctx, req.UserId)
	if err != nil {
		return nil, err
	}
	return &archivev1.WeakTopicList{Topics: topics}, nil
}

func (s *ArchiveService) GetFocusSignals(ctx context.Context, req *archivev1.UserIDRequest) (*archivev1.FocusSignalList, error) {
	signals, err := s.uc.GetFocusSignals(ctx, req.UserId)
	if err != nil {
		return nil, err
	}
	items := make([]*archivev1.FocusSignal, len(signals))
	for i, sig := range signals {
		items[i] = &archivev1.FocusSignal{
			Topic:  sig.Topic,
			Weight: sig.Weight,
			Source: sig.Source,
		}
	}
	return &archivev1.FocusSignalList{Signals: items}, nil
}

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
