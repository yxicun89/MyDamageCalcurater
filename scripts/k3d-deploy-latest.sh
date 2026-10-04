#!/usr/bin/env bash
# いまのチェックアウト(通常は main の最新)で全サービスのイメージを作り直し、k3d(make up 済み)へ入れ替える。
# 動作確認(docs/verify-m1.md)の前に毎回実行する。古いイメージが残ると、画面は開くのに API が 404 になる
# (例: pokedex に新しいエンドポイントが無い)ことがあるため。
#
# 対象: pokedex の DB(migrate-up)・pokedex(サーバー)・pokedex-importer(CronJob・import-k8s が使う)・
# calc・gateway・web・judge・balance・speed・M2(NATS・TiDB・record・team。scripts/k3d-m2-deploy.sh。ADR-0226)。
# M2 が準備できなくても(TiDB の起動失敗等)他のサービスの入れ替えは続け、理由を表示して最後に非ゼロで終わる。
# balance・speed は実データの read model(data/generated/readmodel。`make pokedex-export` の出力)で入れる。
# 無ければ balance・speed は入れ替えず、理由を表示して最後に非ゼロで終わる(黙って成功扱いにしない)。
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"
# API 契約・SQL の生成物は Git に置かない。無ければ make gen を案内して止まる(make 経由なら先に生成済み。ADR-0807)
./scripts/ensure-gen.sh check

CLUSTER=${CLUSTER:-pokecalc}
READMODEL_DIR=${READMODEL_DIR:-data/generated/readmodel}
POKEDEX_SERVER_IMAGE=${POKEDEX_SERVER_IMAGE:-pokecalc/pokedex:0.1.0}
POKEDEX_IMPORTER_IMAGE=${POKEDEX_IMPORTER_IMAGE:-pokecalc/pokedex-importer:0.1.0}
MIGRATE_LOCAL_PORT=${MIGRATE_LOCAL_PORT:-13306}

CLUSTER="$CLUSTER" ./scripts/require-k3d-context.sh deploy-latest

# pokedex の DB を最新の migration まで上げる(新しい表・列を前提にするコードより先に)。
# migrate の Job は共有の overlay 全体の apply でしか作り直せないので、ここでは mysql へ一時的に
# port-forward し、Job と同じ4つの DSN(Secret mysql-auth。値は表示しない)で `make migrate-up` を実行する。
# POKEDEX_PROVISION_DSN(root)を渡すので「用途別ユーザーのプロビジョニング → Up → importer の表ごとの
# 権限の付け直し」まで1回で行う(ADR-0125。migrator には GRANT の権限が無く、付け直さないと
# 表を足す migration の後に importer がその表に書けない)。
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
# local_dsn <Secret のキー>: DSN の接続先を port-forward 先に付け替えて出力する。キーが無い・付け替えられない
# ときは理由を stderr に出して非ゼロで返す(値は表示しない)。
local_dsn() {
  local key=$1 dsn
  dsn=$(kubectl -n pokecalc get secret mysql-auth -o jsonpath="{.data.$key}" | base64 -d \
    | sed -E "s/@tcp\(mysql:[0-9]+\)/@tcp(127.0.0.1:$MIGRATE_LOCAL_PORT)/")
  case "$dsn" in
    *"@tcp(127.0.0.1:$MIGRATE_LOCAL_PORT)"*) printf '%s' "$dsn" ;;
    "") echo "deploy-latest: Secret mysql-auth に $key が無い(make up で作り直す。docs/runbooks/data.md)" >&2; return 1 ;;
    *) echo "deploy-latest: $key の接続先が想定(mysql:<port>)と違うので付け替えられない" >&2; return 1 ;;
  esac
}
migrator_dsn=$(local_dsn pokedex-migrator-dsn)
provision_dsn=$(local_dsn pokedex-dsn)
reader_dsn=$(local_dsn pokedex-reader-dsn)
importer_dsn=$(local_dsn pokedex-importer-dsn)
POKEDEX_PROVISION_DSN="$provision_dsn" POKEDEX_READER_DSN="$reader_dsn" POKEDEX_IMPORTER_DSN="$importer_dsn" \
  POKEDEX_DATABASE_DSN="$migrator_dsn" make --no-print-directory migrate-up
