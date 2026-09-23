#!/usr/bin/env bash
# scripts/argocd-bootstrap.sh の自動テスト(ADR-0405 §2。issue #105)。`make test-scripts`(make test に含む)から流す。
#
# 方式: 手書きの ok/ng ヘルパー付きの bash スクリプト。ルートの scripts/ には Go モジュールが無く、
# 既存の前例も scripts/check-publishable.sh --self-test(同じく手書きヘルパー)なので、それに合わせて依存を増やさない
# (bats は入れない)。services/gateway/deploytest は API レーンの Go パッケージなので、運用レーンの検査は置かない。
#
# 実クラスタ・実ネットワークには触らない:
#   - curl と kubectl は PATH の先頭に置いた偽物に差し替える(curl は固定の manifest を返し、kubectl は呼び出しを記録するだけ)。
#   - 念のため KUBECONFIG を存在しないファイルに、HTTP(S) のプロキシを閉じたポートに向け、本物が呼ばれても何も起きないようにする。
#   - 対象スクリプトは一時ディレクトリの使い捨て git リポジトリへコピーして流す(`cd "$(git rev-parse --show-toplevel)"` がそこへ向く)。
#     定数(期待ハッシュ・digest)を差し替えるケースはコピー側だけを書き換える。
#
# manifest は実物(上流の install.yaml)ではなく、同じ image 参照だけを持つ最小の架空 YAML を使う。
# 実物の SHA-256 とは一致しないので、「実物の期待ハッシュのまま流すと不一致で止まる」ことの検査にもなる。
set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
readonly ROOT
readonly SCRIPT_REL="scripts/argocd-bootstrap.sh"
readonly SCRIPT="$ROOT/$SCRIPT_REL"

# 上流 v3.5.3 の install.yaml に実際に現れる image 参照(タグ形式)。版を上げたらここも上流に合わせて直す。
readonly ARGOCD_TAG_REF="quay.io/argoproj/argocd:v3.5.3"
readonly DEX_TAG_REF="ghcr.io/dexidp/dex:v2.45.1"
readonly REDIS_TAG_REF="public.ecr.aws/docker/library/redis:8.2.3-alpine"
readonly ARGOCD_REPO="quay.io/argoproj/argocd"
readonly DEX_REPO="ghcr.io/dexidp/dex"
readonly REDIS_REPO="public.ecr.aws/docker/library/redis"

readonly DIGEST_CONSTS="ARGOCD_IMAGE_DIGEST DEX_IMAGE_DIGEST REDIS_IMAGE_DIGEST"

REAL_MKTEMP=$(command -v mktemp)
readonly REAL_MKTEMP
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

FAILURES=0
PASSES=0
CURRENT=""

ok() { PASSES=$((PASSES + 1)); }
ng() {
  printf '  NG [%s]: %s\n' "$CURRENT" "$1" >&2
  FAILURES=$((FAILURES + 1))
}
begin() {
  CURRENT=$1
  printf '%s\n' "- $1"
}

sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{ print $1 }'
  else
    shasum -a 256 "$1" | awk '{ print $1 }'
  fi
}

# get_const ファイル 名前 — `NAME=value` / `readonly NAME=value`(引用符あり・なし)の値を返す。
get_const() {
  sed -nE "s/^[[:space:]]*(readonly[[:space:]]+)?$2=\"?([^\"[:space:]#]*)\"?.*$/\2/p" "$1" | head -n 1
}

# count_const ファイル 名前 — 定数の定義行の数。
count_const() {
  grep -cE "^[[:space:]]*(readonly[[:space:]]+)?$2=" "$1" || true
}

# set_const ファイル 名前 値 — コピーしたスクリプトの定数の定義行を書き換える(値は空文字も可)。
set_const() {
  local file=$1 name=$2 value=$3
  if [ "$(count_const "$file" "$name")" != 1 ]; then
    ng "$SCRIPT_REL に $name の定義行(\`$name=...\` または \`readonly $name=...\`)がちょうど1行ない。テストが差し替えられない"
    return 1
  fi
  awk -v name="$name" -v value="$value" '
    $0 ~ ("^[[:space:]]*(readonly[[:space:]]+)?" name "=") { sub(name "=.*$", name "=\"" value "\"") }
    { print }
  ' "$file" >"$file.new" && mv "$file.new" "$file" && chmod +x "$file"
}

