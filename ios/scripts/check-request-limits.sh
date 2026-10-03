#!/usr/bin/env bash
# iOS の件数上限(PokeCalcCore の RequestLimits)が api/openapi.yaml の maxItems と一致するか確かめる
# (issue #110。ADR-0501「issue #110」2章)。
#
# なぜ要るか: RequestLimits は契約の写し(coding-rules §2 の「意図的な重複」)。XCTest はシミュレータの
# サンドボックスで走りリポジトリのファイルを読めないので、同期の検査はここ(ホスト側)で行う。
#
#   ios/scripts/check-request-limits.sh
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/../.." && pwd)"
openapi="$repo_root/api/openapi.yaml"
limits_swift="$repo_root/ios/PokeCalcKit/Sources/PokeCalcCore/RequestLimits.swift"

# components.schemas.<schema>.properties.<property>.maxItems を取り出す(見つからなければ空)。
contract_max_items() {
  awk -v schema="$1" -v property="$2" '
    /^    [A-Za-z]/ { in_schema = ($0 == "    " schema ":"); in_prop = 0; next }
    in_schema && /^        [A-Za-z]/ { in_prop = ($0 ~ "^        " property ":") ; next }
    in_schema && in_prop && /^          maxItems:/ { print $2; exit }
  ' "$openapi"
}

# paths.<path>.get.parameters[].(name==<name>).schema.maxItems を取り出す(見つからなければ空)。
# `getMovesByIds` の `ids` のように、component.schemas のプロパティではなく1つのオペレーションの
# クエリパラメータとして maxItems を持つケース用(ADR-0501「getMovesByIds による構築編集の技の
# 一括解決」1章)。paths の各エントリはトップレベルから2スペース、`- name: <name>` は8スペース、
# その配下の `schema.maxItems` は12スペースの固定インデント(api/openapi.yaml のスタイル)を前提にする。
contract_query_max_items() {
  awk -v path="$1" -v name="$2" '
    /^  [^ ]/ { in_path = ($0 == "  " path ":"); in_param = 0; next }
    in_path && /^        - name: / { in_param = ($0 == "        - name: " name); next }
    in_path && in_param && /^            maxItems:/ { print $2; exit }
  ' "$openapi"
}

# paths.<path>.get.parameters[].(name==<name>).schema の1行の flow 形式(`schema: { ..., <key>: <数値> }`)から
# <key> の値を取り出す(見つからなければ空)。`listMoveLearners` の `limit` の `default` 用(ADR-0502 §7)。
contract_query_inline_schema_value() {
  awk -v path="$1" -v name="$2" -v key="$3" '
    /^  [^ ]/ { in_path = ($0 == "  " path ":"); in_param = 0; next }
    in_path && /^        - / { in_param = ($0 == "        - name: " name); next }
    in_path && in_param && /^          schema: \{/ {
      if (match($0, key ": [0-9]+")) { value = substr($0, RSTART, RLENGTH); sub(/.*: /, "", value); print value }
      exit
    }
  ' "$openapi"
}

# components.schemas.<schema>.<key>(スキーマ直下の数値。例 AdjustHits.maximum)を取り出す(見つからなければ空)。
contract_schema_value() {
  awk -v schema="$1" -v key="$2" '
    /^    [A-Za-z]/ { in_schema = ($0 == "    " schema ":"); next }
    in_schema && $0 ~ "^      " key ": " { print $2; exit }
  ' "$openapi"
}

# RequestLimits.swift の `public static let <name> = <数値>` の値(見つからなければ空)。
swift_limit() {
  sed -n "s/^ *public static let $1 = \([0-9][0-9]*\)$/\1/p" "$limits_swift"
}

status=0
check() {
  local schema="$1" property="$2" name="$3"
  local contract ios
  contract="$(contract_max_items "$schema" "$property")"
  ios="$(swift_limit "$name")"
  if [ -z "$contract" ] || [ -z "$ios" ]; then
    echo "ios-check-request-limits: $schema.$property.maxItems または RequestLimits.$name が見つからない" >&2
    status=1
  elif [ "$contract" != "$ios" ]; then
    echo "ios-check-request-limits: $schema.$property.maxItems=$contract と RequestLimits.$name=$ios が違う" >&2
    status=1
  fi
}

# check の「クエリパラメータ版」(components.schemas ではなく paths.<path>.get のクエリパラメータの
# maxItems と照合する。`getMovesByIds` の `ids` 用)。
check_query() {
  local path="$1" name="$2" limit_name="$3"
  local contract ios
  contract="$(contract_query_max_items "$path" "$name")"
  ios="$(swift_limit "$limit_name")"
  if [ -z "$contract" ] || [ -z "$ios" ]; then
    echo "ios-check-request-limits: $path クエリ $name.maxItems または RequestLimits.$limit_name が見つからない" >&2
    status=1
  elif [ "$contract" != "$ios" ]; then
    echo "ios-check-request-limits: $path クエリ $name.maxItems=$contract と RequestLimits.$limit_name=$ios が違う" >&2
    status=1
  fi
}

# 契約の1つの値と RequestLimits の1つの定数を照合する(`check` / `check_query` の汎用版。ADR-0502 §7)。
check_value() {
  local label="$1" contract="$2" limit_name="$3"
  local ios
  ios="$(swift_limit "$limit_name")"
  if [ -z "$contract" ] || [ -z "$ios" ]; then
    echo "ios-check-request-limits: $label または RequestLimits.$limit_name が見つからない" >&2
    status=1
  elif [ "$contract" != "$ios" ]; then
    echo "ios-check-request-limits: $label=$contract と RequestLimits.$limit_name=$ios が違う" >&2
    status=1
  fi
}

check ReverseRequest observations maxObservations
check ReverseRequest itemCandidates maxItemCandidates
check BulkCalcRequest itemVariants maxItemVariants
check_query /api/pokedex/moves/batch ids maxMoveBatchIds
check_value "/api/pokedex/moves/{key}/learners クエリ limit.default" \
  "$(contract_query_inline_schema_value '/api/pokedex/moves/{key}/learners' limit default)" moveLearnersPageSize
check_value "AdjustHits.maximum" "$(contract_schema_value AdjustHits maximum)" maxAdjustHits

if [ "$status" -eq 0 ]; then
  echo "ios-check-request-limits: OK(RequestLimits は api/openapi.yaml と一致)"
fi
exit "$status"
