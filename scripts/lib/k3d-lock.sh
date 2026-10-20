#!/usr/bin/env bash
# k3d の共有クラスタを変更するスクリプト(deploy・up・bootstrap 等)を、同じクラスタに対して同時に1つだけにする排他ロック。
#
# 背景: 複数のセッションが同時に `k3d image import` を流すと、k3d-tools コンテナの名前が衝突して失敗する。
# macOS に flock が無いので、ios/scripts/xcode-test-lock.sh と同じ mkdir ロックにしている。
#
# 使い方(各スクリプトの先頭、set の直後・cd の前。source した関数が元のスクリプトを子として流し直す):
#   source "$(dirname "$0")/lib/k3d-lock.sh"
#   k3d_lock_run <表示名> "$0" "$@"
# 取得できたら子として本体を実行し、終了(正常・異常・INT/TERM)でロックを解放して子の終了コードで抜ける。
# ロックを持った状態で呼ばれた子スクリプト(deploy-latest → k3d-m2-deploy 等)は K3D_LOCK_HELD により素通りする。
# trap を本体に差し込まないので、本体側の EXIT trap と衝突しない。
#
# 環境変数: K3D_SCRIPT_LOCK_DIR(既定 /tmp/pokecalc-k3d-<CLUSTER>.lock。リポジトリ外・クラスタ単位)・
#           K3D_SCRIPT_LOCK_TIMEOUT(待つ秒数。既定 1800)・K3D_SCRIPT_LOCK_POLL(確認の間隔。既定 5 秒)・
#           K3D_SCRIPT_LOCK_DISABLE=1(ロックしない)
# 保持者が死んでいる(プロセスが無い)ロックは自動で回収する。

k3d_lock_dir() { printf '%s' "${K3D_SCRIPT_LOCK_DIR:-/tmp/pokecalc-k3d-${CLUSTER:-pokecalc}.lock}"; }

# 自分(pid)が取ったロックだけを解放する。
k3d_lock_release() {
  local dir
  dir="$(k3d_lock_dir)"
  if [ -f "$dir/pid" ] && [ "$(cat "$dir/pid" 2>/dev/null)" = "${K3D_LOCK_OWNER_PID:-}" ]; then
    rm -rf "$dir"
  fi
}

k3d_lock_acquire() {
  local label="${1:-k3d}" dir waited=0 timeout poll holder_pid holder_info
  dir="$(k3d_lock_dir)"
  timeout="${K3D_SCRIPT_LOCK_TIMEOUT:-1800}"
  poll="${K3D_SCRIPT_LOCK_POLL:-5}"
  while ! mkdir "$dir" 2>/dev/null; do
    holder_pid="$(cat "$dir/pid" 2>/dev/null || true)"
    holder_info="$(cat "$dir/info" 2>/dev/null || true)"
    if [ -n "$holder_pid" ] && ! kill -0 "$holder_pid" 2>/dev/null; then
      # 読んでから消すまでに別のセッションが新しく取ることがあるので、別名へ移して(原子的)pid を確かめてから消す。
      if mv "$dir" "$dir.stale.$$" 2>/dev/null; then
        if [ "$(cat "$dir.stale.$$/pid" 2>/dev/null || true)" = "$holder_pid" ]; then
          echo "$label: 保持者(pid $holder_pid)が終了しているロックを解除します" >&2
          rm -rf "$dir.stale.$$"
        elif ! mv "$dir.stale.$$" "$dir" 2>/dev/null; then
          rm -rf "$dir.stale.$$"
        fi
      fi
      continue
    fi
    # mkdir 直後に pid を書く前の一瞬を除き、pid が書かれないまま 1 分以上残るロックは古いとみなす。
    if [ -z "$holder_pid" ] && [ -d "$dir" ] && [ -n "$(find "$dir" -maxdepth 0 -mmin +1 2>/dev/null)" ]; then
      echo "$label: pid の無い古いロックを解除します" >&2
      rm -rf "$dir"
      continue
    fi
    if [ "$waited" -ge "$timeout" ]; then
      echo "$label: 他の k3d 操作の完了を ${timeout} 秒待ちましたが、ロックが解けません(保持者: pid ${holder_pid:-?} ${holder_info:-不明})。" >&2
      echo "$label: 中断します。他に操作が動いていないことを確かめてから K3D_SCRIPT_LOCK_DISABLE=1 を付けて再実行してください" >&2
      return 1
    fi
    if [ $((waited % 30)) -eq 0 ]; then
      echo "$label: 他のセッションの k3d 操作が終わるのを待っています(${waited} 秒経過。保持者: pid ${holder_pid:-?} ${holder_info:-不明})。image import が衝突するため順番に実行します" >&2
    fi
    sleep "$poll"
    waited=$((waited + poll))
  done
  K3D_LOCK_OWNER_PID="$$"
  printf '%s' "$$" >"$dir/pid"
  printf '%s(%s)' "$label" "$(pwd)" >"$dir/info"
  return 0
}

# ロックを取って "$@"(元のスクリプト)を子として実行し、終了後に解放して子の終了コードで exit する。
# すでにロックを持つ親の子孫、または無効化されているときは何もせず戻る(呼び出し元がそのまま本体を続ける)。
k3d_lock_run() {
  local label="$1" status=0
  shift
  if [ "${K3D_LOCK_HELD:-0}" = "1" ] || [ "${K3D_SCRIPT_LOCK_DISABLE:-0}" = "1" ]; then
    return 0
  fi
  k3d_lock_acquire "$label" || exit 1
  trap 'k3d_lock_release; exit 143' INT TERM
  K3D_LOCK_HELD=1 "$@" || status=$?
  trap - INT TERM
  k3d_lock_release
  exit "$status"
}
