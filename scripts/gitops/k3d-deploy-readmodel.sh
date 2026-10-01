#!/usr/bin/env bash
# balance/speed 共通: pokedex export の read model を ConfigMap にして、k3d のサービスに読ませる
# (ADR-0403・ADR-0603、ADR-0408 §2、issue #263)。SERVICE=balance または SERVICE=speed を必須の環境変数として受け取る。
# speed はビルドコンテキストがリポジトリのルート(engine を含むため。ADR-0600 §2)。リポジトリのルートで実行する。
set -euo pipefail

case "${SERVICE:-}" in
  balance | speed) ;;
  *)
    echo "k3d-deploy-readmodel.sh: SERVICE must be 'balance' or 'speed' (got '${SERVICE:-}')" >&2
    exit 1
    ;;
esac

svc_dir="services/$SERVICE"
# Argo CD の Application が在るクラスタでは手動 apply を拒否する(desired state と乖離して OutOfSync になる。ADR-0412 §5)。
if [ "${ALLOW_MANUAL_OVERLAY:-}" != "1" ]; then
  app=$(kubectl --context "k3d-${CLUSTER:-pokecalc}" -n argocd get applications.argoproj.io "pokecalc-$SERVICE" -o name 2>/dev/null || true)
  if [ -n "$app" ]; then
    echo "Argo CD の Application pokecalc-$SERVICE が有効です。手動 overlay は Argo CD と取り合うため中止します。" >&2
    echo "gitops overlay は initContainer で read model を作ります(argocd app sync)。意図して上書きするなら ALLOW_MANUAL_OVERLAY=1 を付けてください。" >&2
    exit 1
  fi
fi
svc_upper=$(printf '%s' "$SERVICE" | tr '[:lower:]' '[:upper:]')

readmodel_dir_var="${svc_upper}_READMODEL_DIR"
eval "readmodel_dir=\${${readmodel_dir_var}:-data/generated/readmodel}"
image_var="${svc_upper}_IMAGE"
eval "image=\${${image_var}:-pokecalc/$SERVICE:local}"
cluster=${CLUSTER:-pokecalc}
max_bytes=$((1000 * 1000)) # ConfigMap の上限(1 MiB)に余裕を持たせる

# balance は pokedex export を種別ごとの複数ファイルに、speed は1ファイルにまとめて出す(ADR-0403・ADR-0603)。
case "$SERVICE" in
  balance) readmodel_files=(pokemon-types.json moves.json abilities.json) ;;
  speed) readmodel_files=(speed-pokemon.json) ;;
esac

paths=()
for name in "${readmodel_files[@]}"; do
  [ -f "$readmodel_dir/$name" ] || { echo "missing $readmodel_dir/$name (run the pokedex export first)" >&2; exit 1; }
  paths+=("$readmodel_dir/$name")
done
total=$(cat "${paths[@]}" | wc -c | tr -d ' ')
[ "$total" -le "$max_bytes" ] || { echo "read model is ${total} bytes, over the ConfigMap limit" >&2; exit 1; }

context="k3d-${cluster}"
kubectl --context "$context" version --client >/dev/null

readmodel_abs=$(cd "$readmodel_dir" && pwd)
(cd "$svc_dir" && GOWORK=off go run ./cmd/checkreadmodel "$readmodel_abs")

# speed との違い: speed のイメージは engine を含むので、ビルドコンテキストはリポジトリのルート(ADR-0600 §2)。
case "$SERVICE" in
  balance) docker build -q -t "$image" "$svc_dir" >/dev/null ;;
  speed) docker build -q -f "$svc_dir/Dockerfile" -t "$image" . >/dev/null ;;
esac
k3d image import "$image" --cluster "$cluster" >/dev/null

from_file_args=()
for name in "${readmodel_files[@]}"; do
  from_file_args+=(--from-file="$readmodel_dir/$name")
done

# --server-side は last-applied-configuration annotation を作らないので、256 KiB の annotation 上限
# (client-side apply の制約)を超える大きさの read model でも ConfigMap を作れる(ADR-0403 §3・ADR-0603 §3)。
kubectl --context "$context" -n pokecalc create configmap "${SERVICE}-readmodel" \
  "${from_file_args[@]}" \
  --dry-run=client -o yaml | kubectl --context "$context" apply --server-side --force-conflicts -f - >/dev/null

hash=$(cat "${paths[@]}" | shasum -a 256 | cut -c1-16)
rendered=$(kubectl kustomize "$svc_dir/deploy/k8s/overlays/local-readmodel" \
  | sed "s/pokecalc.example\/readmodel-hash: unset/pokecalc.example\/readmodel-hash: \"${hash}\"/")
printf '%s\n' "$rendered" | grep -q "readmodel-hash: \"${hash}\"" \
  || { echo "failed to stamp the readmodel hash into the Deployment" >&2; exit 1; }
printf '%s\n' "$rendered" | kubectl --context "$context" apply --server-side --force-conflicts -f - >/dev/null
kubectl --context "$context" -n pokecalc rollout restart "deployment/$SERVICE" >/dev/null
kubectl --context "$context" -n pokecalc rollout status "deployment/$SERVICE" --timeout=120s
