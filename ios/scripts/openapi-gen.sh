#!/usr/bin/env bash
# api/openapi.yaml から Swift の API クライアントを生成する(ADR-0017 §2)。
#
#   ios/scripts/openapi-gen.sh           生成物を ios/PokeCalcKit/Sources/PokeCalcAPI/Generated に書く
#   ios/scripts/openapi-gen.sh --check   一時ディレクトリに生成し、コミット済みの生成物と差分が無いことを確かめる
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ios_dir="$(cd "$script_dir/.." && pwd)"
repo_dir="$(cd "$ios_dir/.." && pwd)"
# shellcheck source=ios/scripts/xcode-env.sh
source "$script_dir/xcode-env.sh"

readonly tool_dir="$ios_dir/tools/openapi-gen"
readonly spec="$repo_dir/api/openapi.yaml"
readonly config="$tool_dir/openapi-generator-config.yaml"
readonly generated_dir="$ios_dir/PokeCalcKit/Sources/PokeCalcAPI/Generated"

mode="write"
case "${1:-}" in
  "") ;;
  --check) mode="check" ;;
  *) echo "usage: $0 [--check]" >&2; exit 2 ;;
esac

swift build --package-path "$tool_dir" -c release --product swift-openapi-generator >/dev/null
generator="$(swift build --package-path "$tool_dir" -c release --show-bin-path)/swift-openapi-generator"

# 生成器は成功時も詳細を stderr に出すので、失敗したときだけ表示する。
generate_into() {
  local log
  log="$(mktemp)"
  if ! "$generator" generate --config "$config" --output-directory "$1" "$spec" >"$log" 2>&1; then
    cat "$log" >&2
    rm -f "$log"
    return 1
  fi
  rm -f "$log"
}

if [ "$mode" = "write" ]; then
  mkdir -p "$generated_dir"
  generate_into "$generated_dir"
  echo "ios-gen: $generated_dir を生成"
  exit 0
fi

work_dir="$(mktemp -d)"
trap 'rm -rf "$work_dir"' EXIT
generate_into "$work_dir"
if ! diff -r "$work_dir" "$generated_dir" >/dev/null; then
  echo "ios-gen-check: 生成物が api/openapi.yaml と一致しない。make ios-gen を実行してコミットする" >&2
  diff -r "$work_dir" "$generated_dir" | head -40 >&2 || true
  exit 1
fi
echo "ios-gen-check: 生成物は api/openapi.yaml と一致"
