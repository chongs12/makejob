# 当前权限模型说明

## 1. 当前结论

当前项目已经有“后台管理”和“管理员”概念，但还没有做成一套可在后台自由配置的完整权限系统。

现状更准确地说是：

- 有后台管理页面和后台管理接口。
- 有管理员角色 `admin`，也有普通用户角色 `pro_member` / `free_member`。
- 后端权限控制主要靠“用户角色字段 + Casbin 硬编码策略”。
- 没有“超级管理员初始化向导”或“首个管理员创建接口”。
- 没有“权限点可视化配置台”或“角色权限在线编辑器”。

## 2. 当前角色模型

用户角色定义在 [backend/internal/model/user.go](/D:/gogogo/makejob/backend/internal/model/user.go)：

- `admin`
- `pro_member`
- `free_member`

另外，项目里还存在一个实际使用中的特殊状态：

- `disabled`

这个值不是正式常量角色，但在禁用用户时会直接写入 `users.role`，见 [backend/internal/repository/admin_repo.go](/D:/gogogo/makejob/backend/internal/repository/admin_repo.go)。

这意味着当前系统的“权限/状态”实际上是混在一个字段里的：

- `admin` 代表管理员
- `pro_member` 代表普通付费用户
- `free_member` 代表普通免费用户
- `disabled` 代表被禁用用户

这不是一个很干净的建模方式，后续如果继续扩展权限，建议把“角色”和“状态”拆开。

## 3. 管理员现在是怎么创建的

### 方式一：空库首次启动时自动种子创建

后端启动时会执行 `AutoMigrate` 和 `SeedData`，见 [backend/cmd/server/main.go](/D:/gogogo/makejob/backend/cmd/server/main.go)。

默认管理员由 [backend/internal/model/seed.go](/D:/gogogo/makejob/backend/internal/model/seed.go) 中的 `seedAdminUser` 创建：

- 邮箱：`admin@makejob.com`
- 密码：`admin123456`
- 角色：`admin`
- 会员等级：`pro`

但这里有一个很重要的限制：

- `SeedData` 先检查 `industries` 表是否已有数据。
- 只要 `industries` 表里已有数据，就会直接跳过整个 seed 过程。
- 这意味着默认管理员只会在“首次空库初始化”时自动创建。

所以，默认管理员并不是“每次启动自动补齐”，而是“只在首次空库 seed 时出现”。

### 方式二：已有管理员提升其他用户

当前已经实现了管理员给用户改角色的能力：

- 接口：`PUT /api/admin/users/:id/role`
- 处理器： [backend/internal/handler/admin_handler.go](/D:/gogogo/makejob/backend/internal/handler/admin_handler.go)
- 服务层： [backend/internal/service/admin_service.go](/D:/gogogo/makejob/backend/internal/service/admin_service.go)

可分配的角色只有三种：

- `admin`
- `pro_member`
- `free_member`

也就是说，如果系统里已经有一个管理员，那么最正常的创建管理员方式是：

1. 先注册一个普通用户
2. 由现有管理员把该用户角色改成 `admin`

### 方式三：没有任何管理员时，手动改库

如果当前数据库不是空库，且系统里已经没有任何管理员，那么现在没有公开的页面或接口能“自举”出第一个管理员。

此时只能手动处理数据库。最稳妥的做法是：

1. 先让目标账号正常注册
2. 手动把 `users` 表中该用户的 `role` 改成 `admin`
3. 视需要把 `membership_level` 改成 `pro`

示例 SQL：

```sql
UPDATE users
SET role = 'admin', membership_level = 'pro'
WHERE email = 'your-admin@example.com';
```

如果当前系统里一个用户都没有，也可以手动插入用户，但这会涉及密码哈希、必填字段和数据库方言差异，不建议作为日常运维手段。

## 4. 后台管理是否存在

有。

前端已经存在后台页面，位于 [frontend/pages/admin](/D:/gogogo/makejob/frontend/pages/admin)：

- `index.vue`
- `users.vue`
- `questions.vue`
- `industries.vue`
- `prompts.vue`
- `ai-config.vue`
- `live2d.vue`
- `tts.vue`
- `scraper.vue`
- `orders.vue`

这些页面统一走前端管理员中间件 [frontend/middleware/admin.ts](/D:/gogogo/makejob/frontend/middleware/admin.ts)：

- 未登录跳转 `/auth/login`
- 已登录但不是管理员时跳回首页

后端 `/api/admin` 路由也已经单独分组，见 [backend/cmd/server/main.go](/D:/gogogo/makejob/backend/cmd/server/main.go)：

- 先过 `Auth()` 登录校验
- 再过 `Casbin()` 权限校验

## 5. 管理员和普通用户的区别

有区别，但区别比较粗粒度，不是完整的组织权限体系。

### 管理员 `admin`

Casbin 中直接授予：

- `/api/admin/*` 的全部访问权限

这意味着管理员可以进入后台，并调用后台管理接口。

目前后端已经接入的主要后台能力包括：

- 仪表盘查看
- 用户列表
- 修改用户角色
- 禁用/启用用户
- 题目管理
- 分类管理
- 行业管理
- Prompt 管理
- AI 配置管理
- Live2D 管理
- TTS 配置管理
- 抓题 Scraper 管理

### 普通用户 `pro_member`

`pro_member` 是“付费普通用户”，不是后台管理员。

当前主要可访问：

