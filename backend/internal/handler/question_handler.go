// Package handler 提供HTTP请求处理层
package handler

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"makejob-backend/internal/common"
	"makejob-backend/internal/middleware"
	"makejob-backend/internal/repository"
	"makejob-backend/internal/service"
)

// QuestionHandler 题目处理器
type QuestionHandler struct {
	questionService service.QuestionService
}

// NewQuestionHandler 创建题目处理器实例
func NewQuestionHandler(questionService service.QuestionService) *QuestionHandler {
	return &QuestionHandler{
		questionService: questionService,
	}
}

// RegisterRoutes 注册路由
func (h *QuestionHandler) RegisterRoutes(public *gin.RouterGroup, protected *gin.RouterGroup) {
	// 公开路由（可匿名访问）
	if public != nil {
		public.GET("/questions", h.ListQuestions)
		public.GET("/questions/:id", h.GetQuestion)
		public.GET("/question-sets", h.ListQuestionSets)
		public.GET("/question-sets/:slug", h.GetQuestionSetDetail)
		public.GET("/mistake-topics", h.ListMistakeTopics)
		public.GET("/mistake-topics/:code", h.GetMistakeTopic)
		public.GET("/industries", h.ListIndustries)
		public.GET("/categories", h.GetCategories)
	}

	// 需要认证的路由
	if protected != nil {
		// 答题相关
		protected.POST("/questions/:id/submit", h.SubmitAnswer)
		protected.POST("/questions/:id/run", h.RunCode)
		protected.POST("/questions/:id/favorite", h.ToggleFavorite)

		// 用户相关
		protected.GET("/user/favorites", h.GetFavorites)
		protected.GET("/user/wrong-questions", h.GetWrongQuestions)
		protected.GET("/user/notes", h.ListNotes)
		protected.POST("/user/notes", h.CreateNote)
		protected.PUT("/user/notes/:id", h.UpdateNote)
		protected.DELETE("/user/notes/:id", h.DeleteNote)
		protected.GET("/user/practice-stats", h.GetPracticeStats)
		protected.GET("/user/practice-recommendations", h.GetPracticeRecommendations)

		// 考试相关
		protected.POST("/exams/random", h.GenerateRandomExam)
		protected.POST("/exams/timed", h.GenerateTimedExam)
		protected.POST("/exams/submit", h.SubmitExam)
	}
}

// ListQuestions 获取题目列表
// @Summary 获取题目列表
// @Description 分页获取题目列表，支持筛选和搜索
// @Tags 题库
// @Accept json
// @Produce json
// @Param page query int false "页码，默认1"
// @Param page_size query int false "每页数量，默认10"
// @Param category_id query int false "分类ID"
// @Param industry_id query int false "行业ID"
// @Param type query string false "题目类型(choice/multi/code/subjective)"
// @Param difficulty query string false "难度(easy/medium/hard)"
// @Param keyword query string false "搜索关键词"
// @Param tags query string false "标签，逗号分隔"
// @Success 200 {object} common.Response{data=common.PageResult}
// @Router /api/questions [get]
func (h *QuestionHandler) ListQuestions(c *gin.Context) {
	var params repository.QuestionListParams

	// 统一读取并规范化分页参数，避免题库相关列表接口行为不一致。
	pageParam := common.ReadPageParam(c)
	params.Page = pageParam.Page
	params.PageSize = pageParam.PageSize

	// 解析筛选参数
	if categoryIDStr := c.Query("category_id"); categoryIDStr != "" {
		if categoryID, err := strconv.ParseUint(categoryIDStr, 10, 32); err == nil {
			cid := uint(categoryID)
			params.CategoryID = &cid
		}
	}
	if industryIDStr := c.Query("industry_id"); industryIDStr != "" {
		if industryID, err := strconv.ParseUint(industryIDStr, 10, 32); err == nil {
			iid := uint(industryID)
			params.IndustryID = &iid
		}
	}
	params.Type = c.Query("type")
	params.Difficulty = c.Query("difficulty")
	params.Keyword = c.Query("keyword")
	params.Tags = c.Query("tags")

	result, err := h.questionService.ListQuestions(c.Request.Context(), params)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "获取题目列表失败: "+err.Error())
		}
		return
	}

	common.Success(c, result)
}

