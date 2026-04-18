后端: cd d:/gogogo/makejob/backend && go run cmd/server/main.go（需先配置 PostgreSQL 和 Redis）
前端: cd d:/gogogo/makejob/frontend-react && npm install && npm run dev -w @makejob/web
后台管理: cd d:/gogogo/makejob/frontend-react && npm run dev -w @makejob/admin
说明: 旧 Nuxt 前端已废弃，当前统一以 React + Vite 工作区为准
