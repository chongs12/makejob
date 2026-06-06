# Tasks

## P2-1: User Service - Fix RefreshToken Bug
- [x] Task 1: 修改 service/user.go - 修复 Login/Register 返回双 token
  - [x] SubTask 1.1: Login 方法生成 access_token（2h）和 refresh_token（7d）
  - [x] SubTask 1.2: Register 方法生成 access_token（2h）和 refresh_token（7d）
- [x] Task 2: 修改 service/user.go - 修复 RefreshToken 方法
  - [x] SubTask 2.1: 验证 refresh_token
  - [x] SubTask 2.2: 生成新的 access_token 和 refresh_token

## P2-2: User Service - Add Logout RPC
- [x] Task 3: 修改 conf/conf.go - 添加 Redis 配置
- [x] Task 4: 修改 biz/user.go - 添加 TokenBlacklist 接口
- [x] Task 5: 新建 data/redis_client.go - 实现 TokenBlacklist
- [x] Task 6: 修改 service/user.go - 添加 Logout handler

## P2-3: Membership Service - Complete Implementation
- [x] Task 7: 重写 biz/membership.go - 实体和接口
  - [x] SubTask 7.1: 定义 MembershipOrder 实体
  - [x] SubTask 7.2: 定义 UserMembership 实体
  - [x] SubTask 7.3: 定义 OrderRepo 接口
  - [x] SubTask 7.4: 定义 MembershipRepo 接口
  - [x] SubTask 7.5: 实现 CreateOrder UseCase
  - [x] SubTask 7.6: 实现 HandlePaymentCallback UseCase
  - [x] SubTask 7.7: 实现 CheckFeatureAccess UseCase
- [x] Task 8: 重写 data/membership_repo.go - GORM 实现
- [x] Task 9: 重写 service/membership.go - gRPC handler
- [x] Task 10: 更新 configs/config.yaml

# Task Dependencies
- Task 1-2 无依赖
- Task 3 无依赖
- Task 4 依赖 Task 3
- Task 5 依赖 Task 4
- Task 6 依赖 Task 4, 5
- Task 7 无依赖
- Task 8 依赖 Task 7
- Task 9 依赖 Task 7, 8
- Task 10 无依赖
