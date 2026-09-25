#!/usr/bin/env bash
# Claude Code / Codex の PreToolUse フックから呼ばれる Bash コマンドの危険操作ガード(ADR-0800。issue #273・#239)。
# stdin から `{"tool_name":"Bash","tool_input":{"command":"..."}}` 形式の JSON を受け取り、
# 文字列だけを見て判定する(実コマンドは実行しない。git push の宛先解決だけは例外的に
# `git rev-parse --abbrev-ref HEAD` を呼ぶ。ADR-0800 §2 参照)。
#   - 危険と判定: stderr に理由を書いて exit 2(呼び出し元が人間に確認を求める)
#   - 危険でない: 何も出力せず exit 0
#   - command が無い・空、tool_name が Bash 以外、jq が無い環境: 判定できないので exit 0(fail-open)
# 方式: 引用符とシェルのメタ文字を空白に置き換えて1本のトークン列に平坦化し、
# git・make・kubectl・k3d・gh のいずれかのトークンが現れるたびに、その位置から対応する
# check_* 関数を呼ぶ(bash -c・eval・サブシェル・前置きコマンド等の形に関わらず本体コマンドを拾うため)。
# ADR-0800 §2 の方針により、見逃すより誤検知(過剰な確認要求)を許容する。
# 自動テスト: scripts/ai-guard/bash-guard_test.sh(`make test-scripts` から実行)。
set -uo pipefail

# jq は make doctor の前提ツールだが、無い環境でツールを止めてしまわないよう fail-open にする。
if ! command -v jq >/dev/null 2>&1; then
  printf 'bash-guard: 警告 - jq が見つからないため判定をスキップします(fail-open)\n' >&2
  exit 0
fi

INPUT="$(cat)"

TOOL_NAME="$(printf '%s' "$INPUT" | jq -r '.tool_name // empty' 2>/dev/null || true)"
if [ "$TOOL_NAME" != "Bash" ]; then
  exit 0
fi

# Claude Code は command を文字列で渡すが、Codex は配列で渡す可能性があるため両対応する。
COMMAND="$(printf '%s' "$INPUT" | jq -r '(.tool_input.command // empty) | if type == "array" then join(" ") else . end' 2>/dev/null || true)"

if [ -z "$COMMAND" ]; then
  exit 0
fi

BLOCK_REASON=""
W=()
KV_VERB=""
KV_VERB_IDX=-1

# tokenize 文字列 — 空白区切りでグローバル配列 W に分割する。
tokenize() {
  W=()
  read -ra W <<<"$1"
}

# strip_redirects 文字列 — リダイレクト演算子(> >> < N>&M &> 等)とその対象トークン、
# および末尾のバックグラウンド演算子(&)を取り除く。判定前の前処理として、
# リダイレクト先のファイル名等が git push の宛先候補などに紛れ込まないようにする。
strip_redirects() {
  local s="$1"
  s="$(printf '%s' "$s" | sed -E 's/[0-9]*>&[0-9]+//g')"
  s="$(printf '%s' "$s" | sed -E 's/&>>?[[:space:]]*[^[:space:]]*//g')"
  s="$(printf '%s' "$s" | sed -E 's/[0-9]*>>?[[:space:]]*[^[:space:]]*//g')"
  s="$(printf '%s' "$s" | sed -E 's/<[[:space:]]*[^[:space:]]*//g')"
  s="$(printf '%s' "$s" | sed -E 's/&[[:space:]]*$//')"
  printf '%s' "$s"
}

# flatten_metachars 文字列 — クォート文字とシェルのメタ文字(&&・||・;・|・&・(・)・{・}・`・改行・タブ)
# を空白に置き換える。bash -c・eval・サブシェル・前置きコマンド等の形に関わらず、
# 本体のコマンド名(git・make・kubectl・k3d・gh)をトークン列から拾えるようにするため。
flatten_metachars() {
  local s="$1"
  local ch
  for ch in '&' '|' ';' '(' ')' '{' '}' '<' '>' '"' "'" '`' $'\n' $'\t'; do
    s="${s//$ch/ }"
  done
  printf '%s' "$s"
}

# resolve_current_branch — 現在のブランチ名を返す(判定できなければ空文字)。
# git push の宛先が省略されている・HEAD/@ の場合にだけ呼ぶ(I/O を使う唯一の箇所。ADR-0800 §2)。
resolve_current_branch() {
  git rev-parse --abbrev-ref HEAD 2>/dev/null
}

