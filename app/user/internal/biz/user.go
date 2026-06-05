package biz

import (
	"context"
	"time"

	kratosErr "github.com/go-kratos/kratos/v2/errors"
	"golang.org/x/crypto/bcrypt"
)

// UserRepo data 层必须实现的接口
type UserRepo interface {
	Create(ctx context.Context, user *User) error
	GetByID(ctx context.Context, id uint64) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	BatchGetByIDs(ctx context.Context, ids []uint64) ([]*User, error)
	Update(ctx context.Context, user *User) error
}

// --- 领域实体 ---

type User struct {
	ID                uint64     `gorm:"primaryKey"`
	Username          string     `gorm:"size:100;not null;uniqueIndex"`
	Password          string     `gorm:"column:password_hash;size:255;not null"`
	Email             string     `gorm:"size:200;not null;uniqueIndex"`
	Role              string     `gorm:"size:20;not null;default:'user'"`
	MembershipLevel   string     `gorm:"size:20;not null;default:'free'"`
	MembershipExpireAt *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (User) TableName() string { return "users" }

// UserUseCase 用户业务用例
type UserUseCase struct {
	repo UserRepo
}

func NewUserUseCase(repo UserRepo) *UserUseCase {
	return &UserUseCase{repo: repo}
}

// Register 用户注册（bcrypt 哈希密码）
func (uc *UserUseCase) Register(ctx context.Context, username, password, email string) (*User, error) {
	// 检查邮箱是否已存在
	if _, err := uc.repo.GetByEmail(ctx, email); err == nil {
		return nil, ErrEmailExists
	}

	// bcrypt 哈希密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, kratosErr.InternalServer("HASH_FAILED", "密码哈希失败").WithCause(err)
	}

	user := &User{
		Username: username,
		Password: string(hashedPassword),
		Email:    email,
		Role:     "user",
	}
	if err := uc.repo.Create(ctx, user); err != nil {
		return nil, err
	}
	// 清除密码字段再返回
	user.Password = ""
	return user, nil
}

// Login 用户登录（bcrypt 验证密码）
func (uc *UserUseCase) Login(ctx context.Context, email, password string) (*User, error) {
	user, err := uc.repo.GetByEmail(ctx, email)
	if err != nil {
		return nil, ErrUserNotFound
	}

	// bcrypt 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, ErrInvalidPassword
	}

	user.Password = ""
	return user, nil
}

// GetUserByID 根据 ID 获取用户
func (uc *UserUseCase) GetUserByID(ctx context.Context, id uint64) (*User, error) {
	user, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return nil, ErrUserNotFound
	}
	user.Password = ""
	return user, nil
}

// BatchGetUsers 批量获取用户
func (uc *UserUseCase) BatchGetUsers(ctx context.Context, ids []uint64) ([]*User, error) {
	users, err := uc.repo.BatchGetByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, u := range users {
		u.Password = ""
	}
	return users, nil
}

// UpdateProfile 更新用户资料
func (uc *UserUseCase) UpdateProfile(ctx context.Context, id uint64, username, avatar string) (*User, error) {
	user, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return nil, ErrUserNotFound
	}
	if username != "" {
		user.Username = username
	}
	if err := uc.repo.Update(ctx, user); err != nil {
		return nil, err
	}
	user.Password = ""
	return user, nil
}

// GetMembershipStatus 获取会员状态
func (uc *UserUseCase) GetMembershipStatus(ctx context.Context, id uint64) (string, *time.Time, bool, error) {
	user, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return "", nil, false, ErrUserNotFound
	}
	isActive := user.MembershipLevel == "pro" && user.MembershipExpireAt != nil && user.MembershipExpireAt.After(time.Now())
	return user.MembershipLevel, user.MembershipExpireAt, isActive, nil
}

// UpgradeMembership 升级会员
func (uc *UserUseCase) UpgradeMembership(ctx context.Context, id uint64, plan string) error {
	user, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return ErrUserNotFound
	}
	expireAt := time.Now().AddDate(0, 1, 0) // 默认 1 个月
	user.MembershipLevel = "pro"
	user.MembershipExpireAt = &expireAt
	return uc.repo.Update(ctx, user)
}
