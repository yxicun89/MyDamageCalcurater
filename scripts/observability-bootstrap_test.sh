#!/usr/bin/env bash
# scripts/observability-bootstrap.sh と deploy/k8s/base/observability/ の自動テスト(ADR-0406 §4〜5)。
# `make test-scripts`(make test に含む)から流す。
#
# 方式は scripts/argocd-bootstrap_test.sh(ADR-0405)と同じ: 手書きの ok/ng ヘルパー付きの bash(bats は入れない)。
#
# 実クラスタ・実ネットワークには触らない:
#   - スクリプトの振る舞いは、helm・kubectl・mktemp を PATH の先頭に置いた偽物に差し替えて流す
#     (helm pull は架空の .tgz を返し、helm upgrade・kubectl は呼び出しを記録するだけ)。
#   - 念のため KUBECONFIG を存在しないファイルに、HTTP(S) のプロキシを閉じたポートに向ける。
#   - 対象スクリプトは一時ディレクトリの使い捨て git リポジトリへコピーして流す。定数(期待ハッシュ)を差し替える
#     ケースはコピー側だけを書き換える。
#   - values・ServiceMonitor の構造の検査には本物の helm を使うが、`helm template` をこのテストが一時ディレクトリに
#     作るローカルの最小 chart に対して流すだけ(chart repo へは行かない。HELM_*_HOME も一時ディレクトリに向ける)。
#     YAML を構造として読むための道具で、yq 等の依存を増やさないため(helm は doctor.sh の前提ツール)。
#
# .tgz は実物ではなく架空の中身なので、実物の期待ハッシュのままでは必ず不一致になる
# (「本物の定数のまま流すと止まる」ことの検査にもなる)。
set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
readonly ROOT
readonly SCRIPT_REL="scripts/observability-bootstrap.sh"
readonly SCRIPT="$ROOT/$SCRIPT_REL"
readonly ADR_REL="docs/adr/0406-observability-metrics-and-stack.md"
readonly OBS_REL="deploy/k8s/base/observability"
readonly VALUES_REL="$OBS_REL/values"
readonly SM_REL="$OBS_REL/servicemonitors"
readonly NAMESPACE="observability"

# chart の名前(values ファイル名・helm pull の chart 名の末尾)・版の定数名・ハッシュの定数名・helm リポジトリ上の名前。
readonly CHARTS="kube-prometheus-stack loki alloy"
version_const() {
  case "$1" in
    kube-prometheus-stack) echo KUBE_PROMETHEUS_STACK_VERSION ;;
    loki) echo LOKI_VERSION ;;
    alloy) echo ALLOY_VERSION ;;
  esac
}
sha_const() {
  case "$1" in
    kube-prometheus-stack) echo KUBE_PROMETHEUS_STACK_SHA256 ;;
    loki) echo LOKI_SHA256 ;;
    alloy) echo ALLOY_SHA256 ;;
  esac
}
repo_chart() {
  case "$1" in
    kube-prometheus-stack) echo prometheus-community/kube-prometheus-stack ;;
    loki) echo grafana/loki ;;
    alloy) echo grafana/alloy ;;
  esac
}
readonly ALL_CONSTS="KUBE_PROMETHEUS_STACK_VERSION KUBE_PROMETHEUS_STACK_SHA256 LOKI_VERSION LOKI_SHA256 ALLOY_VERSION ALLOY_SHA256"
readonly SHA_CONSTS="KUBE_PROMETHEUS_STACK_SHA256 LOKI_SHA256 ALLOY_SHA256"
readonly VERSION_CONSTS="KUBE_PROMETHEUS_STACK_VERSION LOKI_VERSION ALLOY_VERSION"

# ServiceMonitor の対象6サービスと、その既存 Service 定義の置き場所。
readonly SERVICES="balance speed judge gateway pokedex calc"
service_yaml() {
  case "$1" in
    gateway | pokedex | calc) echo "deploy/k8s/base/$1/service.yaml" ;;
    balance | speed | judge) echo "services/$1/deploy/k8s/base/service.yaml" ;;
  esac
}

REAL_MKTEMP=$(command -v mktemp)
readonly REAL_MKTEMP
REAL_HELM=$(command -v helm || true)
readonly REAL_HELM
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

# ---------------------------------------------------------------------------
# YAML を構造として読む(本物の helm でローカルの最小 chart を描画する。ネットワークには行かない)
# ---------------------------------------------------------------------------

QCOUNT=0
# yaml_query YAMLファイル テンプレート本体 — 本体の中では $v が YAML 全体(map)。
# 結果として出したい行は `# R ` で始める(helm の描画結果は YAML でなければならないのでコメント行にする)。
# 標準出力には `# R ` を外した行だけを返す。YAML が読めない・helm が無いときは非0。
yaml_query() {
  local file=$1 body=$2 chart
  if [ -z "$REAL_HELM" ]; then
    echo "helm が PATH に無い(doctor.sh の前提ツール)" >&2
    return 1
  fi
  QCOUNT=$((QCOUNT + 1))
  chart="$WORK/yq/$QCOUNT"
  mkdir -p "$chart/templates" "$WORK/yq-home"
  printf 'apiVersion: v2\nname: q\nversion: 0.0.0\n' >"$chart/Chart.yaml"
  printf '{{- $v := toYaml .Values | fromYaml -}}\n%s\n' "$body" >"$chart/templates/q.yaml"
  HELM_CONFIG_HOME="$WORK/yq-home" HELM_CACHE_HOME="$WORK/yq-home" HELM_DATA_HOME="$WORK/yq-home" \
    "$REAL_HELM" template q "$chart" -f "$file" 2>"$chart/err" >"$chart/out" || {
    cat "$chart/err" >&2
    return 1
  }
  sed -n 's/^# R //p' "$chart/out"
}

