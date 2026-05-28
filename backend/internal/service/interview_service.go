// Package service 提供业务逻辑层实现
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"makejob-backend/internal/ai"
	"makejob-backend/internal/common"
	"makejob-backend/internal/model"
	"makejob-backend/internal/repository"
)

// InterviewService 面试服务接口
type InterviewService interface {
	CreateInterview(ctx context.Context, userID uint, req *CreateInterviewRequest) (*InterviewResponse, error)
	GetInterview(ctx context.Context, userID, interviewID uint) (*InterviewDetailResponse, error)
	ListInterviews(ctx context.Context, userID uint, page, pageSize int) (*common.PageResult, error)
	SubmitAnswer(ctx context.Context, userID, interviewID uint, req *InterviewAnswerRequest) (*InterviewAnswerResponse, error)
	GetNextQuestion(ctx context.Context, userID, interviewID uint) (*NextQuestionResponse, error)
	FinishInterview(ctx context.Context, userID, interviewID uint) (*InterviewReportResponse, error)
	GetReport(ctx context.Context, userID, interviewID uint) (*InterviewReportResponse, error)
	IsRealtimeInterview(ctx context.Context, userID, interviewID uint) (bool, error)
	GetRealtimeContext(ctx context.Context, userID, interviewID uint) (*RealtimeInterviewContext, error)
	BindRealtimeDialog(ctx context.Context, userID, interviewID uint, dialogID string) error
	AppendRealtimeUserAnswer(ctx context.Context, userID, interviewID uint, answer string) error
	AppendRealtimeAssistantReply(ctx context.Context, userID, interviewID uint, reply string) (*ai.InterviewQuestion, int, bool, error)
}

// CreateInterviewRequest 创建面试请求DTO
type CreateInterviewRequest struct {
	IndustryCode   string   `json:"industry_code" binding:"required"`
	Difficulty     string   `json:"difficulty" binding:"omitempty,oneof=easy medium hard mixed"`
	Topics         []string `json:"topics"`
	QuestionCount  int      `json:"question_count" binding:"omitempty,min=3,max=20"`
	Live2DModelKey string   `json:"live2d_model_key"`
	InterviewMode  string   `json:"interview_mode" binding:"omitempty,oneof=general resume_driven"`
	ResumeText     string   `json:"resume_text"`
	JobDescription string   `json:"job_description"`
}

// InterviewResponse 创建面试响应DTO
type InterviewResponse struct {
	InterviewID   uint                  `json:"interview_id"`
	Status        string                `json:"status"`
	FirstQuestion *ai.InterviewQuestion `json:"first_question"`
	CreatedAt     time.Time             `json:"created_at"`
}

// InterviewDetailResponse 面试详情响应DTO
type InterviewDetailResponse struct {
	ID              uint                       `json:"id"`
	IndustryCode    string                     `json:"industry_code"`
	Live2DModelKey  string                     `json:"live2d_model_key"`
	Status          string                     `json:"status"`
	Score           float64                    `json:"score"`
	TotalQuestions  int                        `json:"total_questions"`
	Messages        []InterviewMessageResponse `json:"messages"`
	CurrentQuestion *ai.InterviewQuestion      `json:"current_question,omitempty"`
	StartedAt       *time.Time                 `json:"started_at"`
	EndedAt         *time.Time                 `json:"ended_at"`
}

// InterviewMessageResponse 面试消息响应DTO
type InterviewMessageResponse struct {
	Role        string                `json:"role"`
	Content     string                `json:"content"`
	MessageType string                `json:"message_type"`
	Question    *ai.InterviewQuestion `json:"question,omitempty"`
	CreatedAt   time.Time             `json:"created_at"`
}

// InterviewAnswerRequest 提交回答请求DTO
type InterviewAnswerRequest struct {
	Answer        string                        `json:"answer"`
	FinalCode     string                        `json:"final_code"`
	Language      string                        `json:"language"`
	QuestionType  string                        `json:"question_type"`
	ProcessEvents []InterviewCodingProcessEvent `json:"process_events"`
}

// InterviewAnswerResponse 提交回答响应DTO
type InterviewAnswerResponse struct {
	Feedback     *ai.AnswerFeedback    `json:"feedback"`
	NextQuestion *ai.InterviewQuestion `json:"next_question,omitempty"`
	IsFinished   bool                  `json:"is_finished"`
}

// NextQuestionResponse 获取下一题响应DTO
type NextQuestionResponse struct {
	Question   *ai.InterviewQuestion `json:"question"`
	QuestionNo int                   `json:"question_no"`
	IsLast     bool                  `json:"is_last"`
}

// InterviewReportResponse 面试报告响应DTO
type InterviewReportResponse struct {
	InterviewID uint                `json:"interview_id"`
	Report      *ai.InterviewReport `json:"report"`
	Duration    int64               `json:"duration_seconds"`
	CompletedAt time.Time           `json:"completed_at"`
}

