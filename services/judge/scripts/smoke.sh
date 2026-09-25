#!/usr/bin/env sh
# 判定レーンのスモーク(issue #257)。healthz だけでなく、balance・speed の smoke と同じ深さで
# POST /api/judge/v1/outspeed-and-ko の 200・400・422 を確かめる。
#
# 環境変数:
#   JUDGE_URL   judge 自身の Ingress の基底 URL(既定 http://localhost:8080。judge は
#               `/api/judge` prefix の自分の Ingress を持つ。ADR-0700 §6-3)
#   API_URL     gateway の基底 URL(既定 http://localhost:8080。judge が使う性格・種族・技の ID を
#               引くために使う。judge 自身は pokedex-svc の一覧系 API を公開しないため、
#               gateway の公開 API 経由で引く。services/gateway/scripts/smoke.sh の ID 取得部分と
#               同じ流儀。gateway・judge は同じ Traefik の別 path prefix なので既定値は同じでよい)
#
# 使用可能なポケモン・技のリストをこのスクリプトに直書きしない(CLAUDE.md ドメイン規約)。pokedex-svc へ
# 未投入(gateway 経由で 503)のときは、gateway smoke と同じ例のマスタ(services/calc/testdata の架空 ID)を使い、
# 「未投入」として区別して表示する(失敗にしない)。jq は使わない(gateway smoke と同じ理由)。
set -eu

base_url=${JUDGE_URL:-http://localhost:8080}
api_url=${API_URL:-http://localhost:8080}

# 架空の UUID(版 4 の形。16進のみ)。judge は gateway と同じ端末ID/セッションIDの形を要求する。
device_id=00000000-0000-4000-8000-0000000a0001
session_id=00000000-0000-4000-8000-0000000a0002

body_file=$(mktemp)
list_file=$(mktemp)
trap 'rm -f "$body_file" "$list_file"' EXIT

status=000

fail() {
  echo "judge smoke: $1 (HTTP $status)" >&2
  cat "$body_file" >&2
  echo >&2
  exit 1
}

# request METHOD PATH BASE [BODY] [HEADER_MODE]
#   HEADER_MODE: valid(既定)/ none(端末ID・セッションIDを付けない)
request() {
  method=$1
  path=$2
  base=$3
  data=${4:-}
  header_mode=${5:-valid}
  set -- -sS --connect-timeout 5 --max-time 30 -o "$body_file" -w '%{http_code}' -X "$method" -H 'Content-Type: application/json'
  case "$header_mode" in
    valid) set -- "$@" -H "X-Device-Id: $device_id" -H "X-Session-Id: $session_id" ;;
    none) ;;
    *) echo "judge smoke: unknown header mode $header_mode" >&2; exit 2 ;;
  esac
  if [ -n "$data" ]; then
    set -- "$@" --data "$data"
  fi
  status=$(curl "$@" "$base$path") || true
  : "${status:=000}"
}

# ロールアウト直後、Traefik が終了中の Pod に振り分けて 000/502 を返すことがあるので、それらだけ再試行する
# (services/gateway/scripts/smoke.sh と同じ理由。404・422・503 は意味のある最終状態なので再試行しない)。
request_with_retry() {
  attempt=0
  while :; do
    request "$@"
    case "$status" in
      000|502) ;;
      *) return 0 ;;
    esac
    attempt=$((attempt + 1))
    if [ "$attempt" -ge 30 ]; then
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

expect_error() {
  expect_status "$1" "$3"
  if ! grep -qF "\"code\":\"$2\"" "$body_file"; then
    fail "$3: body is missing code $2"
  fi
}

split_objects() {
  printf '%s' "$1" | tr -d ' \n' | sed -e 's/^\[//' -e 's/\]$//' -e 's/},{/}\n{/g' >"$list_file"
  printf '\n' >>"$list_file"
}

# 0. healthz(既存の疎通確認。SP0/JD0 から変わらない)。
request_with_retry GET /api/judge/healthz "$base_url"
expect_status 200 "GET /api/judge/healthz"
if ! grep -qF '"status":"ok"' "$body_file"; then
  fail "GET /api/judge/healthz: body is missing status:ok"
fi

# 1. GET /api/pokedex/natures(gateway 経由)を叩いて ID の入手元を見分ける
#    (services/gateway/scripts/smoke.sh §0 と同じ流儀)。
request_with_retry GET /api/pokedex/natures "$api_url"
pokedex_status=$status

master_source=""
case "$pokedex_status" in
  200)
    master_source=pokedex
    natures_body=$(cat "$body_file")
    ;;
  503)
    master_source=example
    ;;
  *)
    fail "GET /api/pokedex/natures (via gateway): want 200 か 503"
    ;;
esac

if [ "$master_source" = pokedex ]; then
  # 性格: 無補正(plus キーを持たない)の先頭の id。
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
    fail "GET /api/pokedex/natures: 無補正の性格が無い"
  fi

  # 種族: species?limit=1 の先頭の key(攻撃側・防御側の両方に使う)。
  request_with_retry GET "/api/pokedex/species?limit=1" "$api_url"
  expect_status 200 "GET /api/pokedex/species (via gateway)"
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
    fail "GET /api/pokedex/species: 種族が空"
  fi

  # 技: moves?limit=200 のうち category=physical・威力 0 でないものの id を先頭から最大10件
  #    (services/gateway/scripts/smoke.sh と同じ探し方。タイプ相性で無効化される組み合わせを避ける)。
  request_with_retry GET "/api/pokedex/moves?limit=200" "$api_url"
  expect_status 200 "GET /api/pokedex/moves (via gateway)"
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
    fail "GET /api/pokedex/moves: 威力のある物理技が無い"
  fi

  attacker="{\"speciesKey\":\"$species_key\",\"natureId\":\"$nature_id\",\"sp\":{\"hp\":0,\"atk\":32,\"def\":0,\"spa\":0,\"spd\":0,\"spe\":32}}"
  defender_sp='{"hp":32,"atk":0,"def":0,"spa":0,"spd":0,"spe":0}'
  candidates=$move_ids
