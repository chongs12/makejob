package biz

import (
	"context"
	"strings"
	"time"

	kratosErr "github.com/go-kratos/kratos/v2/errors"
	"gorm.io/gorm"

	"makejob/pkg/auth"
)

var (
	ErrPostNotFound    = kratosErr.NotFound("POST_NOT_FOUND", "帖子不存在")
	ErrCommentNotFound = kratosErr.NotFound("COMMENT_NOT_FOUND", "评论不存在")
	ErrForbidden       = kratosErr.Forbidden("FORBIDDEN", "无权操作")
	ErrTitleRequired   = kratosErr.BadRequest("TITLE_REQUIRED", "标题不能为空")
	ErrTitleTooLong    = kratosErr.BadRequest("TITLE_TOO_LONG", "标题不能超过120字符")
	ErrContentTooLong  = kratosErr.BadRequest("CONTENT_TOO_LONG", "内容不能超过5000字符")
	ErrTooManyTags     = kratosErr.BadRequest("TOO_MANY_TAGS", "标签不能超过5个")
	ErrPostTypeInvalid = kratosErr.BadRequest("POST_TYPE_INVALID", "帖子类型无效，仅支持 discussion/article/question") // FIX B5
)

// PostFilter 帖子查询过滤条件（FIX B5）
type PostFilter struct {
	PostType string // discussion/article/question
	Keyword  string // 搜索标题/内容
	Tag      string // 标签过滤
	SortBy   string // latest/popular/most_liked
}

// CommunityRepo data 层必须实现的接口
type CommunityRepo interface {
	ListPosts(ctx context.Context, page, pageSize int32) ([]*Post, int64, error)
	ListPostsFiltered(ctx context.Context, page, pageSize int32, filter PostFilter) ([]*Post, int64, error) // FIX B5
	CreatePost(ctx context.Context, post *Post) error
	GetPost(ctx context.Context, id uint64) (*Post, error)
	Update(ctx context.Context, post *Post) error
	DeletePost(ctx context.Context, id, authorID uint64) error
	DeletePostWithAssociations(ctx context.Context, id, authorID uint64) error // FIX B5: 级联删除
	CreateComment(ctx context.Context, comment *Comment) error
	ListComments(ctx context.Context, postID uint64, page, pageSize int32) ([]*Comment, int64, error)
	IncrementLikeCount(ctx context.Context, postID uint64, delta int32) error
	IncrementViewCount(ctx context.Context, postID uint64) error // FIX B5: 浏览量自增
	ListByAuthorID(ctx context.Context, authorID uint64, page, pageSize int32) ([]*Post, int64, error)
	// RunInTransaction FIX B4: 在事务中执行 fn，fn 内所有 repo 操作共享同一事务
	RunInTransaction(ctx context.Context, fn func(txCtx context.Context) error) error
}

// --- 领域实体 ---

