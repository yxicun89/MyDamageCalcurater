#!/usr/bin/env bash
# 公開前の機械的な検査(coding-rules §1、ADR-0002、docs/audit-r1.md の R-3)。
#
#   scripts/check-publishable.sh              既定(速い検査 A〜E)
#   scripts/check-publishable.sh --full       既定 + F(Git 作者情報・生成コードの差分。遅い)
#   scripts/check-publishable.sh --self-test  検査自体のテスト(違反を仕込んで検出できることを確認)
#
# 検査対象は `git ls-files` の追跡ファイルだけ。
# ヒットしても値そのものは出力しない(秘密・メールを端末やログに残さないため)。
# 表示するのは「[区分] ファイル:行  種類」だけ。1 件でもあれば終了コード 1。
#
#   A 絶対パス・個人情報(メール・端末名・ネットワーク)
#   B 秘密らしき文字列
#   C 追跡してはいけないファイル・サイズ・テキスト以外
#   D 第三者データ(testdata/golden の日本語、テストの NameJa)
#   E 再現性・依存(バージョン固定、lockfile、module path)
#   F Git メタデータと生成コード(--full のみ)
set -euo pipefail

# ---------------------------------------------------------------------------
# 許可リスト・除外(ここに集約する。足すときは理由を1行で書く)
# ---------------------------------------------------------------------------

# 検査パターンそのものを含むので、内容検査(A・B・E)から外す。
readonly SELF_PATH="scripts/check-publishable.sh"
# 公開用の Git identity を書く許可リスト。メールを含むので A から外す(F が読む)。
readonly AUTHORS_ALLOWLIST_PATH="scripts/publishable-authors.txt"
# git grep の pathspec。除外するファイルをここに足す。
readonly -a CONTENT_EXCLUDES=(
  ":(exclude)$SELF_PATH"
  ":(exclude)$AUTHORS_ALLOWLIST_PATH"
)

# A(絶対パス・個人情報)だけから外すファイル。
#   docs/audit-r1.md : 検査対象のパターン(/Users/ など)を説明する文書で、実際の値ではない
readonly -a A_EXCLUDES=(":(exclude)docs/audit-r1.md")

# B(秘密らしき文字列「キー名=値」)の許可(ERE)。値そのものではなく、k8s の Secret/Key の
# *名前*(pokedex-svc の manifest 検査。ADR-0105 §6・用途別の最小権限。ADR-0110)を指す
# 定数だけを対象にする(秘密の値ではない)。`tidb-root-auth` は ADR-0211 §3.2 の
# TidbInitializer が参照する Secret 名(`passwordSecret: tidb-root-auth`。値ではなく名前)。
# 3つ目の代替(`[:=][[:space:]]*"?\$\{[A-Za-z_][A-Za-z0-9_]*\}$`)は、scripts/up.sh が Secret の
# manifest を heredoc で組み立てる行(例: `mysql-root-password: "${mysql_root_pw_value}"`)を許す。
# 値の**全体**が単一のシェル変数参照であることまで要求する(区切り文字の直後から `${...}` が
# 始まり、他の文字を挟まない)。`password: "realsecret${x}"` のように本物の値へ無害な変数参照を
# 継ぎ足して検出を逃れる細工は、この形では通らない(self-test で確認)。
readonly B_KEYVALUE_ALLOW='=[[:space:]]*"(mysql-auth|pokedex-dsn|pokedex-reader-dsn|pokedex-importer-dsn|pokedex-migrator-dsn|mysql-root-password)$|:[[:space:]]*tidb-root-auth$|[:=][[:space:]]*"?\$\{[A-Za-z_][A-Za-z0-9_]*\}$'

# 許可するメールアドレス(ERE。一致した文字列全体に対して評価)。
#   noreply@anthropic.com : コミットの共同著者表記(公開情報)
#   @example.com/.org     : RFC 2606 の予約ドメイン(架空データ用)
readonly EMAIL_ALLOW='^noreply@anthropic\.com$|@example\.(com|org)$'
# 許可する IPv4(ERE。前後の区切り文字ごと評価)。
#   127.0.0.1 : ループバック / 0.0.0.0 : 全インターフェースの bind 指定(どちらも個人を特定しない)
readonly IPV4_ALLOW='(^|[^0-9.])(127\.0\.0\.1|0\.0\.0\.0)([^0-9.]|$)'

# サイズ上限(バイト)。ゴールデンのテストベクタ(.gz)は許可リストで別扱い。
readonly MAX_FILE_BYTES=3145728
# サイズ・テキスト以外の検査を免除するパス(bash の glob)。
#   testdata/golden の .gz: ADR-0002 が認めた例外(数値と英語識別子だけのテストベクタ。大きい・圧縮済み)
readonly -a BINARY_AND_LARGE_ALLOW=("testdata/golden/*.gz")
# テキストとみなす MIME(`file --mime-type`。bash の glob)。空ファイル(.gitkeep など)と JSON を含む。
# x-empty は macOS の file が返す表記(Linux は inode/x-empty)。
readonly -a TEXT_MIME_GLOBS=("text/*" "application/json" "inode/x-empty" "application/x-empty")