# yaml_get YAMLファイル キー... — dig でたどった値を JSON で返す(無ければ null)。
yaml_get() {
  local file=$1 keys="" k
  shift
  for k in "$@"; do keys="$keys \"$k\""; done
  yaml_query "$file" "# R {{ dig$keys nil \$v | toJson }}"
}

# ---------------------------------------------------------------------------
# 偽の helm・kubectl・mktemp
# ---------------------------------------------------------------------------

# 架空の chart(.tgz の中身は実物ではない。chart ごとに別の内容)。
CHART_FIXTURES="$WORK/charts"
mkdir -p "$CHART_FIXTURES"
for c in $CHARTS; do
  printf 'fake chart archive for %s (test fixture, not the upstream chart)\n' "$c" >"$CHART_FIXTURES/$c.tgz"
done
fixture_sha() { sha256_of "$CHART_FIXTURES/$1.tgz"; }

install_fakes() {
  local bin="$1/bin"
  mkdir -p "$bin"
  cat >"$bin/helm" <<'EOF'
#!/bin/sh
# 偽の helm: 呼び出しを記録する。pull は FAKE_CHART_DIR の架空 .tgz を <dest>/<name>-<version>.tgz に置く。
# upgrade は引数中の .tgz の SHA-256(その時点で存在すれば)を記録するだけ。
printf '%s\n' "$*" >>"$FAKE_LOG_DIR/helm.log"
printf 'helm %s\n' "$*" >>"$FAKE_LOG_DIR/seq.log"
sha() { if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | awk '{print $1}'; else shasum -a 256 "$1" | awk '{print $1}'; fi; }
cmd=$1
shift
case "$cmd" in
  pull)
    chart=""; ver=""; dest="."
    while [ $# -gt 0 ]; do
      case "$1" in
        --version) ver=$2; shift 2; continue ;;
        --version=*) ver=${1#--version=} ;;
        -d | --destination) dest=$2; shift 2; continue ;;
        --destination=*) dest=${1#--destination=} ;;
        -*) ;;
        *) [ -z "$chart" ] && chart=$1 ;;
      esac
      shift
    done
    name=${chart##*/}
    src="$FAKE_CHART_DIR/$name.tgz"
    if [ ! -f "$src" ]; then echo "Error: chart \"$chart\" not found" >&2; exit 1; fi
    printf '%s %s %s\n' "$chart" "$ver" "$dest" >>"$FAKE_LOG_DIR/pulls.log"
    cp "$src" "$dest/$name-$ver.tgz" ;;
  upgrade | install)
    tgz_sha=NONE
    for a in "$@"; do
      case "$a" in
        *.tgz) if [ -f "$a" ]; then tgz_sha=$(sha "$a"); else tgz_sha=MISSING; fi ;;
      esac
    done
    printf '%s\t%s %s\n' "$tgz_sha" "$cmd" "$*" >>"$FAKE_LOG_DIR/upgrades.log" ;;
esac
exit 0
EOF
  cat >"$bin/kubectl" <<'EOF'
#!/bin/sh
# 偽の kubectl: 呼び出しを記録するだけ。namespace の有無は FAKE_NS_EXISTS で決める。
printf '%s\n' "$*" >>"$FAKE_LOG_DIR/kubectl.log"
printf 'kubectl %s\n' "$*" >>"$FAKE_LOG_DIR/seq.log"
case " $* " in
  *" get "*observability*)
    case " $* " in
      *" namespace "* | *" ns "* | *" namespaces "* | *namespace/observability*)
        [ "${FAKE_NS_EXISTS:-0}" = 1 ] && exit 0
        echo 'Error from server (NotFound): namespaces "observability" not found' >&2
        exit 1 ;;
    esac
    exit 0 ;;
  *" create "*" namespace "*observability* | *" create "*" ns "*observability*)
    if [ "${FAKE_NS_EXISTS:-0}" = 1 ]; then
      echo 'Error from server (AlreadyExists): namespaces "observability" already exists' >&2
      exit 1
    fi
    exit 0 ;;
esac
exit 0
EOF
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
  chmod +x "$bin/helm" "$bin/kubectl" "$bin/mktemp"
}

# new_repo 名前 — 使い捨て git リポジトリに対象スクリプトと values・ServiceMonitor をコピーし、スクリプトのパスを返す。
new_repo() {
  local dir="$WORK/$1/repo"
  mkdir -p "$dir/scripts" "$dir/deploy/k8s/base"
  git -C "$dir" init -q
  cp "$SCRIPT" "$dir/$SCRIPT_REL"
  chmod +x "$dir/$SCRIPT_REL"
  if [ -d "$ROOT/$OBS_REL" ]; then cp -R "$ROOT/$OBS_REL" "$dir/deploy/k8s/base/"; fi
  printf '%s' "$dir/$SCRIPT_REL"
}

# fixture_repo 名前 — 3つの期待ハッシュを架空 .tgz に合わせたコピー(版は本物のまま)。
fixture_repo() {
  local script c
  script=$(new_repo "$1") || return 1
  for c in $CHARTS; do
    set_const "$script" "$(sha_const "$c")" "$(fixture_sha "$c")" || return 1
  done
  printf '%s' "$script"
}

RC=0
CASE_DIR=""
OUT=""

