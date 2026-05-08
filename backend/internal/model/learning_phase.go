package model

import "strings"

const (
	// LearningPhaseFoundation 表示补齐基础概念与方法框架阶段。
	LearningPhaseFoundation = "foundation"
	// LearningPhaseDrill 表示围绕薄弱点做专项强化训练阶段。
	LearningPhaseDrill = "drill"
	// LearningPhaseReview 表示回看与复盘纠偏阶段。
	LearningPhaseReview = "review"
	// LearningPhaseMock 表示模拟或限时验证阶段。
	LearningPhaseMock = "mock"
)

// NormalizeLearningPhase 标准化学习阶段枚举，未知值回退到基础阶段。
func NormalizeLearningPhase(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case LearningPhaseFoundation, LearningPhaseDrill, LearningPhaseReview, LearningPhaseMock:
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return LearningPhaseFoundation
	}
}

// ResolveLearningPhaseFromTaskType 根据任务类型推导默认学习阶段。
func ResolveLearningPhaseFromTaskType(taskType string) string {
	switch strings.ToLower(strings.TrimSpace(taskType)) {
	case TaskTypePractice:
		return LearningPhaseDrill
	case TaskTypeReview:
		return LearningPhaseReview
	case TaskTypeInterview:
		return LearningPhaseMock
	default:
		return LearningPhaseFoundation
	}
}

// BuildLearningPhaseGoal 返回阶段对应的默认阶段目标文案。
func BuildLearningPhaseGoal(phase string) string {
	switch NormalizeLearningPhase(phase) {
	case LearningPhaseDrill:
		return "围绕当前高频薄弱点做专项强化训练。"
	case LearningPhaseReview:
		return "回看近期训练表现，修正易错点并巩固方法。"
	case LearningPhaseMock:
		return "用模拟或限时任务验证当前阶段的真实掌握度。"
	default:
		return "先补齐核心概念、基础方法和通用解题框架。"
	}
}
