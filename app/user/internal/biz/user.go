package biz

import (
	"context"
	"time"

	kratosErr "github.com/go-kratos/kratos/v2/errors"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// BaseModel 所有实体公共基础字段（FIX U1: 符合全局规范 1.4，支持软删除）
type BaseModel struct {
	ID        uint           `gorm:"primaryKey;autoIncrement"`
	CreatedAt time.Time      `gorm:"not null;autoCreateTime"`
	UpdatedAt time.Time      `gorm:"not null;autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// UserRepo data 层必须实现的接口
type UserRepo interface {
	Create(ctx context.Context, user *User) error
	GetByID(ctx context.Context, id uint64) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	GetByUsername(ctx context.Context, username string) (*User, error)
	BatchGetByIDs(ctx context.Context, ids []uint64) ([]*User, error)
	List(ctx context.Context, page, pageSize int32) ([]*User, int64, error)
	Update(ctx context.Context, user *User) error
	GetAdminStats(ctx context.Context) (int64, int64, int64, int64, error)
}

// TokenBlacklist token 黑名单接口，用于登出和 token 吊销
type TokenBlacklist interface {
	// Add 将指定 JTI 加入黑名单并设置 TTL
	Add(ctx context.Context, tokenJTI string, ttl time.Duration) error
	// IsBlacklisted 检查指定 JTI 是否已在黑名单中
	IsBlacklisted(ctx context.Context, tokenJTI string) (bool, error)
}

// --- 领域实体 ---

// User 用户实体（对齐单体 users 表结构）
type User struct {
	BaseModel
	Username          string     `gorm:"size:50;not null;uniqueIndex"` // 对齐单体 size:50
	Password          string     `gorm:"column:password_hash;size:255;not null"`
	Email             string     `gorm:"size:100;not null;uniqueIndex"` // 对齐单体 size:100
	Avatar            string     `gorm:"size:500"`
	Role              string     `gorm:"size:20;not null;default:'free_member'"` // 对齐单体默认值
	MembershipLevel   string     `gorm:"size:10;not null;default:'free'"`         // 对齐单体 size:10
	MembershipExpireAt *time.Time

	// 运行期字段，不落库（表中无这些列）
	MembershipType string `gorm:"-"` // 表中无此列
	IsDisabled     bool   `gorm:"-"` // 表中无此列
}

func (User) TableName() string { return "users" }

// UserUseCase 用户业务用例
type UserUseCase struct {
	repo UserRepo
}

func NewUserUseCase(repo UserRepo) *UserUseCase {
	return &UserUseCase{repo: repo}
}

// Register 用户注册（对齐单体：检查 email + username 唯一性，Role 默认 free_member）
func (uc *UserUseCase) Register(ctx context.Context, username, password, email string) (*User, error) {
	// 检查邮箱是否已存在
	if _, err := uc.repo.GetByEmail(ctx, email); err == nil {
		return nil, ErrEmailExists
	}
	// 检查用户名是否已存在（对齐单体）
	if _, err := uc.repo.GetByUsername(ctx, username); err == nil {
		return nil, ErrUsernameExists
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
		Role:     "free_member", // 对齐单体默认角色
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

// UpdateProfile 更新用户资料（对齐单体：检查用户名唯一性）
func (uc *UserUseCase) UpdateProfile(ctx context.Context, id uint64, username, avatar string) (*User, error) {
	user, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return nil, ErrUserNotFound
	}
	if username != "" {
		// 对齐单体：检查用户名唯一性（排除自己）
		existing, _ := uc.repo.GetByUsername(ctx, username)
		if existing != nil && uint64(existing.ID) != id {
			return nil, ErrUsernameExists
		}
		user.Username = username
	}
	if avatar != "" {
		user.Avatar = avatar
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

// ListUsers 管理后台分页获取用户列表。
func (uc *UserUseCase) ListUsers(ctx context.Context, page, pageSize int32) ([]*User, int64, error) {
	users, total, err := uc.repo.List(ctx, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	for _, user := range users {
		user.Password = ""
	}
	return users, total, nil
}

// UpdateUserRole 管理后台更新用户角色，并在非 disabled 角色下恢复可用状态。
func (uc *UserUseCase) UpdateUserRole(ctx context.Context, id uint64, role string) error {
	user, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return ErrUserNotFound
	}
	user.Role = role
	if role != "disabled" {
		user.IsDisabled = false
	}
	return uc.repo.Update(ctx, user)
}

// ToggleUserBan 管理后台切换用户封禁状态，保持与旧后台的禁用/恢复语义一致。
func (uc *UserUseCase) ToggleUserBan(ctx context.Context, id uint64) error {
	user, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return ErrUserNotFound
	}
	user.IsDisabled = !user.IsDisabled
	if user.IsDisabled {
		user.Role = "disabled"
	} else if user.Role == "disabled" {
		user.Role = "user"
	}
	return uc.repo.Update(ctx, user)
}

// GetAdminStats 返回管理后台用户统计。
func (uc *UserUseCase) GetAdminStats(ctx context.Context) (int64, int64, int64, int64, error) {
	return uc.repo.GetAdminStats(ctx)
}
