package wasmapi_test

// P1-9 受け入れ条件 AC-9/AC-10 の検証(ビルドと一致テストの配線)。
//
// ここは engine の計算ではなく「`make wasm` と `make test-wasm` が仕様どおり存在するか」を見る。
// 一致テストそのものは Node 側(scripts/wasm-conformance.mjs)で走るので、Go からは
// 配線が欠けていないことだけを固定する。配線が無いまま緑になるのを防ぐのが目的
// (CLAUDE.md / docs/development-workflow.md「未実装ターゲットの正常終了を成功と数えない」)。

import (
	"os"
	"strings"
	"testing"
)

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile("../../" + rel)
	if err != nil {
		t.Fatalf("%s を読めない: %v", rel, err)
	}
	return string(b)
}

// TestWasmBuildScriptIsImplemented は scripts/wasm.sh が本実装されていることを確かめる。
func TestWasmBuildScriptIsImplemented(t *testing.T) {
	src := readRepoFile(t, "scripts/wasm.sh")

	for _, want := range []string{"set -euo pipefail", "GOOS=js", "GOARCH=wasm", "web/public", "wasm_exec.js", "engine.wasm"} {
		if !strings.Contains(src, want) {
			t.Errorf("scripts/wasm.sh に %q が無い(ADR-0011 §6)", want)
		}
	}
	// wasm_exec.js は Go の配布物から取る(手でコピーしたものを置かない)。
	if !strings.Contains(src, "GOROOT") {
		t.Error("scripts/wasm.sh は wasm_exec.js を $(go env GOROOT) 配下から取得すること(ADR-0011 §6)")
	}
	if strings.Contains(src, "P1-9 で") {
		t.Error("scripts/wasm.sh がスタブのまま(未実装の正常終了を成功と数えない)")
	}
}

// TestWasmConformanceTargetIsWired は `make wasm` / `make test-wasm` が Makefile にあることを確かめる。
func TestWasmConformanceTargetIsWired(t *testing.T) {
	mk := readRepoFile(t, "Makefile")

	for _, want := range []string{"\nwasm:", "\ntest-wasm:"} {
		if !strings.Contains(mk, want) {
			t.Errorf("Makefile に %q ターゲットが無い(ADR-0011 §7)", strings.Trim(want, "\n:"))
		}
	}
	if !strings.Contains(mk, "wasm-conformance") {
		t.Error("Makefile の test-wasm が scripts/wasm-conformance.mjs を呼んでいない(ADR-0011 §7)")
	}
	// 一致テストは make test には含めない(WASM ビルドと Node が要るため。ADR-0011 §7)。
	testTarget := sectionOf(mk, "\ntest:")
	if strings.Contains(testTarget, "test-wasm") {
		t.Error("make test が test-wasm を含んでいる。WASM 一致テストは make test-wasm 側(ADR-0011 §7)")
	}
}

// TestWasmConformanceHarnessExists は一致テストの構成要素が揃っていることを確かめる。
func TestWasmConformanceHarnessExists(t *testing.T) {
	for _, rel := range []string{
		"scripts/wasm-conformance.mjs",         // Node 側の一致テスト
		"engine/cmd/wasmexpect/main.go",        // ネイティブ Go の期待値生成
		"engine/cmd/wasm/main_js.go",           // WASM エントリ(js && wasm)
		"engine/wasmapi/testdata/vectors.json", // 入力ベクタ
	} {
		if _, err := os.Stat("../../" + rel); err != nil {
			t.Errorf("%s が無い: %v", rel, err)
		}
	}

	// 生成物はコミットしない(.gitignore に入れる。ADR-0011 §6)。
	ignore := readRepoFile(t, ".gitignore")
	for _, want := range []string{"web/public/engine.wasm", "web/public/wasm_exec.js"} {
		if !strings.Contains(ignore, want) {
			t.Errorf(".gitignore に %q が無い(WASM 生成物はコミットしない)", want)
		}
	}

	// Node や wasm_exec.js が無いときは「スキップして成功」にしないこと。
	mjs := readRepoFile(t, "scripts/wasm-conformance.mjs")
	if strings.Contains(mjs, "process.exit(0)") && !strings.Contains(mjs, "process.exitCode = 1") {
		t.Error("一致テストが前提不足で正常終了してはならない(必ず失敗させる)")
	}
}

// TestTypeChartInjectionIsWired は、ベクタ先頭の相性表を各リクエストへ注入する処理が
// 期待値生成(ネイティブ)と Node ハーネスの両方にあることを確かめる(ADR-0011 §13)。
//
// 一致テストは Go と WASM の**レスポンス**を比べるので、片側が注入を忘れると
// 「両方が同じ type_chart_missing」でも一致してしまいかねない。
// ベクタの schemaVersion を上げ、両側がそれを見ていることをここで固定する。
func TestTypeChartInjectionIsWired(t *testing.T) {
	for _, f := range []struct {
		rel  string
		want []string
	}{
		{"engine/cmd/wasmexpect/main.go", []string{"typeChart", "schemaVersion"}},
		{"scripts/wasm-conformance.mjs", []string{"typeChart", "schemaVersion"}},
	} {
		src := readRepoFile(t, f.rel)
		for _, want := range f.want {
			if !strings.Contains(src, want) {
				t.Errorf("%s に %q が無い(ベクタ先頭の相性表を各リクエストへ注入すること。ADR-0011 §13)", f.rel, want)
			}
		}
		// 旧 schemaVersion のままなら、注入を実装していない可能性が高い。
		if strings.Contains(src, "schemaVersion !== 1") || strings.Contains(src, "SchemaVersion != 1") {
			t.Errorf("%s がベクタの schemaVersion 1 を前提にしている(2 に上げた。ADR-0011 §13)", f.rel)
		}
	}
}

// sectionOf は Makefile から header で始まるレシピ部分を粗く切り出す。
func sectionOf(mk, header string) string {
	i := strings.Index(mk, header)
	if i < 0 {
		return ""
	}
	rest := mk[i+1:]
	if j := strings.Index(rest, "\n.PHONY"); j >= 0 {
		return rest[:j]
	}
	return rest
}
