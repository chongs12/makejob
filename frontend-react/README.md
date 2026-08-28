# PreHire React Frontend

本文档基于当前代码状态更新于 `2026-07-09`，描述 `frontend-react` 工作区的真实现状。后端已微服务化（15 个 Kratos 服务 + Gateway，详见 `docs/phase-progress-and-run-guide.md`），前端通过 Gateway HTTP `:8080` 的 `/api/v1` 访问后端。

## 1. 目录定位

`frontend-react` 是 PreHire 当前有效前端主线，包含两个应用和两个共享包：

- `apps/web`：用户前台
- `apps/admin`：后台管理
- `packages/api-client`：共享 HTTP 客户端与鉴权能力
- `packages/shared-types`：前后端共享接口类型

## 2. 技术栈

- React 19
- Vite 7
- TanStack Router
- TanStack Query
- Zustand
- `pixi.js` + `pixi-live2d-display`
- 前台编程题编辑器使用 `monaco-editor`

## 3. 当前实现状态

这不是纯骨架项目，当前已经落地到可运行业务页阶段。

### 3.1 前台 Web

当前已经接入并实现页面主线：

- 首页 `/`
- 题库 `/practice`
- 题目详情 `/practice/$questionId`
- 编程题编辑页 `/practice/editor/$questionId`
- 错题 `/practice/wrong`
- 收藏 `/practice/favorites`
- 笔记 `/practice/notes`
- 错因专题 `/practice/topics/$topicCode`
- 社区列表、详情、发帖、编辑、我的帖子
- AI 面试入口、实时面试页、面试报告页
- 学习陪伴首页和陪伴工作区
- 成长档案 `/growth`
- 登录页 `/auth/login`

### 3.2 后台 Admin

当前已经接入并实现页面主线：

- 总览 `/dashboard`
- 运行任务 `/runtime`
- AI 配置 `/ai-configs`
- Prompt 管理 `/prompts`
- Live2D 管理 `/live2d`
- TTS 配置 `/tts`
- 行业与分类 `/taxonomy`
- 题目流水线 `/question-pipeline`
- 题库管理 `/questions`
- 后台登录 `/auth/login`

## 4. 已经接起来的关键能力

- 登录态初始化、令牌恢复和鉴权守卫
- 前后台共享 API 客户端
- 社区发帖、评论、点赞
- 题库检索、答题、收藏、笔记、错题与推荐
- AI 面试实时页的 WebSocket、TTS 播放、ASR 录音、Live2D 舞台
- 学习陪伴与成长档案联动
- 后台题目流水线的同步生成、流式生成、异步恢复和导入

## 5. 启动命令

```bash
npm run dev:web
npm run dev:admin
```

构建命令：

```bash
npm run build
```

前台测试命令：

```bash
npm run test:web
```

## 6. 当前验证结论

基于 `2026-05-15` 的本地核验：

- `npm run build` 通过
- `npm run test:web` 通过

## 7. 当前需要注意的点

- 当前 README 只描述 React 工作区，不再为旧前端提供说明。
- AI 面试和陪伴页对后端接口、Live2D 资源和第三方配置有依赖。
- 后台存在少量样式层面的构建告警，但不阻塞产物生成。
