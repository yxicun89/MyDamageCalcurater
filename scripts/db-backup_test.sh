#!/usr/bin/env bash
# scripts/db-backup.sh の自動テスト(P7-4。ADR-0225。make test-scripts から流す)。
# mysqldump・mysql は偽物に差し替える(実 DB は使わない。実 DB の往復は scripts/db-backup-restore_docker_test.sh)。
# 受け入れ条件: AC-B1(devices を含む)・保存先の機密扱い・世代30日・purge journal の別保持(90日・追記のみ)。
set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
readonly ROOT
fakepw="FAKESECRET""0123"
readonly SCRIPT="$ROOT/scripts/db-backup.sh"

failures=0
ok() { echo "ok: $1"; }
ng() { echo "NG: $1" >&2; failures=$((failures + 1)); }

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/bin"

# 偽の mysqldump: 引数を記録し、FAKE_DUMP_TABLES(空白区切り)の CREATE TABLE を出す。パスワードは環境変数 MYSQL_PWD で受ける。
cat > "$work/bin/mysqldump" <<'FAKE'
#!/usr/bin/env bash
echo "mysqldump $* PWD_SET=${MYSQL_PWD:+yes}" >> "${FAKE_LOG:?}"
for t in ${FAKE_DUMP_TABLES:-devices purge_journal calc_events}; do
  echo "CREATE TABLE \`$t\` (id INT);"
  echo "INSERT INTO \`$t\` VALUES (1);"
done
FAKE
# 偽の mysql: purge_journal の SELECT には FAKE_JOURNAL_ROWS(TSV: device_id<TAB>requested_at)を返す。
cat > "$work/bin/mysql" <<'FAKE'
#!/usr/bin/env bash
echo "mysql $*" >> "${FAKE_LOG:?}"
case "$*" in
  *purge_journal*) [ -f "${FAKE_JOURNAL_ROWS:-/nonexistent}" ] && cat "$FAKE_JOURNAL_ROWS" ;;
esac
exit 0
FAKE
chmod +x "$work/bin/mysqldump" "$work/bin/mysql"

# run_backup 引数... — EXTRA_ENV(BACKUP_NOW=エポック秒で「今」を固定など)を付けて流す。rc / out / log を残す。
run_backup() {
  : > "$work/log"
  rc=0
  out=$(cd "$ROOT" && env FAKE_LOG="$work/log" BACKUP_DIR="$work/backups" MYSQLDUMP_BIN="$work/bin/mysqldump" MYSQL_BIN="$work/bin/mysql" \
    DB_HOST=127.0.0.1 DB_PORT=4000 DB_USER=root DB_NAME=record_test MYSQL_PWD="$fakepw" \
    "${EXTRA_ENV[@]}" "$SCRIPT" "$@" 2>&1) || rc=$?
  log=$(cat "$work/log")
}
gen_id() { date -u -r "$1" +%Y%m%dT%H%M%SZ 2>/dev/null || date -u -d "@$1" +%Y%m%dT%H%M%SZ; }
iso_us() { date -u -r "$1" +%Y-%m-%dT%H:%M:%S.000000Z 2>/dev/null || date -u -d "@$1" +%Y-%m-%dT%H:%M:%S.000000Z; }
mode_of() { stat -f %Lp "$1" 2>/dev/null || stat -c %a "$1"; }

day=86400

# 1) 使い方の誤りは非0(kind が不明。journal は record/team だけ)
EXTRA_ENV=(BACKUP_NOW=1790000000)
run_backup full unknown
if [ "$rc" -ne 0 ]; then ok "不明な kind は失敗する"; else ng "不明な kind が通った: ${out}"; fi
run_backup journal pokedex
if [ "$rc" -ne 0 ]; then ok "pokedex に purge journal は無いので journal は失敗する"; else ng "pokedex の journal が通った"; fi

# 2) 保存先は .gitignore 済みの場所に限る(機密を Git に入れない)。リポジトリ内の無視されない場所は拒否。
: > "$work/log"
rc=0
out=$(cd "$ROOT" && env FAKE_LOG="$work/log" BACKUP_DIR="$ROOT/docs/backups-not-ignored" MYSQLDUMP_BIN="$work/bin/mysqldump" MYSQL_BIN="$work/bin/mysql" \
  DB_HOST=h DB_PORT=1 DB_USER=u DB_NAME=record_test MYSQL_PWD=x "$SCRIPT" full record 2>&1) || rc=$?
