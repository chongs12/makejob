package service

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	sharedv1 "makejob/api/makejob/shared/v1"
	"google.golang.org/protobuf/types/known/timestamppb"

	userv1 "makejob/api/makejob/user/v1"
	"makejob/app/user/internal/biz"
	"makejob/app/user/internal/conf"
	"makejob/pkg/auth"
)

// UserService 用户 gRPC 服务实现
type UserService struct {
	userv1.UnimplementedUserServiceServer
	uc        *biz.UserUseCase
	jwtSecret string
	jwtExpire time.Duration
	blacklist biz.TokenBlacklist
	logger    *log.Helper
}

// NewUserService 创建用户服务实例
func NewUserService(uc *biz.UserUseCase, jwtCfg *conf.JWT, blacklist biz.TokenBlacklist, logger log.Logger) *UserService {
	return &UserService{
		uc:        uc,
		jwtSecret: jwtCfg.Secret,
		jwtExpire: time.Duration(jwtCfg.ExpireHours) * time.Hour,
		blacklist: blacklist,
		logger:    log.NewHelper(logger),
	}
}

// Register 用户注册，返回 access_token 和 refresh_token
func (s *UserService) Register(ctx context.Context, req *userv1.RegisterRequest) (*userv1.AuthResponse, error) {
	user, err := s.uc.Register(ctx, req.Username, req.Password, req.Email)
	if err != nil {
		return nil, err
	}

	accessToken, err := auth.GenerateToken(uint64(user.ID), user.Email, user.Role, s.jwtSecret, s.jwtExpire)
	if err != nil {
		return nil, err
	}

	refreshToken, err := auth.GenerateToken(uint64(user.ID), user.Email, user.Role, s.jwtSecret, 7*24*time.Hour)
	if err != nil {
		return nil, err
	}

	return &userv1.AuthResponse{
		Token:        accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(s.jwtExpire).Unix(),
		User:         toProtoUser(user),
	}, nil
}

// Login 用户登录，返回 access_token 和 refresh_token
func (s *UserService) Login(ctx context.Context, req *userv1.LoginRequest) (*userv1.AuthResponse, error) {
	user, err := s.uc.Login(ctx, req.Email, req.Password)
	if err != nil {
		return nil, err
	}

	accessToken, err := auth.GenerateToken(uint64(user.ID), user.Email, user.Role, s.jwtSecret, s.jwtExpire)
	if err != nil {
		return nil, err
	}

	refreshToken, err := auth.GenerateToken(uint64(user.ID), user.Email, user.Role, s.jwtSecret, 7*24*time.Hour)
	if err != nil {
		return nil, err
	}

	return &userv1.AuthResponse{
		Token:        accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(s.jwtExpire).Unix(),
		User:         toProtoUser(user),
	}, nil
}

// RefreshToken 使用 refresh_token 换取新的 token 对（对齐单体：验证用户仍存在）
func (s *UserService) RefreshToken(ctx context.Context, req *userv1.RefreshTokenRequest) (*userv1.TokenResponse, error) {
	claims, err := auth.ParseToken(req.RefreshToken, s.jwtSecret)
	if err != nil {
		return nil, errors.Unauthorized("INVALID_REFRESH_TOKEN", "refresh token 无效或已过期")
	}

	// 对齐单体：验证用户仍存在，防止已删除用户刷新 token
	if _, err := s.uc.GetUserByID(ctx, claims.UserID); err != nil {
		return nil, errors.Unauthorized("USER_NOT_FOUND", "用户不存在或已被删除")
	}

	accessToken, err := auth.GenerateToken(claims.UserID, claims.Email, claims.Role, s.jwtSecret, s.jwtExpire)
	if err != nil {
		return nil, err
	}

	refreshToken, err := auth.GenerateToken(claims.UserID, claims.Email, claims.Role, s.jwtSecret, 7*24*time.Hour)
	if err != nil {
		return nil, err
	}

	return &userv1.TokenResponse{
		Token:        accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(s.jwtExpire).Unix(),
	}, nil
}

