package model

import "time"

// CommunityLike 社区帖子点赞 GORM model（对齐单体 community_post_likes 表）。
// 点赞关系承担"开关表"语义，取消点赞直接硬删除，不保留软删除字段。
type CommunityLike struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement"`
	PostID    uint64    `gorm:"uniqueIndex:idx_community_post_likes_post_user;not null"`
	UserID    uint64    `gorm:"uniqueIndex:idx_community_post_likes_post_user;not null"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

// TableName 返回社区点赞表名（对齐单体 community_post_likes）。
func (CommunityLike) TableName() string { return "community_post_likes" }
