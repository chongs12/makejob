package model

import "time"

// CommunityLike 社区帖子点赞 GORM model
type CommunityLike struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement"`
	PostID    uint64    `gorm:"uniqueIndex:idx_post_user;not null"` // S2: 唯一约束
	UserID    uint64    `gorm:"uniqueIndex:idx_post_user;not null"` // S2: 唯一约束
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

func (CommunityLike) TableName() string { return "community_likes" }
