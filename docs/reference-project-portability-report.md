# 参考项目结构可移植性评估报告

> 评估对象：Open-LLM-VTuber-Web（参考项目）
> 目标项目：MakeJob CompanionWorkspace（学习陪伴工作区）
> 核心业务差异：参考项目是**纯展示型 Live2D 看板娘 + 聊天框**；目标项目是**Live2D 面试问答**（AI 出题、用户语音/文字回答、情绪驱动动作切换、计时器）

---

## 一、✅ 通用可移植（直接借鉴）

### 1. Context 分层解耦模式

**参考项目做法：** 14 个独立 Context 各管各的——`AiStateContext` 管 AI 状态机、`ChatHistoryContext` 管对话历史、`Live2DConfigContext` 管模型配置、`SubtitleContext` 管字幕。每个 Context 只暴露 state + setter，消费方按需 useContext。

**为什么通用：** 这是 React 社区公认的状态管理最佳实践，与业务复杂度无关。MakeJob 的 CompanionWorkspacePage 把 13 个 useState + 4 个 useQuery + 2 个 useMutation 全堆在一个 924 行组件里，任何状态变更都要在巨量代码中定位——这正是 Context 分层要解决的问题。

**移植方案：** 至少拆出 3 个 Context：
- `CompanionChatContext`：history / composer / sending / chat mutation
- `CompanionPlanContext`：currentPlanQuery / planProgressQuery / task mutations / feedback 状态
- `CompanionStageContext`：stageEnabled / modelKey / dialogue / emotion / mouthOpen

### 2. Live2D 逻辑封装在 hooks 层

**参考项目做法：** `useLive2DModel`（初始化/销毁）、`useLive2DResize`（尺寸适配）、`useLive2DExpression`（表情控制）各司其职，`Live2D` 组件只做组合和渲染 canvas。

**为什么通用：** 将副作用逻辑从组件中抽离是 React hooks 的核心价值。MakeJob 的 `Live2DSceneStage`（415 行）和 `live2dStageRuntime.ts`（835 行）把模型加载、参数动画、动作节流、表达式混合全混在一起，任何改动都需要理解整个文件。

**移植方案：** 拆成：
- `useLive2DStageRuntime`：创建/销毁 Pixi Application，管理 disposed 状态
- `useLive2DStageAnimator`：参数平滑插值（ticker 回调）
- `useLive2DStageExpression`：表达式加载/混合/切换
- `useLive2DStageInteraction`：拖拽/缩放/视线跟随

### 3. 串行异步任务队列

**参考项目做法：** `TaskQueue` 类维护 `(() => Promise<void>)[]`，串行执行，互斥锁保证同一时刻只有一个任务在跑。

**为什么通用：** MakeJob 的面试场景同样需要串行处理：AI 出题 → 用户回答 → 后端评分 → Live2D 播放反馈动画 → 下一题。当前这些步骤散落在不同的 useEffect 和事件处理函数里，没有统一的流程控制。

**移植方案：** 移植 `TaskQueue` 核心逻辑，修复竞态缺陷（见下方 ⚠️ 部分），用于面试问答流程控制。

### 4. AI 状态机枚举

**参考项目做法：** `AiStateEnum` 定义 5 种状态（IDLE / THINKING_SPEAKING / INTERRUPTED / LISTENING / WAITING），状态转换由 Context 的 setter 统一管理。

**为什么通用：** MakeJob 的面试流程同样有明确的状态：等待出题 / 用户作答中 / 后端评分中 / Live2D 反馈播放中 / 下一题。当前用多个 boolean（`sending`、`isRunning`）组合表达，容易出现非法状态（如 sending=true 且 isRunning=true）。

**移植方案：** 定义 `InterviewStateEnum`：`IDLE` / `QUESTION_LOADING` / `ANSWERING` / `GRADING` / `FEEDBACK_PLAYING` / `COMPLETED`，用 Context 管理状态转换。

### 5. localStorage 统一防御模式

**参考项目做法：** 所有 localStorage 读写函数开头检查 `typeof window === 'undefined'`，读取时 try-catch 解析失败自动清理。

**为什么通用：** SSR 兼容和防御性编程是通用需求。MakeJob 的 `companionStorage.ts` 已有类似模式但每个函数重复模板代码。

**移植方案：** 封装 `createLocalStorageAccessor<T>(key, options)` 工厂函数，内置 SSR 兼容、JSON 序列化、版本校验、过期清理。

---

## 二、⚠️ 需要改造（对方业务简单才成立）

### 1. 14 层 Context 嵌套 Provider

**参考项目做法：** `App.tsx` 中 14 个 Provider 层层嵌套包裹 `AppContent`。

**为什么简单才成立：** 参考项目是单页应用无路由，所有组件都在同一棵 Provider 树下。MakeJob 用 TanStack Router，每个路由页面有自己的组件树——如果也用 14 层嵌套，每个路由入口都要重复这套 Provider 树，维护成本极高。

