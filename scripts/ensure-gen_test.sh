#!/usr/bin/env bash
# scripts/ensure-gen.sh の自動テスト(ADR-0171)。`make test-scripts`(make test に含む)から流す。
#
# 固定すること:
#   - 生成物の一覧がすべて .gitignore で無視され、追跡されていない
#   - stale: 出力が無い・入力が新しい・GEN_FORCE=1 で「要る」(0)、出力が新しければ「要らない」(1)
#   - check: 欠けた生成物があれば失敗し、make gen を案内する
set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
readonly ROOT
readonly ENSURE="$ROOT/scripts/ensure-gen.sh"

failures=0
ok() { echo "ok: $1"; }
ng() { echo "NG: $1" >&2; failures=$((failures + 1)); }

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

# --- 一覧と .gitignore -------------------------------------------------------
list=$("$ENSURE" list)
if [ -n "$list" ]; then ok "list が生成物を返す"; else ng "list が空"; fi
while IFS= read -r f; do
  if git -C "$ROOT" check-ignore -q --no-index "$f"; then
    ok "$f は .gitignore で無視される"
  else
    ng "$f が .gitignore で無視されていない"
  fi
  if [ -z "$(git -C "$ROOT" ls-files -- "$f")" ]; then
    ok "$f は追跡されていない"
  else
    ng "$f が追跡されている(git rm --cached で外す)"
  fi
done <<<"$list"

# --- stale ------------------------------------------------------------------
mkdir -p "$work/in"
touch "$work/in/spec.yaml"
if "$ENSURE" stale "$work/out.go" -- "$work/in"; then ok "stale: 出力が無ければ要る"; else ng "stale: 出力が無いのに要らないと判定"; fi

touch "$work/out.go"
touch -t 202001010000 "$work/in/spec.yaml"
if "$ENSURE" stale "$work/out.go" -- "$work/in"; then ng "stale: 出力が新しいのに要ると判定"; else ok "stale: 出力が新しければ要らない"; fi

if GEN_FORCE=1 "$ENSURE" stale "$work/out.go" -- "$work/in"; then ok "stale: GEN_FORCE=1 なら要る"; else ng "stale: GEN_FORCE=1 でも要らないと判定"; fi

touch -t 202001010000 "$work/out.go"
touch -t 202101010000 "$work/in/spec.yaml"
if "$ENSURE" stale "$work/out.go" -- "$work/in"; then ok "stale: 入力が新しければ要る"; else ng "stale: 入力が新しいのに要らないと判定"; fi

touch "$work/out.go" "$work/out2.go"
touch -t 202001010000 "$work/in/spec.yaml"
rm "$work/out2.go"
if "$ENSURE" stale "$work/out.go" "$work/out2.go" -- "$work/in"; then ok "stale: 出力の1つが欠ければ要る"; else ng "stale: 出力の1つが欠けても要らないと判定"; fi

# --- check(コピーしたスクリプトを空のリポジトリ構成で動かす)-----------------
mkdir -p "$work/repo/scripts"
cp "$ENSURE" "$work/repo/scripts/ensure-gen.sh"
if out=$("$work/repo/scripts/ensure-gen.sh" check 2>&1); then
  ng "check: 生成物が無いのに成功した"
else
  if echo "$out" | grep -q "make gen"; then ok "check: 欠落を make gen で案内する"; else ng "check: make gen の案内が無い: $out"; fi
fi

if [ "$failures" -ne 0 ]; then
  echo "ensure-gen_test: $failures 件失敗" >&2
  exit 1
fi
echo "ensure-gen_test: すべて成功"
