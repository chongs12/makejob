# Tasks

## P4-9: Realtime Service - Full Implementation

- [x] Task 1: 修改 conf/conf.go - 添加 Volcengine 和依赖服务配置
  - [x] SubTask 1.1: 添加 Volcengine 结构体（AppID, Token, WSUrl）
  - [x] SubTask 1.2: 添加 InterviewService 和 RAGService 配置
  - [x] SubTask 1.3: 更新 Bootstrap 结构体

- [x] Task 2: 重写 biz/realtime.go - 核心业务逻辑
  - [x] SubTask 2.1: 定义 SessionManager 管理活跃会话
  - [x] SubTask 2.2: 定义 InterviewClient 接口
  - [x] SubTask 2.3: 定义 RAGClient 接口
  - [x] SubTask 2.4: 实现 HandleSession 方法（WebSocket 处理主逻辑）
  - [x] SubTask 2.5: 实现 clientToVolc goroutine
  - [x] SubTask 2.6: 实现 volcToClient goroutine
  - [x] SubTask 2.7: 实现 ragInjector goroutine

- [x] Task 3: 新建 data/volcengine_client.go - Volcengine WebSocket 客户端
  - [x] SubTask 3.1: 实现二进制协议 encode/decode
  - [x] SubTask 3.2: 实现 Connect 方法
  - [x] SubTask 3.3: 实现 SendAudio 方法
  - [x] SubTask 3.4: 实现 ReadEvent 方法
  - [x] SubTask 3.5: 实现 InjectContext 方法
  - [x] SubTask 3.6: 实现 Close 方法

- [x] Task 4: 新建 server/http.go - WebSocket 处理器
  - [x] SubTask 4.1: 实现 WebSocket upgrade handler
  - [x] SubTask 4.2: 实现路由 /ws/interview/:interview_id
  - [x] SubTask 4.3: 实现认证（token 验证）

- [x] Task 5: 修改 service/realtime.go - 实现 gRPC handler
  - [x] SubTask 5.1: 实现 InitSession handler
  - [x] SubTask 5.2: 实现 GetSessionStatus handler
  - [x] SubTask 5.3: 实现 InjectRAGContext handler
  - [x] SubTask 5.4: 实现 EndSession handler

- [x] Task 6: 修改 main.go - 启动 HTTP server
- [x] Task 7: 更新 configs/config.yaml

# Task Dependencies
- Task 1 无依赖
- Task 2 依赖 Task 1
- Task 3 依赖 Task 1
- Task 4 依赖 Task 2, 3
- Task 5 依赖 Task 2
- Task 6 依赖 Task 4, 5
- Task 7 依赖 Task 1