# 第三者データの検査対象(ADR-0002 の例外条件)。
readonly GOLDEN_DIR="testdata/golden"
# NameJa は、この接頭辞で始まる架空名か、日本語を含まない値(プレースホルダ "?" など。
# 実在の日本語名を入れないという目的に反しない)だけを許す。
readonly FICTIONAL_NAME_PREFIX="テスト"
readonly -a NAMEJA_PATHSPECS=("engine/*_test.go" "engine/wasmapi/testdata/vectors.json" "services/*_test.go" "services/pokedex/importer/testdata/*" "services/pokedex/internal/storetest/*")

# importer の架空データ(JSON の文字列値すべてが対象。ADR-0101)。
readonly IMPORTER_TESTDATA_DIR="services/pokedex/importer/testdata"
# data/importer/*.json のうち、日本語を一切禁止するもの(英語IDと4096基準の整数だけ。ADR-0101 §2)。
readonly -a IMPORTER_NO_JAPANESE_FILES=("data/importer/config.json" "data/importer/effects.json")
# レギュレーションの日本語ラベルだけ許すファイル(ADR-0101 §7 人間の確認事項3)。
readonly IMPORTER_REGULATIONS_FILE="data/importer/regulations.json"

# 日本語(ひらがな・カタカナ・漢字・半角カナ)の Unicode 範囲。ADR-0002: 1 文字でもあれば失敗。
readonly JAPANESE_CLASS='[\x{3040}-\x{30FF}\x{3400}-\x{4DBF}\x{4E00}-\x{9FFF}\x{F900}-\x{FAFF}\x{FF66}-\x{FF9F}]'
# 1 ファイルあたりに表示する日本語ヒット行の上限(全量を出すと端末が埋まるため)。
readonly JAPANESE_HITS_PER_FILE=20

# ---------------------------------------------------------------------------
# 結果の集計
# ---------------------------------------------------------------------------

count_A=0 count_B=0 count_C=0 count_D=0 count_E=0 count_F=0
NOTES=()

# report 区分 場所 種類 — 違反を1件表示して数える(値は出さない)。
report() {
  local category="$1" location="$2" kind="$3"
  printf '[%s] %s  %s\n' "$category" "$location" "$kind"
  eval "count_$category=\$((count_$category + 1))"
}

# note メッセージ — 違反ではない情報(スキップの明示など)。
note() {
  NOTES+=("$1")
  printf '[--] %s\n' "$1"
}

# ---------------------------------------------------------------------------
# 共通の走査部品
# ---------------------------------------------------------------------------

# scan_content 区分 種類 ERE [許可ERE] [i] — 追跡ファイルの内容を検査し、ヒットを file:line で報告する。
# git grep -o の出力(file:line:一致文字列)から一致文字列を許可リストで振り分け、値は捨てる。
# 区分固有の除外は変数 SCAN_EXTRA_EXCLUDES(pathspec の配列)で渡す。
SCAN_EXTRA_EXCLUDES=()
scan_content() {
  local category="$1" kind="$2" pattern="$3" allow="${4:-}" icase="${5:-}"
  local -a options=(-n -o -I -E)
  if [ -n "$icase" ]; then options+=(-i); fi
  local location
  while IFS= read -r location; do
    report "$category" "$location" "$kind"
  done < <(
    git grep "${options[@]}" -e "$pattern" -- . "${CONTENT_EXCLUDES[@]}" ${SCAN_EXTRA_EXCLUDES[@]+"${SCAN_EXTRA_EXCLUDES[@]}"} 2>/dev/null |
      ALLOW="$allow" awk -F: '{
        matched = $0
        sub(/^[^:]*:[0-9]+:/, "", matched)
        if (ENVIRON["ALLOW"] != "" && matched ~ ENVIRON["ALLOW"]) next
        print $1 ":" $2
      }' | sort -u || true
  )
}

# matches_any_glob 値 glob... — 値がいずれかの glob(bash の [[ ]])に一致すれば 0。
matches_any_glob() {
  local value="$1" glob
  shift
  for glob in "$@"; do
    # shellcheck disable=SC2053 # glob として評価するのが目的
    if [[ "$value" == $glob ]]; then return 0; fi
  done
  return 1
}

