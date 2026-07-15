package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	kratosErr "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrPlanNotFound          = kratosErr.NotFound("PLAN_NOT_FOUND", "学习计划不存在")
	ErrTaskNotFound          = kratosErr.NotFound("TASK_NOT_FOUND", "学习任务不存在")
	ErrFeedbackNotFound      = kratosErr.NotFound("FEEDBACK_NOT_FOUND", "任务反馈不存在")
	ErrNoActivePlan          = kratosErr.NotFound("NO_ACTIVE_PLAN", "当前没有进行中的学习计划")
	ErrInvalidLevel          = kratosErr.BadRequest("INVALID_LEVEL", "无效的计划级别")
	ErrInvalidDuration       = kratosErr.BadRequest("INVALID_DURATION", "无效的时长参数")
	ErrIndustryRequired      = kratosErr.BadRequest("INDUSTRY_REQUIRED", "行业不能为空")
	ErrGoalRequired          = kratosErr.BadRequest("GOAL_REQUIRED", "学习目标不能为空")
	ErrInvalidStatus         = kratosErr.BadRequest("INVALID_STATUS", "无效的任务状态")
	ErrStatusTransition      = kratosErr.BadRequest("STATUS_TRANSITION", "非法的状态转换")
	ErrPlanAccessDenied      = kratosErr.Forbidden("PLAN_ACCESS_DENIED", "无权访问该计划")
	ErrTaskNotBelong         = kratosErr.BadRequest("TASK_NOT_BELONG", "任务不属于该计划")
	ErrFeedbackTaskStatus    = kratosErr.BadRequest("FEEDBACK_TASK_STATUS", "仅已完成任务允许提交反馈")
	ErrPlanCompleted         = kratosErr.BadRequest("PLAN_COMPLETED", "计划已完成，无法调整")
	ErrAdjustFailed          = kratosErr.InternalServer("ADJUST_FAILED", "调整计划失败")
	ErrAdjustPreviewNotFound = kratosErr.NotFound("ADJUST_PREVIEW_NOT_FOUND", "调整预览不存在")
	ErrAdjustPreviewExpired  = kratosErr.BadRequest("ADJUST_PREVIEW_EXPIRED", "调整预览已过期，请重新生成")
	ErrAdjustPreviewApplied  = kratosErr.BadRequest("ADJUST_PREVIEW_APPLIED", "该调整预览已应用，请勿重复提交")
	ErrAdjustPreviewMismatch = kratosErr.Forbidden("ADJUST_PREVIEW_MISMATCH", "调整预览与当前计划不匹配")
	ErrFeedbackPublish       = kratosErr.InternalServer("FEEDBACK_PUBLISH_FAILED", "发布诊断消息失败")
	ErrPlanGenerateFailed    = kratosErr.InternalServer("PLAN_GENERATE_FAILED", "生成学习计划失败")
	ErrDiagnosisFailed       = kratosErr.InternalServer("DIAGNOSIS_FAILED", "诊断分析失败")
)

// LearningPlan 学习计划实体（FIX P2: 添加 DeletedAt 支持软删除）
// 对齐单体 learning_plans 表结构：非持久化字段标记 gorm:"-"
type LearningPlan struct {
	ID             uint64         `gorm:"primaryKey;autoIncrement"`
	UserID         uint64         `gorm:"index;not null"`
	IndustryID     uint64         `gorm:"column:industry_id;index;not null"`
	Title          string         `gorm:"size:200;not null"`
	Description    string         `gorm:"size:1000"`
	PlanJSON       string         `gorm:"column:plan_json;type:text"`
	Status         string         `gorm:"size:20;not null;default:'generating'"`
	TotalTasks     int32          `gorm:"not null;default:0"`
	CompletedTasks int32          `gorm:"not null;default:0"`
	StartDate      *time.Time     `gorm:"column:start_date"`
	EndDate        *time.Time     `gorm:"column:end_date"`
	Phase          string         `gorm:"size:50"`
	PhaseGoal      string         `gorm:"size:500"`
	CreatedAt      time.Time      `gorm:"not null;autoCreateTime"`
	UpdatedAt      time.Time      `gorm:"not null;autoUpdateTime"`
	DeletedAt      gorm.DeletedAt `gorm:"index"`

	// 运行期字段，仅内存流转，不落库
	Level                     string `gorm:"-"`
	DurationDays              int32  `gorm:"-"`
	DailyStudyMinutes         int32  `gorm:"-"`
	Industry                  string `gorm:"-"`
	EntryPhase                string `gorm:"-"`
	AdjustmentSummariesJSON   string `gorm:"-"`
	AdjustmentReasonCodesJSON string `gorm:"-"`
	PhaseBlueprintSummaryJSON string `gorm:"-"`
}

// PhaseBlueprintSummaryEntry 阶段蓝图摘要条目
type PhaseBlueprintSummaryEntry struct {
	Phase     string `json:"phase"`
	PhaseGoal string `json:"phase_goal"`
	StartDay  int    `json:"start_day"`
	EndDay    int    `json:"end_day"`
}

// TableName 指定学习计划表名
func (LearningPlan) TableName() string {
	return "learning_plans"
}

// CalculatePhase 根据任务完成进度计算当前学习阶段。
// foundation（基础）: 0-33%, drill（刷题）: 34-66%, mock（模拟）: 67-100%
func CalculatePhase(completed, total int32) string {
	if total <= 0 {
		return "foundation"
	}
	ratio := float64(completed) / float64(total)
	if ratio < 0.34 {
		return "foundation"
	}
	if ratio < 0.67 {
		return "drill"
	}
	return "mock"
}

// PhaseGoalMap 各阶段目标描述。
var PhaseGoalMap = map[string]string{
	"foundation": "夯实基础：掌握核心概念与基本用法",
	"drill":      "强化刷题：通过大量练习提升解题速度与准确率",
	"mock":       "模拟实战：模拟真实面试场景，查漏补缺",
}

// buildTaskSourceLabel 根据任务类型和阶段生成来源标签。
func buildTaskSourceLabel(taskType, phase string) string {
	switch taskType {
	case "practice":
		return "刷题练习"
	case "interview":
		return "模拟面试"
	case "review":
		return "复习巩固"
	case "study":
		if phase == "foundation" {
			return "基础学习"
		}
		return "进阶学习"
	default:
		return "AI 智能生成"
	}
}

// buildTaskReason 根据任务标题和阶段生成推荐原因。
func buildTaskReason(title, phase string) string {
	phaseLabel := PhaseGoalMap[phase]
	if phaseLabel != "" {
		return fmt.Sprintf("本任务属于%s阶段：%s", phase, phaseLabel)
	}
	return "根据学习计划智能推荐"
}

// LearningTask 学习任务实体
// 对齐单体 learning_tasks 表结构：非持久化字段标记 gorm:"-"
type LearningTask struct {
	ID          uint64         `gorm:"primaryKey;autoIncrement"`
	PlanID      uint64         `gorm:"index;not null"`
	Title       string         `gorm:"size:200;not null"`
	Description string         `gorm:"size:1000"`
	TaskType    string         `gorm:"size:20;not null"`
	Phase       string         `gorm:"size:50"`
	PhaseGoal   string         `gorm:"column:phase_goal;size:500"`
	TargetID    *uint64        `gorm:"column:target_id"`
	Status      string         `gorm:"size:20;not null;default:'pending'"`
	DueDate     *time.Time     `gorm:"column:due_date"`
	CompletedAt *time.Time     `gorm:"default:null"`
	SortOrder   int32          `gorm:"not null;default:0"`
	CreatedAt   time.Time      `gorm:"not null;autoCreateTime"`
	UpdatedAt   time.Time      `gorm:"not null;autoUpdateTime"`
	DeletedAt   gorm.DeletedAt `gorm:"index"`

	// 运行期字段，仅内存流转，不落库
	DayNumber           int32  `gorm:"-"`
	DurationMinutes     int32  `gorm:"-"`
	Priority            string `gorm:"-"`
	Source              string `gorm:"-"`
	SourceLabel         string `gorm:"-"`
	Reason              string `gorm:"-"`
	PriorityExplanation string `gorm:"-"`
	SourceRef           string `gorm:"-"`
	CollectionHint      string `gorm:"-"`
}

