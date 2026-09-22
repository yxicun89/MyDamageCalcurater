#!/usr/bin/env bash
# pokedex export の read model を ConfigMap にして、k3d の speed に読ませる(ADR-0603)。
# リポジトリのルートで実行する(engine を含むためイメージのビルドコンテキストがルート。ADR-0600 §2)。
set -euo pipefail

speed_dir=${SPEED_DIR:-services/speed}
readmodel_dir=${SPEED_READMODEL_DIR:-data/generated/readmodel}
image=${SPEED_IMAGE:-pokecalc/speed:local}
cluster=${CLUSTER:-pokecalc}
readmodel_file=speed-pokemon.json
max_bytes=$((1000 * 1000)) # ConfigMap の上限(1 MiB)に余裕を持たせる

[ -f "$readmodel_dir/$readmodel_file" ] \
  || { echo "missing $readmodel_dir/$readmodel_file (run the pokedex export first)" >&2; exit 1; }
total=$(wc -c <"$readmodel_dir/$readmodel_file" | tr -d ' ')
[ "$total" -le "$max_bytes" ] || { echo "read model is ${total} bytes, over the ConfigMap limit" >&2; exit 1; }

context="k3d-${cluster}"
kubectl --context "$context" version --client >/dev/null

readmodel_abs=$(cd "$readmodel_dir" && pwd)
(cd "$speed_dir" && GOWORK=off go run ./cmd/checkreadmodel "$readmodel_abs")

# balance との違い: speed のイメージは engine を含むので、ビルドコンテキストはリポジトリのルート。
docker build -q -f "$speed_dir/Dockerfile" -t "$image" . >/dev/null
k3d image import "$image" --cluster "$cluster" >/dev/null

# --server-side は last-applied-configuration annotation を作らないので、256 KiB の annotation 上限
# (client-side apply の制約)を超える大きさの read model でも ConfigMap を作れる(ADR-0603 §3)。
kubectl --context "$context" -n pokecalc create configmap speed-readmodel \
  --from-file="$readmodel_dir/$readmodel_file" \
  --dry-run=client -o yaml | kubectl --context "$context" apply --server-side --force-conflicts -f - >/dev/null

hash=$(shasum -a 256 <"$readmodel_dir/$readmodel_file" | cut -c1-16)
rendered=$(kubectl kustomize "$speed_dir/deploy/k8s/overlays/local-readmodel" \
  | sed "s/pokecalc.example\/readmodel-hash: unset/pokecalc.example\/readmodel-hash: \"${hash}\"/")
printf '%s\n' "$rendered" | grep -q "readmodel-hash: \"${hash}\"" \
  || { echo "failed to stamp the readmodel hash into the Deployment" >&2; exit 1; }
printf '%s\n' "$rendered" | kubectl --context "$context" apply --server-side --force-conflicts -f - >/dev/null
kubectl --context "$context" -n pokecalc rollout restart deployment/speed >/dev/null
kubectl --context "$context" -n pokecalc rollout status deployment/speed --timeout=120s
