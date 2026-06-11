# Community 服务 — 字段级差异分析

## 1. ListPosts（帖子列表）

**单体端点**: `GET /api/community/posts` → `PageResult{list: []CommunityPostItem}`
**微服务 RPC**: `CommunityService.ListPosts` → `ListPostsResponse`

| 项目 | 单体返回 | 微服务返回 | 优先级 | 修复建议 |
|------|---------|-----------|--------|---------|
| `is_pinned` | ✅ 是否置顶 | ❌ 缺失 | P1 | 需在帖子表增加 `is_pinned` 字段 |
| `is_recommended` | ✅ 是否推荐 | ❌ 缺失 | P1 | 需在帖子表增加 `is_recommended` 字段 |
| `author.role` | ✅ 作者角色 | ❌ 缺失（`PostSummary` 无 `author_role`） | P1 | 需关联 user 表获取角色 |
| `author` 结构 | ✅ 嵌套对象 `{id, username, avatar, role}` | ⚠️ 扁平字段 `author_id, author_name, author_avatar` | P1 | Gateway `normalizeCommunityPostListPayload` 已构造嵌套 `author` 对象 |
| `tags` 格式 | ✅ `[]string` 数组 | ⚠️ CSV 字符串 `"tag1,tag2"` | P1 | Gateway `normalizeCommunityPostListPayload` 已拆分为数组 |
| `is_liked` | ✅ 当前用户是否点赞 | ❌ 缺失（proto 有字段但未填充） | P0 | 需查询 `community_interactions` 表 |
| `is_author` | ✅ 当前用户是否为作者 | ❌ 缺失（proto 有字段但未填充） | P0 | 需比较 `author_id` 与当前用户 ID |

**说明**: Gateway 的 normalizer 已处理了 `tags` 拆分和 `author` 嵌套，但 `is_liked` 和 `is_author` 需要在 service 层计算。

---

## 2. GetPost（帖子详情）

**单体端点**: `GET /api/community/posts/:id` → `CommunityPostItem`
**微服务 RPC**: `CommunityService.GetPost` → `PostDetail`

| 项目 | 单体返回 | 微服务返回 | 优先级 | 修复建议 |
|------|---------|-----------|--------|---------|
| `is_pinned` | ✅ 有 | ❌ 缺失 | P1 | 同列表 |
| `is_recommended` | ✅ 有 | ❌ 缺失 | P1 | 同列表 |
| `author.role` | ✅ 有 | ❌ 缺失 | P1 | 需关联 user 表 |
| `is_liked` | ✅ 有 | ❌ 缺失（proto 有但未填充） | P0 | 需查询 `community_interactions` |
| `is_author` | ✅ 有 | ❌ 缺失（proto 有但未填充） | P0 | 需比较 author_id 与当前用户 |

---

## 3. ListComments（评论列表）

**单体端点**: `GET /api/community/posts/:id/comments` → `[]CommunityCommentItem`
**微服务 RPC**: `CommunityService.ListComments` → `ListCommentsResponse`

| 项目 | 单体返回 | 微服务返回 | 优先级 | 修复建议 |
|------|---------|-----------|--------|---------|
| `author.role` | ✅ 有 | ❌ 缺失 | P1 | proto `Comment` 无 `author_role` |
| `author` 结构 | ✅ 嵌套对象 | ⚠️ 扁平 `author_id, author_name, author_avatar` | P1 | Gateway `normalizeCommunityCommentListPayload` 已构造嵌套对象 |
| `is_author` | ✅ 有 | ❌ 缺失（proto 有但未填充） | P0 | 需比较 author_id 与当前用户 |

---

## 4. CreatePost（创建帖子）

**单体端点**: `POST /api/community/posts` → `CommunityPostItem`（完整帖子）
**微服务 RPC**: `CommunityService.CreatePost` → `Post`（仅 id, title, created_at）

| 项目 | 单体返回 | 微服务返回 | 优先级 | 修复建议 |
|------|---------|-----------|--------|---------|
| 返回内容 | 完整帖子对象 | 仅 `{id, title, created_at}` | P1 | 前端创建后可能需要立即展示完整帖子 |

---

## 5. UpdatePost（更新帖子）

**单体端点**: `PUT /api/community/posts/:id` → `CommunityPostItem`
**微服务 RPC**: `CommunityService.UpdatePost` → `Post`（仅 id, title, created_at）

| 项目 | 单体返回 | 微服务返回 | 优先级 | 修复建议 |
|------|---------|-----------|--------|---------|
| 返回内容 | 完整帖子对象 | 仅 `{id, title, created_at}` | P1 | 同 CreatePost |

---

## 6. DeletePost（删除帖子）

**单体端点**: `DELETE /api/community/posts/:id` → `{id: uint}`
**微服务 RPC**: `CommunityService.DeletePost` → `Empty`

| 项目 | 单体返回 | 微服务返回 | 优先级 | 修复建议 |
|------|---------|-----------|--------|---------|
| 返回值 | `{id}` | 空 | P2 | 前端可用于确认删除哪个帖子 |

---

## 7. CreateComment（创建评论）

**单体端点**: `POST /api/community/posts/:id/comments` → `CommunityCommentItem`
**微服务 RPC**: `CommunityService.CreateComment` → `Comment`

| 项目 | 单体返回 | 微服务返回 | 优先级 | 修复建议 |
|------|---------|-----------|--------|---------|
| `author` 结构 | ✅ 嵌套对象 | ⚠️ 扁平字段 | P1 | 需前端自行构造或 Gateway 处理 |
| `author_id` 来源 | 从 auth context | ⚠️ 从 request body（安全问题） | P1 | 需改为从 auth context 获取 |
