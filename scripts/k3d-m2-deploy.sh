#!/usr/bin/env bash
# k3d-m2-deploy.sh — M2(NATS・TiDB・record・team)を既存の k3d クラスタへ入れる(ADR-0226)。冪等。
# `make deploy-latest`(scripts/k3d-deploy-latest.sh)と `make up`(scripts/up.sh)が呼ぶ。単独でも実行できる。
#
# 順序(上から下へ。前の段が失敗したら、それに依存する段はスキップして理由を表示する):
#   1. TiDB Operator の導入(scripts/tidb-operator-bootstrap.sh)
#   2. Secret tidb-root-auth(無ければ乱数で作る。既存の値は変えない)
#   3. TidbCluster・TidbInitializer の適用(deploy/k8s/overlays/local/tidb)と、PD/TiKV/TiDB・TidbInitializer の完了待ち
#   4. Secret record-db-auth・team-db-auth(無いキーだけ作る。record と team で分ける。ADR-0211 §4)
#   5. record-migrate・team-migrate の Job(古い Job を消してから適用し、完了を待つ)
#   6. record・team の server イメージを build し、NATS・NetworkPolicy とあわせてコミット識別のタグで apply して
#      rollout を待つ(deploy/k8s/overlays/local-m2 を scripts/k3d-deploy-tagged.sh で)。TiDB が使えないときは NATS だけ入れる
#
# 非致命: ある段が失敗しても、依存しない後続の段は続ける(TiDB が準備できなくても NATS は入る)。ただし
# 失敗した事実は最後にまとめて表示し、終了コード 1 で終わる(黙って成功扱いにしない)。calc・gateway・pokedex は
# 計算を TiDB に依存しない(CLAUDE.md 絶対ルール5)ので、この失敗はそれらに影響しない。
# Secret の値は表示しない(コマンドライン引数にもログにも出さない。値を持つ Secret は `kubectl create` で作る。issue #327)。
#
# 環境変数: CLUSTER(既定 pokecalc)、M2_POLL_SECONDS(リソース出現待ちの間隔。既定 5)、
#           RECORD_MIGRATE_IMAGE・TEAM_MIGRATE_IMAGE(migrate Job のイメージ。既定は base の 0.1.0)。
# 自動テスト: scripts/k3d-m2-deploy_test.sh(make test-scripts)。
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

CLUSTER="${CLUSTER:-pokecalc}"
TIDB_CLUSTER_NAME="pokecalc-tidb"
POLL_SECONDS="${M2_POLL_SECONDS:-5}"
RECORD_MIGRATE_IMAGE="${RECORD_MIGRATE_IMAGE:-pokecalc/record-migrate:0.1.0}"
TEAM_MIGRATE_IMAGE="${TEAM_MIGRATE_IMAGE:-pokecalc/team-migrate:0.1.0}"

CLUSTER="$CLUSTER" ./scripts/require-k3d-context.sh k3d-m2-deploy

failures=()
fail() {
  echo "k3d-m2-deploy: 警告: $1" >&2
  failures+=("$1")
}

# wait_for_resource <種別/名前> <秒数>: operator が非同期に作るリソース(StatefulSet 等)が現れるまで待つ。
# 現れる前に `rollout status` を呼ぶと NotFound で即失敗するため。
wait_for_resource() {
  local resource="$1" deadline=$((SECONDS + $2))
  until kubectl -n pokecalc get "$resource" >/dev/null 2>&1; do
    if [ "$SECONDS" -ge "$deadline" ]; then return 1; fi
    sleep "$POLL_SECONDS"
  done
}

echo "== namespace"
kubectl apply -f deploy/k8s/base/namespace.yaml

# --- 1. TiDB Operator ---
tidb_ready=1
echo "== TiDB Operator"
if ! ./scripts/tidb-operator-bootstrap.sh; then
  fail "TiDB Operator の導入に失敗した(TiDB・record・team をスキップ)"
  tidb_ready=0
fi

# --- 2. tidb-root-auth ---
# キー名は "root"(TidbInitializer の passwordSecret はキー名をユーザー名として扱う。ADR-0211 §3.2 追記)。
# どのサービスの Pod にもマウントしない(ADR-0211 §4)。
if [ "$tidb_ready" = "1" ]; then
  if kubectl -n pokecalc get secret tidb-root-auth >/dev/null 2>&1; then
    echo "Secret 'tidb-root-auth' は既に存在する(既存のパスワードをそのまま使う)"
  else
    echo "Secret 'tidb-root-auth' が無いので乱数で作る"
    tidb_root_pw_value="$(openssl rand -hex 16)"
    manifest="$(mktemp)"
    chmod 600 "$manifest"
    trap 'rm -f "$manifest"' EXIT
    cat > "$manifest" <<SECRET_MANIFEST
apiVersion: v1
kind: Secret
metadata:
  name: tidb-root-auth
  namespace: pokecalc