// interviewService 面试服务实现
type interviewService struct {
	interviewRepo        repository.InterviewRepository
	interviewMessageRepo repository.InterviewMessageRepository
	codingAttemptRepo    repository.InterviewCodingAttemptRepository
	learningArchiveRepo  repository.LearningArchiveRepository
	interviewAgent       ai.InterviewAgent
	quizAnalyzer         ai.QuizAnalyzer
	resumeParser         ai.ResumeParser
	industryRepo         repository.IndustryRepository
	live2dDirective      Live2DDirectiveService
	realtimeEnabled      bool
}

// NewInterviewService 创建面试服务实例
func NewInterviewService(
	interviewRepo repository.InterviewRepository,
	interviewMessageRepo repository.InterviewMessageRepository,
	codingAttemptRepo repository.InterviewCodingAttemptRepository,
	learningArchiveRepo repository.LearningArchiveRepository,
	interviewAgent ai.InterviewAgent,
	quizAnalyzer ai.QuizAnalyzer,
	deps ...interface{},
) InterviewService {
	s := &interviewService{
		interviewRepo:        interviewRepo,
		interviewMessageRepo: interviewMessageRepo,
		codingAttemptRepo:    codingAttemptRepo,
		learningArchiveRepo:  learningArchiveRepo,
		interviewAgent:       interviewAgent,
		quizAnalyzer:         quizAnalyzer,
	}
	for _, dep := range deps {
		switch value := dep.(type) {
		case repository.IndustryRepository:
			s.industryRepo = value
		case Live2DDirectiveService:
			s.live2dDirective = value
		case RealtimeInterviewServiceOption:
			s.realtimeEnabled = value.Enabled
		case ai.ResumeParser:
			s.resumeParser = value
		}
	}
	return s
}

// CreateInterview 创建面试会话
func (s *interviewService) CreateInterview(ctx context.Context, userID uint, req *CreateInterviewRequest) (*InterviewResponse, error) {
	// 解析行业ID（从行业代码转换）
	var industryID uint
	if s.industryRepo != nil {
		industry, err := s.industryRepo.GetByCode(ctx, req.IndustryCode)
		if err != nil {
			return nil, fmt.Errorf("查询行业失败: %w", err)
		}
		if industry != nil {
			industryID = industry.ID
		}
	}
	if industryID == 0 {
		industryID = parseIndustryCode(req.IndustryCode)
	}

	// 创建面试记录
	now := time.Now()
	interview := &model.MockInterview{
		UserID:         userID,
		IndustryID:     industryID,
		Status:         model.InterviewStatusOngoing,
		Live2DModelKey: strings.TrimSpace(req.Live2DModelKey),
		TotalQuestions: req.QuestionCount,
		StartedAt:      &now,
	}

	if err := s.interviewRepo.Create(ctx, interview); err != nil {
		return nil, err
	}

	if s.realtimeEnabled {
		metadata := buildRealtimeInterviewMetadata(req)
		if metadata.InterviewMode == "resume_driven" {
			if strings.TrimSpace(req.ResumeText) == "" {
				return nil, common.NewBusinessError(common.CodeBadRequest, "简历驱动面试模式需要提供简历文本")
			}
			if s.resumeParser != nil {
				profile, parseErr := s.resumeParser.Parse(ctx, req.ResumeText, req.JobDescription)
				if parseErr != nil {
					return nil, common.NewBusinessError(common.CodeInternalError, "简历解析失败: "+parseErr.Error())
				}
				if profile != nil {
					if profileJSON, marshalErr := json.Marshal(profile); marshalErr == nil {
						metadata.ResumeProfileJSON = string(profileJSON)
					}
				}
			}
		}
		interview.AISessionID = encodeRealtimeDialogID("")
		interview.AIFeedback = metadata.toStorageValue()
		if err := s.interviewRepo.Update(ctx, interview); err != nil {
			return nil, err
		}

		return &InterviewResponse{
			InterviewID: interview.ID,
			Status:      interview.Status,
			CreatedAt:   now,
		}, nil
	}

	// 调用AI开始面试
	config := ai.InterviewConfig{
		IndustryCode:   req.IndustryCode,
		Difficulty:     req.Difficulty,
		Topics:         req.Topics,
		QuestionCount:  req.QuestionCount,
		UserWeakTopics: s.resolveUserWeakTopicsForInterview(ctx, userID),
	}

	sessionID, firstQuestion, err := s.interviewAgent.StartInterview(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("启动AI面试失败: %w", err)
	}
	s.decorateInterviewQuestionWithLive2D(ctx, interview, &firstQuestion, nil)

	// 保存sessionID到独立字段，避免后续被报告摘要覆盖。
	interview.AISessionID = sessionID
	if err := s.interviewRepo.Update(ctx, interview); err != nil {
		return nil, err
	}

	// 保存第一个问题为消息
	questionMsg, err := buildInterviewQuestionMessage(interview.ID, firstQuestion)
	if err != nil {
		return nil, err
	}
	if err := s.interviewMessageRepo.Create(ctx, questionMsg); err != nil {
		return nil, err
	}

	return &InterviewResponse{
		InterviewID:   interview.ID,
		Status:        interview.Status,
		FirstQuestion: &firstQuestion,
		CreatedAt:     now,
	}, nil
}

