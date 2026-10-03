#!/usr/bin/env bash
# scripts/db-restore.sh の自動テスト(P7-4。ADR-0225。make test-scripts から流す)。mysql と失効ジョブは偽物。
# 受け入れ条件: AC-B1(devices の無い世代を拒否)・復元の順序(ADR-0209 §9-3)・AC-B3(JetStream に触れない)。
set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
readonly ROOT
fakepw="FAKESECRET""0123"
readonly SCRIPT="$ROOT/scripts/db-restore.sh"

failures=0
ok() { echo "ok: $1"; }
ng() { echo "NG: $1" >&2; failures=$((failures + 1)); }

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/bin"

# 偽の mysql: 引数と標準入力(SQL)を、呼び出しごとに記録する。
cat > "$work/bin/mysql" <<'FAKE'
#!/usr/bin/env bash
{
  echo "CALL mysql $*"
  if [ ! -t 0 ]; then sed 's/^/  STDIN: /'; fi
} >> "${FAKE_LOG:?}"
exit "${FAKE_MYSQL_RC:-0}"
FAKE
# 偽の失効ジョブ(RESTORE_EXPIRE_CMD)。
cat > "$work/bin/expire" <<'FAKE'
#!/usr/bin/env bash
echo "CALL expire" >> "${FAKE_LOG:?}"
exit "${FAKE_EXPIRE_RC:-0}"
FAKE
chmod +x "$work/bin/mysql" "$work/bin/expire"

backups="$work/backups"
make_gen() { # kind id tables...
  local kind="$1" id="$2"; shift 2
  local dir="$backups/$kind/$id"
  mkdir -p "$dir"
  { for t in "$@"; do echo "CREATE TABLE \`$t\` (id INT);"; done; } | gzip > "$dir/dump.sql.gz"
  printf 'taken_at: 2026-10-01T00:00:00Z\ndb: %s_test\ntables: %s\n' "$kind" "$*" > "$dir/MANIFEST"
}
make_gen record 20261001T000000Z devices purge_journal calc_events favorites frequent_opponents
make_gen record 20261002T000000Z devices purge_journal calc_events favorites frequent_opponents
make_gen record 20260901T000000Z purge_journal calc_events # devices が無い
make_gen team 20261001T000000Z devices purge_journal teams team_members
mkdir -p "$backups/journal"
printf 'dev-a\t2026-10-01T12:00:00.000000Z\ndev-b\t2026-10-03T00:00:00.000000Z\n' > "$backups/journal/record.tsv"

# run_restore [KEY=VAL ...] kind 世代 — rc / out / log を残す。
run_restore() {
  : > "$work/log"
  rc=0
  local envs=()
  while [ "$#" -gt 0 ] && [[ "$1" == *=* ]]; do envs+=("$1"); shift; done
  out=$(cd "$ROOT" && env FAKE_LOG="$work/log" BACKUP_DIR="$backups" MYSQL_BIN="$work/bin/mysql" RESTORE_EXPIRE_CMD="$work/bin/expire" \
    DB_HOST=127.0.0.1 DB_PORT=4000 DB_USER=root DB_NAME=record_test MYSQL_PWD="$fakepw" "${envs[@]}" "$SCRIPT" "$@" 2>&1) || rc=$?
  log=$(cat "$work/log")
}
line_of() { grep -n -m1 -- "$1" "$work/log" | cut -d: -f1; }

# 1) 確認なしでは何もしない(DB を上書きする操作。人間の確認 = CONFIRM_RESTORE に DB 名)
run_restore record 20261001T000000Z
if [ "$rc" -ne 0 ] && [ -z "$log" ]; then ok "CONFIRM_RESTORE が無ければ DB に触れずに失敗する"; else ng "確認なしで動いた(rc=${rc}): ${log}"; fi
run_restore CONFIRM_RESTORE=other_db record 20261001T000000Z
if [ "$rc" -ne 0 ] && [ -z "$log" ]; then ok "CONFIRM_RESTORE が DB 名と違えば失敗する"; else ng "別 DB 名で動いた"; fi

# 2) devices の無い世代は拒否(AC-B1)。DB に触れない
run_restore CONFIRM_RESTORE=record_test record 20260901T000000Z
if [ "$rc" -ne 0 ] && [ -z "$log" ] && echo "$out" | grep -q "devices"; then
  ok "devices を含まない世代の復元を拒否する(AC-B1)。DB は変えない"
else
  ng "devices 無しの世代(rc=${rc}): ${out} / ${log}"
fi
run_restore CONFIRM_RESTORE=record_test record 99999999T000000Z
if [ "$rc" -ne 0 ] && [ -z "$log" ]; then ok "存在しない世代は失敗する"; else ng "存在しない世代で動いた"; fi

