#!/usr/bin/env bash
# P4-11: k3d 上の Web(nginx の静的配信)のスモーク。`make web-k3d-smoke` から呼ぶ(直接実行してもよい)。
# 画面(/ と /reverse。SPA のフォールバック)・engine.wasm の MIME・/healthz を curl で確かめ、1項目1行で結果を出す。
#
# 環境変数:
#   WEB_URL            Web の基底 URL。既定 http://localhost:8080(利用者が開く入口。k3d → gateway → web)。
#                      Web だけを直接確かめるとき(診断用)は `make web-k3d-open` の http://localhost:5173 を渡す。
#   WEB_SMOKE_RETRIES  ロールアウト直後・port-forward 直後の接続失敗を再試行する回数(既定 30。1秒間隔)。
#
# 失敗が1つでもあれば非ゼロで終わる(スキップして成功扱いにしない)。
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

base_url=${WEB_URL:-http://localhost:8080}
base_url=${base_url%/}
retries=${WEB_SMOKE_RETRIES:-30}
failures=0

# 最初の1件が届くまで待つ(port-forward の立ち上がり・Pod の入れ替わりを吸収する)。
attempt=0
until curl -fsS -o /dev/null "$base_url/healthz" 2>/dev/null; do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge "$retries" ]; then
    echo "NG  $base_url/healthz に $retries 回つながらなかった" >&2
    exit 1
  fi
  sleep 1
done

# check <説明> <パス> <期待するステータス> [期待する Content-Type の前方一致]
check() {
  local label=$1 path=$2 want_status=$3 want_type=${4:-}
  local out status type
  if ! out=$(curl -sS -o /dev/null -w '%{http_code} %{content_type}' "$base_url$path"); then
    echo "NG  $label: $path に接続できない"
    failures=$((failures + 1))
    return
  fi
  status=${out%% *}
  type=${out#* }
  if [ "$status" != "$want_status" ]; then
    echo "NG  $label: $path のステータスが $status(期待 $want_status)"
    failures=$((failures + 1))
    return
  fi
  if [ -n "$want_type" ] && [[ "$type" != "$want_type"* ]]; then
    echo "NG  $label: $path の Content-Type が '$type'(期待 $want_type)"
    failures=$((failures + 1))
    return
  fi
  echo "OK  $label: $path -> $status${want_type:+ $type}"
}

check "ヘルスチェック" /healthz 200
check "トップ" / 200 text/html
check "逆算を直接開く(SPA のフォールバック)" /reverse 200 text/html
check "engine.wasm の MIME" /engine.wasm 200 application/wasm
check "wasm_exec.js" /wasm_exec.js 200
check "無いアセットは 404(index.html で代用しない)" /static/no-such-file.js 404
check "未知の /api/* は画面(index.html)で代用しない" /api/no-such-endpoint 404

# index.html が読む JS が実際に返ること(gateway 経由では予約パスと衝突すると 404 → 白画面になる。issue #268)。
entry_js=$(curl -sS "$base_url/" | sed -n 's/.*src="\(\/[^"]*\.js\)".*/\1/p' | head -n 1)
if [ -z "$entry_js" ]; then
  echo "NG  index.html が読む JS: / から script の src が見つからない"
  failures=$((failures + 1))
else
  check "index.html が読む JS" "$entry_js" 200
fi

if [ "$failures" -gt 0 ]; then
  echo "web smoke: $failures 件失敗($base_url)" >&2
  exit 1
fi
echo "web smoke: すべて成功($base_url)"
