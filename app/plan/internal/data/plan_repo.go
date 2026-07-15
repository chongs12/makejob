package data

import (
	"context"
	"time"

	"gorm.io/gorm"

	"makejob/app/plan/internal/biz"
)

// txContextKey 用于在 context 中透传事务 DB。
type txContextKey struct{}

// planRepo 实现 biz.PlanRepo 接口
type planRepo struct {
	db *gorm.DB
}

// NewPlanRepo 创建学习计划仓库实现
func NewPlanRepo(db *gorm.DB) biz.PlanRepo {
	return &planRepo{db: db}
}

// getDB 从 context 获取事务 DB，若无则返回默认 DB。
func (r *planRepo) getDB(ctx context.Context) *gorm.DB {
	if tx, ok := ctx.Value(txContextKey{}).(*gorm.DB); ok {
		return tx
	}
	return r.db
}

// Create 创建学习计划
func (r *planRepo) Create(ctx context.Context, plan *biz.LearningPlan) error {
	return r.getDB(ctx).WithContext(ctx).Create(plan).Error
}

// GetByID 根据 ID 查询学习计划
func (r *planRepo) GetByID(ctx context.Context, id uint64) (*biz.LearningPlan, error) {
	var plan biz.LearningPlan
	if err := r.getDB(ctx).WithContext(ctx).First(&plan, id).Error; err != nil {
		return nil, err
	}
	return &plan, nil
}

// GetByUserID 查询用户当前最新的活跃计划
func (r *planRepo) GetByUserID(ctx context.Context, userID uint64) (*biz.LearningPlan, error) {
	var plan biz.LearningPlan
	if err := r.getDB(ctx).WithContext(ctx).
		Where("user_id = ? AND status = ?", userID, "active").
		Order("created_at DESC").
		First(&plan).Error; err != nil {
		return nil, err
	}
	return &plan, nil
}

// Update 更新学习计划核心字段
func (r *planRepo) Update(ctx context.Context, plan *biz.LearningPlan) error {
	return r.getDB(ctx).WithContext(ctx).Model(&biz.LearningPlan{}).Where("id = ?", plan.ID).Updates(map[string]any{
		"title":           plan.Title,
		"description":     plan.Description,
		"industry_id":     plan.IndustryID,
		"plan_json":       plan.PlanJSON,
		"status":          plan.Status,
		"completed_tasks": plan.CompletedTasks,
		"total_tasks":     plan.TotalTasks,
		"start_date":      plan.StartDate,
		"end_date":        plan.EndDate,
		"phase":           plan.Phase,
		"phase_goal":      plan.PhaseGoal,
	}).Error
}

// ListByUserID 分页查询用户的计划列表
func (r *planRepo) ListByUserID(ctx context.Context, userID uint64, page, pageSize int32) ([]*biz.LearningPlan, int64, error) {
	var plans []*biz.LearningPlan
	var total int64

	query := r.getDB(ctx).WithContext(ctx).Where("user_id = ?", userID)
	if err := query.Model(&biz.LearningPlan{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(int(offset)).Limit(int(pageSize)).Find(&plans).Error; err != nil {
		return nil, 0, err
	}
	return plans, total, nil
}

// Transaction 在事务中执行多表操作
func (r *planRepo) Transaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txCtx := context.WithValue(ctx, txContextKey{}, tx)
		return fn(txCtx)
	})
}

// taskRepo 实现 biz.TaskRepo 接口
type taskRepo struct {
	db *gorm.DB
}

// newTaskRepo 创建任务仓库实例
func newTaskRepo(db *gorm.DB) *taskRepo {
	return &taskRepo{db: db}
}

// NewTaskRepo 创建学习任务仓库实现
func NewTaskRepo(db *gorm.DB) biz.TaskRepo {
	return newTaskRepo(db)
}

// getDB 从 context 获取事务 DB，若无则返回默认 DB。
func (r *taskRepo) getDB(ctx context.Context) *gorm.DB {
	if tx, ok := ctx.Value(txContextKey{}).(*gorm.DB); ok {
		return tx
	}
	return r.db
}

// BatchCreate 批量创建学习任务
func (r *taskRepo) BatchCreate(ctx context.Context, tasks []*biz.LearningTask) error {
	if len(tasks) == 0 {
		return nil
	}
	return r.getDB(ctx).WithContext(ctx).CreateInBatches(tasks, 100).Error
}

