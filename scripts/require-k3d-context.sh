#!/usr/bin/env bash
# require-k3d-context.sh <呼び出し元の名前>
#
# kubectl の現在の context が k3d-$CLUSTER(既定 pokecalc)でなければ、理由を出して終了コード 1 で終わる。
# クラスタを変える操作(apply・rollout・create job・image import 後の apply 等)の前に呼び、別クラスタ
# (cloud・他プロジェクト)へ適用しないようにする(issue #295)。kubectl は config current-context の読み取りだけ。
#
# 使い方(Makefile): @CLUSTER=$(CLUSTER) ./scripts/require-k3d-context.sh api-k3d-deploy
# 使い方(スクリプト): CLUSTER="$CLUSTER" ./scripts/require-k3d-context.sh up.sh
# 自動テスト: scripts/require-k3d-context_test.sh(make test-scripts)。
set -euo pipefail

caller="${1:-require-k3d-context}"
cluster="${CLUSTER:-pokecalc}"
want="k3d-${cluster}"

if ! command -v kubectl >/dev/null 2>&1; then
  echo "${caller}: kubectl が無いため context を確かめられない(別クラスタへ適用しないよう中断)" >&2
  exit 1
fi

current="$(kubectl config current-context 2>/dev/null || true)"
if [ "$current" != "$want" ]; then
  echo "${caller}: 現在の kubectl context '${current:-(未設定)}' が '${want}' ではない(別クラスタへ適用してしまうため中断)" >&2
  echo "${caller}: k3d のクラスタへ切り替えるには kubectl config use-context ${want}(無ければ make up)" >&2
  exit 1
fi
