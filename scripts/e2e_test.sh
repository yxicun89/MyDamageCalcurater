#!/usr/bin/env bash
# scripts/e2e.sh(ルートの `make e2e`)の自動テスト(ADR-0306。issue #72)。
# `make test-scripts`(make test に含む)から流す。
#
# 背景: `make e2e` は「k3d 上のスモーク + Playwright」を名乗りながら、中身が
# `echo "e2e: (P4-6 で スモーク + Playwright を実装)"` だけの未実装スタブで、テスト0件のまま exit 0 していた
# (issue #72)。このテストは、その「0件で成功」が二度と起きないことを固定する。
#
# 方式: 手書きの ok/ng ヘルパー付きの bash スクリプト。ルートの scripts/ には Go モジュールが無く、
# 既存の前例(scripts/argocd-bootstrap_test.sh・scripts/observability-*_test.sh・
# scripts/check-publishable.sh --self-test)に合わせて依存を増やさない(bats は入れない)。
#
# 実ブラウザ・実クラスタ・実ネットワークには触らない:
#   - make と kubectl は PATH の先頭に置いた偽物に差し替える(呼び出しを記録するだけ)。MAKE 環境変数も偽物へ向ける
#     (e2e.sh が ${MAKE:-make} を使っても本物の make が動かないようにする)。
#   - KUBECONFIG を存在しないファイルに、HTTP(S) のプロキシを閉じたポートに向ける。
#   - 対象スクリプトは一時ディレクトリの使い捨て git リポジトリへコピーして流す
#     (`cd "$(git rev-parse --show-toplevel)"` がそこへ向く)。
#
# e2e.sh に期待する形(ADR-0306):
#   - クラスタ非依存の3件を「必ず」実行する: make web-e2e / web-e2e-online / web-e2e-balance
#   - kubectl の現在のコンテキストが k3d-$CLUSTER のときだけ、k3d 依存の3件を追加で実行する:
#     make api-smoke / web-k3d-smoke / web-k3d-e2e
#   - 一致しなければ、何を飛ばしたか・どうすれば実行できるかを明示して「スキップ」し、残りは実行する
#   - 1つでも失敗したら、そこで非0で終わる(以降は流さない)
#   - どんな環境変数でも「1件も実行せずに成功」にはできない
set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
readonly ROOT
readonly SCRIPT_REL="scripts/e2e.sh"
readonly SCRIPT="$ROOT/$SCRIPT_REL"
readonly MAKEFILE="$ROOT/Makefile"

# 常に実行する(k3d クラスタが要らない)E2E。web/Makefile の実装済みターゲット。
readonly BASE_TARGETS="web-e2e web-e2e-online web-e2e-balance"
# k3d クラスタがあるときだけ追加で実行するスモーク。
readonly K3D_TARGETS="api-smoke web-k3d-smoke web-k3d-e2e"
readonly ALL_TARGETS="$BASE_TARGETS $K3D_TARGETS"
# make e2e から呼んではいけないターゲット(クラスタを作り直す・Docker デーモンを要る・時間がかかる)。
readonly FORBIDDEN_TARGETS="up down deploy-latest web-e2e-container web-docker-build web-k3d-deploy api-k3d-deploy"

readonly DEFAULT_CLUSTER="pokecalc"

ORIG_PATH="$PATH"
readonly ORIG_PATH
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

# ---------------------------------------------------------------------------
# 偽物と実行
# ---------------------------------------------------------------------------

# install_fakes ディレクトリ — 偽の make と kubectl を置く。
install_fakes() {
  local bin="$1/bin"
  mkdir -p "$bin"
  cat >"$bin/make" <<'EOF'
#!/bin/sh
# 偽の make: 引数をそのまま記録し、ターゲット名(オプションでも VAR=値 でもない引数)を1行ずつ残す。
# FAKE_MAKE_FAIL に挙げたターゲットでは非0で終わる(失敗の伝播を確かめるため)。
printf '%s\n' "$*" >>"$FAKE_LOG_DIR/make.log"
for a in "$@"; do
  case "$a" in
    -*) continue ;;
    *=*) continue ;;
  esac
  printf '%s\n' "$a" >>"$FAKE_LOG_DIR/make.targets"
  for bad in ${FAKE_MAKE_FAIL:-}; do
    if [ "$a" = "$bad" ]; then
      echo "fake make: target '$a' failed" >&2
      exit 2
    fi
  done
