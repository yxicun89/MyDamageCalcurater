#!/usr/bin/env bash
# scripts/ensure-gen.sh の自動テスト(ADR-0807)。`make test-scripts`(make test に含む)から流す。
#
# 固定すること:
#   - 生成物の一覧がすべて .gitignore で無視され、追跡されていない
#   - 生成物の一覧に iOS の出力先(ios/scripts/openapi-targets.sh の全対象)が入る
#   - stale: 出力が無い・入力が新しい・入力が無い・入力ディレクトリ内の削除/改名・GEN_FORCE=1 で「要る」(0)、
#     出力が新しければ「要らない」(1)
#   - check / check-ios: 欠けた生成物があれば失敗し、make gen / make ios-gen を案内する
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

# shellcheck source=ios/scripts/openapi-targets.sh
source "$ROOT/ios/scripts/openapi-targets.sh"
for target in "${IOS_OPENAPI_TARGETS[@]}"; do
  out="${target##*|}"
  if grep -Fxq "$out" <<<"$list"; then ok "list に iOS の $out が入る"; else ng "list に iOS の $out が無い"; fi
done

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

# 入力ディレクトリ内のファイルの削除・改名(出力より新しいファイルが無くても、ディレクトリ自身が新しくなる)
touch "$work/in/a.sql" "$work/in/b.sql"
touch -t 202001010000 "$work/in" "$work/in/spec.yaml" "$work/in/a.sql" "$work/in/b.sql"
touch "$work/out.go"
if "$ENSURE" stale "$work/out.go" -- "$work/in"; then ng "stale: 前提の準備に失敗(変更前から要ると判定)"; else ok "stale: 変更前は要らない"; fi
sleep 1
rm "$work/in/a.sql"
if "$ENSURE" stale "$work/out.go" -- "$work/in"; then ok "stale: 入力ディレクトリのファイル削除で要る"; else ng "stale: 入力ファイルを消しても要らないと判定"; fi
touch -t 202001010000 "$work/in"
if "$ENSURE" stale "$work/out.go" -- "$work/in"; then ng "stale: 前提の準備に失敗"; else ok "stale: 削除の前提を戻せば要らない"; fi
sleep 1
mv "$work/in/b.sql" "$work/in/c.sql"
if "$ENSURE" stale "$work/out.go" -- "$work/in"; then ok "stale: 入力ディレクトリのファイル改名で要る"; else ng "stale: 入力ファイルを改名しても要らないと判定"; fi

# 入力ファイルそのものが無い(改名された単独の入力)
touch "$work/out.go"
if "$ENSURE" stale "$work/out.go" -- "$work/missing.yaml"; then ok "stale: 入力が無ければ要る"; else ng "stale: 入力が無いのに要らないと判定"; fi

# --- check(コピーしたスクリプトを空のリポジトリ構成で動かす)-----------------
mkdir -p "$work/repo/scripts"
cp "$ENSURE" "$work/repo/scripts/ensure-gen.sh"
if out=$("$work/repo/scripts/ensure-gen.sh" check 2>&1); then
  ng "check: 生成物が無いのに成功した"
else
  if echo "$out" | grep -q "make gen"; then ok "check: 欠落を make gen で案内する"; else ng "check: make gen の案内が無い: $out"; fi
fi
mkdir -p "$work/repo/ios/scripts"
cp "$ROOT/ios/scripts/openapi-targets.sh" "$work/repo/ios/scripts/openapi-targets.sh"
if out=$("$work/repo/scripts/ensure-gen.sh" check-ios 2>&1); then
  ng "check-ios: 生成物が無いのに成功した"
else
  if echo "$out" | grep -q "make ios-gen"; then ok "check-ios: 欠落を make ios-gen で案内する"; else ng "check-ios: make ios-gen の案内が無い: $out"; fi
fi

if [ "$failures" -ne 0 ]; then
  echo "ensure-gen_test: $failures 件失敗" >&2
  exit 1
fi
echo "ensure-gen_test: すべて成功"
