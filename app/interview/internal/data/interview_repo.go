package data

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"gorm.io/gorm"

	"makejob/app/interview/internal/biz"
	"makejob/app/interview/internal/data/model"
)

// txContextKey 用于在 context 中透传事务 DB（FIX I1）
type txContextKey struct{}

type interviewRepo struct {
	db *gorm.DB
}

// NewInterviewRepo 创建面试仓库（由 Wire 调用）
func NewInterviewRepo(db *gorm.DB) biz.InterviewRepo {
	return &interviewRepo{db: db}
}

// getDB 从 context 获取事务 DB，若无则返回默认 DB
func (r *interviewRepo) getDB(ctx context.Context) *gorm.DB {
	if tx, ok := ctx.Value(txContextKey{}).(*gorm.DB); ok {
		return tx
	}
	return r.db
}

// Transaction 在事务中执行操作，将 tx 注入 context（FIX I1）
func (r *interviewRepo) Transaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txCtx := context.WithValue(ctx, txContextKey{}, tx)
		return fn(txCtx)
	})
}

// Create 创建面试记录并回填数据库生成的主键与时间字段。
func (r *interviewRepo) Create(ctx context.Context, interview *biz.Interview) error {
	m := toModel(interview)
	if err := r.getDB(ctx).WithContext(ctx).Create(m).Error; err != nil {
		return err
	}
	interview.ID = uint64(m.ID)
	interview.CreatedAt = m.CreatedAt
	interview.UpdatedAt = m.UpdatedAt
	return nil
}

// GetByID 按主键读取面试记录，并在事务上下文中复用同一个 DB 连接。
func (r *interviewRepo) GetByID(ctx context.Context, id uint64) (*biz.Interview, error) {
	var m model.MockInterview
	if err := r.getDB(ctx).WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return toBiz(&m), nil
}

// ListByUser 分页查询用户面试列表，并支持事务内一致性读取。
func (r *interviewRepo) ListByUser(ctx context.Context, userID uint64, page, pageSize int32) ([]*biz.Interview, int64, error) {
	var models []model.MockInterview
	var total int64

	query := r.getDB(ctx).WithContext(ctx).Where("user_id = ?", userID)
	query.Model(&model.MockInterview{}).Count(&total)

	offset := (page - 1) * pageSize
	if err := query.Offset(int(offset)).Limit(int(pageSize)).
		Order("created_at DESC").Find(&models).Error; err != nil {
		return nil, 0, err
	}

	interviews := make([]*biz.Interview, len(models))
	for i, m := range models {
		interviews[i] = toBiz(&m)
	}
	return interviews, total, nil
}

// Update 更新面试记录，若存在事务上下文则写入同一事务。
func (r *interviewRepo) Update(ctx context.Context, interview *biz.Interview) error {
	m := toModel(interview)
	return r.getDB(ctx).WithContext(ctx).Save(m).Error
}

// CreateMessage 创建面试消息，并在事务内复用同一个 DB 连接。
func (r *interviewRepo) CreateMessage(ctx context.Context, msg *biz.InterviewMessage) error {
	m := &model.InterviewMessage{
		InterviewID:   msg.InterviewID,
		Role:          msg.Role,
		Content:       msg.Content,
		MessageType:   msg.MessageType,
		QuestionIndex: msg.QuestionIndex,
	}
	return r.getDB(ctx).WithContext(ctx).Create(m).Error
}

// ListMessages 按时间正序读取面试消息，并支持事务内读取未提交写入。
func (r *interviewRepo) ListMessages(ctx context.Context, interviewID uint64) ([]*biz.InterviewMessage, error) {
	var models []model.InterviewMessage
	if err := r.getDB(ctx).WithContext(ctx).Where("interview_id = ?", interviewID).
		Order("created_at ASC").Find(&models).Error; err != nil {
		return nil, err
	}
	msgs := make([]*biz.InterviewMessage, len(models))
	for i, m := range models {
		msgs[i] = &biz.InterviewMessage{
			ID:            m.ID,
			InterviewID:   m.InterviewID,
			Role:          m.Role,
			Content:       m.Content,
			MessageType:   m.MessageType,
			QuestionIndex: m.QuestionIndex,
			CreatedAt:     time.UnixMilli(m.CreatedAt),
		}
	}
	return msgs, nil
}

