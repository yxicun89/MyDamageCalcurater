#!/usr/bin/env sh
set -eu

mode=${1:-ready}
balance_dir=${BALANCE_DIR:-services/balance}
application_file=$balance_dir/deploy/argocd/application.yaml
overlay_file=$balance_dir/deploy/k8s/overlays/gitops/kustomization.yaml

fail() {
  echo "balance GitOps check failed: $1" >&2
  exit 1
}

case "$mode" in
  template|ready) ;;
  *) fail "mode must be template or ready" ;;
esac

[ -f "$application_file" ] || fail "Application manifest is missing"
[ -f "$overlay_file" ] || fail "GitOps overlay is missing"

repo_url=$(awk '$1 == "repoURL:" { print $2; exit }' "$application_file")
target_revision=$(awk '$1 == "targetRevision:" { print $2; exit }' "$application_file")
source_path=$(awk '$1 == "path:" { print $2; exit }' "$application_file")
image_name=$(awk '$1 == "newName:" { print $2; exit }' "$overlay_file")
image_digest=$(awk '$1 == "digest:" { print $2; exit }' "$overlay_file")

[ -n "$repo_url" ] || fail "repoURL is missing"
[ -n "$target_revision" ] || fail "targetRevision is missing"
[ "$source_path" = "services/balance/deploy/k8s/overlays/gitops" ] || fail "Application source path must use the GitOps overlay"
[ -n "$image_name" ] || fail "image newName is missing"
printf '%s\n' "$image_digest" | grep -Eq '^sha256:[0-9a-f]{64}$' || fail "image digest must be sha256"
if grep -Eq '^[[:space:]]*newTag:' "$overlay_file"; then
  fail "GitOps image must use digest instead of a tag"
fi

if grep -Eq '(^|[[:space:]])automated:' "$application_file"; then
  fail "automated sync must remain disabled for TB0"
fi

if [ "$mode" = "template" ]; then
  echo "balance GitOps template: valid"
  exit 0
fi

case "$repo_url" in
  https://*|ssh://git@*|git@*:*) ;;
  *) fail "repoURL must be an HTTPS or SSH Git clone URL" ;;
esac
case "$repo_url" in
  https://*@*|ssh://*:*@*|*\?*|*\#*) fail "repoURL must not contain embedded credentials or query data" ;;
esac
case "$repo_url" in
  *example.invalid*|*REPLACE_*) fail "repoURL placeholder has not been replaced" ;;
esac
case "$image_name" in
  *example.invalid*|*REPLACE_*) fail "image repository placeholder has not been replaced" ;;
  *@*) fail "image newName must not include a tag or digest" ;;
esac
if [ "$image_digest" = "sha256:0000000000000000000000000000000000000000000000000000000000000000" ]; then
  fail "image digest placeholder has not been replaced"
fi

echo "balance GitOps configuration: ready"
