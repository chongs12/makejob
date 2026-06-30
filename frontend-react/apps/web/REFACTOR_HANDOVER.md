# MakeJob 前台前端重构交接文档

> 交接时间：2026-06-16
> 最后更新：2026-06-19
> 重构范围：`apps/web` 前台页面视觉与交互升级
> 设计风格：LeetCode 式专业简洁风 + 统一 inline styles

---

## 一、已重构页面

### 1. 全局导航栏 (`router.tsx`)
- **改动**：完全重写 `RootLayout` 中的 `header`
- **旧状态**：78px 高导航 + 44px debug 状态条，信息泄露
- **新设计**：
  - 56px 高度 sticky header
  - 白色毛玻璃背景（`backdrop-filter: blur(16px)`）
  - 左侧 `M` 渐变徽章 + MakeJob logo
  - 中部 pill 导航链接（hover 高亮）
  - 搜索框（带 `SearchOutlined`，跳转题库）
  - 右侧橙色「发布」按钮 + 用户头像/登录入口
- **关键移除**：彻底删除 `topbar-status` debug 条

### 2. 首页 (`features/home/HomePage.tsx`)
- **改动**：从"大方块卡片堆叠"改为 LeetCode 风格双栏门户
- **新布局**：
  - Hero 区：左侧价值主张文案 + 右侧「每日一题」卡片
  - 已登录时展示数据条（今日答题 / 连续打卡 / 正确率 / 收藏数）
  - 主内容：社区动态列表 + 题库预览
  - 右侧边栏：学习统计、热门行业、快捷入口
- **风格**：紧凑信息密度，白色卡片 + 12px 圆角 + 细边框

### 3. 题库列表页 (`features/practice/PracticePage.tsx`)
- **改动**：从松散卡片改为高密表格布局
- **新布局**：
  - 顶部筛选栏：行业 / 难度 / 分类 `Select` + 搜索 `Input`
  - 数据条：今日完成 / 累计作答 / 正确率 / 连续打卡 / 错题待复习 / 已收藏
  - 主表格：行首状态（✓/⭐/○）+ 标题 + 分类 + 通过率进度条 + 难度标签 + 收藏星
  - 右侧边栏：推荐补练、核心题单、快速入口
- **依赖更新**：`shared/practiceCatalog.ts` 新增 `is_answered?: boolean` 字段

### 4. 社区全模块 (`features/community/CommunityPages.tsx`)
- **改动**：5 个子页面全部重写，脱离 `styles.css`
- **包含页面**：
  - `CommunityPage`（广场）
  - `CommunityPostDetailPage`（详情）
  - `CommunityCreatePostPage`（发布）
  - `CommunityEditPostPage`（编辑）
  - `CommunityMyPostsPage`（我的帖子）
- **新设计**：
  - 广场/我的帖子：双栏布局（左 feed + 右辅助边栏）
  - Feed 条目：hover 阴影 + 类型标签 + 摘要 + 互动数据行
  - 详情页：独立内容卡片 + 评论区卡片 + 头像评论列表
  - 编辑器：AntD `Input` / `TextArea` / `Select`，带字数统计
  - 筛选器：AntD 组件，紧凑行内排列

### 5. 成长档案 (`features/growth/GrowthPage.tsx`)
- **改动**：从纵向堆叠 7 大模块改为仪表板 Dashboard 风格
- **新布局**：
  - 顶部核心指标行：4 列紧凑卡片（学习天数 / 连续打卡 / 面试场次 / 总正确率）
  - 当前主计划横幅：蓝色背景 + 进度条 + 下一步任务 + 行动按钮
  - 行动中心：`Tabs` 三标签页（趋势信号 / 本周补强 / 推荐补练）
  - 双栏数据区：左栏练习概览（环形正确率 + 分类迷你进度条）+ 右栏最近面试/计划列表
  - 底部学习日志：`Timeline` 时间轴

### 6. 管理端 — 题单管理 (`admin/src/features/question-set/QuestionSetPage.tsx`)
- **新增页面**：完全新建的题单 CRUD 管理页
- **路由**：`/question-sets`
- **功能**：
  - 题单列表表格（ID / 名称 / 行业 / 题目数 / 创建时间 / 操作）
  - 顶部筛选栏（关键词 + 行业）
  - 新建/编辑弹窗（名称、行业、描述、封面图 URL）
  - 详情抽屉（右侧滑出，展示题单信息 + 关联题目列表 + 移除按钮）
  - 添加题目弹窗（从题库复选框多选，分页加载）
