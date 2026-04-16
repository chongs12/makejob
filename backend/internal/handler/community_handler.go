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

func NewCommunityHandler(communityService service.CommunityService) *CommunityHandler {
	return &CommunityHandler{
		communityService: communityService,
	}
}

func (h *CommunityHandler) RegisterRoutes(public *gin.RouterGroup, protected *gin.RouterGroup) {
	if public != nil {
		public.GET("/community/posts", h.ListPosts)
		public.GET("/community/posts/:id", h.GetPostDetail)
	}

	if protected != nil {
		protected.POST("/community/posts", h.CreatePost)
	}
}

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

	result, err := h.communityService.ListPosts(c.Request.Context(), params)
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

func (h *CommunityHandler) GetPostDetail(c *gin.Context) {
	postID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "invalid community post id")
		return
	}

	post, serviceErr := h.communityService.GetPostDetail(c.Request.Context(), uint(postID))
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
