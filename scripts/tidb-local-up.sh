#!/usr/bin/env bash
# make dev 用に tiup playground で TiDB v8.5.8 を 127.0.0.1:4000 に起動し、record・team の
# 2 DB を作る(ADR-0211 §2)。tiup playground は開発用の使い捨てプロセスで、db-local-up.sh の
# MySQL コンテナと違い「既に動いていれば何もしない」以上の永続化は保証しない
# (揮発させても実害は無く、このスクリプトを再実行すれば DB 名を作り直すだけで済む)。
# 停止は人間が明示的に行う(Ctrl+C または `tiup clean local`。CLAUDE.md「クラスタ削除・DBのデータ削除」の
# 対象外。使い捨ての開発用プロセスであり永続データの削除ではないため)。
set -euo pipefail
cd "$(dirname "$0")/.."

TIDB_VERSION="v8.5.8"
DB_PORT="${TIDB_LOCAL_DB_PORT:-4000}"
PD_PORT="${TIDB_LOCAL_PD_PORT:-2379}"
# 使い捨ての mysql クライアント(db-local-up.sh と同じ pin 済みイメージを再利用)。
MYSQL_CLIENT_IMAGE="mysql:9.7.2@sha256:29abb0a179982e4a8928138bfc7f918af9eda64e7eeb1b1d084c1720a20159e6"

command -v tiup >/dev/null || {
  echo "tidb-local-up: tiup が見つからない(scripts/doctor.sh を参照。curl のインストーラで導入する)" >&2
  exit 1
}
command -v docker >/dev/null || { echo "tidb-local-up: docker が見つからない(使い捨ての mysql クライアント用)" >&2; exit 1; }

run_sql() {
  # ローカルの mysql クライアントが無い開発機でも動くよう、使い捨てコンテナから叩く
  # (host.docker.internal は OrbStack/Docker Desktop の既定機能)。
  docker run --rm "$MYSQL_CLIENT_IMAGE" \
    mysql -h host.docker.internal -P "$DB_PORT" -u root -e "$1"
}

if run_sql "SELECT 1" >/dev/null 2>&1; then
  echo "tidb-local-up: 127.0.0.1:${DB_PORT} は既に応答している(起動済みとみなす)"
else
  echo "tidb-local-up: tiup playground ${TIDB_VERSION} を起動する(バックグラウンド)..."
  nohup tiup playground "$TIDB_VERSION" \
    --host 127.0.0.1 --db.port "$DB_PORT" --pd.port "$PD_PORT" --without-monitor \
    >/tmp/tidb-local-playground.log 2>&1 &
  disown

  echo "tidb-local-up: TiDB の起動を待つ(最大2分)..."
  i=0
  until run_sql "SELECT 1" >/dev/null 2>&1 || [ "$i" -ge 40 ]; do
    sleep 3
    i=$((i + 1))
  done
  if ! run_sql "SELECT 1" >/dev/null 2>&1; then
    echo "tidb-local-up: TiDB が起動しない。/tmp/tidb-local-playground.log を確認すること" >&2
    exit 1
  fi
fi

echo "tidb-local-up: record・team の DB を作る(既にあれば何もしない)..."
run_sql "CREATE DATABASE IF NOT EXISTS record; CREATE DATABASE IF NOT EXISTS team;"

echo "tidb-local-up: 完了。DSN 例(root にパスワードは無い):"
echo "  root:@tcp(127.0.0.1:${DB_PORT})/record?parseTime=true"
echo "  root:@tcp(127.0.0.1:${DB_PORT})/team?parseTime=true"