- **API 对接**：管理端 7 个题单 RPC 全覆盖
- **封面图字段**：表单中保留但标记为「预留字段」，disabled 状态，提示暂无上传服务和前台展示

### 7. 登录页及全局组件 (`router.tsx`)
- `LoginPage` 组件 — 改用 Ant Design `Input`/`Input.Password`/`Button` + inline styles
- `LoginRequiredDialog` 组件 — 改用 inline styles 遮罩弹窗 + Ant Design Button
- `RouteLoadingFallback` 组件 — 改用 inline styles + Ant Design Spin

### 8. 面试模块（部分）
- `features/interview/InterviewPage.tsx` (`InterviewHubPage`) — 简洁双栏布局，inline styles
- `features/interview/InterviewReportPage.tsx` — 全面重构，inline styles + Ant Design
- `features/interview/InterviewHistoryPage.tsx` — **新增页面**，面试历史记录列表，inline styles

### 9. 学习陪伴模块（部分）
- `features/companion/CompanionHubPage.tsx` — 沉浸学习型布局，inline styles

---

## 二、未重构页面（待办）

### 1. 编程题编辑器 (`features/practice/PracticeDetailPages.tsx` 中的 `PracticeEditorPage`)

- **className 数量**：47 处
- **状态**：❌ **未重构**，仍使用 `editor-immersive`、`editor-topbar`、`editor-body`、`editor-workspace`、`editor-toolbar` 等旧类名
- **说明**：沉浸式暗色主题独立页面（无导航栏），含 Monaco Editor，重构时需保留特殊布局逻辑
- **同文件其他页面**：`PracticeQuestionPage`、`PracticeWrongPage`、`PracticeFavoritesPage`、`PracticeNotesPage`、`MistakeTopicPage` 均已重构

### 2. 面试会话页 (`features/interview/InterviewSessionPage.tsx`)

- **className 数量**：45 处
- **状态**：❌ **未重构**
- **旧类名**：`page-panel`、`interview-page-panel`、`interview-session-layout`、`interview-stage-shell`、`status-card`、`companion-card-head`、`section-kicker`、`primary-button`、`secondary-button`、`ghost-button` 等
- **说明**：沉浸式独立页面（Live2D 舞台 + 答题面板），重构时需保留 Live2D 集成逻辑

### 3. 学习陪伴工作区 (`features/companion/CompanionWorkspacePage.tsx`)

- **className 数量**：102 处（最重）
- **状态**：❌ **未重构**
- **旧类名**：`page-panel`、`companion-page-panel`、`companion-layout`、`companion-sidebar`、`companion-stage-shell`、`companion-input-panel`、`companion-composer`、`status-card`、`companion-card-head`、`section-kicker`、`companion-progress-bar` 等
- **说明**：沉浸式独立页面（Live2D 舞台 + 对话面板 + 侧边栏），重构工作量最大

### 4. 陪伴共享子组件 (`features/companion/companionShared.tsx`)

- **className 数量**：11 处
- **状态**：❌ **部分未重构**
- **旧类名**：`companion-phase-timeline-compact`、`companion-phase-timeline-segment`、`companion-topic-pills`、`companion-empty-text` 等
- **说明**：`GoalList`、`CompanionTaskFeedbackPanel` 等组件已在 CompanionHubPage 重构时更新，但 `PhaseTimeline`、`TopicPills` 等组件仍使用旧类名

### 5. Live2D 子组件（无需重构）

- `features/interview/InterviewLive2DStage.tsx` — 纯 canvas 渲染，无 className，无需重构
- `features/companion/CompanionLive2DStage.tsx` — 纯 canvas 渲染，无 className，无需重构

---

## 三、待重构汇总

| 文件 | className 数 | 优先级 | 说明 |
|------|-------------|--------|------|
| `CompanionWorkspacePage.tsx` | 102 | 中 | 最重，沉浸式独立页面 |
| `PracticeDetailPages.tsx`（编辑器） | 47 | 高 | 用户高频路径 |
| `InterviewSessionPage.tsx` | 45 | 中 | 沉浸式独立页面 |
| `companionShared.tsx`（部分） | 11 | 低 | 共享子组件 |
| **合计** | **205** | | |

---

## 四、统一设计规范（已确立）

所有已重构页面遵循以下规范，未重构页面后续应参照执行。

