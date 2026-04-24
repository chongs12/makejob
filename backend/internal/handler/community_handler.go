package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"makejob-backend/internal/common"
	"makejob-backend/internal/middleware"
	"makejob-backend/internal/service"
)

type CommunityHandler struct {
	communityService service.CommunityService
}

// NewCommunityHandler 创建社区接口处理器。
func NewCommunityHandler(communityService service.CommunityService) *CommunityHandler {
	return &CommunityHandler{
		communityService: communityService,
	}
}

// RegisterRoutes 注册社区模块的公开与鉴权路由。
func (h *CommunityHandler) RegisterRoutes(public *gin.RouterGroup, protected *gin.RouterGroup) {
	if public != nil {
		public.GET("/community/posts", h.ListPosts)
		public.GET("/community/posts/:id", h.GetPostDetail)
		public.GET("/community/posts/:id/comments", h.ListComments)
	}

	if protected != nil {
		protected.POST("/community/posts", h.CreatePost)
		protected.GET("/community/my-posts", h.ListMyPosts)
		protected.PUT("/community/posts/:id", h.UpdatePost)
		protected.DELETE("/community/posts/:id", h.DeletePost)
		protected.POST("/community/posts/:id/comments", h.CreateComment)
		protected.POST("/community/posts/:id/like", h.ToggleLike)
	}
}

// ListPosts 返回社区帖子分页列表。
func (h *CommunityHandler) ListPosts(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	params := service.CommunityPostListParams{
		Page:     page,
		PageSize: pageSize,
		PostType: c.Query("type"),
		Keyword:  c.Query("keyword"),
		Tag:      c.Query("tag"),
	}

	currentUserID := optionalUserID(c)
	result, err := h.communityService.ListPosts(c.Request.Context(), params, currentUserID)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "list community posts failed: "+err.Error())
		}
		return
	}

	common.Success(c, result)
}

// GetPostDetail 返回单个社区帖子的详情内容。
func (h *CommunityHandler) GetPostDetail(c *gin.Context) {
	postID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "invalid community post id")
		return
	}

	post, serviceErr := h.communityService.GetPostDetail(c.Request.Context(), uint(postID), optionalUserID(c))
	if serviceErr != nil {
		if businessErr, ok := serviceErr.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "get community post failed: "+serviceErr.Error())
		}
		return
	}

	common.Success(c, post)
}

// CreatePost 创建一条新的社区帖子。
func (h *CommunityHandler) CreatePost(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "login required")
		return
	}

	var req service.CreateCommunityPostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "invalid community post payload: "+err.Error())
		return
	}

	post, serviceErr := h.communityService.CreatePost(c.Request.Context(), userID, &req)
	if serviceErr != nil {
		if businessErr, ok := serviceErr.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "create community post failed: "+serviceErr.Error())
		}
		return
	}

	common.SuccessWithMessage(c, "community post created", post)
}

// ListMyPosts 返回当前登录用户发布的帖子列表。
func (h *CommunityHandler) ListMyPosts(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "login required")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	params := service.CommunityPostListParams{
		Page:     page,
		PageSize: pageSize,
		PostType: c.Query("type"),
		Keyword:  c.Query("keyword"),
		Tag:      c.Query("tag"),
	}

	result, err := h.communityService.ListMyPosts(c.Request.Context(), userID, params)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "list my community posts failed: "+err.Error())
		}
		return
	}

	common.Success(c, result)
}

// UpdatePost 更新当前用户自己的社区帖子。
func (h *CommunityHandler) UpdatePost(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "login required")
		return
	}

	postID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "invalid community post id")
		return
	}

	var req service.UpdateCommunityPostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "invalid community post payload: "+err.Error())
		return
	}

	post, serviceErr := h.communityService.UpdatePost(c.Request.Context(), userID, uint(postID), &req)
	if serviceErr != nil {
		if businessErr, ok := serviceErr.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "update community post failed: "+serviceErr.Error())
		}
		return
	}

	common.SuccessWithMessage(c, "community post updated", post)
}

// DeletePost 删除当前用户自己的社区帖子。
func (h *CommunityHandler) DeletePost(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "login required")
		return
	}

	postID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "invalid community post id")
		return
	}

	serviceErr := h.communityService.DeletePost(c.Request.Context(), userID, uint(postID))
	if serviceErr != nil {
		if businessErr, ok := serviceErr.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "delete community post failed: "+serviceErr.Error())
		}
		return
	}

	common.SuccessWithMessage(c, "community post deleted", gin.H{"id": uint(postID)})
}

// ListComments 返回指定帖子的评论列表。
func (h *CommunityHandler) ListComments(c *gin.Context) {
	postID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "invalid community post id")
		return
	}

	comments, serviceErr := h.communityService.ListComments(c.Request.Context(), uint(postID), optionalUserID(c))
	if serviceErr != nil {
		if businessErr, ok := serviceErr.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "list community comments failed: "+serviceErr.Error())
		}
		return
	}

	common.Success(c, comments)
}

// CreateComment 为指定帖子创建评论。
func (h *CommunityHandler) CreateComment(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "login required")
		return
	}

	postID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "invalid community post id")
		return
	}

	var req service.CreateCommunityCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "invalid community comment payload: "+err.Error())
		return
	}

	comment, serviceErr := h.communityService.CreateComment(c.Request.Context(), userID, uint(postID), &req)
	if serviceErr != nil {
		if businessErr, ok := serviceErr.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "create community comment failed: "+serviceErr.Error())
		}
		return
	}

	common.SuccessWithMessage(c, "community comment created", comment)
}

// ToggleLike 切换当前用户对帖子的点赞状态。
func (h *CommunityHandler) ToggleLike(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "login required")
		return
	}

	postID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "invalid community post id")
		return
	}

	result, serviceErr := h.communityService.ToggleLike(c.Request.Context(), userID, uint(postID))
	if serviceErr != nil {
		if businessErr, ok := serviceErr.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "toggle community like failed: "+serviceErr.Error())
		}
		return
	}

	common.Success(c, result)
}

// optionalUserID 读取可选登录态中的用户 ID。
func optionalUserID(c *gin.Context) *uint {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		return nil
	}
	return &userID
}
