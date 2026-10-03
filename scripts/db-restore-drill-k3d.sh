#!/usr/bin/env bash
# k3d の実 TiDB で復元訓練をする(P7-4。ADR-0225・ADR-0227)。稼働中の record・team DB は上書きしない。
# 使い捨ての別名 DB(record_restore_drill / team_restore_drill)を作り、最新世代を
# RESTORE_FROM_DB=<元 DB> CONFIRM_RESTORE=<別名 DB> で復元し、元 DB と行数(全表)・墓石(devices.purged_at)・
# purge_journal が一致することを確かめて、別名 DB を削除する。
# 削除するのは、このスクリプトが今回作った別名 DB だけ(名前を厳密に検査。元の DB・既存の別名 DB には DROP を流さない)。
# 権限: 別名 DB の作成・削除は migrator には無い(ADR-0211 §4: 自分の DB の DDL のみ)ので、tidb-root-auth の root を使う
#   (スクリプトの内部で読み、画面・コマンド行・ps に出さない。権限付与の追加はしない)。
# 前提: scripts/db-backup-k3d.sh で世代を取った直後(間に record・team への書き込みがあると行数が合わない)。
# 使い方: scripts/db-restore-drill-k3d.sh [record|team ...](省略時は両方)
set -euo pipefail
umask 077

cd "$(git rev-parse --show-toplevel)"
# shellcheck source=scripts/k3d-db-lib.sh
. scripts/k3d-db-lib.sh

created_db=""
drop_created() {
  # このスクリプトが作った別名 DB だけを消す(名前を再検査してから)。
  if [ -n "$created_db" ] && [[ "$created_db" =~ ^(record|team)_restore_drill$ ]]; then
    "$MYSQL_BIN" -h "$KDB_HOST" -P "$TIDB_PORT" -u root -e "DROP DATABASE IF EXISTS \`${created_db}\`" >/dev/null 2>&1 || echo "db-restore-drill-k3d: 別名 DB ${created_db} を消せなかった(手で削除する)" >&2
    created_db=""
  fi
}
on_exit() { drop_created; kdb_cleanup; }
trap on_exit EXIT

kdb_init db-restore-drill-k3d

kinds=("$@")
[ "${#kinds[@]}" -gt 0 ] || kinds=(record team)
for k in "${kinds[@]}"; do
  case "$k" in record | team) ;; *) echo "使い方: scripts/db-restore-drill-k3d.sh [record|team ...]" >&2; exit 2 ;; esac
done