# ---------------------------------------------------------------------------
# A. 絶対パス・個人情報
# ---------------------------------------------------------------------------
check_a() {
  SCAN_EXTRA_EXCLUDES=("${A_EXCLUDES[@]}")
  # レーン用 worktree と権限定義の ~/.ssh は、利用者名を含まない共有のプレースホルダとして許可する。
  scan_content A "絶対パス(~/)" '~/[A-Za-z0-9.][A-Za-z0-9._*/<>-]*' \
    '^~/((MyDamageCalcurater|pokecalc)[A-Za-z0-9._*/<>-]*|\.ssh/\*\*)$'
  scan_content A "絶対パス(/Users/)" '/Users/'
  scan_content A "絶対パス(/home/)" '/home/[a-z]'
  scan_content A "絶対パス(/var/folders)" '/var/folders'
  scan_content A "絶対パス(/private/tmp)" '/private/tmp'
  scan_content A "絶対パス(Windows のユーザーフォルダ)" 'C:\\Users'
  scan_content A "メールアドレス" '[A-Za-z0-9._%+-]+@[A-Za-z0-9-]+(\.[A-Za-z0-9-]+)*\.[A-Za-z]{2,}' "$EMAIL_ALLOW"
  scan_content A "端末名" 'MacBook|iMac|Mac mini'
  scan_content A "ネットワーク(.ts.net)" '\.ts\.net'
  scan_content A "ネットワーク(IPv4)" '(^|[^0-9.])([0-9]{1,3}\.){3}[0-9]{1,3}([^0-9.]|$)' "$IPV4_ALLOW"
  SCAN_EXTRA_EXCLUDES=()
}

# ---------------------------------------------------------------------------
# B. 秘密らしき文字列
# ---------------------------------------------------------------------------
check_b() {
  scan_content B "秘密らしき文字列(キー名=値)" \
    '(password|passwd|secret|api[_-]?key|private[_-]?key|access[_-]?token)[A-Za-z0-9_-]*['"'"'"]?[[:space:]]*[:=][[:space:]]*['"'"'"]?[^[:space:]'"'"'"]{8,}' \
    "$B_KEYVALUE_ALLOW" i
  scan_content B "秘密らしき文字列(秘密鍵ブロック)" '-----BEGIN [A-Z ]*PRIVATE KEY-----'
  scan_content B "秘密らしき文字列(AWS アクセスキー形式)" 'AKIA[0-9A-Z]{16}'
  scan_content B "秘密らしき文字列(GitHub トークン形式)" 'gh[pousr]_[A-Za-z0-9]{36}'
  scan_content B "秘密らしき文字列(sk- 形式の API キー)" 'sk-[A-Za-z0-9]{20,}'
  scan_content B "秘密らしき文字列(JWT)" 'eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+'
}

# ---------------------------------------------------------------------------
# C. 追跡してはいけないファイル・サイズ・テキスト以外
# ---------------------------------------------------------------------------