// BaseModel 所有实体公共基础字段（FIX B6: 符合全局规范 1.4）
type BaseModel struct {
	ID        uint           `gorm:"primaryKey;autoIncrement"`
	CreatedAt time.Time      `gorm:"not null;autoCreateTime"`
	UpdatedAt time.Time      `gorm:"not null;autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// Post 帖子领域实体（FIX B6: 添加 GORM tag、BaseModel、TableName、PostType、ViewCount）
type Post struct {
	BaseModel
	AuthorID     uint64 `gorm:"index;not null"`
	Title        string `gorm:"size:200;not null"`
	Content      string `gorm:"type:text"`
	Summary      string `gorm:"size:500"`
	Tags         string `gorm:"size:500"`
	PostType     string `gorm:"size:20;not null;default:'discussion'"` // FIX B6: discussion/article/question
	Category     string `gorm:"size:50"`
	AuthorName   string `gorm:"size:100"`
	LikeCount    int32  `gorm:"not null;default:0"`
	CommentCount int32  `gorm:"not null;default:0"`
	ViewCount    int32  `gorm:"not null;default:0"` // FIX B6: 浏览量字段
}

// TableName 返回帖子表名（FIX B6: 符合全局规范）
func (Post) TableName() string { return "community_posts" }

type Comment struct {
	ID        uint64
	PostID    uint64
	AuthorID  uint64
	Content   string
	CreatedAt time.Time
}

// PostLike 帖子点赞领域实体
type PostLike struct {
	ID        uint64
	PostID    uint64
	UserID    uint64
	CreatedAt time.Time
}

// LikeRepo 点赞数据层接口
type LikeRepo interface {
	GetByPostAndUser(ctx context.Context, postID, userID uint64) (*PostLike, error)
	Create(ctx context.Context, like *PostLike) error
	Delete(ctx context.Context, postID, userID uint64) error
}

// CommunityUseCase 社区业务用例
type CommunityUseCase struct {
	repo     CommunityRepo
	likeRepo LikeRepo
}

// NewCommunityUseCase 创建社区用例
func NewCommunityUseCase(repo CommunityRepo, likeRepo LikeRepo) *CommunityUseCase {
	return &CommunityUseCase{repo: repo, likeRepo: likeRepo}
}

// ListPosts 获取帖子列表（FIX B5: 支持过滤和排序）
func (uc *CommunityUseCase) ListPosts(ctx context.Context, page, pageSize int32, filter PostFilter) ([]*Post, int64, error) {
	return uc.repo.ListPostsFiltered(ctx, page, pageSize, filter)
}

// CreatePost 创建帖子（FIX B5: 添加 post_type/title/content 校验）
func (uc *CommunityUseCase) CreatePost(ctx context.Context, authorID uint64, title, content, postType, category, tags string) (*Post, error) {
	// post_type 必填校验
	if postType == "" {
		return nil, ErrPostTypeInvalid
	}
	if postType != "discussion" && postType != "article" && postType != "question" {
		return nil, ErrPostTypeInvalid
	}
	// title 非空校验
	if title == "" {
		return nil, ErrTitleRequired
	}
	if len([]rune(title)) > 120 {
		return nil, ErrTitleTooLong
	}
	// content 长度校验
	if len([]rune(content)) > 5000 {
		return nil, ErrContentTooLong
	}
	// tags 数量校验
	if tags != "" {
		tagList := strings.Split(tags, ",")
		if len(tagList) > 5 {
			return nil, ErrTooManyTags
		}
	}

	// 计算摘要
	runes := []rune(content)
	summary := content
	if len(runes) > 200 {
		summary = string(runes[:200])
	}

	post := &Post{
		AuthorID: authorID,
		Title:    title,
		Content:  content,
		Summary:  summary,
		PostType: postType,
		Category: category,
		Tags:     tags,
	}
	if err := uc.repo.CreatePost(ctx, post); err != nil {
		return nil, err
	}
	return post, nil
}

// GetPost 获取帖子详情（FIX B5: 增加浏览量统计）
func (uc *CommunityUseCase) GetPost(ctx context.Context, id uint64) (*Post, error) {
	post, err := uc.repo.GetPost(ctx, id)
	if err != nil {
		return nil, err
	}
	if post == nil {
		return nil, ErrPostNotFound
	}
	// FIX B5: 原子自增浏览量（异步，不阻塞主查询）
	go uc.repo.IncrementViewCount(context.Background(), id)
	return post, nil
}

// DeletePost 删除帖子（仅作者本人可删除，FIX B5: 级联删除评论和点赞）
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
	return uc.repo.DeletePostWithAssociations(ctx, id, authorID)
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

// UpdatePost 更新帖子（仅作者本人可操作，校验字段长度和标签数量）
func (uc *CommunityUseCase) UpdatePost(ctx context.Context, postID uint64, title, content, tags string) (*Post, error) {
	if title == "" {
		return nil, ErrTitleRequired
	}
	if len([]rune(title)) > 120 {
		return nil, ErrTitleTooLong
	}
	if len([]rune(content)) > 5000 {
		return nil, ErrContentTooLong
	}
	if tags != "" {
		tagList := strings.Split(tags, ",")
		if len(tagList) > 5 {
			return nil, ErrTooManyTags
		}
	}

	post, err := uc.repo.GetPost(ctx, postID)
	if err != nil {
		return nil, err
	}
	if post == nil {
		return nil, ErrPostNotFound
	}

	currentUserID := auth.GetUserIDFromContext(ctx)
	if post.AuthorID != currentUserID {
		return nil, ErrForbidden
	}

	post.Title = title
	post.Content = content
	post.Tags = tags
	// 重新计算摘要：取 content 前 200 个字符
	runes := []rune(content)
	if len(runes) > 200 {
		post.Summary = string(runes[:200])
	} else {
		post.Summary = content
	}

	if err := uc.repo.Update(ctx, post); err != nil {
		return nil, err
	}
	return post, nil
}

// ToggleLike 切换帖子点赞状态（已赞则取消，未赞则点赞）
// FIX B4: 使用事务保证原子性，避免 like_count 与实际记录不一致
func (uc *CommunityUseCase) ToggleLike(ctx context.Context, postID uint64) (bool, int32, error) {
	userID := auth.GetUserIDFromContext(ctx)

	// 先验证帖子存在（事务外）
	post, err := uc.repo.GetPost(ctx, postID)
	if err != nil {
		return false, 0, err
	}
	if post == nil {
		return false, 0, ErrPostNotFound
	}

	var liked bool
	var likeCount int32

	// 在事务中完成所有写操作
	err = uc.repo.RunInTransaction(ctx, func(txCtx context.Context) error {
		existing, err := uc.likeRepo.GetByPostAndUser(txCtx, postID, userID)
		if err != nil {
			return err
		}

		if existing != nil {
			// 已赞 → 取消点赞
			if err := uc.likeRepo.Delete(txCtx, postID, userID); err != nil {
				return err
			}
			if err := uc.repo.IncrementLikeCount(txCtx, postID, -1); err != nil {
				return err
			}
			liked = false
			likeCount = post.LikeCount - 1
		} else {
			// 未赞 → 点赞
			like := &PostLike{
				PostID: postID,
				UserID: userID,
			}
			if err := uc.likeRepo.Create(txCtx, like); err != nil {
				return err
			}
			if err := uc.repo.IncrementLikeCount(txCtx, postID, 1); err != nil {
				return err
			}
			liked = true
			likeCount = post.LikeCount + 1
		}
		return nil
	})
	if err != nil {
		return false, 0, err
	}

	return liked, likeCount, nil
}

// ListMyPosts 获取当前用户发布的帖子列表
func (uc *CommunityUseCase) ListMyPosts(ctx context.Context, page, pageSize int32) ([]*Post, int64, error) {
	userID := auth.GetUserIDFromContext(ctx)
	return uc.repo.ListByAuthorID(ctx, userID, page, pageSize)
}