// TableName 指定学习任务表名
func (LearningTask) TableName() string {
	return "learning_tasks"
}

// CreatePlanRequest 创建计划的业务请求
type CreatePlanRequest struct {
	UserID            uint64
	IndustryCode      string
	Goal              string
	DailyHours        int32
	Level             string
	DurationDays      int32
	DailyStudyMinutes int32
	WeakTopics        []string
}

// PlanAgentRequest AI 生成计划请求
type PlanAgentRequest struct {
	PlanID            uint64
	UserID            uint64
	IndustryCode      string
	Goal              string
	DailyHours        int32
	WeakTopics        []string
	RecentActivities  []string
	Level             string
	DurationDays      int32
	DailyStudyMinutes int32
}

// PlanAgentResponse AI 生成计划响应
type PlanAgentResponse struct {
	PlanTitle string
	Tasks     []*PlanAgentTask
	Summary   string
}

// PlanAgentTask AI 返回的单个任务
type PlanAgentTask struct {
	Title           string
	Description     string
	TaskType        string
	Phase           string
	PhaseGoal       string
	DayNumber       int32
	DurationMinutes int32
	Priority        string
	SortOrder       int32
}

// PlanAgentAdjustRequest AI 调整计划请求
type PlanAgentAdjustRequest struct {
	Plan            *LearningPlan
	CurrentTasks    []*LearningTask
	Feedbacks       []*TaskFeedback
	Reason          string
	ExtraWeakTopics []string
}

// PlanAgentAdjustResponse AI 调整计划响应
type PlanAgentAdjustResponse struct {
	Add     []*PlanAgentTask
	Remove  []uint64
	Reorder map[uint64]int32
	Summary string
}

// PlanAdjustmentPreviewTask 调整预览中的任务快照，供前端确认新增与最终顺序。
type PlanAdjustmentPreviewTask struct {
	TaskID          uint64 `json:"task_id"`
	Title           string `json:"title"`
	Description     string `json:"description"`
	TaskType        string `json:"task_type"`
	Phase           string `json:"phase"`
	PhaseGoal       string `json:"phase_goal"`
	DurationMinutes int32  `json:"duration_minutes"`
	Priority        string `json:"priority"`
	Status          string `json:"status"`
	SortOrder       int32  `json:"sort_order"`
	Source          string `json:"source"`
	SourceLabel     string `json:"source_label"`
	Reason          string `json:"reason"`
	IsNew           bool   `json:"is_new"`
}

// PlanAdjustmentPreviewRemoval 调整预览中的待删除任务摘要。
type PlanAdjustmentPreviewRemoval struct {
	TaskID    uint64 `json:"task_id"`
	Title     string `json:"title"`
	Phase     string `json:"phase"`
	SortOrder int32  `json:"sort_order"`
}

// PlanAdjustmentPreviewReorder 调整预览中的重排任务摘要。
type PlanAdjustmentPreviewReorder struct {
	TaskID        uint64 `json:"task_id"`
	Title         string `json:"title"`
	Phase         string `json:"phase"`
	FromSortOrder int32  `json:"from_sort_order"`
	ToSortOrder   int32  `json:"to_sort_order"`
}

// PlanAdjustmentPreviewDetails 保存一次预览生成的结构化 diff 与预览后任务快照。
type PlanAdjustmentPreviewDetails struct {
	Summary      string                         `json:"summary"`
	Add          []*PlanAgentTask               `json:"add"`
	Remove       []PlanAdjustmentPreviewRemoval `json:"remove"`
	Reorder      []PlanAdjustmentPreviewReorder `json:"reorder"`
	PreviewTasks []PlanAdjustmentPreviewTask    `json:"preview_tasks"`
}

// PlanAdjustmentPreviewResult 是预览阶段返回给前端的完整结果。
type PlanAdjustmentPreviewResult struct {
	PreviewToken   string
	AddedCount     int32
	RemovedCount   int32
	ReorderedCount int32
	Summary        string
	Add            []PlanAdjustmentPreviewTask
	Remove         []PlanAdjustmentPreviewRemoval
	Reorder        []PlanAdjustmentPreviewReorder
	PreviewTasks   []PlanAdjustmentPreviewTask
}

// PlanAdjustmentApplyResult 是确认应用后返回的执行结果。
type PlanAdjustmentApplyResult struct {
	Adjustment *PlanAdjustment
	Tasks      []*LearningTask
}

// PlanRepo 学习计划仓库接口，data 层必须实现
type PlanRepo interface {
	// Create 创建学习计划
	Create(ctx context.Context, plan *LearningPlan) error
	// GetByID 根据 ID 查询学习计划
	GetByID(ctx context.Context, id uint64) (*LearningPlan, error)
	// GetByUserID 查询用户当前活跃计划
	GetByUserID(ctx context.Context, userID uint64) (*LearningPlan, error)
	// Update 更新学习计划
	Update(ctx context.Context, plan *LearningPlan) error
	// ListByUserID 分页查询用户计划列表
	ListByUserID(ctx context.Context, userID uint64, page, pageSize int32) ([]*LearningPlan, int64, error)
	// Transaction 在事务中执行多表操作
	Transaction(ctx context.Context, fn func(txCtx context.Context) error) error
}

// TaskRepo 学习任务仓库接口
type TaskRepo interface {
	// BatchCreate 批量创建学习任务
	BatchCreate(ctx context.Context, tasks []*LearningTask) error
	// ListByPlanID 查询计划下所有任务
	ListByPlanID(ctx context.Context, planID uint64) ([]*LearningTask, error)
	// GetByID 根据 ID 获取单个任务
	GetByID(ctx context.Context, id uint64) (*LearningTask, error)
	// Update 更新任务
	Update(ctx context.Context, task *LearningTask) error
	// CountByPlanID 统计计划下任务总数
	CountByPlanID(ctx context.Context, planID uint64) (int32, error)
	// CountCompletedByPlanID 统计计划下已完成任务数
	CountCompletedByPlanID(ctx context.Context, planID uint64) (int32, error)
	// BatchDelete 批量软删除计划下待处理任务
	BatchDelete(ctx context.Context, planID uint64, ids []uint64) (int64, error)
	// BatchUpdateSortOrder 批量更新计划下任务排序
	BatchUpdateSortOrder(ctx context.Context, planID uint64, updates map[uint64]int32) (int64, error)
}

// PlanAgentClient AI 服务客户端接口
type PlanAgentClient interface {
	// GeneratePlan 调用 AI Gateway 的 PlanAgent RPC 生成计划
	GeneratePlan(ctx context.Context, req *PlanAgentRequest) (*PlanAgentResponse, error)
	// AdjustPlan 调用 AI Gateway 生成任务调整方案
	AdjustPlan(ctx context.Context, req *PlanAgentAdjustRequest) (*PlanAgentAdjustResponse, error)
}

// MQPublisher MQ 消息发布接口
type MQPublisher interface {
	// PublishPlanGenerate 发布计划生成 MQ 消息
	PublishPlanGenerate(ctx context.Context, planID, userID uint64, req *CreatePlanRequest) error
	// PublishFeedbackDiagnosis 发布反馈诊断 MQ 消息
	PublishFeedbackDiagnosis(ctx context.Context, feedbackID, planID, taskID, userID uint64, feedbackText, difficultyFeeling string, problemAreas []string) error
}

