# MakeJob 微服务启动指南

## 一、前置条件

- Go >= 1.23
- Docker（用于启动基础设施）
- protoc + protoc-gen-go + protoc-gen-go-grpc（仅开发时需要）

## 二、启动基础设施

```bash
cd D:/gogogo/makejob
docker-compose up -d
```

启动后验证：
```bash
docker ps
# 应看到以下容器：
# - makejob-rabbitmq    (5672, 15672)
# - makejob-etcd        (2379)
# - makejob-minio       (9000, 9001)
# - makejob-milvus      (19530, 9091)
```

PostgreSQL 由宿主机提供，端口 5434。

## 三、编译服务

```bash
cd D:/gogogo/makejob

# 编译全部
go build ./...

# 或单独编译某个服务
go build ./app/gateway/cmd/server
go build ./app/user/cmd/server
go build ./app/question/cmd/server
go build ./app/interview/cmd/server
go build ./app/growth/cmd/server
go build ./app/admin/cmd/server
go build ./app/community/cmd/server
go build ./app/learning_archive/cmd/server
```

## 四、启动服务

每个服务在独立终端中启动。**启动顺序建议**：先启动被依赖的服务，最后启动 gateway。

### 1. User 服务（端口 9004）
```bash
cd app/user
go run cmd/server/main.go -conf configs/config.yaml
```

### 2. Question 服务（端口 9002）
```bash
cd app/question
go run cmd/server/main.go -conf configs/config.yaml
```

### 3. Interview 服务（端口 9003）
```bash
cd app/interview
go run cmd/server/main.go -conf configs/config.yaml
```

### 4. Growth 服务（端口 9005）
```bash
cd app/growth
go run cmd/server/main.go -conf configs/config.yaml
```

### 5. Admin 服务（端口 9006）
```bash
cd app/admin
go run cmd/server/main.go -conf configs/config.yaml
```

### 6. Community 服务（端口 9007）
```bash
cd app/community
go run cmd/server/main.go -conf configs/config.yaml
```

### 7. LearningArchive 服务（端口 9008）
```bash
cd app/learning_archive
go run cmd/server/main.go -conf configs/config.yaml
```

### 8. Gateway 网关（端口 8082）— 最后启动
```bash
cd app/gateway
go run cmd/server/main.go -conf configs/config.yaml
```

## 五、端口分配

| 服务 | gRPC 端口 | HTTP 端口 |
|---|---|---|
| user | 9004 | 8004 |
| question | 9002 | 8002 |
| interview | 9003 | 8003 |
| growth | 9005 | 8005 |
| admin | 9006 | 8006 |
| community | 9007 | 8007 |
| learning_archive | 9008 | 8008 |
| **gateway** | - | **8082** |

## 六、验证服务

```bash
# 健康检查
curl http://localhost:8082/api/health

# 用户注册
curl -X POST http://localhost:8082/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"test","email":"test@example.com","password":"123456"}'

# 用户登录
curl -X POST http://localhost:8082/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"123456"}'

# 获取行业列表（无需登录）
curl http://localhost:8082/api/industries

# 使用 token 访问受保护接口
TOKEN="eyJhbG..."
curl http://localhost:8082/api/user/profile -H "Authorization: Bearer $TOKEN"
```

## 七、配置说明

每个服务的配置文件在 `app/<service>/configs/config.yaml`，主要配置项：

- `server.grpc.addr` — gRPC 监听地址
- `server.http.addr` — HTTP 监听地址（gateway 仅有）
- `data.database.source` — PostgreSQL 连接串
- `data.redis.addr` — Redis 地址
- `mq.url` — RabbitMQ 连接串（interview 服务使用）
- `jwt.secret` — JWT 签名密钥

**注意**：admin 和 gateway 服务依赖 `backend/config.yaml`（AI、RAG、爬虫等配置）。bridge 会按以下顺序搜索：
1. `backend/config.yaml`（项目根目录）
2. `../../backend/config.yaml`（从 `app/xxx/` 运行时）

如需指定自定义路径，设置环境变量：
```bash
export MAKEJOB_BACKEND_CONFIG=/path/to/backend/config.yaml
```
