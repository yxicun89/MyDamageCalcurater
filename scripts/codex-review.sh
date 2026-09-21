#!/usr/bin/env bash
# 直近コミット(または作業中の差分)を Codex にレビューさせ、.reviews/ に保存する。
# Codex が無い/未ログインならスキップ(失敗扱いにしない)。
set -euo pipefail
if ! command -v codex >/dev/null 2>&1; then echo "codex not found: skip"; exit 0; fi
mkdir -p .reviews
out=".reviews/codex-$(git rev-parse --short HEAD 2>/dev/null || echo wip)-$(date +%H%M%S).md"
prompt="AGENTS.md と CLAUDE.md に従い、このリポジトリの直近の変更(git diff HEAD~1 と未コミットの差分)をレビューしてください。指摘は 重大/重要/軽微 に分類し、ファイルと行を示してください。ファイルは変更しないこと。"
# 注: codex exec のオプションは P0-5 で `codex exec --help` を確認して調整する
if codex exec --sandbox read-only "$prompt" > "$out" 2>&1; then
  echo "review saved: $out"; cat "$out"
else
  echo "codex review failed (未ログインの可能性): $out"; exit 0
fi
