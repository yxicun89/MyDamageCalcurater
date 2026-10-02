#!/usr/bin/env bash
# OpenAPI 契約から Swift の API クライアントを生成する(ADR-0500 §2。契約ごとに別ターゲット。ADR-0503)。
#
#   ios/scripts/openapi-gen.sh           生成物を各ターゲットの Generated に書く
#   ios/scripts/openapi-gen.sh --check   一時ディレクトリに生成し、コミット済みの生成物と差分が無いことを確かめる
#
# 生成するもの(「契約・設定・出力先」の組。契約を足すときはこの表に1行足す):
#   api/openapi.yaml                  → PokeCalcAPI(pokedex・calc・record・team)
#   services/speed/api/openapi.yaml   → PokeCalcSpeedAPI(素早さ。ADR-0503)
#   services/judge/api/openapi.yaml   → PokeCalcJudgeAPI(判定。ADR-0504)
#   services/balance/api/openapi.yaml → PokeCalcBalanceAPI(タイプバランス。ADR-0505)
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ios_dir="$(cd "$script_dir/.." && pwd)"
repo_dir="$(cd "$ios_dir/.." && pwd)"
# shellcheck source=ios/scripts/xcode-env.sh
source "$script_dir/xcode-env.sh"

readonly tool_dir="$ios_dir/tools/openapi-gen"
readonly sources_dir="$ios_dir/PokeCalcKit/Sources"

# 「契約(リポジトリルートからの相対)|設定(tool_dir からの相対)|出力先(Sources からの相対)」。
readonly targets=(
  "api/openapi.yaml|openapi-generator-config.yaml|PokeCalcAPI/Generated"
  "services/speed/api/openapi.yaml|openapi-generator-config.speed.yaml|PokeCalcSpeedAPI/Generated"
  "services/judge/api/openapi.yaml|openapi-generator-config.judge.yaml|PokeCalcJudgeAPI/Generated"
  "services/balance/api/openapi.yaml|openapi-generator-config.balance.yaml|PokeCalcBalanceAPI/Generated"
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
# $1 = 契約、$2 = 設定、$3 = 出力先ディレクトリ
generate_into() {
  local log
  log="$(mktemp)"
  if ! "$generator" generate --config "$2" --output-directory "$3" "$1" >"$log" 2>&1; then
    cat "$log" >&2
    rm -f "$log"
    return 1
  fi
  rm -f "$log"
}

failed=0
work_dir="$(mktemp -d)"
trap 'rm -rf "$work_dir"' EXIT

for target in "${targets[@]}"; do
  IFS='|' read -r spec_rel config_rel out_rel <<<"$target"
  spec="$repo_dir/$spec_rel"
  config="$tool_dir/$config_rel"
  generated_dir="$sources_dir/$out_rel"

  if [ "$mode" = "write" ]; then
    mkdir -p "$generated_dir"
    generate_into "$spec" "$config" "$generated_dir"
    echo "ios-gen: $generated_dir を生成"
    continue
  fi

  check_dir="$work_dir/$out_rel"
  mkdir -p "$check_dir"
  generate_into "$spec" "$config" "$check_dir"
  if ! diff -r "$check_dir" "$generated_dir" >/dev/null 2>&1; then
    echo "ios-gen-check: 生成物が $spec_rel と一致しない。make ios-gen を実行してコミットする" >&2
    diff -r "$check_dir" "$generated_dir" 2>&1 | head -40 >&2 || true
    failed=1
  else
    echo "ios-gen-check: $out_rel は $spec_rel と一致"
  fi
done

exit "$failed"
