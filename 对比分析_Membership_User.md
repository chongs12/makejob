# Membership + User 服务 — 字段级差异分析

## Membership 服务

### 1. GetMembershipStatus（会员状态）

**单体端点**: `GET /api/membership/status` → `MembershipStatusResponse`
**微服务 RPC**: `MembershipService.GetMembershipStatus` → `MembershipStatus`

| 项目 | 单体返回 | 微服务返回 | 优先级 | 修复建议 |
|------|---------|-----------|--------|---------|
| `daily_practice_limit` | ✅ 每日练习限制 | ❌ 缺失 | P0 | 前端需要限制用户操作次数 |
| `daily_interview_limit` | ✅ 每日面试限制 | ❌ 缺失 | P0 | 同上 |
| `practice_used_today` | ✅ 今日已练习数 | ❌ 缺失 | P1 | 需查询当天操作记录 |
| `interview_used_today` | ✅ 今日已面试数 | ❌ 缺失 | P1 | 同上 |
| `expires_at` | ✅ 字段名 | ⚠️ proto 字段名 `expire_at`（无 s） | P2 | 确认前端读取的字段名 |

**说明**: `daily_practice_limit` 和 `daily_interview_limit` 是会员功能限制的核心字段，缺失导致前端无法展示剩余次数。

---

### 2. ListPlans（会员计划列表）

**单体端点**: `GET /api/membership/plans` → `[]MembershipPlanResponse`
**微服务 RPC**: `MembershipService.ListPlans` → `ListPlansResponse`

| 项目 | 单体返回 | 微服务返回 | 优先级 | 修复建议 |
|------|---------|-----------|--------|---------|
| `original_price` | ✅ 原价 | ❌ 缺失 | P1 | 前端需要展示折扣信息 |
| `is_popular` | ✅ 是否热门推荐 | ❌ 缺失 | P1 | 前端需要高亮推荐套餐 |

---

### 3. ListOrders（订单列表）

**单体端点**: `GET /api/membership/orders` → `PageResult{list: []OrderResponse}`
**微服务 RPC**: `MembershipService.ListOrders` → `ListOrdersResponse`

| 项目 | 单体返回 | 微服务返回 | 优先级 | 修复建议 |
|------|---------|-----------|--------|---------|
| 分页结构 | `{list, total, page, page_size}` | `{orders, total}` | P1 | 缺少 `page` 和 `page_size` 字段 |

---

## User 服务

### 4. GetProfile / Login / Register

**单体端点**: `GET /api/auth/me` / `POST /api/auth/login` → `UserProfile` / `LoginResponse`
**微服务 RPC**: `UserService.GetProfile` / `Login` → `UserProfile` / `AuthResponse`

| 项目 | 单体返回 | 微服务返回 | 优先级 | 修复建议 |
|------|---------|-----------|--------|---------|
| `membership_level` | ✅ 在 UserProfile 中 | ✅ 有 | — | 对齐 |
| `membership_expire_at` | ❌ 单体 UserProfile 无此字段 | ✅ proto 有 | — | 微服务更丰富 |
| `user` 嵌套在 AuthResponse | ✅ `{token, refresh_token, expires_at, user: {...}}` | ✅ 同结构 | — | 对齐 |

基本对齐，无重大差异。

---

### 5. AdminListUsers（管理员用户列表）

**单体端点**: `GET /api/admin/users` → `PageResult{list: []AdminUserListItem}`
**微服务 RPC**: `UserService.AdminListUsers` → `AdminListUsersResponse`

| 项目 | 单体返回 | 微服务返回 | 优先级 | 修复建议 |
|------|---------|-----------|--------|---------|
| `updated_at` | ✅ 有 | ❌ 缺失 | P2 | proto `AdminUserInfo` 无此字段 |

差异较小。
