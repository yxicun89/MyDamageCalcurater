#!/usr/bin/env bash
# scripts/ai-guard/bash-guard.sh の自動テスト(ADR-0800。issue #273・#239)。`make test-scripts`(make test に含む)から流す。
#
# 方式: 既存の scripts/*_test.sh と同じ、手書きの ok/ng ヘルパー付きの bash スクリプト(bats は入れない)。
# 各ケースで Claude Code / Codex の PreToolUse フックと同じ形の JSON(`{"tool_input":{"command":"..."}}`)を stdin で渡し、
# 終了コードを確かめる:
#   - 2 = block(人間の確認が必要な操作。stderr に理由を書く)
#   - 0 = 通過(何も出力しない。通常の許可フローに委ねる)
# bash-guard.sh はコマンドを実行せず、文字列だけで判定する(ADR-0800 §2)。ここでも実コマンドは一切流さない。
# JSON は jq -n --arg で組み立てる(引用符・改行を含むコマンドも正しくエスケープするため)。
#
# 後半は設定ファイルの静的検査(.claude/settings.json・.codex/config.toml が bash-guard.sh を呼び、
# 広すぎる許可を持たないこと)。
set -uo pipefail

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
readonly ROOT
readonly GUARD_REL="scripts/ai-guard/bash-guard.sh"
readonly GUARD="$ROOT/$GUARD_REL"
readonly CLAUDE_SETTINGS="$ROOT/.claude/settings.json"
readonly CODEX_CONFIG="$ROOT/.codex/config.toml"

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

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

# MAIN_REPO — ブランチ main に checkout 済みの使い捨て git リポジトリ。
# "git push"(宛先省略)・"git push origin HEAD" のように宛先が現在のブランチに解決される
# ケースを、CI でも再現性のある形で検証するために使う(現在のブランチが main なら block、という契約)。
MAIN_REPO="$WORK/main-repo"
mkdir -p "$MAIN_REPO"
git -C "$MAIN_REPO" init -q
git -C "$MAIN_REPO" symbolic-ref HEAD refs/heads/main
git -C "$MAIN_REPO" -c user.email=bash-guard-test@example.com -c user.name=bash-guard-test commit -q --allow-empty -m init

# OTHER_REPO_MAIN — MAIN_REPO とは別の、ブランチ main の使い捨てリポジトリ。
# "git -C <dir> push"(宛先省略)・"cd <dir> && git push" が、フック自身の cwd ではなく <dir> の
# 実際のブランチを見て判定することを確認するために使う(critic 3回目指摘)。
OTHER_REPO_MAIN="$WORK/other-repo-main"
mkdir -p "$OTHER_REPO_MAIN"
git -C "$OTHER_REPO_MAIN" init -q
git -C "$OTHER_REPO_MAIN" symbolic-ref HEAD refs/heads/main
git -C "$OTHER_REPO_MAIN" -c user.email=bash-guard-test@example.com -c user.name=bash-guard-test commit -q --allow-empty -m init

# OTHER_REPO_FEATURE — 同様だがブランチが main ではない使い捨てリポジトリ(誤検知しないことの確認用)。
OTHER_REPO_FEATURE="$WORK/other-repo-feature"
mkdir -p "$OTHER_REPO_FEATURE"
git -C "$OTHER_REPO_FEATURE" init -q
git -C "$OTHER_REPO_FEATURE" symbolic-ref HEAD refs/heads/feature-x
git -C "$OTHER_REPO_FEATURE" -c user.email=bash-guard-test@example.com -c user.name=bash-guard-test commit -q --allow-empty -m init

# FAKE_BIN — PATH の先頭に置く偽の gh(ADR-0803)。本物の gh・GitHub には一切触れない。
#   - `gh pr view ... --json headRefOid` は固定 SHA(FAKE_SHA。FAKE_GH_SHA で差し替え)を返す。FAKE_GH_SLEEP があればその秒数 sleep する(時間切れの検証)
#   - `gh pr checks ...` は引数を $FAKE_GH_LOG に記録し、環境変数 FAKE_GH_CHECKS_RC(既定 1=失敗)で終了する
#   - それ以外(`gh pr merge` を含む)は呼ばれたら 97 で失敗し、記録に残す(ガードがマージ等を実行しないことの検証用)
FAKE_BIN="$WORK/fake-bin"
FAKE_GH_LOG="$WORK/fake-gh.log"
mkdir -p "$FAKE_BIN"
: >"$FAKE_GH_LOG"
cat >"$FAKE_BIN/gh" <<'FAKE'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"$FAKE_GH_LOG"
if [ -n "${FAKE_GH_SLEEP:-}" ]; then exec sleep "$FAKE_GH_SLEEP"; fi
if [ "${1:-}" = "pr" ] && [ "${2:-}" = "view" ]; then
  printf "%s\n" "${FAKE_GH_SHA:-$FAKE_SHA_OUT}"
  exit "${FAKE_GH_VIEW_RC:-0}"
fi
if [ "${1:-}" = "pr" ] && [ "${2:-}" = "checks" ]; then
  exit "${FAKE_GH_CHECKS_RC:-1}"
fi
exit 97
FAKE
chmod +x "$FAKE_BIN/gh"
# FAKE_SHA — 偽 gh の `pr view --json headRefOid` が返す、固定の HEAD の SHA。
# FAKE_GH_SHA で差し替えると「検証後に HEAD が動いた」状態を作れる。
readonly FAKE_SHA=0123456789abcdef0123456789abcdef01234567
export FAKE_SHA_OUT="$FAKE_SHA"
export FAKE_GH_LOG
export FAKE_GH_CHECKS_RC=1
export PATH="$FAKE_BIN:$PATH"

# NOGH_BIN — gh だけが無い PATH(ガードが使う道具だけをリンクする)。
NOGH_BIN="$WORK/nogh-bin"
mkdir -p "$NOGH_BIN"
for tool in bash jq sed tr grep cat git env uname mktemp rm sleep; do
  src=$(command -v "$tool" 2>/dev/null) && ln -s "$src" "$NOGH_BIN/$tool"
done

