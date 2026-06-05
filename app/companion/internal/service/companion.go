package service

import (
	"context"

	companionv1 "makejob/api/makejob/companion/v1"
	"makejob/app/companion/internal/biz"
)

// CompanionService 陪伴助手 gRPC 服务实现
type CompanionService struct {
	companionv1.UnimplementedCompanionServiceServer
	uc *biz.CompanionUseCase
}

// NewCompanionService 创建陪伴助手 gRPC 服务
func NewCompanionService(uc *biz.CompanionUseCase) *CompanionService {
	return &CompanionService{uc: uc}
}

// Chat 陪伴对话
func (s *CompanionService) Chat(ctx context.Context, req *companionv1.CompanionChatRequest) (*companionv1.CompanionChatResponse, error) {
	return &companionv1.CompanionChatResponse{}, nil
}

// GetCompanionState 查询陪伴助手状态
func (s *CompanionService) GetCompanionState(ctx context.Context, req *companionv1.GetCompanionStateRequest) (*companionv1.CompanionState, error) {
	state, err := s.uc.GetState(ctx, req.UserId)
	if err != nil {
		return &companionv1.CompanionState{Emotion: "neutral"}, nil
	}
	return &companionv1.CompanionState{
		Emotion:   state.Emotion,
		LastTopic: state.LastTopic,
	}, nil
}

// SynthesizeSpeech 语音合成
func (s *CompanionService) SynthesizeSpeech(ctx context.Context, req *companionv1.SynthesizeSpeechRequest) (*companionv1.SynthesizeSpeechResponse, error) {
	return &companionv1.SynthesizeSpeechResponse{}, nil
}
