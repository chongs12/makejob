# Checklist

## P4-9: Realtime Service - Full Implementation

### 配置
- [x] conf/conf.go 添加 Volcengine 配置（AppID, Token, WSUrl）
- [x] conf/conf.go 添加 InterviewService 和 RAGService 配置
- [x] configs/config.yaml 包含所有配置

### 核心业务逻辑
- [x] biz/realtime.go 定义 SessionManager
- [x] biz/realtime.go 定义 InterviewClient 接口
- [x] biz/realtime.go 定义 RAGClient 接口
- [x] biz/realtime.go 实现 HandleSession 方法
- [x] biz/realtime.go 实现 clientToVolc goroutine
- [x] biz/realtime.go 实现 volcToClient goroutine
- [x] biz/realtime.go 实现 ragInjector goroutine

### Volcengine 客户端
- [x] data/volcengine_client.go 实现二进制协议 encode/decode
- [x] data/volcengine_client.go 实现 Connect 方法
- [x] data/volcengine_client.go 实现 SendAudio 方法
- [x] data/volcengine_client.go 实现 ReadEvent 方法
- [x] data/volcengine_client.go 实现 InjectContext 方法
- [x] data/volcengine_client.go 实现 Close 方法

### WebSocket 处理器
- [x] server/http.go 实现 WebSocket upgrade handler
- [x] server/http.go 实现路由 /ws/interview/:interview_id
- [x] server/http.go 实现认证

### gRPC handler
- [x] service/realtime.go 实现 InitSession handler
- [x] service/realtime.go 实现 GetSessionStatus handler
- [x] service/realtime.go 实现 InjectRAGContext handler
- [x] service/realtime.go 实现 EndSession handler

### 启动入口
- [x] main.go 启动 HTTP server
- [x] go build 编译通过
- [x] go vet 通过
