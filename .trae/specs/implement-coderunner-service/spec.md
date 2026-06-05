# CodeRunner 服务完整实现 Spec

## Why
CodeRunner 是微服务架构中的基础设施服务，负责沙箱代码执行能力。当前只有空壳骨架，需要完整实现 Execute 和 ListLanguages 两个 RPC，封装 Piston API，供 Question 和 Interview 服务调用。

## What Changes
- 实现 PistonExecutor 接口和 HTTP 客户端调用 Piston API
- 实现 CodeRunnerUseCase 业务逻辑（单次执行 + 批量测试用例）
- 实现 gRPC handler 的 Execute 和 ListLanguages 方法
- 添加配置结构支持 Piston 端点和超时设置
- 添加领域错误定义（UNSUPPORTED_LANGUAGE, EXECUTION_TIMEOUT, PISTON_UNAVAILABLE）
- 更新 main.go 的依赖装配逻辑
- 编写单元测试验证核心逻辑

## Impact
- Affected specs: P1-1 CodeRunner Service
- Affected code: 
  - `app/coderunner/internal/conf/conf.go` (修改)
  - `app/coderunner/internal/biz/coderunner.go` (重写)
  - `app/coderunner/internal/biz/errors.go` (新建)
  - `app/coderunner/internal/data/data.go` (修改)
  - `app/coderunner/internal/data/piston_client.go` (新建)
  - `app/coderunner/internal/service/coderunner.go` (重写)
  - `app/coderunner/internal/server/grpc.go` (修改)
  - `app/coderunner/cmd/server/main.go` (修改)

## ADDED Requirements

### Requirement: Piston API 集成
系统 SHALL 通过 HTTP 调用 Piston API 执行用户代码。

#### Scenario: 成功执行代码
- **WHEN** 调用 Execute(language="go", code="package main\nimport \"fmt\"\nfunc main(){fmt.Println(\"hello\")}")
- **THEN** 返回 stdout="hello\n", success=true, exit_code=0

#### Scenario: 批量测试用例
- **WHEN** 传入 test_cases 和代码
- **THEN** 逐个执行测试用例，返回 passed_count/total_count

### Requirement: 多语言支持
系统 SHALL 支持 go, python, javascript, java, cpp 五种编程语言。

#### Scenario: 查询语言列表
- **WHEN** 调用 ListLanguages
- **THEN** 返回 5 种语言及其版本

#### Scenario: 不支持的语言
- **WHEN** 调用 Execute(language="ruby", ...)
- **THEN** 返回 UNSUPPORTED_LANGUAGE 错误

### Requirement: 错误处理
系统 SHALL 正确处理各种错误场景。

#### Scenario: Piston 服务不可用
- **WHEN** Piston API 连接失败或返回非 200
- **THEN** 返回 PISTON_UNAVAILABLE (503) 错误

#### Scenario: 代码执行超时
- **WHEN** Piston 返回 signal="SIGKILL"
- **THEN** 返回 EXECUTION_TIMEOUT (408) 错误

## MODIFIED Requirements

### Requirement: 配置结构
Bootstrap 结构体需要添加 Piston 相关配置字段。

## REMOVED Requirements
无

## 全局规范遵循
- 错误处理：使用 kratos errors 包
- 构造函数：NewXxx(deps...) 模式
- 禁止全局变量
- 禁止 init() 函数
- 使用 context 传播
