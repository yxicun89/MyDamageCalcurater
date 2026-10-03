#!/usr/bin/env bash
# TiDB Operator v1.6.6 の CRD 本体を k3d クラスタへ導入する(ADR-0211 §3.1)。冪等
# (`kubectl apply --server-side` と `helm upgrade --install` を使うため、既存クラスタに対して
# 何度実行しても安全)。呼び出し側(scripts/up.sh)は、このスクリプトの失敗を非致命として扱う
# (6レーン共通の `make up` を、record/team に無関係な理由で止めないため。ADR-0211 §3.1)。
set -euo pipefail

TIDB_OPERATOR_VERSION="v1.6.6"

command -v kubectl >/dev/null || { echo "tidb-operator-bootstrap: kubectl が見つからない" >&2; exit 1; }
command -v curl >/dev/null || { echo "tidb-operator-bootstrap: curl が見つからない" >&2; exit 1; }
command -v helm >/dev/null || { echo "tidb-operator-bootstrap: helm が見つからない" >&2; exit 1; }

echo "tidb-operator-bootstrap: CRD を適用する(server-side apply。冪等)..."
kubectl apply --server-side \
  -f "https://raw.githubusercontent.com/pingcap/tidb-operator/${TIDB_OPERATOR_VERSION}/manifests/crd.yaml"

# helm の公開リポジトリ(charts.pingcap.org)は 2026-10 時点で DNS が引けない(NXDOMAIN)ため、
# 同じ版の Git タグ(GitHub のソースアーカイブ)の charts/tidb-operator を helm へ直接渡す(ADR-0226)。
# アーカイブは sha256 を固定して検証する(改ざん・差し替えを検知する)。
archive_url="https://github.com/pingcap/tidb-operator/archive/refs/tags/${TIDB_OPERATOR_VERSION}.tar.gz"
archive_sha256="f69dc040956302fa2a9cd61987158cf88978db9b1f5a1a4a70849310118d2a2a"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

echo "tidb-operator-bootstrap: chart を取得する(${archive_url})..."
curl -fsSL --max-time 120 -o "$work/chart.tar.gz" "$archive_url"
actual_sha256="$(shasum -a 256 "$work/chart.tar.gz" | awk '{print $1}')"
if [ "$actual_sha256" != "$archive_sha256" ]; then
  echo "tidb-operator-bootstrap: アーカイブの sha256 が固定値と違う(期待 ${archive_sha256}、実際 ${actual_sha256})" >&2
  exit 1
fi
tar -xzf "$work/chart.tar.gz" -C "$work" --include '*/charts/tidb-operator/*'
chart_dir="$(find "$work" -type d -path '*/charts/tidb-operator' | head -n 1)"
[ -n "$chart_dir" ] || { echo "tidb-operator-bootstrap: アーカイブに charts/tidb-operator が無い" >&2; exit 1; }

echo "tidb-operator-bootstrap: tidb-operator ${TIDB_OPERATOR_VERSION} を導入する(helm upgrade --install。冪等)..."
helm upgrade --install tidb-operator "$chart_dir" \
  --namespace=tidb-admin --create-namespace \
  --set operatorImage="pingcap/tidb-operator:${TIDB_OPERATOR_VERSION}" \
  --set scheduler.create=false \
  --wait --timeout=180s

echo "tidb-operator-bootstrap: 完了"
