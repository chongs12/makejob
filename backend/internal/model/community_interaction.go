package model

// CommunityComment 表示社区帖子下的一级评论。
type CommunityComment struct {
	BaseModel
	PostID   uint          `json:"post_id" gorm:"not null;index"`
	Post     CommunityPost `json:"post" gorm:"foreignKey:PostID"`
	AuthorID uint          `json:"author_id" gorm:"not null;index"`
	Author   User          `json:"author" gorm:"foreignKey:AuthorID"`
	Content  string        `json:"content" gorm:"type:text;not null"`
}

// TableName 指定社区评论表名。
func (CommunityComment) TableName() string {
	return "community_comments"
}

// CommunityPostLike 表示用户对社区帖子的点赞记录。
type CommunityPostLike struct {
	BaseModel
	PostID uint `json:"post_id" gorm:"not null;uniqueIndex:idx_community_post_likes_post_user"`
	UserID uint `json:"user_id" gorm:"not null;uniqueIndex:idx_community_post_likes_post_user"`
}

// TableName 指定社区点赞表名。
func (CommunityPostLike) TableName() string {
	return "community_post_likes"
}