// ListByPlanID 查询计划下所有任务
func (r *taskRepo) ListByPlanID(ctx context.Context, planID uint64) ([]*biz.LearningTask, error) {
	var tasks []*biz.LearningTask
	if err := r.getDB(ctx).WithContext(ctx).Where("plan_id = ?", planID).Order("sort_order ASC").Find(&tasks).Error; err != nil {
		return nil, err
	}
	return tasks, nil
}

// GetByID 根据 ID 获取单个任务
func (r *taskRepo) GetByID(ctx context.Context, id uint64) (*biz.LearningTask, error) {
	var task biz.LearningTask
	if err := r.getDB(ctx).WithContext(ctx).First(&task, id).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

// Update 更新任务状态和内容
func (r *taskRepo) Update(ctx context.Context, task *biz.LearningTask) error {
	return r.getDB(ctx).WithContext(ctx).Model(&biz.LearningTask{}).Where("id = ?", task.ID).Updates(map[string]any{
		"title":        task.Title,
		"description":  task.Description,
		"task_type":    task.TaskType,
		"phase":        task.Phase,
		"phase_goal":   task.PhaseGoal,
		"target_id":    task.TargetID,
		"due_date":     task.DueDate,
		"status":       task.Status,
		"completed_at": task.CompletedAt,
		"sort_order":   task.SortOrder,
	}).Error
}

// CountByPlanID 统计计划下任务总数
func (r *taskRepo) CountByPlanID(ctx context.Context, planID uint64) (int32, error) {
	var count int64
	if err := r.getDB(ctx).WithContext(ctx).Model(&biz.LearningTask{}).Where("plan_id = ?", planID).Count(&count).Error; err != nil {
		return 0, err
	}
	return int32(count), nil
}

// CountCompletedByPlanID 统计计划下已完成任务数
func (r *taskRepo) CountCompletedByPlanID(ctx context.Context, planID uint64) (int32, error) {
	var count int64
	if err := r.getDB(ctx).WithContext(ctx).Model(&biz.LearningTask{}).
		Where("plan_id = ? AND status IN ?", planID, []string{"completed", "skipped"}).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return int32(count), nil
}

// BatchDelete 仅软删除指定计划下 pending 状态的任务
func (r *taskRepo) BatchDelete(ctx context.Context, planID uint64, ids []uint64) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	result := r.getDB(ctx).WithContext(ctx).
		Where("plan_id = ? AND status = ? AND id IN ?", planID, "pending", ids).
		Delete(&biz.LearningTask{})
	return result.RowsAffected, result.Error
}

// BatchUpdateSortOrder 仅更新指定计划下 pending 状态任务的排序
func (r *taskRepo) BatchUpdateSortOrder(ctx context.Context, planID uint64, updates map[uint64]int32) (int64, error) {
	if len(updates) == 0 {
		return 0, nil
	}
	var affected int64
	db := r.getDB(ctx).WithContext(ctx)
	for id, sortOrder := range updates {
		result := db.Model(&biz.LearningTask{}).
			Where("plan_id = ? AND status = ? AND id = ?", planID, "pending", id).
			Update("sort_order", sortOrder)
		if result.Error != nil {
			return affected, result.Error
		}
		affected += result.RowsAffected
	}
	return affected, nil
}

// taskFeedbackRepo 实现 biz.TaskFeedbackRepo 接口
type taskFeedbackRepo struct {
	db *gorm.DB
}

// newTaskFeedbackRepo 创建反馈仓库实例
func newTaskFeedbackRepo(db *gorm.DB) *taskFeedbackRepo {
	return &taskFeedbackRepo{db: db}
}

// NewTaskFeedbackRepo 创建任务反馈仓库实现
func NewTaskFeedbackRepo(db *gorm.DB) biz.TaskFeedbackRepo {
	return newTaskFeedbackRepo(db)
}

// getDB 从 context 获取事务 DB，若无则返回默认 DB。
func (r *taskFeedbackRepo) getDB(ctx context.Context) *gorm.DB {
	if tx, ok := ctx.Value(txContextKey{}).(*gorm.DB); ok {
		return tx
	}
	return r.db
}

// Create 创建反馈记录
func (r *taskFeedbackRepo) Create(ctx context.Context, feedback *biz.TaskFeedback) error {
	return r.getDB(ctx).WithContext(ctx).Create(feedback).Error
}

// GetByID 根据 ID 获取反馈记录
func (r *taskFeedbackRepo) GetByID(ctx context.Context, id uint64) (*biz.TaskFeedback, error) {
	var feedback biz.TaskFeedback
	if err := r.getDB(ctx).WithContext(ctx).First(&feedback, id).Error; err != nil {
		return nil, err
	}
	return &feedback, nil
}

