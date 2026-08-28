# PreHire — AI 求职面试平台

> 基于 15 个 Go/Kratos 微服务 + React 19 的 AI 求职面试练习平台：AI 面试官（Live2D 数字人 + 实时语音对话）、智能题库（代码运行 + 自动判题）、陪伴对话、成长规划与学习档案。已完整部署到本地 kind K8s 集群，并集成 OTel/Jaeger/Prometheus/Grafana 全链路可观测性。

## 功能特性

- **AI 面试官**：Live2D 数字人形象 + 火山引擎实时语音对话，模拟真实面试问答场景
- **智能题库**：多行业多分类题目，支持代码运行（Piston 引擎）、结构化解析与自动判题
- **陪伴对话**：Live2D 陪伴角色，TTS 语音播放 + 口型同步，提供情绪化陪伴体验
- **成长规划**：学习档案、成长路线与周度聚焦，跟踪求职准备进度

## 技术栈

| 层 | 技术 |
|---|---|
| 后端 | Go 1.25 · Kratos v2.8.3 · 15 个微服务 · gRPC · Gin |
| 前端 | React 19 · Vite · Pixi.js · Live2D（Cubism 4） |
| AI | 火山引擎 Ark（doubao / minimax）· eino · RAG（Milvus 向量检索） |
| 基础设施 | PostgreSQL · Redis · RabbitMQ · etcd · MinIO · Milvus · Piston |
| 部署 | kind（3 节点）· Helm Chart · ingress-nginx |
| 可观测性 | OTel Collector · Jaeger · Prometheus · Grafana |

## 架构总览

```
浏览器 http://localhost:8080
  └─ ingress-nginx（hostPort 80，Docker 发布 8080）
       ├─ /              → 前端 nginx（React 静态资源）
       ├─ /api/v1        → gateway:8082（15 个微服务统一入口）
       ├─ /live2d-assets → gateway 静态资源（Live2D Core + 模型）
       ├─ *.localhost    → Prometheus / Jaeger / Grafana（可观测性）
       └─ /interviews/*/ws → 实时语音面试 WebSocket 长连接
```

- **15 个微服务**：gateway / user / membership / question / interview / realtime / growth / plan / companion / community / learning-archive / ai-gateway / rag / coderunner / admin
- **有状态基础设施外置**：数据库与中间件跑在宿主机（docker-compose），无状态业务容器化——标准的"有状态外置、无状态容器化"云原生架构
- 服务间通过 K8s Service DNS 互访（如 `makejob-question:9003`），基础设施通过 `host.docker.internal:端口` 访问宿主机

---

## 快速开始

### 前置要求

| 依赖 | 版本 | 说明 |
|---|---|---|
| Docker Desktop | 4.x（WSL2 后端） | 基础设施 + kind 集群 |
| kind | v0.24+ | 本地 K8s（`go install sigs.k8s.io/kind@v0.24.0`） |
| helm | v3.16+ | Chart 部署 |
| kubectl | v1.32+ | 集群操作 |
| Go | 1.25 | 编译微服务 |
| Node.js | 20+ | 前端构建 |

> Windows/Git Bash 用户：`kind`/`helm`/`kubectl` 可能不在 PATH，先执行 `export PATH="$PATH:$(go env GOPATH)/bin"`。

### 路径 A：本地开发模式（轻量，不依赖 K8s，适合功能调试）

```bash
# 1. 启动基础设施（PostgreSQL / Redis 单独起，其余用 compose）
docker run -d --name makejob-postgres -e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=password \
  -e POSTGRES_DB=makejob -p 5434:5432 -v pgdata:/var/lib/postgresql/data postgres:16
docker run -d --name makejob-redis -p 6384:6379 redis:7
docker compose up -d          # RabbitMQ / etcd / MinIO / Milvus
docker compose -f docs/backend/piston/docker-compose.yml up -d   # 代码执行引擎（Piston）

# 2. 编译并启动微服务（配置默认连 localhost:端口）
make build
./bin/gateway -conf app/gateway/configs/config.yaml &      # 建议逐窗口启动，或自建脚本
# ... 其余服务同理

# 3. 启动前端（Vite dev，代理 /api/v1 与 /live2d-assets 到 127.0.0.1:8082）
cd frontend-react && npm install && npm run dev:web
# 浏览器打开 http://localhost:3101
```

### 路径 B：K8s 完整部署（作品核心，演示/面试推荐）

