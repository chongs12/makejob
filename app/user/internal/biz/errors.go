package biz

import (
	kratosErr "github.com/go-kratos/kratos/v2/errors"
)

var (
	ErrUserNotFound    = kratosErr.NotFound("USER_NOT_FOUND", "用户不存在")
	ErrInvalidPassword = kratosErr.BadRequest("INVALID_PASSWORD", "密码错误")
	ErrEmailExists     = kratosErr.Conflict("EMAIL_EXISTS", "邮箱已注册")
	ErrUsernameExists  = kratosErr.Conflict("USERNAME_EXISTS", "用户名已存在")
	ErrUnauthorized    = kratosErr.Unauthorized("UNAUTHORIZED", "未授权")
)
