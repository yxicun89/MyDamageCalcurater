#!/usr/bin/env sh
# API レーンのスモーク(ADR-0203 §5。test-strategy.md L5)。gateway 経由で calc-svc の3操作・ヘッダ検証・
# pokedex 未設定の 503・Web の静的配信(ADR-0205。デプロイ前は 503、デプロイ後は 200)を確かめる。
# k3d 上では `make api-smoke`、`make dev` の上では API_URL を渡して使う。
#
# 環境変数:
#   API_URL             gateway の基底 URL(既定 http://localhost:8080。k3d の loadbalancer)
#   API_SMOKE_RETRIES   ロールアウト直後の 000/404/502/503 を再試行する回数(既定 30。1秒間隔)
#   API_SMOKE_BALANCE   /api/balance/healthz が balance の Ingress に届くかの確認。
#                       auto(既定: kubectl で balance の Ingress があるときだけ見る)/ on / off
#   API_SMOKE_NAMESPACE auto のときに balance の Ingress を探す namespace(既定 pokecalc)
#
# マスタは local overlay が読ませる架空の例(services/calc/testdata/master.example.json)。jq は使わない
# (レスポンスは1行の JSON なので grep / sed で足りる)。失敗したらステータスと本文を出して非ゼロで終わる。
set -eu

base_url=${API_URL:-http://localhost:8080}
retries=${API_SMOKE_RETRIES:-30}
balance_mode=${API_SMOKE_BALANCE:-auto}
namespace=${API_SMOKE_NAMESPACE:-pokecalc}

# 架空の UUID(版 4 の形)。gateway は正準形の UUID だけを通す(ADR-0202)。
device_id=00000000-0000-4000-8000-00000000d001
session_id=00000000-0000-4000-8000-00000000d002

# 例のマスタの架空の ID(services/calc/calctest と同じ値)。
attacker='{"speciesKey":"9001-000","natureId":"testatkup","sp":{"hp":0,"atk":32,"def":0,"spa":0,"spd":0,"spe":32}}'
defender='{"speciesKey":"9002-000","natureId":"testneutrala","sp":{"hp":32,"atk":0,"def":0,"spa":0,"spd":0,"spe":0}}'
calc_body='{"format":"single","attacker":'"$attacker"',"defender":'"$defender"',"moveId":"testbeam"}'
bulk_body='{"format":"single","attacker":'"$attacker"',"defenderSpeciesKey":"9002-000","moveId":"testbeam"}'
reverse_body='{"format":"single","side":"defender","known":'"$attacker"',"unknownSpeciesKey":"9002-000","moveId":"testbeam","observations":[{"percent":18}]}'

body_file=$(mktemp)
trap 'rm -f "$body_file"' EXIT

status=000

fail() {
  echo "api smoke: $1 (HTTP $status)" >&2
  cat "$body_file" >&2
  echo >&2
  exit 1
}

# request METHOD PATH [BODY] [HEADER_MODE]
#   HEADER_MODE: valid(既定)/ none(端末ID・セッションIDを付けない)/ bad-session(セッションIDが UUID でない)
request() {
  method=$1
  path=$2
  data=${3:-}
  header_mode=${4:-valid}
  set -- -sS --connect-timeout 5 --max-time 30 -o "$body_file" -w '%{http_code}' -X "$method" -H 'Content-Type: application/json'
  case "$header_mode" in
    valid) set -- "$@" -H "X-Device-Id: $device_id" -H "X-Session-Id: $session_id" ;;
    none) ;;
    bad-session) set -- "$@" -H "X-Device-Id: $device_id" -H 'X-Session-Id: not-a-uuid' ;;
    *) echo "api smoke: unknown header mode $header_mode" >&2; exit 2 ;;
  esac
  if [ -n "$data" ]; then
    set -- "$@" --data "$data"
  fi
  # curl は接続拒否・タイムアウトでも `-w '%{http_code}'` により自分で "000" を書き出す。
  # ここで `|| printf '000'` を足すと失敗時に出力が二重になり("000000")、下の case の
  # どの分岐にも一致せず再試行されなくなる(critic 指摘で修正)。`|| true` は command
  # substitution の非ゼロ終了で set -e が働かないようにするためだけに使い、出力には触らない。
  status=$(curl "$@" "$base_url$path") || true
  : "${status:=000}"
}

