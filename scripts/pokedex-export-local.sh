#!/usr/bin/env bash
# k3d の mysql から balance/speed 向けの read model(data/generated/readmodel/ の6ファイル)を書き出す。
# docs/verify-m1.md §3 の「port-forward → pokedex_reader の DSN を読む → make pokedex-export」を1コマンドにしたもの
# (DSN は Secret mysql-auth の pokedex-reader-dsn から読み、画面にも履歴にも出さない。SELECT のみのロール。ADR-0110)。
# 前提: make up 済み・マスタ投入済み(make import-k8s)。出力先は .gitignore 済み(ADR-0002。実データは Git に入れない)。
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

CLUSTER=${CLUSTER:-pokecalc}
LOCAL_PORT=${EXPORT_LOCAL_PORT:-13307}
OUT_DIR=${READMODEL_DIR:-data/generated/readmodel}

CLUSTER="$CLUSTER" ./scripts/require-k3d-context.sh pokedex-export-k3d

command -v nc >/dev/null 2>&1 || { echo "pokedex-export-local: nc が無い(port-forward の疎通確認に使う)" >&2; exit 1; }
if nc -z 127.0.0.1 "$LOCAL_PORT" 2>/dev/null; then
  echo "pokedex-export-local: 127.0.0.1:$LOCAL_PORT は別のプロセスが使用中。EXPORT_LOCAL_PORT=<空きポート> を付けて再実行する" >&2
  exit 1
fi

kubectl -n pokecalc port-forward svc/mysql "$LOCAL_PORT:3306" >/dev/null 2>&1 &
pf_pid=$!
cleanup() {
  kill "$pf_pid" 2>/dev/null || true
  wait "$pf_pid" 2>/dev/null || true
}
trap cleanup EXIT

ready=0
for _ in $(seq 1 20); do
  if ! kill -0 "$pf_pid" 2>/dev/null; then break; fi
  if nc -z 127.0.0.1 "$LOCAL_PORT" 2>/dev/null; then ready=1; break; fi
  sleep 0.5
done
if [ "$ready" != 1 ]; then
  echo "pokedex-export-local: mysql への port-forward が張れなかった(kubectl -n pokecalc get pods で mysql-0 が Running か確認する)" >&2
  exit 1
fi

dsn=$(kubectl -n pokecalc get secret mysql-auth -o jsonpath='{.data.pokedex-reader-dsn}' | base64 -d \
  | sed -E "s/@tcp\(mysql:[0-9]+\)/@tcp(127.0.0.1:$LOCAL_PORT)/")
case "$dsn" in
  *"@tcp(127.0.0.1:$LOCAL_PORT)"*) ;;
  "") echo "pokedex-export-local: Secret mysql-auth に pokedex-reader-dsn が無い(make up で作り直す。docs/runbooks/data.md)" >&2; exit 1 ;;
  *) echo "pokedex-export-local: pokedex-reader-dsn の接続先が想定(mysql:<port>)と違うので付け替えられない" >&2; exit 1 ;;
esac

POKEDEX_DATABASE_DSN="$dsn" make --no-print-directory pokedex-export
unset dsn
ls "$OUT_DIR"
