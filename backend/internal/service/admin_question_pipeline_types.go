package service

const (
	defaultQuestionPipelineCount = 8
	maxQuestionPipelineCount     = 20
	maxPipelineMaterialSources   = 6
	questionPipelineModePlanned  = "planned"
	questionPipelineModeDirect   = "direct_single"
)

// AdminQuestionPipelineGenerateRequest 描述后台题目流水线生成请求。
type AdminQuestionPipelineGenerateRequest struct {
	IndustryCode     string   `json:"industry_code" binding:"required"`
	Requirement      string   `json:"requirement" binding:"required"`
	AgentPrompt      string   `json:"agent_prompt"`
	GenerationMode   string   `json:"generation_mode"`
	CandidateCount   int      `json:"candidate_count"`
	IncludeScraped   bool     `json:"include_scraped"`
	IncludeGenerated bool     `json:"include_generated"`
	Sources          []string `json:"sources"`
}

// AdminQuestionPipelineCard 描述前端确认前的候选题卡。
type AdminQuestionPipelineCard struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Content     string   `json:"content"`
	Type        string   `json:"type"`
	Difficulty  string   `json:"difficulty"`
	Category    string   `json:"category"`
	Answer      string   `json:"answer"`
	Explanation string   `json:"explanation"`
	Tags        []string `json:"tags"`
	Confidence  float64  `json:"confidence"`
	SourceType  string   `json:"source_type"`
	SourceLabel string   `json:"source_label"`
	SourceTitle string   `json:"source_title"`
	SourceURL   string   `json:"source_url"`
}

// AdminQuestionPipelineStats 描述题目流水线执行摘要。
type AdminQuestionPipelineStats struct {
	SearchedCount   int `json:"searched_count"`
	FetchedCount    int `json:"fetched_count"`
	ScrapedCount    int `json:"scraped_count"`
	GeneratedCount  int `json:"generated_count"`
	CandidateCount  int `json:"candidate_count"`
	SelectedSources int `json:"selected_sources"`
}

// AdminQuestionPipelineGenerateResponse 描述后台题目流水线候选结果。
type AdminQuestionPipelineGenerateResponse struct {
	IndustryCode   string                      `json:"industry_code"`
	Requirement    string                      `json:"requirement"`
	GenerationMode string                      `json:"generation_mode"`
	Cards          []AdminQuestionPipelineCard `json:"cards"`
	Warnings       []string                    `json:"warnings,omitempty"`
	Stats          AdminQuestionPipelineStats  `json:"stats"`
}

// AdminQuestionPipelineStreamEvent 描述题目流水线 SSE 推送事件的统一结构。
type AdminQuestionPipelineStreamEvent struct {
	Event      string                                 `json:"event"`
	Message    string                                 `json:"message,omitempty"`
	TraceID    string                                 `json:"trace_id,omitempty"`
	RawOutput  string                                 `json:"raw_output,omitempty"`
	SlotIndex  int                                    `json:"slot_index,omitempty"`
	RetryIndex int                                    `json:"retry_index,omitempty"`
	Card       *AdminQuestionPipelineCard             `json:"card,omitempty"`
	Response   *AdminQuestionPipelineGenerateResponse `json:"response,omitempty"`
}

// AdminQuestionPipelineStreamEmitter 描述题目流水线流式推送回调。
type AdminQuestionPipelineStreamEmitter func(event AdminQuestionPipelineStreamEvent) error

// AdminQuestionPipelineImportRequest 描述后台题目流水线导入请求。
type AdminQuestionPipelineImportRequest struct {
	IndustryCode string                            `json:"industry_code" binding:"required"`
	Cards        []AdminQuestionPipelineImportCard `json:"cards" binding:"required,min=1"`
}

// AdminQuestionPipelineImportCard 描述前端确认后回传的题卡数据。
type AdminQuestionPipelineImportCard struct {
	Title       string   `json:"title" binding:"required"`
	Content     string   `json:"content" binding:"required"`
	Type        string   `json:"type" binding:"required"`
	Difficulty  string   `json:"difficulty" binding:"required"`
	Category    string   `json:"category" binding:"required"`
	Answer      string   `json:"answer" binding:"required"`
	Explanation string   `json:"explanation"`
	Tags        []string `json:"tags"`
}

type questionPipelineMaterial struct {
	SourceType string
	Source     string
	Title      string
	URL        string
	Content    string
}

// questionPipelineTopicPlan 描述模型在生成题卡前拆出的单个考点计划。
type questionPipelineTopicPlan struct {
	Topic      string   `json:"topic"`
	Focus      string   `json:"focus"`
	Difficulty string   `json:"difficulty"`
	Category   string   `json:"category"`
	Tags       []string `json:"tags"`
}

// questionPipelinePlanResponse 描述模型拆解出的题卡计划列表。
type questionPipelinePlanResponse struct {
	Topics []questionPipelineTopicPlan `json:"topics"`
}

// questionPipelineModelCard 描述模型直接生成的结构化题卡。
type questionPipelineModelCard struct {
	Title       string   `json:"title"`
	Content     string   `json:"content"`
	Type        string   `json:"type"`
	Difficulty  string   `json:"difficulty"`
	Category    string   `json:"category"`
	Answer      string   `json:"answer"`
	Explanation string   `json:"explanation"`
	Tags        []string `json:"tags"`
}

// questionPipelineCardsResponse 描述模型返回的题卡数组。
type questionPipelineCardsResponse struct {
	Cards []questionPipelineModelCard `json:"cards"`
}

// questionPipelineConstraintProfile 描述从岗位要求与智能体指令中提炼出的硬约束。
type questionPipelineConstraintProfile struct {
	CandidateCount      int
	RequireSubjective   bool
	PreferDistinctTopic bool
	ExcludeProjectCards bool
	GoFeatureOnly       bool
	ExactLanguageCounts map[string]int
	ExactLanguageOrder  []string
	RemainingLanguage   string
}