// GetInterview 获取面试详情
func (s *interviewService) GetInterview(ctx context.Context, userID, interviewID uint) (*InterviewDetailResponse, error) {
	interview, err := s.interviewRepo.GetByID(ctx, interviewID)
	if err != nil {
		return nil, err
	}
	if interview == nil {
		return nil, common.NewBusinessError(common.CodeNotFound, "面试记录不存在")
	}

	// 验证面试归属
	if interview.UserID != userID {
		return nil, common.NewBusinessError(common.CodeForbidden, "无权访问该面试记录")
	}

	// 获取消息列表
	messages, err := s.interviewMessageRepo.ListByInterview(ctx, interviewID)
	if err != nil {
		return nil, err
	}

	messageResponses := make([]InterviewMessageResponse, len(messages))
	for i, msg := range messages {
		question := parseInterviewQuestionMetadata(msg.MetadataJSON)
		messageResponses[i] = InterviewMessageResponse{
			Role:        msg.Role,
			Content:     msg.Content,
			MessageType: msg.MessageType,
			Question:    question,
			CreatedAt:   time.Unix(msg.CreatedAt, 0),
		}
	}

	// resolveInterviewIndustryCode 解析面试记录对应的行业编码。
	industryCode := s.resolveInterviewIndustryCode(ctx, interview.IndustryID)

	return &InterviewDetailResponse{
		ID:              interview.ID,
		IndustryCode:    industryCode,
		Live2DModelKey:  strings.TrimSpace(interview.Live2DModelKey),
		Status:          interview.Status,
		Score:           interview.Score,
		TotalQuestions:  interview.TotalQuestions,
		Messages:        messageResponses,
		CurrentQuestion: resolveCurrentInterviewQuestionFromMessages(messageResponses, interview.TotalQuestions, interview.Status),
		StartedAt:       interview.StartedAt,
		EndedAt:         interview.EndedAt,
	}, nil
}

// IsRealtimeInterview 判断当前面试记录是否属于实时语音面试链路。
func (s *interviewService) IsRealtimeInterview(ctx context.Context, userID, interviewID uint) (bool, error) {
	interview, err := s.interviewRepo.GetByID(ctx, interviewID)
	if err != nil {
		return false, err
	}
	if interview == nil {
		return false, common.NewBusinessError(common.CodeNotFound, "面试记录不存在")
	}
	if interview.UserID != userID {
		return false, common.NewBusinessError(common.CodeForbidden, "无权访问该面试记录")
	}
	return isRealtimeInterviewRecord(interview), nil
}

// resolveInterviewIndustryCode 解析面试记录的行业编码，失败时返回空字符串。
func (s *interviewService) resolveInterviewIndustryCode(ctx context.Context, industryID uint) string {
	if industryID == 0 || s.industryRepo == nil {
		return ""
	}

	industry, err := s.industryRepo.GetByID(ctx, industryID)
	if err != nil || industry == nil {
		return ""
	}

	return industry.Code
}

// ListInterviews 获取面试列表
func (s *interviewService) ListInterviews(ctx context.Context, userID uint, page, pageSize int) (*common.PageResult, error) {
	pageParam := common.PageParam{Page: page, PageSize: pageSize}
	pageParam.Normalize()

	interviews, total, err := s.interviewRepo.ListByUser(ctx, userID, pageParam.Page, pageParam.PageSize)
	if err != nil {
		return nil, err
	}

	return common.NewPageResult(interviews, total, pageParam), nil
}

