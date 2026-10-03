#!/usr/bin/env bash
# 画像の k3d 配線(ADR-0807 追記)の静的検査。make test-scripts から流す。クラスタ・ネットワークには触らない。
#   - kustomize の描画: local 系(local・local-api)の gateway に hostPath が1つだけ・readOnly・
#     GATEWAY_IMAGES_DIR が volumeMount の mountPath と一致。base・cloud の描画に hostPath が無い
#   - scripts/images-k3d.sh: manifest が無ければ 0(docker に触らない)・あれば dist の中身だけを docker cp
set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
readonly ROOT

failures=0
ok() { echo "ok: $1"; }
ng() { echo "NG: $1" >&2; failures=$((failures + 1)); }

command -v kubectl >/dev/null 2>&1 || { echo "images-k3d_test: kubectl が無い(brew install kubectl)" >&2; exit 1; }

for ov in local local-api; do
  out=$(kubectl kustomize "$ROOT/deploy/k8s/overlays/$ov" 2>&1) || { ng "$ov が描画できない"; continue; }
  # Deployment 単位に分けて gateway を取り出す
  gw=$(printf '%s\n' "$out" | awk 'BEGIN{RS="---\n"} /kind: Deployment/ && /name: gateway/ {print}')
  [ "$(printf '%s\n' "$out" | grep -c 'hostPath:')" = 1 ] && ok "$ov: hostPath は全体で1つ" || ng "$ov: hostPath が1つではない"
  [ "$(printf '%s\n' "$gw" | grep -c 'hostPath:')" = 1 ] && ok "$ov: hostPath は gateway にある" || ng "$ov: gateway に hostPath が無い"
  printf '%s\n' "$gw" | grep -q 'type: DirectoryOrCreate' && ok "$ov: DirectoryOrCreate" || ng "$ov: DirectoryOrCreate が無い"
  printf '%s\n' "$gw" | grep -A3 'name: images' | grep -q 'readOnly: true' && ok "$ov: volumeMount は readOnly" || ng "$ov: volumeMount が readOnly ではない"
  dir=$(printf '%s\n' "$gw" | grep -A1 'name: GATEWAY_IMAGES_DIR' | sed -n 's/.*value: //p')
  mnt=$(printf '%s\n' "$gw" | grep -B2 'readOnly: true' | sed -n 's/.*mountPath: //p')
  if [ -n "$dir" ] && [ "$dir" = "$mnt" ]; then ok "$ov: GATEWAY_IMAGES_DIR($dir) = mountPath"; else ng "$ov: GATEWAY_IMAGES_DIR('$dir') と mountPath('$mnt') が一致しない"; fi
  printf '%s\n' "$gw" | grep -q 'readOnlyRootFilesystem: true' && ok "$ov: readOnlyRootFilesystem は維持" || ng "$ov: readOnlyRootFilesystem が無い"
done

for ov in base overlays/cloud; do
  out=$(kubectl kustomize "$ROOT/deploy/k8s/$ov" 2>&1) || { ng "$ov が描画できない"; continue; }
  if printf '%s\n' "$out" | grep -q 'hostPath:'; then ng "$ov に hostPath がある(クラウドへ持ち込まない)"; else ok "$ov: hostPath なし"; fi
  if printf '%s\n' "$out" | grep -q 'GATEWAY_IMAGES_DIR'; then ng "$ov に GATEWAY_IMAGES_DIR がある"; else ok "$ov: GATEWAY_IMAGES_DIR なし"; fi
done

# --- スクリプト ---
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
repo="$work/repo"
mkdir -p "$repo/scripts" "$work/bin" "$repo/out"
cp "$ROOT/scripts/images-k3d.sh" "$ROOT/scripts/require-k3d-context.sh" "$repo/scripts/"
git -C "$repo" init -q
cat > "$work/bin/docker" <<'FAKE'
#!/usr/bin/env bash
echo "docker $*" >> "${FAKE_LOG:?}"
FAKE
cat > "$work/bin/kubectl" <<'FAKE'
#!/usr/bin/env bash
[ "$*" = "config current-context" ] && echo "${FAKE_KUBE_CONTEXT:-k3d-pokecalc}"
exit 0
FAKE
chmod +x "$work/bin/"*
export FAKE_LOG="$work/calls.log"

: > "$FAKE_LOG"
(cd "$repo" && PATH="$work/bin:$PATH" ASSETS_OUT="$repo/out" bash scripts/images-k3d.sh) > "$work/o1" 2>&1
rc=$?
[ "$rc" = 0 ] && grep -q '画像なし' "$work/o1" && [ ! -s "$FAKE_LOG" ] && ok "manifest が無ければ 0・docker に触らない" || ng "manifest なしの挙動 rc=$rc"

echo '{}' > "$repo/out/manifest.json"
: > "$FAKE_LOG"
(cd "$repo" && PATH="$work/bin:$PATH" ASSETS_OUT="$repo/out" bash scripts/images-k3d.sh) > "$work/o2" 2>&1
rc=$?
[ "$rc" = 0 ] && ok "manifest ありで 0" || ng "manifest ありが失敗 rc=$rc"
grep -q "docker cp $repo/out/. k3d-pokecalc-server-0:/var/lib/pokecalc-images" "$FAKE_LOG" && ok "dist の中身だけを docker cp" || ng "docker cp の引数が違う: $(cat "$FAKE_LOG")"

: > "$FAKE_LOG"
(cd "$repo" && FAKE_KUBE_CONTEXT=other PATH="$work/bin:$PATH" ASSETS_OUT="$repo/out" bash scripts/images-k3d.sh) > "$work/o3" 2>&1
rc=$?
[ "$rc" != 0 ] && [ ! -s "$FAKE_LOG" ] && ok "別 context では失敗し docker に触らない" || ng "別 context の扱い rc=$rc"

if [ "$failures" -ne 0 ]; then
  echo "images-k3d_test: $failures 件失敗" >&2
  exit 1
fi
echo "images-k3d_test: 全て成功"
