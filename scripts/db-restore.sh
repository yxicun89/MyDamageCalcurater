#!/usr/bin/env bash
# db-restore.sh <kind> <世代 ID|latest> — db-backup.sh の世代から DB を復元する(P7-4。ADR-0225・ADR-0209 §9-3)。
#
# DB の表を置き換える操作なので、CONFIRM_RESTORE=<DB 名>(DB_NAME と同じ値)が無ければ DB に触れずに失敗する。
# record-svc / team-svc は止めた状態で流す(手順は docs/runbooks/data.md)。順序:
#   1) 世代の検証(MANIFEST・dump.sql.gz。record/team は devices を含むこと)  2) ダンプの読み込み
#   3) 保管済み purge journal の取り込み  4) 墓石・journal の再適用(削除済みデータの除去)
#   5) 失効ジョブの強制1回実行(RESTORE_EXPIRE_CMD)  6) 最後の1行に「restore-ok <kind> <世代>」(Ready にしてよい印)
# 入力(環境変数): DB_HOST DB_PORT DB_USER DB_NAME MYSQL_PWD MYSQL_BIN BACKUP_DIR(db-backup.sh と同じ)、
#   CONFIRM_RESTORE、RESTORE_EXPIRE_CMD(record・team で必須。`record expire` / `team expire` を DB へ向けて流すコマンド)。
# 出力は機密を含みうるので、標準出力には世代 ID・DB 名・件数だけを出す。
set -euo pipefail
umask 077

ROOT=$(cd "$(dirname "$0")/.." && pwd)
readonly ROOT
cd "$ROOT"

usage() {
  echo "使い方: CONFIRM_RESTORE=<DB 名> scripts/db-restore.sh <pokedex|record|team> <世代 ID|latest>" >&2
  exit 2
}
die() { echo "db-restore: $*" >&2; exit 1; }

[ "$#" -eq 2 ] || usage
readonly KIND="$1" WANT="$2"
case "$KIND" in pokedex | record | team) ;; *) usage ;; esac
HAS_JOURNAL=0
[ "$KIND" = pokedex ] || HAS_JOURNAL=1

DB_HOST="${DB_HOST:-127.0.0.1}"
DB_PORT="${DB_PORT:-3306}"
DB_USER="${DB_USER:-root}"
DB_NAME="${DB_NAME:-}"
[ -n "$DB_NAME" ] || die "DB_NAME が必要"
readonly MYSQL_BIN="${MYSQL_BIN:-mysql}"
[ "${CONFIRM_RESTORE:-}" = "$DB_NAME" ] || die "DB ${DB_NAME} を上書きする。CONFIRM_RESTORE=${DB_NAME} を付けて再実行する"
if [ "$HAS_JOURNAL" = 1 ] && [ -z "${RESTORE_EXPIRE_CMD:-}" ]; then
  die "RESTORE_EXPIRE_CMD(失効ジョブの強制実行。例: record expire)が必要"
fi

