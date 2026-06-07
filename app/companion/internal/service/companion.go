package service

import (
	"context"

	kratosErr "github.com/go-kratos/kratos/v2/errors"
	"google.golang.org/protobuf/types/known/timestamppb"

	companionv1 "makejob/api/makejob/companion/v1"
	"makejob/app/companion/internal/biz"
	"makejob/pkg/auth"
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

// Chat 陪伴对话，处理用户消息并返回 AI 回复
func (s *CompanionService) Chat(ctx context.Context, req *companionv1.CompanionChatRequest) (*companionv1.CompanionChatResponse, error) {
	userID := resolveUserID(ctx, req.GetUserId())
	if userID == 0 {
		return nil, kratosErr.BadRequest("USER_ID_REQUIRED", "用户 ID 不能为空")
	}

	result, err := s.uc.Chat(ctx, userID, req.GetMessage(), req.GetContextType())
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &companionv1.CompanionChatResponse{
		Reply:       result.Reply,
		Emotion:     result.Emotion,
		Suggestions: result.Suggestions,
	}, nil
}

// GetCompanionState 查询陪伴助手状态
func (s *CompanionService) GetCompanionState(ctx context.Context, req *companionv1.GetCompanionStateRequest) (*companionv1.CompanionState, error) {
	userID := resolveUserID(ctx, req.GetUserId())
	if userID == 0 {
		return nil, kratosErr.BadRequest("USER_ID_REQUIRED", "用户 ID 不能为空")
	}

	session, err := s.uc.GetCompanionState(ctx, userID)
	if err != nil {
		return nil, toGRPCError(err)
	}

	var lastActiveAt *timestamppb.Timestamp
	if !session.LastChatAt.IsZero() {
		lastActiveAt = timestamppb.New(session.LastChatAt)
	}

	return &companionv1.CompanionState{
		Emotion:      session.LastEmotion,
		LastTopic:    session.LastTopic,
		LastActiveAt: lastActiveAt,
	}, nil
}

// SynthesizeSpeech 语音合成
func (s *CompanionService) SynthesizeSpeech(ctx context.Context, req *companionv1.SynthesizeSpeechRequest) (*companionv1.SynthesizeSpeechResponse, error) {
	audioResult, err := s.uc.SynthesizeSpeech(ctx, req.GetText(), req.GetVoice())
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &companionv1.SynthesizeSpeechResponse{
		AudioData: audioResult.AudioData,
		AudioUrl:  audioResult.AudioURL,
	}, nil
}

// resolveUserID 优先使用认证上下文中的用户 ID，避免信任请求体透传字段
func resolveUserID(ctx context.Context, requested uint64) uint64 {
	if userID := auth.GetUserIDFromContext(ctx); userID != 0 {
		return userID
	}
	return requested
}

// toGRPCError 将错误转换为 gRPC 兼容的 Kratos 错误
func toGRPCError(err error) error {
	if kratosErr.FromError(err) != nil {
		return err
	}
	return kratosErr.InternalServer("INTERNAL", err.Error())
}
