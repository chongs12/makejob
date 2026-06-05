package data

import (
	"context"

	"gorm.io/gorm"

	"makejob/app/user/internal/biz"
)

type userRepo struct {
	db *gorm.DB
}

// NewUserRepo 创建用户仓库实现
func NewUserRepo(db *gorm.DB) biz.UserRepo {
	return &userRepo{db: db}
}

func (r *userRepo) Create(ctx context.Context, user *biz.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *userRepo) GetByID(ctx context.Context, id uint64) (*biz.User, error) {
	var user biz.User
	if err := r.db.WithContext(ctx).First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepo) GetByEmail(ctx context.Context, email string) (*biz.User, error) {
	var user biz.User
	if err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepo) BatchGetByIDs(ctx context.Context, ids []uint64) ([]*biz.User, error) {
	var users []*biz.User
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func (r *userRepo) Update(ctx context.Context, user *biz.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}
