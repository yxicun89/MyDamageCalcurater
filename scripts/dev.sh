#!/usr/bin/env bash
# k8s を使わずローカルで calc-svc・gateway を起動する高速な開発ループ(ADR-0203 §6)。
# 例のマスタ(services/calc/testdata/master.example.json。架空データ)と共有の相性表
# (testdata/golden/typechart.json)を calc-svc に読ませ、gateway をその calc-svc へ向ける。
# CORS は Vite の開発サーバの既定オリジン(http://localhost:5173)だけ許可する。
#
# 環境変数:
#   DEV_CALC_PORT     calc-svc の待ち受けポート(既定 8081)
#   DEV_GATEWAY_PORT  gateway の待ち受けポート(既定 8080。k3d の loadbalancer と同じポートなので、
#                     k3d クラスタを動かしている間はどちらかのポートを変えること)
#
# Ctrl-C(SIGINT)・SIGTERM で calc-svc・gateway の両方を止める。確認は `make dev` の後に
#   API_URL=http://localhost:<DEV_GATEWAY_PORT> API_SMOKE_BALANCE=off services/gateway/scripts/smoke.sh
set -euo pipefail
cd "$(dirname "$0")/.."
repo_root="$(pwd)"

calc_port="${DEV_CALC_PORT:-8081}"
gateway_port="${DEV_GATEWAY_PORT:-8080}"

calc_master="$repo_root/services/calc/testdata/master.example.json"
calc_typechart="$repo_root/testdata/golden/typechart.json"

pids=()
cleanup() {
  echo
  echo "dev: calc-svc・gateway を止めます..."
  for pid in "${pids[@]:-}"; do
    kill "$pid" >/dev/null 2>&1 || true
  done
  wait >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

echo "dev: calc-svc を http://localhost:$calc_port で起動します(マスタ: $calc_master)"
(
  cd "$repo_root/services"
  export CALC_ADDR=":$calc_port"
  export CALC_MASTER_PATH="$calc_master"
  export CALC_TYPECHART_PATH="$calc_typechart"
  exec go run ./calc/cmd/calc
) &
pids+=("$!")

echo "dev: gateway を http://localhost:$gateway_port で起動します(calc-svc: http://127.0.0.1:$calc_port)"
(
  cd "$repo_root/services"
  export GATEWAY_ADDR=":$gateway_port"
  export GATEWAY_CALC_URL="http://127.0.0.1:$calc_port"
  export GATEWAY_CORS_ALLOWED_ORIGINS="http://localhost:5173"
  exec go run ./gateway/cmd/gateway
) &
pids+=("$!")

echo "dev: 起動しました。calc-svc=http://localhost:$calc_port gateway=http://localhost:$gateway_port"
echo "dev: Ctrl-C で両方を止めます"
wait
