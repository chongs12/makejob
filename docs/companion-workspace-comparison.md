# Open-LLM-VTuber-Web vs MakeJob CompanionWorkspace 架构对比

## 1. 路由/页面层级结构

| 维度 | Open-LLM-VTuber-Web | MakeJob CompanionWorkspace |
|------|---------------------|---------------------------|
| 路由方案 | 无路由，单页应用 | TanStack Router，`/companion` → `/companion/room` |
| 模式切换 | `ModeProvider` 提供 window/pet 两种模式，条件渲染不同 UI | 无模式概念，入口页和工作区是两个独立路由 |
| 布局结构 | `App` → 14 层 Provider 嵌套 → `AppContent`（Live2D 背景 + Sidebar + Footer 叠加） | `RootLayout`（导航栏 + `<Outlet>`）→ 子页面各自独立 |
| UI 分区 | 左侧 Sidebar（440px）+ 右侧 Live2D 画布 + 底部 Footer（120px），三区域固定 | 左侧边栏（计划/目标/历史）+ 右侧舞台区（Live2D + 输入），双栏布局 |

## 2. 状态管理

| 维度 | Open-LLM-VTuber-Web | MakeJob CompanionWorkspace |
|------|---------------------|---------------------------|
| 整体方案 | 14 个 React Context 嵌套 Provider | Zustand（auth）+ TanStack Query（服务端）+ useState（本地） |
| AI 状态 | `AiStateContext`：枚举状态机 IDLE/THINKING_SPEAKING/INTERRUPTED/LISTENING/WAITING | 无状态机，`sending` boolean 控制 |
| 对话历史 | `ChatHistoryContext`：支持多轮会话切换，`historyList` 管理会话列表 | 单轮 `CompanionHistoryItem[]` useState，无多轮切换 |
| Live2D 配置 | `Live2DConfigContext`：`modelInfo` 对象（url/scale/emotionMap/tapMotions）+ localStorage 持久化 | `readSelectedLive2DModelKey()` 读 localStorage，props 逐层传递 |
| 音频任务 | `audioTaskQueue` 工具类 + `useAudioTask` hook 管理播放队列 | 无音频队列，TTS 由后端控制 |
| 微信/语音 | `VADContext`：Voice Activity Detection，麦克风开关 + 自动启动配置 | 无 VAD，纯文本输入 |
| 字幕 | `SubtitleContext`：独立管理字幕文本和显示状态 | 对话内容直接显示在聊天列表 |

## 3. UI 组件拆分方式

| 维度 | Open-LLM-VTuber-Web | MakeJob CompanionWorkspace |
|------|---------------------|---------------------------|
| 拆分粒度 | 细粒度：`components/canvas/`（6 个文件）、`components/sidebar/`（12 个文件）、`components/footer/`、`components/ui/` | 粗粒度：单文件 924 行，所有逻辑堆在一个组件 |
| hooks 抽取 | `hooks/canvas/`（6 个）、`hooks/utils/`（5 个）、`hooks/sidebar/`、`hooks/footer/` | 仅 `useCompanionStudyLogSync` 一个自定义 hook |
| 样式分离 | `sidebar-styles.tsx`、`canvas-styles.tsx` 独立样式文件 | 无样式分离，className 引用全局 `styles.css` |
| 容器/展示 | `WebSocketHandler` 作为容器组件消费所有 Context，子组件只负责展示 | 无分离，数据获取 + 业务逻辑 + 渲染全在一个文件 |

## 4. 样式方案

| 维度 | Open-LLM-VTuber-Web | MakeJob CompanionWorkspace |
|------|---------------------|---------------------------|
| 框架 | **Chakra UI v3** + Emotion | 无框架，全局 `styles.css`（~3000 行） |
| 写法 | Chakra 的 `Box`/`Flex` 组件 + style props（`bg`, `p`, `gap` 等） | 原生 HTML + `className` 引用 CSS 类 |
| 主题 | Chakra 内置 token（`gray.800`, `whiteAlpha.200` 等） | 无统一 token，颜色值硬编码在 CSS 中 |
| 响应式 | Chakra 响应式语法 `{ base: 'column', md: 'row' }` | 无响应式，固定布局 |
| 动画 | `framer-motion` 库 | CSS transition |

