package main

// 期待値生成(wasmexpect)が調整の4関数(ADR-0250)を扱えることの検査。
//
// 一致テスト(scripts/wasm-conformance.mjs)は wasmexpect の出力を「正」として WASM と比べるので、
// wasmexpect が新しい fn を知らない・相性表の注入を誤る(adjustIndices に注入すると unknown_field)と、
// 一致テストが動かないか、両側が同じエラー封筒で一致して空振りする。ここでネイティブ側だけで先に落とす。

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const repoVectors = "../../wasmapi/testdata/vectors.json"

func TestRunHandlesAdjustVectors(t *testing.T) {
	out := filepath.Join(t.TempDir(), "expected.json")
	if err := run(repoVectors, out); err != nil {
		t.Fatalf("run(ベクタ) = %v(調整の fn を知らない?)", err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("期待値を読めない: %v", err)
	}
	var expected map[string]string
	if err := json.Unmarshal(b, &expected); err != nil {
		t.Fatalf("期待値が JSON でない: %v", err)
	}

	raw, err := os.ReadFile(repoVectors)
	if err != nil {
		t.Fatalf("ベクタを読めない: %v", err)
	}
	var f vectorFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("ベクタが JSON でない: %v", err)
	}
	seen := map[string]int{}
	for _, v := range f.Vectors {
		switch v.Fn {
		case "adjustIndices", "adjustMinSpToKo", "adjustMinSpToSurvive", "adjustAllocation":
		default:
			continue
		}
		seen[v.Fn]++
		var env map[string]json.RawMessage
		if err := json.Unmarshal([]byte(expected[v.Name]), &env); err != nil {
			t.Errorf("%s: 期待値が JSON でない: %v", v.Name, err)
			continue
		}
		if env["result"] == nil {
			t.Errorf("%s: 期待値が成功封筒でない(fn の呼び分け・typeChart の注入を確かめる): %s", v.Name, expected[v.Name])
		}
	}
	for _, fn := range []string{"adjustIndices", "adjustMinSpToKo", "adjustMinSpToSurvive", "adjustAllocation"} {
		if seen[fn] == 0 {
			t.Errorf("fn=%s のベクタが無い", fn)
		}
	}
}

// adjustIndices のリクエストには相性表を注入しない(契約に typeChart が無い。ADR-0250 §4)。
// 他の fn には従来どおり注入する。
func TestRequestWithTypeChartSkipsAdjustIndices(t *testing.T) {
	chart := json.RawMessage(`{"types":["normal"]}`)
	got, err := requestWithTypeChart(vector{Name: "x", Fn: "adjustIndices", Request: json.RawMessage(`{"individual":{}}`)}, chart)
	if err != nil {
		t.Fatalf("requestWithTypeChart = %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(got), &m); err != nil {
		t.Fatalf("JSON でない: %v", err)
	}
	if _, ok := m["typeChart"]; ok {
		t.Errorf("adjustIndices に typeChart を注入した: %s", got)
	}

	got, err = requestWithTypeChart(vector{Name: "y", Fn: "adjustAllocation", Request: json.RawMessage(`{"self":{}}`)}, chart)
	if err != nil {
		t.Fatalf("requestWithTypeChart = %v", err)
	}
	m = nil
	if err := json.Unmarshal([]byte(got), &m); err != nil {
		t.Fatalf("JSON でない: %v", err)
	}
	if _, ok := m["typeChart"]; !ok {
		t.Errorf("adjustAllocation に typeChart を注入していない: %s", got)
	}
}
