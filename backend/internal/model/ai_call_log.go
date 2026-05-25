// Package model 定义数据库模型。
package model

const (
	AICallSourceAdminDebug       = "admin_debug"       // 管理端调试调用来源。
	AICallSourceInterviewRuntime = "interview_runtime" // 面试链路运行时调用来源。
	AICallSourcePlanRuntime      = "plan_runtime"      // 学习计划链路运行时调用来源。
	AICallSourceCompanionRuntime = "companion_runtime" // 陪伴与 Live2D 链路运行时调用来源。
	AICallSourceQuizRuntime      = "quiz_runtime"      // 题目分析链路运行时调用来源。
)

// AICallLog 表示一次 AI 调用的观测日志。
type AICallLog struct {
	BaseModel
	TraceID            string `json:"trace_id" gorm:"size:64;not null;index;comment:调用链路ID"`
	TaskID             *uint  `json:"task_id,omitempty" gorm:"index;comment:关联异步任务ID"`
	Source             string `json:"source" gorm:"size:32;not null;index;comment:调用来源"`
	Scene              string `json:"scene" gorm:"size:32;not null;index;comment:场景"`
	IndustryID         *uint  `json:"industry_id,omitempty" gorm:"index;comment:行业ID"`
	PromptSource       string `json:"prompt_source" gorm:"size:64;comment:Prompt来源"`
	SelectedPromptID   *uint  `json:"selected_prompt_id,omitempty" gorm:"index;comment:命中的Prompt模板ID"`
	SelectedPromptName string `json:"selected_prompt_name" gorm:"size:255;comment:命中的Prompt模板名称"`
	RenderedPrompt     string `json:"rendered_prompt" gorm:"type:text;comment:渲染后的Prompt"`
	RequestMessages    string `json:"request_messages" gorm:"type:text;comment:请求消息JSON"`
	RuntimeConfig      string `json:"runtime_config" gorm:"type:text;comment:运行时配置JSON"`
	SceneConfig        string `json:"scene_config" gorm:"type:text;comment:场景配置JSON"`
	Provider           string `json:"provider" gorm:"size:64;comment:Provider"`
	Model              string `json:"model" gorm:"size:128;comment:模型"`
	UserInput          string `json:"user_input" gorm:"type:text;comment:用户输入"`
	ModelOutput        string `json:"model_output" gorm:"type:text;comment:模型输出"`
	ModelError         string `json:"model_error" gorm:"type:text;comment:模型错误"`
	LatencyMS          int64  `json:"latency_ms" gorm:"not null;default:0;comment:耗时毫秒"`
	IsSuccess          bool   `json:"is_success" gorm:"not null;default:false;index;comment:是否成功"`
	InputTokens        int    `json:"input_tokens" gorm:"not null;default:0;comment:输入token数"`
	OutputTokens       int    `json:"output_tokens" gorm:"not null;default:0;comment:输出token数"`
}

// TableName 返回 AI 调用日志表名。
func (AICallLog) TableName() string {
	return "ai_call_logs"
}
