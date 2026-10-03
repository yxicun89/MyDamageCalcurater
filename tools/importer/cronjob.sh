#!/bin/sh
# pokedex-import の CronJob 用エントリポイント(ADR-0104 §2)。
# 取得(Node)→ 上流の最新版の検出(失敗は警告だけ)→ 照合・投入(Go)を行う。引数でフェーズを選ぶ
# (ADR-0101 追記 2026-10-01・issue #301):
#   fetch   取得段(initContainer。DB の資格情報を持たない): 容量確認 → 取得 → 上流の検出 → 引き渡しファイルを書く
#   import  投入段(main): 引き渡しファイルの確認 → 投入 → 旧版の整理 → 引き渡しファイルを消す
#   (なし)  従来どおり1プロセスで全工程(make dev・ロックのテスト用)
# flock は fetch の終了で解放され import の開始まで隙間ができるため、その間は引き渡しファイル
# (owner=<Pod 名>・expires=<epoch 秒>)で他の Pod の開始を防ぐ。期限切れは stale として無視する。
# 読み取り専用ルートで動くため、HOME・npm のキャッシュは /tmp(emptyDir)に置く。
#
# 手動実行(make import-k8s)と CronJob の定期実行が同時に走ると、共有 PVC 上の取得キャッシュ・DB投入が
# 競合しうる(issue #106)。fetch.mjs を呼ぶ前に flock で非ブロッキングに排他し、取れなければ何もせず
# 終了コード1(ADR-0104 §3「再試行で直りうる失敗」)で諦める。ロックは advisory lock なので、プロセスの
# 終了理由(正常・異常・SIGKILL)によらずカーネルが自動解放し、stale lock を残さない(ADR-0109)。
#
# 容量(issue #111・ADR-0104 追記): ロックの直後、取得より前に prune.mjs check で PVC の空きを確かめる。
# 予約容量を下回ると stderr に importer-capacity を出して終了コード3で止まる(download・DB 更新に進まない)。
# 終了コード3は「再試行しても直らない容量不足」で、docs/runbooks/data.md の手順で人が回復する(1=再試行で直りうる失敗)。
# 取り込み(pokedex-import)が成功した後に、同じロックを持ったまま prune.mjs prune で旧版を消す。
# 終了コード3 には、ID が消える投入(ErrKeyRemoved。ADR-0131)もある。DB は変えない。stderr の
# `<種類>:<ID>` を確かめ、消えてよければ手動 Job に IMPORT_ALLOW_REMOVED を付けて1回流す
# (docs/runbooks/data.md「ID が消えて止まったとき」)。CronJob の定期実行には付けない(消滅を自動で通さない)。
# prune の失敗は握りつぶさない(容量回復の失敗に気付けなくなるため)。pokedex-import は exec にしない。
set -eu

export HOME="${HOME:-/tmp}"
export npm_config_cache="${npm_config_cache:-/tmp/npm-cache}"

# /app 配下のパスとロックファイルの場所。テスト(ネイティブ Go)から差し替えられるよう環境変数で
# 上書き可能にする(ADR-0109 §2。IMPORT_APP_DIR・IMPORT_LOCK_FILE はテスト契約なので名前を変えない)。
APP_DIR="${IMPORT_APP_DIR:-/app}"
LOCK_FILE="${IMPORT_LOCK_FILE:-${APP_DIR}/data/generated/.import.lock}"

PHASE="${1:-all}"
case "$PHASE" in
  fetch | import | all) ;;
  *)
    echo "cronjob: 不明なフェーズ: $PHASE(fetch | import | 引数なし)" >&2
    exit 2
    ;;
esac

HANDOFF_FILE="${IMPORT_HANDOFF_FILE:-${APP_DIR}/data/generated/.import.handoff}"
# 引き渡しの有効期間。fetch の成功後に Job が生き続けられる最長は activeDeadlineSeconds(3600)なので、
# 既定はそれと同じ 3600 秒(SIGKILL・OOM で EXIT trap が走らず残った引き渡しが、次の Job を止める最長時間でもある)。
HANDOFF_TTL="${IMPORT_HANDOFF_TTL_SECONDS:-3600}"
# import は引き渡しを持っているので、fetch の確認などで一瞬ロックを持つ他の Pod を待つ。
LOCK_WAIT="${IMPORT_LOCK_WAIT_SECONDS:-60}"
SELF="${HOSTNAME:-$(hostname)}"

case "$HANDOFF_TTL" in
  '' | *[!0-9]*)
    echo "cronjob: IMPORT_HANDOFF_TTL_SECONDS が数値でない: $HANDOFF_TTL" >&2
    exit 2
    ;;
