#!/usr/bin/env bash
# コンフリクトマーカー(<<<<<<<・=======・>>>>>>>)が追跡ファイルに残っていないか検査する。
# マージの途中で commit してしまう事故と、マーカーが残ったスクリプトで開発環境が止まる事故を防ぐ(2026-10-03)。
set -euo pipefail
cd "$(dirname "$0")/.."

# 行頭の <<<<<<< / >>>>>>> だけを見る(= は Markdown の見出し下線と区別できないので見ない)。
# この検査自体と、マーカーを説明する文書・テストは除く。
found=$(git grep -nE '^(<<<<<<<|>>>>>>>) ' -- . \
  ':!scripts/check-conflict-markers.sh' ':!docs/**' ':!**/*_test.sh' ':!**/testdata/**' || true)
if [ -n "$found" ]; then
  printf '%s\n' "$found" >&2
  echo "コンフリクトマーカーが残っています。解消してから commit してください。" >&2
  exit 1
fi
echo "check-conflict-markers: 0 件"
