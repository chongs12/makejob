#!/usr/bin/env bash
# e2e trace 验证脚本：注册->登录->调 interviews，断言 Jaeger 收到跨服务 trace。
# 对应 docs/observability-k8s-checklist.md 3.5。
#
# 前置：observability 栈(Collector+Jaeger) + gateway + interview + user 服务已启动。
# 用法：bash scripts/e2e-trace-check.sh
#   可选环境变量：GATEWAY(http://localhost:8082) JAEGER(http://localhost:16686)
set -euo pipefail

GATEWAY="${GATEWAY:-http://localhost:8082}"
JAEGER="${JAEGER:-http://localhost:16686}"
TS="$(date +%s)"
USERNAME="e2e${TS}"
EMAIL="e2e-trace${TS}@test.com"
PASSWORD="Test123456!"

echo "=== e2e trace 验证 ==="
echo "gateway: $GATEWAY  jaeger: $JAEGER"
echo "测试用户: $EMAIL"

# 1. 注册
echo "--- 注册 ---"
HTTP=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$GATEWAY/api/v1/auth/register" \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"$USERNAME\",\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}")
echo "register: $HTTP"
[ "$HTTP" = "200" ] || { echo "✗ 注册失败"; exit 1; }

# 2. 登录获取 JWT
echo "--- 登录 ---"
LOGIN=$(curl -s -X POST "$GATEWAY/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}")
TOKEN=$(echo "$LOGIN" | grep -oE '"access_token":"[^"]+"' | sed 's/"access_token":"//;s/"//')
[ -n "$TOKEN" ] || { echo "✗ 登录未返回 access_token: $LOGIN"; exit 1; }
echo "login: 获取 JWT (${#TOKEN} 字符)"

# 3. 调 interviews（gateway -> interview 跨服务）
echo "--- 调 interviews ---"
HTTP=$(curl -s -o /dev/null -w '%{http_code}' -H "Authorization: Bearer $TOKEN" "$GATEWAY/api/v1/interviews")
echo "GET /api/v1/interviews: $HTTP"

# 4. 等 trace 导出（Collector batch + Jaeger 索引）
echo "--- 等 trace 导出(4s) ---"
sleep 4

# 5. 断言 Jaeger services
echo "--- 断言 Jaeger services ---"
SERVICES=$(curl -s "$JAEGER/api/services")
FAIL=0
for svc in makejob.gateway makejob.interview makejob.user; do
  if echo "$SERVICES" | grep -q "$svc"; then
    echo "✓ $svc"
  else
    echo "✗ $svc 缺失"
    FAIL=1
  fi
done

# 6. 断言 gateway trace 含跨服务 span（span 数 >= 2）
echo "--- 断言 gateway 跨服务 trace ---"
TRACES=$(curl -s "$JAEGER/api/traces?service=makejob.gateway&limit=5&lookback=1h")
echo "$TRACES" | python -c "
import sys, json
d = json.load(sys.stdin)
found = False
for t in d.get('data', []):
    ops = [s['operationName'] for s in t.get('spans', [])]
    if len(ops) >= 2:
        found = True
        print(f'✓ gateway trace 含 {len(ops)} span: {ops}')
        break
if not found:
    print('✗ gateway 无跨服务 trace（span 数 < 2）')
    sys.exit(1)
" || FAIL=1

echo ""
if [ "$FAIL" = "0" ]; then
  echo "=== ✓ e2e trace 验证通过 ==="
  exit 0
else
  echo "=== ✗ e2e trace 验证失败 ==="
  exit 1
fi
