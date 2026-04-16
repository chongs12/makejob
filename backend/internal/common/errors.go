// Package common 提供通用的工具函数和常量定义
package common

// 错误码常量定义
const (
	// CodeSuccess 成功
	CodeSuccess = 0

	// CodeBadRequest 请求参数错误
	CodeBadRequest = 400

	// CodeUnauthorized 未授权
	CodeUnauthorized = 401

	// CodeForbidden 禁止访问
	CodeForbidden = 403

	// CodeNotFound 资源不存在
	CodeNotFound = 404

	// CodeInternalError 服务器内部错误
	CodeInternalError = 500

	// CodeTokenExpired 令牌已过期
	CodeTokenExpired = 4011

	// CodeTokenInvalid 令牌无效
	CodeTokenInvalid = 4012
)

// 错误消息映射
var codeMessages = map[int]string{
	CodeSuccess:       "成功",
	CodeBadRequest:    "请求参数错误",
	CodeUnauthorized:  "未授权",
	CodeForbidden:     "禁止访问",
	CodeNotFound:      "资源不存在",
	CodeInternalError: "服务器内部错误",
	CodeTokenExpired:  "令牌已过期",
	CodeTokenInvalid:  "令牌无效",
}

// GetMessage 根据错误码获取错误消息
func GetMessage(code int) string {
	if msg, ok := codeMessages[code]; ok {
		return msg
	}
	return "未知错误"
}

// BusinessError 业务错误结构
type BusinessError struct {
	Code    int
	Message string
}

// Error 实现error接口
func (e *BusinessError) Error() string {
	return e.Message
}

// NewBusinessError 创建业务错误
func NewBusinessError(code int, message string) *BusinessError {
	return &BusinessError{
		Code:    code,
		Message: message,
	}
}

// NewBusinessErrorWithCode 根据错误码创建业务错误
func NewBusinessErrorWithCode(code int) *BusinessError {
	return &BusinessError{
		Code:    code,
		Message: GetMessage(code),
	}
}
