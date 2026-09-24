#!/usr/bin/env bash
# k3d クラスタを作成(なければ)し、ローカル overlay を適用する。
# サービスの Deployment はフェーズ進行に応じて base/kustomization.yaml へ追加される。
set -euo pipefail
cd "$(dirname "$0")/.."

CLUSTER="${CLUSTER:-pokecalc}"
POKEDEX_MIGRATE_IMAGE="${POKEDEX_MIGRATE_IMAGE:-pokecalc/pokedex-migrate:0.1.0}"
POKEDEX_IMPORTER_IMAGE="${POKEDEX_IMPORTER_IMAGE:-pokecalc/pokedex-importer:0.1.0}"
POKEDEX_SERVER_IMAGE="${POKEDEX_SERVER_IMAGE:-pokecalc/pokedex:0.1.0}"
RECORD_MIGRATE_IMAGE="${RECORD_MIGRATE_IMAGE:-pokecalc/record-migrate:0.1.0}"
TEAM_MIGRATE_IMAGE="${TEAM_MIGRATE_IMAGE:-pokecalc/team-migrate:0.1.0}"
TIDB_CLUSTER_NAME="pokecalc-tidb"

if k3d cluster list 2>/dev/null | awk '{print $1}' | grep -qx "$CLUSTER"; then
  echo "k3d クラスタ '$CLUSTER' は既に存在します"
else
  echo "k3d クラスタ '$CLUSTER' を作成します..."
  k3d cluster create --config deploy/k3d.yaml
fi

# k3d cluster create はコンテキストを 'k3d-<CLUSTER>' に切り替えるため、確認は
# 作成・既存チェックの後に行う(意図しない別クラスタへの適用を防ぐ)。
current_context="$(kubectl config current-context)"
if [ "$current_context" != "k3d-$CLUSTER" ]; then
  echo "up.sh: 現在の kubectl context '$current_context' が 'k3d-$CLUSTER' ではない(別クラスタへ適用してしまうため中断)" >&2
  exit 1
fi

echo "namespace を作成します..."
kubectl apply -f deploy/k8s/base/namespace.yaml

# TiDB Operator の導入(ADR-0211 §3.1)。`make up` は6レーン共通の入口のため、この失敗で
# record/team に無関係な他サービスの起動まで止めない(非致命。tidb_ready=0 の場合、以降の
# TiDB 関連ステップは実行時にすべてスキップする)。
tidb_ready=1
echo "TiDB Operator を導入します..."
if ! ./scripts/tidb-operator-bootstrap.sh; then
  echo "警告: TiDB Operator の導入に失敗。record/team 以外は続行します(TiDB 関連の手順をスキップ)" >&2
  tidb_ready=0
fi

# tidb-root-auth は値を持つため Git に置かない(mysql-auth と同じ理由。ADR-0100 §9)。
# deploy/k8s/overlays/local/tidb の TidbInitializer が参照するため、その適用より前に用意する。
# どのサービス Pod にもマウントしない(ADR-0211 §4)。
if [ "$tidb_ready" = "1" ]; then
  if kubectl -n pokecalc get secret tidb-root-auth >/dev/null 2>&1; then
    echo "Secret 'tidb-root-auth' は既に存在します(既存のパスワードをそのまま使います)..."
  else
    echo "Secret 'tidb-root-auth' が無いので乱数で作成します..."
    tidb_root_pw_value="$(openssl rand -hex 16)"
    tidb_root_auth_manifest="$(mktemp)"
    chmod 600 "$tidb_root_auth_manifest"
    trap 'rm -f "$tidb_root_auth_manifest"' EXIT
    cat > "$tidb_root_auth_manifest" <<SECRET_MANIFEST
apiVersion: v1
kind: Secret
metadata:
  name: tidb-root-auth
  namespace: pokecalc
type: Opaque
stringData:
  root-password: "${tidb_root_pw_value}"
SECRET_MANIFEST
    kubectl apply -f "$tidb_root_auth_manifest"
    rm -f "$tidb_root_auth_manifest"
    trap - EXIT
  fi
