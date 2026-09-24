#!/usr/bin/env bash
# 監視スタック(kube-prometheus-stack・Loki・Alloy)をローカル k3d に導入する(ADR-0406 §4。ADR-0405 と同じ流儀)。
# 自動テスト: scripts/observability-bootstrap_test.sh(helm・kubectl を PATH 上の偽物に差し替えて流す)。
#
# 手順: helm repo add/update → helm pull --version <固定版> -d <mktemp -d> → .tgz の SHA-256 を3つとも検証
# (1つでも不一致なら1つもインストールしない。chart ごとに検証してすぐ入れる、はしない)
# → namespace observability を冪等に作成 → 検証済み .tgz から helm upgrade --install --version <固定版>
# → kubectl apply -k deploy/k8s/base/observability(ServiceMonitor・PrometheusRule・ダッシュボードConfigMap。
#    kube-prometheus-stack の CRD が要るので後。ADR-0407 §3〜4)
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

# --- 固定値(このファイル1箇所にだけ持つ。版を上げるときはここを明示的に更新する。ADR-0406 §4) ---
# prometheus-community/kube-prometheus-stack(app v0.94.1)
readonly KUBE_PROMETHEUS_STACK_VERSION=91.5.1
readonly KUBE_PROMETHEUS_STACK_SHA256=b6bcf53edcb86fd1385b677d343c3f7366f0ed0d43fa7e7f7988eba28261a69f
# grafana/loki(app 3.6.12)
readonly LOKI_VERSION=7.3.0
readonly LOKI_SHA256=04a339f712d770a1f599f05fc0a5a3cde18e43914e49ae6a49f7171be86bcc09
# grafana/alloy(app v1.19.2)
readonly ALLOY_VERSION=1.12.1
readonly ALLOY_SHA256=cdd1ec39f99c3c506d5b521156d72236ea143c089f50da6b64398f831734829c

readonly NAMESPACE=observability
readonly VALUES_DIR=deploy/k8s/base/observability/values
readonly OBSERVABILITY_KUSTOMIZATION=deploy/k8s/base/observability

# 6定数がどれも空でないことを、取得・適用のどれにも触る前に確かめる(環境変数では上書きできない。readonly で固定済み)。
for const_name in KUBE_PROMETHEUS_STACK_VERSION KUBE_PROMETHEUS_STACK_SHA256 \
  LOKI_VERSION LOKI_SHA256 ALLOY_VERSION ALLOY_SHA256; do
  if [ -z "${!const_name}" ]; then
    echo "observability-bootstrap: $const_name が空です。導入を中止します" >&2
    exit 1
  fi
done

# sha256_of ファイル — macOS(shasum)・Linux(sha256sum)のどちらでも動く SHA-256。
sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{ print $1 }'
  else
    shasum -a 256 "$1" | awk '{ print $1 }'
  fi
}

helm repo add prometheus-community https://prometheus-community.github.io/helm-charts --force-update >/dev/null
helm repo add grafana https://grafana.github.io/helm-charts --force-update >/dev/null
helm repo update prometheus-community grafana >/dev/null

tmp_dir=$(mktemp -d)
trap 'rm -rf "$tmp_dir"' EXIT

# fetch_and_verify <repo/chart> <chart名> <版> <期待SHA256> — 取得して検証する。標準出力に検証済み .tgz のパスを返す。
# (bash 3.2 でも動くよう連想配列は使わない。macOS の既定 /bin/bash が対象。ADR-0405 と同じ制約)
fetch_and_verify() {
  local repo_chart=$1 name=$2 version=$3 expected_sha=$4 tgz actual_sha
  echo "observability-bootstrap: $repo_chart --version $version を取得します" >&2
  helm pull "$repo_chart" --version "$version" -d "$tmp_dir" >&2
  tgz="$tmp_dir/$name-$version.tgz"
  if [ ! -f "$tgz" ]; then
    echo "observability-bootstrap: $repo_chart の取得物 $tgz が見つかりません" >&2
    exit 1
  fi
  actual_sha=$(sha256_of "$tgz")
  if [ "$actual_sha" != "$expected_sha" ]; then
    echo "observability-bootstrap: $repo_chart の SHA-256 が一致しません。導入を中止します" >&2
    echo "  期待: $expected_sha" >&2
    echo "  実際: $actual_sha" >&2
    exit 1
  fi
  printf '%s' "$tgz"
}

# 3つとも取得・検証してから(1つも入れずに)導入へ進む(ADR-0406 §4)。
kube_prometheus_stack_tgz=$(fetch_and_verify prometheus-community/kube-prometheus-stack kube-prometheus-stack \
  "$KUBE_PROMETHEUS_STACK_VERSION" "$KUBE_PROMETHEUS_STACK_SHA256")
loki_tgz=$(fetch_and_verify grafana/loki loki "$LOKI_VERSION" "$LOKI_SHA256")
alloy_tgz=$(fetch_and_verify grafana/alloy alloy "$ALLOY_VERSION" "$ALLOY_SHA256")

echo "observability-bootstrap: 3チャートとも検証済みです。導入します" >&2

kubectl get namespace "$NAMESPACE" >/dev/null 2>&1 || kubectl create namespace "$NAMESPACE"

helm upgrade kube-prometheus-stack "$kube_prometheus_stack_tgz" --install \
  --version "$KUBE_PROMETHEUS_STACK_VERSION" -n "$NAMESPACE" -f "$VALUES_DIR/kube-prometheus-stack.yaml"

helm upgrade loki "$loki_tgz" --install \
  --version "$LOKI_VERSION" -n "$NAMESPACE" -f "$VALUES_DIR/loki.yaml"

helm upgrade alloy "$alloy_tgz" --install \
  --version "$ALLOY_VERSION" -n "$NAMESPACE" -f "$VALUES_DIR/alloy.yaml"

# ServiceMonitor は kube-prometheus-stack の CRD が要るので、導入した後に適用する。
echo "observability-bootstrap: ServiceMonitor・PrometheusRule・ダッシュボードを適用します" >&2
kubectl apply -k "$OBSERVABILITY_KUSTOMIZATION"

echo "observability-bootstrap: 完了しました" >&2