BACKUP_DIR="${BACKUP_DIR:-data/generated/backups}"
case "$BACKUP_DIR" in /*) ;; *) BACKUP_DIR="$ROOT/$BACKUP_DIR" ;; esac
readonly BACKUP_DIR

# 世代の検証。通れば 0、通らなければ理由を標準エラーに出して 1(DB には触れない)。
check_gen() {
  local dir="$BACKUP_DIR/$KIND/$1"
  [ -f "$dir/MANIFEST" ] && [ -f "$dir/dump.sql.gz" ] || { echo "世代 $1: MANIFEST か dump.sql.gz が無い" >&2; return 1; }
  if [ "$HAS_JOURNAL" = 1 ]; then
    gzip -dc "$dir/dump.sql.gz" | grep -q '^CREATE TABLE `devices`' || { echo "世代 $1: devices(墓石)を含まない。復元しない(ADR-0209 §9-1)" >&2; return 1; }
  fi
}

gen=""
if [ "$WANT" = latest ]; then
  for d in $(ls -1 "$BACKUP_DIR/$KIND" 2>/dev/null | grep -E '^[0-9]{8}T[0-9]{6}Z$' | sort -r); do
    if check_gen "$d" 2> /dev/null; then gen="$d"; break; fi
  done
  [ -n "$gen" ] || die "$KIND に検証を通る世代が無い(devices を含む世代が要る)"
else
  [[ "$WANT" =~ ^[0-9]{8}T[0-9]{6}Z$ ]] || die "世代 ID の形式が違う: $WANT"
  check_gen "$WANT" || die "世代 $WANT を復元できない"
  gen="$WANT"
fi

conn=(-h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER")
run_sql() { "$MYSQL_BIN" "${conn[@]}" "$DB_NAME"; } # SQL は標準入力で渡す

echo "restore $KIND $gen -> db=$DB_NAME"

# 2) ダンプの読み込み(mysqldump の DROP TABLE IF EXISTS で、その DB の表を置き換える)
gzip -dc "$BACKUP_DIR/$KIND/$gen/dump.sql.gz" | run_sql || die "ダンプの読み込みに失敗した(以降を流さない)"
echo "loaded dump"

if [ "$HAS_JOURNAL" = 1 ]; then
  # 3) 保管済み journal(世代取得後の削除要求を含む)を DB の purge_journal へ。重複は入れない。
  journal="$BACKUP_DIR/journal/$KIND.tsv"
  n=0
  if [ -s "$journal" ]; then
    sql=$(mktemp)
    trap 'rm -f "$sql"' EXIT
    while IFS=$'\t' read -r dev ts; do
      [ -n "$dev" ] || continue
      [[ "$dev" =~ ^[A-Za-z0-9_-]{1,36}$ ]] || die "journal の device_id が不正(行をスキップせず中止): 形式違反"
      [[ "$ts" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]{1,6})?Z$ ]] || die "journal の requested_at が不正(device_id=${dev})"
      t="${ts/T/ }"
      t="${t%Z}"
      printf "INSERT INTO purge_journal (device_id, requested_at) SELECT '%s', '%s' FROM DUAL WHERE NOT EXISTS (SELECT 1 FROM purge_journal WHERE device_id = '%s' AND requested_at = '%s');\n" "$dev" "$t" "$dev" "$t" >> "$sql"
      n=$((n + 1))
    done < "$journal"
    run_sql < "$sql" || die "journal の取り込みに失敗した"
  fi
  echo "journal merged: ${n} 行"

  # 4) 再適用(どちらも冪等)。a. journal の端末を devices に反映(墓石 purged_at = max(既存, requested_at))
  #    b. 墓石の時刻以前の業務の行を消す(時刻以後は削除後の利用者の操作なので残す。ADR-0209 §5・§7)。
  case "$KIND" in
    record)
      business="DELETE FROM calc_events WHERE EXISTS (SELECT 1 FROM devices d WHERE d.device_id = calc_events.device_id AND d.purged_at IS NOT NULL AND calc_events.occurred_at <= d.purged_at);
DELETE FROM favorites WHERE EXISTS (SELECT 1 FROM devices d WHERE d.device_id = favorites.device_id AND d.purged_at IS NOT NULL AND favorites.created_at <= d.purged_at);
DELETE FROM frequent_opponents WHERE EXISTS (SELECT 1 FROM devices d WHERE d.device_id = frequent_opponents.device_id AND d.purged_at IS NOT NULL);"
      ;;
    team)
      business="DELETE FROM team_members WHERE team_id IN (SELECT t.id FROM teams t JOIN devices d ON d.device_id = t.device_id WHERE d.purged_at IS NOT NULL AND t.created_at <= d.purged_at);
DELETE FROM teams WHERE EXISTS (SELECT 1 FROM devices d WHERE d.device_id = teams.device_id AND d.purged_at IS NOT NULL AND teams.created_at <= d.purged_at);"
      ;;
  esac
  run_sql <<SQL || die "再適用に失敗した"
INSERT INTO devices (device_id, last_seen_at, purged_at)
  SELECT j.device_id, MAX(j.requested_at), MAX(j.requested_at) FROM purge_journal j
  LEFT JOIN devices d ON d.device_id = j.device_id WHERE d.device_id IS NULL GROUP BY j.device_id;
UPDATE devices d JOIN (SELECT device_id, MAX(requested_at) AS m FROM purge_journal GROUP BY device_id) j ON j.device_id = d.device_id
  SET d.purged_at = j.m WHERE d.purged_at IS NULL OR d.purged_at < j.m;
${business}
SQL
  echo "reapplied tombstones and journal"

  # 5) 失効ジョブの強制1回実行(期限切れの行を戻さない)
  sh -c "$RESTORE_EXPIRE_CMD" || die "失効ジョブが失敗した(Ready にしない)"
  echo "expire job done"
fi

echo "restore-ok $KIND $gen"
