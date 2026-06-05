# Tasks

- [x] Task 1: 修改配置结构 conf.go - 添加 Piston 配置字段
  - [x] SubTask 1.1: 添加 Piston 结构体（Endpoint, TimeoutMs）
  - [x] SubTask 1.2: 更新 Bootstrap 结构体包含 Piston 字段
  - [x] SubTask 1.3: 设置默认值（Endpoint=http://localhost:2000, TimeoutMs=10000）

- [x] Task 2: 创建领域错误定义 biz/errors.go
  - [x] SubTask 2.1: 定义 ErrUnsupportedLanguage (BadRequest)
  - [x] SubTask 2.2: 定义 ErrExecutionTimeout (GatewayTimeout)
  - [x] SubTask 2.3: 定义 ErrPistonUnavailable (ServiceUnavailable)

- [x] Task 3: 重写领域层 biz/coderunner.go - 定义接口和 UseCase
  - [x] SubTask 3.1: 定义 PistonExecutor 接口
  - [x] SubTask 3.2: 定义 ExecuteInput/Output 结构体
  - [x] SubTask 3.3: 实现 CodeRunnerUseCase 的 Execute 方法

- [x] Task 4: 创建 Piston 客户端 data/piston_client.go
  - [x] SubTask 4.1: 实现 pistonClient 结构体
  - [x] SubTask 4.2: 实现语言版本映射
  - [x] SubTask 4.3: 实现 Execute 方法调用 Piston API
  - [x] SubTask 4.4: 实现错误处理（超时、不可用）

- [x] Task 5: 更新 data 层 data.go - 移除数据库依赖
  - [x] SubTask 5.1: 简化 Data 结构体（CodeRunner 无数据库）
  - [x] SubTask 5.2: 更新 NewData 函数

- [x] Task 6: 重写 service 层 service/coderunner.go
  - [x] SubTask 6.1: 实现 Execute 方法
  - [x] SubTask 6.2: 实现 ListLanguages 方法
  - [x] SubTask 6.3: 添加 proto 转换函数

- [x] Task 7: 更新 server 层 server/grpc.go
  - [x] SubTask 7.1: 移除 JWT 认证（内部服务）
  - [x] SubTask 7.2: 保留 Recovery + Logging 中间件

- [x] Task 8: 更新 main.go 启动入口
  - [x] SubTask 8.1: 更新 wireApp 函数
  - [x] SubTask 8.2: 创建 PistonClient
  - [x] SubTask 8.3: 移除数据库初始化

- [x] Task 9: 更新配置文件 configs/config.yaml
  - [x] SubTask 9.1: 添加 piston 配置段

- [ ] Task 10: 编写单元测试
  - [ ] SubTask 10.1: biz/coderunner_test.go - UseCase 测试
  - [ ] SubTask 10.2: data/piston_client_test.go - 客户端测试

# Task Dependencies
- Task 1 无依赖
- Task 2 无依赖
- Task 3 依赖 Task 1, 2
- Task 4 依赖 Task 1, 3
- Task 5 依赖 Task 1
- Task 6 依赖 Task 3, 4
- Task 7 依赖 Task 1
- Task 8 依赖 Task 3, 4, 5, 6, 7
- Task 9 依赖 Task 1
- Task 10 依赖 Task 3, 4
