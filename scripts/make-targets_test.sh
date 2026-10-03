#!/usr/bin/env bash
# ルート Makefile の help と未実装ターゲットの自動テスト(issue #261・#294・#318・#321)。
# `make test-scripts`(make test に含む)から流す。クラスタ・ネットワークには触らない。
#
# 固定すること:
#   - make help の左列がターゲット名で(Makefile のファイル名ではない)、数字入りのターゲットも出る
#   - 未実装の make assets が成功(終了コード 0)で終わらない
#   - make lint の k8s-render が全レーンの overlay を描画する(各レーンの *-kustomize を呼ぶ)
set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
readonly ROOT
MAKE_BIN="${MAKE:-make}"

failures=0
ok() { echo "ok: $1"; }
ng() { echo "NG: $1" >&2; failures=$((failures + 1)); }

help_out=$("$MAKE_BIN" -C "$ROOT" --no-print-directory help | sed 's/\x1b\[[0-9;]*m//g')

if echo "$help_out" | awk '{print $1}' | grep -qE '(^|/)Makefile$'; then
  ng "make help の左列に Makefile のファイル名が出ている"
else
  ok "make help の左列はターゲット名"
fi

for target in k8s-render api-k3d-deploy web-k3d-open import-k8s e2e; do
  if echo "$help_out" | grep -qE "^[[:space:]]+${target}[[:space:]]"; then
    ok "make help に ${target} が出る"
  else
    ng "make help に ${target} が出ない(数字入り・include 先のターゲットも出すこと)"
  fi
done

dups=$(echo "$help_out" | awk '{print $1}' | sort | uniq -d)
if [ -z "$dups" ]; then
  ok "make help に重複が無い"
else
  ng "make help に重複がある: ${dups}"
fi

rc=0
"$MAKE_BIN" -C "$ROOT" --no-print-directory assets >/dev/null 2>&1 || rc=$?
if [ "$rc" -ne 0 ]; then
  ok "未実装の make assets は非0(${rc})で終わる"
else
  ng "未実装の make assets が終了コード 0 で終わった(成功と数えられてしまう)"
fi

render_plan=$("$MAKE_BIN" -C "$ROOT" --no-print-directory -n k8s-render 2>/dev/null)
for overlay in deploy/k8s/overlays/local deploy/k8s/overlays/cloud deploy/k8s/overlays/local/tidb \
  deploy/k8s/overlays/local-api deploy/k8s/overlays/local-web \
  services/balance/deploy/k8s/overlays/gitops services/speed/deploy/k8s/overlays/gitops \
  services/judge/deploy/k8s/overlays/local deploy/argocd deploy/k8s/base/observability; do
  if echo "$render_plan" | grep -qE "kubectl kustomize [^ ]*${overlay} "; then
    ok "k8s-render が ${overlay} を描画する"
  else
    ng "k8s-render が ${overlay} を描画しない"
  fi
done

# kubectl が無い環境では、各 *-kustomize の分かりにくいエラーより先に理由を出して止まる(issue #75)。
# PATH を make・coreutils だけにした一時ディレクトリへ向け、kubectl を見えなくする。
nokube=$(mktemp -d)
trap 'rm -rf "$nokube"' EXIT
for tool in bash env make grep sed awk; do
  src=$(command -v "$tool") && ln -s "$src" "$nokube/$tool"
done
rc=0
out=$(PATH="$nokube" "$MAKE_BIN" -C "$ROOT" --no-print-directory k8s-render 2>&1) || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "kubectl が無い"; then
  ok "kubectl が無いと k8s-render は理由を出して失敗する"
else
  ng "kubectl が無いときの k8s-render(終了コード ${rc}): ${out}"
fi

# issue #295: 全 *-k3d-deploy は、最初の kubectl / k3d より前に共通の context ガードを呼ぶ(-n の出力で順序を見る)。
# issue #291: 5つの直接デプロイは k3d-deploy-tagged.sh 経由(先頭で require-k3d-context.sh を呼ぶ。
# 別クラスタなら docker・apply に触らないことは k3d-deploy-tagged_test.sh が確かめる)。
for target in api-k3d-deploy balance-k3d-deploy speed-k3d-deploy judge-k3d-deploy web-k3d-deploy \
  balance-k3d-deploy-readmodel speed-k3d-deploy-readmodel; do
  plan=$("$MAKE_BIN" -C "$ROOT" --no-print-directory -n "$target" 2>/dev/null)
  guard_line=$(echo "$plan" | grep -nE "(require-k3d-context|k3d-deploy-tagged)\.sh ${target}" | head -n 1 | cut -d: -f1)
  first_cluster_line=$(echo "$plan" | grep -nE '(^|[ ;&])(kubectl|k3d) ' | head -n 1 | cut -d: -f1)
  if [ -z "$guard_line" ]; then
    ng "make -n ${target} に require-k3d-context.sh が無い"
  elif [ -n "$first_cluster_line" ] && [ "$guard_line" -gt "$first_cluster_line" ]; then
    ng "make -n ${target}: require-k3d-context.sh が最初の kubectl/k3d より後ろにある"
  else
    ok "${target} は kubectl/k3d の前に context ガードを呼ぶ"
  fi
done

if [ "$failures" -ne 0 ]; then
  echo "make-targets_test: ${failures} 件失敗" >&2
  exit 1
fi
echo "make-targets_test: すべて成功"
