package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims JWT 声明
type Claims struct {
	UserID uint64 `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// GenerateToken 生成 JWT token
func GenerateToken(userID uint64, email, role, secret string, expireDuration time.Duration) (string, error) {
	claims := Claims{
		UserID: userID,
		Email:  email,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expireDuration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "makejob",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ParseToken 解析 JWT token（验证签名方法防止 alg=none 攻击）
func ParseToken(tokenString string, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// 验证签名方法，防止 alg=none 攻击
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, jwt.ErrSignatureInvalid
}

// ContextKey context key 类型
type ContextKey string

const (
	ContextKeyUserID ContextKey = "user_id"
	ContextKeyRole   ContextKey = "role"
	ContextKeyEmail  ContextKey = "email"
)

// GetUserIDFromContext 从 context 获取用户 ID
func GetUserIDFromContext(ctx context.Context) uint64 {
	if v := ctx.Value(ContextKeyUserID); v != nil {
		if id, ok := v.(uint64); ok {
			return id
		}
	}
	return 0
}

// GetRoleFromContext 从 context 获取用户角色
func GetRoleFromContext(ctx context.Context) string {
	if v := ctx.Value(ContextKeyRole); v != nil {
		if role, ok := v.(string); ok {
			return role
		}
	}
	return ""
}
