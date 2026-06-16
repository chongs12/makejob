package service

import (
	"context"
	"fmt"
	"strings"

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

	// 构建富上下文消息：将 plan/goal 信息注入到 message 中供 AI 参考
	message := buildEnrichedMessage(req)

	result, err := s.uc.Chat(ctx, userID, message, req.GetContextType(), req.GetLive2DModelKey())
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &companionv1.CompanionChatResponse{
		Reply:       result.Reply,
		Emotion:     result.Emotion,
		Action:      result.Action,
		Suggestions: result.Suggestions,
		AudioUrl:    result.AudioURL,
		Live2DDirective: toProtoLive2DDirective(result.Live2DDirective),
	}, nil
}

// buildEnrichedMessage 将前端传入的 messages 数组和 context 对象合并为单个富文本消息。
func buildEnrichedMessage(req *companionv1.CompanionChatRequest) string {
	var parts []string

	// 注入上下文信息
	ctx := req.GetContext()
	if ctx != nil {
		var contextParts []string
		if ctx.CurrentPlanTitle != "" {
			contextParts = append(contextParts, fmt.Sprintf("当前计划：%s（进度 %.0f%%）", ctx.CurrentPlanTitle, ctx.CurrentPlanProgress*100))
		}
		if len(ctx.TodayGoals) > 0 {
			contextParts = append(contextParts, fmt.Sprintf("今日目标：%s", strings.Join(ctx.TodayGoals, "、")))
		}
		if len(ctx.ActiveGoals) > 0 {
			contextParts = append(contextParts, fmt.Sprintf("进行中任务：%s", strings.Join(ctx.ActiveGoals, "、")))
		}
		if ctx.FocusedTaskTitle != "" {
			contextParts = append(contextParts, fmt.Sprintf("聚焦任务：%s", ctx.FocusedTaskTitle))
		}
		if ctx.CompletedTodayCount > 0 || ctx.SkippedTodayCount > 0 {
			contextParts = append(contextParts, fmt.Sprintf("今日完成 %d 项，跳过 %d 项", ctx.CompletedTodayCount, ctx.SkippedTodayCount))
		}
		if ctx.LatestTaskAction != "" {
			contextParts = append(contextParts, fmt.Sprintf("最近操作：%s", ctx.LatestTaskAction))
		}
		if len(contextParts) > 0 {
			parts = append(parts, "[用户学习上下文]\n"+strings.Join(contextParts, "\n"))
		}
	}

	// 注入对话历史
	messages := req.GetMessages()
	if len(messages) > 0 {
		var historyParts []string
		for _, m := range messages {
			role := m.GetRole()
			if role == "user" {
				historyParts = append(historyParts, "用户: "+m.GetContent())
			} else if role == "assistant" {
				historyParts = append(historyParts, "助手: "+m.GetContent())
			}
		}
		if len(historyParts) > 0 {
			parts = append(parts, "[对话历史]\n"+strings.Join(historyParts, "\n"))
		}
	}

	// 最后一条用户消息作为主消息
	mainMessage := req.GetMessage()
	if mainMessage != "" {
		parts = append(parts, mainMessage)
	}

	return strings.Join(parts, "\n\n")
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

// toProtoLive2DDirective 将 biz Live2DDirectiveResponse 转换为 proto Live2DDirective
func toProtoLive2DDirective(resp *biz.Live2DDirectiveResponse) *companionv1.Live2DDirective {
	if resp == nil {
		return nil
	}
	expressionMix := make([]*companionv1.Live2DDirectiveExpressionLayer, 0, len(resp.ExpressionMix))
	for _, e := range resp.ExpressionMix {
		expressionMix = append(expressionMix, &companionv1.Live2DDirectiveExpressionLayer{
			Key:    e.Key,
			Weight: float32(e.Weight),
		})
	}
	parameterOverrides := make([]*companionv1.Live2DDirectiveParameterOverride, 0, len(resp.ParameterOverrides))
	for _, p := range resp.ParameterOverrides {
		parameterOverrides = append(parameterOverrides, &companionv1.Live2DDirectiveParameterOverride{
			Id:    p.ID,
			Value: float32(p.Value),
		})
	}
	mouthOpen := float32(0)
	if resp.MouthOpen != nil {
		mouthOpen = float32(*resp.MouthOpen)
	}
	return &companionv1.Live2DDirective{
		Reply:              resp.Reply,
		Emotion:            resp.Emotion,
		Action:             resp.Action,
		ExpressionMix:      expressionMix,
		ParameterOverrides: parameterOverrides,
		MotionKey:          resp.MotionKey,
		MotionGroup:        resp.MotionGroup,
		MotionPriority:     resp.MotionPriority,
		MotionDurationMs:   int32(resp.MotionDurationMS),
		Intensity:          float32(resp.Intensity),
		DurationMs:         int32(resp.DurationMS),
		MouthOpen:          mouthOpen,
		Source:             resp.Source,
	}
}
