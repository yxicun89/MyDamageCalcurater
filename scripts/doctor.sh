#!/usr/bin/env bash
# 前提ツールの確認。不足があれば brew のコマンドを表示する。
set -euo pipefail
# 不足ツールを全部列挙してから終了コードを返す設計のため、失敗は if / || true で個別に受ける。
missing=0
check() { # name, brew-package, required(1/0)
  if command -v "$1" >/dev/null 2>&1; then
    # --version が失敗・未対応でもバージョン欄が空になるだけで、確認は続ける(|| true)。
    printf "  ok   %-12s %s\n" "$1" "$($1 --version 2>/dev/null | head -1 || true)"
  else
    if [ "$3" = "1" ]; then
      printf "  NG   %-12s brew install %s\n" "$1" "$2"; missing=1
    else
      printf "  --   %-12s (任意) brew install %s\n" "$1" "$2"
    fi
  fi
}
# 固定版の検査(ADR-0133)。固定版より古ければ NG、新しい分は ok(最新追従は make deps-outdated)。
ver_ge() { # have, want: have >= want なら 0
  [ "$(printf '%s\n%s\n' "$2" "$1" | sort -V | head -1)" = "$2" ]
}
min_version() { # name, have, want, 失敗にするか(1/0)
  if [ -z "$2" ]; then return 0; fi
  if ver_ge "$2" "$3"; then
    printf "  ok   %-12s %s (>= %s)\n" "$1" "$2" "$3"
  elif [ "$4" = "1" ]; then
    printf "  NG   %-12s %s は固定版 %s より古い\n" "$1" "$2" "$3"; missing=1
  else
    printf "  warn %-12s %s は固定版 %s と異なる(更新推奨)\n" "$1" "$2" "$3"
  fi
}
echo "== 必須"
check git git 1
check go go 1
check node node 1
check jq jq 1
check docker orbstack 1
check k3d k3d 1
check kubectl kubectl 1
check helm helm 1
echo "== 固定版(ADR-0133。更新時は ADR・deploy/k3d.yaml・各 Dockerfile と合わせる)"
root=$(cd "$(dirname "$0")/.." && pwd)
v_go=$(go version 2>/dev/null | sed -E 's/.*go([0-9]+\.[0-9]+(\.[0-9]+)?).*/\1/' || true)
v_k3d=$(k3d version 2>/dev/null | sed -nE 's/^k3d version v?([0-9.]+).*/\1/p' | head -1 || true)
v_kubectl=$(kubectl version --client 2>/dev/null | sed -nE 's/^Client Version: v?([0-9.]+).*/\1/p' | head -1 || true)
v_helm=$(helm version --short 2>/dev/null | sed -nE 's/^v?([0-9.]+).*/\1/p' | head -1 || true)
v_node=$(node -v 2>/dev/null | sed 's/^v//' || true)
want_node=$(tr -d '[:space:]v' < "$root/web/.node-version" 2>/dev/null || true)
min_version go "$v_go" 1.27.1 1
min_version k3d "$v_k3d" 5.9.0 1
min_version kubectl "$v_kubectl" 1.36.1 1
min_version helm "$v_helm" 4.3.0 1
# Node は web/.node-version(= package.json engines = Dockerfile)と一致が正。古いだけなら警告(brew upgrade node)。
if [ -n "$v_node" ] && [ -n "$want_node" ]; then
  if [ "$v_node" = "$want_node" ]; then printf "  ok   %-12s %s\n" node "$v_node"
  else printf "  warn %-12s %s は web/.node-version の %s と異なる\n" node "$v_node" "$want_node"; fi
fi
echo "== TiDB(M2で必要)"
check tiup "tiup (curl のインストーラ: https://tiup-mirrors.pingcap.com/install.sh)" 0
echo "== iOS(M3で必要)"
check xcodebuild "Xcode(App Store)" 0
echo "== 任意"
check codex "codex(npm i -g @openai/codex)" 0
check tailscale "--cask tailscale" 0
check claude "Claude Code" 0
if docker info >/dev/null 2>&1; then echo "  ok   docker デーモン起動中"; else echo "  NG   docker デーモンが起動していません(OrbStack/Docker Desktop を起動)"; missing=1; fi
exit $missing
