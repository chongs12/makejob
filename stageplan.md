# MakeJob 项目现状速览

更新时间: 2026-04-16

> 说明：本文档保留了大量早期 Nuxt 方案分析，当前前端主线已切换为 `frontend-react/`。后文中出现的 `frontend/` 路径，默认视为历史上下文而非当前实现。

本文档面向后续接手该仓库的 agent，目标是用最短时间建立正确心智模型，避免重复踩坑。

## 1. 一句话结论

这是一个“AI 面试/刷题/学习计划”产品原型，采用前后端分离结构：

- 后端: Go + Gin + Gorm + PostgreSQL + Redis + Casbin
- 前端: React + Vite + TanStack Router + React Query

项目的页面和接口覆盖面已经比较广，但整体还处于“流程已铺开、真实能力和前后端闭环未完全打通”的阶段。尤其是 AI 能力、会员限制、管理后台数据、前端状态管理与接口返回字段之间，存在多处不一致。

## 2. 仓库结构

根目录主要内容:

- `backend/`: Go 后端
- `frontend-react/`: React 前端工作区
- `frontend/`: 已废弃的 Nuxt 前端目录，待本地调试缓存释放后彻底删除
- `start.md`: 启动提示，内容很短
- `产品计划书.md`: 产品规划文档
- `debug.log`: 调试日志
- `-p/`: 空目录，可忽略

额外观察:

- 当前根目录是 git 仓库
- React 工作区位于 `frontend-react/`
- 旧 Nuxt 目录只保留了被浏览器占用的本地调试缓存残留

这意味着当前仓库更像“本地开发快照”而不是标准化、精简过的代码仓。

## 3. 启动方式

从 `start.md` 和配置文件看，预期启动方式是:

### 后端

```powershell
cd D:\gogogo\makejob\backend
go run cmd/server/main.go
```

依赖:

- PostgreSQL
- Redis

### 前端

```powershell
cd D:\gogogo\makejob\frontend-react
npm install
npm run dev -w @makejob/web
```

默认关键配置位于 `backend/config.yaml`:

- HTTP 端口: `8080`
- PostgreSQL: `localhost:5434`
- Redis: `localhost:6384`
- 前端 API Base: `http://localhost:8080/api`

## 4. 后端架构概览

后端入口在 `backend/cmd/server/main.go`，启动流程大致如下:

1. 读取配置
2. 初始化日志
3. 初始化 PostgreSQL
4. 自动迁移表结构
5. 自动插入种子数据
6. 初始化 Redis
7. 组装 Repository / Service / Handler
8. 注册 Gin 中间件和路由
9. 启动 HTTP 服务

### 4.1 分层结构

代码分层比较标准:

- `internal/model`: Gorm 模型、数据库初始化、种子数据
- `internal/repository`: 数据访问层
- `internal/service`: 业务逻辑层
- `internal/handler`: HTTP 接口层
- `internal/middleware`: JWT/CORS/Casbin/限流/会员相关中间件
- `internal/ai`: AI 接口抽象与 agent/provider
- `internal/asr`, `internal/tts`, `internal/scraper`: 语音/抓取相关抽象
- `pkg/jwt`, `pkg/logger`: 公共工具

### 4.2 中间件

全局已注册:

- 请求日志
- CORS
- RateLimit
- `gin.Recovery()`

认证路由使用:

- `middleware.Auth()`
- 管理路由额外使用 `middleware.Casbin()`

### 4.3 数据库与种子数据

数据库启动后会自动迁移并插入种子数据。当前种子数据包括:

- 行业: `go` / `java` / `frontend`
- Go 题库分类
- 大量 Go 面试题
- Prompt 模板
- 管理配置
- 默认管理员账号

默认管理员:

- 邮箱: `admin@makejob.com`
- 密码: `admin123456`

## 5. 后端实际能力

### 5.1 认证

已实现:

- 注册: `POST /api/auth/register`
- 登录: `POST /api/auth/login`
- 刷新 Token: `POST /api/auth/refresh`
- 获取个人资料: `GET /api/user/profile`
- 更新个人资料: `PUT /api/user/profile`

