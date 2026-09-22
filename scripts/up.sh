#!/usr/bin/env bash
# k3d クラスタを作成(なければ)し、ローカル overlay を適用する。
# サービスの Deployment はフェーズ進行に応じて base/kustomization.yaml へ追加される。
set -euo pipefail
cd "$(dirname "$0")/.."

CLUSTER="${CLUSTER:-pokecalc}"
POKEDEX_MIGRATE_IMAGE="${POKEDEX_MIGRATE_IMAGE:-pokecalc/pokedex-migrate:0.1.0}"
POKEDEX_IMPORTER_IMAGE="${POKEDEX_IMPORTER_IMAGE:-pokecalc/pokedex-importer:0.1.0}"
POKEDEX_SERVER_IMAGE="${POKEDEX_SERVER_IMAGE:-pokecalc/pokedex:0.1.0}"

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

# mysql-auth は値を持つため Git に置かない(ADR-0100 §9)。namespace の作成直後、
# 他のリソース(mysql・pokedex-migrate)が参照する前に用意する。無いときだけ乱数で
# 作る(既存は上書きしない)。
if kubectl -n pokecalc get secret mysql-auth >/dev/null 2>&1; then
  echo "Secret 'mysql-auth' は既に存在します(上書きしません)"
else
  echo "Secret 'mysql-auth' が無いので乱数で作成します..."
  root_pw_key="mysql-root-password"
  dsn_key="pokedex-dsn"
  root_pw_value="$(openssl rand -hex 16)"
  kubectl -n pokecalc create secret generic mysql-auth \
    --from-literal="${root_pw_key}=${root_pw_value}" \
    --from-literal="${dsn_key}=root:${root_pw_value}@tcp(mysql:3306)/pokedex?parseTime=true"
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
