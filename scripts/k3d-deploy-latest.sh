#!/usr/bin/env bash
# いまのチェックアウト(通常は main の最新)で全サービスのイメージを作り直し、k3d(make up 済み)へ入れ替える。
# 動作確認(docs/verify-m1.md)の前に毎回実行する。古いイメージが残ると、画面は開くのに API が 404 になる
# (例: pokedex に新しいエンドポイントが無い)ことがあるため。
#
# 対象: pokedex(サーバー)・calc・gateway・web・judge・balance・speed。
# balance・speed は実データの read model(data/generated/readmodel。`make pokedex-export` の出力)で入れる。
# 無ければ balance・speed は入れ替えず、理由を表示して最後に非ゼロで終わる(黙って成功扱いにしない)。
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

CLUSTER=${CLUSTER:-pokecalc}
READMODEL_DIR=${READMODEL_DIR:-data/generated/readmodel}
POKEDEX_SERVER_IMAGE=${POKEDEX_SERVER_IMAGE:-pokecalc/pokedex:0.1.0}

context="$(kubectl config current-context)"
if [ "$context" != "k3d-$CLUSTER" ]; then
  echo "deploy-latest: kubectl の context が '$context'(期待 k3d-$CLUSTER)。別クラスタへ適用しないよう中断" >&2
  exit 1
fi

echo "== pokedex"
docker build -q -f services/pokedex/Dockerfile --target server -t "$POKEDEX_SERVER_IMAGE" . >/dev/null
k3d image import "$POKEDEX_SERVER_IMAGE" --cluster "$CLUSTER" >/dev/null
kubectl -n pokecalc rollout restart deployment/pokedex
kubectl -n pokecalc rollout status deployment/pokedex --timeout=180s

echo "== calc・gateway"
make --no-print-directory api-k3d-deploy
echo "== web"
make --no-print-directory web-k3d-deploy
echo "== judge"
make --no-print-directory judge-k3d-deploy

missing=0
if [ -f "$READMODEL_DIR/speed-pokemon.json" ] && [ -f "$READMODEL_DIR/pokemon-types.json" ]; then
  echo "== balance(read model: $READMODEL_DIR)"
  make --no-print-directory balance-k3d-deploy-readmodel BALANCE_READMODEL_DIR="$READMODEL_DIR"
  echo "== speed(read model: $READMODEL_DIR)"
  make --no-print-directory speed-k3d-deploy-readmodel SPEED_READMODEL_DIR="$READMODEL_DIR"
else
  missing=1
  echo "deploy-latest: $READMODEL_DIR に read model が無いので balance・speed は入れ替えていない。" >&2
  echo "  make import-k8s の後に make pokedex-export を実行してから、もう一度このコマンドを実行する(docs/runbooks/data.md)" >&2
fi

kubectl -n pokecalc get deploy -o 'custom-columns=NAME:.metadata.name,READY:.status.readyReplicas,IMAGE:.spec.template.spec.containers[0].image'
if [ "$missing" = 1 ]; then
  exit 1
fi
echo "deploy-latest: 全サービスを $(git rev-parse --short HEAD) の内容で入れ替えた"