// TaskFeedback 任务反馈实体（FIX P3: 添加 DeletedAt 支持软删除）
type TaskFeedback struct {
	ID                    uint64         `gorm:"primaryKey;autoIncrement"`
	PlanID                uint64         `gorm:"index;not null"`
	TaskID                uint64         `gorm:"index;not null"`
	UserID                uint64         `gorm:"index;not null"`
	DifficultyFeeling     string         `gorm:"size:20;not null"`
	FeedbackText          string         `gorm:"size:2000"`
	ActualDurationMinutes int32          `gorm:"not null;default:0"`
	ProblemAreasJSON      string         `gorm:"type:jsonb"`
	DiagnosisJSON         string         `gorm:"type:jsonb"`
	DiagnosisStatus       string         `gorm:"size:20;not null;default:'pending'"`
	CreatedAt             time.Time      `gorm:"not null;autoCreateTime"`
	UpdatedAt             time.Time      `gorm:"not null;autoUpdateTime"`
	DeletedAt             gorm.DeletedAt `gorm:"index"`
}

// TableName 指定任务反馈表名
func (TaskFeedback) TableName() string {
	return "task_feedbacks"
}

// TaskFeedbackRepo 任务反馈仓库接口
type TaskFeedbackRepo interface {
	// Create 创建反馈记录
	Create(ctx context.Context, feedback *TaskFeedback) error
	// GetByID 根据 ID 获取反馈记录
	GetByID(ctx context.Context, id uint64) (*TaskFeedback, error)
	// Update 更新反馈记录
	Update(ctx context.Context, feedback *TaskFeedback) error
	// ListByPlanID 查询计划下所有反馈记录
	ListByPlanID(ctx context.Context, planID uint64) ([]*TaskFeedback, error)
}

// PlanAdjustmentRepo 计划调整记录仓库接口
type PlanAdjustmentRepo interface {
	// Create 创建调整记录
	Create(ctx context.Context, adjustment *PlanAdjustment) error
}

// PlanAdjustmentPreviewRepo 调整预览仓库接口。
type PlanAdjustmentPreviewRepo interface {
	Create(ctx context.Context, preview *PlanAdjustmentPreview) error
	GetByToken(ctx context.Context, token string) (*PlanAdjustmentPreview, error)
	MarkApplied(ctx context.Context, previewID uint64) error
}

// PlanAdjustment 计划调整记录实体
type PlanAdjustment struct {
	ID             uint64    `gorm:"primaryKey;autoIncrement"`
	PlanID         uint64    `gorm:"index;not null"`
	Reason         string    `gorm:"size:500"`
	AddedCount     int32     `gorm:"not null;default:0"`
	RemovedCount   int32     `gorm:"not null;default:0"`
	ReorderedCount int32     `gorm:"not null;default:0"`
	Summary        string    `gorm:"size:1000"`
	DetailsJSON    string    `gorm:"type:text"`
	CreatedAt      time.Time `gorm:"not null;autoCreateTime"`
}

// TableName 指定计划调整记录表名
func (PlanAdjustment) TableName() string {
	return "plan_adjustments"
}

// PlanAdjustmentPreview 调整预览持久化实体，保存待确认的 AI 调整方案。
type PlanAdjustmentPreview struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement"`
	Token       string    `gorm:"size:64;uniqueIndex;not null"`
	UserID      uint64    `gorm:"index;not null"`
	PlanID      uint64    `gorm:"index;not null"`
	Reason      string    `gorm:"size:500"`
	Status      string    `gorm:"size:20;not null;default:'pending'"`
	Summary     string    `gorm:"size:1000"`
	DetailsJSON string    `gorm:"type:text;not null"`
	ExpiresAt   time.Time `gorm:"index;not null"`
	AppliedAt   *time.Time
	CreatedAt   time.Time      `gorm:"not null;autoCreateTime"`
	UpdatedAt   time.Time      `gorm:"not null;autoUpdateTime"`
	DeletedAt   gorm.DeletedAt `gorm:"index"`
}

// TableName 指定调整预览表名。
func (PlanAdjustmentPreview) TableName() string {
	return "plan_adjustment_previews"
}

// SubmitFeedbackBizRequest 提交反馈的业务请求
type SubmitFeedbackBizRequest struct {
	DifficultyFeeling     string
	FeedbackText          string
	ActualDurationMinutes int32
	ProblemAreas          []string
}

// DiagnosisClient AI 诊断分析客户端接口
type DiagnosisClient interface {
	// Analyze 调用 AI Gateway 的 QuizAnalyzer 执行诊断分析
	Analyze(ctx context.Context, task *LearningTask, feedbackText, difficultyFeeling string, problemAreas []string) (string, error)
}

// PlanUseCase 学习计划业务用例
type PlanUseCase struct {
	repo                  PlanRepo
	taskRepo              TaskRepo
	feedbackRepo          TaskFeedbackRepo
	adjustmentRepo        PlanAdjustmentRepo
	adjustmentPreviewRepo PlanAdjustmentPreviewRepo
	industryRepo          IndustryRepo
	aiClient              PlanAgentClient
	diagClient            DiagnosisClient
	publisher             MQPublisher
	archiveClient         LearningArchiveClient
	logger                *log.Helper
}

// LearningArchiveClient 学习档案 gRPC 客户端接口。
type LearningArchiveClient interface {
	WritePlanFeedback(ctx context.Context, entry *PlanFeedbackArchiveEntry) error
	// GetWeakTopics 读取用户高频薄弱主题，供计划生成/调整消费画像。
	GetWeakTopics(ctx context.Context, userID uint64) ([]string, error)
}

// PlanFeedbackArchiveEntry 计划反馈诊断写入学习档案的参数。
type PlanFeedbackArchiveEntry struct {
	UserID          uint64
	FeedbackID      uint64
	IndustryCode    string
	PlanPhase       string
	PlanPhaseGoal   string
	EntryPhase      string
	TaskPhase       string
	TaskPhaseGoal   string
	Language        string
	MistakeTags     []string
	Suggestions     []string
	EvidenceSummary string
	OccurredAt      time.Time
}

// IndustryRepo 行业仓储接口，用于 code→id 解析
type IndustryRepo interface {
	GetIDByCode(ctx context.Context, code string) (uint64, error)
}

// NewPlanUseCase 创建学习计划业务用例
func NewPlanUseCase(repo PlanRepo, taskRepo TaskRepo, feedbackRepo TaskFeedbackRepo, adjustmentRepo PlanAdjustmentRepo, adjustmentPreviewRepo PlanAdjustmentPreviewRepo, industryRepo IndustryRepo, aiClient PlanAgentClient, diagClient DiagnosisClient, publisher MQPublisher, archiveClient LearningArchiveClient, logger log.Logger) *PlanUseCase {
	return &PlanUseCase{
		repo:                  repo,
		taskRepo:              taskRepo,
		feedbackRepo:          feedbackRepo,
		adjustmentRepo:        adjustmentRepo,
		adjustmentPreviewRepo: adjustmentPreviewRepo,
		industryRepo:          industryRepo,
		aiClient:              aiClient,
		diagClient:            diagClient,
		publisher:             publisher,
		archiveClient:         archiveClient,
		logger:                log.NewHelper(logger),
	}
}

// CalculatePlanProgress 计算学习计划进度百分比。
func CalculatePlanProgress(completedTasks, totalTasks int32) float32 {
	if totalTasks <= 0 {
		return 0
	}
	return float32(completedTasks) * 100 / float32(totalTasks)
}

