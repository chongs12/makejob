// Package ai 提供AI能力抽象层
// 定义了面试Agent、学习规划Agent、陪伴聊天Agent、刷题分析Agent等接口
// 当前为Mock实现，后续可无缝替换为Eino框架真实集成
package ai

// Message AI对话消息
type Message struct {
	Role    string `json:"role"` // system/user/assistant
	Content string `json:"content"`
}

// InterviewConfig 面试配置
type InterviewConfig struct {
	IndustryCode  string   `json:"industry_code"`
	Difficulty    string   `json:"difficulty"` // easy/medium/hard/mixed
	Topics        []string `json:"topics"`     // 面试主题
	QuestionCount int      `json:"question_count"`
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
	OverallScore     float64                   `json:"overall_score"`
	TotalQuestions   int                       `json:"total_questions"`
	CorrectCount     int                       `json:"correct_count"`
	DimensionScores  map[string]float64        `json:"dimension_scores"` // 各维度评分
	Strengths        []string                  `json:"strengths"`
	Weaknesses       []string                  `json:"weaknesses"`
	Suggestions      []string                  `json:"suggestions"`
	Summary          string                    `json:"summary"`
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
	Question       string               `json:"question"`
	Language       string               `json:"language"`
	FinalCode      string               `json:"final_code"`
	FinalAnswer    string               `json:"final_answer"`
	ProcessEvents  []CodingProcessEvent `json:"process_events"`
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
	Duration    int        `json:"duration_days"`
	Tasks       []PlanTask `json:"tasks"`
}

// PlanTask 计划任务
type PlanTask struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	TaskType    string `json:"task_type"` // study/practice/interview/review
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
	Content string `json:"content"`
	Emotion string `json:"emotion"` // happy/neutral/encouraging/thinking
	Action  string `json:"action"`  // idle/wave/nod/celebrate
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
