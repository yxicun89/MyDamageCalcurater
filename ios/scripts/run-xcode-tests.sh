#!/usr/bin/env bash
# xcodebuild test を実行し、結果バンドルの件数で合否を判定する(ADR-0017 §7)。
#
#   ios/scripts/run-xcode-tests.sh <表示名> <xcodebuild の引数...>
#
# xcodebuild の終了コードに加え、次のどれかでも失敗にする(スキップや空実行を成功と数えないため):
#   - 実行したテストが 0 件
#   - 失敗・スキップ・想定内の失敗(expected failure)が 1 件以上
set -euo pipefail

if [ "$#" -lt 2 ]; then
  echo "usage: $0 <label> <xcodebuild args...>" >&2
  exit 2
fi
label="$1"
shift

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=ios/scripts/xcode-env.sh
source "$script_dir/xcode-env.sh"

work_dir="$(mktemp -d)"
trap 'rm -rf "$work_dir"' EXIT
result_bundle="$work_dir/result.xcresult"
summary_json="$work_dir/summary.json"

status=0
xcodebuild test "$@" -resultBundlePath "$result_bundle" || status=$?

if [ ! -d "$result_bundle" ]; then
  echo "$label: 結果バンドルが作られなかった(xcodebuild の終了コード $status)" >&2
  exit 1
fi
xcrun xcresulttool get test-results summary --path "$result_bundle" > "$summary_json"

# summary の数値を取り出す(plutil は JSON も読める)。無いキーは 0 とみなす。
summary_count() {
  plutil -extract "$1" raw -o - "$summary_json" 2>/dev/null || echo 0
}
total="$(summary_count totalTestCount)"
passed="$(summary_count passedTests)"
failed="$(summary_count failedTests)"
skipped="$(summary_count skippedTests)"
expected_failures="$(summary_count expectedFailures)"

echo "$label: 全 $total 件 / 成功 $passed / 失敗 $failed / スキップ $skipped / 想定内の失敗 $expected_failures"

if [ "$status" -ne 0 ] || [ "$total" -eq 0 ] || [ "$failed" -ne 0 ] || [ "$skipped" -ne 0 ] || [ "$expected_failures" -ne 0 ]; then
  echo "$label: 失敗(xcodebuild の終了コード $status)" >&2
  exit 1
fi
