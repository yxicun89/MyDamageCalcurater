#!/bin/sh
# pokedex-import の CronJob 用エントリポイント(ADR-0104 §2)。
# 取得(Node)→ 上流の最新版の検出(失敗は警告だけ)→ 照合・投入(Go)を1つのコンテナで順に行う。
# 読み取り専用ルートで動くため、HOME・npm のキャッシュは /tmp(emptyDir)に置く。
set -eu

export HOME="${HOME:-/tmp}"
export npm_config_cache="${npm_config_cache:-/tmp/npm-cache}"

echo "cronjob: 固定版の取得"
node /app/tools/importer/fetch.mjs

echo "cronjob: 上流の最新版の検出(失敗しても取り込みは続ける)"
node /app/tools/importer/check-upstream.mjs || echo "cronjob: 上流の検出に失敗した(ログを参照。取り込みは続ける)" >&2

echo "cronjob: 照合・投入"
exec /app/pokedex-import -data /app/data -upstream /app/data/generated/upstream/latest.json