// CreatePlan 创建学习计划并发布异步生成消息
func (uc *PlanUseCase) CreatePlan(ctx context.Context, req *CreatePlanRequest) (*LearningPlan, error) {
	validLevels := map[string]bool{"beginner": true, "intermediate": true, "advanced": true}
	if req.IndustryCode == "" {
		return nil, ErrIndustryRequired
	}
	if req.Goal == "" {
		return nil, ErrGoalRequired
	}
	if req.Level == "" {
		req.Level = "intermediate"
	}
	if !validLevels[req.Level] {
		return nil, ErrInvalidLevel
	}
	if req.DurationDays <= 0 {
		req.DurationDays = 30
	}
	if req.DurationDays < 7 || req.DurationDays > 365 {
		return nil, ErrInvalidDuration
	}
	if req.DailyStudyMinutes <= 0 {
		req.DailyStudyMinutes = 60
	}
	if req.DailyStudyMinutes < 15 || req.DailyStudyMinutes > 480 {
		return nil, ErrInvalidDuration
	}
	if req.DailyHours <= 0 {
		req.DailyHours = int32(math.Ceil(float64(req.DailyStudyMinutes) / 60))
	}

	// 解析 industry code → industry_id
	industryID, err := uc.industryRepo.GetIDByCode(ctx, req.IndustryCode)
	if err != nil {
		return nil, kratosErr.BadRequest("INVALID_INDUSTRY", fmt.Sprintf("行业代码 %s 无效: %v", req.IndustryCode, err))
	}

	plan := &LearningPlan{
		UserID:      req.UserID,
		IndustryID:  industryID,
		Title:       fmt.Sprintf("%s 学习计划", req.IndustryCode),
		Description: req.Goal,
		Status:      "generating",
		// 运行期字段保留，用于 AI 生成
		Level:             req.Level,
		DurationDays:      req.DurationDays,
		DailyStudyMinutes: req.DailyStudyMinutes,
		Industry:          req.IndustryCode,
	}
	if err := uc.repo.Create(ctx, plan); err != nil {
		return nil, kratosErr.InternalServer("CREATE_PLAN_FAILED", "创建学习计划失败").WithCause(err)
	}

	if err := uc.publisher.PublishPlanGenerate(ctx, plan.ID, req.UserID, req); err != nil {
		uc.logger.Errorf("发布计划生成消息失败: plan_id=%d err=%v", plan.ID, err)
	}
	return plan, nil
}

// GeneratePlan 消费 MQ 消息后调用 AI 生成计划并原子落库。
func (uc *PlanUseCase) GeneratePlan(ctx context.Context, planID, userID uint64, req *CreatePlanRequest) error {
	// 合并学习档案高频薄弱主题，让计划生成贴合用户真实画像（降级：失败只用表单弱项）。
	weakTopics := make([]string, 0, len(req.WeakTopics)+8)
	weakTopics = append(weakTopics, req.WeakTopics...)
	if uc.archiveClient != nil {
		if topics, archErr := uc.archiveClient.GetWeakTopics(ctx, userID); archErr == nil {
			weakTopics = append(weakTopics, topics...)
		} else {
			uc.logger.Warnf("读取档案弱项失败，回退表单弱项: plan_id=%d err=%v", planID, archErr)
		}
	}
	aiResp, err := uc.aiClient.GeneratePlan(ctx, &PlanAgentRequest{
		PlanID:            planID,
		UserID:            userID,
		IndustryCode:      req.IndustryCode,
		Goal:              req.Goal,
		DailyHours:        req.DailyHours,
		WeakTopics:        weakTopics,
		Level:             req.Level,
		DurationDays:      req.DurationDays,
		DailyStudyMinutes: req.DailyStudyMinutes,
	})
	if err != nil {
		return ErrPlanGenerateFailed.WithCause(err)
	}

	return uc.repo.Transaction(ctx, func(txCtx context.Context) error {
		plan, txErr := uc.repo.GetByID(txCtx, planID)
		if txErr != nil {
			return txErr
		}
		if plan.UserID != userID {
			return ErrPlanAccessDenied
		}

		existingCount, txErr := uc.taskRepo.CountByPlanID(txCtx, planID)
		if txErr != nil {
			return txErr
		}
		if existingCount > 0 {
			plan.TotalTasks = existingCount
			plan.Status = "active"
			return uc.repo.Update(txCtx, plan)
		}

		if aiResp.PlanTitle != "" {
			plan.Title = aiResp.PlanTitle
		}
		if plan.Description == "" && aiResp.Summary != "" {
			plan.Description = aiResp.Summary
		}

		tasks := make([]*LearningTask, 0, len(aiResp.Tasks))
		for idx, task := range aiResp.Tasks {
			sortOrder := task.SortOrder
			if sortOrder <= 0 {
				sortOrder = int32(idx + 1)
			}
			dayNumber := task.DayNumber
			if dayNumber <= 0 {
				dayNumber = int32(idx + 1)
			}
			taskType := task.TaskType
			if taskType == "" {
				taskType = "study"
			}
			priority := task.Priority
			if priority == "" {
				priority = "medium"
			}
			durationMinutes := task.DurationMinutes
			if durationMinutes <= 0 {
				durationMinutes = 30
			}
			tasks = append(tasks, &LearningTask{
				PlanID:          planID,
				Title:           task.Title,
				Description:     task.Description,
				TaskType:        taskType,
				Phase:           task.Phase,
				DayNumber:       dayNumber,
				DurationMinutes: durationMinutes,
				Priority:        priority,
				Status:          "pending",
				SortOrder:       sortOrder,
				Source:          "ai_generated",
				SourceLabel:     buildTaskSourceLabel(taskType, task.Phase),
				Reason:          buildTaskReason(task.Title, task.Phase),
			})
		}

		if len(tasks) > 0 {
			if txErr := uc.taskRepo.BatchCreate(txCtx, tasks); txErr != nil {
				return txErr
			}
		}

		plan.TotalTasks = int32(len(tasks))
		plan.Status = "active"
		plan.Phase = CalculatePhase(0, plan.TotalTasks)
		if goal, ok := PhaseGoalMap[plan.Phase]; ok {
			plan.PhaseGoal = goal
		}
		return uc.repo.Update(txCtx, plan)
	})
}

// MarkPlanGenerateFailed 在消息最终失败后将计划标记为失败。
func (uc *PlanUseCase) MarkPlanGenerateFailed(ctx context.Context, planID uint64) error {
	plan, err := uc.repo.GetByID(ctx, planID)
	if err != nil {
		return err
	}
	if plan.Status == "active" || plan.Status == "completed" {
		return nil
	}
	plan.Status = "failed"
	return uc.repo.Update(ctx, plan)
}

// GetPlanWithTasks 获取计划详情并校验用户归属。
func (uc *PlanUseCase) GetPlanWithTasks(ctx context.Context, userID, planID uint64) (*LearningPlan, []*LearningTask, error) {
	plan, err := uc.repo.GetByID(ctx, planID)
	if err != nil {
		return nil, nil, ErrPlanNotFound
	}
	if plan.UserID != userID {
		return nil, nil, ErrPlanAccessDenied
	}
	tasks, err := uc.taskRepo.ListByPlanID(ctx, planID)
	if err != nil {
		return nil, nil, kratosErr.InternalServer("LIST_TASKS_FAILED", "获取任务列表失败").WithCause(err)
	}
	return plan, tasks, nil
}

