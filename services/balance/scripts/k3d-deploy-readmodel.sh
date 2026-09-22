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

readmodel_abs=$(cd "$readmodel_dir" && pwd)
(cd "$balance_dir" && GOWORK=off go run ./cmd/checkreadmodel "$readmodel_abs")

docker build -q -t "$image" "$balance_dir" >/dev/null
k3d image import "$image" --cluster "$cluster" >/dev/null
kubectl -n pokecalc create configmap balance-readmodel \
  --from-file="$readmodel_dir/pokemon-types.json" --from-file="$readmodel_dir/moves.json" --from-file="$readmodel_dir/abilities.json" \
  --dry-run=client -o yaml | kubectl apply -f - >/dev/null
hash=$(cat "$readmodel_dir"/pokemon-types.json "$readmodel_dir"/moves.json "$readmodel_dir"/abilities.json | shasum -a 256 | cut -c1-16)
kubectl kustomize "$balance_dir/deploy/k8s/overlays/local-readmodel" \
  | sed "s/pokecalc.example\/readmodel-hash: unset/pokecalc.example\/readmodel-hash: \"${hash}\"/" \
  | kubectl apply -f - >/dev/null
kubectl -n pokecalc rollout status deployment/balance --timeout=120s
