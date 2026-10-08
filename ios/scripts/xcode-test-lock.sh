#!/usr/bin/env bash
# シミュレータでのテスト(xcodebuild test)を、この Mac で同時に1つだけにする排他ロック(F-15・I-ios-2)。
#
# 背景: 複数のセッション(レーン)が同じ Mac の同じシミュレータでテストを流すと、CPU を奪い合って
# XCUITest のランナーが途中で落ち(`Restarting after unexpected exit, crash, or test timeout`・
# `TEST INTERRUPTED`・終了コード 65/75)、失敗・ハング風に見える。ユーザーの実行でも起きた。
#
# 使い方(run-xcode-tests.sh から source する):
#   source ios/scripts/xcode-test-lock.sh
#   xcode_test_lock_acquire <表示名>   # 順番待ちして取る(終了時に自動で解放する)
#
# 環境変数: IOS_TEST_LOCK_DIR(既定 /tmp/pokecalc-ios-xcode-tests.lock)・
#           IOS_TEST_LOCK_TIMEOUT(待つ秒数。既定 10800 = 3 時間)・IOS_TEST_LOCK_POLL(確認の間隔。既定 10 秒)・
#           IOS_TEST_LOCK_DISABLE=1(ロックしない)
# 保持者が死んでいる(プロセスが無い)ロックは自動で解除する。ロックの置き場の中には pid と説明を書く。

xcode_test_lock_dir() { printf '%s' "${IOS_TEST_LOCK_DIR:-/tmp/pokecalc-ios-xcode-tests.lock}"; }

xcode_test_lock_release() {
  local dir
  dir="$(xcode_test_lock_dir)"
  # 自分が取ったロックだけを解放する(pid が自分のとき)。
  if [ -f "$dir/pid" ] && [ "$(cat "$dir/pid" 2>/dev/null)" = "${XCODE_TEST_LOCK_OWNER_PID:-}" ]; then
    rm -rf "$dir"
  fi
}

xcode_test_lock_acquire() {
  local label="${1:-xcode-test}" dir waited=0 timeout poll holder_pid holder_info
  if [ "${IOS_TEST_LOCK_DISABLE:-0}" = "1" ]; then
    return 0
  fi
  dir="$(xcode_test_lock_dir)"
  timeout="${IOS_TEST_LOCK_TIMEOUT:-10800}"
  poll="${IOS_TEST_LOCK_POLL:-10}"
  while ! mkdir "$dir" 2>/dev/null; do
    holder_pid="$(cat "$dir/pid" 2>/dev/null || true)"
    holder_info="$(cat "$dir/info" 2>/dev/null || true)"
    if [ -n "$holder_pid" ] && ! kill -0 "$holder_pid" 2>/dev/null; then
      echo "$label: 保持者(pid $holder_pid)が終了しているロックを解除します" >&2
      rm -rf "$dir"
      continue
    fi
    # mkdir の直後で pid をまだ書いていない一瞬を除き、pid が書かれないまま長く残るロックも古いとみなす。
    if [ -z "$holder_pid" ] && [ -d "$dir" ] && [ -n "$(find "$dir" -maxdepth 0 -mmin +1 2>/dev/null)" ]; then
      echo "$label: pid の無い古いロックを解除します" >&2
      rm -rf "$dir"
      continue
    fi
    if [ "$waited" -ge "$timeout" ]; then
      echo "$label: 他のテストの完了を ${timeout} 秒待ちましたが、ロックが解けません(保持者: pid ${holder_pid:-?} ${holder_info:-不明})。" >&2
      echo "$label: 中断します。続けるなら IOS_TEST_LOCK_DISABLE=1 を付けて再実行してください(同じ Mac で他のテストが動いていないことを確かめてから)" >&2
      return 1
    fi
    if [ $((waited % 60)) -eq 0 ]; then
      echo "$label: 他のセッションのテストが終わるのを待っています(${waited} 秒経過。保持者: pid ${holder_pid:-?} ${holder_info:-不明})。同じ Mac で XCUITest を同時に流すと途中で落ちるため、順番に実行します" >&2
    fi
    sleep "$poll"
    waited=$((waited + poll))
  done
  XCODE_TEST_LOCK_OWNER_PID="$$"
  printf '%s' "$$" >"$dir/pid"
  printf '%s(%s)' "$label" "$(pwd)" >"$dir/info"
  # 呼び出し元の EXIT trap を上書きしないよう、ここでは追加の関数を呼ぶだけにする(呼び出し側が release を trap に入れる)。
  return 0
}
