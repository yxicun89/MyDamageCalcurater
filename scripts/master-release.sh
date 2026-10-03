#!/usr/bin/env bash
# マスタ更新を calc・balance・speed へ一貫して反映する単一の入口(issue #108・#403 D21、ADR-0135)。
# リポジトリのルートで実行する(make master-release)。上から順に、どこで失敗しても非0で止まり、旧版の consumer を表示する。
#   1. 最新の import Job(CronJob pokedex-import 由来)の完了を待つ(Job は作らない。無ければ import-k8s を案内)
#   2. read model を export して検証(DSN は環境変数 POKEDEX_DATABASE_DSN。無ければ mysql へ port-forward して reader DSN を使う。値は表示しない)
#   3. export の dataVersion が動いている版と同じなら、再生成・rollout をせず「変化なし」で 0 終了
#   4. 違えば balance・speed の read model を入れ替え(Argo CD 管理なら飛ばして案内)、calc を rollout restart して readyz を待つ
#   5. 全 consumer の版一致(check-master-version.sh)→ smoke
# importer Pod に k8s の書き込み権限は与えない。この orchestration は手元の make から行う。
set -euo pipefail

cluster="${CLUSTER:-pokecalc}"
namespace="${NAMESPACE:-pokecalc}"
context="k3d-${cluster}"
readmodel_dir="${READMODEL_DIR:-data/generated/readmodel}"
job_timeout="${IMPORT_JOB_TIMEOUT:-600s}"
readyz_tries="${READYZ_TRIES:-30}"
readyz_sleep="${READYZ_SLEEP:-2}"
local_port="${MIGRATE_LOCAL_PORT:-13307}"

CLUSTER="$cluster" ./scripts/require-k3d-context.sh master-release
command -v jq >/dev/null 2>&1 || { echo "master-release: jq が必要です(make doctor で確認)" >&2; exit 2; }

exported=0
kc() { kubectl --context "$context" -n "$namespace" "$@"; }
check_versions() { CLUSTER="$cluster" NAMESPACE="$namespace" READMODEL_DIR="$readmodel_dir" ./scripts/check-master-version.sh; }

# 失敗を表示して止まる。export 済みなら、どの consumer が旧版かも出す(期待値が無いと比べられない)。
fail() {
  echo "master-release: 失敗($1)" >&2
  if [ "$exported" = 1 ]; then check_versions >&2 || true; fi
  exit 1
}

echo "== 1. 最新の import Job の完了を待つ"
jobs_json=$(kc get jobs -o json 2>/dev/null) || fail "import Job の一覧を取得できない"
job=$(printf '%s' "$jobs_json" | jq -r '[.items[] | select(any(.metadata.ownerReferences[]?; .kind == "CronJob" and .name == "pokedex-import"))] | sort_by(.metadata.creationTimestamp) | last | .metadata.name // empty')
if [ -z "$job" ]; then
  echo "master-release: import Job が無い。先に make import-k8s で import を流してから、もう一度 make master-release を実行する(docs/runbooks/data.md)" >&2
  exit 1
fi
echo "  最新の Job: $job"
state=$(printf '%s' "$jobs_json" | jq -r --arg n "$job" '.items[] | select(.metadata.name == $n) | if (.status.succeeded // 0) >= 1 then "complete" elif any(.status.conditions[]?; .type == "Failed" and .status == "True") then "failed" else "running" end')
case "$state" in
  complete) echo "  完了済み" ;;
  failed) fail "import Job $job が失敗している。kubectl -n $namespace logs job/$job を確認して直し、import をやり直す" ;;
  *)
    kc wait --for=condition=complete "job/$job" --timeout="$job_timeout" >/dev/null 2>&1 \
      || fail "import Job $job が $job_timeout 以内に完了しなかった(失敗または実行中)"
    ;;
esac

