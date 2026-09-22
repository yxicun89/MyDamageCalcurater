#!/usr/bin/env bash
# pokedex export の read model を ConfigMap にして、k3d の balance に読ませる(ADR-0403)。
set -euo pipefail

balance_dir=${BALANCE_DIR:-services/balance}
readmodel_dir=${BALANCE_READMODEL_DIR:-data/generated/readmodel}
image=${BALANCE_IMAGE:-pokecalc/balance:local}
cluster=${CLUSTER:-pokecalc}
max_bytes=$((1000 * 1000)) # ConfigMap の上限(1 MiB)に余裕を持たせる

for name in pokemon-types.json moves.json abilities.json; do
  [ -f "$readmodel_dir/$name" ] || { echo "missing $readmodel_dir/$name (run the pokedex export first)" >&2; exit 1; }
done
total=$(cat "$readmodel_dir"/pokemon-types.json "$readmodel_dir"/moves.json "$readmodel_dir"/abilities.json | wc -c | tr -d ' ')
[ "$total" -le "$max_bytes" ] || { echo "read model is ${total} bytes, over the ConfigMap limit" >&2; exit 1; }

context="k3d-${cluster}"
kubectl --context "$context" version --client >/dev/null

readmodel_abs=$(cd "$readmodel_dir" && pwd)
(cd "$balance_dir" && GOWORK=off go run ./cmd/checkreadmodel "$readmodel_abs")

docker build -q -t "$image" "$balance_dir" >/dev/null
k3d image import "$image" --cluster "$cluster" >/dev/null

# --server-side は last-applied-configuration annotation を作らないので、256 KiB の annotation 上限
# (client-side apply の制約)を超える大きさの read model でも ConfigMap を作れる(ADR-0403 §3)。
kubectl --context "$context" -n pokecalc create configmap balance-readmodel \
  --from-file="$readmodel_dir/pokemon-types.json" --from-file="$readmodel_dir/moves.json" --from-file="$readmodel_dir/abilities.json" \
  --dry-run=client -o yaml | kubectl --context "$context" apply --server-side --force-conflicts -f - >/dev/null

hash=$(cat "$readmodel_dir"/pokemon-types.json "$readmodel_dir"/moves.json "$readmodel_dir"/abilities.json | shasum -a 256 | cut -c1-16)
rendered=$(kubectl kustomize "$balance_dir/deploy/k8s/overlays/local-readmodel" \
  | sed "s/pokecalc.example\/readmodel-hash: unset/pokecalc.example\/readmodel-hash: \"${hash}\"/")
printf '%s\n' "$rendered" | grep -q "readmodel-hash: \"${hash}\"" \
  || { echo "failed to stamp the readmodel hash into the Deployment" >&2; exit 1; }
printf '%s\n' "$rendered" | kubectl --context "$context" apply --server-side --force-conflicts -f - >/dev/null
kubectl --context "$context" -n pokecalc rollout restart deployment/balance >/dev/null
kubectl --context "$context" -n pokecalc rollout status deployment/balance --timeout=120s
