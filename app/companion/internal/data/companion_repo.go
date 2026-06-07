package data

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"makejob/app/companion/internal/biz"
)

// companionRepo 实现 biz.CompanionRepo 接口
type companionRepo struct {
	db *gorm.DB
}

// NewCompanionRepo 创建陪伴助手仓库实现
func NewCompanionRepo(db *gorm.DB) biz.CompanionRepo {
	return &companionRepo{db: db}
}

// GetSession 根据用户 ID 获取陪伴会话
func (r *companionRepo) GetSession(ctx context.Context, userID uint64) (*biz.CompanionSession, error) {
	var session biz.CompanionSession
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&session).Error; err != nil {
		return nil, err
	}
	return &session, nil
}

// CreateOrUpdate 创建或更新陪伴会话（基于 user_id 唯一约束 upsert）
func (r *companionRepo) CreateOrUpdate(ctx context.Context, session *biz.CompanionSession) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"last_emotion", "last_topic", "session_count", "last_chat_at", "messages_json", "updated_at"}),
		}).
		Create(session).Error
}