# with_checks_rc 終了コード — 以降の run_guard で偽 gh の `pr checks` をその終了コードにする。
with_checks_rc() { export FAKE_GH_CHECKS_RC="$1"; }

# run_guard コマンド文字列 — bash-guard.sh に PreToolUse 形の JSON を渡し、終了コードを GUARD_RC に、
# stdout・stderr を $WORK/out・$WORK/err に残す。
run_guard() {
  jq -n --arg c "$1" '{tool_name:"Bash", tool_input:{command:$c}}' >"$WORK/in.json"
  bash "$GUARD" <"$WORK/in.json" >"$WORK/out" 2>"$WORK/err"
  GUARD_RC=$?
}

# run_guard_in ディレクトリ コマンド文字列 — 指定ディレクトリを cwd にして bash-guard.sh を実行する。
# git push の宛先が省略・HEAD/@ の場合は現在のブランチに解決して判定するため、
# その契約(ブランチが main なら block)を再現性を持って確かめるのに使う。
run_guard_in() {
  local dir="$1" cmd="$2"
  jq -n --arg c "$cmd" '{tool_name:"Bash", tool_input:{command:$c}}' >"$WORK/in.json"
  (cd "$dir" && bash "$GUARD") <"$WORK/in.json" >"$WORK/out" 2>"$WORK/err"
  GUARD_RC=$?
}

# expect_block コマンド文字列 — exit 2 で止まり、stderr に理由があること。
expect_block() {
  run_guard "$1"
  if [ "$GUARD_RC" = 2 ]; then ok; else ng "block されるべき(exit 2)が exit $GUARD_RC: $1"; return; fi
  if [ -s "$WORK/err" ]; then ok; else ng "block したのに stderr に理由が無い: $1"; fi
}

# expect_block_in ディレクトリ コマンド文字列 — run_guard_in 版の expect_block。
expect_block_in() {
  run_guard_in "$1" "$2"
  if [ "$GUARD_RC" = 2 ]; then ok; else ng "block されるべき(exit 2)が exit $GUARD_RC: $2"; return; fi
  if [ -s "$WORK/err" ]; then ok; else ng "block したのに stderr に理由が無い: $2"; fi
}

# expect_pass コマンド文字列 — exit 0 で通り、stdout に何も出さないこと(誤検知しない)。
expect_pass() {
  run_guard "$1"
  if [ "$GUARD_RC" = 0 ]; then ok; else ng "通過すべき(exit 0)が exit $GUARD_RC: $1"; return; fi
  if [ ! -s "$WORK/out" ]; then ok; else ng "通過時は stdout に何も出さない: $1"; fi
}

# expect_pass_in ディレクトリ コマンド文字列 — run_guard_in 版の expect_pass。
expect_pass_in() {
  run_guard_in "$1" "$2"
  if [ "$GUARD_RC" = 0 ]; then ok; else ng "通過すべき(exit 0)が exit $GUARD_RC: $2"; return; fi
  if [ ! -s "$WORK/out" ]; then ok; else ng "通過時は stdout に何も出さない: $2"; fi
}

test_prerequisites() {
  begin "前提: jq がある・$GUARD_REL が存在する"
  if command -v jq >/dev/null 2>&1; then ok; else ng "jq が無い(make doctor で確認)"; fi
  if [ -f "$GUARD" ]; then ok; else ng "$GUARD_REL が無い"; fi
  if [ -x "$GUARD" ]; then ok; else ng "$GUARD_REL に実行権限が無い(フックから直接呼ぶため)"; fi
}

# ---- block すべきもの ----

test_block_cluster_delete() {
  begin "block: クラスタ削除(k3d・make down)"
  expect_block "k3d cluster delete pokecalc"
  expect_block "k3d cluster delete --all"
  expect_block "k3d cluster rm pokecalc"
  expect_block "make down"
  expect_block "make -C . down"
  expect_block "make test down"
}

test_block_kubectl_data_delete() {
  begin "block: kubectl による ns/pvc/pv/statefulset/secret の削除(順序・省略形・スラッシュ表記に頑健)"
  expect_block "kubectl delete namespace pokecalc"
  expect_block "kubectl delete ns pokecalc"
  expect_block "kubectl delete namespaces pokecalc"
  expect_block "kubectl delete pvc mysql-data"
  expect_block "kubectl -n pokecalc delete pvc mysql-data"
  expect_block "kubectl --namespace=pokecalc delete pvc mysql-data"
  expect_block "kubectl delete pvc/mysql-data"
  expect_block "kubectl delete persistentvolumeclaim mysql-data"
  expect_block "kubectl delete persistentvolumeclaims --all -n pokecalc"
  expect_block "kubectl delete pv pvc-1234"
  expect_block "kubectl delete persistentvolume pvc-1234"
  expect_block "kubectl delete statefulset mysql"
  expect_block "kubectl delete sts/mysql"
  expect_block "kubectl delete secret mysql-auth"
  expect_block "kubectl -n pokecalc delete secrets/mysql-auth"
  expect_block "kubectl delete pod,pvc -l app=mysql"
  expect_block "kubectl --context k3d-pokecalc delete --wait=false ns pokecalc"
}

test_block_kubectl_delete_kustomize() {
  begin "block: kubectl delete -k(kustomize 一括削除)"
  expect_block "kubectl delete -k deploy/k8s/overlays/local"
  expect_block "kubectl delete --kustomize deploy/k8s/overlays/local"
  expect_block "kubectl -n pokecalc delete -k deploy/k8s/overlays/local"
}

test_block_migrate_down() {
  begin "block: make migrate-down*(CONFIRM_DESTROY の有無に関わらず)"
  expect_block "make migrate-down"
  expect_block "make migrate-down-record"
  expect_block "make migrate-down-team"
  expect_block "make migrate-down CONFIRM_DESTROY=pokedex"
  expect_block "make CONFIRM_DESTROY=team migrate-down-team"
}

test_block_import() {
  begin "block: make import・make import-k8s(DB に書き込む)"
  expect_block "make import"
  expect_block "make import-k8s"
  expect_block "make import POKEDEX_DATABASE_DSN=x"
}

