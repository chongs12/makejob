---
description: MakeJob API 测试快捷命令。自动登录获取 JWT token 并执行 API 请求。支持 admin/debug 两种角色，解决每次手动复制 token 的重复劳动。
---

# API 测试命令

用于 MakeJob 项目的 API 接口测试。自动处理 JWT token 获取和请求发送。

## 用法

```
$ARGUMENTS
```

参数格式：`<方法> <路径> [角色=admin] [请求体]`

示例：
- `GET /api/v1/questions` → 用 admin token 请求题库列表
- `GET /api/v1/questions page=1 page_size=3` → 带查询参数
- `POST /api/v1/admin/configs admin {"key":"value"}` → admin POST 请求
- `GET /api/v1/questions debug` → 用 debug(free_member) token
- `POST /api/v1/auth/login public {"email":"admin@makejob.com","password":"admin123456"}` → 无需 token 的公开接口

## 执行流程

1. **确定角色和 token**：
   - `admin` 角色（默认）：使用 `admin@makejob.com / admin123456`
   - `debug` 角色：使用 `test@test.com / test123`（free_member）
   - `public` 角色：不获取 token，直接请求

2. **登录获取 JWT token**：
   ```powershell
   $loginResp = Invoke-RestMethod -Uri "http://localhost:8082/api/v1/auth/login" -Method Post -ContentType "application/json" -Body '{"email":"admin@makejob.com","password":"admin123456"}'
   $token = $loginResp.data.access_token
   ```

3. **发送 API 请求**：
   ```powershell
   $headers = @{Authorization="Bearer $token"}
   $resp = Invoke-RestMethod -Uri "http://localhost:8082/api/v1/questions?page=1&page_size=3" -Headers $headers
   $resp | ConvertTo-Json -Depth 10
   ```

## 注意事项

- Gateway 地址固定为 `http://localhost:8082`
- 如果登录失败（500/连接拒绝），先确认 Gateway 是否运行：`netstat -ano | findstr :8082`
- JWT token 有有效期（通常 24 小时），过期后需重新登录
- 响应格式：成功时 `resp.data` 为业务数据，失败时 `resp.error` 为错误信息
