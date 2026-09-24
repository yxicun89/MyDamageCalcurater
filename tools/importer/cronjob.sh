#!/bin/sh
# pokedex-import の CronJob 用エントリポイント(ADR-0104 §2)。
# 取得(Node)→ 上流の最新版の検出(失敗は警告だけ)→ 照合・投入(Go)を1つのコンテナで順に行う。
# 読み取り専用ルートで動くため、HOME・npm のキャッシュは /tmp(emptyDir)に置く。
#
# 手動実行(make import-k8s)と CronJob の定期実行が同時に走ると、共有 PVC 上の取得キャッシュ・DB投入が
# 競合しうる(issue #106)。fetch.mjs を呼ぶ前に flock で非ブロッキングに排他し、取れなければ何もせず
# 終了コード1(ADR-0104 §3「再試行で直りうる失敗」)で諦める。ロックは advisory lock なので、プロセスの
# 終了理由(正常・異常・SIGKILL)によらずカーネルが自動解放し、stale lock を残さない(ADR-0109)。
set -eu

export HOME="${HOME:-/tmp}"
export npm_config_cache="${npm_config_cache:-/tmp/npm-cache}"

# /app 配下のパスとロックファイルの場所。テスト(ネイティブ Go)から差し替えられるよう環境変数で
# 上書き可能にする(ADR-0109 §2。IMPORT_APP_DIR・IMPORT_LOCK_FILE はテスト契約なので名前を変えない)。
APP_DIR="${IMPORT_APP_DIR:-/app}"
LOCK_FILE="${IMPORT_LOCK_FILE:-${APP_DIR}/data/generated/.import.lock}"

exec 9>"$LOCK_FILE"
flock -n 9 || {
  echo "cronjob: 別の import が実行中(ロック $LOCK_FILE を取得できない)。今回は諦める" >&2
  exit 1
}

echo "cronjob: 固定版の取得"
node "$APP_DIR/tools/importer/fetch.mjs"

echo "cronjob: 上流の最新版の検出(失敗しても取り込みは続ける)"
node "$APP_DIR/tools/importer/check-upstream.mjs" || echo "cronjob: 上流の検出に失敗した(ログを参照。取り込みは続ける)" >&2

echo "cronjob: 照合・投入"
exec "$APP_DIR/pokedex-import" -data "$APP_DIR/data" -upstream "$APP_DIR/data/generated/upstream/latest.json"
