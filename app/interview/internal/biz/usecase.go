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

// InterviewUseCase 面试业务用例
type InterviewUseCase struct {
	repo       InterviewRepo
	ai         AIServiceClient
	archive    LearningArchiveClient
	industry   IndustryClient
	rag        RAGClient
	codeRunner CodeRunnerClient
	reportRepo ReportRepo
	publisher  MQPublisher
	logger     log.Logger
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
) *InterviewUseCase {
	return &InterviewUseCase{
		repo:       repo,
		ai:         ai,
		archive:    archive,
		industry:   industry,
		rag:        rag,
		codeRunner: codeRunner,
		reportRepo: reportRepo,
		publisher:  publisher,
		logger:     logger,
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
		Status:         "ongoing",
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
	if strings.TrimSpace(interview.ResumeText) != "" && uc.publisher != nil {
		if err := uc.publisher.PublishInterviewResumeParse(ctx, interview.ID, interview.UserID, interview.ResumeText); err != nil {
			log.Warnf("发布简历解析消息失败: interview_id=%d err=%v", interview.ID, err)
		}
	}
	if isRealtimeMode(interview.InterviewMode) {
		return interview, nil, nil
	}
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
	if firstQuestionMsg := BuildQuestionMessage(interview.ID, 0, firstQuestion); firstQuestionMsg != nil {
		if err := uc.repo.CreateMessage(ctx, firstQuestionMsg); err != nil {
			return nil, nil, kratosErr.InternalServer("SAVE_FIRST_QUESTION_FAILED", "保存第一道题失败").WithCause(err)
		}
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
	if interview.Status != "ongoing" {
		return nil, nil, ErrInterviewNotOngoing
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
		History:       NormalizeHistoryMessages(history),
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
	feedback := aiResp.Feedback
	if feedback == nil {
		feedback = &AnswerFeedback{}
	}
	feedbackText := strings.TrimSpace(feedback.Feedback)
	if feedbackText != "" {
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
	}
	if nextQuestionMsg := BuildQuestionMessage(interviewID, index+1, aiResp.Question); nextQuestionMsg != nil {
		if err := uc.repo.CreateMessage(ctx, nextQuestionMsg); err != nil {
			return nil, nil, kratosErr.InternalServer("SAVE_NEXT_QUESTION_FAILED", "保存下一题失败").WithCause(err)
		}
	}

	// 5. 更新面试进度
	interview.CurrentIndex = index + 1
	if err := uc.repo.Update(ctx, interview); err != nil {
		return nil, nil, kratosErr.InternalServer("UPDATE_FAILED", "更新面试状态失败").WithCause(err)
	}

	// FIX I4: 异步写入学习档案，使用独立超时 context，不阻塞答题响应
	go func() {
		archiveCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = uc.archive.WriteEntry(archiveCtx, &ArchiveEntry{
			UserID:          interview.UserID,
			SourceType:      "interview_answer",
			InterviewID:     interviewID,
			QuestionIndex:   index,
			IndustryCode:    interview.IndustryCode,
			EvidenceSummary: feedbackText,
			OccurredAt:      time.Now(),
		})
	}()

	return feedback, aiResp.Question, nil
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
		query := interview.JobDescription
		if query == "" {
			query = interview.IndustryCode + " 面试问题"
		}
		docs, err := uc.rag.Retrieve(ctx, query, 5)
		if err == nil && len(docs) > 0 {
			for _, doc := range docs {
				if ragContext != "" {
					ragContext += "\n"
				}
				ragContext += doc.Content
			}
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
func (uc *InterviewUseCase) ListInterviews(ctx context.Context, userID uint64, page, pageSize int32) ([]*Interview, int64, error) {
	return uc.repo.ListByUser(ctx, userID, page, pageSize)
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
	interview, err := uc.repo.GetByID(ctx, interviewID)
	if err != nil {
		return err
	}
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

	// FIX I1+I2: 在事务中完成报告创建和面试状态更新，reportRepo.Create 已使用 ON CONFLICT 保证幂等
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
	// 获取面试信息
	interview, err := uc.repo.GetByID(ctx, interviewID)
	if err != nil {
		return err
	}

	// 写入学习档案
	return uc.archive.WriteEntry(ctx, &ArchiveEntry{
		UserID:       interview.UserID,
		SourceType:   "interview_coding",
		InterviewID:  interviewID,
		IndustryCode: interview.IndustryCode,
		OccurredAt:   interview.CreatedAt,
	})
}

// FinishInterview 结束面试并触发报告生成
func (uc *InterviewUseCase) FinishInterview(ctx context.Context, interviewID, userID uint64) error {
	// 获取面试记录
	interview, err := uc.repo.GetByID(ctx, interviewID)
	if err != nil {
		return ErrInterviewNotFound
	}
	// 验证面试归属
	if interview.UserID != userID {
		return ErrUnauthorized
	}
	// 验证面试状态为进行中
	if interview.Status != "ongoing" {
		return ErrInterviewNotOngoing
	}

	// 更新状态为报告生成中
	now := time.Now()
	interview.Status = "report_generating"
	interview.FinishedAt = &now
	if err := uc.repo.Update(ctx, interview); err != nil {
		return kratosErr.InternalServer("UPDATE_FAILED", "更新面试状态失败").WithCause(err)
	}

	// 发布报告生成 MQ 消息
	if uc.publisher != nil {
		if err := uc.publisher.PublishInterviewReportGenerate(ctx, interviewID, userID); err != nil {
			log.Errorf("发布报告生成消息失败: interview_id=%d err=%v", interviewID, err)
		}
	}

	return nil
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
	if err := uc.repo.CreateMessage(ctx, codeMsg); err != nil {
		return nil, kratosErr.InternalServer("SAVE_CODE_FAILED", "保存编程答案失败").WithCause(err)
	}

	// 3. 调用 CodeRunner 执行代码（降级处理）
	result := &CodingAnswerResult{}
	if uc.codeRunner != nil {
		runnerResult, err := uc.codeRunner.Execute(ctx, language, code, nil)
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
	if err := uc.repo.CreateCodingAttempt(ctx, attempt); err != nil {
		return nil, kratosErr.InternalServer("SAVE_ATTEMPT_FAILED", "保存编程答题记录失败").WithCause(err)
	}

	// 6. 保存 AI 评审反馈消息
	if result.AIFeedback != "" {
		aiMsg := &InterviewMessage{
			InterviewID:   interviewID,
			Role:          "assistant",
			Content:       result.AIFeedback,
			MessageType:   "text",
			QuestionIndex: questionIndex,
		}
		if err := uc.repo.CreateMessage(ctx, aiMsg); err != nil {
			log.Errorf("保存 AI 评审反馈失败: %v", err)
		}
	}

	return result, nil
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
	return isRealtimeMode(interview.InterviewMode), nil
}

// RealtimeContextResult 实时面试上下文结果
type RealtimeContextResult struct {
	InterviewID   uint64
	IndustryCode  string
	Difficulty    string
	History       []*InterviewMessage
	CurrentTopic  string
	QuestionIndex int32
}

// GetRealtimeContext 加载实时面试上下文（面试信息 + 最近 10 条消息）
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
	if !isRealtimeMode(interview.InterviewMode) {
		return nil, ErrInterviewNotRealtime
	}

	// 加载最近 10 条消息
	recentMessages, err := uc.repo.ListMessagesLimited(ctx, interviewID, 10)
	if err != nil {
		return nil, kratosErr.InternalServer("HISTORY_FAILED", "获取最近消息失败").WithCause(err)
	}

	return &RealtimeContextResult{
		InterviewID:   interview.ID,
		IndustryCode:  interview.IndustryCode,
		Difficulty:    interview.Difficulty,
		History:       recentMessages,
		CurrentTopic:  ResolveCurrentTopic(recentMessages),
		QuestionIndex: interview.CurrentIndex,
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
	if !isRealtimeMode(interview.InterviewMode) {
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
	if !isRealtimeMode(interview.InterviewMode) {
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
	if !isRealtimeMode(interview.InterviewMode) {
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
		return &GetReportResult{Status: "generating"}, nil
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

// unmarshalStringSlice 从 JSON 字符串解析字符串切片
func unmarshalStringSlice(jsonStr string) []string {
	if jsonStr == "" {
		return nil
	}
	var result []string
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil
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