// GetCurrentPlanWithTasks 获取用户当前活跃计划详情。
func (uc *PlanUseCase) GetCurrentPlanWithTasks(ctx context.Context, userID uint64) (*LearningPlan, []*LearningTask, error) {
	plan, err := uc.repo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, nil, ErrNoActivePlan
	}
	tasks, err := uc.taskRepo.ListByPlanID(ctx, plan.ID)
	if err != nil {
		return nil, nil, kratosErr.InternalServer("LIST_TASKS_FAILED", "获取任务列表失败").WithCause(err)
	}
	return plan, tasks, nil
}

// ListPlansWithProgress 分页查询计划列表。
func (uc *PlanUseCase) ListPlansWithProgress(ctx context.Context, userID uint64, page, pageSize int32) ([]*LearningPlan, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 10
	}
	return uc.repo.ListByUserID(ctx, userID, page, pageSize)
}

// UpdateTaskStatus 原子更新任务状态并同步计划进度。
func (uc *PlanUseCase) UpdateTaskStatus(ctx context.Context, userID, planID, taskID uint64, newStatus string) (*LearningTask, *LearningPlan, error) {
	validStatuses := map[string]bool{"in_progress": true, "completed": true, "skipped": true}
	if !validStatuses[newStatus] {
		return nil, nil, ErrInvalidStatus
	}

	var updatedTask *LearningTask
	var updatedPlan *LearningPlan
	err := uc.repo.Transaction(ctx, func(txCtx context.Context) error {
		plan, txErr := uc.repo.GetByID(txCtx, planID)
		if txErr != nil {
			return ErrPlanNotFound
		}
		if plan.UserID != userID {
			return ErrPlanAccessDenied
		}

		task, txErr := uc.taskRepo.GetByID(txCtx, taskID)
		if txErr != nil {
			return ErrTaskNotFound
		}
		if task.PlanID != planID {
			return ErrTaskNotBelong
		}

		validTransitions := map[string]map[string]bool{
			"pending": {
				"in_progress": true,
				"completed":   true,
				"skipped":     true,
			},
			"in_progress": {
				"completed": true,
			},
		}
		nextMap, ok := validTransitions[task.Status]
		if !ok || !nextMap[newStatus] {
			return ErrStatusTransition
		}

		task.Status = newStatus
		if newStatus == "completed" {
			now := time.Now()
			task.CompletedAt = &now
		} else {
			task.CompletedAt = nil
		}
		if txErr := uc.taskRepo.Update(txCtx, task); txErr != nil {
			return kratosErr.InternalServer("UPDATE_TASK_FAILED", "更新任务失败").WithCause(txErr)
		}

		completedCount, txErr := uc.taskRepo.CountCompletedByPlanID(txCtx, planID)
		if txErr != nil {
			return kratosErr.InternalServer("COUNT_TASKS_FAILED", "统计任务失败").WithCause(txErr)
		}
		totalTasks, txErr := uc.taskRepo.CountByPlanID(txCtx, planID)
		if txErr != nil {
			return kratosErr.InternalServer("COUNT_TASKS_FAILED", "统计任务失败").WithCause(txErr)
		}

		plan.CompletedTasks = completedCount
		plan.TotalTasks = totalTasks
		plan.Phase = CalculatePhase(completedCount, totalTasks)
		if goal, ok := PhaseGoalMap[plan.Phase]; ok {
			plan.PhaseGoal = goal
		}
		if totalTasks > 0 && completedCount >= totalTasks {
			plan.Status = "completed"
		} else if plan.Status == "completed" {
			plan.Status = "active"
		}
		if txErr := uc.repo.Update(txCtx, plan); txErr != nil {
			return kratosErr.InternalServer("UPDATE_PLAN_FAILED", "更新计划失败").WithCause(txErr)
		}

		updatedTask = task
		updatedPlan = plan
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return updatedTask, updatedPlan, nil
}

// SubmitTaskFeedback 提交任务反馈并异步触发诊断。
func (uc *PlanUseCase) SubmitTaskFeedback(ctx context.Context, userID, planID, taskID uint64, req *SubmitFeedbackBizRequest) (*TaskFeedback, error) {
	task, err := uc.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, ErrTaskNotFound
	}
	if task.PlanID != planID {
		return nil, ErrTaskNotBelong
	}
	if task.Status != "completed" {
		return nil, ErrFeedbackTaskStatus
	}

	plan, err := uc.repo.GetByID(ctx, planID)
	if err != nil {
		return nil, ErrPlanNotFound
	}
	if plan.UserID != userID {
		return nil, ErrPlanAccessDenied
	}

	problemAreasJSON := "[]"
	if len(req.ProblemAreas) > 0 {
		payload, marshalErr := json.Marshal(req.ProblemAreas)
		if marshalErr == nil {
			problemAreasJSON = string(payload)
		}
	}

	feedback := &TaskFeedback{
		PlanID:                planID,
		TaskID:                taskID,
		UserID:                userID,
		DifficultyFeeling:     req.DifficultyFeeling,
		FeedbackText:          req.FeedbackText,
		ActualDurationMinutes: req.ActualDurationMinutes,
		ProblemAreasJSON:      problemAreasJSON,
		DiagnosisStatus:       "pending",
	}
	if err := uc.feedbackRepo.Create(ctx, feedback); err != nil {
		return nil, kratosErr.InternalServer("CREATE_FEEDBACK_FAILED", "创建反馈失败").WithCause(err)
	}

	if err := uc.publisher.PublishFeedbackDiagnosis(ctx, feedback.ID, planID, taskID, userID, req.FeedbackText, req.DifficultyFeeling, req.ProblemAreas); err != nil {
		uc.logger.Errorf("发布诊断消息失败: feedback_id=%d err=%v", feedback.ID, err)
	}
	return feedback, nil
}

// ProcessFeedbackDiagnosis 处理诊断消息并保存诊断结果，并同步写入学习档案。
func (uc *PlanUseCase) ProcessFeedbackDiagnosis(ctx context.Context, feedbackID uint64, feedbackText, difficultyFeeling string, problemAreas []string) error {
	feedback, err := uc.feedbackRepo.GetByID(ctx, feedbackID)
	if err != nil {
		return ErrFeedbackNotFound
	}
	if feedback.DiagnosisStatus == "completed" && feedback.DiagnosisJSON != "" {
		return nil
	}

	task, err := uc.taskRepo.GetByID(ctx, feedback.TaskID)
	if err != nil {
		return ErrTaskNotFound
	}

	diagnosisJSON, err := uc.diagClient.Analyze(ctx, task, feedbackText, difficultyFeeling, problemAreas)
	if err != nil {
		return ErrDiagnosisFailed.WithCause(err)
	}

	feedback.DiagnosisJSON = diagnosisJSON
	feedback.DiagnosisStatus = "completed"
	if err := uc.feedbackRepo.Update(ctx, feedback); err != nil {
		return kratosErr.InternalServer("SAVE_DIAGNOSIS_FAILED", "保存诊断结果失败").WithCause(err)
	}

	uc.syncPlanFeedbackArchive(ctx, feedback, task, diagnosisJSON)
	return nil
}

// syncPlanFeedbackArchive 将反馈诊断结果同步写入学习档案（通过 gRPC）。
func (uc *PlanUseCase) syncPlanFeedbackArchive(ctx context.Context, feedback *TaskFeedback, task *LearningTask, diagnosisJSON string) {
	if uc.archiveClient == nil {
		return
	}

	var result struct {
		Score       float64  `json:"score"`
		IsCorrect   bool     `json:"is_correct"`
		Feedback    string   `json:"feedback"`
		KeyPoints   []string `json:"key_points"`
		Suggestions string   `json:"suggestions"`
	}
	if err := json.Unmarshal([]byte(diagnosisJSON), &result); err != nil {
		uc.logger.Warnf("解析诊断结果 JSON 失败: %v", err)
		return
	}

	mistakeTags := normalizeFeedbackMistakeTags(result.KeyPoints, result.IsCorrect)
	if len(mistakeTags) == 0 {
		return
	}

	suggestions := []string{}
	if result.Suggestions != "" {
		suggestions = []string{result.Suggestions}
	}

	plan, _ := uc.repo.GetByID(ctx, feedback.PlanID)
	industryCode := ""
	planPhase := ""
	planPhaseGoal := ""
	entryPhase := ""
	if plan != nil {
		industryCode = fmt.Sprintf("%d", plan.IndustryID)
		planPhase = plan.Phase
		planPhaseGoal = plan.PhaseGoal
		entryPhase = plan.EntryPhase
	}

	taskPhase := ""
	taskPhaseGoal := ""
	if task != nil {
		taskPhase = task.Phase
		taskPhaseGoal = task.PhaseGoal
	}

	occurredAt := feedback.CreatedAt
	if occurredAt.IsZero() {
		occurredAt = time.Now()
	}

	entry := &PlanFeedbackArchiveEntry{
		UserID:          feedback.UserID,
		FeedbackID:      feedback.ID,
		IndustryCode:    industryCode,
		PlanPhase:       planPhase,
		PlanPhaseGoal:   planPhaseGoal,
		EntryPhase:      entryPhase,
		TaskPhase:       taskPhase,
		TaskPhaseGoal:   taskPhaseGoal,
		MistakeTags:     mistakeTags,
		Suggestions:     suggestions,
		EvidenceSummary: result.Feedback,
		OccurredAt:      occurredAt,
	}

	if err := uc.archiveClient.WritePlanFeedback(ctx, entry); err != nil {
		uc.logger.Warnf("同步计划反馈学习档案失败: feedback_id=%d, err=%v", feedback.ID, err)
	}
}

// normalizeFeedbackMistakeTags 从诊断关键点中推断错因标签。
func normalizeFeedbackMistakeTags(keyPoints []string, isCorrect bool) []string {
	if isCorrect {
		return nil
	}
	result := make([]string, 0, len(keyPoints))
	for _, kp := range keyPoints {
		kp = strings.TrimSpace(kp)
		if kp == "" {
			continue
		}
		switch {
		case strings.Contains(kp, "边界"):
			result = appendUniqueStr(result, "边界条件生疏")
		case strings.Contains(kp, "复杂度"):
			result = appendUniqueStr(result, "复杂度意识薄弱")
		case strings.Contains(kp, "状态") || strings.Contains(kp, "定义"):
			result = appendUniqueStr(result, "状态定义不清")
		case strings.Contains(kp, "循环") || strings.Contains(kp, "索引"):
			result = appendUniqueStr(result, "循环/索引控制不稳")
		case strings.Contains(kp, "数据结构"):
			result = appendUniqueStr(result, "数据结构选择不当")
		case strings.Contains(kp, "调试"):
			result = appendUniqueStr(result, "调试路径混乱")
		case strings.Contains(kp, "实现") || strings.Contains(kp, "不完整"):
			result = appendUniqueStr(result, "代码实现不完整")
		default:
			result = appendUniqueStr(result, "状态定义不清")
		}
	}
	return result
}

func appendUniqueStr(existing []string, s string) []string {
	for _, e := range existing {
		if e == s {
			return existing
		}
	}
	return append(existing, s)
}

// isMissingTaskFeedbackTableError 判断是否为 task_feedbacks 表未创建导致的可降级查询错误。
func isMissingTaskFeedbackTableError(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "task_feedbacks") && strings.Contains(message, "SQLSTATE 42P01")
}