# forbidden_kind パス — 追跡してはいけない名前なら理由を表示して 0、そうでなければ 1。
forbidden_kind() {
  local path="$1" base="${1##*/}"
  case "$path" in
    docs/local/* | */docs/local/*) echo "追跡禁止(個人用メモ docs/local/)"; return 0 ;;
    data/generated/* | */data/generated/*) echo "追跡禁止(第三者由来の生成データ data/generated/)"; return 0 ;;
    .reviews/* | */.reviews/*) echo "追跡禁止(レビュー成果物 .reviews/)"; return 0 ;;
    node_modules/* | */node_modules/*) echo "追跡禁止(node_modules/)"; return 0 ;;
  esac
  case "$base" in
    .env.example) return 1 ;;
    .env | .env.*) echo "追跡禁止(環境変数ファイル。サンプルは .env.example だけ)"; return 0 ;;
    *.pem | *.key | *.p12 | *.pfx | *.jks) echo "追跡禁止(鍵・証明書)"; return 0 ;;
    *.wasm) echo "追跡禁止(WASM 生成物。make wasm で作る)"; return 0 ;;
    kubeconfig*) echo "追跡禁止(kubeconfig)"; return 0 ;;
    .DS_Store) echo "追跡禁止(.DS_Store)"; return 0 ;;
  esac
  return 1
}

check_c() {
  local -a existing=()
  local path reason
  while IFS= read -r -d '' path; do
    if reason="$(forbidden_kind "$path")"; then
      report C "$path" "$reason"
    fi
    # 追跡されているが作業ツリーに無い(削除待ち)ものは、内容の検査ができないので除く。
    if [ -f "$path" ]; then existing+=("$path"); fi
  done < <(git ls-files -z)

  if [ "${#existing[@]}" -eq 0 ]; then return 0; fi

  # サイズ(wc -c をまとめて実行。最後の total 行は捨てる)
  local size
  while read -r size path; do
    [ "$path" = "total" ] && continue
    if [ "$size" -gt "$MAX_FILE_BYTES" ] && ! matches_any_glob "$path" "${BINARY_AND_LARGE_ALLOW[@]}"; then
      report C "$path" "サイズ超過(上限 ${MAX_FILE_BYTES} バイト)"
    fi
  done < <(printf '%s\0' "${existing[@]}" | xargs -0 wc -c)

  # テキスト以外(file をまとめて実行。出力は「パス| MIME」)
  local mime
  while IFS='|' read -r path mime; do
    mime="${mime# }"
    if ! matches_any_glob "$mime" "${TEXT_MIME_GLOBS[@]}" && ! matches_any_glob "$path" "${BINARY_AND_LARGE_ALLOW[@]}"; then
      report C "$path" "テキスト以外のファイル(${mime})"
    fi
  done < <(printf '%s\0' "${existing[@]}" | xargs -0 file -N --mime-type -F '|')
}

# ---------------------------------------------------------------------------
# D. 第三者データ(ADR-0002 の例外条件)
# ---------------------------------------------------------------------------
check_d() {
  command -v perl >/dev/null || { echo "check-publishable: perl が見つからない(D の日本語検査に必要)" >&2; exit 2; }

  # testdata/golden: .gz は展開して検査する。日本語が 1 文字でもあれば失敗。
  local path hits line
  while IFS= read -r -d '' path; do
    [ -f "$path" ] || continue
    hits=0
    while IFS= read -r line; do
      report D "$path:$line" "第三者データの疑い(日本語を含む。testdata/golden は数値と英語識別子だけ。ADR-0002)"
      hits=$((hits + 1))
    done < <(
      { if [[ "$path" == *.gz ]]; then gzip -dc -- "$path"; else cat -- "$path"; fi; } |
        perl -CS -X -ne 'if (/'"$JAPANESE_CLASS"'/) { print "$.\n"; exit if ++$n >= '"$JAPANESE_HITS_PER_FILE"' }' || true
    )
    if [ "$hits" -ge "$JAPANESE_HITS_PER_FILE" ]; then
      note "$path: 日本語のヒットが ${JAPANESE_HITS_PER_FILE} 行に達したので以降の表示を省略"
    fi
  done < <(git ls-files -z -- "$GOLDEN_DIR")

  # テストの NameJa: 架空名(テスト…)以外は失敗
  local location
  while IFS= read -r location; do
    report D "$location" "NameJa が架空名(${FICTIONAL_NAME_PREFIX}…)でも日本語なしのプレースホルダでもない(実在名をテストに入れない。ADR-0002)"
  done < <(
    git grep -n -o -I -E -e '[Nn]ameJa"?:[[:space:]]*"[^"]*' -- "${NAMEJA_PATHSPECS[@]}" 2>/dev/null |
      PREFIX="\"${FICTIONAL_NAME_PREFIX}" LC_ALL=C awk -F: '{
        matched = $0
        sub(/^[^:]*:[0-9]+:/, "", matched)
        if (index(matched, ENVIRON["PREFIX"]) > 0) next
        if (matched !~ /[^ -~]/) next
        print $1 ":" $2
      }' | sort -u || true
  )

  # services/pokedex/importer/testdata/ 配下の JSON: 文字列値に日本語があれば
  # 「テスト」始まりであることを要求する(架空データだけを許す。NameJa キーに限らず全値が対象)。
  while IFS= read -r -d '' path; do
    [ -f "$path" ] || continue
    while IFS= read -r line; do
      report D "$path:$line" "日本語が「${FICTIONAL_NAME_PREFIX}」始まりの文字列でない(services/pokedex/importer/testdata は架空データだけを許す)"
    done < <(
      perl -CS -Mutf8 -X -ne '
        while (/"((?:[^"\\]|\\.)*)"/g) {
          my $s = $1;
          if ($s =~ /'"$JAPANESE_CLASS"'/ && $s !~ /^'"$FICTIONAL_NAME_PREFIX"'/) { print "$.\n"; last }
        }
      ' <"$path" || true
    )
  done < <(git ls-files -z -- "$IMPORTER_TESTDATA_DIR")

  # data/importer/config.json・effects.json: 日本語を一切禁止する(英語IDと4096基準の整数だけ)。
  for path in "${IMPORTER_NO_JAPANESE_FILES[@]}"; do
    git ls-files --error-unmatch -- "$path" >/dev/null 2>&1 || continue
    [ -f "$path" ] || continue
    while IFS= read -r line; do
      report D "$path:$line" "日本語を含んではいけないファイル(英語IDと4096基準の整数だけ。ADR-0101 §2)"
    done < <(perl -CS -X -ne 'print "$.\n" if /'"$JAPANESE_CLASS"'/' <"$path" || true)
  done

  # data/importer/regulations.json: 日本語は nameJa の値だけ許す(人が管理するレギュレーション名)。
  if git ls-files --error-unmatch -- "$IMPORTER_REGULATIONS_FILE" >/dev/null 2>&1 && [ -f "$IMPORTER_REGULATIONS_FILE" ]; then
    while IFS= read -r line; do
      report D "$IMPORTER_REGULATIONS_FILE:$line" "nameJa 以外に日本語がある(regulations.json は日本語のレギュレーション名だけ許す)"
    done < <(perl -CS -X -ne 'print "$.\n" if /'"$JAPANESE_CLASS"'/ && !/[Nn]ameJa"?:/' <"$IMPORTER_REGULATIONS_FILE" || true)
  fi
}

# ---------------------------------------------------------------------------
# E. 再現性・依存
# ---------------------------------------------------------------------------
check_e() {
  # package.json は依存を完全固定(^ ~ 禁止)し、隣の package-lock.json を追跡する。
  local manifest lockfile
  while IFS= read -r -d '' manifest; do
    [ -f "$manifest" ] || continue
    local location
    while IFS= read -r location; do
      report E "$manifest:$location" "依存のバージョンが範囲指定(^ または ~)。完全固定にする"
    done < <(grep -n -o -E '"[^"]+"[[:space:]]*:[[:space:]]*"[\^~]' "$manifest" | cut -d: -f1 | sort -u || true)
    lockfile="${manifest%package.json}package-lock.json"
    if ! git ls-files --error-unmatch -- "$lockfile" >/dev/null 2>&1; then
      report E "$manifest" "lockfile(${lockfile##*/})が追跡されていない"
    fi
  done < <(git ls-files -z -- 'package.json' '*/package.json')

  # Go の module path にアカウント名を入れない(coding-rules §1。公開用は example.com/pokecalc)
  scan_content E "module path に GitHub のアカウント名(github.com/<誰か>/pokecalc)" \
    'github\.com/[A-Za-z0-9_.-]+/pokecalc'
}