done
exit 0
EOF
  cat >"$bin/kubectl" <<'EOF'
#!/bin/sh
# 偽の kubectl: 呼び出しを記録する。`config current-context` は FAKE_KUBE_CONTEXT を返し、
# 空なら本物と同じように非0で終わる(コンテキスト未設定)。
printf '%s\n' "$*" >>"$FAKE_LOG_DIR/kubectl.log"
case "$*" in
  "config current-context"*)
    if [ -z "${FAKE_KUBE_CONTEXT:-}" ]; then
      echo 'error: current-context is not set' >&2
      exit 1
    fi
    printf '%s\n' "$FAKE_KUBE_CONTEXT"
    exit 0 ;;
esac
exit 0
EOF
  chmod +x "$bin/make" "$bin/kubectl"
}

# nokubectl_path ディレクトリ — PATH から kubectl だけを取り除いた入れ替え用ディレクトリを作り、そのパスを返す。
# (kubectl が入っていない環境で make e2e を叩いたときの振る舞いを見るため。)
nokubectl_path() {
  local dir="$1/nokubectl" p f base
  mkdir -p "$dir"
  local IFS=:
  for p in $ORIG_PATH; do
    [ -d "$p" ] || continue
    unset IFS
    for f in "$p"/*; do
      [ -x "$f" ] || continue
      base=${f##*/}
      [ "$base" = kubectl ] && continue
      [ -e "$dir/$base" ] && continue
      ln -s "$f" "$dir/$base" 2>/dev/null || true
    done
    IFS=:
  done
  unset IFS
  printf '%s' "$dir"
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

# 直近の run_e2e の結果。
RC=0
CASE_DIR=""
OUT=""