# new_repo 名前 — 使い捨て git リポジトリに対象スクリプトをコピーし、そのパスを返す。
new_repo() {
  local dir="$WORK/$1/repo"
  mkdir -p "$dir/scripts"
  git -C "$dir" init -q
  cp "$SCRIPT" "$dir/$SCRIPT_REL"
  chmod +x "$dir/$SCRIPT_REL"
  printf '%s' "$dir/$SCRIPT_REL"
}

# write_fixture ファイル [追加の image 行...] — 架空の最小 manifest。image は上流と同じタグ参照
# (argocd は上流と同じく複数回・initContainer にも現れる)。追加の引数は末尾の Deployment の image 行として足す。
write_fixture() {
  local file=$1
  shift
  cat >"$file" <<EOF
# test fixture for scripts/argocd-bootstrap_test.sh (not the upstream manifest)
apiVersion: apps/v1
kind: Deployment
metadata:
  name: argocd-server
spec:
  template:
    spec:
      containers:
      - name: argocd-server
        image: $ARGOCD_TAG_REF
        imagePullPolicy: Always
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: argocd-repo-server
spec:
  template:
    spec:
      initContainers:
      - name: copyutil
        image: $ARGOCD_TAG_REF
      containers:
      - name: argocd-repo-server
        image: $ARGOCD_TAG_REF
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: argocd-dex-server
spec:
  template:
    spec:
      initContainers:
      - name: copyutil
        image: $ARGOCD_TAG_REF
      containers:
      - name: dex
        image: $DEX_TAG_REF
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: argocd-redis
spec:
  template:
    spec:
      containers:
      - name: redis
        image: $REDIS_TAG_REF
EOF
  local extra
  for extra in "$@"; do
    printf '      - name: extra\n        image: %s\n' "$extra" >>"$file"
  done
}

# install_fakes ディレクトリ — 偽の curl と kubectl を置く。
install_fakes() {
  local bin="$1/bin"
  mkdir -p "$bin"
  cat >"$bin/curl" <<'EOF'
#!/bin/sh
# 偽の curl: 呼び出しを記録し、FAKE_FIXTURE の内容を -o/--output のファイル(無ければ標準出力)へ書く。
printf '%s\n' "$*" >>"$FAKE_LOG_DIR/curl.log"
out=""
url=""
while [ $# -gt 0 ]; do
  case "$1" in
    -o | --output) out=$2; shift 2; continue ;;
    --output=*) out=${1#--output=} ;;
    -[!-]*o) out=$2; shift 2; continue ;;
    http://* | https://*) url=$1 ;;
  esac
  shift
done
printf '%s\n' "$url" >>"$FAKE_LOG_DIR/curl.urls"
if [ -n "$out" ]; then cp "$FAKE_FIXTURE" "$out"; else cat "$FAKE_FIXTURE"; fi
EOF
  cat >"$bin/kubectl" <<'EOF'
#!/bin/sh
# 偽の kubectl: 呼び出しを記録するだけ。apply は適用内容を applied.yaml に保存する。
printf '%s\n' "$*" >>"$FAKE_LOG_DIR/kubectl.log"
case " $* " in
  *" apply "*)
    prev=""
    for a in "$@"; do
      if [ "$prev" = "-f" ] || [ "$prev" = "--filename" ]; then
        if [ "$a" = "-" ]; then cat >"$FAKE_LOG_DIR/applied.yaml"; else cp "$a" "$FAKE_LOG_DIR/applied.yaml"; fi
      fi
      prev=$a
    done
    exit 0 ;;
  *" get "*argocd*)
    case " $* " in
      *" namespace "* | *" ns "* | *" namespaces "* | *namespace/argocd*)
        [ "${FAKE_NS_EXISTS:-0}" = 1 ] && exit 0
        echo 'Error from server (NotFound): namespaces "argocd" not found' >&2
        exit 1 ;;
    esac
    exit 0 ;;
  *" create "*" namespace "*argocd* | *" create "*" ns "*argocd*)
    if [ "${FAKE_NS_EXISTS:-0}" = 1 ]; then
      echo 'Error from server (AlreadyExists): namespaces "argocd" already exists' >&2
      exit 1
    fi
    exit 0 ;;
