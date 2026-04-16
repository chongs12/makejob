// Package middleware 提供Gin中间件功能
package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"makejob-backend/internal/common"
	"makejob-backend/internal/model"
	"makejob-backend/internal/service"
)

// MembershipCheck 会员检查中间件
// 检查用户是否为Pro会员，免费用户检查每日使用限额
func MembershipCheck(membershipSvc service.MembershipService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从上下文获取用户ID
		userID, exists := GetUserID(c)
		if !exists {
			common.Unauthorized(c, "未登录")
			c.Abort()
			return
		}

		// 查询会员状态
		status, err := membershipSvc.GetMembershipStatus(c.Request.Context(), userID)
		if err != nil {
			common.InternalError(c, "获取会员状态失败")
			c.Abort()
			return
		}

		// Pro会员直接放行
		if status.Level == model.MembershipLevelPro && status.IsActive {
			c.Next()
			return
		}

		// 免费用户检查每日限额
		path := c.Request.URL.Path

		// 根据请求路径判断是刷题还是面试
		if isPracticePath(path) {
			// 检查刷题限额
			if status.PracticeUsedToday >= status.DailyPracticeLimit {
				common.Forbidden(c, "今日刷题次数已达上限，请升级会员享受无限刷题")
				c.Abort()
				return
			}
		} else if isInterviewPath(path) {
			// 检查面试限额
			if status.InterviewUsedToday >= status.DailyInterviewLimit {
				common.Forbidden(c, "今日模拟面试次数已达上限，请升级会员享受无限面试")
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// MembershipCheckWithType 带类型指定的会员检查中间件
// resourceType: "practice" | "interview"
func MembershipCheckWithType(membershipSvc service.MembershipService, resourceType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从上下文获取用户ID
		userID, exists := GetUserID(c)
		if !exists {
			common.Unauthorized(c, "未登录")
			c.Abort()
			return
		}

		// 查询会员状态
		status, err := membershipSvc.GetMembershipStatus(c.Request.Context(), userID)
		if err != nil {
			common.InternalError(c, "获取会员状态失败")
			c.Abort()
			return
		}

		// Pro会员直接放行
		if status.Level == model.MembershipLevelPro && status.IsActive {
			c.Next()
			return
		}

		// 根据资源类型检查限额
		switch resourceType {
		case "practice":
			if status.PracticeUsedToday >= status.DailyPracticeLimit {
				common.Forbidden(c, "今日刷题次数已达上限，请升级会员享受无限刷题")
				c.Abort()
				return
			}
		case "interview":
			if status.InterviewUsedToday >= status.DailyInterviewLimit {
				common.Forbidden(c, "今日模拟面试次数已达上限，请升级会员享受无限面试")
				c.Abort()
				return
			}
		default:
			// 未知资源类型，放行
		}

		c.Next()
	}
}

// isPracticePath 判断是否为刷题相关路径
func isPracticePath(path string) bool {
	practicePaths := []string{
		"/api/practice",
		"/api/questions",
		"/api/quiz",
	}

	lowerPath := strings.ToLower(path)
	for _, pp := range practicePaths {
		if strings.HasPrefix(lowerPath, pp) {
			return true
		}
	}
	return false
}

// isInterviewPath 判断是否为面试相关路径
func isInterviewPath(path string) bool {
	interviewPaths := []string{
		"/api/interview",
		"/api/mock-interview",
	}

	lowerPath := strings.ToLower(path)
	for _, ip := range interviewPaths {
		if strings.HasPrefix(lowerPath, ip) {
			return true
		}
	}
	return false
}

// RequireProMembership 要求Pro会员中间件
// 只允许Pro会员访问
func RequireProMembership(membershipSvc service.MembershipService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从上下文获取用户ID
		userID, exists := GetUserID(c)
		if !exists {
			common.Unauthorized(c, "未登录")
			c.Abort()
			return
		}

		// 查询会员状态
		status, err := membershipSvc.GetMembershipStatus(c.Request.Context(), userID)
		if err != nil {
			common.InternalError(c, "获取会员状态失败")
			c.Abort()
			return
		}

		// 检查是否为Pro会员
		if status.Level != model.MembershipLevelPro || !status.IsActive {
			common.Forbidden(c, "此功能需要Pro会员")
			c.Abort()
			return
		}

		c.Next()
	}
}

// OptionalMembershipCheck 可选会员检查中间件
// 记录使用情况但不阻止访问，用于统计
func OptionalMembershipCheck(membershipSvc service.MembershipService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从上下文获取用户ID
		userID, exists := GetUserID(c)
		if !exists {
			c.Next()
			return
		}

		// 查询会员状态（用于记录统计）
		status, err := membershipSvc.GetMembershipStatus(c.Request.Context(), userID)
		if err != nil {
			// 记录失败不影响主流程
			c.Next()
			return
		}

		// 将会员状态存入上下文，供后续使用
		c.Set("membership_level", status.Level)
		c.Set("membership_is_active", status.IsActive)
		c.Set("practice_used_today", status.PracticeUsedToday)
		c.Set("interview_used_today", status.InterviewUsedToday)

		c.Next()
	}
}

// GetMembershipLevel 从上下文获取会员等级
func GetMembershipLevel(c *gin.Context) (string, bool) {
	level, exists := c.Get("membership_level")
	if !exists {
		return "", false
	}
	l, ok := level.(string)
	return l, ok
}

// IsProMember 检查是否为Pro会员
func IsProMember(c *gin.Context) bool {
	level, exists := GetMembershipLevel(c)
	if !exists {
		return false
	}
	return level == model.MembershipLevelPro
}
