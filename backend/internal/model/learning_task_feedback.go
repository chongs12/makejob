// Package model 提供数据模型定义
package model

// TrainingType 训练反馈类型枚举。
const (
	TrainingTypeCoding      = "coding"       // 编程题
	TrainingTypeChoice      = "choice"       // 选择题
	TrainingTypeShortAnswer = "short_answer" // 简答题
	TrainingTypeGeneric     = "generic"      // 通用任务
)

// DifficultySelfAssessment 难度自评枚举。
const (
	DifficultyTooEasy   = "too_easy"   // 太简单
	DifficultyJustRight = "just_right" // 刚好
	DifficultyTooHard   = "too_hard"   // 太难
)

// LearningTaskFeedback 表示学习任务完成后的结构化训练反馈。
type LearningTaskFeedback struct {
	BaseModel
	PlanID                   uint   `json:"plan_id" gorm:"not null;index;comment:学习计划ID"`
	TaskID                   uint   `json:"task_id" gorm:"not null;index;comment:学习任务ID"`
	UserID                   uint   `json:"user_id" gorm:"not null;index;comment:用户ID"`
	QuestionID               *uint  `json:"question_id" gorm:"index;comment:关联题目ID"`
	TrainingType             string `json:"training_type" gorm:"size:32;not null;comment:训练类型(coding/choice/short_answer/generic)"`
	MistakeTagsJSON          string `json:"mistake_tags_json" gorm:"type:text;comment:错因标签JSON"`
	AttemptCount             int    `json:"attempt_count" gorm:"not null;default:0;comment:尝试次数"`
	TimeSpentSeconds         int    `json:"time_spent_seconds" gorm:"not null;default:0;comment:耗时秒数"`
	DifficultySelfAssessment string `json:"difficulty_self_assessment" gorm:"size:32;comment:用户自评难度(too_easy/just_right/too_hard)"`
	WrongAnswerJSON          string `json:"wrong_answer_json" gorm:"type:text;comment:错误答案或错误片段JSON"`
	Summary                  string `json:"summary" gorm:"type:text;comment:补充说明"`

	// 关联关系
	Plan LearningPlan `json:"plan,omitempty" gorm:"foreignKey:PlanID"`
	Task LearningTask `json:"task,omitempty" gorm:"foreignKey:TaskID"`
	User User         `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

// TableName 指定学习任务反馈表名。
func (LearningTaskFeedback) TableName() string {
	return "learning_task_feedback"
}
