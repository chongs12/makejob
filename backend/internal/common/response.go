// Package common 提供统一的API响应格式
package common

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response 统一API响应结构
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// PageResult 分页查询结果结构
type PageResult struct {
	List     interface{} `json:"list"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}

// PageParam 分页参数结构
type PageParam struct {
	Page     int `form:"page" json:"page"`
	PageSize int `form:"page_size" json:"page_size"`
}

// GetOffset 计算分页偏移量
func (p PageParam) GetOffset() int {
	if p.Page <= 0 {
		p.Page = 1
	}
	if p.PageSize <= 0 {
		p.PageSize = 10
	}
	return (p.Page - 1) * p.PageSize
}

// GetLimit 获取分页限制数量
func (p PageParam) GetLimit() int {
	if p.PageSize <= 0 {
		p.PageSize = 10
	}
	if p.PageSize > 100 {
		p.PageSize = 100 // 最大限制100条
	}
	return p.PageSize
}

// Normalize 规范化分页参数
func (p *PageParam) Normalize() {
	if p.Page <= 0 {
		p.Page = 1
	}
	if p.PageSize <= 0 {
		p.PageSize = 10
	}
	if p.PageSize > 100 {
		p.PageSize = 100
	}
}

// Success 返回成功响应
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    CodeSuccess,
		Message: "success",
		Data:    data,
	})
}

// SuccessWithMessage 返回带自定义消息的成功响应
func SuccessWithMessage(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    CodeSuccess,
		Message: message,
		Data:    data,
	})
}

// Error 返回错误响应
func Error(c *gin.Context, code int, message string) {
	c.JSON(http.StatusOK, Response{
		Code:    code,
		Message: message,
		Data:    nil,
	})
}

// ErrorWithHTTPStatus 返回带HTTP状态码的错误响应
func ErrorWithHTTPStatus(c *gin.Context, httpStatus int, code int, message string) {
	c.JSON(httpStatus, Response{
		Code:    code,
		Message: message,
		Data:    nil,
	})
}

// ErrorWithCode 根据错误码返回错误响应
func ErrorWithCode(c *gin.Context, code int) {
	Error(c, code, GetMessage(code))
}

// SuccessWithPage 返回分页成功响应
func SuccessWithPage(c *gin.Context, list interface{}, total int64, page, pageSize int) {
	c.JSON(http.StatusOK, Response{
		Code:    CodeSuccess,
		Message: "success",
		Data: PageResult{
			List:     list,
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		},
	})
}

// BadRequest 返回参数错误响应
func BadRequest(c *gin.Context, message string) {
	if message == "" {
		message = GetMessage(CodeBadRequest)
	}
	Error(c, CodeBadRequest, message)
}

// Unauthorized 返回未授权响应
func Unauthorized(c *gin.Context, message string) {
	if message == "" {
		message = GetMessage(CodeUnauthorized)
	}
	ErrorWithHTTPStatus(c, http.StatusUnauthorized, CodeUnauthorized, message)
}

// Forbidden 返回禁止访问响应
func Forbidden(c *gin.Context, message string) {
	if message == "" {
		message = GetMessage(CodeForbidden)
	}
	ErrorWithHTTPStatus(c, http.StatusForbidden, CodeForbidden, message)
}

// NotFound 返回资源不存在响应
func NotFound(c *gin.Context, message string) {
	if message == "" {
		message = GetMessage(CodeNotFound)
	}
	ErrorWithHTTPStatus(c, http.StatusNotFound, CodeNotFound, message)
}

// InternalError 返回服务器内部错误响应
func InternalError(c *gin.Context, message string) {
	if message == "" {
		message = GetMessage(CodeInternalError)
	}
	ErrorWithHTTPStatus(c, http.StatusInternalServerError, CodeInternalError, message)
}
