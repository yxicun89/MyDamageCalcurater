#!/usr/bin/env bash
# Git に置かない生成物(ADR-0171)の一覧・欠落の案内・再生成の要否判定。
#
#   scripts/ensure-gen.sh check                    生成物が揃っているか。欠けていれば一覧と「make gen」の案内を出して 1
#   scripts/ensure-gen.sh list                     生成物の一覧(リポジトリのルートからの相対パス。1行1件)
#   scripts/ensure-gen.sh stale <出力...> -- <入力...>
#                                                  再生成が要るなら 0、要らなければ 1(Makefile の gen-* が使う)。
#                                                  出力のどれかが無い・入力(ディレクトリなら配下のファイル)のどれかが
#                                                  出力のどれよりも新しい・GEN_FORCE=1 のとき「要る」
#
# iOS の生成物(ios/PokeCalcKit/Sources/PokeCalcAPI/Generated)は追跡を続けるので、ここには含めない(ADR-0171 §5)。
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# 生成物の一覧の正。.gitignore・Makefile の gen-* と揃える(scripts/ensure-gen_test.sh が .gitignore との一致を検査する)。
generated_files() {
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

cmd_check() {
  local missing=() f
  while IFS= read -r f; do
    [ -f "$repo_root/$f" ] || missing+=("$f")
  done < <(generated_files)
  if [ "${#missing[@]}" -eq 0 ]; then
    return 0
  fi
  {
    echo "生成物が無い(API 契約・SQL から作るファイルは Git に置かない。ADR-0171):"
    printf '  %s\n' "${missing[@]}"
    echo "リポジトリのルートで make gen を実行してから、もう一度実行する。"
  } >&2
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
  local out
  for out in "${outputs[@]}"; do
    [ -f "$out" ] || return 0
    # 入力のどれかがこの出力より新しければ再生成が要る(ディレクトリは配下のファイルを見る)。
    if [ -n "$(find "${inputs[@]}" -type f -newer "$out" -print -quit)" ]; then
      return 0
    fi
  done
  return 1
}

case "${1:-}" in
  check) cmd_check ;;
  list) generated_files ;;
  stale) shift; cmd_stale "$@" ;;
  *)
    echo "usage: $0 check | list | stale <出力...> -- <入力...>" >&2
    exit 2
    ;;
esac
