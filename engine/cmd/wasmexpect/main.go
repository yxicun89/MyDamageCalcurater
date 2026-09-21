// Command wasmexpect は Go/WASM 一致テストの期待値を「ネイティブ Go」で生成する。
//
// 同じベクタ(先頭の typeChart を各リクエストへ注入したもの)を engine/wasmapi へ通した結果(レスポンス JSON 文字列)を
// {"<ベクタ名>": "<レスポンス JSON>"} の1ファイルに書き出す。
// scripts/wasm-conformance.mjs がこれと WASM の出力をバイト比較する(ADR-0011 §7)。
//
// 期待値はコミットしない。実行のたびに生成するので、陳腐化も SHA 管理も要らない。
// 「WASM が engine と同じ結果を返すか」だけを見る道具であって、計算の正しさは
// ゴールデンテスト(make test-golden)が担保する。
//
//	go run ./cmd/wasmexpect -vectors wasmapi/testdata/vectors.json -out /tmp/expected.json
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"example.com/pokecalc/engine/wasmapi"
)

type vector struct {
	Name    string          `json:"name"`
	Fn      string          `json:"fn"`
	Request json.RawMessage `json:"request"`
}

// vectorSchemaVersion は読めるベクタの版。2 でファイル先頭の typeChart を各リクエストへ注入する
// (ADR-0011 §13)。注入を持たない旧版のベクタは「未知の schemaVersion」として拒否する。
const vectorSchemaVersion = 2

type vectorFile struct {
	SchemaVersion int `json:"schemaVersion"`
	// TypeChart はファイル先頭で1度だけ定義したタイプ相性表。各リクエストの typeChart として注入する。
	TypeChart json.RawMessage `json:"typeChart"`
	Vectors   []vector        `json:"vectors"`
}

func main() {
	vectorsPath := flag.String("vectors", "wasmapi/testdata/vectors.json", "入力ベクタの JSON")
	outPath := flag.String("out", "", "期待値の出力先(必須)")
	flag.Parse()

	if *outPath == "" {
		fmt.Fprintln(os.Stderr, "wasmexpect: -out は必須")
		os.Exit(2)
	}
	if err := run(*vectorsPath, *outPath); err != nil {
		fmt.Fprintln(os.Stderr, "wasmexpect:", err)
		os.Exit(1)
	}
}

// requestWithTypeChart はリクエストに共有の typeChart を足した JSON 文字列を返す。
// リクエストに typeChart が直書きされていたら、表を1か所で定義する方針に反するので失敗にする。
func requestWithTypeChart(v vector, typeChart json.RawMessage) (string, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(v.Request, &fields); err != nil {
		return "", fmt.Errorf("ベクタ %q のリクエストが JSON オブジェクトでない: %w", v.Name, err)
	}
	if _, dup := fields["typeChart"]; dup {
		return "", fmt.Errorf("ベクタ %q に typeChart が直書きされている(表はファイル先頭で1度だけ定義する)", v.Name)
	}
	fields["typeChart"] = typeChart
	b, err := json.Marshal(fields)
	if err != nil {
		return "", fmt.Errorf("ベクタ %q のリクエストを組み立てられない: %w", v.Name, err)
	}
	return string(b), nil
}

func run(vectorsPath, outPath string) error {
	b, err := os.ReadFile(vectorsPath)
	if err != nil {
		return fmt.Errorf("ベクタを読めない: %w", err)
	}
	var f vectorFile
	if err := json.Unmarshal(b, &f); err != nil {
		return fmt.Errorf("ベクタの JSON が壊れている: %w", err)
	}
	if f.SchemaVersion != vectorSchemaVersion {
		return fmt.Errorf("ベクタの schemaVersion=%d は未知(%d のみ対応)", f.SchemaVersion, vectorSchemaVersion)
	}
	if len(f.TypeChart) == 0 || string(f.TypeChart) == "null" {
		return fmt.Errorf("ベクタ先頭の typeChart が無い")
	}
	if len(f.Vectors) == 0 {
		return fmt.Errorf("ベクタが空")
	}

	out := make(map[string]string, len(f.Vectors))
	for _, v := range f.Vectors {
		if _, dup := out[v.Name]; dup {
			return fmt.Errorf("ベクタ名が重複している: %q", v.Name)
		}
		req, err := requestWithTypeChart(v, f.TypeChart)
		if err != nil {
			return err
		}
		switch v.Fn {
		case "calc":
			out[v.Name] = wasmapi.Calc(req)
		case "calcBulk":
			out[v.Name] = wasmapi.CalcBulk(req)
		case "calcReverse":
			out[v.Name] = wasmapi.CalcReverse(req)
		default:
			return fmt.Errorf("ベクタ %q の fn=%q は未知", v.Name, v.Fn)
		}
	}

	enc, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("期待値を JSON にできない: %w", err)
	}
	if err := os.WriteFile(outPath, append(enc, '\n'), 0o644); err != nil {
		return fmt.Errorf("期待値を書けない: %w", err)
	}
	fmt.Fprintf(os.Stderr, "wasmexpect: %d 件の期待値を %s に書いた\n", len(out), outPath)
	return nil
}
