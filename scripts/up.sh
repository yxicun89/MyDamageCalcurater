#!/usr/bin/env bash
# k3d クラスタを作成(なければ)し、ローカル overlay を適用する。
# サービスの Deployment はフェーズ進行に応じて base/kustomization.yaml へ追加される。
set -euo pipefail
cd "$(dirname "$0")/.."

CLUSTER="${CLUSTER:-pokecalc}"

if k3d cluster list 2>/dev/null | awk '{print $1}' | grep -qx "$CLUSTER"; then
  echo "k3d クラスタ '$CLUSTER' は既に存在します"
else
  echo "k3d クラスタ '$CLUSTER' を作成します..."
  k3d cluster create --config deploy/k3d.yaml
fi

echo "Kustomize(overlays/local)を適用します..."
kubectl apply -k deploy/k8s/overlays/local

echo
echo "完了。gateway 実装後は http://localhost:8080 で計算画面にアクセスできます。"