// SubmitAnswer 提交回答
func (s *interviewService) SubmitAnswer(ctx context.Context, userID, interviewID uint, req *InterviewAnswerRequest) (*InterviewAnswerResponse, error) {
	interview, err := s.interviewRepo.GetByID(ctx, interviewID)
	if err != nil {
		return nil, err
	}
	if interview == nil {
		return nil, common.NewBusinessError(common.CodeNotFound, "面试记录不存在")
	}

	// 验证面试归属
	if interview.UserID != userID {
		return nil, common.NewBusinessError(common.CodeForbidden, "无权访问该面试记录")
	}

	// 验证面试状态
	if !interview.IsOngoing() {
		return nil, common.NewBusinessError(common.CodeBadRequest, "面试已结束")
	}

	if strings.TrimSpace(req.Answer) == "" && strings.TrimSpace(req.FinalCode) == "" {
		return nil, common.NewBusinessError(common.CodeBadRequest, "回答内容不能为空")
	}

	// 获取sessionID
	sessionID := resolveInterviewSessionID(interview)
	if sessionID == "" {
		return nil, common.NewBusinessError(common.CodeInternalError, "面试会话不存在")
	}

	// 获取当前消息列表并基于用户已回答数确定题目索引，避免反馈消息打乱计数。
	messages, err := s.interviewMessageRepo.ListByInterview(ctx, interviewID)
	if err != nil {
		return nil, err
	}

	questionIndex := countAnsweredInterviewQuestions(messages)
	if questionIndex >= interview.TotalQuestions {
		return nil, common.NewBusinessError(common.CodeBadRequest, "已完成所有题目")
	}

	currentQuestion := resolveCurrentQuestionFromStoredMessages(messages)
	if currentQuestion != nil && strings.EqualFold(strings.TrimSpace(currentQuestion.Type), "coding") {
		if strings.TrimSpace(req.FinalCode) == "" {
			return nil, common.NewBusinessError(common.CodeBadRequest, "编程题必须提交最终代码")
		}

		if err := s.persistCodingAttempt(ctx, interview, questionIndex, currentQuestion, req); err != nil {
			return nil, err
		}
	}

	// 保存用户回答
	questionType := ""
	if currentQuestion != nil {
		questionType = currentQuestion.Type
	}
	answerMsg := buildInterviewAnswerMessage(interviewID, questionType, req.Answer, req.FinalCode, req.Language)
	if err := s.interviewMessageRepo.Create(ctx, answerMsg); err != nil {
		return nil, err
	}

	// 检查是否还有下一题
	isFinished := questionIndex+1 >= interview.TotalQuestions
	var nextQuestion *ai.InterviewQuestion

	if !isFinished {
		// 自动获取下一题
		nextQ, hasNext, err := s.interviewAgent.GetNextQuestion(ctx, sessionID)
		if err != nil {
			return nil, fmt.Errorf("获取下一题失败: %w", err)
		}
		if hasNext {
			s.decorateInterviewQuestionWithLive2D(ctx, interview, &nextQ, currentQuestion)
			nextQuestion = &nextQ
			// 保存下一题
			questionMsg, err := buildInterviewQuestionMessage(interviewID, nextQ)
			if err != nil {
				return nil, err
			}
			if err := s.interviewMessageRepo.Create(ctx, questionMsg); err != nil {
				return nil, err
			}
		} else {
			isFinished = true
		}
	}

	return &InterviewAnswerResponse{
		Feedback:     nil,
		NextQuestion: nextQuestion,
		IsFinished:   isFinished,
	}, nil
}

// GetNextQuestion 获取下一题
func (s *interviewService) GetNextQuestion(ctx context.Context, userID, interviewID uint) (*NextQuestionResponse, error) {
	interview, err := s.interviewRepo.GetByID(ctx, interviewID)
	if err != nil {
		return nil, err
	}
	if interview == nil {
		return nil, common.NewBusinessError(common.CodeNotFound, "面试记录不存在")
	}

	// 验证面试归属
	if interview.UserID != userID {
		return nil, common.NewBusinessError(common.CodeForbidden, "无权访问该面试记录")
	}

	// 验证面试状态
	if !interview.IsOngoing() {
		return nil, common.NewBusinessError(common.CodeBadRequest, "面试已结束")
	}

	// 获取sessionID
	sessionID := resolveInterviewSessionID(interview)
	if sessionID == "" {
		return nil, common.NewBusinessError(common.CodeInternalError, "面试会话不存在")
	}

	// 获取当前消息列表并基于已回答数量确定下一题编号，避免反馈消息影响编号。
	messages, err := s.interviewMessageRepo.ListByInterview(ctx, interviewID)
	if err != nil {
		return nil, err
	}

	nextQuestionNo := countAnsweredInterviewQuestions(messages) + 1
	if nextQuestionNo > interview.TotalQuestions {
		return nil, common.NewBusinessError(common.CodeBadRequest, "已完成所有题目")
	}

	// 调用AI获取下一题
	question, hasNext, err := s.interviewAgent.GetNextQuestion(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("获取下一题失败: %w", err)
	}

	if !hasNext {
		return &NextQuestionResponse{
			Question:   nil,
			QuestionNo: nextQuestionNo,
			IsLast:     true,
		}, nil
	}
	s.decorateInterviewQuestionWithLive2D(ctx, interview, &question, resolveCurrentQuestionFromStoredMessages(messages))

	// 保存问题到消息
	questionMsg, err := buildInterviewQuestionMessage(interviewID, question)
	if err != nil {
		return nil, err
	}
	if err := s.interviewMessageRepo.Create(ctx, questionMsg); err != nil {
		return nil, err
	}

	return &NextQuestionResponse{
		Question:   &question,
		QuestionNo: nextQuestionNo,
		IsLast:     !hasNext || nextQuestionNo >= interview.TotalQuestions,
	}, nil
}

