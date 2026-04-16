// Package service 提供业务逻辑层实现
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"

	"makejob-backend/internal/common"
	"makejob-backend/internal/config"
	"makejob-backend/internal/model"
	"makejob-backend/internal/repository"
	appjwt "makejob-backend/pkg/jwt"
)

// AuthService 认证服务接口
type AuthService interface {
	// Register 用户注册
	Register(ctx context.Context, req *RegisterRequest) (*RegisterResponse, error)
	// Login 用户登录
	Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error)
	// RefreshToken 刷新令牌
	RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error)
	// GetProfile 获取用户资料
	GetProfile(ctx context.Context, userID uint) (*UserProfile, error)
	// UpdateProfile 更新用户资料
	UpdateProfile(ctx context.Context, userID uint, req *UpdateProfileRequest) error
}

// RegisterRequest 注册请求DTO
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=30"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6,max=50"`
}

// RegisterResponse 注册响应DTO
type RegisterResponse struct {
	Token        string      `json:"token"`
	RefreshToken string      `json:"refresh_token"`
	ExpiresAt    int64       `json:"expires_at"`
	User         UserProfile `json:"user"`
}

// LoginRequest 登录请求DTO
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse 登录响应DTO
type LoginResponse struct {
	Token        string      `json:"token"`
	RefreshToken string      `json:"refresh_token"`
	ExpiresAt    int64       `json:"expires_at"`
	User         UserProfile `json:"user"`
}

// TokenResponse 令牌响应DTO
type TokenResponse struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"`
}

// UserProfile 用户资料DTO
type UserProfile struct {
	ID              uint      `json:"id"`
	Username        string    `json:"username"`
	Email           string    `json:"email"`
	Avatar          string    `json:"avatar"`
	Role            string    `json:"role"`
	MembershipLevel string    `json:"membership_level"`
	CreatedAt       time.Time `json:"created_at"`
}

// UpdateProfileRequest 更新资料请求DTO
type UpdateProfileRequest struct {
	Username string `json:"username,omitempty" binding:"omitempty,min=3,max=30"`
	Avatar   string `json:"avatar,omitempty"`
}

// authService 认证服务实现
type authService struct {
	userRepo repository.UserRepository
	cfg      *config.Config
}

// NewAuthService 创建认证服务实例
func NewAuthService(userRepo repository.UserRepository, cfg *config.Config) AuthService {
	return &authService{
		userRepo: userRepo,
		cfg:      cfg,
	}
}

// Register 用户注册
func (s *authService) Register(ctx context.Context, req *RegisterRequest) (*RegisterResponse, error) {
	// 检查邮箱是否已存在
	exists, err := s.userRepo.ExistsByEmail(ctx, req.Email)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, common.NewBusinessError(common.CodeBadRequest, "邮箱已被注册")
	}

	// 检查用户名是否已存在
	exists, err = s.userRepo.ExistsByUsername(ctx, req.Username)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, common.NewBusinessError(common.CodeBadRequest, "用户名已被使用")
	}

	// bcrypt加密密码
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("密码加密失败: %w", err)
	}

	// 创建用户
	user := &model.User{
		Username:        req.Username,
		Email:           req.Email,
		PasswordHash:    string(passwordHash),
		Role:            model.UserRoleFreeMember,
		MembershipLevel: model.MembershipLevelFree,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	// 注册成功后自动生成token（自动登录）
	token, err := appjwt.GenerateToken(user.ID, user.Username, user.Role)
	if err != nil {
		return nil, fmt.Errorf("生成访问令牌失败: %w", err)
	}

	refreshToken, err := appjwt.GenerateRefreshToken(user.ID)
	if err != nil {
		return nil, fmt.Errorf("生成刷新令牌失败: %w", err)
	}

	expiresAt := time.Now().Add(time.Duration(s.cfg.JWT.Expire) * time.Hour).Unix()

	return &RegisterResponse{
		Token:        token,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
		User:         *convertToUserProfile(user),
	}, nil
}

// Login 用户登录
func (s *authService) Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
	// 查找用户
	user, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, common.NewBusinessError(common.CodeUnauthorized, "邮箱或密码错误")
	}

	// 校验密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, common.NewBusinessError(common.CodeUnauthorized, "邮箱或密码错误")
	}

	// 生成JWT Token
	token, err := appjwt.GenerateToken(user.ID, user.Username, user.Role)
	if err != nil {
		return nil, fmt.Errorf("生成访问令牌失败: %w", err)
	}

	// 生成Refresh Token
	refreshToken, err := appjwt.GenerateRefreshToken(user.ID)
	if err != nil {
		return nil, fmt.Errorf("生成刷新令牌失败: %w", err)
	}

	// 计算过期时间
	expiresAt := time.Now().Add(time.Duration(s.cfg.JWT.Expire) * time.Hour).Unix()

	return &LoginResponse{
		Token:        token,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
		User:         *convertToUserProfile(user),
	}, nil
}

// RefreshToken 刷新令牌
func (s *authService) RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	// 解析refresh token
	claims, err := appjwt.ParseToken(refreshToken)
	if err != nil {
		if errors.Is(err, errors.New("令牌已过期")) {
			return nil, common.NewBusinessError(common.CodeTokenExpired, "刷新令牌已过期")
		}
		return nil, common.NewBusinessError(common.CodeTokenInvalid, "无效的刷新令牌")
	}

	// 获取用户ID
	userID := claims.UserID

	// 验证用户是否存在
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, common.NewBusinessError(common.CodeUnauthorized, "用户不存在")
	}

	// 生成新的Token对
	newToken, err := appjwt.GenerateToken(user.ID, user.Username, user.Role)
	if err != nil {
		return nil, fmt.Errorf("生成访问令牌失败: %w", err)
	}

	newRefreshToken, err := appjwt.GenerateRefreshToken(user.ID)
	if err != nil {
		return nil, fmt.Errorf("生成刷新令牌失败: %w", err)
	}

	expiresAt := time.Now().Add(time.Duration(s.cfg.JWT.Expire) * time.Hour).Unix()

	return &TokenResponse{
		Token:        newToken,
		RefreshToken: newRefreshToken,
		ExpiresAt:    expiresAt,
	}, nil
}

// GetProfile 获取用户资料
func (s *authService) GetProfile(ctx context.Context, userID uint) (*UserProfile, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, common.NewBusinessError(common.CodeNotFound, "用户不存在")
	}

	return convertToUserProfile(user), nil
}

// UpdateProfile 更新用户资料
func (s *authService) UpdateProfile(ctx context.Context, userID uint, req *UpdateProfileRequest) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if user == nil {
		return common.NewBusinessError(common.CodeNotFound, "用户不存在")
	}

	// 如果更新用户名，检查是否已被使用
	if req.Username != "" && req.Username != user.Username {
		exists, err := s.userRepo.ExistsByUsername(ctx, req.Username)
		if err != nil {
			return err
		}
		if exists {
			return common.NewBusinessError(common.CodeBadRequest, "用户名已被使用")
		}
		user.Username = req.Username
	}

	// 更新头像
	if req.Avatar != "" {
		user.Avatar = req.Avatar
	}

	return s.userRepo.Update(ctx, user)
}

// convertToUserProfile 将User模型转换为UserProfile DTO
func convertToUserProfile(user *model.User) *UserProfile {
	return &UserProfile{
		ID:              user.ID,
		Username:        user.Username,
		Email:           user.Email,
		Avatar:          user.Avatar,
		Role:            user.Role,
		MembershipLevel: user.MembershipLevel,
		CreatedAt:       user.CreatedAt,
	}
}
