#!/usr/bin/env sh
# API レーンのスモーク(ADR-0203 §5・ADR-0206 §3・§4。test-strategy.md L5)。gateway 経由で calc-svc の3操作・
# ヘッダ検証・pokedex(200 か 503。ADR-0206)・Web の静的配信(ADR-0205。デプロイ前は 503、デプロイ後は 200)を
# 確かめる。k3d 上では `make api-smoke`、`make dev` の上では API_URL を渡して使う。
#
# 環境変数:
#   API_URL             gateway の基底 URL(既定 http://localhost:8080。k3d の loadbalancer)
#   API_SMOKE_RETRIES   ロールアウト直後、Traefik が終了中の Pod に振り分けて返す 000(接続不可)・502
#                       (Bad Gateway)だけを再試行する回数(既定 30。1秒間隔)。404・503 は意味のある
#                       最終状態(pokedex 未投入・Web 未デプロイ等)でもありうるので再試行しない
#   API_SMOKE_BALANCE   /api/balance/healthz が balance の Ingress に届くかの確認。
#                       auto(既定: kubectl で balance の Ingress があるときだけ見る)/ on / off
#   API_SMOKE_NAMESPACE auto のときに balance の Ingress を探す namespace(既定 pokecalc)
#
# 計算に使う ID(性格・種族・技)は、まず GET /api/pokedex/natures を叩いて入手元を見分け(ADR-0206 §3・§4)、
# pokedex-svc に繋がっている(200)なら公開 API から実際に引く。繋がっていない(503)なら
# 例のマスタ(services/calc/testdata/master.example.json と同じ架空 ID)を使う。マスタの一覧やレギュレーション
# (M-C)依存の ID をこのスクリプトに直書きしない(CLAUDE.md ドメイン規約)。jq は使わない
# (レスポンスは1行の JSON なので tr / sed で足りる)。失敗したらステータスと本文を出して非ゼロで終わる。
set -eu

base_url=${API_URL:-http://localhost:8080}
retries=${API_SMOKE_RETRIES:-30}
balance_mode=${API_SMOKE_BALANCE:-auto}
namespace=${API_SMOKE_NAMESPACE:-pokecalc}

# 架空の UUID(版 4 の形)。gateway は正準形の UUID だけを通す(ADR-0202)。
device_id=00000000-0000-4000-8000-00000000d001
session_id=00000000-0000-4000-8000-00000000d002

body_file=$(mktemp)
list_file=$(mktemp)
import_hint=0
trap 'rm -f "$body_file" "$list_file"' EXIT

status=000

fail() {
  echo "api smoke: $1 (HTTP $status)" >&2
  cat "$body_file" >&2
  echo >&2
  if [ "$import_hint" = 1 ]; then
    echo "api smoke: pokedex-svc の DB が未投入の可能性がある。'make import-k8s' を実行してから再試行すること" >&2
  fi
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

# ロールアウト直後、Traefik がまだ終了中の Pod に振り分けて 000(接続不可)や 502(Bad Gateway)を返すことが
# あるので、それらだけを数回再試行する。404・503 は意味のある最終状態(pokedex 未投入の 503、Web 未デプロイの
# 503、内部 API の 404 等)でもありうるので、ここでは再試行しない(空振りせず、すぐに最終状態として扱う)。
request_with_retry() {
  attempt=0
  while :; do
    request "$@"
    case "$status" in
      000|502) ;;
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

# split_objects RAW_JSON_ARRAY: RAW_JSON_ARRAY([{"a":1},{"a":2},...])を1オブジェクト1行にして list_file へ書く。
# grep -o は POSIX に無いので、1行にまとめてから "},{" で改行を入れて要素ごとに割る(ADR-0206 §3 実装の注意)。
# 各オブジェクトに配列(例 SpeciesSummary.types)はあっても入れ子オブジェクトは無いので、この分割で安全。
# 末尾に改行を足す(足さないと `while read` が最後の要素を読み落とす)。
split_objects() {
  printf '%s' "$1" | tr -d ' \n' | sed -e 's/^\[//' -e 's/\]$//' -e 's/},{/}\n{/g' >"$list_file"
  printf '\n' >>"$list_file"
}

# 0. GET /api/pokedex/natures を先頭に叩き、入手元を見分ける(ADR-0206 §3・§4)。
request_with_retry GET /api/pokedex/natures
pokedex_status=$status

master_source=""
case "$pokedex_status" in
  200)
    master_source=pokedex
    natures_body=$(cat "$body_file")
    ;;
  503)
    if grep -qF '"code":"upstream_unavailable"' "$body_file"; then
      master_source=example
    elif grep -qF '"code":"master_unavailable"' "$body_file"; then
      master_source=example
      import_hint=1
      echo "api smoke: pokedex-svc の DB が未投入('master_unavailable')。'make import-k8s' を実行すること" >&2
    else
      fail "/api/pokedex/natures: 未知の 503"
    fi
    ;;
  *)
    fail "/api/pokedex/natures: 200 か 503 以外"
    ;;
