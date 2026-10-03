#!/usr/bin/env bash
# clone ごとに1回流す Git の設定(ADR-0171 §5)。何度流しても同じ結果になる。
#
#   scripts/setup-git.sh
#
# - merge ドライバ pokecalc-ios-gen: iOS の API 生成物(.gitattributes で指定)の衝突を、
#   合成した仕様からの再生成で解く(ios/scripts/merge-generated.sh)。未登録の clone では通常の 3-way マージになる。
set -euo pipefail
cd "$(dirname "$0")/.."

git config merge.pokecalc-ios-gen.name "iOS の API 生成物を合成した仕様から再生成する(ADR-0171)"
git config merge.pokecalc-ios-gen.driver "ios/scripts/merge-generated.sh %O %A %B %P"
echo "setup-git: merge ドライバ pokecalc-ios-gen を登録した"
