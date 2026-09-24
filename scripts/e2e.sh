#!/usr/bin/env bash
# ルートの `make e2e`(ADR-0306。issue #72)。
#
# 常に実行する3件(k3d クラスタ不要。この順で必ず実行する):
#   make web-e2e / web-e2e-online / web-e2e-balance
# kubectl の現在のコンテキストが k3d-$CLUSTER のときだけ、追加で3件を実行する:
#   make api-smoke / web-k3d-smoke / web-k3d-e2e
# 一致しなければ、飛ばしたことと飛ばしたターゲット名・用意の仕方(make up)を明示してスキップする。
# E2E_REQUIRE_K3D=1 のときはスキップせず、コンテキストが一致しなければ非0で終わる(リリース前の確認用)。
#
# 呼び出しは必ず `make <ターゲット>` 経由にする。Playwright の起動方法の正は web/Makefile・web/package.json に置き、
# ここへは写経しない。クラスタは作らない・消さない・変えない(kubectl は config current-context の読み取りだけ)。
# 自動テスト: scripts/e2e_test.sh(make と kubectl を偽物に差し替えて流す。make test-scripts に含む)。
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

readonly CLUSTER="${CLUSTER:-pokecalc}"
readonly MAKE_BIN="${MAKE:-make}"

# 常に実行する(k3d クラスタ不要)E2E。web/Makefile の実装済みターゲット。
readonly BASE_TARGETS="web-e2e web-e2e-online web-e2e-balance"
# k3d クラスタがあるときだけ追加で実行するスモーク。
readonly K3D_TARGETS="api-smoke web-k3d-smoke web-k3d-e2e"

# run_target ターゲット名 — `make <ターゲット>` を実行する。失敗したらターゲット名を出して非0で終わり、
# 以降のターゲットは実行しない(呼び出し元の set -e に頼らず、失敗時のメッセージを必ず出すため)。
run_target() {
  local target="$1" rc=0
  echo "e2e: make ${target} を実行します" >&2
  "$MAKE_BIN" "$target" || rc=$?
  if [ "$rc" -ne 0 ]; then
    echo "e2e: make ${target} が失敗しました(終了コード ${rc})。以降のターゲットは実行しません" >&2
    exit "$rc"
  fi
}

# current_kubectl_context — kubectl が無い・コンテキスト未設定なら非0で終わる(値は出さない)。
current_kubectl_context() {
  command -v kubectl >/dev/null 2>&1 || return 1
  kubectl config current-context 2>/dev/null || return 1
}

# has_matching_k3d_context — 現在のコンテキストが k3d-$CLUSTER と一致するか。
has_matching_k3d_context() {
  local ctx
  ctx=$(current_kubectl_context) || return 1
  [ "$ctx" = "k3d-${CLUSTER}" ]
}

# skip_k3d_targets — k3d 依存の3件を飛ばしたことと、飛ばしたターゲット名・用意の仕方を明示する。
skip_k3d_targets() {
  echo "e2e: kubectl のコンテキストが k3d-${CLUSTER} ではないため、k3d 依存の3件をスキップします: ${K3D_TARGETS}" >&2
  echo "e2e: 実行するには先に k3d クラスタを用意してください(make up)。用意できたら make e2e を再実行してください" >&2
}

echo "e2e: 常時実行の3件(k3d クラスタ不要)" >&2
for target in $BASE_TARGETS; do
  run_target "$target"
done

if has_matching_k3d_context; then
  echo "e2e: kubectl のコンテキストが k3d-${CLUSTER}。k3d 依存の3件も実行します" >&2
  for target in $K3D_TARGETS; do
    run_target "$target"
  done
elif [ "${E2E_REQUIRE_K3D:-}" = "1" ]; then
  echo "e2e: E2E_REQUIRE_K3D=1 が指定されていますが、kubectl のコンテキストが k3d-${CLUSTER} ではありません。" \
    "k3d 依存の3件(${K3D_TARGETS})を実行できないため終了します(先に make up を実行してください)" >&2
  exit 1
else
  skip_k3d_targets
fi

echo "e2e: 完了しました" >&2