# ---------------------------------------------------------------------------
# F. Git メタデータ・生成コード(--full のみ)
# ---------------------------------------------------------------------------

# mask_identity "名前 <メール>" — メールは先頭 1 文字だけ残してマスクする(x***@***)。
mask_identity() {
  local email="${1##*<}"
  email="${email%>*}"
  printf '%s***@***' "${email:0:1}"
}

check_f() {
  if [ ! -f "$AUTHORS_ALLOWLIST_PATH" ]; then
    note "F(Git 作者情報): スキップ(許可リスト ${AUTHORS_ALLOWLIST_PATH} 未作成。R-2-9 で履歴を書き換えた後に作る)"
  elif ! git rev-parse --verify -q HEAD >/dev/null; then
    note "F(Git 作者情報): スキップ(コミットが無い)"
  else
    # 作者・コミッターの identity を集め、許可リスト(1 行 1 identity。# はコメント)に無いものを報告する。
    local allowed identity count
    allowed="$(grep -v -e '^[[:space:]]*#' -e '^[[:space:]]*$' "$AUTHORS_ALLOWLIST_PATH" || true)"
    while read -r count identity; do
      if ! printf '%s\n' "$allowed" | grep -Fxq -- "$identity"; then
        report F "git-log" "許可リスト外の Git identity(メール $(mask_identity "$identity")、作者・コミッターとして ${count} 回)"
      fi
    done < <(git log --format='%an <%ae>%n%cn <%ce>' | sort | uniq -c | sed -E 's/^ *([0-9]+) /\1 /')
  fi

  # 生成コードが仕様と一致すること: make gen の前後で作業ツリーの差分が変わらないこと。
  # (作業中の未コミット変更があっても誤検知しないよう、実行前後を比べる)
  if [ ! -f Makefile ]; then
    note "F(生成コード): スキップ(Makefile が無い)"
    return 0
  fi
  local before after
  before="$(git diff | shasum)"
  make gen >/dev/null 2>&1 || { report F "make gen" "make gen が失敗した"; return 0; }
  after="$(git diff | shasum)"
  if [ "$before" != "$after" ]; then
    local changed
    while IFS= read -r changed; do
      report F "$changed" "make gen で差分が出た(生成コードが仕様と一致していない)"
    done < <(git diff --name-only)
  fi
}

# ---------------------------------------------------------------------------
# 自己テスト(--self-test)
# ---------------------------------------------------------------------------

# repeat 文字 回数 — 文字列の繰り返し。ダミー秘密を「検査パターンにそのまま一致する定数」にしないために使う。
repeat() {
  local out="" i
  for ((i = 0; i < $2; i++)); do out+="$1"; done
  printf '%s' "$out"
}

# 自己テストの結果(検出漏れ = 検査が意味を成さない)を数える。
SELFTEST_FAILURES=0

selftest_fail() {
  printf '  NG: %s\n' "$1" >&2
  SELFTEST_FAILURES=$((SELFTEST_FAILURES + 1))
}

# selftest_new_repo 名前 — 違反のない基準リポジトリを作り、そのパスを返す。
# 許可リストに載る値(noreply@anthropic.com、@example.com、127.0.0.1、.env.example、testdata/golden の .gz)を
# 含めて、許可リストが効いていること(誤検知しないこと)も確かめる。
selftest_new_repo() {
  local dir="$SELFTEST_TMP/$1"
  mkdir -p "$dir/tools/golden" "$dir/testdata/golden" "$dir/engine"
  (
    cd "$dir"
    git init -q
    printf 'Co-Authored-By: Claude <noreply@anthropic.com>\ncontact: dev@example.com\nbind 127.0.0.1 and 0.0.0.0\n' >README.md
    : >.env.example
    printf '{"name":"x","dependencies":{"pkg":"1.2.3"}}\n' >tools/golden/package.json
    printf '{}\n' >tools/golden/package-lock.json
    printf '{"base":100,"sp":32}\n' >testdata/golden/fixed.json
    printf '{"hp":100}\n{"hp":101}\n' | gzip -c >testdata/golden/vectors.jsonl.gz
    printf 'module example.com/pokecalc/engine\n' >engine/go.mod
    printf 'package engine\n\nvar x = Species{NameJa: "テスト種"}\nvar y = Item{NameJa: "?"}\n' >engine/sample_test.go
    git add -f -A
    GIT_AUTHOR_NAME=Allowed GIT_AUTHOR_EMAIL=allowed@example.com \
      GIT_COMMITTER_NAME=Allowed GIT_COMMITTER_EMAIL=allowed@example.com \
      git -c commit.gpgsign=false commit -q -m base
  )
  printf '%s' "$dir"
}