echo "== 2. read model を export して検証"
pf_pid=""
cleanup() { if [ -n "$pf_pid" ]; then kill "$pf_pid" 2>/dev/null || true; wait "$pf_pid" 2>/dev/null || true; fi; }
trap cleanup EXIT
if [ -z "${POKEDEX_DATABASE_DSN:-}" ]; then
  command -v nc >/dev/null 2>&1 || fail "nc が無い(port-forward の疎通確認に使う)"
  if nc -z 127.0.0.1 "$local_port" 2>/dev/null; then
    fail "127.0.0.1:$local_port は使用中。MIGRATE_LOCAL_PORT=<空きポート> を付けて再実行する"
  fi
  # 関数 kc を背景で呼ぶとサブシェルの PID になり、kill しても kubectl が残る。kubectl を直接背景で起動する。
  kubectl --context "$context" -n "$namespace" port-forward svc/mysql "$local_port:3306" >/dev/null 2>&1 &
  pf_pid=$!
  ready=0
  for _ in $(seq 1 20); do
    kill -0 "$pf_pid" 2>/dev/null || break
    if nc -z 127.0.0.1 "$local_port" 2>/dev/null; then ready=1; break; fi
    sleep 0.5
  done
  [ "$ready" = 1 ] || fail "mysql への port-forward が張れなかった"
  # reader DSN(SELECT のみ)の接続先を port-forward 先へ付け替える。値は表示しない。
  dsn=$(kc get secret mysql-auth -o jsonpath='{.data.pokedex-reader-dsn}' | base64 -d | sed -E "s/@tcp\(mysql:[0-9]+\)/@tcp(127.0.0.1:$local_port)/") \
    || fail "Secret mysql-auth の pokedex-reader-dsn を読めない"
  case "$dsn" in
    *"@tcp(127.0.0.1:$local_port)"*) ;;
    *) fail "pokedex-reader-dsn の接続先が想定(mysql:<port>)と違う" ;;
  esac
  export POKEDEX_DATABASE_DSN="$dsn"
  unset dsn
fi
make --no-print-directory pokedex-export || fail "read model の export に失敗"
cleanup
pf_pid=""
exported=1
readmodel_abs=$(cd "$readmodel_dir" && pwd)
for svc in balance speed; do
  (cd "services/$svc" && GOWORK=off go run ./cmd/checkreadmodel "$readmodel_abs") || fail "$svc の read model 検証に失敗"
done

echo "== 3. 動いている版との比較"
if check_versions; then
  echo "master-release: 変化なし(再生成・rollout はしない)"
  exit 0
fi
echo "  版が違う consumer がある。反映する"

echo "== 4. 反映(balance・speed の read model → calc の再起動)"
for svc in balance speed; do
  if kubectl --context "$context" -n argocd get applications.argoproj.io "pokecalc-$svc" -o name >/dev/null 2>&1; then
    echo "  $svc: Argo CD(Application pokecalc-$svc)が管理しているので飛ばす。docs/runbooks/$svc.md の sync で最新にする" >&2
    continue
  fi
  svc_upper=$(printf '%s' "$svc" | tr '[:lower:]' '[:upper:]')
  make --no-print-directory "$svc-k3d-deploy-readmodel" "${svc_upper}_READMODEL_DIR=$readmodel_dir" || fail "$svc の read model の入れ替えに失敗"
done
kc rollout restart deployment/calc >/dev/null || fail "calc の rollout restart に失敗"
kc rollout status deployment/calc --timeout=180s || fail "calc の rollout が完了しない"
readyz_ok=0
for _ in $(seq 1 "$readyz_tries"); do
  if kubectl --context "$context" get --raw "/api/v1/namespaces/${namespace}/services/calc:http/proxy/readyz" >/dev/null 2>&1; then
    readyz_ok=1
    break
  fi
  sleep "$readyz_sleep"
done
[ "$readyz_ok" = 1 ] || fail "calc の /readyz が Ready にならない(マスタ未読み込みの可能性)"

echo "== 5. 全 consumer の版一致 → smoke"
check_versions || { echo "master-release: 失敗(版が揃っていない consumer がある)" >&2; exit 1; }
API_SMOKE_STRICT=1 make --no-print-directory api-smoke || fail "api-smoke に失敗"
make --no-print-directory balance-smoke-readmodel || fail "balance の readmodel smoke に失敗"
make --no-print-directory speed-smoke-readmodel || fail "speed の readmodel smoke に失敗"
echo "master-release: 完了(calc・balance・speed が同じ dataVersion で、smoke も成功)"
