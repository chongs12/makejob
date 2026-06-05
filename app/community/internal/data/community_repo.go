package data

import (
	"context"

	"gorm.io/gorm"

	"makejob/app/community/internal/biz"
	"makejob/app/community/internal/data/model"
)

type communityRepo struct {
	db *gorm.DB
}

// NewCommunityRepo 创建社区仓库实现
func NewCommunityRepo(db *gorm.DB) biz.CommunityRepo {
	return &communityRepo{db: db}
}

func (r *communityRepo) ListPosts(ctx context.Context, page, pageSize int32) ([]*biz.Post, int64, error) {
	var posts []*biz.Post
	var total int64

	r.db.WithContext(ctx).Model(&biz.Post{}).Count(&total)
	offset := (page - 1) * pageSize
	if err := r.db.WithContext(ctx).Order("created_at DESC").Offset(int(offset)).Limit(int(pageSize)).Find(&posts).Error; err != nil {
		return nil, 0, err
	}
	return posts, total, nil
}

func (r *communityRepo) CreatePost(ctx context.Context, post *biz.Post) error {
	return r.db.WithContext(ctx).Create(post).Error
}

func (r *communityRepo) GetPost(ctx context.Context, id uint64) (*biz.Post, error) {
	var post biz.Post
	if err := r.db.WithContext(ctx).First(&post, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &post, nil
}

func (r *communityRepo) DeletePost(ctx context.Context, id, authorID uint64) error {
	result := r.db.WithContext(ctx).Where("id = ? AND author_id = ?", id, authorID).Delete(&biz.Post{})
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
	if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
		return err
	}
	comment.ID = uint64(m.ID)
	comment.CreatedAt = m.CreatedAt
	return nil
}

func (r *communityRepo) ListComments(ctx context.Context, postID uint64, page, pageSize int32) ([]*biz.Comment, int64, error) {
	var total int64
	query := r.db.WithContext(ctx).Model(&model.CommunityComment{}).Where("post_id = ?", postID)
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
