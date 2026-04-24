package service

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"makejob-backend/internal/common"
	"makejob-backend/internal/model"
	"makejob-backend/internal/repository"
)

// CommunityPostAuthor 定义社区帖子作者展示信息。
type CommunityPostAuthor struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
	Role     string `json:"role"`
}

// CommunityPostItem 定义社区帖子在前端使用的展示结构。
type CommunityPostItem struct {
	ID            uint                `json:"id"`
	PostType      string              `json:"post_type"`
	Title         string              `json:"title"`
	Content       string              `json:"content"`
	Summary       string              `json:"summary"`
	Tags          []string            `json:"tags"`
	ViewCount     int                 `json:"view_count"`
	CommentCount  int                 `json:"comment_count"`
	LikeCount     int                 `json:"like_count"`
	IsPinned      bool                `json:"is_pinned"`
	IsRecommended bool                `json:"is_recommended"`
	CreatedAt     string              `json:"created_at"`
	UpdatedAt     string              `json:"updated_at"`
	IsLiked       bool                `json:"is_liked"`
	IsAuthor      bool                `json:"is_author"`
	Author        CommunityPostAuthor `json:"author"`
}

// CommunityCommentItem 定义帖子评论展示结构。
type CommunityCommentItem struct {
	ID        uint                `json:"id"`
	Content   string              `json:"content"`
	CreatedAt string              `json:"created_at"`
	UpdatedAt string              `json:"updated_at"`
	IsAuthor  bool                `json:"is_author"`
	Author    CommunityPostAuthor `json:"author"`
}

// CommunityLikeToggleResponse 定义点赞切换后的响应结构。
type CommunityLikeToggleResponse struct {
	Liked     bool `json:"liked"`
	LikeCount int  `json:"like_count"`
}

// CommunityPostListParams 定义社区帖子列表查询参数。
type CommunityPostListParams struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	PostType string `form:"type"`
	Keyword  string `form:"keyword"`
	Tag      string `form:"tag"`
}

// CreateCommunityPostRequest 定义创建社区帖子的请求体。
type CreateCommunityPostRequest struct {
	PostType string   `json:"post_type" binding:"required,oneof=article moment"`
	Title    string   `json:"title"`
	Content  string   `json:"content" binding:"required"`
	Tags     []string `json:"tags"`
}

// UpdateCommunityPostRequest 定义更新社区帖子的请求体。
type UpdateCommunityPostRequest struct {
	PostType string   `json:"post_type" binding:"required,oneof=article moment"`
	Title    string   `json:"title"`
	Content  string   `json:"content" binding:"required"`
	Tags     []string `json:"tags"`
}

// CreateCommunityCommentRequest 定义创建评论的请求体。
type CreateCommunityCommentRequest struct {
	Content string `json:"content" binding:"required"`
}

// CommunityService 定义社区模块的业务能力。
type CommunityService interface {
	ListPosts(ctx context.Context, params CommunityPostListParams, currentUserID *uint) (*common.PageResult, error)
	GetPostDetail(ctx context.Context, id uint, currentUserID *uint) (*CommunityPostItem, error)
	CreatePost(ctx context.Context, userID uint, req *CreateCommunityPostRequest) (*CommunityPostItem, error)
	ListMyPosts(ctx context.Context, userID uint, params CommunityPostListParams) (*common.PageResult, error)
	UpdatePost(ctx context.Context, userID, postID uint, req *UpdateCommunityPostRequest) (*CommunityPostItem, error)
	DeletePost(ctx context.Context, userID, postID uint) error
	ListComments(ctx context.Context, postID uint, currentUserID *uint) ([]CommunityCommentItem, error)
	CreateComment(ctx context.Context, userID, postID uint, req *CreateCommunityCommentRequest) (*CommunityCommentItem, error)
	ToggleLike(ctx context.Context, userID, postID uint) (*CommunityLikeToggleResponse, error)
}

type communityService struct {
	communityRepo repository.CommunityRepository
	userRepo      repository.UserRepository
}

// NewCommunityService 创建社区业务服务实现。
func NewCommunityService(
	communityRepo repository.CommunityRepository,
	userRepo repository.UserRepository,
) CommunityService {
	return &communityService{
		communityRepo: communityRepo,
		userRepo:      userRepo,
	}
}

