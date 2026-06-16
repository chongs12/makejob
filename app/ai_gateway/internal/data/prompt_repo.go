package data

import (
	"context"

	"gorm.io/gorm"

	"makejob/app/ai_gateway/internal/biz"
)

type promptRepo struct {
	db *gorm.DB
}

// NewPromptRepo 创建 Prompt 模板仓库实现。
func NewPromptRepo(db *gorm.DB) biz.PromptRepo {
	return &promptRepo{db: db}
}

// GetActiveTemplate 查询指定场景下最新生效的 Prompt 模板，并兼容单体时期的旧场景命名。
func (r *promptRepo) GetActiveTemplate(ctx context.Context, scene string) (*biz.PromptTemplate, error) {
	for _, candidateScene := range promptSceneCandidates(scene) {
		var tpl biz.PromptTemplate
		err := r.db.WithContext(ctx).
			Where("scene = ? AND is_active = ?", candidateScene, true).
			Order("id DESC").
			First(&tpl).Error
		if err == nil {
			return &tpl, nil
		}
		if err != gorm.ErrRecordNotFound {
			return nil, err
		}
	}
	return nil, gorm.ErrRecordNotFound
}

// promptSceneCandidates 返回当前场景可接受的模板场景候选列表，兼容旧后台的简化命名。
func promptSceneCandidates(scene string) []string {
	switch scene {
	case "question_generator", "quiz_analyzer":
		return []string{scene, "quiz"}
	case "interview_agent":
		return []string{scene, "interview"}
	case "plan_agent":
		return []string{scene, "plan"}
	case "companion_agent":
		return []string{scene, "companion"}
	default:
		return []string{scene}
	}
}