fi

# mysql-auth は値を持つため Git に置かない(ADR-0100 §9)。namespace の作成直後、
# 他のリソース(mysql・pokedex-migrate)が参照する前に用意する。
#
# 用途別の最小権限(ADR-0110・issue #104): root(pokedex-dsn)に加えて、pokedex-svc・importer・
# migrate それぞれの DB ユーザー用の DSN を持つ。新規クラスタは4つの DSN を一度に作り、
# 既存クラスタでは無いキーだけを kubectl patch --type=merge で追記する(既存の値は変えない)。
# 値はコマンドライン引数(--from-literal・-p/--patch)にもログ(set -x)にも出さない。
root_pw_key="mysql-root-password"
provision_dsn_key="pokedex-dsn"
reader_dsn_key="pokedex-reader-dsn"
importer_dsn_key="pokedex-importer-dsn"
migrator_dsn_key="pokedex-migrator-dsn"

if kubectl -n pokecalc get secret mysql-auth >/dev/null 2>&1; then
  echo "Secret 'mysql-auth' は既に存在します(無いキーだけ追記します)..."

  # jsonpath はキーが無ければ空文字・終了コード0を返す(kubectl で確認済み)。ここで
  # コマンド自体が失敗する(API サーバに届かない・権限が無い等)場合は、キーが無いのと
  # 区別できず「無いので追記する」に倒れて既存のパスワードを意図せず上書きしうるため、
  # 一時的なエラーは `set -e` にそのまま止めさせる(エラーを黙って握りつぶさない)。
  existing_reader="$(kubectl -n pokecalc get secret mysql-auth -o jsonpath="{.data.${reader_dsn_key}}")"
  existing_importer="$(kubectl -n pokecalc get secret mysql-auth -o jsonpath="{.data.${importer_dsn_key}}")"
  existing_migrator="$(kubectl -n pokecalc get secret mysql-auth -o jsonpath="{.data.${migrator_dsn_key}}")"

  patch_entries=""
  if [ -z "$existing_reader" ]; then
    reader_pw_value="$(openssl rand -hex 16)"
    patch_entries="${patch_entries}  ${reader_dsn_key}: \"pokedex_reader:${reader_pw_value}@tcp(mysql:3306)/pokedex?parseTime=true\"
"
  fi
  if [ -z "$existing_importer" ]; then
    importer_pw_value="$(openssl rand -hex 16)"
    patch_entries="${patch_entries}  ${importer_dsn_key}: \"pokedex_importer:${importer_pw_value}@tcp(mysql:3306)/pokedex?parseTime=true\"
"
  fi
  if [ -z "$existing_migrator" ]; then
    migrator_pw_value="$(openssl rand -hex 16)"
    patch_entries="${patch_entries}  ${migrator_dsn_key}: \"pokedex_migrator:${migrator_pw_value}@tcp(mysql:3306)/pokedex?parseTime=true\"
"
  fi

  if [ -n "$patch_entries" ]; then
    patch_file="$(mktemp)"
    chmod 600 "$patch_file"
    trap 'rm -f "$patch_file"' EXIT
    {
      printf 'stringData:\n'
      printf '%s' "$patch_entries"
    } > "$patch_file"
    kubectl -n pokecalc patch secret mysql-auth --type=merge --patch-file "$patch_file"
    rm -f "$patch_file"
    trap - EXIT
  else
    echo "追加するキーはありません(3ユーザーぶん全て既にあります)"
  fi
else
  echo "Secret 'mysql-auth' が無いので乱数で作成します..."
  root_pw_value="$(openssl rand -hex 16)"
  reader_pw_value="$(openssl rand -hex 16)"
  importer_pw_value="$(openssl rand -hex 16)"
  migrator_pw_value="$(openssl rand -hex 16)"

  mysql_auth_manifest="$(mktemp)"
  chmod 600 "$mysql_auth_manifest"
  trap 'rm -f "$mysql_auth_manifest"' EXIT
  cat > "$mysql_auth_manifest" <<SECRET_MANIFEST
