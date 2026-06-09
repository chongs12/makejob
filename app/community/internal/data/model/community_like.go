package model

import "time"

// CommunityLike 社区帖子点赞 GORM model。
// 点赞关系承担“开关表”语义，取消点赞直接硬删除，不保留软删除字段。
type CommunityLike struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement"`
	PostID    uint64    `gorm:"uniqueIndex:idx_post_user;not null"` // S2: 唯一约束
	UserID    uint64    `gorm:"uniqueIndex:idx_post_user;not null"` // S2: 唯一约束
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

// TableName 返回社区点赞表名。
func (CommunityLike) TableName() string { return "community_likes" }
