#!/usr/bin/env bash
# ローカル k3d のクラスタ内レジストリへ speed イメージを push し、digest 参照を表示する(SP5。ADR-0605)。
# レジストリは balance レーンが持つ `balance-registry` namespace のものを共有する(hostPort は 1 ノード 1 つ。ADR-0605 §1)。
# Docker Desktop のデーモンからは Mac の localhost に届かないため、docker save した tar を crane で push する。
set -euo pipefail

speed_dir=${SPEED_DIR:-services/speed}
local_port=${SPEED_REGISTRY_PORT:-5002}
tag=$(git rev-parse --short=12 HEAD)
# イメージには engine も入る(ビルドコンテキストはリポジトリのルート。ADR-0600 §2)ので、
# 未コミットの変更は speed と engine の両方を見て印を付ける。
if [ -n "$(git status --porcelain -- "$speed_dir" engine)" ]; then
  tag="${tag}-dirty"   # 未コミットの変更があると tag と中身がずれるので印を付ける
fi
image="pokecalc/speed:${tag}"
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

docker build -q -f "$speed_dir/Dockerfile" -t "$image" . >/dev/null
docker save "$image" -o "$work_dir/image.tar"

kubectl -n balance-registry port-forward svc/registry "${local_port}:5000" >/dev/null 2>&1 &
forward_pid=$!
ready=false
for _ in $(seq 1 30); do
  if ! kill -0 "$forward_pid" 2>/dev/null; then
    echo "port-forward to the registry exited (is localhost:${local_port} already in use?)" >&2
    exit 1
  fi
  if curl -fsS "http://localhost:${local_port}/v2/" >/dev/null 2>&1; then
    ready=true
    break
  fi
  sleep 1
done
[ "$ready" = true ] || { echo "registry did not answer on localhost:${local_port}" >&2; exit 1; }

crane push --insecure "$work_dir/image.tar" "localhost:${local_port}/pokecalc/speed:${tag}" >/dev/null
digest=$(crane digest --insecure "localhost:${local_port}/pokecalc/speed:${tag}")
echo "localhost:5000/pokecalc/speed@${digest}"
