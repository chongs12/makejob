// Package handler 提供HTTP请求处理层
package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"makejob-backend/internal/common"
	"makejob-backend/internal/scraper"
	"makejob-backend/internal/service"
)

// ScraperHandler 爬虫处理器
type ScraperHandler struct {
	scraperService service.ScraperService
}

// NewScraperHandler 创建爬虫处理器实例
func NewScraperHandler(scraperService service.ScraperService) *ScraperHandler {
	return &ScraperHandler{
		scraperService: scraperService,
	}
}

// RegisterRoutes 注册爬虫相关路由
func (h *ScraperHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/scraper/sources", h.GetSources)
	r.POST("/scraper/search", h.Search)
	r.POST("/scraper/fetch", h.Fetch)
	r.POST("/scraper/clean", h.Clean)
	r.POST("/scraper/import", h.Import)
	r.POST("/scraper/import/async", h.ImportAsync)
	r.GET("/scraper/tasks", h.ListTasks)
	r.GET("/scraper/tasks/:id", h.GetTask)
	r.POST("/scraper/tasks/:id/retry", h.RetryTask)
}

// GetSources 获取支持的数据源列表
// @Summary 获取数据源列表
// @Description 获取支持的爬取数据源列表
// @Tags 管理后台-面经采集
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} common.Response{data=[]scraper.Source}
// @Router /api/admin/scraper/sources [get]
func (h *ScraperHandler) GetSources(c *gin.Context) {
	sources, err := h.scraperService.GetSources(c.Request.Context())
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "获取数据源列表失败: "+err.Error())
		}
		return
	}

	common.Success(c, sources)
}

// Search 搜索面经
// @Summary 搜索面经
// @Description 从指定数据源搜索面经
// @Tags 管理后台-面经采集
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body scraper.SearchRequest true "搜索参数"
// @Success 200 {object} common.Response{data=[]scraper.SearchResult}
// @Router /api/admin/scraper/search [post]
func (h *ScraperHandler) Search(c *gin.Context) {
	var req scraper.SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	results, err := h.scraperService.Search(c.Request.Context(), req)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "搜索面经失败: "+err.Error())
		}
		return
	}

	common.Success(c, results)
}

// Fetch 爬取面经内容
// @Summary 爬取面经内容
// @Description 从指定URL爬取面经内容
// @Tags 管理后台-面经采集
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body scraper.FetchRequest true "爬取参数"
// @Success 200 {object} common.Response{data=scraper.FetchResult}
// @Router /api/admin/scraper/fetch [post]
func (h *ScraperHandler) Fetch(c *gin.Context) {
	var req scraper.FetchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	result, err := h.scraperService.Fetch(c.Request.Context(), req)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "爬取面经失败: "+err.Error())
		}
		return
	}

	common.Success(c, result)
}

// Clean AI清洗面经内容
// @Summary AI清洗面经
// @Description 使用AI清洗面经内容，提取结构化题目
// @Tags 管理后台-面经采集
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body scraper.CleanRequest true "清洗参数"
// @Success 200 {object} common.Response{data=scraper.CleanResult}
// @Router /api/admin/scraper/clean [post]
func (h *ScraperHandler) Clean(c *gin.Context) {
	var req scraper.CleanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	result, err := h.scraperService.Clean(c.Request.Context(), req)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "清洗面经失败: "+err.Error())
		}
		return
	}

	common.Success(c, result)
}

// Import 导入题目到题库
// @Summary 导入题目
// @Description 将清洗后的题目导入题库
// @Tags 管理后台-面经采集
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body scraper.ImportRequest true "导入参数"
// @Success 200 {object} common.Response{data=scraper.ImportResult}
// @Router /api/admin/scraper/import [post]
func (h *ScraperHandler) Import(c *gin.Context) {
	var req scraper.ImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	result, err := h.scraperService.Import(c.Request.Context(), req)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "导入题目失败: "+err.Error())
		}
		return
	}

	common.Success(c, result)
}

// ImportAsync 创建异步导入任务，由后台 worker 后续消费执行。
// @Summary 异步创建导入任务
// @Description 将清洗后的题目先入库为任务，再由独立执行器异步导入题库
// @Tags 管理后台-面经采集
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body scraper.ImportRequest true "导入参数"
// @Success 200 {object} common.Response{data=model.ScraperTask}
// @Router /api/admin/scraper/import/async [post]
func (h *ScraperHandler) ImportAsync(c *gin.Context) {
	var req scraper.ImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	task, err := h.scraperService.CreateImportTask(c.Request.Context(), req)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "创建异步导入任务失败: "+err.Error())
		}
		return
	}

	common.SuccessWithMessage(c, "导入任务已创建", task)
}

// ListTasks 获取爬取任务列表
// @Summary 获取爬取任务列表
// @Description 分页获取爬取任务历史记录
// @Tags 管理后台-面经采集
// @Accept json
// @Produce json
// @Security Bearer
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Param status query string false "任务状态筛选"
// @Param task_type query string false "任务类型筛选"
// @Success 200 {object} common.Response{data=common.PageResult}
// @Router /api/admin/scraper/tasks [get]
func (h *ScraperHandler) ListTasks(c *gin.Context) {
	pageParam := common.ReadPageParam(c)
	filter := scraper.TaskListFilter{
		Status:   c.Query("status"),
		TaskType: c.Query("task_type"),
	}

	result, err := h.scraperService.ListTasks(c.Request.Context(), pageParam.Page, pageParam.PageSize, filter)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "获取任务列表失败: "+err.Error())
		}
		return
	}

	common.Success(c, result)
}

// GetTask 获取单条抓取/导入任务详情。
// @Summary 获取任务详情
// @Description 按任务 ID 查看当前任务的状态、错误和执行时间
// @Tags 管理后台-面经采集
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "任务ID"
// @Success 200 {object} common.Response{data=scraper.TaskDetail}
// @Router /api/admin/scraper/tasks/{id} [get]
func (h *ScraperHandler) GetTask(c *gin.Context) {
	taskID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "无效的任务ID")
		return
	}

	task, err := h.scraperService.GetTask(c.Request.Context(), uint(taskID))
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "获取任务详情失败: "+err.Error())
		}
		return
	}

	common.Success(c, task)
}

// RetryTask 重新投递一条失败的异步导入任务。
// @Summary 重试任务
// @Description 仅允许将失败的异步导入任务重置为 pending，再由 worker 继续消费
// @Tags 管理后台-面经采集
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "任务ID"
// @Success 200 {object} common.Response{data=model.ScraperTask}
// @Router /api/admin/scraper/tasks/{id}/retry [post]
func (h *ScraperHandler) RetryTask(c *gin.Context) {
	taskID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "无效的任务ID")
		return
	}

	task, err := h.scraperService.RetryTask(c.Request.Context(), uint(taskID))
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "重试任务失败: "+err.Error())
		}
		return
	}

	common.SuccessWithMessage(c, "任务已重新投递", task)
}
