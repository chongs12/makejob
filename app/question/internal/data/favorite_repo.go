package data

import (
	"context"

	"gorm.io/gorm"

	"makejob/app/question/internal/biz"
	"makejob/app/question/internal/data/model"
)

type favoriteRepo struct {
	db *gorm.DB
}

func NewFavoriteRepo(db *gorm.DB) biz.FavoriteRepo {
	return &favoriteRepo{db: db}
}

func (r *favoriteRepo) Create(ctx context.Context, fav *biz.UserFavorite) error {
	m := &model.UserFavorite{
		UserID:     fav.UserID,
		QuestionID: fav.QuestionID,
	}
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *favoriteRepo) Delete(ctx context.Context, userID, questionID uint64) error {
	return r.db.WithContext(ctx).
		Where("user_id = ? AND question_id = ?", userID, questionID).
		Delete(&model.UserFavorite{}).Error
}

func (r *favoriteRepo) ListByUser(ctx context.Context, userID uint64, page, pageSize int32) ([]*biz.Question, int64, error) {
	var total int64
	r.db.WithContext(ctx).
		Model(&model.UserFavorite{}).
		Where("user_id = ?", userID).
		Count(&total)

	var questions []QuestionModel
	err := r.db.WithContext(ctx).
		Table("user_favorites AS uf").
		Select("q.*").
		Joins("LEFT JOIN questions q ON q.id = uf.question_id").
		Where("uf.user_id = ?", userID).
		Order("uf.id DESC").
		Offset(int((page - 1) * pageSize)).
		Limit(int(pageSize)).
		Scan(&questions).Error
	if err != nil {
		return nil, 0, err
	}

	items := make([]*biz.Question, len(questions))
	for i, q := range questions {
		items[i] = &biz.Question{
			ID:           uint64(q.ID),
			Title:        q.Title,
			Content:      q.Content,
			Difficulty:   q.Difficulty,
			Type:         q.Type,
			IndustryCode: q.IndustryCode,
		}
	}
	return items, total, nil
}

func (r *favoriteRepo) Exists(ctx context.Context, userID, questionID uint64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.UserFavorite{}).
		Where("user_id = ? AND question_id = ?", userID, questionID).
		Count(&count).Error
	return count > 0, err
}
