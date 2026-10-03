#!/usr/bin/env bash
# db-backup.sh full|journal <kind> — MySQL(pokedex)・TiDB(record・team)の論理バックアップ(P7-4。ADR-0225)。
#
#   full <kind>     mysqldump で世代を取り、record/team は purge journal も同期し、古い世代を消す
#   journal <kind>  purge journal の同期だけ(軽い。定期実行向け。record・team のみ)
#
# 入力(環境変数): DB_HOST DB_PORT DB_USER DB_NAME(必須)、MYSQL_PWD(パスワード。引数には出さない)、
#   MYSQLDUMP_BIN MYSQL_BIN(既定は PATH のもの)、BACKUP_DIR(既定 data/generated/backups。Git の無視対象のみ)、
#   BACKUP_NOW(エポック秒。テスト用)、BACKUP_RETENTION_DAYS(既定 30)、JOURNAL_RETENTION_DAYS(既定 90)。
# 出力は機密(端末 ID・保存データ)。標準出力には件数・世代 ID・DB 名・表名だけを出す。
set -euo pipefail
umask 077

ROOT=$(cd "$(dirname "$0")/.." && pwd)
readonly ROOT
cd "$ROOT"

usage() {
  echo "使い方: scripts/db-backup.sh full <pokedex|record|team> | journal <record|team>" >&2
  exit 2
}
die() { echo "db-backup: $*" >&2; exit 1; }

[ "$#" -eq 2 ] || usage
readonly MODE="$1" KIND="$2"
case "$MODE" in full | journal) ;; *) usage ;; esac
case "$KIND" in
  pokedex | record | team) ;;
  *) usage ;;
esac
HAS_JOURNAL=0
[ "$KIND" = pokedex ] || HAS_JOURNAL=1
if [ "$MODE" = journal ] && [ "$HAS_JOURNAL" = 0 ]; then
  die "pokedex に purge journal は無い(journal は record・team のみ)"
fi

DB_HOST="${DB_HOST:-127.0.0.1}"
DB_PORT="${DB_PORT:-3306}"
DB_USER="${DB_USER:-root}"
DB_NAME="${DB_NAME:-}"
[ -n "$DB_NAME" ] || die "DB_NAME が必要"
readonly MYSQLDUMP_BIN="${MYSQLDUMP_BIN:-mysqldump}" MYSQL_BIN="${MYSQL_BIN:-mysql}"
readonly BACKUP_RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-30}" JOURNAL_RETENTION_DAYS="${JOURNAL_RETENTION_DAYS:-90}"
NOW="${BACKUP_NOW:-$(date +%s)}"
readonly NOW
case "$NOW$BACKUP_RETENTION_DAYS$JOURNAL_RETENTION_DAYS" in *[!0-9]*) die "BACKUP_NOW・保持日数は整数" ;; esac

