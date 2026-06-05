package data

import (
	"context"

	"gorm.io/gorm"

	"makejob/app/ai_gateway/internal/biz"
)

type callLogRepo struct {
	db *gorm.DB
}

// NewCallLogRepo 创建 AI 调用日志仓库实现
func NewCallLogRepo(db *gorm.DB) biz.CallLogRepo {
	return &callLogRepo{db: db}
}

// Create 插入一条 AI 调用日志记录
func (r *callLogRepo) Create(ctx context.Context, log *biz.AICallLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}