# 3) 正常系: 順序 = ダンプ読み込み → journal の取り込み → 墓石・journal の再適用 → 失効ジョブ。成功の印はその後
run_restore CONFIRM_RESTORE=record_test record 20261001T000000Z
l_load=$(line_of 'STDIN: CREATE TABLE `devices`')
l_journal=$(line_of 'INSERT INTO purge_journal')
l_tomb=$(line_of 'DELETE FROM calc_events')
l_expire=$(line_of 'CALL expire')
if [ "$rc" -eq 0 ] && [ -n "$l_load" ] && [ -n "$l_journal" ] && [ -n "$l_tomb" ] && [ -n "$l_expire" ] \
  && [ "$l_load" -lt "$l_journal" ] && [ "$l_journal" -lt "$l_tomb" ] && [ "$l_tomb" -lt "$l_expire" ]; then
  ok "復元の順序: ダンプ → journal の取り込み → 墓石・journal の再適用 → 失効ジョブの強制実行(ADR-0209 §9-3)"
else
  ng "順序が違う/欠けている(rc=${rc} load=${l_load} journal=${l_journal} tomb=${l_tomb} expire=${l_expire}): ${out}"
fi
if grep -q 'dev-a' "$work/log" && grep -q 'dev-b' "$work/log"; then
  ok "journal の全端末(世代取得より後の dev-b を含む)を再適用の対象にする(AC-B2b)"
else
  ng "journal の端末が SQL に無い: ${log}"
fi
if grep -q 'purged_at' "$work/log"; then ok "墓石 devices.purged_at を基準にした再適用 SQL を流す(AC-B2)"; else ng "purged_at を使っていない"; fi
if echo "$out" | tail -1 | grep -q "^restore-ok"; then ok "最後の行(restore-ok)は全工程の成功後にだけ出す(Ready の前提の印)"; else ng "restore-ok が最後に無い: ${out}"; fi
if grep "^CALL" "$work/log" | grep -q "FAKESECRET"; then ng "パスワードの値が引数に出ている"; else ok "パスワードの値を引数に出さない"; fi

# 4) latest は最新の世代を選ぶ
run_restore CONFIRM_RESTORE=record_test record latest
if [ "$rc" -eq 0 ] && echo "$out" | grep -q "20261002T000000Z"; then ok "latest は最新の世代を選ぶ"; else ng "latest(rc=${rc}): ${out}"; fi

# 5) 失敗したら、成功の印を出さず非0(Ready にしてはいけない)
run_restore CONFIRM_RESTORE=record_test FAKE_EXPIRE_RC=1 record 20261001T000000Z
if [ "$rc" -ne 0 ] && ! echo "$out" | grep -q "^restore-ok"; then ok "失効ジョブが失敗したら restore-ok を出さず失敗する"; else ng "失効失敗でも成功扱い(rc=${rc}): ${out}"; fi
run_restore CONFIRM_RESTORE=record_test FAKE_MYSQL_RC=1 record 20261001T000000Z
if [ "$rc" -ne 0 ] && ! grep -q "CALL expire" "$work/log" && ! echo "$out" | grep -q "^restore-ok"; then
  ok "ダンプの読み込みが失敗したら後続を流さず失敗する"
else
  ng "読み込み失敗で続行(rc=${rc})"
fi

# 6) team: 構築の表を再適用の対象にする
printf 'dev-t\t2026-10-03T00:00:00.000000Z\n' > "$backups/journal/team.tsv"
run_restore CONFIRM_RESTORE=team_test DB_NAME=team_test team 20261001T000000Z
if [ "$rc" -eq 0 ] && grep -q 'DELETE FROM team_members' "$work/log" && grep -q 'DELETE FROM teams' "$work/log"; then
  ok "team は teams・team_members を再適用の対象にする"
else
  ng "team の復元(rc=${rc}): ${out} / ${log}"
fi

# 7) pokedex: 墓石も journal も無い。ダンプの読み込みだけ(失効ジョブも流さない)
make_gen pokedex 20261001T000000Z species moves
run_restore CONFIRM_RESTORE=pokedex_test DB_NAME=pokedex_test pokedex 20261001T000000Z
if [ "$rc" -eq 0 ] && ! grep -q 'purge_journal\|CALL expire' "$work/log" && echo "$out" | grep -q "^restore-ok"; then
  ok "pokedex はダンプの読み込みだけで成功する(journal・失効は無い)"
else
  ng "pokedex の復元(rc=${rc}): ${out} / ${log}"
fi

# 8) JetStream は対象外(AC-B3): スクリプトは NATS・kubectl を一切呼ばない(コメント以外に出てこない)
if [ ! -f "$ROOT/scripts/db-restore.sh" ] || [ ! -f "$ROOT/scripts/db-backup.sh" ]; then
  ng "scripts/db-backup.sh / scripts/db-restore.sh が無い"
elif grep -n -i -E 'nats|jetstream|kubectl' "$ROOT/scripts/db-restore.sh" "$ROOT/scripts/db-backup.sh" | grep -v -E ':[0-9]+:[[:space:]]*#' | grep -q .; then
  ng "バックアップ/復元スクリプトが NATS・kubectl を呼んでいる(コメント以外)"
else
  ok "バックアップ/復元スクリプトは JetStream・NATS・kubectl を呼ばない(イベントを再生しない。AC-B3)"
fi

if [ "$failures" -ne 0 ]; then
  echo "db-restore_test: ${failures} 件失敗" >&2
  exit 1
fi
echo "db-restore_test: すべて成功"
