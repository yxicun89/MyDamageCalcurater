#!/usr/bin/env bash
# 各 openapi.yaml から Swift の API クライアントを生成する(ADR-0500 §2・ADR-0415・ADR-0807)。
#
#   ios/scripts/openapi-gen.sh           出力が無いか入力が新しい対象だけ生成する(強制は GEN_FORCE=1)
#   ios/scripts/openapi-gen.sh --check   一時ディレクトリに生成し、作業ツリーの生成物と差分が無いことを確かめる
#
# 生成物は Git に置かない(ADR-0807)。make ios-* の各ターゲットが前段で呼ぶ。
# Xcode で直接開く前は、リポジトリのルートで make ios-gen を1回流す。
# 生成対象の一覧は ios/scripts/openapi-targets.sh。
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ios_dir="$(cd "$script_dir/.." && pwd)"
repo_dir="$(cd "$ios_dir/.." && pwd)"
# shellcheck source=ios/scripts/xcode-env.sh
source "$script_dir/xcode-env.sh"
# shellcheck source=ios/scripts/openapi-targets.sh
source "$script_dir/openapi-targets.sh"

readonly tool_dir="$ios_dir/tools/openapi-gen"
# 生成済みの印(入力より新しければ生成を省く)。Git 管理外。
readonly stamp_dir="$ios_dir/.gen-stamps"

mode="write"
case "${1:-}" in
  "") ;;
  --check) mode="check" ;;
  *) echo "usage: $0 [--check]" >&2; exit 2 ;;
esac

# is_stale 名前 仕様 設定 出力先 — 生成が要るなら 0。
is_stale() {
  local name="$1" spec="$2" config="$3" out="$4"
  [ -n "$(find "$out" -name '*.swift' -print -quit 2>/dev/null)" ] || return 0
  "$repo_dir/scripts/ensure-gen.sh" stale "$stamp_dir/$name" -- \
    "$spec" "$config" "$tool_dir/Package.swift" "$tool_dir/Package.resolved" "$script_dir/openapi-targets.sh"
}

# 生成が要る対象を先に集め、無ければ生成器のビルドも省く。
pending=()
for target in "${IOS_OPENAPI_TARGETS[@]}"; do
  IFS='|' read -r name spec config out <<<"$target"
  spec="$repo_dir/$spec" config="$repo_dir/$config" out="$repo_dir/$out"
  if [ "$mode" = "check" ] || is_stale "$name" "$spec" "$config" "$out"; then
    pending+=("$name|$spec|$config|$out")
  fi
done
if [ "${#pending[@]}" -eq 0 ]; then
  exit 0
fi

swift build --package-path "$tool_dir" -c release --product swift-openapi-generator >/dev/null
generator="$(swift build --package-path "$tool_dir" -c release --show-bin-path)/swift-openapi-generator"

# 生成器は成功時も詳細を stderr に出すので、失敗したときだけ表示する。
generate_into() {
  local config="$1" out="$2" spec="$3" log
  log="$(mktemp)"
  if ! "$generator" generate --config "$config" --output-directory "$out" "$spec" >"$log" 2>&1; then
    cat "$log" >&2
    rm -f "$log"
    return 1
  fi
  rm -f "$log"
}

for target in "${pending[@]}"; do
  IFS='|' read -r name spec config generated_dir <<<"$target"
  if [ "$mode" = "write" ]; then
    rm -rf "$generated_dir"
    mkdir -p "$generated_dir" "$stamp_dir"
    generate_into "$config" "$generated_dir" "$spec"
    touch "$stamp_dir/$name"
    echo "ios-gen: $name: ${generated_dir#"$repo_dir"/} を生成"
    continue
  fi
  work_dir="$(mktemp -d)"
  generate_into "$config" "$work_dir" "$spec"
  if ! diff -r "$work_dir" "$generated_dir" >/dev/null 2>&1; then
    echo "ios-gen-check: $name の生成物が ${spec#"$repo_dir"/} と一致しない(または無い)。make ios-gen を実行する(ADR-0807)" >&2
    diff -r "$work_dir" "$generated_dir" 2>&1 | head -40 >&2 || true
    rm -rf "$work_dir"
    exit 1
  fi
  rm -rf "$work_dir"
  echo "ios-gen-check: $name の生成物は ${spec#"$repo_dir"/} と一致"
done
