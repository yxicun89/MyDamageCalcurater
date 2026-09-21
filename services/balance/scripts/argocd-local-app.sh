#!/usr/bin/env bash
# balance の Argo CD Application をクラスタに適用する(TB0。ADR-0018)。
# repoURL にはアカウント名が入るので Git には書かず、`git remote get-url origin` から適用時に埋め込む。
# 認証情報は Argo CD の repository Secret(ユーザーが登録)にあり、ここでは扱わない。
set -euo pipefail

balance_dir=${BALANCE_DIR:-services/balance}
repo_url=${BALANCE_GITOPS_REPO_URL:-$(git remote get-url origin)}

case "$repo_url" in
  https://*) ;;
  *) echo "repoURL must be an HTTPS clone URL" >&2; exit 1 ;;
esac
case "$repo_url" in
  https://*@*|*\?*|*\#*) echo "repoURL must not contain credentials or query data" >&2; exit 1 ;;
esac

BALANCE_GITOPS_REPO_URL="$repo_url" "$balance_dir/scripts/check-gitops.sh" ready

kubectl kustomize "$balance_dir/deploy/argocd" \
  | sed "s#repoURL: https://git.example.invalid/pokecalc.git#repoURL: ${repo_url}#" \
  | kubectl apply -f -
