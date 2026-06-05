package biz

import (
	"context"
)

// RealtimeRepo 实时会话仓库接口，data 层必须实现
type RealtimeRepo interface {
	// CreateSession 创建会话
	CreateSession(ctx context.Context, session *Session) error
	// GetSession 根据会话 ID 查询会话
	GetSession(ctx context.Context, sessionID string) (*Session, error)
}

// Session 实时会话领域实体
type Session struct {
	ID          string `gorm:"primaryKey;size:64"`
	InterviewID uint64 `gorm:"index"`
	UserID      uint64 `gorm:"index"`
	Status      string `gorm:"size:20;not null;default:'active'"`
}

// RealtimeUseCase 实时会话业务用例
type RealtimeUseCase struct {
	repo RealtimeRepo
}

// NewRealtimeUseCase 创建实时会话业务用例
func NewRealtimeUseCase(repo RealtimeRepo) *RealtimeUseCase {
	return &RealtimeUseCase{repo: repo}
}

// CreateSession 创建会话
func (uc *RealtimeUseCase) CreateSession(ctx context.Context, session *Session) error {
	return uc.repo.CreateSession(ctx, session)
}

// GetSession 根据会话 ID 查询会话
func (uc *RealtimeUseCase) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	return uc.repo.GetSession(ctx, sessionID)
}
