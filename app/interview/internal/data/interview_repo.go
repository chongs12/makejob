package data

import (
	"context"

	"gorm.io/gorm"

	"makejob/app/interview/internal/biz"
	"makejob/app/interview/internal/data/model"
)

type interviewRepo struct {
	db *gorm.DB
}

// NewInterviewRepo 创建面试仓库（由 Wire 调用）
func NewInterviewRepo(db *gorm.DB) biz.InterviewRepo {
	return &interviewRepo{db: db}
}

// Create 创建面试记录并回填数据库生成的主键与时间字段。
func (r *interviewRepo) Create(ctx context.Context, interview *biz.Interview) error {
	m := toModel(interview)
	if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
		return err
	}
	interview.ID = uint64(m.ID)
	interview.CreatedAt = m.CreatedAt
	interview.UpdatedAt = m.UpdatedAt
	return nil
}

func (r *interviewRepo) GetByID(ctx context.Context, id uint64) (*biz.Interview, error) {
	var m model.MockInterview
	if err := r.db.WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return toBiz(&m), nil
}

func (r *interviewRepo) ListByUser(ctx context.Context, userID uint64, page, pageSize int32) ([]*biz.Interview, int64, error) {
	var models []model.MockInterview
	var total int64

	query := r.db.WithContext(ctx).Where("user_id = ?", userID)
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

func (r *interviewRepo) Update(ctx context.Context, interview *biz.Interview) error {
	m := toModel(interview)
	return r.db.WithContext(ctx).Save(m).Error
}

func (r *interviewRepo) CreateMessage(ctx context.Context, msg *biz.InterviewMessage) error {
	m := &model.InterviewMessage{
		InterviewID:   msg.InterviewID,
		Role:          msg.Role,
		Content:       msg.Content,
		MessageType:   msg.MessageType,
		QuestionIndex: msg.QuestionIndex,
	}
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *interviewRepo) ListMessages(ctx context.Context, interviewID uint64) ([]*biz.InterviewMessage, error) {
	var models []model.InterviewMessage
	if err := r.db.WithContext(ctx).Where("interview_id = ?", interviewID).
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
			CreatedAt:     m.CreatedAt,
		}
	}
	return msgs, nil
}

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
	return r.db.WithContext(ctx).Create(m).Error
}

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
	return r.db.WithContext(ctx).Save(m).Error
}

// ListCodingAttempts 获取面试的所有编程答题记录
func (r *interviewRepo) ListCodingAttempts(ctx context.Context, interviewID uint64) ([]*biz.CodingAttempt, error) {
	var models []model.InterviewCodingAttempt
	if err := r.db.WithContext(ctx).Where("interview_id = ?", interviewID).
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
	if err := r.db.WithContext(ctx).Where("interview_id = ?", interviewID).
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
			CreatedAt:     m.CreatedAt,
		}
	}
	return msgs, nil
}

// BindRealtimeDialog 绑定实时对话 ID 到面试记录
func (r *interviewRepo) BindRealtimeDialog(ctx context.Context, interviewID uint64, dialogID string) error {
	return r.db.WithContext(ctx).Model(&model.MockInterview{}).
		Where("id = ?", interviewID).
		Update("realtime_dialog_id", dialogID).Error
}

// AppendMessageAndBumpIndex 追加消息并递增 current_question_index（事务操作）
func (r *interviewRepo) AppendMessageAndBumpIndex(ctx context.Context, msg *biz.InterviewMessage) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 插入消息
		m := &model.InterviewMessage{
			InterviewID:   msg.InterviewID,
			Role:          msg.Role,
			Content:       msg.Content,
			MessageType:   msg.MessageType,
			QuestionIndex: msg.QuestionIndex,
		}
		if err := tx.Create(m).Error; err != nil {
			return err
		}
		// 递增 current_question_index
		return tx.Model(&model.MockInterview{}).
			Where("id = ?", msg.InterviewID).
			Update("current_index", gorm.Expr("current_index + 1")).Error
	})
}

// --- Model ↔ Biz 转换 ---

func toModel(iv *biz.Interview) *model.MockInterview {
	return &model.MockInterview{
		Model:            gorm.Model{ID: uint(iv.ID)},
		UserID:           iv.UserID,
		IndustryCode:     iv.IndustryCode,
		Difficulty:       iv.Difficulty,
		Status:           iv.Status,
		InterviewMode:    iv.InterviewMode,
		QuestionCount:    iv.QuestionCount,
		CurrentIndex:     iv.CurrentIndex,
		OverallScore:     iv.OverallScore,
		ResumeText:       iv.ResumeText,
		ResumeParsedJSON: iv.ResumeParsedJSON,
		JobDescription:   iv.JobDescription,
		Live2DModelKey:   iv.Live2DModelKey,
		RealtimeDialogID: iv.RealtimeDialogID,
		FinishedAt:       iv.FinishedAt,
	}
}

func toBiz(m *model.MockInterview) *biz.Interview {
	return &biz.Interview{
		ID:               uint64(m.ID),
		UserID:           m.UserID,
		IndustryCode:     m.IndustryCode,
		Difficulty:       m.Difficulty,
		Status:           m.Status,
		InterviewMode:    m.InterviewMode,
		QuestionCount:    m.QuestionCount,
		CurrentIndex:     m.CurrentIndex,
		OverallScore:     m.OverallScore,
		ResumeText:       m.ResumeText,
		ResumeParsedJSON: m.ResumeParsedJSON,
		JobDescription:   m.JobDescription,
		Live2DModelKey:   m.Live2DModelKey,
		RealtimeDialogID: m.RealtimeDialogID,
		FinishedAt:       m.FinishedAt,
		CreatedAt:        m.CreatedAt,
		UpdatedAt:        m.UpdatedAt,
	}
}