# check_secret_paths 文字列 — ".env"(".env.example" は除く)・".ssh" をどんな前置きでも検出する。
# "process.env.HOME" や "go env GOPATH" を誤検知しないよう、直前が英数字・"_" の場合は除外する。
check_secret_paths() {
  local frag="$1"
  local frag_no_example
  frag_no_example="$(printf '%s' "$frag" | sed -E 's/\.env\.example//g')"
  if printf '%s' "$frag_no_example" | grep -Eq '(^|[^A-Za-z0-9_])\.env([^A-Za-z0-9_]|$)'; then
    BLOCK_REASON=".env ファイル(環境変数の秘密情報)を読もうとしています"
    return 0
  fi
  if printf '%s' "$frag" | grep -Eq '(^|[^A-Za-z0-9_])\.ssh(/|[^A-Za-z0-9_]|$)'; then
    BLOCK_REASON=".ssh(秘密鍵ディレクトリ)を読もうとしています"
    return 0
  fi
  return 1
}

# lower 文字列 — ASCII を小文字化する(bash 3.2 でも動くよう ${var,,} は使わない)。
lower() {
  printf '%s' "$1" | tr '[:upper:]' '[:lower:]'
}

# kubectl_flag_takes_value フラグ — 次のトークンを値として消費するフラグかどうか。
# verb の前(グローバルフラグ)・verb の後(delete/get のリソース前)の両方で使う。
# "=" を含む自己完結形("--namespace=x")はここでは扱わず、呼び出し側で別処理する。
kubectl_flag_takes_value() {
  case "$1" in
    -n | --namespace | --context | --kubeconfig | -s | --server | -o | --output | -l | --selector | -f | --filename | --field-selector | -v | --v | --grace-period | --cascade)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

# find_kubectl_verb 開始位置 — グローバル配列 W の開始位置(kubectl トークンの index)より後ろから
# 最初のサブコマンド(delete/get 等)を探し、KV_VERB・KV_VERB_IDX に設定する。
# フラグ(値あり・値なし・"="自己完結)を読み飛ばす。
find_kubectl_verb() {
  local start="$1"
  local n=${#W[@]}
  local i=$((start + 1))
  local t
  KV_VERB=""
  KV_VERB_IDX=-1
  while [ "$i" -lt "$n" ]; do
    t="${W[$i]}"
    case "$t" in
      -*)
        case "$t" in
          *=*) i=$((i + 1)) ;;
          *)
            if kubectl_flag_takes_value "$t"; then
              i=$((i + 2))
            else
              i=$((i + 1))
            fi
            ;;
        esac
        ;;
      *)
        KV_VERB="$t"
        KV_VERB_IDX=$i
        return 0
        ;;
    esac
  done
  return 1
}

# kubectl_resource_is_destructive トークン — "ns,pvc" や "pvc/name"・"statefulsets.apps" のような
# 表記を ","・"/" で分解し、"." より前(APIグループ接尾辞を除いた部分)を小文字化して比較する。
# データ削除に当たるリソース種別(複数形・省略形込み)が含まれるかを見る。pod・job は対象外。
kubectl_resource_is_destructive() {
  local token="$1"
  local part seg base
  local IFS=','
  local parts
  read -ra parts <<<"$token"
  for part in "${parts[@]}"; do
    seg="${part%%/*}"
    base="${seg%%.*}"
    base="$(lower "$base")"
    case "$base" in
      ns | namespace | namespaces | pvc | persistentvolumeclaim | persistentvolumeclaims | pv | persistentvolume | statefulset | statefulsets | sts | secret | secrets)
        return 0
        ;;
    esac
  done
  return 1
}

# kubectl_resource_is_secret トークン — "secret/name"・"secrets.v1"・"pods,secrets" 形式にも対応して
# secret(s) かどうかを見る。
kubectl_resource_is_secret() {
  local token="$1"
  local part seg base
  local IFS=','
  local parts
  read -ra parts <<<"$token"
  for part in "${parts[@]}"; do
    seg="${part%%/*}"
    base="${seg%%.*}"
    base="$(lower "$base")"
    case "$base" in
      secret | secrets) return 0 ;;
    esac
  done
  return 1
}

