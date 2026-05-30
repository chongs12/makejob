package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"makejob-backend/internal/common"
	"makejob-backend/internal/repository"
	"makejob-backend/internal/service"
)

// AdminRAGDocumentHandler 管理后台RAG文档处理器
type AdminRAGDocumentHandler struct {
	ragDocService service.RAGDocumentService
}

// NewAdminRAGDocumentHandler 创建管理后台RAG文档处理器实例
func NewAdminRAGDocumentHandler(ragDocService service.RAGDocumentService) *AdminRAGDocumentHandler {
	return &AdminRAGDocumentHandler{
		ragDocService: ragDocService,
	}
}

// RegisterRoutes 注册管理后台RAG文档路由
func (h *AdminRAGDocumentHandler) RegisterRoutes(admin *gin.RouterGroup) {
	docs := admin.Group("/rag-documents")
	{
		docs.GET("", h.ListDocuments)
		docs.GET("/stats", h.GetDocumentStats)
		docs.GET("/:id", h.GetDocument)
		docs.POST("", h.CreateDocument)
		docs.PUT("/:id", h.UpdateDocument)
		docs.DELETE("/:id", h.DeleteDocument)
		docs.POST("/batch-import", h.BatchImportDocuments)
		docs.POST("/sync", h.SyncToVectorDB)
		docs.POST("/sync-all", h.SyncAllPending)
	}
}

// ListDocuments 获取RAG文档列表
func (h *AdminRAGDocumentHandler) ListDocuments(c *gin.Context) {
	pageParam := common.ReadPageParam(c)
	collection := c.Query("collection")
	docType := c.Query("doc_type")
	keyword := c.Query("keyword")
	syncStatus := c.Query("sync_status")

	params := repository.RAGDocumentListParams{
		Page:       pageParam.Page,
		PageSize:   pageParam.PageSize,
		Collection: collection,
		DocType:    docType,
		Keyword:    keyword,
		SyncStatus: syncStatus,
	}

	result, err := h.ragDocService.ListDocuments(c.Request.Context(), params)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "获取文档列表失败: "+err.Error())
		}
		return
	}

	common.Success(c, result)
}

// GetDocument 获取RAG文档详情
func (h *AdminRAGDocumentHandler) GetDocument(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "无效的文档ID")
		return
	}

	doc, err := h.ragDocService.GetDocument(c.Request.Context(), uint(id))
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "获取文档详情失败: "+err.Error())
		}
		return
	}

	common.Success(c, doc)
}

// CreateDocument 创建RAG文档
func (h *AdminRAGDocumentHandler) CreateDocument(c *gin.Context) {
	var req service.CreateRAGDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	doc, err := h.ragDocService.CreateDocument(c.Request.Context(), &req)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "创建文档失败: "+err.Error())
		}
		return
	}

	common.Success(c, doc)
}

// UpdateDocument 更新RAG文档
func (h *AdminRAGDocumentHandler) UpdateDocument(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "无效的文档ID")
		return
	}

	var req service.UpdateRAGDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	if err := h.ragDocService.UpdateDocument(c.Request.Context(), uint(id), &req); err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "更新文档失败: "+err.Error())
		}
		return
	}

	common.SuccessWithMessage(c, "更新成功", nil)
}

// DeleteDocument 删除RAG文档
func (h *AdminRAGDocumentHandler) DeleteDocument(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "无效的文档ID")
		return
	}

	if err := h.ragDocService.DeleteDocument(c.Request.Context(), uint(id)); err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "删除文档失败: "+err.Error())
		}
		return
	}

	common.SuccessWithMessage(c, "删除成功", nil)
}

// BatchImportDocuments 批量导入RAG文档
func (h *AdminRAGDocumentHandler) BatchImportDocuments(c *gin.Context) {
	var req service.BatchImportRAGDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	result, err := h.ragDocService.BatchImportDocuments(c.Request.Context(), &req)
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

// SyncToVectorDB 同步文档到向量库
func (h *AdminRAGDocumentHandler) SyncToVectorDB(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	if err := h.ragDocService.SyncToVectorDB(c.Request.Context(), req.IDs); err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "同步失败: "+err.Error())
		}
		return
	}

	common.SuccessWithMessage(c, "同步成功", nil)
}

// SyncAllPending 同步所有待同步文档
func (h *AdminRAGDocumentHandler) SyncAllPending(c *gin.Context) {
	if err := h.ragDocService.SyncAllPending(c.Request.Context()); err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "同步失败: "+err.Error())
		}
		return
	}

	common.SuccessWithMessage(c, "同步成功", nil)
}

// GetDocumentStats 获取文档统计信息
func (h *AdminRAGDocumentHandler) GetDocumentStats(c *gin.Context) {
	collection := c.Query("collection")

	stats, err := h.ragDocService.GetDocumentStats(c.Request.Context(), collection)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "获取统计信息失败: "+err.Error())
		}
		return
	}

	common.Success(c, stats)
}