esac
exit 0
EOF
  # 偽の mktemp: macOS の `mktemp -d` は TMPDIR を見ないことがあるので、作る場所をケースの tmp に寄せ、作ったパスを記録する
  # (終了時に消えたかを確かめるため)。
  cat >"$bin/mktemp" <<'EOF'
#!/bin/sh
dir=0
for a in "$@"; do
  case "$a" in -d | -[!-]*d*) dir=1 ;; esac
done
if [ "$dir" = 1 ]; then p=$("$REAL_MKTEMP" -d "$FAKE_TMP_ROOT/mktemp.XXXXXX"); else p=$("$REAL_MKTEMP" "$FAKE_TMP_ROOT/mktemp.XXXXXX"); fi || exit 1
printf '%s\n' "$p" >>"$FAKE_LOG_DIR/mktemp.log"
printf '%s\n' "$p"
EOF
  chmod +x "$bin/curl" "$bin/kubectl" "$bin/mktemp"
}

# 直近の run_script の結果。
RC=0
CASE_DIR=""
OUT=""

# run_script ケース名 スクリプト manifest 名前空間あり(0/1) [VAR=値...] — 偽物の下でスクリプトを流す。
run_script() {
  local name=$1 script=$2 fixture=$3 ns_exists=$4
  shift 4
  CASE_DIR="$WORK/$name/case"
  mkdir -p "$CASE_DIR/logs" "$CASE_DIR/tmp"
  : >"$CASE_DIR/logs/mktemp.log"
  : >"$CASE_DIR/logs/curl.log"
  : >"$CASE_DIR/logs/curl.urls"
  : >"$CASE_DIR/logs/kubectl.log"
  install_fakes "$CASE_DIR"
  (
    # 利用者と同じくリポジトリの中から呼ぶ(ルートではなく scripts/ から呼び、スクリプト自身がルートへ cd することも通す)。
    cd "$(dirname "$script")" &&
      env "$@" \
        PATH="$CASE_DIR/bin:$PATH" \
        TMPDIR="$CASE_DIR/tmp" \
        REAL_MKTEMP="$REAL_MKTEMP" \
        FAKE_TMP_ROOT="$CASE_DIR/tmp" \
        FAKE_LOG_DIR="$CASE_DIR/logs" \
        FAKE_FIXTURE="$fixture" \
        FAKE_NS_EXISTS="$ns_exists" \
        KUBECONFIG="$CASE_DIR/no-such-kubeconfig" \
        HTTP_PROXY="http://127.0.0.1:9" http_proxy="http://127.0.0.1:9" \
        HTTPS_PROXY="http://127.0.0.1:9" https_proxy="http://127.0.0.1:9" \
        ALL_PROXY="http://127.0.0.1:9" all_proxy="http://127.0.0.1:9" \
        NO_PROXY="" no_proxy="" \
        "$script" </dev/null >"$CASE_DIR/out" 2>&1
  )
  RC=$?
  OUT=$(cat "$CASE_DIR/out")
}

kubectl_called_with() { grep -Eq "$1" "$CASE_DIR/logs/kubectl.log"; }
apply_called() { kubectl_called_with '(^|[[:space:]])apply([[:space:]]|$)'; }
curl_called() { [ -s "$CASE_DIR/logs/curl.log" ]; }
# 一時ディレクトリの置き場(TMPDIR と偽の mktemp の置き場は同じ)が空 = スクリプトが作った一時物は終了時に消えた。
tmp_is_clean() { [ -z "$(ls -A "$CASE_DIR/tmp")" ]; }
mktemp_dir_called() { [ -s "$CASE_DIR/logs/mktemp.log" ]; }

# expect_rejected — 直近の実行が「適用前に非0で止まった」こと。一時ディレクトリも消えていること。
expect_rejected() {
  if [ "$RC" -eq 0 ]; then ng "終了コードが 0(非0で止まるべき)。出力: $OUT"; else ok; fi
  if apply_called; then ng "kubectl apply が呼ばれた(適用前に止まるべき)。kubectl: $(tr '\n' ';' <"$CASE_DIR/logs/kubectl.log")"; else ok; fi
  if tmp_is_clean; then ok; else ng "失敗時に一時ディレクトリが残った(trap で消すべき): $(ls -A "$CASE_DIR/tmp")"; fi
}

# expect_fetched — 取得(偽の curl)まで到達したこと。スタブのように何もせず止まるだけの実装を合格にしないため。
expect_fetched() {
  if curl_called; then ok; else ng "curl が呼ばれていない(manifest を取得してから検証して止まるべき)。出力: $OUT"; fi
}

