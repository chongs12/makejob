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

type CommunityPostAuthor struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
	Role     string `json:"role"`
}

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
	Author        CommunityPostAuthor `json:"author"`
}

type CommunityPostListParams struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	PostType string `form:"type"`
	Keyword  string `form:"keyword"`
	Tag      string `form:"tag"`
}

type CreateCommunityPostRequest struct {
	PostType string   `json:"post_type" binding:"required,oneof=article moment"`
	Title    string   `json:"title"`
	Content  string   `json:"content" binding:"required"`
	Tags     []string `json:"tags"`
}

type CommunityService interface {
	ListPosts(ctx context.Context, params CommunityPostListParams) (*common.PageResult, error)
	GetPostDetail(ctx context.Context, id uint) (*CommunityPostItem, error)
	CreatePost(ctx context.Context, userID uint, req *CreateCommunityPostRequest) (*CommunityPostItem, error)
}

type communityService struct {
	communityRepo repository.CommunityRepository
	userRepo      repository.UserRepository
}

func NewCommunityService(
	communityRepo repository.CommunityRepository,
	userRepo repository.UserRepository,
) CommunityService {
	return &communityService{
		communityRepo: communityRepo,
		userRepo:      userRepo,
	}
}

func (s *communityService) ListPosts(ctx context.Context, params CommunityPostListParams) (*common.PageResult, error) {
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

	items := make([]CommunityPostItem, 0, len(posts))
	for _, post := range posts {
		items = append(items, buildCommunityPostItem(&post))
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

func (s *communityService) GetPostDetail(ctx context.Context, id uint) (*CommunityPostItem, error) {
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

	item := buildCommunityPostItem(post)
	return &item, nil
}

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

	content := strings.TrimSpace(req.Content)
	if content == "" {
		return nil, common.NewBusinessError(common.CodeBadRequest, "content is required")
	}
	if utf8.RuneCountInString(content) > 5000 {
		return nil, common.NewBusinessError(common.CodeBadRequest, "content is too long")
	}

	title := strings.TrimSpace(req.Title)
	if req.PostType == model.CommunityPostTypeArticle {
		if title == "" {
			return nil, common.NewBusinessError(common.CodeBadRequest, "title is required for article posts")
		}
		if utf8.RuneCountInString(title) > 120 {
			return nil, common.NewBusinessError(common.CodeBadRequest, "title is too long")
		}
	} else if utf8.RuneCountInString(title) > 120 {
		return nil, common.NewBusinessError(common.CodeBadRequest, "title is too long")
	}

	tags := normalizeTags(req.Tags)
	post := &model.CommunityPost{
		AuthorID:  userID,
		PostType:  req.PostType,
		Title:     title,
		Content:   content,
		Summary:   buildSummary(content),
		Tags:      strings.Join(tags, ","),
		ViewCount: 0,
	}

	if err := s.communityRepo.CreatePost(ctx, post); err != nil {
		return nil, err
	}

	created, err := s.communityRepo.GetPostByID(ctx, post.ID)
	if err != nil {
		return nil, err
	}
	if created == nil {
		return nil, fmt.Errorf("created community post not found")
	}

	item := buildCommunityPostItem(created)
	return &item, nil
}

func buildCommunityPostItem(post *model.CommunityPost) CommunityPostItem {
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
		Author: CommunityPostAuthor{
			ID:       post.Author.ID,
			Username: post.Author.Username,
			Avatar:   post.Author.Avatar,
			Role:     post.Author.Role,
		},
	}
}

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

func buildSummary(content string) string {
	runes := []rune(strings.TrimSpace(content))
	if len(runes) <= 120 {
		return string(runes)
	}
	return string(runes[:120]) + "..."
}
