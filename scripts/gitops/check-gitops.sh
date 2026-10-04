#!/usr/bin/env sh
# balance/speed 共通の Argo CD GitOps 設定チェック(ADR-0408 §2、issue #263)。
# SERVICE=balance または SERVICE=speed を必須の環境変数として受け取る。
# 呼び出し方はこれまでと同じ($SERVICE_DIR/scripts/check-gitops.sh ではなく scripts/gitops/check-gitops.sh に変わっただけ)。
set -eu

case "${SERVICE:-}" in
  balance | speed) ;;
  *)
    echo "check-gitops.sh: SERVICE must be 'balance' or 'speed' (got '${SERVICE:-}')" >&2
    exit 1
    ;;
esac

mode=${1:-ready}
# --no-live: live の Deployment の注釈検査を省く。Application を最初に作るときだけ(scripts/gitops/argocd-local-app.sh)。
# 作成前は手動 overlay の上書きが残っているのが普通で、そのまま止めると Application を作れず sync で戻せない(鶏と卵)。
live_check=1
if [ "${2:-}" = "--no-live" ]; then
  live_check=0
fi
svc_dir="services/$SERVICE"
svc_upper=$(printf '%s' "$SERVICE" | tr '[:lower:]' '[:upper:]')
application_file="$svc_dir/deploy/argocd/application.yaml"
overlay_file="$svc_dir/deploy/k8s/overlays/gitops/kustomization.yaml"

fail() {
  echo "$SERVICE GitOps check failed: $1" >&2
  exit 1
}

case "$mode" in
  template | ready) ;;
  *) fail "mode must be template or ready" ;;
esac

[ -f "$application_file" ] || fail "Application manifest is missing"
[ -f "$overlay_file" ] || fail "GitOps overlay is missing"

# repoURL はアカウント名を含むので Git に書かない(ADR-0018)。ファイルは placeholder のままであることを必須にし、
# 適用時の値は ready モードでだけ ${SERVICE}_GITOPS_REPO_URL から受け取る(scripts/gitops/argocd-local-app.sh)。
file_repo_url=$(awk '$1 == "repoURL:" { print $2; exit }' "$application_file")
[ "$file_repo_url" = "https://git.example.invalid/pokecalc.git" ] || fail "application.yaml repoURL must stay the placeholder (the real URL must not be committed)"

# spec.project は AppProject pokecalc に限定する(project: default は任意の repo・namespace・cluster スコープ
# 資源への同期を許してしまう。ADR-0408 §1)。
project=$(awk '$1 == "project:" { print $2; exit }' "$application_file")
[ "$project" = "pokecalc" ] || fail "spec.project must be pokecalc, not '$project' (ADR-0408 §1: AppProject pokecalc に限定する)"

repo_url=$file_repo_url
repo_url_var="${svc_upper}_GITOPS_REPO_URL"
if [ "$mode" = "ready" ]; then
  eval "repo_url=\${${repo_url_var}:-\$file_repo_url}"
fi

target_revision=$(awk '$1 == "targetRevision:" { print $2; exit }' "$application_file")
source_path=$(awk '$1 == "path:" { print $2; exit }' "$application_file")
# images は name ごとに値を取る(先頭一致だと pokedex など別 image の値と取り違える。ADR-0412 §3)。
image_field() {
  awk -v want="pokecalc/$1" -v field="$2" '
    $1 == "-" && $2 == "name:" { cur = $3; next }
    $1 == "name:" { cur = $2; next }
    cur == want && $1 == field ":" { print $2; exit }
  ' "$overlay_file"
}
image_name=$(image_field "$SERVICE" newName)
image_digest=$(image_field "$SERVICE" digest)
pokedex_name=$(image_field pokedex newName)
pokedex_digest=$(image_field pokedex digest)

[ -n "$repo_url" ] || fail "repoURL is missing"
[ -n "$target_revision" ] || fail "targetRevision is missing"
[ "$source_path" = "$svc_dir/deploy/k8s/overlays/gitops" ] || fail "Application source path must use the GitOps overlay"
[ -n "$image_name" ] || fail "image newName is missing"
printf '%s\n' "$image_digest" | grep -Eq '^sha256:[0-9a-f]{64}$' || fail "image digest must be sha256"
if grep -Eq '^[[:space:]]*newTag:' "$overlay_file"; then
  fail "GitOps image must use digest instead of a tag"
fi

if grep -Eq '(^|[[:space:]])automated:' "$application_file"; then
  fail "automated sync must remain disabled (ADR-0408 §4)"
fi

# read model の供給経路(ADR-0412): pokedex image は digest 固定、描画に initContainer があり ConfigMap が無いこと。
[ -n "$pokedex_name" ] || fail "images に pokecalc/pokedex の newName が無い(read model の initContainer 用。ADR-0412)"
printf '%s\n' "$pokedex_digest" | grep -Eq '^sha256:[0-9a-f]{64}$' || fail "pokedex image digest must be sha256 (ADR-0412)"
rendered=$(kubectl kustomize "$svc_dir/deploy/k8s/overlays/gitops") || fail "kubectl kustomize of the GitOps overlay failed"
printf '%s\n' "$rendered" | grep -q 'name: readmodel-export' || fail "rendered Deployment has no readmodel-export initContainer (read model の供給経路が無い。ADR-0412)"
if printf '%s\n' "$rendered" | grep -Eq '^kind: (ConfigMap|Secret)$'; then
  fail "GitOps overlay must not render a ConfigMap or Secret (read model は initContainer で作る。ADR-0002・ADR-0412)"
fi

if [ "$mode" = "template" ]; then
  echo "$SERVICE GitOps template: valid"
  exit 0
fi

# 手動 overlay(local-readmodel)が生きている Deployment に残っていたら Argo CD と取り合う(ADR-0412 §5)。
# クラスタに届かないときは検査しない。
live_annotations=""
if [ "$live_check" = 1 ]; then
  live_annotations=$(kubectl --context "k3d-${CLUSTER:-pokecalc}" -n pokecalc get deployment "$SERVICE" -o jsonpath='{.spec.template.metadata.annotations}' 2>/dev/null || true)
fi
case "$live_annotations" in
  *readmodel-hash*) fail "live Deployment has the local-readmodel annotation pokecalc.example/readmodel-hash (手動 overlay の上書きが残っている。argocd app sync pokecalc-$SERVICE で gitops overlay に戻す)" ;;
esac

if [ "$pokedex_digest" = "sha256:0000000000000000000000000000000000000000000000000000000000000000" ]; then
  fail "pokedex image digest placeholder has not been replaced (make pokedex-registry-push の digest を書く。ADR-0412)"
fi

case "$repo_url" in
  https://* | ssh://git@* | git@*:*) ;;
  *) fail "repoURL must be an HTTPS or SSH Git clone URL" ;;
esac
case "$repo_url" in
  https://*@* | ssh://*:*@* | *\?* | *\#*) fail "repoURL must not contain embedded credentials or query data" ;;
esac
case "$repo_url" in
  *example.invalid* | *REPLACE_*) fail "repoURL placeholder has not been replaced" ;;
esac
case "$image_name" in
  *example.invalid* | *REPLACE_*) fail "image repository placeholder has not been replaced" ;;
  *@*) fail "image newName must not include a tag or digest" ;;
esac
if [ "$image_digest" = "sha256:0000000000000000000000000000000000000000000000000000000000000000" ]; then
  fail "image digest placeholder has not been replaced"
fi

echo "$SERVICE GitOps configuration: ready"
