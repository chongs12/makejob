package data

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"makejob/app/interview/internal/biz"
	"makejob/app/interview/internal/data/model"
)

type reportRepo struct {
	db *gorm.DB
}

// NewReportRepo 创建面试报告仓库（由 Wire 调用）
func NewReportRepo(db *gorm.DB) biz.ReportRepo {
	return &reportRepo{db: db}
}

// Create 创建面试报告记录
func (r *reportRepo) Create(ctx context.Context, report *biz.InterviewReport) error {
	m := &model.InterviewReport{
		InterviewID:           report.InterviewID,
		OverallScore:          report.OverallScore,
		DimensionScoresJSON:   report.DimensionScoresJSON,
		StrengthsJSON:         report.StrengthsJSON,
		WeaknessesJSON:        report.WeaknessesJSON,
		SuggestionsJSON:       report.SuggestionsJSON,
		Summary:               report.Summary,
		CodingDiagnosticsJSON: report.CodingDiagnosticsJSON,
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "interview_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"overall_score":           m.OverallScore,
			"dimension_scores_json":   m.DimensionScoresJSON,
			"strengths_json":          m.StrengthsJSON,
			"weaknesses_json":         m.WeaknessesJSON,
			"suggestions_json":        m.SuggestionsJSON,
			"summary":                 m.Summary,
			"coding_diagnostics_json": m.CodingDiagnosticsJSON,
		}),
	}).Create(m).Error
}

// GetByInterviewID 根据面试 ID 获取报告
func (r *reportRepo) GetByInterviewID(ctx context.Context, interviewID uint64) (*biz.InterviewReport, error) {
	var m model.InterviewReport
	if err := r.db.WithContext(ctx).Where("interview_id = ?", interviewID).First(&m).Error; err != nil {
		return nil, err
	}
	return &biz.InterviewReport{
		ID:                    m.ID,
		InterviewID:           m.InterviewID,
		OverallScore:          m.OverallScore,
		DimensionScoresJSON:   m.DimensionScoresJSON,
		StrengthsJSON:         m.StrengthsJSON,
		WeaknessesJSON:        m.WeaknessesJSON,
		SuggestionsJSON:       m.SuggestionsJSON,
		Summary:               m.Summary,
		CodingDiagnosticsJSON: m.CodingDiagnosticsJSON,
		CreatedAt:             m.CreatedAt,
	}, nil
}
