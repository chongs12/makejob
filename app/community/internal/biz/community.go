package biz

import (
	"context"
	"time"

	kratosErr "github.com/go-kratos/kratos/v2/errors"
)

var (
	ErrPostNotFound    = kratosErr.NotFound("POST_NOT_FOUND", "帖子不存在")
	ErrCommentNotFound = kratosErr.NotFound("COMMENT_NOT_FOUND", "评论不存在")
	ErrForbidden       = kratosErr.Forbidden("FORBIDDEN", "无权操作")
)

// CommunityRepo data 层必须实现的接口
type CommunityRepo interface {
	ListPosts(ctx context.Context, page, pageSize int32) ([]*Post, int64, error)
	CreatePost(ctx context.Context, post *Post) error
	GetPost(ctx context.Context, id uint64) (*Post, error)
	DeletePost(ctx context.Context, id, authorID uint64) error
	CreateComment(ctx context.Context, comment *Comment) error
	ListComments(ctx context.Context, postID uint64, page, pageSize int32) ([]*Comment, int64, error)
}

// --- 领域实体 ---

type Post struct {
	ID         uint64
	AuthorID   uint64
	Title      string
	Content    string
	Category   string
	AuthorName string
	CreatedAt  time.Time
}

type Comment struct {
	ID        uint64
	PostID    uint64
	AuthorID  uint64
	Content   string
	CreatedAt time.Time
}

// CommunityUseCase 社区业务用例
type CommunityUseCase struct {
	repo CommunityRepo
}

// NewCommunityUseCase 创建社区用例
func NewCommunityUseCase(repo CommunityRepo) *CommunityUseCase {
	return &CommunityUseCase{repo: repo}
}

// ListPosts 获取帖子列表
func (uc *CommunityUseCase) ListPosts(ctx context.Context, page, pageSize int32) ([]*Post, int64, error) {
	return uc.repo.ListPosts(ctx, page, pageSize)
}

// CreatePost 创建帖子
func (uc *CommunityUseCase) CreatePost(ctx context.Context, authorID uint64, title, content, category string) (*Post, error) {
	post := &Post{
		AuthorID: authorID,
		Title:    title,
		Content:  content,
		Category: category,
	}
	if err := uc.repo.CreatePost(ctx, post); err != nil {
		return nil, err
	}
	return post, nil
}

// GetPost 获取帖子详情
func (uc *CommunityUseCase) GetPost(ctx context.Context, id uint64) (*Post, error) {
	post, err := uc.repo.GetPost(ctx, id)
	if err != nil {
		return nil, err
	}
	if post == nil {
		return nil, ErrPostNotFound
	}
	return post, nil
}

// DeletePost 删除帖子（仅作者本人可删除）
func (uc *CommunityUseCase) DeletePost(ctx context.Context, id, authorID uint64) error {
	post, err := uc.repo.GetPost(ctx, id)
	if err != nil {
		return err
	}
	if post == nil {
		return ErrPostNotFound
	}
	if post.AuthorID != authorID {
		return ErrForbidden
	}
	return uc.repo.DeletePost(ctx, id, authorID)
}

// CreateComment 创建评论
func (uc *CommunityUseCase) CreateComment(ctx context.Context, postID, authorID uint64, content string) (*Comment, error) {
	post, err := uc.repo.GetPost(ctx, postID)
	if err != nil {
		return nil, err
	}
	if post == nil {
		return nil, ErrPostNotFound
	}
	comment := &Comment{
		PostID:   postID,
		AuthorID: authorID,
		Content:  content,
	}
	if err := uc.repo.CreateComment(ctx, comment); err != nil {
		return nil, err
	}
	return comment, nil
}

// ListComments 获取评论列表
func (uc *CommunityUseCase) ListComments(ctx context.Context, postID uint64, page, pageSize int32) ([]*Comment, int64, error) {
	return uc.repo.ListComments(ctx, postID, page, pageSize)
}
