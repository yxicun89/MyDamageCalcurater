#!/usr/bin/env bash
# Git に置かない生成物(ADR-0807)の一覧・欠落の案内・再生成の要否判定。
#
#   scripts/ensure-gen.sh check                    Go・TypeScript の生成物が揃っているか。欠けていれば一覧と「make gen」の案内を出して 1
#   scripts/ensure-gen.sh check-ios                iOS の生成物(ディレクトリ)が揃っているか。欠けていれば「make ios-gen」を案内して 1
#   scripts/ensure-gen.sh list                     生成物の一覧(リポジトリのルートからの相対パス。1行1件。iOS はディレクトリ)
#   scripts/ensure-gen.sh stale <出力...> -- <入力...>
#                                                  再生成が要るなら 0、要らなければ 1(Makefile の gen-*・ios/scripts/openapi-gen.sh が使う)。
#                                                  「要る」のは次のどれか: GEN_FORCE=1 / 出力のどれかが無い / 入力のどれかが無い /
#                                                  入力(ディレクトリなら自身と配下すべて)のどれかが出力より新しい。
#                                                  ディレクトリ自身の更新時刻も比べるので、入力ファイルの削除・改名も検出する
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Go・TypeScript の生成物の一覧の正。.gitignore・Makefile の gen-* と揃える
# (scripts/ensure-gen_test.sh が .gitignore との一致を検査する)。
go_ts_files() {
  cat <<'EOF'
services/internal/api/openapi.gen.go
services/balance/internal/api/openapi.gen.go
services/speed/internal/api/openapi.gen.go
services/judge/internal/api/openapi.gen.go
services/pokedex/internal/store/db.go
services/pokedex/internal/store/models.go
services/pokedex/internal/store/pokedex.sql.go
services/pokedex/internal/store/querier.go
web/src/api/openapi.gen.ts
web/src/api/balance.gen.ts
web/src/speed/speed.gen.ts
web/src/judge/judge.gen.ts
EOF
}

# iOS の生成物の出力先(ディレクトリ)。正は ios/scripts/openapi-targets.sh。
ios_dirs() {
  # shellcheck source=ios/scripts/openapi-targets.sh
  source "$repo_root/ios/scripts/openapi-targets.sh"
  local target
  for target in "${IOS_OPENAPI_TARGETS[@]}"; do
    printf '%s\n' "${target##*|}"
  done
}

# report_missing 案内のコマンド 欠けたパス... — 欠落を一覧にして案内する。
report_missing() {
  local command="$1"
  shift
  {
    echo "生成物が無い(API 契約・SQL から作るファイルは Git に置かない。ADR-0807):"
    printf '  %s\n' "$@"
    echo "リポジトリのルートで $command を実行してから、もう一度実行する。"
  } >&2
}

cmd_check() {
  local missing=() f
  while IFS= read -r f; do
    [ -f "$repo_root/$f" ] || missing+=("$f")
  done < <(go_ts_files)
  [ "${#missing[@]}" -eq 0 ] && return 0
  report_missing "make gen" "${missing[@]}"
  return 1
}

cmd_check_ios() {
  local missing=() d
  while IFS= read -r d; do
    [ -n "$(find "$repo_root/$d" -name '*.swift' -print -quit 2>/dev/null)" ] || missing+=("$d")
  done < <(ios_dirs)
  [ "${#missing[@]}" -eq 0 ] && return 0
  report_missing "make ios-gen" "${missing[@]}"
  return 1
}

# 更新時刻は stat の書式が macOS と Linux で違うため、find -newer で比べる。
cmd_stale() {
  local outputs=() inputs=() seen_sep=0 arg
  for arg in "$@"; do
    if [ "$arg" = "--" ]; then
      seen_sep=1
    elif [ "$seen_sep" -eq 0 ]; then
      outputs+=("$arg")
    else
      inputs+=("$arg")
    fi
  done
  if [ "${#outputs[@]}" -eq 0 ] || [ "${#inputs[@]}" -eq 0 ]; then
    echo "usage: $0 stale <出力...> -- <入力...>" >&2
    exit 2
  fi
  [ "${GEN_FORCE:-}" = "1" ] && return 0
  local out in
  for in in "${inputs[@]}"; do
    [ -e "$in" ] || return 0
  done
  for out in "${outputs[@]}"; do
    [ -f "$out" ] || return 0
    # 入力のどれかがこの出力より新しければ再生成が要る。ディレクトリ自身も比べる
    # (配下のファイルの追加・削除・改名でディレクトリの更新時刻が変わる)。
    if [ -n "$(find "${inputs[@]}" -newer "$out" -print -quit)" ]; then
      return 0
    fi
  done
  return 1
}

case "${1:-}" in
  check) cmd_check ;;
  check-ios) cmd_check_ios ;;
  list) go_ts_files; ios_dirs ;;
  stale) shift; cmd_stale "$@" ;;
  *)
    echo "usage: $0 check | check-ios | list | stale <出力...> -- <入力...>" >&2
    exit 2
    ;;
esac
