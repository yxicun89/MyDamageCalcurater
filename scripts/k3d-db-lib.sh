#!/usr/bin/env bash
# k3d の DB(TiDB の record・team と mysql の pokedex)へ届かせる共通部品(P7-4。ADR-0225・ADR-0227)。
# db-backup-k3d.sh と db-restore-drill-k3d.sh が source する(単体では実行しない)。
# 流儀は pokedex-export-local.sh と同じ: port-forward と Secret の読み取りはスクリプトの内部で行い、
# パスワード・DSN はコマンド行・ログ・画面・ps に出さない(環境変数 MYSQL_PWD でだけ渡す)。
# 呼ぶ側が先に set -euo pipefail を済ませ、ルートへ cd している前提。

KDB_PIDS=()
KDB_TMP=""

kdb_cleanup() {
  local pid
  for pid in ${KDB_PIDS[@]+"${KDB_PIDS[@]}"}; do
    kill "$pid" 2>/dev/null || true
    wait "$pid" 2>/dev/null || true
  done
  [ -z "$KDB_TMP" ] || rm -rf "$KDB_TMP"
}

kdb_die() { echo "${KDB_CALLER:-k3d-db}: $*" >&2; exit 1; }

# kdb_init <呼び出し元> — context の確認と一時ディレクトリ。
kdb_init() {
  KDB_CALLER="$1"
  CLUSTER="${CLUSTER:-pokecalc}" ./scripts/require-k3d-context.sh "$KDB_CALLER"
  command -v nc >/dev/null 2>&1 || kdb_die "nc が無い(port-forward の疎通確認に使う)"
  KDB_TMP=$(mktemp -d)
  chmod 700 "$KDB_TMP"
}

# kdb_forward <svc> <ローカルポート> <リモートポート> — port-forward を張って待つ。
kdb_forward() {
  local svc="$1" lport="$2" rport="$3" pid ready=0
  if nc -z 127.0.0.1 "$lport" 2>/dev/null; then
    kdb_die "127.0.0.1:${lport} は別のプロセスが使用中。ポートを変えて再実行する"
  fi
  kubectl -n pokecalc port-forward "svc/${svc}" "${lport}:${rport}" >/dev/null 2>&1 &
  pid=$!
  KDB_PIDS+=("$pid")
  for _ in $(seq 1 30); do
    kill -0 "$pid" 2>/dev/null || break
    if nc -z 127.0.0.1 "$lport" 2>/dev/null; then ready=1; break; fi
    sleep 0.5
  done
  [ "$ready" = 1 ] || kdb_die "${svc} への port-forward が張れなかった(kubectl -n pokecalc get pods で Running か確認する)"
}

# kdb_secret <Secret 名> <キー> — 値を標準出力へ(呼ぶ側は必ず $(...) で受け、表示しない)。
kdb_secret() {
  kubectl -n pokecalc get secret "$1" -o "jsonpath={.data.$2}" | base64 -d
}

# kdb_dsn_user <DSN> / kdb_dsn_pass <DSN> — `user:pass@tcp(...)/db` から取り出す(パスワードは乱数 hex)。
kdb_dsn_user() { printf '%s' "$1" | sed -E 's/^([^:]*):.*/\1/'; }
kdb_dsn_pass() { printf '%s' "$1" | sed -E 's/^[^:]*:(.*)@tcp\(.*/\1/'; }

# kdb_clients — MYSQL_BIN・MYSQLDUMP_BIN・KDB_HOST を決める。ホストに mysql・mysqldump が無ければ、
# 固定 digest の mysql イメージの docker ラッパー(パスワードは `-e MYSQL_PWD` で値を渡さず継承)を使う。
kdb_clients() {
  if command -v mysql >/dev/null 2>&1 && command -v mysqldump >/dev/null 2>&1; then
    MYSQL_BIN=$(command -v mysql)
    MYSQLDUMP_BIN=$(command -v mysqldump)
    KDB_HOST=127.0.0.1
  else
    command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1 ||
      kdb_die "ホストに mysql・mysqldump が無く、docker も使えない(どちらかを用意する)"
    local image tool
    image=$(grep -E '^readonly MYSQL_IMAGE=' scripts/test-db-docker.sh | sed -E 's/^readonly MYSQL_IMAGE="(.*)"$/\1/')
    [ -n "$image" ] || kdb_die "scripts/test-db-docker.sh から MYSQL_IMAGE を読めなかった"
    for tool in mysql mysqldump; do
      cat > "$KDB_TMP/$tool" <<WRAP
#!/usr/bin/env bash
exec docker run --rm -i -e MYSQL_PWD ${image} ${tool} "\$@"
WRAP
      chmod 700 "$KDB_TMP/$tool"
    done
    MYSQL_BIN="$KDB_TMP/mysql"
    MYSQLDUMP_BIN="$KDB_TMP/mysqldump"
    KDB_HOST=host.docker.internal
  fi
  export MYSQL_BIN MYSQLDUMP_BIN
}
