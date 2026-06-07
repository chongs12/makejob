package model

// CommunityComment 社区评论 GORM model（FIX C1: 使用 BaseModel 替代 gorm.Model）
type CommunityComment struct {
	BaseModel
	PostID   uint64 `gorm:"index;not null"`
	AuthorID uint64 `gorm:"index;not null"`
	Content  string `gorm:"type:text;not null"`
}

func (CommunityComment) TableName() string { return "community_comments" }
