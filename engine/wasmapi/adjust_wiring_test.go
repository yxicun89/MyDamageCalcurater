package wasmapi_test

// 調整の4関数(ADR-0250 §1)の WASM への配線。engine/cmd/wasm は js && wasm でしかビルドされないので、
// 登録と Node ハーネスの前提はソースの文字列で固定する(wiring_test.go と同じやり方)。

import (
	"strings"
	"testing"
)

// adjustWasmFns は JS のグローバル名と wasmapi の関数名の組(ADR-0250 §1 の表)。
var adjustWasmFns = []struct{ js, goFn string }{
	{"adjustIndices", "wasmapi.AdjustIndices"},
	{"adjustMinSpToKo", "wasmapi.AdjustMinSPToKO"},
	{"adjustMinSpToSurvive", "wasmapi.AdjustMinSPToSurvive"},
	{"adjustAllocation", "wasmapi.AdjustAllocation"},
}

func TestAdjustFunctionsAreRegisteredInWasm(t *testing.T) {
	src := readRepoFile(t, "engine/cmd/wasm/main_js.go")
	for _, f := range adjustWasmFns {
		want := `api.Set("` + f.js + `", register(` + f.goFn + `))`
		if !strings.Contains(src, want) {
			t.Errorf("engine/cmd/wasm/main_js.go に %s が無い(globalThis.pokecalc.%s を登録すること)", want, f.js)
		}
		// パッケージ doc の登録一覧(JS から見える契約)にも載せる。
		if !strings.Contains(src, f.js+"(requestJSON: string): string") {
			t.Errorf("engine/cmd/wasm/main_js.go の doc の登録一覧に %s が無い", f.js)
		}
	}
	// 登録完了の目印は最後に立てる(既存の約束を崩さない)。
	if i, j := strings.LastIndex(src, `api.Set("adjust`), strings.Index(src, `js.Global().Set("pokecalcReady", true)`); i < 0 || j < 0 || i > j {
		t.Error("調整の登録は pokecalcReady を立てる前に行うこと")
	}
}

func TestAdjustFunctionsAreCheckedByConformanceHarness(t *testing.T) {
	mjs := readRepoFile(t, "scripts/wasm-conformance.mjs")
	for _, f := range adjustWasmFns {
		if !strings.Contains(mjs, "'"+f.js+"'") {
			t.Errorf("scripts/wasm-conformance.mjs が %s の登録を確かめていない", f.js)
		}
	}
	// adjustIndices には相性表を注入しない(契約に typeChart が無い。ADR-0250 §4)。
	if !strings.Contains(mjs, "adjustIndices") || !strings.Contains(mjs, "typeChart") {
		t.Error("scripts/wasm-conformance.mjs に adjustIndices の typeChart 注入の扱いが無い")
	}
}
