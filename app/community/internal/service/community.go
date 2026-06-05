package service

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	communityv1 "makejob/api/makejob/community/v1"
	sharedv1 "makejob/api/makejob/shared/v1"
	"makejob/app/community/internal/biz"
)

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
	posts, total, err := s.uc.ListPosts(ctx, page, pageSize)
	if err != nil {
		return nil, err
	}
	items := make([]*communityv1.PostSummary, len(posts))
	for i, p := range posts {
		items[i] = &communityv1.PostSummary{
			Id:         p.ID,
			Title:      p.Title,
			Category:   p.Category,
			AuthorId:   p.AuthorID,
			AuthorName: p.AuthorName,
			CreatedAt:  timestamppb.New(p.CreatedAt),
		}
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

func (s *CommunityService) CreatePost(ctx context.Context, req *communityv1.CreatePostRequest) (*communityv1.Post, error) {
	post, err := s.uc.CreatePost(ctx, req.AuthorId, req.Title, req.Content, req.Category)
	if err != nil {
		return nil, err
	}
	return &communityv1.Post{
		Id:        post.ID,
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
		Id:         post.ID,
		Title:      post.Title,
		Content:    post.Content,
		Category:   post.Category,
		AuthorId:   post.AuthorID,
		AuthorName: post.AuthorName,
		CreatedAt:  timestamppb.New(post.CreatedAt),
	}, nil
}

func (s *CommunityService) DeletePost(ctx context.Context, req *communityv1.DeletePostRequest) (*emptypb.Empty, error) {
	if err := s.uc.DeletePost(ctx, req.Id, req.AuthorId); err != nil {
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