```bash
# 1. 基础设施（同路径 A 第 1 步，K8s 内服务通过 host.docker.internal 访问宿主机）

# 2. 创建 kind 集群（1 control-plane + 2 worker）
kind create cluster --name makejob --config deploy/kind-cluster.yaml

# 3. 安装 ingress-nginx 控制器并钉到 control-plane 节点（打通 Docker 8080 → 节点 80）
helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm install ingress-nginx ingress-nginx/ingress-nginx -n ingress-nginx --create-namespace
kubectl patch deploy ingress-nginx-controller -n ingress-nginx --type=merge -p \
  '{"spec":{"template":{"spec":{"nodeSelector":{"kubernetes.io/hostname":"makejob-control-plane","kubernetes.io/os":"linux"}}}}}'

# 4. 构建镜像并载入集群
make docker-build-all          # 15 个后端镜像 makejob/<svc>:latest
make docker-build-frontend     # 前端镜像 makejob/frontend:latest
for img in $(docker images --format '{{.Repository}}:{{.Tag}}' | grep '^makejob/'); do
  kind load docker-image "$img" --name makejob
done

# 5. Helm 部署
helm upgrade --install makejob deploy/helm/makejob

# 6. 验证
kubectl get pods               # 全部 Pod Running
# 浏览器打开 http://localhost:8080 （必须用 localhost，不能用 127.0.0.1！）
```

## 可观测性

部署后可观测性栈通过 ingress 子域名访问（`*.localhost` 由浏览器自动解析到本机）：

| 组件 | 地址 | 说明 |
|---|---|---|
| Jaeger | http://jaeger.localhost:8080 | 全链路分布式追踪 |
| Prometheus | http://prometheus.localhost:8080 | 指标查询 / targets 抓取状态 |
| Grafana | http://grafana.localhost:8080 | 预置「PreHire 全栈概览」仪表盘（admin / makejob123） |

---

## 验证清单

- [ ] `kubectl get pods`：全部 Pod `Running`、`1/1`
- [ ] `http://localhost:8080` 打开前端页面（**必须 localhost**）
- [ ] 注册 / 登录可用（gateway → user → PostgreSQL）
- [ ] 题库"运行代码"出结果（依赖宿主机 Piston :2000）
- [ ] 面试 / 陪伴页 Live2D 形象正常（依赖 gateway 镜像内 live2d-src 资源）
- [ ] Jaeger 能看到完整 trace 链（操作页面后刷新）
- [ ] Prometheus 16 个 targets 全部 `UP`

## 常见问题

| 现象 | 原因与解决 |
|---|---|
| 页面打不开 / ERR_EMPTY_RESPONSE | ingress-nginx Pod 不在 control-plane 节点（重装后需重新 patch nodeSelector），或浏览器用了 127.0.0.1 |
| "运行代码"500 / 代码执行引擎不可用 | 宿主机 Piston 未启动：`docker compose -f docs/backend/piston/docker-compose.yml up -d` |
| Live2D 报"Cubism Core 未挂载" | gateway 镜像缺少 live2d-src（需按 Dockerfile 重新构建） |
| 服务间 `lookup xxx` 解析失败 | configmap 地址必须用 K8s Service DNS（`makejob-<svc>:port`） |
| 连不上数据库 / Redis / Milvus | K8s 内访问宿主机必须用 `host.docker.internal:端口`，不能用 localhost |
| AI 功能 500 | 火山引擎 Ark 配额耗尽或 key 无订阅；模型配置在数据库 `admin_configs` 表 |
| Grafana 空 / 打不开 | 确认内存限制 ≥ 512Mi（128Mi 会触发 SQLite 锁）；访问走 `http://grafana.localhost:8080` |

## 参考与致谢

- 前端 Live2D 数字人实现（Cubism 4 + Pixi.js 渲染、口型同步）受 [LLM_Live2D](https://github.com/entropy622/LLM_Live2D) 项目启发
- 代码执行引擎使用开源项目 [Piston](https://github.com/engineer-man/piston)

## 目录结构

```
app/                    # 15 个 Go/Kratos 微服务（cmd/server 入口 + internal/{conf,biz,data,service}）
frontend-react/         # React 19 前端（monorepo：apps/web 用户端 + apps/admin 管理端）
deploy/
  kind-cluster.yaml     # kind 集群配置（3 节点 + extraPortMappings）
  helm/makejob/         # Helm Chart（values + templates，含 configmap/secret/deployment/ingress）
live2d-src/             # Live2D 模型资源（打包进 gateway 镜像）
docker-compose.yml      # 基础设施：RabbitMQ / etcd / MinIO / Milvus
docs/backend/piston/    # 代码执行引擎 Piston 的本地部署配置
```
