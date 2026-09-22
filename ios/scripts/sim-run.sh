#!/usr/bin/env bash
# アプリをシミュレータにビルド・インストールし、モックで起動する(手順書 docs/runbooks/ios.md 用)。
#
#   ios/scripts/sim-run.sh <シミュレータ名> <画面: root | calc | reverse | team> <外観: light | dark> <文字サイズ>
#
# 文字サイズは simctl の content_size(large が標準。extra-extra-large・accessibility-large など)。
# 画面を直接開く環境変数は ios/PokeCalc/RootView.swift の POKECALC_OPEN_*_AT_LAUNCH と同じ。
set -euo pipefail

if [ "$#" -ne 4 ]; then
  echo "usage: $0 <simulator> <root|calc|reverse|team> <light|dark> <content_size>" >&2
  exit 2
fi
simulator="$1"
screen="$2"
appearance="$3"
content_size="$4"

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ios_dir="$(cd "$script_dir/.." && pwd)"
# shellcheck source=ios/scripts/xcode-env.sh
source "$script_dir/xcode-env.sh"

readonly bundle_id="com.example.pokecalc"
# ios/build/ は .gitignore 済み。毎回同じ場所に作り、差分ビルドで速くする。
readonly derived_data="$ios_dir/build/DerivedData"
readonly screenshot_dir="$ios_dir/build/screenshots"
# 起動してから画面が描き終わるまで待つ秒数(モックの読み込みと最初の計算が終わるのを待つ)。
readonly settle_seconds=5

case "$screen" in
  root) open_key="" ;;
  calc) open_key="POKECALC_OPEN_CALC_SCREEN_AT_LAUNCH" ;;
  reverse) open_key="POKECALC_OPEN_REVERSE_SCREEN_AT_LAUNCH" ;;
  team) open_key="POKECALC_OPEN_TEAM_LIST_SCREEN_AT_LAUNCH" ;;
  *) echo "画面は root / calc / reverse / team のどれか: ${screen}" >&2; exit 2 ;;
esac

xcrun simctl boot "$simulator" 2>/dev/null || true
xcrun simctl bootstatus "$simulator" >/dev/null
# Simulator.app があれば画面を開く(無い環境でも、下のスクリーンショットで確かめられる)。
open -b com.apple.iphonesimulator 2>/dev/null || true

xcodebuild build -quiet -project "$ios_dir/PokeCalc.xcodeproj" -scheme PokeCalc \
  -destination "platform=iOS Simulator,name=$simulator" -derivedDataPath "$derived_data"
app_path="$derived_data/Build/Products/Debug-iphonesimulator/PokeCalc.app"

xcrun simctl install "$simulator" "$app_path"
xcrun simctl ui "$simulator" appearance "$appearance"
xcrun simctl ui "$simulator" content_size "$content_size"
xcrun simctl terminate "$simulator" "$bundle_id" 2>/dev/null || true

launch_env=(SIMCTL_CHILD_POKECALC_USE_MOCK=1)
if [ -n "$open_key" ]; then
  launch_env+=("SIMCTL_CHILD_${open_key}=1")
fi
env "${launch_env[@]}" xcrun simctl launch "$simulator" "$bundle_id" >/dev/null
sleep "$settle_seconds"
mkdir -p "$screenshot_dir"
screenshot="$screenshot_dir/$screen-$appearance-$content_size.png"
xcrun simctl io "$simulator" screenshot "$screenshot" >/dev/null 2>&1
open "$screenshot"
echo "ios-sim-run: ${simulator} で起動(画面 ${screen} / 外観 ${appearance} / 文字 ${content_size} / モック)。スクリーンショット: ios/build/screenshots/${screenshot##*/}"
