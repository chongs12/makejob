// Package ai 提供AI能力抽象层
// 定义了面试Agent、学习规划Agent、陪伴聊天Agent、刷题分析Agent等接口
// 当前为Mock实现，后续可无缝替换为Eino框架真实集成
package ai

import "context"

// Message AI对话消息
type Message struct {
	Role    string `json:"role"` // system/user/assistant
	Content string `json:"content"`
}

// InterviewConfig 面试配置
type InterviewConfig struct {
	IndustryCode   string   `json:"industry_code"`
	Difficulty     string   `json:"difficulty"` // easy/medium/hard/mixed
	Topics         []string `json:"topics"`     // 面试主题
	QuestionCount  int      `json:"question_count"`
	UserWeakTopics []string `json:"user_weak_topics"` // 从学习档案注入的用户近期高频薄弱主题
}

// ResumeProfile 描述从简历文本中解析出的结构化画像，供简历驱动面试模式使用。
type ResumeProfile struct {
	Summary      string   `json:"summary"`       // 一段话概括候选人背景
	Skills       []string `json:"skills"`        // 核心技术栈
	Projects     []string `json:"projects"`      // 重点项目经历（简述）
	Strengths    []string `json:"strengths"`     // 简历中体现的优势
	WeakSignals  []string `json:"weak_signals"`  // 简历中可能的薄弱信号（如技术栈跨度大但深度不足）
}

// InterviewQuestion 面试问题
type InterviewQuestion struct {
	Question       string `json:"question"`
	Topic          string `json:"topic"`
	Difficulty     string `json:"difficulty"`
	Type           string `json:"type"` // technical/behavioral/coding
	Hints          string `json:"hints,omitempty"`
	Language       string `json:"language,omitempty"`
	StarterCode    string `json:"starter_code,omitempty"`
	EditorMode     string `json:"editor_mode,omitempty"`
	EvaluationMode string `json:"evaluation_mode,omitempty"`
	Live2DDirective *Live2DDirective `json:"live2d_directive,omitempty"`
}

// AnswerFeedback 答案反馈
type AnswerFeedback struct {
	Score       float64  `json:"score"` // 0-100
	IsCorrect   bool     `json:"is_correct"`
	Feedback    string   `json:"feedback"`    // 详细评价
	KeyPoints   []string `json:"key_points"`  // 关键知识点
	Suggestions string   `json:"suggestions"` // 改进建议
	FollowUp    string   `json:"follow_up"`   // 追问(可选)
}

// InterviewReport 面试报告
type InterviewReport struct {
	OverallScore      float64                   `json:"overall_score"`
	TotalQuestions    int                       `json:"total_questions"`
	CorrectCount      int                       `json:"correct_count"`
	DimensionScores   map[string]float64        `json:"dimension_scores"` // 各维度评分
	Strengths         []string                  `json:"strengths"`
	Weaknesses        []string                  `json:"weaknesses"`
	Suggestions       []string                  `json:"suggestions"`
	Summary           string                    `json:"summary"`
	CodingDiagnostics []CodingQuestionDiagnosis `json:"coding_diagnostics,omitempty"`
}

// CodingProcessEvent 表示编程题过程采集中的单条事件。
type CodingProcessEvent struct {
	Type        string                 `json:"type"`
	TimestampMS int64                  `json:"timestamp_ms"`
	Payload     map[string]interface{} `json:"payload,omitempty"`
}

// InterviewCodingDiagnosisInput 表示编程面试诊断所需的完整上下文。
type InterviewCodingDiagnosisInput struct {
	Question      string               `json:"question"`
	Language      string               `json:"language"`
	FinalCode     string               `json:"final_code"`
	FinalAnswer   string               `json:"final_answer"`
	ProcessEvents []CodingProcessEvent `json:"process_events"`
}

// CodingQuestionDiagnosis 表示单道编程题的结构化诊断结果。
type CodingQuestionDiagnosis struct {
	QuestionIndex  int      `json:"question_index"`
	Language       string   `json:"language"`
	Score          float64  `json:"score"`
	MistakeTags    []string `json:"mistake_tags"`
	StrengthTags   []string `json:"strength_tags"`
	Evidence       []string `json:"evidence"`
	Suggestions    []string `json:"suggestions"`
	ProcessSummary string   `json:"process_summary"`
}

// LearningPlan 学习计划
type LearningPlan struct {
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Phase       string     `json:"phase,omitempty"`
	PhaseGoal   string     `json:"phase_goal,omitempty"`
	Duration    int        `json:"duration_days"`
	Tasks       []PlanTask `json:"tasks"`
}

// PlanTask 计划任务
type PlanTask struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	TaskType    string `json:"task_type"` // study/practice/interview/review
	Phase       string `json:"phase,omitempty"`
	PhaseGoal   string `json:"phase_goal,omitempty"`
	DayNumber   int    `json:"day_number"`
	Duration    int    `json:"duration_minutes"`
	Priority    string `json:"priority"` // high/medium/low
}