// GetQuestion 获取题目详情
// @Summary 获取题目详情
// @Description 根据ID获取题目详情，包括用户收藏状态和笔记
// @Tags 题库
// @Accept json
// @Produce json
// @Param id path int true "题目ID"
// @Success 200 {object} common.Response{data=service.QuestionDetail}
// @Router /api/questions/{id} [get]
func (h *QuestionHandler) GetQuestion(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "无效的题目ID")
		return
	}

	// 尝试获取用户ID（可选认证）
	var userID uint
	if uid, exists := middleware.GetUserID(c); exists {
		userID = uid
	}

	question, err := h.questionService.GetQuestion(c.Request.Context(), uint(id), userID)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "获取题目详情失败: "+err.Error())
		}
		return
	}

	common.Success(c, question)
}

// ListQuestionSets 获取当前行业下的核心题单摘要。
// @Summary 获取核心题单摘要
// @Description 返回题库首页可直接展示的核心题单及题目预览
// @Tags 题库
// @Accept json
// @Produce json
// @Param industry_id query int false "行业ID"
// @Success 200 {object} common.Response{data=[]service.QuestionSetSummary}
// @Router /api/question-sets [get]
func (h *QuestionHandler) ListQuestionSets(c *gin.Context) {
	var industryID uint
	if industryIDStr := c.Query("industry_id"); industryIDStr != "" {
		id, err := strconv.ParseUint(industryIDStr, 10, 32)
		if err != nil {
			common.BadRequest(c, "无效的行业ID")
			return
		}
		industryID = uint(id)
	}

	sets, err := h.questionService.ListQuestionSets(c.Request.Context(), industryID)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "获取核心题单失败: "+err.Error())
		}
		return
	}

	common.Success(c, sets)
}

// GetQuestionSetDetail 获取指定核心题单的完整题目集合。
// @Summary 获取核心题单详情
// @Description 返回某个题单下可直接进入练习的完整题目列表
// @Tags 题库
// @Accept json
// @Produce json
// @Param slug path string true "题单 slug"
// @Param industry_id query int false "行业ID"
// @Success 200 {object} common.Response{data=service.QuestionSetDetail}
// @Router /api/question-sets/{slug} [get]
func (h *QuestionHandler) GetQuestionSetDetail(c *gin.Context) {
	slug := strings.TrimSpace(c.Param("slug"))
	if slug == "" {
		common.BadRequest(c, "题单不能为空")
		return
	}

	var industryID uint
	if industryIDStr := c.Query("industry_id"); industryIDStr != "" {
		id, err := strconv.ParseUint(industryIDStr, 10, 32)
		if err != nil {
			common.BadRequest(c, "无效的行业ID")
			return
		}
		industryID = uint(id)
	}

	detail, err := h.questionService.GetQuestionSetDetail(c.Request.Context(), industryID, slug)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "获取题单详情失败: "+err.Error())
		}
		return
	}

	common.Success(c, detail)
}

// ListMistakeTopics 获取错因专题卡片列表。
// @Summary 获取错因专题列表
// @Description 返回全部错因专题，或按 codes 参数筛选指定专题
// @Tags 题库
// @Accept json
// @Produce json
// @Param codes query string false "专题编码列表，逗号分隔"
// @Success 200 {object} common.Response{data=[]service.MistakeTopicCard}
// @Router /api/mistake-topics [get]
func (h *QuestionHandler) ListMistakeTopics(c *gin.Context) {
	rawCodes := strings.TrimSpace(c.Query("codes"))
	codes := make([]string, 0)
	if rawCodes != "" {
		for _, part := range strings.Split(rawCodes, ",") {
			trimmed := strings.TrimSpace(part)
			if trimmed == "" {
				continue
			}
			codes = append(codes, trimmed)
		}
	}

	topics, err := h.questionService.ListMistakeTopics(c.Request.Context(), codes)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "获取错因专题列表失败: "+err.Error())
		}
		return
	}

	common.Success(c, topics)
}

