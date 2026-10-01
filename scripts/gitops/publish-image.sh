#!/usr/bin/env sh
# balance/speed 共通: リリース用イメージを push し、追跡できる digest 参照を表示する
# (ADR-0408 §2、issue #263)。SERVICE=balance または SERVICE=speed を必須の環境変数として受け取る。
set -eu

case "${SERVICE:-}" in
  balance | speed) ;;
  *)
    echo "publish-image.sh: SERVICE must be 'balance' or 'speed' (got '${SERVICE:-}')" >&2
    exit 1
    ;;
esac

svc_dir="services/$SERVICE"
svc_upper=$(printf '%s' "$SERVICE" | tr '[:lower:]' '[:upper:]')

fail() {
  echo "$SERVICE image publish failed: $1" >&2
  exit 1
}

image_var="${svc_upper}_RELEASE_IMAGE"
eval "image=\${${image_var}:-}"
platforms_var="${svc_upper}_PLATFORMS"
eval "platforms=\${${platforms_var}:-linux/amd64,linux/arm64}"

[ -n "$image" ] || fail "$image_var is required"
case "$image" in
  *@sha256:*) fail "$image_var must be a pushable tag, not a digest" ;;
  *:latest | *:local) fail "latest and local tags are not publishable" ;;
  *:*) ;;
  *) fail "$image_var must include an immutable release tag" ;;
esac
case "$image" in
  *example.invalid* | *REPLACE_*) fail "image repository placeholder has not been replaced" ;;
esac
printf '%s\n' "$image" | grep -Eq '^[A-Za-z0-9._:/-]+$' || fail "$image_var contains unsupported characters"
printf '%s\n' "$platforms" | grep -Eq '^[A-Za-z0-9_./,-]+$' || fail "$platforms_var contains unsupported characters"

# speed との違い: speed のイメージは engine を含むので、ビルドコンテキストはリポジトリのルート(ADR-0600 §2・ADR-0605 §2)。
case "$SERVICE" in
  balance) docker buildx build --platform "$platforms" --push --tag "$image" "$svc_dir" ;;
  speed) docker buildx build --platform "$platforms" --push --tag "$image" -f "$svc_dir/Dockerfile" . ;;
esac
digest=$(docker buildx imagetools inspect "$image" | awk '$1 == "Digest:" { print $2; exit }')
printf '%s\n' "$digest" | grep -Eq '^sha256:[0-9a-f]{64}$' || fail "registry did not return a sha256 digest"

repository=${image%:*}
echo "published immutable image reference: $repository@$digest"
