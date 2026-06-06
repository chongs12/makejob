# Community 服务增强实现 Spec (P2-4~P2-6)

## Why
Community 服务当前只有基础的帖子 CRUD 和评论功能，缺少 UpdatePost、ToggleLike、ListMyPosts 等功能，需要补齐。

## What Changes
- P2-4: 实现 UpdatePost RPC（帖子更新，仅作者可修改）
- P2-5: 实现 ToggleLike RPC（点赞/取消点赞，事务保证原子性）
- P2-6: 实现 ListMyPosts RPC + 增强现有接口

## Impact
- Affected specs: P2-4, P2-5, P2-6
- Affected code:
  - `app/community/internal/biz/community.go` (修改)
  - `app/community/internal/data/community_repo.go` (修改)
  - `app/community/internal/service/community.go` (修改)

## ADDED Requirements

### Requirement: UpdatePost RPC
系统 SHALL 支持作者修改帖子标题、内容和标签。

#### Scenario: 作者更新帖子
- **WHEN** 作者调用 UpdatePost(post_id, title, content, tags)
- **THEN** 帖子被更新，summary 被重新计算

#### Scenario: 非作者更新帖子
- **WHEN** 非作者调用 UpdatePost(post_id, ...)
- **THEN** 返回 FORBIDDEN 错误

### Requirement: ToggleLike RPC
系统 SHALL 支持点赞/取消点赞切换模式。

#### Scenario: 首次点赞
- **WHEN** 用户首次调用 ToggleLike(post_id)
- **THEN** 返回 liked=true, like_count+1

#### Scenario: 取消点赞
- **WHEN** 用户再次调用 ToggleLike(post_id)
- **THEN** 返回 liked=false, like_count-1

### Requirement: ListMyPosts RPC
系统 SHALL 支持查询当前用户发布的帖子。

#### Scenario: 查询我的帖子
- **WHEN** 调用 ListMyPosts(page, page_size)
- **THEN** 返回当前用户的帖子列表

## 全局规范遵循
- 错误处理：使用 kratos errors 包
- 构造函数：NewXxx(deps...) 模式
- 禁止全局变量和 init() 函数
- 使用 context 传播
- 使用中文注释
- 使用事务保证原子性
