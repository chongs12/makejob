---
description: MakeJob 微服务构建 + 重启快捷命令。编译指定 Go 服务，终止旧进程，启动新进程，验证端口监听。解决每次手动 kill/build/start 的重复劳动。
---

# 微服务构建重启命令

用于 MakeJob 项目的 Go 微服务开发调试。自动处理构建、进程管理和健康检查。

## 用法

```
$ARGUMENTS
```

参数格式：`<服务名> [端口号]`

示例：
- `companion` → 构建并重启 companion 服务（端口 9003）
- `gateway 8082` → 构建并重启 gateway（端口 8082）
- `interview` → 构建并重启 interview 服务（端口 9004）
- `question` → 构建并重启 question 服务（端口 9003）
- `ai_gateway` → 构建并重启 ai_gateway（端口 9011）

## 服务端口映射

| 服务 | 端口 | 构建路径 |
|------|------|----------|
| gateway | 8082 | `./app/gateway/cmd/server/` 或 `./app/gateway/...` |
| companion | 9003 | `./app/companion/...` |
| interview | 9004 | `./app/interview/...` |
| question | 9003 | `./app/question/...` |
| ai_gateway | 9011 | `./app/ai_gateway/...` |
| realtime | 9005 | `./app/realtime/...` |
| rag | 9006 | `./app/rag/...` |
| plan | 9007 | `./app/plan/...` |
| learning_archive | 9008 | `./app/learning_archive/...` |
| community | 9009 | `./app/community/...` |
| user | 9101 | `./app/user/...` |
| membership | 9002 | `./app/membership/...` |
| admin | 9010 | `./app/admin/...` |
| growth | 9012 | `./app/growth/...` |

## 执行流程

1. **构建服务**：
   ```powershell
   go build ./app/<service>/... 2>&1
   ```
   构建失败则中止，不执行后续步骤。

2. **终止旧进程**：
   ```powershell
   Get-Process | Where-Object {$_.ProcessName -eq "<service>"} | Stop-Process -Force -ErrorAction SilentlyContinue
   ```
   或通过路径匹配：
   ```powershell
   Get-Process | Where-Object {$_.Path -like "*<service>*"} | Stop-Process -Force -ErrorAction SilentlyContinue
   ```
   等待 2 秒确保端口释放。

3. **启动新进程**：
   ```powershell
   Start-Process -FilePath "go" -ArgumentList "run", "./cmd/server" -WorkingDirectory "D:\gogogo\makejob\app\<service>" -WindowStyle Hidden
   ```
   Gateway 服务特殊处理：使用预编译的 `gateway.exe`。
   ```powershell
   go build -o gateway.exe ./app/gateway/cmd/server/
   Start-Process -FilePath "D:\gogogo\makejob\gateway.exe" -WorkingDirectory "D:\gogogo\makejob\app\gateway" -WindowStyle Hidden
   ```

4. **验证端口**：
   ```powershell
   Start-Sleep -Seconds 3
   netstat -ano | findstr :<port>
   ```
   端口监听正常则输出成功，否则提示检查日志。

## 注意事项

- 工作目录固定为 `D:\gogogo\makejob`
- 所有后台进程使用 `-WindowStyle Hidden` 避免弹窗
- 如果端口被占用但进程名不匹配，用 `netstat -ano | findstr :<port>` 找 PID 再 `Stop-Process -Id <PID> -Force`
- Gateway 需要单独编译为 exe（不能直接 `go run`），其他服务可以 `go run`