# check_kubectl 開始位置 — グローバル配列 W の kubectl トークン位置から判定する。
#   - delete -k / --kustomize は常にブロック
#   - delete の対象が ns/pvc/pv/statefulset/secret 系ならブロック(pod/job は対象外)
#   - get の対象が secret(s) ならブロック
check_kubectl() {
  local start="$1"
  find_kubectl_verb "$start" || return 1
  local verb="$KV_VERB"
  local rstart=$((KV_VERB_IDX + 1))
  local n=${#W[@]}
  local j t resource

  if [ "$verb" = "delete" ]; then
    for ((j = rstart; j < n; j++)); do
      case "${W[$j]}" in
        -k | --kustomize)
          BLOCK_REASON="kubectl delete -k/--kustomize はまとめて削除するため常に確認が必要です"
          return 0
          ;;
      esac
    done

    resource=""
    for ((j = rstart; j < n; j++)); do
      t="${W[$j]}"
      case "$t" in
        -*)
          case "$t" in
            *=*) : ;;
            *) kubectl_flag_takes_value "$t" && j=$((j + 1)) ;;
          esac
          ;;
        *)
          resource="$t"
          break
          ;;
      esac
    done
    [ -z "$resource" ] && return 1
    if kubectl_resource_is_destructive "$resource"; then
      BLOCK_REASON="kubectl delete の対象に namespace/pvc/pv/statefulset/secret が含まれます(${resource})"
      return 0
    fi
    return 1
  fi

  if [ "$verb" = "get" ]; then
    resource=""
    for ((j = rstart; j < n; j++)); do
      t="${W[$j]}"
      case "$t" in
        -*)
          case "$t" in
            *=*) : ;;
            *) kubectl_flag_takes_value "$t" && j=$((j + 1)) ;;
          esac
          ;;
        *)
          resource="$t"
          break
          ;;
      esac
    done
    [ -z "$resource" ] && return 1
    if kubectl_resource_is_secret "$resource"; then
      BLOCK_REASON="kubectl get secret は秘密情報を出力するため常に確認が必要です"
      return 0
    fi
    return 1
  fi

  return 1
}

# check_k3d 開始位置 — グローバル配列 W の k3d トークン位置から判定する。
# "k3d --verbose cluster delete x" のように cluster の前後にフラグが挟まっても検出できるよう、
# フラグ(-で始まるトークン)を読み飛ばしてから "cluster" と delete/rm を探す。
check_k3d() {
  local start="$1"
  local n=${#W[@]}
  local i=$((start + 1))
  while [ "$i" -lt "$n" ] && [[ "${W[$i]}" == -* ]]; do
    i=$((i + 1))
  done
  if [ "$i" -lt "$n" ] && [ "${W[$i]}" = "cluster" ]; then
    local j=$((i + 1))
    while [ "$j" -lt "$n" ] && [[ "${W[$j]}" == -* ]]; do
      j=$((j + 1))
    done
    if [ "$j" -lt "$n" ]; then
      case "${W[$j]}" in
        delete | rm)
          BLOCK_REASON="k3d cluster delete/rm はクラスタを削除します"
          return 0
          ;;
      esac
    fi
  fi
  return 1
}

# check_make 開始位置 — グローバル配列 W の make トークン位置より後ろのトークンが破壊的ターゲット名に
# 一致するかだけを見る(前置き・"-C"・変数代入の位置を厳密に解釈しない)。
# migrate-down* は前方一致(未知の migrate-down-foo のようなターゲットも念のため止める)。
check_make() {
  local start="$1"
  local n=${#W[@]}
  local i t
  for ((i = start + 1; i < n; i++)); do
    t="${W[$i]}"
    case "$t" in
      down | import | import-k8s | migrate-down*)
        BLOCK_REASON="make ${t} は破壊的操作です"
        return 0
        ;;
    esac
  done
  return 1
}

# gh_flag_takes_value フラグ — gh の "pr merge"・"api" 判定で読み飛ばす、値を取るフラグ。
gh_flag_takes_value() {
  case "$1" in
    -R | --repo | --hostname | -H | --header | -X | --method)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