// FinishInterview 结束面试
func (s *interviewService) FinishInterview(ctx context.Context, userID, interviewID uint) (*InterviewReportResponse, error) {
	interview, err := s.interviewRepo.GetByID(ctx, interviewID)
	if err != nil {
		return nil, err
	}
	if interview == nil {
		return nil, common.NewBusinessError(common.CodeNotFound, "面试记录不存在")
	}

	// 验证面试归属
	if interview.UserID != userID {
		return nil, common.NewBusinessError(common.CodeForbidden, "无权访问该面试记录")
	}

	// 验证面试状态
	if !interview.IsOngoing() {
		if interview.IsCompleted() {
			return s.GetReport(ctx, userID, interviewID)
		}
		return nil, common.NewBusinessError(common.CodeBadRequest, "面试已结束")
	}

	// 获取sessionID
	sessionID := resolveInterviewSessionID(interview)
	if sessionID == "" && !isRealtimeInterviewSessionID(interview.AISessionID) {
		return nil, common.NewBusinessError(common.CodeInternalError, "面试会话不存在")
	}

	if isRealtimeInterviewSessionID(interview.AISessionID) {
		return s.finishRealtimeInterview(ctx, userID, interview)
	}

	// 在面试结束时统一补做评分，避免过程内打断节奏，同时保证报告仍有完整依据。
	if err := s.evaluateInterviewAnswersForReport(ctx, interviewID, sessionID); err != nil {
		return nil, fmt.Errorf("补全作答评分失败: %w", err)
	}

	// 生成面试报告
	report, err := s.interviewAgent.GenerateReport(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("生成面试报告失败: %w", err)
	}

	if err := s.enrichInterviewReportWithCoding(ctx, interview, &report); err != nil {
		return nil, fmt.Errorf("生成编程题诊断失败: %w", err)
	}
	reportJSON, err := serializeInterviewReport(report)
	if err != nil {
		return nil, fmt.Errorf("序列化面试报告失败: %w", err)
	}

	now := time.Now()
	if err := s.persistLearningArchiveEntries(ctx, interview, report, now); err != nil {
		return nil, err
	}

	// 更新面试记录
	interview.Status = model.InterviewStatusCompleted
	interview.Score = report.OverallScore
	interview.EndedAt = &now
	interview.ReportJSON = reportJSON
	interview.AIFeedback = buildInterviewReportSummary(report)

	if err := s.interviewRepo.Update(ctx, interview); err != nil {
		return nil, err
	}

	// 结束AI会话
	if err := s.interviewAgent.EndInterview(ctx, sessionID); err != nil {
		// 记录错误但不影响主流程
		// TODO: 记录日志
	}

	// 计算面试时长
	var duration int64
	if interview.StartedAt != nil {
		duration = int64(now.Sub(*interview.StartedAt).Seconds())
	}

	return &InterviewReportResponse{
		InterviewID: interviewID,
		Report:      &report,
		Duration:    duration,
		CompletedAt: now,
	}, nil
}

// evaluateInterviewAnswersForReport 在结束面试前按回答顺序补齐评分结果，供报告生成使用。
func (s *interviewService) evaluateInterviewAnswersForReport(ctx context.Context, interviewID uint, sessionID string) error {
	// 某些测试或精简场景不会注入消息仓库，此时直接跳过补评分，保持结束流程可继续执行。
	if s.interviewMessageRepo == nil {
		return nil
	}

	messages, err := s.interviewMessageRepo.ListByInterview(ctx, interviewID)
	if err != nil {
		return err
	}

	answeredCount := 0
	for _, item := range messages {
		if item.Role != model.MessageRoleUser {
			continue
		}
		answer := strings.TrimSpace(item.Content)
		if answer == "" {
			continue
		}
		if _, err := s.interviewAgent.EvaluateAnswer(ctx, sessionID, answeredCount, answer); err != nil {
			return err
		}
		answeredCount++
	}

	return nil
}

