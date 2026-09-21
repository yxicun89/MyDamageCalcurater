#!/usr/bin/env bash
# ローカル k3d のクラスタ内レジストリへ balance イメージを push し、digest 参照を表示する(TB0。ADR-0018)。
# Docker Desktop のデーモンからは Mac の localhost に届かないため、docker save した tar を crane で push する。
set -euo pipefail

balance_dir=${BALANCE_DIR:-services/balance}
local_port=${BALANCE_REGISTRY_PORT:-5000}
tag=$(git rev-parse --short=12 HEAD)
image="pokecalc/balance:${tag}"
work_dir=$(mktemp -d)
cleanup() {
  if [ -n "${forward_pid:-}" ]; then
    kill "$forward_pid" 2>/dev/null || true
    wait "$forward_pid" 2>/dev/null || true
  fi
  rm -rf "$work_dir"
}
trap cleanup EXIT

command -v crane >/dev/null || { echo "crane is required (brew install crane)" >&2; exit 1; }

docker build -q -t "$image" "$balance_dir" >/dev/null
docker save "$image" -o "$work_dir/image.tar"

kubectl -n balance-registry port-forward svc/registry "${local_port}:5000" >/dev/null 2>&1 &
forward_pid=$!
for _ in $(seq 1 30); do
  curl -fsS "http://localhost:${local_port}/v2/" >/dev/null 2>&1 && break
  sleep 1
done

crane push --insecure "$work_dir/image.tar" "localhost:${local_port}/pokecalc/balance:${tag}" >/dev/null 2>&1
digest=$(crane digest --insecure "localhost:${local_port}/pokecalc/balance:${tag}")
echo "localhost:5000/pokecalc/balance@${digest}"
