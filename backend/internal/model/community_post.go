package model

const (
	CommunityPostTypeArticle = "article"
	CommunityPostTypeMoment  = "moment"
)

// CommunityPost represents a forum-style article or short status post.
type CommunityPost struct {
	BaseModel
	AuthorID      uint   `json:"author_id" gorm:"not null;index"`
	Author        User   `json:"author" gorm:"foreignKey:AuthorID"`
	PostType      string `json:"post_type" gorm:"size:20;not null;index"`
	Title         string `json:"title" gorm:"size:200"`
	Content       string `json:"content" gorm:"type:text;not null"`
	Summary       string `json:"summary" gorm:"size:300"`
	Tags          string `json:"tags" gorm:"size:300"`
	ViewCount     int    `json:"view_count" gorm:"not null;default:0"`
	CommentCount  int    `json:"comment_count" gorm:"not null;default:0"`
	LikeCount     int    `json:"like_count" gorm:"not null;default:0"`
	IsPinned      bool   `json:"is_pinned" gorm:"not null;default:false"`
	IsRecommended bool   `json:"is_recommended" gorm:"not null;default:false"`
}

func (CommunityPost) TableName() string {
	return "community_posts"
}
