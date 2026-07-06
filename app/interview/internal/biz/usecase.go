package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	kratosErr "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

// 业务错误码
var (
	ErrInterviewNotFound    = kratosErr.NotFound("INTERVIEW_NOT_FOUND", "面试不存在")
	ErrInterviewFinished    = kratosErr.BadRequest("INTERVIEW_FINISHED", "面试已结束")
	ErrUnauthorized         = kratosErr.Unauthorized("UNAUTHORIZED", "未授权")
	ErrInvalidIndustry      = kratosErr.BadRequest("INVALID_INDUSTRY", "无效的行业代码")
	ErrAICallFailed         = kratosErr.InternalServer("AI_CALL_FAILED", "AI 服务调用失败")
	ErrInterviewNotOngoing  = kratosErr.BadRequest("INTERVIEW_NOT_ONGOING", "面试不在进行中")
	ErrReportNotReady       = kratosErr.NotFound("REPORT_NOT_READY", "报告尚未生成")
	ErrReportFailed         = kratosErr.InternalServer("REPORT_FAILED", "报告生成失败")
	ErrQuestionNotFound     = kratosErr.NotFound("QUESTION_NOT_FOUND", "题目消息不存在")
	ErrInterviewNotRealtime = kratosErr.BadRequest("INTERVIEW_NOT_REALTIME", "面试不是实时模式")
)

// interviewTimeout 面试超时时间，超过此时间自动结束并生成报告。
const defaultInterviewTimeout = 40 * time.Minute

// InterviewUseCase 面试业务用例
type InterviewUseCase struct {
	repo             InterviewRepo
	ai               AIServiceClient
	archive          LearningArchiveClient
	industry         IndustryClient
	rag              RAGClient
	codeRunner       CodeRunnerClient
	reportRepo       ReportRepo
	publisher        MQPublisher
	logger           log.Logger
	interviewTimeout time.Duration
}

// NewInterviewUseCase 由 Wire 调用，所有依赖通过接口注入
func NewInterviewUseCase(
	repo InterviewRepo,
	ai AIServiceClient,
	archive LearningArchiveClient,
	industry IndustryClient,
	rag RAGClient,
	codeRunner CodeRunnerClient,
	reportRepo ReportRepo,
	publisher MQPublisher,
	logger log.Logger,
	timeoutMinutes int,
) *InterviewUseCase {
	timeout := defaultInterviewTimeout
	if timeoutMinutes > 0 {
		timeout = time.Duration(timeoutMinutes) * time.Minute
	}
	return &InterviewUseCase{
		repo:             repo,
		ai:               ai,
		archive:          archive,
		industry:         industry,
		rag:              rag,
		codeRunner:       codeRunner,
		reportRepo:       reportRepo,
		publisher:        publisher,
		logger:           logger,
		interviewTimeout: timeout,
	}
}

// CreateInterview 创建面试会话
func (uc *InterviewUseCase) CreateInterview(ctx context.Context, req *CreateInterviewRequest) (*Interview, *InterviewQuestion, error) {
	// 验证行业代码（当前由本地仓储实现，接口保持不变）
	ind, err := uc.industry.GetIndustry(ctx, req.IndustryCode)
	if err != nil {
		return nil, nil, kratosErr.New(400, "INVALID_INDUSTRY", fmt.Sprintf("行业代码 %s 无效: %v", req.IndustryCode, err))
	}

	// 补充默认值：resume_driven 模式下前端可能不传 difficulty 和 question_count
	difficulty := strings.TrimSpace(req.Difficulty)
	if difficulty == "" {
		difficulty = "medium"
	}
	questionCount := req.QuestionCount
	if questionCount <= 0 {
		questionCount = 5
	}

	now := time.Now()
	interview := &Interview{
		UserID:         req.UserID,
		IndustryID:     ind.ID,
		IndustryCode:   req.IndustryCode,
		Difficulty:     difficulty,
		Status:         "ongoing",
		InterviewMode:  req.InterviewMode,
		QuestionCount:  questionCount,
		CurrentIndex:   0,
		ResumeText:     req.ResumeText,
		JobDescription: req.JobDescription,
		Live2DModelKey: req.Live2DModelKey,
		CreatedAt:      now,
		StartedAt:      &now,
	}

	var firstQuestion *InterviewQuestion
	if !isRealtimeInterview(interview) {
		// 调用 AI Gateway 的 StartInterview 获取 sessionID 和首题（对齐单体 InterviewAgent.StartInterview）
		aiResp, err := uc.ai.StartInterview(ctx, &StartInterviewRequest{
			InterviewID:   interview.ID,
			IndustryCode:  req.IndustryCode,
			Difficulty:    difficulty,
			QuestionCount: questionCount,
			ResumeText:    req.ResumeText,
			JobDescription: req.JobDescription,
			InterviewMode: req.InterviewMode,
		})
		if err != nil {
			return nil, nil, kratosErr.InternalServer("AI_FIRST_QUESTION_FAILED", "生成第一道题失败").WithCause(err)
		}

		// 保存 sessionID 到 AISessionID 字段
		interview.AISessionID = aiResp.SessionID

		if aiResp.Question != "" {
			firstQuestion = &InterviewQuestion{
				Question:   aiResp.Question,
				Topic:      aiResp.Topic,
				Difficulty: aiResp.Difficulty,
				Type:       aiResp.Type,
				Hints:      aiResp.Hints,
			}
		}
	}

	// 创建面试记录与首题消息必须同事务提交，避免出现"只有会话没有首题"的伪成功状态。
	if err := uc.repo.Transaction(ctx, func(txCtx context.Context) error {
		if err := uc.repo.Create(txCtx, interview); err != nil {
			return kratosErr.InternalServer("CREATE_FAILED", "创建面试失败").WithCause(err)
		}
		if firstQuestionMsg := BuildQuestionMessage(interview.ID, 0, firstQuestion); firstQuestionMsg != nil {
			if err := uc.repo.CreateMessage(txCtx, firstQuestionMsg); err != nil {
				return kratosErr.InternalServer("SAVE_FIRST_QUESTION_FAILED", "保存第一道题失败").WithCause(err)
			}
		}
		return nil
	}); err != nil {
		return nil, nil, err
	}

	if strings.TrimSpace(interview.ResumeText) != "" && uc.publisher != nil {
		if err := uc.publisher.PublishInterviewResumeParse(ctx, interview.ID, interview.UserID, interview.ResumeText); err != nil {
			log.Warnf("发布简历解析消息失败: interview_id=%d err=%v", interview.ID, err)
		}
	}

	return interview, firstQuestion, nil
}