// CreateCodingAttempt 创建编程题作答记录，并在事务内复用同一个 DB 连接。
func (r *interviewRepo) CreateCodingAttempt(ctx context.Context, attempt *biz.CodingAttempt) error {
	m := &model.InterviewCodingAttempt{
		InterviewID:     attempt.InterviewID,
		QuestionIndex:   attempt.QuestionIndex,
		Language:        attempt.Language,
		Code:            attempt.Code,
		Passed:          attempt.Passed,
		TestCasesPassed: attempt.TestCasesPassed,
		TotalTestCases:  attempt.TotalTestCases,
		Output:          attempt.Output,
		ErrorMsg:        attempt.ErrorMsg,
		AIScore:         attempt.AIScore,
		AIFeedback:      attempt.AIFeedback,
	}
	return r.getDB(ctx).WithContext(ctx).Create(m).Error
}

// UpdateCodingAttempt 更新编程题作答记录，并在事务内复用同一个 DB 连接。
func (r *interviewRepo) UpdateCodingAttempt(ctx context.Context, attempt *biz.CodingAttempt) error {
	m := &model.InterviewCodingAttempt{
		ID:              attempt.ID,
		InterviewID:     attempt.InterviewID,
		QuestionIndex:   attempt.QuestionIndex,
		Language:        attempt.Language,
		Code:            attempt.Code,
		Passed:          attempt.Passed,
		TestCasesPassed: attempt.TestCasesPassed,
		TotalTestCases:  attempt.TotalTestCases,
		Output:          attempt.Output,
		ErrorMsg:        attempt.ErrorMsg,
		AIScore:         attempt.AIScore,
		AIFeedback:      attempt.AIFeedback,
	}
	return r.getDB(ctx).WithContext(ctx).Save(m).Error
}

// ListCodingAttempts 获取面试的所有编程答题记录
func (r *interviewRepo) ListCodingAttempts(ctx context.Context, interviewID uint64) ([]*biz.CodingAttempt, error) {
	var models []model.InterviewCodingAttempt
	if err := r.getDB(ctx).WithContext(ctx).Where("interview_id = ?", interviewID).
		Order("question_index ASC").Find(&models).Error; err != nil {
		return nil, err
	}
	attempts := make([]*biz.CodingAttempt, len(models))
	for i, m := range models {
		attempts[i] = &biz.CodingAttempt{
			ID:              m.ID,
			InterviewID:     m.InterviewID,
			QuestionIndex:   m.QuestionIndex,
			Language:        m.Language,
			Code:            m.Code,
			Passed:          m.Passed,
			TestCasesPassed: m.TestCasesPassed,
			TotalTestCases:  m.TotalTestCases,
			Output:          m.Output,
			ErrorMsg:        m.ErrorMsg,
			AIScore:         m.AIScore,
			AIFeedback:      m.AIFeedback,
		}
	}
	return attempts, nil
}

// ListMessagesLimited 获取面试最近 N 条消息，并按时间正序返回。
func (r *interviewRepo) ListMessagesLimited(ctx context.Context, interviewID uint64, limit int32) ([]*biz.InterviewMessage, error) {
	var models []model.InterviewMessage
	if err := r.getDB(ctx).WithContext(ctx).Where("interview_id = ?", interviewID).
		Order("created_at DESC").Limit(int(limit)).Find(&models).Error; err != nil {
		return nil, err
	}
	for left, right := 0, len(models)-1; left < right; left, right = left+1, right-1 {
		models[left], models[right] = models[right], models[left]
	}
	msgs := make([]*biz.InterviewMessage, len(models))
	for i, m := range models {
		msgs[i] = &biz.InterviewMessage{
			ID:            m.ID,
			InterviewID:   m.InterviewID,
			Role:          m.Role,
			Content:       m.Content,
			MessageType:   m.MessageType,
			QuestionIndex: m.QuestionIndex,
			CreatedAt:     time.UnixMilli(m.CreatedAt),
		}
	}
	return msgs, nil
}

// BindRealtimeDialog 绑定实时对话 ID 到面试记录（复用 ai_session_id 列）
func (r *interviewRepo) BindRealtimeDialog(ctx context.Context, interviewID uint64, dialogID string) error {
	return r.getDB(ctx).WithContext(ctx).Model(&model.MockInterview{}).
		Where("id = ?", interviewID).
		Update("ai_session_id", dialogID).Error
}

// AppendMessageAndBumpIndex 追加消息（表无 current_index 列，不再递增）
func (r *interviewRepo) AppendMessageAndBumpIndex(ctx context.Context, msg *biz.InterviewMessage) error {
	m := &model.InterviewMessage{
		InterviewID:   msg.InterviewID,
		Role:          msg.Role,
		Content:       msg.Content,
		MessageType:   msg.MessageType,
		QuestionIndex: msg.QuestionIndex,
	}
	return r.getDB(ctx).WithContext(ctx).Create(m).Error
}