BACKUP_DIR="${BACKUP_DIR:-data/generated/backups}"
case "$BACKUP_DIR" in /*) ;; *) BACKUP_DIR="$PWD/$BACKUP_DIR" ;; esac

kdb_clients
TIDB_PORT=${BACKUP_TIDB_PORT:-14000}
kdb_forward pokecalc-tidb-tidb "$TIDB_PORT" 4000

root_pw=$(kdb_secret tidb-root-auth root)
[ -n "$root_pw" ] || kdb_die "Secret tidb-root-auth に root が無い(make deploy-latest で作る)"
MYSQL_PWD="$root_pw"
export MYSQL_PWD

sql() { "$MYSQL_BIN" -h "$KDB_HOST" -P "$TIDB_PORT" -u root -N -B -e "$1"; }
count() { sql "SELECT COUNT(*) FROM \`$1\`.\`$2\`$3"; }

failures=0
for kind in "${kinds[@]}"; do
  drill="${kind}_restore_drill"
  [[ "$drill" =~ ^(record|team)_restore_drill$ ]] || kdb_die "別名 DB 名が想定外: ${drill}"
  echo "== ${kind} -> ${drill}"

  latest=$(ls -1 "$BACKUP_DIR/$kind" 2>/dev/null | grep -E '^[0-9]{8}T[0-9]{6}Z$' | sort -r | head -1 || true)
  [ -n "$latest" ] || kdb_die "${kind} の世代が無い(先に make db-backup-k3d)"
  tables=$(sed -n 's/^tables: //p' "$BACKUP_DIR/$kind/$latest/MANIFEST")
  for t in $tables; do [[ "$t" =~ ^[a-z_0-9]+$ ]] || kdb_die "MANIFEST の表名が不正"; done

  # 既存の別名 DB は消さない(自分が作ったものではないので)。
  [ -z "$(sql "SHOW DATABASES LIKE '${drill}'")" ] || kdb_die "別名 DB ${drill} が既にある(前回の残りなら手で確認して削除する。自動では消さない)"
  sql "CREATE DATABASE \`${drill}\`"
  created_db="$drill"

  # 失効ジョブ(復元の手順5)は別名 DB に向けて実際に流す。保持日数は本番の ConfigMap と同じ値。
  retention_env=$(kubectl -n pokecalc get configmap "${kind}-retention" -o go-template='{{range $k, $v := .data}}{{$k}}={{$v}}{{"\n"}}{{end}}')
  [ -n "$retention_env" ] || kdb_die "ConfigMap ${kind}-retention を読めなかった"
  while IFS='=' read -r k v; do
    [ -n "$k" ] || continue
    [[ "$k" =~ ^[A-Z_]+$ && "$v" =~ ^[0-9]+$ ]] || kdb_die "ConfigMap ${kind}-retention の値が想定外"
    export "$k=$v"
  done <<< "$retention_env"
  upper=$(printf '%s' "$kind" | tr '[:lower:]' '[:upper:]')
  export "${upper}_EXPIRE_BATCH_LIMIT=1000"
  # go の DSN は 127.0.0.1 の port-forward 向け(docker ラッパー使用時も、失効ジョブはホストの go が流す)。
  export "${upper}_APP_DSN=root:${root_pw}@tcp(127.0.0.1:${TIDB_PORT})/${drill}?parseTime=true"

  DB_HOST="$KDB_HOST" DB_PORT="$TIDB_PORT" DB_USER=root DB_NAME="$drill" \
    RESTORE_FROM_DB="$kind" CONFIRM_RESTORE="$drill" \
    RESTORE_EXPIRE_CMD="cd services && go run ./${kind}/cmd/${kind} expire" \
    ./scripts/db-restore.sh "$kind" "$latest" | tee "$KDB_TMP/restore.out" | grep -E '^(restore|loaded|journal|reapplied|expire|restore-ok)' || true
  grep -q "^restore-ok ${kind} ${latest}\$" "$KDB_TMP/restore.out" || { echo "NG: restore-ok が出なかった(${kind})" >&2; failures=$((failures + 1)); drop_created; continue; }

  for t in $tables; do
    a=$(count "$kind" "$t" "") b=$(count "$drill" "$t" "")
    if [ "$a" = "$b" ]; then echo "ok: ${t} ${a} 行"; else echo "NG: ${t} 元 ${a} 行 / 復元 ${b} 行" >&2; failures=$((failures + 1)); fi
  done
  a=$(count "$kind" devices " WHERE purged_at IS NOT NULL") b=$(count "$drill" devices " WHERE purged_at IS NOT NULL")
  if [ "$a" = "$b" ]; then echo "ok: 墓石(devices.purged_at 有り) ${a} 行"; else echo "NG: 墓石 元 ${a} / 復元 ${b}" >&2; failures=$((failures + 1)); fi

  drop_created
  [ -z "$(sql "SHOW DATABASES LIKE '${drill}'")" ] && echo "ok: 別名 DB ${drill} を削除した" || { echo "NG: 別名 DB ${drill} が残っている" >&2; failures=$((failures + 1)); }
done

[ "$failures" = 0 ] || { echo "db-restore-drill-k3d: ${failures} 件失敗" >&2; exit 1; }
echo "restore-drill-ok ${kinds[*]}"