// GetMistakeTopic 获取单个错因专题详情。
// @Summary 获取错因专题详情
// @Description 根据专题编码返回专题详情
// @Tags 题库
// @Accept json
// @Produce json
// @Param code path string true "专题编码"
// @Success 200 {object} common.Response{data=service.MistakeTopicCard}
// @Router /api/mistake-topics/{code} [get]
func (h *QuestionHandler) GetMistakeTopic(c *gin.Context) {
	topic, err := h.questionService.GetMistakeTopic(c.Request.Context(), c.Param("code"))
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "获取错因专题失败: "+err.Error())
		}
		return
	}

	common.Success(c, topic)
}

// ListIndustries 获取行业列表
// @Summary 获取行业列表
// @Description 获取前台可用的行业列表
// @Tags 题库
// @Accept json
// @Produce json
// @Success 200 {object} common.Response{data=[]model.Industry}
// @Router /api/industries [get]
func (h *QuestionHandler) ListIndustries(c *gin.Context) {
	industries, err := h.questionService.ListIndustries(c.Request.Context())
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

// GetCategories 获取分类列表
// @Summary 获取分类列表
// @Description 获取分类树形列表
// @Tags 题库
// @Accept json
// @Produce json
// @Param industry_id query int false "行业ID"
// @Param industry_code query string false "行业编码"
// @Success 200 {object} common.Response{data=[]service.CategoryTree}
// @Router /api/categories [get]
func (h *QuestionHandler) GetCategories(c *gin.Context) {
	var industryID uint
	if industryIDStr := c.Query("industry_id"); industryIDStr != "" {
		if id, err := strconv.ParseUint(industryIDStr, 10, 32); err == nil {
			industryID = uint(id)
		}
	}

	categories, err := h.questionService.GetCategories(c.Request.Context(), industryID, c.Query("industry_code"))
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

// SubmitAnswer 提交答案
// @Summary 提交答案
// @Description 提交题目答案并获取判分结果
// @Tags 题库
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "题目ID"
// @Param request body service.SubmitAnswerRequest true "答案信息"
// @Success 200 {object} common.Response{data=service.SubmitAnswerResponse}
// @Router /api/questions/{id}/submit [post]
func (h *QuestionHandler) SubmitAnswer(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "未登录")
		return
	}

	questionID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "无效的题目ID")
		return
	}

	var req service.SubmitAnswerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	resp, err := h.questionService.SubmitAnswer(c.Request.Context(), userID, uint(questionID), &req)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "提交答案失败: "+err.Error())
		}
		return
	}

	common.Success(c, resp)
}

// RunCode 运行代码（不触发AI分析，不保存记录）
func (h *QuestionHandler) RunCode(c *gin.Context) {
	questionID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "无效的题目ID")
		return
	}

	var req service.SubmitAnswerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	resp, err := h.questionService.RunCode(c.Request.Context(), uint(questionID), &req)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "运行代码失败: "+err.Error())
		}
		return
	}

	common.Success(c, resp)
}

// ToggleFavorite 切换收藏状态
// @Summary 切换收藏状态
// @Description 收藏或取消收藏题目
// @Tags 题库
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "题目ID"
// @Success 200 {object} common.Response{data=bool}
// @Router /api/questions/{id}/favorite [post]
func (h *QuestionHandler) ToggleFavorite(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "未登录")
		return
	}

	questionID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "无效的题目ID")
		return
	}

	isFavorited, err := h.questionService.ToggleFavorite(c.Request.Context(), userID, uint(questionID))
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "操作失败: "+err.Error())
		}
		return
	}

	common.Success(c, gin.H{"is_favorited": isFavorited})
}

