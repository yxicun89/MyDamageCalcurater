#!/usr/bin/env bash
# POKECALC_API_BASE_URL を渡してビルドし、成果物(PokeCalc.app)の Info.plist に
# PokeCalcAPIBaseURL キーとその値が入っていることを確かめる(ADR-0500 §5)。
# 併せて、ATS(App Transport Security)の例外が issue #250 / ADR-0501「issue #250」の判断どおり
# `NSAllowsLocalNetworking` だけ(`NSAllowsArbitraryLoads` は無し)になっていることも確かめる。
#
# なぜ要るか: `INFOPLIST_KEY_PokeCalcAPIBaseURL` のような openapi/Apple 既知でない独自キーは
# `GENERATE_INFOPLIST_FILE=YES` の自動生成 Info.plist には反映されない(シミュレータ向けビルドで
# 確認済み)。`INFOPLIST_FILE`(実ファイル)+ `GENERATE_INFOPLIST_FILE` のマージで
# 対応した(ios/PokeCalc-Info.plist・ios/PokeCalc/Config/PokeCalc.xcconfig)。この検査は
# その組み合わせが実際に効いていることを固定する。ATS の例外は `AppConfiguration` が http を受理する
# 範囲(非修飾ホスト名〈localhost など〉・.local。IP アドレスは受理しない)と実行時の挙動が一致していないと、受理したのに通信できない
# (issue #250 の不具合)が起きるため。
#
#   ios/scripts/check-infoplist.sh <destination>
set -euo pipefail

if [ "$#" -lt 1 ]; then
  echo "usage: $0 <xcodebuild destination>" >&2
  exit 2
fi
destination="$1"

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ios_dir="$(cd "$script_dir/.." && pwd)"
# shellcheck source=ios/scripts/xcode-env.sh
source "$script_dir/xcode-env.sh"

readonly test_url="https://pokecalc-check.example.invalid"
work_dir="$(mktemp -d)"
trap 'rm -rf "$work_dir"' EXIT

build_log="$work_dir/build.log"
if ! xcodebuild build \
  -project "$ios_dir/PokeCalc.xcodeproj" \
  -scheme PokeCalc \
  -destination "$destination" \
  -derivedDataPath "$work_dir/DerivedData" \
  "POKECALC_API_BASE_URL=$test_url" \
  >"$build_log" 2>&1; then
  cat "$build_log" >&2
  echo "ios-check-infoplist: ビルド失敗" >&2
  exit 1
fi

app_plist="$(find "$work_dir/DerivedData/Build/Products" -maxdepth 2 -name 'PokeCalc.app' -print -quit)/Info.plist"
if [ ! -f "$app_plist" ]; then
  echo "ios-check-infoplist: PokeCalc.app の Info.plist が見つからない" >&2
  exit 1
fi

actual="$(plutil -extract PokeCalcAPIBaseURL raw -o - "$app_plist" 2>/dev/null || true)"
if [ "$actual" != "$test_url" ]; then
  echo "ios-check-infoplist: Info.plist の PokeCalcAPIBaseURL が期待値と違う(期待 '$test_url' 実際 '${actual:-<キー無し>}')" >&2
  exit 1
fi
echo "ios-check-infoplist: Info.plist に PokeCalcAPIBaseURL = $test_url が入っている"

# ATS(App Transport Security)の例外が issue #250 / ADR-0501「issue #250」の判断どおりに限定されていることを確かめる。
# `NSAllowsLocalNetworking` は `AppConfiguration` が http を受理する範囲(unqualified/`.local`/IP アドレス。
# ADR-0500 §5・ADR-0501「issue #250」)と ATS の実行時の挙動を一致させるために要る。
# `NSAllowsArbitraryLoads` は使わない(それだと任意の http を無条件で許してしまい、
# `AppConfiguration` の受理条件より ATS の方が広くなってしまう)。
local_networking="$(plutil -extract NSAppTransportSecurity.NSAllowsLocalNetworking raw -o - "$app_plist" 2>/dev/null || true)"
if [ "$local_networking" != "true" ]; then
  echo "ios-check-infoplist: Info.plist に NSAppTransportSecurity.NSAllowsLocalNetworking = true が無い" \
    "(実際 '${local_networking:-<キー無し>}'。issue #250: http を受理する接続先〈localhost・IP アドレス・.local〉が" \
    "実行時に ATS で拒否されないために必要)" >&2
  exit 1
fi

arbitrary_loads="$(plutil -extract NSAppTransportSecurity.NSAllowsArbitraryLoads raw -o - "$app_plist" 2>/dev/null || true)"
if [ -n "$arbitrary_loads" ]; then
  echo "ios-check-infoplist: Info.plist に NSAppTransportSecurity.NSAllowsArbitraryLoads がある" \
    "(実際 '$arbitrary_loads'。issue #250: 任意の http を許すのではなく NSAllowsLocalNetworking だけで足りるはず)" >&2
  exit 1
fi
echo "ios-check-infoplist: NSAppTransportSecurity.NSAllowsLocalNetworking = true・NSAllowsArbitraryLoads 無し"