if [ "$rc" -ne 0 ] && [ ! -e "$ROOT/docs/backups-not-ignored" ]; then
  ok "Git の無視対象でないリポジトリ内の保存先は拒否する"
else
  ng "無視されない保存先を許した(rc=${rc}): ${out}"
  rm -rf "$ROOT/docs/backups-not-ignored"
fi
if (cd "$ROOT" && git check-ignore -q data/generated/backups/record/x/dump.sql.gz); then
  ok "既定の保存先 data/generated/backups/ は .gitignore 済み"
else
  ng "data/generated/backups/ が .gitignore に無い"
fi

# 3) 通常のバックアップ(record): 世代ディレクトリ・MANIFEST・devices を含む・パスワードは argv に出さない・権限
run_backup full record
gen="$work/backups/record/$(gen_id 1790000000)"
if [ "$rc" -eq 0 ] && [ -f "$gen/dump.sql.gz" ] && [ -f "$gen/MANIFEST" ]; then
  ok "世代ディレクトリに dump.sql.gz と MANIFEST を作る"
else
  ng "世代の成果物が無い(rc=${rc}): ${out} / $(ls -R "$work/backups" 2>&1)"
fi
if grep -q "^tables:.*devices" "$gen/MANIFEST" 2>/dev/null && grep -q "^taken_at:" "$gen/MANIFEST" 2>/dev/null && grep -q "^db:" "$gen/MANIFEST" 2>/dev/null; then
  ok "MANIFEST に taken_at・db・tables(devices を含む)を書く(AC-B1)"
else
  ng "MANIFEST の内容: $(cat "$gen/MANIFEST" 2>&1)"
fi
if echo "$log" | grep "^mysqldump" | grep -q -- "--single-transaction" && ! echo "$log" | grep "^mysqldump" | grep -q -- "--ignore-table"; then
  ok "一貫したスナップショット(--single-transaction)で、テーブルを除外しない"
else
  ng "mysqldump の引数: ${log}"
fi
if echo "$log" | grep -q "FAKESECRET"; then ng "パスワードの値がコマンドライン引数に出ている"; else ok "パスワードの値を引数に出さない(環境変数 MYSQL_PWD)"; fi
if [ "$(mode_of "$gen")" = "700" ] && [ "$(mode_of "$gen/dump.sql.gz")" = "600" ]; then
  ok "世代は 700・ダンプは 600(出力は機密)"
else
  ng "権限 dir=$(mode_of "$gen") file=$(mode_of "$gen/dump.sql.gz")"
fi
if gzip -dc "$gen/dump.sql.gz" 2>/dev/null | grep -q 'CREATE TABLE `devices`'; then ok "ダンプに devices(墓石)が入る"; else ng "ダンプに devices が無い"; fi

# 4) devices の無いダンプは失敗し、世代を残さない(AC-B1)
EXTRA_ENV=(BACKUP_NOW=1790003600 FAKE_DUMP_TABLES="calc_events purge_journal")
run_backup full record
if [ "$rc" -ne 0 ] && [ ! -e "$work/backups/record/$(gen_id 1790003600)" ]; then
  ok "devices を含まない record のダンプは失敗し、世代を残さない"
else
  ng "devices 無しを許した(rc=${rc}): ${out}"
fi
run_backup full team
if [ "$rc" -ne 0 ]; then ok "team でも devices が無ければ失敗する"; else ng "team で devices 無しを許した"; fi
EXTRA_ENV=(BACKUP_NOW=1790007200 FAKE_DUMP_TABLES="species moves")
run_backup full pokedex
if [ "$rc" -eq 0 ] && [ -f "$work/backups/pokedex/$(gen_id 1790007200)/dump.sql.gz" ]; then
  ok "pokedex は devices 無しでも取れる(墓石を持たない)"
else
  ng "pokedex のバックアップ(rc=${rc}): ${out}"
fi

