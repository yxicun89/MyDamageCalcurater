#!/usr/bin/env bash
# scripts/lib/k3d-lock.sh(k3d 操作の排他ロック)の自動テスト。make test-scripts から流す。
# ロック関数だけを一時ディレクトリで検証する。クラスタ・k3d・kubectl には触らない。
set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
readonly ROOT
readonly LIB="$ROOT/scripts/lib/k3d-lock.sh"

failures=0
ok() { echo "ok: $1"; }
ng() { echo "NG: $1" >&2; failures=$((failures + 1)); }

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
export K3D_SCRIPT_LOCK_DIR="$work/cluster.lock"
export K3D_SCRIPT_LOCK_POLL=1
unset K3D_LOCK_HELD K3D_SCRIPT_LOCK_DISABLE

# 1. 取得するとロックができ、k3d_lock_run は子の終了コードを返して終了時に解放する
out=$(bash -c "source '$LIB'; k3d_lock_run t1 bash -c 'test -d \"\$K3D_SCRIPT_LOCK_DIR\" && exit 7'" 2>&1)
rc=$?
if [ "$rc" = 7 ] && [ ! -e "$K3D_SCRIPT_LOCK_DIR" ]; then ok "子の終了コードを返し、終了時にロックを解放する"; else ng "終了コード=$rc ロック残り=$([ -e "$K3D_SCRIPT_LOCK_DIR" ] && echo あり || echo なし) out=$out"; fi

# 2. ロックを持った子孫は素通りする(deploy-latest → m2-deploy のような入れ子で自己待ちしない)
bash -c "source '$LIB'; k3d_lock_run outer bash -c 'source \"$LIB\"; k3d_lock_run inner true; echo inner-ran'" >"$work/nested.out" 2>&1
rc=$?
if [ "$rc" = 0 ] && grep -q inner-ran "$work/nested.out" && ! grep -q 待って "$work/nested.out"; then ok "入れ子は素通りする"; else ng "入れ子 rc=$rc: $(cat "$work/nested.out")"; fi

# 3. 保持中の2つ目は待ち、解放後に取得する
mkdir "$K3D_SCRIPT_LOCK_DIR"
sleep 30 &
holder=$!
printf '%s' "$holder" >"$K3D_SCRIPT_LOCK_DIR/pid"
printf 'holder' >"$K3D_SCRIPT_LOCK_DIR/info"
( sleep 2; kill "$holder" 2>/dev/null; rm -rf "$K3D_SCRIPT_LOCK_DIR" ) &
out=$(K3D_SCRIPT_LOCK_TIMEOUT=20 bash -c "source '$LIB'; k3d_lock_run t3 echo got" 2>&1)
rc=$?
if [ "$rc" = 0 ] && grep -q "待っています" <<<"$out" && grep -q got <<<"$out"; then ok "2つ目は待機メッセージを出し、解放後に実行する"; else ng "待機 rc=$rc out=$out"; fi
wait 2>/dev/null

# 4. 生きた保持者のロックはタイムアウトで失敗し、ロックを壊さない
mkdir "$K3D_SCRIPT_LOCK_DIR"
sleep 30 &
holder=$!
printf '%s' "$holder" >"$K3D_SCRIPT_LOCK_DIR/pid"
out=$(K3D_SCRIPT_LOCK_TIMEOUT=2 bash -c "source '$LIB'; k3d_lock_run t4 echo should-not-run" 2>&1)
rc=$?
if [ "$rc" != 0 ] && ! grep -q should-not-run <<<"$out" && grep -q "ロックが解けません" <<<"$out" && [ -d "$K3D_SCRIPT_LOCK_DIR" ]; then ok "生きた保持者にはタイムアウトで失敗し、ロックは残る"; else ng "タイムアウト rc=$rc out=$out"; fi
kill "$holder" 2>/dev/null
rm -rf "$K3D_SCRIPT_LOCK_DIR"
wait 2>/dev/null

