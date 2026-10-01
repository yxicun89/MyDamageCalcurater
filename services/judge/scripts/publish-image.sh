#!/usr/bin/env sh
set -eu

image=${JUDGE_RELEASE_IMAGE:-}
judge_dir=${JUDGE_DIR:-services/judge}
platforms=${JUDGE_PLATFORMS:-linux/amd64,linux/arm64}

fail() {
  echo "judge image publish failed: $1" >&2
  exit 1
}

[ -n "$image" ] || fail "JUDGE_RELEASE_IMAGE is required"
case "$image" in
  *@sha256:*) fail "JUDGE_RELEASE_IMAGE must be a pushable tag, not a digest" ;;
  *:latest|*:local) fail "latest and local tags are not publishable" ;;
  *:*) ;;
  *) fail "JUDGE_RELEASE_IMAGE must include an immutable release tag" ;;
esac
case "$image" in
  *example.invalid*|*REPLACE_*) fail "image repository placeholder has not been replaced" ;;
esac
printf '%s\n' "$image" | grep -Eq '^[A-Za-z0-9._:/-]+$' || fail "JUDGE_RELEASE_IMAGE contains unsupported characters"
printf '%s\n' "$platforms" | grep -Eq '^[A-Za-z0-9_./,-]+$' || fail "JUDGE_PLATFORMS contains unsupported characters"

# engine を含むため、ビルドコンテキストはリポジトリのルート(ADR-0700 §1・ADR-0709 §2)。
docker buildx build --platform "$platforms" --push --tag "$image" -f "$judge_dir/Dockerfile" .
digest=$(docker buildx imagetools inspect "$image" | awk '$1 == "Digest:" { print $2; exit }')
printf '%s\n' "$digest" | grep -Eq '^sha256:[0-9a-f]{64}$' || fail "registry did not return a sha256 digest"

repository=${image%:*}
echo "published immutable image reference: $repository@$digest"