说明:

- JWT 已实现
- Refresh Token 也已实现
- `GetMe` 和 `Logout`/`ChangePassword` 有预留代码，但没有完全纳入主流程

### 5.2 题库 / 刷题

已实现接口大体包括:

- 题目列表: `GET /api/questions`
- 题目详情: `GET /api/questions/:id`
- 分类树: `GET /api/categories`
- 提交答案: `POST /api/questions/:id/submit`
- 收藏: `POST /api/questions/:id/favorite`
- 错题/收藏/笔记/统计
- 随机组卷: `POST /api/exams/random`
- 限时组卷: `POST /api/exams/timed`
- 提交试卷: `POST /api/exams/submit`

当前特征:

- 选择题和多选题能按规则判分
- 编程题、主观题目前不会真实判对，只会返回 `false`
- 编程题/主观题的 AI 分析接口已预留，但当前 provider 主要仍是 mock
- 试卷 session 暂存在内存 `sync.Map` 中，不是持久化设计

### 5.3 模拟面试

已实现接口:

- 创建面试
- 面试列表/详情
- 提交回答
- 获取下一题
- 结束面试
- 查看报告
- WebSocket 面试通信

当前特征:

- 服务层结构是完整的
- 面试消息会落库
- AI 会话 ID 临时塞在 `MockInterview.AIFeedback` 字段里，属于权宜设计
- 实际 AI 仍主要接 mock agent，不是真实模型服务

### 5.4 学习计划

已实现接口:

- 创建计划 `POST /api/plans`
- 当前计划 / 历史计划
- 任务状态更新
- 计划调整
- 进度统计

当前特征:

- 后端要求 `GeneratePlanRequest` 中必须有 `industry_id`
- 计划内容由 AI 抽象层生成，但当前依然主要走 mock
- 任务会真实落库

### 5.5 会员

已实现:

- 方案列表
- 会员状态
- 创建订单
- 订单列表/详情
- mock 支付回调

当前特征:

- 数据模型完整
- 会员限制中间件已写
- 但主业务路由里尚未真正完成会员 gating 接入

### 5.6 管理后台

后端已提供大量 `/api/admin/...` 路由，包括:

- dashboard
- users
- questions
- industries
- prompts
- ai-config
- live2d
- tts
- scraper

整体上更像“后台基础接口框架 + CRUD 初版”。

## 6. AI/ASR/TTS/Scraper 的真实状态

这是本项目最容易产生误判的部分。

结论:

- 抽象层已经铺好
- 真实 provider 尚未真正接入主流程
- 当前运行主链仍然偏 mock

具体表现:

- `main.go` 里直接初始化的是 `internal/ai/mock`
- 面试、计划、题目分析等能力都以 mock provider 为主
- `tts`、`asr`、`scraper` 有接口、factory、mock 实现，但没有形成完整生产链路

因此不要把“有目录/有接口定义”误判为“功能已可用”。

## 7. 前端架构概览

前端是 Nuxt 3 项目，关键技术栈:

- Nuxt 3
- Pinia
- Element Plus
- Tailwind CSS

配置位于 `frontend/nuxt.config.ts`:

- `runtimeConfig.public.apiBase = http://localhost:8080/api`
- 路由策略:
  - `/` 预渲染
  - `/dashboard/**` SSR
  - `/admin/**` CSR

主要页面:

- `/`: 首页
- `/auth/login`, `/auth/register`
- `/dashboard`
- `/practice`
- `/interview`
- `/plan`
- `/membership`
- `/companion`
- `/admin/*`

## 8. 前端当前状态判断

前端的“页面壳”完成度比“真实闭环”高很多。

### 8.1 已经具备的部分

- 页面路由齐全
- 通用布局和中后台布局已经有了
- API 封装 `useApi.ts` 已统一
- 鉴权中间件已接入页面

### 8.2 典型问题

很多页面仍是“半静态 + 半真实 API”的状态：

- dashboard 数据大量写死
- admin 首页统计和列表写死
- 多个页面虽然调接口，但字段名和后端返回不匹配
- store 设计存在重复和分叉

## 9. 当前最重要的前后端不一致

