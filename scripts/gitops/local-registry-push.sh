#!/usr/bin/env bash
# balance/speed 共通: ローカル k3d のクラスタ内レジストリへイメージを push し、digest 参照を表示する
# (ADR-0408 §2、issue #263)。レジストリ自体は balance-registry namespace のものを両サービスで共有する
# (ADR-0605 §1)。SERVICE=balance または SERVICE=speed を必須の環境変数として受け取る。
# Docker Desktop のデーモンからは Mac の localhost に届かないため、docker save した tar を crane で push する。
set -euo pipefail

case "${SERVICE:-}" in
  balance | speed) ;;
  *)
    echo "local-registry-push.sh: SERVICE must be 'balance' or 'speed' (got '${SERVICE:-}')" >&2
    exit 1
    ;;
esac

svc_dir="services/$SERVICE"
svc_upper=$(printf '%s' "$SERVICE" | tr '[:lower:]' '[:upper:]')

# macOS の AirPlay 受信が 5000 を使うことがあるので、Mac 側の port-forward は balance=5001 / speed=5002 に分ける
# (ノード側は両方とも localhost:5000 のまま)。
case "$SERVICE" in
  balance) default_port=5001 ;;
  speed) default_port=5002 ;;
esac
port_var="${svc_upper}_REGISTRY_PORT"
eval "local_port=\${${port_var}:-$default_port}"

tag=$(git rev-parse --short=12 HEAD)
# speed のイメージには engine も入る(ビルドコンテキストはリポジトリのルート。ADR-0600 §2)ので、
# 未コミットの変更は speed と engine の両方を見て印を付ける。
dirty_paths="$svc_dir"
case "$SERVICE" in
  speed) dirty_paths="$svc_dir engine" ;;
esac
if [ -n "$(git status --porcelain -- $dirty_paths)" ]; then
  tag="${tag}-dirty" # 未コミットの変更があると tag と中身がずれるので印を付ける
fi
image="pokecalc/$SERVICE:${tag}"
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

case "$SERVICE" in
  balance) docker build -q -t "$image" "$svc_dir" >/dev/null ;;
  speed) docker build -q -f "$svc_dir/Dockerfile" -t "$image" . >/dev/null ;;
esac
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

crane push --insecure "$work_dir/image.tar" "localhost:${local_port}/pokecalc/$SERVICE:${tag}" >/dev/null
digest=$(crane digest --insecure "localhost:${local_port}/pokecalc/$SERVICE:${tag}")
echo "localhost:5000/pokecalc/$SERVICE@${digest}"