**改造方案：** 不要照搬 14 层嵌套。用 Zustand 替代大部分 Context（参考项目自己也用了 Zustand 但只用于简单场景）：
- `useCompanionChatStore`（Zustand）：对话历史 + composer + sending
- `useCompanionPlanStore`（Zustand）：计划 + 任务 + 进度
- `useCompanionStageStore`（Zustand）：舞台状态
- 服务端数据继续用 TanStack Query

### 2. WebSocketHandler 作为"消息路由器"

**参考项目做法：** `WebSocketHandler` 是一个纯容器组件，消费所有 Context，接收 WS 消息后 switch 分发到各 Context 的 setter。

**为什么简单才成立：** 参考项目只有 WS 一种通信方式，消息类型有限（音频、控制、模型更新、聊天历史）。MakeJob 的面试模块需要 REST + WebSocket + SSE 多种通信方式，且面试消息有复杂的业务含义（出题、回答、评分、诊断），不能用简单的 switch 分发。

**改造方案：** 抽取 `useInterviewChat` hook，内部根据消息类型调用不同的处理函数（而不是一个巨大的 switch）。REST 用于请求-响应式调用，SSE 用于流式输出（如果需要打字机效果），WS 保留给实时互动场景。

### 3. ChatHistoryContext 的多轮会话管理

**参考项目做法：** `historyList` 管理多个会话，`currentHistoryUid` 切换当前会话，`messages` 只显示当前会话的消息。

**为什么简单才成立：** 参考项目的"会话"只是聊天记录的集合，没有业务状态。MakeJob 的"面试会话"有完整的生命周期（created → ongoing → report_generating → completed），每个会话关联计划、任务、报告、Live2D 模型配置——这不是一个简单的消息列表能承载的。

**改造方案：** 用 TanStack Query 管理会话数据（已有 `currentPlanQuery`），不引入独立的 HistoryContext。对话历史作为 plan 的附属数据通过 query 获取，而不是独立管理。

### 4. Audio Task Queue 的 20ms 间隔

**参考项目做法：** `audioTaskQueue` 用 20ms 间隔串行播放 AI 语音片段，实现无缝拼接。

**为什么简单才成立：** 参考项目的音频全部来自后端 TTS，格式统一，播放顺序就是对话顺序。MakeJob 的面试场景中，音频可能来自 TTS（AI 出题）或用户录音回放，且需要与 Live2D 口型同步、与计时器联动——20ms 的固定间隔不够灵活。

**改造方案：** 移植 TaskQueue 的串行机制，但移除固定间隔，改为每个任务自行控制完成时机（通过 Promise resolve）。增加超时机制防止队列永久阻塞。

### 5. AiStateContext 的自动回退定时器

**参考项目做法：** `WAITING` 状态下 2 秒自动回退到 `IDLE`。

**为什么简单才成立：** 参考项目的"等待"只是用户在打字，2 秒无输入就认为用户不操作了。MakeJob 的面试场景中，用户可能在思考一道难题 30 秒以上——2 秒超时会导致 AI 误判用户放弃。

**改造方案：** 状态超时时间可配置，面试场景下 WAITING 超时设为 60 秒或更长，或完全由后端控制状态转换（通过 WebSocket 消息）。

---

## 三、❌ 坚决避开（致命缺陷）

### 1. Live2D 实例挂在 window 全局变量上

**参考项目做法：** `window.LAppLive2DManager`、`window.getLAppAdapter()`、`window.testSetExpression`——Live2D SDK 的核心实例通过 window 全局变量暴露，任何代码都能直接访问和修改。

**致命问题：**
- 绕过 React 状态管理，组件无法感知 Live2D 实例的变化
- 类型不安全（`window as any` 到处都是）
- 多实例冲突（如果页面需要两个 Live2D 角色）
- React Strict Mode 下行为不可预测
- 调试困难——变更来源不可追踪

**MakeJob 现状：** `live2dStageRuntime.ts` 用 `Live2DStageRuntime` 对象封装实例，通过 `useRef` 持有——虽然也有"上帝对象"问题，但至少在 React 生命周期内。不要改成 window 全局变量。

### 2. 无 Error Boundary

**参考项目做法：** 整个应用没有任何 Error Boundary。任何组件渲染异常都会导致白屏，用户只能刷新页面。

**致命问题：** Live2D 渲染、WebSocket 消息处理、音频播放都是高概率出错的异步操作。没有 Error Boundary 意味着一次网络抖动或一个模型加载失败就会让整个页面崩溃。

**MakeJob 现状：** 已有 `SectionErrorBoundary` 组件，将侧边栏和舞台区隔离——这是正确的做法，应该继续保持并扩展。

### 3. Audio 元素事件监听器未清理

