package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"makejob-backend/internal/common"
	"makejob-backend/internal/model"
	"makejob-backend/internal/rag"
	"makejob-backend/internal/repository"
)

// AdminRAGHandler 管理后台RAG处理器
type AdminRAGHandler struct {
	ragService   *rag.Service
	questionRepo repository.QuestionRepository
}

// NewAdminRAGHandler 创建管理后台RAG处理器实例
func NewAdminRAGHandler(ragService *rag.Service, questionRepo repository.QuestionRepository) *AdminRAGHandler {
	return &AdminRAGHandler{
		ragService:   ragService,
		questionRepo: questionRepo,
	}
}

// RegisterRoutes 注册管理后台RAG路由
func (h *AdminRAGHandler) RegisterRoutes(admin *gin.RouterGroup) {
	rag := admin.Group("/rag")
	{
		rag.POST("/index-all", h.IndexAllQuestions)
		rag.POST("/index", h.IndexQuestions)
		rag.DELETE("/index", h.DeleteIndex)
		rag.GET("/search", h.SearchQuestions)
	}
}

// IndexAllQuestionsRequest 全量索引请求
type IndexAllQuestionsRequest struct {
	IndustryID uint `json:"industry_id"` // 可选，按行业筛选
}

// IndexAllQuestions 全量索引所有题目
func (h *AdminRAGHandler) IndexAllQuestions(c *gin.Context) {
	var req IndexAllQuestionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 允许空body
		req = IndexAllQuestionsRequest{}
	}

	// 查询所有题目
	params := repository.QuestionListParams{
		Page:     1,
		PageSize: 10000, // 全量
	}
	if req.IndustryID > 0 {
		params.IndustryID = &req.IndustryID
	}
	questions, _, err := h.questionRepo.List(c.Request.Context(), params)
	if err != nil {
		common.InternalError(c, "查询题目失败: "+err.Error())
		return
	}

	if len(questions) == 0 {
		common.Success(c, gin.H{"indexed": 0})
		return
	}

	// 批量索引
	if err := h.ragService.IndexQuestions(c.Request.Context(), questions); err != nil {
		common.InternalError(c, "索引题目失败: "+err.Error())
		return
	}

	common.Success(c, gin.H{"indexed": len(questions)})
}

// IndexQuestionsRequest 增量索引请求
type IndexQuestionsRequest struct {
	QuestionIDs []uint `json:"question_ids" binding:"required,min=1"`
}

// IndexQuestions 增量索引指定题目
func (h *AdminRAGHandler) IndexQuestions(c *gin.Context) {
	var req IndexQuestionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Error(c, common.CodeBadRequest, "参数错误: "+err.Error())
		return
	}

	// 查询指定题目
	questions := make([]model.Question, 0, len(req.QuestionIDs))
	for _, id := range req.QuestionIDs {
		q, err := h.questionRepo.GetByID(c.Request.Context(), id)
		if err != nil {
			common.InternalError(c, "查询题目失败: "+err.Error())
			return
		}
		if q != nil {
			questions = append(questions, *q)
		}
	}

	if len(questions) == 0 {
		common.Success(c, gin.H{"indexed": 0})
		return
	}

	// 批量索引
	if err := h.ragService.IndexQuestions(c.Request.Context(), questions); err != nil {
		common.InternalError(c, "索引题目失败: "+err.Error())
		return
	}

	common.Success(c, gin.H{"indexed": len(questions)})
}

// DeleteIndexRequest 删除索引请求
type DeleteIndexRequest struct {
	QuestionIDs []uint `json:"question_ids" binding:"required,min=1"`
}

// DeleteIndex 删除指定题目的索引
func (h *AdminRAGHandler) DeleteIndex(c *gin.Context) {
	var req DeleteIndexRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Error(c, common.CodeBadRequest, "参数错误: "+err.Error())
		return
	}

	// 转换为文档ID
	docIDs := make([]string, 0, len(req.QuestionIDs))
	for _, id := range req.QuestionIDs {
		docIDs = append(docIDs, rag.QuestionIDToDocID(id))
	}

	// 删除索引
	if err := h.ragService.DeleteByIDs(c.Request.Context(), docIDs); err != nil {
		common.InternalError(c, "删除索引失败: "+err.Error())
		return
	}

	common.Success(c, gin.H{"deleted": len(docIDs)})
}

// SearchQuestions 检索测试
func (h *AdminRAGHandler) SearchQuestions(c *gin.Context) {
	query := c.Query("query")
	if query == "" {
		common.Error(c, common.CodeBadRequest, "query参数不能为空")
		return
	}

	topKStr := c.DefaultQuery("top_k", "5")
	topK, err := strconv.Atoi(topKStr)
	if err != nil || topK <= 0 {
		topK = 5
	}

	docs, err := h.ragService.RetrieveByQuery(c.Request.Context(), query, topK)
	if err != nil {
		common.InternalError(c, "检索失败: "+err.Error())
		return
	}

	common.Success(c, gin.H{
		"query":   query,
		"results": docs,
	})
}