### 4.1 样式体系
- **全部使用 inline styles**，不再新增 `styles.css` 类名
- 每个页面顶部定义 `THEME` 常量对象：
  ```ts
  const THEME = {
    bg: '#f8f9fa',
    cardBg: '#ffffff',
    primary: '#f97316',
    primaryLight: '#fff7ed',
    primaryDark: '#ea580c',
    textMain: '#1f2937',
    textSecondary: '#6b7280',
    textMuted: '#9ca3af',
    border: '#f3f4f6',
    borderHover: '#e5e7eb',
    shadow: '0 1px 2px rgba(0,0,0,0.05)',
    shadowCard: '0 4px 6px -1px rgba(0,0,0,0.07), 0 2px 4px -2px rgba(0,0,0,0.05)',
    shadowHover: '0 10px 15px -3px rgba(0,0,0,0.08), 0 4px 6px -4px rgba(0,0,0,0.05)',
    radius: 12,
    radiusSm: 8,
    success: '#22c55e',
    warning: '#f59e0b',
    danger: '#ef4444',
    accent: '#3b82f6',
    purple: '#8b5cf6',
  }
  ```

### 4.2 卡片规范
- 背景：`THEME.cardBg`
- 边框：`1px solid ${THEME.border}`
- 圆角：`THEME.radius`（12px）
- 内边距：视内容密度用 `16px 20px` 或 `20px 24px`
- hover 效果：`boxShadow: THEME.shadowHover` + `borderColor: THEME.borderHover`

### 4.3 布局规范
- 页面容器：`maxWidth: 1200`, `margin: '0 auto'`, `padding: '24px'`
- 双栏布局：`gridTemplateColumns: '1fr 360px'`, `gap: 24`
- 响应式：使用 `repeat(auto-fit, minmax(220px, 1fr))` 等 CSS Grid 弹性列

### 4.4 组件规范
- **全部使用 Ant Design v6** 组件，不再使用原生 `select` / `input` / `button`
- 按钮层级：主行动 `type="primary"`，次要行动默认 `type="default"`，危险操作 `danger`
- 图标：统一从 `@ant-design/icons` 导入，不用第三方图标库
- 空状态：统一使用 `<Empty image={Empty.PRESENTED_IMAGE_SIMPLE} />`
- 加载：统一使用 `<Spin />` + 必要时 `Skeleton`

### 4.5 排版规范
- 页面标题：`fontSize: 22`, `fontWeight: 700`, `color: THEME.textMain`
- 卡片标题：`fontSize: 15~16`, `fontWeight: 700`
- 正文：`fontSize: 13~14`, `color: THEME.textSecondary`
- 辅助文字：`fontSize: 12~13`, `color: THEME.textMuted`

---

## 五、技术栈现状

| 依赖 | 版本 | 说明 |
|------|------|------|
| React | ^19.2.0 | 使用 type imports |
| TanStack Router | ^1.133.48 | 文件已配好懒加载 |
| TanStack Query | ^5.90.5 | 数据获取模式稳定 |
| Ant Design | ^6.4.4 | 全部 UI 组件来源 |
| Zustand | ^5.0.8 | 状态管理 |
| Monaco Editor | ^0.55.1 | 仅编程题编辑器使用 |

---

## 六、已知遗留问题

1. **`styles.css` 仍有大量代码**，但所有已重构页面已完全脱离它。未重构页面仍依赖它，建议等全部重构完成后删除/清理。
2. **Chunk 体积警告**：`vite build` 提示 `index.js > 500KB`，这是 Monaco / PixiJS / PDF.js 等第三方库导致，非业务代码问题。后续可考虑 `manualChunks` 优化。
3. **Admin 面板**：`apps/admin` 下 TaxonomyPage、QuestionPage、Login 等已按玻璃拟态风格重构，与前台是独立的设计语言（后台保留 glassmorphism，前台走 LeetCode 简洁风）。
4. **题单封面图 `cover_image` 是预留字段**：数据库有字段、管理端表单可填写，但无实际上传服务（缺少 UploadImage RPC 和 OSS/MinIO 对接）。当前管理端表单已将该字段标记为「预留字段」并 disabled，前台也不展示封面图。
5. **面试数据问题**（后端）：
   - `mock_interviews` 表 ID 80-90 的 `created_at` 为零值 `0001-01-01`，导致排序异常
   - `report_json` 与 `interview_reports` 表数据不一致，部分已完成面试的报告返回 404
   - `report_failed` 状态未返回有意义的错误信息

---

## 七、下一步建议（优先级排序）

1. **高优**：`PracticeEditorPage` 编程题编辑器重构 — 用户高频路径
2. **中优**：`InterviewSessionPage.tsx` 面试会话页重构 — 核心功能页
3. **中优**：`CompanionWorkspacePage.tsx` 学习陪伴工作区重构 — 工作量最大
4. **低优**：`companionShared.tsx` 剩余子组件样式统一
5. **低优**：`styles.css` 彻底清理