# selftest_run 名前 リポジトリ [オプション] — 検査を実行して出力を $SELFTEST_OUTPUT、終了コードを $SELFTEST_STATUS に入れる。
selftest_run() {
  local dir="$2"
  shift 2
  SELFTEST_STATUS=0
  SELFTEST_OUTPUT="$(cd "$dir" && bash "$SELFTEST_SCRIPT" "$@" 2>&1)" || SELFTEST_STATUS=$?
}

# selftest_expect_clean 名前 リポジトリ [オプション]
selftest_expect_clean() {
  local name="$1" dir="$2"
  shift 2
  selftest_run "$name" "$dir" "$@"
  if [ "$SELFTEST_STATUS" -ne 0 ]; then
    selftest_fail "$name: 違反が無いのに失敗した(終了コード $SELFTEST_STATUS)"
    printf '%s\n' "$SELFTEST_OUTPUT" >&2
  fi
}

# selftest_expect_hits 名前 リポジトリ 期待する部分文字列... — 失敗すること、各期待が出力に含まれることを確認する。
# 直前に selftest_run 済みであること。値の漏れは selftest_expect_no_leak で確認する。
selftest_expect_hits() {
  local name="$1" expected
  shift
  if [ "$SELFTEST_STATUS" -eq 0 ]; then
    selftest_fail "$name: 違反を仕込んだのに成功した(検出できていない)"
    return 0
  fi
  for expected in "$@"; do
    if ! printf '%s\n' "$SELFTEST_OUTPUT" | grep -Fq -- "$expected"; then
      selftest_fail "$name: 検出されていない: $expected"
    fi
  done
}

# selftest_expect_no_hit 名前 除外パターン... — 出力にそのファイル名の検出行が無いこと
# (許可リストが効いて誤検知していないことの確認。selftest_expect_hits の逆)。
selftest_expect_no_hit() {
  local name="$1" excluded
  shift
  for excluded in "$@"; do
    if printf '%s\n' "$SELFTEST_OUTPUT" | grep -Fq -- "$excluded"; then
      selftest_fail "$name: 許可リストが効かず誤検知している: $excluded"
    fi
  done
}

# selftest_expect_no_leak 名前 値... — 出力に値そのものが含まれないこと。
selftest_expect_no_leak() {
  local name="$1" value
  shift
  for value in "$@"; do
    if printf '%s\n' "$SELFTEST_OUTPUT" | grep -Fq -- "$value"; then
      selftest_fail "$name: 出力に値が含まれている(値は出さない規則に違反)"
    fi
  done
}

# selftest_add 内容 リポジトリ 相対パス — 追跡ファイルを 1 つ足す(内容は printf の書式なしで書く)。
selftest_add() {
  local content="$1" dir="$2" path="$3"
  mkdir -p "$dir/$(dirname "$path")"
  printf '%s\n' "$content" >"$dir/$path"
  (cd "$dir" && git add -f "$path")
}

