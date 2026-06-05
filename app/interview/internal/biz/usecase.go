package biz

import (
	"context"
	"fmt"
	"time"

	kratosErr "github.com/go-kratos/kratos/v2/errors"
)

// 业务错误码
var (
	ErrInterviewNotFound = kratosErr.NotFound("INTERVIEW_NOT_FOUND", "面试不存在")
	ErrInterviewFinished = kratosErr.BadRequest("INTERVIEW_FINISHED", "面试已结束")
	ErrUnauthorized      = kratosErr.Unauthorized("UNAUTHORIZED", "未授权")
	ErrInvalidIndustry   = kratosErr.BadRequest("INVALID_INDUSTRY", "无效的行业代码")
	ErrAICallFailed      = kratosErr.InternalServer("AI_CALL_FAILED", "AI 服务调用失败")
)

// InterviewUseCase 面试业务用例
type InterviewUseCase struct {
	repo     InterviewRepo
	ai       AIServiceClient
	archive  LearningArchiveClient
	industry IndustryClient
}

// NewInterviewUseCase 由 Wire 调用，所有依赖通过接口注入
func NewInterviewUseCase(
	repo InterviewRepo,
	ai AIServiceClient,
	archive LearningArchiveClient,
	industry IndustryClient,
) *InterviewUseCase {
	return &InterviewUseCase{
		repo:     repo,
		ai:       ai,
		archive:  archive,
		industry: industry,
	}
}

// CreateInterview 创建面试会话
func (uc *InterviewUseCase) CreateInterview(ctx context.Context, req *CreateInterviewRequest) (*Interview, *InterviewQuestion, error) {
	// 验证行业代码（gRPC 调用 Industry 服务）
	ind, err := uc.industry.GetIndustry(ctx, req.IndustryCode)
	if err != nil {
		return nil, nil, kratosErr.New(400, "INVALID_INDUSTRY", fmt.Sprintf("行业代码 %s 无效: %v", req.IndustryCode, err))
	}
	_ = ind

	interview := &Interview{
		UserID:         req.UserID,
		IndustryCode:   req.IndustryCode,
		Difficulty:     req.Difficulty,
		Status:         "created",
		InterviewMode:  req.InterviewMode,
		QuestionCount:  req.QuestionCount,
		CurrentIndex:   0,
		ResumeText:     req.ResumeText,
		JobDescription: req.JobDescription,
		Live2DModelKey: req.Live2DModelKey,
	}

	if err := uc.repo.Create(ctx, interview); err != nil {
		return nil, nil, kratosErr.InternalServer("CREATE_FAILED", "创建面试失败").WithCause(err)
	}

	// 生成第一道题（通过 AI 服务）
	aiResp, err := uc.ai.InterviewAgent(ctx, &InterviewAgentRequest{
		InterviewID:  interview.ID,
		IndustryCode: req.IndustryCode,
		Difficulty:   req.Difficulty,
		ResumeText:   req.ResumeText,
		JobDesc:      req.JobDescription,
	})
	if err != nil {
		return nil, nil, kratosErr.InternalServer("AI_FIRST_QUESTION_FAILED", "生成第一道题失败").WithCause(err)
	}
	var firstQuestion *InterviewQuestion
	if aiResp != nil {
		firstQuestion = aiResp.Question
	}

	return interview, firstQuestion, nil
}

// SubmitAnswer 提交答案并获取 AI 反馈
func (uc *InterviewUseCase) SubmitAnswer(ctx context.Context, interviewID, userID uint64, index int32, answer string) (*AnswerFeedback, *InterviewQuestion, error) {
	// 1. 获取面试会话
	interview, err := uc.repo.GetByID(ctx, interviewID)
	if err != nil {
		return nil, nil, ErrInterviewNotFound
	}
	if interview.UserID != userID {
		return nil, nil, ErrUnauthorized
	}
	if interview.Status == "completed" {
		return nil, nil, ErrInterviewFinished
	}

	// 2. 保存用户答案
	msg := &InterviewMessage{
		InterviewID:   interviewID,
		Role:          "user",
		Content:       answer,
		MessageType:   "text",
		QuestionIndex: index,
	}
	if err := uc.repo.CreateMessage(ctx, msg); err != nil {
		return nil, nil, kratosErr.InternalServer("SAVE_FAILED", "保存答案失败").WithCause(err)
	}

	// 3. 调用 AI 服务评估答案（gRPC 跨服务调用）
	history, err := uc.repo.ListMessages(ctx, interviewID)
	if err != nil {
		return nil, nil, kratosErr.InternalServer("HISTORY_FAILED", "获取历史消息失败").WithCause(err)
	}
	aiResp, err := uc.ai.InterviewAgent(ctx, &InterviewAgentRequest{
		InterviewID:   interviewID,
		IndustryCode:  interview.IndustryCode,
		Difficulty:    interview.Difficulty,
		History:       history,
		UserAnswer:    answer,
		QuestionIndex: index,
		ResumeText:    interview.ResumeText,
		JobDesc:       interview.JobDescription,
	})
	if err != nil {
		return nil, nil, kratosErr.InternalServer("AI_CALL_FAILED", "AI 服务调用失败").WithCause(err)
	}
	if aiResp == nil {
		return nil, nil, kratosErr.InternalServer("AI_EMPTY_RESPONSE", "AI 服务返回空响应")
	}

	// 4. 保存 AI 回复（nil 安全）
	feedbackText := ""
	if aiResp.Feedback != nil {
		feedbackText = aiResp.Feedback.Feedback
	}
	aiMsg := &InterviewMessage{
		InterviewID:   interviewID,
		Role:          "assistant",
		Content:       feedbackText,
		MessageType:   "text",
		QuestionIndex: index,
	}
	if err := uc.repo.CreateMessage(ctx, aiMsg); err != nil {
		return nil, nil, kratosErr.InternalServer("SAVE_AI_MSG_FAILED", "保存 AI 回复失败").WithCause(err)
	}

	// 5. 更新面试进度
	interview.CurrentIndex = index + 1
	if aiResp.ShouldEnd {
		interview.Status = "completed"
	}
	if err := uc.repo.Update(ctx, interview); err != nil {
		return nil, nil, kratosErr.InternalServer("UPDATE_FAILED", "更新面试状态失败").WithCause(err)
	}

	// 6. 同步写入学习档案（使用请求 context，随请求生命周期取消）
	// 归档失败不影响主流程，显式忽略
	_ = uc.archive.WriteEntry(ctx, &ArchiveEntry{
		UserID:          interview.UserID,
		SourceType:      "interview_answer",
		InterviewID:     interviewID,
		QuestionIndex:   index,
		IndustryCode:    interview.IndustryCode,
		EvidenceSummary: feedbackText,
		OccurredAt:      time.Now(),
	})

	return aiResp.Feedback, aiResp.Question, nil
}