// Logout 用户登出，将 access_token 和 refresh_token 的 JTI 加入黑名单
func (s *UserService) Logout(ctx context.Context, req *userv1.LogoutRequest) (*userv1.LogoutResponse, error) {
	// 尝试将 access_token 的 JTI 加入黑名单
	if req.AccessToken != "" {
		if claims, err := auth.ParseToken(req.AccessToken, s.jwtSecret); err == nil {
			ttl := time.Until(claims.ExpiresAt.Time)
			if ttl > 0 {
				if err := s.blacklist.Add(ctx, claims.ID, ttl); err != nil {
					s.logger.Warnw("msg", "access_token 加入黑名单失败", "err", err)
				}
			}
		}
	}

	// 尝试将 refresh_token 的 JTI 加入黑名单
	if req.RefreshToken != "" {
		if claims, err := auth.ParseToken(req.RefreshToken, s.jwtSecret); err == nil {
			ttl := time.Until(claims.ExpiresAt.Time)
			if ttl > 0 {
				if err := s.blacklist.Add(ctx, claims.ID, ttl); err != nil {
					s.logger.Warnw("msg", "refresh_token 加入黑名单失败", "err", err)
				}
			}
		}
	}

	return &userv1.LogoutResponse{}, nil
}

func (s *UserService) GetProfile(ctx context.Context, req *userv1.UserIDRequest) (*userv1.UserProfile, error) {
	user, err := s.uc.GetUserByID(ctx, req.UserId)
	if err != nil {
		return nil, err
	}
	return toProtoUser(user), nil
}

func (s *UserService) UpdateProfile(ctx context.Context, req *userv1.UpdateProfileRequest) (*userv1.UserProfile, error) {
	// FIX U3: 校验当前登录用户与目标用户一致，禁止修改他人资料
	currentUserID := auth.GetUserIDFromContext(ctx)
	if currentUserID == 0 {
		return nil, errors.Unauthorized("UNAUTHORIZED", "未授权")
	}
	if req.UserId != 0 && req.UserId != currentUserID {
		return nil, errors.Forbidden("FORBIDDEN", "无权修改他人资料")
	}

	user, err := s.uc.UpdateProfile(ctx, currentUserID, req.Username, req.Avatar)
	if err != nil {
		return nil, err
	}
	return toProtoUser(user), nil
}

func (s *UserService) GetUserByID(ctx context.Context, req *userv1.UserIDRequest) (*userv1.UserProfile, error) {
	user, err := s.uc.GetUserByID(ctx, req.UserId)
	if err != nil {
		return nil, err
	}
	return toProtoUser(user), nil
}

func (s *UserService) BatchGetUsers(ctx context.Context, req *userv1.BatchGetUsersRequest) (*userv1.BatchGetUsersResponse, error) {
	users, err := s.uc.BatchGetUsers(ctx, req.Ids)
	if err != nil {
		return nil, err
	}
	items := make([]*userv1.UserProfile, len(users))
	for i, u := range users {
		items[i] = toProtoUser(u)
	}
	return &userv1.BatchGetUsersResponse{Users: items}, nil
}

func (s *UserService) GetMembershipStatus(ctx context.Context, req *userv1.UserIDRequest) (*userv1.MembershipStatus, error) {
	level, expireAt, isActive, err := s.uc.GetMembershipStatus(ctx, req.UserId)
	if err != nil {
		return nil, err
	}
	resp := &userv1.MembershipStatus{
		Level:    level,
		IsActive: isActive,
	}
	if expireAt != nil {
		resp.ExpireAt = timestamppb.New(*expireAt)
	}
	return resp, nil
}

