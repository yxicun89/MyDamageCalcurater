#!/usr/bin/env bash
# iOS スクリプトが共通で読む環境設定。source して使う(ダメ計 ios/scripts/xcode-env.sh と同じ考え方)。
#
# xcode-select が CommandLineTools を指していると xcodebuild / simctl / Foundation 入りの swift build が使えないので、
# DEVELOPER_DIR が未設定のときだけ Xcode.app に向ける(利用者の明示的な設定を上書きしない)。
# Xcode.app が無ければ何もしない(終了もしない。Xcode が必須の処理は require-xcode.sh を使う)。
readonly WISHLIST_DEFAULT_XCODE_DEVELOPER_DIR="/Applications/Xcode.app/Contents/Developer"

if [ -z "${DEVELOPER_DIR:-}" ] \
  && xcode-select -p 2>/dev/null | grep -q "CommandLineTools" \
  && [ -d "$WISHLIST_DEFAULT_XCODE_DEVELOPER_DIR" ]; then
  export DEVELOPER_DIR="$WISHLIST_DEFAULT_XCODE_DEVELOPER_DIR"
fi