// GetInterview 获取面试详情
func (uc *InterviewUseCase) GetInterview(ctx context.Context, interviewID, userID uint64) (*Interview, []*InterviewMessage, error) {
	interview, err := uc.repo.GetByID(ctx, interviewID)
	if err != nil {
		return nil, nil, ErrInterviewNotFound
	}
	if interview.UserID != userID {
		return nil, nil, ErrUnauthorized
	}

	messages, err := uc.repo.ListMessages(ctx, interviewID)
	if err != nil {
		return nil, nil, err
	}

	return interview, messages, nil
}

// ListInterviews 获取用户面试列表
func (uc *InterviewUseCase) ListInterviews(ctx context.Context, userID uint64, page, pageSize int32) ([]*Interview, int64, error) {
	return uc.repo.ListByUser(ctx, userID, page, pageSize)
}

// GetInterviewStats 供 growth 服务调用的聚合接口
func (uc *InterviewUseCase) GetInterviewStats(ctx context.Context, userID uint64) (*InterviewStats, error) {
	interviews, total, err := uc.repo.ListByUser(ctx, userID, 1, 1000)
	if err != nil {
		return nil, err
	}

	stats := &InterviewStats{
		TotalInterviews: int32(total),
	}
	var totalScore float64
	for _, iv := range interviews {
		totalScore += iv.OverallScore
	}
	if total > 0 {
		stats.AvgScore = totalScore / float64(total)
	}
	return stats, nil
}

// ProcessResumeParse MQ 消费者：处理简历解析
func (uc *InterviewUseCase) ProcessResumeParse(ctx context.Context, interviewID, userID uint64, resumeText string) error {
	// 调用 AI 服务解析简历
	_, err := uc.ai.ResumeParser(ctx, &ResumeParserRequest{ResumeText: resumeText})
	if err != nil {
		return fmt.Errorf("failed to parse resume: %w", err)
	}
	// 解析结果可用于后续面试出题
	return nil
}

// GenerateReport MQ 消费者：生成面试报告
func (uc *InterviewUseCase) GenerateReport(ctx context.Context, interviewID, userID uint64) error {
	interview, err := uc.repo.GetByID(ctx, interviewID)
	if err != nil {
		return err
	}
	messages, err := uc.repo.ListMessages(ctx, interviewID)
	if err != nil {
		return err
	}

	// 统计答题数量（用户消息数即为答题数）
	var answeredCount int32
	for _, msg := range messages {
		if msg.Role == "user" {
			answeredCount++
		}
	}

	// 基于答题数量给出基础分数（实际应由 AI 评估）
	if answeredCount > 0 {
		interview.OverallScore = float64(answeredCount) * 10.0 // 简单计算
	}
	interview.Status = "completed"
	return uc.repo.Update(ctx, interview)
}

// PersistCodingArchive MQ 消费者：持久化编程题归档
func (uc *InterviewUseCase) PersistCodingArchive(ctx context.Context, interviewID, userID uint64) error {
	// 获取面试信息
	interview, err := uc.repo.GetByID(ctx, interviewID)
	if err != nil {
		return err
	}

	// 写入学习档案
	return uc.archive.WriteEntry(ctx, &ArchiveEntry{
		UserID:        interview.UserID,
		SourceType:    "interview_coding",
		InterviewID:   interviewID,
		IndustryCode:  interview.IndustryCode,
		OccurredAt:    interview.CreatedAt,
	})
}