# 5) 世代30日: 31日前の世代は消え、29日前は残る。他の kind の世代には触れない。
now=1800000000
mkdir -p "$work/backups/record/$(gen_id $((now - 31 * day)))" "$work/backups/record/$(gen_id $((now - 29 * day)))" "$work/backups/team/$(gen_id $((now - 31 * day)))"
EXTRA_ENV=(BACKUP_NOW=$now)
run_backup full record
if [ ! -e "$work/backups/record/$(gen_id $((now - 31 * day)))" ] && [ -e "$work/backups/record/$(gen_id $((now - 29 * day)))" ]; then
  ok "31日前の世代を消し、29日前の世代は残す(世代30日)"
else
  ng "世代の削除: $(ls "$work/backups/record")"
fi
if [ -e "$work/backups/team/$(gen_id $((now - 31 * day)))" ]; then ok "他の kind の世代は、その kind の実行では消さない"; else ng "別 kind の世代を消した"; fi
if [ -e "$work/backups/record/$(gen_id $now)/dump.sql.gz" ]; then ok "今回取った世代は消さない"; else ng "今回の世代が無い"; fi

# 6) purge journal の同期: 世代と別の場所(journal/)に追記のみ。重複しない・既存行を書き換えない
printf 'dev-a\t2026-10-01T00:00:00.000000Z\ndev-b\t2026-10-02T00:00:00.000000Z\n' > "$work/rows1"
EXTRA_ENV=(BACKUP_NOW=$now FAKE_JOURNAL_ROWS="$work/rows1")
run_backup journal record
journal="$work/backups/journal/record.tsv"
if [ "$rc" -eq 0 ] && [ "$(wc -l < "$journal" | tr -d ' ')" = 2 ]; then
  ok "journal を世代ディレクトリとは別の場所(journal/record.tsv)に書く"
else
  ng "journal 同期(rc=${rc}): ${out} / $(cat "$journal" 2>&1)"
fi
head_before=$(cat "$journal" 2>/dev/null)
printf 'dev-a\t2026-10-01T00:00:00.000000Z\ndev-b\t2026-10-02T00:00:00.000000Z\ndev-c\t2026-10-03T00:00:00.000000Z\n' > "$work/rows2"
EXTRA_ENV=(BACKUP_NOW=$now FAKE_JOURNAL_ROWS="$work/rows2")
run_backup journal record
if [ "$(wc -l < "$journal" | tr -d ' ')" = 3 ] && [ "$(head -2 "$journal")" = "$head_before" ] && [ "$(tail -1 "$journal" | cut -f1)" = dev-c ]; then
  ok "2回目は新しい行だけを末尾に足す(重複なし・既存行は不変)"
else
  ng "追記のみになっていない: $(cat "$journal")"
fi
: > "$work/rows3"
EXTRA_ENV=(BACKUP_NOW=$now FAKE_JOURNAL_ROWS="$work/rows3")
run_backup journal record
if [ "$(wc -l < "$journal" | tr -d ' ')" = 3 ]; then ok "DB 側の journal が空でも、保持した行は消えない"; else ng "journal の行が減った: $(cat "$journal")"; fi
printf 'dev-d\t2026-10-04T00:00:00.000000Z\n' > "$work/rows4"
EXTRA_ENV=(BACKUP_NOW=$now FAKE_JOURNAL_ROWS="$work/rows4")
run_backup full record
if grep -q '^dev-d' "$journal"; then ok "full の実行でも journal を同期する"; else ng "full が journal を同期していない"; fi

# 7) journal の保持90日: 91日前の行は消え、89日前は残る
printf 'dev-old\t%s\ndev-fresh\t%s\n' "$(iso_us $((now - 91 * day)))" "$(iso_us $((now - 89 * day)))" >> "$journal"
EXTRA_ENV=(BACKUP_NOW=$now)
run_backup journal record
if ! grep -q '^dev-old' "$journal" && grep -q '^dev-fresh' "$journal"; then
  ok "journal は91日前の行を消し、89日前の行は残す(保持90日。世代30日より長い)"
else
  ng "journal の保持: $(cat "$journal")"
fi
if [ "$(mode_of "$work/backups/journal")" = 700 ] && [ "$(mode_of "$journal")" = 600 ]; then ok "journal ディレクトリは 700・ファイルは 600"; else ng "journal の権限"; fi

if [ "$failures" -ne 0 ]; then
  echo "db-backup_test: ${failures} 件失敗" >&2
  exit 1
fi
echo "db-backup_test: すべて成功"
