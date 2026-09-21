//go:build js && wasm

// Command wasm はブラウザ(WASM)向けの計算エントリ。
//
// 役割は「JS のグローバルに関数を登録し、文字列を engine/wasmapi へ渡すだけ」の薄いラッパー。
// 変換・検証・エラー整形は engine/wasmapi にあり、ネイティブ Go でテストできる(ADR-0011 §2)。
//
// 起動時に次を登録してから待機する:
//
//	globalThis.pokecalc = {
//	  calc(requestJSON: string): string,         // wasmapi.Calc
//	  calcBulk(requestJSON: string): string,     // wasmapi.CalcBulk
//	  calcReverse(requestJSON: string): string,  // wasmapi.CalcReverse
//	}
//	globalThis.pokecalcReady = true              // 登録完了の目印(最後に立てる)
//
// ここにマスタ・計算・丸め・DTO 変換を書かない(書くなら engine/wasmapi 側。テストできる場所)。
package main

import (
	"encoding/json"
	"syscall/js"

	"example.com/pokecalc/engine/wasmapi"
)

// invalidArgsResponse は引数が「文字列1つ」でないときに返す封筒。wasmapi のエラー封筒と同じ形。
func invalidArgsResponse() string {
	b, _ := json.Marshal(map[string]map[string]string{"error": {
		"code":    wasmapi.CodeInvalidJSON,
		"message": "引数は JSON 文字列1つでなければならない",
	}})
	return string(b)
}

// internalResponse は回復した panic に返す封筒。
func internalResponse() string {
	b, _ := json.Marshal(map[string]map[string]string{"error": {
		"code":    wasmapi.CodeInternal,
		"message": "内部エラーが発生した",
	}})
	return string(b)
}

// register は fn を「文字列1つ → 文字列」の JS 関数にする。panic は JS へ出さない
// (Go の panic を投げると go.run の Promise が reject され、以後の呼び出しがすべて死ぬ)。
func register(fn func(string) string) js.Func {
	return js.FuncOf(func(_ js.Value, args []js.Value) (ret any) {
		defer func() {
			if r := recover(); r != nil {
				ret = internalResponse()
			}
		}()
		if len(args) != 1 || args[0].Type() != js.TypeString {
			return invalidArgsResponse()
		}
		return fn(args[0].String())
	})
}

func main() {
	api := js.Global().Get("Object").New()
	api.Set("calc", register(wasmapi.Calc))
	api.Set("calcBulk", register(wasmapi.CalcBulk))
	api.Set("calcReverse", register(wasmapi.CalcReverse))
	js.Global().Set("pokecalc", api)
	js.Global().Set("pokecalcReady", true)

	// コールバックを受け付け続けるため、プログラムを終了させない。
	select {}
}
