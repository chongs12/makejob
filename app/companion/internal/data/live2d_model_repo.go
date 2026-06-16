package data

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"makejob/app/companion/internal/biz"
)

// live2DModelRepo 实现 biz.Live2DModelRepo 接口
type live2DModelRepo struct {
	db *gorm.DB
}

// NewLive2DModelRepo 创建 Live2D 模型仓库
func NewLive2DModelRepo(db *gorm.DB) biz.Live2DModelRepo {
	return &live2DModelRepo{db: db}
}

func (r *live2DModelRepo) GetByID(ctx context.Context, id uint) (*biz.Live2DModel, error) {
	var model biz.Live2DModel
	if err := r.db.WithContext(ctx).First(&model, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get live2d model by id: %w", err)
	}
	return &model, nil
}

func (r *live2DModelRepo) GetByKey(ctx context.Context, key string) (*biz.Live2DModel, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, nil
	}

	// 支持 "db:123" 格式
	if strings.HasPrefix(key, "db:") {
		idStr := strings.TrimPrefix(key, "db:")
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			return nil, nil
		}
		return r.GetByID(ctx, uint(id))
	}

	// 直接按 ID 查询
	id, err := strconv.ParseUint(key, 10, 64)
	if err != nil {
		return nil, nil
	}
	return r.GetByID(ctx, uint(id))
}
