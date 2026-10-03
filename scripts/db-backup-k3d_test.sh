#!/usr/bin/env bash
# scripts/db-backup-k3d.sh・db-restore-drill-k3d.sh・k3d-db-lib.sh の静的検査(P7-4。ADR-0227。make test-scripts から流す)。
# 実クラスタには触らない(実行の確認は ADR-0227 §7 の実クラスタでの記録)。確かめること:
#   set -euo pipefail・ルートへの cd・require-k3d-context・値を出さない(Secret の値を echo/printf/ps に流さない)・
#   別名 DB 名の厳密な検査・DROP は別名 DB 変数だけ・MYSQL_PWD は環境変数で渡す。
set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
readonly ROOT
failures=0
ok() { echo "ok: $1"; }
ng() { echo "NG: $1" >&2; failures=$((failures + 1)); }
has() { grep -Eq -- "$2" "$1"; } # file regex

backup="$ROOT/scripts/db-backup-k3d.sh"
drill="$ROOT/scripts/db-restore-drill-k3d.sh"
lib="$ROOT/scripts/k3d-db-lib.sh"

for f in "$backup" "$drill" "$lib"; do
  n=$(basename "$f")
  [ -f "$f" ] || { ng "$n が無い"; continue; }
  bash -n "$f" && ok "$n の構文" || ng "$n の構文エラー"
  has "$f" '^# ' && ok "$n に説明コメント" || ng "$n に説明コメントが無い"
done
for f in "$backup" "$drill"; do
  n=$(basename "$f")
  [ -x "$f" ] && ok "$n は実行可能" || ng "$n が実行可能でない"
  has "$f" '^set -euo pipefail' && ok "$n: set -euo pipefail" || ng "$n: set -euo pipefail が無い"
  has "$f" '^cd "\$\(git rev-parse --show-toplevel\)"' && ok "$n: ルートへ cd" || ng "$n: ルートへの cd が無い"
  has "$f" 'kdb_init (db-backup-k3d|db-restore-drill-k3d)' && ok "$n: kdb_init を呼ぶ" || ng "$n: kdb_init が無い"
  has "$f" 'trap .*EXIT' && ok "$n: EXIT で後始末" || ng "$n: EXIT trap が無い"
  has "$f" 'umask 077' && ok "$n: umask 077" || ng "$n: umask 077 が無い"
  # 値を出さない: dsn / root_pw / MYSQL_PWD の値を echo・printf で出していない。
  if grep -nE '(echo|printf)[^|]*\$\{?(dsn|root_pw|MYSQL_PWD)' "$f" >/dev/null; then ng "$n: 秘密の値を出力しうる"; else ok "$n: 秘密の値を出力しない"; fi
  if grep -nE '(-p"?\$|--password)' "$f" >/dev/null; then ng "$n: パスワードをコマンド行に渡している"; else ok "$n: パスワードをコマンド行に渡さない"; fi
  if grep -nE '\bset -x\b' "$f" >/dev/null; then ng "$n: set -x がある"; else ok "$n: set -x が無い"; fi
done

# lib: require-k3d-context・docker ラッパーは値を渡さず継承(-e MYSQL_PWD)。
has "$lib" 'require-k3d-context.sh' && ok "lib: require-k3d-context を呼ぶ" || ng "lib: require-k3d-context が無い"
has "$lib" '-e MYSQL_PWD ' && ok "lib: docker は -e MYSQL_PWD(値なし)で継承" || ng "lib: docker の MYSQL_PWD 渡しが不正"
if grep -nE 'MYSQL_PWD=[^ ]' "$lib" >/dev/null; then ng "lib: MYSQL_PWD に値を埋めている"; else ok "lib: MYSQL_PWD に値を埋めない"; fi
if grep -nE 'jsonpath.*>&2|base64 -d *>&2' "$lib" >/dev/null; then ng "lib: Secret を標準エラーへ出しうる"; else ok "lib: Secret を標準エラーへ出さない"; fi

# backup: 3 kind・非破壊(DROP・DELETE・TRUNCATE・復元が無い)・db-backup.sh full を流す。
has "$backup" 'db-backup.sh full' && ok "backup: db-backup.sh full を流す" || ng "backup: db-backup.sh full が無い"
if grep -nEi 'DROP |DELETE |TRUNCATE |db-restore' "$backup" >/dev/null; then ng "backup: 破壊的な操作を含む"; else ok "backup: 非破壊(DROP・DELETE・TRUNCATE・復元なし)"; fi
has "$backup" 'record \| team \| pokedex' && ok "backup: record・team・pokedex を受ける" || ng "backup: kind の検査が無い"

# drill: 別名 DB 名の厳密な検査・DROP は created_db のみ・既存の別名 DB は消さない・CONFIRM_RESTORE は別名 DB。
has "$drill" '\^\(record\|team\)_restore_drill\$' && ok "drill: 別名 DB 名を厳密に検査" || ng "drill: 別名 DB 名の検査が無い"
if grep -n 'DROP DATABASE' "$drill" | grep -v '^[0-9]*:#' | grep -v 'created_db' >/dev/null; then ng "drill: created_db 以外への DROP がありうる"; else ok "drill: DROP は created_db(自分が作った別名 DB)だけ"; fi
has "$drill" 'SHOW DATABASES LIKE' && ok "drill: 既存の別名 DB を検査して消さない" || ng "drill: 既存の別名 DB の検査が無い"
has "$drill" 'CONFIRM_RESTORE="\$drill"' && ok "drill: CONFIRM_RESTORE は別名 DB" || ng "drill: CONFIRM_RESTORE が別名 DB でない"
has "$drill" 'RESTORE_FROM_DB="\$kind"' && ok "drill: RESTORE_FROM_DB は元 DB" || ng "drill: RESTORE_FROM_DB が無い"
has "$drill" 'restore-ok' && ok "drill: restore-ok を確認する" || ng "drill: restore-ok の確認が無い"
has "$drill" 'purged_at IS NOT NULL' && ok "drill: 墓石を照合する" || ng "drill: 墓石の照合が無い"
if grep -nE 'DROP DATABASE[^;]*\$\{?kind' "$drill" >/dev/null; then ng "drill: 元 DB(kind)を DROP しうる"; else ok "drill: 元 DB を DROP しない"; fi
has "$drill" 'tidb-root-auth' && ok "drill: root は tidb-root-auth から(内部で読む)" || ng "drill: root の取得元が不明"

# Makefile
has "$ROOT/Makefile" '^db-backup-k3d:' && ok "Makefile: db-backup-k3d" || ng "Makefile: db-backup-k3d が無い"
has "$ROOT/Makefile" '^db-restore-drill-k3d:' && ok "Makefile: db-restore-drill-k3d" || ng "Makefile: db-restore-drill-k3d が無い"

[ "$failures" -eq 0 ] || { echo "db-backup-k3d_test: ${failures} 件失敗" >&2; exit 1; }
echo "db-backup-k3d_test: all ok"