apiVersion: v1
kind: Secret
metadata:
  name: mysql-auth
  namespace: pokecalc
type: Opaque
stringData:
  ${root_pw_key}: "${root_pw_value}"
  ${provision_dsn_key}: "root:${root_pw_value}@tcp(mysql:3306)/pokedex?parseTime=true"
  ${reader_dsn_key}: "pokedex_reader:${reader_pw_value}@tcp(mysql:3306)/pokedex?parseTime=true"
  ${importer_dsn_key}: "pokedex_importer:${importer_pw_value}@tcp(mysql:3306)/pokedex?parseTime=true"
  ${migrator_dsn_key}: "pokedex_migrator:${migrator_pw_value}@tcp(mysql:3306)/pokedex?parseTime=true"
SECRET_MANIFEST
  kubectl apply -f "$mysql_auth_manifest"
  rm -f "$mysql_auth_manifest"
  trap - EXIT
fi

# pokedex-migrate は k3d のノードに直接 import する(ADR-0100 §9)。レジストリを介さない
# ローカル専用の経路。
echo "pokedex-migrate イメージを build して k3d に import します..."
docker build -f services/pokedex/Dockerfile --target migrate -t "$POKEDEX_MIGRATE_IMAGE" .
k3d image import "$POKEDEX_MIGRATE_IMAGE" -c "$CLUSTER"

# pokedex(検索 API・内部 API)の Deployment が使うイメージも、overlay を apply する前に
# k3d へ import しておく(ADR-0105 §6)。
echo "pokedex(server)イメージを build して k3d に import します..."
docker build -f services/pokedex/Dockerfile --target server -t "$POKEDEX_SERVER_IMAGE" .
k3d image import "$POKEDEX_SERVER_IMAGE" -c "$CLUSTER"

# Job は一度作成すると Pod テンプレートを更新できない(kubectl apply が失敗する)ため、
# overlay を apply する前に(実行中の可能性がある正しい Job を消してしまわないよう、
# ここでだけ)古い Job を消しておく。Job の作成は次の overlay apply の1か所だけで行う
# (base/pokedex を単独で apply すると namespace が付かず default に作られてしまうため、
# 二重に apply しない)。Job の削除はデータ削除ではない(CLAUDE.md の対象外)。
echo "pokedex-migrate の古い Job を消します(あれば)..."
kubectl -n pokecalc delete job pokedex-migrate --ignore-not-found

echo "Kustomize(overlays/local)を適用します(mysql・pokedex-migrate Job を含む)..."
kubectl apply -k deploy/k8s/overlays/local

echo "MySQL の Ready を待ちます..."
kubectl -n pokecalc rollout status statefulset/mysql --timeout=180s

# Job 自体は MySQL の起動前に走り始めても、initContainer が MySQL への接続を待つため
# 成功する(deploy/k8s/base/pokedex/job-migrate.yaml)。ここでは Job の完了を確認し、
# 失敗していれば up.sh を失敗させる。
echo "pokedex-migrate Job の完了を待ちます..."
kubectl -n pokecalc wait --for=condition=complete job/pokedex-migrate --timeout=300s

# TiDB(record-svc/team-svc用。ADR-0211)。bootstrap 済みのときだけ進める。overlays/local/tidb は
# 独立した kustomization のまま、ここで別の kubectl apply -k として非致命的に適用する
# (base・mysql を含む上の overlay apply と同じコマンドに混ぜると、CRD が無いときにそちら全体を
# 巻き込んで失敗させてしまうため。ADR-0211 §3.2 実装時の追記)。
if [ "$tidb_ready" = "1" ]; then
  echo "TiDB(overlays/local/tidb)を適用します..."
  if ! kubectl apply -k deploy/k8s/overlays/local/tidb; then
    echo "警告: overlays/local/tidb の適用に失敗。record/team の migrate はスキップします" >&2
    tidb_ready=0
  fi
fi

