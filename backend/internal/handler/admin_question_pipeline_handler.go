package handler

import (
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
