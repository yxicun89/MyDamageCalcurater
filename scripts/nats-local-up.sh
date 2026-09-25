#!/usr/bin/env bash
# make dev 用に docker で NATS v2.15.0(JetStream 有効)を 127.0.0.1:4222 に起動する(ADR-0212 §2)。
# MySQL(db-local-up.sh)と同じ「単純なコンテナを docker run する」流儀(NATS は単一バイナリで
# JetStream を内蔵しており、tiup のような専用ツールは不要)。
# 停止・削除はこのスクリプトの範囲外(JetStream のファイルストレージはコンテナ内。
# 使い捨てのイベントキューであり、消えても実害は直近分のイベントを失うだけ)。
set -euo pipefail

CONTAINER_NAME="${CALC_NATS_CONTAINER:-pokecalc-nats-local}"
NATS_IMAGE="nats:2.15.0@sha256:cd3fcd4ecdda44e3a66728a5334af0a959bc3979b32810e033d1c547241cd0f4"

command -v docker >/dev/null || { echo "nats-local-up: docker が見つからない" >&2; exit 1; }

if docker ps --format '{{.Names}}' | grep -qx "$CONTAINER_NAME"; then
  echo "nats-local-up: '$CONTAINER_NAME' は既に起動しています(127.0.0.1:4222)"
  exit 0
fi

if docker ps -a --format '{{.Names}}' | grep -qx "$CONTAINER_NAME"; then
  echo "nats-local-up: 既存のコンテナ '$CONTAINER_NAME' を起動します..."
  docker start "$CONTAINER_NAME" >/dev/null
else
  echo "nats-local-up: '$CONTAINER_NAME' を作成します..."
  docker run -d \
    --name "$CONTAINER_NAME" \
    -p 127.0.0.1:4222:4222 \
    "$NATS_IMAGE" -js -sd /data >/dev/null
fi

echo "nats-local-up: 起動しました(127.0.0.1:4222)。CALC_NATS_URL 例:"
echo "  nats://127.0.0.1:4222"