// GetReport 获取面试报告
func (s *interviewService) GetReport(ctx context.Context, userID, interviewID uint) (*InterviewReportResponse, error) {
	interview, err := s.interviewRepo.GetByID(ctx, interviewID)
	if err != nil {
		return nil, err
	}
	if interview == nil {
		return nil, common.NewBusinessError(common.CodeNotFound, "面试记录不存在")
	}

	// 验证面试归属
	if interview.UserID != userID {
		return nil, common.NewBusinessError(common.CodeForbidden, "无权访问该面试记录")
	}

	// 验证面试状态
	if interview.IsOngoing() {
		return nil, common.NewBusinessError(common.CodeBadRequest, "面试尚未结束")
	}

	// 计算面试时长
	var duration int64
	completedAt := time.Now()
	if interview.EndedAt != nil {
		completedAt = *interview.EndedAt
		if interview.StartedAt != nil {
			duration = int64(interview.EndedAt.Sub(*interview.StartedAt).Seconds())
		}
	}

	// 优先读取持久化的完整报告，兼容历史数据时再回退到摘要结构。
	report, err := parseStoredInterviewReport(interview.ReportJSON)
	if err != nil {
		return nil, fmt.Errorf("解析面试报告失败: %w", err)
	}
	if report == nil {
		report = buildFallbackInterviewReport(interview)
	}

	return &InterviewReportResponse{
		InterviewID: interviewID,
		Report:      report,
		Duration:    duration,
		CompletedAt: completedAt,
	}, nil
}

// resolveInterviewSessionID 返回当前面试可用的 AI 会话 ID，并兼容历史字段回退。
func resolveInterviewSessionID(interview *model.MockInterview) string {
	if interview == nil {
		return ""
	}
	if strings.TrimSpace(interview.AISessionID) != "" {
		return strings.TrimSpace(interview.AISessionID)
	}
	if interview.IsOngoing() && !strings.Contains(strings.TrimSpace(interview.AIFeedback), `"mode":"realtime_dialog"`) {
		return strings.TrimSpace(interview.AIFeedback)
	}
	return ""
}

// isRealtimeInterviewRecord 判断一条面试记录是否由实时语音链路创建。
func isRealtimeInterviewRecord(interview *model.MockInterview) bool {
	if interview == nil {
		return false
	}
	if isRealtimeInterviewSessionID(interview.AISessionID) {
		return true
	}
	return strings.Contains(strings.TrimSpace(interview.AIFeedback), `"mode":"realtime_dialog"`)
}

// serializeInterviewReport 将完整面试报告序列化为持久化字符串。
func serializeInterviewReport(report ai.InterviewReport) (string, error) {
	payload, err := json.Marshal(report)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

// parseStoredInterviewReport 解析数据库中保存的完整面试报告 JSON。
func parseStoredInterviewReport(raw string) (*ai.InterviewReport, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}

	var report ai.InterviewReport
	if err := json.Unmarshal([]byte(raw), &report); err != nil {
		return nil, err
	}
	return &report, nil
}

// buildInterviewReportSummary 生成报告摘要，供历史列表和兼容字段快速展示。
func buildInterviewReportSummary(report ai.InterviewReport) string {
	return fmt.Sprintf("总分: %.0f, 正确: %d/%d", report.OverallScore, report.CorrectCount, report.TotalQuestions)
}

// buildFallbackInterviewReport 为历史旧数据构造最小可用报告结构。
func buildFallbackInterviewReport(interview *model.MockInterview) *ai.InterviewReport {
	return &ai.InterviewReport{
		OverallScore:   interview.Score,
		TotalQuestions: interview.TotalQuestions,
		Summary:        interview.AIFeedback,
	}
}

// countAnsweredInterviewQuestions 统计当前面试中用户已经完成作答的题目数量。
func countAnsweredInterviewQuestions(messages []model.InterviewMessage) int {
	count := 0
	for _, item := range messages {
		if item.Role == model.MessageRoleUser {
			count++
		}
	}
	return count
}

// resolveCurrentQuestionFromStoredMessages 从持久化消息中恢复当前问题及其元数据。
func resolveCurrentQuestionFromStoredMessages(messages []model.InterviewMessage) *ai.InterviewQuestion {
	for index := len(messages) - 1; index >= 0; index-- {
		item := messages[index]
		if item.Role != model.MessageRoleAI || item.MessageType != model.MessageTypeText {
			continue
		}

		question := parseInterviewQuestionMetadata(item.MetadataJSON)
		if question != nil {
			return question
		}
		if strings.TrimSpace(item.Content) == "" {
			continue
		}
		return &ai.InterviewQuestion{
			Question: item.Content,
			Type:     "technical",
		}
	}
	return nil
}

// persistCodingAttempt 将当前编程题的最终代码和过程事件写入持久化记录。
func (s *interviewService) persistCodingAttempt(
	ctx context.Context,
	interview *model.MockInterview,
	questionIndex int,
	question *ai.InterviewQuestion,
	req *InterviewAnswerRequest,
) error {
	if s.codingAttemptRepo == nil || interview == nil || question == nil {
		return nil
	}

	attempt, events, err := buildCodingAttemptFromRequest(interview.ID, interview.UserID, questionIndex, *question, req)
	if err != nil {
		return fmt.Errorf("构造编程题作答记录失败: %w", err)
	}
	if err := s.codingAttemptRepo.UpsertAttempt(ctx, attempt); err != nil {
		return err
	}

	for index := range events {
		events[index].AttemptID = attempt.ID
	}
	if err := s.codingAttemptRepo.ReplaceEvents(ctx, attempt.ID, events); err != nil {
		return err
	}
	return nil
}

