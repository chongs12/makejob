package data

import (
	"context"
	"time"

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

// List 分页查询用户列表及总数，供管理后台使用。
func (r *userRepo) List(ctx context.Context, page, pageSize int32) ([]*biz.User, int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&biz.User{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	var users []*biz.User
	if err := r.db.WithContext(ctx).
		Order("created_at DESC").
		Offset(int(offset)).
		Limit(int(pageSize)).
		Find(&users).Error; err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

func (r *userRepo) Update(ctx context.Context, user *biz.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

// GetAdminStats 聚合查询管理后台需要的用户统计。
func (r *userRepo) GetAdminStats(ctx context.Context) (int64, int64, int64, int64, error) {
	var totalUsers int64
	if err := r.db.WithContext(ctx).Model(&biz.User{}).Count(&totalUsers).Error; err != nil {
		return 0, 0, 0, 0, err
	}

	var proMembers int64
	if err := r.db.WithContext(ctx).Model(&biz.User{}).Where("membership_level = ?", "pro").Count(&proMembers).Error; err != nil {
		return 0, 0, 0, 0, err
	}

	var newUsersToday int64
	today := time.Now().Format("2006-01-02")
	if err := r.db.WithContext(ctx).Model(&biz.User{}).Where("DATE(created_at) = ?", today).Count(&newUsersToday).Error; err != nil {
		return 0, 0, 0, 0, err
	}

	// 当前用户服务没有独立活跃统计表，先保持兼容返回 0。
	return totalUsers, proMembers, newUsersToday, 0, nil
}
