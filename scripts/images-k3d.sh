#!/usr/bin/env bash
# make images-k3d: 変換済みの画像(make assets の出力)を k3d のノードへ置き、gateway の /images/* で配信する(ADR-0807 追記)。
# gateway の local overlay が、ノード上の NODE_DIR を hostPath(読み取り専用)で GATEWAY_IMAGES_DIR に見せている。
# 画像が無くてもアプリは動く(エンブレム)ので、dist に manifest.json が無ければ何もせず終了コード 0。
# 使い方: make assets && make images-k3d。gateway の再起動は不要(ファイルを読むだけ)。
# 自動テスト: scripts/images-k3d_test.sh(make test-scripts)。
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

CLUSTER=${CLUSTER:-pokecalc}
DIST=${ASSETS_OUT:-data/generated/images/dist}
NODE=${NODE:-k3d-${CLUSTER}-server-0}
NODE_DIR=${NODE_DIR:-/var/lib/pokecalc-images}

if [ ! -f "$DIST/manifest.json" ]; then
  echo "images-k3d: $DIST/manifest.json が無いので画像なしのまま(エンブレム表示)。画像を使うには data/generated/images/src に置いて make assets"
  exit 0
fi

CLUSTER="$CLUSTER" ./scripts/require-k3d-context.sh images-k3d
command -v docker >/dev/null 2>&1 || { echo "images-k3d: docker が無い" >&2; exit 1; }

# 古いファイルを残さないよう中身だけ消してから、dist の中身だけをコピーする(hostPath は読み取り専用なので Pod からは書けない)。
# ディレクトリ自体は消さない(作り直すと、動作中の Pod の bind mount が消えた古い実体を指したままになり 404 が続く)。
docker exec "$NODE" sh -c "mkdir -p '$NODE_DIR' && find '$NODE_DIR' -mindepth 1 -delete"
docker cp "$DIST/." "$NODE:$NODE_DIR"
# gateway は非 root(65532)で読むので、全員が読めるようにする。
docker exec "$NODE" chmod -R a+rX "$NODE_DIR"
echo "images-k3d: $DIST の中身を $NODE:$NODE_DIR へ置いた。確認: curl -s http://localhost:8080/images/manifest.json"
