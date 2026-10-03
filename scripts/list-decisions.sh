#!/usr/bin/env bash
# scripts/list-decisions.sh [--all | --archive] — 判断のエントリの索引を出す(ADR-0170)。
#
# 既定:      docs/decisions/*.md のうち、状態が open または 未回答 のものを「パス<TAB>状態<TAB>タイトル」で出す。
# --all:     docs/decisions/*.md の全件を同じ形で出す。
# --archive: 凍結アーカイブ docs/ai-shared/DECISIONS.md の見出しを「行番号: 見出し」で出す(grep -n '^## ' と同じ)。
#
# 状態はファイル先頭の「- 状態: <値>」の行から読む。状態の行が無いファイルは「(状態なし)」として、既定でも出す(書き忘れを見落とさない)。
# 1件も無ければ何も出さずに 0 で終わる。
set -euo pipefail

usage() {
  echo "使い方: $0 [--all | --archive]" >&2
  exit 2
}

mode=open
case "${1:-}" in
  "") ;;
  --all) mode=all ;;
  --archive) mode=archive ;;
  *) usage ;;
esac
[ "$#" -le 1 ] || usage

root=$(cd "$(dirname "$0")/.." && pwd)

if [ "$mode" = archive ]; then
  archive="${root}/docs/ai-shared/DECISIONS.md"
  [ -f "$archive" ] || { echo "アーカイブが無い: docs/ai-shared/DECISIONS.md" >&2; exit 1; }
  grep -n '^## ' "$archive" || true
  exit 0
fi

dir="${root}/docs/decisions"
[ -d "$dir" ] || exit 0

for f in "$dir"/*.md; do
  [ -e "$f" ] || continue
  [ "$(basename "$f")" = README.md ] && continue
  rel="docs/decisions/$(basename "$f")"
  title=$(grep -m1 '^# ' "$f" | sed 's/^# //' || true)
  state=$(grep -m1 '^- 状態:' "$f" | sed 's/^- 状態:[[:space:]]*//' || true)
  [ -n "$state" ] || state="(状態なし)"
  if [ "$mode" = open ]; then
    case "$state" in
      open* | 未回答* | "(状態なし)") ;;
      *) continue ;;
    esac
  fi
  printf '%s\t%s\t%s\n' "$rel" "$state" "$title"
done
