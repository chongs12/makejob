# MakeJob 前端 Dockerfile（monorepo: apps/web）
# 对应 docs/observability-k8s-checklist.md 4.3
# 构建上下文需为仓库根目录：docker build -f frontend-react/Dockerfile -t makejob/frontend .

# ===== Stage 1: builder =====
FROM node:20-alpine AS builder
WORKDIR /build

# 先拷贝依赖清单，利用层缓存
COPY frontend-react/package.json frontend-react/package-lock.json ./
COPY frontend-react/apps/web/package.json ./apps/web/
COPY frontend-react/apps/admin/package.json ./apps/admin/
COPY frontend-react/packages/api-client/package.json ./packages/api-client/
RUN npm ci

# 拷贝源码并构建 web + admin（admin 部署在 /admin 路径，见 nginx.conf）
COPY frontend-react/ .
RUN npm run build:web && npm run build:admin

# ===== Stage 2: runtime =====
FROM nginx:alpine
COPY --from=builder /build/apps/web/dist /usr/share/nginx/html
COPY --from=builder /build/apps/admin/dist /usr/share/nginx/html/admin
COPY frontend-react/nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
