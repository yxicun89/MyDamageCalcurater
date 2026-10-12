package readmodel_test

// ADR-0107 決定7: 技の追加効果は balance/speed の read model には**足さない**。
//
// 理由(ADR-0107):
//   - judge は read model のファイルを読まない(ADR-0700 §2・§4(4): リクエストごとに公開 API を呼ぶ)。
//     技の追加効果を運ぶのは内部 export の MasterMove.effect(→ calc-svc)。
//   - services/balance/internal/master/moves.go の LoadMoves は DisallowUnknownFields を使い、
//     services/balance/schema/moves.schema.json も additionalProperties: false。
//     moves.json にフィールドを足すと別レーン(タイプバランス)の読み込みがその場で壊れる。
//
// このテストは「良かれと思って moves.json に足す」のを止めるための番人。
// もし足す必要が出たら、先に ADR-0107 決定7 を更新し、タイプバランスレーンと合意すること。

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"

	"example.com/pokecalc/services/pokedex/internal/storetest"
)

func TestMovesReadModelEntryKeysAreFixed(t *testing.T) {
	files, _ := export(t, storetest.New())

	var doc struct {
		SchemaVersion int                          `json:"schemaVersion"`
		Moves         []map[string]json.RawMessage `json:"moves"`
	}
	if err := json.Unmarshal(files.Moves, &doc); err != nil {
		t.Fatalf("moves.json が読めない: %v", err)
	}
	if len(doc.Moves) == 0 {
		t.Fatal("moves.json が空(fixture の誤り)")
	}

	want := []string{"category", "moveId", "type"}
	for i, entry := range doc.Moves {
		got := make([]string, 0, len(entry))
		for k := range entry {
			got = append(got, k)
		}
		sort.Strings(got)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("moves[%d] のキー = %v, want %v\n"+
				"balance の LoadMoves は未知のキーを拒否する。技の追加効果は MasterMove.effect で運ぶ(ADR-0107 決定7)",
				i, got, want)
		}
	}
}
