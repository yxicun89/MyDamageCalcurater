#!/usr/bin/env bash
# Xcode(xcodebuild)が使えることを確かめる。source して使う。使えなければメッセージを出して終了コード 2 で終わる。
# 「未実装・実行不能のターゲットを成功扱いにしない」ため、skip して 0 を返すことはしない。
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=apps/wishlist/ios/scripts/xcode-env.sh
source "$script_dir/xcode-env.sh"

if ! xcodebuild -version >/dev/null 2>&1; then
  echo "Xcode が要る: xcodebuild が使えない(xcode-select が CommandLineTools を指し、/Applications/Xcode.app も使えない)。" >&2
  echo "  Xcode 27 を入れて、sudo xcode-select -s /Applications/Xcode.app/Contents/Developer を実行するか、DEVELOPER_DIR を設定する。" >&2
  exit 2
fi
