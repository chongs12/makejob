package repository

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"makejob-backend/internal/model"
)

type CommunityPostListParams struct {
	Page     int
	PageSize int
	PostType string
	Keyword  string
	Tag      string
}

type CommunityRepository interface {
	ListPosts(ctx context.Context, params CommunityPostListParams) ([]model.CommunityPost, int64, error)
	GetPostByID(ctx context.Context, id uint) (*model.CommunityPost, error)
	CreatePost(ctx context.Context, post *model.CommunityPost) error
	IncrementPostViews(ctx context.Context, id uint) error
}

type communityRepository struct {
	db *gorm.DB
}

func NewCommunityRepository(db *gorm.DB) CommunityRepository {
	return &communityRepository{db: db}
}

func (r *communityRepository) ListPosts(ctx context.Context, params CommunityPostListParams) ([]model.CommunityPost, int64, error) {
	var posts []model.CommunityPost
	var total int64

	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 10
	}
	if params.PageSize > 50 {
		params.PageSize = 50
	}

	query := r.db.WithContext(ctx).Model(&model.CommunityPost{})

	if params.PostType != "" {
		query = query.Where("post_type = ?", params.PostType)
	}

	if keyword := strings.TrimSpace(params.Keyword); keyword != "" {
		likeKeyword := "%" + keyword + "%"
		query = query.Where("title LIKE ? OR content LIKE ?", likeKeyword, likeKeyword)
	}

	if tag := strings.TrimSpace(params.Tag); tag != "" {
		query = query.Where("tags LIKE ?", "%"+tag+"%")
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count community posts failed: %w", err)
	}

	offset := (params.Page - 1) * params.PageSize
	if err := query.
		Preload("Author").
		Order("is_pinned DESC, created_at DESC").
		Offset(offset).
		Limit(params.PageSize).
		Find(&posts).Error; err != nil {
		return nil, 0, fmt.Errorf("list community posts failed: %w", err)
	}

	return posts, total, nil
}

func (r *communityRepository) GetPostByID(ctx context.Context, id uint) (*model.CommunityPost, error) {
	var post model.CommunityPost
	if err := r.db.WithContext(ctx).Preload("Author").First(&post, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get community post failed: %w", err)
	}

	return &post, nil
}

func (r *communityRepository) CreatePost(ctx context.Context, post *model.CommunityPost) error {
	if err := r.db.WithContext(ctx).Create(post).Error; err != nil {
		return fmt.Errorf("create community post failed: %w", err)
	}
	return nil
}

func (r *communityRepository) IncrementPostViews(ctx context.Context, id uint) error {
	if err := r.db.WithContext(ctx).
		Model(&model.CommunityPost{}).
		Where("id = ?", id).
		UpdateColumn("view_count", gorm.Expr("view_count + 1")).Error; err != nil {
		return fmt.Errorf("increment community post views failed: %w", err)
	}
	return nil
}
