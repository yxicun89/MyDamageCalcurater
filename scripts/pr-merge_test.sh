#!/usr/bin/env bash
# scripts/pr-merge.sh の自動テスト(ADR-0804)。`make test-scripts` から流す。
# 偽の gh・make・npm を PATH の先頭に置き、使い捨ての git リポジトリ(origin + clone)で、
# ゲート(PR の状態・checks・保護ファイル・ローカル検証・マージ)が期待どおり働くことを確かめる。
# 実際の GitHub・実際の make には一切触れない。
set -uo pipefail

SRC=$(cd "$(dirname "$0")/.." && pwd)
readonly SRC
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

# 使い捨てリポジトリ: origin(bare)と clone。clone に scripts/pr-merge.sh を置く。
git init -q --bare "$WORK/origin.git"
git clone -q "$WORK/origin.git" "$WORK/repo" 2>/dev/null
mkdir -p "$WORK/repo/scripts"
cp "$SRC/scripts/pr-merge.sh" "$WORK/repo/scripts/pr-merge.sh"
(
  cd "$WORK/repo" || exit 1
  git -c user.email=t@example.com -c user.name=t checkout -q -b main
  git add -A
  git -c user.email=t@example.com -c user.name=t commit -q -m init
  git push -q origin main
  git push -q origin HEAD:refs/pull/5/head
)
SHA=$(git -C "$WORK/repo" rev-parse HEAD)

# 偽コマンド。振る舞いは環境変数で切り替える。呼ばれたコマンドは $WORK/calls に追記する。
BIN="$WORK/bin"
mkdir -p "$BIN"
cat >"$BIN/gh" <<'EOF'
#!/usr/bin/env bash
echo "gh $*" >>"$FAKE_CALLS"
case "$1 $2" in
  "pr view")
    printf '{"state":"%s","isDraft":%s,"mergeable":"%s","headRefOid":"%s","baseRefName":"main","headRefName":"x"}\n' \
      "${FAKE_STATE:-OPEN}" "${FAKE_DRAFT:-false}" "${FAKE_MERGEABLE:-MERGEABLE}" "$FAKE_SHA" ;;
  "pr checks") printf '%s\n' "${FAKE_CHECKS:-[]}" ;;
  "pr diff") printf '%s\n' "${FAKE_FILES:-docs/a.md}" ;;
  "pr merge") exit 0 ;;
