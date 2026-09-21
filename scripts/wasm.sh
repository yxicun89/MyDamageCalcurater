#!/usr/bin/env bash
# engine を WASM にビルドして web/public へ配置する(ADR-0011 §6)。
#
#   web/public/engine.wasm    engine/cmd/wasm を GOOS=js GOARCH=wasm でビルドしたもの
#   web/public/wasm_exec.js   Go の配布物からコピーしたランタイム(手で置いたものは使わない)
#
# 生成物は .gitignore 済みでコミットしない。
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="$ROOT/web/public"

command -v go >/dev/null || { echo "wasm: go が見つからない" >&2; exit 1; }

# wasm_exec.js は Go 1.24 以降は lib/wasm、1.23 以前は misc/wasm にある。どちらも無ければ失敗する。
GOROOT_DIR="$(go env GOROOT)"
WASM_EXEC=""
for candidate in "$GOROOT_DIR/lib/wasm/wasm_exec.js" "$GOROOT_DIR/misc/wasm/wasm_exec.js"; do
  if [ -f "$candidate" ]; then
    WASM_EXEC="$candidate"
    break
  fi
done
if [ -z "$WASM_EXEC" ]; then
  echo "wasm: wasm_exec.js が見つからない($GOROOT_DIR/lib/wasm と misc/wasm を探した)" >&2
  exit 1
fi

mkdir -p "$OUT_DIR"
(cd "$ROOT/engine" && GOOS=js GOARCH=wasm go build -trimpath -o "$OUT_DIR/engine.wasm" ./cmd/wasm)
cp "$WASM_EXEC" "$OUT_DIR/wasm_exec.js"

bytes="$(wc -c <"$OUT_DIR/engine.wasm" | tr -d ' ')"
gz="$(gzip -9 -c "$OUT_DIR/engine.wasm" | wc -c | tr -d ' ')"
echo "wasm: web/public/engine.wasm ${bytes} bytes (gzip ${gz} bytes), wasm_exec.js を配置"