# check_gh 開始位置 — グローバル配列 W の gh トークン位置から判定する。
#   - "-R owner/repo"・"--repo owner/repo" 等のフラグを読み飛ばした上で "pr merge" ならブロック
#     (どんな追加引数でも常にブロック)
#   - "gh api ..." でパスに "/merge" を含むものはブロック(API 直叩きでの PR マージ回避)
check_gh() {
  local start="$1"
  local n=${#W[@]}
  local i=$((start + 1))
  local t first="" second=""
  while [ "$i" -lt "$n" ]; do
    t="${W[$i]}"
    case "$t" in
      -*)
        case "$t" in
          *=*) i=$((i + 1)) ;;
          *)
            if gh_flag_takes_value "$t"; then
              i=$((i + 2))
            else
              i=$((i + 1))
            fi
            ;;
        esac
        continue
        ;;
    esac
    if [ -z "$first" ]; then
      first="$t"
      i=$((i + 1))
    elif [ -z "$second" ]; then
      second="$t"
      i=$((i + 1))
      break
    else
      break
    fi
  done

  if [ "$first" = "pr" ] && [ "$second" = "merge" ]; then
    BLOCK_REASON="gh pr merge は PR を確定でマージするため常に確認が必要です"
    return 0
  fi

  if [ "$first" = "api" ]; then
    local j
    for ((j = start + 1; j < n; j++)); do
      case "${W[$j]}" in
        */merge | */merge/*)
          BLOCK_REASON="gh api で /merge を含むパスを叩いています(PR の確定マージに相当します)"
          return 0
          ;;
      esac
    done
  fi

  return 1
}

# check_git 開始位置 — グローバル配列 W の git トークン位置から判定する。
#   - push 以外(log/diff/checkout/pull/fetch/status 等)は対象外("main" を含んでいても無視)
#   - 強制系(--force[-with-lease]・-f・+refspec・--mirror・--all)は常にブロック
#   - push の宛先候補(remote 以降のフラグ以外の引数すべて)を1つずつ判定する:
#       - "main"・"refs/heads/main"(refspec ならコロンの右側)に完全一致すればブロック
#       - 宛先が無い・HEAD・@ の場合は現在のブランチを解決し、main ならブロック。
#         解決できない場合も安全側でブロックする。
check_git() {
  local start="$1"
  local n=${#W[@]}
  local i=$((start + 1))
  local t sub=""
  while [ "$i" -lt "$n" ]; do
    t="${W[$i]}"
    case "$t" in
      -C | -c | --git-dir | --work-tree | --namespace)
        i=$((i + 2))
        ;;
      -*)
        i=$((i + 1))
        ;;
      *)
        sub="$t"
        break
        ;;
    esac
  done
  [ "$sub" = "push" ] || return 1

  local start2=$((i + 1))
  local j
  for ((j = start2; j < n; j++)); do
    case "${W[$j]}" in
      --force | --force-with-lease* | -f | --mirror | --all)
        BLOCK_REASON="git push の強制オプション(${W[$j]})は履歴を壊す可能性があるため常に確認が必要です"
        return 0
        ;;
      +*)
        BLOCK_REASON="git push の force refspec(${W[$j]})は履歴を壊す可能性があるため常に確認が必要です"
        return 0
        ;;
    esac
  done

  local any_dest=0
  local tok dst cur
  for ((j = start2; j < n; j++)); do
    tok="${W[$j]}"
    case "$tok" in
      -*) continue ;;
    esac
    any_dest=1
    dst="${tok##*:}"
    case "$dst" in
      main | refs/heads/main)
        BLOCK_REASON="git push の宛先が main です(${tok})"
        return 0
        ;;
      "" | HEAD | @)
        cur="$(resolve_current_branch)"
        if [ -z "$cur" ] || [ "$cur" = "main" ]; then
          BLOCK_REASON="git push の宛先が現在のブランチに解決され、それが main か判定できません(現在のブランチ: ${cur:-不明})"
          return 0
        fi
        ;;
    esac
  done

  if [ "$any_dest" = 0 ]; then
    cur="$(resolve_current_branch)"
    if [ -z "$cur" ] || [ "$cur" = "main" ]; then
      BLOCK_REASON="git push に宛先の指定が無く、現在のブランチが main か判定できません(現在のブランチ: ${cur:-不明})"
      return 0
    fi
  fi

  return 1
}

# check_command コマンド全体 — リダイレクト・クォート・シェルのメタ文字を取り除いた1本のトークン列にし、
# git・make・kubectl・k3d・gh のいずれかのトークンが現れるたびに対応する check_* を呼ぶ。
check_command() {
  local raw="$1"
  local cleaned
  cleaned="$(strip_redirects "$raw")"

  if check_secret_paths "$cleaned"; then
    return 0
  fi

  local flat
  flat="$(flatten_metachars "$cleaned")"
  tokenize "$flat"

  local n=${#W[@]}
  local i base
  for ((i = 0; i < n; i++)); do
    base="${W[$i]##*/}"
    case "$base" in
      git)
        check_git "$i" && return 0
        ;;
      make)
        check_make "$i" && return 0
        ;;
      kubectl)
        check_kubectl "$i" && return 0
        ;;
      k3d)
        check_k3d "$i" && return 0
        ;;
      gh)
        check_gh "$i" && return 0
        ;;
    esac
  done
  return 1
}

if check_command "$COMMAND"; then
  printf 'bash-guard: ブロックしました - %s\n' "$BLOCK_REASON" >&2
  printf 'bash-guard: この操作は人間が自分の端末で実行することを想定しています。続行する前に人間の確認を得てください(ADR-0800 §2)。\n' >&2
  exit 2
fi

exit 0
