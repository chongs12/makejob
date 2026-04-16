// Package handler 提供HTTP请求处理层
package handler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"makejob-backend/internal/common"
	"makejob-backend/internal/middleware"
	"makejob-backend/internal/service"
	applogger "makejob-backend/pkg/logger"

	"go.uber.org/zap"
)

// InterviewHandler 面试处理器
type InterviewHandler struct {
	interviewService service.InterviewService
}

// NewInterviewHandler 创建面试处理器实例
func NewInterviewHandler(svc service.InterviewService) *InterviewHandler {
	return &InterviewHandler{
		interviewService: svc,
	}
}

// RegisterRoutes 注册面试相关路由
func (h *InterviewHandler) RegisterRoutes(protected *gin.RouterGroup) {
	interviews := protected.Group("/interviews")
	{
		interviews.POST("", h.CreateInterview)
		interviews.GET("", h.ListInterviews)
		interviews.GET("/:id", h.GetInterview)
		interviews.POST("/:id/answer", h.SubmitAnswer)
		interviews.GET("/:id/next", h.GetNextQuestion)
		interviews.POST("/:id/finish", h.FinishInterview)
		interviews.GET("/:id/report", h.GetReport)
		interviews.GET("/:id/ws", h.WebSocket)
	}
}

// CreateInterview 创建面试会话
// @Summary 创建面试会话
// @Description 创建新的模拟面试会话
// @Tags 面试
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body service.CreateInterviewRequest true "面试配置"
// @Success 200 {object} common.Response{data=service.InterviewResponse}
// @Failure 400 {object} common.Response
// @Router /api/interviews [post]
func (h *InterviewHandler) CreateInterview(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "未登录")
		return
	}

	var req service.CreateInterviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	resp, err := h.interviewService.CreateInterview(c.Request.Context(), userID, &req)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "创建面试失败: "+err.Error())
		}
		return
	}

	common.Success(c, resp)
}

// ListInterviews 获取面试列表
// @Summary 获取面试列表
// @Description 获取当前用户的面试历史列表
// @Tags 面试
// @Accept json
// @Produce json
// @Security Bearer
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Success 200 {object} common.Response{data=common.PageResult}
// @Failure 401 {object} common.Response
// @Router /api/interviews [get]
func (h *InterviewHandler) ListInterviews(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "未登录")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	result, err := h.interviewService.ListInterviews(c.Request.Context(), userID, page, pageSize)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "获取面试列表失败: "+err.Error())
		}
		return
	}

	common.Success(c, result)
}

// GetInterview 获取面试详情
// @Summary 获取面试详情
// @Description 获取指定面试的详细信息
// @Tags 面试
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "面试ID"
// @Success 200 {object} common.Response{data=service.InterviewDetailResponse}
// @Failure 404 {object} common.Response
// @Router /api/interviews/{id} [get]
func (h *InterviewHandler) GetInterview(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "未登录")
		return
	}

	interviewID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "无效的面试ID")
		return
	}

	resp, err := h.interviewService.GetInterview(c.Request.Context(), userID, uint(interviewID))
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "获取面试详情失败: "+err.Error())
		}
		return
	}

	common.Success(c, resp)
}

// SubmitAnswer 提交回答
// @Summary 提交面试回答
// @Description 提交当前题目的回答并获取AI评估
// @Tags 面试
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "面试ID"
// @Param request body service.InterviewAnswerRequest true "回答内容"
// @Success 200 {object} common.Response{data=service.InterviewAnswerResponse}
// @Failure 400 {object} common.Response
// @Router /api/interviews/{id}/answer [post]
func (h *InterviewHandler) SubmitAnswer(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "未登录")
		return
	}

	interviewID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "无效的面试ID")
		return
	}

	var req service.InterviewAnswerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	resp, err := h.interviewService.SubmitAnswer(c.Request.Context(), userID, uint(interviewID), &req)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "提交回答失败: "+err.Error())
		}
		return
	}

	common.Success(c, resp)
}

// GetNextQuestion 获取下一题
// @Summary 获取下一道面试题
// @Description 手动获取下一道面试题目
// @Tags 面试
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "面试ID"
// @Success 200 {object} common.Response{data=service.NextQuestionResponse}
// @Failure 400 {object} common.Response
// @Router /api/interviews/{id}/next [get]
func (h *InterviewHandler) GetNextQuestion(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "未登录")
		return
	}

	interviewID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "无效的面试ID")
		return
	}

	resp, err := h.interviewService.GetNextQuestion(c.Request.Context(), userID, uint(interviewID))
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "获取下一题失败: "+err.Error())
		}
		return
	}

	common.Success(c, resp)
}

// FinishInterview 结束面试
// @Summary 结束面试
// @Description 结束面试并生成报告
// @Tags 面试
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "面试ID"
// @Success 200 {object} common.Response{data=service.InterviewReportResponse}
// @Failure 400 {object} common.Response
// @Router /api/interviews/{id}/finish [post]
func (h *InterviewHandler) FinishInterview(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "未登录")
		return
	}

	interviewID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "无效的面试ID")
		return
	}

	resp, err := h.interviewService.FinishInterview(c.Request.Context(), userID, uint(interviewID))
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "结束面试失败: "+err.Error())
		}
		return
	}

	common.Success(c, resp)
}