# fixture_repo ケース名 manifest — 期待ハッシュだけを manifest に合わせたコピー(digest 定数は本物のまま)。
fixture_repo() {
  local script
  script=$(new_repo "$1") || return 1
  set_const "$script" EXPECTED_INSTALL_YAML_SHA256 "$(sha256_of "$2")" || return 1
  printf '%s' "$script"
}

# ---------------------------------------------------------------------------
# 1. スクリプトの形(静的)
# ---------------------------------------------------------------------------

test_static_shape() {
  begin "静的: 実行可能・bash・構文・ルートへの cd"
  if [ ! -f "$SCRIPT" ]; then
    ng "$SCRIPT_REL が無い"
    return
  fi
  if [ -x "$SCRIPT" ]; then ok; else ng "$SCRIPT_REL に実行ビットが無い"; fi
  if [ "$(head -n 1 "$SCRIPT")" = "#!/usr/bin/env bash" ]; then ok; else ng "1行目が #!/usr/bin/env bash でない"; fi
  if bash -n "$SCRIPT" 2>/dev/null; then ok; else ng "bash -n で構文エラー"; fi
  # shellcheck disable=SC2016 # 文字列そのものを探す
  if grep -Fq 'cd "$(git rev-parse --show-toplevel)"' "$SCRIPT"; then ok; else
    ng 'cd "$(git rev-parse --show-toplevel)" から始まっていない(ADR-0405 §1)'
  fi
}

test_static_constants() {
  begin "静的: 固定値がこのファイルに1回ずつ・正しい形で定義されている"
  local name value
  for name in ARGOCD_INSTALL_COMMIT EXPECTED_INSTALL_YAML_SHA256 $DIGEST_CONSTS; do
    if [ "$(count_const "$SCRIPT" "$name")" = 1 ]; then ok; else ng "$name の定義行がちょうど1行ない"; fi
  done
  value=$(get_const "$SCRIPT" ARGOCD_INSTALL_COMMIT)
  if printf '%s' "$value" | grep -Eq '^[0-9a-f]{40}$'; then ok; else ng "ARGOCD_INSTALL_COMMIT がコミットSHA(40桁の16進)でない: '$value'"; fi
  value=$(get_const "$SCRIPT" EXPECTED_INSTALL_YAML_SHA256)
  if printf '%s' "$value" | grep -Eq '^[0-9a-f]{64}$'; then ok; else ng "EXPECTED_INSTALL_YAML_SHA256 が64桁の16進でない: '$value'"; fi
  for name in $DIGEST_CONSTS; do
    value=$(get_const "$SCRIPT" "$name")
    if printf '%s' "$value" | grep -Eq '^sha256:[0-9a-f]{64}$'; then ok; else ng "$name が sha256:<64桁の16進> でない: '$value'"; fi
  done
}

test_static_no_tag_url() {
  begin "静的: タグの URL を取得・直接 apply しない"
  if grep -Eq 'argoproj/argo-cd/v[0-9]' "$SCRIPT"; then ng "タグ(v...)の URL が残っている。コミットSHAで固定する"; else ok; fi
  if grep -Eq 'kubectl[^#]*apply[^#]*-f[[:space:]]+https?://' "$SCRIPT"; then ng "リモート URL を kubectl apply へ直接渡している"; else ok; fi
}

