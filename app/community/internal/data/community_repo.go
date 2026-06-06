package data

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"makejob/app/community/internal/biz"
	"makejob/app/community/internal/data/model"
)

// txContextKey 事务 DB 的 context key
type txContextKey struct{}

type communityRepo struct {
	db *gorm.DB
}

// NewCommunityRepo 创建社区仓库实现
func NewCommunityRepo(db *gorm.DB) biz.CommunityRepo {
	return &communityRepo{db: db}
}

// getDB 从 context 获取事务 DB，若无则返回默认 DB
func (r *communityRepo) getDB(ctx context.Context) *gorm.DB {
	if tx, ok := ctx.Value(txContextKey{}).(*gorm.DB); ok {
		return tx
	}
	return r.db
}

// RunInTransaction FIX B4: 在事务中执行 fn，fn 内所有 repo 操作共享同一事务
func (r *communityRepo) RunInTransaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txCtx := context.WithValue(ctx, txContextKey{}, tx)
		return fn(txCtx)
	})
}

func (r *communityRepo) ListPosts(ctx context.Context, page, pageSize int32) ([]*biz.Post, int64, error) {
	var posts []*biz.Post
	var total int64

	db := r.getDB(ctx).WithContext(ctx)
	db.Model(&biz.Post{}).Count(&total)
	offset := (page - 1) * pageSize
	if err := db.Order("created_at DESC").Offset(int(offset)).Limit(int(pageSize)).Find(&posts).Error; err != nil {
		return nil, 0, err
	}
	return posts, total, nil
}

func (r *communityRepo) CreatePost(ctx context.Context, post *biz.Post) error {
	return r.getDB(ctx).WithContext(ctx).Create(post).Error
}

