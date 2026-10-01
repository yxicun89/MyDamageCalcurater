#!/usr/bin/env bash
# make test-db-docker(issue #223)。Docker で使い捨ての MySQL と TiDB を起動し、`make test-db`(-tags mysql の
# pokedex・-tags tidb の record/team)を流して、終わったら(失敗・中断でも)コンテナとネットワークを消す。
#
# - MySQL は scripts/db-local-up.sh と同じ digest、TiDB は v8.5.8(deploy/k8s/overlays/local/tidb と同じ版。
#   digest は Docker Hub の pingcap/tidb:v8.5.8 の index)で固定する。ポートはホストの 127.0.0.1 の空きポートを Docker に選ばせる(開発用の 3306・4000 と衝突しない)
# - DB 名は各テストの安全装置どおり _test で終わる名前(pokedex_test・record_test・team_test)
# - TiDB には golang-migrate 用に tidb_skip_isolation_level_check=1 を設定する(scripts/tidb-local-up.sh と同じ理由。ADR-0211)
# - Docker が無い・起動しないときは「スキップして成功」にせず失敗する(CLAUDE.md 絶対ルール 6)
# - 消すのはこのスクリプトが作った使い捨てのコンテナ・ネットワークだけ(人のデータには触らない)
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

readonly MYSQL_IMAGE="mysql:9.7.2@sha256:29abb0a179982e4a8928138bfc7f918af9eda64e7eeb1b1d084c1720a20159e6"
readonly TIDB_IMAGE="pingcap/tidb:v8.5.8@sha256:df168c764bf2dfdb166dc37a5c3b0e210d29d5f3ab2d33317fd0fdf7b32037f5"
readonly MAKE_BIN="${MAKE:-make}"
readonly WAIT_SECONDS="${TEST_DB_WAIT_SECONDS:-180}"

command -v docker >/dev/null 2>&1 || { echo "test-db-docker: docker が無い(スキップせず失敗する)" >&2; exit 1; }
docker info >/dev/null 2>&1 || { echo "test-db-docker: Docker が起動していない(スキップせず失敗する)" >&2; exit 1; }

suffix="$$-$(date +%s)"
readonly NETWORK="pokecalc-testdb-${suffix}"
readonly MYSQL_NAME="pokecalc-testdb-mysql-${suffix}"
readonly TIDB_NAME="pokecalc-testdb-tidb-${suffix}"

cleanup() {
  docker rm -f "$MYSQL_NAME" "$TIDB_NAME" >/dev/null 2>&1 || true
  docker network rm "$NETWORK" >/dev/null 2>&1 || true
}
# INT・TERM では exit して EXIT の trap(後片付け)に任せる。trap cleanup INT だけだと後片付けの後も続きを実行してしまう。
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

# 値はコマンドライン引数に出さない(環境変数で渡す)。使い捨てのコンテナ限りのパスワード。
MYSQL_ROOT_PASSWORD="$(openssl rand -hex 16)"
export MYSQL_ROOT_PASSWORD

docker network create "$NETWORK" >/dev/null
echo "test-db-docker: MySQL と TiDB を起動します(使い捨て。終了時に消します)..." >&2
docker run -d --name "$MYSQL_NAME" --network "$NETWORK" -p 127.0.0.1::3306 \
  -e MYSQL_ROOT_PASSWORD -e MYSQL_DATABASE=pokedex_test \
  "$MYSQL_IMAGE" --character-set-server=utf8mb4 --collation-server=utf8mb4_0900_ai_ci >/dev/null
docker run -d --name "$TIDB_NAME" --network "$NETWORK" -p 127.0.0.1::4000 "$TIDB_IMAGE" >/dev/null

# mysql_sql ホスト ポート SQL — MySQL コンテナの中のクライアントから SQL を流す。
mysql_sql() {
  local host="$1" port="$2" sql="$3"
  if [ "$host" = "127.0.0.1" ]; then
    MYSQL_PWD="$MYSQL_ROOT_PASSWORD" docker exec -e MYSQL_PWD "$MYSQL_NAME" mysql -h 127.0.0.1 -P "$port" -uroot -e "$sql"
  else
    docker exec "$MYSQL_NAME" mysql -h "$host" -P "$port" -uroot -e "$sql"
  fi
}

# wait_ready 名前 ホスト ポート — SELECT 1 が通るまで待つ。
wait_ready() {
  local name="$1" host="$2" port="$3" waited=0
  until mysql_sql "$host" "$port" "SELECT 1" >/dev/null 2>&1; do
    if [ "$waited" -ge "$WAIT_SECONDS" ]; then
      echo "test-db-docker: ${name} が ${WAIT_SECONDS} 秒以内に起動しない" >&2
      docker logs --tail 30 "$( [ "$name" = MySQL ] && echo "$MYSQL_NAME" || echo "$TIDB_NAME")" >&2 || true
      exit 1
    fi
    sleep 2
    waited=$((waited + 2))
  done
}

# MySQL はエントリポイントが一時サーバーで初期化した後に本番の起動をし直すので、TCP(127.0.0.1)で確かめる。
wait_ready MySQL 127.0.0.1 3306
wait_ready TiDB "$TIDB_NAME" 4000
mysql_sql "$TIDB_NAME" 4000 "CREATE DATABASE IF NOT EXISTS record_test; CREATE DATABASE IF NOT EXISTS team_test; SET GLOBAL tidb_skip_isolation_level_check=1;"

mysql_port="$(docker port "$MYSQL_NAME" 3306/tcp | head -1 | sed 's/.*://')"
tidb_port="$(docker port "$TIDB_NAME" 4000/tcp | head -1 | sed 's/.*://')"

echo "test-db-docker: make test-db を流します(MySQL 127.0.0.1:${mysql_port}・TiDB 127.0.0.1:${tidb_port})" >&2
POKEDEX_TEST_DSN="root:${MYSQL_ROOT_PASSWORD}@tcp(127.0.0.1:${mysql_port})/pokedex_test?parseTime=true" \
RECORD_TEST_DSN="root:@tcp(127.0.0.1:${tidb_port})/record_test?parseTime=true" \
TEAM_TEST_DSN="root:@tcp(127.0.0.1:${tidb_port})/team_test?parseTime=true" \
  "$MAKE_BIN" --no-print-directory test-db
echo "test-db-docker: 完了しました" >&2
