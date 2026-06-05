package biz

import (
	"context"
)

// CompanionRepo 陪伴助手仓库接口，data 层必须实现
type CompanionRepo interface {
	// GetState 获取用户陪伴状态
	GetState(ctx context.Context, userID uint64) (*CompanionState, error)
}

// CompanionState 陪伴助手状态领域实体
type CompanionState struct {
	UserID    uint64 `gorm:"primaryKey"`
	Emotion   string `gorm:"size:50;not null;default:'neutral'"`
	LastTopic string `gorm:"size:200"`
}

// CompanionUseCase 陪伴助手业务用例
type CompanionUseCase struct {
	repo CompanionRepo
}

// NewCompanionUseCase 创建陪伴助手业务用例
func NewCompanionUseCase(repo CompanionRepo) *CompanionUseCase {
	return &CompanionUseCase{repo: repo}
}

// GetState 获取用户陪伴状态
func (uc *CompanionUseCase) GetState(ctx context.Context, userID uint64) (*CompanionState, error) {
	return uc.repo.GetState(ctx, userID)
}
