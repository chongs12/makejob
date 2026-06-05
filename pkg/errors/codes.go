package errors

import (
	kratosErr "github.com/go-kratos/kratos/v2/errors"
)

// 通用错误
var (
	ErrUnauthorized = kratosErr.Unauthorized("UNAUTHORIZED", "未授权")
	ErrForbidden    = kratosErr.Forbidden("FORBIDDEN", "禁止访问")
	ErrNotFound     = kratosErr.NotFound("NOT_FOUND", "资源不存在")
	ErrInternal     = kratosErr.InternalServer("INTERNAL", "内部错误")
	ErrBadRequest   = kratosErr.BadRequest("BAD_REQUEST", "请求参数错误")
)

// 用户域错误
var (
	ErrUserNotFound    = kratosErr.NotFound("USER_NOT_FOUND", "用户不存在")
	ErrEmailExists     = kratosErr.Conflict("EMAIL_EXISTS", "邮箱已注册")
	ErrInvalidPassword = kratosErr.BadRequest("INVALID_PASSWORD", "密码错误")
	ErrTokenExpired    = kratosErr.Unauthorized("TOKEN_EXPIRED", "Token 已过期")
	ErrTokenInvalid    = kratosErr.Unauthorized("TOKEN_INVALID", "Token 无效")
)

// 面试域错误
var (
	ErrInterviewNotFound = kratosErr.NotFound("INTERVIEW_NOT_FOUND", "面试不存在")
	ErrInterviewFinished = kratosErr.BadRequest("INTERVIEW_FINISHED", "面试已结束")
	ErrInvalidAnswer     = kratosErr.BadRequest("INVALID_ANSWER", "无效的回答")
)

// 题目域错误
var (
	ErrQuestionNotFound = kratosErr.NotFound("QUESTION_NOT_FOUND", "题目不存在")
	ErrAlreadyFavorited = kratosErr.Conflict("ALREADY_FAVORITED", "已收藏")
	ErrFavoriteNotFound = kratosErr.NotFound("FAVORITE_NOT_FOUND", "收藏不存在")
)

// Wrap 包装错误，附加上下文，保留原始状态码
func Wrap(err error, code int, reason, message string) error {
	return kratosErr.New(code, reason, message).WithCause(err)
}

// WrapInternal 包装为 500 内部错误
func WrapInternal(err error, reason, message string) error {
	return kratosErr.InternalServer(reason, message).WithCause(err)
}