else
  echo "judge smoke: pokedex-svc の DB が未投入('$pokedex_status')。'make import-k8s' を実行してから再試行すること" >&2
  # 例のマスタの架空 ID(services/calc/calctest・services/gateway/scripts/smoke.sh と同じ値)。
  attacker='{"speciesKey":"9001-000","natureId":"testatkup","sp":{"hp":0,"atk":32,"def":0,"spa":0,"spd":0,"spe":32}}'
  species_key=9002-000
  defender_sp='{"hp":32,"atk":0,"def":0,"spa":0,"spd":0,"spe":0}'
  candidates=testbeam
fi

# 2. POST /api/judge/v1/outspeed-and-ko: 候補の技を先頭から試し、200 になった最初の技を採用する
#    (タイプ相性で無効化される・威力0などで判定できない組み合わせを避ける)。
move_id=""
set -f
set -- $candidates
set +f
for candidate in "$@"; do
  candidate_body="{\"speciesKey\":\"$species_key\",\"natureId\":$([ "$master_source" = pokedex ] && printf '"%s"' "$nature_id" || printf '"testneutrala"'),\"sp\":$defender_sp,\"moveId\":\"$candidate\"}"
  body='{"format":"single","attacker":'"$attacker"',"defenders":['"$candidate_body"'],"moveId":"'"$candidate"'"}'
  request_with_retry POST /api/judge/v1/outspeed-and-ko "$base_url" "$body"
  if [ "$status" = 200 ]; then
    move_id=$candidate
    break
  fi
done
if [ -z "$move_id" ]; then
  fail "POST /api/judge/v1/outspeed-and-ko: 候補の技(最大10件・例のマスタなら1件)のどれも 200 にならなかった"
fi
expect_status 200 "POST /api/judge/v1/outspeed-and-ko"
if ! grep -qF '"attackerKo":{' "$body_file"; then
  fail "POST /api/judge/v1/outspeed-and-ko: matchups[0].attackerKo が無い"
fi
hits=$(tr -d ' \n' <"$body_file" | sed -n 's/.*"attackerKo":{[^}]*"hits":\([0-9][0-9]*\).*/\1/p')
if [ -z "$hits" ]; then
  fail "POST /api/judge/v1/outspeed-and-ko: attackerKo.hits が読み取れない"
fi

# 3. ヘッダなしは 400 invalid_request(judge は gateway と違い missing_header を分けない。
#    services/judge/internal/httpapi/server.go の requireRequestContext)。
request_with_retry POST /api/judge/v1/outspeed-and-ko "$base_url" "$body" none
expect_error 400 invalid_request "POST /api/judge/v1/outspeed-and-ko without device/session headers"

# 4. 未知の speciesKey は 422 unknown_species(pokedex-svc に到達できているときだけ検証できる。
#    未投入なら pokedex-svc 自体に到達できず 503 になり区別できないため、pokedex 未投入時はスキップする)。
unknown_species_result=skipped
if [ "$master_source" = pokedex ]; then
  unknown_attacker="{\"speciesKey\":\"0000-999\",\"natureId\":\"$nature_id\",\"sp\":{\"hp\":0,\"atk\":32,\"def\":0,\"spa\":0,\"spd\":0,\"spe\":32}}"
  unknown_body='{"format":"single","attacker":'"$unknown_attacker"',"defenders":['"$candidate_body"'],"moveId":"'"$move_id"'"}'
  request_with_retry POST /api/judge/v1/outspeed-and-ko "$base_url" "$unknown_body"
  expect_error 422 unknown_species "POST /api/judge/v1/outspeed-and-ko with an unknown speciesKey"
  unknown_species_result=422
fi

# 5. defenders が 7 件(上限6件超)は 400 invalid_request(上流を呼ぶ前に検査する。ADR-0703 §1・§4)。
seven_defenders=$(i=0; out=""; while [ "$i" -lt 7 ]; do out="$out$candidate_body,"; i=$((i + 1)); done; printf '%s' "$out" | sed 's/,$//')
too_many_body='{"format":"single","attacker":'"$attacker"',"defenders":['"$seven_defenders"'],"moveId":"'"$move_id"'"}'
request_with_retry POST /api/judge/v1/outspeed-and-ko "$base_url" "$too_many_body"
expect_error 400 invalid_request "POST /api/judge/v1/outspeed-and-ko with 7 defenders (max is 6)"

if [ "$master_source" = pokedex ]; then
  echo "judge smoke: master=pokedex species=$species_key move=$move_id nature=$nature_id"
else
  echo "judge smoke: master=example"
fi
echo "judge smoke: health=200 outspeed=200 (hits=$hits) missing_headers=400 unknown_species=$unknown_species_result too_many_defenders=400"