# run_script ケース名 スクリプト 名前空間あり(0/1) [VAR=値...] — 偽物の下でスクリプトを流す。
run_script() {
  local name=$1 script=$2 ns_exists=$3 f
  shift 3
  CASE_DIR="$WORK/$name/case"
  mkdir -p "$CASE_DIR/logs" "$CASE_DIR/tmp"
  for f in mktemp.log helm.log kubectl.log seq.log pulls.log upgrades.log; do : >"$CASE_DIR/logs/$f"; done
  install_fakes "$CASE_DIR"
  (
    cd "$(dirname "$script")" &&
      env "$@" \
        PATH="$CASE_DIR/bin:$PATH" \
        TMPDIR="$CASE_DIR/tmp" \
        REAL_MKTEMP="$REAL_MKTEMP" \
        FAKE_TMP_ROOT="$CASE_DIR/tmp" \
        FAKE_LOG_DIR="$CASE_DIR/logs" \
        FAKE_CHART_DIR="$CHART_FIXTURES" \
        FAKE_NS_EXISTS="$ns_exists" \
        KUBECONFIG="$CASE_DIR/no-such-kubeconfig" \
        HELM_CONFIG_HOME="$CASE_DIR/helm-home" HELM_CACHE_HOME="$CASE_DIR/helm-home" HELM_DATA_HOME="$CASE_DIR/helm-home" \
        HTTP_PROXY="http://127.0.0.1:9" http_proxy="http://127.0.0.1:9" \
        HTTPS_PROXY="http://127.0.0.1:9" https_proxy="http://127.0.0.1:9" \
        ALL_PROXY="http://127.0.0.1:9" all_proxy="http://127.0.0.1:9" \
        NO_PROXY="" no_proxy="" \
        "$script" </dev/null >"$CASE_DIR/out" 2>&1
  )
  RC=$?
  OUT=$(cat "$CASE_DIR/out")
}

upgrade_called() { [ -s "$CASE_DIR/logs/upgrades.log" ]; }
pull_called() { [ -s "$CASE_DIR/logs/pulls.log" ]; }
kubectl_called_with() { grep -Eq "$1" "$CASE_DIR/logs/kubectl.log"; }
kubectl_apply_called() { kubectl_called_with '(^|[[:space:]])apply([[:space:]]|$)'; }
tmp_is_clean() { [ -z "$(ls -A "$CASE_DIR/tmp")" ]; }
mktemp_dir_called() { [ -s "$CASE_DIR/logs/mktemp.log" ]; }
# seq_line 正規表現 — seq.log(helm・kubectl の呼び出し順)で最初に一致する行番号(無ければ空)。
seq_line() { grep -nE "$1" "$CASE_DIR/logs/seq.log" | head -n 1 | cut -d: -f1; }

# expect_rejected — 直近の実行が「helm upgrade --install の前に非0で止まった」こと。一時ディレクトリも消えていること。
expect_rejected() {
  if [ "$RC" -eq 0 ]; then ng "終了コードが 0(非0で止まるべき)。出力: $OUT"; else ok; fi
  if upgrade_called; then
    ng "helm upgrade/install が呼ばれた(検証に失敗したら1つも入れずに止まるべき): $(cut -f2 "$CASE_DIR/logs/upgrades.log" | tr '\n' ';')"
  else ok; fi
  if kubectl_apply_called; then ng "kubectl apply が呼ばれた(検証に失敗したら何も適用しないべき)"; else ok; fi
  if tmp_is_clean; then ok; else ng "失敗時に一時ディレクトリが残った(trap で消すべき): $(ls -A "$CASE_DIR/tmp")"; fi
}

# expect_pulled — 取得(偽の helm pull)まで到達したこと。スタブのように何もせず止まるだけの実装を合格にしないため。
expect_pulled() {
  if pull_called; then ok; else ng "helm pull が呼ばれていない(取得してから検証して止まるべき)。出力: $OUT"; fi
}

# ---------------------------------------------------------------------------
# 1. スクリプトの形(静的)
# ---------------------------------------------------------------------------

test_static_shape() {
  begin "静的: 実行可能・bash・構文・最初のコマンドがルートへの cd・mktemp -d と trap"
  if [ ! -f "$SCRIPT" ]; then
    ng "$SCRIPT_REL が無い"
    return
  fi
  if [ -x "$SCRIPT" ]; then ok; else ng "$SCRIPT_REL に実行ビットが無い"; fi
  if [ "$(head -n 1 "$SCRIPT")" = "#!/usr/bin/env bash" ]; then ok; else ng "1行目が #!/usr/bin/env bash でない"; fi
  if bash -n "$SCRIPT" 2>/dev/null; then ok; else ng "bash -n で構文エラー"; fi
  # コメント・空行・`set -...` を除いた最初の行がルートへの cd であること(ADR-0406 §4)。
  local first
  first=$(grep -vE '^[[:space:]]*(#|$)|^[[:space:]]*set[[:space:]]+-' "$SCRIPT" | head -n 1)
  # shellcheck disable=SC2016 # 文字列そのものを比べる
  if [ "$first" = 'cd "$(git rev-parse --show-toplevel)"' ]; then ok; else
    ng "最初のコマンドが cd \"\$(git rev-parse --show-toplevel)\" でない: '$first'"
  fi
  if grep -Eq '(^|[^[:alnum:]_])mktemp[[:space:]]+-d([[:space:]]|\)|$)' "$SCRIPT"; then ok; else ng "一時ディレクトリを mktemp -d で作っていない"; fi
  if grep -Eq "^[[:space:]]*trap[[:space:]]+['\"]rm -rf " "$SCRIPT"; then ok; else ng "一時ディレクトリを trap 'rm -rf ...' で消していない"; fi
}