test_block_secret_read() {
  begin "block: 秘密の読み取り(kubectl get secret・.env・~/.ssh。迂回形も文字列で判定)"
  expect_block "kubectl get secret mysql-auth"
  expect_block "kubectl get secrets"
  expect_block "kubectl -n pokecalc get secret mysql-auth -o jsonpath='{.data.password}'"
  expect_block "kubectl get secret mysql-auth -o yaml"
  expect_block "kubectl get secret/mysql-auth -o json"
  expect_block "cat .env"
  expect_block "cat services/pokedex/.env"
  expect_block "node -e 'console.log(require(\"fs\").readFileSync(\".env\",\"utf8\"))'"
  expect_block "cat ~/.ssh/id_ed25519"
  expect_block "docker run --rm -v ~/.ssh:/x alpine ls /x"
}

test_block_push_to_main() {
  begin "block: main への git push(書き方・位置によらず)"
  expect_block "git push origin main"
  expect_block "git push origin HEAD:refs/heads/main"
  expect_block "git push origin HEAD:main"
  expect_block "git push origin main:main"
  expect_block "git push origin feat/x:main"
  expect_block "git push origin :main"
  expect_block "git push origin refs/heads/main"
  expect_block "git push origin main -q"
  expect_block "git push -q origin main"
  expect_block "git push --no-verify origin main"
  expect_block "git -C /tmp/repo push origin main"
}

test_block_force_push() {
  begin "block: 強制系の git push(--force・-f・+refspec・--mirror・--all)"
  expect_block "git push --force origin feat/x"
  expect_block "git push origin feat/x --force"
  expect_block "git push -f origin feat/x"
  expect_block "git push origin feat/x -f"
  expect_block "git push --force-with-lease origin feat/x"
  expect_block "git push origin +feat/x"
  expect_block "git push origin +HEAD:feat/x"
  expect_block "git push --mirror origin"
  expect_block "git push --all origin"
}

test_block_pr_merge() {
  # ADR-0803: 対象 PR(番号/URL)の明示・--match-head-commit で HEAD の SHA を固定・CI が全件成功(gh pr checks が 0)の
  # すべてを満たすときだけ通す。それ以外はすべて block(fail-safe)。
  local rc m="--match-head-commit $FAKE_SHA"
  for rc in 1 8 127 124; do
    with_checks_rc "$rc"
    begin "block: gh pr merge は CI が全件成功でない(gh pr checks の終了コード $rc)ならどんな追加引数でも block"
    expect_block "gh pr merge 123 $m"
    expect_block "gh pr merge 123 $m --squash --delete-branch"
    expect_block "gh pr merge --auto --merge 123 $m"
    expect_block "gh pr merge -R o/r 123 $m"
    expect_block "gh pr merge"
    expect_block "gh pr merge 123"
  done

  with_checks_rc 0
  begin "block: CI 全件成功でも --admin(チェック回避)・動的/危険な引数・複数の対象は block"
  expect_block "gh pr merge 123 $m --admin"
  expect_block "gh pr merge --admin --squash 123 $m"
  expect_block "gh pr merge --admin=true 123 $m"
  expect_block "gh pr merge -R --admin 5 $m"
  expect_block "gh pr merge -R -x 5 $m"
  expect_block "gh pr merge --repo --admin 5 $m"
  expect_block "gh pr merge --repo=-x 5 $m"
  expect_block "gh pr merge 123 $m \$(echo x)"
  expect_block "gh pr merge \$(echo 123) $m"
  expect_block "gh pr merge \$PR $m"
  expect_block 'gh pr merge `echo 123`'
  expect_block "gh pr merge \${PR} $m"
  expect_block "gh pr merge 123 $m -b \"\$(id)\""
  expect_block "gh pr merge ~/x $m"
  expect_block "gh pr merge 1* $m"
  expect_block "gh pr merge 1 2 $m"
  expect_block "gh pr merge -R \"o/r;x\" 123 $m"
  expect_block "gh pr merge -R \$R 123 $m"
  expect_block "gh pr merge --repo=o/r\$X 123 $m"
  expect_block "gh pr merge --body 'two words' 123 $m"
  expect_block "gh pr merge 123 $m --admin; echo ok"

  begin "block: 迂回形(まとめ書きフラグ・-R の別表記・未知フラグ・ブランチ名/引数なし)"
  expect_block "gh pr merge -sb 504 $m"
  expect_block "gh pr merge -A 504 $m"
  expect_block "gh pr merge -sA 504 $m"
  expect_block "gh pr merge -R o/r"
  expect_block "gh pr merge -Ro/r 504 $m --admin"
  expect_block "gh pr merge --unknown-flag 504 $m"
  expect_block "gh pr merge -x 504 $m"
  expect_block "gh pr merge feat/ops-merge-on-green $m"
  expect_block "gh pr merge $m"
  expect_block "gh pr merge --squash $m"
  expect_block "gh pr merge http://github.com/o/r/pull/1 $m"
  expect_block "gh pr merge https://github.com/o/r/issues/1 $m"

  begin "block: --match-head-commit の省略・不一致(検証後に追加された commit をマージしない。TOCTOU 対策)"
  expect_block "gh pr merge 123"
  expect_block "gh pr merge 123 --squash --delete-branch"
  expect_block "gh pr merge 123 --match-head-commit ffffffffffffffffffffffffffffffffffffffff"
  expect_block "gh pr merge 123 --match-head-commit=ffffffffffffffffffffffffffffffffffffffff"
  expect_block "gh pr merge 123 --match-head-commit"
  expect_block "gh pr merge 123 --match-head-commit ${FAKE_SHA:0:7}"

  begin "block: 環境変数で対象・認証・設定を差し替える形(GH_REPO 等。export 形も)"
  expect_block "GH_REPO=o/r gh pr merge 123 $m"
  expect_block "export GH_REPO=o/r && gh pr merge 123 $m"
  expect_block "export GH_REPO=o/r; gh pr merge 123 $m"
  expect_block "GH_HOST=example.com gh pr merge 123 $m"
  expect_block "GH_TOKEN=x gh pr merge 123 $m"
  expect_block "GH_ENTERPRISE_TOKEN=x gh pr merge 123 $m"
  expect_block "GH_CONFIG_DIR=/tmp/x gh pr merge 123 $m"
  expect_block "env GH_REPO=o/r gh pr merge 123 $m"

  begin "block: 他コマンドとの連結・cd/pushd/xargs 越し(対象を静的に確定できない)"
  expect_block "git push origin feat/x && gh pr merge 123 $m"
  expect_block "git push -u origin feat/x; gh pr merge 123 $m"
  expect_block "make test && gh pr merge 123 $m"
  expect_block "gh pr merge 123 $m && git status"
  expect_block "gh pr merge 123 $m | cat"
  expect_block "gh pr merge 123 $m || true"
  expect_block "cd /tmp && gh pr merge 123 $m"
  expect_block "pushd /tmp && gh pr merge 123 $m"
  expect_block "pushd /tmp; gh pr merge 123 $m"
  expect_block "cd /tmp; gh pr merge 123 $m"
  expect_block "xargs gh pr merge $m"
  expect_block "echo 123 | xargs gh pr merge $m"
  expect_block "echo 123 | xargs -I{} gh pr merge {} $m"
  expect_block $'gh pr merge 123 '"$m"$'\nrm x'

  begin "block: gh alias の登録・取り込み、組み込みでないサブコマンド越しの merge"
  expect_block "gh alias set m 'pr merge'"
  expect_block "gh alias import aliases.yml"
  expect_block "gh m 123 --admin merge"
  expect_block "gh co merge 1"
  expect_block "gh alias exec merge 1"
  expect_block "gh extension exec merge 1"

  begin "block: 時間切れ(gh が応答しない)は判定不能として block"
  with_checks_rc 0
  FAKE_GH_SLEEP=5 BASH_GUARD_GH_TIMEOUT=1 expect_block "gh pr merge 123 $m"

  begin "pass: CI 全件成功・PR 番号/URL を明示・--match-head-commit が現在の HEAD と一致なら gh pr merge を通す"
  with_checks_rc 0
  expect_pass "gh pr merge 123 $m"
  expect_pass "gh pr merge 123 --match-head-commit=$FAKE_SHA"
  expect_pass "gh pr merge 123 --squash --delete-branch $m"
  expect_pass "gh pr merge --auto --merge 123 $m"
  expect_pass "gh pr merge -sd 123 $m"
  expect_pass "gh pr merge -R o/r 123 $m"
  expect_pass "gh pr merge -Ro/r 123 $m"
  expect_pass "gh pr merge -R=o/r 123 $m"
  expect_pass "gh pr merge -sR o/r 123 $m"
  expect_pass "gh pr merge --repo o/r 123 $m"
  expect_pass "gh pr merge --repo=o/r 123 $m"
  expect_pass "gh -R o/r pr merge 123 $m"
  expect_pass "gh pr merge https://github.com/o/r/pull/123 $m"
  expect_pass "gh pr merge 123 $m --body-file notes.md"
  expect_pass "gh pr merge 123 $m -A me@example.com"
  expect_pass $'gh pr \\\nmerge 123 '"$m"
  expect_pass "GH pr merge 123 $m"
  expect_pass "gh pr merge 123 $m "$'\n'
  # 以降の既存テスト(CI 失敗扱いで block を期待するもの)のため、既定の失敗に戻す。
  with_checks_rc 1
}

