#!/usr/bin/env bash
# calc・balance・speed が、いま export した read model(metadata.json)と同じ dataVersion で動いているかを確かめる
# (issue #281・#108、ADR-0135)。リポジトリのルートで実行する。クラスタの変更はしない(読み取りだけ)。
#   - 期待値: $READMODEL_DIR/metadata.json の dataVersion(既定 data/generated/readmodel。make pokedex-export が書く)
#   - calc: GET /readyz の dataVersion(calc-svc が読み込み済みの版)
#   - balance / speed: Deployment の注釈 pokecalc.example/data-version(scripts/gitops/k3d-deploy-readmodel.sh が配備時に付ける)
# 1つでも違う・取れないときは、どの consumer が旧版かを表示して終了コード 1。秘密は出さない(公開データの版だけ)。
set -euo pipefail
command -v jq >/dev/null 2>&1 || { echo "check-master-version: jq が必要です(make doctor で確認)" >&2; exit 2; }

cluster="${CLUSTER:-pokecalc}"
namespace="${NAMESPACE:-pokecalc}"
readmodel_dir="${READMODEL_DIR:-data/generated/readmodel}"

CLUSTER="$cluster" ./scripts/require-k3d-context.sh check-master-version

metadata="$readmodel_dir/metadata.json"
[ -f "$metadata" ] || { echo "check-master-version: $metadata が無い(先に make pokedex-export)" >&2; exit 1; }
expected=$(jq -r '.dataVersion // empty' "$metadata")
[ -n "$expected" ] || { echo "check-master-version: $metadata に dataVersion が無い" >&2; exit 1; }

context="k3d-${cluster}"
actual_calc=$(kubectl --context "$context" get --raw "/api/v1/namespaces/${namespace}/services/calc:http/proxy/readyz" 2>/dev/null | jq -r '.dataVersion // empty' || true)
actual_balance=$(kubectl --context "$context" -n "$namespace" get deployment/balance -o json 2>/dev/null | jq -r '.metadata.annotations["pokecalc.example/data-version"] // empty' || true)
actual_speed=$(kubectl --context "$context" -n "$namespace" get deployment/speed -o json 2>/dev/null | jq -r '.metadata.annotations["pokecalc.example/data-version"] // empty' || true)

echo "期待する dataVersion: $expected"
stale=()
for pair in "calc:$actual_calc" "balance:$actual_balance" "speed:$actual_speed"; do
  name=${pair%%:*}
  actual=${pair#*:}
  if [ "$actual" = "$expected" ]; then
    echo "  ok    $name: $actual"
  else
    echo "  STALE $name: ${actual:-(取得できない)}"
    stale+=("$name")
  fi
done

if [ "${#stale[@]}" -gt 0 ]; then
  echo "check-master-version: 旧版または未確認の consumer: ${stale[*]}(docs/runbooks/data.md「5a. 投入後に calc・balance・speed へ反映し、動いている版を確かめる」)" >&2
  exit 1
fi
echo "check-master-version: calc・balance・speed の版が一致"