test_static_constants() {
  begin "静的: 3つの版・3つのハッシュがこのファイルに1回ずつ・正しい形で定義され、ADR-0406 §4 と一致する"
  local name value c ver sha adr="$ROOT/$ADR_REL"
  for name in $ALL_CONSTS; do
    if [ "$(count_const "$SCRIPT" "$name")" = 1 ]; then ok; else ng "$name の定義行がちょうど1行ない"; fi
  done
  for name in $VERSION_CONSTS; do
    value=$(get_const "$SCRIPT" "$name")
    # 範囲指定(^ ~ x *)や v 接頭辞を許さない。厳密な X.Y.Z だけ。
    if printf '%s' "$value" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$'; then ok; else ng "$name が厳密な版(X.Y.Z)でない: '$value'"; fi
  done
  for name in $SHA_CONSTS; do
    value=$(get_const "$SCRIPT" "$name")
    if printf '%s' "$value" | grep -Eq '^[0-9a-f]{64}$'; then ok; else ng "$name が64桁の16進でない: '$value'"; fi
  done
  # 値そのものがファイル内に1回だけ現れる(コメントや2つ目の変数に写していない)。
  for name in $ALL_CONSTS; do
    value=$(get_const "$SCRIPT" "$name")
    [ -n "$value" ] || continue
    local n
    n=$(grep -oF "$value" "$SCRIPT" | wc -l | tr -d ' ')
    if [ "$n" = 1 ]; then ok; else ng "$name の値 '$value' が $SCRIPT_REL に $n 回現れる(定数の定義1回だけにする)"; fi
  done
  # ADR-0406 §4 の「<chart> バージョン `<版>` … SHA-256: `<hash>`」と組で一致する(テスト側に値を写さず ADR と照合する)。
  if [ ! -f "$adr" ]; then
    ng "$ADR_REL が無い"
    return
  fi
  for c in $CHARTS; do
    ver=$(get_const "$SCRIPT" "$(version_const "$c")")
    sha=$(get_const "$SCRIPT" "$(sha_const "$c")")
    local block
    block=$(grep -F -A2 "\`$(repo_chart "$c")\` バージョン \`$ver\`" "$adr" || true)
    if [ -n "$block" ]; then ok; else ng "$(repo_chart "$c") の版 '$ver' が $ADR_REL §4 の記載と一致しない"; fi
    if [ -n "$sha" ] && printf '%s\n' "$block" | grep -Fq "\`$sha\`"; then ok; else
      ng "$(sha_const "$c") が $ADR_REL §4 の $(repo_chart "$c") $ver の SHA-256 と一致しない"
    fi
  done
}

test_constants_not_duplicated() {
  begin "静的: ハッシュを runbook・サービス・deploy・Makefile に二重に持たない"
  local name value hits
  for name in $SHA_CONSTS; do
    value=$(get_const "$SCRIPT" "$name")
    [ -n "$value" ] || continue
    hits=$(grep -rlF "$value" "$ROOT/docs/runbooks" "$ROOT/services" "$ROOT/Makefile" "$ROOT/deploy" "$ROOT/scripts" 2>/dev/null |
      grep -vF "$SCRIPT" || true)
    if [ -z "$hits" ]; then ok; else ng "$name の値が $SCRIPT_REL 以外にもある: $(printf '%s' "$hits" | sed "s#$ROOT/##g" | tr '\n' ' ')"; fi
  done
}

test_static_path_lookup() {
  begin "静的: helm・kubectl を絶対パスで呼ばない(PATH 経由。テストの偽物が効くように)"
  if grep -vE '^[[:space:]]*#' "$SCRIPT" | grep -Eq '(^|[[:space:]"'"'"'=(;|&])/[^[:space:]"'"'"']*/(helm|kubectl)([[:space:]"'"'"';|&)]|$)'; then
    ng "helm または kubectl を絶対パスで呼んでいる"
  else ok; fi
}

# ---------------------------------------------------------------------------
# 2. 振る舞い(偽の helm・kubectl の下で流す)
# ---------------------------------------------------------------------------

