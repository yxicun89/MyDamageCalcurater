#!/usr/bin/env bash
# pokedex(server イメージ。/pokedex バイナリ、export サブコマンド持ち。ADR-0105 §5)を
# クラスタ内共有レジストリ balance-registry へ push し、digest 参照を表示する。
# タイプバランスレーン issue #237(gitops overlay の initContainer に pokedex export を使う案)の依頼。
# レジストリは balance レーンが持つ balance-registry namespace のものを、speed と同じ方式で共有する
# (ADR-0018 §2・ADR-0605 §1)。Docker Desktop のデーモンからは Mac の localhost に届かないため、
# docker save した tar を crane で push する(services/balance・services/speed の
# scripts/local-registry-push.sh と同じ方式。crane は `brew install crane` の最新安定版)。
#
# 検証用に POKEDEX_REGISTRY_HOST(例 localhost:15000)を指定すると、kubectl の context 検査と
# 共有クラスタへの port-forward を行わず、そのホストへ直接 push する(使い捨てのローカル
# registry:2系コンテナ等に向ける。共有クラスタへは検証で push しない)。
#
# 共有クラスタへの push は取り違えを防ぐため、宛先を表示したうえで POKEDEX_REGISTRY_PUSH_CONFIRM=1 が
# 無ければ中断する(実装中に検証用の切り替えを付け忘れて共有クラスタへ push した事故の再発防止)。
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

pokedex_dir=${POKEDEX_DIR:-services/pokedex}
cluster=${CLUSTER:-pokecalc}
local_port=${POKEDEX_REGISTRY_PORT:-5003}
tag=$(git rev-parse --short=12 HEAD)
# イメージには engine も入る(ビルドコンテキストはリポジトリのルート。Dockerfile 冒頭のコメント参照)ので、
# 未コミットの変更は pokedex と engine の両方を見て印を付ける(speed の local-registry-push.sh と同じ考え方)。
if [ -n "$(git status --porcelain -- "$pokedex_dir" engine)" ]; then
  tag="${tag}-dirty"   # 未コミットの変更があると tag と中身がずれるので印を付ける
fi
image="pokecalc/pokedex:${tag}"
work_dir=$(mktemp -d)
forward_pid=""
cleanup() {
  if [ -n "$forward_pid" ]; then
    kill "$forward_pid" 2>/dev/null || true
    wait "$forward_pid" 2>/dev/null || true
  fi
  rm -rf "$work_dir"
}
trap cleanup EXIT

command -v crane >/dev/null || { echo "pokedex-registry-push: crane is required (brew install crane)" >&2; exit 1; }

docker build -f "$pokedex_dir/Dockerfile" --target server -t "$image" . >/dev/null
docker save "$image" -o "$work_dir/image.tar"

if [ -n "${POKEDEX_REGISTRY_HOST:-}" ]; then
  # 検証用の宛先切り替え(使い捨てレジストリ)。push 先と参照に使うホストは同じ。
  push_host="$POKEDEX_REGISTRY_HOST"
  ref_host="$POKEDEX_REGISTRY_HOST"
else
  context="$(kubectl config current-context)"
  if [ "$context" != "k3d-${cluster}" ]; then
    echo "pokedex-registry-push: kubectl の context が '$context'(期待 k3d-${cluster})。別クラスタへ push しないよう中断" >&2
    exit 1
  fi
  echo "pokedex-registry-push: 宛先 = 共有クラスタ(context ${context})の balance-registry、イメージ ${image}" >&2
  if [ "${POKEDEX_REGISTRY_PUSH_CONFIRM:-}" != 1 ]; then
    echo "pokedex-registry-push: 共有クラスタへ push するときは POKEDEX_REGISTRY_PUSH_CONFIRM=1 を付けて実行する(検証なら POKEDEX_REGISTRY_HOST=<使い捨てレジストリ>)" >&2
    exit 1
  fi

  kubectl -n balance-registry port-forward svc/registry "${local_port}:5000" >/dev/null 2>&1 &
  forward_pid=$!
  ready=false
  for _ in $(seq 1 30); do
    if ! kill -0 "$forward_pid" 2>/dev/null; then
      echo "pokedex-registry-push: port-forward to the registry exited (is localhost:${local_port} already in use?)" >&2
      exit 1
    fi
    if curl -fsS "http://localhost:${local_port}/v2/" >/dev/null 2>&1; then
      ready=true
      break
    fi
    sleep 1
  done
  [ "$ready" = true ] || { echo "pokedex-registry-push: registry did not answer on localhost:${local_port}" >&2; exit 1; }

  # push は Mac 側の port-forward 経由(local_port)。ただし書き出す参照は、k3d ノードの containerd が
  # 平文 pull を許可している localhost:5000(ノード内から見たレジストリのアドレス)にする。
  # (balance・speed の local-registry-push.sh と同じ理由。Mac 側ポートは push 用の一時的なトンネルでしかない。)
  push_host="localhost:${local_port}"
  ref_host="localhost:5000"
fi

echo "pokedex-registry-push: push ${push_host}/pokecalc/pokedex:${tag}" >&2
crane push --insecure "$work_dir/image.tar" "${push_host}/pokecalc/pokedex:${tag}" >/dev/null
digest=$(crane digest --insecure "${push_host}/pokecalc/pokedex:${tag}")
echo "${ref_host}/pokecalc/pokedex@${digest}"