// isPlanAdjustParseFailure 判断调整计划是否因 AI 结构化输出解析失败而触发可降级错误。
func isPlanAdjustParseFailure(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "结果解析失败") || strings.Contains(message, "PARSE_FAILED")
}

// MarkFeedbackDiagnosisFailed 在消息最终失败后将反馈诊断标记为失败。
func (uc *PlanUseCase) MarkFeedbackDiagnosisFailed(ctx context.Context, feedbackID uint64) error {
	feedback, err := uc.feedbackRepo.GetByID(ctx, feedbackID)
	if err != nil {
		return err
	}
	if feedback.DiagnosisStatus == "completed" {
		return nil
	}
	feedback.DiagnosisStatus = "failed"
	return uc.feedbackRepo.Update(ctx, feedback)
}

// PreviewAdjustPlan 生成待确认的调整预览，并冻结结构化 diff 供前端确认。
func (uc *PlanUseCase) PreviewAdjustPlan(ctx context.Context, userID, planID uint64, reason string) (*PlanAdjustmentPreviewResult, error) {
	return uc.previewAdjustPlan(ctx, userID, planID, reason, false)
}

// ApplyAdjustPlan 根据预览令牌执行一次已冻结的调整方案并落库。
func (uc *PlanUseCase) ApplyAdjustPlan(ctx context.Context, userID, planID uint64, previewToken string) (*PlanAdjustmentApplyResult, error) {
	if strings.TrimSpace(previewToken) == "" {
		return nil, ErrAdjustPreviewNotFound
	}
	if uc.adjustmentPreviewRepo == nil {
		return nil, ErrAdjustFailed.WithCause(fmt.Errorf("adjustment preview repo is nil"))
	}

	preview, err := uc.adjustmentPreviewRepo.GetByToken(ctx, strings.TrimSpace(previewToken))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrAdjustPreviewNotFound
		}
		return nil, ErrAdjustFailed.WithCause(err)
	}
	if preview.UserID != userID || preview.PlanID != planID {
		return nil, ErrAdjustPreviewMismatch
	}
	if preview.Status == "applied" {
		return nil, ErrAdjustPreviewApplied
	}
	if preview.ExpiresAt.Before(time.Now()) {
		return nil, ErrAdjustPreviewExpired
	}

	plan, err := uc.repo.GetByID(ctx, planID)
	if err != nil {
		return nil, ErrPlanNotFound
	}
	if plan.UserID != userID {
		return nil, ErrPlanAccessDenied
	}
	if plan.Status == "completed" {
		return nil, ErrPlanCompleted
	}

	var details PlanAdjustmentPreviewDetails
	if err := json.Unmarshal([]byte(preview.DetailsJSON), &details); err != nil {
		return nil, ErrAdjustFailed.WithCause(err)
	}

	adjustment := &PlanAdjustment{
		PlanID:  planID,
		Reason:  preview.Reason,
		Summary: details.Summary,
	}
	err = uc.repo.Transaction(ctx, func(txCtx context.Context) error {
		addedTasks := buildAdjustmentAddedTasks(planID, preview.Reason, details.Add)
		if len(addedTasks) > 0 {
			if txErr := uc.taskRepo.BatchCreate(txCtx, addedTasks); txErr != nil {
				return txErr
			}
		}

		removeIDs := make([]uint64, 0, len(details.Remove))
		for _, item := range details.Remove {
			removeIDs = append(removeIDs, item.TaskID)
		}
		removedCount, txErr := uc.taskRepo.BatchDelete(txCtx, planID, removeIDs)
		if txErr != nil {
			return txErr
		}

		reorderMap := make(map[uint64]int32, len(details.Reorder))
		for _, item := range details.Reorder {
			reorderMap[item.TaskID] = item.ToSortOrder
		}
		reorderedCount, txErr := uc.taskRepo.BatchUpdateSortOrder(txCtx, planID, reorderMap)
		if txErr != nil {
			return txErr
		}

		totalTasks, txErr := uc.taskRepo.CountByPlanID(txCtx, planID)
		if txErr != nil {
			return txErr
		}
		completedTasks, txErr := uc.taskRepo.CountCompletedByPlanID(txCtx, planID)
		if txErr != nil {
			return txErr
		}

		plan.TotalTasks = totalTasks
		plan.CompletedTasks = completedTasks
		if totalTasks > 0 && completedTasks >= totalTasks {
			plan.Status = "completed"
		} else {
			plan.Status = "active"
		}
		if txErr := uc.repo.Update(txCtx, plan); txErr != nil {
			return txErr
		}

		detailsBytes, txErr := json.Marshal(details)
		if txErr != nil {
			return txErr
		}
		adjustment.AddedCount = int32(len(addedTasks))
		adjustment.RemovedCount = int32(removedCount)
		adjustment.ReorderedCount = int32(reorderedCount)
		adjustment.DetailsJSON = string(detailsBytes)
		if txErr := uc.adjustmentRepo.Create(txCtx, adjustment); txErr != nil {
			return txErr
		}
		return uc.adjustmentPreviewRepo.MarkApplied(txCtx, preview.ID)
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrAdjustPreviewApplied
		}
		return nil, ErrAdjustFailed.WithCause(err)
	}

	_, tasks, err := uc.GetPlanWithTasks(ctx, userID, planID)
	if err != nil {
		return nil, err
	}
	uc.logger.Infof("计划调整应用完成: plan_id=%d preview_token=%s added=%d removed=%d reordered=%d", planID, preview.Token, adjustment.AddedCount, adjustment.RemovedCount, adjustment.ReorderedCount)
	return &PlanAdjustmentApplyResult{Adjustment: adjustment, Tasks: tasks}, nil
}