## 5. Live2D 初始化/销毁/动作触发

| 维度 | Open-LLM-VTuber-Web | MakeJob CompanionWorkspace |
|------|---------------------|---------------------------|
| SDK | Cubism SDK（WebSDK 内嵌），直接调用 `LAppDelegate`/`LAppLive2DManager` | `pixi-live2d-display` 库 + `Live2DCubismCore` 动态脚本加载 |
| 初始化 | `useLive2DModel` hook：解析 model URL → `updateModelConfig` → `initializeLive2D` → canvas 绑定 | `Live2DSceneStage` 组件：`createLive2DStageRuntime()` → Pixi Application 创建 → 模型加载 |
| 销毁 | 组件卸载时由 React effect cleanup 处理 | `destroyLive2DStageRuntime()` 手动销毁 Pixi Application |
| 动作触发 | `useLive2DExpression` hook：通过 `LAppAdapter` 调用 `setExpression`/`startMotion` | `applyLive2DStageVisualState()` 函数：直接操作 Pixi 模型的 expression/motion |
| 唇 sync | 音频驱动：`_wavFileHandler.start()` 绑定音频文件，模型自动口型 | `mouthOpen` prop 驱动：外部计算音量 → 逐帧设置口型参数 |
| 拖拽交互 | `useLive2DModel` 内处理 pointer events，区分 tap/drag（阈值判断） | `Live2DSceneStage` 内处理 pointer events，move/scale 两种模式 |
| 模型切换 | `Live2DConfigContext.setModelInfo()` → 触发 effect 重新加载 | `onSelectModelKey` 回调 → `useEffect` 销毁旧 runtime + 创建新 runtime |
| 背景 | `Background` 独立组件，`BgUrlContext` 管理 URL | `backgroundImageUrl` prop 传入 `Live2DSceneStage` |

---

## 对方最值得借鉴的 3 个结构亮点

### 1. Context 分层解耦，每个关注点独立

Open-LLM-VTuber-Web 将 14 个关注点拆成独立 Context（AI 状态、Live2D 配置、对话历史、VAD、字幕等），每个 Context 只管自己的状态和 setter。MakeJob 的 `CompanionWorkspacePage` 把所有状态堆在一个 924 行的组件里（16 个 useState + 11 个 useQuery + 2 个 useMutation），导致数据流难以追踪。

**借鉴点**：至少拆出 `CompanionChatContext`（对话历史 + 发送状态）和 `CompanionPlanContext`（计划 + 任务 + 进度），让舞台组件和侧边栏各自消费需要的 Context，而不是通过 20+ props 逐层传递。

### 2. Live2D 逻辑完全封装在 hooks 层

Open-LLM-VTuber-Web 把 Live2D 的初始化（`useLive2DModel`）、尺寸适配（`useLive2DResize`）、表情控制（`useLive2DExpression`）、音频播放（`useAudioTask`）全部拆成独立 hooks，`Live2D` 组件只负责组合这些 hooks 和渲染 canvas。MakeJob 的 `Live2DSceneStage` 是一个 415 行的单文件，runtime 创建/销毁/交互/状态全混在一起。

**借鉴点**：把 `Live2DSceneStage` 拆成 `useLive2DStageRuntime`（初始化/销毁）、`useLive2DStageInteraction`（拖拽/缩放）、`useLive2DStageExpression`（表情/动作）三个 hooks，组件只做组合和渲染。

### 3. WebSocketHandler 作为独立服务层

Open-LLM-VTuber-Web 的 `WebSocketHandler` 是一个纯容器组件，不渲染任何 UI，只负责：接收 WS 消息 → 分发到对应 Context → 触发状态变更。这让 WebSocket 逻辑与 UI 完全解耦。MakeJob 的对话逻辑（`sendCompanionChatRequest` + 历史管理 + 状态更新）全部内联在页面组件的事件处理函数里。

**借鉴点**：抽取 `CompanionChatService`（或 hook），统一管理对话请求发送、历史追加、情绪/动作解析、错误处理，页面组件只调用 `sendMessage(text)` 和读取 `history`。
