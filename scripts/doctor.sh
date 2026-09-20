#!/usr/bin/env bash
# 前提ツールの確認。不足があれば brew のコマンドを表示する。
set -u
missing=0
check() { # name, brew-package, required(1/0)
  if command -v "$1" >/dev/null 2>&1; then
    printf "  ok   %-12s %s\n" "$1" "$($1 --version 2>/dev/null | head -1)"
  else
    if [ "$3" = "1" ]; then
      printf "  NG   %-12s brew install %s\n" "$1" "$2"; missing=1
    else
      printf "  --   %-12s (任意) brew install %s\n" "$1" "$2"
    fi
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
check tiup "tiup (curl のインストーラ)" 1
echo "== iOS(M3で必要)"
check xcodebuild "Xcode(App Store)" 0
echo "== 任意"
check codex "codex(npm i -g @openai/codex)" 0
check tailscale "--cask tailscale" 0
check claude "Claude Code" 0
if docker info >/dev/null 2>&1; then echo "  ok   docker デーモン起動中"; else echo "  NG   docker デーモンが起動していません(OrbStack/Docker Desktop を起動)"; missing=1; fi
exit $missing