// enrichInterviewReportWithCoding 为面试报告补齐编程题过程诊断结果。
func (s *interviewService) enrichInterviewReportWithCoding(ctx context.Context, interview *model.MockInterview, report *ai.InterviewReport) error {
	if s.codingAttemptRepo == nil || interview == nil || report == nil {
		return nil
	}

	attempts, err := s.codingAttemptRepo.ListByInterview(ctx, interview.ID)
	if err != nil {
		return err
	}
	if len(attempts) == 0 {
		return nil
	}

	diagnostics := make([]ai.CodingQuestionDiagnosis, 0, len(attempts))
	for _, attempt := range attempts {
		processEvents := convertAttemptEventsToAI(attempt.Events)
		diagnosis, err := s.generateCodingDiagnosis(ctx, &attempt, processEvents)
		if err != nil {
			return err
		}

		diagnosis.QuestionIndex = attempt.QuestionIndex
		attempt.ProcessSummary = diagnosis.ProcessSummary
		if diagnosisJSON, marshalErr := json.Marshal(diagnosis); marshalErr == nil {
			attempt.DiagnosisJSON = string(diagnosisJSON)
		}
		if err := s.codingAttemptRepo.UpsertAttempt(ctx, &attempt); err != nil {
			return err
		}
		diagnostics = append(diagnostics, diagnosis)
	}

	report.CodingDiagnostics = diagnostics
	report.Weaknesses = mergeInterviewReportWeaknesses(report.Weaknesses, diagnostics)
	report.Suggestions = mergeInterviewReportSuggestions(report.Suggestions, diagnostics)
	return nil
}

// generateCodingDiagnosis 优先调用 AI 诊断器，失败时回退到本地规则诊断。
func (s *interviewService) generateCodingDiagnosis(
	ctx context.Context,
	attempt *model.InterviewCodingAttempt,
	processEvents []ai.CodingProcessEvent,
) (ai.CodingQuestionDiagnosis, error) {
	if attempt == nil {
		return ai.CodingQuestionDiagnosis{}, fmt.Errorf("编程题作答记录不存在")
	}

	if s.quizAnalyzer != nil {
		diagnosis, err := s.quizAnalyzer.DiagnoseInterviewCoding(ctx, ai.InterviewCodingDiagnosisInput{
			Question:      attempt.QuestionPrompt,
			Language:      attempt.Language,
			FinalCode:     attempt.FinalCode,
			FinalAnswer:   attempt.FinalAnswer,
			ProcessEvents: processEvents,
		})
		if err == nil {
			if strings.TrimSpace(diagnosis.ProcessSummary) == "" {
				diagnosis.ProcessSummary = buildCodingProcessSummary(processEvents, 0, 0, 0)
			}
			return diagnosis, nil
		}
	}

	return buildLocalCodingDiagnosis(attempt, processEvents), nil
}

// persistLearningArchiveEntries 将编程题诊断结果沉淀到长期学习档案中。
func (s *interviewService) persistLearningArchiveEntries(
	ctx context.Context,
	interview *model.MockInterview,
	report ai.InterviewReport,
	occurredAt time.Time,
) error {
	if s.learningArchiveRepo == nil || interview == nil {
		return nil
	}

	industryCode := s.resolveInterviewIndustryCode(ctx, interview.IndustryID)

	for _, diagnosis := range report.CodingDiagnostics {
		mistakeTagsJSON, err := json.Marshal(diagnosis.MistakeTags)
		if err != nil {
			return fmt.Errorf("序列化学习档案错因标签失败: %w", err)
		}
		strengthTagsJSON, err := json.Marshal(diagnosis.StrengthTags)
		if err != nil {
			return fmt.Errorf("序列化学习档案优势标签失败: %w", err)
		}
		suggestionsJSON, err := json.Marshal(diagnosis.Suggestions)
		if err != nil {
			return fmt.Errorf("序列化学习档案建议失败: %w", err)
		}

		entry := &model.LearningArchiveEntry{
			UserID:           interview.UserID,
			SourceType:       model.LearningArchiveSourceInterviewCoding,
			SourceRef:        fmt.Sprintf("interview:%d:question:%d", interview.ID, diagnosis.QuestionIndex),
			InterviewID:      interview.ID,
			QuestionIndex:    diagnosis.QuestionIndex,
			IndustryCode:     industryCode,
			TaskPhase:        model.LearningPhaseMock,
			TaskPhaseGoal:    model.BuildLearningPhaseGoal(model.LearningPhaseMock),
			Language:         diagnosis.Language,
			MistakeTagsJSON:  string(mistakeTagsJSON),
			StrengthTagsJSON: string(strengthTagsJSON),
			SuggestionsJSON:  string(suggestionsJSON),
			EvidenceSummary:  strings.Join(diagnosis.Evidence, "；"),
			OccurredAt:       &occurredAt,
		}
		if err := s.learningArchiveRepo.Upsert(ctx, entry); err != nil {
			return err
		}
	}

	if len(report.CodingDiagnostics) == 0 && len(report.Weaknesses) > 0 {
		mistakeTagsJSON, err := json.Marshal(report.Weaknesses)
		if err != nil {
			return fmt.Errorf("序列化面试报告薄弱项失败: %w", err)
		}
		suggestionsJSON, err := json.Marshal(report.Suggestions)
		if err != nil {
			return fmt.Errorf("序列化面试报告建议失败: %w", err)
		}

		entry := &model.LearningArchiveEntry{
			UserID:          interview.UserID,
			SourceType:      model.LearningArchiveSourceInterviewCoding,
			SourceRef:       fmt.Sprintf("interview:%d:report", interview.ID),
			InterviewID:     interview.ID,
			IndustryCode:    industryCode,
			TaskPhase:       model.LearningPhaseMock,
			TaskPhaseGoal:   model.BuildLearningPhaseGoal(model.LearningPhaseMock),
			MistakeTagsJSON: string(mistakeTagsJSON),
			SuggestionsJSON: string(suggestionsJSON),
			OccurredAt:      &occurredAt,
		}
		if err := s.learningArchiveRepo.Upsert(ctx, entry); err != nil {
			return err
		}
	}

	return nil
}

