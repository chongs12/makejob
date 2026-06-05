package model

import "gorm.io/gorm"

// CommunityComment 社区评论 GORM model
type CommunityComment struct {
	gorm.Model
	PostID   uint64 `gorm:"index;not null"`
	AuthorID uint64 `gorm:"index;not null"`
	Content  string `gorm:"type:text;not null"`
}

func (CommunityComment) TableName() string { return "community_comments" }
