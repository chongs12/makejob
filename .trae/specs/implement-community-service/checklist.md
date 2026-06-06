# Checklist

## P2-4: UpdatePost RPC
- [x] biz/community.go 定义 PostRepo.Update 接口
- [x] biz/community.go 实现 UpdatePost UseCase
- [x] UpdatePost 验证帖子存在
- [x] UpdatePost 验证作者权限
- [x] UpdatePost 校验 title/content/tags
- [x] UpdatePost 重新计算 summary
- [x] data/community_repo.go 实现 Update 方法
- [x] service/community.go 实现 UpdatePost handler
- [x] go build 编译通过

## P2-5: ToggleLike RPC
- [x] biz/community.go 定义 PostLike 实体
- [x] biz/community.go 定义 LikeRepo 接口
- [x] biz/community.go 定义 PostRepo.IncrementLikeCount 接口
- [x] biz/community.go 实现 ToggleLike UseCase
- [x] ToggleLike 使用事务保证原子性
- [x] data/community_repo.go 实现 LikeRepo
- [x] data/community_repo.go 实现 IncrementLikeCount
- [x] service/community.go 实现 ToggleLike handler
- [x] go build 编译通过

## P2-6: ListMyPosts RPC
- [x] biz/community.go 定义 PostRepo.ListByAuthorID 接口
- [x] biz/community.go 实现 ListMyPosts UseCase
- [x] data/community_repo.go 实现 ListByAuthorID
- [x] service/community.go 实现 ListMyPosts handler
- [x] go build 编译通过

## 通用
- [x] go vet 通过