// GetFavorites 获取收藏列表
// @Summary 获取收藏列表
// @Description 获取用户收藏的题目列表
// @Tags 用户
// @Accept json
// @Produce json
// @Security Bearer
// @Param page query int false "页码，默认1"
// @Param page_size query int false "每页数量，默认10"
// @Success 200 {object} common.Response{data=common.PageResult}
// @Router /api/user/favorites [get]
func (h *QuestionHandler) GetFavorites(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "未登录")
		return
	}

	pageParam := common.ReadPageParam(c)

	result, err := h.questionService.GetFavorites(c.Request.Context(), userID, pageParam.Page, pageParam.PageSize)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "获取收藏列表失败: "+err.Error())
		}
		return
	}

	common.Success(c, result)
}

// GetWrongQuestions 获取错题本
// @Summary 获取错题本
// @Description 获取用户答错的题目列表
// @Tags 用户
// @Accept json
// @Produce json
// @Security Bearer
// @Param page query int false "页码，默认1"
// @Param page_size query int false "每页数量，默认10"
// @Success 200 {object} common.Response{data=common.PageResult}
// @Router /api/user/wrong-questions [get]
func (h *QuestionHandler) GetWrongQuestions(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "未登录")
		return
	}

	pageParam := common.ReadPageParam(c)

	result, err := h.questionService.GetWrongQuestions(c.Request.Context(), userID, pageParam.Page, pageParam.PageSize)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "获取错题列表失败: "+err.Error())
		}
		return
	}

	common.Success(c, result)
}

// ListNotes 获取笔记列表
// @Summary 获取笔记列表
// @Description 获取用户的学习笔记列表
// @Tags 用户
// @Accept json
// @Produce json
// @Security Bearer
// @Param page query int false "页码，默认1"
// @Param page_size query int false "每页数量，默认10"
// @Success 200 {object} common.Response{data=common.PageResult}
// @Router /api/user/notes [get]
func (h *QuestionHandler) ListNotes(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "未登录")
		return
	}

	pageParam := common.ReadPageParam(c)

	result, err := h.questionService.ListNotes(c.Request.Context(), userID, pageParam.Page, pageParam.PageSize)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "获取笔记列表失败: "+err.Error())
		}
		return
	}

	common.Success(c, result)
}

// CreateNote 创建笔记
// @Summary 创建笔记
// @Description 创建新的学习笔记
// @Tags 用户
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body service.CreateNoteRequest true "笔记信息"
// @Success 200 {object} common.Response{data=model.UserNote}
// @Router /api/user/notes [post]
func (h *QuestionHandler) CreateNote(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "未登录")
		return
	}

	var req service.CreateNoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	note, err := h.questionService.CreateNote(c.Request.Context(), userID, &req)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "创建笔记失败: "+err.Error())
		}
		return
	}

	common.SuccessWithMessage(c, "创建成功", note)
}

// UpdateNote 更新笔记
// @Summary 更新笔记
// @Description 更新学习笔记
// @Tags 用户
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "笔记ID"
// @Param request body service.UpdateNoteRequest true "笔记信息"
// @Success 200 {object} common.Response
// @Router /api/user/notes/{id} [put]
func (h *QuestionHandler) UpdateNote(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "未登录")
		return
	}

	noteID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "无效的笔记ID")
		return
	}

	var req service.UpdateNoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	if err := h.questionService.UpdateNote(c.Request.Context(), userID, uint(noteID), &req); err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "更新笔记失败: "+err.Error())
		}
		return
	}

	common.SuccessWithMessage(c, "更新成功", nil)
}

// DeleteNote 删除笔记
// @Summary 删除笔记
// @Description 删除学习笔记
// @Tags 用户
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "笔记ID"
// @Success 200 {object} common.Response
// @Router /api/user/notes/{id} [delete]
func (h *QuestionHandler) DeleteNote(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "未登录")
		return
	}

	noteID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "无效的笔记ID")
		return
	}

	if err := h.questionService.DeleteNote(c.Request.Context(), userID, uint(noteID)); err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "删除笔记失败: "+err.Error())
		}
		return
	}

	common.SuccessWithMessage(c, "删除成功", nil)
}