# 固定値を二重管理しない(issue #105 受け入れ条件)。runbook・サービスのスクリプト・Makefile に値を写さない。
test_constants_not_duplicated() {
  begin "静的: 固定値を runbook・サービス側に二重に持たない"
  local name value hits
  for name in ARGOCD_INSTALL_COMMIT EXPECTED_INSTALL_YAML_SHA256 $DIGEST_CONSTS; do
    value=$(get_const "$SCRIPT" "$name")
    [ -n "$value" ] || continue
    value=${value#sha256:}
    hits=$(grep -rlF "$value" "$ROOT/docs/runbooks" "$ROOT/services" "$ROOT/Makefile" "$ROOT/deploy" 2>/dev/null || true)
    if [ -z "$hits" ]; then ok; else ng "$name の値が $SCRIPT_REL 以外にもある: $(printf '%s' "$hits" | sed "s#$ROOT/##g" | tr '\n' ' ')"; fi
  done
}

# ---------------------------------------------------------------------------
# 2. 振る舞い(偽の curl・kubectl の下で流す)
# ---------------------------------------------------------------------------

test_happy_path() {
  begin "正常系: 期待ハッシュ一致・3イメージとも digest 化 → apply まで到達する"
  local fixture="$WORK/happy.yaml" script commit want_url line name repo digest
  write_fixture "$fixture"
  script=$(fixture_repo happy "$fixture") || return
  run_script happy "$script" "$fixture" 0

  if [ "$RC" -eq 0 ]; then ok; else ng "終了コード $RC(0 であるべき)。出力: $OUT"; fi

  commit=$(get_const "$script" ARGOCD_INSTALL_COMMIT)
  want_url="https://raw.githubusercontent.com/argoproj/argo-cd/$commit/manifests/install.yaml"
  if [ "$(sort -u "$CASE_DIR/logs/curl.urls")" = "$want_url" ]; then ok; else
    ng "取得 URL が $want_url(コミットSHA固定)でない: $(tr '\n' ' ' <"$CASE_DIR/logs/curl.urls")"
  fi

  if kubectl_called_with '(^|[[:space:]])apply([[:space:]]|$)' &&
    kubectl_called_with '(^|[[:space:]])--server-side([[:space:]=]|$)' &&
    kubectl_called_with '(^|[[:space:]])-f -([[:space:]]|$)' &&
    kubectl_called_with '((^|[[:space:]])-n[[:space:]]+argocd|--namespace[[:space:]=]argocd)([[:space:]]|$)'; then ok; else
    ng "kubectl apply -n argocd --server-side -f - が呼ばれていない。kubectl: $(tr '\n' ';' <"$CASE_DIR/logs/kubectl.log")"
  fi
  if kubectl_called_with 'rollout status .*deployment/argocd-server'; then ok; else ng "rollout status deployment/argocd-server を待っていない"; fi
  if grep -nE '(^|[[:space:]])apply([[:space:]]|$)' "$CASE_DIR/logs/kubectl.log" | head -n 1 | cut -d: -f1 |
    { read -r a; r=$(grep -nE 'rollout status' "$CASE_DIR/logs/kubectl.log" | head -n 1 | cut -d: -f1); [ -n "${a:-}" ] && [ -n "$r" ] && [ "$a" -lt "$r" ]; }; then ok; else
    ng "apply の後に rollout status を呼んでいない"
  fi

  local applied="$CASE_DIR/logs/applied.yaml"
  if [ ! -s "$applied" ]; then
    ng "apply に manifest が渡っていない(標準入力が空)"
  else
    # image 行はすべて <repo>@sha256:<64桁> 。タグ参照は1つも残らない。
    local bad
    bad=$(grep -E '^[[:space:]]*(-[[:space:]]+)?image:' "$applied" |
      sed -E 's/^[[:space:]]*(-[[:space:]]+)?image:[[:space:]]*//; s/^"//; s/"[[:space:]]*$//; s/[[:space:]]+$//' |
      grep -Ev '^[^@]+@sha256:[0-9a-f]{64}$' || true)
    if [ -z "$bad" ]; then ok; else ng "digest 形式でない image が適用された: $bad"; fi
    # 件数は write_fixture の image 行の数(argocd は server・repo-server の init と本体・dex の init の4箇所)。
    for pair in "ARGOCD_IMAGE_DIGEST:$ARGOCD_REPO:4" "DEX_IMAGE_DIGEST:$DEX_REPO:1" "REDIS_IMAGE_DIGEST:$REDIS_REPO:1"; do
      name=${pair%%:*}
      line=${pair#*:}
      repo=${line%:*}
      local want_n=${line##*:}
      digest=$(get_const "$script" "$name")
      local got_n
      got_n=$(grep -cF "image: $repo@$digest" "$applied" || true)
      if [ "$got_n" = "$want_n" ]; then ok; else ng "image: $repo@\$$name が $want_n 箇所であるべきところ $got_n 箇所"; fi
    done
    # イメージ以外は書き換えない(manifest の他の行はそのまま)。
    if [ "$(grep -vE 'image:' "$fixture")" = "$(grep -vE 'image:' "$applied")" ]; then ok; else
      ng "image 行以外の内容が変わっている(image の参照だけを書き換えるべき)"
    fi
  fi
  if mktemp_dir_called; then ok; else ng "取得用の一時ディレクトリを mktemp -d で作っていない(ADR-0405 §1)"; fi
  if tmp_is_clean; then ok; else ng "成功時に一時ディレクトリが残った(trap で消すべき): $(ls -A "$CASE_DIR/tmp")"; fi
}

test_namespace_idempotent() {
  begin "正常系: argocd namespace が無ければ作り、あれば作らない(冪等。ADR-0605)"
  local fixture="$WORK/ns.yaml" script
  write_fixture "$fixture"
  script=$(fixture_repo ns-missing "$fixture") || return
  run_script ns-missing "$script" "$fixture" 0
  if kubectl_called_with 'create (namespace|ns) argocd'; then ok; else ng "namespace が無いのに create namespace argocd を呼んでいない"; fi
  if [ "$RC" -eq 0 ]; then ok; else ng "namespace が無いときに失敗した: $OUT"; fi

  script=$(fixture_repo ns-exists "$fixture") || return
  run_script ns-exists "$script" "$fixture" 1
  if kubectl_called_with 'create (namespace|ns) argocd'; then ng "namespace があるのに create namespace argocd を呼んだ(balance 導入済みで speed から再実行すると壊れる)"; else ok; fi
  if [ "$RC" -eq 0 ]; then ok; else ng "namespace があるときに失敗した(冪等であるべき): $OUT"; fi
  if apply_called; then ok; else ng "namespace があるときに apply まで到達しない"; fi
}

test_hash_mismatch_real_constant() {
  begin "ハッシュ不一致: 本物の期待ハッシュのまま、内容の違う manifest を返す → 適用前に止まる"
  local fixture="$WORK/tampered-real.yaml" script
  write_fixture "$fixture"
  script=$(new_repo hash-real) || return
  run_script hash-real "$script" "$fixture" 0
  expect_fetched
  expect_rejected
}

test_hash_mismatch_tampered_after_review() {
  begin "ハッシュ不一致: 期待ハッシュを取った後に1行改ざんされた manifest → 適用前に止まる"
  local reviewed="$WORK/reviewed.yaml" tampered="$WORK/tampered.yaml" script
  write_fixture "$reviewed"
  script=$(fixture_repo hash-tampered "$reviewed") || return
  sed 's/name: argocd-server$/name: argocd-server-evil/' "$reviewed" >"$tampered"
  run_script hash-tampered "$script" "$tampered" 0
  expect_fetched
  expect_rejected
}

test_hash_not_overridable_by_env() {
  begin "ハッシュ不一致: 環境変数で期待ハッシュ・取得元を上書きしても検証を迂回できない"
  local fixture="$WORK/env.yaml" script
  write_fixture "$fixture"
  script=$(new_repo hash-env) || return
  run_script hash-env "$script" "$fixture" 0 \
    EXPECTED_INSTALL_YAML_SHA256="$(sha256_of "$fixture")" \
    ARGOCD_INSTALL_COMMIT=v3.5.3
  expect_rejected
  if grep -q 'argo-cd/v3.5.3/' "$CASE_DIR/logs/curl.urls"; then ng "環境変数 ARGOCD_INSTALL_COMMIT でタグの URL に差し替えられた"; else ok; fi
}

test_tag_image_left_unpinned() {
  begin "digest 未固定: 上流の image 参照が想定と違い書き換えられない → 適用前に止まる"
  # redis を別レジストリの同名タグにした manifest(書き換え対象に一致しないのでタグのまま残る)。
  local fixture="$WORK/unpinned.yaml" script
  write_fixture "$fixture"
  sed "s#image: $REDIS_TAG_REF#image: docker.io/library/redis:8.2.3-alpine#" "$fixture" >"$fixture.new" && mv "$fixture.new" "$fixture"
  script=$(fixture_repo unpinned "$fixture") || return
  run_script unpinned "$script" "$fixture" 0
  expect_fetched
  expect_rejected
}

test_extra_tag_image() {
  begin "digest 未固定: 3イメージ以外のタグ参照の image が混ざっている → 適用前に止まる"
  local fixture="$WORK/extra.yaml" script
  write_fixture "$fixture" "docker.io/library/busybox:1.37"
  script=$(fixture_repo extra "$fixture") || return
  run_script extra "$script" "$fixture" 0
  expect_fetched
  expect_rejected
}

test_malformed_digest_constant() {
  local name fixture script
  for name in $DIGEST_CONSTS; do
    begin "digest 不正: $name が sha256:<64桁> でない(63桁) → 適用前に止まる"
    fixture="$WORK/malformed-$name.yaml"
    write_fixture "$fixture"
    script=$(fixture_repo "malformed-$name" "$fixture") || continue
    set_const "$script" "$name" "sha256:$(printf '%063d' 0)" || continue
    run_script "malformed-$name" "$script" "$fixture" 0
    expect_fetched
    expect_rejected
  done
}

test_empty_constants() {
  local name fixture script
  for name in EXPECTED_INSTALL_YAML_SHA256 $DIGEST_CONSTS; do
    begin "定数が空: $name=\"\" → 適用前に止まる"
    fixture="$WORK/empty-$name.yaml"
    write_fixture "$fixture"
    if [ "$name" = EXPECTED_INSTALL_YAML_SHA256 ]; then
      script=$(new_repo "empty-$name") || continue
    else
      script=$(fixture_repo "empty-$name" "$fixture") || continue
    fi
    set_const "$script" "$name" "" || continue
    run_script "empty-$name" "$script" "$fixture" 0
    expect_rejected
  done
}

# ---------------------------------------------------------------------------
# 3. runbook(balance §3・speed §5)が共有スクリプトを呼ぶ形になっている(ADR-0405 §3)
# ---------------------------------------------------------------------------

# runbook_section ファイル — 「## <番号>. Argo CD を入れる」節の本文(次の「## 」の手前まで)。
runbook_section() {
  awk '
    /^## [0-9]+\. Argo CD を入れる/ { on = 1; print; next }
    on && /^## / { exit }
    on { print }
  ' "$1"
}

check_runbook() {
  local rel=$1 section
  section=$(runbook_section "$ROOT/$rel")
  if [ -z "$section" ]; then
    ng "$rel に「## <番号>. Argo CD を入れる」節が無い"
    return
  fi
  if printf '%s\n' "$section" | grep -Eq '^\./scripts/argocd-bootstrap\.sh[[:space:]]*$'; then ok; else
    ng "$rel の Argo CD 導入節が ./scripts/argocd-bootstrap.sh を1行で呼んでいない"
  fi
  # shellcheck disable=SC2016 # 文字列そのものを探す
  if printf '%s\n' "$section" | grep -Fq 'cd "$(git rev-parse --show-toplevel)"'; then ok; else
    ng "$rel の Argo CD 導入節のコマンドが cd \"\$(git rev-parse --show-toplevel)\" から始まっていない"
  fi
  if printf '%s\n' "$section" | grep -Eq 'kubectl[^`]*apply'; then ng "$rel の Argo CD 導入節に kubectl apply が残っている"; else ok; fi
  if grep -Fq 'raw.githubusercontent.com/argoproj/argo-cd' "$ROOT/$rel"; then ng "$rel に Argo CD の install.yaml の生 URL が残っている"; else ok; fi
}

test_runbooks() {
  begin "runbook: balance §3 が共有スクリプトを呼ぶ"
  check_runbook docs/runbooks/balance.md
  begin "runbook: speed §5 が共有スクリプトを呼び、「導入済みならとばす」の注記を残す"
  check_runbook docs/runbooks/speed.md
  if runbook_section "$ROOT/docs/runbooks/speed.md" | grep -q 'とばす'; then ok; else
    ng "docs/runbooks/speed.md の Argo CD 導入節から「すでに入れていればとばす」の注記が消えた(ADR-0405 §3)"
  fi
}

# ---------------------------------------------------------------------------

test_static_shape
test_static_constants
test_static_no_tag_url
test_constants_not_duplicated
test_happy_path
test_namespace_idempotent
test_hash_mismatch_real_constant
test_hash_mismatch_tampered_after_review
test_hash_not_overridable_by_env
test_tag_image_left_unpinned
test_extra_tag_image
test_malformed_digest_constant
test_empty_constants
test_runbooks

if [ "$FAILURES" -gt 0 ]; then
  printf 'argocd-bootstrap test: %d failed, %d passed\n' "$FAILURES" "$PASSES" >&2
  exit 1
fi
printf 'argocd-bootstrap test: all %d checks passed\n' "$PASSES"
