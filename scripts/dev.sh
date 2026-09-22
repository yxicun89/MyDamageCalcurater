#!/usr/bin/env bash
# k8s を使わずローカルで calc-svc・gateway を起動する高速な開発ループ(ADR-0203 §6)。
# 例のマスタ一式(services/calc/testdata/master.example.json。架空データ + 相性表。ADR-0204)を
# calc-svc に読ませ、gateway をその calc-svc へ向ける。
# CORS は Vite の開発サーバの既定オリジン(http://localhost:5173)だけ許可する。
#
# `go run` は SIGINT/SIGTERM をビルドした実バイナリへ転送しない(go run 自身のプロセスだけが
# 終了し、実際にポートを掴んでいるバイナリが残ることがある)。そのため先に `go build` してから
# バイナリを直接起動する(calc-svc・gateway は SIGTERM で graceful shutdown する。
# services/{calc,gateway}/cmd/*/main.go の signal.NotifyContext)。
#
# 環境変数:
#   DEV_CALC_PORT     calc-svc の待ち受けポート(既定 8081)
#   DEV_GATEWAY_PORT  gateway の待ち受けポート(既定 8080。k3d の loadbalancer と同じポートなので、
#                     k3d クラスタを動かしている間はどちらかのポートを変えること)
#
# Ctrl-C(SIGINT)・SIGTERM で calc-svc・gateway の両方を止める(終了コードはそれぞれ 130 / 143)。
# どちらかが先に落ちたら残りも止めて非ゼロで終わる(空振りの起動を検知する)。確認は
# `make dev` の後に
#   API_URL=http://localhost:<DEV_GATEWAY_PORT> API_SMOKE_BALANCE=off services/gateway/scripts/smoke.sh
set -euo pipefail
cd "$(dirname "$0")/.."
repo_root="$(pwd)"

calc_port="${DEV_CALC_PORT:-8081}"
gateway_port="${DEV_GATEWAY_PORT:-8080}"

calc_master="$repo_root/services/calc/testdata/master.example.json"

tmpdir="$(mktemp -d)"
pids=()

cleanup() {
  echo
  echo "dev: calc-svc・gateway を止めます..."
  for pid in "${pids[@]:-}"; do
    [ -n "$pid" ] && kill "$pid" >/dev/null 2>&1 || true
  done
  for pid in "${pids[@]:-}"; do
    [ -n "$pid" ] && wait "$pid" 2>/dev/null || true
  done
  rm -rf "$tmpdir"
}
trap cleanup EXIT
# INT/TERM は明示的に exit コードを渡す(慣習どおり 128+シグナル番号)。EXIT トラップの
# cleanup がその後に走り、実際のプロセス停止・tmpdir の削除を行う。
trap 'exit_code=130; exit "$exit_code"' INT
trap 'exit_code=143; exit "$exit_code"' TERM
exit_code=0

echo "dev: calc-svc・gateway をビルドします..."
(cd "$repo_root/services" && go build -o "$tmpdir/calc" ./calc/cmd/calc)
(cd "$repo_root/services" && go build -o "$tmpdir/gateway" ./gateway/cmd/gateway)

echo "dev: calc-svc を http://localhost:$calc_port で起動します(マスタ: $calc_master)"
CALC_ADDR=":$calc_port" \
CALC_MASTER_PATH="$calc_master" \
"$tmpdir/calc" &
pids+=("$!")

echo "dev: gateway を http://localhost:$gateway_port で起動します(calc-svc: http://127.0.0.1:$calc_port)"
GATEWAY_ADDR=":$gateway_port" \
GATEWAY_CALC_URL="http://127.0.0.1:$calc_port" \
GATEWAY_CORS_ALLOWED_ORIGINS="http://localhost:5173" \
"$tmpdir/gateway" &
pids+=("$!")

echo "dev: 起動しました。calc-svc=http://localhost:$calc_port gateway=http://localhost:$gateway_port"
echo "dev: Ctrl-C で両方を止めます"

# bash 3.2(macOS 既定)には `wait -n` が無いので、どちらかが落ちたことを kill -0 でポーリングする。
while :; do
  for pid in "${pids[@]}"; do
    if ! kill -0 "$pid" 2>/dev/null; then
      echo "dev: プロセス $pid が終了しました。残りも止めます" >&2
      exit_code=1
      exit "$exit_code"
    fi
  done
  sleep 1
done
