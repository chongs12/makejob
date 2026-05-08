// Package model 提供数据模型定义
package model

import "time"

// LearningTaskDiagnosisWeaknessStatus 定义任务诊断中的薄弱点状态。
const (
	LearningTaskDiagnosisWeaknessImproved   = "improved"   // 该薄弱点基本已被克服
	LearningTaskDiagnosisWeaknessPartial    = "partial"    // 该薄弱点有所改善但仍需观察
	LearningTaskDiagnosisWeaknessUnresolved = "unresolved" // 该薄弱点仍未解决
)

// LearningTaskDiagnosisActionType 定义任务诊断建议动作类型。
const (
	LearningTaskDiagnosisActionRaiseDifficulty      = "raise_difficulty"       // 提升后续难度
	LearningTaskDiagnosisActionKeepProgress         = "keep_progress"          // 保持当前推进节奏
	LearningTaskDiagnosisActionRepeatSamePattern    = "repeat_same_pattern"    // 同模式继续巩固
	LearningTaskDiagnosisActionSwitchVariantPattern = "switch_variant_pattern" // 换变式继续巩固
	LearningTaskDiagnosisActionAddReviewTask        = "add_review_task"        // 插入复盘任务
)

// LearningTaskDiagnosis 表示基于训练反馈生成的一条结构化诊断结果。
type LearningTaskDiagnosis struct {
	BaseModel
	FeedbackID       uint       `json:"feedback_id" gorm:"not null;uniqueIndex;comment:关联训练反馈ID"`
	PlanID           uint       `json:"plan_id" gorm:"not null;index;comment:学习计划ID"`
	TaskID           uint       `json:"task_id" gorm:"not null;index;comment:学习任务ID"`
	UserID           uint       `json:"user_id" gorm:"not null;index;comment:用户ID"`
	IndustryCode     string     `json:"industry_code" gorm:"size:50;index;comment:行业编码"`
	PlanPhase        string     `json:"plan_phase" gorm:"size:20;comment:诊断发生时计划所在阶段"`
	PlanPhaseGoal    string     `json:"plan_phase_goal" gorm:"type:text;comment:诊断发生时计划阶段目标"`
	EntryPhase       string     `json:"entry_phase" gorm:"size:20;comment:本轮计划入口阶段"`
	TaskPhase        string     `json:"task_phase" gorm:"size:20;comment:诊断对应任务阶段"`
	TaskPhaseGoal    string     `json:"task_phase_goal" gorm:"type:text;comment:诊断对应任务阶段目标"`
	WeaknessStatus   string     `json:"weakness_status" gorm:"size:32;not null;comment:薄弱点状态(improved/partial/unresolved)"`
	ActionJSON       string     `json:"action_json" gorm:"type:text;comment:建议动作JSON"`
	MistakeTagsJSON  string     `json:"mistake_tags_json" gorm:"type:text;comment:错因标签JSON"`
	StrengthTagsJSON string     `json:"strength_tags_json" gorm:"type:text;comment:优势标签JSON"`
	SuggestionsJSON  string     `json:"suggestions_json" gorm:"type:text;comment:建议列表JSON"`
	EvidenceJSON     string     `json:"evidence_json" gorm:"type:text;comment:证据列表JSON"`
	Summary          string     `json:"summary" gorm:"type:text;comment:诊断摘要"`
	OccurredAt       *time.Time `json:"occurred_at" gorm:"comment:诊断对应的训练发生时间"`

	// 关联关系
	Feedback LearningTaskFeedback `json:"feedback,omitempty" gorm:"foreignKey:FeedbackID"`
	Plan     LearningPlan         `json:"plan,omitempty" gorm:"foreignKey:PlanID"`
	Task     LearningTask         `json:"task,omitempty" gorm:"foreignKey:TaskID"`
	User     User                 `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

// TableName 指定学习任务诊断表名。
func (LearningTaskDiagnosis) TableName() string {
	return "learning_task_diagnoses"
}
