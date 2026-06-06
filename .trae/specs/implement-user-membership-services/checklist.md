# Checklist

## P2-1: User Service - Fix RefreshToken Bug
- [x] Login 返回 access_token 和 refresh_token
- [x] Register 返回 access_token 和 refresh_token
- [x] access_token 有效期 2 小时
- [x] refresh_token 有效期 7 天
- [x] RefreshToken 验证 refresh_token 后签发新 token 对
- [x] go build 编译通过

## P2-2: User Service - Add Logout RPC
- [x] conf/conf.go 包含 Redis 配置
- [x] biz/user.go 定义 TokenBlacklist 接口
- [x] data/redis_client.go 实现 TokenBlacklist
- [x] service/user.go 实现 Logout handler
- [x] Logout 后 token 被加入黑名单
- [x] go build 编译通过

## P2-3: Membership Service - Complete Implementation
- [x] biz/membership.go 定义 MembershipOrder 实体
- [x] biz/membership.go 定义 UserMembership 实体
- [x] biz/membership.go 定义 OrderRepo/MembershipRepo 接口
- [x] biz/membership.go 实现 CreateOrder UseCase
- [x] biz/membership.go 实现 HandlePaymentCallback UseCase
- [x] biz/membership.go 实现 CheckFeatureAccess UseCase
- [x] data/membership_repo.go 实现 GORM 实现
- [x] service/membership.go 实现所有 handler
- [x] go build 编译通过

## 通用
- [x] go vet 通过
