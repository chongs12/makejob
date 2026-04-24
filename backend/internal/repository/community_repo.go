package repository

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"makejob-backend/internal/model"
)

// CommunityPostListParams 定义社区帖子列表查询参数。
type CommunityPostListParams struct {
	Page     int
	PageSize int
	PostType string
	Keyword  string
	Tag      string
	AuthorID uint
}

// CommunityRepository 定义社区模块的数据访问能力。
type CommunityRepository interface {
	ListPosts(ctx context.Context, params CommunityPostListParams) ([]model.CommunityPost, int64, error)
	GetPostByID(ctx context.Context, id uint) (*model.CommunityPost, error)
	CreatePost(ctx context.Context, post *model.CommunityPost) error
	UpdatePost(ctx context.Context, post *model.CommunityPost) error
	DeletePostCascade(ctx context.Context, id uint) error
	IncrementPostViews(ctx context.Context, id uint) error
	ListComments(ctx context.Context, postID uint) ([]model.CommunityComment, error)
	CreateComment(ctx context.Context, comment *model.CommunityComment) error
	TogglePostLike(ctx context.Context, postID, userID uint) (bool, int, error)
	ListLikedPostIDs(ctx context.Context, userID uint, postIDs []uint) (map[uint]bool, error)
}

type communityRepository struct {
	db *gorm.DB
}

// NewCommunityRepository 创建社区仓储实现。
func NewCommunityRepository(db *gorm.DB) CommunityRepository {
	return &communityRepository{db: db}
}

// ListPosts 按筛选条件返回社区帖子分页结果。
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

	if params.AuthorID > 0 {
		query = query.Where("author_id = ?", params.AuthorID)
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

// GetPostByID 根据帖子 ID 读取完整帖子信息。
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

// CreatePost 创建一条新的社区帖子记录。
func (r *communityRepository) CreatePost(ctx context.Context, post *model.CommunityPost) error {
	if err := r.db.WithContext(ctx).Create(post).Error; err != nil {
		return fmt.Errorf("create community post failed: %w", err)
	}
	return nil
}

// UpdatePost 更新社区帖子的可编辑字段。
func (r *communityRepository) UpdatePost(ctx context.Context, post *model.CommunityPost) error {
	updates := map[string]interface{}{
		"post_type": post.PostType,
		"title":     post.Title,
		"content":   post.Content,
		"summary":   post.Summary,
		"tags":      post.Tags,
	}
	if err := r.db.WithContext(ctx).
		Model(&model.CommunityPost{}).
		Where("id = ?", post.ID).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("update community post failed: %w", err)
	}
	return nil
}

// DeletePostCascade 删除帖子及其评论、点赞等关联数据。
func (r *communityRepository) DeletePostCascade(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("post_id = ?", id).Delete(&model.CommunityComment{}).Error; err != nil {
			return fmt.Errorf("delete community comments failed: %w", err)
		}
		if err := tx.Unscoped().Where("post_id = ?", id).Delete(&model.CommunityPostLike{}).Error; err != nil {
			return fmt.Errorf("delete community post likes failed: %w", err)
		}
		if err := tx.Delete(&model.CommunityPost{}, id).Error; err != nil {
			return fmt.Errorf("delete community post failed: %w", err)
		}
		return nil
	})
}

// IncrementPostViews 为帖子浏览量执行原子递增。
func (r *communityRepository) IncrementPostViews(ctx context.Context, id uint) error {
	if err := r.db.WithContext(ctx).
		Model(&model.CommunityPost{}).
		Where("id = ?", id).
		UpdateColumn("view_count", gorm.Expr("view_count + 1")).Error; err != nil {
		return fmt.Errorf("increment community post views failed: %w", err)
	}
	return nil
}

// ListComments 返回某个帖子下的评论列表。
func (r *communityRepository) ListComments(ctx context.Context, postID uint) ([]model.CommunityComment, error) {
	var comments []model.CommunityComment
	if err := r.db.WithContext(ctx).
		Where("post_id = ?", postID).
		Preload("Author").
		Order("created_at ASC").
		Find(&comments).Error; err != nil {
		return nil, fmt.Errorf("list community comments failed: %w", err)
	}
	return comments, nil
}

// CreateComment 创建评论并同步更新帖子评论数。
func (r *communityRepository) CreateComment(ctx context.Context, comment *model.CommunityComment) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(comment).Error; err != nil {
			return fmt.Errorf("create community comment failed: %w", err)
		}
		if err := tx.Model(&model.CommunityPost{}).
			Where("id = ?", comment.PostID).
			UpdateColumn("comment_count", gorm.Expr("comment_count + 1")).Error; err != nil {
			return fmt.Errorf("increment community post comment count failed: %w", err)
		}
		return nil
	})
}

// TogglePostLike 切换点赞状态并返回最新点赞结果。
func (r *communityRepository) TogglePostLike(ctx context.Context, postID, userID uint) (bool, int, error) {
	var liked bool
	var likeCount int

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var like model.CommunityPostLike
		err := tx.Where("post_id = ? AND user_id = ?", postID, userID).First(&like).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			return fmt.Errorf("get community post like failed: %w", err)
		}

		if err == gorm.ErrRecordNotFound {
			newLike := &model.CommunityPostLike{
				PostID: postID,
				UserID: userID,
			}
			if createErr := tx.Create(newLike).Error; createErr != nil {
				return fmt.Errorf("create community post like failed: %w", createErr)
			}
			if updateErr := tx.Model(&model.CommunityPost{}).
				Where("id = ?", postID).
				UpdateColumn("like_count", gorm.Expr("like_count + 1")).Error; updateErr != nil {
				return fmt.Errorf("increment community post like count failed: %w", updateErr)
			}
			liked = true
		} else {
			if deleteErr := tx.Unscoped().Delete(&like).Error; deleteErr != nil {
				return fmt.Errorf("delete community post like failed: %w", deleteErr)
			}
			if updateErr := tx.Model(&model.CommunityPost{}).
				Where("id = ?", postID).
				UpdateColumn("like_count", gorm.Expr("GREATEST(like_count - 1, 0)")).Error; updateErr != nil {
				return fmt.Errorf("decrement community post like count failed: %w", updateErr)
			}
			liked = false
		}

		if countErr := tx.Model(&model.CommunityPost{}).
			Where("id = ?", postID).
			Pluck("like_count", &likeCount).Error; countErr != nil {
			return fmt.Errorf("read community post like count failed: %w", countErr)
		}

		return nil
	})
	if err != nil {
		return false, 0, err
	}

	return liked, likeCount, nil
}

// ListLikedPostIDs 返回当前用户已点赞的帖子集合。
func (r *communityRepository) ListLikedPostIDs(ctx context.Context, userID uint, postIDs []uint) (map[uint]bool, error) {
	result := make(map[uint]bool, len(postIDs))
	if userID == 0 || len(postIDs) == 0 {
		return result, nil
	}

	var likedPostIDs []uint
	if err := r.db.WithContext(ctx).
		Model(&model.CommunityPostLike{}).
		Where("user_id = ? AND post_id IN ?", userID, postIDs).
		Pluck("post_id", &likedPostIDs).Error; err != nil {
		return nil, fmt.Errorf("list liked community post ids failed: %w", err)
	}

	for _, postID := range likedPostIDs {
		result[postID] = true
	}

	return result, nil
}