// UserProfile 用户画像(用于AI推断)
type UserProfile struct {
	Level           string   `json:"level"` // beginner/intermediate/advanced
	WeakTopics      []string `json:"weak_topics"`
	StrongTopics    []string `json:"strong_topics"`
	DailyStudyTime  int      `json:"daily_study_time"` // 分钟
	DurationDays    int      `json:"duration_days"`    // 学习计划总天数
	GoalDescription string   `json:"goal_description"`
}

// CompanionResponse 陪伴响应
type CompanionResponse struct {
	Content         string            `json:"content"`
	Emotion         string            `json:"emotion"` // happy/neutral/encouraging/thinking
	Action          string            `json:"action"`  // idle/wave/nod/celebrate
	Live2DDirective *Live2DDirective  `json:"live2d_directive,omitempty"`
}

// Live2DExpressionLayer 描述一层要叠加到前端模型上的表情指令。
type Live2DExpressionLayer struct {
	Key    string  `json:"key"`
	Weight float64 `json:"weight"`
}

// Live2DParameterOverride 描述一条需要写入模型参数的覆盖值。
type Live2DParameterOverride struct {
	ID    string  `json:"id"`
	Value float64 `json:"value"`
}

// Live2DDirective 描述大模型为当前回复生成的结构化 Live2D 控制指令。
type Live2DDirective struct {
	Emotion            string                    `json:"emotion,omitempty"`
	Action             string                    `json:"action,omitempty"`
	Reply              string                    `json:"reply,omitempty"`
	ExpressionMix      []Live2DExpressionLayer   `json:"expression_mix,omitempty"`
	ParameterOverrides []Live2DParameterOverride `json:"parameter_overrides,omitempty"`
	MotionKey          string                    `json:"motion_key,omitempty"`
	MotionGroup        string                    `json:"motion_group,omitempty"`
	MotionPriority     string                    `json:"motion_priority,omitempty"`
	MotionDurationMS   int                       `json:"motion_duration_ms,omitempty"`
	Intensity          float64                   `json:"intensity,omitempty"`
	DurationMS         int                       `json:"duration_ms,omitempty"`
	MouthOpen          *float64                  `json:"mouth_open,omitempty"`
	Source             string                    `json:"source,omitempty"`
}

// Live2DManifestExpression 描述模型可用表达式的稳定清单项。
type Live2DManifestExpression struct {
	Key   string `json:"key"`
	Label string `json:"label,omitempty"`
}

// Live2DManifestParameter 描述模型可控参数的稳定清单项。
type Live2DManifestParameter struct {
	ID    string  `json:"id"`
	Min   float64 `json:"min,omitempty"`
	Max   float64 `json:"max,omitempty"`
	Label string  `json:"label,omitempty"`
}

// Live2DManifestMotion 描述模型可供大模型选择的一条稳定动作清单项。
type Live2DManifestMotion struct {
	Key   string `json:"key"`
	Group string `json:"group,omitempty"`
	File  string `json:"file,omitempty"`
	Label string `json:"label,omitempty"`
}

// Live2DManifest 描述当前模型可供大模型调用的表达式和参数白名单。
type Live2DManifest struct {
	ModelKey    string                    `json:"model_key"`
	ModelName   string                    `json:"model_name"`
	Scene       string                    `json:"scene"`
	ModelURL    string                    `json:"model_url"`
	Expressions []Live2DManifestExpression `json:"expressions,omitempty"`
	Parameters  []Live2DManifestParameter  `json:"parameters,omitempty"`
	Motions     []Live2DManifestMotion     `json:"motions,omitempty"`
}

// Live2DDirectiveContext 描述一次 Live2D 指令生成需要的文本和业务上下文。
type Live2DDirectiveContext struct {
	Scene             string
	Model             Live2DManifest
	UserMessage       string
	AssistantReply    string
	UserEmotion       string
	QuestionIndex     int
	Question          *InterviewQuestion
	RecentMessages    []Message
	CurrentDirective  *Live2DDirective
	AdditionalContext map[string]string
}

// Live2DDirectiveGenerator 定义基于模型清单生成结构化 Live2D 指令的能力。
type Live2DDirectiveGenerator interface {
	GenerateDirective(ctx context.Context, req Live2DDirectiveContext) (*Live2DDirective, error)
}

// CodeAnalysis 代码分析结果
type CodeAnalysis struct {
	IsCorrect       bool     `json:"is_correct"`
	Score           float64  `json:"score"`
	Feedback        string   `json:"feedback"`
	Issues          []string `json:"issues"`
	Improvements    []string `json:"improvements"`
	MistakeTags     []string `json:"mistake_tags,omitempty"`
	StrengthTags    []string `json:"strength_tags,omitempty"`
	TimeComplexity  string   `json:"time_complexity"`
	SpaceComplexity string   `json:"space_complexity"`
}

// InterviewSession 面试会话状态（用于Mock实现）
type InterviewSession struct {
	SessionID    string
	Config       InterviewConfig
	CurrentIndex int
	Questions    []InterviewQuestion
	Answers      []string
	Feedbacks    []AnswerFeedback
	StartTime    int64
	IsActive     bool
}