test_happy_path() {
  begin "正常系: 3チャートともハッシュ一致 → 3つとも検証済み .tgz から helm upgrade --install --version <固定版> まで到達する"
  local script c ver sha line
  script=$(fixture_repo happy) || return
  run_script happy "$script" 0

  if [ "$RC" -eq 0 ]; then ok; else ng "終了コード $RC(0 であるべき)。出力: $OUT"; fi

  # 公式リポジトリを追加している(ADR-0406 §4)。
  if grep -Eq '^repo add prometheus-community https://prometheus-community\.github\.io/helm-charts/?([[:space:]]|$)' "$CASE_DIR/logs/helm.log"; then ok; else
    ng "helm repo add prometheus-community https://prometheus-community.github.io/helm-charts が呼ばれていない"
  fi
  if grep -Eq '^repo add grafana https://grafana\.github\.io/helm-charts/?([[:space:]]|$)' "$CASE_DIR/logs/helm.log"; then ok; else
    ng "helm repo add grafana https://grafana.github.io/helm-charts が呼ばれていない"
  fi

  for c in $CHARTS; do
    ver=$(get_const "$script" "$(version_const "$c")")
    sha=$(fixture_sha "$c")
    # pull: 固定版を明示し、mktemp -d で作った一時ディレクトリへ取得している。
    if grep -Eq "^$(repo_chart "$c") $ver $CASE_DIR/tmp/mktemp\." "$CASE_DIR/logs/pulls.log"; then ok; else
      ng "$(repo_chart "$c") を --version $ver で mktemp -d の一時ディレクトリへ helm pull していない。pulls: $(tr '\n' ';' <"$CASE_DIR/logs/pulls.log")"
    fi
    # upgrade: 検証済みのローカル .tgz(中身のハッシュが一致するもの)を chart として渡している。
    line=$(grep -F "$sha	" "$CASE_DIR/logs/upgrades.log" | head -n 1 | cut -f2)
    if [ -z "$line" ]; then
      ng "$c: 取得した .tgz を chart に渡す helm upgrade が無い(repo 経由の再解決ではなく検証済みのローカル chart を使う)。upgrades: $(cut -f2 "$CASE_DIR/logs/upgrades.log" | tr '\n' ';')"
      continue
    fi
    if printf '%s\n' "$line" | grep -Eq '^upgrade .*(--install|(^|[[:space:]])-i)([[:space:]]|$)'; then ok; else ng "$c: helm upgrade --install でない: $line"; fi
    if printf '%s\n' "$line" | grep -Eq "(^|[[:space:]])--version(=|[[:space:]]+)$ver([[:space:]]|$)"; then ok; else ng "$c: helm upgrade に --version $ver が明示されていない: $line"; fi
    if printf '%s\n' "$line" | grep -Eq "((^|[[:space:]])-n[[:space:]]+|--namespace(=|[[:space:]]+))$NAMESPACE([[:space:]]|$)"; then ok; else ng "$c: namespace $NAMESPACE を指定していない: $line"; fi
    if printf '%s\n' "$line" | grep -Eq "((^|[[:space:]])-f[[:space:]]+|--values(=|[[:space:]]+))([^[:space:]]*/)?$VALUES_REL/$c\.yaml([[:space:]]|$)"; then ok; else
      ng "$c: values に $VALUES_REL/$c.yaml を渡していない: $line"
    fi
  done
  if [ "$(wc -l <"$CASE_DIR/logs/upgrades.log" | tr -d ' ')" = 3 ]; then ok; else ng "helm upgrade がちょうど3回でない: $(cut -f2 "$CASE_DIR/logs/upgrades.log" | tr '\n' ';')"; fi

  # 取得→検証→適用: 最後の pull が最初の upgrade より前(3つとも検証してから入れ始める)。
  local last_pull first_upgrade
  last_pull=$(grep -nE '^helm pull ' "$CASE_DIR/logs/seq.log" | tail -n 1 | cut -d: -f1)
  first_upgrade=$(seq_line '^helm (upgrade|install) ')
  if [ -n "$last_pull" ] && [ -n "$first_upgrade" ] && [ "$last_pull" -lt "$first_upgrade" ]; then ok; else
    ng "3つとも取得・検証する前に helm upgrade を始めている(取得→検証→適用の順。ADR-0406 §4)"
  fi

  # ServiceMonitor(kube-prometheus-stack の CRD が要る)は kube-prometheus-stack を入れた後に kustomize で適用する。
  local kps_line sm_line
  kps_line=$(grep -n "^helm upgrade .*kube-prometheus-stack-" "$CASE_DIR/logs/seq.log" | head -n 1 | cut -d: -f1)
  sm_line=$(seq_line "^kubectl .*apply .*-k[[:space:]=]+([^[:space:]]*/)?$OBS_REL/?([[:space:]]|$)")
  if [ -n "$sm_line" ]; then ok; else ng "kubectl apply -k $OBS_REL(ServiceMonitor)が呼ばれていない"; fi
  if [ -n "$kps_line" ] && [ -n "$sm_line" ] && [ "$kps_line" -lt "$sm_line" ]; then ok; else
    ng "ServiceMonitor の適用が kube-prometheus-stack(CRD)の導入より後になっていない"
  fi

  if mktemp_dir_called; then ok; else ng "取得用の一時ディレクトリを mktemp -d で作っていない"; fi
  if tmp_is_clean; then ok; else ng "成功時に一時ディレクトリが残った(trap で消すべき): $(ls -A "$CASE_DIR/tmp")"; fi
}

test_namespace_idempotent() {
  begin "正常系: observability namespace が無ければ(helm upgrade より前に)作り、あれば作らない(冪等)"
  local script create_line first_upgrade
  script=$(fixture_repo ns-missing) || return
  run_script ns-missing "$script" 0
  if kubectl_called_with "create (namespace|ns) $NAMESPACE"; then ok; else ng "namespace が無いのに create namespace $NAMESPACE を呼んでいない"; fi
  if [ "$RC" -eq 0 ]; then ok; else ng "namespace が無いときに失敗した: $OUT"; fi
  create_line=$(seq_line "^kubectl create (namespace|ns) $NAMESPACE")
  first_upgrade=$(seq_line '^helm (upgrade|install) ')
  if [ -n "$create_line" ] && [ -n "$first_upgrade" ] && [ "$create_line" -lt "$first_upgrade" ]; then ok; else
    ng "namespace の作成が helm upgrade より前でない"
  fi

  script=$(fixture_repo ns-exists) || return
  run_script ns-exists "$script" 1
  if kubectl_called_with "create (namespace|ns) $NAMESPACE"; then ng "namespace があるのに create namespace $NAMESPACE を呼んだ(再実行で壊れる)"; else ok; fi
  if [ "$RC" -eq 0 ]; then ok; else ng "namespace があるときに失敗した(冪等であるべき): $OUT"; fi
  if upgrade_called; then ok; else ng "namespace があるときに helm upgrade まで到達しない"; fi
}

