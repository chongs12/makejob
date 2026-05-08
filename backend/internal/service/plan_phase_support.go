package service

import (
	"sort"
	"strings"

	"makejob-backend/internal/ai"
	"makejob-backend/internal/model"
)

// normalizeLearningPlanPhases 为学习计划及任务补齐稳定的阶段与阶段目标字段。
func normalizeLearningPlanPhases(plan ai.LearningPlan) ai.LearningPlan {
	for index := range plan.Tasks {
		phase := resolveOptionalLearningPhase(plan.Tasks[index].Phase, plan.Tasks[index].TaskType)
		plan.Tasks[index].Phase = phase
		plan.Tasks[index].PhaseGoal = resolveLearningPhaseGoal(phase, plan.Tasks[index].PhaseGoal)
	}

	plan.Phase = resolveOptionalPlanPhase(plan.Phase)
	if plan.Phase == "" {
		if len(plan.Tasks) > 0 {
			plan.Phase = plan.Tasks[0].Phase
		} else {
			plan.Phase = model.LearningPhaseFoundation
		}
	}
	plan.PhaseGoal = resolveLearningPhaseGoal(plan.Phase, plan.PhaseGoal)
	return plan
}

// arrangeLearningPlanByPhase 将任务整理为连续阶段窗口，并按阶段窗口重新分配天数。
func arrangeLearningPlanByPhase(plan ai.LearningPlan, preservePhaseEntryOrder bool) ai.LearningPlan {
	if len(plan.Tasks) == 0 {
		plan.Phase = model.LearningPhaseFoundation
		plan.PhaseGoal = resolveLearningPhaseGoal(plan.Phase, plan.PhaseGoal)
		return plan
	}

	if plan.Duration <= 0 {
		plan.Duration = len(plan.Tasks)
	}
	if plan.Duration < len(plan.Tasks) {
		plan.Duration = len(plan.Tasks)
	}

	orderedTasks, phaseOrder := orderLearningPlanTasksByPhase(plan.Tasks, preservePhaseEntryOrder)
	rebuiltTasks := make([]ai.PlanTask, 0, len(orderedTasks))
	totalTasks := len(orderedTasks)
	phaseBuckets := buildLearningPhaseBuckets(orderedTasks, phaseOrder)
	processedTasks := 0
	for _, bucket := range phaseBuckets {
		windowStart, windowEnd := calculateLearningPhaseWindow(processedTasks, len(bucket.Tasks), totalTasks, plan.Duration)
		rebuiltTasks = append(rebuiltTasks, spreadLearningPhaseTasks(bucket.Tasks, windowStart, windowEnd)...)
		processedTasks += len(bucket.Tasks)
	}

	plan.Tasks = rebuiltTasks
	plan.Phase = phaseOrder[0]
	plan.PhaseGoal = resolveLearningPhaseGoal(plan.Phase, plan.PhaseGoal)
	return plan
}

// resolvePlanPhaseFields 解析计划当前应返回的阶段与阶段目标，优先按任务进度推导当前阶段。
func resolvePlanPhaseFields(plan *model.LearningPlan, tasks []model.LearningTask) (string, string) {
	if len(tasks) > 0 {
		return resolveCurrentPlanPhase(tasks)
	}
	if plan != nil {
		if phase := resolveOptionalPlanPhase(plan.Phase); phase != "" {
			return phase, resolveLearningPhaseGoal(phase, plan.PhaseGoal)
		}
	}
	return resolveCurrentPlanPhase(tasks)
}

// resolveTaskPhaseFields 解析任务当前应返回的阶段与阶段目标，兼容旧数据缺少字段的情况。
func resolveTaskPhaseFields(task model.LearningTask) (string, string) {
	phase := resolveOptionalLearningPhase(task.Phase, task.TaskType)
	return phase, resolveLearningPhaseGoal(phase, task.PhaseGoal)
}

// resolveCurrentPlanPhase 根据当前任务序列挑选计划的主阶段，优先选择未完成任务。
func resolveCurrentPlanPhase(tasks []model.LearningTask) (string, string) {
	for _, task := range tasks {
		if task.Status == model.TaskStatusPending || task.Status == model.TaskStatusInProgress {
			return resolveTaskPhaseFields(task)
		}
	}
	if len(tasks) == 0 {
		return model.LearningPhaseFoundation, model.BuildLearningPhaseGoal(model.LearningPhaseFoundation)
	}
	return resolveTaskPhaseFields(tasks[0])
}

// resolveOptionalPlanPhase 仅在阶段字段非空时做规范化，便于旧数据继续走回退逻辑。
func resolveOptionalPlanPhase(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	return model.NormalizeLearningPhase(raw)
}

// resolveOptionalLearningPhase 解析任务阶段，缺失时按任务类型推导默认阶段。
func resolveOptionalLearningPhase(rawPhase string, taskType string) string {
	if strings.TrimSpace(rawPhase) != "" {
		return model.NormalizeLearningPhase(rawPhase)
	}
	return model.ResolveLearningPhaseFromTaskType(taskType)
}

// resolveLearningPhaseGoal 解析阶段目标，缺失时回退到阶段默认文案。
func resolveLearningPhaseGoal(phase string, rawGoal string) string {
	goal := strings.TrimSpace(rawGoal)
	if goal != "" {
		return goal
	}
	return model.BuildLearningPhaseGoal(phase)
}

