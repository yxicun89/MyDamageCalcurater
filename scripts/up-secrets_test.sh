#!/usr/bin/env bash
# scripts/up.sh が Secret を作る経路の静的な検査(issue #327)。make test-scripts から流す。
# `kubectl apply` は入力(stringData の平文)を last-applied-configuration 注釈に写すため、値を持つ Secret は
# `kubectl create`(--save-config なし)で作り、既存の Secret には patch で無いキーだけを足す。
# up.sh はクラスタ・Docker を使うので実行せず、文面を確かめる。
set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
readonly UP_SRC="$ROOT/scripts/up.sh"

failures=0
# コメント行は検査しない(「--save-config なし」のような説明文で誤検出しないため)。
UP=$(mktemp)
trap 'rm -f "$UP"' EXIT
grep -vE '^[[:space:]]*#' "$UP_SRC" > "$UP"
ok() { echo "ok: $1"; }
ng() { echo "NG: $1" >&2; failures=$((failures + 1)); }

for manifest in tidb_root_auth_manifest mysql_auth_manifest; do
  if grep -qE "kubectl[[:space:]]+create[[:space:]]+-f[[:space:]]+\"\\\$${manifest}\"" "$UP"; then
    ok "${manifest} は kubectl create -f で作る"
  else
    ng "${manifest} を kubectl create -f で作っていない"
  fi
  if grep -qE "kubectl[[:space:]]+apply[^#]*${manifest}" "$UP"; then
    ng "${manifest} を kubectl apply している(平文が last-applied-configuration 注釈に残る)"
  else
    ok "${manifest} を kubectl apply しない"
  fi
done

if grep -qE -- '--save-config' "$UP"; then
  ng "up.sh が --save-config を使っている(注釈に入力が写る)"
else
  ok "up.sh は --save-config を使わない"
fi

if grep -qE 'create[[:space:]]+secret[^\n]*--from-literal' "$UP"; then
  ng "up.sh が --from-literal で値をコマンドラインに出している"
else
  ok "Secret の値をコマンドライン引数に出さない"
fi

if [ "$failures" -ne 0 ]; then
  echo "up-secrets_test: ${failures} 件失敗" >&2
  exit 1
fi
echo "up-secrets_test: すべて成功"