esac

if [ "$master_source" = pokedex ]; then
  # 性格: 無補正(plus キーを持たない。生成型 Nature の plus/minus は omitempty)の先頭の id。
  split_objects "$natures_body"
  nature_id=""
  while IFS= read -r line; do
    case "$line" in
      *'"plus"'*) continue ;;
    esac
    id=$(printf '%s' "$line" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')
    if [ -n "$id" ]; then
      nature_id=$id
      break
    fi
  done <"$list_file"
  if [ -z "$nature_id" ]; then
    fail "/api/pokedex/natures: 無補正の性格が無い"
  fi

  # 種族: species?limit=1 の先頭の key(攻撃側・防御側の両方に使う)。
  request_with_retry GET "/api/pokedex/species?limit=1"
  expect_status 200 "GET /api/pokedex/species"
  split_objects "$(cat "$body_file")"
  species_key=""
  while IFS= read -r line; do
    key=$(printf '%s' "$line" | sed -n 's/.*"key":"\([^"]*\)".*/\1/p')
    if [ -n "$key" ]; then
      species_key=$key
      break
    fi
  done <"$list_file"
  if [ -z "$species_key" ]; then
    fail "/api/pokedex/species: 種族が空"
  fi

  # 技: moves?limit=200 のうち category=physical・威力 0 でないものの id を先頭から最大10件。
  request_with_retry GET "/api/pokedex/moves?limit=200"
  expect_status 200 "GET /api/pokedex/moves"
  split_objects "$(cat "$body_file")"
  move_ids=""
  move_count=0
  while IFS= read -r line; do
    case "$line" in
      *'"category":"physical"'*) ;;
      *) continue ;;
    esac
    case "$line" in
      *'"power":0,'*|*'"power":0}'*) continue ;;
    esac
    id=$(printf '%s' "$line" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')
    if [ -z "$id" ]; then
      continue
    fi
    if [ -z "$move_ids" ]; then
      move_ids=$id
    else
      move_ids="$move_ids
$id"
    fi
    move_count=$((move_count + 1))
    if [ "$move_count" -ge 10 ]; then
      break
    fi
  done <"$list_file"
  if [ -z "$move_ids" ]; then
    fail "/api/pokedex/moves: 威力のある物理技が無い"
  fi

  attacker="{\"speciesKey\":\"$species_key\",\"natureId\":\"$nature_id\",\"sp\":{\"hp\":0,\"atk\":32,\"def\":0,\"spa\":0,\"spd\":0,\"spe\":32}}"
  defender="{\"speciesKey\":\"$species_key\",\"natureId\":\"$nature_id\",\"sp\":{\"hp\":32,\"atk\":0,\"def\":0,\"spa\":0,\"spd\":0,\"spe\":0}}"
  defender_species_key=$species_key
  candidates=$move_ids
else
  # 例のマスタの架空 ID(services/calc/calctest と同じ値)。
  attacker='{"speciesKey":"9001-000","natureId":"testatkup","sp":{"hp":0,"atk":32,"def":0,"spa":0,"spd":0,"spe":32}}'
  defender='{"speciesKey":"9002-000","natureId":"testneutrala","sp":{"hp":32,"atk":0,"def":0,"spa":0,"spd":0,"spe":0}}'
  defender_species_key=9002-000
  candidates=testbeam
fi

# 1. POST /api/calc: 候補の技を先頭から試し、200 かつ maxDamage >= 1 になった最初の技を採用する
#    (タイプ相性で無効化される組み合わせを避ける。ADR-0206 §3)。採用した技で rolls が16個・category があること。
move_id=""
max_damage=""
set -f
set -- $candidates
set +f
for candidate in "$@"; do
  calc_body='{"format":"single","attacker":'"$attacker"',"defender":'"$defender"',"moveId":"'"$candidate"'"}'
  request_with_retry POST /api/calc "$calc_body"
  if [ "$status" = 200 ]; then
    md=$(tr -d ' \n' <"$body_file" | sed -n 's/.*"maxDamage":\([0-9][0-9]*\).*/\1/p')
    if [ -n "$md" ] && [ "$md" -ge 1 ]; then
      move_id=$candidate
      max_damage=$md
      break
    fi
  fi
done
if [ -z "$move_id" ]; then
  fail "/api/calc: 候補の技(最大10件)のどれもダメージが出なかった"
fi
expect_status 200 "POST /api/calc"
expect_body '"category":"physical"' "POST /api/calc"
rolls=$(tr -d ' \n' <"$body_file" | sed -n 's/.*"rolls":\[\([^]]*\)\].*/\1/p')
roll_count=$(printf '%s' "$rolls" | tr ',' '\n' | grep -c '^[0-9][0-9]*$' || true)
if [ "$roll_count" != 16 ]; then
  fail "POST /api/calc: want 16 rolls, got $roll_count"
fi

# 2. POST /api/calc/bulk: rows がある(1行以上)。
bulk_body='{"format":"single","attacker":'"$attacker"',"defenderSpeciesKey":"'"$defender_species_key"'","moveId":"'"$move_id"'"}'
request_with_retry POST /api/calc/bulk "$bulk_body"
expect_status 200 "POST /api/calc/bulk"
expect_body '"rows":[{' "POST /api/calc/bulk"

# 3. POST /api/calc/reverse: 採用した技の実点数(maxDamage)を観測にして candidates がある(1件以上)ことを確かめる
#    (同じ攻撃側・同じ技・同じ防御側で観測した実点数なので、真の構成が必ず候補に入る。ADR-0206 §3)。
reverse_body='{"format":"single","side":"defender","known":'"$attacker"',"unknownSpeciesKey":"'"$defender_species_key"'","moveId":"'"$move_id"'","observations":[{"damage":'"$max_damage"'}]}'
request_with_retry POST /api/calc/reverse "$reverse_body"
expect_status 200 "POST /api/calc/reverse"
expect_body '"candidates":[{' "POST /api/calc/reverse"

# 4. ヘッダの検証は gateway が行う(ADR-0202)。
request_with_retry POST /api/calc "$calc_body" none
expect_error 400 missing_header "POST /api/calc without device/session headers"
request_with_retry POST /api/calc "$calc_body" bad-session
expect_error 400 invalid_header "POST /api/calc with a non-UUID session id"

# 5. サービス間の内部 API(/internal/*。ADR-0204)は gateway が外に出さない(ヘッダの有無によらず 404 not_found)。
# 404 は再試行しない(意味のある最終状態)。
request_with_retry GET /internal/pokedex/master "" none
expect_error 404 not_found "GET /internal/pokedex/master (internal API must not be exposed by the gateway)"

# 6. Web の静的配信(ADR-0205)。gateway の後ろに置かれていれば 200、Web レーンの Service がまだ無ければ
#    503 upstream_unavailable(接続不可)。それ以外(404 や他の 5xx)は Web を後ろに置けていないので失敗。
#    「まだデプロイされていない」の 503 も、Web が返す 404・500 も意味のある最終状態なので、
#    request_with_retry を使っても(000/502 しか再試行しないので)再試行で消えることはない。
request_with_retry GET / "" none
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

# 7. 任意: balance がデプロイされているとき、/api/balance は balance の Ingress に届く(gateway の `/` が奪わない)。
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

if [ "$master_source" = pokedex ]; then
  echo "api smoke: master=pokedex species=$species_key move=$move_id nature=$nature_id"
else
  echo "api smoke: master=example"
fi
echo "api smoke: calc=200 bulk=200 reverse=200 missing_header=400 invalid_header=400 pokedex=$pokedex_status internal=404 balance=$balance_result web=$web_result"
