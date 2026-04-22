// Package handler 提供HTTP请求处理层
package handler

import (
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"makejob-backend/internal/common"
	"makejob-backend/internal/service"
)

// AdminHandler 管理员处理器
type AdminHandler struct {
	adminService service.AdminService
}

// NewAdminHandler 创建管理员处理器实例
func NewAdminHandler(adminService service.AdminService) *AdminHandler {
	return &AdminHandler{
		adminService: adminService,
	}
}

// RegisterRoutes 注册管理员路由
func (h *AdminHandler) RegisterRoutes(r *gin.RouterGroup) {
	// 仪表盘
	r.GET("/dashboard", h.GetDashboard)

	// 用户管理
	r.GET("/users", h.ListUsers)
	r.PUT("/users/:id/role", h.UpdateUserRole)
	r.PUT("/users/:id/disable", h.DisableUser)

	// 题库管理
	r.GET("/questions", h.ListQuestions)
	r.POST("/questions", h.CreateQuestion)
	r.PUT("/questions/:id", h.UpdateQuestion)
	r.DELETE("/questions/:id", h.DeleteQuestion)
	r.POST("/questions/import", h.BatchImportQuestions)
	r.POST("/question-pipeline/generate", h.GenerateQuestionPipeline)
	r.POST("/question-pipeline/import", h.ImportQuestionPipeline)

	// 分类管理
	r.GET("/categories", h.ListCategories)
	r.POST("/categories", h.CreateCategory)
	r.PUT("/categories/:id", h.UpdateCategory)
	r.DELETE("/categories/:id", h.DeleteCategory)

	// 行业管理
	r.GET("/industries", h.ListIndustries)
	r.POST("/industries", h.CreateIndustry)
	r.PUT("/industries/:id", h.UpdateIndustry)

	// Prompt模板管理
	r.GET("/prompts", h.ListPrompts)
	r.POST("/prompts", h.CreatePrompt)
	r.PUT("/prompts/:id", h.UpdatePrompt)
	r.DELETE("/prompts/:id", h.DeletePrompt)

	// AI配置管理
	r.GET("/ai-configs", h.GetAIConfigs)
	r.PUT("/ai-configs", h.UpdateAIConfigs)
	r.POST("/prompts/test-render", h.TestRenderPrompt)
	r.GET("/ai-call-logs", h.ListAICallLogs)

	// Live2D模型管理
	r.GET("/live2d-models", h.ListLive2DModels)
	r.POST("/live2d-models/import", h.ImportLive2DPackage)
	r.POST("/live2d-models", h.CreateLive2DModel)
	r.PUT("/live2d-models/:id", h.UpdateLive2DModel)
	r.DELETE("/live2d-models/:id", h.DeleteLive2DModel)

	// TTS配置管理
	r.GET("/tts-configs", h.ListTTSConfigs)
	r.POST("/tts-configs", h.CreateTTSConfig)
	r.PUT("/tts-configs/:id", h.UpdateTTSConfig)
	r.DELETE("/tts-configs/:id", h.DeleteTTSConfig)
}

// ==================== 仪表盘 ====================

// GetDashboard 获取仪表盘数据
// @Summary 获取仪表盘数据
// @Description 获取管理后台仪表盘统计数据
// @Tags 管理后台
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} common.Response{data=service.DashboardResponse}
// @Router /api/admin/dashboard [get]
func (h *AdminHandler) GetDashboard(c *gin.Context) {
	dashboard, err := h.adminService.GetDashboard(c.Request.Context())
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "获取仪表盘数据失败: "+err.Error())
		}
		return
	}

	common.Success(c, dashboard)
}

// ==================== 用户管理 ====================