// --- Model ↔ Biz 转换 ---

func toModel(iv *biz.Interview) *model.MockInterview {
	m := &model.MockInterview{
		UserID:              iv.UserID,
		IndustryID:          iv.IndustryID,
		Status:              iv.Status,
		InterviewType:       iv.InterviewType,
		KnowledgeTopicsJSON: marshalKnowledgeTopics(iv.KnowledgeTopics),
		ResumeText:          iv.ResumeText,
		JobDescription:      iv.JobDescription,
		ResumeParsedJSON:    iv.ResumeParsedJSON,
		Score:               iv.OverallScore,
		TotalQuestions:      iv.QuestionCount,
		AIFeedback:          iv.AIFeedback,
		AISessionID:         iv.AISessionID,
		ReportJSON:          iv.ReportJSON,
		StartedAt:           iv.StartedAt,
		EndedAt:             iv.FinishedAt,
		Live2DModelKey:      iv.Live2DModelKey,
	}
	if iv.ID > 0 {
		m.Model.ID = uint(iv.ID)
	}
	if !iv.CreatedAt.IsZero() {
		m.CreatedAt = iv.CreatedAt
	}
	return m
}

func toBiz(m *model.MockInterview) *biz.Interview {
	return &biz.Interview{
		ID:              uint64(m.ID),
		UserID:          m.UserID,
		IndustryID:      m.IndustryID,
		Status:          m.Status,
		InterviewType:   m.InterviewType,
		KnowledgeTopics: unmarshalKnowledgeTopics(m.KnowledgeTopicsJSON),
		ResumeText:      m.ResumeText,
		JobDescription:  m.JobDescription,
		ResumeParsedJSON: m.ResumeParsedJSON,
		OverallScore:    m.Score,
		QuestionCount:   m.TotalQuestions,
		AIFeedback:      m.AIFeedback,
		AISessionID:     m.AISessionID,
		ReportJSON:      m.ReportJSON,
		StartedAt:       m.StartedAt,
		FinishedAt:      m.EndedAt,
		Live2DModelKey:  m.Live2DModelKey,
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
	}
}

// marshalKnowledgeTopics 序列化知识点列表为 JSON 字符串，空切片返回 "[]"。
func marshalKnowledgeTopics(topics []string) string {
	if topics == nil {
		return "[]"
	}
	data, err := json.Marshal(topics)
	if err != nil {
		return "[]"
	}
	return string(data)
}

// unmarshalKnowledgeTopics 反序列化知识点 JSON，失败或为空返回 nil。
func unmarshalKnowledgeTopics(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var topics []string
	if err := json.Unmarshal([]byte(raw), &topics); err != nil {
		return nil
	}
	return topics
}

// GetStats SQL 聚合查询面试统计（FIX I3: 避免全量加载）
func (r *interviewRepo) GetStats(ctx context.Context, userID uint64) (*biz.InterviewStats, error) {
	var stats struct {
		Total          int64
		Avg            float64
		TotalQuestions int64
		Completed      int64
	}
	if err := r.db.WithContext(ctx).Model(&model.MockInterview{}).
		Where("user_id = ?", userID).
		Select("COUNT(*) as total, COALESCE(AVG(score), 0) as avg, COALESCE(SUM(total_questions), 0) as total_questions, COUNT(CASE WHEN status = 'completed' THEN 1 END) as completed").
		Scan(&stats).Error; err != nil {
		return nil, err
	}

	// 查询当天完成的面试数量
	today := time.Now().Format("2006-01-02")
	var todayCount int64
	r.db.WithContext(ctx).Model(&model.MockInterview{}).
		Where("user_id = ? AND status = 'completed' AND DATE(ended_at) = ?", userID, today).
		Count(&todayCount)

	return &biz.InterviewStats{
		TotalInterviews:        int32(stats.Total),
		AvgScore:               stats.Avg,
		TotalQuestionsAnswered: int32(stats.TotalQuestions),
		AvgAccuracy:            stats.Avg / 100,
		CompletedInterviews:    int32(stats.Completed),
		TodayCount:             int32(todayCount),
	}, nil
}

// GetAdminStats 统计全站面试总量，供管理后台仪表盘聚合使用。
func (r *interviewRepo) GetAdminStats(ctx context.Context) (int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&model.MockInterview{}).Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}