# 保存先: リポジトリ内なら Git の無視対象に限る(機密を Git に入れない)。リポジトリ外は許す。
BACKUP_DIR="${BACKUP_DIR:-data/generated/backups}"
case "$BACKUP_DIR" in /*) ;; *) BACKUP_DIR="$ROOT/$BACKUP_DIR" ;; esac
readonly BACKUP_DIR
case "$BACKUP_DIR/" in
  "$ROOT"/*)
    git check-ignore -q -- "$BACKUP_DIR/$KIND/x" ||
      die "保存先 $BACKUP_DIR は Git の無視対象でない(機密をコミットしない。data/generated/ 以下かリポジトリ外にする)"
    ;;
esac

# 途中で失敗しても一時ファイル・一時ディレクトリを残さない(EXIT で消す)。
cleanup_paths=()
cleanup() { [ "${#cleanup_paths[@]}" -eq 0 ] || rm -rf "${cleanup_paths[@]}"; }
trap cleanup EXIT

epoch_fmt() { # epoch format
  date -u -r "$1" "+$2" 2>/dev/null || date -u -d "@$1" "+$2"
}
conn=(-h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER")

mkdir -p "$BACKUP_DIR/$KIND"
chmod 700 "$BACKUP_DIR" "$BACKUP_DIR/$KIND" 2>/dev/null || true

# ---- purge journal の同期(追記のみ。世代とは別の場所。保持は JOURNAL_RETENTION_DAYS)
sync_journal() {
  local dir="$BACKUP_DIR/journal" file="$BACKUP_DIR/journal/$KIND.tsv" rows tmp cutoff before after
  mkdir -p "$dir"
  chmod 700 "$dir"
  [ -f "$file" ] || { : > "$file"; chmod 600 "$file"; }
  rows=$(mktemp "$dir/.sync-rows.XXXXXX")
  tmp=$(mktemp "$dir/.sync-new.XXXXXX")
  cleanup_paths+=("$rows" "$tmp")
  "$MYSQL_BIN" "${conn[@]}" -N -B "$DB_NAME" \
    -e "SELECT device_id, DATE_FORMAT(requested_at, '%Y-%m-%dT%H:%i:%s.%fZ') FROM purge_journal ORDER BY id" > "$rows"
  before=$(wc -l < "$file" | tr -d ' ')
  cutoff=$(epoch_fmt $((NOW - JOURNAL_RETENTION_DAYS * 86400)) %Y-%m-%dT%H:%M:%S.000000Z)
  # 既存行は不変のまま、無い行だけを末尾に足し、保持期間より古い行(ISO 文字列の辞書順 = 時刻順)だけを取り除く。
  awk -F'\t' -v cutoff="$cutoff" -v existing="$file" '
    FILENAME == existing { if ($0 != "") { seen[$0] = 1; if ($2 >= cutoff) print } next }
    $0 != "" && !($0 in seen) { seen[$0] = 1; if ($2 >= cutoff) print }
  ' "$file" "$rows" > "$tmp"
  mv "$tmp" "$file"
  chmod 600 "$file"
  after=$(wc -l < "$file" | tr -d ' ')
  echo "journal $KIND: ${before} -> ${after} 行 (${file})"
}

if [ "$MODE" = journal ]; then
  sync_journal
  exit 0
fi

# ---- full: 世代の取得
gen=$(epoch_fmt "$NOW" %Y%m%dT%H%M%SZ)
final="$BACKUP_DIR/$KIND/$gen"
tmpdir="$BACKUP_DIR/$KIND/.tmp-$gen-$$"
cleanup_paths+=("$tmpdir")
mkdir -p "$tmpdir"
chmod 700 "$tmpdir"

# TiDB(record・team)は UNLOCK TABLES が開いているトランザクションを確定させ、mysqldump の SAVEPOINT が消えて失敗する。
# autocommit=0 で接続すると、その後の SAVEPOINT から最後までが1つのトランザクション(= 一貫した読み取り)になる。
dump_extra=()
[ "$HAS_JOURNAL" = 0 ] || dump_extra=(--init-command="SET autocommit=0")
"$MYSQLDUMP_BIN" "${conn[@]}" --single-transaction --no-tablespaces --set-gtid-purged=OFF ${dump_extra[@]+"${dump_extra[@]}"} "$DB_NAME" | gzip > "$tmpdir/dump.sql.gz" ||
  die "mysqldump が失敗した(世代を残さない)"
chmod 600 "$tmpdir/dump.sql.gz"

tables=$(gzip -dc "$tmpdir/dump.sql.gz" | sed -n 's/^CREATE TABLE `\([^`]*\)`.*/\1/p' | tr '\n' ' ' | sed 's/ $//')
if [ "$HAS_JOURNAL" = 1 ]; then
  case " $tables " in
    *" devices "*) ;;
    *) die "ダンプに devices(墓石)が無い。世代を残さない(ADR-0209 §9-1)" ;;
  esac
fi
printf 'taken_at: %s\ndb: %s\ntables: %s\n' "$(epoch_fmt "$NOW" %Y-%m-%dT%H:%M:%SZ)" "$DB_NAME" "$tables" > "$tmpdir/MANIFEST"
chmod 600 "$tmpdir/MANIFEST"
rm -rf "$final" # 同じ秒の再実行は、検証を通った新しい世代で置き換える
mv "$tmpdir" "$final"
echo "backup $KIND $gen db=$DB_NAME tables=$tables"

[ "$HAS_JOURNAL" = 0 ] || sync_journal

# ---- 世代の失効(その kind の世代だけ。判定は世代 ID の時刻)
old_id=$(epoch_fmt $((NOW - BACKUP_RETENTION_DAYS * 86400)) %Y%m%dT%H%M%SZ)
for d in "$BACKUP_DIR/$KIND"/*/; do
  id=$(basename "$d")
  [[ "$id" =~ ^[0-9]{8}T[0-9]{6}Z$ ]] || continue
  if [[ "$id" < "$old_id" ]]; then
    rm -rf "${d%/}"
    echo "expired $KIND $id"
  fi
done