esac
EOF
cat >"$BIN/make" <<'EOF'
#!/usr/bin/env bash
echo "make $*" >>"$FAKE_CALLS"
if [ "$1" = "${FAKE_MAKE_FAIL:-}" ]; then exit 1; fi
EOF
cat >"$BIN/npm" <<'EOF'
#!/usr/bin/env bash
echo "npm $*" >>"$FAKE_CALLS"
EOF
chmod +x "$BIN"/*

# run_gate 引数... — 環境変数は呼び出し側で設定。結果は GATE_RC・$WORK/out・$WORK/calls。
run_gate() {
  : >"$WORK/calls"
  PATH="$BIN:$PATH" FAKE_CALLS="$WORK/calls" FAKE_SHA="${FAKE_SHA_FORCE:-$SHA}" \
    "$WORK/repo/scripts/pr-merge.sh" "$@" >"$WORK/out" 2>&1
  GATE_RC=$?
}
merged() { grep -q '^gh pr merge' "$WORK/calls"; }
expect_stop() { # 理由の部分文字列
  if [ "$GATE_RC" -ne 0 ]; then ok; else ng "中止(非0)になるべきが exit 0: $1"; fi
  if ! merged; then ok; else ng "マージしてはいけない: $1"; fi
  if grep -q "$1" "$WORK/out"; then ok; else ng "理由に「$1」が出ていない: $(cat "$WORK/out")"; fi
}

begin "引数: 番号なし・数字以外・余計な引数は使い方を出して exit 2"
run_gate
[ "$GATE_RC" = 2 ] && ok || ng "番号なしは exit 2 のはず($GATE_RC)"
run_gate abc
[ "$GATE_RC" = 2 ] && ok || ng "数字以外は exit 2 のはず($GATE_RC)"
run_gate 5 --force
[ "$GATE_RC" = 2 ] && ok || ng "未知のオプションは exit 2 のはず($GATE_RC)"

begin "全部通る(checks なし): ローカル検証をしてマージする"
unset FAKE_STATE FAKE_DRAFT FAKE_MERGEABLE FAKE_CHECKS FAKE_FILES FAKE_MAKE_FAIL
run_gate 5
[ "$GATE_RC" = 0 ] && ok || ng "exit 0 のはずが $GATE_RC: $(cat "$WORK/out")"
for t in "make lint" "make check-publishable" "make test"; do
  grep -q "^$t\$" "$WORK/calls" && ok || ng "$t を実行していない"
done
grep -q "^gh pr merge 5 --merge --match-head-commit $SHA\$" "$WORK/calls" && ok || ng "検証した SHA を固定してマージしていない: $(grep 'pr merge' "$WORK/calls")"
grep -q -- '--admin' "$WORK/calls" && ng "--admin を付けてはいけない" || ok

begin "--check はゲートだけでマージしない"
run_gate 5 --check
[ "$GATE_RC" = 0 ] && ok || ng "exit 0 のはず"
merged && ng "--check でマージした" || ok

begin "PR の状態: CLOSED・ドラフト・コンフリクトは中止"
FAKE_STATE=CLOSED run_gate 5
expect_stop "OPEN ではありません"
FAKE_DRAFT=true run_gate 5
expect_stop "ドラフト"
FAKE_MERGEABLE=CONFLICTING run_gate 5
expect_stop "コンフリクト"

begin "checks: 失敗・実行中があれば中止、全部成功ならローカル検証へ進んでマージ"
FAKE_CHECKS='[{"name":"go","bucket":"fail"}]' run_gate 5
expect_stop "go=fail"
FAKE_CHECKS='[{"name":"go","bucket":"pending"}]' run_gate 5
expect_stop "go=pending"
FAKE_CHECKS='[{"name":"go","bucket":"pass"},{"name":"x","bucket":"skipping"}]' run_gate 5
[ "$GATE_RC" = 0 ] && merged && ok || ng "全成功ならマージするはず: $(cat "$WORK/out")"

begin "保護ファイル: 権限・ガード・ゲート自身・クラウド・ワークフローを変える PR は人間がマージする"
for f in .claude/settings.json .codex/config.toml scripts/ai-guard/bash-guard.sh scripts/pr-merge.sh scripts/pr-merge_test.sh \
  deploy/k8s/overlays/cloud/kustomization.yaml terraform/main.tf .github/workflows/ci.yml; do
  FAKE_FILES=$'docs/a.md\n'"$f" run_gate 5
  expect_stop "人間がマージ"
done

begin "ローカル検証: lint・check-publishable・test のどれかが失敗したらマージしない"
for t in lint check-publishable test; do
  FAKE_MAKE_FAIL=$t run_gate 5
  expect_stop "ローカル検証に失敗"
done

begin "engine / golden を変える PR だけ make test-golden も流す"
FAKE_FILES=engine/x.go run_gate 5
grep -q '^make test-golden$' "$WORK/calls" && ok || ng "engine 変更で test-golden を流していない"
FAKE_FILES=docs/a.md run_gate 5
grep -q '^make test-golden$' "$WORK/calls" && ng "docs だけで test-golden を流した" || ok

begin "PR の先頭が取得した SHA と違えば中止"
FAKE_SHA_FORCE=0000000000000000000000000000000000000001 run_gate 5
expect_stop "一致しません"

printf '\n'
if [ "$FAILURES" -eq 0 ]; then
  printf 'pr-merge test: all %s checks passed\n' "$PASSES"
else
  printf 'pr-merge test: %s failed, %s passed\n' "$FAILURES" "$PASSES" >&2
  exit 1
fi