**参考项目做法：** `use-audio-task.ts` 和 `use-live2d-model.ts` 中创建 `new Audio()` 后绑定 `canplaythrough`/`ended`/`error` 监听器，但中断场景下从不调用 `removeEventListener`。

**致命问题：** 在面试场景中，用户可能在 AI 出题过程中点击"跳过"或"下一题"，此时旧的 Audio 元素被 `pause()` 但监听器闭包仍持有旧的 model 引用和 context 函数引用。高频操作下会造成内存峰值和 stale closure 问题。

**MakeJob 防范：** 如果移植音频播放逻辑，必须在 cleanup 中显式 `removeEventListener` 并将 Audio 元素引用置 null。

### 4. TaskQueue.clearQueue 的竞态条件

**参考项目做法：** `clearQueue` 将 `running` 设为 false，但如果此时 `runNextTask` 正在 await 一个正在执行的任务，`clearQueue` 后的下一个 `addTask` 会触发新的 `runNextTask`，与尚未完成的旧任务并行执行。

**致命问题：** 在面试场景中，如果用户快速点击"提交答案"和"跳过"，两个操作可能同时入队并行执行，导致答案提交和题目跳过同时发生，破坏面试状态一致性。

**MakeJob 防范：** 如果移植 TaskQueue，需要在 `clearQueue` 中等待当前任务完成后再重置 `running` 标志。

### 5. WebSocket 服务全局单例永不销毁

**参考项目做法：** `WebSocketService` 用静态 `instance` 模式，模块加载时创建，生命周期等同于整个应用。

**致命问题：** MakeJob 的面试页面和陪伴页面是不同路由，如果 WS 连接在面试页面建立后切换到陪伴页面，旧的 WS 连接和消息处理器仍然存在——面试的消息可能被错误地分发到陪伴页面的 Context。

**MakeJob 防范：** 如果引入 WS，必须按路由/功能域管理连接生命周期，页面卸载时断开连接。

---

## 四、总结矩阵

| 模式/实践 | 可移植性 | 理由 |
|-----------|---------|------|
| Context 分层解耦 | ✅ 直接移植 | 通用最佳实践，解决 MakeJob 924 行单文件的核心痛点 |
| Live2D hooks 封装 | ✅ 直接移植 | 将 835 行 runtime 拆成可测试的独立 hooks |
| 串行异步任务队列 | ✅ 移植 + 修竞态 | 面试问答流程需要串行控制，TaskQueue 核心逻辑可用 |
| AI 状态机枚举 | ✅ 直接移植 | 替代多个 boolean 组合，防止非法状态 |
| localStorage 防御模式 | ✅ 直接移植 | 封装工厂函数消除重复模板 |
| 14 层 Provider 嵌套 | ⚠️ 改用 Zustand | 单页应用才适合，多路由项目应用 Zustand store |
| WebSocketHandler 路由 | ⚠️ 改为 hook 模式 | 面试场景消息类型更复杂，需要类型安全的分发 |
| 多轮会话 HistoryContext | ⚠️ 用 TanStack Query 替代 | 面试会话有完整生命周期，不是简单消息列表 |
| Audio 20ms 固定间隔 | ⚠️ 改为自控完成时机 | 面试场景需要更灵活的时序控制 |
| WAITING 2 秒超时 | ⚠️ 可配置化 | 面试用户思考时间远超 2 秒 |
| window 全局变量挂实例 | ❌ 坚决避开 | 绕过 React 生命周期，类型不安全，多实例冲突 |
| 无 Error Boundary | ❌ 坚决避开 | 一次异常导致白屏，MakeJob 已有正确实践 |
| Audio 监听器不清理 | ❌ 坚决避开 | 中断场景内存泄漏 + stale closure |
| TaskQueue 竞态 | ❌ 坚决避开 | 高频操作破坏状态一致性 |
| WS 全局单例 | ❌ 坚决避开 | 路由切换时消息泄漏到错误页面 |

---

## 五、MakeJob 面试场景特有挑战（参考项目未涉及）

| 挑战 | 参考项目有无 | 说明 |
|------|-------------|------|
| 计时器与 Live2D 联动 | ❌ 无 | 面试倒计时需要与 AI 状态机和 Live2D 表情联动（如时间紧迫时角色表情变化） |
| 语音识别（ASR）集成 | ⚠️ 有 VAD 但不同 | 参考项目的 VAD 用于检测用户是否在说话，MakeJob 需要完整的 ASR 转文字 |
| 编程题 Monaco Editor | ❌ 无 | 面试中的编程题需要嵌入代码编辑器，与 Live2D 舞台共存 |
| 多题型切换 | ❌ 无 | 选择题 / 主观题 / 编程题的 UI 和交互完全不同 |
| 面试报告生成 | ❌ 无 | 面试结束后需要异步生成报告，期间页面需要等待状态 |
| 诊断数据可视化 | ❌ 无 | 编程题的过程诊断需要展示代码执行轨迹 |
