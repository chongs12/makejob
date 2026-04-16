// Package middleware 提供Gin中间件功能
package middleware

import (
	"errors"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"makejob-backend/internal/common"
	"makejob-backend/internal/config"
)

// ContextKey 定义上下文键类型，避免冲突
type ContextKey string

const (
	// ContextKeyUserID 用户ID上下文键
	ContextKeyUserID ContextKey = "user_id"
	// ContextKeyRole 用户角色上下文键
	ContextKeyRole ContextKey = "role"
	// ContextKeyUsername 用户名上下文键
	ContextKeyUsername ContextKey = "username"
)

// JWTClaims JWT令牌声明结构
type JWTClaims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// Auth JWT认证中间件
// 验证Authorization头中的Bearer Token，并将用户信息存入上下文
func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			common.Unauthorized(c, "缺少Authorization请求头")
			c.Abort()
			return
		}

		// 提取Bearer Token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			common.Unauthorized(c, "Authorization格式错误，应为Bearer {token}")
			c.Abort()
			return
		}

		tokenString := parts[1]
		claims, err := ParseToken(tokenString)
		if err != nil {
			if errors.Is(err, jwt.ErrTokenExpired) {
				common.Error(c, common.CodeTokenExpired, "令牌已过期")
			} else {
				common.Error(c, common.CodeTokenInvalid, "无效的令牌")
			}
			c.Abort()
			return
		}

		// 将用户信息存入上下文
		c.Set(string(ContextKeyUserID), claims.UserID)
		c.Set(string(ContextKeyRole), claims.Role)
		c.Set(string(ContextKeyUsername), claims.Username)

		c.Next()
	}
}

// ParseToken 解析JWT令牌
func ParseToken(tokenString string) (*JWTClaims, error) {
	cfg := config.GetConfig()
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(cfg.JWT.Secret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("无效的令牌声明")
}

// GenerateToken 生成JWT令牌
func GenerateToken(userID uint, username, role string) (string, error) {
	cfg := config.GetConfig()
	now := time.Now()

	claims := JWTClaims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(cfg.JWT.Expire) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "makejob-backend",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.JWT.Secret))
}

// GenerateRefreshToken 生成刷新令牌
func GenerateRefreshToken(userID uint) (string, error) {
	cfg := config.GetConfig()
	now := time.Now()

	claims := jwt.RegisteredClaims{
		Subject:   string(rune(userID)),
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(cfg.JWT.RefreshExpire) * time.Hour)),
		IssuedAt:  jwt.NewNumericDate(now),
		Issuer:    "makejob-backend",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.JWT.Secret))
}

// GetUserID 从上下文获取用户ID
func GetUserID(c *gin.Context) (uint, bool) {
	userID, exists := c.Get(string(ContextKeyUserID))
	if !exists {
		return 0, false
	}
	id, ok := userID.(uint)
	return id, ok
}

// GetRole 从上下文获取用户角色
func GetRole(c *gin.Context) (string, bool) {
	role, exists := c.Get(string(ContextKeyRole))
	if !exists {
		return "", false
	}
	r, ok := role.(string)
	return r, ok
}

// GetUsername 从上下文获取用户名
func GetUsername(c *gin.Context) (string, bool) {
	username, exists := c.Get(string(ContextKeyUsername))
	if !exists {
		return "", false
	}
	u, ok := username.(string)
	return u, ok
}

// OptionalAuth 可选认证中间件
// 验证Token但不强制要求，用于需要获取用户信息但允许匿名访问的接口
func OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.Next()
			return
		}

		tokenString := parts[1]
		claims, err := ParseToken(tokenString)
		if err == nil {
			c.Set(string(ContextKeyUserID), claims.UserID)
			c.Set(string(ContextKeyRole), claims.Role)
			c.Set(string(ContextKeyUsername), claims.Username)
		}

		c.Next()
	}
}