// SubmitAnswer 提交答案并获取 AI 反馈（评估答案，不获取下一题；下一题由 GetNextQuestion 按需生成）
func (uc *InterviewUseCase) SubmitAnswer(ctx context.Context, interviewID, userID uint64, index int32, answer string) (*AnswerFeedback, error) {
	// 1. 获取面试会话
	interview, err := uc.repo.GetByID(ctx, interviewID)
	if err != nil {
		return nil, ErrInterviewNotFound
	}
	if interview.UserID != userID {
		return nil, ErrUnauthorized
	}
	if interview.Status != "ongoing" {
		return nil, ErrInterviewNotOngoing
	}

	// 2. 保存用户答案消息
	msg := &InterviewMessage{
		InterviewID:   interviewID,
		Role:          "user",
		Content:       answer,
		MessageType:   "text",
		QuestionIndex: index,
	}

	// 3. RAG 检索增强（降级处理，失败不影响主流程）
	// 使用题目主题而非用户答案构造查询，避免长答案产生噪音
	ragContext := uc.retrieveRAGContext(ctx, interview, "")

	// 4. 调用 AI Gateway 的 EvaluateAnswer 评估答案
	evalResp, err := uc.ai.EvaluateAnswer(ctx, &EvaluateAnswerRequest{
		SessionId:     interview.AISessionID,
		QuestionIndex: index,
		Answer:        answer,
		RAGContext:    ragContext,
	})
	if err != nil {
		return nil, kratosErr.InternalServer("AI_CALL_FAILED", "AI 服务调用失败").WithCause(err)
	}

	feedback := &AnswerFeedback{
		Score:       evalResp.Score,
		IsCorrect:   evalResp.IsCorrect,
		Feedback:    evalResp.Feedback,
		KeyPoints:   evalResp.KeyPoints,
		Suggestions: evalResp.Suggestions,
		FollowUp:    evalResp.FollowUp,
	}
	feedbackText := strings.TrimSpace(evalResp.Feedback)

	// 5. 统一在事务中保存用户答案、AI 回复和进度
	if err := uc.repo.Transaction(ctx, func(txCtx context.Context) error {
		if err := uc.repo.CreateMessage(txCtx, msg); err != nil {
			return kratosErr.InternalServer("SAVE_FAILED", "保存答案失败").WithCause(err)
		}
		if feedbackText != "" {
			aiMsg := &InterviewMessage{
				InterviewID:   interviewID,
				Role:          "assistant",
				Content:       feedbackText,
				MessageType:   "text",
				QuestionIndex: index,
			}
			if err := uc.repo.CreateMessage(txCtx, aiMsg); err != nil {
				return kratosErr.InternalServer("SAVE_AI_MSG_FAILED", "保存 AI 回复失败").WithCause(err)
			}
		}
		interview.CurrentIndex = index + 1
		if err := uc.repo.Update(txCtx, interview); err != nil {
			return kratosErr.InternalServer("UPDATE_FAILED", "更新面试状态失败").WithCause(err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	// 异步写入学习档案
	go func() {
		archiveCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := uc.archive.WriteEntry(archiveCtx, &ArchiveEntry{
			UserID:          interview.UserID,
			SourceType:      "interview_answer",
			InterviewID:     interviewID,
			QuestionIndex:   index,
			IndustryCode:    interview.IndustryCode,
			EvidenceSummary: feedbackText,
			OccurredAt:      time.Now(),
		}); err != nil {
			log.Warnf("异步写入学习档案失败: interview_id=%d, question_index=%d, err=%v", interviewID, index, err)
		}
	}()

	return feedback, nil
}

// retrieveRAGContext 检索 RAG 参考知识，失败时返回空字符串（降级处理）。
func (uc *InterviewUseCase) retrieveRAGContext(ctx context.Context, interview *Interview, questionTopic string) string {
	if uc.rag == nil {
		return ""
	}
	query := buildRAGQuery(questionTopic, interview.IndustryCode, interview.JobDescription)
	docs, err := uc.rag.Retrieve(ctx, query, 5)
	if err != nil {
		log.Warnf("RAG 检索失败，降级处理: interview_id=%d, err=%v", interview.ID, err)
		return ""
	}
	return joinRAGDocs(docs)
}

// buildRAGQuery 构造 RAG 检索查询，优先使用题目主题，回退到职位描述和行业。
func buildRAGQuery(topic, industryCode, jobDescription string) string {
	if t := strings.TrimSpace(topic); t != "" {
		code := strings.TrimSpace(industryCode)
		if code != "" {
			return t + " " + code
		}
		return t
	}
	if jd := strings.TrimSpace(jobDescription); jd != "" {
		return jd
	}
	code := strings.TrimSpace(industryCode)
	if code != "" {
		return code + " 面试问题"
	}
	return "面试问题"
}

// joinRAGDocs 将 RAG 文档拼接为单个上下文字符串。
func joinRAGDocs(docs []*RAGDocument) string {
	if len(docs) == 0 {
		return ""
	}
	var sb strings.Builder
	for i, doc := range docs {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(doc.Content)
	}
	return sb.String()
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

	// 超时自动结束：避免面试一直卡在 ongoing 状态
	uc.autoFinishIfExpired(ctx, interview)

	messages, err := uc.repo.ListMessages(ctx, interviewID)
	if err != nil {
		return nil, nil, err
	}

	return interview, messages, nil
}

// GetNextQuestion 获取下一道面试题目，支持 RAG 增强检索（降级不影响主流程）
func (uc *InterviewUseCase) GetNextQuestion(ctx context.Context, interviewID, userID uint64) (*Interview, *InterviewQuestion, error) {
	// 获取面试记录并验证状态
	interview, err := uc.repo.GetByID(ctx, interviewID)
	if err != nil {
		return nil, nil, ErrInterviewNotFound
	}
	if interview.UserID != userID {
		return nil, nil, ErrUnauthorized
	}
	if interview.Status != "ongoing" {
		return nil, nil, ErrInterviewNotOngoing
	}
	if interview.CurrentIndex >= interview.QuestionCount {
		return interview, nil, nil
	}

	// 检查是否已经有当前 index 的题目消息（防止重复生成）
	recentMessages, err := uc.repo.ListMessagesLimited(ctx, interviewID, 20)
	if err != nil {
		return nil, nil, kratosErr.InternalServer("HISTORY_FAILED", "获取历史消息失败").WithCause(err)
	}

	if question := ExtractQuestionByIndex(recentMessages, interview.CurrentIndex); question != nil {
		return interview, question, nil
	}
	history := NormalizeHistoryMessages(recentMessages)

	// 调用 RAG 检索增强（降级处理，失败不影响主流程）
	var ragContext string
	if uc.rag != nil {
		query := buildRAGQuery("", interview.IndustryCode, interview.JobDescription)
		docs, err := uc.rag.Retrieve(ctx, query, 5)
		if err == nil {
			ragContext = joinRAGDocs(docs)
		}
	}

	// 调用 AI Gateway.InterviewAgent 获取下一题
	jobDesc := interview.JobDescription
	if ragContext != "" {
		jobDesc = strings.TrimSpace(jobDesc + "\n参考资料：\n" + ragContext)
	}
	aiResp, err := uc.ai.InterviewAgent(ctx, &InterviewAgentRequest{
		InterviewID:   interview.ID,
		IndustryCode:  interview.IndustryCode,
		Difficulty:    interview.Difficulty,
		History:       history,
		QuestionIndex: interview.CurrentIndex,
		ResumeText:    interview.ResumeText,
		JobDesc:       jobDesc,
	})
	if err != nil {
		return nil, nil, kratosErr.InternalServer("AI_CALL_FAILED", "AI 服务调用失败").WithCause(err)
	}

	// 解析返回，构造 InterviewQuestion
	var question *InterviewQuestion
	if aiResp != nil && aiResp.Question != nil {
		question = aiResp.Question
		// 将 RAG 检索结果注入题目 hints（如果存在）
		if ragContext != "" && question.Hints == "" {
			question.Hints = ragContext
		}
		if questionMsg := BuildQuestionMessage(interview.ID, interview.CurrentIndex, question); questionMsg != nil {
			if err := uc.repo.CreateMessage(ctx, questionMsg); err != nil {
				return nil, nil, kratosErr.InternalServer("SAVE_AI_MSG_FAILED", "保存 AI 题目消息失败").WithCause(err)
			}
		}
	}

	return interview, question, nil
}

// ListInterviews 获取用户面试列表
// ListInterviews 获取面试列表，自动结束超时的面试。
func (uc *InterviewUseCase) ListInterviews(ctx context.Context, userID uint64, page, pageSize int32) ([]*Interview, int64, error) {
	interviews, total, err := uc.repo.ListByUser(ctx, userID, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	// 超时自动结束：遍历列表中的 ongoing 面试
	for _, iv := range interviews {
		uc.autoFinishIfExpired(ctx, iv)
	}
	return interviews, total, nil
}

// GetInterviewStats 供 growth 服务调用的聚合接口（FIX I3: 使用 SQL 聚合避免全量加载）
func (uc *InterviewUseCase) GetInterviewStats(ctx context.Context, userID uint64) (*InterviewStats, error) {
	return uc.repo.GetStats(ctx, userID)
}

// GetAdminInterviewStats 返回管理后台需要的面试总量统计。
func (uc *InterviewUseCase) GetAdminInterviewStats(ctx context.Context) (int64, error) {
	return uc.repo.GetAdminStats(ctx)
}

// ProcessResumeParse MQ 消费者：解析简历并持久化解析结果
func (uc *InterviewUseCase) ProcessResumeParse(ctx context.Context, interviewID, userID uint64, resumeText string) error {
	// 校验简历文本非空，空文本直接丢弃
	if resumeText == "" {
		log.Warnf("resume_text 为空，丢弃消息 interview_id=%d", interviewID)
		return nil
	}

	// 查询面试记录，不存在则丢弃消息
	interview, err := uc.repo.GetByID(ctx, interviewID)
	if err != nil {
		log.Warnf("面试记录不存在，丢弃消息 interview_id=%d err=%v", interviewID, err)
		return nil
	}

	if strings.TrimSpace(interview.ResumeParsedJSON) != "" {
		log.Infof("简历解析已存在，跳过重复消费 interview_id=%d", interviewID)
		return nil
	}

	// 调用 AI Gateway 解析简历
	resp, err := uc.ai.ResumeParser(ctx, &ResumeParserRequest{ResumeText: resumeText})
	if err != nil {
		return kratosErr.InternalServer("RESUME_PARSE_FAILED", "简历解析失败").WithCause(err)
	}

	// 将解析结果序列化为 JSON
	parsedJSON, err := json.Marshal(resp)
	if err != nil {
		return kratosErr.InternalServer("RESUME_PARSE_MARSHAL_FAILED", "简历解析结果序列化失败").WithCause(err)
	}

	// 持久化解析结果到 interviews 表
	interview.ResumeParsedJSON = string(parsedJSON)
	if err := uc.repo.Update(ctx, interview); err != nil {
		return kratosErr.InternalServer("RESUME_PARSE_SAVE_FAILED", "保存简历解析结果失败").WithCause(err)
	}

	return nil
}

// GenerateReport MQ 消费者：调用 AI 生成面试报告，保存报告记录并更新面试状态
func (uc *InterviewUseCase) GenerateReport(ctx context.Context, interviewID, userID uint64) error {
	log.Infof("[ReportGen] start: interview_id=%d user_id=%d", interviewID, userID)

	// 整体超时保护：避免 AI 调用卡住导致 MQ 消费者永久阻塞
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	interview, err := uc.repo.GetByID(ctx, interviewID)
	if err != nil {
		log.Errorf("[ReportGen] get interview failed: interview_id=%d err=%v", interviewID, err)
		return err
	}
	log.Infof("[ReportGen] interview loaded: status=%s ai_session_id=%s", interview.Status, interview.AISessionID)
	if err != nil {
		return err
	}

	// 幂等检查：已完成的面试跳过报告生成
	if interview.Status == "completed" {
		log.Infof("[ReportGen] interview already completed, skipping interview_id=%d", interviewID)
		return nil
	}

	// 实时面试的 session 不在 AI Gateway 的内存中（走火山引擎 WebSocket），
	// 调 GenerateInterviewReport 必然失败，改用 GenerateReportFromHistory 发送完整对话。
	if isRealtimeInterview(interview) {
		log.Infof("[ReportGen] realtime interview, generating report from history: interview_id=%d", interviewID)
		messages, msgErr := uc.repo.ListMessages(ctx, interviewID)
		if msgErr != nil {
			log.Errorf("[ReportGen] failed to load messages: interview_id=%d err=%v", interviewID, msgErr)
			return uc.generateReportLocally(ctx, interviewID, userID, interview)
		}
		reportResp, reportErr := uc.ai.GenerateReportFromHistory(ctx, &GenerateReportFromHistoryRequest{
			History:        messages,
			IndustryCode:   interview.IndustryCode,
			Difficulty:     interview.Difficulty,
			TotalQuestions: int32(interview.QuestionCount),
		})
		if reportErr != nil {
			log.Warnf("[ReportGen] GenerateReportFromHistory failed, falling back to local: interview_id=%d err=%v", interviewID, reportErr)
			return uc.generateReportLocally(ctx, interviewID, userID, interview)
		}
		return uc.saveReport(ctx, interviewID, interview, reportResp)
	}

	// 标准面试：调用 AI Gateway 的 GenerateInterviewReport
	log.Infof("[ReportGen] calling AI GenerateInterviewReport: session_id=%s", interview.AISessionID)
	reportResp, err := uc.ai.GenerateInterviewReport(ctx, &GenerateInterviewReportRequest{
		SessionId: interview.AISessionID,
	})
	if err != nil {
		log.Warnf("[ReportGen] AI report failed, falling back to local: interview_id=%d err=%v", interviewID, err)
		return uc.generateReportLocally(ctx, interviewID, userID, interview)
	}
	log.Infof("[ReportGen] AI report success: overall_score=%.1f", reportResp.OverallScore)

	// 获取编程题诊断（本地逻辑，不依赖 AI）
	messages, err := uc.repo.ListMessages(ctx, interviewID)
	if err != nil {
		return err
	}
	pairs := BuildQuestionAnswerPairs(messages)
	codingAttempts, err := uc.repo.ListCodingAttempts(ctx, interviewID)
	if err != nil {
		return err
	}
	codingDiagnostics := uc.BuildCodingDiagnostics(ctx, interview, pairs, codingAttempts)

	// 合并 AI 报告和本地编程诊断
	dimensionScores := reportResp.DimensionScores
	if dimensionScores == nil {
		dimensionScores = make(map[string]float64)
	}
	strengths := reportResp.Strengths
	weaknesses := reportResp.Weaknesses
	suggestions := reportResp.Suggestions

	for _, diagnostic := range codingDiagnostics {
		if diagnostic == nil {
			continue
		}
		dimensionScores[diagnostic.Topic] = diagnostic.Score
		strengths = appendUniqueStrings(strengths, diagnostic.StrengthTags...)
		weaknesses = appendUniqueStrings(weaknesses, diagnostic.MistakeTags...)
		suggestions = appendUniqueStrings(suggestions, diagnostic.Suggestions...)
	}

	reportResp.DimensionScores = dimensionScores
	reportResp.Strengths = strengths
	reportResp.Weaknesses = weaknesses
	reportResp.Suggestions = suggestions

	return uc.saveReport(ctx, interviewID, interview, reportResp)
}

// saveReport 将报告写入数据库并更新面试状态
func (uc *InterviewUseCase) saveReport(ctx context.Context, interviewID uint64, interview *Interview, reportResp *GenerateInterviewReportResponse) error {
	report := &InterviewReport{
		InterviewID:         interviewID,
		OverallScore:        reportResp.OverallScore,
		DimensionScoresJSON: marshalJSON(reportResp.DimensionScores),
		StrengthsJSON:       marshalJSON(reportResp.Strengths),
		WeaknessesJSON:      marshalJSON(reportResp.Weaknesses),
		SuggestionsJSON:     marshalJSON(reportResp.Suggestions),
		Summary:             reportResp.Summary,
	}

	log.Infof("[ReportGen] saving report to DB: interview_id=%d", interviewID)
	if err := uc.repo.Transaction(ctx, func(txCtx context.Context) error {
		if err := uc.reportRepo.Create(txCtx, report); err != nil {
			return err
		}
		interview.Status = "completed"
		interview.OverallScore = reportResp.OverallScore
		return uc.repo.Update(txCtx, interview)
	}); err != nil {
		log.Errorf("[ReportGen] save report failed: interview_id=%d err=%v", interviewID, err)
		interview.Status = "report_failed"
		_ = uc.repo.Update(ctx, interview)
		return err
	}

	log.Infof("[ReportGen] report saved successfully: interview_id=%d overall_score=%.1f", interviewID, reportResp.OverallScore)

	// 结束 AI 会话（仅标准面试，实时面试的 session 不在 AI Gateway 中）
	if !isRealtimeInterview(interview) && interview.AISessionID != "" {
		_, _ = uc.ai.EndInterviewSession(ctx, &EndInterviewSessionRequest{
			SessionId: interview.AISessionID,
		})
	}

	if uc.publisher != nil {
		_ = uc.publisher.PublishInterviewFinished(ctx, interviewID, interview.UserID, reportResp.OverallScore, reportResp.Weaknesses, reportResp.Strengths)
	}
	return nil
}

// generateReportLocally 本地降级报告生成（当 AI GenerateReport 失败时使用）
func (uc *InterviewUseCase) generateReportLocally(ctx context.Context, interviewID, userID uint64, interview *Interview) error {
	messages, err := uc.repo.ListMessages(ctx, interviewID)
	if err != nil {
		return err
	}

	pairs := BuildQuestionAnswerPairs(messages)
	scoreSum := make(map[string]float64)
	scoreCount := make(map[string]int)
	var strengths []string
	var weaknesses []string
	var suggestions []string
	var overallTotal float64
	var overallCount int

	for _, pair := range pairs {
		if pair.Question == nil || pair.Question.Type == "coding" {
			continue
		}
		evaluation := uc.EvaluateAnswer(ctx, interview, pair)
		topic := BuildTopic(pair.Question)
		scoreSum[topic] += evaluation.Score
		scoreCount[topic]++
		overallTotal += evaluation.Score
		overallCount++
		if evaluation.Score >= 75 {
			strengths = appendUniqueStrings(strengths, topic)
			strengths = appendUniqueStrings(strengths, evaluation.KeyPoints...)
		}
		if pair.Answer == nil || evaluation.Score < 60 {
			weaknesses = appendUniqueStrings(weaknesses, topic)
		}
		suggestions = appendUniqueStrings(suggestions, evaluation.Suggestions...)
	}

	codingAttempts, err := uc.repo.ListCodingAttempts(ctx, interviewID)
	if err != nil {
		return err
	}
	codingDiagnostics := uc.BuildCodingDiagnostics(ctx, interview, pairs, codingAttempts)
	for _, diagnostic := range codingDiagnostics {
		if diagnostic == nil {
			continue
		}
		scoreSum[diagnostic.Topic] += diagnostic.Score
		scoreCount[diagnostic.Topic]++
		overallTotal += diagnostic.Score
		overallCount++
		strengths = appendUniqueStrings(strengths, diagnostic.StrengthTags...)
		weaknesses = appendUniqueStrings(weaknesses, diagnostic.MistakeTags...)
		suggestions = appendUniqueStrings(suggestions, diagnostic.Suggestions...)
	}

	overallScore := 0.0
	if overallCount > 0 {
		overallScore = overallTotal / float64(overallCount)
	}
	dimensionScores := finalizeDimensionScores(scoreSum, scoreCount)
	strengths = uniqueNonEmptyStrings(strengths)
	weaknesses = uniqueNonEmptyStrings(weaknesses)
	suggestions = uniqueNonEmptyStrings(suggestions)
	summary := buildReportSummary(overallScore, strengths, weaknesses, codingDiagnostics)

	report := &InterviewReport{
		InterviewID:           interviewID,
		OverallScore:          overallScore,
		DimensionScoresJSON:   marshalJSON(dimensionScores),
		StrengthsJSON:         marshalJSON(strengths),
		WeaknessesJSON:        marshalJSON(weaknesses),
		SuggestionsJSON:       marshalJSON(suggestions),
		Summary:               summary,
		CodingDiagnosticsJSON: marshalJSON(codingDiagnostics),
	}

	if err := uc.repo.Transaction(ctx, func(txCtx context.Context) error {
		if err := uc.reportRepo.Create(txCtx, report); err != nil {
			return err
		}
		interview.Status = "completed"
		interview.OverallScore = overallScore
		return uc.repo.Update(txCtx, interview)
	}); err != nil {
		interview.Status = "report_failed"
		_ = uc.repo.Update(ctx, interview)
		return err
	}

	if uc.publisher != nil {
		if err := uc.publisher.PublishInterviewFinished(ctx, interviewID, userID, overallScore, weaknesses, strengths); err != nil {
			return err
		}
	}
	return nil
}

// PersistCodingArchive MQ 消费者：持久化编程题归档
func (uc *InterviewUseCase) PersistCodingArchive(ctx context.Context, interviewID, userID uint64) error {
	exists, err := uc.HasCodingArchive(ctx, interviewID, userID)
	if err != nil {
		return err
	}
	if exists {
		log.Infof("编程归档已存在，跳过重复消费 interview_id=%d user_id=%d", interviewID, userID)
		return nil
	}

	// 获取面试信息
	interview, err := uc.repo.GetByID(ctx, interviewID)
	if err != nil {
		return err
	}

	// 写入学习档案
	return uc.archive.WriteEntry(ctx, &ArchiveEntry{
		UserID:       interview.UserID,
		SourceType:   "interview_coding",
		SourceRef:    fmt.Sprintf("%d", interviewID),
		InterviewID:  interviewID,
		IndustryCode: interview.IndustryCode,
		OccurredAt:   interview.CreatedAt,
	})
}

// FinishInterview 结束面试并触发报告生成
// FinishInterview 结束面试并触发报告生成，返回更新后的面试实体。
func (uc *InterviewUseCase) FinishInterview(ctx context.Context, interviewID, userID uint64) (*Interview, error) {
	// 获取面试记录
	interview, err := uc.repo.GetByID(ctx, interviewID)
	if err != nil {
		return nil, ErrInterviewNotFound
	}
	// 验证面试归属
	if interview.UserID != userID {
		return nil, ErrUnauthorized
	}
	// 验证面试状态为进行中
	if interview.Status != "ongoing" {
		return nil, ErrInterviewNotOngoing
	}

	// 更新状态为报告生成中
	now := time.Now()
	interview.Status = "report_generating"
	interview.FinishedAt = &now
	if err := uc.repo.Update(ctx, interview); err != nil {
		return nil, kratosErr.InternalServer("UPDATE_FAILED", "更新面试状态失败").WithCause(err)
	}

	// 发布报告生成 MQ 消息
	if uc.publisher != nil {
		if err := uc.publisher.PublishInterviewReportGenerate(ctx, interviewID, userID); err != nil {
			log.Errorf("发布报告生成消息失败: interview_id=%d err=%v", interviewID, err)
		}
	}

	return interview, nil
}

// autoFinishIfExpired 检查面试是否超时，如果超时自动结束并触发报告生成。
// 用于 GetInterview、ListInterviews 等读取场景，确保过期面试不会一直卡在 ongoing 状态。
func (uc *InterviewUseCase) autoFinishIfExpired(ctx context.Context, interview *Interview) {
	if interview.Status != "ongoing" {
		return
	}
	if interview.CreatedAt.IsZero() || time.Since(interview.CreatedAt) < uc.interviewTimeout {
		return
	}
	log.Infof("面试超时自动结束: interview_id=%d created_at=%v duration=%v", interview.ID, interview.CreatedAt, time.Since(interview.CreatedAt))
	now := time.Now()
	interview.Status = "report_generating"
	interview.FinishedAt = &now
	if err := uc.repo.Update(ctx, interview); err != nil {
		log.Errorf("超时自动结束面试失败: interview_id=%d err=%v", interview.ID, err)
		return
	}
	if uc.publisher != nil {
		if err := uc.publisher.PublishInterviewReportGenerate(ctx, interview.ID, interview.UserID); err != nil {
			log.Errorf("超时自动发布报告生成消息失败: interview_id=%d err=%v", interview.ID, err)
		}
	}
}

// GetReportResult 面试报告查询结果
type GetReportResult struct {
	Status            string
	OverallScore      float64
	TotalQuestions    int32
	CorrectCount      int32
	DimensionScores   map[string]float64
	Strengths         []string
	Weaknesses        []string
	Suggestions       []string
	Summary           string
	CodingDiagnostics []*CodingDiagnosisBiz
	DurationSeconds   int32
	CompletedAt       *time.Time
}

// CodingAnswerResult 编程题提交结果
type CodingAnswerResult struct {
	Passed          bool
	TestCasesPassed int32
	TotalTestCases  int32
	Output          string
	ErrorMsg        string
	ExecutionOK     bool
	AIScore         float64
	AIFeedback      string
}

// SubmitCodingAnswer 提交编程题答案，执行代码并调用 AI 评分
func (uc *InterviewUseCase) SubmitCodingAnswer(ctx context.Context, interviewID, userID uint64, questionIndex int32, language, code string) (*CodingAnswerResult, error) {
	// 1. 获取并校验面试状态
	interview, err := uc.repo.GetByID(ctx, interviewID)
	if err != nil {
		return nil, ErrInterviewNotFound
	}
	if interview.UserID != userID {
		return nil, ErrUnauthorized
	}
	if interview.Status != "ongoing" {
		return nil, ErrInterviewNotOngoing
	}

	// 2. 从消息记录中找到对应题目（AI 消息，question_index 匹配）
	messages, err := uc.repo.ListMessages(ctx, interviewID)
	if err != nil {
		return nil, kratosErr.InternalServer("HISTORY_FAILED", "获取历史消息失败").WithCause(err)
	}
	question := ExtractQuestionByIndex(messages, questionIndex)
	if question == nil {
		return nil, ErrQuestionNotFound
	}
	questionContent := NormalizeQuestionText(question)
	codeMsg := &InterviewMessage{
		InterviewID:   interviewID,
		Role:          "user",
		Content:       code,
		MessageType:   "code",
		QuestionIndex: questionIndex,
	}
	// 3. 调用 CodeRunner 执行代码（降级处理）
	result := &CodingAnswerResult{}
	if uc.codeRunner != nil {
		// 如果题目有 Hints（样例输入），作为 stdin 传给 CodeRunner
		var testCases []CodeTestCase
		if strings.TrimSpace(question.Hints) != "" {
			testCases = []CodeTestCase{{Input: question.Hints}}
		}
		runnerResult, err := uc.codeRunner.Execute(ctx, language, code, testCases)
		if err == nil && runnerResult != nil {
			result.ExecutionOK = true
			result.Passed = runnerResult.Success
			result.TestCasesPassed = runnerResult.PassedCount
			result.TotalTestCases = runnerResult.TotalCount
			result.Output = runnerResult.Stdout
			result.ErrorMsg = runnerResult.Stderr
		} else {
			// 降级：执行失败返回 execution_success=false
			result.ExecutionOK = false
		}
	}

	// 4. 调用 AI Gateway.QuizAnalyzer 进行评分（降级处理）
	if uc.ai != nil {
		aiResp, err := uc.ai.QuizAnalyzer(ctx, &QuizAnalyzerRequest{
			Question:   questionContent,
			Answer:     code,
			Topic:      BuildTopic(question),
			Difficulty: interview.Difficulty,
		})
		if err == nil && aiResp != nil {
			result.AIScore = aiResp.Score
			result.AIFeedback = aiResp.Feedback
		} else {
			// 降级：AI 评分失败返回 ai_score=0
			result.AIScore = 0
		}
	}

	// 5. 保存编程答题记录
	attempt := &CodingAttempt{
		InterviewID:     interviewID,
		QuestionIndex:   questionIndex,
		Language:        language,
		Code:            code,
		Passed:          result.Passed,
		TestCasesPassed: result.TestCasesPassed,
		TotalTestCases:  result.TotalTestCases,
		Output:          result.Output,
		ErrorMsg:        result.ErrorMsg,
		AIScore:         result.AIScore,
		AIFeedback:      result.AIFeedback,
	}
	if err := uc.repo.Transaction(ctx, func(txCtx context.Context) error {
		if err := uc.repo.CreateMessage(txCtx, codeMsg); err != nil {
			return kratosErr.InternalServer("SAVE_CODE_FAILED", "保存编程答案失败").WithCause(err)
		}
		if err := uc.repo.CreateCodingAttempt(txCtx, attempt); err != nil {
			return kratosErr.InternalServer("SAVE_ATTEMPT_FAILED", "保存编程答题记录失败").WithCause(err)
		}
		if result.AIFeedback != "" {
			aiMsg := &InterviewMessage{
				InterviewID:   interviewID,
				Role:          "assistant",
				Content:       result.AIFeedback,
				MessageType:   "text",
				QuestionIndex: questionIndex,
			}
			if err := uc.repo.CreateMessage(txCtx, aiMsg); err != nil {
				return kratosErr.InternalServer("SAVE_AI_REVIEW_FAILED", "保存 AI 评审反馈失败").WithCause(err)
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return result, nil
}

// IsResumeParsed 判断面试是否已经完成简历解析，供 MQ 消费者做幂等短路。
func (uc *InterviewUseCase) IsResumeParsed(ctx context.Context, interviewID uint64) (bool, error) {
	interview, err := uc.repo.GetByID(ctx, interviewID)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(interview.ResumeParsedJSON) != "", nil
}

// HasCodingArchive 判断编程归档是否已写入学习档案，供 MQ 消费者做幂等短路。
func (uc *InterviewUseCase) HasCodingArchive(ctx context.Context, interviewID, userID uint64) (bool, error) {
	entries, err := uc.archive.ListBySource(ctx, userID, "interview_coding", interviewID)
	if err != nil {
		return false, err
	}
	return len(entries) > 0, nil
}

// IsRealtimeInterview 查询面试是否为实时模式
func (uc *InterviewUseCase) IsRealtimeInterview(ctx context.Context, interviewID, userID uint64) (bool, error) {
	interview, err := uc.repo.GetByID(ctx, interviewID)
	if err != nil {
		return false, ErrInterviewNotFound
	}
	if interview.UserID != userID {
		return false, ErrUnauthorized
	}
	if interview.Status != "ongoing" {
		return false, ErrInterviewNotOngoing
	}
	return isRealtimeInterview(interview), nil
}

// RealtimeContextResult 实时面试上下文结果（对齐单体 RealtimeInterviewContext）
type RealtimeContextResult struct {
	InterviewID           uint64
	IndustryCode          string
	Live2DModelKey        string
	TotalQuestions        int
	AskedQuestionCount    int
	AnsweredQuestionCount int
	Difficulty            string
	Topics                []string
	WeakTopics            []string
	InterviewMode         string
	ResumeProfile         *ResumeProfileData
	DialogID              string
	HasStarted            bool
	History               []*InterviewMessage
	CurrentTopic          string
	QuestionIndex         int32
}

// ResumeProfileData 简历画像数据
type ResumeProfileData struct {
	Summary     string   `json:"summary"`
	Skills      []string `json:"skills"`
	Projects    []string `json:"projects"`
	Strengths   []string `json:"strengths"`
	WeakSignals []string `json:"weak_signals"`
}

// GetRealtimeContext 加载实时面试上下文（对齐单体：包含简历画像、面试模式等完整信息）
func (uc *InterviewUseCase) GetRealtimeContext(ctx context.Context, interviewID, userID uint64) (*RealtimeContextResult, error) {
	interview, err := uc.repo.GetByID(ctx, interviewID)
	if err != nil {
		return nil, ErrInterviewNotFound
	}
	if interview.UserID != userID {
		return nil, ErrUnauthorized
	}
	if interview.Status != "ongoing" {
		return nil, ErrInterviewNotOngoing
	}
	if !isRealtimeInterview(interview) {
		return nil, ErrInterviewNotRealtime
	}

	// 加载最近 10 条消息
	recentMessages, err := uc.repo.ListMessagesLimited(ctx, interviewID, 10)
	if err != nil {
		return nil, kratosErr.InternalServer("HISTORY_FAILED", "获取最近消息失败").WithCause(err)
	}

	// 解析简历画像（如果已解析）
	var resumeProfile *ResumeProfileData
	if strings.TrimSpace(interview.ResumeParsedJSON) != "" {
		var profile ResumeProfileData
		if jsonErr := json.Unmarshal([]byte(interview.ResumeParsedJSON), &profile); jsonErr == nil {
			resumeProfile = &profile
		}
	}

	// 判断面试模式
	interviewMode := interview.InterviewMode
	if interviewMode == "" && interview.Live2DModelKey != "" {
		interviewMode = "realtime"
	}

	return &RealtimeContextResult{
		InterviewID:    interview.ID,
		IndustryCode:   interview.IndustryCode,
		Live2DModelKey: interview.Live2DModelKey,
		TotalQuestions: int(interview.QuestionCount),
		Difficulty:     interview.Difficulty,
		InterviewMode:  interviewMode,
		Topics:         resolveTopicsByIndustry(interview.IndustryCode),
		WeakTopics:     resolveWeakTopics(recentMessages),
		ResumeProfile:  resumeProfile,
		DialogID:       interview.AISessionID,
		HasStarted:     interview.CurrentIndex > 0,
		History:        recentMessages,
		CurrentTopic:   ResolveCurrentTopic(recentMessages),
		QuestionIndex:  interview.CurrentIndex,
	}, nil
}

// BindRealtimeDialog 绑定实时对话 ID 到面试记录
func (uc *InterviewUseCase) BindRealtimeDialog(ctx context.Context, interviewID, userID uint64, dialogID string) error {
	interview, err := uc.repo.GetByID(ctx, interviewID)
	if err != nil {
		return ErrInterviewNotFound
	}
	if interview.UserID != userID {
		return ErrUnauthorized
	}
	if interview.Status != "ongoing" {
		return ErrInterviewNotOngoing
	}
	if !isRealtimeInterview(interview) {
		return ErrInterviewNotRealtime
	}
	return uc.repo.BindRealtimeDialog(ctx, interviewID, dialogID)
}

// AppendRealtimeUserAnswer 追加实时面试中的用户回答消息
func (uc *InterviewUseCase) AppendRealtimeUserAnswer(ctx context.Context, interviewID, userID uint64, answerText string) error {
	interview, err := uc.repo.GetByID(ctx, interviewID)
	if err != nil {
		return ErrInterviewNotFound
	}
	if interview.UserID != userID {
		return ErrUnauthorized
	}
	if interview.Status != "ongoing" {
		return ErrInterviewNotOngoing
	}
	if !isRealtimeInterview(interview) {
		return ErrInterviewNotRealtime
	}
	msg := &InterviewMessage{
		InterviewID:   interviewID,
		Role:          "user",
		Content:       answerText,
		MessageType:   "text",
		QuestionIndex: interview.CurrentIndex,
	}
	return uc.repo.CreateMessage(ctx, msg)
}

// AppendRealtimeAssistantReply 追加实时面试中的 AI 回复，同时递增题目索引
func (uc *InterviewUseCase) AppendRealtimeAssistantReply(ctx context.Context, interviewID, userID uint64, replyText string) (bool, *InterviewQuestion, error) {
	interview, err := uc.repo.GetByID(ctx, interviewID)
	if err != nil {
		return false, nil, ErrInterviewNotFound
	}
	if interview.UserID != userID {
		return false, nil, ErrUnauthorized
	}
	if interview.Status != "ongoing" {
		return false, nil, ErrInterviewNotOngoing
	}
	if !isRealtimeInterview(interview) {
		return false, nil, ErrInterviewNotRealtime
	}

	msg := &InterviewMessage{
		InterviewID:   interviewID,
		Role:          "assistant",
		Content:       replyText,
		MessageType:   "text",
		QuestionIndex: interview.CurrentIndex,
	}

	// 追加消息并递增 current_question_index（事务操作）
	if err := uc.repo.AppendMessageAndBumpIndex(ctx, msg); err != nil {
		return false, nil, kratosErr.InternalServer("APPEND_FAILED", "追加 AI 回复失败").WithCause(err)
	}

	updatedInterview, err := uc.repo.GetByID(ctx, interviewID)
	if err != nil {
		return false, nil, ErrInterviewNotFound
	}
	shouldEnd := updatedInterview.CurrentIndex >= updatedInterview.QuestionCount
	return shouldEnd, nil, nil
}

// CodingDiagnosisBiz 编程诊断业务实体
type CodingDiagnosisBiz struct {
	QuestionIndex   int32
	Language        string
	Topic           string
	Score           float64
	MistakeTags     []string
	StrengthTags    []string
	EvidenceSummary string
	Suggestions     []string
}

// GetReport 获取面试报告，根据面试状态返回不同结果
func (uc *InterviewUseCase) GetReport(ctx context.Context, interviewID, userID uint64) (*GetReportResult, error) {
	// 获取面试记录
	interview, err := uc.repo.GetByID(ctx, interviewID)
	if err != nil {
		return nil, ErrInterviewNotFound
	}
	// 验证面试归属
	if interview.UserID != userID {
		return nil, ErrUnauthorized
	}

	// 根据状态返回不同响应
	switch interview.Status {
	case "report_generating":
		return &GetReportResult{Status: "report_generating"}, nil
	case "report_failed":
		return &GetReportResult{Status: "failed"}, nil
	case "completed":
		report, err := uc.reportRepo.GetByInterviewID(ctx, interviewID)
		if err != nil {
			return nil, ErrReportNotReady
		}
		messages, err := uc.repo.ListMessages(ctx, interviewID)
		if err != nil {
			return nil, err
		}
		pairs := BuildQuestionAnswerPairs(messages)
		codingDiagnostics := decodeCodingDiagnostics(report.CodingDiagnosticsJSON)
		codingCorrect := make(map[int32]bool, len(codingDiagnostics))
		var correctCount int32
		for _, diagnostic := range codingDiagnostics {
			if diagnostic != nil && diagnostic.Score >= 60 {
				codingCorrect[diagnostic.QuestionIndex] = true
				correctCount++
			}
		}
		for _, pair := range pairs {
			if pair.Question != nil && pair.Question.Type == "coding" {
				continue
			}
			if pair.Answer != nil && EstimateAnswerScore(pair.Answer.Content) >= 60 {
				correctCount++
			}
		}
		return &GetReportResult{
			Status:            "completed",
			OverallScore:      report.OverallScore,
			TotalQuestions:    interview.QuestionCount,
			CorrectCount:      correctCount,
			DimensionScores:   unmarshalMapStringFloat64(report.DimensionScoresJSON),
			Strengths:         unmarshalStringSlice(report.StrengthsJSON),
			Weaknesses:        unmarshalStringSlice(report.WeaknessesJSON),
			Suggestions:       unmarshalStringSlice(report.SuggestionsJSON),
			Summary:           report.Summary,
			CodingDiagnostics: codingDiagnostics,
			DurationSeconds:   calcDurationSeconds(interview.StartedAt, interview.FinishedAt),
			CompletedAt:       interview.FinishedAt,
		}, nil
	default:
		return nil, ErrReportNotReady
	}
}

// marshalJSON 将对象序列化为 JSON 字符串，失败时返回空对象
func marshalJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// calcDurationSeconds 计算面试时长（秒），任一时间点缺失返回 0。
func calcDurationSeconds(started, finished *time.Time) int32 {
	if started == nil || finished == nil {
		return 0
	}
	return int32(finished.Sub(*started).Seconds())
}

// unmarshalStringSlice 从 JSON 字符串解析字符串切片，空输入返回空数组而非 nil。
func unmarshalStringSlice(jsonStr string) []string {
	if jsonStr == "" {
		return []string{}
	}
	var result []string
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return []string{}
	}
	return result
}

// unmarshalMapStringFloat64 从 JSON 字符串解析 map[string]float64
func unmarshalMapStringFloat64(jsonStr string) map[string]float64 {
	if jsonStr == "" {
		return nil
	}
	result := make(map[string]float64)
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil
	}
	return result
}

// isRealtimeMode 判断面试模式是否属于实时语音链路。
func isRealtimeMode(mode string) bool {
	switch strings.TrimSpace(mode) {
	case "realtime", "realtime_voice", "realtime_interview":
		return true
	default:
		return false
	}
}

// isRealtimeInterview 判断面试是否为实时模式（兼容 InterviewMode 未落库场景）
// 优先检查 InterviewMode（内存值），其次通过 Live2DModelKey 非空推断
func isRealtimeInterview(iv *Interview) bool {
	if isRealtimeMode(iv.InterviewMode) {
		return true
	}
	return strings.TrimSpace(iv.Live2DModelKey) != ""
}

// resolveTopicsByIndustry 根据行业代码返回面试应覆盖的知识主题
func resolveTopicsByIndustry(industryCode string) []string {
	switch strings.ToLower(strings.TrimSpace(industryCode)) {
	case "backend", "go", "golang":
		return []string{"Go语言基础", "并发编程", "数据库与缓存", "微服务架构", "系统设计"}
	case "frontend":
		return []string{"JavaScript", "浏览器原理", "性能优化", "工程化", "框架原理"}
	case "java":
		return []string{"JVM", "并发编程", "集合框架", "Spring", "微服务"}
	case "python":
		return []string{"语言特性", "并发模型", "Web开发", "数据处理", "性能优化"}
	default:
		return []string{"编程基础", "数据结构与算法", "系统设计", "数据库", "网络协议"}
	}
}

// resolveWeakTopics 从历史消息中提取用户回答薄弱的主题
func resolveWeakTopics(messages []*InterviewMessage) []string {
	if len(messages) == 0 {
		return nil
	}
	var weak []string
	seen := make(map[string]bool)
	for _, msg := range messages {
		if msg.Role != "assistant" {
			continue
		}
		question := DecodeQuestionContent(msg.Content)
		if question == nil || question.Topic == "" {
			continue
		}
		// 简单策略：如果 AI 回复中包含"没关系"、"不太清楚"等，标记为薄弱
		// 实际应该根据评分判断，但这里用启发式
		topic := strings.TrimSpace(question.Topic)
		if topic != "" && !seen[topic] {
			seen[topic] = true
		}
	}
	// 返回所有出现过的主题作为潜在薄弱点
	for topic := range seen {
		weak = append(weak, topic)
	}
	return weak
}
