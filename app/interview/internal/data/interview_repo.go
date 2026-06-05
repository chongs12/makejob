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

func (r *interviewRepo) Create(ctx context.Context, interview *biz.Interview) error {
	m := toModel(interview)
	return r.db.WithContext(ctx).Create(m).Error
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
	}
	return r.db.WithContext(ctx).Save(m).Error
}

// --- Model ↔ Biz 转换 ---

func toModel(iv *biz.Interview) *model.MockInterview {
	return &model.MockInterview{
		Model:          gorm.Model{ID: uint(iv.ID)},
		UserID:         iv.UserID,
		IndustryCode:   iv.IndustryCode,
		Difficulty:     iv.Difficulty,
		Status:         iv.Status,
		InterviewMode:  iv.InterviewMode,
		QuestionCount:  iv.QuestionCount,
		CurrentIndex:   iv.CurrentIndex,
		OverallScore:   iv.OverallScore,
		ResumeText:     iv.ResumeText,
		JobDescription: iv.JobDescription,
		Live2DModelKey: iv.Live2DModelKey,
	}
}

func toBiz(m *model.MockInterview) *biz.Interview {
	return &biz.Interview{
		ID:             uint64(m.ID),
		UserID:         m.UserID,
		IndustryCode:   m.IndustryCode,
		Difficulty:     m.Difficulty,
		Status:         m.Status,
		InterviewMode:  m.InterviewMode,
		QuestionCount:  m.QuestionCount,
		CurrentIndex:   m.CurrentIndex,
		OverallScore:   m.OverallScore,
		ResumeText:     m.ResumeText,
		JobDescription: m.JobDescription,
		Live2DModelKey: m.Live2DModelKey,
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
	}
}
