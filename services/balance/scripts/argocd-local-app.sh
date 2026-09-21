#!/usr/bin/env bash
# balance の Argo CD Application をクラスタに適用する(TB0。ADR-0018)。
# repoURL にはアカウント名が入るので Git には書かず、`git remote get-url origin` から適用時に埋め込む。
# 認証情報は Argo CD の repository Secret(ユーザーが登録)にあり、ここでは扱わない。
set -euo pipefail

balance_dir=${BALANCE_DIR:-services/balance}
repo_url=${BALANCE_GITOPS_REPO_URL:-$(git remote get-url origin)}

# 許可する文字だけの HTTPS URL(資格情報・query・fragment・sed の特殊文字を含まない)
if ! printf '%s\n' "$repo_url" | grep -Eq '^https://[A-Za-z0-9._~/-]+$'; then
  echo "repoURL must be a plain HTTPS clone URL (no credentials, query or special characters)" >&2
  exit 1
fi

BALANCE_GITOPS_REPO_URL="$repo_url" "$balance_dir/scripts/check-gitops.sh" ready

rendered=$(mktemp)
trap 'rm -f "$rendered"' EXIT
kubectl kustomize "$balance_dir/deploy/argocd" \
  | sed "s#repoURL: https://git.example.invalid/pokecalc.git#repoURL: ${repo_url}#" >"$rendered"

# 置換がちょうど1か所で、placeholder が残っていないことを確かめてから適用する。
if [ "$(grep -cxF "    repoURL: ${repo_url}" "$rendered")" != "1" ] || grep -q 'example.invalid' "$rendered"; then
  echo "failed to render the Application with the repository URL" >&2
  exit 1
fi
kubectl apply -f "$rendered"