test_pr_merge_gate_checks_what_it_merges() {
  begin "gh pr merge: 検証する PR とマージする PR を一致させる(-R/対象を gh pr view・gh pr checks にも渡す)"
  with_checks_rc 0
  local mark m="--match-head-commit $FAKE_SHA"
  mark=$(wc -l <"$FAKE_GH_LOG")
  run_guard "gh pr merge 123 $m"
  if [ "$(tail -n +$((mark + 1)) "$FAKE_GH_LOG")" = "$(printf 'pr view 123 --json headRefOid -q .headRefOid\npr checks 123')" ]; then ok; else ng "pr view → pr checks の順に対象 123 で呼ばれるべき: $(tail -n +$((mark + 1)) "$FAKE_GH_LOG")"; fi
  mark=$(wc -l <"$FAKE_GH_LOG")
  run_guard "gh pr merge -R o/r 123 --squash $m"
  if [ "$(tail -n +$((mark + 1)) "$FAKE_GH_LOG")" = "$(printf 'pr view -R o/r 123 --json headRefOid -q .headRefOid\npr checks -R o/r 123')" ]; then ok; else ng "-R と対象が両方に渡されるべき: $(tail -n +$((mark + 1)) "$FAKE_GH_LOG")"; fi
  mark=$(wc -l <"$FAKE_GH_LOG")
  run_guard "gh -R o/r pr merge 123 $m"
  if [ "$(tail -n +$((mark + 1)) "$FAKE_GH_LOG")" = "$(printf 'pr view -R o/r 123 --json headRefOid -q .headRefOid\npr checks -R o/r 123')" ]; then ok; else ng "gh 直後の -R も渡されるべき: $(tail -n +$((mark + 1)) "$FAKE_GH_LOG")"; fi
  mark=$(wc -l <"$FAKE_GH_LOG")
  run_guard "gh pr merge -sR=o/r 123 $m"
  if [ "$(tail -n +$((mark + 1)) "$FAKE_GH_LOG")" = "$(printf 'pr view -R o/r 123 --json headRefOid -q .headRefOid\npr checks -R o/r 123')" ]; then ok; else ng "まとめ書き中の -R の値も取り出されるべき: $(tail -n +$((mark + 1)) "$FAKE_GH_LOG")"; fi
  mark=$(wc -l <"$FAKE_GH_LOG")
  run_guard "gh pr merge 123"
  if [ "$(wc -l <"$FAKE_GH_LOG")" = "$mark" ]; then ok; else ng "SHA 省略のとき gh は一切呼ばれないべき"; fi
  run_guard "gh pr merge 1 --admin $m"
  if [ "$(wc -l <"$FAKE_GH_LOG")" = "$mark" ]; then ok; else ng "--admin のとき gh は一切呼ばれないべき"; fi
  run_guard "GH_REPO=o/r gh pr merge 1 $m"
  if [ "$(wc -l <"$FAKE_GH_LOG")" = "$mark" ]; then ok; else ng "GH_REPO のとき gh は一切呼ばれないべき"; fi
  # HEAD が変わった(不一致)なら、pr view だけで止まり pr checks に進まない。
  mark=$(wc -l <"$FAKE_GH_LOG")
  FAKE_GH_SHA=ffffffffffffffffffffffffffffffffffffffff run_guard "gh pr merge 123 $m"
  if [ "$GUARD_RC" = 2 ]; then ok; else ng "HEAD 不一致は block されるべき"; fi
  if [ "$(tail -n +$((mark + 1)) "$FAKE_GH_LOG")" = "pr view 123 --json headRefOid -q .headRefOid" ]; then ok; else ng "不一致のとき pr checks に進まないべき: $(tail -n +$((mark + 1)) "$FAKE_GH_LOG")"; fi
  with_checks_rc 1
}