// ListUsers 获取用户列表
// @Summary 获取用户列表
// @Description 分页获取用户列表，支持关键词搜索和角色过滤
// @Tags 管理后台-用户管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Param keyword query string false "搜索关键词"
// @Param role query string false "角色过滤"
// @Success 200 {object} common.Response{data=common.PageResult}
// @Router /api/admin/users [get]
func (h *AdminHandler) ListUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	keyword := c.Query("keyword")
	role := c.Query("role")

	result, err := h.adminService.ListUsers(c.Request.Context(), page, pageSize, keyword, role)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "获取用户列表失败: "+err.Error())
		}
		return
	}

	common.Success(c, result)
}

// UpdateUserRoleRequest 更新用户角色请求
type UpdateUserRoleRequest struct {
	Role string `json:"role" binding:"required"`
}

// UpdateUserRole 更新用户角色
// @Summary 更新用户角色
// @Description 更新指定用户的角色
// @Tags 管理后台-用户管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "用户ID"
// @Param request body UpdateUserRoleRequest true "角色信息"
// @Success 200 {object} common.Response
// @Router /api/admin/users/{id}/role [put]
func (h *AdminHandler) UpdateUserRole(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "无效的用户ID")
		return
	}

	var req UpdateUserRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	if err := h.adminService.UpdateUserRole(c.Request.Context(), uint(userID), req.Role); err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "更新用户角色失败: "+err.Error())
		}
		return
	}

	common.SuccessWithMessage(c, "更新成功", nil)
}

// DisableUser 禁用用户
// @Summary 禁用用户
// @Description 禁用指定用户
// @Tags 管理后台-用户管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "用户ID"
// @Success 200 {object} common.Response
// @Router /api/admin/users/{id}/disable [put]
func (h *AdminHandler) DisableUser(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "无效的用户ID")
		return
	}

	if err := h.adminService.DisableUser(c.Request.Context(), uint(userID)); err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "禁用用户失败: "+err.Error())
		}
		return
	}

	common.SuccessWithMessage(c, "禁用成功", nil)
}

// ==================== 题库管理 ====================

// CreateQuestion 创建题目
// @Summary 创建题目
// @Description 创建新题目
// @Tags 管理后台-题库管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body service.AdminCreateQuestionRequest true "题目信息"
// @Success 200 {object} common.Response{data=model.Question}
// @Router /api/admin/questions [post]
func (h *AdminHandler) ListQuestions(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	keyword := c.Query("keyword")
	difficulty := c.Query("difficulty")

	var categoryID uint
	if categoryIDStr := c.Query("category_id"); categoryIDStr != "" {
		id, err := strconv.ParseUint(categoryIDStr, 10, 32)
		if err != nil {
			common.BadRequest(c, "无效的分类ID")
			return
		}
		categoryID = uint(id)
	}

	result, err := h.adminService.ListQuestions(c.Request.Context(), page, pageSize, keyword, difficulty, categoryID)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "获取题库列表失败: "+err.Error())
		}
		return
	}

	common.Success(c, result)
}

func (h *AdminHandler) CreateQuestion(c *gin.Context) {
	var req service.AdminCreateQuestionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	question, err := h.adminService.CreateQuestion(c.Request.Context(), &req)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "创建题目失败: "+err.Error())
		}
		return
	}

	common.Success(c, question)
}

// UpdateQuestion 更新题目
// @Summary 更新题目
// @Description 更新指定题目
// @Tags 管理后台-题库管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "题目ID"
// @Param request body service.AdminUpdateQuestionRequest true "题目信息"
// @Success 200 {object} common.Response
// @Router /api/admin/questions/{id} [put]
func (h *AdminHandler) UpdateQuestion(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "无效的题目ID")
		return
	}

	var req service.AdminUpdateQuestionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	if err := h.adminService.UpdateQuestion(c.Request.Context(), uint(id), &req); err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "更新题目失败: "+err.Error())
		}
		return
	}

	common.SuccessWithMessage(c, "更新成功", nil)
}

