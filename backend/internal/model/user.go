// Package model 提供数据模型定义
package model

import (
	"time"
)

// UserRole 用户角色枚举
const (
	UserRoleAdmin      = "admin"
	UserRoleProMember  = "pro_member"
	UserRoleFreeMember = "free_member"
)

// MembershipLevel 会员等级枚举
const (
	MembershipLevelFree = "free"
	MembershipLevelPro  = "pro"
)

// User 用户表
type User struct {
	BaseModel
	Username           string     `json:"username" gorm:"size:50;not null;uniqueIndex;comment:用户名"`
	Email              string     `json:"email" gorm:"size:100;not null;uniqueIndex;comment:邮箱"`
	PasswordHash       string     `json:"-" gorm:"size:255;not null;comment:密码哈希"`
	Avatar             string     `json:"avatar" gorm:"size:500;comment:头像URL"`
	Role               string     `json:"role" gorm:"size:20;not null;default:'free_member';comment:角色(admin/pro_member/free_member)"`
	MembershipLevel    string     `json:"membership_level" gorm:"size:10;not null;default:'free';comment:会员等级(free/pro)"`
	MembershipExpireAt *time.Time `json:"membership_expire_at" gorm:"comment:会员过期时间"`
}

// TableName 指定表名
func (User) TableName() string {
	return "users"
}

// IsPro 检查用户是否为Pro会员
func (u *User) IsPro() bool {
	if u.MembershipLevel != MembershipLevelPro {
		return false
	}
	if u.MembershipExpireAt == nil {
		return false
	}
	return u.MembershipExpireAt.After(time.Now())
}

// IsAdmin 检查用户是否为管理员
func (u *User) IsAdmin() bool {
	return u.Role == UserRoleAdmin
}
