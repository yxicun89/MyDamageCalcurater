#!/usr/bin/env bash
# k3d の実 DB(record・team = TiDB、pokedex = MySQL)の論理バックアップを取る(P7-4。ADR-0225・ADR-0227)。
# db-backup.sh full <kind> を、port-forward と Secret の読み取りをこのスクリプトの内部で行って流す
# (パスワードは画面・コマンド行・ps に出さない。環境変数 MYSQL_PWD だけで渡す)。バックアップは非破壊。
# 保存先は BACKUP_DIR(既定 data/generated/backups。Git の無視対象。機密なのでログ・PR に中身を貼らない)。
# 使い方: scripts/db-backup-k3d.sh [record|team|pokedex ...](省略時は3つ全部)
# 前提: make deploy-latest 済み(TiDB・record・team が稼働)。接続は最小権限: record・team は <db>_migrator(SELECT を含む)、pokedex は pokedex_reader(SELECT のみ)。
set -euo pipefail
umask 077

cd "$(git rev-parse --show-toplevel)"
# shellcheck source=scripts/k3d-db-lib.sh
. scripts/k3d-db-lib.sh
trap kdb_cleanup EXIT

kdb_init db-backup-k3d

kinds=("$@")
[ "${#kinds[@]}" -gt 0 ] || kinds=(record team pokedex)
for k in "${kinds[@]}"; do
  case "$k" in record | team | pokedex) ;; *) echo "使い方: scripts/db-backup-k3d.sh [record|team|pokedex ...]" >&2; exit 2 ;; esac
done

kdb_clients
TIDB_PORT=${BACKUP_TIDB_PORT:-14000}
MYSQL_PORT=${BACKUP_MYSQL_PORT:-13308}

forwarded_tidb=0
forwarded_mysql=0
for kind in "${kinds[@]}"; do
  case "$kind" in
    record | team)
      if [ "$forwarded_tidb" = 0 ]; then kdb_forward pokecalc-tidb-tidb "$TIDB_PORT" 4000; forwarded_tidb=1; fi
      port=$TIDB_PORT
      secret="${kind}-db-auth"
      key="${kind}-migrator-dsn"
      ;;
    pokedex)
      if [ "$forwarded_mysql" = 0 ]; then kdb_forward mysql "$MYSQL_PORT" 3306; forwarded_mysql=1; fi
      port=$MYSQL_PORT
      secret=mysql-auth
      key=pokedex-reader-dsn
      ;;
  esac
  dsn=$(kdb_secret "$secret" "$key")
  [ -n "$dsn" ] || kdb_die "Secret ${secret} に ${key} が無い(make deploy-latest で作る)"
  user=$(kdb_dsn_user "$dsn")
  MYSQL_PWD=$(kdb_dsn_pass "$dsn")
  export MYSQL_PWD
  unset dsn
  echo "== ${kind}"
  DB_HOST="$KDB_HOST" DB_PORT="$port" DB_USER="$user" DB_NAME="$kind" ./scripts/db-backup.sh full "$kind"
  unset MYSQL_PWD user
done