- 用户资料
- 修改密码
- 收藏 / 笔记 / 记录
- 练习相关接口
- 题目读取
- 模拟面试相关接口
- 计划相关接口
- 陪练相关接口
- 会员信息接口

### 普通用户 `free_member`

`free_member` 是“免费普通用户”，不是后台管理员。

当前也能访问用户相关基础能力，但业务接口范围比 `pro_member` 更窄，主要是部分 `GET` / `POST` 能力。

### 被禁用用户 `disabled`

系统通过把 `role` 直接改成 `disabled` 达到禁用效果。

由于 Casbin 里没有给 `disabled` 配任何策略，所以这类用户通常无法通过原有权限校验。

## 6. 各角色当前能做什么

权限规则定义在 [backend/internal/service/casbin_service.go](/D:/gogogo/makejob/backend/internal/service/casbin_service.go)。

### `admin`

- 允许访问 `/api/admin/*`
- 本质上拥有后台所有已接入接口的权限

### `pro_member`

- `GET/PUT /api/user/profile`
- `PUT /api/user/password`
- `/api/user/favorites/*`
- `/api/user/notes/*`
- `/api/user/records/*`
- `/api/practice/*`
- `GET /api/questions/*`
- `/api/interview/*`
- `/api/plan/*`
- `/api/companion/*`
- `GET /api/membership/*`

### `free_member`

- `GET/PUT /api/user/profile`
- `PUT /api/user/password`
- `GET/POST /api/practice/*`
- `GET /api/questions/*`
- `GET/POST /api/interview/*`
- `GET /api/plan/*`
- `GET/POST /api/companion/*`
- `GET /api/membership/*`

## 7. 当前权限是怎么管理的

### 7.1 角色管理

当前唯一真正“可运营”的权限管理动作，是管理员修改用户角色：

- `PUT /api/admin/users/:id/role`

可选角色只有：

- `admin`
- `pro_member`
- `free_member`

另外还有一个禁用接口：

- `PUT /api/admin/users/:id/disable`

它实际上是把 `role` 改成 `disabled`，再次调用时再切回 `free_member`。

### 7.2 策略管理

Casbin service 暴露了这些方法：

- `AddPolicy`
- `RemovePolicy`
- `AddRoleForUser`
- `RemoveRoleForUser`

但当前项目里没有看到对应的后台页面或开放接口来调用这些方法。

所以现在的权限策略管理方式实际上是：

1. 开发修改 [backend/internal/service/casbin_service.go](/D:/gogogo/makejob/backend/internal/service/casbin_service.go)
2. 重新部署后端

换句话说，当前权限策略不是运行时可配置，而是代码硬编码。

## 8. 当前后台功能覆盖情况

从前后端联动看，后台管理已经覆盖以下方向：

- 用户管理
- 题库与分类管理
- 行业管理
- Prompt 管理
- AI 参数管理
- Live2D 管理
- TTS 管理
- Scraper 抓题流程

但也有明显缺口：

- 前端存在 `orders.vue` 页面，并请求 `/api/admin/orders`
- 后端当前注册的后台路由里没有对应的订单管理接口

这说明后台并不是所有页面都已经完全打通，部分页面仍可能处于占位、半成品或待接后端状态。

## 9. 当前限制与风险

### 9.1 没有首个管理员自助创建能力

一旦默认 seed 没跑到，或者管理员账号被删光，系统没有自恢复入口。

### 9.2 角色和用户状态混在一起

`disabled` 被直接写进 `role` 字段，这会让后续扩展更容易混乱。

### 9.3 角色粒度过粗

现在只有：

- 管理员
- 付费普通用户
- 免费普通用户

没有这些常见中间层：

- 超级管理员
- 运营管理员
- 内容管理员
- 审核人员
- 客服人员

### 9.4 权限策略是硬编码

权限变更不能由运营后台完成，只能改代码。

### 9.5 后台页面与后端能力并非完全一致

目前至少订单管理存在前端页面已出现、后端接口未完整接入的情况。

## 10. 现阶段推荐的管理方式

在当前代码结构下，建议按下面方式运维：

### 场景一：新环境初始化

1. 使用空数据库启动后端
2. 登录默认管理员 `admin@makejob.com / admin123456`
3. 第一时间修改默认密码
4. 再通过后台把需要的用户提升为管理员

### 场景二：已有管理员，新增管理员

1. 让目标用户先正常注册
2. 使用已有管理员登录后台
3. 在用户管理中把该用户角色改成 `admin`

### 场景三：没有管理员，但库里已有数据

1. 让目标账号先注册
2. 手动执行数据库更新，把该用户的 `role` 改为 `admin`
3. 重登验证后台访问是否正常

### 场景四：需要新增权限能力

1. 先定义清楚新角色或新资源范围
2. 修改 [backend/internal/service/casbin_service.go](/D:/gogogo/makejob/backend/internal/service/casbin_service.go) 中的策略
3. 如有必要，补充前端路由中间件和后台入口
4. 联调确认前后端接口一致

## 11. 后续建议

如果后续要继续替换 mock、推进真实业务开发，权限系统建议优先做这几件事：

1. 增加“首个管理员初始化”机制，避免必须手动改库。
2. 把“用户状态”和“用户角色”拆成两个字段。
3. 把 Casbin 策略从硬编码改为数据库或配置化管理。
4. 增加后台权限管理页，至少支持角色分配和策略查看。
5. 补齐后台页面与后端接口的一致性，先清理掉未打通的页面。

