# MakeJob 统一多阶段 Dockerfile
# 用法: docker build --build-arg SERVICE_NAME=interview -t makejob/interview .
# SERVICE_NAME 为 app/<svc>/cmd/server 的 <svc> 目录名（gateway/user/interview/...）
# 对应 docs/observability-k8s-checklist.md 4.2

# ===== Stage 1: builder =====
FROM golang:1.25-alpine AS builder

ARG SERVICE_NAME
RUN apk add --no-cache git

WORKDIR /build
# 先拷贝依赖清单，利用层缓存
COPY go.mod go.sum ./
RUN go mod download

# 拷贝源码并编译指定服务
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/server \
    ./app/${SERVICE_NAME}/cmd/server

# ===== Stage 2: runtime =====
# distroless static: 无 shell，体积小，非 root 运行，适合静态编译的 Go 二进制
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /
COPY --from=builder /out/server /server
# Live2D 静态资源（live2dcubismcore.min.js + 模型），gateway 通过 /live2d-assets 提供
# EnsureAssetsDir 会命中候选路径 live2d-src（工作目录 / 下），无需额外环境变量
COPY --from=builder /build/live2d-src /live2d-src

# 6060 = telemetry(metrics/healthz/pprof)；业务 gRPC/HTTP 端口由 config.yaml 决定，不在此 EXPOSE
EXPOSE 6060

# config.yaml 由 K8s ConfigMap 挂载到 /etc/makejob/config.yaml（本地 docker run 用 -v 挂载）
ENTRYPOINT ["/server", "-conf", "/etc/makejob/config.yaml"]