// mergeInterviewReportWeaknesses 将编程题错因标签补充到总报告薄弱项里。
func mergeInterviewReportWeaknesses(existing []string, diagnostics []ai.CodingQuestionDiagnosis) []string {
	result := appendUniqueStrings(existing)
	for _, diagnosis := range diagnostics {
		result = appendUniqueStrings(result, diagnosis.MistakeTags...)
	}
	return result
}

// mergeInterviewReportSuggestions 将编程题建议补充到总报告建议里。
func mergeInterviewReportSuggestions(existing []string, diagnostics []ai.CodingQuestionDiagnosis) []string {
	result := appendUniqueStrings(existing)
	for _, diagnosis := range diagnostics {
		result = appendUniqueStrings(result, diagnosis.Suggestions...)
	}
	return result
}

// decorateInterviewQuestionWithLive2D 为当前题目补齐结构化 Live2D 指令，失败时静默回退主链路。
func (s *interviewService) decorateInterviewQuestionWithLive2D(
	ctx context.Context,
	interview *model.MockInterview,
	question *ai.InterviewQuestion,
	previousQuestion *ai.InterviewQuestion,
) {
	if s.live2dDirective == nil || interview == nil || question == nil {
		return
	}
	if strings.TrimSpace(interview.Live2DModelKey) == "" || strings.TrimSpace(question.Question) == "" {
		return
	}

	manifest, err := s.live2dDirective.ResolveActiveManifest(ctx, model.Live2DSceneInterview, interview.Live2DModelKey)
	if err != nil || manifest == nil {
		return
	}

	directiveCtx, cancel := context.WithTimeout(ctx, live2DDirectiveTimeout)
	defer cancel()

	directive, err := s.live2dDirective.GenerateDirective(directiveCtx, ai.Live2DDirectiveContext{
		Scene:          model.Live2DSceneInterview,
		Model:          *manifest,
		AssistantReply: strings.TrimSpace(question.Question),
		Question:       question,
		QuestionIndex:  0,
		CurrentDirective: func() *ai.Live2DDirective {
			if previousQuestion == nil {
				return nil
			}
			return previousQuestion.Live2DDirective
		}(),
	})
	if err != nil {
		return
	}
	question.Live2DDirective = directive
}

// resolveUserWeakTopicsForInterview 从学习档案中提取用户近期高频薄弱主题，供面试出题参考。
func (s *interviewService) resolveUserWeakTopicsForInterview(ctx context.Context, userID uint) []string {
	if s.learningArchiveRepo == nil {
		return nil
	}

	entries, err := s.learningArchiveRepo.ListRecentByUser(ctx, userID, 20, nil)
	if err != nil || len(entries) == 0 {
		return nil
	}

	signals := buildTrainingFocusSignals(entries, nil, 3)
	if len(signals) == 0 {
		return nil
	}

	tags := make([]string, 0, len(signals))
	for _, signal := range signals {
		if tag := strings.TrimSpace(signal.Tag); tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags
}

// parseIndustryCode 解析行业代码为行业ID（简化实现）
func parseIndustryCode(code string) uint {
	// 简单的映射，实际应该从数据库查询
	switch code {
	case "go", "golang":
		return 1
	case "java":
		return 2
	case "python":
		return 3
	case "frontend":
		return 4
	case "ai":
		return 5
	default:
		return 1
	}
}
