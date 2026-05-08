package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"makejob-backend/internal/common"
	"makejob-backend/internal/middleware"
	"makejob-backend/internal/service"
)

// stubPlanHandlerService 为计划 handler 测试提供最小服务桩，记录关键调用参数。
type stubPlanHandlerService struct {
	submitCalled bool
	submitUserID uint
	submitPlanID uint
	submitTaskID uint
	submitReq    *service.SubmitTaskFeedbackRequest

	adjustCalled bool
	adjustUserID uint
	adjustPlanID uint
	adjustResp   *service.PlanDetailResponse
}

// GeneratePlan 返回空结果，满足 PlanService 接口要求。
func (s *stubPlanHandlerService) GeneratePlan(_ context.Context, _ uint, _ *service.GeneratePlanRequest) (*service.PlanDetailResponse, error) {
	return nil, nil
}

// GetCurrentPlan 返回空结果，满足 PlanService 接口要求。
func (s *stubPlanHandlerService) GetCurrentPlan(_ context.Context, _ uint) (*service.PlanDetailResponse, error) {
	return nil, nil
}

// GetPlan 返回空结果，满足 PlanService 接口要求。
func (s *stubPlanHandlerService) GetPlan(_ context.Context, _, _ uint) (*service.PlanDetailResponse, error) {
	return nil, nil
}

// ListPlans 返回空结果，满足 PlanService 接口要求。
func (s *stubPlanHandlerService) ListPlans(_ context.Context, _ uint, _, _ int) (*common.PageResult, error) {
	return nil, nil
}

// UpdateTaskStatus 直接返回成功，满足 PlanService 接口要求。
func (s *stubPlanHandlerService) UpdateTaskStatus(_ context.Context, _, _, _ uint, _ *service.UpdateTaskStatusRequest) error {
	return nil
}

// SubmitTaskFeedback 记录反馈请求参数，便于断言 handler 绑定和透传是否正确。
func (s *stubPlanHandlerService) SubmitTaskFeedback(_ context.Context, userID, planID, taskID uint, req *service.SubmitTaskFeedbackRequest) error {
	s.submitCalled = true
	s.submitUserID = userID
	s.submitPlanID = planID
	s.submitTaskID = taskID
	if req == nil {
		s.submitReq = nil
		return nil
	}

	copied := *req
	copied.MistakeTags = append([]string(nil), req.MistakeTags...)
	copied.WrongAnswer = append(json.RawMessage(nil), req.WrongAnswer...)
	s.submitReq = &copied
	return nil
}

// AdjustPlan 返回预置计划详情，便于断言诊断来源字段是否出现在 HTTP 响应中。
func (s *stubPlanHandlerService) AdjustPlan(_ context.Context, userID, planID uint) (*service.PlanDetailResponse, error) {
	s.adjustCalled = true
	s.adjustUserID = userID
	s.adjustPlanID = planID
	return s.adjustResp, nil
}

// GetProgress 返回空结果，满足 PlanService 接口要求。
func (s *stubPlanHandlerService) GetProgress(_ context.Context, _, _ uint) (*service.PlanProgressResponse, error) {
	return nil, nil
}

// buildTestPlanRouter 构造仅包含计划路由的测试路由，并注入固定用户身份。
func buildTestPlanRouter(planService service.PlanService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	api := router.Group("/api")
	protected := api.Group("")
	protected.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUserID), uint(12))
		c.Next()
	})

	NewPlanHandler(planService).RegisterRoutes(protected)
	return router
}