type: Opaque
stringData:
  root: "${tidb_root_pw_value}"
SECRET_MANIFEST
    unset tidb_root_pw_value
    # apply ではなく create(--save-config なし)。apply は入力の stringData(平文)を
    # last-applied-configuration 注釈に写すため(issue #327)。
    if ! kubectl create -f "$manifest"; then
      fail "Secret tidb-root-auth を作れなかった(TiDB・record・team をスキップ)"
      tidb_ready=0
    fi
    rm -f "$manifest"
    trap - EXIT
  fi
fi

# --- 3. TidbCluster・TidbInitializer ---
if [ "$tidb_ready" = "1" ]; then
  echo "== TiDB(overlays/local/tidb)"
  if ! kubectl apply -k deploy/k8s/overlays/local/tidb; then
    fail "overlays/local/tidb の適用に失敗した(record・team をスキップ)"
    tidb_ready=0
  fi
fi

if [ "$tidb_ready" = "1" ]; then
  echo "== TiDB(PD・TiKV・TiDB)の Ready を待つ"
  for component in pd tikv tidb; do
    sts="statefulset/${TIDB_CLUSTER_NAME}-${component}"
    if ! wait_for_resource "$sts" 180 || ! kubectl -n pokecalc rollout status "$sts" --timeout=300s; then
      fail "TiDB の ${component} が Ready にならなかった(record・team をスキップ)"
      tidb_ready=0
      break
    fi
  done
fi

if [ "$tidb_ready" = "1" ]; then
  # migrate Job より前に完了していること(順序を誤ると Unknown database・認証エラーになる。ADR-0211 AC-T8)。
  echo "== TidbInitializer(record・team の DB 作成・root パスワード設定)の完了を待つ"
  if ! kubectl -n pokecalc wait --for=jsonpath='{.status.phase}'=Completed tidbinitializer/pokecalc --timeout=240s; then
    fail "TidbInitializer が Completed にならなかった(record・team をスキップ。原因は kubectl -n pokecalc logs job/pokecalc-tidb-tidb-initializer -c mysql-client。復旧は docs/adr/0226-api-m2-k3d-deploy.md)"
    tidb_ready=0
  fi
fi

# --- 4. record-db-auth・team-db-auth ---
# record が team の Secret を参照できる形にしない(ADR-0211 §4)ため2本に分ける。
create_db_auth_secret() {
  # 引数: secret名 db名 provisionキー migratorキー appキー
  # 既存の Secret には、無いキー(app・migrator)だけを patch で足す。provision キー(root パスワードを含む DSN)は
  # 既存なら補わない(tidb-root-auth を作り直した場合に古いままになりうる既知の制約。ADR-0211)。
  local secret_name="$1" db_name="$2" provision_key="$3" migrator_key="$4" app_key="$5"
  local tidb_host="${TIDB_CLUSTER_NAME}-tidb"
  local dsn_tail="@tcp(${tidb_host}:4000)/${db_name}?parseTime=true"
  if kubectl -n pokecalc get secret "$secret_name" >/dev/null 2>&1; then
    echo "Secret '${secret_name}' は既に存在する(無いキーだけ追記する)"
    local existing_app existing_migrator patch_entries="" pw
    existing_app="$(kubectl -n pokecalc get secret "$secret_name" -o jsonpath="{.data.${app_key}}")"
    existing_migrator="$(kubectl -n pokecalc get secret "$secret_name" -o jsonpath="{.data.${migrator_key}}")"
    if [ -z "$existing_app" ]; then
      pw="$(openssl rand -hex 16)"
      patch_entries="${patch_entries}  ${app_key}: \"${db_name}_app:${pw}${dsn_tail}\"
"
    fi
    if [ -z "$existing_migrator" ]; then
      pw="$(openssl rand -hex 16)"
      patch_entries="${patch_entries}  ${migrator_key}: \"${db_name}_migrator:${pw}${dsn_tail}\"
"
    fi
    if [ -n "$patch_entries" ]; then
      local patch_file
      patch_file="$(mktemp)"
      chmod 600 "$patch_file"
      trap 'rm -f "$patch_file"' EXIT
      { printf 'stringData:\n'; printf '%s' "$patch_entries"; } > "$patch_file"
      kubectl -n pokecalc patch secret "$secret_name" --type=merge --patch-file "$patch_file"
      rm -f "$patch_file"
      trap - EXIT
    else
      echo "追加するキーは無い(app・migrator とも既にある)"
    fi
  else
    echo "Secret '${secret_name}' が無いので乱数で作る"
    local app_pw migrator_pw root_pw manifest
    app_pw="$(openssl rand -hex 16)"
    migrator_pw="$(openssl rand -hex 16)"
    root_pw="$(kubectl -n pokecalc get secret tidb-root-auth -o jsonpath='{.data.root}' | base64 -d)"
    manifest="$(mktemp)"
    chmod 600 "$manifest"
    trap 'rm -f "$manifest"' EXIT
    cat > "$manifest" <<SECRET_MANIFEST
apiVersion: v1
kind: Secret
metadata:
  name: ${secret_name}
  namespace: pokecalc
type: Opaque
stringData:
  ${provision_key}: "root:${root_pw}${dsn_tail}"
  ${migrator_key}: "${db_name}_migrator:${migrator_pw}${dsn_tail}"
  ${app_key}: "${db_name}_app:${app_pw}${dsn_tail}"
SECRET_MANIFEST
    # create(--save-config なし): apply は平文を last-applied-configuration 注釈に写す(issue #327)。
    kubectl create -f "$manifest"
    rm -f "$manifest"
    trap - EXIT
  fi
}