以下问题对后续 agent 最重要，优先级很高。

### 9.1 认证用户信息接口不一致

前端 `frontend/stores/auth.ts` 使用:

- `GET /auth/me`

但后端实际注册的接口是:

- `GET /api/user/profile`

后端虽然有 `GetMe` 方法，但没有在主路由里注册到 `/api/auth/me`。

影响:

- `auth` store 登录后刷新用户信息会失败
- 一旦触发失败，前端可能直接 `logout()`

### 9.2 管理员中间件依赖 localStorage.user，但登录并未写入

前端 `frontend/middleware/admin.ts` 会从 `localStorage.user` 读角色。

但 `frontend/stores/auth.ts` 在登录成功后:

- 只保存 `token`
- 保存 `refreshToken`
- 将 `user` 放进内存 state
- 没有写入 `localStorage.user`

影响:

- 管理后台页面很可能始终被判定为非 admin

### 9.3 学习计划创建参数不匹配

后端创建计划要求:

- `industry_id` 必填

前端 `frontend/pages/plan/index.vue` 提交时只传:

- `level`
- `daily_study_time`
- `duration_days`
- `goal_description`

没有传 `industry_id`。

影响:

- 计划创建大概率直接失败

### 9.4 学习计划详情字段不匹配

前端页面按这些字段使用任务:

- `task.date`
- `task.scheduled_date`
- `task.day`
- `task.type`

后端真实返回更接近:

- `day_number`
- `due_date`
- `task_type`

影响:

- 分组显示
- 标签显示
- 进度抽屉

都可能不按预期工作。

### 9.5 学习计划进度接口字段不匹配

前端期望:

- `overall_progress`
- `by_type`
- `daily_trend`

后端实际返回:

- `progress`
- `task_type_stats`
- `daily_progress`

影响:

- 进度抽屉的数据渲染基本对不上

### 9.6 模拟面试创建返回字段不匹配

后端创建面试返回 DTO 中主键字段是:

- `interview_id`

前端 `frontend/pages/interview/index.vue` 创建成功后跳转使用:

- `res.data.id`

影响:

- 创建成功后页面跳转可能失败

### 9.7 模拟面试行业值不规范

前端面试页面下拉给出的值包括:

- `go_basic`
- `concurrency`
- `web_framework`
- `database`
- `microservice`

后端更像期待行业 code:

- `go`
- `java`
- `python`
- `frontend`
- `ai`

影响:

- 行业解析可能退回默认值
- 真实业务语义混乱

### 9.8 随机组卷 / 限时组卷返回字段不匹配

后端返回的是:

- `exam_id`
- `questions`
- `time_limit`

前端 `frontend/pages/practice/index.vue` 使用:

- `res.data.id`

影响:

- 创建试卷后跳转逻辑不成立

### 9.9 题型枚举不一致

后端题型常量:

- `choice`
- `multi`
- `code`
- `subjective`

前端多个地方写成:

- `multiple`
- `coding`

影响:

- 筛选条件
- 题型标签
- 是否跳转代码编辑器页

都可能出错。

### 9.10 用户状态存在双轨

前端同时存在:

- `stores/auth.ts`
- `stores/user.ts`

其中:

- `auth` store 管 token 和基本 user
- `user` store 管 profile

但两者没有形成统一的单一事实源。

影响:

- 登录态判断
- 角色判断
- profile 刷新
- 退出登录

容易互相打架。

## 10. 会员能力的现状判断

会员体系不是“没有”，而是“做了一半”。

已完成:

- 订单模型
- 会员状态计算
- mock 支付
- 中间件

未完成或未接通:

- 主业务路由并未普遍启用会员限制
- 前端付费页与实际限制策略是否一致，尚不可信
- 使用次数统计与扣减主链未确认已全部闭环

尤其要注意:

- `main.go` 中对 practice 路由的会员校验有 `TODO`

## 11. 管理后台现状

后端:

- 有较完整的 admin 路由和服务层

前端:

- 页面数量不少
- 但首页明显写死数据
- 是否全部 CRUD 闭环，需要逐页验证，不应默认可用

权限:

