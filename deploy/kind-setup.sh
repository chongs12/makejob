#!/usr/bin/env bash
# kind 集群一键创建 + 依赖安装脚本
# 对应 docs/observability-k8s-checklist.md 5.1
set -euo pipefail

CLUSTER_NAME="${CLUSTER_NAME:-makejob}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "=== 1. 创建 kind 集群 ==="
if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
  echo "集群 ${CLUSTER_NAME} 已存在，跳过创建"
else
  kind create cluster --name "$CLUSTER_NAME" --config "$SCRIPT_DIR/kind-cluster.yaml"
fi

# 设置 kubectl context
kubectl config use-context "kind-${CLUSTER_NAME}"

echo "=== 2. 安装 ingress-nginx ==="
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml
kubectl wait --namespace ingress-nginx \
  --for=condition=ready pod \
  --selector=app.kubernetes.io/component=controller \
  --timeout=120s

echo "=== 3. 安装 metrics-server（HPA 依赖）==="
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
# metrics-server 在 kind 里需要 --kubelet-insecure-tls（kind 节点用自签证书）
kubectl patch deployment metrics-server -n kube-system \
  --type='json' -p='[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--kubelet-insecure-tls"}]'

echo "=== 4. 加载本地镜像到 kind ==="
SERVICES="gateway user membership question interview realtime growth plan companion community learning_archive ai_gateway rag coderunner admin"
for svc in $SERVICES; do
  echo "  loading makejob/$svc ..."
  kind load docker-image "makejob/$svc" --name "$CLUSTER_NAME" 2>/dev/null || echo "  (skip: makejob/$svc 未构建)"
done
echo "  loading makejob/frontend ..."
kind load docker-image "makejob/frontend" --name "$CLUSTER_NAME" 2>/dev/null || echo "  (skip: makejob/frontend 未构建)"

echo "=== 5. 部署基础设施（PostgreSQL/Redis/RabbitMQ/etcd/Milvus/MinIO）==="
kubectl apply -f "$SCRIPT_DIR/infra/" 2>/dev/null || echo "  (infra/ 目录不存在，跳过；可用根目录 docker-compose 起基础设施)"

echo "=== 集群就绪 ==="
kubectl get nodes
echo ""
echo "下一步: helm install makejob deploy/helm/makejob/"
