# Checklist

- [x] conf.go 包含 Piston 配置字段（Endpoint, TimeoutMs）
- [x] biz/errors.go 定义三个领域错误
- [x] biz/coderunner.go 定义 PistonExecutor 接口
- [x] biz/coderunner.go 实现 CodeRunnerUseCase.Execute 方法
- [x] data/piston_client.go 实现 Piston API 调用
- [x] data/piston_client.go 实现语言版本映射（5种语言）
- [x] data/piston_client.go 正确处理超时和不可用错误
- [x] data/data.go 简化为无数据库版本
- [x] service/coderunner.go 实现 Execute 方法
- [x] service/coderunner.go 实现 ListLanguages 方法
- [x] service/coderunner.go 正确进行 proto 转换
- [x] server/grpc.go 移除 JWT 认证
- [x] main.go 正确装配依赖（无数据库）
- [x] config.yaml 包含 piston 配置段
- [x] go build ./app/coderunner/cmd/server/ 编译通过
- [ ] 单元测试通过
