#!/usr/bin/env bash
# xcode-test-lock.sh の自動テスト(F-15)。ロックの置き場を一時ディレクトリに向けて、取得・解放・待ち・古いロックの解除を確かめる。
set -uo pipefail

SRC="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FAILURES=0
PASSES=0
ok() { PASSES=$((PASSES + 1)); }
ng() {
  printf '  NG: %s\n' "$1" >&2
  FAILURES=$((FAILURES + 1))
}

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
export IOS_TEST_LOCK_DIR="$WORK/lock"
export IOS_TEST_LOCK_POLL=1

# 各ケースは別プロセス(サブシェルではなく bash -c)で動かす。ロックの所有者は $$ なので、プロセスごとに独立させる。
run_acquire() { # 引数: 待つ秒数 表示名。標準出力に結果
  IOS_TEST_LOCK_TIMEOUT="$1" bash -c '
    source "$0/xcode-test-lock.sh"
    if xcode_test_lock_acquire "$1" 2>"$2"; then
      echo acquired
      xcode_test_lock_release
    else
      echo timeout
    fi
  ' "$SRC" "$2" "$WORK/stderr.txt"
}

echo "- 空いていれば取れて、終われば解放される"
[ "$(run_acquire 5 a)" = acquired ] && ok || ng "空きのロックを取れなかった"
[ -d "$IOS_TEST_LOCK_DIR" ] && ng "解放後にロックが残っている" || ok

echo "- 生きているプロセスが持っている間は待ち、時間切れで失敗する"
sleep 30 &
HOLDER=$!
mkdir "$IOS_TEST_LOCK_DIR"
printf '%s' "$HOLDER" >"$IOS_TEST_LOCK_DIR/pid"
printf 'other(/x)' >"$IOS_TEST_LOCK_DIR/info"
RESULT="$(run_acquire 2 b)"
[ "$RESULT" = timeout ] && ok || ng "保持者が生きているのに取れた/待たなかった: $RESULT"
grep -q "other(/x)" "$WORK/stderr.txt" && ok || ng "保持者の情報を表示していない: $(cat "$WORK/stderr.txt")"
[ -d "$IOS_TEST_LOCK_DIR" ] && ok || ng "待っただけなのに他人のロックを消した"

echo "- 保持者が死んでいれば解除して取れる"
kill "$HOLDER" 2>/dev/null
wait "$HOLDER" 2>/dev/null
RESULT="$(run_acquire 5 c)"
[ "$RESULT" = acquired ] && ok || ng "死んだ保持者のロックを解除できなかった: $RESULT"
grep -q "解除" "$WORK/stderr.txt" && ok || ng "解除したことを表示していない"

echo "- 他人のロックは解放しない(pid が違うとき)"
mkdir "$IOS_TEST_LOCK_DIR"
printf '%s' "999999" >"$IOS_TEST_LOCK_DIR/pid"
bash -c 'source "$0/xcode-test-lock.sh"; XCODE_TEST_LOCK_OWNER_PID=1; xcode_test_lock_release' "$SRC"
[ -d "$IOS_TEST_LOCK_DIR" ] && ok || ng "自分のものでないロックを解放した"
rm -rf "$IOS_TEST_LOCK_DIR"

echo "- IOS_TEST_LOCK_DISABLE=1 ではロックしない"
mkdir "$IOS_TEST_LOCK_DIR"
printf '%s' "$$" >"$IOS_TEST_LOCK_DIR/pid"
RESULT="$(IOS_TEST_LOCK_DISABLE=1 run_acquire 1 d)"
[ "$RESULT" = acquired ] && ok || ng "DISABLE でも待った: $RESULT"
rm -rf "$IOS_TEST_LOCK_DIR"

echo "- 同時に2つ取りに行っても、同時に持つのは1つだけ"
COUNTER="$WORK/counter"
: >"$COUNTER"
for i in 1 2 3; do
  bash -c '
    source "$0/xcode-test-lock.sh"
    xcode_test_lock_acquire "p$1" 2>/dev/null || exit 1
    # 保持中に他の誰かが保持していたら記録する(同じファイルに +1 してから -1)。
    n=$(wc -l <"$2"); echo x >>"$2"
    [ "$n" -ne 0 ] && echo overlap >>"$2.err"
    sleep 1
    : >"$2"
    xcode_test_lock_release
  ' "$SRC" "$i" "$COUNTER" &
done
wait
[ -f "$COUNTER.err" ] && ng "同時に複数が保持した" || ok

printf '\n'
if [ "$FAILURES" -eq 0 ]; then
  printf 'xcode-test-lock test: all %s checks passed\n' "$PASSES"
else
  printf 'xcode-test-lock test: %s failed, %s passed\n' "$FAILURES" "$PASSES" >&2
  exit 1
fi
