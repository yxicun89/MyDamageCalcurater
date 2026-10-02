#!/usr/bin/env bash
# Claude Code / Codex の PreToolUse フックから呼ばれる Bash コマンドの危険操作ガード(ADR-0800。issue #273・#239)。
# stdin から `{"tool_name":"Bash","tool_input":{"command":"..."}}` 形式の JSON を受け取り、
# 文字列だけを見て判定する(判定対象のコマンドは実行しない。例外は読み取り専用の2つだけ:
# git push の宛先解決の `git rev-parse --abbrev-ref HEAD`(ADR-0800 §2)と、
# PR マージの CI 検証の `gh pr checks <検証済み引数>`(ADR-0803))。
#   - 危険と判定: stderr に理由を書いて exit 2(呼び出し元が人間に確認を求める)
#   - 危険でない: 何も出力せず exit 0
#   - command が無い・空、tool_name が Bash 以外、jq が無い環境: 判定できないので exit 0(fail-open)
# 方式: 引用符とグルーピング記号を空白に、コマンドの区切り(&&・||・;・|・&・改行)を番兵トークン
# "__SEP__" に置き換えて1本のトークン列に平坦化し、git・make・kubectl・k3d・gh のいずれかのトークンが
# 現れるたびに、その位置から対応する check_* 関数を呼ぶ(bash -c・eval・サブシェル・前置きコマンド等の
# 形に関わらず本体コマンドを拾うため)。各 check_* 内の「このコマンド自身の引数」を集めるループは
# "__SEP__" に達したら止める(止めないと後続コマンドのトークンを自分の引数と誤認する)。
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

# flatten_metachars 文字列 — クォート文字とシェルのメタ文字を空白に置き換え、1本のトークン列にする。
# bash -c・eval・サブシェル・前置きコマンド等の形に関わらず、本体のコマンド名
# (git・make・kubectl・k3d・gh)をトークン列から拾えるようにするため。
# コマンドの区切り(&&・||・;・|・&・改行)は、単なる空白ではなく番兵トークン "__SEP__" に変える
# (critic 3回目指摘: 空白に潰すと「このコマンド自身の引数はどこまでか」が分からなくなり、
# `kubectl delete ...; echo ok` の "echo ok" を delete の対象候補と誤認したり、逆に
# `git push origin && gh pr create ...` で "origin" の後に続く別コマンドのトークンが無いことを
# 「宛先省略」と区別できなくなったりする。呼び出し側の引数収集ループは "__SEP__" で止める)。
# 括弧・波括弧・バッククォート・リダイレクト記号・クォートは、コマンド自身の引数の区切りではない
# (グルーピング/クォートの記号でしかない)ので、従来通り単なる空白にする。
flatten_metachars() {
  local s="$1"
  local ch
  # 行継続(バックスラッシュ+改行)は区切りではなく単なる空白。改行を区切りにする前に空白へ変える
  # (critic 5回目指摘: 先に区切りにすると `git push origin \<改行>main` が宛先なしに見えて通ってしまう)
  s="${s//\\$'\n'/ }"
  for ch in '&' '|' ';' $'\n'; do
    s="${s//$ch/ __SEP__ }"
  done
  for ch in '(' ')' '{' '}' '<' '>' '`' $'\t'; do
    s="${s//$ch/ }"
  done
  # クォートとバックスラッシュは空白ではなく削除する(critic 4回目指摘: 空白にすると
  # `gi""t push origin mai""n` や `ma\in` が単語ごと割れて判定をすり抜ける。削除すればシェルが
  # 解釈する後の単語と一致する)。
  for ch in '"' "'" '\\'; do
    s="${s//$ch/}"
  done
  printf '%s' "$s"
}

