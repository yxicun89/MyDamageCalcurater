#!/usr/bin/env bash
# TiDB Operator v1.6.6 の CRD 本体を k3d クラスタへ導入する(ADR-0211 §3.1)。冪等
# (`kubectl apply --server-side` と `helm upgrade --install` を使うため、既存クラスタに対して
# 何度実行しても安全)。呼び出し側(scripts/up.sh)は、このスクリプトの失敗を非致命として扱う
# (6レーン共通の `make up` を、record/team に無関係な理由で止めないため。ADR-0211 §3.1)。
set -euo pipefail

TIDB_OPERATOR_VERSION="v1.6.6"

command -v kubectl >/dev/null || { echo "tidb-operator-bootstrap: kubectl が見つからない" >&2; exit 1; }
command -v helm >/dev/null || { echo "tidb-operator-bootstrap: helm が見つからない" >&2; exit 1; }

echo "tidb-operator-bootstrap: CRD を適用する(server-side apply。冪等)..."
kubectl apply --server-side \
  -f "https://raw.githubusercontent.com/pingcap/tidb-operator/${TIDB_OPERATOR_VERSION}/manifests/crd.yaml"

echo "tidb-operator-bootstrap: helm repo を登録する..."
helm repo add pingcap https://charts.pingcap.org/ >/dev/null 2>&1 || true
helm repo update pingcap >/dev/null

echo "tidb-operator-bootstrap: tidb-operator ${TIDB_OPERATOR_VERSION} を導入する(helm upgrade --install。冪等)..."
helm upgrade --install tidb-operator pingcap/tidb-operator \
  --namespace=tidb-admin --create-namespace --version="$TIDB_OPERATOR_VERSION" \
  --set operatorImage="pingcap/tidb-operator:${TIDB_OPERATOR_VERSION}" \
  --set scheduler.create=false \
  --wait --timeout=180s

echo "tidb-operator-bootstrap: 完了"
