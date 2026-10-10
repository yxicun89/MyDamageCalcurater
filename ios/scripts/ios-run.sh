#!/usr/bin/env bash
# アプリをビルドしてシミュレータで起動する(F-14。手順書 docs/verify.md)。
#
#   ios/scripts/ios-run.sh <シミュレータ名>
#
# 環境変数:
#   POKECALC_API_BASE_URL  設定すると、その URL に接続するアプリをビルドする(空ならモックのまま)
#   IOS_RUN_MOCK=1         起動時にモックを強制する(POKECALC_USE_MOCK=1 を起動環境に渡す)
# シミュレータが起動していなければ起動し、起動済みならそのまま使う(shutdown はしない)。
# 画面を指定して開くスクリーンショット付きの起動は sim-run.sh(make ios-sim-run)。
set -euo pipefail

if [ "$#" -ne 1 ]; then
  echo "usage: $0 <simulator>" >&2
  exit 2
fi
simulator="$1"

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ios_dir="$(cd "$script_dir/.." && pwd)"
# shellcheck source=ios/scripts/xcode-env.sh
source "$script_dir/xcode-env.sh"

readonly bundle_id="com.example.pokecalc"
# ios/build/ は .gitignore 済み。sim-run.sh と同じ場所で差分ビルドを共有する。
readonly derived_data="$ios_dir/build/DerivedData"

xcrun simctl boot "$simulator" 2>/dev/null || true
xcrun simctl bootstatus "$simulator" >/dev/null
open -b com.apple.iphonesimulator 2>/dev/null || true

build_args=()
if [ -n "${POKECALC_API_BASE_URL:-}" ]; then
  build_args+=("POKECALC_API_BASE_URL=$POKECALC_API_BASE_URL")
fi
xcodebuild build -quiet -project "$ios_dir/PokeCalc.xcodeproj" -scheme PokeCalc \
  -destination "platform=iOS Simulator,name=$simulator" -derivedDataPath "$derived_data" \
  ${build_args[@]+"${build_args[@]}"}
app_path="$derived_data/Build/Products/Debug-iphonesimulator/PokeCalc.app"

xcrun simctl install "$simulator" "$app_path"
xcrun simctl terminate "$simulator" "$bundle_id" 2>/dev/null || true

launch_env=()
mode="接続先 ${POKECALC_API_BASE_URL:-なし(モック)}"
if [ "${IOS_RUN_MOCK:-0}" = "1" ]; then
  launch_env+=("SIMCTL_CHILD_POKECALC_USE_MOCK=1")
  mode="モック強制"
fi
env ${launch_env[@]+"${launch_env[@]}"} xcrun simctl launch "$simulator" "$bundle_id" >/dev/null
echo "ios-run: ${simulator} で起動しました(${mode})"
