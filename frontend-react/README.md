# MakeJob React Frontend

这个目录是 MakeJob 新前端重构的 React + Vite 工作区。

当前目标不是一次性搬完全部页面，而是先建立稳定的新主干：

- `apps/web`：业务前台，承载刷题、面试、陪伴、AI 实时交互
- `apps/admin`：后台管理，承载题库、Live2D、提示词、TTS、运营配置
- `packages/api-client`：统一的 HTTP 客户端与鉴权头注入
- `packages/shared-types`：共享接口类型与基础数据结构

## 设计原则

- 先并行重构，不覆盖现有 `frontend`
- 先打通登录、路由、鉴权、基础布局，再逐步迁移业务模块
- 文本流、语音流、Live2D 渲染分别设计，不把实时能力混成一团

## 启动方式

依赖安装完成后，可分别启动：

```bash
npm run dev:web
npm run dev:admin
```

## 当前状态

当前骨架已具备：

- React 19 + Vite 7 基础结构
- TanStack Router 路由骨架
- TanStack Query 预留上下文
- Zustand 登录态存储
- `Bearer {token}` 自动注入的共享 API 客户端

业务页面仍是占位页，后续按迁移文档逐页替换。