// previewAdjustPlan 生成调整预览，兼容旧直通接口时可选择对解析失败做空调整降级。
func (uc *PlanUseCase) previewAdjustPlan(ctx context.Context, userID, planID uint64, reason string, allowParseFailure bool) (*PlanAdjustmentPreviewResult, error) {
	if uc.adjustmentPreviewRepo == nil {
		return nil, ErrAdjustFailed.WithCause(fmt.Errorf("adjustment preview repo is nil"))
	}

	plan, err := uc.repo.GetByID(ctx, planID)
	if err != nil {
		return nil, ErrPlanNotFound
	}
	if plan.UserID != userID {
		return nil, ErrPlanAccessDenied
	}
	if plan.Status == "completed" {
		return nil, ErrPlanCompleted
	}

	tasks, err := uc.taskRepo.ListByPlanID(ctx, planID)
	if err != nil {
		return nil, kratosErr.InternalServer("LIST_TASKS_FAILED", "获取任务列表失败").WithCause(err)
	}
	feedbacks, err := uc.feedbackRepo.ListByPlanID(ctx, planID)
	if err != nil {
		if isMissingTaskFeedbackTableError(err) {
			uc.logger.Warnf("task_feedbacks 表缺失，AdjustPlan 降级为空反馈继续执行: plan_id=%d err=%v", planID, err)
			feedbacks = []*TaskFeedback{}
		} else {
			return nil, kratosErr.InternalServer("LIST_FEEDBACKS_FAILED", "获取反馈列表失败").WithCause(err)
		}
	}

	// 读取学习档案高频薄弱主题，让计划调整基于真实画像（降级：失败则只用本地反馈弱项）。
	var extraWeakTopics []string
	if uc.archiveClient != nil {
		if topics, archErr := uc.archiveClient.GetWeakTopics(ctx, userID); archErr == nil {
			extraWeakTopics = topics
		} else {
			uc.logger.Warnf("读取档案弱项失败，回退本地反馈: plan_id=%d err=%v", planID, archErr)
		}
	}

	aiResp, err := uc.aiClient.AdjustPlan(ctx, &PlanAgentAdjustRequest{
		Plan:            plan,
		CurrentTasks:    tasks,
		Feedbacks:       feedbacks,
		Reason:          reason,
		ExtraWeakTopics: extraWeakTopics,
	})
	if err != nil {
		if allowParseFailure && isPlanAdjustParseFailure(err) {
			uc.logger.Warnf("AI 调整计划解析失败，降级为保留原计划: plan_id=%d err=%v", planID, err)
			aiResp = &PlanAgentAdjustResponse{
				Add:     []*PlanAgentTask{},
				Remove:  []uint64{},
				Reorder: map[uint64]int32{},
				Summary: "本次未生成稳定的调整结果，已暂时保留原计划内容，请稍后重试。",
			}
		} else {
			return nil, ErrAdjustFailed.WithCause(err)
		}
	}

	details := buildAdjustmentPreviewDetails(reason, tasks, aiResp)
	detailsBytes, err := json.Marshal(details)
	if err != nil {
		return nil, ErrAdjustFailed.WithCause(err)
	}

	preview := &PlanAdjustmentPreview{
		Token:       uuid.NewString(),
		UserID:      userID,
		PlanID:      planID,
		Reason:      reason,
		Status:      "pending",
		Summary:     details.Summary,
		DetailsJSON: string(detailsBytes),
		ExpiresAt:   time.Now().Add(15 * time.Minute),
	}
	if err := uc.adjustmentPreviewRepo.Create(ctx, preview); err != nil {
		return nil, ErrAdjustFailed.WithCause(err)
	}

	return &PlanAdjustmentPreviewResult{
		PreviewToken:   preview.Token,
		AddedCount:     int32(len(details.Add)),
		RemovedCount:   int32(len(details.Remove)),
		ReorderedCount: int32(len(details.Reorder)),
		Summary:        details.Summary,
		Add:            buildPreviewTaskList(reason, details.Add),
		Remove:         details.Remove,
		Reorder:        details.Reorder,
		PreviewTasks:   details.PreviewTasks,
	}, nil
}

// AdjustPlan 兼容旧入口：内部先生成预览，再立即按同一份预览执行落库。
func (uc *PlanUseCase) AdjustPlan(ctx context.Context, userID, planID uint64, reason string) (*PlanAdjustment, error) {
	preview, err := uc.previewAdjustPlan(ctx, userID, planID, reason, true)
	if err != nil {
		return nil, err
	}
	applied, err := uc.ApplyAdjustPlan(ctx, userID, planID, preview.PreviewToken)
	if err != nil {
		return nil, err
	}
	return applied.Adjustment, nil
}

// buildAdjustmentAddedTasks 将预览中的新增任务还原为待落库任务实体。
func buildAdjustmentAddedTasks(planID uint64, reason string, add []*PlanAgentTask) []*LearningTask {
	addedTasks := make([]*LearningTask, 0, len(add))
	for _, task := range add {
		taskType := task.TaskType
		if taskType == "" {
			taskType = "study"
		}
		priority := task.Priority
		if priority == "" {
			priority = "medium"
		}
		durationMinutes := task.DurationMinutes
		if durationMinutes <= 0 {
			durationMinutes = 30
		}
		addedTasks = append(addedTasks, &LearningTask{
			PlanID:          planID,
			Title:           task.Title,
			Description:     task.Description,
			TaskType:        taskType,
			Phase:           task.Phase,
			PhaseGoal:       task.PhaseGoal,
			DayNumber:       task.DayNumber,
			DurationMinutes: durationMinutes,
			Priority:        priority,
			Status:          "pending",
			SortOrder:       task.SortOrder,
			Source:          "plan_adjustment",
			SourceLabel:     "计划调整",
			Reason:          reason,
		})
	}
	return addedTasks
}