// ListPosts 返回社区帖子分页结果，并附带当前用户的点赞/作者状态。
func (s *communityService) ListPosts(ctx context.Context, params CommunityPostListParams, currentUserID *uint) (*common.PageResult, error) {
	repoParams := repository.CommunityPostListParams{
		Page:     params.Page,
		PageSize: params.PageSize,
		PostType: strings.TrimSpace(params.PostType),
		Keyword:  strings.TrimSpace(params.Keyword),
		Tag:      strings.TrimSpace(params.Tag),
	}

	posts, total, err := s.communityRepo.ListPosts(ctx, repoParams)
	if err != nil {
		return nil, err
	}

	likedMap, err := s.buildLikedMap(ctx, currentUserID, posts)
	if err != nil {
		return nil, err
	}

	items := make([]CommunityPostItem, 0, len(posts))
	for _, post := range posts {
		items = append(items, buildCommunityPostItem(&post, likedMap[post.ID], isCommunityAuthor(post.AuthorID, currentUserID)))
	}

	page := repoParams.Page
	if page <= 0 {
		page = 1
	}
	pageSize := repoParams.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}

	return &common.PageResult{
		List:     items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// GetPostDetail 返回帖子详情，并在读取后增加浏览量。
func (s *communityService) GetPostDetail(ctx context.Context, id uint, currentUserID *uint) (*CommunityPostItem, error) {
	post, err := s.communityRepo.GetPostByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if post == nil {
		return nil, common.NewBusinessError(common.CodeNotFound, "community post not found")
	}

	if err := s.communityRepo.IncrementPostViews(ctx, id); err != nil {
		return nil, err
	}
	post.ViewCount++

	likedMap, err := s.buildLikedMap(ctx, currentUserID, []model.CommunityPost{*post})
	if err != nil {
		return nil, err
	}

	item := buildCommunityPostItem(post, likedMap[post.ID], isCommunityAuthor(post.AuthorID, currentUserID))
	return &item, nil
}

// CreatePost 创建一条新的社区帖子。
func (s *communityService) CreatePost(ctx context.Context, userID uint, req *CreateCommunityPostRequest) (*CommunityPostItem, error) {
	if req == nil {
		return nil, common.NewBusinessError(common.CodeBadRequest, "request body is required")
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, common.NewBusinessError(common.CodeNotFound, "user not found")
	}

	postType, title, content, tags, err := validateCommunityPostPayload(req.PostType, req.Title, req.Content, req.Tags)
	if err != nil {
		return nil, err
	}

	post := &model.CommunityPost{
		AuthorID:  userID,
		PostType:  postType,
		Title:     title,
		Content:   content,
		Summary:   buildSummary(content),
		Tags:      strings.Join(tags, ","),
		ViewCount: 0,
	}

	if err := s.communityRepo.CreatePost(ctx, post); err != nil {
		return nil, err
	}

	post.Author = *user
	item := buildCommunityPostItem(post, false, true)
	return &item, nil
}

// ListMyPosts 返回当前用户自己的帖子列表。
func (s *communityService) ListMyPosts(ctx context.Context, userID uint, params CommunityPostListParams) (*common.PageResult, error) {
	repoParams := repository.CommunityPostListParams{
		Page:     params.Page,
		PageSize: params.PageSize,
		PostType: strings.TrimSpace(params.PostType),
		Keyword:  strings.TrimSpace(params.Keyword),
		Tag:      strings.TrimSpace(params.Tag),
		AuthorID: userID,
	}

	posts, total, err := s.communityRepo.ListPosts(ctx, repoParams)
	if err != nil {
		return nil, err
	}

	currentUserID := userID
	likedMap, err := s.buildLikedMap(ctx, &currentUserID, posts)
	if err != nil {
		return nil, err
	}

	items := make([]CommunityPostItem, 0, len(posts))
	for _, post := range posts {
		items = append(items, buildCommunityPostItem(&post, likedMap[post.ID], true))
	}

	page := repoParams.Page
	if page <= 0 {
		page = 1
	}
	pageSize := repoParams.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}

	return &common.PageResult{
		List:     items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// UpdatePost 更新当前用户自己的社区帖子。
func (s *communityService) UpdatePost(ctx context.Context, userID, postID uint, req *UpdateCommunityPostRequest) (*CommunityPostItem, error) {
	if req == nil {
		return nil, common.NewBusinessError(common.CodeBadRequest, "request body is required")
	}

	post, err := s.communityRepo.GetPostByID(ctx, postID)
	if err != nil {
		return nil, err
	}
	if post == nil {
		return nil, common.NewBusinessError(common.CodeNotFound, "community post not found")
	}
	if post.AuthorID != userID {
		return nil, common.NewBusinessError(common.CodeForbidden, "you cannot edit this community post")
	}

	postType, title, content, tags, err := validateCommunityPostPayload(req.PostType, req.Title, req.Content, req.Tags)
	if err != nil {
		return nil, err
	}

	post.PostType = postType
	post.Title = title
	post.Content = content
	post.Summary = buildSummary(content)
	post.Tags = strings.Join(tags, ",")

	if err := s.communityRepo.UpdatePost(ctx, post); err != nil {
		return nil, err
	}

	updated, err := s.communityRepo.GetPostByID(ctx, postID)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, fmt.Errorf("updated community post not found")
	}

	item := buildCommunityPostItem(updated, false, true)
	return &item, nil
}

// DeletePost 删除当前用户自己的帖子及其关联内容。
func (s *communityService) DeletePost(ctx context.Context, userID, postID uint) error {
	post, err := s.communityRepo.GetPostByID(ctx, postID)
	if err != nil {
		return err
	}
	if post == nil {
		return common.NewBusinessError(common.CodeNotFound, "community post not found")
	}
	if post.AuthorID != userID {
		return common.NewBusinessError(common.CodeForbidden, "you cannot delete this community post")
	}

	return s.communityRepo.DeletePostCascade(ctx, postID)
}

// ListComments 返回指定帖子下的评论列表。
func (s *communityService) ListComments(ctx context.Context, postID uint, currentUserID *uint) ([]CommunityCommentItem, error) {
	post, err := s.communityRepo.GetPostByID(ctx, postID)
	if err != nil {
		return nil, err
	}
	if post == nil {
		return nil, common.NewBusinessError(common.CodeNotFound, "community post not found")
	}

	comments, err := s.communityRepo.ListComments(ctx, postID)
	if err != nil {
		return nil, err
	}

	items := make([]CommunityCommentItem, 0, len(comments))
	for _, comment := range comments {
		items = append(items, buildCommunityCommentItem(&comment, isCommunityAuthor(comment.AuthorID, currentUserID)))
	}

	return items, nil
}

// CreateComment 为指定帖子创建新评论。
func (s *communityService) CreateComment(ctx context.Context, userID, postID uint, req *CreateCommunityCommentRequest) (*CommunityCommentItem, error) {
	if req == nil {
		return nil, common.NewBusinessError(common.CodeBadRequest, "request body is required")
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, common.NewBusinessError(common.CodeNotFound, "user not found")
	}

	post, err := s.communityRepo.GetPostByID(ctx, postID)
	if err != nil {
		return nil, err
	}
	if post == nil {
		return nil, common.NewBusinessError(common.CodeNotFound, "community post not found")
	}

	content := strings.TrimSpace(req.Content)
	if content == "" {
		return nil, common.NewBusinessError(common.CodeBadRequest, "comment content is required")
	}
	if utf8.RuneCountInString(content) > 1000 {
		return nil, common.NewBusinessError(common.CodeBadRequest, "comment content is too long")
	}

	comment := &model.CommunityComment{
		PostID:   postID,
		AuthorID: userID,
		Content:  content,
	}
	if err := s.communityRepo.CreateComment(ctx, comment); err != nil {
		return nil, err
	}

	comment.Author = *user
	item := buildCommunityCommentItem(comment, true)
	return &item, nil
}

// ToggleLike 切换当前用户对指定帖子的点赞状态。
func (s *communityService) ToggleLike(ctx context.Context, userID, postID uint) (*CommunityLikeToggleResponse, error) {
	post, err := s.communityRepo.GetPostByID(ctx, postID)
	if err != nil {
		return nil, err
	}
	if post == nil {
		return nil, common.NewBusinessError(common.CodeNotFound, "community post not found")
	}

	liked, likeCount, err := s.communityRepo.TogglePostLike(ctx, postID, userID)
	if err != nil {
		return nil, err
	}

	return &CommunityLikeToggleResponse{
		Liked:     liked,
		LikeCount: likeCount,
	}, nil
}

// buildCommunityPostItem 将帖子模型转换为前端展示结构。
func buildCommunityPostItem(post *model.CommunityPost, isLiked bool, isAuthor bool) CommunityPostItem {
	return CommunityPostItem{
		ID:            post.ID,
		PostType:      post.PostType,
		Title:         post.Title,
		Content:       post.Content,
		Summary:       post.Summary,
		Tags:          splitTags(post.Tags),
		ViewCount:     post.ViewCount,
		CommentCount:  post.CommentCount,
		LikeCount:     post.LikeCount,
		IsPinned:      post.IsPinned,
		IsRecommended: post.IsRecommended,
		CreatedAt:     post.CreatedAt.Format("2006-01-02 15:04"),
		UpdatedAt:     post.UpdatedAt.Format("2006-01-02 15:04"),
		IsLiked:       isLiked,
		IsAuthor:      isAuthor,
		Author: CommunityPostAuthor{
			ID:       post.Author.ID,
			Username: post.Author.Username,
			Avatar:   post.Author.Avatar,
			Role:     post.Author.Role,
		},
	}
}

// buildCommunityCommentItem 将评论模型转换为前端展示结构。
func buildCommunityCommentItem(comment *model.CommunityComment, isAuthor bool) CommunityCommentItem {
	return CommunityCommentItem{
		ID:        comment.ID,
		Content:   comment.Content,
		CreatedAt: comment.CreatedAt.Format("2006-01-02 15:04"),
		UpdatedAt: comment.UpdatedAt.Format("2006-01-02 15:04"),
		IsAuthor:  isAuthor,
		Author: CommunityPostAuthor{
			ID:       comment.Author.ID,
			Username: comment.Author.Username,
			Avatar:   comment.Author.Avatar,
			Role:     comment.Author.Role,
		},
	}
}

// buildLikedMap 批量查询当前用户对帖子列表的点赞状态。
func (s *communityService) buildLikedMap(ctx context.Context, currentUserID *uint, posts []model.CommunityPost) (map[uint]bool, error) {
	if currentUserID == nil || *currentUserID == 0 || len(posts) == 0 {
		return map[uint]bool{}, nil
	}

	postIDs := make([]uint, 0, len(posts))
	for _, post := range posts {
		postIDs = append(postIDs, post.ID)
	}

	return s.communityRepo.ListLikedPostIDs(ctx, *currentUserID, postIDs)
}

// isCommunityAuthor 判断当前用户是否为对应内容作者。
func isCommunityAuthor(authorID uint, currentUserID *uint) bool {
	return currentUserID != nil && *currentUserID != 0 && authorID == *currentUserID
}

// validateCommunityPostPayload 校验并规范化帖子输入内容。
func validateCommunityPostPayload(postType string, rawTitle string, rawContent string, rawTags []string) (string, string, string, []string, error) {
	normalizedPostType := strings.TrimSpace(postType)
	if normalizedPostType != model.CommunityPostTypeArticle && normalizedPostType != model.CommunityPostTypeMoment {
		return "", "", "", nil, common.NewBusinessError(common.CodeBadRequest, "invalid community post type")
	}

	content := strings.TrimSpace(rawContent)
	if content == "" {
		return "", "", "", nil, common.NewBusinessError(common.CodeBadRequest, "content is required")
	}
	if utf8.RuneCountInString(content) > 5000 {
		return "", "", "", nil, common.NewBusinessError(common.CodeBadRequest, "content is too long")
	}

	title := strings.TrimSpace(rawTitle)
	if normalizedPostType == model.CommunityPostTypeArticle && title == "" {
		return "", "", "", nil, common.NewBusinessError(common.CodeBadRequest, "title is required for article posts")
	}
	if utf8.RuneCountInString(title) > 120 {
		return "", "", "", nil, common.NewBusinessError(common.CodeBadRequest, "title is too long")
	}

	tags := normalizeTags(rawTags)
	return normalizedPostType, title, content, tags, nil
}

// normalizeTags 规范化标签列表，限制数量并去重。
func normalizeTags(tags []string) []string {
	result := make([]string, 0, len(tags))
	seen := map[string]bool{}

	for _, tag := range tags {
		value := strings.TrimSpace(tag)
		if value == "" {
			continue
		}
		if utf8.RuneCountInString(value) > 20 {
			value = string([]rune(value)[:20])
		}
		key := strings.ToLower(value)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, value)
		if len(result) >= 5 {
			break
		}
	}

	return result
}

// splitTags 将逗号分隔的标签文本拆分为数组。
func splitTags(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}

	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		tag := strings.TrimSpace(part)
		if tag != "" {
			result = append(result, tag)
		}
	}
	return result
}

// buildSummary 为帖子正文生成列表摘要文本。
func buildSummary(content string) string {
	runes := []rune(strings.TrimSpace(content))
	if len(runes) <= 120 {
		return string(runes)
	}
	return string(runes[:120]) + "..."
}