// GetReport 获取面试报告
// @Summary 获取面试报告
// @Description 获取已结束面试的详细报告
// @Tags 面试
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "面试ID"
// @Success 200 {object} common.Response{data=service.InterviewReportResponse}
// @Failure 404 {object} common.Response
// @Router /api/interviews/{id}/report [get]
func (h *InterviewHandler) GetReport(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "未登录")
		return
	}

	interviewID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "无效的面试ID")
		return
	}

	resp, err := h.interviewService.GetReport(c.Request.Context(), userID, uint(interviewID))
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "获取面试报告失败: "+err.Error())
		}
		return
	}

	common.Success(c, resp)
}

// WebSocket消息类型定义
type WSMessageType string

const (
	WSMessageTypeUserAnswer WSMessageType = "user_answer"
	WSMessageTypeAIQuestion WSMessageType = "ai_question"
	WSMessageTypeAIFeedback WSMessageType = "ai_feedback"
	WSMessageTypeError      WSMessageType = "error"
	WSMessageTypeConnected  WSMessageType = "connected"
	WSMessageTypeFinished   WSMessageType = "finished"
)

// WebSocket消息结构
type WSMessage struct {
	Type      WSMessageType `json:"type"`
	Content   string        `json:"content,omitempty"`
	Data      interface{}   `json:"data,omitempty"`
	Timestamp int64         `json:"timestamp"`
}

// WebSocket upgrader配置
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// 允许所有来源，生产环境应该根据配置限制
		return true
	},
}

// WebSocket WebSocket实时消息流
// @Summary WebSocket实时面试通信
// @Description 用于面试过程中的实时消息推送，支持发送答案和接收AI回复
// @Tags 面试
// @Security Bearer
// @Param id path int true "面试ID"
// @Router /api/interviews/{id}/ws [get]
func (h *InterviewHandler) WebSocket(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, common.Response{
			Code:    common.CodeUnauthorized,
			Message: "未登录",
		})
		return
	}

	interviewID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, common.Response{
			Code:    common.CodeBadRequest,
			Message: "无效的面试ID",
		})
		return
	}

	// 升级HTTP连接为WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		applogger.Error("WebSocket升级失败", zap.Error(err))
		return
	}
	defer conn.Close()

	// 发送连接成功消息
	sendWSMessage(conn, WSMessage{
		Type:      WSMessageTypeConnected,
		Content:   "连接成功",
		Timestamp: time.Now().Unix(),
	})

	// 主循环处理消息
	for {
		var msg WSMessage
		err := conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				applogger.Error("WebSocket读取错误", zap.Error(err))
			}
			break
		}

		switch msg.Type {
		case WSMessageTypeUserAnswer:
			// 处理用户答案
			h.handleUserAnswer(conn, userID, uint(interviewID), msg.Content)
		default:
			sendWSMessage(conn, WSMessage{
				Type:      WSMessageTypeError,
				Content:   "未知的消息类型: " + string(msg.Type),
				Timestamp: time.Now().Unix(),
			})
		}
	}
}

// handleUserAnswer 处理用户通过WebSocket发送的答案
func (h *InterviewHandler) handleUserAnswer(conn *websocket.Conn, userID, interviewID uint, answer string) {
	req := service.InterviewAnswerRequest{
		Answer: answer,
	}

	resp, err := h.interviewService.SubmitAnswer(context.Background(), userID, interviewID, &req)
	if err != nil {
		sendWSMessage(conn, WSMessage{
			Type:      WSMessageTypeError,
			Content:   err.Error(),
			Timestamp: time.Now().Unix(),
		})
		return
	}

	// 发送AI反馈
	sendWSMessage(conn, WSMessage{
		Type:      WSMessageTypeAIFeedback,
		Content:   resp.Feedback.Feedback,
		Data:      resp.Feedback,
		Timestamp: time.Now().Unix(),
	})

	// 如果有下一题，发送下一题
	if resp.NextQuestion != nil {
		sendWSMessage(conn, WSMessage{
			Type:      WSMessageTypeAIQuestion,
			Content:   resp.NextQuestion.Question,
			Data:      resp.NextQuestion,
			Timestamp: time.Now().Unix(),
		})
	}

	// 如果面试结束，发送结束消息
	if resp.IsFinished {
		sendWSMessage(conn, WSMessage{
			Type:      WSMessageTypeFinished,
			Content:   "面试已完成",
			Timestamp: time.Now().Unix(),
		})
	}
}

// sendWSMessage 发送WebSocket消息
func sendWSMessage(conn *websocket.Conn, msg WSMessage) error {
	if msg.Timestamp == 0 {
		msg.Timestamp = time.Now().Unix()
	}
	err := conn.WriteJSON(msg)
	if err != nil {
		applogger.Error("WebSocket写入错误", zap.Error(err))
	}
	return err
}
