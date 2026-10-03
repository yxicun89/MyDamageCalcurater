#!/usr/bin/env bash
# scripts/up.sh・scripts/k3d-m2-deploy.sh が Secret を作る経路の静的な検査(issue #327)。make test-scripts から流す。
# `kubectl apply` は入力(stringData の平文)を last-applied-configuration 注釈に写すため、値を持つ Secret は
# `kubectl create`(--save-config なし)で作り、既存の Secret には patch で無いキーだけを足す。
# up.sh は k3d・Docker・クラスタを使うため、偽 kubectl で流す代わりに文面を確かめる(issue #327 の回帰テスト案の代替)。
set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
failures=0
ok() { echo "ok: $1"; }
ng() { echo "NG: $1" >&2; failures=$((failures + 1)); }

# check_file <スクリプト> <マニフェスト変数名...>: コメント行は検査しない(「--save-config なし」のような説明文で誤検出しないため)。
check_file() {
  local src="$1" name manifest
  shift
  name=$(basename "$src")
  local stripped
  stripped=$(mktemp)
  grep -vE '^[[:space:]]*#' "$src" > "$stripped"
  for manifest in "$@"; do
    if grep -qE "kubectl[[:space:]]+create[[:space:]]+-f[[:space:]]+\"\\\$${manifest}\"" "$stripped"; then
      ok "${name}: ${manifest} は kubectl create -f で作る"
    else
      ng "${name}: ${manifest} を kubectl create -f で作っていない"
    fi
    if grep -qE "kubectl[^#]*[[:space:]]apply[^#]*${manifest}" "$stripped"; then
      ng "${name}: ${manifest} を kubectl apply している(平文が last-applied-configuration 注釈に残る)"
    else
      ok "${name}: ${manifest} を kubectl apply しない"
    fi
  done
  if grep -qE -- '--save-config' "$stripped"; then
    ng "${name} が --save-config を使っている(注釈に入力が写る)"
  else
    ok "${name} は --save-config を使わない"
  fi
  if grep -qE 'create[[:space:]]+secret.*--from-literal' "$stripped"; then
    ng "${name} が --from-literal で値をコマンドラインに出している"
  else
    ok "${name}: Secret の値をコマンドライン引数に出さない"
  fi
  rm -f "$stripped"
}

# tidb-root-auth・record-db-auth・team-db-auth は k3d-m2-deploy.sh(ADR-0226)、mysql-auth は up.sh が作る。
check_file "$ROOT/scripts/up.sh" mysql_auth_manifest
check_file "$ROOT/scripts/k3d-m2-deploy.sh" manifest

if [ "$failures" -ne 0 ]; then
  echo "up-secrets_test: ${failures} 件失敗" >&2
  exit 1
fi
echo "up-secrets_test: すべて成功"
