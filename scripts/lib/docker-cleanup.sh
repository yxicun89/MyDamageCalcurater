#!/usr/bin/env bash
# 動作確認のたびに docker build が残すゴミ(古いイメージ層・ビルドキャッシュ・k3d ノード内の未使用イメージ)を掃除する。
# deploy-latest・up などが終了時に呼ぶ(EXIT trap)。ストレージを膨らませないための後始末で、失敗しても本体の終了コードは変えない。
#
# 消すもの: ① タグの付かなくなったイメージ(dangling) ② 古いビルドキャッシュ ③ k3d ノード内の未使用イメージ(crictl rmi --prune)
# 消さないもの: 起動中のコンテナ・ボリューム(DB のデータ)・使用中のイメージ・タグ付きのイメージ
#
# 環境変数: DOCKER_CLEANUP=0(掃除しない)・DOCKER_CLEANUP_CACHE_AGE(この時間より古いビルドキャッシュを消す。既定 24h)
#
# 使い方: source "$(dirname "$0")/lib/docker-cleanup.sh" のあと、trap docker_cleanup EXIT

docker_cleanup() {
  local rc=$?
  [ "${DOCKER_CLEANUP:-1}" = 0 ] && return "$rc"
  command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1 || return "$rc"
  echo "== docker の後始末(タグ無しイメージ・古いビルドキャッシュ・k3d ノードの未使用イメージ)" >&2
  docker image prune -f >/dev/null 2>&1 || true
  docker builder prune -af --filter "until=${DOCKER_CLEANUP_CACHE_AGE:-24h}" >/dev/null 2>&1 || true
  local node
  for node in $(docker ps --format '{{.Names}}' --filter "name=^k3d-${CLUSTER:-pokecalc}-(server|agent)-" 2>/dev/null); do
    docker exec "$node" crictl rmi --prune >/dev/null 2>&1 || true
  done
  return "$rc"
}