// DeleteQuestion 删除题目
// @Summary 删除题目
// @Description 删除指定题目
// @Tags 管理后台-题库管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "题目ID"
// @Success 200 {object} common.Response
// @Router /api/admin/questions/{id} [delete]
func (h *AdminHandler) DeleteQuestion(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "无效的题目ID")
		return
	}

	if err := h.adminService.DeleteQuestion(c.Request.Context(), uint(id)); err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "删除题目失败: "+err.Error())
		}
		return
	}

	common.SuccessWithMessage(c, "删除成功", nil)
}

// BatchImportQuestions 批量导入题目
// @Summary 批量导入题目
// @Description 批量导入JSON格式的题目数据
// @Tags 管理后台-题库管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body service.BatchImportRequest true "导入数据"
// @Success 200 {object} common.Response{data=service.BatchImportResponse}
// @Router /api/admin/questions/import [post]
func (h *AdminHandler) BatchImportQuestions(c *gin.Context) {
	var req service.BatchImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	result, err := h.adminService.BatchImportQuestions(c.Request.Context(), &req)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "批量导入失败: "+err.Error())
		}
		return
	}

	common.Success(c, result)
}

// ==================== 分类管理 ====================

// CreateCategory 创建分类
// @Summary 创建分类
// @Description 创建新分类
// @Tags 管理后台-分类管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body service.CreateCategoryRequest true "分类信息"
// @Success 200 {object} common.Response{data=model.Category}
// @Router /api/admin/categories [post]
func (h *AdminHandler) ListCategories(c *gin.Context) {
	categories, err := h.adminService.ListCategories(c.Request.Context())
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "获取分类列表失败: "+err.Error())
		}
		return
	}

	common.Success(c, categories)
}

func (h *AdminHandler) CreateCategory(c *gin.Context) {
	var req service.CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	category, err := h.adminService.CreateCategory(c.Request.Context(), &req)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "创建分类失败: "+err.Error())
		}
		return
	}

	common.Success(c, category)
}

// UpdateCategory 更新分类
// @Summary 更新分类
// @Description 更新指定分类
// @Tags 管理后台-分类管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "分类ID"
// @Param request body service.UpdateCategoryRequest true "分类信息"
// @Success 200 {object} common.Response
// @Router /api/admin/categories/{id} [put]
func (h *AdminHandler) UpdateCategory(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "无效的分类ID")
		return
	}

	var req service.UpdateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	if err := h.adminService.UpdateCategory(c.Request.Context(), uint(id), &req); err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "更新分类失败: "+err.Error())
		}
		return
	}

	common.SuccessWithMessage(c, "更新成功", nil)
}

// DeleteCategory 删除分类
// @Summary 删除分类
// @Description 删除指定分类
// @Tags 管理后台-分类管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "分类ID"
// @Success 200 {object} common.Response
// @Router /api/admin/categories/{id} [delete]
func (h *AdminHandler) DeleteCategory(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "无效的分类ID")
		return
	}

	if err := h.adminService.DeleteCategory(c.Request.Context(), uint(id)); err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "删除分类失败: "+err.Error())
		}
		return
	}

	common.SuccessWithMessage(c, "删除成功", nil)
}

// ==================== 行业管理 ====================

// ListIndustries 获取行业列表
// @Summary 获取行业列表
// @Description 获取所有行业列表
// @Tags 管理后台-行业管理
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} common.Response{data=[]model.Industry}
// @Router /api/admin/industries [get]
func (h *AdminHandler) ListIndustries(c *gin.Context) {
	industries, err := h.adminService.ListIndustries(c.Request.Context())
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "获取行业列表失败: "+err.Error())
		}
		return
	}

	common.Success(c, industries)
}

// CreateIndustry 创建行业
// @Summary 创建行业
// @Description 创建新行业
// @Tags 管理后台-行业管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body service.CreateIndustryRequest true "行业信息"
// @Success 200 {object} common.Response{data=model.Industry}
// @Router /api/admin/industries [post]
func (h *AdminHandler) CreateIndustry(c *gin.Context) {
	var req service.CreateIndustryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	industry, err := h.adminService.CreateIndustry(c.Request.Context(), &req)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "创建行业失败: "+err.Error())
		}
		return
	}

	common.Success(c, industry)
}

