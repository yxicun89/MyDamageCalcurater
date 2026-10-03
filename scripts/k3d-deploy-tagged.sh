#!/usr/bin/env bash
# k3d-deploy-tagged.sh <呼び出し元の名前> <overlayのパス> <イメージ名>... — k3d へコミット識別のタグでデプロイする(issue #291)。
#
# `*-docker-build` が作った `<イメージ名>:local` を、git のコミット(未コミットの変更があれば -dirty。scripts/image-tag.sh)
# のタグへ付け替えて k3d に入れ、overlay を描画した結果の image だけを `:local` からそのタグへ置き換えて apply する
# (Git の overlay は変えない)。`:local` を上書きして rollout restart する従来の方式と違い、Pod の image から
# ビルド元のコミットが分かり、`kubectl rollout undo` で前のタグ(k3d ノードに残っている)へ戻せる。
#
# - 手順の最初に require-k3d-context.sh で context を確かめる(別クラスタへ適用しない。issue #295)。
# - Deployment 名はイメージ名の最後の要素(pokecalc/gateway -> gateway)。
# - 同じコミットでの再実行は image が変わらず、rollout は起きない(冪等)。ただし -dirty は未コミットの中身が
#   変わっても同じタグなので、そのときだけ rollout restart する。
# - 環境変数 CLUSTER(既定 pokecalc)、TAG_PATHS(image-tag.sh に渡すパス。空白区切り。既定はリポジトリ全体)。
#
# 使い方(Makefile): @CLUSTER=$(CLUSTER) TAG_PATHS="services engine" ./scripts/k3d-deploy-tagged.sh api-k3d-deploy deploy/k8s/overlays/local-api pokecalc/calc pokecalc/gateway
# 自動テスト: scripts/k3d-deploy-tagged_test.sh(make test-scripts)。
set -euo pipefail

if [ "$#" -lt 3 ]; then
  echo "usage: k3d-deploy-tagged.sh <caller> <overlay> <image>..." >&2
  exit 2
fi
caller="$1"
overlay="$2"
shift 2
cluster="${CLUSTER:-pokecalc}"

cd "$(git rev-parse --show-toplevel)"
CLUSTER="$cluster" ./scripts/require-k3d-context.sh "$caller"

# shellcheck disable=SC2086 # TAG_PATHS は空白区切りのパス列
tag="$(./scripts/image-tag.sh ${TAG_PATHS:-})"

tagged=()
deployments=()
for image in "$@"; do
  docker tag "${image}:local" "${image}:${tag}"
  tagged+=("${image}:${tag}")
  deployments+=("deployment/${image##*/}")
done
k3d image import "${tagged[@]}" --cluster "$cluster"

manifest="$(kubectl kustomize "$overlay")"
for image in "$@"; do
  manifest="$(printf '%s\n' "$manifest" | sed "s#image: ${image}:local#image: ${image}:${tag}#")"
done
printf '%s\n' "$manifest" | kubectl apply -f -

case "$tag" in
  *-dirty) kubectl -n pokecalc rollout restart "${deployments[@]}" ;;
esac
for d in "${deployments[@]}"; do
  kubectl -n pokecalc rollout status "$d" --timeout=120s
done
echo "${caller}: ${tag} をデプロイした(戻すときは docs/runbooks/rollback.md)"
