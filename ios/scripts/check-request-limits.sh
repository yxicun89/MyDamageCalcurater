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

check ReverseRequest observations maxObservations
check ReverseRequest itemCandidates maxItemCandidates
check BulkCalcRequest itemVariants maxItemVariants
check_query /api/pokedex/moves/batch ids maxMoveBatchIds

# --- 素早さ(P6-24。ADR-0503): services/speed/api/openapi.yaml の範囲と、既存の定数(SPLimits・RankLimits)が一致するか ---
# 素早さの画面は SP の最大・ランクの範囲を専用の定数に複製せず SPLimits / RankLimits を使う。
# その値が素早さの契約(PositionRequest の sp・rank)と食い違ったら気づけるようにする。
speed_openapi="$repo_root/services/speed/api/openapi.yaml"
domain_swift="$repo_root/ios/PokeCalcKit/Sources/PokeCalcCore/DomainTypes.swift"

# components.schemas.<schema>.properties.<property>.<key>(minimum / maximum / maxItems / minItems)の値。見つからなければ空。
# $1 = 契約ファイル、$2 = schema、$3 = property、$4 = key
schema_property_value() {
  awk -v schema="$2" -v property="$3" -v key="$4" '
    /^    [A-Za-z]/ { in_schema = ($0 == "    " schema ":"); in_prop = 0; next }
    in_schema && /^        [A-Za-z]/ { in_prop = ($0 ~ "^        " property ":") ; next }
    in_schema && in_prop && $1 == key ":" { print $2; exit }
  ' "$1"
}

# components.schemas.<schema>.<key> の値(プロパティではなく、スキーマ自身の maxLength など)。見つからなければ空。
# $1 = 契約ファイル、$2 = schema、$3 = key
schema_value() {
  awk -v schema="$2" -v key="$3" '
    /^    [A-Za-z]/ { in_schema = ($0 == "    " schema ":"); next }
    in_schema && /^      [A-Za-z]/ && $1 == key ":" { print $2; exit }
  ' "$1"
}

speed_contract_value() { schema_property_value "$speed_openapi" "$@"; }

# DomainTypes.swift の `public static let <name> = <数値>`(負数可)の値。
domain_limit() {
  sed -n "s/^ *public static let $1 = \(-\{0,1\}[0-9][0-9]*\)$/\1/p" "$domain_swift" | head -1
}

check_speed() {
  local property="$1" key="$2" name="$3"
  local contract ios
  contract="$(speed_contract_value PositionRequest "$property" "$key")"
  ios="$(domain_limit "$name")"
  if [ -z "$contract" ] || [ -z "$ios" ]; then
    echo "ios-check-request-limits: speed の PositionRequest.$property.$key または $name が見つからない" >&2
    status=1
  elif [ "$contract" != "$ios" ]; then
    echo "ios-check-request-limits: speed の PositionRequest.$property.$key=$contract と $name=$ios が違う" >&2
    status=1
  fi
}

check_speed sp maximum maxPerStat
check_speed rank minimum min
check_speed rank maximum max

# --- 判定(P6-25。ADR-0504 §2): services/judge/api/openapi.yaml の範囲と、定数(RequestLimits・SPLimits・RankLimits)が一致するか ---
# 判定の画面も SP の最大・ランクの範囲は SPLimits / RankLimits を使い、複製しない。候補数・技 ID の長さは RequestLimits に写しを持つ。
judge_openapi="$repo_root/services/judge/api/openapi.yaml"

check_judge() {
  local label="$1" contract="$2" ios="$3" ios_name="$4"
  if [ -z "$contract" ] || [ -z "$ios" ]; then
    echo "ios-check-request-limits: judge の $label または $ios_name が見つからない" >&2
    status=1
  elif [ "$contract" != "$ios" ]; then
    echo "ios-check-request-limits: judge の $label=$contract と $ios_name=$ios が違う" >&2
    status=1
  fi
}

check_judge "OutspeedAndKoRequest.defenders.maxItems" \
  "$(schema_property_value "$judge_openapi" OutspeedAndKoRequest defenders maxItems)" "$(swift_limit maxJudgeDefenders)" maxJudgeDefenders
check_judge "OutspeedAndKoRequest.defenders.minItems" \
  "$(schema_property_value "$judge_openapi" OutspeedAndKoRequest defenders minItems)" "$(swift_limit minJudgeDefenders)" minJudgeDefenders
check_judge "MoveId.maxLength" \
  "$(schema_value "$judge_openapi" MoveId maxLength)" "$(swift_limit maxJudgeMoveIdLength)" maxJudgeMoveIdLength
# 能力ポイント(StatBlock の6項目)とランク(RankBlock の5項目)は、1つでも契約とずれたら気づけるよう全項目を見る。
for stat in hp atk def spa spd spe; do
  check_judge "StatBlock.$stat.maximum" "$(schema_property_value "$judge_openapi" StatBlock "$stat" maximum)" "$(domain_limit maxPerStat)" maxPerStat
done
for stat in atk def spa spd spe; do
  check_judge "RankBlock.$stat.minimum" "$(schema_property_value "$judge_openapi" RankBlock "$stat" minimum)" "$(domain_limit min)" RankLimits.min
  check_judge "RankBlock.$stat.maximum" "$(schema_property_value "$judge_openapi" RankBlock "$stat" maximum)" "$(domain_limit max)" RankLimits.max
done

# --- タイプバランス(P6-26。ADR-0505 §2): services/balance/api/openapi.yaml の範囲と、RequestLimits が一致するか ---
# メンバー数の上限・下限は analyze と coverage の両方を見る(片方だけ変わってもずれに気づく)。
balance_openapi="$repo_root/services/balance/api/openapi.yaml"

check_balance() {
  local label="$1" contract="$2" ios="$3" ios_name="$4"
  if [ -z "$contract" ] || [ -z "$ios" ]; then
    echo "ios-check-request-limits: balance の $label または $ios_name が見つからない" >&2
    status=1
  elif [ "$contract" != "$ios" ]; then
    echo "ios-check-request-limits: balance の $label=$contract と $ios_name=$ios が違う" >&2
    status=1
  fi
}

for schema in AnalyzeRequest CoverageRequest; do
  check_balance "$schema.members.maxItems" \
    "$(schema_property_value "$balance_openapi" "$schema" members maxItems)" "$(swift_limit maxBalanceMembers)" maxBalanceMembers
  check_balance "$schema.members.minItems" \
    "$(schema_property_value "$balance_openapi" "$schema" members minItems)" "$(swift_limit minBalanceMembers)" minBalanceMembers
done
check_balance "CoverageRequestMember.moveIds.maxItems" \
  "$(schema_property_value "$balance_openapi" CoverageRequestMember moveIds maxItems)" "$(swift_limit maxBalanceMovesPerMember)" maxBalanceMovesPerMember
check_balance "MoveId.maxLength" \
  "$(schema_value "$balance_openapi" MoveId maxLength)" "$(swift_limit maxBalanceMoveIdLength)" maxBalanceMoveIdLength

if [ "$status" -eq 0 ]; then
  echo "ios-check-request-limits: OK(RequestLimits は api/openapi.yaml と一致)"
fi
exit "$status"