type learningPhaseBucket struct {
	Phase string
	Tasks []ai.PlanTask
}

// orderLearningPlanTasksByPhase 按阶段顺序整理任务，并保持桶内相对顺序稳定。
func orderLearningPlanTasksByPhase(tasks []ai.PlanTask, preservePhaseEntryOrder bool) ([]ai.PlanTask, []string) {
	if len(tasks) == 0 {
		return nil, nil
	}

	sortedTasks := append([]ai.PlanTask(nil), tasks...)
	if !preservePhaseEntryOrder {
		sort.SliceStable(sortedTasks, func(i, j int) bool {
			leftWeight := learningPhaseWeight(sortedTasks[i].Phase)
			rightWeight := learningPhaseWeight(sortedTasks[j].Phase)
			if leftWeight != rightWeight {
				return leftWeight < rightWeight
			}
			if sortedTasks[i].DayNumber != sortedTasks[j].DayNumber {
				return sortedTasks[i].DayNumber < sortedTasks[j].DayNumber
			}
			return sortedTasks[i].Priority < sortedTasks[j].Priority
		})
	}

	phaseOrder := buildLearningPhaseOrder(sortedTasks, preservePhaseEntryOrder)
	buckets := buildLearningPhaseBuckets(sortedTasks, phaseOrder)
	orderedTasks := make([]ai.PlanTask, 0, len(sortedTasks))
	for _, bucket := range buckets {
		orderedTasks = append(orderedTasks, bucket.Tasks...)
	}
	return orderedTasks, phaseOrder
}

// buildLearningPhaseOrder 生成当前计划应采用的阶段顺序。
func buildLearningPhaseOrder(tasks []ai.PlanTask, preservePhaseEntryOrder bool) []string {
	if preservePhaseEntryOrder {
		order := make([]string, 0, 4)
		seen := make(map[string]struct{}, 4)
		for _, task := range tasks {
			if _, exists := seen[task.Phase]; exists {
				continue
			}
			seen[task.Phase] = struct{}{}
			order = append(order, task.Phase)
		}
		if len(order) > 0 {
			return order
		}
	}

	order := make([]string, 0, 4)
	for _, phase := range []string{
		model.LearningPhaseFoundation,
		model.LearningPhaseDrill,
		model.LearningPhaseReview,
		model.LearningPhaseMock,
	} {
		if containsLearningPhase(tasks, phase) {
			order = append(order, phase)
		}
	}
	if len(order) == 0 {
		return []string{model.LearningPhaseFoundation}
	}
	return order
}

// buildLearningPhaseBuckets 根据阶段顺序汇总任务桶。
func buildLearningPhaseBuckets(tasks []ai.PlanTask, phaseOrder []string) []learningPhaseBucket {
	taskGroups := make(map[string][]ai.PlanTask, len(phaseOrder))
	for _, task := range tasks {
		taskGroups[task.Phase] = append(taskGroups[task.Phase], task)
	}

	buckets := make([]learningPhaseBucket, 0, len(phaseOrder))
	for _, phase := range phaseOrder {
		group := taskGroups[phase]
		if len(group) == 0 {
			continue
		}
		buckets = append(buckets, learningPhaseBucket{
			Phase: phase,
			Tasks: group,
		})
	}
	return buckets
}

// calculateLearningPhaseWindow 计算单个阶段桶在整个计划中应占用的天数窗口。
func calculateLearningPhaseWindow(processedTasks int, bucketSize int, totalTasks int, totalDays int) (int, int) {
	if totalTasks <= 0 || totalDays <= 0 || bucketSize <= 0 {
		return 1, 1
	}

	windowStart := processedTasks*totalDays/totalTasks + 1
	windowEnd := (processedTasks + bucketSize) * totalDays / totalTasks
	if windowEnd < windowStart {
		windowEnd = windowStart
	}
	if windowEnd > totalDays {
		windowEnd = totalDays
	}
	return windowStart, windowEnd
}

// spreadLearningPhaseTasks 将同一阶段内的任务均匀分布到该阶段窗口。
func spreadLearningPhaseTasks(tasks []ai.PlanTask, windowStart int, windowEnd int) []ai.PlanTask {
	if len(tasks) == 0 {
		return nil
	}

	span := windowEnd - windowStart
	result := make([]ai.PlanTask, 0, len(tasks))
	for index, task := range tasks {
		rewritten := task
		if len(tasks) == 1 || span <= 0 {
			rewritten.DayNumber = windowStart
		} else {
			rewritten.DayNumber = windowStart + index*span/(len(tasks)-1)
		}
		rewritten.PhaseGoal = resolveLearningPhaseGoal(rewritten.Phase, rewritten.PhaseGoal)
		result = append(result, rewritten)
	}
	return result
}

// containsLearningPhase 判断任务列表中是否包含指定阶段。
func containsLearningPhase(tasks []ai.PlanTask, targetPhase string) bool {
	for _, task := range tasks {
		if task.Phase == targetPhase {
			return true
		}
	}
	return false
}

// learningPhaseWeight 返回阶段的稳定排序权重。
func learningPhaseWeight(phase string) int {
	switch phase {
	case model.LearningPhaseFoundation:
		return 0
	case model.LearningPhaseDrill:
		return 1
	case model.LearningPhaseReview:
		return 2
	case model.LearningPhaseMock:
		return 3
	default:
		return 99
	}
}