test_hash_mismatch_real_constants() {
  begin "ハッシュ不一致: 本物の期待ハッシュのまま架空の .tgz を返す → helm upgrade の前に止まる"
  local script
  script=$(new_repo hash-real) || return
  run_script hash-real "$script" 0
  expect_pulled
  expect_rejected
}

test_hash_mismatch_each_chart() {
  local c other script
  for c in $CHARTS; do
    begin "ハッシュ不一致: $c だけ期待ハッシュが合わない → どれも helm upgrade せずに止まる"
    script=$(new_repo "mismatch-$c") || continue
    for other in $CHARTS; do
      [ "$other" = "$c" ] && continue
      set_const "$script" "$(sha_const "$other")" "$(fixture_sha "$other")" || continue 2
    done
    # 先頭1文字だけ違う期待値(取得物が1バイトでも変わればハッシュはこのようにずれる)。
    local s first
    s=$(fixture_sha "$c")
    first=${s:0:1}
    if [ "$first" = 0 ]; then first=1; else first=0; fi
    set_const "$script" "$(sha_const "$c")" "$first${s:1}" || continue
    run_script "mismatch-$c" "$script" 0
    expect_pulled
    expect_rejected
  done
}

test_hash_not_overridable_by_env() {
  begin "ハッシュ不一致: 環境変数で期待ハッシュ・版を上書きしても検証を迂回できない"
  local script c envs=()
  script=$(new_repo hash-env) || return
  for c in $CHARTS; do envs+=("$(sha_const "$c")=$(fixture_sha "$c")"); done
  envs+=("LOKI_VERSION=0.0.1")
  run_script hash-env "$script" 0 "${envs[@]}"
  expect_rejected
  if grep -Eq '^grafana/loki 0\.0\.1 ' "$CASE_DIR/logs/pulls.log"; then ng "環境変数 LOKI_VERSION で取得する版を差し替えられた"; else ok; fi
}

test_empty_constants() {
  local name script
  for name in $ALL_CONSTS; do
    begin "定数が空: $name=\"\" → helm upgrade の前に止まる"
    script=$(fixture_repo "empty-$name") || continue
    set_const "$script" "$name" "" || continue
    run_script "empty-$name" "$script" 0
    expect_rejected
  done
}

test_pull_failure() {
  begin "取得失敗: helm pull が失敗する → helm upgrade の前に止まる"
  local script
  script=$(fixture_repo pull-fail) || return
  # 架空 chart の置き場を空にして、偽の helm pull を失敗させる。
  local saved=$CHART_FIXTURES
  CHART_FIXTURES="$WORK/pull-fail/no-charts"
  mkdir -p "$CHART_FIXTURES"
  run_script pull-fail "$script" 0
  CHART_FIXTURES=$saved
  expect_rejected
}

# ---------------------------------------------------------------------------
# 3. values(deploy/k8s/base/observability/values/)
# ---------------------------------------------------------------------------

# expect_json ラベル 実際 期待(JSON) — yaml_get の結果を比べる。
expect_json() {
  if [ "$2" = "$3" ]; then ok; else ng "$1 が $3 であるべきところ ${2:-(読めない)}"; fi
}

test_values_files() {
  begin "values: 3ファイルがあり YAML として読める"
  local c
  for c in $CHARTS; do
    if [ -f "$ROOT/$VALUES_REL/$c.yaml" ]; then ok; else ng "$VALUES_REL/$c.yaml が無い"; continue; fi
    if yaml_get "$ROOT/$VALUES_REL/$c.yaml" __no_such_key__ >/dev/null; then ok; else ng "$VALUES_REL/$c.yaml が YAML として読めない"; fi
  done
}

test_values_loki() {
  begin "values: Loki は SingleBinary・filesystem ストレージ(ADR-0406 §4)"
  local f="$ROOT/$VALUES_REL/loki.yaml"
  [ -f "$f" ] || { ng "$VALUES_REL/loki.yaml が無い"; return; }
  expect_json "loki.yaml の deploymentMode" "$(yaml_get "$f" deploymentMode)" '"SingleBinary"'
  expect_json "loki.yaml の loki.storage.type" "$(yaml_get "$f" loki storage type)" '"filesystem"'
}

test_values_loki_local_footprint() {
  begin "values: Loki は個人用 k3d で Pending にならないよう chunks/results キャッシュとキャナリアを無効化する"
  local f="$ROOT/$VALUES_REL/loki.yaml"
  [ -f "$f" ] || { ng "$VALUES_REL/loki.yaml が無い"; return; }
  # chart 既定は有効(chunks-cache は memory request 9830Mi、results-cache は 1229Mi の StatefulSet)。
  # k3d は servers:1/agents:0 が既定のため、既定のままだと Pending のまま残る。
  expect_json "loki.yaml の chunksCache.enabled" "$(yaml_get "$f" chunksCache enabled)" 'false'
  expect_json "loki.yaml の resultsCache.enabled" "$(yaml_get "$f" resultsCache enabled)" 'false'
  # lokiCanary はトップレベルのキー(chart 既定 enabled: true)。monitoring.lokiCanary という
  # キーは chart に存在しない(誤って書いても描画に反映されず DaemonSet が残ってしまう)。
  expect_json "loki.yaml の lokiCanary.enabled" "$(yaml_get "$f" lokiCanary enabled)" 'false'
}