func (r *communityRepo) GetPost(ctx context.Context, id uint64) (*biz.Post, error) {
	var post biz.Post
	if err := r.getDB(ctx).WithContext(ctx).First(&post, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &post, nil
}

// Update 更新帖子指定字段（S3: 使用 Updates 避免覆盖非目标字段）
func (r *communityRepo) Update(ctx context.Context, post *biz.Post) error {
	return r.getDB(ctx).WithContext(ctx).Model(post).Updates(map[string]any{
		"title":     post.Title,
		"content":   post.Content,
		"tags":      post.Tags,
		"summary":   post.Summary,
		"post_type": post.PostType,
	}).Error
}

func (r *communityRepo) DeletePost(ctx context.Context, id, authorID uint64) error {
	result := r.getDB(ctx).WithContext(ctx).Where("id = ? AND author_id = ?", id, authorID).Delete(&biz.Post{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return biz.ErrPostNotFound
	}
	return nil
}

func (r *communityRepo) CreateComment(ctx context.Context, comment *biz.Comment) error {
	m := &model.CommunityComment{
		PostID:   comment.PostID,
		AuthorID: comment.AuthorID,
		Content:  comment.Content,
	}
	if err := r.getDB(ctx).WithContext(ctx).Create(m).Error; err != nil {
		return err
	}
	comment.ID = uint64(m.ID)
	comment.CreatedAt = m.CreatedAt
	return nil
}

func (r *communityRepo) ListComments(ctx context.Context, postID uint64, page, pageSize int32) ([]*biz.Comment, int64, error) {
	var total int64
	db := r.getDB(ctx).WithContext(ctx)
	query := db.Model(&model.CommunityComment{}).Where("post_id = ?", postID)
	query.Count(&total)

	var models []*model.CommunityComment
	offset := (page - 1) * pageSize
	if err := query.Order("created_at ASC").Offset(int(offset)).Limit(int(pageSize)).Find(&models).Error; err != nil {
		return nil, 0, err
	}

	comments := make([]*biz.Comment, len(models))
	for i, m := range models {
		comments[i] = &biz.Comment{
			ID:        uint64(m.ID),
			PostID:    m.PostID,
			AuthorID:  m.AuthorID,
			Content:   m.Content,
			CreatedAt: m.CreatedAt,
		}
	}
	return comments, total, nil
}

// IncrementLikeCount 原子增减帖子点赞计数
func (r *communityRepo) IncrementLikeCount(ctx context.Context, postID uint64, delta int32) error {
	return r.getDB(ctx).WithContext(ctx).Model(&biz.Post{}).Where("id = ?", postID).
		Update("like_count", gorm.Expr("like_count + ?", delta)).Error
}

// ListByAuthorID 按作者 ID 分页查询帖子列表
func (r *communityRepo) ListByAuthorID(ctx context.Context, authorID uint64, page, pageSize int32) ([]*biz.Post, int64, error) {
	var posts []*biz.Post
	var total int64

	db := r.getDB(ctx).WithContext(ctx)
	query := db.Model(&biz.Post{}).Where("author_id = ?", authorID)
	query.Count(&total)

	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(int(offset)).Limit(int(pageSize)).Find(&posts).Error; err != nil {
		return nil, 0, err
	}
	return posts, total, nil
}

// ListPostsFiltered FIX B5: 支持 post_type/keyword/tag 动态过滤和 sort_by 排序
func (r *communityRepo) ListPostsFiltered(ctx context.Context, page, pageSize int32, filter biz.PostFilter) ([]*biz.Post, int64, error) {
	var posts []*biz.Post
	var total int64

	db := r.getDB(ctx).WithContext(ctx)
	query := db.Model(&biz.Post{})

	if filter.PostType != "" {
		query = query.Where("post_type = ?", filter.PostType)
	}
	if filter.Keyword != "" {
		like := fmt.Sprintf("%%%s%%", filter.Keyword)
		query = query.Where("(title LIKE ? OR content LIKE ?)", like, like)
	}
	if filter.Tag != "" {
		like := fmt.Sprintf("%%%s%%", filter.Tag)
		query = query.Where("tags LIKE ?", like)
	}

	query.Count(&total)

	orderBy := "created_at DESC"
	switch filter.SortBy {
	case "popular":
		orderBy = "view_count DESC"
	case "most_liked":
		orderBy = "like_count DESC"
	}

	offset := (page - 1) * pageSize
	if err := query.Order(orderBy).Offset(int(offset)).Limit(int(pageSize)).Find(&posts).Error; err != nil {
		return nil, 0, err
	}
	return posts, total, nil
}

// IncrementViewCount FIX B5: 原子自增浏览量
func (r *communityRepo) IncrementViewCount(ctx context.Context, postID uint64) error {
	return r.getDB(ctx).WithContext(ctx).Model(&biz.Post{}).Where("id = ?", postID).
		Update("view_count", gorm.Expr("view_count + 1")).Error
}

// DeletePostWithAssociations FIX B5: 事务中级联删除帖子、评论和点赞
func (r *communityRepo) DeletePostWithAssociations(ctx context.Context, id, authorID uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 先删除关联的评论
		if err := tx.Where("post_id = ?", id).Delete(&model.CommunityComment{}).Error; err != nil {
			return err
		}
		// 再删除关联的点赞
		if err := tx.Where("post_id = ?", id).Delete(&model.CommunityLike{}).Error; err != nil {
			return err
		}
		// 最后删除帖子本身
		result := tx.Where("id = ? AND author_id = ?", id, authorID).Delete(&biz.Post{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return biz.ErrPostNotFound
		}
		return nil
	})
}

// --- 点赞仓库实现 ---

type likeRepo struct {
	db *gorm.DB
}

// NewLikeRepo 创建点赞仓库实现
func NewLikeRepo(db *gorm.DB) biz.LikeRepo {
	return &likeRepo{db: db}
}

// getDB 从 context 获取事务 DB，若无则返回默认 DB
func (r *likeRepo) getDB(ctx context.Context) *gorm.DB {
	if tx, ok := ctx.Value(txContextKey{}).(*gorm.DB); ok {
		return tx
	}
	return r.db
}

// GetByPostAndUser 查询指定用户对指定帖子的点赞记录
func (r *likeRepo) GetByPostAndUser(ctx context.Context, postID, userID uint64) (*biz.PostLike, error) {
	var m model.CommunityLike
	if err := r.getDB(ctx).WithContext(ctx).Where("post_id = ? AND user_id = ?", postID, userID).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &biz.PostLike{
		ID:        m.ID,
		PostID:    m.PostID,
		UserID:    m.UserID,
		CreatedAt: m.CreatedAt,
	}, nil
}

// Create 创建点赞记录
func (r *likeRepo) Create(ctx context.Context, like *biz.PostLike) error {
	m := &model.CommunityLike{
		PostID: like.PostID,
		UserID: like.UserID,
	}
	if err := r.getDB(ctx).WithContext(ctx).Create(m).Error; err != nil {
		return err
	}
	like.ID = m.ID
	like.CreatedAt = m.CreatedAt
	return nil
}

// Delete 删除点赞记录
func (r *likeRepo) Delete(ctx context.Context, postID, userID uint64) error {
	result := r.getDB(ctx).WithContext(ctx).Where("post_id = ? AND user_id = ?", postID, userID).Delete(&model.CommunityLike{})
	if result.Error != nil {
		return result.Error
	}
	return nil
}
