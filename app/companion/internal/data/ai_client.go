package data

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	aiv1 "makejob/api/makejob/ai/v1"
	"makejob/app/companion/internal/biz"
	"makejob/app/companion/internal/conf"
	"makejob/pkg/middleware"
)

// companionAIClient 实现 biz.CompanionClient 接口，通过 gRPC 调用 AI Gateway
type companionAIClient struct {
	client aiv1.AIServiceClient
	conn   *grpc.ClientConn
}

// NewCompanionAIClient 创建 AI 服务客户端
func NewCompanionAIClient(cfg *conf.AI) (biz.CompanionClient, error) {
	conn, err := grpc.Dial(cfg.ServiceAddr, middleware.CommonDialOptions()...)
	if err != nil {
		return nil, fmt.Errorf("failed to dial AI service at %s: %w", cfg.ServiceAddr, err)
	}
	return &companionAIClient{
		client: aiv1.NewAIServiceClient(conn),
		conn:   conn,
	}, nil
}

// Close 关闭 gRPC 连接
func (c *companionAIClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// CompanionAgent 调用 AI Gateway 的 CompanionAgent RPC
func (c *companionAIClient) CompanionAgent(ctx context.Context, req *biz.CompanionAgentRequest) (*biz.CompanionAgentResponse, error) {
	resp, err := c.client.CompanionAgent(ctx, &aiv1.CompanionAgentRequest{
		UserId:                req.UserID,
		Message:               req.Message,
		ContextType:           req.ContextType,
		Username:              req.Username,
		RecentTopics:          req.RecentTopics,
		ConversationStateJson: req.ConversationStateJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("CompanionAgent gRPC call failed: %w", err)
	}

	// 转换 Live2DDirective（如果有）
	var live2dDirective *biz.Live2DDirectiveResponse
	if resp.GetLive2DDirective() != nil {
		live2dDirective = toBizLive2DDirectiveResponse(resp.GetLive2DDirective())
	}

	return &biz.CompanionAgentResponse{
		Reply:             resp.GetReply(),
		Emotion:           resp.GetEmotion(),
		Suggestions:       resp.GetSuggestions(),
		Action:            resp.GetAction(),
		SuggestedActions:  toBizSuggestedActions(resp.GetSuggestedActions()),
		Live2DDirective:   live2dDirective,
		InlineTriggers:    toBizInlineTriggers(resp.GetInlineTriggers()),
		Intent:            toBizIntentInfo(resp.GetIntent()),
		PendingAction:     toBizPendingAction(resp.GetPendingAction()),
		ConversationState: toBizConversationState(resp.GetConversationState()),
	}, nil
}

// toBizSuggestedActions 将 proto SuggestedAction 列表转换为 biz SuggestedAction 列表。
func toBizSuggestedActions(actions []*aiv1.SuggestedAction) []biz.SuggestedAction {
	if len(actions) == 0 {
		return nil
	}
	result := make([]biz.SuggestedAction, 0, len(actions))
	for _, a := range actions {
		result = append(result, biz.SuggestedAction{
			Type:   a.GetType(),
			Target: a.GetTarget(),
			Params: a.GetParams(),
		})
	}
	return result
}

// toBizInlineTriggers 将 proto InlineTrigger 列表转换为 biz InlineTriggerItem 列表。
func toBizInlineTriggers(items []*aiv1.InlineTrigger) []biz.InlineTriggerItem {
	if len(items) == 0 {
		return nil
	}
	result := make([]biz.InlineTriggerItem, 0, len(items))
	for _, it := range items {
		result = append(result, biz.InlineTriggerItem{
			Keyword:      it.GetKeyword(),
			ActionType:   it.GetActionType(),
			Target:       it.GetTarget(),
			PositionHint: it.GetPositionHint(),
		})
	}
	return result
}

// toBizIntentInfo 将 proto IntentInfo 转换为 biz IntentInfo 指针（nullable）。
func toBizIntentInfo(info *aiv1.IntentInfo) *biz.IntentInfo {
	if info == nil {
		return nil
	}
	return &biz.IntentInfo{
		Type:       info.GetType(),
		Confidence: info.GetConfidence(),
		Stage:      info.GetStage(),
	}
}

// toBizPendingAction 将 proto PendingAction 转换为 biz PendingAction 指针（nullable）。
func toBizPendingAction(action *aiv1.PendingAction) *biz.PendingAction {
	if action == nil {
		return nil
	}
	return &biz.PendingAction{
		Type:        action.GetType(),
		Ready:       action.GetReady(),
		Params:      action.GetParams(),
		MissingInfo: action.GetMissingInfo(),
	}
}

// toBizConversationState 将 proto ConversationState 转换为 biz ConversationState 指针（nullable）。
func toBizConversationState(state *aiv1.ConversationState) *biz.ConversationState {
	if state == nil {
		return nil
	}
	return &biz.ConversationState{
		Phase:           state.GetPhase(),
		CollectedParams: state.GetCollectedParams(),
	}
}

// GetGreeting 调用 AI Gateway 的 GetGreeting RPC
func (c *companionAIClient) GetGreeting(ctx context.Context, level, timeOfDay string) (*biz.CompanionAgentResponse, error) {
	resp, err := c.client.GetGreeting(ctx, &aiv1.GetGreetingRequest{
		Level:     level,
		TimeOfDay: timeOfDay,
	})
	if err != nil {
		return nil, fmt.Errorf("GetGreeting gRPC call failed: %w", err)
	}
	return &biz.CompanionAgentResponse{
		Reply:   resp.GetContent(),
		Emotion: resp.GetEmotion(),
		Action:  resp.GetAction(),
	}, nil
}

// GetEncouragement 调用 AI Gateway 的 GetEncouragement RPC
func (c *companionAIClient) GetEncouragement(ctx context.Context, achievement string) (*biz.CompanionAgentResponse, error) {
	resp, err := c.client.GetEncouragement(ctx, &aiv1.GetEncouragementRequest{
		Achievement: achievement,
	})
	if err != nil {
		return nil, fmt.Errorf("GetEncouragement gRPC call failed: %w", err)
	}
	return &biz.CompanionAgentResponse{
		Reply:   resp.GetContent(),
		Emotion: resp.GetEmotion(),
		Action:  resp.GetAction(),
	}, nil
}

// toBizLive2DDirectiveResponse 将 proto Live2DDirective 转换为 biz Live2DDirectiveResponse
func toBizLive2DDirectiveResponse(directive *aiv1.Live2DDirective) *biz.Live2DDirectiveResponse {
	expressionMix := make([]biz.ExpressionLayer, 0, len(directive.GetExpressionMix()))
	for _, e := range directive.GetExpressionMix() {
		expressionMix = append(expressionMix, biz.ExpressionLayer{
			Key:    e.GetKey(),
			Weight: float64(e.GetWeight()),
		})
	}
	parameterOverrides := make([]biz.ParameterOverride, 0, len(directive.GetParameterOverrides()))
	for _, p := range directive.GetParameterOverrides() {
		parameterOverrides = append(parameterOverrides, biz.ParameterOverride{
			ID:    p.GetId(),
			Value: float64(p.GetValue()),
		})
	}
	var mouthOpen *float64
	if directive.MouthOpen != 0 {
		v := float64(directive.MouthOpen)
		mouthOpen = &v
	}
	return &biz.Live2DDirectiveResponse{
		Emotion:            directive.GetEmotion(),
		Action:             directive.GetAction(),
		Reply:              directive.GetReply(),
		MotionKey:          directive.GetMotionKey(),
		MotionGroup:        directive.GetMotionGroup(),
		MotionPriority:     directive.GetMotionPriority(),
		MotionDurationMS:   int(directive.GetMotionDurationMs()),
		Intensity:          float64(directive.GetIntensity()),
		DurationMS:         int(directive.GetDurationMs()),
		MouthOpen:          mouthOpen,
		Source:             directive.GetSource(),
		ExpressionMix:      expressionMix,
		ParameterOverrides: parameterOverrides,
	}
}

// Live2DDirector 调用 AI Gateway 的 Live2DDirector RPC
func (c *companionAIClient) Live2DDirector(ctx context.Context, req *biz.Live2DDirectorRequest) (*biz.Live2DDirectiveResponse, error) {
	resp, err := c.client.Live2DDirector(ctx, &aiv1.Live2DDirectiveRequest{
		Context:     req.Context,
		EmotionHint: req.EmotionHint,
		ReplyText:   req.ReplyText,
	})
	if err != nil {
		return nil, fmt.Errorf("Live2DDirector gRPC call failed: %w", err)
	}
	expressionMix := make([]biz.ExpressionLayer, 0, len(resp.GetExpressionMix()))
	for _, e := range resp.GetExpressionMix() {
		expressionMix = append(expressionMix, biz.ExpressionLayer{
			Key:    e.GetKey(),
			Weight: float64(e.GetWeight()),
		})
	}
	parameterOverrides := make([]biz.ParameterOverride, 0, len(resp.GetParameterOverrides()))
	for _, p := range resp.GetParameterOverrides() {
		parameterOverrides = append(parameterOverrides, biz.ParameterOverride{
			ID:    p.GetId(),
			Value: float64(p.GetValue()),
		})
	}
	var mouthOpen *float64
	if resp.MouthOpen != 0 {
		v := float64(resp.MouthOpen)
		mouthOpen = &v
	}
	return &biz.Live2DDirectiveResponse{
		Emotion:            resp.GetEmotion(),
		Action:             resp.GetAction(),
		Reply:              resp.GetReply(),
		MotionKey:          resp.GetMotionKey(),
		MotionGroup:        resp.GetMotionGroup(),
		MotionPriority:     resp.GetMotionPriority(),
		MotionDurationMS:   int(resp.GetMotionDurationMs()),
		Intensity:          float64(resp.GetIntensity()),
		DurationMS:         int(resp.GetDurationMs()),
		MouthOpen:          mouthOpen,
		Source:             resp.GetSource(),
		ExpressionMix:      expressionMix,
		ParameterOverrides: parameterOverrides,
	}, nil
}
