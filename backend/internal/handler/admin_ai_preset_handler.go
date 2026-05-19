// Package handler 提供 HTTP 请求处理层。
package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"makejob-backend/internal/common"
	"makejob-backend/internal/service"
)

// CreateAIPresetRequest 描述创建 AI 预设时提交的完整配置快照。
type CreateAIPresetRequest struct {
	Name    string            `json:"name" binding:"required"`
	Configs map[string]string `json:"configs" binding:"required"`
}

// UpdateAIPresetRequest 描述更新 AI 预设时允许修改的字段。
type UpdateAIPresetRequest struct {
	Name    *string           `json:"name,omitempty"`
	Configs map[string]string `json:"configs,omitempty"`
}

// ListAIPresets 返回 AI 预设列表。
func (h *AdminHandler) ListAIPresets(c *gin.Context) {
	presets, err := h.adminService.ListAIPresets(c.Request.Context())
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "获取 AI 预设失败: "+err.Error())
		}
		return
	}

	common.Success(c, presets)
}

// CreateAIPreset 创建新的 AI 预设。
func (h *AdminHandler) CreateAIPreset(c *gin.Context) {
	var req CreateAIPresetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	preset, err := h.adminService.CreateAIPreset(c.Request.Context(), &service.CreateAIPresetRequest{
		Name:    req.Name,
		Configs: req.Configs,
	})
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "创建 AI 预设失败: "+err.Error())
		}
		return
	}

	common.Success(c, preset)
}

// UpdateAIPreset 更新指定 AI 预设。
func (h *AdminHandler) UpdateAIPreset(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "无效的 AI 预设 ID")
		return
	}

	var req UpdateAIPresetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	preset, err := h.adminService.UpdateAIPreset(c.Request.Context(), uint(id), &service.UpdateAIPresetRequest{
		Name:    req.Name,
		Configs: req.Configs,
	})
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "更新 AI 预设失败: "+err.Error())
		}
		return
	}

	common.Success(c, preset)
}

// DeleteAIPreset 删除指定 AI 预设。
func (h *AdminHandler) DeleteAIPreset(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "无效的 AI 预设 ID")
		return
	}

	if err := h.adminService.DeleteAIPreset(c.Request.Context(), uint(id)); err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "删除 AI 预设失败: "+err.Error())
		}
		return
	}

	common.SuccessWithMessage(c, "删除成功", nil)
}

// ApplyAIPreset 应用指定 AI 预设并覆盖当前全局运行配置。
func (h *AdminHandler) ApplyAIPreset(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "无效的 AI 预设 ID")
		return
	}

	configs, err := h.adminService.ApplyAIPreset(c.Request.Context(), uint(id))
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "应用 AI 预设失败: "+err.Error())
		}
		return
	}

	common.SuccessWithMessage(c, "应用成功", configs)
}