// UpdateIndustry 更新行业
// @Summary 更新行业
// @Description 更新指定行业
// @Tags 管理后台-行业管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "行业ID"
// @Param request body service.UpdateIndustryRequest true "行业信息"
// @Success 200 {object} common.Response
// @Router /api/admin/industries/{id} [put]
func (h *AdminHandler) UpdateIndustry(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "无效的行业ID")
		return
	}

	var req service.UpdateIndustryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	if err := h.adminService.UpdateIndustry(c.Request.Context(), uint(id), &req); err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "更新行业失败: "+err.Error())
		}
		return
	}

	common.SuccessWithMessage(c, "更新成功", nil)
}

// ==================== Prompt模板管理 ====================

// ListPrompts 获取Prompt模板列表
// @Summary 获取Prompt模板列表
// @Description 获取Prompt模板列表，支持行业ID和场景过滤
// @Tags 管理后台-Prompt管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param industry_id query int false "行业ID"
// @Param scene query string false "场景"
// @Success 200 {object} common.Response{data=[]model.PromptTemplate}
// @Router /api/admin/prompts [get]
func (h *AdminHandler) ListPrompts(c *gin.Context) {
	var industryID *uint
	if idStr := c.Query("industry_id"); idStr != "" {
		if id, err := strconv.ParseUint(idStr, 10, 32); err == nil {
			uid := uint(id)
			industryID = &uid
		}
	}
	scene := c.Query("scene")

	templates, err := h.adminService.ListPrompts(c.Request.Context(), industryID, scene)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "获取Prompt模板列表失败: "+err.Error())
		}
		return
	}

	common.Success(c, templates)
}

// CreatePrompt 创建Prompt模板
// @Summary 创建Prompt模板
// @Description 创建新Prompt模板
// @Tags 管理后台-Prompt管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body service.CreatePromptRequest true "Prompt模板信息"
// @Success 200 {object} common.Response{data=model.PromptTemplate}
// @Router /api/admin/prompts [post]
func (h *AdminHandler) CreatePrompt(c *gin.Context) {
	var req service.CreatePromptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	tpl, err := h.adminService.CreatePrompt(c.Request.Context(), &req)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "创建Prompt模板失败: "+err.Error())
		}
		return
	}

	common.Success(c, tpl)
}

// UpdatePrompt 更新Prompt模板
// @Summary 更新Prompt模板
// @Description 更新指定Prompt模板
// @Tags 管理后台-Prompt管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "Prompt模板ID"
// @Param request body service.UpdatePromptRequest true "Prompt模板信息"
// @Success 200 {object} common.Response
// @Router /api/admin/prompts/{id} [put]
func (h *AdminHandler) UpdatePrompt(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "无效的Prompt模板ID")
		return
	}

	var req service.UpdatePromptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	if err := h.adminService.UpdatePrompt(c.Request.Context(), uint(id), &req); err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "更新Prompt模板失败: "+err.Error())
		}
		return
	}

	common.SuccessWithMessage(c, "更新成功", nil)
}

// DeletePrompt 删除Prompt模板
// @Summary 删除Prompt模板
// @Description 删除指定Prompt模板
// @Tags 管理后台-Prompt管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "Prompt模板ID"
// @Success 200 {object} common.Response
// @Router /api/admin/prompts/{id} [delete]
func (h *AdminHandler) DeletePrompt(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "无效的Prompt模板ID")
		return
	}

	if err := h.adminService.DeletePrompt(c.Request.Context(), uint(id)); err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "删除Prompt模板失败: "+err.Error())
		}
		return
	}

	common.SuccessWithMessage(c, "删除成功", nil)
}

// ==================== AI配置管理 ====================

