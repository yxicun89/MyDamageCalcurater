#!/usr/bin/env bash
# iOS レーンのスクリプトが共通で読む環境設定(ADR-0500 §7)。source して使う。
#
# xcode-select が CommandLineTools を指していると xcodebuild / simctl が使えないので、
# DEVELOPER_DIR が未設定のときだけ Xcode.app に向ける(利用者の明示的な設定を上書きしない)。
readonly DEFAULT_XCODE_DEVELOPER_DIR="/Applications/Xcode.app/Contents/Developer"

if [ -z "${DEVELOPER_DIR:-}" ] && xcode-select -p 2>/dev/null | grep -q "CommandLineTools"; then
  if [ ! -d "$DEFAULT_XCODE_DEVELOPER_DIR" ]; then
    echo "Xcode が見つからない(xcode-select が CommandLineTools を指しており、$DEFAULT_XCODE_DEVELOPER_DIR も無い)" >&2
    exit 1
  fi
  export DEVELOPER_DIR="$DEFAULT_XCODE_DEVELOPER_DIR"
fi