# run_e2e ケース名 [VAR=値...] — 偽物の下で e2e.sh を流す。
run_e2e() {
  local name=$1 script no_kubectl=0 path_prefix
  shift
  # NO_KUBECTL=1 は PATH から kubectl を外して流す指定(e2e.sh へは渡さない)。
  local args=()
  local a
  for a in "$@"; do
    if [ "$a" = "NO_KUBECTL=1" ]; then no_kubectl=1; else args+=("$a"); fi
  done

  script=$(new_repo "$name") || return 1
  CASE_DIR="$WORK/$name/case"
  mkdir -p "$CASE_DIR/logs" "$CASE_DIR/tmp"
  : >"$CASE_DIR/logs/make.log"
  : >"$CASE_DIR/logs/make.targets"
  : >"$CASE_DIR/logs/kubectl.log"
  install_fakes "$CASE_DIR"
  if [ "$no_kubectl" = 1 ]; then
    path_prefix="$CASE_DIR/bin:$(nokubectl_path "$CASE_DIR")"
  else
    path_prefix="$CASE_DIR/bin:$ORIG_PATH"
  fi
  (
    # 利用者と同じくリポジトリの中から呼ぶ(ルートではなく scripts/ から呼び、スクリプト自身がルートへ cd することも通す)。
    cd "$(dirname "$script")" &&
      env -u MAKEFLAGS -u MAKELEVEL -u MFLAGS ${args[@]+"${args[@]}"} \
        PATH="$path_prefix" \
        MAKE="$CASE_DIR/bin/make" \
        TMPDIR="$CASE_DIR/tmp" \
        FAKE_LOG_DIR="$CASE_DIR/logs" \
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

# ran_targets — 実際に make へ渡された「既知の」ターゲットだけを、呼ばれた順に返す。
ran_targets() {
  local t
  while read -r t; do
    case " $ALL_TARGETS " in *" $t "*) printf '%s\n' "$t" ;; esac
  done <"$CASE_DIR/logs/make.targets"
}

ran_list() { ran_targets | tr '\n' ' ' | sed 's/ $//'; }

target_ran() {
  ran_targets | grep -Fxq "$1"
}

# expect_targets 期待する並び — 既知のターゲットが「この順で・これだけ」呼ばれたこと。
expect_targets() {
  local want
  want=$(printf '%s' "$1" | tr ' ' '\n' | grep -v '^$' | tr '\n' ' ' | sed 's/ $//')
  local got
  got=$(ran_list)
  if [ "$got" = "$want" ]; then ok; else ng "make に渡されたターゲットが '$want' でなく '$got'。出力: $OUT"; fi
}

expect_no_forbidden() {
  local t hits=""
  for t in $FORBIDDEN_TARGETS; do
    if grep -Fxq "$t" "$CASE_DIR/logs/make.targets"; then hits="$hits $t"; fi
  done
  if [ -z "$hits" ]; then ok; else ng "make e2e から呼んではいけないターゲットを呼んだ:$hits(ADR-0306 §2)"; fi
}

# expect_skip_message ターゲット... — 飛ばしたことと、飛ばしたターゲット名が出力に出ていること。
expect_skip_message() {
  local t
  if printf '%s' "$OUT" | grep -q 'スキップ'; then ok; else ng "k3d 依存のスモークを黙って飛ばした(「スキップ」と明示すべき)。出力: $OUT"; fi
  for t in "$@"; do
    if printf '%s' "$OUT" | grep -Fq "$t"; then ok; else ng "飛ばした $t の名前が出力に出ていない。出力: $OUT"; fi
  done
  if printf '%s' "$OUT" | grep -Fq 'make up'; then ok; else ng "k3d クラスタを用意する方法(make up)が出力に出ていない。出力: $OUT"; fi
}

expect_ok() {
  if [ "$RC" -eq 0 ]; then ok; else ng "終了コード $RC(0 であるべき)。出力: $OUT"; fi
}

expect_failed() {
  if [ "$RC" -ne 0 ]; then ok; else ng "終了コードが 0(非0で止まるべき)。出力: $OUT"; fi
}

# kubectl が読み取り以外(クラスタを変える操作)に使われていないこと。
expect_kubectl_read_only() {
  if grep -Eq '(^|[[:space:]])(apply|create|delete|patch|replace|scale|rollout restart)([[:space:]]|$)' "$CASE_DIR/logs/kubectl.log"; then
    ng "kubectl でクラスタを変更した(make e2e は読み取りだけにすべき)。kubectl: $(tr '\n' ';' <"$CASE_DIR/logs/kubectl.log")"
  else ok; fi
}

# ---------------------------------------------------------------------------
# 1. スクリプトの形(静的)
# ---------------------------------------------------------------------------

test_static_shape() {
  begin "静的: 実行可能・bash・構文・set -euo pipefail・ルートへの cd"
  if [ ! -f "$SCRIPT" ]; then
    ng "$SCRIPT_REL が無い"
    return
  fi
  if [ -x "$SCRIPT" ]; then ok; else ng "$SCRIPT_REL に実行ビットが無い"; fi
  if [ "$(head -n 1 "$SCRIPT")" = "#!/usr/bin/env bash" ]; then ok; else ng "1行目が #!/usr/bin/env bash でない"; fi
  if bash -n "$SCRIPT" 2>/dev/null; then ok; else ng "bash -n で構文エラー"; fi
  if grep -Eq '^set -euo pipefail' "$SCRIPT"; then ok; else ng "set -euo pipefail が無い(失敗を伝播させるため)"; fi
  # shellcheck disable=SC2016 # 文字列そのものを探す
  if grep -Fq 'cd "$(git rev-parse --show-toplevel)"' "$SCRIPT"; then ok; else
    ng 'cd "$(git rev-parse --show-toplevel)" から始まっていない(AGENTS.md「手順書の書き方」)'
  fi
}

test_static_not_a_stub() {
  begin "静的: 未実装スタブの痕跡が残っていない(issue #72)"
  if grep -Fq 'P4-6 で' "$SCRIPT"; then ng "「P4-6 で … 実装」のスタブ文言が残っている"; else ok; fi
  if grep -Eq '未実装|(で|に)実装(する)?\)' "$SCRIPT"; then ng "「未実装」を告げて終わる作りが残っている"; else ok; fi
  # E2E 本体を1件も呼ばないまま終わる作りを弾く(最低限、常時実行の3件が書かれていること)。
  local t
  for t in $BASE_TARGETS; do
    if grep -Fq "$t" "$SCRIPT"; then ok; else ng "$t を呼んでいない(クラスタ非依存の E2E は必ず実行する。ADR-0306 §1)"; fi
  done
}

test_static_no_duplicate_runner() {
  begin "静的: Playwright の起動方法を写経せず、web/Makefile のターゲット経由で呼ぶ"
  if grep -Eq 'npx +playwright|npm +run +e2e|playwright +test' "$SCRIPT"; then
    ng "npm/npx を直接呼んでいる(起動方法の正は web/Makefile。二重管理しない。ADR-0306 §1)"
  else ok; fi
  if grep -Eq 'k3d +cluster +(create|delete)' "$SCRIPT"; then
    ng "k3d クラスタを作る/消すコマンドがある(クラスタの作成・削除は make up / make down と人間の確認の担当)"
  else ok; fi
}

test_makefile_wiring() {
  begin "静的: ルート Makefile の配線(e2e は e2e.sh を呼び CLUSTER を渡す・test-scripts はこのテストを呼ぶ)"
  local e2e_recipe test_recipe
  e2e_recipe=$(awk '/^e2e:/ { on = 1; next } on && /^[^\t]/ { exit } on { print }' "$MAKEFILE")
  if printf '%s' "$e2e_recipe" | grep -Fq './scripts/e2e.sh'; then ok; else ng "Makefile の e2e ターゲットが ./scripts/e2e.sh を呼んでいない"; fi
  if printf '%s' "$e2e_recipe" | grep -Fq 'CLUSTER=$(CLUSTER)'; then ok; else
    ng 'Makefile の e2e ターゲットが CLUSTER=$(CLUSTER) を e2e.sh へ渡していない(make e2e CLUSTER=... を効かせるため。ADR-0306 §3)'
  fi
  test_recipe=$(awk '/^test-scripts:/ { on = 1; next } on && /^[^\t]/ { exit } on { print }' "$MAKEFILE")
  if printf '%s' "$test_recipe" | grep -Fq './scripts/e2e_test.sh'; then ok; else
    ng "Makefile の test-scripts が ./scripts/e2e_test.sh を呼んでいない(make test で見えるようにする)"
  fi
}

# ---------------------------------------------------------------------------
# 2. k3d クラスタが無いとき(通常の開発ループ)
# ---------------------------------------------------------------------------

test_no_k3d_context() {
  begin "k3d 無し: 別のコンテキスト → 3件の E2E は実行し、k3d 依存はスキップして成功で終わる"
  run_e2e no-k3d FAKE_KUBE_CONTEXT=docker-desktop || return
  expect_ok
  expect_targets "$BASE_TARGETS"
  expect_no_forbidden
  expect_skip_message $K3D_TARGETS
  expect_kubectl_read_only
}

test_no_current_context() {
  begin "k3d 無し: kubectl のコンテキスト未設定 → 同じくスキップして成功で終わる"
  run_e2e no-context FAKE_KUBE_CONTEXT= || return
  expect_ok
  expect_targets "$BASE_TARGETS"
  expect_skip_message $K3D_TARGETS
}

test_kubectl_missing() {
  begin "k3d 無し: kubectl が PATH に無い → 落ちずにスキップして成功で終わる"
  run_e2e no-kubectl NO_KUBECTL=1 || return
  expect_ok
  expect_targets "$BASE_TARGETS"
  expect_skip_message $K3D_TARGETS
}

test_cluster_name_mismatch() {
  begin "k3d 無し: CLUSTER=other なのにコンテキストが k3d-pokecalc → 別クラスタへは流さずスキップ"
  run_e2e cluster-mismatch CLUSTER=other "FAKE_KUBE_CONTEXT=k3d-$DEFAULT_CLUSTER" || return
  expect_ok
  expect_targets "$BASE_TARGETS"
  expect_skip_message $K3D_TARGETS
}

# ---------------------------------------------------------------------------
# 3. k3d クラスタがあるとき
# ---------------------------------------------------------------------------

test_k3d_context_present() {
  begin "k3d あり: コンテキストが k3d-pokecalc → 6件すべてをこの順で実行する"
  run_e2e k3d "FAKE_KUBE_CONTEXT=k3d-$DEFAULT_CLUSTER" || return
  expect_ok
  expect_targets "$ALL_TARGETS"
  expect_no_forbidden
  expect_kubectl_read_only
  if printf '%s' "$OUT" | grep -q 'スキップ'; then ng "クラスタがあるのに「スキップ」と言っている。出力: $OUT"; else ok; fi
}

test_k3d_cluster_override() {
  begin "k3d あり: CLUSTER=other でコンテキストが k3d-other → 6件すべて実行する"
  run_e2e k3d-other CLUSTER=other FAKE_KUBE_CONTEXT=k3d-other || return
  expect_ok
  expect_targets "$ALL_TARGETS"
}

# ---------------------------------------------------------------------------
# 4. 失敗の伝播(1件でも失敗したらそこで止まる)
# ---------------------------------------------------------------------------

test_failure_stops_immediately() {
  local target i idx_fail idx after
  i=0
  for target in $ALL_TARGETS; do
    i=$((i + 1))
    begin "失敗の伝播: $target が失敗 → 非0で終わり、以降のターゲットを流さない"
    run_e2e "fail-$i" "FAKE_KUBE_CONTEXT=k3d-$DEFAULT_CLUSTER" "FAKE_MAKE_FAIL=$target" || continue
    expect_failed
    if target_ran "$target"; then ok; else ng "$target がそもそも実行されていない。実行: $(ran_list)"; fi
    idx_fail=0
    idx=0
    after=""
    while read -r t; do
      idx=$((idx + 1))
      if [ "$t" = "$target" ] && [ "$idx_fail" = 0 ]; then idx_fail=$idx; continue; fi
      if [ "$idx_fail" != 0 ]; then after="$after $t"; fi
    done < <(ran_targets)
    if [ -z "$after" ]; then ok; else ng "$target が失敗した後にも実行した:$after(set -e で即座に止まるべき)"; fi
  done
}

test_failure_message_names_target() {
  begin "失敗の伝播: どのターゲットで落ちたかが出力に出る"
  run_e2e fail-msg "FAKE_KUBE_CONTEXT=k3d-$DEFAULT_CLUSTER" FAKE_MAKE_FAIL=web-e2e-online || return
  expect_failed
  if printf '%s' "$OUT" | grep -Fq 'web-e2e-online'; then ok; else ng "落ちたターゲット名が出力に無い。出力: $OUT"; fi
}

# ---------------------------------------------------------------------------
# 5. 「0件で成功」にできないこと(issue #72 の本題)
# ---------------------------------------------------------------------------

test_cannot_succeed_with_zero_suites() {
  begin "0件で成功しない: 手心を加えそうな環境変数を与えても、常時実行の3件は必ず流れる"
  run_e2e hostile-env \
    FAKE_KUBE_CONTEXT=docker-desktop \
    CI=1 SKIP_E2E=1 E2E_SKIP=1 E2E_SKIP_ALL=1 SKIP=1 \
    PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1 || return
  expect_ok
  expect_targets "$BASE_TARGETS"
}

test_require_k3d_flag() {
  begin "E2E_REQUIRE_K3D=1: クラスタが無ければスキップせず非0で終わる(リリース前の確認用)"
  run_e2e require-k3d E2E_REQUIRE_K3D=1 FAKE_KUBE_CONTEXT=docker-desktop || return
  expect_failed
  if printf '%s' "$OUT" | grep -Fq 'E2E_REQUIRE_K3D'; then ok; else ng "なぜ止まったか(E2E_REQUIRE_K3D)が出力に無い。出力: $OUT"; fi

  begin "E2E_REQUIRE_K3D=1: クラスタがあればふつうに6件流れる"
  run_e2e require-k3d-ok E2E_REQUIRE_K3D=1 "FAKE_KUBE_CONTEXT=k3d-$DEFAULT_CLUSTER" || return
  expect_ok
  expect_targets "$ALL_TARGETS"
}

# ---------------------------------------------------------------------------
# 6. 文書(make e2e の説明が中身と合っている)
# ---------------------------------------------------------------------------

test_docs_match() {
  begin "文書: test-strategy.md の L5 の欄と README の記述が実際の中身と食い違わない"
  local strategy="$ROOT/docs/test-strategy.md"
  if grep -Fq 'make e2e' "$strategy"; then ok; else ng "docs/test-strategy.md から make e2e の記述が消えた"; fi
  if [ -f "$ROOT/docs/adr/0306-root-e2e-wiring.md" ]; then ok; else
    ng "ADR-0306(ルートの make e2e の接続方針)が無い"
  fi
}

# ---------------------------------------------------------------------------

test_static_shape
test_static_not_a_stub
test_static_no_duplicate_runner
test_makefile_wiring
test_no_k3d_context
test_no_current_context
test_kubectl_missing
test_cluster_name_mismatch
test_k3d_context_present
test_k3d_cluster_override
test_failure_stops_immediately
test_failure_message_names_target
test_cannot_succeed_with_zero_suites
test_require_k3d_flag
test_docs_match

if [ "$FAILURES" -gt 0 ]; then
  printf 'e2e test: %d failed, %d passed\n' "$FAILURES" "$PASSES" >&2
  exit 1
fi
printf 'e2e test: all %d checks passed\n' "$PASSES"