if [ "$tidb_ready" = "1" ]; then
  echo "== record-db-auth・team-db-auth"
  if ! create_db_auth_secret record-db-auth record record-provision-dsn record-migrator-dsn record-app-dsn \
    || ! create_db_auth_secret team-db-auth team team-provision-dsn team-migrator-dsn team-app-dsn; then
    fail "record-db-auth・team-db-auth を用意できなかった(record・team をスキップ)"
    tidb_ready=0
  fi
fi

# --- 5. migrate Job ---
if [ "$tidb_ready" = "1" ]; then
  echo "== record-migrate・team-migrate"
  docker build -q -f services/record/Dockerfile --target migrate -t "$RECORD_MIGRATE_IMAGE" . >/dev/null
  k3d image import "$RECORD_MIGRATE_IMAGE" --cluster "$CLUSTER" >/dev/null
  docker build -q -f services/team/Dockerfile --target migrate -t "$TEAM_MIGRATE_IMAGE" . >/dev/null
  k3d image import "$TEAM_MIGRATE_IMAGE" --cluster "$CLUSTER" >/dev/null
  # Job は作成後に Pod テンプレートを更新できないため、古い Job を消してから適用する(Job の削除はデータ削除ではない)。
  kubectl -n pokecalc delete job record-migrate team-migrate --ignore-not-found
  # job-migrate.yaml は metadata.namespace を持たない(overlay に含めない)ので -n が要る。
  if ! kubectl -n pokecalc apply -f deploy/k8s/base/record/job-migrate.yaml \
    || ! kubectl -n pokecalc apply -f deploy/k8s/base/team/job-migrate.yaml; then
    fail "record-migrate・team-migrate Job を適用できなかった(record・team をスキップ)"
    tidb_ready=0
  fi
fi

if [ "$tidb_ready" = "1" ]; then
  if ! kubectl -n pokecalc wait --for=condition=complete job/record-migrate --timeout=300s \
    || ! kubectl -n pokecalc wait --for=condition=complete job/team-migrate --timeout=300s; then
    fail "record-migrate・team-migrate Job が完了しなかった(record・team をスキップ。kubectl -n pokecalc logs job/record-migrate)"
    tidb_ready=0
  fi
fi

# --- 6. NATS・record・team ---
# NATS(overlays/local/nats)は TiDB に依存しないので TiDB の成否にかかわらず入れたいが、record・team と同じ
# overlay(local-m2)で描画するため、record・team の rollout を待つのは TiDB が使えるときだけにする。
# TiDB が使えないときは NATS だけを適用する。
if [ "$tidb_ready" = "1" ]; then
  echo "== record・team のイメージ build と NATS・NetworkPolicy を含む apply(overlays/local-m2)"
  docker build -q -f services/record/Dockerfile --target server -t pokecalc/record:local . >/dev/null
  docker build -q -f services/team/Dockerfile --target server -t pokecalc/team:local . >/dev/null
  if ! CLUSTER="$CLUSTER" TAG_PATHS="services" ./scripts/k3d-deploy-tagged.sh k3d-m2-deploy \
    deploy/k8s/overlays/local-m2 pokecalc/record pokecalc/team; then
    fail "record・team のデプロイ(apply・rollout)に失敗した"
  fi
else
  echo "== NATS のみ(TiDB が使えないので record・team は入れない)"
  if ! kubectl apply -k deploy/k8s/overlays/local/nats -n pokecalc; then
    fail "NATS の適用に失敗した"
  fi
fi
if ! kubectl -n pokecalc rollout status statefulset/nats --timeout=120s; then
  fail "NATS が Ready にならなかった"
fi

if [ "${#failures[@]}" -gt 0 ]; then
  echo "k3d-m2-deploy: 失敗 ${#failures[@]} 件(M2 は k3d で動いていない。calc・gateway・pokedex には影響しない):" >&2
  for f in "${failures[@]}"; do echo "  - $f" >&2; done
  exit 1
fi
echo "k3d-m2-deploy: NATS・TiDB・record・team を入れた"
