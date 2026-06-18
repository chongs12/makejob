# MakeJob 前台前端重构交接文档

> 交接时间：2026-06-16  
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

---

## 二、未重构页面（待办）

### 1. 练习详情模块 (`features/practice/PracticeDetailPages.tsx`)
该文件为巨型文件（~1900 行），包含 6 个导出页面，全部仍使用 `styles.css` 旧类名：
- `PracticeQuestionPage` — 单题详情页（选择/多选/主观题作答）
- `PracticeEditorPage` — 编程题编辑器（含 Monaco Editor）
- `PracticeWrongPage` — 错题本列表
- `PracticeFavoritesPage` — 收藏题目列表
- `PracticeNotesPage` — 题目笔记列表
- `MistakeTopicPage` — 错题专题详情

> **建议**：这是前台未重构中体量最大的文件，优先拆解为多个独立文件后再逐个重写。

### 2. 面试模块
- `features/interview/InterviewPage.tsx` (`InterviewHubPage`) — 面试主页
- `features/interview/InterviewSessionPage.tsx` — 面试进行中会话页
- `features/interview/InterviewReportPage.tsx` — 面试报告页
- `features/interview/InterviewLive2DStage.tsx` — Live2D 面试场景子组件

### 3. 学习陪伴模块
- `features/companion/CompanionHubPage.tsx` — 学习陪伴主页
- `features/companion/CompanionWorkspacePage.tsx` — 学习陪伴工作区（独立房间）
- `features/companion/CompanionLive2DStage.tsx` — Live2D 陪伴场景子组件
- `features/companion/companionShared.tsx` — 共享子组件（GoalList、PhaseTimeline 等）

### 4. 登录页
- `router.tsx` 内联的 `LoginPage` 组件 — 仍使用 `className="page-panel narrow-panel"` 等旧样式

---

## 三、统一设计规范（已确立）

所有已重构页面遵循以下规范，未重构页面后续应参照执行。

### 3.1 样式体系
- **全部使用 inline styles**，不再新增 `styles.css` 类名
- 每个页面顶部定义 `THEME` 常量对象：
  ```ts
  const THEME = {
    primary: '#3b82f6',
    primaryLight: '#eff6ff',
    textPrimary: '#1f2937',
    textSecondary: '#6b7280',
    textTertiary: '#9ca3af',
    border: '#e5e7eb',
    borderLight: '#f3f4f6',
    bg: '#f8fafc',
    white: '#ffffff',
    radius: 12,
    radiusSm: 8,
    shadow: '0 1px 3px rgba(0,0,0,0.04), 0 1px 2px rgba(0,0,0,0.02)',
    shadowHover: '0 4px 12px rgba(0,0,0,0.06), 0 2px 4px rgba(0,0,0,0.04)',
    green: '#10b981',
    orange: '#f59e0b',
    red: '#ef4444',
    purple: '#8b5cf6',
  }
  ```

### 3.2 卡片规范
- 背景：`THEME.white`
- 边框：`1px solid ${THEME.border}`
- 圆角：`THEME.radius`（12px）
- 内边距：视内容密度用 `16px 20px` 或 `20px 24px`
- hover 效果：`boxShadow: THEME.shadowHover` + `borderColor: '#d1d5db'`

### 3.3 布局规范
- 页面容器：`maxWidth: 1200`, `margin: '0 auto'`, `padding: '24px 16px'`
- 双栏布局：`gridTemplateColumns: '1fr 300px'`, `gap: 24`
- 响应式：使用 `repeat(auto-fit, minmax(220px, 1fr))` 等 CSS Grid 弹性列

### 3.4 组件规范
- **全部使用 Ant Design v6** 组件，不再使用原生 `select` / `input` / `button`
- 按钮层级：主行动 `type="primary"`，次要行动默认 `type="default"`，危险操作 `danger`
- 图标：统一从 `@ant-design/icons` 导入，不用第三方图标库
- 空状态：统一使用 `<Empty image={Empty.PRESENTED_IMAGE_SIMPLE} />`
- 加载：统一使用 `<Spin />` + 必要时 `Skeleton`

### 3.5 排版规范
- 页面标题：`fontSize: 22`, `fontWeight: 700`, `color: THEME.textPrimary`
- 卡片标题：`fontSize: 15~16`, `fontWeight: 700`
- 正文：`fontSize: 13~14`, `color: THEME.textSecondary`
- 辅助文字：`fontSize: 12~13`, `color: THEME.textTertiary`

---

## 四、技术栈现状

| 依赖 | 版本 | 说明 |
|------|------|------|
| React | ^19.2.0 | 使用 type imports |
| TanStack Router | ^1.133.48 | 文件已配好懒加载 |
| TanStack Query | ^5.90.5 | 数据获取模式稳定 |
| Ant Design | ^6.4.4 | 全部 UI 组件来源 |
| Zustand | ^5.0.8 | 状态管理 |
| Monaco Editor | ^0.55.1 | 仅编程题编辑器使用 |

---

## 五、已知遗留问题

1. **`styles.css` 仍有 ~3000 行代码**，但所有已重构页面已完全脱离它。未重构页面仍依赖它，建议等全部重构完成后删除/清理。
2. **Chunk 体积警告**：`vite build` 提示 `index.js > 500KB`，这是 Monaco / PixiJS / PDF.js 等第三方库导致，非业务代码问题。后续可考虑 `manualChunks` 优化。
3. **Admin 面板**：`apps/admin` 下 TaxonomyPage、QuestionPage、Login 等已按玻璃拟态风格重构，与前台是独立的设计语言（后台保留 glassmorphism，前台走 LeetCode 简洁风）。
4. **题单封面图 `cover_image` 是预留字段**：数据库有字段、管理端表单可填写，但无实际上传服务（缺少 UploadImage RPC 和 OSS/MinIO 对接）。当前管理端表单已将该字段标记为「预留字段」并 disabled，前台也不展示封面图。未来若接入图片存储，需同时恢复表单启用和前台展示。

---

## 六、下一步建议（优先级排序）

1. **高优**：`PracticeDetailPages.tsx` 拆解重构 — 这是用户高频路径（做题、看题、编辑器）
2. **中优**：`InterviewPage.tsx` + `InterviewReportPage.tsx` — 面试报告页视觉冲击力强，值得重点设计
3. **中优**：`CompanionHubPage.tsx` — 学习陪伴是产品核心闭环
4. **低优**：`router.tsx` 内联 `LoginPage` 重写 + 彻底清理 `styles.css`
5. **低优**：`InterviewSessionPage.tsx` / `CompanionWorkspacePage.tsx` — 这两个是沉浸式独立页面（无导航栏），重构优先级相对较低
