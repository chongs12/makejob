// Package middleware 提供Gin中间件功能
package middleware

import (
	"net/http"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"makejob-backend/internal/common"
	"makejob-backend/internal/config"
	"makejob-backend/pkg/logger"
)

// CasbinEnforcer Casbin权限检查器接口
type CasbinEnforcer interface {
	Enforce(rvals ...interface{}) (bool, error)
}

var (
	// 全局Casbin执行器实例
	enforcer CasbinEnforcer
)

// InitCasbin 初始化Casbin权限检查器
// 从配置文件加载RBAC模型
func InitCasbin() (CasbinEnforcer, error) {
	cfg := config.GetConfig()

	// 加载模型配置
	m, err := model.NewModelFromFile(cfg.Casbin.ModelPath)
	if err != nil {
		// 如果文件不存在，使用默认模型
		m, err = model.NewModelFromString(defaultModel)
		if err != nil {
			return nil, err
		}
	}

	// 创建执行器（不使用适配器，策略在运行时动态加载）
	e, err := casbin.NewEnforcer(m)
	if err != nil {
		return nil, err
	}

	// 添加默认策略（实际项目中应从数据库加载）
	// 角色继承关系：用户 -> 角色
	// 权限策略：角色 -> 资源 -> 操作

	// 示例策略：普通用户权限
	e.AddPolicy("user", "/api/user/profile", "GET")
	e.AddPolicy("user", "/api/user/profile", "PUT")

	// 示例策略：管理员权限
	e.AddPolicy("admin", "/api/*", "*")

	enforcer = e
	return e, nil
}

// GetEnforcer 获取Casbin执行器实例
func GetEnforcer() CasbinEnforcer {
	if enforcer == nil {
		var err error
		enforcer, err = InitCasbin()
		if err != nil {
			logger.Error("初始化Casbin失败", zap.Error(err))
			// 返回一个允许所有请求的默认执行器
			return &defaultEnforcer{}
		}
	}
	return enforcer
}

// defaultEnforcer 默认执行器（允许所有请求）
type defaultEnforcer struct{}

func (d *defaultEnforcer) Enforce(rvals ...interface{}) (bool, error) {
	return true, nil
}

// Casbin RBAC模型配置（当文件不存在时使用）
const defaultModel = `
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && r.obj == p.obj && r.act == p.act || r.sub == "admin"
`

// Casbin 权限检查中间件
// 检查当前用户角色是否有权限访问请求的资源
func Casbin() gin.HandlerFunc {
	return func(c *gin.Context) {
		e := GetEnforcer()

		// 获取用户角色
		role, exists := GetRole(c)
		if !exists {
			role = "anonymous" // 未登录用户
		}

		// 获取请求路径和方法
		obj := c.Request.URL.Path
		act := c.Request.Method

		// 检查权限
		allowed, err := e.Enforce(role, obj, act)
		if err != nil {
			logger.Error("Casbin权限检查失败",
				zap.Error(err))
			c.JSON(http.StatusInternalServerError, common.Response{
				Code:    common.CodeInternalError,
				Message: "权限检查失败",
			})
			c.Abort()
			return
		}

		if !allowed {
			common.Forbidden(c, "没有权限执行此操作")
			c.Abort()
			return
		}

		c.Next()
	}
}

// CasbinWithRole 带指定角色的权限检查中间件
// 用于需要特定角色才能访问的接口
func CasbinWithRole(requiredRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		e := GetEnforcer()

		role, exists := GetRole(c)
		if !exists {
			common.Unauthorized(c, "未登录")
			c.Abort()
			return
		}

		// 检查用户是否有所需角色
		// 这里简化处理，实际应查询角色继承关系
		if role != requiredRole && role != "admin" {
			common.Forbidden(c, "需要 "+requiredRole+" 权限")
			c.Abort()
			return
		}

		obj := c.Request.URL.Path
		act := c.Request.Method

		allowed, err := e.Enforce(role, obj, act)
		if err != nil || !allowed {
			common.Forbidden(c, "没有权限执行此操作")
			c.Abort()
			return
		}

		c.Next()
	}
}

// AddPolicy 动态添加权限策略
func AddPolicy(role, path, method string) error {
	e := GetEnforcer()
	if enforcerImpl, ok := e.(*casbin.Enforcer); ok {
		_, err := enforcerImpl.AddPolicy(role, path, method)
		return err
	}
	return nil
}

// AddRoleForUser 为用户添加角色
func AddRoleForUser(user, role string) error {
	e := GetEnforcer()
	if enforcerImpl, ok := e.(*casbin.Enforcer); ok {
		_, err := enforcerImpl.AddGroupingPolicy(user, role)
		return err
	}
	return nil
}
