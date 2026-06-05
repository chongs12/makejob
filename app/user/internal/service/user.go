package service

import (
	"context"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	userv1 "makejob/api/makejob/user/v1"
	"makejob/app/user/internal/biz"
	"makejob/app/user/internal/conf"
	"makejob/pkg/auth"
)

type UserService struct {
	userv1.UnimplementedUserServiceServer
	uc         *biz.UserUseCase
	jwtSecret  string
	jwtExpire  time.Duration
}

func NewUserService(uc *biz.UserUseCase, jwtCfg *conf.JWT) *UserService {
	return &UserService{
		uc:        uc,
		jwtSecret: jwtCfg.Secret,
		jwtExpire: time.Duration(jwtCfg.ExpireHours) * time.Hour,
	}
}

func (s *UserService) Register(ctx context.Context, req *userv1.RegisterRequest) (*userv1.AuthResponse, error) {
	user, err := s.uc.Register(ctx, req.Username, req.Password, req.Email)
	if err != nil {
		return nil, err
	}

	token, err := auth.GenerateToken(user.ID, user.Email, user.Role, s.jwtSecret, s.jwtExpire)
	if err != nil {
		return nil, err
	}

	return &userv1.AuthResponse{
		Token:     token,
		ExpiresAt: time.Now().Add(s.jwtExpire).Unix(),
		User:      toProtoUser(user),
	}, nil
}

func (s *UserService) Login(ctx context.Context, req *userv1.LoginRequest) (*userv1.AuthResponse, error) {
	user, err := s.uc.Login(ctx, req.Email, req.Password)
	if err != nil {
		return nil, err
	}

	token, err := auth.GenerateToken(user.ID, user.Email, user.Role, s.jwtSecret, s.jwtExpire)
	if err != nil {
		return nil, err
	}

	return &userv1.AuthResponse{
		Token:     token,
		ExpiresAt: time.Now().Add(s.jwtExpire).Unix(),
		User:      toProtoUser(user),
	}, nil
}

func (s *UserService) RefreshToken(ctx context.Context, req *userv1.RefreshTokenRequest) (*userv1.TokenResponse, error) {
	// 解析旧 token 获取用户信息
	claims, err := auth.ParseToken(req.RefreshToken, s.jwtSecret)
	if err != nil {
		return nil, err
	}

	token, err := auth.GenerateToken(claims.UserID, claims.Email, claims.Role, s.jwtSecret, s.jwtExpire)
	if err != nil {
		return nil, err
	}

	return &userv1.TokenResponse{
		Token:        token,
		RefreshToken: token,
		ExpiresAt:    time.Now().Add(s.jwtExpire).Unix(),
	}, nil
}

func (s *UserService) GetProfile(ctx context.Context, req *userv1.UserIDRequest) (*userv1.UserProfile, error) {
	user, err := s.uc.GetUserByID(ctx, req.UserId)
	if err != nil {
		return nil, err
	}
	return toProtoUser(user), nil
}

func (s *UserService) UpdateProfile(ctx context.Context, req *userv1.UpdateProfileRequest) (*userv1.UserProfile, error) {
	user, err := s.uc.UpdateProfile(ctx, req.UserId, req.Username, req.Avatar)
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

// --- 辅助函数 ---

func toProtoUser(u *biz.User) *userv1.UserProfile {
	pb := &userv1.UserProfile{
		Id:              u.ID,
		Username:        u.Username,
		Email:           u.Email,
		Role:            u.Role,
		MembershipLevel: u.MembershipLevel,
		CreatedAt:       timestamppb.New(u.CreatedAt),
	}
	if u.MembershipExpireAt != nil {
		pb.MembershipExpireAt = timestamppb.New(*u.MembershipExpireAt)
	}
	return pb
}
