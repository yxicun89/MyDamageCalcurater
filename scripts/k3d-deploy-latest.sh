#!/usr/bin/env bash
# いまのチェックアウト(通常は main の最新)で全サービスのイメージを作り直し、k3d(make up 済み)へ入れ替える。
# 動作確認(docs/verify-m1.md)の前に毎回実行する。古いイメージが残ると、画面は開くのに API が 404 になる
# (例: pokedex に新しいエンドポイントが無い)ことがあるため。
#
# 対象: pokedex の DB(migrate-up)・pokedex(サーバー)・pokedex-importer(CronJob・import-k8s が使う)・
# calc・gateway・web・judge・balance・speed。
# balance・speed は実データの read model(data/generated/readmodel。`make pokedex-export` の出力)で入れる。
# 無ければ balance・speed は入れ替えず、理由を表示して最後に非ゼロで終わる(黙って成功扱いにしない)。
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

CLUSTER=${CLUSTER:-pokecalc}
READMODEL_DIR=${READMODEL_DIR:-data/generated/readmodel}
POKEDEX_SERVER_IMAGE=${POKEDEX_SERVER_IMAGE:-pokecalc/pokedex:0.1.0}
POKEDEX_IMPORTER_IMAGE=${POKEDEX_IMPORTER_IMAGE:-pokecalc/pokedex-importer:0.1.0}
MIGRATE_LOCAL_PORT=${MIGRATE_LOCAL_PORT:-13306}

context="$(kubectl config current-context)"
if [ "$context" != "k3d-$CLUSTER" ]; then
  echo "deploy-latest: kubectl の context が '$context'(期待 k3d-$CLUSTER)。別クラスタへ適用しないよう中断" >&2
  exit 1
fi

# pokedex の DB を最新の migration まで上げる(新しい表・列を前提にするコードより先に)。
# migrate の Job は共有の overlay 全体の apply でしか作り直せないので、ここでは mysql へ一時的に
# port-forward し、migrator 用の DSN(Secret mysql-auth。値は表示しない)で `make migrate-up` を実行する。
echo "== pokedex の DB(migrate-up)"
command -v nc >/dev/null 2>&1 || { echo "deploy-latest: nc が無い(port-forward の疎通確認に使う)" >&2; exit 1; }
if nc -z 127.0.0.1 "$MIGRATE_LOCAL_PORT" 2>/dev/null; then
  echo "deploy-latest: 127.0.0.1:$MIGRATE_LOCAL_PORT は別のプロセスが使用中。MIGRATE_LOCAL_PORT=<空きポート> を付けて再実行する" >&2
  exit 1
fi
kubectl -n pokecalc port-forward svc/mysql "$MIGRATE_LOCAL_PORT:3306" >/dev/null 2>&1 &
pf_pid=$!
trap 'kill "$pf_pid" 2>/dev/null || true' EXIT
ready=0
for _ in $(seq 1 20); do
  if ! kill -0 "$pf_pid" 2>/dev/null; then break; fi
  if nc -z 127.0.0.1 "$MIGRATE_LOCAL_PORT" 2>/dev/null; then ready=1; break; fi
  sleep 0.5
done
if [ "$ready" != 1 ]; then
  echo "deploy-latest: mysql への port-forward が張れなかった(kubectl -n pokecalc get pods で mysql-0 が Running か確認する)" >&2
  exit 1
fi
migrator_dsn=$(kubectl -n pokecalc get secret mysql-auth -o jsonpath='{.data.pokedex-migrator-dsn}' | base64 -d \
  | sed -E "s/@tcp\(mysql:[0-9]+\)/@tcp(127.0.0.1:$MIGRATE_LOCAL_PORT)/")
case "$migrator_dsn" in
  *"@tcp(127.0.0.1:$MIGRATE_LOCAL_PORT)"*) ;;
  "") echo "deploy-latest: Secret mysql-auth に pokedex-migrator-dsn が無い(make up で作り直す。docs/runbooks/data.md)" >&2; exit 1 ;;
  *) echo "deploy-latest: pokedex-migrator-dsn の接続先が想定(mysql:<port>)と違うので付け替えられない" >&2; exit 1 ;;
esac
POKEDEX_DATABASE_DSN="$migrator_dsn" make --no-print-directory migrate-up
POKEDEX_DATABASE_DSN="$migrator_dsn" make --no-print-directory migrate-version
unset migrator_dsn
kill "$pf_pid" 2>/dev/null || true
wait "$pf_pid" 2>/dev/null || true
trap - EXIT

echo "== pokedex-importer(CronJob・make import-k8s が使うイメージ)"
docker build -q -f services/pokedex/Dockerfile --target importer -t "$POKEDEX_IMPORTER_IMAGE" . >/dev/null
k3d image import "$POKEDEX_IMPORTER_IMAGE" --cluster "$CLUSTER" >/dev/null

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