if [ "$tidb_ready" = "1" ]; then
  echo "TiDB(PD/TiKV/TiDB)の Ready を待ちます..."
  if ! kubectl -n pokecalc rollout status "statefulset/${TIDB_CLUSTER_NAME}-pd" --timeout=300s \
    || ! kubectl -n pokecalc rollout status "statefulset/${TIDB_CLUSTER_NAME}-tikv" --timeout=300s \
    || ! kubectl -n pokecalc rollout status "statefulset/${TIDB_CLUSTER_NAME}-tidb" --timeout=300s; then
    echo "警告: TiDB(PD/TiKV/TiDB)が Ready になりませんでした。record/team の migrate はスキップします" >&2
    tidb_ready=0
  fi
fi

if [ "$tidb_ready" = "1" ]; then
  echo "TidbInitializer(record・team の DB 作成・root パスワード設定)の完了を待ちます..."
  if ! kubectl -n pokecalc wait --for=jsonpath='{.status.phase}'=Completed tidbinitializer/pokecalc --timeout=180s; then
    echo "警告: TidbInitializer が完了しませんでした。record/team の migrate はスキップします" >&2
    tidb_ready=0
  fi
fi

if [ "$tidb_ready" = "1" ]; then
  # record-db-auth・team-db-auth は値を持つため Git に置かない(mysql-auth と同じ理由)。
  # root パスワードは tidb-root-auth の値を使い(このコマンド内では表示しない)、
  # ここで新しく生成するのは app・migrator の2ロールぶんのパスワードだけ(ADR-0211 §4)。
  # 各サービスの Secret は分ける(record が team のキーを参照できる形にしない。ADR-0211 §4 却下案)。
  tidb_root_pw="$(kubectl -n pokecalc get secret tidb-root-auth -o jsonpath='{.data.root-password}' | base64 -d)"
  tidb_host="${TIDB_CLUSTER_NAME}-tidb"

  create_db_auth_secret() {
    # 引数: secret名 db名 provisionキー migratorキー appキー
    local secret_name="$1" db_name="$2" provision_key="$3" migrator_key="$4" app_key="$5"
    if kubectl -n pokecalc get secret "$secret_name" >/dev/null 2>&1; then
      echo "Secret '${secret_name}' は既に存在します(無いキーだけ追記します)..."
      local existing_app existing_migrator
      existing_app="$(kubectl -n pokecalc get secret "$secret_name" -o jsonpath="{.data.${app_key}}")"
      existing_migrator="$(kubectl -n pokecalc get secret "$secret_name" -o jsonpath="{.data.${migrator_key}}")"
      local patch_entries=""
      if [ -z "$existing_app" ]; then
        local app_pw_value; app_pw_value="$(openssl rand -hex 16)"
        patch_entries="${patch_entries}  ${app_key}: \"${db_name}_app:${app_pw_value}@tcp(${tidb_host}:4000)/${db_name}?parseTime=true\"
"
      fi
      if [ -z "$existing_migrator" ]; then
        local migrator_pw_value; migrator_pw_value="$(openssl rand -hex 16)"
        patch_entries="${patch_entries}  ${migrator_key}: \"${db_name}_migrator:${migrator_pw_value}@tcp(${tidb_host}:4000)/${db_name}?parseTime=true\"
"
      fi
      if [ -n "$patch_entries" ]; then
        local patch_file; patch_file="$(mktemp)"
        chmod 600 "$patch_file"
        { printf 'stringData:\n'; printf '%s' "$patch_entries"; } > "$patch_file"
        kubectl -n pokecalc patch secret "$secret_name" --type=merge --patch-file "$patch_file"
        rm -f "$patch_file"
      else
        echo "追加するキーはありません(app・migratorぶん全て既にあります)"
      fi
    else
      echo "Secret '${secret_name}' が無いので乱数で作成します..."
      local app_pw_value migrator_pw_value
      app_pw_value="$(openssl rand -hex 16)"
      migrator_pw_value="$(openssl rand -hex 16)"
      local manifest; manifest="$(mktemp)"
      chmod 600 "$manifest"
      cat > "$manifest" <<SECRET_MANIFEST
apiVersion: v1
kind: Secret
metadata:
  name: ${secret_name}
  namespace: pokecalc
type: Opaque
stringData:
  ${provision_key}: "root:${tidb_root_pw}@tcp(${tidb_host}:4000)/${db_name}?parseTime=true"
  ${migrator_key}: "${db_name}_migrator:${migrator_pw_value}@tcp(${tidb_host}:4000)/${db_name}?parseTime=true"
  ${app_key}: "${db_name}_app:${app_pw_value}@tcp(${tidb_host}:4000)/${db_name}?parseTime=true"
SECRET_MANIFEST
      kubectl apply -f "$manifest"
      rm -f "$manifest"
    fi
  }

  create_db_auth_secret record-db-auth record record-provision-dsn record-migrator-dsn record-app-dsn
  create_db_auth_secret team-db-auth team team-provision-dsn team-migrator-dsn team-app-dsn
  unset tidb_root_pw

  echo "record-migrate イメージを build して k3d に import します..."
  docker build -f services/record/Dockerfile --target migrate -t "$RECORD_MIGRATE_IMAGE" .
  k3d image import "$RECORD_MIGRATE_IMAGE" -c "$CLUSTER"
  echo "team-migrate イメージを build して k3d に import します..."
  docker build -f services/team/Dockerfile --target migrate -t "$TEAM_MIGRATE_IMAGE" .
  k3d image import "$TEAM_MIGRATE_IMAGE" -c "$CLUSTER"

  # record-migrate・team-migrate は overlay の kustomize resources に含めない(ADR-0211 §3.2・「影響」)ため、
  # pokedex-migrate と同様に古い Job を消してから個別に apply する。
  echo "record-migrate・team-migrate の古い Job を消します(あれば)..."
  kubectl -n pokecalc delete job record-migrate team-migrate --ignore-not-found

  echo "record-migrate・team-migrate Job を適用します..."
  kubectl apply -f deploy/k8s/base/record/job-migrate.yaml
  kubectl apply -f deploy/k8s/base/team/job-migrate.yaml

  echo "record-migrate・team-migrate Job の完了を待ちます..."
  kubectl -n pokecalc wait --for=condition=complete job/record-migrate --timeout=300s
  kubectl -n pokecalc wait --for=condition=complete job/team-migrate --timeout=300s