test_pr_merge_gate_without_gh() {
  begin "gh pr merge: gh が無い環境は判定不能として block(fail-safe)"
  with_checks_rc 0
  jq -n --arg c "gh pr merge 123 --match-head-commit $FAKE_SHA" '{tool_name:"Bash", tool_input:{command:$c}}' >"$WORK/in.json"
  PATH="$NOGH_BIN" "$NOGH_BIN/bash" "$GUARD" <"$WORK/in.json" >"$WORK/out" 2>"$WORK/err"
  GUARD_RC=$?
  if [ "$GUARD_RC" = 2 ]; then ok; else ng "gh が無いなら block(exit 2)されるべきが exit $GUARD_RC"; fi
  if [ -s "$WORK/err" ]; then ok; else ng "理由が stderr に無い"; fi
  with_checks_rc 1
}

test_pr_merge_never_executed_by_guard() {
  begin "ガードは PR のマージ自体も他の gh サブコマンドも実行しない(実行するのは検証済み引数の gh pr view / gh pr checks だけ。全テストを通したログ全体で検証)"
  if grep -qvE '^pr (view|checks)( |$)' "$FAKE_GH_LOG"; then ng "pr view / pr checks 以外の gh 呼び出しが記録された: $(grep -vE '^pr (view|checks)( |$)' "$FAKE_GH_LOG" | head -3)"; else ok; fi
  if grep -qE '^pr merge|^alias|^api' "$FAKE_GH_LOG"; then ng "gh pr merge 等が実行された"; else ok; fi
  if [ -s "$FAKE_GH_LOG" ]; then ok; else ng "ログが空(偽 gh が一度も使われていない=テストが無効)"; fi
}

# ADR-0804: 検証ゲート付きのマージスクリプトは通す(素のマージコマンドは ADR-0803 の条件を満たすときだけ通る)。
test_allow_pr_merge_gate_script() {
  begin "pass: scripts/pr-merge.sh(検証ゲート付きのマージ。ADR-0804)"
  expect_pass "scripts/pr-merge.sh 123"
  expect_pass "scripts/pr-merge.sh 123 --check"
  expect_pass "./scripts/pr-merge.sh 123"
  begin "block: ゲートを迂回する書き方は止めたまま"
  expect_block "gh pr merge 123 --admin"
  expect_block "scripts/pr-merge.sh 123 && gh pr merge 124"
  expect_block "bash -c 'gh pr merge 123'"
}

test_block_wrappers_and_compound() {
  begin "block: ラッパー(timeout・env・nohup・変数代入)・複合コマンド・サブシェル越し"
  expect_block "timeout 60 make down"
  expect_block "env KUBECONFIG=/tmp/k make import"
  expect_block "nohup k3d cluster delete pokecalc"
  expect_block "FOO=bar make migrate-down"
  expect_block "timeout 30 env A=1 nohup git push origin main"
  expect_block "make test && make down"
  expect_block "echo ok; git push origin main"
  expect_block "go test ./... || gh pr merge 1"
  expect_block "cd /tmp && kubectl delete ns pokecalc"
  expect_block "bash -c 'make down'"
  expect_block $'make test\nmake down'
}

test_block_git_push_destination_edge_cases() {
  begin "block: git push 宛先判定の抜け(critic指摘。リダイレクト・バックグラウンド・クォート・複数refspec・宛先省略)"
  expect_block "git push origin main 2>&1"
  expect_block "git push origin main > /dev/null"
  expect_block "git push origin main &"
  expect_block 'git push origin "main"'
  expect_block "git push origin feat/x:main feat/y"
  # "git push"(宛先省略)・"git push origin HEAD" は現在のブランチに解決する契約:
  # 現在のブランチが main の一時リポジトリで block されることを確かめる。
  expect_block_in "$MAIN_REPO" "git push"
  expect_block_in "$MAIN_REPO" "git push origin HEAD"
}

test_block_command_splitting_edge_cases() {
  begin "block: 断片分割の抜け(critic指摘。bash/sh/zsh -c・サブシェル・eval・env -i・timeout の各形・絶対パス)"
  expect_block 'bash -c "make test && make down"'
  expect_block 'sh -c "git push origin main"'
  expect_block 'bash -lc "make down"'
  expect_block 'zsh -c "make down"'
  expect_block "(make down)"
  expect_block "{ git push origin main; }"
  expect_block "if true; then make down; fi"
  expect_block 'eval "make down"'
  expect_block "env -i make down"
  expect_block "timeout -s KILL 60 make down"
  expect_block "timeout 1m make down"
  expect_block "/usr/bin/git push origin main"
}

test_block_kubectl_edge_cases() {
  begin "block: kubectl 判定の抜け(critic指摘。複数形statefulsets・APIグループ接尾辞・値ありフラグの位置・カンマ区切り)"
  expect_block "kubectl delete statefulsets mysql"
  expect_block "kubectl delete statefulsets.apps mysql"
  expect_block "kubectl -v 6 delete pvc x"
  expect_block "kubectl delete --grace-period 0 pvc x"
  expect_block "kubectl delete --cascade orphan pvc x"
  expect_block "kubectl get pods,secrets"
  expect_block "kubectl get secrets.v1 x"
}

