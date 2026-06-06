# User + Membership 服务实现 Spec (P2-1~P2-3)

## Why
User 服务存在 RefreshToken Bug（Login/Register 不返回 refresh_token，RefreshToken 逻辑错误），需要修复。Membership 服务只有空壳骨架，需要完整实现 8 个 RPC。

## What Changes
- P2-1: 修复 User 服务 RefreshToken Bug
  - Login/Register 返回双 token（access + refresh）
  - RefreshToken 正确验证后签发新 token 对
- P2-2: 添加 User 服务 Logout RPC
  - Redis Token 黑名单机制
  - 登出后 token 立即失效
- P2-3: 完整实现 Membership 服务
  - 订单创建、支付回调、订单查询
  - 功能权限检查
  - 会员信息查询

## Impact
- Affected specs: P2-1, P2-2, P2-3
- Affected code:
  - `app/user/internal/service/user.go` (修改)
  - `app/user/internal/biz/user.go` (修改)
  - `app/user/internal/data/redis_client.go` (新建)
  - `app/membership/internal/biz/membership.go` (重写)
  - `app/membership/internal/data/membership_repo.go` (重写)
  - `app/membership/internal/service/membership.go` (重写)

## ADDED Requirements

### Requirement: RefreshToken 修复
Login/Register SHALL 返回 access_token 和 refresh_token。

#### Scenario: 登录返回双 token
- **WHEN** 调用 Login(email, password)
- **THEN** 返回 access_token（2小时）和 refresh_token（7天）

#### Scenario: 刷新 token
- **WHEN** 调用 RefreshToken(refresh_token)
- **THEN** 返回新的 access_token 和 refresh_token

### Requirement: Logout RPC
系统 SHALL 支持用户登出，通过 Redis 黑名单使 token 立即失效。

#### Scenario: 登出后 token 失效
- **WHEN** 调用 Logout(access_token, refresh_token)
- **THEN** 后续使用这些 token 的请求被拒绝

### Requirement: Membership 完整实现
系统 SHALL 提供完整的会员服务能力。

#### Scenario: 创建订单
- **WHEN** 调用 CreateOrder(user_id, plan_id)
- **THEN** 返回 order_no 和 amount

#### Scenario: 支付回调
- **WHEN** 调用 HandlePaymentCallback(order_no, status="success")
- **THEN** 订单状态变为 paid，会员等级更新

#### Scenario: 功能权限检查
- **WHEN** 调用 CheckFeatureAccess(user_id, feature="daily_practice")
- **THEN** free 用户返回限制，pro 用户返回无限制

## 全局规范遵循
- 错误处理：使用 kratos errors 包
- 构造函数：NewXxx(deps...) 模式
- 禁止全局变量和 init() 函数
- 使用 context 传播
- 使用中文注释