// GetAIConfigs 获取AI配置列表
// @Summary 获取AI配置列表
// @Description 获取所有AI配置
// @Tags 管理后台-AI配置
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} common.Response{data=[]model.AdminConfig}
// @Router /api/admin/ai-configs [get]
func (h *AdminHandler) GetAIConfigs(c *gin.Context) {
	configs, err := h.adminService.GetAIConfigs(c.Request.Context())
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "获取AI配置失败: "+err.Error())
		}
		return
	}

	common.Success(c, configs)
}

// UpdateAIConfigsRequest 更新AI配置请求
type UpdateAIConfigsRequest struct {
	Configs map[string]string `json:"configs" binding:"required"`
}

// UpdateAIConfigs 更新AI配置
// @Summary 更新AI配置
// @Description 批量更新AI配置
// @Tags 管理后台-AI配置
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body UpdateAIConfigsRequest true "配置信息"
// @Success 200 {object} common.Response
// @Router /api/admin/ai-configs [put]
func (h *AdminHandler) UpdateAIConfigs(c *gin.Context) {
	var req UpdateAIConfigsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	if err := h.adminService.UpdateAIConfigs(c.Request.Context(), req.Configs); err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "更新AI配置失败: "+err.Error())
		}
		return
	}

	common.SuccessWithMessage(c, "更新成功", nil)
}

// ==================== Live2D模型管理 ====================

const maxLive2DPackageBytes = 200 << 20

// ListLive2DModels 获取Live2D模型列表
// @Summary 获取Live2D模型列表
// @Description 获取所有Live2D模型
// @Tags 管理后台-Live2D管理
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} common.Response{data=[]model.Live2DModel}
// @Router /api/admin/live2d-models [get]
func (h *AdminHandler) ListLive2DModels(c *gin.Context) {
	models, err := h.adminService.ListLive2DModels(c.Request.Context())
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "获取Live2D模型列表失败: "+err.Error())
		}
		return
	}

	common.Success(c, models)
}

// ImportLive2DPackage 导入管理员上传的 Live2D ZIP 包。
// @Summary 导入Live2D模型包
// @Description 上传并解压 Live2D ZIP 包，自动识别模型地址和缩略图
// @Tags 管理后台-Live2D管理
// @Accept mpfd
// @Produce json
// @Security Bearer
// @Param file formData file true "Live2D ZIP 模型包"
// @Success 200 {object} common.Response{data=service.ImportLive2DPackageResponse}
// @Router /api/admin/live2d-models/import [post]
func (h *AdminHandler) ImportLive2DPackage(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		common.BadRequest(c, "请上传Live2D模型ZIP包")
		return
	}

	if !strings.EqualFold(filepath.Ext(fileHeader.Filename), ".zip") {
		common.BadRequest(c, "仅支持上传.zip模型包")
		return
	}
	if fileHeader.Size <= 0 {
		common.BadRequest(c, "上传的模型包为空")
		return
	}
	if fileHeader.Size > maxLive2DPackageBytes {
		common.BadRequest(c, "模型包过大，请控制在200MB以内")
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		common.InternalError(c, "打开Live2D模型包失败: "+err.Error())
		return
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		common.InternalError(c, "读取Live2D模型包失败: "+err.Error())
		return
	}

	resp, err := h.adminService.ImportLive2DPackage(c.Request.Context(), fileHeader.Filename, content)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "导入Live2D模型包失败: "+err.Error())
		}
		return
	}

	common.Success(c, resp)
}

// CreateLive2DModel 创建Live2D模型
// @Summary 创建Live2D模型
// @Description 创建新Live2D模型
// @Tags 管理后台-Live2D管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body service.CreateLive2DModelRequest true "Live2D模型信息"
// @Success 200 {object} common.Response{data=model.Live2DModel}
// @Router /api/admin/live2d-models [post]
func (h *AdminHandler) CreateLive2DModel(c *gin.Context) {
	var req service.CreateLive2DModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	m, err := h.adminService.CreateLive2DModel(c.Request.Context(), &req)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "创建Live2D模型失败: "+err.Error())
		}
		return
	}

	common.Success(c, m)
}

