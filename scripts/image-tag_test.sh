#!/usr/bin/env bash
# scripts/image-tag.sh の自動テスト(issue #291)。make test-scripts から流す。
# 使い捨ての git リポジトリにスクリプトをコピーして流す(このリポジトリの作業ツリーの状態に依存しない)。
set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
readonly ROOT

failures=0
ok() { echo "ok: $1"; }
ng() { echo "NG: $1" >&2; failures=$((failures + 1)); }

repo=$(mktemp -d)
trap 'rm -rf "$repo"' EXIT
git -C "$repo" init -q
mkdir -p "$repo/scripts" "$repo/svc" "$repo/other"
cp "$ROOT/scripts/image-tag.sh" "$repo/scripts/"
echo a > "$repo/svc/a.txt"
echo b > "$repo/other/b.txt"
git -C "$repo" add -A
git -C "$repo" -c user.name=test -c user.email=test@example.invalid commit -qm init
head=$(git -C "$repo" rev-parse --short=12 HEAD)

tag() { (cd "$repo" && ./scripts/image-tag.sh "$@"); }

[ "$(tag)" = "$head" ] && ok "変更が無ければ HEAD の12桁" || ng "変更が無いときのタグ: $(tag)(期待 ${head})"
[ "$(tag)" = "$(tag)" ] && ok "同じ状態なら同じタグ(冪等)" || ng "2回の結果が違う"

echo changed >> "$repo/other/b.txt"
[ "$(tag)" = "${head}-dirty" ] && ok "全体指定: 未コミットの変更で -dirty" || ng "全体指定のタグ: $(tag)"
[ "$(tag svc)" = "$head" ] && ok "パス指定: 対象外の変更では -dirty にしない" || ng "パス指定(svc)のタグ: $(tag svc)"
git -C "$repo" checkout -q -- other/b.txt

echo new > "$repo/svc/untracked.txt"
[ "$(tag svc)" = "${head}-dirty" ] && ok "パス指定: 未追跡のファイルでも -dirty" || ng "未追跡のファイルがあるときのタグ: $(tag svc)"

if [ "$failures" -ne 0 ]; then
  echo "image-tag_test: ${failures} 件失敗" >&2
  exit 1
fi
echo "image-tag_test: すべて成功"