selftest() {
  SELFTEST_TMP="$(mktemp -d)"
  trap 'rm -rf "${SELFTEST_TMP:-}"' EXIT
  SELFTEST_SCRIPT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/$(basename "${BASH_SOURCE[0]}")"
  local dir

  echo "自己テスト: 違反のない基準リポジトリ(許可リストの値を含む)"
  dir="$(selftest_new_repo clean)"
  selftest_expect_clean "基準(既定)" "$dir"
  selftest_expect_clean "基準(--full)" "$dir" --full

  echo "自己テスト: A 絶対パス・個人情報"
  dir="$(selftest_new_repo a)"
  local email="dummy.person@mailbox.test"
  selftest_add "p=~/work" "$dir" a1.txt
  selftest_add "p=/home/dummyuser/work" "$dir" a2.txt
  selftest_add "p=/var/folders/zz/dummy" "$dir" a3.txt
  selftest_add "p=/private/tmp/dummy" "$dir" a4.txt
  selftest_add 'p=C:\Users\dummyuser' "$dir" a5.txt
  selftest_add "mail: $email" "$dir" a6.txt
  selftest_add "host: dummy-MacBook-Pro" "$dir" a7.txt
  selftest_add "host: dummy.tail1234.ts.net" "$dir" a8.txt
  selftest_add "ip: 192.0.2.10" "$dir" a9.txt
  selftest_run "A" "$dir"
  selftest_expect_hits "A" a1.txt:1 a2.txt:1 a3.txt:1 a4.txt:1 a5.txt:1 a6.txt:1 a7.txt:1 a8.txt:1 a9.txt:1
  selftest_expect_no_leak "A" dummyuser "$email" dummy-MacBook tail1234 192.0.2.10

  echo "自己テスト: B 秘密らしき文字列"
  dir="$(selftest_new_repo b)"
  local pw="dummyvalue$(repeat 9 4)" aws="AKI""A$(repeat X 16)" gh="gh""p_$(repeat a 36)"
  local sk="sk""-$(repeat b 24)" jwt="ey""Jhbg.ey""Jzdw.$(repeat c 8)" pem="-----BEGIN ""RSA PRIVATE KEY-----"
  selftest_add "password = \"$pw\"" "$dir" b1.txt
  selftest_add "{\"client_secret_key\": \"$pw\"}" "$dir" b7.txt
  selftest_add "$pem" "$dir" b2.txt
  selftest_add "id: $aws" "$dir" b3.txt
  selftest_add "token: $gh" "$dir" b4.txt
  selftest_add "key: $sk" "$dir" b5.txt
  selftest_add "auth: $jwt" "$dir" b6.txt
  # b8: シェル変数参照だけの行(scripts/up.sh が Secret manifest を heredoc で組み立てる形。
  # ADR-0211 §3.2)は許可リストで見逃す(値がハードコードされた秘密ではないため)。
  selftest_add 'root: "${tidb_root_pw_value}"' "$dir" b8.txt
  # b9: 本物の値らしき文字列の末尾に無害な変数参照を継ぎ足しただけでは許可リストをすり抜けない
  # こと(b8 の許可条件が「値の末尾が変数参照」ではなく「値の全体が変数参照」であることの確認)。
  local sneaky="hunter2secretvalue"
  selftest_add "password: \"${sneaky}\${x}\"" "$dir" b9.txt
  selftest_run "B" "$dir"
  selftest_expect_hits "B" b1.txt:1 b7.txt:1 b2.txt:1 b3.txt:1 b4.txt:1 b5.txt:1 b6.txt:1 b9.txt:1
  selftest_expect_no_hit "B" b8.txt
  selftest_expect_no_leak "B" "$pw" "$aws" "$gh" "$sk" "$jwt" "RSA PRIVATE" "$sneaky"

  echo "自己テスト: C 追跡してはいけないファイル・サイズ・テキスト以外"
  dir="$(selftest_new_repo c)"
  selftest_add "X=1" "$dir" .env
  selftest_add "dummy" "$dir" certs/dummy.pem
  selftest_add "dummy" "$dir" certs/dummy.key
  selftest_add "dummy" "$dir" web/public/engine.wasm
  selftest_add "dummy" "$dir" kubeconfig-local.yaml
  selftest_add "dummy" "$dir" docs/local/note.md
  selftest_add "dummy" "$dir" data/generated/master.json
  selftest_add "dummy" "$dir" .reviews/r1.md
  selftest_add "dummy" "$dir" node_modules/pkg/index.js
  selftest_add "dummy" "$dir" .DS_Store
  head -c $((MAX_FILE_BYTES + 1000)) /dev/zero | tr '\0' 'x' >"$dir/big.txt"
  head -c 2048 /dev/urandom >"$dir/blob.bin"
  (cd "$dir" && git add -f big.txt blob.bin)
  selftest_run "C" "$dir"
  selftest_expect_hits "C" .env certs/dummy.pem certs/dummy.key web/public/engine.wasm kubeconfig-local.yaml \
    docs/local/note.md data/generated/master.json .reviews/r1.md node_modules/pkg/index.js .DS_Store \
    "big.txt  サイズ超過" "blob.bin  テキスト以外"

  echo "自己テスト: D 第三者データ"
  dir="$(selftest_new_repo d)"
  selftest_add '{"name":"ひらがな"}' "$dir" testdata/golden/d1.json
  printf '{"name":"カタカナ"}\n' | gzip -c >"$dir/testdata/golden/d2.jsonl.gz"
  selftest_add '{"name":"漢字"}' "$dir" testdata/golden/d3.json
  selftest_add 'package engine
var y = Species{NameJa: "実在の名前"}' "$dir" engine/d4_test.go
  selftest_add '[{"nameJa":"実在の名前"}]' "$dir" engine/wasmapi/testdata/vectors.json
  (cd "$dir" && git add -f testdata/golden/d2.jsonl.gz)
  selftest_run "D" "$dir"
  selftest_expect_hits "D" testdata/golden/d1.json:1 testdata/golden/d2.jsonl.gz:1 testdata/golden/d3.json:1 \
    engine/d4_test.go:2 engine/wasmapi/testdata/vectors.json:1
  selftest_expect_no_leak "D" ひらがな カタカナ 漢字 実在の名前

  echo "自己テスト: D 追加(importer 固有の日本語検査)"
  dir="$(selftest_new_repo d2)"
  selftest_add '{"nameJa":"実在の名前"}' "$dir" services/pokedex/importer/testdata/fictional/importer/regulations.json
  selftest_add '{"note":"テスト"}' "$dir" data/importer/config.json
  selftest_add '{"items":{"testorb":"テスト"}}' "$dir" data/importer/effects.json
  selftest_add '{
  "id":"m-c",
  "nameJa":"テストレギュ",
  "note":"実在の注記"
}' "$dir" data/importer/regulations.json
  selftest_run "D2" "$dir"
  selftest_expect_hits "D2" \
    "services/pokedex/importer/testdata/fictional/importer/regulations.json:1" \
    "data/importer/config.json:1" "data/importer/effects.json:1" "data/importer/regulations.json:4"
  selftest_expect_no_leak "D2" 実在の名前 実在の注記

  echo "自己テスト: D 追加(架空データ・nameJa だけの日本語は誤検知しない)"
  dir="$(selftest_new_repo d3)"
  selftest_add '{"nameJa":"テストモン"}' "$dir" services/pokedex/importer/testdata/fictional/species.json
  selftest_add '{
  "id":"m-c",
  "nameJa":"テストレギュ"
}' "$dir" data/importer/regulations.json
  selftest_expect_clean "D3(importer 固有チェックの誤検知なし)" "$dir"

  echo "自己テスト: E 再現性・依存"
  dir="$(selftest_new_repo e)"
  selftest_add '{"dependencies":{"pkg":"^1.2.3"}}' "$dir" tools/golden/package.json
  selftest_add '{"dependencies":{"pkg":"1.2.3"}}' "$dir" web/package.json
  selftest_add 'module github.com/dummyaccount/pokecalc/engine' "$dir" engine/go.mod
  selftest_run "E" "$dir"
  selftest_expect_hits "E" tools/golden/package.json:1 "web/package.json  lockfile" engine/go.mod:1
  selftest_expect_no_leak "E" dummyaccount
  (cd "$dir" && git rm -q -f tools/golden/package-lock.json)
  selftest_run "E(lockfile)" "$dir"
  selftest_expect_hits "E(lockfile)" "tools/golden/package.json  lockfile"

  echo "自己テスト: F Git 作者情報(--full)"
  dir="$(selftest_new_repo f)"
  selftest_run "F(許可リスト無し)" "$dir" --full
  selftest_expect_clean "F(許可リスト無し)" "$dir" --full
  if ! printf '%s\n' "$SELFTEST_OUTPUT" | grep -Fq "スキップ(許可リスト"; then
    selftest_fail "F: 許可リストが無いときのスキップ表示が出ない"
  fi
  selftest_add '# 公開用の identity' "$dir" scripts/publishable-authors.txt
  printf 'Allowed <allowed@example.com>\n' >>"$dir/scripts/publishable-authors.txt"
  (cd "$dir" && git add -f scripts/publishable-authors.txt &&
    GIT_AUTHOR_NAME=Other GIT_AUTHOR_EMAIL=other.person@mailbox.test \
      GIT_COMMITTER_NAME=Allowed GIT_COMMITTER_EMAIL=allowed@example.com \
      git -c commit.gpgsign=false commit -q -m other)
  selftest_run "F" "$dir" --full
  selftest_expect_hits "F" "[F] git-log" "o***@***"
  selftest_expect_no_leak "F" other.person mailbox.test
  # 許可リストに足せば通る
  printf 'Other <other.person@mailbox.test>\n' >>"$dir/scripts/publishable-authors.txt"
  selftest_expect_clean "F(許可リストに追加後)" "$dir" --full

  if [ "$SELFTEST_FAILURES" -ne 0 ]; then
    echo "自己テスト失敗: ${SELFTEST_FAILURES} 件(検出できない検査がある)" >&2
    return 1
  fi
  echo "自己テスト成功: A〜F の違反を検出し、値は出力に含まれず、違反が無ければ成功する"
}

