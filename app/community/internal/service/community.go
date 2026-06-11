package service

import (
	"context"

	kratosErr "github.com/go-kratos/kratos/v2/errors"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	communityv1 "makejob/api/makejob/community/v1"
	sharedv1 "makejob/api/makejob/shared/v1"
	"makejob/app/community/internal/biz"
	"makejob/pkg/auth"
)

var ErrUnauthorized = kratosErr.Unauthorized("UNAUTHORIZED", "未授权")

// CommunityService 实现 gRPC CommunityServiceServer
type CommunityService struct {
	communityv1.UnimplementedCommunityServiceServer
	uc *biz.CommunityUseCase
}

// NewCommunityService 创建社区服务
func NewCommunityService(uc *biz.CommunityUseCase) *CommunityService {
	return &CommunityService{uc: uc}
}

func (s *CommunityService) ListPosts(ctx context.Context, req *communityv1.ListPostsRequest) (*communityv1.ListPostsResponse, error) {
	var page, pageSize int32 = 1, 20
	if req.Page != nil {
		page = req.Page.Page
		pageSize = req.Page.PageSize
	}
	// FIX B5: 传递过滤条件
	filter := biz.PostFilter{
		PostType: req.PostType,
		Keyword:  req.Keyword,
		Tag:      req.Tag,
		SortBy:   req.SortBy,
	}
	posts, total, err := s.uc.ListPosts(ctx, page, pageSize, filter)
	if err != nil {
		return nil, err
	}
	items := make([]*communityv1.PostSummary, len(posts))
	for i, p := range posts {
		items[i] = toProtoPostSummary(p)
	}
	return &communityv1.ListPostsResponse{
		Posts: items,
		PageResult: &sharedv1.PageResult{
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		},
	}, nil
}

// toProtoPostSummary 将帖子领域实体转换为列表摘要，补齐前端列表卡片所需字段。
func toProtoPostSummary(p *biz.Post) *communityv1.PostSummary {
	return &communityv1.PostSummary{
		Id:           uint64(p.ID),
		Title:        p.Title,
		Category:     p.Category,
		AuthorId:     p.AuthorID,
		AuthorName:   p.AuthorName,
		LikeCount:    p.LikeCount,
		CommentCount: p.CommentCount,
		CreatedAt:    timestamppb.New(p.CreatedAt),
		PostType:     p.PostType,
		Summary:      p.Summary,
		Tags:         p.Tags,
		ViewCount:    p.ViewCount,
		UpdatedAt:    timestamppb.New(p.UpdatedAt),
	}
}

func (s *CommunityService) CreatePost(ctx context.Context, req *communityv1.CreatePostRequest) (*communityv1.Post, error) {
	// FIX C3: 从认证上下文获取用户 ID，禁止从请求体获取身份
	authorID := auth.GetUserIDFromContext(ctx)
	if authorID == 0 {
		return nil, ErrUnauthorized
	}
	post, err := s.uc.CreatePost(ctx, authorID, req.Title, req.Content, req.PostType, req.Category, req.Tags)
	if err != nil {
		return nil, err
	}
	return &communityv1.Post{
		Id:        uint64(post.ID),
		Title:     post.Title,
		CreatedAt: timestamppb.New(post.CreatedAt),
	}, nil
}

func (s *CommunityService) GetPost(ctx context.Context, req *communityv1.GetPostRequest) (*communityv1.PostDetail, error) {
	post, err := s.uc.GetPost(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &communityv1.PostDetail{
		Id:           uint64(post.ID),
		Title:        post.Title,
		Content:      post.Content,
		Category:     post.Category,
		AuthorId:     post.AuthorID,
		AuthorName:   post.AuthorName,
		LikeCount:    post.LikeCount,
		CommentCount: post.CommentCount,
		ViewCount:    post.ViewCount,
		CreatedAt:    timestamppb.New(post.CreatedAt),
		PostType:     post.PostType,
		Summary:      post.Summary,
		Tags:         post.Tags,
		UpdatedAt:    timestamppb.New(post.UpdatedAt),
	}, nil
}

func (s *CommunityService) DeletePost(ctx context.Context, req *communityv1.DeletePostRequest) (*emptypb.Empty, error) {
	// FIX C2: 从认证上下文获取用户 ID，禁止从请求体获取身份
	authorID := auth.GetUserIDFromContext(ctx)
	if authorID == 0 {
		return nil, ErrUnauthorized
	}
	if err := s.uc.DeletePost(ctx, req.Id, authorID); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *CommunityService) ListComments(ctx context.Context, req *communityv1.ListCommentsRequest) (*communityv1.ListCommentsResponse, error) {
	var page, pageSize int32 = 1, 20
	if req.Page != nil {
		page = req.Page.Page
		pageSize = req.Page.PageSize
	}
	comments, total, err := s.uc.ListComments(ctx, req.PostId, page, pageSize)
	if err != nil {
		return nil, err
	}
	items := make([]*communityv1.Comment, len(comments))
	for i, c := range comments {
		items[i] = &communityv1.Comment{
			Id:        c.ID,
			PostId:    c.PostID,
			AuthorId:  c.AuthorID,
			Content:   c.Content,
			CreatedAt: timestamppb.New(c.CreatedAt),
		}
	}
	return &communityv1.ListCommentsResponse{
		Comments: items,
		PageResult: &sharedv1.PageResult{
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		},
	}, nil
}

func (s *CommunityService) CreateComment(ctx context.Context, req *communityv1.CreateCommentRequest) (*communityv1.Comment, error) {
	comment, err := s.uc.CreateComment(ctx, req.PostId, req.AuthorId, req.Content)
	if err != nil {
		return nil, err
	}
	return &communityv1.Comment{
		Id:        comment.ID,
		PostId:    comment.PostID,
		AuthorId:  comment.AuthorID,
		Content:   comment.Content,
		CreatedAt: timestamppb.New(comment.CreatedAt),
	}, nil
}

// UpdatePost 更新帖子
func (s *CommunityService) UpdatePost(ctx context.Context, req *communityv1.UpdatePostRequest) (*communityv1.Post, error) {
	post, err := s.uc.UpdatePost(ctx, req.Id, req.Title, req.Content, req.Tags)
	if err != nil {
		return nil, err
	}
	return &communityv1.Post{
		Id:        uint64(post.ID),
		Title:     post.Title,
		CreatedAt: timestamppb.New(post.CreatedAt),
	}, nil
}

// ToggleLike 切换帖子点赞状态
func (s *CommunityService) ToggleLike(ctx context.Context, req *communityv1.ToggleLikeRequest) (*communityv1.LikeResponse, error) {
	liked, likeCount, err := s.uc.ToggleLike(ctx, req.PostId)
	if err != nil {
		return nil, err
	}
	return &communityv1.LikeResponse{
		Liked:     liked,
		LikeCount: likeCount,
	}, nil
}

// ListMyPosts 获取当前用户的帖子列表
func (s *CommunityService) ListMyPosts(ctx context.Context, req *communityv1.ListMyPostsRequest) (*communityv1.ListMyPostsResponse, error) {
	var page, pageSize int32 = 1, 20
	if req.Page != nil {
		page = req.Page.Page
		pageSize = req.Page.PageSize
	}
	posts, total, err := s.uc.ListMyPosts(ctx, page, pageSize)
	if err != nil {
		return nil, err
	}
	items := make([]*communityv1.PostSummary, len(posts))
	for i, p := range posts {
		items[i] = toProtoPostSummary(p)
	}
	return &communityv1.ListMyPostsResponse{
		Posts: items,
		PageResult: &sharedv1.PageResult{
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		},
	}, nil
}
