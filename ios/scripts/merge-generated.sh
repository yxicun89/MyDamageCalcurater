#!/usr/bin/env bash
# git の merge ドライバ: iOS の API 生成物(Generated/*.swift)の衝突を、合成した仕様からの再生成で解く(ADR-0171 §5)。
#
#   .gitattributes の `merge=pokecalc-ios-gen` から、git が `<base> <ours> <theirs> <path>` を渡して呼ぶ。
#   登録は scripts/setup-git.sh(clone ごとに1回)。
#
# git はドライバを呼ぶ時点で作業ツリーをまだ書き換えていない(マージ結果はメモリ上にある)ため、
# 作業ツリーの api/openapi.yaml は使えない。git merge が設定する GITHEAD_<相手のコミット> から相手を知り、
# HEAD・相手・merge-base の仕様と生成設定を git merge-file で合成して、その結果から生成した内容で <ours> を上書きする。
# 同じ仕様からの生成結果は .git の下にキャッシュし、9ファイルで生成器を1回だけ呼ぶ。
#
# 次のときは自動で解かず、こちら側の内容を残して衝突のまま返す(終了コード 1):
#   - git merge 以外(rebase・cherry-pick など。相手のコミットが分からない)
#   - 仕様か生成設定そのものが衝突した
#   - swift(Xcode)が無い
set -euo pipefail

if [ "$#" -ne 4 ]; then
  echo "usage: $0 <base> <ours> <theirs> <path>" >&2
  exit 2
fi
ours_file="$2"
path="$4"

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_dir="$(cd "$script_dir/../.." && pwd)"
readonly spec_path="api/openapi.yaml"
readonly config_path="ios/tools/openapi-gen/openapi-generator-config.yaml"

give_up() {
  echo "pokecalc-ios-gen: $path は自動で解けない($1)。仕様の衝突を解いてから make ios-gen を実行し、git add する(ADR-0171 §5)" >&2
  exit 1
}

# git merge は相手のコミットを GITHEAD_<sha>=<名前> で渡す。ちょうど1つのときだけ使う(octopus は対象外)。
theirs_commit=""
count=0
while IFS= read -r name; do
  theirs_commit="${name#GITHEAD_}"
  count=$((count + 1))
done < <(env | sed -n 's/^\(GITHEAD_[0-9a-f]\{40,64\}\)=.*/\1/p')
[ "$count" -eq 1 ] || give_up "git merge 以外か、相手が1つでない"
command -v swift >/dev/null 2>&1 || give_up "swift が無い"

cd "$repo_dir"
base_commit="$(git merge-base HEAD "$theirs_commit")" || give_up "merge-base が無い"

work_dir="$(mktemp -d)"
trap 'rm -rf "$work_dir"' EXIT

# merge_input パス — HEAD・相手・merge-base のそのファイルを合成して標準出力に出す。衝突したら失敗。
merge_input() {
  local rel="$1" name
  for name in base ours theirs; do
    local commit
    case "$name" in
      base) commit="$base_commit" ;;
      ours) commit="HEAD" ;;
      theirs) commit="$theirs_commit" ;;
    esac
    git show "$commit:$rel" >"$work_dir/$name" 2>/dev/null || return 1
  done
  git merge-file -p "$work_dir/ours" "$work_dir/base" "$work_dir/theirs"
}

merge_input "$spec_path" >"$work_dir/openapi.yaml" || give_up "$spec_path が衝突した"
merge_input "$config_path" >"$work_dir/openapi-generator-config.yaml" || give_up "$config_path が衝突した"

# 生成器はこちら側(作業ツリー)の ios/tools/openapi-gen で作るので、その版もキーに含める。
key="$(cat "$work_dir/openapi.yaml" "$work_dir/openapi-generator-config.yaml" ios/tools/openapi-gen/Package.resolved | git hash-object --stdin)"
cache_root="$(git rev-parse --git-path pokecalc-ios-gen-cache)"
cache_dir="$cache_root/$key"
if [ ! -d "$cache_dir" ]; then
  mkdir -p "$cache_root"
  staging="$(mktemp -d "$cache_root/tmp.XXXXXX")"
  if ! "$script_dir/openapi-gen.sh" --into "$staging" "$work_dir/openapi.yaml" "$work_dir/openapi-generator-config.yaml" >/dev/null; then
    rm -rf "$staging"
    give_up "生成に失敗した"
  fi
  mv "$staging" "$cache_dir"
fi

generated="$cache_dir/$(basename "$path")"
[ -f "$generated" ] || give_up "合成した仕様からはこのファイルが生成されない"
cp "$generated" "$ours_file"
echo "pokecalc-ios-gen: $path を合成した仕様から再生成して解いた" >&2