esac
case "$LOCK_WAIT" in
  '' | *[!0-9]*)
    echo "cronjob: IMPORT_LOCK_WAIT_SECONDS が数値でない: $LOCK_WAIT" >&2
    exit 2
    ;;
esac

# 引き渡しファイルの owner を返す(期限は見ない)。
handoff_raw_owner() {
  if [ -f "$HANDOFF_FILE" ]; then sed -n 's/^owner=//p' "$HANDOFF_FILE" | head -n 1; fi
}

exec 9>"$LOCK_FILE"
if [ "$PHASE" = import ]; then
  # BusyBox の flock(本番の alpine イメージ)は -w を持たないので、-n を1秒間隔で試して待つ。
  _waited=0
  until flock -n 9; do
    if [ "$_waited" -ge "$LOCK_WAIT" ]; then
      echo "cronjob: ロック $LOCK_FILE を ${LOCK_WAIT} 秒待っても取得できない" >&2
      # 自分の引き渡しを残すと、期限まで全 Job が止まる。自分のものだけ消して諦める。
      if [ "$(handoff_raw_owner)" = "$SELF" ]; then rm -f "$HANDOFF_FILE"; fi
      exit 1
    fi
    _waited=$((_waited + 1))
    sleep 1
  done
else
  flock -n 9 || {
    echo "cronjob: 別の import が実行中(ロック $LOCK_FILE を取得できない)。今回は諦める" >&2
    exit 1
  }
fi

# 引き渡しファイルの内容。無い・読めない・期限切れなら owner は空(stale として無視)。
HANDOFF_OWNER=""
if [ -f "$HANDOFF_FILE" ]; then
  _owner="$(sed -n 's/^owner=//p' "$HANDOFF_FILE" | head -n 1)"
  _expires="$(sed -n 's/^expires=//p' "$HANDOFF_FILE" | head -n 1)"
  case "$_expires" in
    '' | *[!0-9]*) ;;
    *) if [ "$_expires" -gt "$(date +%s)" ]; then HANDOFF_OWNER="$_owner"; fi ;;
  esac
fi

# 他の Pod の有効な引き渡しがあるときは、どのフェーズも何も始めない(再試行で直りうる失敗)。
if [ -n "$HANDOFF_OWNER" ] && [ "$HANDOFF_OWNER" != "$SELF" ]; then
  echo "cronjob: 別の Pod($HANDOFF_OWNER)の引き渡しが有効(期限まで待つ)。今回は諦める" >&2
  exit 1
fi

run_fetch() {
  echo "cronjob: 容量の事前確認(不足なら終了コード3)"
  node "$APP_DIR/tools/importer/prune.mjs" check

  echo "cronjob: 固定版の取得"
  node "$APP_DIR/tools/importer/fetch.mjs"

  echo "cronjob: 上流の最新版の検出(失敗しても取り込みは続ける)"
  node "$APP_DIR/tools/importer/check-upstream.mjs" || echo "cronjob: 上流の検出に失敗した(ログを参照。取り込みは続ける)" >&2
}

run_import() {
  echo "cronjob: 照合・投入"
  if [ -n "${IMPORT_ALLOW_REMOVED:-}" ]; then
    "$APP_DIR/pokedex-import" -data "$APP_DIR/data" -upstream "$APP_DIR/data/generated/upstream/latest.json" -allow-removed "$IMPORT_ALLOW_REMOVED"
  else
    "$APP_DIR/pokedex-import" -data "$APP_DIR/data" -upstream "$APP_DIR/data/generated/upstream/latest.json"
  fi

  echo "cronjob: 旧版の整理(現在版+直前の成功版と直近52件の report を残す)"
  node "$APP_DIR/tools/importer/prune.mjs" prune
}

case "$PHASE" in
  fetch)
    run_fetch
    # 成功したら、ロックを解放する前に引き渡しファイルを書く(import の開始までの隙間を埋める)。
    printf 'owner=%s\nexpires=%s\n' "$SELF" "$(($(date +%s) + HANDOFF_TTL))" >"$HANDOFF_FILE.tmp"
    mv "$HANDOFF_FILE.tmp" "$HANDOFF_FILE"
    ;;
  import)
    if [ "$HANDOFF_OWNER" != "$SELF" ]; then
      echo "cronjob: 自分($SELF)への有効な引き渡しが無い(fetch が未了・期限切れ)。投入しない" >&2
      exit 1
    fi
    # 成功・失敗のどちらでも、ロックを持ったまま引き渡しファイルを消す。
    trap 'rm -f "$HANDOFF_FILE"' EXIT
    run_import
    ;;
  all)
    run_fetch
    run_import
    ;;
esac
