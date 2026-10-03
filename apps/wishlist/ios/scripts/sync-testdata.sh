#!/usr/bin/env bash
# apps/wishlist/testdata/query-cases.json(Go・TS と共通のテストベクタ)を、Swift のテストリソースへコピーする。
# SwiftPM のリソースは Package 外のファイルを指せず、シンボリックリンクはシミュレータ実行(xcodebuild)で壊れうるため、コピーで持つ。
#
#   apps/wishlist/ios/scripts/sync-testdata.sh           コピーを更新する
#   apps/wishlist/ios/scripts/sync-testdata.sh --check   コピーが元と同じことを確かめる(違えば終了コード 1)
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
wishlist_dir="$(cd "$script_dir/../.." && pwd)"
readonly src="$wishlist_dir/testdata/query-cases.json"
readonly dst="$wishlist_dir/ios/WishlistKit/Tests/WishlistCoreTests/Resources/query-cases.json"

case "${1:-}" in
  "")
    mkdir -p "$(dirname "$dst")"
    cp "$src" "$dst"
    echo "wishlist-ios-sync-testdata: $dst を更新"
    ;;
  --check)
    if ! cmp -s "$src" "$dst"; then
      echo "wishlist-ios-sync-check: $dst が $src と違う。apps/wishlist/ios/scripts/sync-testdata.sh を実行してコミットする" >&2
      exit 1
    fi
    echo "wishlist-ios-sync-check: テストリソースは元と一致"
    ;;
  *) echo "usage: $0 [--check]" >&2; exit 2 ;;
esac
