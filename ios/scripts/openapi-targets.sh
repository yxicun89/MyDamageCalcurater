#!/usr/bin/env bash
# iOS の API 生成対象の一覧(ADR-0415・ADR-0806)。source して使う。何も実行しない。
#
# 「名前|仕様|生成設定|出力先」の組。パスはリポジトリのルートからの相対。
# ios/scripts/openapi-gen.sh(生成)と scripts/ensure-gen.sh(Git に置かない生成物の一覧)の両方がここを読むので、
# 生成対象を足すときはここに1行足すだけで、生成・追跡禁止・gen-clean の対象に入る
# (.gitignore は ios/PokeCalcKit/Sources/*/Generated をまとめて無視する)。
# 出力先のターゲットには、生成物の欠落を案内する手書きの GenRequired.swift も置く。
# shellcheck disable=SC2034  # source 先が使う
readonly IOS_OPENAPI_TARGETS=(
  "PokeCalcAPI|api/openapi.yaml|ios/tools/openapi-gen/openapi-generator-config.yaml|ios/PokeCalcKit/Sources/PokeCalcAPI/Generated"
  "PokeCalcBalanceAPI|services/balance/api/openapi.yaml|ios/tools/openapi-gen/openapi-generator-balance-config.yaml|ios/PokeCalcKit/Sources/PokeCalcBalanceAPI/Generated"
)
