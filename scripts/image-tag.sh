#!/usr/bin/env bash
# image-tag.sh [パス...] — ローカルでビルドするイメージのタグを出す(issue #291)。
#
# git の HEAD の短いコミット(12桁)。指定したパス(省略時はリポジトリ全体)に未コミットの変更か未追跡のファイルが
# あれば末尾に -dirty を付ける(タグと中身がずれていることの印)。同じコミット・同じ作業ツリーなら何度呼んでも
# 同じ値になる(冪等)。`:local` を上書きし続ける代わりにこのタグで k3d へ入れると、Pod の image からビルド元の
# コミットが分かり、kubectl rollout undo で前のタグへ戻せる。
# 各レーンの local-registry-push.sh・pokedex-registry-push.sh が個別に持っていた計算を共通化したもの。
#
# 使い方: tag="$(./scripts/image-tag.sh services/pokedex engine)"
# 自動テスト: scripts/image-tag_test.sh(make test-scripts)。
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"
tag="$(git rev-parse --short=12 HEAD)"
if [ -n "$(git status --porcelain -- "$@")" ]; then
  tag="${tag}-dirty"
fi
printf '%s\n' "$tag"