// UpdateLive2DModel 更新Live2D模型
// @Summary 更新Live2D模型
// @Description 更新指定Live2D模型
// @Tags 管理后台-Live2D管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "Live2D模型ID"
// @Param request body service.UpdateLive2DModelRequest true "Live2D模型信息"
// @Success 200 {object} common.Response
// @Router /api/admin/live2d-models/{id} [put]
func (h *AdminHandler) UpdateLive2DModel(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "无效的Live2D模型ID")
		return
	}

	var req service.UpdateLive2DModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	if err := h.adminService.UpdateLive2DModel(c.Request.Context(), uint(id), &req); err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "更新Live2D模型失败: "+err.Error())
		}
		return
	}

	common.SuccessWithMessage(c, "更新成功", nil)
}

// DeleteLive2DModel 删除Live2D模型
// @Summary 删除Live2D模型
// @Description 删除指定Live2D模型
// @Tags 管理后台-Live2D管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "Live2D模型ID"
// @Success 200 {object} common.Response
// @Router /api/admin/live2d-models/{id} [delete]
func (h *AdminHandler) DeleteLive2DModel(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "无效的Live2D模型ID")
		return
	}

	if err := h.adminService.DeleteLive2DModel(c.Request.Context(), uint(id)); err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "删除Live2D模型失败: "+err.Error())
		}
		return
	}

	common.SuccessWithMessage(c, "删除成功", nil)
}

// ==================== TTS配置管理 ====================

// ListTTSConfigs 获取TTS配置列表
// @Summary 获取TTS配置列表
// @Description 获取所有TTS配置
// @Tags 管理后台-TTS管理
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} common.Response{data=[]model.TTSConfig}
// @Router /api/admin/tts-configs [get]
func (h *AdminHandler) ListTTSConfigs(c *gin.Context) {
	configs, err := h.adminService.ListTTSConfigs(c.Request.Context())
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "获取TTS配置列表失败: "+err.Error())
		}
		return
	}

	common.Success(c, configs)
}

// CreateTTSConfig 创建TTS配置
// @Summary 创建TTS配置
// @Description 创建新TTS配置
// @Tags 管理后台-TTS管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body service.CreateTTSConfigRequest true "TTS配置信息"
// @Success 200 {object} common.Response{data=model.TTSConfig}
// @Router /api/admin/tts-configs [post]
func (h *AdminHandler) CreateTTSConfig(c *gin.Context) {
	var req service.CreateTTSConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	cfg, err := h.adminService.CreateTTSConfig(c.Request.Context(), &req)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "创建TTS配置失败: "+err.Error())
		}
		return
	}

	common.Success(c, cfg)
}

// UpdateTTSConfig 更新TTS配置
// @Summary 更新TTS配置
// @Description 更新指定TTS配置
// @Tags 管理后台-TTS管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "TTS配置ID"
// @Param request body service.UpdateTTSConfigRequest true "TTS配置信息"
// @Success 200 {object} common.Response
// @Router /api/admin/tts-configs/{id} [put]
func (h *AdminHandler) UpdateTTSConfig(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "无效的TTS配置ID")
		return
	}

	var req service.UpdateTTSConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	if err := h.adminService.UpdateTTSConfig(c.Request.Context(), uint(id), &req); err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "更新TTS配置失败: "+err.Error())
		}
		return
	}

	common.SuccessWithMessage(c, "更新成功", nil)
}

// DeleteTTSConfig 删除TTS配置
// @Summary 删除TTS配置
// @Description 删除指定TTS配置
// @Tags 管理后台-TTS管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "TTS配置ID"
// @Success 200 {object} common.Response
// @Router /api/admin/tts-configs/{id} [delete]
func (h *AdminHandler) DeleteTTSConfig(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "无效的TTS配置ID")
		return
	}

	if err := h.adminService.DeleteTTSConfig(c.Request.Context(), uint(id)); err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "删除TTS配置失败: "+err.Error())
		}
		return
	}

	common.SuccessWithMessage(c, "删除成功", nil)
}