# resolve_current_branch [ディレクトリ] — 現在のブランチ名を返す(判定できなければ空文字)。
# git push の宛先が省略されている・HEAD/@ の場合にだけ呼ぶ(I/O を使う唯一の箇所。ADR-0800 §2)。
# ディレクトリが指定されていれば `git -C <dir>` で解決する("git -C <dir> push"・"cd <dir> && git push" の
# ように宛先解決の基準ディレクトリがフック自身の cwd と異なるケースに対応するため。呼び出し側の check_git 参照)。
resolve_current_branch() {
  local dir="${1:-}"
  if [ -n "$dir" ]; then
    git -C "$dir" rev-parse --abbrev-ref HEAD 2>/dev/null
  else
    git rev-parse --abbrev-ref HEAD 2>/dev/null
  fi
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

# find_kubectl_verb 開始位置 — グローバル配列 W の開始位置(kubectl トークンの index)より後ろから、
# 既知のサブコマンド(動詞)集合に一致する最初のトークンを探し、KV_VERB・KV_VERB_IDX に設定する。
# フラグを個別に読み飛ばそうとはしない(critic 3回目指摘: "--request-timeout 30s delete pvc x" のように
# 値ありフラグの一覧に無いフラグの値("30s")を動詞と誤認するバグがあったため)。動詞集合に一致するトークンを
# 見つけるまで前方のトークン(フラグ・その値・グローバルフラグの値)を無条件に読み飛ばす方式にすることで、
# フラグの値が既知の動詞と偶然一致する極端なケースを除き誤認しない。ADR-0800 §2 の「見逃すより誤検知」方針により、
# 動詞を見逃すリスクの方を優先して潰す。
find_kubectl_verb() {
  local start="$1"
  local n=${#W[@]}
  local i t
  KV_VERB=""
  KV_VERB_IDX=-1
  for ((i = start + 1; i < n; i++)); do
    t="${W[$i]}"
    case "$t" in
      get | delete | describe | apply | logs | rollout | exec | cp | edit | patch | scale | expose | create | replace | run | port-forward | top | drain | cordon | uncordon | label | annotate | taint | wait | explain | diff | kustomize | proxy | auth | api-resources | api-versions | cluster-info | completion | config | events | attach | debug | plugin | version)
        KV_VERB="$t"
        KV_VERB_IDX=$i
        return 0
        ;;
    esac
  done
  return 1
}

# kubectl_collect_candidates 開始位置 — グローバル配列 W の開始位置から末尾まで、フラグ(値あり・値なし・
# "="自己完結。kubectl_flag_takes_value を使う)を読み飛ばした残りのトークンをグローバル配列 CANDIDATES に集める。
# 値ありフラグの一覧に無い未知のフラグに出会うと、その値を誤って候補に含めてしまうことがあるが
# (例: "--timeout 60s" の "60s")、以降のトークンも引き続き候補として集め続けるため、本来のリソース種別
# ("pvc" 等)を見逃さない。呼び出し側は候補が0件(パイプ/xargs等で対象が渡ってくる形)・"$"始まり
# (コマンド置換・変数展開で動的に決まる形)を安全側でブロックする材料として使う(critic 3回目指摘)。
# "__SEP__" 番兵に達したら止める(このコマンド自身の引数は、区切り記号の前までしかない。
# 止めないと、"kubectl delete ...; echo ok" の "echo ok" のような後続コマンドのトークンを
# 削除対象と誤認してしまう。critic 3回目指摘)。
CANDIDATES=()
kubectl_collect_candidates() {
  local start="$1"
  local n=${#W[@]}
  local j t
  CANDIDATES=()
  j="$start"
  while [ "$j" -lt "$n" ]; do
    t="${W[$j]}"
    case "$t" in
      __SEP__) break ;;
      -*)
        case "$t" in
          *=*) j=$((j + 1)) ;;
          *)
            if kubectl_flag_takes_value "$t"; then
              j=$((j + 2))
            else
              j=$((j + 1))
            fi
            ;;
        esac
        ;;
      *)
        CANDIDATES+=("$t")
        j=$((j + 1))
        ;;
    esac
  done
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
      ns | namespace | namespaces | pvc | persistentvolumeclaim | persistentvolumeclaims | pv | persistentvolume | statefulset | statefulsets | sts | secret | secrets | all)
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
#   - delete -k/--kustomize/-f/--filename/-R/--recursive(まとめて削除・間接指定)は常にブロック
#   - delete の対象候補(動詞の後ろのフラグ以外のトークン全て)に ns/pvc/pv/statefulset/secret/all が
#     含まれればブロック(pod/job は対象外)
#   - delete の対象候補が0件(パイプ/xargs越しの動的な対象)、または "$" 始まり(コマンド置換・変数展開で
#     動的に決まる対象)ならブロック(対象不明時は安全側でブロックする。critic 3回目指摘)
#   - get の対象候補に secret(s) が含まれればブロック
check_kubectl() {
  local start="$1"
  find_kubectl_verb "$start" || return 1
  local verb="$KV_VERB"
  local rstart=$((KV_VERB_IDX + 1))
  local n=${#W[@]}
  local j c

  if [ "$verb" = "delete" ]; then
    for ((j = rstart; j < n; j++)); do
      case "${W[$j]}" in
        __SEP__) break ;;
        --kustomize* | --filename* | --recursive* | -[kfR]* | -[!-]*[kfR]*)
          # 短いフラグのまとめ書き(-Rf・-fR)・連結形(-fdeploy.yaml)も含む(critic 4回目指摘)。
          # -lapp=foo のような f を含む別フラグは誤検知になるが、ADR-0800 §2 により許容する
          BLOCK_REASON="kubectl delete -k/--kustomize/-f/--filename/-R/--recursive はまとめて・間接的に削除するため常に確認が必要です"
          return 0
          ;;
      esac
    done

    kubectl_collect_candidates "$rstart"

    if [ "${#CANDIDATES[@]}" -eq 0 ]; then
      BLOCK_REASON="kubectl delete の対象がコマンド上で特定できません(パイプ/xargs 等からの動的な対象の可能性があるため確認が必要です)"
      return 0
    fi

    for c in "${CANDIDATES[@]}"; do
      case "$c" in
        \$*)
          BLOCK_REASON="kubectl delete の対象がコマンド置換/変数展開で動的に決まります(${c})"
          return 0
          ;;
      esac
    done

    for c in "${CANDIDATES[@]}"; do
      if kubectl_resource_is_destructive "$c"; then
        BLOCK_REASON="kubectl delete の対象に namespace/pvc/pv/statefulset/secret/all が含まれます(${c})"
        return 0
      fi
    done
    return 1
  fi

  if [ "$verb" = "get" ]; then
    kubectl_collect_candidates "$rstart"
    for c in "${CANDIDATES[@]}"; do
      if kubectl_resource_is_secret "$c"; then
        BLOCK_REASON="kubectl get secret は秘密情報を出力するため常に確認が必要です"
        return 0
      fi
    done
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