- 后端管理接口要求 JWT + Casbin
- 前端 admin 页面保护逻辑目前本身就有问题，见上文 9.2

## 12. 测试与验证现状

仓库内存在:

- `backend/test_api.ps1`
- `backend/test_api.cmd`

说明项目作者有“手动接口回归”的意识，但当前测试方式仍然是脚本驱动，不是自动化测试体系。

当前观察:

- 未看到成体系的 Go 单测
- 未看到前端测试
- 未看到 CI 配置

此外，`test_api.ps1` 本身也暴露了若干假设:

- 用 `res.data.id` 读取面试/计划结果
- 使用固定用户 `test@test.com`

这些假设未必和当前代码真实匹配。

## 13. 工程质量风险

### 高风险

- 前后端字段名和接口路径不一致较多
- mock 能力与真实能力界线不清晰
- 前端登录态与角色态存在双轨设计
- 会员限制设计存在但主路由未真正落地

### 中风险

- 试卷 session 存在内存里，重启即丢
- 面试会话 ID 暂存在业务字段里，设计不稳
- 仓库直接提交 `node_modules` 和二进制，维护成本高
- 根目录无 git 仓库，版本追踪信息缺失

### 低到中风险

- 终端读取时部分中文注释出现乱码，疑似 Windows 控制台编码问题
- 代码内大量中文注释/文案，维护时建议统一 UTF-8 和终端编码

## 14. 建议后续 agent 的进入顺序

如果你的目标是“最快让项目跑通”，建议顺序如下:

1. 先修认证闭环
2. 再修学习计划闭环
3. 再修模拟面试闭环
4. 再修刷题组卷闭环
5. 最后处理会员限制和管理后台

### 第一步优先修的点

- 统一前端用户信息接口到 `/user/profile`
- 统一 admin 角色来源，不再依赖 `localStorage.user`
- 明确 `auth store` 与 `user store` 的职责，最好收敛成单一来源

### 第二步优先修的点

- 让前端创建计划时传 `industry_id`
- 对齐计划详情、进度统计字段

### 第三步优先修的点

- 对齐面试创建返回字段
- 对齐行业 code
- 检查面试详情页和 report 页是否按后端真实 DTO 渲染

### 第四步优先修的点

- 对齐题型枚举
- 对齐 exam 返回字段
- 校验题目详情页/代码编辑页是否真的可用

## 15. 关键文件索引

后续 agent 最应该先看这些文件:

### 后端入口与配置

- `backend/cmd/server/main.go`
- `backend/config.yaml`
- `backend/internal/config/config.go`
- `backend/internal/model/database.go`
- `backend/internal/model/seed.go`

### 后端主业务

- `backend/internal/handler/auth_handler.go`
- `backend/internal/handler/question_handler.go`
- `backend/internal/handler/interview_handler.go`
- `backend/internal/handler/plan_handler.go`
- `backend/internal/handler/membership_handler.go`
- `backend/internal/service/auth_service.go`
- `backend/internal/service/question_service.go`
- `backend/internal/service/interview_service.go`
- `backend/internal/service/plan_service.go`

### 前端主链

- `frontend/nuxt.config.ts`
- `frontend/composables/useApi.ts`
- `frontend/stores/auth.ts`
- `frontend/stores/user.ts`
- `frontend/middleware/auth.ts`
- `frontend/middleware/admin.ts`
- `frontend/pages/practice/index.vue`
- `frontend/pages/interview/index.vue`
- `frontend/pages/plan/index.vue`
- `frontend/pages/admin/index.vue`

## 16. 最后的判断

当前项目不是“从零开始”，也不是“接近上线”。

更准确的定位是:

- 产品方向明确
- 后端领域模型完整
- 前端页面覆盖面广
- 主流程大多已有第一版
- 但系统仍明显停留在原型深化阶段

最适合的后续工作方式不是大规模重写，而是:

1. 先对齐接口契约
2. 再收敛状态管理
3. 再替换 mock 能力
4. 最后补测试和会员/后台闭环

如果只做小修小补而不先统一契约，这个项目会持续处于“页面很多，但每条链路都差一点”的状态。