// GenerateRandomExam 生成随机试卷
// @Summary 生成随机试卷
// @Description 根据参数生成随机组卷
// @Tags 考试
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body service.RandomExamRequest true "组卷参数"
// @Success 200 {object} common.Response{data=service.ExamResponse}
// @Router /api/exams/random [post]
func (h *QuestionHandler) GenerateRandomExam(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "未登录")
		return
	}

	var req service.RandomExamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	resp, err := h.questionService.GenerateRandomExam(c.Request.Context(), userID, &req)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "生成试卷失败: "+err.Error())
		}
		return
	}

	common.Success(c, resp)
}

// GenerateTimedExam 生成限时模拟试卷
// @Summary 生成限时模拟试卷
// @Description 生成限时模拟考试
// @Tags 考试
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body service.TimedExamRequest true "组卷参数"
// @Success 200 {object} common.Response{data=service.ExamResponse}
// @Router /api/exams/timed [post]
func (h *QuestionHandler) GenerateTimedExam(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "未登录")
		return
	}

	var req service.TimedExamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	resp, err := h.questionService.GenerateTimedExam(c.Request.Context(), userID, &req)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "生成试卷失败: "+err.Error())
		}
		return
	}

	common.Success(c, resp)
}

// SubmitExam 提交试卷
// @Summary 提交试卷
// @Description 提交考试答案并获取成绩
// @Tags 考试
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body service.SubmitExamRequest true "考试答案"
// @Success 200 {object} common.Response{data=service.ExamResult}
// @Router /api/exams/submit [post]
func (h *QuestionHandler) SubmitExam(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "未登录")
		return
	}

	var req service.SubmitExamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	result, err := h.questionService.SubmitExam(c.Request.Context(), userID, &req)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "提交试卷失败: "+err.Error())
		}
		return
	}

	common.Success(c, result)
}

// GetPracticeStats 获取练习统计
// @Summary 获取练习统计
// @Description 获取用户的答题统计数据
// @Tags 用户
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} common.Response{data=repository.UserPracticeStats}
// @Router /api/user/practice-stats [get]
func (h *QuestionHandler) GetPracticeStats(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "未登录")
		return
	}

	stats, err := h.questionService.GetPracticeStats(c.Request.Context(), userID)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "获取统计数据失败: "+err.Error())
		}
		return
	}

	common.Success(c, stats)
}

// GetPracticeRecommendations 获取对症练习推荐。
// @Summary 获取对症练习推荐
// @Description 基于最近学习档案中的错因标签返回推荐练习题
// @Tags 用户
// @Accept json
// @Produce json
// @Security Bearer
// @Param limit query int false "返回题目数量，默认6"
// @Param interview_id query int false "仅按指定面试关联的学习档案生成推荐"
// @Success 200 {object} common.Response{data=service.PracticeRecommendationResponse}
// @Router /api/user/practice-recommendations [get]
func (h *QuestionHandler) GetPracticeRecommendations(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "未登录")
		return
	}

	limit := 6
	if limitValue := c.Query("limit"); limitValue != "" {
		parsedLimit, err := strconv.Atoi(limitValue)
		if err != nil {
			common.BadRequest(c, "无效的 limit 参数")
			return
		}
		limit = parsedLimit
	}

	var interviewID *uint
	if interviewIDValue := c.Query("interview_id"); interviewIDValue != "" {
		parsedInterviewID, err := strconv.ParseUint(interviewIDValue, 10, 32)
		if err != nil {
			common.BadRequest(c, "无效的 interview_id 参数")
			return
		}
		value := uint(parsedInterviewID)
		interviewID = &value
	}

	result, err := h.questionService.GetPracticeRecommendations(c.Request.Context(), userID, interviewID, limit)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "获取练习推荐失败: "+err.Error())
		}
		return
	}

	common.Success(c, result)
}