# gh_merge_flag_takes_value フラグ — "gh pr merge" で次のトークンを値として消費するフラグ。
gh_merge_flag_takes_value() {
  case "$1" in
    -R | --repo | -b | --body | -F | --body-file | -t | --subject | --match-head-commit)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

# run_with_timeout 秒 コマンド... — timeout / gtimeout があればそれで包んで実行する。
# macOS には timeout が標準では無いため、無ければタイムアウト無しで実行する
# (gh 自身の HTTP 標準動作に任せる。無限に待つ可能性は残るが、ここでの失敗は fail-safe=block 側)。
run_with_timeout() {
  local secs="$1"
  shift
  if command -v timeout >/dev/null 2>&1; then
    timeout "$secs" "$@"
  elif command -v gtimeout >/dev/null 2>&1; then
    gtimeout "$secs" "$@"
  else
    "$@"
  fi
}

# check_pr_merge_gate リポジトリ 引数トークン... — PR のマージを CI 全件成功のときだけ通す(ADR-0803)。
# 成功(CI 緑)なら return 1(ブロックしない)、それ以外はすべて BLOCK_REASON を設定して return 0(ブロック)。
# 実行する外部コマンドは、検証済み引数の読み取り専用 `gh pr checks` だけ(判定対象のマージ自体は実行しない)。
check_pr_merge_gate() {
  local repo="$1"
  shift
  local safe='^[A-Za-z0-9._/:@#-]+$'
  local target="" npos=0 t
  local repo_args=()

  # コマンド置換・変数展開はシェルが実行時に値を決めるため、検証した PR とマージされる PR が食い違いうる。
  if printf '%s' "$COMMAND" | grep -Eq '[$`]'; then
    BLOCK_REASON="マージのコマンドに \$ またはバッククォートがあり、対象 PR を静的に確定できません(CI 検証不能)"
    return 0
  fi

  # クォート内の区切り文字(`-R "o/r;x"` 等)は、平坦化後のトークン列では本物の区切りと区別できない。
  # クォート/バックスラッシュと区切り文字(; & | 改行 < >)が同居するコマンドは静的に確定できないのでブロックする
  # (行継続の「バックスラッシュ+改行」だけは除いて判定する)。
  local nocont="${COMMAND//\\$'\n'/ }"
  if [[ "$nocont" == *[\"\'\\]* ]] && { [[ "$nocont" == *[\;\&\|\<\>]* ]] || [[ "$nocont" == *$'\n'* ]]; }; then
    BLOCK_REASON="クォート/バックスラッシュと区切り文字が同居し、対象 PR を静的に確定できません(CI 検証不能)"
    return 0
  fi

  while [ "$#" -gt 0 ]; do
    t="$1"
    shift
    case "$t" in
      --admin | --admin=*)
        BLOCK_REASON="--admin 付きのマージは CI 検証を回避するため許可できません(ADR-0803)"
        return 0
        ;;
      -R | --repo)
        repo="${1:-}"
        [ "$#" -gt 0 ] && shift
        ;;
      --repo=*)
        repo="${t#--repo=}"
        ;;
      -*=*) ;;
      -*)
        if gh_merge_flag_takes_value "$t"; then
          [ "$#" -gt 0 ] && shift
        fi
        ;;
      *)
        npos=$((npos + 1))
        target="$t"
        ;;
    esac
  done

  if [ "$npos" -gt 1 ]; then
    BLOCK_REASON="マージの対象 PR を一意に決められません(位置引数が複数。CI 検証不能)"
    return 0
  fi
  if [ -n "$target" ] && ! [[ "$target" =~ $safe ]]; then
    BLOCK_REASON="マージの対象 PR の指定に許可されない文字があります(CI 検証不能): $target"
    return 0
  fi
  if [ -n "$repo" ]; then
    if ! [[ "$repo" =~ $safe ]]; then
      BLOCK_REASON="-R/--repo の値に許可されない文字があります(CI 検証不能): $repo"
      return 0
    fi
    repo_args=(-R "$repo")
  fi
  if ! command -v gh >/dev/null 2>&1; then
    BLOCK_REASON="gh が見つからず、PR の CI 状態を確認できません(fail-safe でブロック)"
    return 0
  fi

  local rc=0
  if [ -n "$target" ]; then
    run_with_timeout 20 gh pr checks "${repo_args[@]+"${repo_args[@]}"}" "$target" </dev/null >/dev/null 2>&1 || rc=$?
  else
    run_with_timeout 20 gh pr checks "${repo_args[@]+"${repo_args[@]}"}" </dev/null >/dev/null 2>&1 || rc=$?
  fi
  if [ "$rc" = 0 ]; then
    return 1
  fi
  BLOCK_REASON="対象 PR の CI が全件成功ではありません(gh pr checks の終了コード ${rc}。1=失敗 8=未完了 その他=判定不能)。CI を直して全件成功にしてから再実行してください(ADR-0803)"
  return 0
}