// Update 更新反馈记录
func (r *taskFeedbackRepo) Update(ctx context.Context, feedback *biz.TaskFeedback) error {
	return r.getDB(ctx).WithContext(ctx).Model(&biz.TaskFeedback{}).Where("id = ?", feedback.ID).Updates(map[string]any{
		"difficulty_feeling":      feedback.DifficultyFeeling,
		"feedback_text":           feedback.FeedbackText,
		"actual_duration_minutes": feedback.ActualDurationMinutes,
		"problem_areas_json":      feedback.ProblemAreasJSON,
		"diagnosis_json":          feedback.DiagnosisJSON,
		"diagnosis_status":        feedback.DiagnosisStatus,
	}).Error
}

// ListByPlanID 查询计划下所有反馈记录
func (r *taskFeedbackRepo) ListByPlanID(ctx context.Context, planID uint64) ([]*biz.TaskFeedback, error) {
	var feedbacks []*biz.TaskFeedback
	if err := r.getDB(ctx).WithContext(ctx).Where("plan_id = ?", planID).Order("created_at ASC").Find(&feedbacks).Error; err != nil {
		return nil, err
	}
	return feedbacks, nil
}

// planAdjustmentRepo 实现 biz.PlanAdjustmentRepo 接口
type planAdjustmentRepo struct {
	db *gorm.DB
}

// newPlanAdjustmentRepo 创建调整记录仓库实例
func newPlanAdjustmentRepo(db *gorm.DB) *planAdjustmentRepo {
	return &planAdjustmentRepo{db: db}
}

// NewPlanAdjustmentRepo 创建计划调整记录仓库实现
func NewPlanAdjustmentRepo(db *gorm.DB) biz.PlanAdjustmentRepo {
	return newPlanAdjustmentRepo(db)
}

// getDB 从 context 获取事务 DB，若无则返回默认 DB。
func (r *planAdjustmentRepo) getDB(ctx context.Context) *gorm.DB {
	if tx, ok := ctx.Value(txContextKey{}).(*gorm.DB); ok {
		return tx
	}
	return r.db
}

// Create 创建调整记录
func (r *planAdjustmentRepo) Create(ctx context.Context, adjustment *biz.PlanAdjustment) error {
	return r.getDB(ctx).WithContext(ctx).Create(adjustment).Error
}

// planAdjustmentPreviewRepo 实现 biz.PlanAdjustmentPreviewRepo 接口。
type planAdjustmentPreviewRepo struct {
	db *gorm.DB
}

// newPlanAdjustmentPreviewRepo 创建调整预览仓库实例。
func newPlanAdjustmentPreviewRepo(db *gorm.DB) *planAdjustmentPreviewRepo {
	return &planAdjustmentPreviewRepo{db: db}
}

// NewPlanAdjustmentPreviewRepo 创建调整预览仓库实现。
func NewPlanAdjustmentPreviewRepo(db *gorm.DB) biz.PlanAdjustmentPreviewRepo {
	return newPlanAdjustmentPreviewRepo(db)
}

// getDB 从 context 获取事务 DB，若无则返回默认 DB。
func (r *planAdjustmentPreviewRepo) getDB(ctx context.Context) *gorm.DB {
	if tx, ok := ctx.Value(txContextKey{}).(*gorm.DB); ok {
		return tx
	}
	return r.db
}

// Create 创建待确认的调整预览记录。
func (r *planAdjustmentPreviewRepo) Create(ctx context.Context, preview *biz.PlanAdjustmentPreview) error {
	return r.getDB(ctx).WithContext(ctx).Create(preview).Error
}

// GetByToken 根据预览令牌读取调整预览记录。
func (r *planAdjustmentPreviewRepo) GetByToken(ctx context.Context, token string) (*biz.PlanAdjustmentPreview, error) {
	var preview biz.PlanAdjustmentPreview
	if err := r.getDB(ctx).WithContext(ctx).Where("token = ?", token).First(&preview).Error; err != nil {
		return nil, err
	}
	return &preview, nil
}

// MarkApplied 标记调整预览已被应用，避免重复确认。
func (r *planAdjustmentPreviewRepo) MarkApplied(ctx context.Context, previewID uint64) error {
	now := time.Now()
	result := r.getDB(ctx).WithContext(ctx).Model(&biz.PlanAdjustmentPreview{}).Where("id = ? AND status = ?", previewID, "pending").Updates(map[string]any{
		"status":     "applied",
		"applied_at": &now,
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