test_values_grafana_admin() {
  begin "values: Grafana の管理者パスワードを values に書かず、ユーザーが登録する既存 Secret を参照する(ADR-0406 §4)"
  local f="$ROOT/$VALUES_REL/kube-prometheus-stack.yaml" v
  [ -f "$f" ] || { ng "$VALUES_REL/kube-prometheus-stack.yaml が無い"; return; }
  # adminPassword は未設定か空(未設定かつ existingSecret も無いと chart 既定の固定パスワードになるので、下の検査と組で見る)。
  v=$(yaml_get "$f" grafana adminPassword)
  if [ "$v" = null ] || [ "$v" = '""' ]; then ok; else ng "grafana.adminPassword に値が書かれている(values に秘密値を含めない)"; fi
  v=$(yaml_get "$f" grafana admin existingSecret)
  if printf '%s' "$v" | grep -Eq '^"[^"]+"$'; then ok; else ng "grafana.admin.existingSecret が空(ユーザーが手動で登録する Secret の名前を指すべき): ${v:-(読めない)}"; fi
}

test_values_no_ingress() {
  begin "values: Grafana・Prometheus・Alertmanager の Ingress を作らない(port-forward のみ。issue #148)"
  local f="$ROOT/$VALUES_REL/kube-prometheus-stack.yaml" comp v
  [ -f "$f" ] || { ng "$VALUES_REL/kube-prometheus-stack.yaml が無い"; return; }
  for comp in grafana prometheus alertmanager; do
    v=$(yaml_get "$f" "$comp" ingress enabled)
    if [ "$v" = true ]; then ng "$comp.ingress.enabled が true"; elif [ -z "$v" ]; then ng "$comp.ingress.enabled が読めない"; else ok; fi
  done
}