POKEDEX_DATABASE_DSN="$migrator_dsn" make --no-print-directory migrate-version
unset migrator_dsn provision_dsn reader_dsn importer_dsn
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
if ! kubectl -n pokecalc rollout status deployment/pokedex --timeout=180s; then
  echo "deploy-latest: pokedex が Ready にならない。readiness は DB のマスタに連動する(ADR-0129)。" >&2
  echo "  新規クラスタ(初回 import 前)なら、先に make import-k8s を流してから、もう一度このコマンドを実行する" >&2
  exit 1
fi

echo "== calc・gateway"
make --no-print-directory api-k3d-deploy
# 変換済みの画像があれば gateway へ見せる。無い・失敗しても画像なし(エンブレム)で動くので、deploy-latest は失敗させない。
./scripts/images-k3d.sh || echo "deploy-latest: 画像の配置に失敗(画像なしで続行。make images-k3d で再試行)" >&2
echo "== web"
make --no-print-directory web-k3d-deploy
echo "== judge"
make --no-print-directory judge-k3d-deploy

# M2(NATS・TiDB・record・team)。失敗しても他のサービスの入れ替えは済んでいるので続行し、最後に非ゼロで終わる。
# calc・gateway・pokedex は計算を TiDB に依存しない(CLAUDE.md 絶対ルール5)。
echo "== M2(NATS・TiDB・record・team)"
m2_failed=0
CLUSTER="$CLUSTER" ./scripts/k3d-m2-deploy.sh || m2_failed=1

missing=0
if [ -f "$READMODEL_DIR/speed-pokemon.json" ] && [ -f "$READMODEL_DIR/pokemon-types.json" ]; then
  # Argo CD が管理しているサービス(Application pokecalc-<svc> がある)は手動の overlay で上書きしない
  # (k3d-deploy-readmodel.sh が拒否する。ADR-0412 §5)。その場合は飛ばして、Argo CD の sync を案内する。
  for svc in balance speed; do
    if kubectl -n argocd get applications.argoproj.io "pokecalc-$svc" -o name >/dev/null 2>&1; then
      echo "== $svc: Argo CD(Application pokecalc-$svc)が管理しているので飛ばす。最新にするには docs/runbooks/$svc.md の Argo CD の sync" >&2
      continue
    fi
    svc_upper=$(printf '%s' "$svc" | tr '[:lower:]' '[:upper:]')
    echo "== $svc(read model: $READMODEL_DIR)"
    make --no-print-directory "$svc-k3d-deploy-readmodel" "${svc_upper}_READMODEL_DIR=$READMODEL_DIR"
  done
else
  missing=1
  echo "deploy-latest: $READMODEL_DIR に read model が無いので balance・speed は入れ替えていない。" >&2
  echo "  make import-k8s の後に make pokedex-export を実行してから、もう一度このコマンドを実行する(docs/runbooks/data.md)" >&2
fi

kubectl -n pokecalc get deploy -o 'custom-columns=NAME:.metadata.name,READY:.status.readyReplicas,IMAGE:.spec.template.spec.containers[0].image'
if [ "$m2_failed" = 1 ]; then
  echo "deploy-latest: M2(NATS・TiDB・record・team)が準備できていない。理由は上の '== M2' の出力。他のサービスは入れ替え済み" >&2
fi
if [ "$missing" = 1 ] || [ "$m2_failed" = 1 ]; then
  exit 1
fi
echo "deploy-latest: 全サービスを $(git rev-parse --short HEAD) の内容で入れ替えた"