test_block_gh_edge_cases() {
  begin "block: gh 判定の抜け(critic指摘。-R/--repo でのマージ回避・gh api での /merge 直叩き)"
  expect_block "gh -R owner/repo pr merge 1"
  expect_block "gh api -X PUT repos/owner/repo/pulls/1/merge"
}

test_block_secret_path_edge_cases() {
  begin "block: .ssh 判定の抜け(critic指摘。~/ を要求せず \$HOME・絶対パスでも検出)"
  expect_block 'cat $HOME/.ssh/id_rsa'
  expect_block "cat /Users/example/.ssh/id_rsa"
}

test_block_k3d_edge_cases() {
  begin "block: k3d 判定の抜け(critic指摘。cluster の前にグローバルフラグが挟まる形)"
  expect_block "k3d --verbose cluster delete pokecalc"
}

# critic 3回目レビュー(最終回)で新たに見つかった、これまでの修正では塞がっていなかった穴。
# 主因は同じ: flatten_metachars がコマンドの区切り(&&・;・|等)を単なる空白に潰していたため、
# (1) git push の宛先候補ループが remote 自体("origin")も「宛先あり」と数えてしまい、
#     宛先省略(refspec無し)の場合に現在のブランチを確認しないまま通ってしまう、
# (2) kubectl の対象候補収集・delete の -k/-f 判定が、区切りの先にある別コマンドのトークンまで
#     拾ってしまい、逆に「対象不明なら安全側でblock」という安全網を素通りしてしまう、という2点。
# 区切りを番兵トークン "__SEP__" に変え、各引数収集ループがそこで止まるようにして解消した。
test_block_git_push_remote_only_edge_cases() {
  begin "block: git push が remote 名だけ(宛先省略)のとき、区切りの前後に関わらず現在のブランチを見る(critic 3回目・最終指摘)"
  expect_block_in "$MAIN_REPO" "git push origin"
  expect_block_in "$MAIN_REPO" "git push -u origin"
  expect_block_in "$MAIN_REPO" "git push && echo done"
  expect_block_in "$MAIN_REPO" "git push 2>&1 | tail -3"
  expect_block_in "$MAIN_REPO" "git push -u origin && gh pr create --title x --body y"
  expect_block "git -C $OTHER_REPO_MAIN push origin"
  expect_block "cd $OTHER_REPO_MAIN && git push origin"
}

test_pass_git_push_remote_only_non_main() {
  begin "pass: git push が remote 名だけでも、現在のブランチが main でなければ通す(誤検知しないこと)"
  expect_pass_in "$OTHER_REPO_FEATURE" "git push origin"
  expect_pass_in "$OTHER_REPO_FEATURE" "git push -u origin"
  expect_pass_in "$OTHER_REPO_FEATURE" "git push && echo done"
}

test_block_kubectl_separator_boundary_edge_cases() {
  begin "block: kubectl の対象収集・-k/-f 判定が区切りの先の別コマンドまで読まない(critic 3回目・最終指摘)"
  expect_block "kubectl get ns -o name | xargs kubectl delete | tail -3"
  expect_block "kubectl get pvc -o name | xargs kubectl delete && echo ok"
  expect_block "kubectl delete --kustomize=deploy/k8s/overlays/local && echo ok"
  expect_block "kubectl delete -k=deploy/k8s/overlays/local; echo ok"
  expect_block "kubectl delete -f=deploy.yaml; echo ok"
}

test_block_critic4_edge_cases() {
  begin "block: critic 4回目指摘(値を取るpushフラグ・短フラグまとめ・クォート分割・動的な宛先)"
  expect_block_in "$MAIN_REPO" "git push -o ci.skip origin"
  expect_block_in "$MAIN_REPO" "git push --push-option x origin"
  expect_block_in "$MAIN_REPO" "git push --receive-pack x origin"
  expect_block "kubectl delete -Rf deploy/k8s/base"
  expect_block "kubectl delete -fR deploy/k8s/base"
  expect_block "kubectl delete -fdeploy.yaml pod x"
  expect_block "kubectl delete -kdir"
  expect_block 'gi""t push origin main'
  expect_block 'git push origin mai""n'
  expect_block 'git push origin ma\in'
  expect_block 'B=main; git push origin $B'
}

test_pass_critic4_separator_no_false_positive() {
  begin "pass: 区切りの前後で誤検知しない(critic 4回目指摘7)"
  expect_pass "kubectl delete pod x; echo ok"
  expect_pass "kubectl delete pod x && echo ok"
  expect_pass_in "$OTHER_REPO_FEATURE" "git push origin feature && echo ok"
  expect_pass_in "$OTHER_REPO_FEATURE" "git push -o ci.skip origin feature"
  expect_pass_in "$OTHER_REPO_FEATURE" "git push -o ci.skip origin"
}

test_block_critic5_edge_cases() {
  begin "block: critic 5回目指摘(行継続・heads/main・--recurse-submodules・--exec/--repo)"
  expect_block_in "$MAIN_REPO" $'git push origin \\\nmain'
  expect_block_in "$MAIN_REPO" $'git push \\\n origin main'
  expect_block $'git \\\npush origin main'
  expect_block $'gh pr \\\nmerge 1'
  expect_block "git push origin HEAD:heads/main"
  expect_block "git push origin feature:heads/main"
  expect_block_in "$MAIN_REPO" "git push --recurse-submodules check origin"
  expect_block_in "$MAIN_REPO" "git push --exec x origin"
  expect_block_in "$MAIN_REPO" "git push --repo x origin"
}

test_pass_critic5_no_false_positive() {
  begin "pass: critic 5回目の修正が誤検知しない"
  expect_pass_in "$OTHER_REPO_FEATURE" $'git push origin \\\nfeature'
  expect_pass_in "$OTHER_REPO_FEATURE" "git push --recurse-submodules check origin"
  expect_pass_in "$OTHER_REPO_FEATURE" "git push origin HEAD:heads/feature"
}

