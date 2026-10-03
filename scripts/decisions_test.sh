#!/usr/bin/env bash
# scripts/new-decision.sh と scripts/list-decisions.sh の自動テスト(ADR-0170)。make test-scripts から流す。
# 使い捨てのディレクトリにスクリプトをコピーして流す(このリポジトリの docs/decisions/ の中身に依存しない)。
set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
readonly ROOT

failures=0
ok() { echo "ok: $1"; }
ng() { echo "NG: $1" >&2; failures=$((failures + 1)); }

repo=$(mktemp -d)
trap 'rm -rf "$repo"' EXIT
mkdir -p "$repo/scripts" "$repo/docs/ai-shared"
cp "$ROOT/scripts/new-decision.sh" "$ROOT/scripts/list-decisions.sh" "$repo/scripts/"

new() { (cd "$repo" && DECISIONS_DATE=2026-10-03 ./scripts/new-decision.sh "$@"); }
list() { (cd "$repo" && ./scripts/list-decisions.sh "$@"); }

# --- new-decision.sh ---
out=$(new judge move-target "技の対象をマスタに" 2>/dev/null)
f="$repo/docs/decisions/2026-10-03-judge-move-target.md"
[ "$out" = "docs/decisions/2026-10-03-judge-move-target.md" ] && ok "作ったファイルのパスを出す" || ng "出力: ${out}"
[ -f "$f" ] && ok "docs/decisions/<日付>-<lane>-<slug>.md を作る" || ng "ファイルが無い: $f"
grep -qx '# 技の対象をマスタに' "$f" && ok "タイトルを見出しにする" || ng "見出しが無い"
for field in '- 日付: 2026-10-03' '- レーン: judge' '- 状態: open' '- 状況:' '- 既定案:'; do
  grep -q -- "^${field}" "$f" && ok "項目「${field}」がある" || ng "項目「${field}」が無い"
done

before=$(cat "$f")
if new judge move-target >/dev/null 2>&1; then ng "既にあるファイルでも成功した"; else ok "既にあるファイルは作らずに失敗する"; fi
[ "$(cat "$f")" = "$before" ] && ok "既存のエントリを上書きしない" || ng "既存のエントリが書き換わった"

if new nolane some-slug >/dev/null 2>&1; then ng "不明なレーンで成功した"; else ok "不明なレーンは失敗する"; fi
if new api Bad_Slug >/dev/null 2>&1; then ng "不正な slug で成功した"; else ok "不正な slug は失敗する"; fi
if new api >/dev/null 2>&1; then ng "引数不足で成功した"; else ok "引数不足は失敗する"; fi
if (cd "$repo" && DECISIONS_DATE=20261003 ./scripts/new-decision.sh api x >/dev/null 2>&1); then ng "不正な日付で成功した"; else ok "不正な日付は失敗する"; fi
[ "$(ls "$repo/docs/decisions" | wc -l | tr -d ' ')" = 1 ] && ok "失敗したときはファイルを作らない" || ng "余計なファイル: $(ls "$repo/docs/decisions")"

# --- list-decisions.sh ---
new shared answered "決定済み" >/dev/null
sed -i.bak 's/^- 状態: open$/- 状態: 決定/' "$repo/docs/decisions/2026-10-03-shared-answered.md" && rm -f "$repo/docs/decisions/"*.bak
new web waiting "回答待ち" >/dev/null
sed -i.bak 's/^- 状態: open$/- 状態: 未回答(iOS レーン)/' "$repo/docs/decisions/2026-10-03-web-waiting.md" && rm -f "$repo/docs/decisions/"*.bak
printf '# 状態を書き忘れた\n\n本文\n' > "$repo/docs/decisions/2026-10-03-api-nostate.md"
printf '# docs/decisions/ の書き方\n' > "$repo/docs/decisions/README.md"

open_out=$(list)
echo "$open_out" | grep -q $'2026-10-03-judge-move-target.md\topen\t技の対象をマスタに' && ok "open を出す" || ng "open が無い: ${open_out}"
echo "$open_out" | grep -q $'web-waiting.md\t未回答(iOS レーン)\t回答待ち' && ok "未回答を出す" || ng "未回答が無い: ${open_out}"
echo "$open_out" | grep -q 'api-nostate.md'$'\t''(状態なし)' && ok "状態の行が無いものも出す" || ng "状態なしが無い: ${open_out}"
echo "$open_out" | grep -q 'shared-answered' && ng "決定済みを既定で出した" || ok "決定済みは既定で出さない"
echo "$open_out" | grep -q 'README' && ng "README を出した" || ok "README は出さない"

all_out=$(list --all)
[ "$(echo "$all_out" | wc -l | tr -d ' ')" = 4 ] && ok "--all は README 以外の全件" || ng "--all の件数: ${all_out}"

printf '# Decisions\n\n> 凍結\n\n## 2026-09-21: 一つ目\n本文\n## 2026-09-22: 二つ目\n' > "$repo/docs/ai-shared/DECISIONS.md"
arch_out=$(list --archive)
[ "$arch_out" = $'5:## 2026-09-21: 一つ目\n7:## 2026-09-22: 二つ目' ] && ok "--archive は見出しを行番号つきで出す" || ng "--archive の出力: ${arch_out}"

if list --bogus >/dev/null 2>&1; then ng "不明な引数で成功した"; else ok "不明な引数は失敗する"; fi
rm -rf "$repo/docs/decisions"
[ -z "$(list)" ] && ok "docs/decisions/ が無ければ何も出さない" || ng "空のときの出力: $(list)"

if [ "$failures" -ne 0 ]; then
  echo "decisions_test: ${failures} 件失敗" >&2
  exit 1
fi
echo "decisions_test: すべて成功"