// TestPlanHandlerFeedbackAndAdjustFlow 验证反馈提交与调计划接口的 HTTP 串联行为。
func TestPlanHandlerFeedbackAndAdjustFlow(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	planService := &stubPlanHandlerService{
		adjustResp: &service.PlanDetailResponse{
			ID:     8,
			Title:  "调整后的计划",
			Status: "active",
			Tasks: []service.TaskResponse{
				{
					ID:             301,
					Title:          "复盘状态定义题",
					TaskType:       "review",
					Status:         "pending",
					DayNumber:      3,
					SortOrder:      1,
					Source:         "plan_feedback_diagnosis",
					SourceLabel:    "训练反馈诊断",
					Reason:         "最近一次训练反馈显示状态定义仍不稳定",
					SourceRef:      "diagnosis:88",
					CollectionHint: "state-definition",
				},
			},
			CreatedAt: now,
		},
	}
	router := buildTestPlanRouter(planService)

	feedbackRecorder := httptest.NewRecorder()
	feedbackRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/plans/8/tasks/101/feedback",
		strings.NewReader(`{"training_type":"coding","question_id":55,"mistake_tags":["state_definition","boundary_case"],"attempt_count":2,"time_spent_seconds":420,"difficulty_self_assessment":"too_hard","wrong_answer":{"code":"bad"},"summary":"状态转移判断不稳定"}`),
	)
	feedbackRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(feedbackRecorder, feedbackRequest)

	if feedbackRecorder.Code != http.StatusOK {
		t.Fatalf("expected feedback http status 200, got %d", feedbackRecorder.Code)
	}

	var feedbackResp struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(feedbackRecorder.Body.Bytes(), &feedbackResp); err != nil {
		t.Fatalf("unmarshal feedback response: %v", err)
	}
	if feedbackResp.Code != common.CodeSuccess {
		t.Fatalf("expected feedback code %d, got %d", common.CodeSuccess, feedbackResp.Code)
	}
	if feedbackResp.Message != "提交成功" {
		t.Fatalf("expected feedback message 提交成功, got %s", feedbackResp.Message)
	}
	if !planService.submitCalled {
		t.Fatal("expected SubmitTaskFeedback to be called")
	}
	if planService.submitUserID != 12 || planService.submitPlanID != 8 || planService.submitTaskID != 101 {
		t.Fatalf("unexpected feedback call args: user=%d plan=%d task=%d", planService.submitUserID, planService.submitPlanID, planService.submitTaskID)
	}
	if planService.submitReq == nil {
		t.Fatal("expected feedback request to be recorded")
	}
	if planService.submitReq.TrainingType != "coding" {
		t.Fatalf("expected training_type coding, got %s", planService.submitReq.TrainingType)
	}
	if planService.submitReq.QuestionID == nil || *planService.submitReq.QuestionID != 55 {
		t.Fatalf("expected question_id 55, got %+v", planService.submitReq.QuestionID)
	}
	if len(planService.submitReq.MistakeTags) != 2 || planService.submitReq.MistakeTags[0] != "state_definition" {
		t.Fatalf("unexpected mistake_tags: %+v", planService.submitReq.MistakeTags)
	}
	if string(planService.submitReq.WrongAnswer) != `{"code":"bad"}` {
		t.Fatalf("unexpected wrong_answer: %s", string(planService.submitReq.WrongAnswer))
	}

	adjustRecorder := httptest.NewRecorder()
	adjustRequest := httptest.NewRequest(http.MethodPost, "/api/plans/8/adjust", nil)
	router.ServeHTTP(adjustRecorder, adjustRequest)

	if adjustRecorder.Code != http.StatusOK {
		t.Fatalf("expected adjust http status 200, got %d", adjustRecorder.Code)
	}

	var adjustResp struct {
		Code    int                        `json:"code"`
		Message string                     `json:"message"`
		Data    service.PlanDetailResponse `json:"data"`
	}
	if err := json.Unmarshal(adjustRecorder.Body.Bytes(), &adjustResp); err != nil {
		t.Fatalf("unmarshal adjust response: %v", err)
	}
	if adjustResp.Code != common.CodeSuccess {
		t.Fatalf("expected adjust code %d, got %d", common.CodeSuccess, adjustResp.Code)
	}
	if !planService.adjustCalled {
		t.Fatal("expected AdjustPlan to be called")
	}
	if planService.adjustUserID != 12 || planService.adjustPlanID != 8 {
		t.Fatalf("unexpected adjust call args: user=%d plan=%d", planService.adjustUserID, planService.adjustPlanID)
	}
	if len(adjustResp.Data.Tasks) != 1 {
		t.Fatalf("expected 1 adjusted task, got %d", len(adjustResp.Data.Tasks))
	}

	task := adjustResp.Data.Tasks[0]
	if task.Source != "plan_feedback_diagnosis" {
		t.Fatalf("expected source plan_feedback_diagnosis, got %s", task.Source)
	}
	if task.SourceRef == "" {
		t.Fatal("expected source_ref to be returned")
	}
	if task.CollectionHint == "" {
		t.Fatal("expected collection_hint to be returned")
	}
}