# 5. 死んだ保持者のロックは回収して取得する
mkdir "$K3D_SCRIPT_LOCK_DIR"
sleep 0.1 &
dead=$!
wait "$dead"
printf '%s' "$dead" >"$K3D_SCRIPT_LOCK_DIR/pid"
out=$(K3D_SCRIPT_LOCK_TIMEOUT=10 bash -c "source '$LIB'; k3d_lock_run t5 echo reclaimed" 2>&1)
rc=$?
if [ "$rc" = 0 ] && grep -q reclaimed <<<"$out" && grep -q "解除します" <<<"$out" && [ ! -e "$K3D_SCRIPT_LOCK_DIR" ]; then ok "死んだ保持者のロックを回収する"; else ng "回収 rc=$rc out=$out"; fi

# 5b. 死んだ保持者のロックがある状態で 6 並列に取りに行っても、同時に保持するのは常に 1 者(in/out が厳密に交互)
mkdir "$K3D_SCRIPT_LOCK_DIR"
sleep 0.1 &
dead=$!
wait "$dead"
printf '%s' "$dead" >"$K3D_SCRIPT_LOCK_DIR/pid"
log="$work/par.log"
: >"$log"
for _ in 1 2 3 4 5 6; do
  K3D_SCRIPT_LOCK_TIMEOUT=60 bash -c "source '$LIB'; k3d_lock_run par bash -c 'echo in >>\"$log\"; sleep 0.3; echo out >>\"$log\"'" >/dev/null 2>&1 &
done
wait
expected=$(printf 'in\nout\n%.0s' 1 2 3 4 5 6)
if [ "$(cat "$log")" = "$expected" ] && [ ! -e "$K3D_SCRIPT_LOCK_DIR" ] && [ ! -e "$K3D_SCRIPT_LOCK_DIR.reclaim" ]; then ok "回収を伴う 6 並列でも二重取得しない"; else ng "並列: $(tr '\n' ' ' <"$log") 残り=$(ls "$work")"; fi

# 6. SIGTERM で終了してもロックを解放する
bash -c "source '$LIB'; k3d_lock_run t6 sleep 30" >/dev/null 2>&1 &
runner=$!
for _ in 1 2 3 4 5 6 7 8 9 10; do [ -d "$K3D_SCRIPT_LOCK_DIR" ] && break; sleep 0.5; done
held=no
[ -d "$K3D_SCRIPT_LOCK_DIR" ] && held=yes
# 端末の Ctrl-C は前面のプロセスグループ全体に届く。ここでは本体(子)とラッパーの両方へ送って再現する。
kill -TERM "$runner" 2>/dev/null
pkill -TERM -P "$runner" 2>/dev/null || true
wait "$runner" 2>/dev/null
if [ "$held" = yes ] && [ ! -e "$K3D_SCRIPT_LOCK_DIR" ]; then ok "SIGTERM でもロックを解放する"; else ng "SIGTERM held=$held 残り=$([ -e "$K3D_SCRIPT_LOCK_DIR" ] && echo あり || echo なし)"; fi

# 7. K3D_SCRIPT_LOCK_DISABLE=1 ではロックを作らず素通りする
mkdir "$K3D_SCRIPT_LOCK_DIR"
printf '%s' "$$" >"$K3D_SCRIPT_LOCK_DIR/pid"
out=$(K3D_SCRIPT_LOCK_DISABLE=1 bash -c "source '$LIB'; k3d_lock_run t7 true; echo passed" 2>&1)
if grep -q passed <<<"$out"; then ok "DISABLE=1 では待たずに素通りする"; else ng "DISABLE out=$out"; fi
rm -rf "$K3D_SCRIPT_LOCK_DIR"

# 8. 既定のロックはリポジトリの外(一時ディレクトリ)で、クラスタ単位
def=$(env -u K3D_SCRIPT_LOCK_DIR CLUSTER=foo bash -c "source '$LIB'; k3d_lock_dir")
case "$def" in
  /tmp/pokecalc-k3d-foo.lock) ok "既定はクラスタ単位の /tmp 配下" ;;
  *) ng "既定のロック場所=$def" ;;
esac

if [ "$failures" -ne 0 ]; then
  echo "k3d-lock_test: $failures 件失敗" >&2
  exit 1
fi
echo "k3d-lock_test: すべて成功"
