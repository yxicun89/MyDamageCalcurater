#!/usr/bin/env bash
# make dev 用に docker で mysql:9.7.2 を 127.0.0.1:3306 に起動する(ADR-0015 §5・§9)。
# パスワードは .env から読む(サンプルは .env.example。.env は Git に含めない)。
# 停止・削除はこのスクリプトの範囲外(データ削除は人間の確認が必要。CLAUDE.md)。
set -euo pipefail
cd "$(dirname "$0")/.."

ENV_FILE="${ENV_FILE:-.env}"
if [ -f "$ENV_FILE" ]; then
  set -a
  # shellcheck disable=SC1090
  source "$ENV_FILE"
  set +a
fi

CONTAINER_NAME="${POKEDEX_MYSQL_CONTAINER:-pokecalc-mysql-local}"

if [ -z "${MYSQL_ROOT_PASSWORD:-}" ]; then
  echo "db-local-up: MYSQL_ROOT_PASSWORD が無い(.env を作る。.env.example を参照)" >&2
  exit 1
fi
export MYSQL_ROOT_PASSWORD

command -v docker >/dev/null || { echo "db-local-up: docker が見つからない" >&2; exit 1; }

if docker ps --format '{{.Names}}' | grep -qx "$CONTAINER_NAME"; then
  echo "db-local-up: '$CONTAINER_NAME' は既に起動しています(127.0.0.1:3306)"
  exit 0
fi

if docker ps -a --format '{{.Names}}' | grep -qx "$CONTAINER_NAME"; then
  echo "db-local-up: 既存のコンテナ '$CONTAINER_NAME' を起動します..."
  docker start "$CONTAINER_NAME" >/dev/null
else
  echo "db-local-up: '$CONTAINER_NAME' を作成します..."
  docker run -d \
    --name "$CONTAINER_NAME" \
    -p 127.0.0.1:3306:3306 \
    -e MYSQL_ROOT_PASSWORD \
    mysql:9.7.2@sha256:29abb0a179982e4a8928138bfc7f918af9eda64e7eeb1b1d084c1720a20159e6 \
    --character-set-server=utf8mb4 \
    --collation-server=utf8mb4_0900_ai_ci >/dev/null
fi

echo "db-local-up: 起動しました(127.0.0.1:3306)。DSN 例:"
echo "  root:<MYSQL_ROOT_PASSWORD>@tcp(127.0.0.1:3306)/pokedex?parseTime=true"
