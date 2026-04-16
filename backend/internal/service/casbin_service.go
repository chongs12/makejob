// Package service 提供业务逻辑层实现
package service

import (
	"fmt"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"

	"makejob-backend/internal/config"
)

// CasbinService Casbin权限服务接口
type CasbinService interface {
	// InitPolicies 初始化默认权限策略
	InitPolicies() error
	// CheckPermission 检查权限
	CheckPermission(role, path, method string) (bool, error)
	// GetEnforcer 获取Casbin执行器
	GetEnforcer() *casbin.Enforcer
	// AddPolicy 动态添加权限策略
	AddPolicy(role, path, method string) error
	// RemovePolicy 移除权限策略
	RemovePolicy(role, path, method string) error
	// AddRoleForUser 为用户添加角色
	AddRoleForUser(user, role string) error
	// RemoveRoleForUser 移除用户角色
	RemoveRoleForUser(user, role string) error
}

// casbinService Casbin权限服务实现
type casbinService struct {
	enforcer *casbin.Enforcer
}

// NewCasbinService 创建Casbin服务实例
func NewCasbinService(cfg *config.Config) (CasbinService, error) {
	// 加载模型配置
	m, err := model.NewModelFromFile(cfg.Casbin.ModelPath)
	if err != nil {
		// 如果文件不存在，使用默认模型
		m, err = model.NewModelFromString(defaultRBACModel)
		if err != nil {
			return nil, fmt.Errorf("加载Casbin模型失败: %w", err)
		}
	}

	// 创建执行器
	e, err := casbin.NewEnforcer(m)
	if err != nil {
		return nil, fmt.Errorf("创建Casbin执行器失败: %w", err)
	}

	service := &casbinService{
		enforcer: e,
	}

	// 初始化默认策略
	if err := service.InitPolicies(); err != nil {
		return nil, fmt.Errorf("初始化权限策略失败: %w", err)
	}

	return service, nil
}

// defaultRBACModel 默认RBAC模型配置
const defaultRBACModel = `
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && keyMatch(r.obj, p.obj) && (r.act == p.act || p.act == "*")
`

// InitPolicies 初始化默认权限策略
func (s *casbinService) InitPolicies() error {
	// 定义角色
	roles := []struct {
		role        string
		inheritFrom string
	}{
		{"admin", ""},
		{"pro_member", ""},
		{"free_member", ""},
	}

	// 添加角色继承关系（如果存在）
	for _, r := range roles {
		if r.inheritFrom != "" {
			_, _ = s.enforcer.AddGroupingPolicy(r.role, r.inheritFrom)
		}
	}

	// 管理员权限：可访问所有 /api/admin/* 接口
	_, _ = s.enforcer.AddPolicy("admin", "/api/admin/*", "*")

	// Pro会员权限：可访问所有用户接口，无每日限制
	// 用户资料相关
	_, _ = s.enforcer.AddPolicy("pro_member", "/api/user/profile", "GET")
	_, _ = s.enforcer.AddPolicy("pro_member", "/api/user/profile", "PUT")
	_, _ = s.enforcer.AddPolicy("pro_member", "/api/user/password", "PUT")
	_, _ = s.enforcer.AddPolicy("pro_member", "/api/user/favorites/*", "*")
	_, _ = s.enforcer.AddPolicy("pro_member", "/api/user/notes/*", "*")
	_, _ = s.enforcer.AddPolicy("pro_member", "/api/user/records/*", "*")
	// 刷题相关（无限制）
	_, _ = s.enforcer.AddPolicy("pro_member", "/api/practice/*", "*")
	_, _ = s.enforcer.AddPolicy("pro_member", "/api/questions/*", "GET")
	// 模拟面试相关（无限制）
	_, _ = s.enforcer.AddPolicy("pro_member", "/api/interview/*", "*")
	// 学习计划相关
	_, _ = s.enforcer.AddPolicy("pro_member", "/api/plan/*", "*")
	// AI陪伴相关
	_, _ = s.enforcer.AddPolicy("pro_member", "/api/companion/*", "*")
	// 会员相关
	_, _ = s.enforcer.AddPolicy("pro_member", "/api/membership/*", "GET")

	// 免费会员权限：可访问基础用户接口，有每日限制
	// 用户资料相关
	_, _ = s.enforcer.AddPolicy("free_member", "/api/user/profile", "GET")
	_, _ = s.enforcer.AddPolicy("free_member", "/api/user/profile", "PUT")
	_, _ = s.enforcer.AddPolicy("free_member", "/api/user/password", "PUT")
	// 刷题相关（有限制，限制逻辑在业务层实现）
	_, _ = s.enforcer.AddPolicy("free_member", "/api/practice/*", "GET")
	_, _ = s.enforcer.AddPolicy("free_member", "/api/practice/*", "POST")
	_, _ = s.enforcer.AddPolicy("free_member", "/api/questions/*", "GET")
	// 模拟面试（有限制）
	_, _ = s.enforcer.AddPolicy("free_member", "/api/interview/*", "GET")
	_, _ = s.enforcer.AddPolicy("free_member", "/api/interview/*", "POST")
	// 学习计划（只读）
	_, _ = s.enforcer.AddPolicy("free_member", "/api/plan/*", "GET")
	// AI陪伴（有限制）
	_, _ = s.enforcer.AddPolicy("free_member", "/api/companion/*", "GET")
	_, _ = s.enforcer.AddPolicy("free_member", "/api/companion/*", "POST")
	// 会员相关
	_, _ = s.enforcer.AddPolicy("free_member", "/api/membership/*", "GET")

	return nil
}

// CheckPermission 检查权限
func (s *casbinService) CheckPermission(role, path, method string) (bool, error) {
	return s.enforcer.Enforce(role, path, method)
}

// GetEnforcer 获取Casbin执行器
func (s *casbinService) GetEnforcer() *casbin.Enforcer {
	return s.enforcer
}

// AddPolicy 动态添加权限策略
func (s *casbinService) AddPolicy(role, path, method string) error {
	_, err := s.enforcer.AddPolicy(role, path, method)
	return err
}

// RemovePolicy 移除权限策略
func (s *casbinService) RemovePolicy(role, path, method string) error {
	_, err := s.enforcer.RemovePolicy(role, path, method)
	return err
}

// AddRoleForUser 为用户添加角色
func (s *casbinService) AddRoleForUser(user, role string) error {
	_, err := s.enforcer.AddGroupingPolicy(user, role)
	return err
}

// RemoveRoleForUser 移除用户角色
func (s *casbinService) RemoveRoleForUser(user, role string) error {
	_, err := s.enforcer.RemoveGroupingPolicy(user, role)
	return err
}