# check_gh 開始位置 — グローバル配列 W の gh トークン位置から判定する。
#   - "-R owner/repo"・"--repo owner/repo" 等のフラグを読み飛ばした上で PR マージなら
#     check_pr_merge_gate(対象 PR の CI が全件成功のときだけ通す。--admin は常にブロック。ADR-0803)
#   - "gh api ..." でパスに "/merge" を含むものはブロック(API 直叩きでの CI 検証回避)
check_gh() {
  local start="$1"
  local n=${#W[@]}
  local i=$((start + 1))
  local t first="" second="" repo=""
  while [ "$i" -lt "$n" ]; do
    t="${W[$i]}"
    [ "$t" = "__SEP__" ] && break
    case "$t" in
      -*)
        case "$t" in
          --repo=*)
            repo="${t#--repo=}"
            i=$((i + 1))
            ;;
          *=*) i=$((i + 1)) ;;
          *)
            if gh_flag_takes_value "$t"; then
              case "$t" in -R | --repo) repo="${W[$((i + 1))]:-}" ;; esac
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
    local args=()
    while [ "$i" -lt "$n" ] && [ "${W[$i]}" != "__SEP__" ]; do
      args+=("${W[$i]}")
      i=$((i + 1))
    done
    check_pr_merge_gate "$repo" "${args[@]+"${args[@]}"}" && return 0
    return 1
  fi

  if [ "$first" = "api" ]; then
    local j
    for ((j = start + 1; j < n; j++)); do
      case "${W[$j]}" in
        */merge | */merge/* | */merges | */merges/*)
          BLOCK_REASON="gh api で /merge(s) を含むパスを叩いています(PR の確定マージに相当し、CI 検証を迂回します)"
          return 0
          ;;
        *mergePullRequest*)
          BLOCK_REASON="gh api で GraphQL の mergePullRequest を呼んでいます(PR の確定マージに相当し、CI 検証を迂回します)"
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
#       - 現在のブランチの解決は、"git -C <dir> push" の "<dir>"、無ければ "cd <dir> && ... git push" の
#         直近の "<dir>"(git トークンより前で最後に現れた "cd" の次のトークン)を基準にする
#         (critic 3回目指摘: フック自身の cwd で解決すると、別チェックアウトの実際のブランチを見誤る)。
#         どちらも無ければフックの cwd を使う。
check_git() {
  local start="$1"
  local n=${#W[@]}
  local i=$((start + 1))
  local t sub="" c_dir=""
  while [ "$i" -lt "$n" ]; do
    t="${W[$i]}"
    case "$t" in
      __SEP__) return 1 ;;
      -C)
        c_dir="${W[$((i + 1))]:-}"
        i=$((i + 2))
        ;;
      -c | --git-dir | --work-tree | --namespace)
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

  local resolve_dir="$c_dir"
  if [ -z "$resolve_dir" ]; then
    local k
    for ((k = start - 1; k >= 0; k--)); do
      if [ "${W[$k]}" = "cd" ]; then
        resolve_dir="${W[$((k + 1))]:-}"
        break
      fi
    done
  fi

  local start2=$((i + 1))
  local j
  for ((j = start2; j < n; j++)); do
    case "${W[$j]}" in
      __SEP__) break ;;
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

  # remote_seen が立つまでの最初の非フラグトークンは remote 名(例: "origin")であって
  # 宛先(refspec)ではない。critic 3回目指摘: これを宛先候補に含めていたため、
  # "git push origin"(refspec 省略)が「宛先あり("origin")」と誤認され、現在のブランチを
  # 確認しないまま通ってしまうバグがあった。remote の次以降のトークンだけを宛先候補にする。
  local remote_seen=0
  local any_dest=0
  local tok dst cur
  for ((j = start2; j < n; j++)); do
    tok="${W[$j]}"
    case "$tok" in
      __SEP__) break ;;
      # 値を取るフラグは次のトークン(値)ごと読み飛ばす。読み飛ばさないと値が remote 扱いになり、
      # 本物の remote が宛先に数えられて現在のブランチを確認しなくなる(critic 4回目指摘)
      -o | --push-option | --receive-pack | --exec | --repo | --recurse-submodules)
        j=$((j + 1))
        continue
        ;;
      -*) continue ;;
    esac
    if [ "$remote_seen" = 0 ]; then
      remote_seen=1
      continue
    fi
    any_dest=1
    dst="${tok##*:}"
    case "$dst" in
      main | heads/main | refs/heads/main)
        BLOCK_REASON="git push の宛先が main です(${tok})"
        return 0
        ;;
      \$*)
        BLOCK_REASON="git push の宛先が変数・コマンド置換で動的に決まるため main か判定できません(${tok})"
        return 0
        ;;
      "" | HEAD | @)
        cur="$(resolve_current_branch "$resolve_dir")"
        if [ -z "$cur" ] || [ "$cur" = "main" ]; then
          BLOCK_REASON="git push の宛先が現在のブランチに解決され、それが main か判定できません(現在のブランチ: ${cur:-不明})"
          return 0
        fi
        ;;
    esac
  done

  if [ "$any_dest" = 0 ]; then
    cur="$(resolve_current_branch "$resolve_dir")"
    if [ -z "$cur" ] || [ "$cur" = "main" ]; then
      BLOCK_REASON="git push に宛先の指定が無く(remote だけ、または省略)、現在のブランチが main か判定できません(現在のブランチ: ${cur:-不明})"
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

  # リダイレクト除去"前"の生の文字列でも .env/.ssh を検査する(strip_redirects が
  # "cat < .env"・"cat <~/.ssh/id_rsa" のようなリダイレクト対象そのものを消してしまい、
  # 除去後の文字列だけでは見失うケースがあるため。除去後の文字列でも従来通り検査する)。
  if check_secret_paths "$raw"; then
    return 0
  fi
  if check_secret_paths "$cleaned"; then
    return 0
  fi

  local flat
  flat="$(flatten_metachars "$cleaned")"
  tokenize "$flat"

  local n=${#W[@]}
  local i base
  for ((i = 0; i < n; i++)); do
    # コマンド名の比較は大文字小文字を区別せず、エイリアス無効化の先頭 "\" も剥がしてから行う
    # ("GIT push origin main"・"\git push origin main" のような形も検出するため)。
    base="${W[$i]##*/}"
    base="${base#\\}"
    base="$(lower "$base")"
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