test_block_kubectl_delete_indirect_and_dynamic() {
  begin "block: kubectl delete -f/-R(ファイル指定・再帰)・all・パイプ/xargs越し・コマンド置換(critic 3回目指摘)"
  expect_block "kubectl delete -f deploy/k8s/base"
  expect_block "kubectl delete -f manifest.yaml"
  expect_block "kubectl delete -R deploy/k8s/base"
  expect_block "kubectl delete all --all -n pokecalc"
  expect_block "kubectl get pvc -o name | xargs kubectl delete"
  expect_block "xargs -r kubectl delete -n pokecalc"
  expect_block 'kubectl delete $(kubectl get pvc -o name)'
}

test_block_kubectl_verb_flag_value_confusion() {
  begin "block: kubectl の動詞判定が値ありフラグの値を誤って動詞と扱わない(critic 3回目指摘)"
  expect_block "kubectl delete --timeout 60s pvc x"
  expect_block "kubectl --request-timeout 30s delete pvc x"
  expect_block "kubectl --cluster k3d-pokecalc delete pvc x"
  expect_block "kubectl --user admin delete ns pokecalc"
}

test_block_secret_read_before_redirect_strip() {
  begin "block: リダイレクト除去(前)の生文字列でも .env/.ssh を検出する(critic 3回目指摘)"
  expect_block "cat < .env"
  expect_block "grep KEY < .env"
  expect_block 'cat <~/.ssh/id_rsa'
}

test_block_git_push_other_checkout() {
  begin "block: git push の宛先解決が -C <dir>・cd <dir> && を尊重する(critic 3回目指摘)"
  expect_block "git -C $OTHER_REPO_MAIN push"
  expect_block "cd $OTHER_REPO_MAIN && git push"
}

test_block_command_name_case_and_escape() {
  begin "block: コマンド名の大文字小文字・エイリアス無効化バックスラッシュを無視する(critic 3回目指摘)"
  expect_block "GIT push origin main"
  expect_block '\git push origin main'
}

test_block_gh_api_merge_variants() {
  begin "block: gh api の /merges・GraphQL mergePullRequest(critic 3回目指摘)"
  expect_block "gh api repos/o/r/merges -f base=main -f head=feat/x"
  expect_block "gh api graphql -f query='mutation { mergePullRequest(...) }'"
}

# ---- block してはいけないもの(誤検知させない) ----

test_pass_daily_commands() {
  begin "pass: 日常のビルド・テスト・起動"
  expect_pass "make test"
  expect_pass "make lint"
  expect_pass "make build"
  expect_pass "make dev"
  expect_pass "make doctor"
  expect_pass "make up"
  expect_pass "make gen"
  expect_pass "make test-scripts"
  expect_pass "go test ./..."
  expect_pass "gofmt -l ."
}

test_pass_import_readonly() {
  begin "pass: DB に触らない import 系ターゲット"
  expect_pass "make import-dry-run"
  expect_pass "make import-fetch"
  expect_pass "make import-check-upstream"
}

test_pass_kubectl_readonly_and_pod_delete() {
  begin "pass: kubectl の参照系と pod/job の削除(データ削除ではない)"
  expect_pass "kubectl get pods"
  expect_pass "kubectl -n pokecalc get pods -o wide"
  expect_pass "kubectl get all -n pokecalc"
  expect_pass "kubectl describe pod x"
  expect_pass "kubectl logs x"
  expect_pass "kubectl delete pod x"
  expect_pass "kubectl -n pokecalc delete job pokedex-import-manual"
  expect_pass "kubectl apply -k deploy/k8s/overlays/local"
  expect_pass "k3d cluster list"
}

test_pass_git_non_main() {
  begin "pass: main 以外への git push・main を含むが push でないコマンド"
  expect_pass "git push origin feat/some-branch"
  expect_pass "git push -u origin feat/x"
  expect_pass "git push origin HEAD:feat/x"
  expect_pass "git push origin main-fix"
  expect_pass "git push origin feat/main-page"
  expect_pass "git push origin maintenance"
  expect_pass "git log --oneline main..HEAD"
  expect_pass "git diff main"
  expect_pass "git checkout main"
  expect_pass "git pull origin main"
  expect_pass "git fetch origin main"
  expect_pass "git status"
}

test_pass_gh_other() {
  begin "pass: gh pr の merge 以外"
  expect_pass "gh pr create --title x --body y --base main"
  expect_pass "gh pr view 123"
  expect_pass "gh pr list"
}

test_pass_env_substring() {
  begin "pass: .env を含むが .env ファイルではない(単語境界)"
  expect_pass "node -e 'console.log(process.env.HOME)'"
  expect_pass "go env GOPATH"
  expect_pass "cat .env.example"
  expect_pass "cp .env.example /tmp/preview.txt"
}

test_pass_wrappers() {
  begin "pass: ラッパーを剥がした後の正常系"
  expect_pass "timeout 60 make test"
  expect_pass "env X=Y go test ./..."
  expect_pass "nohup make dev"
  expect_pass "FOO=bar make lint"
}

test_pass_edge_cases_no_false_positive() {
  begin "pass: critic指摘の抜け修正が誤検知しないこと(リダイレクト付きの非main push・kubectl getのカンマ区切り)"
  expect_pass "git push origin feat/x 2>&1"
  expect_pass "git push origin feat/x > /dev/null"
  expect_pass "kubectl get pods,configmaps"
}

test_pass_edge_cases_no_false_positive_v2() {
  begin "pass: critic 3回目指摘の修正が誤検知しないこと"
  expect_pass "kubectl delete pod x"
  expect_pass "kubectl get pods -o name"
  expect_pass "git -C $OTHER_REPO_FEATURE push"
}

test_pass_non_command_input() {
  begin "pass: command が無い・空の入力(Bash 以外のツール等)は何もしない"
  printf '%s' '{"tool_name":"Read","tool_input":{"file_path":"README.md"}}' | bash "$GUARD" >"$WORK/out" 2>"$WORK/err"
  rc=$?
  if [ "$rc" = 0 ]; then ok; else ng "command が無い入力で exit $rc"; fi
  expect_pass ""
}

# ---- 設定ファイルの静的検査 ----

