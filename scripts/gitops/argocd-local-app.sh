#!/usr/bin/env bash
# balance/speed 共通の Argo CD Application 適用スクリプト(ADR-0408 §1・§2、issue #263)。
# SERVICE=balance または SERVICE=speed を必須の環境変数として受け取る。
# AppProject(deploy/argocd/appproject.yaml)を Application より先に適用する。逆順だと Application の
# repo が AppProject の sourceRepos にまだ許可されておらず Argo CD に弾かれる。
# repoURL にはアカウント名が入るので Git には書かず、`git remote get-url origin` から適用時に埋め込む。
# 認証情報は Argo CD の repository Secret(ユーザーが登録)にあり、ここでは扱わない。
set -euo pipefail

case "${SERVICE:-}" in
  balance | speed) ;;
  *)
    echo "argocd-local-app.sh: SERVICE must be 'balance' or 'speed' (got '${SERVICE:-}')" >&2
    exit 1
    ;;
esac

script_dir=$(cd "$(dirname "$0")" && pwd)
svc_dir="services/$SERVICE"
svc_upper=$(printf '%s' "$SERVICE" | tr '[:lower:]' '[:upper:]')
repo_url_var="${svc_upper}_GITOPS_REPO_URL"

eval "repo_url=\${${repo_url_var}:-}"
if [ -z "$repo_url" ]; then
  repo_url=$(git remote get-url origin)
fi

# 許可する文字だけの HTTPS URL(資格情報・query・fragment・sed の特殊文字を含まない)
if ! printf '%s\n' "$repo_url" | grep -Eq '^https://[A-Za-z0-9._~/-]+$'; then
  echo "repoURL must be a plain HTTPS clone URL (no credentials, query or special characters)" >&2
  exit 1
fi

env "SERVICE=$SERVICE" "$repo_url_var=$repo_url" "$script_dir/check-gitops.sh" ready --no-live

project_rendered=$(mktemp)
app_rendered=$(mktemp)
combined=$(mktemp)
trap 'rm -f "$project_rendered" "$app_rendered" "$combined"' EXIT

sed "s#https://git.example.invalid/pokecalc.git#${repo_url}#" deploy/argocd/appproject.yaml >"$project_rendered"
if [ "$(grep -cxF "    - ${repo_url}" "$project_rendered")" != "1" ] || grep -q 'example.invalid' "$project_rendered"; then
  echo "failed to render the AppProject with the repository URL" >&2
  exit 1
fi

kubectl kustomize "$svc_dir/deploy/argocd" \
  | sed "s#repoURL: https://git.example.invalid/pokecalc.git#repoURL: ${repo_url}#" >"$app_rendered"

# 置換がちょうど1か所で、placeholder が残っていないことを確かめてから適用する。
if [ "$(grep -cxF "    repoURL: ${repo_url}" "$app_rendered")" != "1" ] || grep -q 'example.invalid' "$app_rendered"; then
  echo "failed to render the Application with the repository URL" >&2
  exit 1
fi

{
  cat "$project_rendered"
  echo "---"
  cat "$app_rendered"
} >"$combined"

# AppProject を先に、Application を後に(1つの apply 呼び出しに1ファイルとして渡し、適用順を保証する)。
kubectl apply -f "$combined"
