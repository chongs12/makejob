package data

import (
	"context"

	"gorm.io/gorm"

	"makejob/app/realtime/internal/biz"
)

type realtimeRepo struct {
	db *gorm.DB
}

// NewRealtimeRepo 创建实时会话仓库实现
func NewRealtimeRepo(db *gorm.DB) biz.RealtimeRepo {
	return &realtimeRepo{db: db}
}

// CreateSession 创建会话记录
func (r *realtimeRepo) CreateSession(ctx context.Context, session *biz.Session) error {
	return r.db.WithContext(ctx).Create(session).Error
}

// GetSession 根据会话 ID 查询会话
func (r *realtimeRepo) GetSession(ctx context.Context, sessionID string) (*biz.Session, error) {
	var session biz.Session
	if err := r.db.WithContext(ctx).Where("id = ?", sessionID).First(&session).Error; err != nil {
		return nil, err
	}
	return &session, nil
}

// UpdateSessionStatus 更新会话状态
func (r *realtimeRepo) UpdateSessionStatus(ctx context.Context, sessionID string, status string) error {
	return r.db.WithContext(ctx).Model(&biz.Session{}).Where("id = ?", sessionID).Update("status", status).Error
}
