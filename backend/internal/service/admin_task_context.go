package service

import "context"

type adminAsyncTaskContextKey string

const adminAsyncTaskIDContextKey adminAsyncTaskContextKey = "admin_async_task_id"

// withAsyncTaskID 将异步任务 ID 写入上下文，供后续 AI 调试与运行日志链路统一复用。
func withAsyncTaskID(ctx context.Context, taskID uint) context.Context {
	if taskID == 0 {
		return ctx
	}
	return context.WithValue(ctx, adminAsyncTaskIDContextKey, taskID)
}

// resolveAIDebugTaskID 优先使用显式请求里的任务 ID，否则回退到上下文中的异步任务 ID。
func resolveAIDebugTaskID(ctx context.Context, requestTaskID *uint) *uint {
	if requestTaskID != nil && *requestTaskID > 0 {
		return requestTaskID
	}
	if ctx == nil {
		return nil
	}

	taskID, ok := ctx.Value(adminAsyncTaskIDContextKey).(uint)
	if !ok || taskID == 0 {
		return nil
	}

	resolved := taskID
	return &resolved
}