test_claude_settings() {
  begin ".claude/settings.json: PreToolUse(Bash)が bash-guard.sh を呼び、広すぎる allow が無い"
  if jq -e . "$CLAUDE_SETTINGS" >/dev/null 2>&1; then ok; else ng "JSON として読めない"; return; fi

  if [ "$(jq '[.hooks.PreToolUse[]? | select(.matcher == "Bash") | .hooks[]? | select(.type == "command" and (.command | test("scripts/ai-guard/bash-guard\\.sh")))] | length' "$CLAUDE_SETTINGS")" -ge 1 ]; then
    ok
  else
    ng "hooks.PreToolUse に matcher \"Bash\" で scripts/ai-guard/bash-guard.sh を呼ぶ command フックが無い"
  fi

  if [ "$(jq '[.hooks.PostToolUse[]? | select(.matcher == "Edit|Write") | .hooks[]? | select(.command | test("gofmt"))] | length' "$CLAUDE_SETTINGS")" -ge 1 ]; then
    ok
  else
    ng "既存の PostToolUse(gofmt フック)が消えている"
  fi

  local broad
  for broad in 'Bash(make *)' 'Bash(kubectl *)' 'Bash(k3d *)' 'Bash(docker *)' 'Bash(git push *)' 'Bash(gh pr merge *)'; do
    if jq -e --arg p "$broad" '.permissions.allow // [] | index($p) == null' "$CLAUDE_SETTINGS" >/dev/null; then
      ok
    else
      ng "permissions.allow に広すぎる $broad が残っている"
    fi
  done

  # gh pr merge は allow のどんな形でも許可しない(block 対象なので allow に置く意味が無く、誤解を招く)。
  if jq -e '[.permissions.allow // [] | .[] | select(test("^Bash\\(gh pr merge"))] | length == 0' "$CLAUDE_SETTINGS" >/dev/null; then
    ok
  else
    ng "permissions.allow に gh pr merge の許可が残っている"
  fi
}

test_never_executes_the_judged_command() {
  begin "bash-guard.sh は判定対象のコマンドを一切実行しない(文字列判定だけ。データレーンの指摘)"
  local marker="$WORK/should-not-exist"
  rm -f "$marker"
  # block されるはずのコマンドに「実行されたら分かる」副作用(marker 作成)を仕込む。
  # bash-guard.sh がこれを eval 等で実際に走らせていれば marker ができてしまう。
  run_guard "make down && touch $marker"
  if [ "$GUARD_RC" = 2 ]; then ok; else ng "block されるべき(exit 2)が exit $GUARD_RC"; fi
  if [ ! -e "$marker" ]; then ok; else ng "判定対象のコマンドが実際に実行された(marker が作られた): $marker"; fi
}

test_codex_config() {
  begin ".codex/config.toml: PreToolUse(^Bash$)が bash-guard.sh を呼び、承認方針を明示する"
  if [ -f "$CODEX_CONFIG" ]; then ok; else ng ".codex/config.toml が無い"; return; fi
  if grep -qE '^[[:space:]]*\[\[hooks\.PreToolUse\]\]' "$CODEX_CONFIG"; then ok; else ng "[[hooks.PreToolUse]] が無い"; fi
  if grep -qE '^[[:space:]]*matcher[[:space:]]*=[[:space:]]*"\^Bash\$"' "$CODEX_CONFIG"; then ok; else ng 'matcher = "^Bash$" が無い'; fi
  if grep -q 'scripts/ai-guard/bash-guard\.sh' "$CODEX_CONFIG"; then ok; else ng "bash-guard.sh を呼んでいない"; fi
  if grep -qE '^[[:space:]]*approval_policy[[:space:]]*=[[:space:]]*"on-request"' "$CODEX_CONFIG"; then ok; else ng 'approval_policy = "on-request" が無い'; fi
  if grep -qE '^[[:space:]]*sandbox_mode[[:space:]]*=[[:space:]]*"workspace-write"' "$CODEX_CONFIG"; then ok; else ng 'sandbox_mode = "workspace-write" が無い'; fi
  if grep -qE '^[[:space:]]*\[agents\]' "$CODEX_CONFIG"; then ok; else ng "既存の [agents] セクションが消えている"; fi
  if grep -qE '^[[:space:]]*max_concurrent_threads_per_session[[:space:]]*=[[:space:]]*2' "$CODEX_CONFIG"; then ok; else ng "[agents] の既存設定が変わっている"; fi
}

test_prerequisites
test_block_cluster_delete
test_block_kubectl_data_delete
test_block_kubectl_delete_kustomize
test_block_migrate_down
test_block_import
test_block_secret_read
test_block_push_to_main
test_block_force_push
test_block_pr_merge
test_pr_merge_gate_checks_what_it_merges
test_pr_merge_gate_without_gh
test_allow_pr_merge_gate_script
test_block_wrappers_and_compound
test_block_git_push_destination_edge_cases
test_block_command_splitting_edge_cases
test_block_kubectl_edge_cases
test_block_gh_edge_cases
test_block_secret_path_edge_cases
test_block_k3d_edge_cases
test_block_kubectl_delete_indirect_and_dynamic
test_block_kubectl_verb_flag_value_confusion
test_block_secret_read_before_redirect_strip
test_block_git_push_other_checkout
test_block_command_name_case_and_escape
test_block_gh_api_merge_variants
test_block_git_push_remote_only_edge_cases
test_pass_git_push_remote_only_non_main
test_block_kubectl_separator_boundary_edge_cases
test_block_critic4_edge_cases
test_pass_critic4_separator_no_false_positive
test_block_critic5_edge_cases
test_pass_critic5_no_false_positive
test_pass_daily_commands
test_pass_import_readonly
test_pass_kubectl_readonly_and_pod_delete
test_pass_git_non_main
test_pass_gh_other
test_pass_env_substring
test_pass_wrappers
test_pass_edge_cases_no_false_positive
test_pass_edge_cases_no_false_positive_v2
test_pass_non_command_input
test_never_executes_the_judged_command
test_pr_merge_never_executed_by_guard
test_claude_settings
test_codex_config

if [ "$FAILURES" -gt 0 ]; then
  printf 'bash-guard test: %d failed, %d passed\n' "$FAILURES" "$PASSES" >&2
  exit 1
fi
printf 'bash-guard test: all %d checks passed\n' "$PASSES"
