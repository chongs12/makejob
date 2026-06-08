# Checklist

## P6-1: Admin Service - Refactor to Delegate to Domain Services

### 配置
- [x] conf/conf.go 添加下游服务地址配置

### biz 层
- [x] biz/admin.go 添加 UserClient 接口
- [x] biz/admin.go 添加 QuestionClient 接口
- [x] biz/admin.go 添加 AIGatewayClient 接口
- [x] biz/admin.go 重构 ListUsers（委托 User Service）
- [x] biz/admin.go 重构 UpdateUserRole（委托 User Service）
- [x] biz/admin.go 重构 DisableUser（委托 User Service）
- [x] biz/admin.go 重构 AdminListQuestions（委托 Question Service）
- [x] biz/admin.go 重构 CreateQuestion（委托 Question Service）
- [x] biz/admin.go 重构 UpdateQuestion（委托 Question Service）
- [x] biz/admin.go 重构 DeleteQuestion（委托 Question Service）
- [x] biz/admin.go 重构 GetDashboard（并发聚合）

### data 层
- [x] data/clients.go 实现 UserClient
- [x] data/clients.go 实现 QuestionClient
- [x] data/clients.go 实现 AIGatewayClient

### service 层
- [x] service/admin.go 移除 bridge 依赖

### 启动入口
- [x] main.go 注入新依赖
- [x] go build 编译通过
- [x] go vet 通过
