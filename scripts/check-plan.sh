#!/usr/bin/env bash
# docs/plan.md(索引)と docs/plan/(区画のファイル)の整合を検査する(ADR-0172)。
#   1. 索引からリンクされた docs/plan/ 配下のファイル・ディレクトリが存在する
#   2. docs/plan/ 配下の Markdown が、索引のリンク(ファイルか、それを含むディレクトリ)から辿れる
#   3. docs/plan.md にチェックボックスの行が無い(タスクの行は区画のファイルに置く)
#   4. 同じタスク(状態の印を除いた本文)の行が 2 回現れない(マージの解消で行が二重になる事故)
#   5. 行頭のタスク ID が重複しない(分割前から重複していた ID だけ許可リストに置く)
# 使い方: scripts/check-plan.sh [リポジトリのルート](省略時はこのスクリプトの 1 つ上)
set -euo pipefail
root=${1:-"$(cd "$(dirname "$0")/.." && pwd)"}
index="$root/docs/plan.md"
dir="$root/docs/plan"

# 分割前(ADR-0172 の時点)の main で ID が重複していたもの。内容を変えずに移したので許可する。新しく足さない。
# P6-24 は main で同じ行が [ ] と [x] の 2 回ある(マージで古い行が残った)。iOS レーンが古い行を消したらここから外す。
# 許可した ID の行は、4(同じ本文の行)の検査からも外す。
allowed_dup_ids=" P4-17 P5-5c P5-5d P6-21 P6-24 "

errors=0
fail() {
  echo "check-plan: $*" >&2
  errors=$((errors + 1))
}

if [ ! -f "$index" ] || [ ! -d "$dir" ]; then
  echo "check-plan: $index または $dir がありません" >&2
  exit 1
fi

# 1. 索引のリンク先が存在する。
links=$(grep -oE '\]\(plan/[^)]*\)' "$index" | sed -E 's/^\]\(//; s/\)$//' | sort -u || true)
if [ -z "$links" ]; then
  fail "docs/plan.md に区画のファイルへのリンク(](plan/…))がありません"
fi
for link in $links; do
  [ -e "$root/docs/$link" ] || fail "docs/plan.md の索引のリンク先がありません: docs/$link"
done

# 2. docs/plan/ 配下の Markdown が索引から辿れる。
while IFS= read -r file; do
  rel=${file#"$root/docs/"}
  reached=0
  for link in $links; do
    case "$link" in
      */) case "$rel" in "$link"*) reached=1 ;; esac ;;
      *) [ "$rel" = "$link" ] && reached=1 ;;
    esac
  done
  [ "$reached" -eq 1 ] || fail "docs/$rel が docs/plan.md の索引から辿れません(索引の表に足す)"
done < <(find "$dir" -type f -name '*.md' | sort)

# 3. 索引にチェックボックスの行が無い。
if grep -nE '^[[:space:]]*- \[.\] ' "$index" >/dev/null; then
  fail "docs/plan.md にチェックボックスの行があります(区画のファイル docs/plan/<区画>.md に置く):"
  grep -nE '^[[:space:]]*- \[.\] ' "$index" >&2
fi

# 区画のファイルのチェックボックスの行を「ファイル:行番号<TAB>本文(状態の印を除く)」で集める。
items=$(find "$dir" -type f -name '*.md' | sort | while IFS= read -r file; do
  awk -v f="${file#"$root/"}" '
    /^[[:space:]]*- \[.\] / {
      body = $0
      sub(/^[[:space:]]*- \[.\] /, "", body)
      print f ":" NR "\t" body
    }' "$file"
done)

# 4. 同じ本文の行が 2 回現れない。
dups=$(printf '%s\n' "$items" | awk -F'\t' -v allowed="$allowed_dup_ids" 'NF > 1 {
    body = $2
    sub(/^\*\*/, "", body)
    split(body, w, /[^0-9A-Za-z-]/)
    if (index(allowed, " " w[1] " ") > 0) next
    n[$2]++; at[$2] = at[$2] " " $1
  } END { for (k in n) if (n[k] > 1) print at[k] "\t" k }')
if [ -n "$dups" ]; then
  fail "同じタスクの行が複数あります(マージの解消で二重になっていないか確認する):"
  printf '%s\n' "$dups" >&2
fi

# 5. 行頭のタスク ID が重複しない。ID は「英大文字+数字…」か「英大文字-英数字…」(P4-16b・AJ4・DOC-api・R-2-9・I-web-1)。
ids=$(printf '%s\n' "$items" | awk -F'\t' 'NF > 1 {
    body = $2
    sub(/^\*\*/, "", body)
    if (match(body, /^[A-Z]+[0-9][0-9A-Za-z]*(-[0-9A-Za-z]+)*/) || match(body, /^[A-Z]+(-[0-9A-Za-z]+)+/)) {
      id = substr(body, 1, RLENGTH)
      rest = substr(body, RLENGTH + 1)
      # ID の直後が区切り(空白・括弧・コロン・強調・行末)のときだけ ID とみなす(「JD2〜JD5」等の範囲は除く)。
      if (rest == "" || rest ~ /^[[:space:]:*(]/ || index(rest, "(") == 1 || index(rest, ":") == 1) print id "\t" $1
    }
  }' | sort)
dup_ids=$(printf '%s\n' "$ids" | awk -F'\t' 'NF > 1 { n[$1]++; at[$1] = at[$1] " " $2 } END { for (k in n) if (n[k] > 1) print k "\t" at[k] }' | sort)
while IFS=$'\t' read -r id where; do
  [ -n "$id" ] || continue
  case "$allowed_dup_ids" in *" $id "*) continue ;; esac
  fail "タスク ID $id が重複しています:$where"
done <<<"$dup_ids"

if [ "$errors" -gt 0 ]; then
  echo "check-plan: $errors 件の不整合があります(ADR-0172)" >&2
  exit 1
fi
echo "check-plan: OK($(printf '%s\n' "$items" | grep -c . || true) 行のタスク)"
