#!/usr/bin/env bash
# api/openapi.yaml から Swift の API クライアントを生成する(ADR-0500 §2)。
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
# 生成対象は「名前|仕様|設定|出力先」の組(ADR-0415。balance は schema 名が衝突するので別モジュール。素早さ・判定も同様。ADR-0503・0504)。
readonly targets=(
  "PokeCalcAPI|$repo_dir/api/openapi.yaml|$tool_dir/openapi-generator-config.yaml|$ios_dir/PokeCalcKit/Sources/PokeCalcAPI/Generated"
  "PokeCalcBalanceAPI|$repo_dir/services/balance/api/openapi.yaml|$tool_dir/openapi-generator-balance-config.yaml|$ios_dir/PokeCalcKit/Sources/PokeCalcBalanceAPI/Generated"
  "PokeCalcSpeedAPI|$repo_dir/services/speed/api/openapi.yaml|$tool_dir/openapi-generator-config.speed.yaml|$ios_dir/PokeCalcKit/Sources/PokeCalcSpeedAPI/Generated"
  "PokeCalcJudgeAPI|$repo_dir/services/judge/api/openapi.yaml|$tool_dir/openapi-generator-config.judge.yaml|$ios_dir/PokeCalcKit/Sources/PokeCalcJudgeAPI/Generated"
)

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
  local config="$1" out="$2" spec="$3" log
  log="$(mktemp)"
  if ! "$generator" generate --config "$config" --output-directory "$out" "$spec" >"$log" 2>&1; then
    cat "$log" >&2
    rm -f "$log"
    return 1
  fi
  rm -f "$log"
}

for target in "${targets[@]}"; do
  IFS='|' read -r name spec config generated_dir <<<"$target"
  if [ "$mode" = "write" ]; then
    mkdir -p "$generated_dir"
    generate_into "$config" "$generated_dir" "$spec"
    echo "ios-gen: $name: $generated_dir を生成"
    continue
  fi
  work_dir="$(mktemp -d)"
  generate_into "$config" "$work_dir" "$spec"
  if ! diff -r "$work_dir" "$generated_dir" >/dev/null; then
    echo "ios-gen-check: $name の生成物が $spec と一致しない。make ios-gen を実行してコミットする" >&2
    diff -r "$work_dir" "$generated_dir" | head -40 >&2 || true
    rm -rf "$work_dir"
    exit 1
  fi
  rm -rf "$work_dir"
  echo "ios-gen-check: $name の生成物は $spec と一致"
done
