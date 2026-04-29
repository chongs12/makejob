package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"makejob-backend/internal/common"
	"makejob-backend/internal/service"
)

// GenerateQuestionPipeline 执行后台题目流水线生成候选题卡。
func (h *AdminHandler) GenerateQuestionPipeline(c *gin.Context) {
	var req service.AdminQuestionPipelineGenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	result, err := h.adminService.GenerateQuestionPipeline(c.Request.Context(), &req)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "生成题目流水线候选失败: "+err.Error())
		}
		return
	}

	common.Success(c, result)
}

// GenerateQuestionPipelineAsync 创建一条异步题目流水线生成任务，交给后台 worker 稍后生成候选题卡。
func (h *AdminHandler) GenerateQuestionPipelineAsync(c *gin.Context) {
	var req service.AdminQuestionPipelineGenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	task, err := h.adminService.CreateQuestionPipelineTask(c.Request.Context(), &req)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "创建题目流水线任务失败: "+err.Error())
		}
		return
	}

	common.SuccessWithMessage(c, "题目流水线任务已创建", task)
}

// GenerateQuestionPipelineStream 以 SSE 方式逐步推送后台题目流水线生成结果。
func (h *AdminHandler) GenerateQuestionPipelineStream(c *gin.Context) {
	var req service.AdminQuestionPipelineGenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		common.InternalError(c, "当前服务不支持流式响应")
		return
	}

	headers := c.Writer.Header()
	headers.Set("Content-Type", "text/event-stream; charset=utf-8")
	headers.Set("Cache-Control", "no-cache, no-transform")
	headers.Set("Connection", "keep-alive")
	headers.Set("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	c.Writer.WriteHeaderNow()
	if err := writeQuestionPipelineSSEPrelude(c, flusher); err != nil {
		return
	}
	flusher.Flush()

	writeEvent := func(event service.AdminQuestionPipelineStreamEvent) error {
		select {
		case <-c.Request.Context().Done():
			return c.Request.Context().Err()
		default:
		}

		return writeQuestionPipelineSSEEvent(c, flusher, event)
	}

	if err := h.adminService.GenerateQuestionPipelineStream(c.Request.Context(), &req, writeEvent); err != nil {
		_ = writeQuestionPipelineSSEEvent(c, flusher, service.AdminQuestionPipelineStreamEvent{
			Event:   "error",
			Message: err.Error(),
		})
	}
}

// ImportQuestionPipeline 将后台确认后的候选题卡批量导入题库。
func (h *AdminHandler) ImportQuestionPipeline(c *gin.Context) {
	var req service.AdminQuestionPipelineImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	result, err := h.adminService.ImportQuestionPipeline(c.Request.Context(), &req)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "导入题目流水线候选失败: "+err.Error())
		}
		return
	}

	common.Success(c, result)
}

// writeQuestionPipelineSSEEvent 将题目流水线流式事件编码为标准 SSE 消息并立即刷新输出。
func writeQuestionPipelineSSEEvent(c *gin.Context, flusher http.Flusher, event service.AdminQuestionPipelineStreamEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal sse event failed: %w", err)
	}
	if _, err := c.Writer.Write([]byte(fmt.Sprintf("event: %s\ndata: %s\n\n", event.Event, payload))); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

// writeQuestionPipelineSSEPrelude 先写出一段预热注释，尽量降低代理和浏览器对小块 SSE 数据的缓冲概率。
func writeQuestionPipelineSSEPrelude(c *gin.Context, flusher http.Flusher) error {
	padding := ":" + strings.Repeat(" ", 2048) + "\n\n"
	if _, err := c.Writer.Write([]byte(padding)); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}
