// Package model 提供数据模型定义
package model

// QuestionType 题目类型枚举
const (
	QuestionTypeChoice     = "choice"     // 单选题
	QuestionTypeMulti      = "multi"      // 多选题
	QuestionTypeCode       = "code"       // 编程题
	QuestionTypeSubjective = "subjective" // 主观题
)

// QuestionDifficulty 题目难度枚举
const (
	QuestionDifficultyEasy   = "easy"   // 简单
	QuestionDifficultyMedium = "medium" // 中等
	QuestionDifficultyHard   = "hard"   // 困难
)

// Question 题目表
type Question struct {
	BaseModel
	CategoryID         uint   `json:"category_id" gorm:"not null;index;comment:分类ID"`
	IndustryID         uint   `json:"industry_id" gorm:"not null;index;comment:行业ID"`
	Type               string `json:"type" gorm:"size:20;not null;comment:题目类型(choice/multi/code/subjective)"`
	Difficulty         string `json:"difficulty" gorm:"size:10;not null;default:'medium';comment:难度(easy/medium/hard)"`
	Title              string `json:"title" gorm:"size:500;not null;comment:题目标题"`
	Content            string `json:"content" gorm:"type:text;not null;comment:题目内容"`
	OptionsJSON        string `json:"options_json,omitempty" gorm:"type:text;comment:选择题选项JSON字符串"`
	Answer             string `json:"answer" gorm:"type:text;not null;comment:答案"`
	Explanation        string `json:"explanation" gorm:"type:text;comment:答案解析"`
	SolutionJSON       string `json:"solution_json,omitempty" gorm:"type:text;comment:结构化解题解析JSON"`
	AnswerTemplateJSON string `json:"answer_template_json,omitempty" gorm:"type:text;comment:主观题参考回答模板JSON"`
	Tags               string `json:"tags" gorm:"size:500;comment:标签，逗号分隔"`
	IsActive           bool   `json:"is_active" gorm:"not null;default:true;comment:是否启用"`

	// 关联关系
	Category Category `json:"category,omitempty" gorm:"foreignKey:CategoryID"`
	Industry Industry `json:"industry,omitempty" gorm:"foreignKey:IndustryID"`
}

// TableName 指定表名
func (Question) TableName() string {
	return "questions"
}

// IsChoice 判断是否为选择题
func (q *Question) IsChoice() bool {
	return q.Type == QuestionTypeChoice || q.Type == QuestionTypeMulti
}

// IsCode 判断是否为编程题
func (q *Question) IsCode() bool {
	return q.Type == QuestionTypeCode
}