func (s *UserService) UpgradeMembership(ctx context.Context, req *userv1.UpgradeRequest) (*userv1.MembershipStatus, error) {
	if err := s.uc.UpgradeMembership(ctx, req.UserId, req.Plan); err != nil {
		return nil, err
	}
	return s.GetMembershipStatus(ctx, &userv1.UserIDRequest{UserId: req.UserId})
}

// AdminListUsers 管理后台分页查询用户。
func (s *UserService) AdminListUsers(ctx context.Context, req *userv1.AdminListUsersRequest) (*userv1.AdminListUsersResponse, error) {
	page, pageSize := int32(1), int32(20)
	if req.GetPage() != nil {
		page = req.GetPage().GetPage()
		pageSize = req.GetPage().GetPageSize()
	}

	users, total, err := s.uc.ListUsers(ctx, page, pageSize)
	if err != nil {
		return nil, err
	}

	items := make([]*userv1.AdminUserInfo, len(users))
	for i, user := range users {
		items[i] = toProtoAdminUser(user)
	}

	return &userv1.AdminListUsersResponse{
		Users: items,
		PageResult: &sharedv1.PageResult{
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		},
	}, nil
}

// AdminUpdateUserRole 管理后台更新用户角色。
func (s *UserService) AdminUpdateUserRole(ctx context.Context, req *userv1.AdminUpdateUserRoleRequest) (*userv1.AdminUpdateUserRoleResponse, error) {
	if err := s.uc.UpdateUserRole(ctx, req.GetUserId(), req.GetRole()); err != nil {
		return nil, err
	}
	return &userv1.AdminUpdateUserRoleResponse{}, nil
}

// AdminBanUser 管理后台切换用户封禁状态。
func (s *UserService) AdminBanUser(ctx context.Context, req *userv1.AdminBanUserRequest) (*userv1.AdminBanUserResponse, error) {
	if err := s.uc.ToggleUserBan(ctx, req.GetUserId()); err != nil {
		return nil, err
	}
	return &userv1.AdminBanUserResponse{}, nil
}

// GetAdminUserStats 管理后台获取用户统计指标。
func (s *UserService) GetAdminUserStats(ctx context.Context, _ *userv1.GetAdminUserStatsRequest) (*userv1.AdminUserStatsResponse, error) {
	totalUsers, proMembers, newUsersToday, todayActiveUsers, err := s.uc.GetAdminStats(ctx)
	if err != nil {
		return nil, err
	}
	return &userv1.AdminUserStatsResponse{
		TotalUsers:       totalUsers,
		ProMembers:       proMembers,
		NewUsersToday:    newUsersToday,
		TodayActiveUsers: todayActiveUsers,
	}, nil
}

// --- 辅助函数 ---

func toProtoUser(u *biz.User) *userv1.UserProfile {
	pb := &userv1.UserProfile{
		Id:              uint64(u.ID),
		Username:        u.Username,
		Email:           u.Email,
		Avatar:          u.Avatar,
		Role:            u.Role,
		MembershipLevel: u.MembershipLevel,
		CreatedAt:       timestamppb.New(u.CreatedAt),
	}
	if u.MembershipExpireAt != nil {
		pb.MembershipExpireAt = timestamppb.New(*u.MembershipExpireAt)
	}
	return pb
}

// toProtoAdminUser 将用户实体转换为管理后台专用响应。
func toProtoAdminUser(u *biz.User) *userv1.AdminUserInfo {
	pb := &userv1.AdminUserInfo{
		Id:              uint64(u.ID),
		Username:        u.Username,
		Email:           u.Email,
		Role:            u.Role,
		Avatar:          u.Avatar,
		MembershipLevel: u.MembershipLevel,
		MembershipType:  u.MembershipType,
		IsDisabled:      u.IsDisabled,
		CreatedAt:       timestamppb.New(u.CreatedAt),
	}
	if u.MembershipExpireAt != nil {
		pb.MembershipExpireAt = timestamppb.New(*u.MembershipExpireAt)
	}
	return pb
}