test_values_servicemonitor_discovery() {
  begin "values: Prometheus が observability 側の ServiceMonitor を拾う(release ラベル必須の既定を外すか、SM に合うセレクタ)"
  local f="$ROOT/$VALUES_REL/kube-prometheus-stack.yaml" nil sel s missing=""
  [ -f "$f" ] || { ng "$VALUES_REL/kube-prometheus-stack.yaml が無い"; return; }
  nil=$(yaml_get "$f" prometheus prometheusSpec serviceMonitorSelectorNilUsesHelmValues)
  sel=$(yaml_query "$f" '{{ range $k, $x := (dig "prometheus" "prometheusSpec" "serviceMonitorSelector" "matchLabels" dict $v) }}# R {{ $k }}={{ $x }}
{{ end }}')
  if [ -n "$sel" ]; then
    # 明示のセレクタがあるなら、6つの ServiceMonitor すべてがそのラベルを持つこと。
    for s in $SERVICES; do
      local labels
      labels=$(yaml_query "$ROOT/$SM_REL/$s.yaml" '{{ range $k, $x := (dig "metadata" "labels" dict $v) }}# R {{ $k }}={{ $x }}
{{ end }}' 2>/dev/null || true)
      while IFS= read -r kv; do
        [ -n "$kv" ] || continue
        printf '%s\n' "$labels" | grep -qxF "$kv" || missing="$missing $s($kv)"
      done <<<"$sel"
    done
    if [ -z "$missing" ]; then ok; else ng "serviceMonitorSelector のラベルを持たない ServiceMonitor がある:$missing"; fi
  elif [ "$nil" = false ]; then
    ok
  else
    ng "prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues が false でなく、serviceMonitorSelector も無い(既定では release ラベルの無い ServiceMonitor を拾わない)"
  fi
}

test_values_alloy() {
  begin "values: Alloy は DaemonSet でコンテナログを Loki へ送るだけ(メトリクス収集・OTel は有効化しない。ADR-0406 §4)"
  local f="$ROOT/$VALUES_REL/alloy.yaml" v content
  [ -f "$f" ] || { ng "$VALUES_REL/alloy.yaml が無い"; return; }
  v=$(yaml_get "$f" controller type)
  if [ "$v" = '"daemonset"' ] || [ "$v" = null ]; then ok; else ng "alloy.yaml の controller.type が daemonset でない: ${v:-(読めない)}"; fi
  content=$(yaml_query "$f" '{{ range (splitList "\n" (dig "alloy" "configMap" "content" "" $v)) }}# R {{ . }}
{{ end }}' || true)
  if printf '%s\n' "$content" | grep -Eq '(^|[[:space:]])loki\.write[[:space:]]+"'; then ok; else ng "alloy.configMap.content に loki.write が無い(Loki へ送る設定)"; fi
  if printf '%s\n' "$content" | grep -Eq '(^|[[:space:]])loki\.source\.[a-z_]+[[:space:]]+"'; then ok; else ng "alloy.configMap.content にログの取り込み(loki.source.*)が無い"; fi
  if printf '%s\n' "$content" | grep -Eq '(^|[[:space:]])(prometheus\.(scrape|remote_write)|otelcol\.)'; then
    ng "alloy.configMap.content にメトリクス収集・OTel の設定がある(ログ収集のみにする)"
  else ok; fi
}

# ---------------------------------------------------------------------------
# 4. ServiceMonitor(deploy/k8s/base/observability/servicemonitors/)と kustomize
# ---------------------------------------------------------------------------

test_servicemonitors() {
  local s f svc want_name want_port ns
  ns=$(yaml_get "$ROOT/deploy/k8s/base/namespace.yaml" metadata name 2>/dev/null | tr -d '"')
  for s in $SERVICES; do
    begin "ServiceMonitor: $s が既存 Service($(service_yaml "$s"))の app.kubernetes.io/name と http ポートの /metrics を指す"
    f="$ROOT/$SM_REL/$s.yaml"
    svc="$ROOT/$(service_yaml "$s")"
    if [ ! -f "$svc" ]; then
      ng "既存の Service 定義 $(service_yaml "$s") が無い"
      continue
    fi
    # 期待値はテストに書き写さず、既存の Service 定義から読む。
    want_name=$(yaml_get "$svc" metadata labels app.kubernetes.io/name)
    want_port=$(yaml_query "$svc" '{{ range (dig "spec" "ports" list $v) }}{{ if eq (toString .name) "http" }}# R http{{ end }}
{{ end }}')
    if [ "$want_port" = http ]; then ok; else ng "$(service_yaml "$s") に http という名前のポートが無い"; fi
    if [ ! -f "$f" ]; then
      ng "$SM_REL/$s.yaml が無い"
      continue
    fi
    expect_json "$s.yaml の apiVersion" "$(yaml_get "$f" apiVersion)" '"monitoring.coreos.com/v1"'
    expect_json "$s.yaml の kind" "$(yaml_get "$f" kind)" '"ServiceMonitor"'
    expect_json "$s.yaml の spec.selector.matchLabels[app.kubernetes.io/name]" \
      "$(yaml_get "$f" spec selector matchLabels app.kubernetes.io/name)" "$want_name"
    # Service は observability とは別の namespace(overlay で pokecalc)にあるので namespaceSelector で指す。
    if [ -n "$ns" ] && yaml_query "$f" '{{ range (dig "spec" "namespaceSelector" "matchNames" list $v) }}# R {{ . }}
{{ end }}' | grep -qxF "$ns"; then ok; else ng "$s.yaml の spec.namespaceSelector.matchNames に $ns が無い"; fi
    local eps
    eps=$(yaml_query "$f" '{{ range (dig "spec" "endpoints" list $v) }}# R {{ .port }}|{{ .path }}|targetPort={{ hasKey . "targetPort" }}
{{ end }}' || true)
    if [ -n "$eps" ]; then ok; else ng "$s.yaml に spec.endpoints が無い"; continue; fi
    # targetPort(番号・別ポート)ではなく、既存 Service の名前付きポート http を指す。
    if [ "$eps" = "http|/metrics|targetPort=false" ]; then ok; else
      ng "$s.yaml の endpoints が「port: http・path: /metrics」の1つだけでない(targetPort・別ポートを使わない。ADR-0406 §4): $(printf '%s' "$eps" | tr '\n' ';')"
    fi
  done
}

test_kustomize_render() {
  begin "kustomize: $OBS_REL が描画でき、6つの ServiceMonitor を含み、Ingress・Secret を含まない"
  if ! command -v kubectl >/dev/null 2>&1; then
    ng "kubectl が PATH に無い"
    return
  fi
  local out s
  if out=$(kubectl kustomize "$ROOT/$OBS_REL" 2>&1); then ok; else ng "kubectl kustomize $OBS_REL が失敗: $out"; return; fi
  if [ "$(printf '%s\n' "$out" | grep -cE '^kind: ServiceMonitor$')" = 6 ]; then ok; else
    ng "描画結果の ServiceMonitor が6つでない: $(printf '%s\n' "$out" | grep -cE '^kind: ServiceMonitor$')"
  fi
  printf '%s\n' "$out" >"$WORK/rendered.yaml"
  for s in $SERVICES; do
    # 各サービスの ServiceMonitor が描画結果に含まれる(名前はサービス名を含む)。
    if awk '/^kind: ServiceMonitor$/{sm=1} /^---/{sm=0} sm && /^  name: /{print $2}' "$WORK/rendered.yaml" | grep -q "$s"; then ok; else
      ng "描画結果に $s の ServiceMonitor が無い"
    fi
  done
  if printf '%s\n' "$out" | grep -qE '^kind: Ingress$'; then ng "描画結果に Ingress がある(issue #148)"; else ok; fi
  if printf '%s\n' "$out" | grep -qE '^kind: Secret$'; then ng "描画結果に Secret がある(秘密値をコミットしない)"; else ok; fi
}

test_cloud_overlay_untouched() {
  begin "kustomize: cloud overlay に ServiceMonitor が混ざらない(ADR-0406 §5)"
  command -v kubectl >/dev/null 2>&1 || { ng "kubectl が PATH に無い"; return; }
  local out
  if out=$(kubectl kustomize "$ROOT/deploy/k8s/overlays/cloud" 2>&1); then ok; else ng "cloud overlay が描画できない: $out"; return; fi
  if printf '%s\n' "$out" | grep -qE '^kind: ServiceMonitor$'; then ng "cloud overlay の描画結果に ServiceMonitor がある"; else ok; fi
}

# ---------------------------------------------------------------------------

test_static_shape
test_static_constants
test_constants_not_duplicated
test_static_path_lookup
test_happy_path
test_namespace_idempotent
test_hash_mismatch_real_constants
test_hash_mismatch_each_chart
test_hash_not_overridable_by_env
test_empty_constants
test_pull_failure
test_values_files
test_values_loki
test_values_loki_local_footprint
test_values_grafana_admin
test_values_no_ingress
test_values_servicemonitor_discovery
test_values_alloy
test_servicemonitors
test_kustomize_render
test_cloud_overlay_untouched

if [ "$FAILURES" -gt 0 ]; then
  printf 'observability-bootstrap test: %d failed, %d passed\n' "$FAILURES" "$PASSES" >&2
  exit 1
fi
printf 'observability-bootstrap test: all %d checks passed\n' "$PASSES"
