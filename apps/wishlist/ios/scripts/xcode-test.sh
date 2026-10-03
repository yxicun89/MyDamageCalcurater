#!/usr/bin/env bash
# xcodebuild でのテスト一式(Xcode 27 とシミュレータが要る。無ければ終了コード 2)。
#   1. WishlistKit のテスト(scheme WishlistKit-Package)をシミュレータで
#   2. アプリの XCUITest(scheme Wishlist)をシミュレータで
#
#   apps/wishlist/ios/scripts/xcode-test.sh [シミュレータ名]   既定は WISHLIST_IOS_SIMULATOR か "iPhone 18 Pro"
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ios_dir="$(cd "$script_dir/.." && pwd)"
# shellcheck source=apps/wishlist/ios/scripts/require-xcode.sh
source "$script_dir/require-xcode.sh"

simulator="${1:-${WISHLIST_IOS_SIMULATOR:-iPhone 18 Pro}}"
destination="platform=iOS Simulator,name=$simulator"

(cd "$ios_dir/WishlistKit" && "$script_dir/run-xcode-tests.sh" "wishlist-ios-xcode-test(kit)" \
  -scheme WishlistKit-Package -destination "$destination")
"$script_dir/run-xcode-tests.sh" "wishlist-ios-xcode-test(ui)" \
  -project "$ios_dir/Wishlist.xcodeproj" -scheme Wishlist -destination "$destination"
