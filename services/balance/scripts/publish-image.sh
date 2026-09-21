#!/usr/bin/env sh
set -eu

image=${BALANCE_RELEASE_IMAGE:-}
balance_dir=${BALANCE_DIR:-services/balance}
platforms=${BALANCE_PLATFORMS:-linux/amd64,linux/arm64}

fail() {
  echo "balance image publish failed: $1" >&2
  exit 1
}

[ -n "$image" ] || fail "BALANCE_RELEASE_IMAGE is required"
case "$image" in
  *@sha256:*) fail "BALANCE_RELEASE_IMAGE must be a pushable tag, not a digest" ;;
  *:latest|*:local) fail "latest and local tags are not publishable" ;;
  *:*) ;;
  *) fail "BALANCE_RELEASE_IMAGE must include an immutable release tag" ;;
esac
case "$image" in
  *example.invalid*|*REPLACE_*) fail "image repository placeholder has not been replaced" ;;
esac
printf '%s\n' "$image" | grep -Eq '^[A-Za-z0-9._:/-]+$' || fail "BALANCE_RELEASE_IMAGE contains unsupported characters"
printf '%s\n' "$platforms" | grep -Eq '^[A-Za-z0-9_./,-]+$' || fail "BALANCE_PLATFORMS contains unsupported characters"

docker buildx build --platform "$platforms" --push --tag "$image" "$balance_dir"
digest=$(docker buildx imagetools inspect "$image" | awk '$1 == "Digest:" { print $2; exit }')
printf '%s\n' "$digest" | grep -Eq '^sha256:[0-9a-f]{64}$' || fail "registry did not return a sha256 digest"

repository=${image%:*}
echo "published immutable image reference: $repository@$digest"
