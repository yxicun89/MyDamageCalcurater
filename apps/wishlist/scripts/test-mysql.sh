#!/usr/bin/env bash
# wishlist の MySQL 実装の契約テスト(go test -tags mysql)を流す。
#
#   make wishlist-test-mysql
#
# - WISHLIST_TEST_DSN があればそれを使う(CREATE/DROP DATABASE ができるユーザー。テストは使い捨ての DB を作って消す)。
# - 無ければ docker で mysql:9.7.2 を使い捨てで起動し、終わったら消す。docker が無ければスキップせず失敗する。
set -euo pipefail
cd "$(git rev-parse --show-toplevel)/apps/wishlist/api"

readonly MYSQL_IMAGE="mysql:9.7.2@sha256:29abb0a179982e4a8928138bfc7f918af9eda64e7eeb1b1d084c1720a20159e6"
readonly WAIT_SECONDS="${TEST_DB_WAIT_SECONDS:-180}"

run_tests() {
  GOWORK=off go test -tags mysql -count=1 ./...
}

if [ -n "${WISHLIST_TEST_DSN:-}" ]; then
  run_tests
  exit 0
fi

command -v docker >/dev/null 2>&1 || { echo "wishlist-test-mysql: docker が無い(WISHLIST_TEST_DSN も無い。スキップせず失敗する)" >&2; exit 1; }
docker info >/dev/null 2>&1 || { echo "wishlist-test-mysql: Docker が起動していない(スキップせず失敗する)" >&2; exit 1; }

readonly NAME="wishlist-testdb-$$-$(date +%s)"
cleanup() { docker rm -f "$NAME" >/dev/null 2>&1 || true; }
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

# 使い捨てのコンテナ限りのパスワード。コマンドライン引数に出さない(環境変数で渡す)。
MYSQL_ROOT_PASSWORD="$(openssl rand -hex 16)"
export MYSQL_ROOT_PASSWORD

echo "wishlist-test-mysql: MySQL を起動します(使い捨て。終了時に消します)..." >&2
docker run -d --name "$NAME" -p 127.0.0.1::3306 -e MYSQL_ROOT_PASSWORD \
  "$MYSQL_IMAGE" --character-set-server=utf8mb4 --collation-server=utf8mb4_0900_ai_ci >/dev/null

# エントリポイントは一時サーバーで初期化してから起動し直すので、TCP(127.0.0.1)で確かめる。
waited=0
until MYSQL_PWD="$MYSQL_ROOT_PASSWORD" docker exec -e MYSQL_PWD "$NAME" mysql -h 127.0.0.1 -uroot -e "SELECT 1" >/dev/null 2>&1; do
  if [ "$waited" -ge "$WAIT_SECONDS" ]; then
    echo "wishlist-test-mysql: MySQL が ${WAIT_SECONDS} 秒以内に起動しない" >&2
    docker logs --tail 30 "$NAME" >&2 || true
    exit 1
  fi
  sleep 2
  waited=$((waited + 2))
done

port="$(docker port "$NAME" 3306/tcp | head -1 | sed 's/.*://')"
WISHLIST_TEST_DSN="root:${MYSQL_ROOT_PASSWORD}@tcp(127.0.0.1:${port})/mysql?parseTime=true" run_tests