# ロールアウト直後は Ingress の反映前(404)や終了中の Pod(502/503)に当たることがあるので、最初の1件だけ再試行する。
request_with_retry() {
  attempt=0
  while :; do
    request "$@"
    case "$status" in
      000|404|502|503) ;;
      *) return 0 ;;
    esac
    attempt=$((attempt + 1))
    if [ "$attempt" -ge "$retries" ]; then
      return 0
    fi
    sleep 1
  done
}

expect_status() {
  if [ "$status" != "$1" ]; then
    fail "$2: want HTTP $1"
  fi
}

expect_body() {
  if ! grep -qF "$1" "$body_file"; then
    fail "$2: body is missing $1"
  fi
}

expect_error() {
  expect_status "$1" "$3"
  expect_body "\"code\":\"$2\"" "$3"
}

# 1. POST /api/calc: rolls が16個・category がある。
request_with_retry POST /api/calc "$calc_body"
expect_status 200 "POST /api/calc"
expect_body '"category":"physical"' "POST /api/calc"
rolls=$(tr -d ' \n' <"$body_file" | sed -n 's/.*"rolls":\[\([^]]*\)\].*/\1/p')
roll_count=$(printf '%s' "$rolls" | tr ',' '\n' | grep -c '^[0-9][0-9]*$' || true)
if [ "$roll_count" != 16 ]; then
  fail "POST /api/calc: want 16 rolls, got $roll_count"
fi

# 2. POST /api/calc/bulk: rows がある(1行以上)。
request POST /api/calc/bulk "$bulk_body"
expect_status 200 "POST /api/calc/bulk"
expect_body '"rows":[{' "POST /api/calc/bulk"

# 3. POST /api/calc/reverse: candidates がある(1件以上)。
request POST /api/calc/reverse "$reverse_body"
expect_status 200 "POST /api/calc/reverse"
expect_body '"candidates":[{' "POST /api/calc/reverse"

# 4. ヘッダの検証は gateway が行う(ADR-0202)。
request POST /api/calc "$calc_body" none
expect_error 400 missing_header "POST /api/calc without device/session headers"
request POST /api/calc "$calc_body" bad-session
expect_error 400 invalid_header "POST /api/calc with a non-UUID session id"

# 5. pokedex-svc(plan.md P2-3)が入るまで GATEWAY_POKEDEX_URL は未設定なので 503 upstream_unavailable。
#    pokedex-svc を deploy/k8s に入れたら、ここを 200 の確認に変える(ADR-0203 §5)。
request GET /api/pokedex/natures
expect_error 503 upstream_unavailable "GET /api/pokedex/natures (pokedex-svc not deployed yet)"

# 6. サービス間の内部 API(/internal/*。ADR-0204)は gateway が外に出さない(ヘッダの有無によらず 404 not_found)。
request GET /internal/pokedex/master "" none
expect_error 404 not_found "GET /internal/pokedex/master (internal API must not be exposed by the gateway)"

# 7. Web の静的配信(ADR-0205)。gateway の後ろに置かれていれば 200、Web レーンの Service がまだ無ければ
#    503 upstream_unavailable(接続不可)。それ以外(404 や他の 5xx)は Web を後ろに置けていないので失敗。
#    「まだデプロイされていない」の 503 は本来の状態でありうるので、request_with_retry ではなく
#    request を使う(再試行で消えない)。
request GET / "" none
case "$status" in
  200) web_result=200 ;;
  503)
    if grep -qF '"code":"upstream_unavailable"' "$body_file"; then
      web_result=503
    else
      fail "GET /"
    fi
    ;;
  *) fail "GET /" ;;
esac

# 8. 任意: balance がデプロイされているとき、/api/balance は balance の Ingress に届く(gateway の `/` が奪わない)。
check_balance=no
case "$balance_mode" in
  on) check_balance=yes ;;
  off) ;;
  auto)
    if command -v kubectl >/dev/null 2>&1 && kubectl -n "$namespace" get ingress balance >/dev/null 2>&1; then
      check_balance=yes
    fi
    ;;
  *) echo "api smoke: API_SMOKE_BALANCE must be auto, on or off (got $balance_mode)" >&2; exit 2 ;;
esac
balance_result=skipped
if [ "$check_balance" = yes ]; then
  request_with_retry GET /api/balance/healthz "" none
  expect_status 200 "GET /api/balance/healthz (must reach balance, not the gateway)"
  balance_result=200
fi

echo "api smoke: calc=200 bulk=200 reverse=200 missing_header=400 invalid_header=400 pokedex=503 internal=404 balance=$balance_result web=$web_result"
