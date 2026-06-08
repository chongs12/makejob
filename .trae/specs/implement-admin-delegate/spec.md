# Admin Service - Refactor to Delegate to Domain Services Spec (P6-1)

## Why
Admin 服务当前直接操作其他服务的数据库表，违反了微服务架构原则。需要重构为通过 gRPC 调用各领域服务来完成管理操作。

## What Changes
- P6-1: 重构 Admin 服务，将直接数据库操作替换为 gRPC 调用各领域服务
  - 用户管理 → User Service
  - 题目管理 → Question Service
  - AI 配置管理 → AI Gateway
  - 数据统计 → 并发调用各服务统计接口

## Impact
- Affected specs: P6-1
- Affected code:
  - `app/admin/internal/biz/admin.go` (重写)
  - `app/admin/internal/data/admin_repo.go` (重写)
  - `app/admin/internal/service/admin.go` (修改)
  - `app/admin/internal/conf/conf.go` (修改)

## ADDED Requirements

### Requirement: 用户管理委托
系统 SHALL 通过 User Service 管理用户。

#### Scenario: ListUsers
- **WHEN** 调用 ListUsers
- **THEN** 通过 User.ListUsers 获取用户列表

#### Scenario: UpdateUserRole
- **WHEN** 调用 UpdateUserRole
- **THEN** 通过 User.UpdateUserRole 更新用户角色

#### Scenario: DisableUser
- **WHEN** 调用 DisableUser
- **THEN** 通过 User.BanUser 禁用用户

### Requirement: 题目管理委托
系统 SHALL 通过 Question Service 管理题目。

### Requirement: 数据统计聚合
系统 SHALL 通过并发 gRPC 调用聚合各服务统计数据。

## 全局规范遵循
- 错误处理：使用 kratos errors 包
- 构造函数：NewXxx(deps...) 模式
- 禁止全局变量和 init() 函数
- 使用 context 传播
- 使用中文注释
- Admin 不再直接访问其他服务的数据库
