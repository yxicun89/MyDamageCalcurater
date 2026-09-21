//go:build !(js && wasm)

package main

// ネイティブ向けのガード。`make build` / `go test ./...` がこのディレクトリを
// 「ビルド対象のファイルが無い」で落とさないために置く。実体は main_js.go(js && wasm)。

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "engine/cmd/wasm は GOOS=js GOARCH=wasm 専用です。`make wasm` を使ってください。")
	os.Exit(1)
}