// buildAdjustmentPreviewDetails 组装预览摘要、diff 和应用后任务快照。
func buildAdjustmentPreviewDetails(reason string, currentTasks []*LearningTask, aiResp *PlanAgentAdjustResponse) PlanAdjustmentPreviewDetails {
	taskByID := make(map[uint64]*LearningTask, len(currentTasks))
	reorderMap := make(map[uint64]int32, len(aiResp.Reorder))
	for taskID, sortOrder := range aiResp.Reorder {
		reorderMap[taskID] = sortOrder
	}

	removeSet := make(map[uint64]struct{}, len(aiResp.Remove))
	removeItems := make([]PlanAdjustmentPreviewRemoval, 0, len(aiResp.Remove))
	reorderItems := make([]PlanAdjustmentPreviewReorder, 0, len(aiResp.Reorder))
	previewTasks := make([]PlanAdjustmentPreviewTask, 0, len(currentTasks)+len(aiResp.Add))

	for _, task := range currentTasks {
		taskByID[task.ID] = task
	}
	for _, taskID := range aiResp.Remove {
		removeSet[taskID] = struct{}{}
		if task, ok := taskByID[taskID]; ok {
			removeItems = append(removeItems, PlanAdjustmentPreviewRemoval{
				TaskID:    task.ID,
				Title:     task.Title,
				Phase:     task.Phase,
				SortOrder: task.SortOrder,
			})
		}
	}
	for taskID, toSortOrder := range reorderMap {
		if task, ok := taskByID[taskID]; ok && task.SortOrder != toSortOrder {
			reorderItems = append(reorderItems, PlanAdjustmentPreviewReorder{
				TaskID:        task.ID,
				Title:         task.Title,
				Phase:         task.Phase,
				FromSortOrder: task.SortOrder,
				ToSortOrder:   toSortOrder,
			})
		}
	}

	for _, task := range currentTasks {
		if _, removed := removeSet[task.ID]; removed {
			continue
		}
		sortOrder := task.SortOrder
		if nextSortOrder, ok := reorderMap[task.ID]; ok {
			sortOrder = nextSortOrder
		}
		previewTasks = append(previewTasks, PlanAdjustmentPreviewTask{
			TaskID:          task.ID,
			Title:           task.Title,
			Description:     task.Description,
			TaskType:        task.TaskType,
			Phase:           task.Phase,
			PhaseGoal:       task.PhaseGoal,
			DurationMinutes: task.DurationMinutes,
			Priority:        task.Priority,
			Status:          task.Status,
			SortOrder:       sortOrder,
			Source:          task.Source,
			SourceLabel:     task.SourceLabel,
			Reason:          task.Reason,
			IsNew:           false,
		})
	}

	addItems := buildPreviewTaskList(reason, aiResp.Add)
	previewTasks = append(previewTasks, addItems...)
	for i := 0; i < len(previewTasks); i++ {
		for j := i + 1; j < len(previewTasks); j++ {
			if previewTasks[j].SortOrder < previewTasks[i].SortOrder || (previewTasks[j].SortOrder == previewTasks[i].SortOrder && previewTasks[j].TaskID < previewTasks[i].TaskID) {
				previewTasks[i], previewTasks[j] = previewTasks[j], previewTasks[i]
			}
		}
	}

	return PlanAdjustmentPreviewDetails{
		Summary:      aiResp.Summary,
		Add:          aiResp.Add,
		Remove:       removeItems,
		Reorder:      reorderItems,
		PreviewTasks: previewTasks,
	}
}

// buildPreviewTaskList 将新增任务转换为前端可直接渲染的预览列表。
func buildPreviewTaskList(reason string, add []*PlanAgentTask) []PlanAdjustmentPreviewTask {
	items := make([]PlanAdjustmentPreviewTask, 0, len(add))
	for _, task := range add {
		items = append(items, PlanAdjustmentPreviewTask{
			TaskID:          0,
			Title:           task.Title,
			Description:     task.Description,
			TaskType:        task.TaskType,
			Phase:           task.Phase,
			PhaseGoal:       task.PhaseGoal,
			DurationMinutes: task.DurationMinutes,
			Priority:        task.Priority,
			Status:          "pending",
			SortOrder:       task.SortOrder,
			Source:          "plan_adjustment",
			SourceLabel:     "计划调整",
			Reason:          reason,
			IsNew:           true,
		})
	}
	return items
}

// --- 进度统计 ---

// PlanProgressResponse 学习进度统计结果
type PlanProgressResponse struct {
	PlanID          uint64
	TotalTasks      int
	CompletedTasks  int
	SkippedTasks    int
	InProgressTasks int
	PendingTasks    int
	Progress        float64
	DailyProgress   []DailyProgress
	TaskTypeStats   []TaskTypeStat
}

// DailyProgress 每日进度
type DailyProgress struct {
	DayNumber int
	Completed int
	Total     int
}

// TaskTypeStat 任务类型统计
type TaskTypeStat struct {
	TaskType  string
	Completed int
	Total     int
}

// GetProgress 获取学习计划进度统计
func (uc *PlanUseCase) GetProgress(ctx context.Context, userID, planID uint64) (*PlanProgressResponse, error) {
	// 验证计划归属
	plan, err := uc.repo.GetByID(ctx, planID)
	if err != nil {
		return nil, ErrPlanNotFound
	}
	if plan.UserID != userID {
		return nil, ErrPlanAccessDenied
	}

	// 获取所有任务
	tasks, err := uc.taskRepo.ListByPlanID(ctx, planID)
	if err != nil {
		return nil, kratosErr.InternalServer("LIST_TASKS_FAILED", "获取任务列表失败").WithCause(err)
	}

	// 统计各状态任务数
	var completedCount, skippedCount, inProgressCount, pendingCount int
	dailyStats := make(map[int32]*dailyStat)
	taskTypeStats := make(map[string]*taskTypeStatData)

	for _, t := range tasks {
		switch t.Status {
		case "completed":
			completedCount++
		case "skipped":
			skippedCount++
		case "in_progress":
			inProgressCount++
		case "pending":
			pendingCount++
		}

		// 更新每日统计
		ds, ok := dailyStats[t.DayNumber]
		if !ok {
			ds = &dailyStat{}
			dailyStats[t.DayNumber] = ds
		}
		ds.total++
		if t.Status == "completed" {
			ds.completed++
		}

		// 更新任务类型统计
		ts, ok := taskTypeStats[t.TaskType]
		if !ok {
			ts = &taskTypeStatData{}
			taskTypeStats[t.TaskType] = ts
		}
		ts.total++
		if t.Status == "completed" {
			ts.completed++
		}
	}

	// 计算进度
	totalTasks := len(tasks)
	var progress float64
	if totalTasks > 0 {
		progress = float64(completedCount) / float64(totalTasks) * 100
	}

	// 构建每日进度
	dailyProgress := make([]DailyProgress, 0, len(dailyStats))
	for day, stats := range dailyStats {
		dailyProgress = append(dailyProgress, DailyProgress{
			DayNumber: int(day),
			Total:     stats.total,
			Completed: stats.completed,
		})
	}

	// 构建任务类型统计
	taskTypeStatList := make([]TaskTypeStat, 0, len(taskTypeStats))
	for taskType, stats := range taskTypeStats {
		taskTypeStatList = append(taskTypeStatList, TaskTypeStat{
			TaskType:  taskType,
			Total:     stats.total,
			Completed: stats.completed,
		})
	}

	return &PlanProgressResponse{
		PlanID:          planID,
		TotalTasks:      totalTasks,
		CompletedTasks:  completedCount,
		SkippedTasks:    skippedCount,
		InProgressTasks: inProgressCount,
		PendingTasks:    pendingCount,
		Progress:        progress,
		DailyProgress:   dailyProgress,
		TaskTypeStats:   taskTypeStatList,
	}, nil
}

// dailyStat 每日统计辅助结构
type dailyStat struct {
	total     int
	completed int
}

// taskTypeStatData 任务类型统计辅助结构
type taskTypeStatData struct {
	total     int
	completed int
}
