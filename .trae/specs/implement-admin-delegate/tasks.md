# Tasks

## P6-1: Admin Service - Refactor to Delegate to Domain Services

- [x] Task 1: 修改 conf/conf.go - 添加下游服务地址配置
- [x] Task 2: 修改 biz/admin.go - 添加下游服务客户端接口
  - [x] SubTask 2.1: 添加 UserClient 接口
  - [x] SubTask 2.2: 添加 QuestionClient 接口
  - [x] SubTask 2.3: 添加 AIGatewayClient 接口
- [x] Task 3: 新建 data/clients.go - 实现下游服务客户端
- [x] Task 4: 修改 biz/admin.go - 重构用户管理 UseCase
  - [x] SubTask 4.1: ListUsers 委托给 User Service
  - [x] SubTask 4.2: UpdateUserRole 委托给 User Service
  - [x] SubTask 4.3: DisableUser 委托给 User Service
- [x] Task 5: 修改 biz/admin.go - 重构题目管理 UseCase
  - [x] SubTask 5.1: AdminListQuestions 委托给 Question Service
  - [x] SubTask 5.2: CreateQuestion 委托给 Question Service
  - [x] SubTask 5.3: UpdateQuestion 委托给 Question Service
  - [x] SubTask 5.4: DeleteQuestion 委托给 Question Service
- [x] Task 6: 修改 biz/admin.go - 重构 GetDashboard UseCase（并发聚合）
- [x] Task 7: 修改 service/admin.go - 移除 bridge 依赖
- [x] Task 8: 修改 main.go - 注入新依赖

# Task Dependencies
- Task 1 无依赖
- Task 2 依赖 Task 1
- Task 3 依赖 Task 2
- Task 4-6 依赖 Task 3
- Task 7 依赖 Task 4-6
- Task 8 依赖 Task 3-7