# ---------------------------------------------------------------------------
# エントリポイント
# ---------------------------------------------------------------------------

usage() {
  echo "使い方: $0 [--full | --self-test]" >&2
}

main() {
  local full=0 mode=check
  case "${1:-}" in
    "") ;;
    --full) full=1 ;;
    --self-test) mode=selftest ;;
    -h | --help) usage; return 0 ;;
    *) usage; return 2 ;;
  esac
  if [ "$#" -gt 1 ]; then usage; return 2; fi

  if [ "$mode" = selftest ]; then
    selftest
    return
  fi

  command -v git >/dev/null || { echo "check-publishable: git が見つからない" >&2; return 2; }
  local root
  root="$(git rev-parse --show-toplevel)" || { echo "check-publishable: git リポジトリの中で実行する" >&2; return 2; }
  cd "$root"

  check_a
  check_b
  check_c
  check_d
  check_e
  if [ "$full" -eq 1 ]; then check_f; fi

  local total=$((count_A + count_B + count_C + count_D + count_E + count_F))
  local scope="既定(A〜E)"
  if [ "$full" -eq 1 ]; then scope="--full(A〜F)"; fi
  if [ "$total" -eq 0 ]; then
    echo "check-publishable: 0 件(${scope})"
    return 0
  fi
  echo "check-publishable: 違反 ${total} 件(${scope}: A=${count_A} B=${count_B} C=${count_C} D=${count_D} E=${count_E} F=${count_F})。値は表示していない。該当箇所を確認して直す" >&2
  return 1
}

main "$@"
