# Tasks

## P2-4: Community Service - Implement UpdatePost
- [x] Task 1: 修改 biz/community.go - 添加 UpdatePost UseCase
  - [x] SubTask 1.1: 添加 PostRepo.Update 方法到接口
  - [x] SubTask 1.2: 实现 UpdatePost 方法（权限验证 + 内容校验 + summary 重算）
- [x] Task 2: 修改 data/community_repo.go - 实现 Update 方法
- [x] Task 3: 修改 service/community.go - 添加 UpdatePost handler

## P2-5: Community Service - Implement ToggleLike
- [x] Task 4: 修改 biz/community.go - 添加 PostLike 实体和 LikeRepo 接口
  - [x] SubTask 4.1: 定义 PostLike 实体
  - [x] SubTask 4.2: 定义 LikeRepo 接口
  - [x] SubTask 4.3: 添加 PostRepo.IncrementLikeCount 方法到接口
  - [x] SubTask 4.4: 实现 ToggleLike UseCase（事务）
- [x] Task 5: 修改 data/community_repo.go - 实现 LikeRepo 和事务
- [x] Task 6: 修改 service/community.go - 添加 ToggleLike handler

## P2-6: Community Service - Implement ListMyPosts + Enhance
- [x] Task 7: 修改 biz/community.go - 添加 ListMyPosts UseCase
  - [x] SubTask 7.1: 添加 PostRepo.ListByAuthorID 方法到接口
  - [x] SubTask 7.2: 实现 ListMyPosts 方法
- [x] Task 8: 修改 data/community_repo.go - 实现 ListByAuthorID
- [x] Task 9: 修改 service/community.go - 添加 ListMyPosts handler

# Task Dependencies
- Task 1-3 无依赖
- Task 4-6 无依赖
- Task 7-9 无依赖