fi

# pokedex-import(CronJob。ADR-0104)が使うイメージも build して k3d に import する。
# CronJob の spec は apply で更新できる(Job と違い事前の delete は不要)。
echo "pokedex-import(importer)イメージを build して k3d に import します..."
docker build -f services/pokedex/Dockerfile --target importer -t "$POKEDEX_IMPORTER_IMAGE" .
k3d image import "$POKEDEX_IMPORTER_IMAGE" -c "$CLUSTER"

# override(日本語名の上書き。任意・Git 管理外の実データ)があるときだけ ConfigMap を作成/更新する。
# 無いときは何もしない(既存の ConfigMap も消さない。消すのは人。ADR-0104 §7)。
if [ -f "data/local/name_ja_overrides.json" ]; then
  echo "override(data/local/name_ja_overrides.json)から ConfigMap 'pokedex-name-overrides' を作成/更新します..."
  kubectl -n pokecalc create configmap pokedex-name-overrides \
    --from-file="name_ja_overrides.json=data/local/name_ja_overrides.json" \
    --dry-run=client -o yaml | kubectl apply -f -
else
  echo "override(data/local/name_ja_overrides.json)が無いので ConfigMap 'pokedex-name-overrides' は作成しません"
fi

echo
echo "完了。gateway 実装後は http://localhost:8080 で計算画面にアクセスできます。"
echo "pokedex-import の初回投入は 'make import-k8s' で手動で1回流してください" \
  "(CronJob は週1回・土曜 12:00 JST に自動実行されます。初回はネットワークが要るため自動では流しません)。"
